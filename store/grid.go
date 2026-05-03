package store

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GridConfigModel struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"index"`
	TraderID  string    `json:"trader_id" gorm:"index"`
	Symbol    string    `json:"symbol" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	GridCount       int     `json:"grid_count" gorm:"default:10"`
	TotalInvestment float64 `json:"total_investment" gorm:"not null"`
	Leverage        int     `json:"leverage" gorm:"default:5"`
	UpperPrice      float64 `json:"upper_price"`
	LowerPrice      float64 `json:"lower_price"`
	UseATRBounds    bool    `json:"use_atr_bounds" gorm:"default:true"`
	ATRMultiplier   float64 `json:"atr_multiplier" gorm:"default:2.0"`
	Distribution    string  `json:"distribution" gorm:"default:gaussian"`

	MaxDrawdownPct     float64 `json:"max_drawdown_pct" gorm:"default:15.0"`
	StopLossPct        float64 `json:"stop_loss_pct" gorm:"default:5.0"`
	DailyLossLimitPct  float64 `json:"daily_loss_limit_pct" gorm:"default:10"`
	MaxPositionSizePct float64 `json:"max_position_size_pct" gorm:"default:30"`

	RegimeCheckInterval  int  `json:"regime_check_interval" gorm:"default:30"`
	AutoPauseOnTrend     bool `json:"auto_pause_on_trend" gorm:"default:true"`
	MinRangingScore      int  `json:"min_ranging_score" gorm:"default:60"`
	TrendResumeThreshold int  `json:"trend_resume_threshold" gorm:"default:70"`

	ShortBoxPeriod int `json:"short_box_period" gorm:"default:72"`
	MidBoxPeriod   int `json:"mid_box_period" gorm:"default:240"`
	LongBoxPeriod  int `json:"long_box_period" gorm:"default:500"`

	NarrowRegimeLeverage   int `json:"narrow_regime_leverage" gorm:"default:2"`
	StandardRegimeLeverage int `json:"standard_regime_leverage" gorm:"default:4"`
	WideRegimeLeverage     int `json:"wide_regime_leverage" gorm:"default:3"`
	VolatileRegimeLeverage int `json:"volatile_regime_leverage" gorm:"default:2"`

	NarrowRegimePositionPct   float64 `json:"narrow_regime_position_pct" gorm:"default:40"`
	StandardRegimePositionPct float64 `json:"standard_regime_position_pct" gorm:"default:70"`
	WideRegimePositionPct     float64 `json:"wide_regime_position_pct" gorm:"default:60"`
	VolatileRegimePositionPct float64 `json:"volatile_regime_position_pct" gorm:"default:40"`

	OrderRefreshSec  int     `json:"order_refresh_sec" gorm:"default:300"`
	UseMakerOnly     bool    `json:"use_maker_only" gorm:"default:true"`
	SlippageTolerPct float64 `json:"slippage_toler_pct" gorm:"default:0.1"`

	AIProvider string `json:"ai_provider" gorm:"default:deepseek"`
	AIModel    string `json:"ai_model" gorm:"default:deepseek-chat"`
	IsActive   bool   `json:"is_active" gorm:"default:false"`

	EnableDirectionAdjust bool    `json:"enable_direction_adjust" gorm:"default:false"`
	DirectionBiasRatio    float64 `json:"direction_bias_ratio" gorm:"default:0.7"`
}

func (GridConfigModel) TableName() string { return "grid_configs" }

type GridInstanceModel struct {
	ID        string     `json:"id" gorm:"primaryKey"`
	ConfigID  string     `json:"config_id" gorm:"index;not null"`
	Symbol    string     `json:"symbol" gorm:"not null"`
	State     string     `json:"state" gorm:"not null"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	CurrentUpperPrice   float64   `json:"current_upper_price"`
	CurrentLowerPrice   float64   `json:"current_lower_price"`
	CurrentGridSpacing  float64   `json:"current_grid_spacing"`
	ActiveLevelCount    int       `json:"active_level_count"`
	CurrentRegime       string    `json:"current_regime"`
	RegimeScore         int       `json:"regime_score"`
	LastRegimeCheck     time.Time `json:"last_regime_check"`
	ConsecutiveTrending int       `json:"consecutive_trending"`

	CurrentRegimeLevel string `json:"current_regime_level" gorm:"default:standard"`

	ShortBoxUpper float64 `json:"short_box_upper"`
	ShortBoxLower float64 `json:"short_box_lower"`
	MidBoxUpper   float64 `json:"mid_box_upper"`
	MidBoxLower   float64 `json:"mid_box_lower"`
	LongBoxUpper  float64 `json:"long_box_upper"`
	LongBoxLower  float64 `json:"long_box_lower"`

	BreakoutLevel        string    `json:"breakout_level" gorm:"default:none"`
	BreakoutDirection    string    `json:"breakout_direction"`
	BreakoutConfirmCount int       `json:"breakout_confirm_count" gorm:"default:0"`
	BreakoutStartTime    time.Time `json:"breakout_start_time"`

	PositionReductionPct float64 `json:"position_reduction_pct" gorm:"default:0"`

	CurrentDirection     string    `json:"current_direction" gorm:"default:neutral"`
	DirectionChangedAt   time.Time `json:"direction_changed_at"`
	DirectionChangeCount int       `json:"direction_change_count" gorm:"default:0"`

	TotalProfit     float64   `json:"total_profit" gorm:"default:0"`
	TotalFees       float64   `json:"total_fees" gorm:"default:0"`
	TotalTrades     int       `json:"total_trades" gorm:"default:0"`
	WinningTrades   int       `json:"winning_trades" gorm:"default:0"`
	MaxDrawdown     float64   `json:"max_drawdown" gorm:"default:0"`
	CurrentDrawdown float64   `json:"current_drawdown" gorm:"default:0"`
	PeakEquity      float64   `json:"peak_equity" gorm:"default:0"`
	DailyProfit     float64   `json:"daily_profit" gorm:"default:0"`
	DailyLoss       float64   `json:"daily_loss" gorm:"default:0"`
	LastDailyReset  time.Time `json:"last_daily_reset"`
}

func (GridInstanceModel) TableName() string { return "grid_instances" }

type GridLevelModel struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	InstanceID       string     `json:"instance_id" gorm:"index;not null"`
	LevelIndex       int        `json:"level_index" gorm:"not null"`
	Price            float64    `json:"price" gorm:"not null"`
	State            string     `json:"state" gorm:"not null"`
	Side             string     `json:"side"`
	OrderID          string     `json:"order_id,omitempty"`
	OrderPrice       float64    `json:"order_price,omitempty"`
	OrderQuantity    float64    `json:"order_quantity,omitempty"`
	OrderCreatedAt   *time.Time `json:"order_created_at,omitempty"`
	PositionSize     float64    `json:"position_size,omitempty"`
	PositionEntry    float64    `json:"position_entry,omitempty"`
	PositionOpenAt   *time.Time `json:"position_open_at,omitempty"`
	AllocationWeight float64    `json:"allocation_weight"`
	AllocatedUSD     float64    `json:"allocated_usd"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GridLevelModel) TableName() string { return "grid_levels" }

type GridEventModel struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	InstanceID  string    `json:"instance_id" gorm:"index;not null"`
	LevelID     string    `json:"level_id,omitempty" gorm:"index"`
	EventType   string    `json:"event_type" gorm:"not null"`
	EventTime   time.Time `json:"event_time" gorm:"autoCreateTime"`
	Price       float64   `json:"price,omitempty"`
	Quantity    float64   `json:"quantity,omitempty"`
	Side        string    `json:"side,omitempty"`
	PnL         float64   `json:"pnl,omitempty" gorm:"column:pn_l"`
	Fee         float64   `json:"fee,omitempty"`
	Message     string    `json:"message,omitempty"`
	OldRegime   string    `json:"old_regime,omitempty"`
	NewRegime   string    `json:"new_regime,omitempty"`
	TriggerType string    `json:"trigger_type,omitempty"`
	RawData     string    `json:"raw_data,omitempty" gorm:"type:text"`
}

func (GridEventModel) TableName() string { return "grid_events" }

type GridRiskEventModel struct {
	ID                  string    `json:"id" gorm:"primaryKey"`
	TraderID            string    `json:"trader_id" gorm:"index;not null"`
	Symbol              string    `json:"symbol" gorm:"index;not null"`
	RiskState           string    `json:"risk_state" gorm:"index;not null"`
	PreviousRiskState   string    `json:"previous_risk_state"`
	PositionSide        string    `json:"position_side" gorm:"index"`
	EventType           string    `json:"event_type" gorm:"index;not null"`
	Reason              string    `json:"reason" gorm:"type:text"`
	TriggerPrice        float64   `json:"trigger_price"`
	ExecutionPrice      float64   `json:"execution_price"`
	EntryPrice          float64   `json:"entry_price"`
	PositionQty         float64   `json:"position_qty"`
	ReducedQty          float64   `json:"reduced_qty"`
	RemainingQty        float64   `json:"remaining_qty"`
	PositionNotional    float64   `json:"position_notional"`
	ReducedNotional     float64   `json:"reduced_notional"`
	UnrealizedLoss      float64   `json:"unrealized_loss"`
	UnrealizedLossPct   float64   `json:"unrealized_loss_pct"`
	UnrealizedLossEqPct float64   `json:"unrealized_loss_equity_pct"`
	PositionPercent     float64   `json:"position_percent"`
	EffectiveLeverage   float64   `json:"effective_leverage"`
	LiquidationPrice    float64   `json:"liquidation_price"`
	LiquidationDistance float64   `json:"liquidation_distance"`
	ShortBoxUpper       float64   `json:"short_box_upper"`
	ShortBoxLower       float64   `json:"short_box_lower"`
	GridUpperPrice      float64   `json:"grid_upper_price"`
	GridLowerPrice      float64   `json:"grid_lower_price"`
	GridSpacing         float64   `json:"grid_spacing"`
	Metadata            string    `json:"metadata" gorm:"type:text"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

func (GridRiskEventModel) TableName() string { return "grid_risk_events" }

type GridRiskEvent struct {
	ID                  string                 `json:"id"`
	TraderID            string                 `json:"trader_id"`
	Symbol              string                 `json:"symbol"`
	RiskState           string                 `json:"risk_state"`
	PreviousRiskState   string                 `json:"previous_risk_state"`
	PositionSide        string                 `json:"position_side"`
	EventType           string                 `json:"event_type"`
	Reason              string                 `json:"reason"`
	TriggerPrice        float64                `json:"trigger_price"`
	ExecutionPrice      float64                `json:"execution_price"`
	EntryPrice          float64                `json:"entry_price"`
	PositionQty         float64                `json:"position_qty"`
	ReducedQty          float64                `json:"reduced_qty"`
	RemainingQty        float64                `json:"remaining_qty"`
	PositionNotional    float64                `json:"position_notional"`
	ReducedNotional     float64                `json:"reduced_notional"`
	UnrealizedLoss      float64                `json:"unrealized_loss"`
	UnrealizedLossPct   float64                `json:"unrealized_loss_pct"`
	UnrealizedLossEqPct float64                `json:"unrealized_loss_equity_pct"`
	PositionPercent     float64                `json:"position_percent"`
	EffectiveLeverage   float64                `json:"effective_leverage"`
	LiquidationPrice    float64                `json:"liquidation_price"`
	LiquidationDistance float64                `json:"liquidation_distance"`
	ShortBoxUpper       float64                `json:"short_box_upper"`
	ShortBoxLower       float64                `json:"short_box_lower"`
	GridUpperPrice      float64                `json:"grid_upper_price"`
	GridLowerPrice      float64                `json:"grid_lower_price"`
	GridSpacing         float64                `json:"grid_spacing"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
}

type GridRegimeAssessmentModel struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	InstanceID      string    `json:"instance_id" gorm:"index;not null"`
	AssessedAt      time.Time `json:"assessed_at" gorm:"autoCreateTime"`
	Regime          string    `json:"regime" gorm:"not null"`
	Score           int       `json:"score" gorm:"not null"`
	Confidence      float64   `json:"confidence"`
	BollingerSignal int       `json:"bollinger_signal"`
	EMASignal       int       `json:"ema_signal"`
	MACDSignal      int       `json:"macd_signal"`
	VolumeSignal    int       `json:"volume_signal"`
	OISignal        int       `json:"oi_signal"`
	FundingSignal   int       `json:"funding_signal"`
	CandleSignal    int       `json:"candle_signal"`
	ATR14           float64   `json:"atr14"`
	BollingerWidth  float64   `json:"bollinger_width"`
	EMADistance     float64   `json:"ema_distance"`
	CurrentPrice    float64   `json:"current_price"`
	AIReasoning     string    `json:"ai_reasoning" gorm:"type:text"`
}

func (GridRegimeAssessmentModel) TableName() string { return "grid_regime_assessments" }

type GridInventoryLotModel struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	TraderID         string     `json:"trader_id" gorm:"index;not null"`
	Symbol           string     `json:"symbol" gorm:"index;not null"`
	PositionSide     string     `json:"position_side" gorm:"index;not null"`
	SourceLevelIndex int        `json:"source_level_index" gorm:"index;not null"`
	ExitLevelIndex   int        `json:"exit_level_index" gorm:"default:-1"`
	EntryPrice       float64    `json:"entry_price" gorm:"not null"`
	EntryQuantity    float64    `json:"entry_quantity" gorm:"not null"`
	RemainingQty     float64    `json:"remaining_qty" gorm:"not null"`
	EntryOrderID     string     `json:"entry_order_id" gorm:"default:''"`
	ExitOrderID      string     `json:"exit_order_id" gorm:"default:''"`
	Status           string     `json:"status" gorm:"index;not null;default:OPEN"`
	OpenedAt         time.Time  `json:"opened_at" gorm:"index"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GridInventoryLotModel) TableName() string { return "grid_inventory_lots" }

type GridInventoryPairView struct {
	ID               string  `json:"id"`
	Symbol           string  `json:"symbol"`
	PositionSide     string  `json:"position_side"`
	SourceLevelIndex int     `json:"source_level_index"`
	ExitLevelIndex   int     `json:"exit_level_index"`
	EntryPrice       float64 `json:"entry_price"`
	ExitOrderPrice   float64 `json:"exit_order_price"`
	EntryValue       float64 `json:"entry_value"`
	ExitPrice        float64 `json:"exit_price"`
	ExitValue        float64 `json:"exit_value"`
	RealizedPnL      float64 `json:"realized_pnl"`
	Fee              float64 `json:"fee"`
	EntryTime        int64   `json:"entry_time"`
	ExitTime         int64   `json:"exit_time"`
	HoldDurationMs   int64   `json:"hold_duration_ms"`
	Status           string  `json:"status"`
	EntryOrderID     string  `json:"entry_order_id"`
	ExitOrderID      string  `json:"exit_order_id"`
	RemainingQty     float64 `json:"remaining_qty"`
	EntryQuantity    float64 `json:"entry_quantity"`
}

type GridStore struct{ db *gorm.DB }

func NewGridStore(db *gorm.DB) *GridStore { return &GridStore{db: db} }

func (s *GridStore) InitTables() error {
	if err := s.db.AutoMigrate(
		&GridConfigModel{},
		&GridInstanceModel{},
		&GridLevelModel{},
		&GridEventModel{},
		&GridRiskEventModel{},
		&GridRegimeAssessmentModel{},
		&GridInventoryLotModel{},
	); err != nil {
		return fmt.Errorf("failed to migrate grid tables: %w", err)
	}
	return nil
}

func (s *GridStore) SaveInventoryLot(lot *GridInventoryLotModel) error {
	if lot.ID == "" {
		lot.ID = uuid.NewString()
	}
	now := time.Now()
	if lot.OpenedAt.IsZero() {
		lot.OpenedAt = now
	}
	lot.UpdatedAt = now
	return s.db.Save(lot).Error
}

func (s *GridStore) LoadOpenInventoryLots(traderID, symbol string) ([]GridInventoryLotModel, error) {
	var lots []GridInventoryLotModel
	err := s.db.Where("trader_id = ? AND symbol = ? AND status IN ?", traderID, symbol, []string{"OPEN", "PARTIAL"}).Order("opened_at ASC, source_level_index ASC").Find(&lots).Error
	return lots, err
}

func (s *GridStore) FindOpenInventoryLot(traderID, symbol, positionSide string, sourceLevelIndex int) (*GridInventoryLotModel, error) {
	var lot GridInventoryLotModel
	err := s.db.Where("trader_id = ? AND symbol = ? AND position_side = ? AND source_level_index = ? AND status IN ?", traderID, symbol, positionSide, sourceLevelIndex, []string{"OPEN", "PARTIAL"}).Order("opened_at ASC").First(&lot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &lot, nil
}

func (s *GridStore) FindOpenInventoryLotByEntryOrder(traderID, symbol, positionSide, entryOrderID string) (*GridInventoryLotModel, error) {
	if entryOrderID == "" {
		return nil, nil
	}

	var lot GridInventoryLotModel
	err := s.db.Where("trader_id = ? AND symbol = ? AND position_side = ? AND entry_order_id = ? AND status IN ?", traderID, symbol, positionSide, entryOrderID, []string{"OPEN", "PARTIAL"}).Order("opened_at ASC").First(&lot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &lot, nil
}

func (s *GridStore) FindSyntheticOpenInventoryLot(traderID, symbol, positionSide string, sourceLevelIndex int) (*GridInventoryLotModel, error) {
	var lot GridInventoryLotModel
	err := s.db.Where("trader_id = ? AND symbol = ? AND position_side = ? AND source_level_index = ? AND entry_order_id = '' AND status IN ?", traderID, symbol, positionSide, sourceLevelIndex, []string{"OPEN", "PARTIAL"}).Order("opened_at ASC").First(&lot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &lot, nil
}

func (s *GridStore) UpdateInventoryLotExitIntent(id string, exitLevelIndex int, exitOrderID string) error {
	updates := map[string]interface{}{
		"exit_level_index": exitLevelIndex,
		"exit_order_id":    exitOrderID,
		"updated_at":       time.Now(),
	}
	return s.db.Model(&GridInventoryLotModel{}).Where("id = ?", id).Updates(updates).Error
}

func (s *GridStore) UpdateOpenInventoryLotsExitIntentBySide(traderID, symbol, positionSide string, exitLevelIndex int, exitOrderID string) error {
	updates := map[string]interface{}{
		"exit_level_index": exitLevelIndex,
		"exit_order_id":    exitOrderID,
		"updated_at":       time.Now(),
	}
	return s.db.Model(&GridInventoryLotModel{}).
		Where("trader_id = ? AND symbol = ? AND position_side = ? AND status IN ?", traderID, symbol, positionSide, []string{"OPEN", "PARTIAL"}).
		Updates(updates).Error
}

func (s *GridStore) UpdateInventoryLotAfterClose(id string, remainingQty float64, exitLevelIndex int, exitOrderID string) error {
	updates := map[string]interface{}{
		"remaining_qty":    remainingQty,
		"exit_level_index": exitLevelIndex,
		"exit_order_id":    exitOrderID,
		"updated_at":       time.Now(),
	}
	if remainingQty <= 0 {
		now := time.Now()
		updates["status"] = "CLOSED"
		updates["closed_at"] = &now
		updates["remaining_qty"] = 0
	} else {
		updates["status"] = "PARTIAL"
	}
	return s.db.Model(&GridInventoryLotModel{}).Where("id = ?", id).Updates(updates).Error
}

func (s *GridStore) GetInventoryPairViews(traderID, symbol string, limit int) ([]GridInventoryPairView, error) {
	var lots []GridInventoryLotModel
	q := s.db.Where("trader_id = ?", traderID)
	if symbol != "" {
		q = q.Where("symbol = ?", symbol)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("opened_at DESC").Find(&lots).Error; err != nil {
		return nil, err
	}
	if len(lots) == 0 {
		return []GridInventoryPairView{}, nil
	}

	entryIDs, exitIDs := make([]string, 0, len(lots)), make([]string, 0, len(lots))
	seenE, seenX := map[string]struct{}{}, map[string]struct{}{}
	for _, lot := range lots {
		if lot.EntryOrderID != "" {
			if _, ok := seenE[lot.EntryOrderID]; !ok {
				seenE[lot.EntryOrderID] = struct{}{}
				entryIDs = append(entryIDs, lot.EntryOrderID)
			}
		}
		if lot.ExitOrderID != "" {
			if _, ok := seenX[lot.ExitOrderID]; !ok {
				seenX[lot.ExitOrderID] = struct{}{}
				exitIDs = append(exitIDs, lot.ExitOrderID)
			}
		}
	}
	orderIDs := append(append([]string{}, entryIDs...), exitIDs...)

	var orders []TraderOrder
	if len(orderIDs) > 0 {
		if err := s.db.Where("trader_id = ? AND exchange_order_id IN ?", traderID, orderIDs).Find(&orders).Error; err != nil {
			return nil, err
		}
	}
	orderByID := make(map[string]TraderOrder, len(orders))
	for _, o := range orders {
		orderByID[o.ExchangeOrderID] = o
	}

	var fills []TraderFill
	if len(orderIDs) > 0 {
		if err := s.db.Where("trader_id = ? AND exchange_order_id IN ?", traderID, orderIDs).Find(&fills).Error; err != nil {
			return nil, err
		}
	}
	fillsByID := make(map[string][]TraderFill)
	for _, f := range fills {
		fillsByID[f.ExchangeOrderID] = append(fillsByID[f.ExchangeOrderID], f)
	}

	aggregate := func(exchangeOrderID string, fallbackPrice, fallbackQty float64, fallbackTime int64) (price, value, fee, pnl float64, ts int64) {
		group := fillsByID[exchangeOrderID]
		if len(group) == 0 {
			if fallbackPrice > 0 && fallbackQty > 0 {
				value = fallbackPrice * fallbackQty
			}
			return fallbackPrice, value, 0, 0, fallbackTime
		}
		var totalQty, totalQuote float64
		for _, fill := range group {
			totalQty += fill.Quantity
			totalQuote += fill.QuoteQuantity
			fee += fill.Commission
			pnl += fill.RealizedPnL
			if fill.CreatedAt > ts {
				ts = fill.CreatedAt
			}
		}
		if totalQty > 0 {
			price = totalQuote / totalQty
		} else {
			price = fallbackPrice
		}
		value = totalQuote
		if value == 0 && price > 0 && fallbackQty > 0 {
			value = price * fallbackQty
		}
		if ts == 0 {
			ts = fallbackTime
		}
		return
	}

	results := make([]GridInventoryPairView, 0, len(lots))
	nowMs := time.Now().UnixMilli()
	for _, lot := range lots {
		entryOrder := orderByID[lot.EntryOrderID]
		exitOrder := orderByID[lot.ExitOrderID]

		entryPrice, entryValue, entryFee, _, entryTime := aggregate(lot.EntryOrderID, firstNonZero(lot.EntryPrice, entryOrder.AvgFillPrice, entryOrder.Price), firstNonZero(lot.EntryQuantity, entryOrder.FilledQuantity, entryOrder.Quantity), entryOrder.FilledAt)
		if entryTime == 0 {
			entryTime = lot.OpenedAt.UnixMilli()
		}
		closedQty := firstNonZero(lot.EntryQuantity-lot.RemainingQty, exitOrder.FilledQuantity, exitOrder.Quantity)
		exitPrice, exitValue, exitFee, realizedPnL, exitTime := aggregate(lot.ExitOrderID, firstNonZero(exitOrder.AvgFillPrice, exitOrder.Price), closedQty, exitOrder.FilledAt)
		if exitTime == 0 && lot.ClosedAt != nil {
			exitTime = lot.ClosedAt.UnixMilli()
		}

		expectedExitLevel := lot.ExitLevelIndex
		if expectedExitLevel < 0 {
			switch lot.PositionSide {
			case "LONG":
				expectedExitLevel = lot.SourceLevelIndex + 1
			case "SHORT":
				expectedExitLevel = lot.SourceLevelIndex - 1
			}
		}

		holdDurationMs := nowMs - entryTime
		if exitTime > 0 {
			holdDurationMs = exitTime - entryTime
		}
		if holdDurationMs < 0 {
			holdDurationMs = 0
		}

		results = append(results, GridInventoryPairView{
			ID:               lot.ID,
			Symbol:           lot.Symbol,
			PositionSide:     lot.PositionSide,
			SourceLevelIndex: lot.SourceLevelIndex,
			ExitLevelIndex:   expectedExitLevel,
			EntryPrice:       roundForDisplay(entryPrice),
			ExitOrderPrice:   roundForDisplay(firstNonZero(exitOrder.Price, exitOrder.AvgFillPrice)),
			EntryValue:       roundForDisplay(entryValue),
			ExitPrice:        roundForDisplay(exitPrice),
			ExitValue:        roundForDisplay(exitValue),
			RealizedPnL:      roundForDisplay(realizedPnL),
			Fee:              roundForDisplay(entryFee + exitFee),
			EntryTime:        entryTime,
			ExitTime:         exitTime,
			HoldDurationMs:   holdDurationMs,
			Status:           lot.Status,
			EntryOrderID:     lot.EntryOrderID,
			ExitOrderID:      lot.ExitOrderID,
			RemainingQty:     roundForDisplay(lot.RemainingQty),
			EntryQuantity:    roundForDisplay(lot.EntryQuantity),
		})
	}
	return results, nil
}

func firstNonZero(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func roundForDisplay(v float64) float64 {
	if v == 0 {
		return 0
	}
	return math.Round(v*1e8) / 1e8
}

func (s *GridStore) LogRiskEvent(event *GridRiskEvent) error {
	if event == nil {
		return nil
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	model := &GridRiskEventModel{
		ID:                  event.ID,
		TraderID:            event.TraderID,
		Symbol:              event.Symbol,
		RiskState:           event.RiskState,
		PreviousRiskState:   event.PreviousRiskState,
		PositionSide:        event.PositionSide,
		EventType:           event.EventType,
		Reason:              event.Reason,
		TriggerPrice:        event.TriggerPrice,
		ExecutionPrice:      event.ExecutionPrice,
		EntryPrice:          event.EntryPrice,
		PositionQty:         event.PositionQty,
		ReducedQty:          event.ReducedQty,
		RemainingQty:        event.RemainingQty,
		PositionNotional:    event.PositionNotional,
		ReducedNotional:     event.ReducedNotional,
		UnrealizedLoss:      event.UnrealizedLoss,
		UnrealizedLossPct:   event.UnrealizedLossPct,
		UnrealizedLossEqPct: event.UnrealizedLossEqPct,
		PositionPercent:     event.PositionPercent,
		EffectiveLeverage:   event.EffectiveLeverage,
		LiquidationPrice:    event.LiquidationPrice,
		LiquidationDistance: event.LiquidationDistance,
		ShortBoxUpper:       event.ShortBoxUpper,
		ShortBoxLower:       event.ShortBoxLower,
		GridUpperPrice:      event.GridUpperPrice,
		GridLowerPrice:      event.GridLowerPrice,
		GridSpacing:         event.GridSpacing,
		CreatedAt:           event.CreatedAt,
	}
	if len(event.Metadata) > 0 {
		if payload, err := json.Marshal(event.Metadata); err == nil {
			model.Metadata = string(payload)
		}
	}
	return s.db.Create(model).Error
}

func (s *GridStore) GetRiskEvents(traderID, symbol string, limit int) ([]GridRiskEvent, error) {
	var models []GridRiskEventModel
	q := s.db.Where("trader_id = ?", traderID)
	if symbol != "" {
		q = q.Where("symbol = ?", symbol)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	events := make([]GridRiskEvent, 0, len(models))
	for _, model := range models {
		event := GridRiskEvent{
			ID:                  model.ID,
			TraderID:            model.TraderID,
			Symbol:              model.Symbol,
			RiskState:           model.RiskState,
			PreviousRiskState:   model.PreviousRiskState,
			PositionSide:        model.PositionSide,
			EventType:           model.EventType,
			Reason:              model.Reason,
			TriggerPrice:        model.TriggerPrice,
			ExecutionPrice:      model.ExecutionPrice,
			EntryPrice:          model.EntryPrice,
			PositionQty:         model.PositionQty,
			ReducedQty:          model.ReducedQty,
			RemainingQty:        model.RemainingQty,
			PositionNotional:    model.PositionNotional,
			ReducedNotional:     model.ReducedNotional,
			UnrealizedLoss:      model.UnrealizedLoss,
			UnrealizedLossPct:   model.UnrealizedLossPct,
			UnrealizedLossEqPct: model.UnrealizedLossEqPct,
			PositionPercent:     model.PositionPercent,
			EffectiveLeverage:   model.EffectiveLeverage,
			LiquidationPrice:    model.LiquidationPrice,
			LiquidationDistance: model.LiquidationDistance,
			ShortBoxUpper:       model.ShortBoxUpper,
			ShortBoxLower:       model.ShortBoxLower,
			GridUpperPrice:      model.GridUpperPrice,
			GridLowerPrice:      model.GridLowerPrice,
			GridSpacing:         model.GridSpacing,
			CreatedAt:           model.CreatedAt,
		}
		if model.Metadata != "" {
			_ = json.Unmarshal([]byte(model.Metadata), &event.Metadata)
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *GridStore) GetLatestRiskEvent(traderID, symbol string) (*GridRiskEvent, error) {
	events, err := s.GetRiskEvents(traderID, symbol, 1)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return &events[0], nil
}
