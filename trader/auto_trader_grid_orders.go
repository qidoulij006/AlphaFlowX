package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Grid Order Placement and Management
// ============================================================================

func normalizeGridPositionSide(positionSide string, fallbackSide string) string {
	normalized := strings.ToUpper(strings.TrimSpace(positionSide))
	if normalized == "LONG" || normalized == "SHORT" {
		return normalized
	}
	if strings.EqualFold(fallbackSide, "buy") {
		return "LONG"
	}
	return "SHORT"
}

func clearGridPendingOrder(level *kernel.GridLevelInfo) {
	level.OrderID = ""
	level.OrderQuantity = 0
	level.OrderPositionSide = ""
	level.OrderReduceOnly = false
	level.LinkedLevelIndex = 0
	if level.PositionSize > 0 {
		level.State = "filled"
	} else {
		level.State = "empty"
	}
}

func (at *AutoTrader) clearTrackedGridOrder(orderID string) {
	if at == nil || at.gridState == nil || strings.TrimSpace(orderID) == "" {
		return
	}
	at.gridState.mu.Lock()
	if levelIdx, ok := at.gridState.OrderBook[orderID]; ok {
		if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) && at.gridState.Levels[levelIdx].OrderID == orderID {
			clearGridPendingOrder(&at.gridState.Levels[levelIdx])
		}
		delete(at.gridState.OrderBook, orderID)
	}
	at.gridState.mu.Unlock()
}

func clearGridFilledPosition(level *kernel.GridLevelInfo) {
	level.State = "empty"
	level.OrderID = ""
	level.OrderQuantity = 0
	level.OrderPositionSide = ""
	level.OrderReduceOnly = false
	level.LinkedLevelIndex = 0
	level.PositionSize = 0
	level.PositionEntry = 0
	level.PositionSide = ""
	level.UnrealizedPnL = 0
}

const gridStandaloneReduceOnlyMinNetBufferRatio = 0.0010
const (
	gridEntryUniformCapMultiplierDefault      = 1.3
	gridEntryUniformCapMultiplierConservative = 1.1
	gridEntryBoundaryEdgeBandRatio            = 0.15
	gridEntrySizingDriftReplaceThreshold      = 0.35
	gridFirstEntryGuardThresholdPctDefault    = 0.22
	gridFirstEntryGuardATRMultiplierDefault   = 10.0
	gridEntrySizingPolicyVersion              = "entry-cap-1.3-edge-1.1-v1"
)

var (
	gridOrderLevelRefPattern = regexp.MustCompile(`(?i)L(\d+)`)
	gridOrderPriceRefPattern = regexp.MustCompile(`([0-9]+\.[0-9]+)`)
)

func gridLevelHasFilledPosition(level kernel.GridLevelInfo) bool {
	return level.PositionSize > 0
}

func gridLevelHasTrackedEntryOrder(level kernel.GridLevelInfo) bool {
	return level.OrderID != "" && !level.OrderReduceOnly
}

func gridLevelCanHostEntryOrder(level kernel.GridLevelInfo) bool {
	return !gridLevelHasTrackedEntryOrder(level)
}

// currentGridInvestmentBase returns the live account equity used as the grid
// capital base for compounding. When balance data is temporarily unavailable,
// it falls back to the configured seed investment so grid trading can continue.
func (at *AutoTrader) currentGridInvestmentBase() float64 {
	if at == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return 0
	}

	fallback := at.config.StrategyConfig.GridConfig.TotalInvestment
	balance, err := at.trader.GetBalance()
	if err != nil || balance == nil {
		return fallback
	}

	if equity, ok := balance["total_equity"].(float64); ok && equity > 0 {
		return equity
	}
	if equity, ok := balance["totalEquity"].(float64); ok && equity > 0 {
		return equity
	}
	if total, ok := balance["totalWalletBalance"].(float64); ok && total > 0 {
		if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
			return total + unrealized
		}
		return total
	}
	if wallet, ok := balance["wallet_balance"].(float64); ok && wallet > 0 {
		return wallet
	}
	if totalEq, ok := balance["totalEq"].(float64); ok && totalEq > 0 {
		return totalEq
	}
	if rawBalance, ok := balance["balance"].(float64); ok && rawBalance > 0 {
		return rawBalance
	}

	return fallback
}

func (at *AutoTrader) currentGridLevelAllocatedUSD(levelIdx int, investmentBase float64) float64 {
	if investmentBase <= 0 || at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return 0
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.GridCount <= 0 {
		return investmentBase
	}

	allocatedUSD := 0.0
	at.gridState.mu.RLock()
	if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) {
		allocatedUSD = at.gridState.Levels[levelIdx].AllocatedUSD
	}
	at.gridState.mu.RUnlock()

	if allocatedUSD > 0 && gridConfig.TotalInvestment > 0 {
		ratio := allocatedUSD / gridConfig.TotalInvestment
		if ratio > 0 {
			return investmentBase * ratio
		}
	}

	return investmentBase / float64(gridConfig.GridCount)
}

func (at *AutoTrader) currentGridEntryUniformCapMultiplier(currentPrice float64) float64 {
	if at == nil || at.gridState == nil {
		return gridEntryUniformCapMultiplierDefault
	}

	at.gridState.mu.RLock()
	lower := at.gridState.LowerPrice
	upper := at.gridState.UpperPrice
	midLower := at.gridState.MidBoxLower
	midUpper := at.gridState.MidBoxUpper
	direction := at.gridState.CurrentDirection
	at.gridState.mu.RUnlock()

	if direction != market.GridDirectionNeutral {
		return gridEntryUniformCapMultiplierConservative
	}

	if currentPrice <= 0 {
		return gridEntryUniformCapMultiplierDefault
	}

	gridRange := upper - lower
	if lower > 0 && upper > lower && gridRange > 0 {
		edgeBand := gridRange * gridEntryBoundaryEdgeBandRatio
		if currentPrice <= lower+edgeBand || currentPrice >= upper-edgeBand {
			return gridEntryUniformCapMultiplierConservative
		}
	}

	midRange := midUpper - midLower
	if midLower > 0 && midUpper > midLower && midRange > 0 {
		edgeBand := midRange * gridEntryBoundaryEdgeBandRatio
		if currentPrice <= midLower+edgeBand || currentPrice >= midUpper-edgeBand {
			return gridEntryUniformCapMultiplierConservative
		}
	}

	return gridEntryUniformCapMultiplierDefault
}

func (at *AutoTrader) capGridEntryQuantityByUniformLimit(levelIdx int, orderPrice float64, quantity float64, investmentBase float64, currentPrice float64) (float64, float64) {
	if quantity <= 0 || orderPrice <= 0 || investmentBase <= 0 || at == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return quantity, 0
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.GridCount <= 0 {
		return quantity, 0
	}

	multiplier := at.currentGridEntryUniformCapMultiplier(currentPrice)
	uniformMarginPerLevel := investmentBase / float64(gridConfig.GridCount)
	uniformNotionalCap := uniformMarginPerLevel * float64(gridConfig.Leverage) * multiplier
	if uniformNotionalCap <= 0 {
		return quantity, multiplier
	}

	levelAllocatedUSD := at.currentGridLevelAllocatedUSD(levelIdx, investmentBase)
	if levelAllocatedUSD > 0 {
		levelNotional := levelAllocatedUSD * float64(gridConfig.Leverage)
		if levelNotional < uniformNotionalCap {
			uniformNotionalCap = levelNotional
		}
	}

	maxQuantity := uniformNotionalCap / orderPrice
	if maxQuantity > 0 && quantity > maxQuantity {
		return maxQuantity, multiplier
	}

	return quantity, multiplier
}

func (at *AutoTrader) latestGridATR14(currentPrice float64) float64 {
	if at == nil || at.gridState == nil {
		return 0
	}

	at.gridState.mu.RLock()
	atr := at.gridState.LastContextATR14
	snapshotPrice := at.gridState.LastContextPrice
	builtAt := at.gridState.LastContextBuiltAt
	at.gridState.mu.RUnlock()

	if atr > 0 && !builtAt.IsZero() && time.Since(builtAt) <= 10*time.Minute {
		return atr
	}

	if ctx, err := at.buildGridContext(); err == nil && ctx != nil && ctx.ATR14 > 0 {
		return ctx.ATR14
	}

	if atr > 0 {
		return atr
	}
	if currentPrice > 0 {
		return currentPrice * minGridSpacingPct
	}
	if snapshotPrice > 0 {
		return snapshotPrice * minGridSpacingPct
	}
	return 0
}

func (at *AutoTrader) currentFirstEntryGuardConfig(currentPrice float64) (thresholdPct float64, atrMultiplier float64, fixedBand float64, atrBand float64, guardBand float64, activeSource string) {
	thresholdPct = gridFirstEntryGuardThresholdPctDefault
	atrMultiplier = gridFirstEntryGuardATRMultiplierDefault
	if at == nil || at.gridState == nil {
		return thresholdPct, atrMultiplier, 0, 0, 0, ""
	}

	at.gridState.mu.RLock()
	lower := at.gridState.LowerPrice
	upper := at.gridState.UpperPrice
	at.gridState.mu.RUnlock()

	gridRange := upper - lower
	if gridRange <= 0 {
		return thresholdPct, atrMultiplier, 0, 0, 0, ""
	}

	fixedBand = gridRange * thresholdPct
	atrBand = atrMultiplier * at.latestGridATR14(currentPrice)
	guardBand = math.Max(fixedBand, atrBand)
	if guardBand <= 0 {
		return thresholdPct, atrMultiplier, fixedBand, atrBand, 0, ""
	}
	if atrBand > fixedBand {
		activeSource = "atr_10x"
	} else {
		activeSource = "fixed_22pct"
	}
	return thresholdPct, atrMultiplier, fixedBand, atrBand, guardBand, activeSource
}

func (at *AutoTrader) currentFirstEntryGuardPrices(currentPrice float64) (shortGuardPrice float64, longGuardPrice float64, thresholdPct float64, atrMultiplier float64, fixedBand float64, atrBand float64, activeSource string) {
	if at == nil || at.gridState == nil {
		return 0, 0, gridFirstEntryGuardThresholdPctDefault, gridFirstEntryGuardATRMultiplierDefault, 0, 0, ""
	}
	at.gridState.mu.RLock()
	lower := at.gridState.LowerPrice
	upper := at.gridState.UpperPrice
	at.gridState.mu.RUnlock()

	thresholdPct, atrMultiplier, fixedBand, atrBand, guardBand, activeSource := at.currentFirstEntryGuardConfig(currentPrice)
	if upper <= lower || guardBand <= 0 {
		return 0, 0, thresholdPct, atrMultiplier, fixedBand, atrBand, activeSource
	}
	shortGuardPrice = upper - guardBand
	longGuardPrice = lower + guardBand
	return shortGuardPrice, longGuardPrice, thresholdPct, atrMultiplier, fixedBand, atrBand, activeSource
}

func (at *AutoTrader) reconcileGridEntryOrderSizing(symbol string, openOrders []OpenOrder) {
	if at == nil || at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return
	}

	investmentBase := at.currentGridInvestmentBase()
	if investmentBase <= 0 {
		return
	}

	type driftedEntry struct {
		order       OpenOrder
		levelIdx    int
		expectedQty float64
	}

	at.gridState.mu.RLock()
	forceReplaceAll := at.gridState.EntrySizingPolicyVersion != gridEntrySizingPolicyVersion
	at.gridState.mu.RUnlock()

	candidates := make([]driftedEntry, 0)
	at.gridState.mu.RLock()
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		isReduceOnly := (strings.EqualFold(order.Side, "SELL") && positionSide == "LONG") ||
			(strings.EqualFold(order.Side, "BUY") && positionSide == "SHORT")
		if isReduceOnly || order.Price <= 0 || order.Quantity <= 0 {
			continue
		}

		levelIdx, ok := at.gridState.OrderBook[order.OrderID]
		if !ok || levelIdx < 0 || levelIdx >= len(at.gridState.Levels) {
			continue
		}

		expectedQty := at.suggestedEntryQuantityWithInvestmentBase(levelIdx, order.Price, investmentBase)
		if expectedQty <= 0 {
			continue
		}
		drift := math.Abs(order.Quantity-expectedQty) / expectedQty
		if !forceReplaceAll && drift < gridEntrySizingDriftReplaceThreshold {
			continue
		}

		candidates = append(candidates, driftedEntry{
			order:       order,
			levelIdx:    levelIdx,
			expectedQty: expectedQty,
		})
	}
	at.gridState.mu.RUnlock()

	for _, candidate := range candidates {
		positionSide := normalizeGridPositionSide(candidate.order.PositionSide, candidate.order.Side)
		if allowed, reason := at.evaluateGridEntryAdmission(symbol, positionSide, candidate.order.Price); !allowed {
			if forceReplaceAll {
				gridTrader := &GridTraderAdapter{Trader: at.trader}
				if err := gridTrader.CancelOrder(symbol, candidate.order.OrderID); err != nil {
					logger.Warnf("%s Failed to cancel entry order %s during policy-version reseed: %v",
						at.gridLogPrefix(), candidate.order.OrderID, err)
					continue
				}
				at.gridState.mu.Lock()
				if candidate.levelIdx >= 0 && candidate.levelIdx < len(at.gridState.Levels) {
					level := &at.gridState.Levels[candidate.levelIdx]
					if level.OrderID == candidate.order.OrderID {
						clearGridPendingOrder(level)
					}
				}
				delete(at.gridState.OrderBook, candidate.order.OrderID)
				at.gridState.mu.Unlock()
				logger.Infof("%s Cancelled entry order %s during policy-version reseed because new admission now blocks replacement: %s",
					at.gridLogPrefix(), candidate.order.OrderID, reason)
				continue
			}
			logger.Infof("%s Keeping drifted entry order %s at $%s because admission now blocks replacement: %s",
				at.gridLogPrefix(), candidate.order.OrderID, formatGridLogPrice(candidate.order.Price), reason)
			continue
		}

		gridTrader := &GridTraderAdapter{Trader: at.trader}
		if err := gridTrader.CancelOrder(symbol, candidate.order.OrderID); err != nil {
			logger.Warnf("%s Failed to cancel drifted entry order %s for resizing: %v",
				at.gridLogPrefix(), candidate.order.OrderID, err)
			continue
		}

		at.gridState.mu.Lock()
		if candidate.levelIdx >= 0 && candidate.levelIdx < len(at.gridState.Levels) {
			level := &at.gridState.Levels[candidate.levelIdx]
			if level.OrderID == candidate.order.OrderID {
				clearGridPendingOrder(level)
			}
		}
		delete(at.gridState.OrderBook, candidate.order.OrderID)
		at.gridState.mu.Unlock()

		action := "place_buy_limit"
		if strings.EqualFold(candidate.order.Side, "SELL") {
			action = "place_sell_limit"
		}

		logger.Infof("%s Replacing drifted entry order %s qty %.4f with system qty %.4f at level %d ($%s, compounding_base $%.2f)",
			at.gridLogPrefix(), candidate.order.OrderID, candidate.order.Quantity, candidate.expectedQty, candidate.levelIdx, formatGridLogPrice(candidate.order.Price), investmentBase)

		if err := at.executeGridDecision(&kernel.Decision{
			Symbol:     symbol,
			Action:     action,
			Price:      candidate.order.Price,
			Quantity:   candidate.expectedQty,
			LevelIndex: candidate.levelIdx,
			Confidence: 99,
			Reasoning:  "System entry-size reconciliation replaced a drifted grid entry order with the current compounding-base quantity.",
			ForceEntry: true,
		}); err != nil {
			logger.Warnf("%s Failed to replace drifted entry order %s at level %d: %v",
				at.gridLogPrefix(), candidate.order.OrderID, candidate.levelIdx, err)
		}
	}

	at.gridState.mu.Lock()
	at.gridState.EntrySizingPolicyVersion = gridEntrySizingPolicyVersion
	at.gridState.mu.Unlock()
}

func (at *AutoTrader) evaluateGridEntryAdmission(symbol string, positionSide string, orderPrice float64) (bool, string) {
	if at == nil || at.gridState == nil || orderPrice <= 0 {
		return true, ""
	}

	currentPrice, err := at.trader.GetMarketPrice(symbol)
	if err != nil {
		currentPrice = orderPrice
	}

	currentPositionQty := at.getCurrentPositionQuantity(symbol, positionSide)
	at.gridState.mu.RLock()
	currentRiskState := at.gridState.CurrentRiskState
	currentRiskSide := at.gridState.RiskStateSide
	at.gridState.mu.RUnlock()

	if currentPositionQty > 0 {
		if strings.EqualFold(currentRiskSide, positionSide) {
			switch currentRiskState {
			case gridRiskStateWarning:
				return false, fmt.Sprintf("%s warning state blocks same-side add-entry in Wave 1-3", strings.ToUpper(positionSide))
			case gridRiskStateSoftReduce, gridRiskStateHardReduce, gridRiskStateEmergency:
				return false, fmt.Sprintf("%s risk state %s only allows reduce-only management", strings.ToUpper(positionSide), currentRiskState)
			}
		}
		return true, ""
	}

	shortGuardPrice, longGuardPrice, _, _, _, _, activeSource := at.currentFirstEntryGuardPrices(currentPrice)
	switch strings.ToUpper(strings.TrimSpace(positionSide)) {
	case "SHORT":
		if shortGuardPrice > 0 {
			if currentPrice >= shortGuardPrice {
				return false, fmt.Sprintf("SHORT first-entry guard blocked because current price $%s is inside guard zone >= $%s (source=%s)",
					formatGridLogPrice(currentPrice), formatGridLogPrice(shortGuardPrice), activeSource)
			}
			if orderPrice >= shortGuardPrice {
				return false, fmt.Sprintf("SHORT first-entry guard blocked because order price $%s is above guard line >= $%s (source=%s)",
					formatGridLogPrice(orderPrice), formatGridLogPrice(shortGuardPrice), activeSource)
			}
		}
	case "LONG":
		if longGuardPrice > 0 {
			if currentPrice <= longGuardPrice {
				return false, fmt.Sprintf("LONG first-entry guard blocked because current price $%s is inside guard zone <= $%s (source=%s)",
					formatGridLogPrice(currentPrice), formatGridLogPrice(longGuardPrice), activeSource)
			}
			if orderPrice <= longGuardPrice {
				return false, fmt.Sprintf("LONG first-entry guard blocked because order price $%s is below guard line <= $%s (source=%s)",
					formatGridLogPrice(orderPrice), formatGridLogPrice(longGuardPrice), activeSource)
			}
		}
	}

	return true, ""
}

func (at *AutoTrader) enforceFirstEntryGuardOnOpenOrders(symbol string, openOrders []OpenOrder) []OpenOrder {
	if at == nil || at.gridState == nil || len(openOrders) == 0 {
		return openOrders
	}

	currentPrice, err := at.trader.GetMarketPrice(symbol)
	if err != nil || currentPrice <= 0 {
		if err != nil {
			logger.Warnf("%s Failed to enforce first-entry guard on live orders: market price unavailable: %v", at.gridLogPrefix(), err)
		}
		return openOrders
	}

	shortGuardPrice, longGuardPrice, _, _, _, _, activeSource := at.currentFirstEntryGuardPrices(currentPrice)
	if shortGuardPrice <= 0 && longGuardPrice <= 0 {
		return openOrders
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("%s Failed to enforce first-entry guard on live orders: positions unavailable: %v", at.gridLogPrefix(), err)
		return openOrders
	}

	positionQtyBySide := map[string]float64{
		"LONG":  0,
		"SHORT": 0,
	}
	for _, pos := range positions {
		posSymbol, _ := pos["symbol"].(string)
		if posSymbol != symbol {
			continue
		}
		size, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		side, _ := pos["side"].(string)
		positionQtyBySide[normalizeGridPositionSide(side, "")] += math.Abs(size)
	}

	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	cancelled := make(map[string]bool)
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if inferGridReduceOnly(order.Side, positionSide) {
			continue
		}

		orderPrice := gridOrderPrice(order)
		reason := ""
		switch {
		case strings.EqualFold(order.Side, "SELL") && positionSide == "SHORT" && positionQtyBySide["SHORT"] <= gridLotQtyTolerance && shortGuardPrice > 0:
			switch {
			case currentPrice >= shortGuardPrice:
				reason = fmt.Sprintf("current price $%s is inside SHORT first-entry guard zone >= $%s (source=%s)",
					formatGridLogPrice(currentPrice), formatGridLogPrice(shortGuardPrice), activeSource)
			case orderPrice >= shortGuardPrice:
				reason = fmt.Sprintf("order price $%s is above SHORT first-entry guard line >= $%s (source=%s)",
					formatGridLogPrice(orderPrice), formatGridLogPrice(shortGuardPrice), activeSource)
			}
		case strings.EqualFold(order.Side, "BUY") && positionSide == "LONG" && positionQtyBySide["LONG"] <= gridLotQtyTolerance && longGuardPrice > 0:
			switch {
			case currentPrice <= longGuardPrice:
				reason = fmt.Sprintf("current price $%s is inside LONG first-entry guard zone <= $%s (source=%s)",
					formatGridLogPrice(currentPrice), formatGridLogPrice(longGuardPrice), activeSource)
			case orderPrice <= longGuardPrice:
				reason = fmt.Sprintf("order price $%s is below LONG first-entry guard line <= $%s (source=%s)",
					formatGridLogPrice(orderPrice), formatGridLogPrice(longGuardPrice), activeSource)
			}
		}
		if reason == "" {
			continue
		}

		if err := gridTrader.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Warnf("%s Failed to cancel first-entry guarded order %s at $%s: %v",
				at.gridLogPrefix(), order.OrderID, formatGridLogPrice(orderPrice), err)
			continue
		}
		cancelled[order.OrderID] = true
		at.clearTrackedGridOrder(order.OrderID)
		logger.Infof("%s Cancelled first-entry guarded %s %s order %s at $%s: %s",
			at.gridLogPrefix(), strings.ToUpper(order.Side), positionSide, order.OrderID, formatGridLogPrice(orderPrice), reason)
	}

	if len(cancelled) == 0 {
		return openOrders
	}

	filtered := make([]OpenOrder, 0, len(openOrders)-len(cancelled))
	for _, order := range openOrders {
		if !cancelled[order.OrderID] {
			filtered = append(filtered, order)
		}
	}
	return filtered
}

func (at *AutoTrader) shouldSeedLivePositionFallbackExit(symbol string, positionSide string) bool {
	if at.store == nil {
		return true
	}

	openPositions, err := at.store.Position().GetOpenPositions(at.id)
	if err != nil {
		logger.Warnf("[Grid] Failed to inspect local open positions for fallback exit gating: %v", err)
		return true
	}

	normalizedSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	normalizedSide := strings.ToUpper(strings.TrimSpace(positionSide))
	for _, pos := range openPositions {
		if strings.ToUpper(strings.TrimSpace(pos.Symbol)) != normalizedSymbol {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(pos.Side)) != normalizedSide {
			continue
		}
		if strings.EqualFold(pos.Source, "snapshot") {
			continue
		}
		return false
	}

	return true
}

func (at *AutoTrader) fallbackExitMeetsMinNetProfit(positionSide string, entryPrice float64, exitPrice float64, quantity float64) bool {
	qty := math.Abs(quantity)
	if qty <= 0 || entryPrice <= 0 || exitPrice <= 0 {
		return false
	}

	grossPnL := (exitPrice - entryPrice) * qty
	if strings.EqualFold(positionSide, "SHORT") {
		grossPnL = (entryPrice - exitPrice) * qty
	}
	if grossPnL <= 0 {
		return false
	}

	referenceNotional := math.Max(entryPrice, exitPrice) * qty
	minRequiredPnL := referenceNotional * gridStandaloneReduceOnlyMinNetBufferRatio
	return grossPnL > minRequiredPnL
}

func (at *AutoTrader) syncLiveOpenOrdersSnapshot(symbol string, openOrders []OpenOrder) {
	if at.store == nil {
		return
	}

	liveOrders := make([]*store.TraderOrder, 0, len(openOrders))
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		reduceOnly := (strings.EqualFold(order.Side, "SELL") && positionSide == "LONG") ||
			(strings.EqualFold(order.Side, "BUY") && positionSide == "SHORT")

		orderAction := "open_short"
		switch {
		case reduceOnly && positionSide == "LONG":
			orderAction = "close_long"
		case reduceOnly && positionSide == "SHORT":
			orderAction = "close_short"
		case !reduceOnly && positionSide == "LONG":
			orderAction = "open_long"
		}

		liveOrders = append(liveOrders, &store.TraderOrder{
			TraderID:        at.id,
			ExchangeID:      at.exchangeID,
			ExchangeType:    at.exchange,
			ExchangeOrderID: order.OrderID,
			Symbol:          symbol,
			Side:            strings.ToUpper(order.Side),
			PositionSide:    positionSide,
			Type:            strings.ToUpper(order.Type),
			TimeInForce:     "GTC",
			Quantity:        order.Quantity,
			Price:           order.Price,
			StopPrice:       order.StopPrice,
			Status:          strings.ToUpper(order.Status),
			ReduceOnly:      reduceOnly,
			OrderAction:     orderAction,
		})
	}

	if err := at.store.Order().SyncLiveOpenOrdersSnapshot(at.id, at.exchangeID, at.exchange, symbol, liveOrders); err != nil {
		logger.Warnf("[Grid] Failed to sync live open orders snapshot for %s: %v", symbol, err)
	}
}

func (at *AutoTrader) cancelGridOrdersMatching(reason string, predicate func(level kernel.GridLevelInfo) bool) error {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	type pendingOrder struct {
		levelIdx int
		orderID  string
	}

	ordersToCancel := make([]pendingOrder, 0)
	at.gridState.mu.RLock()
	for idx, level := range at.gridState.Levels {
		if !gridLevelHasTrackedEntryOrder(level) {
			continue
		}
		if predicate(level) {
			ordersToCancel = append(ordersToCancel, pendingOrder{
				levelIdx: idx,
				orderID:  level.OrderID,
			})
		}
	}
	at.gridState.mu.RUnlock()

	if len(ordersToCancel) == 0 {
		logger.Infof("[Grid] No matching orders to cancel for %s", reason)
		return nil
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	var firstErr error
	for _, order := range ordersToCancel {
		if err := gridTrader.CancelOrder(gridConfig.Symbol, order.orderID); err != nil {
			logger.Warnf("[Grid] Failed to cancel order %s during %s: %v", order.orderID, reason, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		at.gridState.mu.Lock()
		if order.levelIdx >= 0 && order.levelIdx < len(at.gridState.Levels) {
			clearGridPendingOrder(&at.gridState.Levels[order.levelIdx])
		}
		delete(at.gridState.OrderBook, order.orderID)
		at.gridState.mu.Unlock()
	}

	return firstErr
}

func (at *AutoTrader) cancelGridEntryOrders(reason string) error {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to query live open orders for %s: %v", reason, err)
		return at.cancelGridOrdersMatching(reason, func(level kernel.GridLevelInfo) bool {
			return !level.OrderReduceOnly
		})
	}

	cancelledAny := false
	var firstErr error
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if inferGridReduceOnly(order.Side, positionSide) {
			continue
		}
		if err := gridTrader.CancelOrder(gridConfig.Symbol, order.OrderID); err != nil {
			logger.Warnf("[Grid] Failed to cancel live entry order %s during %s: %v", order.OrderID, reason, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		cancelledAny = true
		logger.Infof("[Grid] Cancelled live entry order %s during %s", order.OrderID, reason)

		at.gridState.mu.Lock()
		if levelIdx, ok := at.gridState.OrderBook[order.OrderID]; ok {
			if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) && at.gridState.Levels[levelIdx].OrderID == order.OrderID {
				clearGridPendingOrder(&at.gridState.Levels[levelIdx])
			}
			delete(at.gridState.OrderBook, order.OrderID)
		}
		at.gridState.mu.Unlock()
	}

	at.gridState.mu.Lock()
	for i := range at.gridState.Levels {
		if gridLevelHasTrackedEntryOrder(at.gridState.Levels[i]) {
			clearGridPendingOrder(&at.gridState.Levels[i])
		}
	}
	at.gridState.mu.Unlock()

	if !cancelledAny {
		logger.Infof("[Grid] No live entry orders to cancel for %s", reason)
	}
	return firstErr
}

func (at *AutoTrader) collectFilledGridLevelIndexes() []int {
	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()

	levelIdxs := make([]int, 0, len(at.gridState.Levels))
	for _, level := range at.gridState.Levels {
		if level.State == "filled" && gridLevelHasFilledPosition(level) {
			levelIdxs = append(levelIdxs, level.Index)
		}
	}
	return levelIdxs
}

func (at *AutoTrader) ensurePairedExitOrdersForAllFilledLevels() {
	at.cleanupNonAdjacentPairedExitOrders()
	at.ensureAggregatedRiskExitOrders()
	if at.ensurePairedExitOrdersForInventoryLots() {
		return
	}
	for _, levelIdx := range at.collectFilledGridLevelIndexes() {
		at.ensurePairedExitOrderForFilledLevel(levelIdx)
	}
	at.ensurePairedExitOrdersForLivePositions()
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.GridConfig != nil {
		if openOrders, err := at.trader.GetOpenOrders(at.config.StrategyConfig.GridConfig.Symbol); err == nil {
			at.syncLiveOpenOrdersSnapshot(at.config.StrategyConfig.GridConfig.Symbol, openOrders)
			at.rebuildStandaloneReduceOnlyBook(at.config.StrategyConfig.GridConfig.Symbol, openOrders)
			at.refreshInventoryLotExitBindings(at.config.StrategyConfig.GridConfig.Symbol, openOrders)
		}
	}
}

func (at *AutoTrader) ensurePairedExitOrdersForLivePositions() {
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("[Grid] Failed to load live positions for exit seeding: %v", err)
		return
	}

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		if symbol != gridConfig.Symbol {
			continue
		}
		size, _ := pos["positionAmt"].(float64)
		if math.Abs(size) <= 0 {
			continue
		}
		entryPrice, _ := pos["entryPrice"].(float64)
		if entryPrice <= 0 {
			continue
		}
		positionSide := normalizeGridPositionSide(fmt.Sprint(pos["side"]), "")
		if positionSide != "LONG" && positionSide != "SHORT" {
			continue
		}
		if active, _ := at.aggregatedRiskExitState(positionSide); active {
			continue
		}
		if !at.shouldSeedLivePositionFallbackExit(gridConfig.Symbol, positionSide) {
			continue
		}

		exitSide := "SELL"
		step := 1
		if positionSide == "SHORT" {
			exitSide = "BUY"
			step = -1
		}

		exitPrice := 0.0
		at.gridState.mu.RLock()
		if len(at.gridState.Levels) > 0 {
			entryLevelIdx := at.findClosestGridLevelIndexLocked(entryPrice, nil)
			if entryLevelIdx >= 0 {
				exitLevelIdx := entryLevelIdx + step
				if exitLevelIdx >= 0 && exitLevelIdx < len(at.gridState.Levels) {
					exitPrice = at.gridState.Levels[exitLevelIdx].Price
				}
			}
			if exitPrice <= 0 {
				if positionSide == "LONG" {
					exitPrice = at.gridState.UpperPrice + at.gridState.GridSpacing
				} else {
					exitPrice = at.gridState.LowerPrice - at.gridState.GridSpacing
				}
			}
		}
		at.gridState.mu.RUnlock()
		if exitPrice <= 0 {
			continue
		}

		qty := math.Abs(size)
		if !at.fallbackExitMeetsMinNetProfit(positionSide, entryPrice, exitPrice, qty) {
			logger.Warnf("[Grid] Skipped fallback live-position exit for %s %s qty=%.4f entry=$%s exit=$%s because estimated gross edge is below min net-profit buffer",
				gridConfig.Symbol, positionSide, qty, formatGridLogPrice(entryPrice), formatGridLogPrice(exitPrice))
			continue
		}

		if _, err := at.ensureStandaloneReduceOnlyOrder(exitSide, positionSide, qty, exitPrice); err != nil {
			logger.Warnf("[Grid] Failed to seed live-position exit for %s %s qty=%.4f at $%s: %v",
				gridConfig.Symbol, positionSide, qty, formatGridLogPrice(exitPrice), err)
			continue
		}
	}
}

func (at *AutoTrader) ensurePairedExitOrdersForInventoryLots() bool {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return false
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, gridConfig.Symbol)
	if err != nil || len(lots) == 0 {
		return false
	}

	type aggregatedExitPlan struct {
		side         string
		positionSide string
		exitLevelIdx int
		price        float64
		totalQty     float64
		lots         []store.GridInventoryLotModel
	}

	plansBySignature := make(map[string]*aggregatedExitPlan)
	signatures := make([]string, 0)
	for _, lot := range lots {
		plan, ok := at.buildInventoryLotExitPlan(lot)
		if !ok {
			continue
		}
		signature := at.standaloneReduceOnlySignature(gridConfig.Symbol, plan.side, plan.positionSide, plan.price)
		aggregated := plansBySignature[signature]
		if aggregated == nil {
			aggregated = &aggregatedExitPlan{
				side:         plan.side,
				positionSide: plan.positionSide,
				exitLevelIdx: plan.exitLevelIdx,
				price:        plan.price,
			}
			plansBySignature[signature] = aggregated
			signatures = append(signatures, signature)
		}
		aggregated.totalQty += lot.RemainingQty
		aggregated.lots = append(aggregated.lots, lot)
	}

	for _, signature := range signatures {
		plan := plansBySignature[signature]
		if plan == nil || plan.totalQty <= 0 {
			continue
		}

		orderID, err := at.ensureStandaloneReduceOnlyOrder(plan.side, plan.positionSide, plan.totalQty, plan.price)
		if err != nil {
			logger.Warnf("[Grid] Failed to seed aggregated lot-based adjacent exit for %s %s totalQty=%.4f at $%s: %v",
				plan.side, plan.positionSide, plan.totalQty, formatGridLogPrice(plan.price), err)
			continue
		}
		if orderID == "" {
			if openOrders, lookupErr := at.trader.GetOpenOrders(gridConfig.Symbol); lookupErr == nil {
				orderID = at.findCoveringReduceOnlyOrderID(gridConfig.Symbol, openOrders, plan.side, plan.positionSide, plan.price, plan.totalQty)
			}
		}

		for _, lot := range plan.lots {
			if err := at.store.Grid().UpdateInventoryLotExitIntent(lot.ID, plan.exitLevelIdx, orderID); err != nil {
				logger.Warnf("[Grid] Failed to persist exit intent for lot %s: %v", lot.ID, err)
				continue
			}
			if orderID == "" {
				logger.Warnf("[Grid] Anchored aggregated lot-based adjacent exit for lot %s (%s) at $%s, but no concrete exit order ID was resolved in this cycle",
					lot.ID, plan.positionSide, formatGridLogPrice(plan.price))
				continue
			}
			logger.Infof("[Grid] Seeded aggregated lot-based adjacent exit for lot %s (%s) at $%s via order %s",
				lot.ID, plan.positionSide, formatGridLogPrice(plan.price), orderID)
		}
	}
	return true
}

func (at *AutoTrader) ensurePairedExitOrderForInventoryLot(lot store.GridInventoryLotModel) {
	plan, ok := at.buildInventoryLotExitPlan(lot)
	if !ok {
		return
	}
	orderID, err := at.ensureStandaloneReduceOnlyOrder(plan.side, plan.positionSide, lot.RemainingQty, plan.price)
	if err != nil {
		logger.Warnf("[Grid] Failed to seed lot-based adjacent exit for lot %s (%s) from entry $%s to level %d exit $%s: %v",
			lot.ID, plan.positionSide, formatGridLogPrice(lot.EntryPrice), plan.exitLevelIdx, formatGridLogPrice(plan.price), err)
		return
	}
	if orderID == "" {
		if openOrders, lookupErr := at.trader.GetOpenOrders(at.config.StrategyConfig.GridConfig.Symbol); lookupErr == nil {
			orderID = at.findCoveringReduceOnlyOrderID(at.config.StrategyConfig.GridConfig.Symbol, openOrders, plan.side, plan.positionSide, plan.price, lot.RemainingQty)
		}
	}
	if at.store != nil {
		if err := at.store.Grid().UpdateInventoryLotExitIntent(lot.ID, plan.exitLevelIdx, orderID); err != nil {
			logger.Warnf("[Grid] Failed to persist exit intent for lot %s: %v", lot.ID, err)
		}
	}
}

type inventoryLotExitPlan struct {
	side         string
	positionSide string
	exitLevelIdx int
	price        float64
}

func (at *AutoTrader) buildInventoryLotExitPlan(lot store.GridInventoryLotModel) (inventoryLotExitPlan, bool) {
	if lot.RemainingQty <= 0 {
		return inventoryLotExitPlan{}, false
	}

	at.gridState.mu.RLock()
	if len(at.gridState.Levels) == 0 {
		at.gridState.mu.RUnlock()
		return inventoryLotExitPlan{}, false
	}

	entryLevelIdx := lot.SourceLevelIndex
	if entryLevelIdx < 0 || entryLevelIdx >= len(at.gridState.Levels) {
		entryLevelIdx = at.findClosestGridLevelIndexLocked(lot.EntryPrice, nil)
	}
	if entryLevelIdx < 0 || entryLevelIdx >= len(at.gridState.Levels) {
		at.gridState.mu.RUnlock()
		return inventoryLotExitPlan{}, false
	}

	gridSpacing := at.gridState.GridSpacing
	upperPrice := at.gridState.UpperPrice
	lowerPrice := at.gridState.LowerPrice

	positionSide := strings.ToUpper(strings.TrimSpace(lot.PositionSide))
	if active, _ := at.aggregatedRiskExitState(positionSide); active {
		at.gridState.mu.RUnlock()
		return inventoryLotExitPlan{}, false
	}

	step := 0
	exitSide := ""
	switch positionSide {
	case "LONG":
		step = 1
		exitSide = "SELL"
	case "SHORT":
		step = -1
		exitSide = "BUY"
	default:
		at.gridState.mu.RUnlock()
		return inventoryLotExitPlan{}, false
	}

	exitLevelIdx := lot.ExitLevelIndex
	if exitLevelIdx < 0 {
		exitLevelIdx = entryLevelIdx + step
	}
	if exitLevelIdx < 0 || exitLevelIdx >= len(at.gridState.Levels) {
		at.gridState.mu.RUnlock()
		fallbackPrice := 0.0
		if positionSide == "LONG" {
			fallbackPrice = upperPrice + gridSpacing
		} else {
			fallbackPrice = lowerPrice - gridSpacing
		}
		if fallbackPrice <= 0 {
			return inventoryLotExitPlan{}, false
		}
		return inventoryLotExitPlan{
			side:         exitSide,
			positionSide: positionSide,
			exitLevelIdx: exitLevelIdx,
			price:        fallbackPrice,
		}, true
	}

	entryLevelPrice := at.gridState.Levels[entryLevelIdx].Price
	exitPrice := at.gridState.Levels[exitLevelIdx].Price
	at.gridState.mu.RUnlock()

	minSpacing := at.minimumEntrySpacing()
	if minSpacing > 0 && math.Abs(exitPrice-entryLevelPrice) < minSpacing {
		if positionSide == "LONG" {
			exitPrice = entryLevelPrice + minSpacing
		} else {
			exitPrice = entryLevelPrice - minSpacing
		}
		if formatter, ok := at.trader.(interface {
			FormatPrice(symbol string, price float64) (string, error)
		}); ok {
			if formatted, err := formatter.FormatPrice(at.config.StrategyConfig.GridConfig.Symbol, exitPrice); err == nil {
				if normalized, parseErr := strconv.ParseFloat(formatted, 64); parseErr == nil && normalized > 0 {
					exitPrice = normalized
				}
			}
		}
		logger.Warnf("[Grid] Adjusted lot-based adjacent exit for lot %s (%s): entry level %d @$%s, adjacent level %d @$%s too close; clamped exit to minimum spacing price $%s",
			lot.ID, positionSide, entryLevelIdx, formatGridLogPrice(entryLevelPrice), exitLevelIdx, formatGridLogPrice(at.gridState.Levels[exitLevelIdx].Price), formatGridLogPrice(exitPrice))
	}

	return inventoryLotExitPlan{
		side:         exitSide,
		positionSide: positionSide,
		exitLevelIdx: exitLevelIdx,
		price:        exitPrice,
	}, true
}

func (at *AutoTrader) cleanupNonAdjacentPairedExitOrders() {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil || at.gridState == nil {
		return
	}

	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	type unmatchedReduceOnlyOrder struct {
		order        OpenOrder
		positionSide string
		signature    string
		price        float64
		quantity     float64
	}

	expected := make(map[string]struct{})
	expectedOrderIDs := make(map[string]struct{})
	expectedQtyBySide := map[string]float64{"LONG": 0, "SHORT": 0}
	usedLots := false
	if active, price := at.aggregatedRiskExitState("LONG"); active {
		priceKey := at.normalizeGridPriceKey(gridConfig.Symbol, price)
		if priceKey != "" {
			expected["SELL|LONG|"+priceKey] = struct{}{}
		}
	}
	if active, price := at.aggregatedRiskExitState("SHORT"); active {
		priceKey := at.normalizeGridPriceKey(gridConfig.Symbol, price)
		if priceKey != "" {
			expected["BUY|SHORT|"+priceKey] = struct{}{}
		}
	}
	if at.store != nil {
		if lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, gridConfig.Symbol); err == nil && len(lots) > 0 {
			at.gridState.mu.RLock()
			for _, lot := range lots {
				if lot.RemainingQty <= 0 {
					continue
				}
				positionSide := strings.ToUpper(strings.TrimSpace(lot.PositionSide))
				expectedQtyBySide[positionSide] += lot.RemainingQty
				exitLevelIdx := lot.ExitLevelIndex
				if exitLevelIdx < 0 {
					switch positionSide {
					case "LONG":
						exitLevelIdx = lot.SourceLevelIndex + 1
					case "SHORT":
						exitLevelIdx = lot.SourceLevelIndex - 1
					}
				}
				if exitLevelIdx < 0 || exitLevelIdx >= len(at.gridState.Levels) {
					continue
				}
				exitSide := "SELL"
				if positionSide == "SHORT" {
					exitSide = "BUY"
				}
				priceKey := at.normalizeGridPriceKey(gridConfig.Symbol, at.gridState.Levels[exitLevelIdx].Price)
				if priceKey == "" {
					continue
				}
				expected[strings.ToUpper(exitSide)+"|"+positionSide+"|"+priceKey] = struct{}{}
				if strings.TrimSpace(lot.ExitOrderID) != "" {
					expectedOrderIDs[lot.ExitOrderID] = struct{}{}
				}
				usedLots = true
			}
			at.gridState.mu.RUnlock()
		}
	}
	if !usedLots {
		at.gridState.mu.RLock()
		for idx, level := range at.gridState.Levels {
			if level.State != "filled" || level.PositionSize <= 0 {
				continue
			}

			positionSide := normalizeGridPositionSide(level.PositionSide, level.Side)
			expectedQtyBySide[positionSide] += level.PositionSize
			exitLevelIdx := -1
			exitSide := ""
			switch positionSide {
			case "LONG":
				exitLevelIdx = idx + 1
				exitSide = "SELL"
			case "SHORT":
				exitLevelIdx = idx - 1
				exitSide = "BUY"
			default:
				continue
			}

			if exitLevelIdx < 0 || exitLevelIdx >= len(at.gridState.Levels) {
				continue
			}

			priceKey := at.normalizeGridPriceKey(gridConfig.Symbol, at.gridState.Levels[exitLevelIdx].Price)
			if priceKey == "" {
				continue
			}
			expected[strings.ToUpper(exitSide)+"|"+positionSide+"|"+priceKey] = struct{}{}
		}
		at.gridState.mu.RUnlock()
	}

	liveQtyBySide := map[string]float64{
		"LONG":  at.getCurrentPositionQuantity(gridConfig.Symbol, "LONG"),
		"SHORT": at.getCurrentPositionQuantity(gridConfig.Symbol, "SHORT"),
	}

	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to query open orders while cleaning non-adjacent exits: %v", err)
		return
	}

	unmatchedBySide := map[string][]unmatchedReduceOnlyOrder{
		"LONG":  make([]unmatchedReduceOnlyOrder, 0),
		"SHORT": make([]unmatchedReduceOnlyOrder, 0),
	}
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if !inferGridReduceOnly(order.Side, positionSide) {
			continue
		}

		at.gridState.mu.Lock()
		if seededAt, ok := at.gridState.RecentStandaloneReduceOnlyIDs[order.OrderID]; ok {
			if time.Since(seededAt) < 45*time.Second {
				at.gridState.mu.Unlock()
				continue
			}
			delete(at.gridState.RecentStandaloneReduceOnlyIDs, order.OrderID)
		}
		at.gridState.mu.Unlock()

		if _, ok := expectedOrderIDs[order.OrderID]; ok {
			continue
		}
		priceKey := at.normalizeGridPriceKey(gridConfig.Symbol, gridOrderPrice(order))
		signature := strings.ToUpper(order.Side) + "|" + positionSide + "|" + priceKey
		if _, ok := expected[signature]; ok {
			continue
		}

		unmatchedBySide[positionSide] = append(unmatchedBySide[positionSide], unmatchedReduceOnlyOrder{
			order:        order,
			positionSide: positionSide,
			signature:    signature,
			price:        gridOrderPrice(order),
			quantity:     math.Abs(order.Quantity),
		})
	}

	preservedOrderIDs := make(map[string]struct{})
	for _, side := range []string{"LONG", "SHORT"} {
		deficitQty := liveQtyBySide[side] - expectedQtyBySide[side]
		if deficitQty <= gridLotQtyTolerance {
			continue
		}
		candidates := unmatchedBySide[side]
		if len(candidates) == 0 {
			continue
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].price == candidates[j].price {
				return candidates[i].order.OrderID < candidates[j].order.OrderID
			}
			return candidates[i].price > candidates[j].price
		})
		coveredQty := 0.0
		for _, candidate := range candidates {
			if coveredQty >= deficitQty-gridLotQtyTolerance {
				break
			}
			preservedOrderIDs[candidate.order.OrderID] = struct{}{}
			coveredQty += candidate.quantity
		}
		if coveredQty > 0 {
			logger.Warnf("[Grid] Preserved %.4f qty of unmatched %s reduce-only exits because live position exceeds tracked lots by %.4f",
				coveredQty, side, deficitQty)
		}
	}

	for _, side := range []string{"LONG", "SHORT"} {
		for _, candidate := range unmatchedBySide[side] {
			if _, ok := preservedOrderIDs[candidate.order.OrderID]; ok {
				continue
			}

			if err := gridTrader.CancelOrder(gridConfig.Symbol, candidate.order.OrderID); err != nil {
				logger.Warnf("[Grid] Failed to cancel non-adjacent reduce-only order %s at $%s: %v",
					candidate.order.OrderID, formatGridLogPrice(candidate.price), err)
				continue
			}

			logger.Infof("[Grid] Cancelled non-adjacent reduce-only order %s at $%s to re-align exits with adjacent grid levels",
				candidate.order.OrderID, formatGridLogPrice(candidate.price))

			at.gridState.mu.Lock()
			delete(at.gridState.StandaloneReduceOnlyBook, candidate.signature)
			if levelIdx, ok := at.gridState.OrderBook[candidate.order.OrderID]; ok {
				if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) && at.gridState.Levels[levelIdx].OrderID == candidate.order.OrderID {
					clearGridPendingOrder(&at.gridState.Levels[levelIdx])
				}
				delete(at.gridState.OrderBook, candidate.order.OrderID)
			}
			at.gridState.mu.Unlock()
		}
	}
}

func (at *AutoTrader) refreshInventoryLotExitBindings(symbol string, openOrders []OpenOrder) {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return
	}

	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, symbol)
	if err != nil || len(lots) == 0 {
		return
	}

	at.gridState.mu.RLock()
	if len(at.gridState.Levels) == 0 {
		at.gridState.mu.RUnlock()
		return
	}

	type key struct {
		side         string
		positionSide string
		priceKey     string
	}
	type bindingCandidate struct {
		orderID      string
		totalQty     float64
		remainingQty float64
	}
	ordersByKey := make(map[key][]*bindingCandidate)
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if !inferGridReduceOnly(order.Side, positionSide) {
			continue
		}
		priceKey := at.normalizeGridPriceKey(symbol, gridOrderPrice(order))
		if priceKey == "" {
			continue
		}
		qty := math.Abs(order.Quantity)
		ordersByKey[key{side: strings.ToUpper(order.Side), positionSide: positionSide, priceKey: priceKey}] = append(
			ordersByKey[key{side: strings.ToUpper(order.Side), positionSide: positionSide, priceKey: priceKey}],
			&bindingCandidate{orderID: order.OrderID, totalQty: qty, remainingQty: qty},
		)
	}
	for _, candidates := range ordersByKey {
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].remainingQty > candidates[j].remainingQty
		})
	}

	updates := make([]struct {
		id           string
		exitLevelIdx int
		orderID      string
	}, 0)
	for _, lot := range lots {
		if lot.RemainingQty <= 0 {
			continue
		}
		positionSide := strings.ToUpper(strings.TrimSpace(lot.PositionSide))
		exitLevelIdx := lot.ExitLevelIndex
		if exitLevelIdx < 0 {
			continue
		}
		if exitLevelIdx >= len(at.gridState.Levels) {
			continue
		}
		exitSide := "SELL"
		if positionSide == "SHORT" {
			exitSide = "BUY"
		}
		priceKey := at.normalizeGridPriceKey(symbol, at.gridState.Levels[exitLevelIdx].Price)
		if priceKey == "" {
			continue
		}
		candidates := ordersByKey[key{side: exitSide, positionSide: positionSide, priceKey: priceKey}]
		requiredQty := at.normalizeExecutableQuantity(symbol, lot.RemainingQty)
		if requiredQty <= 0 {
			requiredQty = lot.RemainingQty
		}
		orderID := ""
		for _, candidate := range candidates {
			if candidate.remainingQty+0.5 < requiredQty {
				continue
			}
			orderID = candidate.orderID
			candidate.remainingQty -= requiredQty
			break
		}
		if orderID == "" {
			continue
		}
		if lot.ExitOrderID == orderID {
			continue
		}
		updates = append(updates, struct {
			id           string
			exitLevelIdx int
			orderID      string
		}{id: lot.ID, exitLevelIdx: exitLevelIdx, orderID: orderID})
	}
	at.gridState.mu.RUnlock()

	for _, update := range updates {
		if err := at.store.Grid().UpdateInventoryLotExitIntent(update.id, update.exitLevelIdx, update.orderID); err != nil {
			logger.Warnf("[Grid] Failed to refresh exit binding for lot %s: %v", update.id, err)
		}
	}
}

func (at *AutoTrader) findCoveringReduceOnlyOrderID(symbol string, openOrders []OpenOrder, side string, positionSide string, price float64, quantity float64) string {
	targetPriceKey := at.normalizeGridPriceKey(symbol, price)
	requiredQty := at.normalizeExecutableQuantity(symbol, quantity)
	if requiredQty <= 0 {
		requiredQty = quantity
	}

	bestOrderID := ""
	bestQty := 0.0
	for _, order := range openOrders {
		if !strings.EqualFold(order.Side, side) {
			continue
		}
		normalizedPositionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if normalizedPositionSide != strings.ToUpper(positionSide) {
			continue
		}
		if !inferGridReduceOnly(order.Side, normalizedPositionSide) {
			continue
		}
		if at.normalizeGridPriceKey(symbol, gridOrderPrice(order)) != targetPriceKey {
			continue
		}
		qty := math.Abs(order.Quantity)
		if qty+0.5 < requiredQty {
			continue
		}
		if qty > bestQty {
			bestQty = qty
			bestOrderID = order.OrderID
		}
	}

	return bestOrderID
}

func (at *AutoTrader) findLiveOpenOrderByID(symbol string, orderID string) (*OpenOrder, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, nil
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return nil, err
	}

	for _, order := range openOrders {
		if order.OrderID == orderID {
			copy := order
			return &copy, nil
		}
	}

	return nil, nil
}

func inferGridReduceOnly(side string, positionSide string) bool {
	orderSide := strings.ToUpper(strings.TrimSpace(side))
	normalizedPositionSide := strings.ToUpper(strings.TrimSpace(positionSide))
	return (orderSide == "SELL" && normalizedPositionSide == "LONG") ||
		(orderSide == "BUY" && normalizedPositionSide == "SHORT")
}

func (at *AutoTrader) standaloneReduceOnlySignature(symbol string, side string, positionSide string, price float64) string {
	return strings.ToUpper(strings.TrimSpace(side)) + "|" +
		strings.ToUpper(strings.TrimSpace(positionSide)) + "|" +
		at.normalizeGridPriceKey(symbol, price)
}

func (at *AutoTrader) rebuildStandaloneReduceOnlyBook(symbol string, openOrders []OpenOrder) {
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	at.gridState.StandaloneReduceOnlyBook = make(map[string]float64)
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if !inferGridReduceOnly(order.Side, positionSide) {
			continue
		}
		signature := at.standaloneReduceOnlySignature(symbol, order.Side, positionSide, gridOrderPrice(order))
		if strings.HasSuffix(signature, "|") {
			continue
		}
		at.gridState.StandaloneReduceOnlyBook[signature] += math.Abs(order.Quantity)
	}
}

func (at *AutoTrader) findClosestGridLevelIndexLocked(price float64, predicate func(level kernel.GridLevelInfo) bool) int {
	type candidate struct {
		idx  int
		dist float64
	}

	candidates := make([]candidate, 0, len(at.gridState.Levels))
	for idx, level := range at.gridState.Levels {
		if predicate != nil && !predicate(level) {
			continue
		}
		candidates = append(candidates, candidate{
			idx:  idx,
			dist: math.Abs(level.Price - price),
		})
	}

	if len(candidates) == 0 {
		return -1
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist == candidates[j].dist {
			return candidates[i].idx < candidates[j].idx
		}
		return candidates[i].dist < candidates[j].dist
	})

	return candidates[0].idx
}

func (at *AutoTrader) bootstrapGridStateFromExchange() {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil || at.gridState == nil {
		return
	}

	restoredFromLots := at.restoreGridInventoryLotsToState()

	if restoredFromLots == 0 {
		positions, err := at.trader.GetPositions()
		if err != nil {
			logger.Warnf("[Grid] Failed to bootstrap positions from exchange: %v", err)
		} else {
			at.gridState.mu.Lock()
			restoredPositions := 0
			for _, pos := range positions {
				symbol, _ := pos["symbol"].(string)
				if symbol != gridConfig.Symbol {
					continue
				}

				size, _ := pos["positionAmt"].(float64)
				if size == 0 {
					continue
				}

				entryPrice, _ := pos["entryPrice"].(float64)
				if entryPrice <= 0 {
					continue
				}

				side, _ := pos["side"].(string)
				positionSide := normalizeGridPositionSide(side, "")
				levelIdx := at.findClosestGridLevelIndexLocked(entryPrice, func(level kernel.GridLevelInfo) bool {
					return !gridLevelHasFilledPosition(level)
				})
				if levelIdx < 0 {
					continue
				}

				level := &at.gridState.Levels[levelIdx]
				level.State = "filled"
				level.PositionEntry = entryPrice
				level.PositionSize = math.Abs(size)
				level.PositionSide = positionSide
				level.OrderID = ""
				level.OrderQuantity = 0
				level.OrderPositionSide = ""
				level.OrderReduceOnly = false
				level.LinkedLevelIndex = 0
				restoredPositions++
			}
			at.gridState.mu.Unlock()

			if restoredPositions > 0 {
				logger.Infof("[Grid] Restored %d existing positions from exchange into grid state", restoredPositions)
			}
		}
	} else {
		logger.Infof("[Grid] Skipped net-position bootstrap because %d persistent grid lots were restored", restoredFromLots)
	}

	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to bootstrap open orders from exchange: %v", err)
		return
	}

	at.rebuildStandaloneReduceOnlyBook(gridConfig.Symbol, openOrders)

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	restoredOrders := 0
	for _, order := range openOrders {
		orderPrice := order.Price
		if orderPrice <= 0 {
			orderPrice = order.StopPrice
		}
		if orderPrice <= 0 {
			continue
		}

		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if inferGridReduceOnly(order.Side, positionSide) {
			// Paired exits are managed as standalone reduce-only orders and must
			// not occupy entry grid levels when rebuilding runtime state.
			continue
		}

		levelIdx := at.findClosestGridLevelIndexLocked(orderPrice, func(level kernel.GridLevelInfo) bool {
			return gridLevelCanHostEntryOrder(level)
		})
		if levelIdx < 0 {
			continue
		}

		level := &at.gridState.Levels[levelIdx]
		if !gridLevelHasFilledPosition(*level) {
			level.State = "pending"
		}
		level.OrderID = order.OrderID
		level.OrderQuantity = order.Quantity
		level.OrderPositionSide = positionSide
		level.OrderReduceOnly = inferGridReduceOnly(order.Side, positionSide)
		level.LinkedLevelIndex = 0
		at.gridState.OrderBook[order.OrderID] = levelIdx
		restoredOrders++
	}

	if restoredOrders > 0 {
		logger.Infof("[Grid] Restored %d existing open orders from exchange into grid state", restoredOrders)
	}
}

func (at *AutoTrader) resolveGridOrderIntent(d *kernel.Decision, side string) (string, float64, bool, int, error) {
	if d.LevelIndex < 0 || d.LevelIndex >= len(at.gridState.Levels) {
		return "", 0, false, -1, fmt.Errorf("invalid grid level index %d", d.LevelIndex)
	}

	switch side {
	case "SELL":
		return "SHORT", d.Quantity, false, -1, nil
	case "BUY":
		return "LONG", d.Quantity, false, -1, nil
	default:
		return "", d.Quantity, false, -1, fmt.Errorf("unsupported grid order side %s", side)
	}
}

func (at *AutoTrader) placeGridReduceOnlyOrder(levelIdx int, side string, positionSide string, quantity float64, price float64, linkedLevelIndex int) error {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	if quantity <= 0 {
		return fmt.Errorf("grid reduce-only %s order has non-positive quantity %.4f", side, quantity)
	}

	req := &LimitOrderRequest{
		Symbol:       gridConfig.Symbol,
		Side:         side,
		PositionSide: positionSide,
		Price:        price,
		Quantity:     quantity,
		Leverage:     gridConfig.Leverage,
		PostOnly:     gridConfig.UseMakerOnly,
		ReduceOnly:   true,
		ClientID:     fmt.Sprintf("grid-exit-%d-%d", levelIdx, time.Now().UnixNano()%1000000),
	}

	result, err := gridTrader.PlaceLimitOrder(req)
	if err != nil {
		return fmt.Errorf("failed to place reduce-only limit order: %w", err)
	}

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) {
		level := &at.gridState.Levels[levelIdx]
		level.State = "pending"
		level.OrderID = result.OrderID
		level.OrderQuantity = quantity
		level.OrderPositionSide = positionSide
		level.OrderReduceOnly = true
		level.LinkedLevelIndex = linkedLevelIndex
		at.gridState.OrderBook[result.OrderID] = levelIdx
	}

	logger.Infof("[Grid] Placed %s %s reduce-only limit order at $%.2f, qty=%.4f, level=%d, linkedLevel=%d, orderID=%s",
		side, positionSide, price, quantity, levelIdx, linkedLevelIndex, result.OrderID)
	return nil
}

func (at *AutoTrader) ensureStandaloneReduceOnlyOrder(side string, positionSide string, quantity float64, price float64) (string, error) {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	currentPositionQty := at.getCurrentPositionQuantity(gridConfig.Symbol, positionSide)
	if currentPositionQty <= 0 {
		return "", nil
	}

	signature := at.standaloneReduceOnlySignature(gridConfig.Symbol, side, positionSide, price)
	at.gridState.mu.RLock()
	reservedQty := at.gridState.StandaloneReduceOnlyBook[signature]
	at.gridState.mu.RUnlock()

	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		return "", fmt.Errorf("failed to query open orders for standalone reduce-only order: %w", err)
	}

	targetPriceKey := at.normalizeGridPriceKey(gridConfig.Symbol, price)
	existingOrderIDs := make([]string, 0)
	existingQuantity := 0.0
	otherReduceQty := 0.0
	for _, order := range openOrders {
		normalizedPositionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if normalizedPositionSide != strings.ToUpper(positionSide) {
			continue
		}
		if !inferGridReduceOnly(order.Side, normalizedPositionSide) {
			continue
		}
		qty := math.Abs(order.Quantity)
		if strings.EqualFold(order.Side, side) && at.normalizeGridPriceKey(gridConfig.Symbol, gridOrderPrice(order)) == targetPriceKey {
			existingOrderIDs = append(existingOrderIDs, order.OrderID)
			existingQuantity += qty
			continue
		}
		otherReduceQty += qty
	}

	reservedReduceQty := at.getReservedReduceOnlyQuantity(side, positionSide)
	otherReservedQty := reservedReduceQty - reservedQty
	if otherReservedQty < 0 {
		otherReservedQty = 0
	}
	if otherReservedQty > otherReduceQty {
		otherReduceQty = otherReservedQty
	}

	remainingReducibleQty := currentPositionQty - otherReduceQty
	if remainingReducibleQty <= 0 {
		return "", nil
	}

	desiredQty := math.Min(quantity, remainingReducibleQty)
	desiredQty = at.normalizeExecutableQuantity(gridConfig.Symbol, desiredQty)
	if desiredQty <= 0 {
		return "", nil
	}

	if len(existingOrderIDs) == 1 && math.Abs(existingQuantity-desiredQty) < 0.5 {
		return existingOrderIDs[0], nil
	}
	if reservedQty > 0 && len(existingOrderIDs) > 0 {
		if existingQuantity+0.5 >= desiredQty {
			return existingOrderIDs[0], nil
		}
	}
	if reservedQty > 0 && len(existingOrderIDs) == 0 {
		return "", nil
	}

	for _, orderID := range existingOrderIDs {
		if err := gridTrader.CancelOrder(gridConfig.Symbol, orderID); err != nil {
			return "", fmt.Errorf("failed to cancel existing reduce-only order %s before reseeding standalone order: %w", orderID, err)
		}

		at.gridState.mu.Lock()
		if levelIdx, ok := at.gridState.OrderBook[orderID]; ok {
			if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) && at.gridState.Levels[levelIdx].OrderID == orderID {
				clearGridPendingOrder(&at.gridState.Levels[levelIdx])
			}
			delete(at.gridState.OrderBook, orderID)
		}
		at.gridState.mu.Unlock()
	}

	req := &LimitOrderRequest{
		Symbol:       gridConfig.Symbol,
		Side:         side,
		PositionSide: positionSide,
		Price:        price,
		Quantity:     desiredQty,
		Leverage:     gridConfig.Leverage,
		PostOnly:     gridConfig.UseMakerOnly,
		ReduceOnly:   true,
		ClientID:     fmt.Sprintf("grid-edge-exit-%d", time.Now().UnixNano()%1000000),
	}

	result, err := gridTrader.PlaceLimitOrder(req)
	if err != nil {
		return "", fmt.Errorf("failed to place standalone reduce-only order: %w", err)
	}

	at.gridState.mu.Lock()
	at.gridState.StandaloneReduceOnlyBook[signature] = desiredQty
	if at.gridState.RecentStandaloneReduceOnlyIDs == nil {
		at.gridState.RecentStandaloneReduceOnlyIDs = make(map[string]time.Time)
	}
	at.gridState.RecentStandaloneReduceOnlyIDs[result.OrderID] = time.Now()
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Placed standalone %s %s reduce-only limit order at $%.2f, qty=%.4f, orderID=%s (position=%.4f otherReduce=%.4f remaining=%.4f)",
		side, positionSide, price, desiredQty, result.OrderID, currentPositionQty, otherReduceQty, remainingReducibleQty)
	return result.OrderID, nil
}

func gridOrderPrice(order OpenOrder) float64 {
	if order.Price > 0 {
		return order.Price
	}
	return order.StopPrice
}

func (at *AutoTrader) getCurrentPositionQuantity(symbol string, positionSide string) float64 {
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("[Grid] Failed to get positions for reduce-only sizing: %v", err)
		return 0
	}

	targetSide := strings.ToUpper(strings.TrimSpace(positionSide))
	for _, pos := range positions {
		sym, _ := pos["symbol"].(string)
		if sym != symbol {
			continue
		}
		size, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		side, _ := pos["side"].(string)
		if normalizeGridPositionSide(side, "") != targetSide {
			continue
		}
		return math.Abs(size)
	}

	return 0
}

func (at *AutoTrader) getExistingReduceOnlyQuantity(symbol string, side string, positionSide string) float64 {
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to get open orders for reduce-only sizing: %v", err)
		return 0
	}

	targetSide := strings.ToUpper(strings.TrimSpace(side))
	targetPositionSide := strings.ToUpper(strings.TrimSpace(positionSide))
	total := 0.0
	for _, order := range openOrders {
		if !strings.EqualFold(order.Side, targetSide) {
			continue
		}
		normalizedPositionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if normalizedPositionSide != targetPositionSide {
			continue
		}
		if !inferGridReduceOnly(order.Side, normalizedPositionSide) {
			continue
		}
		total += math.Abs(order.Quantity)
	}
	return total
}

func (at *AutoTrader) getReservedReduceOnlyQuantity(side string, positionSide string) float64 {
	if at.gridState == nil {
		return 0
	}
	targetPrefix := strings.ToUpper(strings.TrimSpace(side)) + "|" + strings.ToUpper(strings.TrimSpace(positionSide)) + "|"
	total := 0.0
	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()
	for signature, qty := range at.gridState.StandaloneReduceOnlyBook {
		if strings.HasPrefix(signature, targetPrefix) {
			total += math.Abs(qty)
		}
	}
	return total
}

func (at *AutoTrader) normalizeExecutableQuantity(symbol string, quantity float64) float64 {
	if quantity <= 0 {
		return 0
	}

	// Keep grid quantities in base units. Some exchange adapters, such as OKX,
	// use FormatQuantity to convert into contract units, and PlaceLimitOrder will
	// perform that conversion again during the real exchange request.
	if formatter, ok := at.trader.(interface {
		QuantityFormattingChangesUnit() bool
	}); ok && formatter.QuantityFormattingChangesUnit() {
		return quantity
	}

	formatter, ok := at.trader.(interface {
		FormatQuantity(symbol string, quantity float64) (string, error)
	})
	if !ok {
		return quantity
	}

	formatted, err := formatter.FormatQuantity(symbol, quantity)
	if err != nil {
		logger.Warnf("[Grid] Failed to format quantity for %s: %v", symbol, err)
		return quantity
	}

	normalized, err := strconv.ParseFloat(formatted, 64)
	if err != nil {
		logger.Warnf("[Grid] Failed to parse formatted quantity %q for %s: %v", formatted, symbol, err)
		return quantity
	}

	return normalized
}

func (at *AutoTrader) normalizeGridPriceKey(symbol string, price float64) string {
	if price <= 0 {
		return ""
	}

	if formatter, ok := at.trader.(interface {
		FormatPrice(symbol string, price float64) (string, error)
	}); ok {
		if formatted, err := formatter.FormatPrice(symbol, price); err == nil && formatted != "" {
			return formatted
		}
	}

	return strconv.FormatFloat(price, 'f', 8, 64)
}

func (at *AutoTrader) cancelDuplicateEntryOrdersAtPrice(symbol string, side string, positionSide string, price float64) error {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to query open orders for duplicate cleanup: %w", err)
	}

	targetPriceKey := at.normalizeGridPriceKey(symbol, price)
	cancelIDs := make([]string, 0)
	for _, order := range openOrders {
		if !strings.EqualFold(order.Side, side) {
			continue
		}
		if normalizeGridPositionSide(order.PositionSide, order.Side) != strings.ToUpper(positionSide) {
			continue
		}
		if inferGridReduceOnly(order.Side, normalizeGridPositionSide(order.PositionSide, order.Side)) {
			continue
		}
		if at.normalizeGridPriceKey(symbol, gridOrderPrice(order)) != targetPriceKey {
			continue
		}
		cancelIDs = append(cancelIDs, order.OrderID)
	}

	if len(cancelIDs) == 0 {
		return nil
	}

	for _, orderID := range cancelIDs {
		if err := gridTrader.CancelOrder(symbol, orderID); err != nil {
			return fmt.Errorf("failed to cancel duplicate entry order %s at $%.8f: %w", orderID, price, err)
		}

		at.gridState.mu.Lock()
		if levelIdx, ok := at.gridState.OrderBook[orderID]; ok {
			if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) {
				clearGridPendingOrder(&at.gridState.Levels[levelIdx])
			}
			delete(at.gridState.OrderBook, orderID)
		}
		at.gridState.mu.Unlock()
	}

	logger.Infof("[Grid] Cancelled %d duplicate entry order(s) for %s %s at $%.8f before placing the latest one",
		len(cancelIDs), side, positionSide, price)
	return nil
}

func (at *AutoTrader) minimumEntrySpacing() float64 {
	if at.gridState == nil {
		return 0
	}
	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()
	if at.gridState.GridSpacing <= 0 {
		return 0
	}
	return at.gridState.GridSpacing * 0.8
}

func (at *AutoTrader) existingEntryOrderPrices(symbol string, side string, positionSide string) ([]float64, error) {
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return nil, err
	}

	prices := make([]float64, 0, len(openOrders))
	for _, order := range openOrders {
		if !strings.EqualFold(order.Side, side) {
			continue
		}
		if normalizeGridPositionSide(order.PositionSide, order.Side) != strings.ToUpper(positionSide) {
			continue
		}
		if inferGridReduceOnly(order.Side, normalizeGridPositionSide(order.PositionSide, order.Side)) {
			continue
		}
		orderPrice := gridOrderPrice(order)
		if orderPrice <= 0 {
			continue
		}
		prices = append(prices, orderPrice)
	}
	return prices, nil
}

func (at *AutoTrader) isEntryPriceSpacingValid(symbol string, side string, positionSide string, price float64) (bool, error) {
	minSpacing := at.minimumEntrySpacing()
	if minSpacing <= 0 {
		return true, nil
	}

	existingPrices, err := at.existingEntryOrderPrices(symbol, side, positionSide)
	if err != nil {
		return false, err
	}

	for _, existingPrice := range existingPrices {
		if math.Abs(existingPrice-price) < minSpacing {
			return false, nil
		}
	}
	return true, nil
}

func (at *AutoTrader) findAlternativeEntryPlacement(symbol string, levelIdx int, side string, positionSide string, price float64) (int, float64, bool, error) {
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return levelIdx, price, false, fmt.Errorf("failed to query open orders for alternative entry placement: %w", err)
	}

	minSpacing := at.minimumEntrySpacing()
	occupiedPriceKeys := make(map[string]struct{})
	existingEntryPrices := make([]float64, 0, len(openOrders))
	for _, order := range openOrders {
		if !strings.EqualFold(order.Side, side) {
			continue
		}
		if normalizeGridPositionSide(order.PositionSide, order.Side) != strings.ToUpper(positionSide) {
			continue
		}
		if inferGridReduceOnly(order.Side, normalizeGridPositionSide(order.PositionSide, order.Side)) {
			continue
		}
		orderPrice := gridOrderPrice(order)
		priceKey := at.normalizeGridPriceKey(symbol, orderPrice)
		if priceKey != "" {
			occupiedPriceKeys[priceKey] = struct{}{}
		}
		if orderPrice > 0 {
			existingEntryPrices = append(existingEntryPrices, orderPrice)
		}
	}

	targetKey := at.normalizeGridPriceKey(symbol, price)
	tooClose := false
	if minSpacing > 0 {
		for _, existingPrice := range existingEntryPrices {
			if math.Abs(existingPrice-price) < minSpacing {
				tooClose = true
				break
			}
		}
	}
	if _, exists := occupiedPriceKeys[targetKey]; !exists && !tooClose {
		return levelIdx, price, false, nil
	}

	type candidate struct {
		idx   int
		price float64
	}

	at.gridState.mu.RLock()
	candidates := make([]candidate, 0, len(at.gridState.Levels))
	expectedSide := strings.ToLower(side)
	for distance := 1; distance < len(at.gridState.Levels); distance++ {
		for _, idx := range []int{levelIdx - distance, levelIdx + distance} {
			if idx < 0 || idx >= len(at.gridState.Levels) {
				continue
			}
			level := at.gridState.Levels[idx]
			if !gridLevelCanHostEntryOrder(level) {
				continue
			}
			if !strings.EqualFold(level.Side, expectedSide) {
				continue
			}
			candidates = append(candidates, candidate{idx: idx, price: level.Price})
		}
	}
	at.gridState.mu.RUnlock()

	for _, candidate := range candidates {
		candidateKey := at.normalizeGridPriceKey(symbol, candidate.price)
		if candidateKey == "" || candidateKey == targetKey {
			continue
		}
		if _, exists := occupiedPriceKeys[candidateKey]; exists {
			continue
		}
		if minSpacing > 0 {
			conflict := false
			for _, existingPrice := range existingEntryPrices {
				if math.Abs(existingPrice-candidate.price) < minSpacing {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
		}
		return candidate.idx, candidate.price, true, nil
	}

	return levelIdx, price, false, nil
}

func (at *AutoTrader) aggregatedRiskExitState(positionSide string) (bool, float64) {
	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()
	switch strings.ToUpper(strings.TrimSpace(positionSide)) {
	case "LONG":
		return at.gridState.AggregatedRiskExitLongActive, at.gridState.AggregatedRiskExitLongPrice
	case "SHORT":
		return at.gridState.AggregatedRiskExitShortActive, at.gridState.AggregatedRiskExitShortPrice
	default:
		return false, 0
	}
}

func (at *AutoTrader) setAggregatedRiskExitState(positionSide string, active bool, price float64) {
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()
	switch strings.ToUpper(strings.TrimSpace(positionSide)) {
	case "LONG":
		at.gridState.AggregatedRiskExitLongActive = active
		at.gridState.AggregatedRiskExitLongPrice = price
	case "SHORT":
		at.gridState.AggregatedRiskExitShortActive = active
		at.gridState.AggregatedRiskExitShortPrice = price
	}
}

func (at *AutoTrader) clearAggregatedRiskExitState(positionSide string) {
	at.setAggregatedRiskExitState(positionSide, false, 0)
}

type trappedInventorySidePlan struct {
	positionSide string
	exitSide     string
	totalQty     float64
	targetPrice  float64
	exitLevelIdx int
}

func (at *AutoTrader) detectTrappedInventorySidePlans(ctx *kernel.GridContext, openOrders []OpenOrder) ([]trappedInventorySidePlan, error) {
	if ctx == nil || ctx.BoxData == nil || at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil, nil
	}
	gridConfig := at.config.StrategyConfig.GridConfig
	investmentBase := at.currentGridInvestmentBase()
	if investmentBase <= 0 {
		return nil, nil
	}
	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, gridConfig.Symbol)
	if err != nil || len(lots) == 0 {
		return nil, err
	}

	type stats struct {
		oldest       time.Time
		totalQty     float64
		hasExit      bool
		exitsTrapped bool
	}
	bySide := map[string]*stats{}
	for _, lot := range lots {
		if lot.RemainingQty <= 0 {
			continue
		}
		side := strings.ToUpper(strings.TrimSpace(lot.PositionSide))
		st := bySide[side]
		if st == nil {
			st = &stats{exitsTrapped: true}
			bySide[side] = st
		}
		if st.oldest.IsZero() || lot.OpenedAt.Before(st.oldest) {
			st.oldest = lot.OpenedAt
		}
		st.totalQty += lot.RemainingQty
	}
	if len(bySide) == 0 {
		return nil, nil
	}

	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if !inferGridReduceOnly(order.Side, positionSide) {
			continue
		}
		st := bySide[positionSide]
		if st == nil {
			continue
		}
		st.hasExit = true
		price := gridOrderPrice(order)
		switch positionSide {
		case "LONG":
			if price <= ctx.BoxData.ShortUpper {
				st.exitsTrapped = false
			}
		case "SHORT":
			if price >= ctx.BoxData.ShortLower {
				st.exitsTrapped = false
			}
		}
	}

	midWidth := ctx.BoxData.MidUpper - ctx.BoxData.MidLower
	if midWidth <= 0 {
		return nil, nil
	}
	nearLowerEdge := ctx.CurrentPrice >= ctx.BoxData.MidLower && ctx.CurrentPrice <= ctx.BoxData.MidLower+midWidth*0.35
	nearUpperEdge := ctx.CurrentPrice <= ctx.BoxData.MidUpper && ctx.CurrentPrice >= ctx.BoxData.MidUpper-midWidth*0.35

	plans := make([]trappedInventorySidePlan, 0, 2)
	for side, st := range bySide {
		if st.oldest.IsZero() || time.Since(st.oldest) < 24*time.Hour {
			continue
		}
		notional := st.totalQty * ctx.CurrentPrice
		if notional/investmentBase < 1.0 {
			continue
		}
		if !st.hasExit || !st.exitsTrapped {
			continue
		}
		if side == "LONG" && !nearLowerEdge {
			continue
		}
		if side == "SHORT" && !nearUpperEdge {
			continue
		}
		price, exitLevelIdx, ok := at.computeTrappedInventoryExitPrice(side, ctx)
		if !ok {
			continue
		}
		exitSide := "SELL"
		if side == "SHORT" {
			exitSide = "BUY"
		}
		plans = append(plans, trappedInventorySidePlan{
			positionSide: side,
			exitSide:     exitSide,
			totalQty:     st.totalQty,
			targetPrice:  price,
			exitLevelIdx: exitLevelIdx,
		})
	}
	return plans, nil
}

func (at *AutoTrader) computeTrappedInventoryExitPrice(positionSide string, ctx *kernel.GridContext) (float64, int, bool) {
	if ctx == nil || ctx.BoxData == nil || at.gridState == nil {
		return 0, -1, false
	}
	shortLower := ctx.BoxData.ShortLower
	shortUpper := ctx.BoxData.ShortUpper
	if shortUpper <= shortLower || ctx.CurrentPrice <= 0 {
		return 0, -1, false
	}
	spacing := ctx.GridSpacing
	if spacing <= 0 {
		spacing = at.minimumEntrySpacing()
	}
	if spacing <= 0 {
		spacing = ctx.CurrentPrice * minGridSpacingPct
	}
	shortWidth := shortUpper - shortLower
	price := 0.0
	positionSide = strings.ToUpper(strings.TrimSpace(positionSide))
	if positionSide == "LONG" {
		minPrice := ctx.CurrentPrice + spacing
		maxPrice := shortUpper
		if minPrice > maxPrice {
			return 0, -1, false
		}
		price = shortLower + shortWidth*0.4
		if price < minPrice {
			price = minPrice
		}
		if price > maxPrice {
			price = maxPrice
		}
	} else if positionSide == "SHORT" {
		minPrice := shortLower
		maxPrice := ctx.CurrentPrice - spacing
		if maxPrice < minPrice {
			return 0, -1, false
		}
		price = shortUpper - shortWidth*0.4
		if price > maxPrice {
			price = maxPrice
		}
		if price < minPrice {
			price = minPrice
		}
	} else {
		return 0, -1, false
	}
	if formatter, ok := at.trader.(interface {
		FormatPrice(symbol string, price float64) (string, error)
	}); ok {
		if formatted, err := formatter.FormatPrice(at.config.StrategyConfig.GridConfig.Symbol, price); err == nil {
			if normalized, parseErr := strconv.ParseFloat(formatted, 64); parseErr == nil && normalized > 0 {
				price = normalized
			}
		}
	}
	at.gridState.mu.RLock()
	exitLevelIdx := at.findClosestGridLevelIndexLocked(price, nil)
	at.gridState.mu.RUnlock()
	return price, exitLevelIdx, price > 0
}

func (at *AutoTrader) hasMatchingAggregatedRiskExit(openOrders []OpenOrder, plan trappedInventorySidePlan) bool {
	for _, order := range openOrders {
		if !strings.EqualFold(order.Side, plan.exitSide) {
			continue
		}
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if positionSide != plan.positionSide {
			continue
		}
		if !inferGridReduceOnly(order.Side, positionSide) {
			continue
		}
		if at.normalizeGridPriceKey(at.config.StrategyConfig.GridConfig.Symbol, gridOrderPrice(order)) == at.normalizeGridPriceKey(at.config.StrategyConfig.GridConfig.Symbol, plan.targetPrice) {
			return true
		}
	}
	return false
}

func (at *AutoTrader) restoreAggregatedRiskExitStateFromLots(openOrders []OpenOrder) {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil || at.gridState == nil {
		return
	}
	gridConfig := at.config.StrategyConfig.GridConfig
	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, gridConfig.Symbol)
	if err != nil || len(lots) == 0 {
		return
	}
	liveBySide := map[string][]OpenOrder{"LONG": {}, "SHORT": {}}
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		if !inferGridReduceOnly(order.Side, positionSide) {
			continue
		}
		liveBySide[positionSide] = append(liveBySide[positionSide], order)
	}
	for _, side := range []string{"LONG", "SHORT"} {
		orders := liveBySide[side]
		if len(orders) != 1 {
			continue
		}
		exitLevelIdx := -2
		valid := false
		for _, lot := range lots {
			if lot.RemainingQty <= 0 || strings.ToUpper(strings.TrimSpace(lot.PositionSide)) != side {
				continue
			}
			if lot.ExitLevelIndex < 0 {
				valid = false
				exitLevelIdx = -2
				break
			}
			if exitLevelIdx == -2 {
				exitLevelIdx = lot.ExitLevelIndex
				valid = true
				continue
			}
			if lot.ExitLevelIndex != exitLevelIdx {
				valid = false
				exitLevelIdx = -2
				break
			}
		}
		if !valid || exitLevelIdx < 0 || exitLevelIdx >= len(at.gridState.Levels) {
			continue
		}
		livePrice := gridOrderPrice(orders[0])
		targetPrice := at.gridState.Levels[exitLevelIdx].Price
		if at.normalizeGridPriceKey(gridConfig.Symbol, livePrice) != at.normalizeGridPriceKey(gridConfig.Symbol, targetPrice) {
			continue
		}
		at.setAggregatedRiskExitState(side, true, livePrice)
	}
}

func (at *AutoTrader) checkAndMitigateTrappedInventoryRisk(ctx *kernel.GridContext) (bool, error) {
	if ctx == nil || ctx.BoxData == nil || at.gridState == nil || at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return false, nil
	}
	gridConfig := at.config.StrategyConfig.GridConfig
	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		return false, err
	}
	at.restoreAggregatedRiskExitStateFromLots(openOrders)
	plans, err := at.detectTrappedInventorySidePlans(ctx, openOrders)
	if err != nil || len(plans) == 0 {
		return false, err
	}

	hasEntryOrders := false
	for _, order := range openOrders {
		if !inferGridReduceOnly(order.Side, normalizeGridPositionSide(order.PositionSide, order.Side)) {
			hasEntryOrders = true
			break
		}
	}
	alreadyCovered := !hasEntryOrders
	for _, plan := range plans {
		if !at.hasMatchingAggregatedRiskExit(openOrders, plan) {
			alreadyCovered = false
			break
		}
	}
	if alreadyCovered {
		for _, plan := range plans {
			at.setAggregatedRiskExitState(plan.positionSide, true, plan.targetPrice)
		}
		return false, nil
	}

	if err := at.cancelAllGridOrders(); err != nil {
		return false, err
	}
	triggeredSides := make([]string, 0, len(plans))
	for _, plan := range plans {
		orderID, err := at.ensureStandaloneReduceOnlyOrder(plan.exitSide, plan.positionSide, plan.totalQty, plan.targetPrice)
		if err != nil {
			return false, err
		}
		at.setAggregatedRiskExitState(plan.positionSide, true, plan.targetPrice)
		if err := at.store.Grid().UpdateOpenInventoryLotsExitIntentBySide(at.id, gridConfig.Symbol, plan.positionSide, plan.exitLevelIdx, orderID); err != nil {
			logger.Warnf("[Grid] Failed to bulk persist trapped-inventory exit intent for %s: %v", plan.positionSide, err)
		}
		triggeredSides = append(triggeredSides, strings.ToLower(plan.positionSide))
		logger.Warnf("%s Activated trapped-inventory aggregated exit for %s at $%s (qty=%.4f, level=%d)",
			at.gridLogPrefix(), plan.positionSide, formatGridLogPrice(plan.targetPrice), plan.totalQty, plan.exitLevelIdx)
	}
	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.noteGridPaused("trapped_inventory_aggregate_exit_" + strings.Join(triggeredSides, "_"))
	at.gridState.mu.Unlock()
	return true, nil
}

func (at *AutoTrader) ensureAggregatedRiskExitOrders() {
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return
	}
	gridConfig := at.config.StrategyConfig.GridConfig
	for _, side := range []string{"LONG", "SHORT"} {
		active, price := at.aggregatedRiskExitState(side)
		if !active || price <= 0 {
			continue
		}
		qty := at.getCurrentPositionQuantity(gridConfig.Symbol, side)
		if qty <= 0 {
			at.clearAggregatedRiskExitState(side)
			continue
		}
		exitSide := "SELL"
		if side == "SHORT" {
			exitSide = "BUY"
		}
		orderID, err := at.ensureStandaloneReduceOnlyOrder(exitSide, side, qty, price)
		if err != nil {
			logger.Warnf("%s Failed to maintain aggregated risk exit for %s at $%s: %v", at.gridLogPrefix(), side, formatGridLogPrice(price), err)
			continue
		}
		exitLevelIdx := -1
		at.gridState.mu.RLock()
		exitLevelIdx = at.findClosestGridLevelIndexLocked(price, nil)
		at.gridState.mu.RUnlock()
		if at.store != nil {
			if err := at.store.Grid().UpdateOpenInventoryLotsExitIntentBySide(at.id, gridConfig.Symbol, side, exitLevelIdx, orderID); err != nil {
				logger.Warnf("[Grid] Failed to persist aggregated risk exit state for %s: %v", side, err)
			}
		}
	}

	at.gridState.mu.Lock()
	at.gridState.EntrySizingPolicyVersion = gridEntrySizingPolicyVersion
	at.gridState.mu.Unlock()
}

func (at *AutoTrader) activateAggregatedRiskExit(positionSide string, reason string) error {
	if at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	positionSide = strings.ToUpper(strings.TrimSpace(positionSide))
	if positionSide != "LONG" && positionSide != "SHORT" {
		return fmt.Errorf("unsupported aggregated risk exit position side %s", positionSide)
	}

	qty := at.getCurrentPositionQuantity(gridConfig.Symbol, positionSide)
	if qty <= 0 {
		at.clearAggregatedRiskExitState(positionSide)
		return nil
	}

	ctx, err := at.buildGridContext()
	if err != nil {
		return err
	}

	price, exitLevelIdx, ok := at.computeTrappedInventoryExitPrice(positionSide, ctx)
	if !ok || price <= 0 {
		currentPrice := ctx.CurrentPrice
		if currentPrice <= 0 {
			currentPrice, err = at.trader.GetMarketPrice(gridConfig.Symbol)
			if err != nil {
				return err
			}
		}
		spacing := ctx.GridSpacing
		if spacing <= 0 {
			spacing = at.minimumEntrySpacing()
		}
		if spacing <= 0 {
			spacing = currentPrice * minGridSpacingPct
		}
		if spacing <= 0 {
			return fmt.Errorf("unable to compute aggregated risk exit spacing")
		}
		if positionSide == "LONG" {
			price = currentPrice + spacing
		} else {
			price = currentPrice - spacing
		}
		if formatter, ok := at.trader.(interface {
			FormatPrice(symbol string, price float64) (string, error)
		}); ok {
			if formatted, ferr := formatter.FormatPrice(gridConfig.Symbol, price); ferr == nil {
				if normalized, perr := strconv.ParseFloat(formatted, 64); perr == nil && normalized > 0 {
					price = normalized
				}
			}
		}
		at.gridState.mu.RLock()
		exitLevelIdx = at.findClosestGridLevelIndexLocked(price, nil)
		at.gridState.mu.RUnlock()
	}

	if err := at.cancelAllGridOrders(); err != nil {
		return err
	}

	exitSide := "SELL"
	if positionSide == "SHORT" {
		exitSide = "BUY"
	}
	orderID, err := at.ensureStandaloneReduceOnlyOrder(exitSide, positionSide, qty, price)
	if err != nil {
		return err
	}

	at.setAggregatedRiskExitState(positionSide, true, price)
	if at.store != nil {
		if err := at.store.Grid().UpdateOpenInventoryLotsExitIntentBySide(at.id, gridConfig.Symbol, positionSide, exitLevelIdx, orderID); err != nil {
			logger.Warnf("[Grid] Failed to persist aggregated risk exit for %s: %v", positionSide, err)
		}
	}

	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.noteGridPaused(reason)
	at.gridState.mu.Unlock()

	logger.Warnf("%s Activated aggregated risk exit for %s at $%s (qty=%.4f, level=%d) due to %s",
		at.gridLogPrefix(), positionSide, formatGridLogPrice(price), qty, exitLevelIdx, reason)
	return nil
}

func (at *AutoTrader) findNearestAvailableExitLevelLocked(levelIdx int, step int) int {
	for idx := levelIdx + step; idx >= 0 && idx < len(at.gridState.Levels); idx += step {
		if at.gridState.Levels[idx].State != "filled" {
			return idx
		}
	}
	return -1
}

func (at *AutoTrader) suggestedEntryQuantity(levelIdx int, price float64) float64 {
	return at.suggestedEntryQuantityWithInvestmentBase(levelIdx, price, at.currentGridInvestmentBase())
}

func (at *AutoTrader) suggestedEntryQuantityWithInvestmentBase(levelIdx int, price float64, investmentBase float64) float64 {
	if price <= 0 || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return 0
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	allocatedUSD := at.currentGridLevelAllocatedUSD(levelIdx, investmentBase)
	if allocatedUSD <= 0 {
		return 0
	}

	quantity := (allocatedUSD * float64(gridConfig.Leverage)) / price
	currentPrice, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		currentPrice = 0
	}
	cappedQuantity, _ := at.capGridEntryQuantityByUniformLimit(levelIdx, price, quantity, investmentBase, currentPrice)
	return cappedQuantity
}

func (at *AutoTrader) currentAutoSeedBudget(candidateCount int) int {
	if candidateCount <= 0 {
		return 0
	}
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	limit := candidateCount
	if limit > 6 {
		limit = 6
	}

	if !at.gridState.ResumedAt.IsZero() {
		if time.Since(at.gridState.ResumedAt) <= 15*time.Minute {
			at.gridState.RecoverySeedWaves++
			switch at.gridState.RecoverySeedWaves {
			case 1:
				if limit > 2 {
					limit = 2
				}
			case 2:
				if limit > 4 {
					limit = 4
				}
			default:
				if limit > 6 {
					limit = 6
				}
			}
		} else {
			at.gridState.ResumedAt = time.Time{}
			at.gridState.RecoverySeedWaves = 0
		}
	}

	if limit < 1 {
		limit = 1
	}
	return limit
}

func (at *AutoTrader) autoSeedMissingEntryOrdersForRangingGrid(ctx *kernel.GridContext) error {
	if ctx == nil || ctx.IsPaused {
		if ctx != nil && ctx.IsPaused {
			logger.Infof("%s Auto-seed skipped: grid is paused", at.gridLogPrefix())
		}
		return nil
	}

	// Only auto-heal under-seeded grids in clearly ranging conditions.
	if !at.qualifiesAsProtectedRanging(ctx) {
		logger.Infof("%s Auto-seed skipped: market not clearly ranging (boll=%.2f ema=%.2f)", at.gridLogPrefix(), ctx.BollingerWidth, ctx.EMADistance)
		return nil
	}

	type candidate struct {
		levelIdx int
		price    float64
		dist     float64
	}

	hasNearbyLongEntry := false
	hasNearbyShortEntry := false
	nearbyBand := ctx.GridSpacing
	if nearbyBand <= 0 {
		nearbyBand = ctx.CurrentPrice * minGridSpacingPct
	}
	for _, summary := range ctx.AggregatedOpenOrders {
		switch summary.Bucket {
		case "long_entry":
			if summary.Price >= ctx.CurrentPrice-nearbyBand && summary.Price <= ctx.CurrentPrice {
				hasNearbyLongEntry = true
			}
		case "short_entry":
			if summary.Price <= ctx.CurrentPrice+nearbyBand && summary.Price >= ctx.CurrentPrice {
				hasNearbyShortEntry = true
			}
		}
	}
	balancedNearbyCoverage := hasNearbyLongEntry && hasNearbyShortEntry

	candidates := make([]candidate, 0, len(at.gridState.Levels))
	at.gridState.mu.RLock()
	for idx, level := range at.gridState.Levels {
		if !gridLevelCanHostEntryOrder(level) || level.Price <= 0 {
			continue
		}
		candidates = append(candidates, candidate{
			levelIdx: idx,
			price:    level.Price,
			dist:     math.Abs(level.Price - ctx.CurrentPrice),
		})
	}
	at.gridState.mu.RUnlock()

	if len(candidates) == 0 {
		logger.Infof("%s Auto-seed skipped: no empty candidate levels available", at.gridLogPrefix())
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist == candidates[j].dist {
			return candidates[i].levelIdx < candidates[j].levelIdx
		}
		return candidates[i].dist < candidates[j].dist
	})

	maxSeeds := at.currentAutoSeedBudget(len(candidates))

	seeded := 0
	skippedReduceOnly := 0
	skippedInvalid := 0
	skippedQty := 0
	failedPlacements := 0
	skippedNearCenter := 0
	for _, candidate := range candidates {
		if seeded >= maxSeeds {
			break
		}

		at.gridState.mu.RLock()
		if candidate.levelIdx < 0 || candidate.levelIdx >= len(at.gridState.Levels) {
			at.gridState.mu.RUnlock()
			continue
		}
		level := at.gridState.Levels[candidate.levelIdx]
		at.gridState.mu.RUnlock()
		if !gridLevelCanHostEntryOrder(level) {
			continue
		}

		if balancedNearbyCoverage && candidate.dist <= nearbyBand {
			skippedNearCenter++
			continue
		}

		action := "place_buy_limit"
		if strings.EqualFold(level.Side, "sell") {
			action = "place_sell_limit"
		}

		quantity := at.suggestedEntryQuantity(candidate.levelIdx, candidate.price)
		if quantity <= 0 {
			skippedQty++
			continue
		}

		decision := &kernel.Decision{
			Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
			Action:     action,
			Price:      candidate.price,
			Quantity:   quantity,
			LevelIndex: candidate.levelIdx,
			Confidence: 96,
			Reasoning:  "Ranging market under-seeded grid auto-recovery: no real entry orders were present, so missing grid entry coverage was reseeded automatically.",
			ForceEntry: true,
		}

		if err := at.executeGridDecision(decision); err != nil {
			logger.Warnf("[Grid] Auto-seed entry order failed at level %d ($%s): %v",
				candidate.levelIdx, formatGridLogPrice(candidate.price), err)
			failedPlacements++
			continue
		}
		seeded++
	}

	if seeded > 0 {
		logger.Infof("%s Auto-seeded %d missing entry orders to restore full ranging grid coverage", at.gridLogPrefix(), seeded)
	} else {
		logger.Infof("%s Auto-seed completed with no new entries (entry long=%d short=%d, candidates=%d, nearCenterSkips=%d, reduceOnlySkips=%d, invalidSkips=%d, qtySkips=%d, placementFailures=%d)",
			at.gridLogPrefix(), ctx.ActiveLongOrderCount, ctx.ActiveShortOrderCount, len(candidates), skippedNearCenter, skippedReduceOnly, skippedInvalid, skippedQty, failedPlacements)
	}

	return nil
}

func (at *AutoTrader) seedFullEntryGrid(investmentBase float64) error {
	if at == nil || at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil
	}
	if investmentBase <= 0 {
		investmentBase = at.currentGridInvestmentBase()
	}

	type entryLevel struct {
		levelIdx int
		price    float64
		side     string
		qty      float64
	}

	entries := make([]entryLevel, 0, len(at.gridState.Levels))
	at.gridState.mu.RLock()
	for idx, level := range at.gridState.Levels {
		if level.Price <= 0 || !gridLevelCanHostEntryOrder(level) {
			continue
		}
		side := "BUY"
		if strings.EqualFold(level.Side, "sell") {
			side = "SELL"
		}
		entries = append(entries, entryLevel{
			levelIdx: idx,
			price:    level.Price,
			side:     side,
			qty:      at.suggestedEntryQuantityWithInvestmentBase(idx, level.Price, investmentBase),
		})
	}
	at.gridState.mu.RUnlock()

	placed := 0
	var firstErr error
	for _, entry := range entries {
		if entry.qty <= 0 {
			if firstErr == nil {
				firstErr = fmt.Errorf("grid level %d has non-positive seeded quantity", entry.levelIdx)
			}
			continue
		}

		action := "place_buy_limit"
		if entry.side == "SELL" {
			action = "place_sell_limit"
		}

		err := at.executeGridDecision(&kernel.Decision{
			Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
			Action:     action,
			Price:      entry.price,
			Quantity:   entry.qty,
			LevelIndex: entry.levelIdx,
			Confidence: 99,
			Reasoning:  "Flat inventory grid rebuild: all entry orders were fully reseeded after the strategy returned to zero position.",
			ForceEntry: true,
		})
		if err != nil {
			logger.Warnf("%s Failed to reseed full entry grid at level %d ($%s): %v",
				at.gridLogPrefix(), entry.levelIdx, formatGridLogPrice(entry.price), err)
			if firstErr == nil && !strings.Contains(err.Error(), "first-entry guard") {
				firstErr = err
			}
			continue
		}
		placed++
	}

	logger.Infof("%s Seeded %d full-grid entry orders after flat reset using compounding base $%.2f", at.gridLogPrefix(), placed, investmentBase)
	return firstErr
}

func (at *AutoTrader) rebuildGridAfterFullExit(currentPrice float64) error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	if currentPrice <= 0 {
		price, err := at.trader.GetMarketPrice(gridConfig.Symbol)
		if err != nil {
			return fmt.Errorf("failed to get market price for flat grid rebuild: %w", err)
		}
		currentPrice = price
	}

	if err := at.cancelAllGridOrders(); err != nil {
		return err
	}

	at.gridState.mu.Lock()
	at.initializeGridLevels(currentPrice, gridConfig)
	at.gridState.OrderBook = make(map[string]int)
	at.gridState.mu.Unlock()

	at.bootstrapGridStateFromExchange()
	investmentBase := at.currentGridInvestmentBase()
	if err := at.seedFullEntryGrid(investmentBase); err != nil {
		return err
	}

	logger.Infof("[Grid] Rebuilt full entry grid after all positions returned to flat at $%.6f using live compounding base $%.2f", currentPrice, investmentBase)
	return nil
}

func (at *AutoTrader) ensurePairedExitOrderForFilledLevel(levelIdx int) {
	type exitPlan struct {
		exitLevelIdx int
		exitSide     string
		positionSide string
		positionSize float64
		entryPrice   float64
		exitPrice    float64
	}

	var plan exitPlan

	at.gridState.mu.RLock()
	if levelIdx < 0 || levelIdx >= len(at.gridState.Levels) {
		at.gridState.mu.RUnlock()
		return
	}

	sourceLevel := at.gridState.Levels[levelIdx]
	if sourceLevel.State != "filled" || sourceLevel.PositionSize <= 0 {
		at.gridState.mu.RUnlock()
		return
	}

	plan.positionSide = normalizeGridPositionSide(sourceLevel.PositionSide, sourceLevel.Side)
	if active, _ := at.aggregatedRiskExitState(plan.positionSide); active {
		at.gridState.mu.RUnlock()
		return
	}
	plan.positionSize = sourceLevel.PositionSize
	plan.entryPrice = sourceLevel.PositionEntry
	gridSpacing := at.gridState.GridSpacing
	upperPrice := at.gridState.UpperPrice
	lowerPrice := at.gridState.LowerPrice

	step := 0
	switch plan.positionSide {
	case "SHORT":
		step = -1
		plan.exitSide = "BUY"
	case "LONG":
		step = 1
		plan.exitSide = "SELL"
	default:
		at.gridState.mu.RUnlock()
		return
	}

	plan.exitLevelIdx = levelIdx + step
	if plan.exitLevelIdx < 0 || plan.exitLevelIdx >= len(at.gridState.Levels) {
		at.gridState.mu.RUnlock()
		fallbackPrice := 0.0
		switch plan.positionSide {
		case "LONG":
			fallbackPrice = upperPrice + gridSpacing
		case "SHORT":
			fallbackPrice = lowerPrice - gridSpacing
		}
		if fallbackPrice <= 0 {
			logger.Warnf("[Grid] Filled level %d (%s) has no available level for paired exit order", levelIdx, plan.positionSide)
			return
		}
		if _, err := at.ensureStandaloneReduceOnlyOrder(plan.exitSide, plan.positionSide, plan.positionSize, fallbackPrice); err != nil {
			logger.Warnf("[Grid] Failed to seed standalone paired exit for filled level %d (%s) at fallback price $%s: %v",
				levelIdx, plan.positionSide, formatGridLogPrice(fallbackPrice), err)
			return
		}
		logger.Infof("[Grid] Seeded standalone paired exit order for filled level %d (%s) at fallback price $%s",
			levelIdx, plan.positionSide, formatGridLogPrice(fallbackPrice))
		return
	}

	exitLevel := at.gridState.Levels[plan.exitLevelIdx]
	plan.exitPrice = exitLevel.Price
	at.gridState.mu.RUnlock()

	// Exit orders are anchored to the immediately adjacent grid level so each
	// filled entry keeps an explicit one-step closed loop. Reduce-only exits are
	// still managed independently from entry level occupancy, but they no longer
	// jump across multiple levels or aggregate unrelated filled levels into a
	// single far-away exit order.
	if _, err := at.ensureStandaloneReduceOnlyOrder(plan.exitSide, plan.positionSide, plan.positionSize, plan.exitPrice); err != nil {
		logger.Warnf("[Grid] Failed to seed paired adjacent exit for filled level %d (%s) at level %d, entry=$%s exit=$%s: %v",
			levelIdx, plan.positionSide, plan.exitLevelIdx, formatGridLogPrice(plan.entryPrice), formatGridLogPrice(plan.exitPrice), err)
		return
	}
	logger.Infof("[Grid] Seeded paired adjacent exit for filled level %d (%s) at level %d without occupying entry grid level",
		levelIdx, plan.positionSide, plan.exitLevelIdx)
	return
}

// checkTotalPositionLimit checks if adding a new position would exceed total limits
// Returns: (allowed bool, currentPositionValue float64, maxAllowed float64)
func (at *AutoTrader) checkTotalPositionLimit(symbol string, additionalValue float64) (bool, float64, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig

	// Calculate max allowed total position value
	// Total position should not exceed: current equity * leverage
	investmentBase := at.currentGridInvestmentBase()
	maxTotalPositionValue := investmentBase * float64(gridConfig.Leverage)

	// Get current position value from exchange
	currentPositionValue := 0.0
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == symbol {
				if size, ok := pos["positionAmt"].(float64); ok {
					if price, ok := pos["markPrice"].(float64); ok {
						currentPositionValue = math.Abs(size) * price
					} else if entryPrice, ok := pos["entryPrice"].(float64); ok {
						currentPositionValue = math.Abs(size) * entryPrice
					}
				}
			}
		}
	}

	// Also count pending orders as potential position
	at.gridState.mu.RLock()
	pendingValue := 0.0
	for _, level := range at.gridState.Levels {
		if gridLevelHasTrackedEntryOrder(level) {
			pendingValue += level.OrderQuantity * level.Price
		}
	}
	at.gridState.mu.RUnlock()

	totalAfterOrder := currentPositionValue + pendingValue + additionalValue
	allowed := totalAfterOrder <= maxTotalPositionValue

	return allowed, currentPositionValue + pendingValue, maxTotalPositionValue
}

// placeGridLimitOrder places a limit order for grid trading
func (at *AutoTrader) placeGridLimitOrder(d *kernel.Decision, side string) error {
	// Check if trader supports GridTrader interface
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		// Fallback to adapter
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	positionSide := ""
	requestedQuantity := d.Quantity
	reduceOnly := false
	linkedLevelIndex := -1
	if d.ForceEntry {
		switch side {
		case "BUY":
			positionSide = "LONG"
		case "SELL":
			positionSide = "SHORT"
		default:
			return fmt.Errorf("unsupported forced entry side %s", side)
		}
	} else {
		var err error
		positionSide, requestedQuantity, reduceOnly, linkedLevelIndex, err = at.resolveGridOrderIntent(d, side)
		if err != nil {
			return err
		}
	}
	if requestedQuantity <= 0 {
		return fmt.Errorf("grid %s order has non-positive quantity %.4f", side, requestedQuantity)
	}
	if !reduceOnly {
		if allowed, reason := at.evaluateGridEntryAdmission(d.Symbol, positionSide, d.Price); !allowed {
			return fmt.Errorf("grid entry admission rejected: %s", reason)
		}
	}

	// CRITICAL: Entry sizing must come from the grid's compounding-base model,
	// not from the AI payload. This keeps live order notional anchored to the
	// configured grid distribution even if the AI mirrors stale small orders.
	investmentBase := at.currentGridInvestmentBase()
	quantity := requestedQuantity
	if !reduceOnly && d.LevelIndex >= 0 && d.Price > 0 && investmentBase > 0 {
		systemQuantity := at.suggestedEntryQuantityWithInvestmentBase(d.LevelIndex, d.Price, investmentBase)
		if systemQuantity > 0 {
			if math.Abs(systemQuantity-requestedQuantity) > 1e-9 {
				logger.Infof("[Grid] Overriding AI entry quantity %.4f with system quantity %.4f at level %d ($%.8f, compounding_base $%.2f)",
					requestedQuantity, systemQuantity, d.LevelIndex, d.Price, investmentBase)
			}
			quantity = systemQuantity
		}
	}

	// CRITICAL: Validate and cap quantity to prevent excessive position sizes
	// This protects against AI miscalculations or leverage misconfigurations
	currentMarketPrice := 0.0
	if !reduceOnly {
		currentMarketPrice, _ = at.trader.GetMarketPrice(d.Symbol)
	}
	if d.Price > 0 && investmentBase > 0 {
		cappedQuantity, capMultiplier := at.capGridEntryQuantityByUniformLimit(d.LevelIndex, d.Price, quantity, investmentBase, currentMarketPrice)
		if quantity > cappedQuantity {
			uniformNotional := (investmentBase / float64(gridConfig.GridCount)) * float64(gridConfig.Leverage)
			logger.Warnf("[Grid] Quantity %.4f exceeds capped grid allowance %.4f (requested_notional $%.2f > uniform_cap $%.2f x %.1f, compounding_base $%.2f), capping",
				quantity, cappedQuantity, quantity*d.Price, uniformNotional, capMultiplier, investmentBase)
			quantity = cappedQuantity
		}

		// Safety check: ensure position value is reasonable (within 2x of the
		// live-equity-based limit as an absolute ceiling).
		positionValue := quantity * d.Price
		absoluteMaxValue := investmentBase * float64(gridConfig.Leverage) * 2 // 2x safety margin
		if positionValue > absoluteMaxValue {
			logger.Errorf("[Grid] CRITICAL: Position value $%.2f exceeds live-equity safety cap $%.2f (compounding_base $%.2f)! Rejecting order.",
				positionValue, absoluteMaxValue, investmentBase)
			return fmt.Errorf("position value $%.2f exceeds safety limit $%.2f", positionValue, absoluteMaxValue)
		}
	}

	placementLevelIdx := d.LevelIndex
	placementPrice := d.Price
	if !reduceOnly {
		altLevelIdx, altPrice, changed, err := at.findAlternativeEntryPlacement(d.Symbol, d.LevelIndex, side, positionSide, d.Price)
		if err != nil {
			return err
		}
		if changed {
			logger.Infof("[Grid] Redirected entry order from level %d price $%.8f to level %d price $%.8f to avoid same-price duplication",
				d.LevelIndex, d.Price, altLevelIdx, altPrice)
			placementLevelIdx = altLevelIdx
			placementPrice = altPrice
		}
		if ok, err := at.isEntryPriceSpacingValid(d.Symbol, side, positionSide, placementPrice); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("entry price $%.8f is too close to an existing %s %s order; minimum spacing is %.8f",
				placementPrice, side, positionSide, at.minimumEntrySpacing())
		}
	}

	if !reduceOnly && placementPrice > 0 && investmentBase > 0 {
		if placementLevelIdx >= 0 {
			systemQuantity := at.suggestedEntryQuantityWithInvestmentBase(placementLevelIdx, placementPrice, investmentBase)
			if systemQuantity > 0 {
				quantity = systemQuantity
			}
		}
		cappedQuantity, capMultiplier := at.capGridEntryQuantityByUniformLimit(placementLevelIdx, placementPrice, quantity, investmentBase, currentMarketPrice)
		if quantity > cappedQuantity {
			uniformNotional := (investmentBase / float64(gridConfig.GridCount)) * float64(gridConfig.Leverage)
			logger.Warnf("[Grid] Quantity %.4f exceeds final entry cap %.4f at price $%.8f (requested_notional $%.2f > uniform_cap $%.2f x %.1f, compounding_base $%.2f), capping",
				quantity, cappedQuantity, placementPrice, quantity*placementPrice, uniformNotional, capMultiplier, investmentBase)
			quantity = cappedQuantity
		}
	}

	// CRITICAL: Check total position limit before placing entry order
	orderValue := quantity * placementPrice
	if !reduceOnly {
		allowed, currentValue, maxValue := at.checkTotalPositionLimit(d.Symbol, orderValue)
		if !allowed {
			logger.Errorf("[Grid] TOTAL POSITION LIMIT EXCEEDED: current_exposure=$%.2f + order=$%.2f > dynamic_limit=$%.2f (compounding_base $%.2f). Rejecting order.",
				currentValue, orderValue, maxValue, investmentBase)
			return fmt.Errorf("total position value $%.2f would exceed limit $%.2f", currentValue+orderValue, maxValue)
		}
	}

	if !reduceOnly {
		if err := at.cancelDuplicateEntryOrdersAtPrice(d.Symbol, side, positionSide, placementPrice); err != nil {
			return err
		}
	}

	if gridConfig.UseMakerOnly {
		if currentMarketPrice <= 0 {
			price, err := at.trader.GetMarketPrice(d.Symbol)
			if err != nil {
				return fmt.Errorf("failed to get market price for maker-only validation: %w", err)
			}
			currentMarketPrice = price
		}
		if currentMarketPrice <= 0 {
			return fmt.Errorf("failed to get market price for maker-only validation")
		}
		crossesMarket := (side == "BUY" && placementPrice >= currentMarketPrice) || (side == "SELL" && placementPrice <= currentMarketPrice)
		if crossesMarket {
			intent := "entry"
			if reduceOnly {
				intent = "exit"
			}
			return fmt.Errorf("maker-only %s order would cross market: side=%s price=%.6f current=%.6f", intent, side, placementPrice, currentMarketPrice)
		}
	}

	req := &LimitOrderRequest{
		Symbol:       d.Symbol,
		Side:         side,
		PositionSide: positionSide,
		Price:        placementPrice,
		Quantity:     quantity, // Use validated/capped quantity
		Leverage:     gridConfig.Leverage,
		PostOnly:     gridConfig.UseMakerOnly,
		ReduceOnly:   reduceOnly,
		ClientID:     fmt.Sprintf("grid-%d-%d", placementLevelIdx, time.Now().UnixNano()%1000000),
	}

	result, err := gridTrader.PlaceLimitOrder(req)
	if err != nil {
		return fmt.Errorf("failed to place limit order: %w", err)
	}

	// Update grid level state
	at.gridState.mu.Lock()
	if placementLevelIdx >= 0 && placementLevelIdx < len(at.gridState.Levels) {
		level := &at.gridState.Levels[placementLevelIdx]
		if !gridLevelHasFilledPosition(*level) {
			level.State = "pending"
		}
		level.OrderID = result.OrderID
		level.OrderQuantity = quantity
		level.OrderPositionSide = positionSide
		level.OrderReduceOnly = reduceOnly
		level.LinkedLevelIndex = linkedLevelIndex
		at.gridState.OrderBook[result.OrderID] = placementLevelIdx
	}
	at.gridState.mu.Unlock()

	logger.Infof("%s Placed %s %s limit order at $%.2f, qty=%.4f, level=%d, reduceOnly=%t, orderID=%s",
		at.gridLogPrefix(), side, positionSide, placementPrice, quantity, placementLevelIdx, reduceOnly, result.OrderID)

	return nil
}

// cancelGridOrder cancels a specific grid order
func (at *AutoTrader) cancelGridOrder(d *kernel.Decision) error {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	resolvedOrderID, err := at.resolveCancelableGridOrderID(d)
	if err != nil {
		return err
	}

	symbol := strings.TrimSpace(d.Symbol)
	if symbol == "" && at.config.StrategyConfig != nil && at.config.StrategyConfig.GridConfig != nil {
		symbol = at.config.StrategyConfig.GridConfig.Symbol
	}

	if err := gridTrader.CancelOrder(symbol, resolvedOrderID); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	// Update state
	at.gridState.mu.Lock()
	if levelIdx, ok := at.gridState.OrderBook[resolvedOrderID]; ok {
		if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) {
			clearGridPendingOrder(&at.gridState.Levels[levelIdx])
		}
		delete(at.gridState.OrderBook, resolvedOrderID)
	}
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Cancelled order: requested=%q resolved=%s", d.OrderID, resolvedOrderID)
	return nil
}

func (at *AutoTrader) resolveCancelableGridOrderID(d *kernel.Decision) (string, error) {
	rawOrderID := strings.TrimSpace(d.OrderID)
	symbol := strings.TrimSpace(d.Symbol)
	if symbol == "" && at.config.StrategyConfig != nil && at.config.StrategyConfig.GridConfig != nil {
		symbol = at.config.StrategyConfig.GridConfig.Symbol
	}

	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return "", fmt.Errorf("failed to query live open orders for cancel resolution: %w", err)
	}

	// 1) Exact exchange order ID match from live open orders.
	if rawOrderID != "" {
		for _, order := range openOrders {
			if order.OrderID == rawOrderID {
				return rawOrderID, nil
			}
		}
	}

	// 2) Exact runtime-tracked order ID match.
	if rawOrderID != "" {
		at.gridState.mu.RLock()
		_, exists := at.gridState.OrderBook[rawOrderID]
		at.gridState.mu.RUnlock()
		if exists {
			return rawOrderID, nil
		}
	}

	// 3) Resolve from explicit level_index first.
	if d.LevelIndex >= 0 {
		if levelOrderID := at.resolveGridOrderIDFromLevelIndex(d.LevelIndex); levelOrderID != "" {
			return levelOrderID, nil
		}
	}

	// 4) Resolve synthetic labels like "L9_buy_limit_0.095040" to actual pending level order.
	if rawOrderID != "" {
		if matches := gridOrderLevelRefPattern.FindStringSubmatch(rawOrderID); len(matches) == 2 {
			levelNum, parseErr := strconv.Atoi(matches[1])
			if parseErr == nil && levelNum > 0 {
				if levelOrderID := at.resolveGridOrderIDFromLevelIndex(levelNum - 1); levelOrderID != "" {
					return levelOrderID, nil
				}
			}
		}
	}

	// 5) As a last safe fallback, resolve by a unique live order price.
	candidatePrice := d.Price
	if candidatePrice <= 0 && rawOrderID != "" {
		if matches := gridOrderPriceRefPattern.FindStringSubmatch(rawOrderID); len(matches) == 2 {
			if parsedPrice, parseErr := strconv.ParseFloat(matches[1], 64); parseErr == nil {
				candidatePrice = parsedPrice
			}
		}
	}
	if candidatePrice > 0 {
		targetPriceKey := at.normalizeGridPriceKey(symbol, candidatePrice)
		priceMatches := make([]OpenOrder, 0, len(openOrders))
		for _, order := range openOrders {
			if at.normalizeGridPriceKey(symbol, gridOrderPrice(order)) == targetPriceKey {
				priceMatches = append(priceMatches, order)
			}
		}

		if len(priceMatches) == 1 {
			return priceMatches[0].OrderID, nil
		}

		sideHint := inferCancelOrderSideHint(rawOrderID)
		if sideHint != "" && len(priceMatches) > 1 {
			sideMatches := make([]OpenOrder, 0, len(priceMatches))
			for _, order := range priceMatches {
				if strings.EqualFold(order.Side, sideHint) {
					sideMatches = append(sideMatches, order)
				}
			}
			if len(sideMatches) == 1 {
				return sideMatches[0].OrderID, nil
			}
			if len(sideMatches) > 1 {
				return "", fmt.Errorf("cancel target is ambiguous at price %.6f: %d live %s orders matched", candidatePrice, len(sideMatches), sideHint)
			}
		}

		if len(priceMatches) > 1 {
			return "", fmt.Errorf("cancel target is ambiguous at price %.6f: %d live orders matched; require level_index or exact exchange order ID", candidatePrice, len(priceMatches))
		}
	}

	return "", fmt.Errorf("unable to resolve a real exchange order ID for cancel_order (requested=%q level_index=%d price=%.6f)", d.OrderID, d.LevelIndex, d.Price)
}

func (at *AutoTrader) resolveGridOrderIDFromLevelIndex(levelIndex int) string {
	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()

	if levelIndex < 0 || levelIndex >= len(at.gridState.Levels) {
		return ""
	}

	level := at.gridState.Levels[levelIndex]
	if level.State != "pending" {
		return ""
	}
	return strings.TrimSpace(level.OrderID)
}

func inferCancelOrderSideHint(raw string) string {
	if raw == "" {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(normalized, "buy"),
		strings.Contains(raw, "做多挂单"),
		strings.Contains(raw, "空头平仓"):
		return "BUY"
	case strings.Contains(normalized, "sell"),
		strings.Contains(raw, "做空挂单"),
		strings.Contains(raw, "多头平仓"):
		return "SELL"
	default:
		return ""
	}
}

// cancelAllGridOrders cancels all grid orders
func (at *AutoTrader) cancelAllGridOrders() error {
	gridConfig := at.config.StrategyConfig.GridConfig

	if err := at.trader.CancelAllOrders(gridConfig.Symbol); err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	// Reset all pending levels
	at.gridState.mu.Lock()
	for i := range at.gridState.Levels {
		if gridLevelHasTrackedEntryOrder(at.gridState.Levels[i]) {
			clearGridPendingOrder(&at.gridState.Levels[i])
		}
	}
	at.gridState.OrderBook = make(map[string]int)
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Cancelled all orders")
	return nil
}

// cancelGridOrdersForRiskPause cancels entry orders while preserving or rebuilding
// reduce-only exit orders for already-filled grid positions.
func (at *AutoTrader) cancelGridOrdersForRiskPause(reason string) error {
	if err := at.cancelGridEntryOrders(reason); err != nil {
		return err
	}

	at.ensurePairedExitOrdersForAllFilledLevels()
	logger.Infof("%s Cancelled entry orders and preserved exit coverage for %s", at.gridLogPrefix(), reason)
	return nil
}

// pauseGrid pauses grid trading
func (at *AutoTrader) pauseGrid(reason string) error {
	if err := at.cancelGridOrdersForRiskPause("pause_grid"); err != nil {
		logger.Warnf("[Grid] Failed to cancel entry orders while pausing grid: %v", err)
	}

	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.noteGridPaused(reason)
	at.gridState.mu.Unlock()

	logger.Infof("%s Paused: %s", at.gridLogPrefix(), reason)
	return nil
}

// resumeGrid resumes grid trading
func (at *AutoTrader) resumeGrid() error {
	at.gridState.mu.Lock()
	at.gridState.IsPaused = false
	at.gridState.PauseReason = ""
	at.gridState.ResumeConfirmCount = 0
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Resumed")
	return nil
}

// adjustGrid adjusts grid parameters
func (at *AutoTrader) adjustGrid(d *kernel.Decision) error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	// Preserve existing position exits while dropping stale entry orders before
	// rebuilding the grid around the new price regime.
	if err := at.cancelGridEntryOrders("adjust_grid"); err != nil {
		logger.Warnf("[Grid] Failed to cancel entry orders during adjust: %v", err)
	}

	// Get current price
	price, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	// Reinitialize grid levels
	at.gridState.mu.Lock()
	at.initializeGridLevels(price, gridConfig)
	at.gridState.OrderBook = make(map[string]int)
	at.gridState.mu.Unlock()
	at.bootstrapGridStateFromExchange()
	at.ensurePairedExitOrdersForAllFilledLevels()

	logger.Infof("[Grid] Adjusted grid bounds around price $%.2f", price)
	return nil
}

// syncGridState syncs grid state with exchange
func (at *AutoTrader) syncGridState() {
	gridConfig := at.config.StrategyConfig.GridConfig

	// Get open orders from exchange
	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to get open orders: %v", err)
		return
	}
	openOrders = at.enforceFirstEntryGuardOnOpenOrders(gridConfig.Symbol, openOrders)

	// Build set of active order IDs
	activeOrderIDs := make(map[string]bool)
	for _, order := range openOrders {
		activeOrderIDs[order.OrderID] = true
	}

	at.syncLiveOpenOrdersSnapshot(gridConfig.Symbol, openOrders)
	at.rebuildStandaloneReduceOnlyBook(gridConfig.Symbol, openOrders)
	at.refreshInventoryLotExitBindings(gridConfig.Symbol, openOrders)
	at.reconcileGridEntryOrderSizing(gridConfig.Symbol, openOrders)

	// Get current positions to verify fills
	positions, err := at.trader.GetPositions()
	currentPositionBySide := map[string]float64{
		"LONG":  0,
		"SHORT": 0,
	}
	if err != nil {
		logger.Warnf("[Grid] Failed to get positions for state sync: %v", err)
	} else {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == gridConfig.Symbol {
				if size, ok := pos["positionAmt"].(float64); ok {
					side, _ := pos["side"].(string)
					normalizedSide := normalizeGridPositionSide(side, "")
					currentPositionBySide[normalizedSide] = math.Abs(size)
				}
			}
		}
		at.reconcileGridInventoryLotsWithExchangePositions(gridConfig.Symbol, positions)
		if at.store != nil {
			if err := SyncPositionSnapshotFromMaps(at.id, at.exchangeID, at.exchange, positions, at.store); err != nil {
				logger.Warnf("[Grid] Failed to sync live position snapshot for %s: %v", gridConfig.Symbol, err)
			}
		}
	}

	// Update levels based on order status
	at.gridState.mu.Lock()
	expectedPositionBySide := map[string]float64{
		"LONG":  0,
		"SHORT": 0,
	}
	filledLevelsNeedingExitSync := make([]int, 0)
	hadTrackedFilledPosition := false
	type openedLot struct {
		levelIdx     int
		positionSide string
		quantity     float64
		entryPrice   float64
		entryOrderID string
	}
	type closedLot struct {
		sourceLevelIdx int
		closedQty      float64
		exitLevelIdx   int
		exitOrderID    string
	}
	newLots := make([]openedLot, 0)
	closedLots := make([]closedLot, 0)
	for _, level := range at.gridState.Levels {
		if level.State == "filled" {
			hadTrackedFilledPosition = true
			positionSide := normalizeGridPositionSide(level.PositionSide, level.Side)
			expectedPositionBySide[positionSide] += level.PositionSize
			filledLevelsNeedingExitSync = append(filledLevelsNeedingExitSync, level.Index)
		}
	}

	for i := range at.gridState.Levels {
		level := &at.gridState.Levels[i]
		if gridLevelHasTrackedEntryOrder(*level) {
			if !activeOrderIDs[level.OrderID] {
				orderID := level.OrderID
				orderPositionSide := normalizeGridPositionSide(level.OrderPositionSide, level.Side)
				if level.OrderReduceOnly {
					linkedIdx := level.LinkedLevelIndex
					if linkedIdx >= 0 && linkedIdx < len(at.gridState.Levels) {
						sourceLevel := &at.gridState.Levels[linkedIdx]
						if sourceLevel.State == "filled" {
							closedQty := math.Min(level.OrderQuantity, sourceLevel.PositionSize)
							expectedPositionBySide[orderPositionSide] -= closedQty
							if expectedPositionBySide[orderPositionSide] < 0 {
								expectedPositionBySide[orderPositionSide] = 0
							}
							closedLots = append(closedLots, closedLot{
								sourceLevelIdx: linkedIdx,
								closedQty:      closedQty,
								exitLevelIdx:   i,
								exitOrderID:    orderID,
							})
						}
					}
					clearGridPendingOrder(level)
					at.gridState.TotalTrades++
					logger.Infof("[Grid] Level %d reduce-only order filled at $%.2f (%s)", i, level.Price, orderPositionSide)
				} else {
					if currentPositionBySide[orderPositionSide] > expectedPositionBySide[orderPositionSide] {
						level.State = "filled"
						level.PositionEntry = level.Price
						level.PositionSize = level.OrderQuantity
						level.PositionSide = orderPositionSide
						level.OrderID = ""
						level.OrderPositionSide = ""
						level.OrderReduceOnly = false
						level.LinkedLevelIndex = 0
						at.gridState.TotalTrades++
						expectedPositionBySide[orderPositionSide] += level.PositionSize
						newLots = append(newLots, openedLot{
							levelIdx:     i,
							positionSide: orderPositionSide,
							quantity:     level.PositionSize,
							entryPrice:   level.PositionEntry,
							entryOrderID: orderID,
						})
						logger.Infof("[Grid] Level %d entry order filled at $%.2f (%s)", i, level.Price, orderPositionSide)
					} else {
						clearGridPendingOrder(level)
						logger.Infof("[Grid] Level %d order cancelled/expired", i)
					}
				}
				delete(at.gridState.OrderBook, orderID)
			}
		}
	}
	at.gridState.mu.Unlock()

	for _, lot := range newLots {
		if err := at.upsertGridInventoryLot(lot.levelIdx, lot.positionSide, lot.quantity, lot.entryPrice, lot.entryOrderID); err != nil {
			logger.Warnf("[Grid] Failed to persist opened grid lot for level %d: %v", lot.levelIdx, err)
		}
	}

	for _, lot := range closedLots {
		at.applyGridExitToInventory(lot.sourceLevelIdx, lot.closedQty, lot.exitLevelIdx, lot.exitOrderID)
	}

	if len(filledLevelsNeedingExitSync) > 0 {
		// Always rebuild exits from the persistent inventory ledger after sync.
		// Directly seeding from in-memory filled levels can resurrect stale
		// remapped levels after a grid rebuild and place exits at the wrong
		// adjacent price.
		if !at.ensurePairedExitOrdersForInventoryLots() {
			at.ensurePairedExitOrdersForLivePositions()
		}
	}

	fullyFlatNow := currentPositionBySide["LONG"] <= gridLotQtyTolerance && currentPositionBySide["SHORT"] <= gridLotQtyTolerance
	if fullyFlatNow && (len(closedLots) > 0 || hadTrackedFilledPosition) {
		currentPrice, priceErr := at.trader.GetMarketPrice(gridConfig.Symbol)
		if priceErr != nil {
			logger.Warnf("%s Failed to fetch market price for flat-grid rebuild: %v", at.gridLogPrefix(), priceErr)
		}
		if err := at.rebuildGridAfterFullExit(currentPrice); err != nil {
			logger.Warnf("%s Failed to rebuild full entry grid after flat exit: %v", at.gridLogPrefix(), err)
		} else {
			return
		}
	}

	if len(closedLots) > 0 {
		if ctx, err := at.buildGridContext(); err != nil {
			logger.Warnf("%s Failed to build context for immediate entry refill after exit fills: %v", at.gridLogPrefix(), err)
		} else if err := at.autoSeedMissingEntryOrdersForRangingGrid(ctx); err != nil {
			logger.Warnf("%s Failed to immediately refill released entry levels after exit fills: %v", at.gridLogPrefix(), err)
		}
	}

	logger.Debugf("[Grid] Synced state: long=%.4f short=%.4f, orders=%d", currentPositionBySide["LONG"], currentPositionBySide["SHORT"], len(openOrders))

	// Check stop loss
	at.checkAndExecuteStopLoss()

	// Check grid skew
	at.autoAdjustGrid()
}

// closeAllPositions closes all open positions for the grid symbol
func (at *AutoTrader) closeAllPositions() error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		if symbol != gridConfig.Symbol {
			continue
		}

		size, _ := pos["positionAmt"].(float64)
		if size == 0 {
			continue
		}

		if size > 0 {
			_, err = at.trader.CloseLong(symbol, size)
		} else {
			_, err = at.trader.CloseShort(symbol, -size)
		}
		if err != nil {
			logger.Infof("Failed to close position: %v", err)
		}
	}

	return nil
}

// checkAndExecuteStopLoss evaluates the grid risk state machine and executes
// staged reductions when the side-level risk has escalated.
func (at *AutoTrader) checkAndExecuteStopLoss() {
	at.evaluateAndExecuteGridStopLoss()
}
