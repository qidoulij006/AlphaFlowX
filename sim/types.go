package sim

import "time"

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type PositionSide string

const (
	PositionLong  PositionSide = "LONG"
	PositionShort PositionSide = "SHORT"
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
)

type Liquidity string

const (
	LiquidityMaker Liquidity = "MAKER"
	LiquidityTaker Liquidity = "TAKER"
)

type AggTradeEvent struct {
	Type       string  `json:"type"`
	Symbol     string  `json:"symbol"`
	Time       int64   `json:"time"`
	AggID      int64   `json:"agg_id"`
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	TakerSide  Side    `json:"taker_side"`
	BuyerMaker bool    `json:"buyer_maker"`
}

func (e AggTradeEvent) At() time.Time {
	return time.UnixMilli(e.Time).UTC()
}

type OrderIntent struct {
	Time         int64        `json:"time"`
	Action       string       `json:"action"`
	OrderID      string       `json:"order_id,omitempty"`
	ClientID     string       `json:"client_id,omitempty"`
	Symbol       string       `json:"symbol"`
	Type         OrderType    `json:"type"`
	Side         Side         `json:"side"`
	PositionSide PositionSide `json:"position_side"`
	Price        float64      `json:"price,omitempty"`
	Quantity     float64      `json:"quantity,omitempty"`
	ReduceOnly   bool         `json:"reduce_only,omitempty"`
	PostOnly     bool         `json:"post_only,omitempty"`
}

type Order struct {
	ID           string       `json:"id"`
	ClientID     string       `json:"client_id,omitempty"`
	Symbol       string       `json:"symbol"`
	Type         OrderType    `json:"type"`
	Side         Side         `json:"side"`
	PositionSide PositionSide `json:"position_side"`
	Price        float64      `json:"price"`
	Quantity     float64      `json:"quantity"`
	Filled       float64      `json:"filled"`
	ReduceOnly   bool         `json:"reduce_only"`
	PostOnly     bool         `json:"post_only"`
	Status       OrderStatus  `json:"status"`
	CreatedAt    int64        `json:"created_at"`
	UpdatedAt    int64        `json:"updated_at"`
}

func (o Order) Remaining() float64 {
	remaining := o.Quantity - o.Filled
	if remaining < 0 {
		return 0
	}
	return remaining
}

type Fill struct {
	OrderID      string       `json:"order_id"`
	ClientID     string       `json:"client_id,omitempty"`
	Symbol       string       `json:"symbol"`
	Side         Side         `json:"side"`
	PositionSide PositionSide `json:"position_side"`
	Price        float64      `json:"price"`
	Quantity     float64      `json:"quantity"`
	Fee          float64      `json:"fee"`
	RealizedPnL  float64      `json:"realized_pnl"`
	Liquidity    Liquidity    `json:"liquidity"`
	Time         int64        `json:"time"`
	Source       string       `json:"source"`
}

type Position struct {
	Side        PositionSide `json:"side"`
	Quantity    float64      `json:"quantity"`
	AvgPrice    float64      `json:"avg_price"`
	RealizedPnL float64      `json:"realized_pnl"`
	Fees        float64      `json:"fees"`
}

type AccountSnapshot struct {
	Time          int64      `json:"time"`
	Balance       float64    `json:"balance"`
	Equity        float64    `json:"equity"`
	UnrealizedPnL float64    `json:"unrealized_pnl"`
	LastPrice     float64    `json:"last_price"`
	Positions     []Position `json:"positions"`
}

type Report struct {
	Symbol         string            `json:"symbol"`
	StartedAt      int64             `json:"started_at"`
	EndedAt        int64             `json:"ended_at"`
	InitialBalance float64           `json:"initial_balance"`
	Balance        float64           `json:"balance"`
	Equity         float64           `json:"equity"`
	RealizedPnL    float64           `json:"realized_pnl"`
	Fees           float64           `json:"fees"`
	UnrealizedPnL  float64           `json:"unrealized_pnl"`
	FillCount      int               `json:"fill_count"`
	OpenOrders     []Order           `json:"open_orders"`
	Positions      []Position        `json:"positions"`
	Fills          []Fill            `json:"fills,omitempty"`
	Snapshots      []AccountSnapshot `json:"snapshots,omitempty"`
}
