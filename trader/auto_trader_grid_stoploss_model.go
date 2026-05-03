package trader

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"
)

const (
	gridRiskStateNormal     = "normal"
	gridRiskStateWarning    = "warning"
	gridRiskStateSoftReduce = "soft_reduce"
	gridRiskStateHardReduce = "hard_reduce"
	gridRiskStateEmergency  = "emergency"
)

type gridSideRiskSnapshot struct {
	PositionSide         string
	CurrentPrice         float64
	EntryPrice           float64
	PositionQty          float64
	PositionNotional     float64
	UnrealizedLoss       float64
	UnrealizedLossPct    float64
	UnrealizedLossEqPct  float64
	PositionPercent      float64
	EffectiveLeverage    float64
	LiquidationPrice     float64
	LiquidationDistance  float64
	WorstLotLossPct      float64
	BreakoutPct          float64
	EntryLocationPenalty float64
	WarningThreshold     float64
	SoftReduceThreshold  float64
	HardReduceThreshold  float64
	RiskState            string
	Reason               string
	ShortBoxUpper        float64
	ShortBoxLower        float64
	GridUpperPrice       float64
	GridLowerPrice       float64
	GridSpacing          float64
}

func gridRiskSeverity(state string) int {
	switch state {
	case gridRiskStateWarning:
		return 1
	case gridRiskStateSoftReduce:
		return 2
	case gridRiskStateHardReduce:
		return 3
	case gridRiskStateEmergency:
		return 4
	default:
		return 0
	}
}

func clampGridRiskThreshold(v, minV float64) float64 {
	if v < minV {
		return minV
	}
	return v
}

func (at *AutoTrader) getCurrentPositionSnapshot(symbol string, positionSide string) (qty, entryPrice float64, found bool) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("%s Failed to fetch positions for risk snapshot: %v", at.gridLogPrefix(), err)
		return 0, 0, false
	}

	targetSide := strings.ToUpper(strings.TrimSpace(positionSide))
	for _, pos := range positions {
		sym, _ := pos["symbol"].(string)
		if sym != symbol {
			continue
		}
		size, ok := pos["positionAmt"].(float64)
		if !ok || size == 0 {
			continue
		}
		side, _ := pos["side"].(string)
		if normalizeGridPositionSide(side, "") != targetSide {
			continue
		}
		entryPrice, _ = pos["entryPrice"].(float64)
		return math.Abs(size), entryPrice, true
	}
	return 0, 0, false
}

func (at *AutoTrader) getWorstLotLossPct(symbol string, positionSide string, currentPrice float64) float64 {
	if at.store == nil || currentPrice <= 0 {
		return 0
	}
	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, symbol)
	if err != nil {
		return 0
	}
	worst := 0.0
	targetSide := strings.ToUpper(strings.TrimSpace(positionSide))
	for _, lot := range lots {
		if lot.RemainingQty <= 0 || strings.ToUpper(strings.TrimSpace(lot.PositionSide)) != targetSide || lot.EntryPrice <= 0 {
			continue
		}
		lossPct := 0.0
		if targetSide == "LONG" {
			lossPct = (lot.EntryPrice - currentPrice) / lot.EntryPrice * 100
		} else {
			lossPct = (currentPrice - lot.EntryPrice) / lot.EntryPrice * 100
		}
		if lossPct > worst {
			worst = lossPct
		}
	}
	return worst
}

func (at *AutoTrader) computeGridRiskPositionPercent(positionNotional, investmentBase float64, leverage int) float64 {
	maxPositionPct := 70.0
	regimeLevel := strings.TrimSpace(at.gridState.CurrentRegimeLevel)
	switch regimeLevel {
	case "narrow":
		maxPositionPct = 40
	case "wide":
		maxPositionPct = 60
	case "volatile":
		maxPositionPct = 40
	}
	maxPosition := 0.0
	if investmentBase > 0 && leverage > 0 {
		maxPosition = investmentBase * maxPositionPct / 100 * float64(leverage)
	}
	if maxPosition <= 0 {
		return 0
	}
	return positionNotional / maxPosition * 100
}

func (at *AutoTrader) computeEntryLocationPenalty(positionSide string, entryPrice, shortLower, shortUpper float64) float64 {
	if entryPrice <= 0 || shortUpper <= shortLower {
		return 0
	}
	width := shortUpper - shortLower
	switch strings.ToUpper(strings.TrimSpace(positionSide)) {
	case "SHORT":
		switch {
		case entryPrice <= shortLower+width*0.35:
			return 1.0
		case entryPrice <= shortLower+width*0.55:
			return 0.5
		default:
			return 0
		}
	case "LONG":
		switch {
		case entryPrice >= shortUpper-width*0.35:
			return 1.0
		case entryPrice >= shortUpper-width*0.55:
			return 0.5
		default:
			return 0
		}
	default:
		return 0
	}
}

func (at *AutoTrader) determineGridRiskState(snapshot *gridSideRiskSnapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.PositionQty <= 0 || snapshot.CurrentPrice <= 0 || snapshot.EntryPrice <= 0 {
		snapshot.RiskState = gridRiskStateNormal
		snapshot.Reason = "no live position"
		return
	}

	baseHard := 5.0
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.GridConfig != nil && at.config.StrategyConfig.GridConfig.StopLossPct > 0 {
		baseHard = math.Max(baseHard, at.config.StrategyConfig.GridConfig.StopLossPct)
	}
	penalty := snapshot.EntryLocationPenalty
	snapshot.WarningThreshold = clampGridRiskThreshold(2.5-penalty, 1.5)
	snapshot.SoftReduceThreshold = clampGridRiskThreshold(4.0-penalty, 2.5)
	snapshot.HardReduceThreshold = clampGridRiskThreshold(baseHard-penalty, 3.5)

	switch {
	case snapshot.LiquidationDistance > 0 && snapshot.LiquidationDistance <= 3:
		snapshot.RiskState = gridRiskStateEmergency
		snapshot.Reason = fmt.Sprintf("liquidation distance %.2f%% <= 3%%", snapshot.LiquidationDistance)
	case snapshot.UnrealizedLossEqPct >= 8:
		snapshot.RiskState = gridRiskStateEmergency
		snapshot.Reason = fmt.Sprintf("unrealized loss %.2f%% of equity >= 8%%", snapshot.UnrealizedLossEqPct)
	case snapshot.UnrealizedLossPct >= snapshot.HardReduceThreshold || snapshot.WorstLotLossPct >= snapshot.HardReduceThreshold+0.5 || snapshot.BreakoutPct >= 4:
		snapshot.RiskState = gridRiskStateHardReduce
		snapshot.Reason = fmt.Sprintf("side loss %.2f%% / worst lot %.2f%% / breakout %.2f%%", snapshot.UnrealizedLossPct, snapshot.WorstLotLossPct, snapshot.BreakoutPct)
	case snapshot.UnrealizedLossPct >= snapshot.SoftReduceThreshold || snapshot.WorstLotLossPct >= snapshot.SoftReduceThreshold+0.75 || snapshot.BreakoutPct > 0:
		snapshot.RiskState = gridRiskStateSoftReduce
		snapshot.Reason = fmt.Sprintf("side loss %.2f%% / worst lot %.2f%% / breakout %.2f%%", snapshot.UnrealizedLossPct, snapshot.WorstLotLossPct, snapshot.BreakoutPct)
	case snapshot.UnrealizedLossPct >= snapshot.WarningThreshold || snapshot.BreakoutPct > 0:
		snapshot.RiskState = gridRiskStateWarning
		snapshot.Reason = fmt.Sprintf("side loss %.2f%% / breakout %.2f%%", snapshot.UnrealizedLossPct, snapshot.BreakoutPct)
	default:
		snapshot.RiskState = gridRiskStateNormal
		snapshot.Reason = "risk metrics within normal range"
	}
}

func (at *AutoTrader) buildGridSideRiskSnapshots() ([]gridSideRiskSnapshot, error) {
	if at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil, nil
	}
	gridConfig := at.config.StrategyConfig.GridConfig
	currentPrice, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return nil, err
	}
	investmentBase := at.currentGridInvestmentBase()
	balance, _ := at.trader.GetBalance()
	equity := investmentBase
	if totalEquity, ok := balance["total_equity"].(float64); ok && totalEquity > 0 {
		equity = totalEquity
	}

	snapshots := make([]gridSideRiskSnapshot, 0, 2)
	for _, side := range []string{"LONG", "SHORT"} {
		qty, entryPrice, found := at.getCurrentPositionSnapshot(gridConfig.Symbol, side)
		if !found || qty <= 0 || entryPrice <= 0 {
			continue
		}
		notional := qty * entryPrice
		unrealizedLoss := 0.0
		unrealizedLossPct := 0.0
		breakoutPct := 0.0
		switch side {
		case "LONG":
			unrealizedLoss = math.Max(0, (entryPrice-currentPrice)*qty)
			unrealizedLossPct = math.Max(0, (entryPrice-currentPrice)/entryPrice*100)
			if at.gridState.LowerPrice > 0 && currentPrice < at.gridState.LowerPrice {
				breakoutPct = (at.gridState.LowerPrice - currentPrice) / at.gridState.LowerPrice * 100
			}
		case "SHORT":
			unrealizedLoss = math.Max(0, (currentPrice-entryPrice)*qty)
			unrealizedLossPct = math.Max(0, (currentPrice-entryPrice)/entryPrice*100)
			if at.gridState.UpperPrice > 0 && currentPrice > at.gridState.UpperPrice {
				breakoutPct = (currentPrice - at.gridState.UpperPrice) / at.gridState.UpperPrice * 100
			}
		}
		eqLossPct := 0.0
		if equity > 0 {
			eqLossPct = unrealizedLoss / equity * 100
		}
		effectiveLeverage := 0.0
		if investmentBase > 0 {
			effectiveLeverage = notional / investmentBase
		}

		snapshot := gridSideRiskSnapshot{
			PositionSide:         side,
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			PositionQty:          qty,
			PositionNotional:     notional,
			UnrealizedLoss:       unrealizedLoss,
			UnrealizedLossPct:    unrealizedLossPct,
			UnrealizedLossEqPct:  eqLossPct,
			PositionPercent:      at.computeGridRiskPositionPercent(notional, investmentBase, gridConfig.Leverage),
			EffectiveLeverage:    effectiveLeverage,
			WorstLotLossPct:      at.getWorstLotLossPct(gridConfig.Symbol, side, currentPrice),
			BreakoutPct:          breakoutPct,
			EntryLocationPenalty: at.computeEntryLocationPenalty(side, entryPrice, at.gridState.ShortBoxLower, at.gridState.ShortBoxUpper),
			LiquidationPrice:     0,
			LiquidationDistance:  0,
			ShortBoxUpper:        at.gridState.ShortBoxUpper,
			ShortBoxLower:        at.gridState.ShortBoxLower,
			GridUpperPrice:       at.gridState.UpperPrice,
			GridLowerPrice:       at.gridState.LowerPrice,
			GridSpacing:          at.gridState.GridSpacing,
		}
		if currentPrice > 0 && gridConfig.Leverage > 0 {
			snapshot.LiquidationDistance = 100.0 / float64(gridConfig.Leverage) * 0.9
			if side == "LONG" {
				snapshot.LiquidationPrice = currentPrice * (1 - snapshot.LiquidationDistance/100)
			} else {
				snapshot.LiquidationPrice = currentPrice * (1 + snapshot.LiquidationDistance/100)
			}
		}
		at.determineGridRiskState(&snapshot)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func (at *AutoTrader) setGridRiskState(state, previous, side, reason string) {
	if at.gridState == nil {
		return
	}
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()
	at.gridState.PreviousRiskState = previous
	at.gridState.CurrentRiskState = state
	at.gridState.RiskStateSide = side
	at.gridState.RiskStateReason = reason
	at.gridState.RiskStateChangedAt = time.Now()
}

func (at *AutoTrader) clearGridRiskState() {
	if at.gridState == nil {
		return
	}
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()
	at.gridState.PreviousRiskState = at.gridState.CurrentRiskState
	at.gridState.CurrentRiskState = gridRiskStateNormal
	at.gridState.RiskStateSide = ""
	at.gridState.RiskStateReason = ""
	at.gridState.RiskStateChangedAt = time.Now()
}

func (at *AutoTrader) logGridRiskEvent(eventType string, snapshot *gridSideRiskSnapshot, previousState string, reducedQty, executionPrice float64, metadata map[string]interface{}) {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil || snapshot == nil {
		return
	}
	remainingQty := snapshot.PositionQty - reducedQty
	if remainingQty < 0 {
		remainingQty = 0
	}
	reducedNotional := reducedQty * snapshot.CurrentPrice
	if executionPrice > 0 {
		reducedNotional = reducedQty * executionPrice
	}
	_ = at.store.Grid().LogRiskEvent(&store.GridRiskEvent{
		TraderID:            at.id,
		Symbol:              at.config.StrategyConfig.GridConfig.Symbol,
		RiskState:           snapshot.RiskState,
		PreviousRiskState:   previousState,
		PositionSide:        snapshot.PositionSide,
		EventType:           eventType,
		Reason:              snapshot.Reason,
		TriggerPrice:        snapshot.CurrentPrice,
		ExecutionPrice:      executionPrice,
		EntryPrice:          snapshot.EntryPrice,
		PositionQty:         snapshot.PositionQty,
		ReducedQty:          reducedQty,
		RemainingQty:        remainingQty,
		PositionNotional:    snapshot.PositionNotional,
		ReducedNotional:     reducedNotional,
		UnrealizedLoss:      snapshot.UnrealizedLoss,
		UnrealizedLossPct:   snapshot.UnrealizedLossPct,
		UnrealizedLossEqPct: snapshot.UnrealizedLossEqPct,
		PositionPercent:     snapshot.PositionPercent,
		EffectiveLeverage:   snapshot.EffectiveLeverage,
		LiquidationPrice:    snapshot.LiquidationPrice,
		LiquidationDistance: snapshot.LiquidationDistance,
		ShortBoxUpper:       snapshot.ShortBoxUpper,
		ShortBoxLower:       snapshot.ShortBoxLower,
		GridUpperPrice:      snapshot.GridUpperPrice,
		GridLowerPrice:      snapshot.GridLowerPrice,
		GridSpacing:         snapshot.GridSpacing,
		Metadata:            metadata,
		CreatedAt:           time.Now(),
	})
}

func (at *AutoTrader) executeGridRiskReduction(snapshot *gridSideRiskSnapshot, reducePct float64, eventType string) error {
	if snapshot == nil || reducePct <= 0 {
		return nil
	}
	gridConfig := at.config.StrategyConfig.GridConfig
	reduceQty := snapshot.PositionQty * reducePct
	reduceQty = at.normalizeExecutableQuantity(gridConfig.Symbol, reduceQty)
	if reduceQty <= 0 {
		return nil
	}

	if eventType == "hard_reduce" || eventType == "emergency_reduce" {
		if err := at.cancelAllGridOrders(); err != nil {
			return err
		}
	} else {
		if err := at.cancelGridEntryOrders("risk_" + snapshot.RiskState); err != nil {
			return err
		}
	}

	var err error
	switch snapshot.PositionSide {
	case "LONG":
		_, err = at.trader.CloseLong(gridConfig.Symbol, reduceQty)
	case "SHORT":
		_, err = at.trader.CloseShort(gridConfig.Symbol, reduceQty)
	default:
		return fmt.Errorf("unsupported risk reduction side %s", snapshot.PositionSide)
	}
	if err != nil {
		return err
	}

	at.clearAggregatedRiskExitState(snapshot.PositionSide)
	remainingQty := at.waitForRiskPositionRefresh(gridConfig.Symbol, snapshot.PositionSide, snapshot.PositionQty, reduceQty, eventType == "emergency_reduce")
	if remainingQty > 0 && eventType != "emergency_reduce" {
		exitSide := "SELL"
		exitPrice := snapshot.CurrentPrice + math.Max(snapshot.GridSpacing*0.25, snapshot.CurrentPrice*0.001)
		if snapshot.PositionSide == "SHORT" {
			exitSide = "BUY"
			exitPrice = snapshot.CurrentPrice - math.Max(snapshot.GridSpacing*0.25, snapshot.CurrentPrice*0.001)
		}
		if formatter, ok := at.trader.(interface {
			FormatPrice(symbol string, price float64) (string, error)
		}); ok {
			if formatted, ferr := formatter.FormatPrice(gridConfig.Symbol, exitPrice); ferr == nil && formatted != "" {
				if normalized, perr := strconv.ParseFloat(formatted, 64); perr == nil && normalized > 0 {
					exitPrice = normalized
				}
			}
		}
		for attempt := 0; attempt < 3; attempt++ {
			if orderID, placeErr := at.ensureStandaloneReduceOnlyOrder(exitSide, snapshot.PositionSide, remainingQty, exitPrice); placeErr == nil && orderID != "" {
				at.setAggregatedRiskExitState(snapshot.PositionSide, true, exitPrice)
				if at.store != nil {
					_ = at.store.Grid().UpdateOpenInventoryLotsExitIntentBySide(at.id, gridConfig.Symbol, snapshot.PositionSide, -1, orderID)
				}
				break
			} else if placeErr != nil {
				logger.Warnf("%s Failed to seed post-risk reduce-only exit for %s: %v", at.gridLogPrefix(), snapshot.PositionSide, placeErr)
				break
			}
			time.Sleep(500 * time.Millisecond)
			remainingQty = at.waitForRiskPositionRefresh(gridConfig.Symbol, snapshot.PositionSide, snapshot.PositionQty, reduceQty, false)
			if remainingQty <= 0 {
				break
			}
		}
	}

	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.noteGridPaused("risk_" + snapshot.RiskState)
	at.gridState.LastRiskActionAt = time.Now()
	at.gridState.mu.Unlock()

	at.logGridRiskEvent(eventType, snapshot, at.gridState.PreviousRiskState, reduceQty, snapshot.CurrentPrice, map[string]interface{}{
		"reduce_pct":                 reducePct,
		"remaining_qty_after_action": remainingQty,
	})
	logger.Warnf("%s Executed %s for %s at current=$%s qty=%.4f (state=%s reason=%s)",
		at.gridLogPrefix(), eventType, snapshot.PositionSide, formatGridLogPrice(snapshot.CurrentPrice), reduceQty, snapshot.RiskState, snapshot.Reason)
	return nil
}

func (at *AutoTrader) waitForRiskPositionRefresh(symbol string, positionSide string, beforeQty float64, reducedQty float64, expectFlat bool) float64 {
	expectedQty := beforeQty - reducedQty
	if expectedQty < 0 {
		expectedQty = 0
	}
	lastQty := at.getCurrentPositionQuantity(symbol, positionSide)
	for attempt := 0; attempt < 6; attempt++ {
		if expectFlat {
			if lastQty <= 0 {
				return 0
			}
		} else {
			if lastQty > 0 && lastQty <= beforeQty && math.Abs(lastQty-expectedQty) <= math.Max(1, expectedQty*0.1) {
				return lastQty
			}
			if lastQty > 0 && lastQty < beforeQty {
				return lastQty
			}
		}
		time.Sleep(500 * time.Millisecond)
		lastQty = at.getCurrentPositionQuantity(symbol, positionSide)
	}
	return lastQty
}

func (at *AutoTrader) evaluateAndExecuteGridStopLoss() {
	snapshots, err := at.buildGridSideRiskSnapshots()
	if err != nil {
		logger.Warnf("%s Failed to build stop-loss risk snapshots: %v", at.gridLogPrefix(), err)
		return
	}
	if len(snapshots) == 0 {
		if at.gridState != nil && at.gridState.CurrentRiskState != "" && at.gridState.CurrentRiskState != gridRiskStateNormal {
			prev := at.gridState.CurrentRiskState
			at.clearGridRiskState()
			at.logGridRiskEvent("risk_recovered", &gridSideRiskSnapshot{
				RiskState:    gridRiskStateNormal,
				PositionSide: "",
			}, prev, 0, 0, nil)
		}
		return
	}

	selected := snapshots[0]
	for _, snapshot := range snapshots[1:] {
		if gridRiskSeverity(snapshot.RiskState) > gridRiskSeverity(selected.RiskState) ||
			(gridRiskSeverity(snapshot.RiskState) == gridRiskSeverity(selected.RiskState) && snapshot.UnrealizedLossPct > selected.UnrealizedLossPct) {
			selected = snapshot
		}
	}

	currentState := gridRiskStateNormal
	if at.gridState != nil && at.gridState.CurrentRiskState != "" {
		currentState = at.gridState.CurrentRiskState
	}
	if selected.RiskState != currentState || selected.PositionSide != at.gridState.RiskStateSide || selected.Reason != at.gridState.RiskStateReason {
		at.setGridRiskState(selected.RiskState, currentState, selected.PositionSide, selected.Reason)
		at.logGridRiskEvent("state_transition", &selected, currentState, 0, 0, map[string]interface{}{
			"warning_threshold":     selected.WarningThreshold,
			"soft_reduce_threshold": selected.SoftReduceThreshold,
			"hard_reduce_threshold": selected.HardReduceThreshold,
			"worst_lot_loss_pct":    selected.WorstLotLossPct,
			"breakout_pct":          selected.BreakoutPct,
		})
	}

	if gridRiskSeverity(selected.RiskState) <= gridRiskSeverity(currentState) {
		return
	}

	switch selected.RiskState {
	case gridRiskStateSoftReduce:
		_ = at.executeGridRiskReduction(&selected, 0.30, "soft_reduce")
	case gridRiskStateHardReduce:
		reducePct := 0.50
		if currentState == gridRiskStateSoftReduce {
			reducePct = 0.40
		}
		_ = at.executeGridRiskReduction(&selected, reducePct, "hard_reduce")
	case gridRiskStateEmergency:
		_ = at.executeGridRiskReduction(&selected, 1.0, "emergency_reduce")
	}
}

func (at *AutoTrader) getGridRiskHistory(limit int) []GridRiskHistoryItem {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil
	}
	events, err := at.store.Grid().GetRiskEvents(at.id, at.config.StrategyConfig.GridConfig.Symbol, limit)
	if err != nil {
		return nil
	}
	history := make([]GridRiskHistoryItem, 0, len(events))
	for _, event := range events {
		history = append(history, GridRiskHistoryItem{
			ID:                  event.ID,
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
			CreatedAt:           event.CreatedAt.UnixMilli(),
			Metadata:            event.Metadata,
		})
	}
	return history
}

func (at *AutoTrader) latestGridRiskHistoryItem() *GridRiskHistoryItem {
	history := at.getGridRiskHistory(1)
	if len(history) == 0 {
		return nil
	}
	return &history[0]
}

func (at *AutoTrader) gridRiskHistoryPayload(limit int) []GridRiskHistoryItem {
	history := at.getGridRiskHistory(limit)
	if len(history) == 0 {
		return []GridRiskHistoryItem{}
	}
	return history
}

func (at *AutoTrader) GetGridRiskHistory(limit int) []GridRiskHistoryItem {
	return at.gridRiskHistoryPayload(limit)
}
