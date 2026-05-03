package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"strings"
	"time"
)

// ============================================================================
// Regime Detection and Strategy Switching
// ============================================================================

// checkBoxBreakout checks for multi-period box breakouts and takes appropriate action
func (at *AutoTrader) checkBoxBreakout() error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	// Get box data
	box, err := market.GetBoxData(gridConfig.Symbol)
	if err != nil {
		logger.Infof("Failed to get box data: %v", err)
		return nil // Non-fatal, continue with other checks
	}

	// Update grid state with box values
	at.gridState.mu.Lock()
	at.gridState.ShortBoxUpper = box.ShortUpper
	at.gridState.ShortBoxLower = box.ShortLower
	at.gridState.MidBoxUpper = box.MidUpper
	at.gridState.MidBoxLower = box.MidLower
	at.gridState.LongBoxUpper = box.LongUpper
	at.gridState.LongBoxLower = box.LongLower
	at.gridState.mu.Unlock()

	// Detect breakout
	breakoutLevel, direction := detectBoxBreakout(box)

	// Get current breakout state
	state := &BreakoutState{
		Level:        market.BreakoutLevel(at.gridState.BreakoutLevel),
		Direction:    at.gridState.BreakoutDirection,
		ConfirmCount: at.gridState.BreakoutConfirmCount,
	}

	// Check if breakout is confirmed (3 candles)
	confirmed := confirmBreakout(state, breakoutLevel, direction)

	// Update grid state
	at.gridState.mu.Lock()
	at.gridState.BreakoutLevel = string(state.Level)
	at.gridState.BreakoutDirection = state.Direction
	at.gridState.BreakoutConfirmCount = state.ConfirmCount
	at.gridState.mu.Unlock()

	if !confirmed {
		return nil
	}

	// Take action based on breakout level
	// Use direction-aware action if enabled
	enableDirectionAdjust := gridConfig.EnableDirectionAdjust
	action := getBreakoutActionWithDirection(breakoutLevel, enableDirectionAdjust)

	// If direction adjustment action, determine the new direction
	if action == BreakoutActionAdjustDirection {
		box, _ := market.GetBoxData(gridConfig.Symbol)
		newDirection := determineGridDirection(box, at.gridState.CurrentDirection, breakoutLevel, direction)
		return at.executeDirectionAdjustment(newDirection)
	}

	return at.executeBreakoutAction(action)
}

// executeBreakoutAction executes the appropriate action for a breakout
func (at *AutoTrader) executeBreakoutAction(action BreakoutAction) error {
	switch action {
	case BreakoutActionReducePosition:
		// Short box breakout: reduce position to 50%
		logger.Infof("Short box breakout confirmed, reducing position to 50%%")
		at.gridState.mu.Lock()
		at.gridState.PositionReductionPct = 50
		at.gridState.mu.Unlock()
		return nil

	case BreakoutActionPauseGrid:
		// Mid box breakout: pause grid + cancel entry orders
		logger.Infof("Mid box breakout confirmed, pausing grid and canceling entry orders")
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.noteGridPaused("mid_box_breakout_pause")
		at.gridState.mu.Unlock()
		return at.cancelGridEntryOrders("mid_box_breakout_pause")

	case BreakoutActionCloseAll:
		// Long box breakout: pause + cancel + close all
		logger.Infof("Long box breakout confirmed, closing all positions")
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.noteGridPaused("long_box_breakout_close_all")
		at.gridState.mu.Unlock()
		if err := at.cancelAllGridOrders(); err != nil {
			logger.Infof("Failed to cancel orders: %v", err)
		}
		return at.closeAllPositions()

	case BreakoutActionAdjustDirection:
		// Direction adjustment is handled separately via executeDirectionAdjustment
		// This case should not be reached, but handle gracefully
		logger.Infof("Direction adjustment action received via executeBreakoutAction")
		return nil
	}

	return nil
}

// executeDirectionAdjustment handles grid direction changes based on box breakout
func (at *AutoTrader) executeDirectionAdjustment(newDirection market.GridDirection) error {
	at.gridState.mu.RLock()
	oldDirection := at.gridState.CurrentDirection
	at.gridState.mu.RUnlock()

	if oldDirection == newDirection {
		return nil // No change needed
	}

	logger.Infof("[Grid] Direction adjustment: %s -> %s", oldDirection, newDirection)

	// Repriced grid levels should only reset entry coverage; keep reduce-only
	// exits alive so existing positions retain their close path.
	if err := at.cancelGridEntryOrders("direction_adjustment"); err != nil {
		logger.Warnf("[Grid] Failed to cancel entry orders during direction adjustment: %v", err)
	}

	// Apply the new direction
	return at.adjustGridDirection(newDirection)
}

// adjustGridDirection handles runtime direction adjustment when breakout is detected
func (at *AutoTrader) adjustGridDirection(newDirection market.GridDirection) error {
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	oldDirection := at.gridState.CurrentDirection
	if oldDirection == newDirection {
		return nil // No change needed
	}

	at.gridState.CurrentDirection = newDirection
	at.gridState.DirectionChangedAt = time.Now()
	at.gridState.DirectionChangeCount++

	logger.Infof("[Grid] Direction changed: %s -> %s (change count: %d)",
		oldDirection, newDirection, at.gridState.DirectionChangeCount)

	// Get current price for recalculation
	currentPrice, err := at.trader.GetMarketPrice(at.gridState.Config.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	// Reapply direction to grid levels
	at.applyGridDirection(currentPrice)

	return nil
}

// checkFalseBreakoutRecovery checks if price has returned to box after breakout
func (at *AutoTrader) checkFalseBreakoutRecovery() error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	at.gridState.mu.RLock()
	breakoutLevel := at.gridState.BreakoutLevel
	isPaused := at.gridState.IsPaused
	positionReduction := at.gridState.PositionReductionPct
	currentDirection := at.gridState.CurrentDirection
	at.gridState.mu.RUnlock()

	// Only check if we had a breakout or non-neutral direction
	needsRecoveryCheck := breakoutLevel != string(market.BreakoutNone) ||
		positionReduction != 0 ||
		isPaused ||
		(gridConfig.EnableDirectionAdjust && currentDirection != market.GridDirectionNeutral)

	if !needsRecoveryCheck {
		return nil
	}

	// Get current box data
	box, err := market.GetBoxData(gridConfig.Symbol)
	if err != nil {
		return nil
	}

	// Only resume from a paused breakout after cooldown + repeated ranging confirmations.
	recoveredToBox := false
	ctx, ctxErr := at.buildGridContext()
	canResume := ctxErr == nil && at.qualifiesForGridResume(ctx)
	if canResume {
		at.gridState.mu.Lock()
		at.gridState.ResumeConfirmCount++
		confirmCount := at.gridState.ResumeConfirmCount
		at.gridState.mu.Unlock()
		logger.Infof("[Grid] Resume confirmation %d/%d after pause", confirmCount, gridPauseResumeConfirmations)
		if confirmCount >= gridPauseResumeConfirmations {
			logger.Infof("Price returned to stable ranging conditions, rebuilding grid after pause")
			at.gridState.mu.Lock()
			at.gridState.BreakoutLevel = string(market.BreakoutNone)
			at.gridState.BreakoutDirection = ""
			at.gridState.BreakoutConfirmCount = 0
			at.gridState.PositionReductionPct = 50 // Recover at 50%
			at.gridState.IsPaused = false
			at.gridState.ResumedAt = time.Now()
			at.gridState.PauseReason = ""
			at.gridState.ResumeConfirmCount = 0
			at.gridState.RecoverySeedWaves = 0
			at.gridState.mu.Unlock()
			recoveredToBox = true
		}
	} else if isPaused {
		at.gridState.mu.Lock()
		at.gridState.ResumeConfirmCount = 0
		at.gridState.mu.Unlock()
	}

	// Check for direction recovery toward neutral (if direction adjustment is enabled)
	if gridConfig.EnableDirectionAdjust && currentDirection != market.GridDirectionNeutral {
		if shouldRecoverDirection(box, currentDirection) {
			newDirection := determineRecoveryDirection(box.CurrentPrice, box, currentDirection)
			if newDirection != currentDirection {
				logger.Infof("[Grid] Direction recovery: %s -> %s (price back in short box)",
					currentDirection, newDirection)
				at.adjustGridDirection(newDirection)
			}
		}
	}

	if recoveredToBox {
		if err := at.rebuildGridAfterBoxRecovery(box.CurrentPrice); err != nil {
			logger.Warnf("[Grid] Failed to rebuild grid after box recovery: %v", err)
		}
	}

	return nil
}

func (at *AutoTrader) rebuildGridAfterBoxRecovery(currentPrice float64) error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	at.gridState.mu.Lock()
	at.initializeGridLevels(currentPrice, gridConfig)
	at.gridState.OrderBook = make(map[string]int)
	at.gridState.mu.Unlock()

	at.bootstrapGridStateFromExchange()
	at.ensurePairedExitOrdersForAllFilledLevels()

	ctx, err := at.buildGridContext()
	if err != nil {
		return err
	}
	if err := at.autoSeedMissingEntryOrdersForRangingGrid(ctx); err != nil {
		return err
	}

	logger.Infof("[Grid] Rebuilt grid after price returned to box at $%.6f", currentPrice)
	return nil
}

// GetGridRiskInfo returns current risk information for frontend display
func (at *AutoTrader) GetGridRiskInfo() *GridRiskInfo {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return &GridRiskInfo{}
	}

	at.gridState.mu.RLock()
	levels := append([]kernel.GridLevelInfo(nil), at.gridState.Levels...)
	orderBook := make(map[string]int, len(at.gridState.OrderBook))
	for orderID, levelIdx := range at.gridState.OrderBook {
		orderBook[orderID] = levelIdx
	}
	currentRegimeLevel := at.gridState.CurrentRegimeLevel
	shortBoxUpper := at.gridState.ShortBoxUpper
	shortBoxLower := at.gridState.ShortBoxLower
	midBoxUpper := at.gridState.MidBoxUpper
	midBoxLower := at.gridState.MidBoxLower
	longBoxUpper := at.gridState.LongBoxUpper
	longBoxLower := at.gridState.LongBoxLower
	gridUpperPrice := at.gridState.UpperPrice
	gridLowerPrice := at.gridState.LowerPrice
	gridBoundarySource := at.gridState.BoundSource
	gridSpacing := at.gridState.GridSpacing
	breakoutLevel := at.gridState.BreakoutLevel
	breakoutDirection := at.gridState.BreakoutDirection
	currentGridDirection := at.gridState.CurrentDirection
	directionChangeCount := at.gridState.DirectionChangeCount
	currentRiskState := at.gridState.CurrentRiskState
	previousRiskState := at.gridState.PreviousRiskState
	currentRiskReason := at.gridState.RiskStateReason
	currentRiskSide := at.gridState.RiskStateSide
	riskStateChangedAt := at.gridState.RiskStateChangedAt.UnixMilli()
	at.gridState.mu.RUnlock()

	// Get current price
	currentPrice, _ := at.trader.GetMarketPrice(gridConfig.Symbol)

	// Calculate effective leverage against the live compounding capital base.
	totalInvestment := at.currentGridInvestmentBase()
	if totalInvestment <= 0 {
		totalInvestment = gridConfig.TotalInvestment
	}
	leverage := gridConfig.Leverage

	// Get current position value
	positions, _ := at.trader.GetPositions()
	openOrders, _ := at.trader.GetOpenOrders(gridConfig.Symbol)
	var currentPositionValue float64
	var currentPositionSize float64
	var currentLongPosition float64
	var currentShortPosition float64
	var currentLongValue float64
	var currentShortValue float64
	for _, pos := range positions {
		if sym, _ := pos["symbol"].(string); sym == gridConfig.Symbol {
			size, _ := pos["positionAmt"].(float64)
			entry, _ := pos["entryPrice"].(float64)
			side, _ := pos["side"].(string)
			value := math.Abs(size * entry)
			currentPositionValue += value
			currentPositionSize += size
			if normalizeGridPositionSide(side, "") == "LONG" {
				currentLongPosition += math.Abs(size)
				currentLongValue += value
			} else {
				currentShortPosition += math.Abs(size)
				currentShortValue += value
			}
		}
	}

	var entryOrderCount int
	var reduceOnlyOrderCount int
	var longOrderCount int
	var shortOrderCount int
	var longFilledLevels int
	var shortFilledLevels int
	gridLevels := make([]GridRiskLevel, 0, len(levels))
	for _, level := range levels {
		gridLevels = append(gridLevels, GridRiskLevel{
			Index: level.Index,
			Price: level.Price,
		})
	}
	gridOrders := make([]GridRiskOrder, 0, len(openOrders))
	totalOrderNotional := 0.0
	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		isReduceOnly := (strings.EqualFold(order.Side, "SELL") && positionSide == "LONG") ||
			(strings.EqualFold(order.Side, "BUY") && positionSide == "SHORT")
		levelIndex := -1
		linkedLevelIndex := -1
		sourceEntryPrice := 0.0
		if idx, ok := orderBook[order.OrderID]; ok {
			levelIndex = idx
			if idx >= 0 && idx < len(levels) {
				linkedLevelIndex = levels[idx].LinkedLevelIndex
				if linkedLevelIndex >= 0 && linkedLevelIndex < len(levels) {
					sourceEntryPrice = levels[linkedLevelIndex].PositionEntry
				}
			}
		}
		orderPrice := order.Price
		if orderPrice <= 0 {
			orderPrice = order.StopPrice
		}
		notional := math.Abs(order.Quantity * orderPrice)
		totalOrderNotional += notional
		bucket := "short_entry"
		switch {
		case isReduceOnly && positionSide == "LONG":
			bucket = "long_exit"
		case isReduceOnly && positionSide == "SHORT":
			bucket = "short_exit"
		case !isReduceOnly && positionSide == "LONG":
			bucket = "long_entry"
		}
		gridOrders = append(gridOrders, GridRiskOrder{
			OrderID:          order.OrderID,
			Price:            orderPrice,
			Quantity:         order.Quantity,
			Notional:         notional,
			Side:             strings.ToUpper(order.Side),
			PositionSide:     positionSide,
			ReduceOnly:       isReduceOnly,
			LevelIndex:       levelIndex,
			LinkedLevelIndex: linkedLevelIndex,
			SourceEntryPrice: sourceEntryPrice,
			Bucket:           bucket,
			Status:           order.Status,
		})
		if isReduceOnly {
			reduceOnlyOrderCount++
		} else {
			entryOrderCount++
		}
		if positionSide == "LONG" {
			longOrderCount++
		} else {
			shortOrderCount++
		}
	}

	for _, level := range levels {
		if level.State == "filled" {
			if normalizeGridPositionSide(level.PositionSide, level.Side) == "LONG" {
				longFilledLevels++
			} else {
				shortFilledLevels++
			}
		}
	}

	if longFilledLevels == 0 && currentLongPosition > 0 {
		longFilledLevels = 1
	}
	if shortFilledLevels == 0 && currentShortPosition > 0 {
		shortFilledLevels = 1
	}

	effectiveLeverage := 0.0
	if totalInvestment > 0 {
		effectiveLeverage = currentPositionValue / totalInvestment
	}

	// Calculate max position based on regime
	regimeLevel := market.RegimeLevel(currentRegimeLevel)
	if regimeLevel == "" {
		regimeLevel = market.RegimeLevelStandard
	}

	// Use default position limit since GridStrategyConfig doesn't have regime-specific limits
	// Default is 70% for standard regime
	maxPositionPct := 70.0
	switch regimeLevel {
	case market.RegimeLevelNarrow:
		maxPositionPct = 40.0
	case market.RegimeLevelStandard:
		maxPositionPct = 70.0
	case market.RegimeLevelWide:
		maxPositionPct = 60.0
	case market.RegimeLevelVolatile:
		maxPositionPct = 40.0
	}

	maxPosition := totalInvestment * maxPositionPct / 100 * float64(leverage)

	// Use default leverage limits since GridStrategyConfig doesn't have regime-specific limits
	recommendedLeverage := leverage
	switch regimeLevel {
	case market.RegimeLevelNarrow:
		recommendedLeverage = min(leverage, 2)
	case market.RegimeLevelStandard:
		recommendedLeverage = min(leverage, 4)
	case market.RegimeLevelWide:
		recommendedLeverage = min(leverage, 3)
	case market.RegimeLevelVolatile:
		recommendedLeverage = min(leverage, 2)
	}

	// Calculate liquidation distance and price only when there's a position
	var liquidationDistance float64
	var liquidationPrice float64
	if currentPositionSize != 0 && currentPrice > 0 {
		liquidationDistance = 100.0 / float64(leverage) * 0.9 // ~90% of theoretical max
		if currentPositionSize > 0 {
			// Long position: liquidation below entry
			liquidationPrice = currentPrice * (1 - liquidationDistance/100)
		} else {
			// Short position: liquidation above entry
			liquidationPrice = currentPrice * (1 + liquidationDistance/100)
		}
	}

	positionPercent := 0.0
	if maxPosition > 0 {
		positionPercent = currentPositionValue / maxPosition * 100
	}

	averageOrderNotional := 0.0
	if len(gridOrders) > 0 {
		averageOrderNotional = totalOrderNotional / float64(len(gridOrders))
	}

	snapshots, _ := at.buildGridSideRiskSnapshots()
	longReferenceStopPrice := 0.0
	shortReferenceStopPrice := 0.0
	for _, snapshot := range snapshots {
		switch snapshot.PositionSide {
		case "LONG":
			if snapshot.EntryPrice > 0 && snapshot.HardReduceThreshold > 0 {
				longReferenceStopPrice = snapshot.EntryPrice * (1 - snapshot.HardReduceThreshold/100)
			}
		case "SHORT":
			if snapshot.EntryPrice > 0 && snapshot.HardReduceThreshold > 0 {
				shortReferenceStopPrice = snapshot.EntryPrice * (1 + snapshot.HardReduceThreshold/100)
			}
		}
	}

	thresholdPct, atrMultiplier, fixedBand, atrBand, _, activeSource := at.currentFirstEntryGuardConfig(currentPrice)
	shortFirstEntryGuardPrice, longFirstEntryGuardPrice, _, _, _, _, _ := at.currentFirstEntryGuardPrices(currentPrice)
	effectiveATR14 := at.latestGridATR14(currentPrice)

	history := at.gridRiskHistoryPayload(20)
	var latestRisk *GridRiskHistoryItem
	if len(history) > 0 {
		latestRisk = &history[0]
	}

	if currentRiskState == "" {
		currentRiskState = gridRiskStateNormal
	}

	stopLossTriggerPrice := 0.0
	stopLossExecutionPrice := 0.0
	stopLossEntryPrice := 0.0
	stopLossPositionQty := 0.0
	stopLossReducedQty := 0.0
	stopLossRemainingQty := 0.0
	stopLossPositionValue := 0.0
	stopLossReducedValue := 0.0
	stopLossUnrealizedLoss := 0.0
	stopLossUnrealizedLossPct := 0.0
	stopLossUnrealizedLossEqPct := 0.0
	if latestRisk != nil {
		stopLossTriggerPrice = latestRisk.TriggerPrice
		stopLossExecutionPrice = latestRisk.ExecutionPrice
		stopLossEntryPrice = latestRisk.EntryPrice
		stopLossPositionQty = latestRisk.PositionQty
		stopLossReducedQty = latestRisk.ReducedQty
		stopLossRemainingQty = latestRisk.RemainingQty
		stopLossPositionValue = latestRisk.PositionNotional
		stopLossReducedValue = latestRisk.ReducedNotional
		stopLossUnrealizedLoss = latestRisk.UnrealizedLoss
		stopLossUnrealizedLossPct = latestRisk.UnrealizedLossPct
		stopLossUnrealizedLossEqPct = latestRisk.UnrealizedLossEqPct
		if currentRiskReason == "" {
			currentRiskReason = latestRisk.Reason
		}
		if currentRiskSide == "" {
			currentRiskSide = latestRisk.PositionSide
		}
		if currentRiskState == gridRiskStateNormal && latestRisk.RiskState != "" {
			currentRiskState = latestRisk.RiskState
		}
		if riskStateChangedAt == 0 {
			riskStateChangedAt = latestRisk.CreatedAt
		}
	}

	return &GridRiskInfo{
		CurrentLeverage:     leverage,
		EffectiveLeverage:   effectiveLeverage,
		RecommendedLeverage: recommendedLeverage,
		CompoundingBase:     totalInvestment,

		CurrentPosition:      currentPositionValue,
		MaxPosition:          maxPosition,
		PositionPercent:      positionPercent,
		CurrentLongPosition:  currentLongPosition,
		CurrentShortPosition: currentShortPosition,
		CurrentLongValue:     currentLongValue,
		CurrentShortValue:    currentShortValue,
		EntryOrderCount:      entryOrderCount,
		ReduceOnlyOrderCount: reduceOnlyOrderCount,
		LongOrderCount:       longOrderCount,
		ShortOrderCount:      shortOrderCount,
		LongFilledLevels:     longFilledLevels,
		ShortFilledLevels:    shortFilledLevels,
		AverageOrderNotional: averageOrderNotional,

		LiquidationPrice:    liquidationPrice,
		LiquidationDistance: liquidationDistance,

		RegimeLevel: string(regimeLevel),

		ShortBoxUpper:      shortBoxUpper,
		ShortBoxLower:      shortBoxLower,
		MidBoxUpper:        midBoxUpper,
		MidBoxLower:        midBoxLower,
		LongBoxUpper:       longBoxUpper,
		LongBoxLower:       longBoxLower,
		CurrentPrice:       currentPrice,
		GridUpperPrice:     gridUpperPrice,
		GridLowerPrice:     gridLowerPrice,
		GridBoundarySource: gridBoundarySource,
		GridSpacing:        gridSpacing,
		MinGridSpacing:     currentPrice * minGridSpacingPct,
		MaxGridSpacing:     currentPrice * maxGridSpacingPct,

		BreakoutLevel:                breakoutLevel,
		BreakoutDirection:            breakoutDirection,
		FirstEntryGuardThresholdPct:  thresholdPct,
		FirstEntryGuardATRMultiplier: atrMultiplier,
		FirstEntryGuardFixedBand:     fixedBand,
		FirstEntryGuardATRBand:       atrBand,
		FirstEntryGuardActiveSource:  activeSource,
		ShortFirstEntryGuardPrice:    shortFirstEntryGuardPrice,
		LongFirstEntryGuardPrice:     longFirstEntryGuardPrice,
		EffectiveATR14:               effectiveATR14,
		LongReferenceStopPrice:       longReferenceStopPrice,
		ShortReferenceStopPrice:      shortReferenceStopPrice,

		CurrentGridDirection:        string(currentGridDirection),
		DirectionChangeCount:        directionChangeCount,
		EnableDirectionAdjust:       gridConfig.EnableDirectionAdjust,
		CurrentRiskState:            currentRiskState,
		PreviousRiskState:           previousRiskState,
		CurrentRiskReason:           currentRiskReason,
		CurrentRiskSide:             currentRiskSide,
		RiskStateChangedAt:          riskStateChangedAt,
		StopLossTriggerPrice:        stopLossTriggerPrice,
		StopLossExecutionPrice:      stopLossExecutionPrice,
		StopLossEntryPrice:          stopLossEntryPrice,
		StopLossPositionQty:         stopLossPositionQty,
		StopLossReducedQty:          stopLossReducedQty,
		StopLossRemainingQty:        stopLossRemainingQty,
		StopLossPositionValue:       stopLossPositionValue,
		StopLossReducedValue:        stopLossReducedValue,
		StopLossUnrealizedLoss:      stopLossUnrealizedLoss,
		StopLossUnrealizedLossPct:   stopLossUnrealizedLossPct,
		StopLossUnrealizedLossEqPct: stopLossUnrealizedLossEqPct,
		RiskHistory:                 history,
		GridLevels:                  gridLevels,
		GridOrders:                  gridOrders,
	}
}
