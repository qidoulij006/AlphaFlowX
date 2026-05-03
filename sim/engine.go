package sim

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type QueueModel string

const (
	// QueueModelTradeThrough only fills resting limit orders when the market trades through the order price.
	// It is conservative when only aggregate trades are available and historical queue depth is unknown.
	QueueModelTradeThrough QueueModel = "trade_through"
	// QueueModelTouch fills resting limit orders when the market trades at or through the order price.
	QueueModelTouch QueueModel = "touch"
)

type EngineConfig struct {
	Symbol         string
	InitialBalance float64
	MakerFeeRate   float64
	TakerFeeRate   float64
	QueueModel     QueueModel
	RecordFills    bool
}

type Engine struct {
	cfg       EngineConfig
	nextID    int64
	orders    map[string]*Order
	positions map[PositionSide]*Position
	fills     []Fill
	lastPrice float64
	startedAt int64
	endedAt   int64

	balance     float64
	realizedPnL float64
	fees        float64
}

func NewEngine(cfg EngineConfig) *Engine {
	if cfg.QueueModel == "" {
		cfg.QueueModel = QueueModelTradeThrough
	}
	return &Engine{
		cfg:    cfg,
		orders: map[string]*Order{},
		positions: map[PositionSide]*Position{
			PositionLong:  {Side: PositionLong},
			PositionShort: {Side: PositionShort},
		},
		balance: cfg.InitialBalance,
	}
}

func (e *Engine) SubmitLimit(intent OrderIntent) (*Order, error) {
	if err := e.validateOrderIntent(intent); err != nil {
		return nil, err
	}
	if intent.Price <= 0 {
		return nil, fmt.Errorf("limit order price must be positive")
	}
	if intent.Quantity <= 0 {
		return nil, fmt.Errorf("limit order quantity must be positive")
	}

	orderID := strings.TrimSpace(intent.OrderID)
	if orderID == "" {
		orderID = strings.TrimSpace(intent.ClientID)
	}
	if orderID == "" {
		e.nextID++
		orderID = fmt.Sprintf("sim-%d", e.nextID)
	}
	if _, exists := e.orders[orderID]; exists {
		return nil, fmt.Errorf("order %s already exists", orderID)
	}

	order := &Order{
		ID:           orderID,
		ClientID:     intent.ClientID,
		Symbol:       intent.Symbol,
		Type:         OrderTypeLimit,
		Side:         intent.Side,
		PositionSide: intent.PositionSide,
		Price:        intent.Price,
		Quantity:     intent.Quantity,
		ReduceOnly:   intent.ReduceOnly,
		PostOnly:     intent.PostOnly,
		Status:       OrderStatusNew,
		CreatedAt:    intent.Time,
		UpdatedAt:    intent.Time,
	}
	e.orders[order.ID] = order
	e.markTime(intent.Time)
	return cloneOrder(order), nil
}

func (e *Engine) SubmitMarket(intent OrderIntent) ([]Fill, error) {
	if err := e.validateOrderIntent(intent); err != nil {
		return nil, err
	}
	if e.lastPrice <= 0 {
		return nil, fmt.Errorf("cannot execute market order without last market price")
	}

	quantity := intent.Quantity
	if quantity <= 0 {
		quantity = e.positionQuantityForClose(intent.Side, intent.PositionSide)
	}
	if intent.ReduceOnly {
		quantity = math.Min(quantity, e.positionQuantityForClose(intent.Side, intent.PositionSide))
	}
	if quantity <= 0 {
		return nil, nil
	}

	e.nextID++
	order := &Order{
		ID:           fmt.Sprintf("sim-market-%d", e.nextID),
		ClientID:     intent.ClientID,
		Symbol:       intent.Symbol,
		Type:         OrderTypeMarket,
		Side:         intent.Side,
		PositionSide: intent.PositionSide,
		Price:        e.lastPrice,
		Quantity:     quantity,
		ReduceOnly:   intent.ReduceOnly,
		Status:       OrderStatusNew,
		CreatedAt:    intent.Time,
		UpdatedAt:    intent.Time,
	}
	fill, ok := e.applyOrderFill(order, e.lastPrice, quantity, LiquidityTaker, intent.Time, "market_order")
	if !ok {
		return nil, nil
	}
	order.Status = OrderStatusFilled
	order.Filled = quantity
	order.UpdatedAt = intent.Time
	e.markTime(intent.Time)
	return []Fill{fill}, nil
}

func (e *Engine) CancelOrder(orderID string, at int64) error {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return fmt.Errorf("order id is required")
	}
	order, exists := e.orders[orderID]
	if !exists {
		return fmt.Errorf("order %s not found", orderID)
	}
	order.Status = OrderStatusCanceled
	order.UpdatedAt = at
	delete(e.orders, orderID)
	e.markTime(at)
	return nil
}

func (e *Engine) SeedPosition(symbol string, positionSide PositionSide, quantity, avgPrice float64, at int64) error {
	if e.cfg.Symbol != "" && !strings.EqualFold(symbol, e.cfg.Symbol) {
		return fmt.Errorf("seed symbol %s does not match engine symbol %s", symbol, e.cfg.Symbol)
	}
	if positionSide != PositionLong && positionSide != PositionShort {
		return fmt.Errorf("unsupported seed position side %s", positionSide)
	}
	if quantity < 0 {
		return fmt.Errorf("seed quantity cannot be negative")
	}
	if quantity > 0 && avgPrice <= 0 {
		return fmt.Errorf("seed average price must be positive")
	}
	e.positions[positionSide] = &Position{
		Side:     positionSide,
		Quantity: quantity,
		AvgPrice: avgPrice,
	}
	e.markTime(at)
	return nil
}

func (e *Engine) OnTrade(event AggTradeEvent) []Fill {
	if event.Symbol != "" && e.cfg.Symbol != "" && !strings.EqualFold(event.Symbol, e.cfg.Symbol) {
		return nil
	}
	e.lastPrice = event.Price
	e.markTime(event.Time)
	if event.Price <= 0 || event.Quantity <= 0 {
		return nil
	}

	remaining := event.Quantity
	candidates := e.matchCandidates(event)
	fills := make([]Fill, 0)
	for _, order := range candidates {
		if remaining <= 0 {
			break
		}
		fillQty := math.Min(order.Remaining(), remaining)
		if order.ReduceOnly {
			fillQty = math.Min(fillQty, e.positionQuantityForClose(order.Side, order.PositionSide))
		}
		if fillQty <= 0 {
			continue
		}
		fill, ok := e.applyOrderFill(order, order.Price, fillQty, LiquidityMaker, event.Time, "agg_trade")
		if !ok {
			continue
		}
		remaining -= fillQty
		fills = append(fills, fill)
		order.Filled += fillQty
		order.UpdatedAt = event.Time
		if order.Remaining() <= 1e-12 {
			order.Status = OrderStatusFilled
			delete(e.orders, order.ID)
		} else {
			order.Status = OrderStatusPartiallyFilled
		}
	}
	return fills
}

func (e *Engine) Report() Report {
	positions := e.Positions()
	unrealized := e.unrealizedPnL()
	report := Report{
		Symbol:         e.cfg.Symbol,
		StartedAt:      e.startedAt,
		EndedAt:        e.endedAt,
		InitialBalance: e.cfg.InitialBalance,
		Balance:        e.balance,
		Equity:         e.balance + unrealized,
		RealizedPnL:    e.realizedPnL,
		Fees:           e.fees,
		UnrealizedPnL:  unrealized,
		FillCount:      len(e.fills),
		OpenOrders:     e.OpenOrders(),
		Positions:      positions,
	}
	if e.cfg.RecordFills {
		report.Fills = append([]Fill(nil), e.fills...)
	}
	return report
}

func (e *Engine) OpenOrders() []Order {
	orders := make([]Order, 0, len(e.orders))
	for _, order := range e.orders {
		orders = append(orders, *order)
	}
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].CreatedAt == orders[j].CreatedAt {
			return orders[i].ID < orders[j].ID
		}
		return orders[i].CreatedAt < orders[j].CreatedAt
	})
	return orders
}

func (e *Engine) Positions() []Position {
	positions := []Position{*e.positions[PositionLong], *e.positions[PositionShort]}
	return positions
}

func (e *Engine) validateOrderIntent(intent OrderIntent) error {
	if intent.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if e.cfg.Symbol != "" && !strings.EqualFold(intent.Symbol, e.cfg.Symbol) {
		return fmt.Errorf("intent symbol %s does not match engine symbol %s", intent.Symbol, e.cfg.Symbol)
	}
	if intent.Side != SideBuy && intent.Side != SideSell {
		return fmt.Errorf("unsupported side %s", intent.Side)
	}
	if intent.PositionSide != PositionLong && intent.PositionSide != PositionShort {
		return fmt.Errorf("unsupported position side %s", intent.PositionSide)
	}
	return nil
}

func (e *Engine) matchCandidates(event AggTradeEvent) []*Order {
	candidates := make([]*Order, 0)
	for _, order := range e.orders {
		if order.Status == OrderStatusCanceled || order.Remaining() <= 0 {
			continue
		}
		if !strings.EqualFold(order.Symbol, event.Symbol) {
			continue
		}
		if !e.orderMatchesTrade(order, event) {
			continue
		}
		candidates = append(candidates, order)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a := candidates[i]
		b := candidates[j]
		if a.Side == SideBuy && b.Side == SideBuy && a.Price != b.Price {
			return a.Price > b.Price
		}
		if a.Side == SideSell && b.Side == SideSell && a.Price != b.Price {
			return a.Price < b.Price
		}
		if a.CreatedAt == b.CreatedAt {
			return a.ID < b.ID
		}
		return a.CreatedAt < b.CreatedAt
	})
	return candidates
}

func (e *Engine) orderMatchesTrade(order *Order, event AggTradeEvent) bool {
	switch event.TakerSide {
	case SideSell:
		if order.Side != SideBuy {
			return false
		}
		if e.cfg.QueueModel == QueueModelTouch {
			return order.Price >= event.Price
		}
		return order.Price > event.Price
	case SideBuy:
		if order.Side != SideSell {
			return false
		}
		if e.cfg.QueueModel == QueueModelTouch {
			return order.Price <= event.Price
		}
		return order.Price < event.Price
	default:
		return false
	}
}

func (e *Engine) applyOrderFill(order *Order, price, quantity float64, liquidity Liquidity, at int64, source string) (Fill, bool) {
	quantity = e.executableQuantity(order.Side, order.PositionSide, order.ReduceOnly, quantity)
	if quantity <= 0 {
		return Fill{}, false
	}

	pnl := e.applyPositionFill(order.Side, order.PositionSide, price, quantity)
	feeRate := e.cfg.MakerFeeRate
	if liquidity == LiquidityTaker {
		feeRate = e.cfg.TakerFeeRate
	}
	fee := price * quantity * feeRate
	e.realizedPnL += pnl
	e.fees += fee
	e.balance += pnl - fee
	e.positions[order.PositionSide].Fees += fee

	fill := Fill{
		OrderID:      order.ID,
		ClientID:     order.ClientID,
		Symbol:       order.Symbol,
		Side:         order.Side,
		PositionSide: order.PositionSide,
		Price:        price,
		Quantity:     quantity,
		Fee:          fee,
		RealizedPnL:  pnl,
		Liquidity:    liquidity,
		Time:         at,
		Source:       source,
	}
	e.fills = append(e.fills, fill)
	return fill, true
}

func (e *Engine) executableQuantity(side Side, positionSide PositionSide, reduceOnly bool, requested float64) float64 {
	if requested <= 0 {
		return 0
	}
	if !reduceOnly && !isClosingSide(side, positionSide) {
		return requested
	}
	closeQty := e.positionQuantityForClose(side, positionSide)
	if closeQty <= 0 {
		return 0
	}
	return math.Min(requested, closeQty)
}

func (e *Engine) positionQuantityForClose(side Side, positionSide PositionSide) float64 {
	if !isClosingSide(side, positionSide) {
		return 0
	}
	position := e.positions[positionSide]
	if position == nil {
		return 0
	}
	return position.Quantity
}

func isClosingSide(side Side, positionSide PositionSide) bool {
	return (positionSide == PositionLong && side == SideSell) || (positionSide == PositionShort && side == SideBuy)
}

func (e *Engine) applyPositionFill(side Side, positionSide PositionSide, price, quantity float64) float64 {
	position := e.positions[positionSide]
	if position == nil {
		position = &Position{Side: positionSide}
		e.positions[positionSide] = position
	}

	switch positionSide {
	case PositionLong:
		if side == SideBuy {
			addPosition(position, price, quantity)
			return 0
		}
		return reducePosition(position, price, quantity, func(avg, exit float64) float64 {
			return exit - avg
		})
	case PositionShort:
		if side == SideSell {
			addPosition(position, price, quantity)
			return 0
		}
		return reducePosition(position, price, quantity, func(avg, exit float64) float64 {
			return avg - exit
		})
	default:
		return 0
	}
}

func addPosition(position *Position, price, quantity float64) {
	if quantity <= 0 {
		return
	}
	if position.Quantity <= 0 {
		position.Quantity = quantity
		position.AvgPrice = price
		return
	}
	position.AvgPrice = (position.AvgPrice*position.Quantity + price*quantity) / (position.Quantity + quantity)
	position.Quantity += quantity
}

func reducePosition(position *Position, price, quantity float64, pnlPerUnit func(avg, exit float64) float64) float64 {
	if position.Quantity <= 0 || quantity <= 0 {
		return 0
	}
	quantity = math.Min(quantity, position.Quantity)
	pnl := pnlPerUnit(position.AvgPrice, price) * quantity
	position.Quantity -= quantity
	position.RealizedPnL += pnl
	if position.Quantity <= 1e-12 {
		position.Quantity = 0
		position.AvgPrice = 0
	}
	return pnl
}

func (e *Engine) unrealizedPnL() float64 {
	if e.lastPrice <= 0 {
		return 0
	}
	long := e.positions[PositionLong]
	short := e.positions[PositionShort]
	pnl := 0.0
	if long != nil && long.Quantity > 0 {
		pnl += (e.lastPrice - long.AvgPrice) * long.Quantity
	}
	if short != nil && short.Quantity > 0 {
		pnl += (short.AvgPrice - e.lastPrice) * short.Quantity
	}
	return pnl
}

func (e *Engine) markTime(t int64) {
	if t <= 0 {
		return
	}
	if e.startedAt == 0 || t < e.startedAt {
		e.startedAt = t
	}
	if t > e.endedAt {
		e.endedAt = t
	}
}

func cloneOrder(order *Order) *Order {
	if order == nil {
		return nil
	}
	cloned := *order
	return &cloned
}
