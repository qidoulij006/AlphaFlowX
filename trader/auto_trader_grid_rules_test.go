package trader

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

type gridRulesTestTrader struct {
	marketPrice        float64
	openOrders         []OpenOrder
	positions          []map[string]interface{}
	balance            map[string]interface{}
	orderSeq           int
	cancelledOrderIDs  []string
	cancelAllCallCount int
	closeLongCalls     []float64
	closeShortCalls    []float64
}

func (t *gridRulesTestTrader) GetBalance() (map[string]interface{}, error) {
	if t.balance != nil {
		result := make(map[string]interface{}, len(t.balance))
		for k, v := range t.balance {
			result[k] = v
		}
		return result, nil
	}
	return map[string]interface{}{
		"totalWalletBalance":    1000.0,
		"totalUnrealizedProfit": 0.0,
		"availableBalance":      1000.0,
	}, nil
}

func (t *gridRulesTestTrader) GetPositions() ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(t.positions))
	for _, pos := range t.positions {
		copied := make(map[string]interface{}, len(pos))
		for k, v := range pos {
			copied[k] = v
		}
		result = append(result, copied)
	}
	return result, nil
}

func (t *gridRulesTestTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}

func (t *gridRulesTestTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}

func (t *gridRulesTestTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	t.closeLongCalls = append(t.closeLongCalls, quantity)
	return nil, nil
}

func (t *gridRulesTestTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	t.closeShortCalls = append(t.closeShortCalls, quantity)
	return nil, nil
}

func (t *gridRulesTestTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (t *gridRulesTestTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (t *gridRulesTestTrader) GetMarketPrice(symbol string) (float64, error) {
	return t.marketPrice, nil
}

func (t *gridRulesTestTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (t *gridRulesTestTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (t *gridRulesTestTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (t *gridRulesTestTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (t *gridRulesTestTrader) CancelAllOrders(symbol string) error {
	t.cancelAllCallCount++
	t.openOrders = nil
	return nil
}

func (t *gridRulesTestTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (t *gridRulesTestTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.4f", quantity), nil
}

func (t *gridRulesTestTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "NEW"}, nil
}

func (t *gridRulesTestTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}

func (t *gridRulesTestTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	result := make([]OpenOrder, len(t.openOrders))
	copy(result, t.openOrders)
	return result, nil
}

func (t *gridRulesTestTrader) PlaceLimitOrder(req *LimitOrderRequest) (*LimitOrderResult, error) {
	t.orderSeq++
	orderID := fmt.Sprintf("test-order-%d", t.orderSeq)
	t.openOrders = append(t.openOrders, OpenOrder{
		OrderID:      orderID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Type:         "LIMIT",
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	})
	return &LimitOrderResult{
		OrderID:      orderID,
		ClientID:     req.ClientID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	}, nil
}

func (t *gridRulesTestTrader) CancelOrder(symbol, orderID string) error {
	t.cancelledOrderIDs = append(t.cancelledOrderIDs, orderID)
	filtered := make([]OpenOrder, 0, len(t.openOrders))
	for _, order := range t.openOrders {
		if order.OrderID != orderID {
			filtered = append(filtered, order)
		}
	}
	t.openOrders = filtered
	return nil
}

func (t *gridRulesTestTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	return nil, nil, nil
}

func newGridRulesTestAutoTrader() (*AutoTrader, *gridRulesTestTrader) {
	gridConfig := &store.GridStrategyConfig{
		Symbol:          "DOGEUSDT",
		GridCount:       5,
		TotalInvestment: 1000,
		Leverage:        5,
		UpperPrice:      102,
		LowerPrice:      98,
		UseMakerOnly:    false,
		Distribution:    "uniform",
	}
	testTrader := &gridRulesTestTrader{marketPrice: 100}
	at := &AutoTrader{
		id:       "grid-rules-test",
		name:     "grid-rules-test",
		exchange: "okx",
		trader:   testTrader,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{GridConfig: gridConfig},
		},
		gridState: NewGridState(gridConfig),
	}
	at.gridState.LowerPrice = gridConfig.LowerPrice
	at.gridState.UpperPrice = gridConfig.UpperPrice
	at.gridState.GridSpacing = (gridConfig.UpperPrice - gridConfig.LowerPrice) / float64(gridConfig.GridCount-1)
	at.initializeGridLevels(100, gridConfig)
	return at, testTrader
}

func TestGridRulesAcrossThreeCycles(t *testing.T) {
	at, testTrader := newGridRulesTestAutoTrader()
	symbol := at.config.StrategyConfig.GridConfig.Symbol
	buyLevelIdx := 1
	sellLevelIdx := 3
	buyPrice := at.gridState.Levels[buyLevelIdx].Price
	sellPrice := at.gridState.Levels[sellLevelIdx].Price

	t.Run("cycle_1_entry_orders_ignore_opposite_live_position", func(t *testing.T) {
		testTrader.positions = []map[string]interface{}{
			{
				"symbol":      symbol,
				"side":        "SHORT",
				"positionAmt": 2.0,
				"entryPrice":  buyPrice,
				"markPrice":   buyPrice,
			},
		}
		buyDecision := &kernel.Decision{
			Symbol:     symbol,
			Action:     "place_buy_limit",
			Price:      buyPrice,
			Quantity:   1,
			LevelIndex: buyLevelIdx,
		}
		if err := at.placeGridLimitOrder(buyDecision, "BUY"); err != nil {
			t.Fatalf("cycle 1 buy entry: %v", err)
		}
		if len(testTrader.openOrders) != 1 {
			t.Fatalf("cycle 1 expected 1 open order after buy, got %d", len(testTrader.openOrders))
		}
		if order := testTrader.openOrders[0]; order.PositionSide != "LONG" || order.Side != "BUY" {
			t.Fatalf("cycle 1 buy should remain open-long entry, got side=%s posSide=%s", order.Side, order.PositionSide)
		}

		testTrader.positions = []map[string]interface{}{
			{
				"symbol":      symbol,
				"side":        "LONG",
				"positionAmt": 2.0,
				"entryPrice":  sellPrice,
				"markPrice":   sellPrice,
			},
		}
		sellDecision := &kernel.Decision{
			Symbol:     symbol,
			Action:     "place_sell_limit",
			Price:      sellPrice,
			Quantity:   1,
			LevelIndex: sellLevelIdx,
		}
		if err := at.placeGridLimitOrder(sellDecision, "SELL"); err != nil {
			t.Fatalf("cycle 1 sell entry: %v", err)
		}
		if len(testTrader.openOrders) != 2 {
			t.Fatalf("cycle 1 expected 2 open orders after sell, got %d", len(testTrader.openOrders))
		}
		if order := testTrader.openOrders[1]; order.PositionSide != "SHORT" || order.Side != "SELL" {
			t.Fatalf("cycle 1 sell should remain open-short entry, got side=%s posSide=%s", order.Side, order.PositionSide)
		}
		t.Logf("cycle 1 verified: BUY stayed LONG entry and SELL stayed SHORT entry under opposite live positions")
	})

	t.Run("cycle_2_same_level_can_hold_filled_position_exit_and_new_entry", func(t *testing.T) {
		at, testTrader = newGridRulesTestAutoTrader()
		buyPrice = at.gridState.Levels[buyLevelIdx].Price
		level := &at.gridState.Levels[buyLevelIdx]
		level.State = "filled"
		level.PositionSide = "SHORT"
		level.PositionSize = 1
		level.PositionEntry = buyPrice

		testTrader.positions = []map[string]interface{}{
			{
				"symbol":      symbol,
				"side":        "SHORT",
				"positionAmt": 1.0,
				"entryPrice":  buyPrice,
				"markPrice":   buyPrice,
			},
		}
		testTrader.openOrders = []OpenOrder{
			{
				OrderID:      "close-short-same-level",
				Symbol:       symbol,
				Side:         "BUY",
				PositionSide: "SHORT",
				Type:         "LIMIT",
				Price:        buyPrice,
				Quantity:     1,
				Status:       "NEW",
			},
		}

		buyDecision := &kernel.Decision{
			Symbol:     symbol,
			Action:     "place_buy_limit",
			Price:      buyPrice,
			Quantity:   1,
			LevelIndex: buyLevelIdx,
		}
		if err := at.placeGridLimitOrder(buyDecision, "BUY"); err != nil {
			t.Fatalf("cycle 2 buy entry on filled level: %v", err)
		}

		if len(testTrader.openOrders) != 2 {
			t.Fatalf("cycle 2 expected reduce-only exit plus new entry, got %d orders", len(testTrader.openOrders))
		}
		if level.State != "filled" {
			t.Fatalf("cycle 2 expected level to remain filled, got %s", level.State)
		}
		if level.PositionSize != 1 {
			t.Fatalf("cycle 2 expected filled position to remain attached, got qty=%.4f", level.PositionSize)
		}
		if level.OrderID == "" {
			t.Fatalf("cycle 2 expected a new entry order to coexist on the filled level")
		}
		if level.OrderPositionSide != "LONG" || level.OrderReduceOnly {
			t.Fatalf("cycle 2 expected coexisting order to be non-reduce-only LONG entry, got posSide=%s reduceOnly=%t", level.OrderPositionSide, level.OrderReduceOnly)
		}
		t.Logf("cycle 2 verified: filled SHORT level kept its close_short BUY exit and accepted a new BUY LONG entry on the same level")
	})

	t.Run("cycle_3_adjust_grid_cancels_only_entries_and_preserves_exits", func(t *testing.T) {
		at, testTrader = newGridRulesTestAutoTrader()
		buyPrice = at.gridState.Levels[buyLevelIdx].Price
		exitLevelIdx := 0
		exitPrice := at.gridState.Levels[exitLevelIdx].Price
		level := &at.gridState.Levels[buyLevelIdx]
		level.State = "filled"
		level.PositionSide = "SHORT"
		level.PositionSize = 1
		level.PositionEntry = buyPrice
		level.OrderID = "entry-buy-same-level"
		level.OrderQuantity = 1
		level.OrderPositionSide = "LONG"
		level.OrderReduceOnly = false
		at.gridState.OrderBook["entry-buy-same-level"] = buyLevelIdx

		testTrader.positions = []map[string]interface{}{
			{
				"symbol":      symbol,
				"side":        "SHORT",
				"positionAmt": 1.0,
				"entryPrice":  buyPrice,
				"markPrice":   buyPrice,
			},
		}
		testTrader.openOrders = []OpenOrder{
			{
				OrderID:      "close-short-same-level",
				Symbol:       symbol,
				Side:         "BUY",
				PositionSide: "SHORT",
				Type:         "LIMIT",
				Price:        exitPrice,
				Quantity:     1,
				Status:       "NEW",
			},
			{
				OrderID:      "entry-buy-same-level",
				Symbol:       symbol,
				Side:         "BUY",
				PositionSide: "LONG",
				Type:         "LIMIT",
				Price:        buyPrice,
				Quantity:     1,
				Status:       "NEW",
			},
		}

		if err := at.adjustGrid(&kernel.Decision{Symbol: symbol, Action: "adjust_grid"}); err != nil {
			t.Fatalf("cycle 3 adjust grid: %v", err)
		}

		if len(testTrader.cancelledOrderIDs) != 1 || testTrader.cancelledOrderIDs[0] != "entry-buy-same-level" {
			t.Fatalf("cycle 3 expected only entry order to be cancelled, got %+v", testTrader.cancelledOrderIDs)
		}
		if testTrader.cancelAllCallCount != 0 {
			t.Fatalf("cycle 3 expected no CancelAllOrders call during adjust, got %d", testTrader.cancelAllCallCount)
		}
		if len(testTrader.openOrders) != 1 || testTrader.openOrders[0].OrderID != "close-short-same-level" {
			t.Fatalf("cycle 3 expected reduce-only exit to remain live, got %+v", testTrader.openOrders)
		}
		t.Logf("cycle 3 verified: adjust_grid cancelled only entry orders and preserved the existing reduce-only exit")
	})

	t.Run("cycle_4_flat_after_final_exit_rebuilds_full_entry_grid", func(t *testing.T) {
		at, testTrader = newGridRulesTestAutoTrader()
		sourceLevelIdx := sellLevelIdx
		exitLevelIdx := 2
		sourceLevel := &at.gridState.Levels[sourceLevelIdx]
		sourceLevel.State = "filled"
		sourceLevel.PositionSide = "SHORT"
		sourceLevel.PositionSize = 1
		sourceLevel.PositionEntry = sourceLevel.Price

		exitLevel := &at.gridState.Levels[exitLevelIdx]
		exitLevel.State = "pending"
		exitLevel.OrderID = "close-short-final"
		exitLevel.OrderQuantity = 1
		exitLevel.OrderPositionSide = "SHORT"
		exitLevel.OrderReduceOnly = true
		exitLevel.LinkedLevelIndex = sourceLevelIdx
		at.gridState.OrderBook["close-short-final"] = exitLevelIdx

		testTrader.positions = nil
		testTrader.balance = map[string]interface{}{
			"total_equity": 1000.0,
		}
		testTrader.openOrders = []OpenOrder{
			{
				OrderID:      "stale-entry-1",
				Symbol:       symbol,
				Side:         "BUY",
				PositionSide: "LONG",
				Type:         "LIMIT",
				Price:        at.gridState.Levels[0].Price,
				Quantity:     1,
				Status:       "NEW",
			},
			{
				OrderID:      "stale-entry-2",
				Symbol:       symbol,
				Side:         "SELL",
				PositionSide: "SHORT",
				Type:         "LIMIT",
				Price:        at.gridState.Levels[len(at.gridState.Levels)-1].Price,
				Quantity:     1,
				Status:       "NEW",
			},
		}

		at.syncGridState()

		if testTrader.cancelAllCallCount != 1 {
			t.Fatalf("cycle 4 expected one CancelAllOrders call after flat exit, got %d", testTrader.cancelAllCallCount)
		}
		expectedRebuiltOrders := at.config.StrategyConfig.GridConfig.GridCount - 2
		if len(testTrader.openOrders) != expectedRebuiltOrders {
			t.Fatalf("cycle 4 expected %d rebuilt entry orders after first-entry guard exclusions, got %d", expectedRebuiltOrders, len(testTrader.openOrders))
		}
		if got := testTrader.openOrders[0].Quantity; got < 10 {
			t.Fatalf("cycle 4 expected flat rebuild to use live 1000 equity base, got first qty=%.4f", got)
		}
		for _, order := range testTrader.openOrders {
			if (order.Side == "SELL" && order.PositionSide == "LONG") || (order.Side == "BUY" && order.PositionSide == "SHORT") {
				t.Fatalf("cycle 4 expected only fresh entry orders after flat rebuild, found reduce-only order %+v", order)
			}
		}
		t.Logf("cycle 4 verified: final exit to flat cancelled all legacy orders and rebuilt guard-eligible entry grid")
	})
}

func TestGridFirstEntryGuardAndRiskAdmission(t *testing.T) {
	at, testTrader := newGridRulesTestAutoTrader()
	symbol := at.config.StrategyConfig.GridConfig.Symbol
	at.gridState.LastContextATR14 = 0.05
	at.gridState.LastContextPrice = 100
	at.gridState.LastContextBuiltAt = time.Now()

	t.Run("blocks_short_first_entry_inside_guard_band", func(t *testing.T) {
		testTrader.positions = nil
		testTrader.marketPrice = 101.20
		err := at.executeGridDecision(&kernel.Decision{
			Symbol:     symbol,
			Action:     "place_sell_limit",
			Price:      101.20,
			Quantity:   1,
			LevelIndex: 3,
			Confidence: 95,
			ForceEntry: true,
		})
		if err == nil {
			t.Fatalf("expected first-entry guard rejection, got nil")
		}
		if !strings.Contains(err.Error(), "first-entry guard") {
			t.Fatalf("expected first-entry guard error, got %v", err)
		}
		testTrader.marketPrice = 100
	})

	t.Run("blocks_short_first_entry_order_price_above_guard_line", func(t *testing.T) {
		testTrader.positions = nil
		testTrader.marketPrice = 100
		err := at.executeGridDecision(&kernel.Decision{
			Symbol:     symbol,
			Action:     "place_sell_limit",
			Price:      102.00,
			Quantity:   1,
			LevelIndex: 4,
			Confidence: 95,
			ForceEntry: true,
		})
		if err == nil {
			t.Fatalf("expected first-entry guard rejection for order price above guard, got nil")
		}
		if !strings.Contains(err.Error(), "order price") {
			t.Fatalf("expected order-price first-entry guard error, got %v", err)
		}
	})

	t.Run("cancels_existing_short_entry_orders_above_guard_when_flat", func(t *testing.T) {
		at, testTrader := newGridRulesTestAutoTrader()
		symbol := at.config.StrategyConfig.GridConfig.Symbol
		testTrader.marketPrice = 100
		testTrader.positions = nil
		testTrader.openOrders = []OpenOrder{
			{OrderID: "short-entry", Symbol: symbol, Side: "SELL", PositionSide: "SHORT", Type: "LIMIT", Price: 102.00, Quantity: 1, Status: "NEW"},
			{OrderID: "short-exit", Symbol: symbol, Side: "BUY", PositionSide: "SHORT", Type: "LIMIT", Price: 99.00, Quantity: 1, Status: "NEW"},
			{OrderID: "long-entry", Symbol: symbol, Side: "BUY", PositionSide: "LONG", Type: "LIMIT", Price: 100.00, Quantity: 1, Status: "NEW"},
		}
		at.gridState.mu.Lock()
		at.gridState.Levels[4].State = "pending"
		at.gridState.Levels[4].OrderID = "short-entry"
		at.gridState.Levels[4].OrderQuantity = 1
		at.gridState.Levels[4].OrderPositionSide = "SHORT"
		at.gridState.OrderBook["short-entry"] = 4
		at.gridState.mu.Unlock()

		openOrders := at.enforceFirstEntryGuardOnOpenOrders(symbol, testTrader.openOrders)
		if len(testTrader.cancelledOrderIDs) != 1 || testTrader.cancelledOrderIDs[0] != "short-entry" {
			t.Fatalf("expected only short-entry to be cancelled, got %#v", testTrader.cancelledOrderIDs)
		}
		if len(openOrders) != 2 {
			t.Fatalf("expected filtered open orders to keep 2 orders, got %d", len(openOrders))
		}
		for _, order := range openOrders {
			if order.OrderID == "short-entry" {
				t.Fatalf("expected short-entry to be removed from filtered open orders")
			}
		}
		at.gridState.mu.RLock()
		_, tracked := at.gridState.OrderBook["short-entry"]
		levelOrderID := at.gridState.Levels[4].OrderID
		levelState := at.gridState.Levels[4].State
		at.gridState.mu.RUnlock()
		if tracked || levelOrderID != "" || levelState != "empty" {
			t.Fatalf("expected grid state to clear cancelled order, tracked=%t orderID=%q state=%q", tracked, levelOrderID, levelState)
		}
	})

	t.Run("blocks_same_side_add_when_warning_active", func(t *testing.T) {
		testTrader.positions = []map[string]interface{}{
			{
				"symbol":      symbol,
				"side":        "SHORT",
				"positionAmt": -2.0,
				"entryPrice":  100.4,
				"markPrice":   100.4,
			},
		}
		at.gridState.CurrentRiskState = gridRiskStateWarning
		at.gridState.RiskStateSide = "SHORT"
		err := at.executeGridDecision(&kernel.Decision{
			Symbol:     symbol,
			Action:     "place_sell_limit",
			Price:      100.60,
			Quantity:   1,
			LevelIndex: 3,
			Confidence: 95,
			ForceEntry: true,
		})
		if err == nil {
			t.Fatalf("expected warning-state add rejection, got nil")
		}
		if !strings.Contains(err.Error(), "warning state") {
			t.Fatalf("expected warning state error, got %v", err)
		}
	})

	t.Run("risk_info_exposes_guard_band_fields", func(t *testing.T) {
		at.gridState.CurrentRiskState = gridRiskStateNormal
		at.gridState.RiskStateSide = ""
		testTrader.positions = nil
		riskInfo := at.GetGridRiskInfo()
		if riskInfo.ShortFirstEntryGuardPrice <= 0 || riskInfo.LongFirstEntryGuardPrice <= 0 {
			t.Fatalf("expected guard prices in risk info, got short=%f long=%f", riskInfo.ShortFirstEntryGuardPrice, riskInfo.LongFirstEntryGuardPrice)
		}
		if riskInfo.FirstEntryGuardActiveSource == "" {
			t.Fatalf("expected active guard source")
		}
		if riskInfo.FirstEntryGuardThresholdPct != gridFirstEntryGuardThresholdPctDefault {
			t.Fatalf("expected threshold pct %.2f, got %f", gridFirstEntryGuardThresholdPctDefault, riskInfo.FirstEntryGuardThresholdPct)
		}
	})
}

func TestGridEntrySizingOverridesAIQuantityWithSystemBase(t *testing.T) {
	at, testTrader := newGridRulesTestAutoTrader()
	symbol := at.config.StrategyConfig.GridConfig.Symbol
	testTrader.balance = map[string]interface{}{
		"total_equity": 1000.0,
	}
	testTrader.marketPrice = 100.0

	levelIdx := 2
	price := at.gridState.Levels[levelIdx].Price
	expectedQty := at.suggestedEntryQuantityWithInvestmentBase(levelIdx, price, 1000.0)
	if math.Abs(expectedQty-10.0) > 0.0001 {
		t.Fatalf("expected baseline system quantity 10.0, got %.4f", expectedQty)
	}

	err := at.executeGridDecision(&kernel.Decision{
		Symbol:     symbol,
		Action:     "place_buy_limit",
		Price:      price,
		Quantity:   1.0,
		LevelIndex: levelIdx,
		Confidence: 95,
		ForceEntry: true,
	})
	if err != nil {
		t.Fatalf("expected entry placement to succeed, got %v", err)
	}
	if len(testTrader.openOrders) != 1 {
		t.Fatalf("expected one entry order, got %d", len(testTrader.openOrders))
	}
	if got := testTrader.openOrders[0].Quantity; math.Abs(got-expectedQty) > 0.0001 {
		t.Fatalf("expected system quantity %.4f to override AI quantity, got %.4f", expectedQty, got)
	}
}

func TestGridReconcileEntryOrderSizingReplacesLargeDrift(t *testing.T) {
	at, testTrader := newGridRulesTestAutoTrader()
	symbol := at.config.StrategyConfig.GridConfig.Symbol
	testTrader.balance = map[string]interface{}{
		"total_equity": 1000.0,
	}
	testTrader.marketPrice = 100.0

	levelIdx := 2
	price := at.gridState.Levels[levelIdx].Price
	testTrader.openOrders = []OpenOrder{{
		OrderID:      "stale-entry",
		Symbol:       symbol,
		Side:         "BUY",
		PositionSide: "LONG",
		Type:         "LIMIT",
		Price:        price,
		Quantity:     1.0,
		Status:       "NEW",
	}}
	at.gridState.OrderBook["stale-entry"] = levelIdx
	at.gridState.Levels[levelIdx].OrderID = "stale-entry"
	at.gridState.Levels[levelIdx].OrderQuantity = 1.0
	at.gridState.Levels[levelIdx].OrderReduceOnly = false
	at.gridState.Levels[levelIdx].OrderPositionSide = "LONG"

	expectedQty := at.suggestedEntryQuantityWithInvestmentBase(levelIdx, price, 1000.0)
	at.reconcileGridEntryOrderSizing(symbol, testTrader.openOrders)

	if len(testTrader.cancelledOrderIDs) != 1 || testTrader.cancelledOrderIDs[0] != "stale-entry" {
		t.Fatalf("expected stale entry order to be cancelled, got %+v", testTrader.cancelledOrderIDs)
	}
	if len(testTrader.openOrders) != 1 {
		t.Fatalf("expected one replacement entry order, got %d", len(testTrader.openOrders))
	}
	if got := testTrader.openOrders[0].Quantity; math.Abs(got-expectedQty) > 0.0001 {
		t.Fatalf("expected replacement quantity %.4f, got %.4f", expectedQty, got)
	}
}

func TestGridReconcileEntryOrderSizingPolicyVersionForcesReseed(t *testing.T) {
	at, testTrader := newGridRulesTestAutoTrader()
	symbol := at.config.StrategyConfig.GridConfig.Symbol
	testTrader.balance = map[string]interface{}{
		"total_equity": 1000.0,
	}
	testTrader.marketPrice = 100.0

	levelIdx := 2
	price := at.gridState.Levels[levelIdx].Price
	expectedQty := at.suggestedEntryQuantityWithInvestmentBase(levelIdx, price, 1000.0)

	testTrader.openOrders = []OpenOrder{{
		OrderID:      "policy-reseed-entry",
		Symbol:       symbol,
		Side:         "BUY",
		PositionSide: "LONG",
		Type:         "LIMIT",
		Price:        price,
		Quantity:     expectedQty,
		Status:       "NEW",
	}}
	at.gridState.OrderBook["policy-reseed-entry"] = levelIdx
	at.gridState.Levels[levelIdx].OrderID = "policy-reseed-entry"
	at.gridState.Levels[levelIdx].OrderQuantity = expectedQty
	at.gridState.Levels[levelIdx].OrderReduceOnly = false
	at.gridState.Levels[levelIdx].OrderPositionSide = "LONG"
	at.gridState.EntrySizingPolicyVersion = "old-policy"

	at.reconcileGridEntryOrderSizing(symbol, testTrader.openOrders)

	if len(testTrader.cancelledOrderIDs) != 1 || testTrader.cancelledOrderIDs[0] != "policy-reseed-entry" {
		t.Fatalf("expected policy reseed to cancel original order, got %+v", testTrader.cancelledOrderIDs)
	}
	if len(testTrader.openOrders) != 1 {
		t.Fatalf("expected one replacement order after policy reseed, got %d", len(testTrader.openOrders))
	}
	if testTrader.openOrders[0].OrderID == "policy-reseed-entry" {
		t.Fatalf("expected a newly placed replacement order, got original order id")
	}
	if at.gridState.EntrySizingPolicyVersion != gridEntrySizingPolicyVersion {
		t.Fatalf("expected policy version to update to %s, got %s", gridEntrySizingPolicyVersion, at.gridState.EntrySizingPolicyVersion)
	}
}

func TestGridGaussianEntryCapUsesUniformMultipliers(t *testing.T) {
	t.Run("normal_zone_caps_at_uniform_1_5x", func(t *testing.T) {
		at, testTrader := newGridRulesTestAutoTrader()
		testTrader.balance = map[string]interface{}{
			"total_equity": 1000.0,
		}
		testTrader.marketPrice = 100.0
		at.gridState.CurrentDirection = market.GridDirectionNeutral
		at.gridState.MidBoxLower = 99.0
		at.gridState.MidBoxUpper = 101.0

		levelIdx := 2
		at.gridState.Levels[levelIdx].AllocatedUSD = 400.0
		decision := &kernel.Decision{
			Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
			Action:     "place_buy_limit",
			Price:      100.0,
			Quantity:   30.0,
			LevelIndex: levelIdx,
			ForceEntry: true,
		}

		if err := at.placeGridLimitOrder(decision, "BUY"); err != nil {
			t.Fatalf("normal zone capped order failed: %v", err)
		}
		if len(testTrader.openOrders) != 1 {
			t.Fatalf("expected one capped entry order, got %d", len(testTrader.openOrders))
		}
		if got := testTrader.openOrders[0].Quantity; math.Abs(got-13.0) > 0.0001 {
			t.Fatalf("expected normal-zone cap at 13.0 qty (uniform 1.3x), got %.4f", got)
		}
	})

	t.Run("edge_zone_caps_at_uniform_1_2x", func(t *testing.T) {
		at, testTrader := newGridRulesTestAutoTrader()
		testTrader.balance = map[string]interface{}{
			"total_equity": 1000.0,
		}
		testTrader.marketPrice = 101.7
		at.gridState.CurrentDirection = market.GridDirectionNeutral
		at.gridState.MidBoxLower = 99.0
		at.gridState.MidBoxUpper = 101.0

		levelIdx := 2
		at.gridState.Levels[levelIdx].AllocatedUSD = 400.0
		decision := &kernel.Decision{
			Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
			Action:     "place_buy_limit",
			Price:      100.0,
			Quantity:   30.0,
			LevelIndex: levelIdx,
			ForceEntry: true,
		}

		if err := at.placeGridLimitOrder(decision, "BUY"); err != nil {
			t.Fatalf("edge zone capped order failed: %v", err)
		}
		if len(testTrader.openOrders) != 1 {
			t.Fatalf("expected one capped entry order, got %d", len(testTrader.openOrders))
		}
		if got := testTrader.openOrders[0].Quantity; math.Abs(got-11.0) > 0.0001 {
			t.Fatalf("expected edge-zone cap at 11.0 qty (uniform 1.1x), got %.4f", got)
		}
	})
}

func TestInventoryLotExitMaintenanceReseedsPartialCoverage(t *testing.T) {
	st := newTestGridStore(t)
	at, testTrader := newGridRulesTestAutoTrader()
	at.store = st

	symbol := at.config.StrategyConfig.GridConfig.Symbol
	sourceLevelIdx := 1
	exitLevelIdx := 2
	exitPrice := at.gridState.Levels[exitLevelIdx].Price

	testTrader.positions = []map[string]interface{}{
		{
			"symbol":      symbol,
			"side":        "LONG",
			"positionAmt": 32869.0,
			"entryPrice":  at.gridState.Levels[sourceLevelIdx].Price,
			"markPrice":   at.gridState.Levels[sourceLevelIdx].Price,
		},
	}
	testTrader.openOrders = []OpenOrder{
		{
			OrderID:      "stale-close-long",
			Symbol:       symbol,
			Side:         "SELL",
			PositionSide: "LONG",
			Type:         "LIMIT",
			Price:        exitPrice,
			Quantity:     20000,
			Status:       "NEW",
		},
	}

	if err := st.Grid().SaveInventoryLot(&store.GridInventoryLotModel{
		TraderID:         at.id,
		Symbol:           symbol,
		PositionSide:     "LONG",
		SourceLevelIndex: sourceLevelIdx,
		ExitLevelIndex:   exitLevelIdx,
		EntryPrice:       at.gridState.Levels[sourceLevelIdx].Price,
		EntryQuantity:    32869,
		RemainingQty:     32869,
		EntryOrderID:     "entry-long-1",
		ExitOrderID:      "stale-close-long",
		Status:           "OPEN",
		OpenedAt:         time.Now(),
	}); err != nil {
		t.Fatalf("save inventory lot: %v", err)
	}

	if ok := at.ensurePairedExitOrdersForInventoryLots(); !ok {
		t.Fatalf("expected lot-based exit maintenance to run")
	}

	if len(testTrader.cancelledOrderIDs) != 1 || testTrader.cancelledOrderIDs[0] != "stale-close-long" {
		t.Fatalf("expected stale partial exit to be cancelled, got %+v", testTrader.cancelledOrderIDs)
	}
	if len(testTrader.openOrders) != 1 {
		t.Fatalf("expected a single reseeded close-long order, got %+v", testTrader.openOrders)
	}
	reseeded := testTrader.openOrders[0]
	if reseeded.OrderID == "stale-close-long" {
		t.Fatalf("expected a new exit order ID after reseed, still got stale order")
	}
	if reseeded.Side != "SELL" || reseeded.PositionSide != "LONG" {
		t.Fatalf("expected SELL LONG reduce-only order, got side=%s posSide=%s", reseeded.Side, reseeded.PositionSide)
	}
	if math.Abs(reseeded.Quantity-32869) > 0.0001 {
		t.Fatalf("expected reseeded exit qty 32869, got %.4f", reseeded.Quantity)
	}
	if math.Abs(reseeded.Price-exitPrice) > 0.0001 {
		t.Fatalf("expected reseeded exit at %.4f, got %.4f", exitPrice, reseeded.Price)
	}

	lots, err := st.Grid().LoadOpenInventoryLots(at.id, symbol)
	if err != nil {
		t.Fatalf("load inventory lots: %v", err)
	}
	if len(lots) != 1 {
		t.Fatalf("expected one open lot after maintenance, got %d", len(lots))
	}
	if lots[0].ExitOrderID != reseeded.OrderID {
		t.Fatalf("expected lot exit binding to update to %s, got %s", reseeded.OrderID, lots[0].ExitOrderID)
	}
}

func TestGridStopLossStateMachineHardReduceAndRiskHistory(t *testing.T) {
	st := newTestGridStore(t)
	at, testTrader := newGridRulesTestAutoTrader()
	at.store = st
	at.gridState.ShortBoxLower = 98.0
	at.gridState.ShortBoxUpper = 102.0
	at.gridState.MidBoxLower = 97.0
	at.gridState.MidBoxUpper = 103.0
	at.gridState.LowerPrice = 98.0
	at.gridState.UpperPrice = 102.0
	at.gridState.GridSpacing = 1.0
	testTrader.marketPrice = 110.0
	testTrader.balance = map[string]interface{}{
		"total_equity": 20000.0,
	}

	symbol := at.config.StrategyConfig.GridConfig.Symbol
	testTrader.positions = []map[string]interface{}{
		{
			"symbol":      symbol,
			"side":        "SHORT",
			"positionAmt": 100.0,
			"entryPrice":  100.0,
			"markPrice":   110.0,
		},
	}
	testTrader.openOrders = []OpenOrder{
		{
			OrderID:      "stale-entry",
			Symbol:       symbol,
			Side:         "SELL",
			PositionSide: "SHORT",
			Type:         "LIMIT",
			Price:        101.0,
			Quantity:     10,
			Status:       "NEW",
		},
	}
	if err := st.Grid().SaveInventoryLot(&store.GridInventoryLotModel{
		TraderID:         at.id,
		Symbol:           symbol,
		PositionSide:     "SHORT",
		SourceLevelIndex: 3,
		ExitLevelIndex:   2,
		EntryPrice:       100.0,
		EntryQuantity:    100.0,
		RemainingQty:     100.0,
		EntryOrderID:     "entry-short-1",
		Status:           "OPEN",
		OpenedAt:         time.Now(),
	}); err != nil {
		t.Fatalf("seed risk lot: %v", err)
	}

	at.checkAndExecuteStopLoss()

	if len(testTrader.closeShortCalls) != 1 {
		t.Fatalf("expected one partial short close, got %+v", testTrader.closeShortCalls)
	}
	if math.Abs(testTrader.closeShortCalls[0]-50.0) > 0.0001 {
		t.Fatalf("expected hard-reduce to close 50 qty, got %.4f", testTrader.closeShortCalls[0])
	}
	if at.gridState.CurrentRiskState != gridRiskStateHardReduce {
		t.Fatalf("expected current risk state hard_reduce, got %s", at.gridState.CurrentRiskState)
	}
	if !at.gridState.IsPaused {
		t.Fatalf("expected grid to pause after hard reduce")
	}
	if len(testTrader.openOrders) != 1 {
		t.Fatalf("expected one post-risk reduce-only exit order, got %+v", testTrader.openOrders)
	}
	if testTrader.openOrders[0].Side != "BUY" || testTrader.openOrders[0].PositionSide != "SHORT" {
		t.Fatalf("expected BUY SHORT reduce-only risk exit, got %+v", testTrader.openOrders[0])
	}

	history := at.GetGridRiskHistory(10)
	if len(history) < 2 {
		t.Fatalf("expected risk history to contain transition and action, got %d", len(history))
	}
	if history[0].EventType != "hard_reduce" {
		t.Fatalf("expected latest risk event hard_reduce, got %s", history[0].EventType)
	}
	if math.Abs(history[0].ReducedQty-50.0) > 0.0001 {
		t.Fatalf("expected reduced qty 50 in history, got %.4f", history[0].ReducedQty)
	}

	riskInfo := at.GetGridRiskInfo()
	if riskInfo.CurrentRiskState != gridRiskStateHardReduce {
		t.Fatalf("expected grid risk info state hard_reduce, got %s", riskInfo.CurrentRiskState)
	}
	if riskInfo.StopLossReducedQty <= 0 {
		t.Fatalf("expected stop-loss reduced qty populated, got %.4f", riskInfo.StopLossReducedQty)
	}
	if len(riskInfo.RiskHistory) == 0 {
		t.Fatalf("expected risk history on grid risk info")
	}
}
