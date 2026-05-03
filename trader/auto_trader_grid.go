package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"sync"
	"time"
)

func formatGridLogPrice(price float64) string {
	switch {
	case price >= 1000:
		return fmt.Sprintf("%.2f", price)
	case price >= 1:
		return fmt.Sprintf("%.4f", price)
	case price >= 0.1:
		return fmt.Sprintf("%.5f", price)
	case price > 0:
		return fmt.Sprintf("%.6f", price)
	default:
		return fmt.Sprintf("%.2f", price)
	}
}

// ============================================================================
// Grid Trading State Management
// ============================================================================

// GridState holds the runtime state for grid trading
type GridState struct {
	mu sync.RWMutex

	// Configuration
	Config *store.GridStrategyConfig

	// Grid levels
	Levels []kernel.GridLevelInfo

	// Calculated bounds
	UpperPrice  float64
	LowerPrice  float64
	GridSpacing float64
	BoundSource string

	// State flags
	IsPaused           bool
	IsInitialized      bool
	PausedAt           time.Time
	ResumedAt          time.Time
	PauseReason        string
	ResumeConfirmCount int
	RecoverySeedWaves  int

	// Performance tracking
	TotalProfit    float64
	TotalTrades    int
	WinningTrades  int
	MaxDrawdown    float64
	PeakEquity     float64
	DailyPnL       float64
	LastDailyReset time.Time

	// Order tracking
	OrderBook                     map[string]int       // OrderID -> LevelIndex
	StandaloneReduceOnlyBook      map[string]float64   // side|positionSide|price -> qty
	RecentStandaloneReduceOnlyIDs map[string]time.Time // freshly seeded reduce-only order IDs protected from immediate cleanup

	// Box state
	ShortBoxUpper float64
	ShortBoxLower float64
	MidBoxUpper   float64
	MidBoxLower   float64
	LongBoxUpper  float64
	LongBoxLower  float64

	// Breakout state
	BreakoutLevel        string
	BreakoutDirection    string
	BreakoutConfirmCount int

	// Position reduction (0 = normal, 50 = reduced after false breakout)
	PositionReductionPct float64

	// Current regime level
	CurrentRegimeLevel string

	// Grid direction adjustment
	CurrentDirection     market.GridDirection
	DirectionChangedAt   time.Time
	DirectionChangeCount int

	// Trap mitigation: when one side stays trapped near a medium-box edge for
	// too long, switch that side from per-lot exits to a single aggregated
	// reduce-only exit.
	AggregatedRiskExitLongActive  bool
	AggregatedRiskExitLongPrice   float64
	AggregatedRiskExitShortActive bool
	AggregatedRiskExitShortPrice  float64

	// Risk stop-loss state machine.
	CurrentRiskState   string
	PreviousRiskState  string
	RiskStateReason    string
	RiskStateSide      string
	RiskStateChangedAt time.Time
	LastRiskActionAt   time.Time

	// Latest grid context snapshot fields reused by admission / dashboard logic.
	LastContextATR14   float64
	LastContextPrice   float64
	LastContextBuiltAt time.Time

	// Entry sizing policy version used to force a one-time entry reseed after
	// business-logic updates that change grid entry sizing or caps.
	EntrySizingPolicyVersion string
}

// NewGridState creates a new grid state
func NewGridState(config *store.GridStrategyConfig) *GridState {
	return &GridState{
		Config:                        config,
		Levels:                        make([]kernel.GridLevelInfo, 0),
		OrderBook:                     make(map[string]int),
		StandaloneReduceOnlyBook:      make(map[string]float64),
		RecentStandaloneReduceOnlyIDs: make(map[string]time.Time),
		CurrentDirection:              market.GridDirectionNeutral,
	}
}

// ============================================================================
// Breakout Detection (price vs grid boundary)
// ============================================================================

// BreakoutType represents the type of price breakout
type BreakoutType string

const (
	BreakoutNone  BreakoutType = "none"
	BreakoutUpper BreakoutType = "upper"
	BreakoutLower BreakoutType = "lower"
)

// checkBreakout detects if price has broken out of grid range
// Returns breakout type and percentage beyond boundary
func (at *AutoTrader) checkBreakout() (BreakoutType, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig

	currentPrice, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return BreakoutNone, 0
	}

	at.gridState.mu.RLock()
	upper := at.gridState.UpperPrice
	lower := at.gridState.LowerPrice
	at.gridState.mu.RUnlock()

	if upper <= 0 || lower <= 0 {
		return BreakoutNone, 0
	}

	// Check upper breakout
	if currentPrice > upper {
		breakoutPct := (currentPrice - upper) / upper * 100
		return BreakoutUpper, breakoutPct
	}

	// Check lower breakout
	if currentPrice < lower {
		breakoutPct := (lower - currentPrice) / lower * 100
		return BreakoutLower, breakoutPct
	}

	return BreakoutNone, 0
}

// checkMaxDrawdown checks if current drawdown exceeds maximum allowed
// Returns: (exceeded bool, currentDrawdown float64)
func (at *AutoTrader) checkMaxDrawdown() (bool, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.MaxDrawdownPct <= 0 {
		return false, 0
	}

	// Get current equity
	balance, err := at.trader.GetBalance()
	if err != nil {
		return false, 0
	}

	currentEquity := 0.0
	if equity, ok := balance["total_equity"].(float64); ok {
		currentEquity = equity
	} else if total, ok := balance["totalWalletBalance"].(float64); ok {
		if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
			currentEquity = total + unrealized
		}
	}

	if currentEquity <= 0 {
		return false, 0
	}

	// Update peak equity
	at.gridState.mu.Lock()
	if currentEquity > at.gridState.PeakEquity {
		at.gridState.PeakEquity = currentEquity
	}
	peakEquity := at.gridState.PeakEquity
	at.gridState.mu.Unlock()

	if peakEquity <= 0 {
		return false, 0
	}

	// Calculate current drawdown
	drawdown := (peakEquity - currentEquity) / peakEquity * 100

	// Update max drawdown tracking
	at.gridState.mu.Lock()
	if drawdown > at.gridState.MaxDrawdown {
		at.gridState.MaxDrawdown = drawdown
	}
	at.gridState.mu.Unlock()

	return drawdown >= gridConfig.MaxDrawdownPct, drawdown
}

// checkDailyLossLimit checks if daily loss exceeds limit
// Returns: (exceeded bool, dailyLossPct float64)
func (at *AutoTrader) checkDailyLossLimit() (bool, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.DailyLossLimitPct <= 0 {
		return false, 0
	}

	at.gridState.mu.Lock()
	// Reset daily PnL if new day
	now := time.Now()
	if now.YearDay() != at.gridState.LastDailyReset.YearDay() ||
		now.Year() != at.gridState.LastDailyReset.Year() {
		at.gridState.DailyPnL = 0
		at.gridState.LastDailyReset = now
	}
	dailyPnL := at.gridState.DailyPnL
	at.gridState.mu.Unlock()

	// Calculate daily loss as percentage of total investment
	dailyLossPct := 0.0
	if gridConfig.TotalInvestment > 0 && dailyPnL < 0 {
		dailyLossPct = (-dailyPnL) / gridConfig.TotalInvestment * 100
	}

	return dailyLossPct >= gridConfig.DailyLossLimitPct, dailyLossPct
}

// updateDailyPnL updates the daily PnL tracking
func (at *AutoTrader) updateDailyPnL(realizedPnL float64) {
	at.gridState.mu.Lock()
	at.gridState.DailyPnL += realizedPnL
	at.gridState.TotalProfit += realizedPnL
	at.gridState.mu.Unlock()
}

// emergencyExit closes all positions and cancels all orders
func (at *AutoTrader) emergencyExit(reason string) error {
	gridConfig := at.config.StrategyConfig.GridConfig

	logger.Errorf("[Grid] EMERGENCY EXIT: %s", reason)

	positions, err := at.trader.GetPositions()
	if err == nil {
		armedAny := false
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); !ok || sym != gridConfig.Symbol {
				continue
			}
			size, ok := pos["positionAmt"].(float64)
			if !ok || size == 0 {
				continue
			}
			positionSide, _ := pos["side"].(string)
			normalizedSide := normalizeGridPositionSide(positionSide, "")
			if normalizedSide == "" {
				if size > 0 {
					normalizedSide = "LONG"
				} else {
					normalizedSide = "SHORT"
				}
			}
			if activateErr := at.activateAggregatedRiskExit(normalizedSide, reason); activateErr != nil {
				logger.Warnf("[Grid] Failed to arm aggregated risk exit for %s during emergency exit: %v", normalizedSide, activateErr)
				continue
			}
			armedAny = true
		}
		if armedAny {
			at.gridState.mu.Lock()
			at.gridState.IsPaused = true
			at.noteGridPaused(reason)
			at.gridState.mu.Unlock()
			return nil
		}
	}

	// Fallback only if aggregated risk exit could not be armed.
	if err := at.cancelAllGridOrders(); err != nil {
		logger.Errorf("[Grid] Failed to cancel orders in emergency fallback: %v", err)
	}
	if positions != nil {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == gridConfig.Symbol {
				if size, ok := pos["positionAmt"].(float64); ok && size != 0 {
					if size > 0 {
						at.trader.CloseLong(gridConfig.Symbol, size)
					} else {
						at.trader.CloseShort(gridConfig.Symbol, -size)
					}
				}
			}
		}
	}

	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.noteGridPaused(reason)
	at.gridState.mu.Unlock()

	return nil
}

// handleBreakout handles price breakout from grid range
func (at *AutoTrader) handleBreakout(breakoutType BreakoutType, breakoutPct float64) error {
	logger.Warnf("[Grid] BREAKOUT DETECTED: %s, %.2f%% beyond boundary", breakoutType, breakoutPct)

	// If breakout exceeds 2%, pause grid and cancel entry orders
	if breakoutPct >= 2.0 {
		logger.Warnf("[Grid] Significant breakout (%.2f%%), pausing grid and canceling entry orders", breakoutPct)

		// Cancel only entry orders to prevent new exposure while preserving reduce-only exits
		if err := at.cancelGridEntryOrders("price_breakout_pause"); err != nil {
			logger.Errorf("[Grid] Failed to cancel entry orders on breakout: %v", err)
		}

		// Pause grid trading
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.noteGridPaused(fmt.Sprintf("%s_breakout %.2f%%", breakoutType, breakoutPct))
		at.gridState.mu.Unlock()

		return fmt.Errorf("grid paused due to %s breakout (%.2f%%)", breakoutType, breakoutPct)
	}

	// If breakout is minor (< 2%), consider adjusting grid
	if breakoutPct >= 1.0 {
		logger.Infof("[Grid] Minor breakout (%.2f%%), considering grid adjustment", breakoutPct)
		// Let AI decide whether to adjust
	}

	return nil
}

// ============================================================================
// AutoTrader Grid Lifecycle
// ============================================================================

// InitializeGrid initializes the grid state and calculates levels
func (at *AutoTrader) InitializeGrid() error {
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return fmt.Errorf("grid configuration not found")
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	at.gridState = NewGridState(gridConfig)

	// Legacy exchange-level stop-loss / take-profit algo orders can conflict
	// with grid exits by force-closing the full side at market. Clear them when
	// a grid trader starts so only grid-managed exits remain active.
	if err := at.trader.CancelStopOrders(gridConfig.Symbol); err != nil {
		logger.Warnf("[Grid] Failed to clear legacy exchange stop/take-profit orders for %s during initialization: %v", gridConfig.Symbol, err)
	} else {
		logger.Infof("[Grid] Cleared legacy exchange stop/take-profit orders for %s during initialization", gridConfig.Symbol)
	}

	// Get current market price
	price, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	// Calculate grid bounds
	if gridConfig.UpperPrice > 0 && gridConfig.LowerPrice > 0 {
		at.gridState.UpperPrice = gridConfig.UpperPrice
		at.gridState.LowerPrice = gridConfig.LowerPrice
		at.gridState.BoundSource = "manual"
	} else if err := at.calculatePreferredGridBounds(price, gridConfig); err != nil {
		logger.Warnf("Failed to calculate preferred grid bounds: %v, using default bounds", err)
		at.calculateDefaultBounds(price, gridConfig)
	}

	// Keep grid spacing inside a practical operating range even when the box is too wide or too narrow.
	at.enforceGridSpacingBounds(price, gridConfig)

	// Calculate grid spacing
	at.gridState.GridSpacing = (at.gridState.UpperPrice - at.gridState.LowerPrice) / float64(gridConfig.GridCount-1)

	// Initialize grid levels
	at.initializeGridLevels(price, gridConfig)
	at.bootstrapGridStateFromExchange()

	at.gridState.IsInitialized = true

	// CRITICAL: Set leverage on exchange before trading
	if err := at.trader.SetLeverage(gridConfig.Symbol, gridConfig.Leverage); err != nil {
		logger.Warnf("[Grid] Failed to set leverage %dx on exchange: %v", gridConfig.Leverage, err)
		// Not fatal - continue with default leverage
	} else {
		logger.Infof("[Grid] Leverage set to %dx for %s", gridConfig.Leverage, gridConfig.Symbol)
	}

	logger.Infof("[Grid] Initialized: %d levels, $%s - $%s, spacing $%s",
		gridConfig.GridCount,
		formatGridLogPrice(at.gridState.LowerPrice),
		formatGridLogPrice(at.gridState.UpperPrice),
		formatGridLogPrice(at.gridState.GridSpacing))

	return nil
}

// RunGridCycle executes one grid trading cycle
func (at *AutoTrader) RunGridCycle() error {
	// Check if trader is stopped (early exit to prevent trades after Stop() is called)
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("[Grid] Trader is stopped, aborting grid cycle")
		return nil
	}

	if at.gridState == nil || !at.gridState.IsInitialized {
		if err := at.InitializeGrid(); err != nil {
			return fmt.Errorf("failed to initialize grid: %w", err)
		}
	}

	// CRITICAL: Check for breakout before executing any trades
	breakoutType, breakoutPct := at.checkBreakout()
	if breakoutType != BreakoutNone {
		if err := at.handleBreakout(breakoutType, breakoutPct); err != nil {
			return err // Grid paused due to breakout
		}
	}

	// CRITICAL: Check max drawdown
	exceeded, drawdown := at.checkMaxDrawdown()
	if exceeded {
		return at.emergencyExit(fmt.Sprintf("max drawdown exceeded: %.2f%%", drawdown))
	}

	// CRITICAL: Check daily loss limit
	dailyExceeded, dailyLossPct := at.checkDailyLossLimit()
	if dailyExceeded {
		logger.Errorf("[Grid] Daily loss limit exceeded: %.2f%%", dailyLossPct)
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.noteGridPaused(fmt.Sprintf("daily_loss_limit %.2f%%", dailyLossPct))
		at.gridState.mu.Unlock()
		return fmt.Errorf("daily loss limit exceeded: %.2f%%", dailyLossPct)
	}

	// Check multi-period box breakout
	if err := at.checkBoxBreakout(); err != nil {
		logger.Infof("Box breakout check error: %v", err)
	}

	// Check for false breakout recovery
	if err := at.checkFalseBreakoutRecovery(); err != nil {
		logger.Infof("False breakout recovery check error: %v", err)
	}

	at.enforceRecentPauseGuard()

	if preCtx, err := at.buildGridContext(); err != nil {
		logger.Warnf("%s Failed to build context for trapped-inventory mitigation: %v", at.gridLogPrefix(), err)
	} else if triggered, err := at.checkAndMitigateTrappedInventoryRisk(preCtx); err != nil {
		logger.Warnf("%s Failed to evaluate trapped-inventory mitigation: %v", at.gridLogPrefix(), err)
	} else if triggered {
		logger.Warnf("%s Triggered trapped-inventory mitigation, switching affected side(s) to aggregated exits", at.gridLogPrefix())
	}

	// Check if grid is paused
	at.gridState.mu.RLock()
	isPaused := at.gridState.IsPaused
	pauseReason := at.gridState.PauseReason
	at.gridState.mu.RUnlock()
	if isPaused {
		if err := at.cancelGridEntryOrders("paused_cycle_cleanup"); err != nil {
			logger.Warnf("%s Failed to cancel lingering entry orders while paused: %v", at.gridLogPrefix(), err)
		}
		at.syncGridState()
		at.ensurePairedExitOrdersForAllFilledLevels()
		logger.Infof("[Grid] Grid is paused, synced state and preserved exit orders")
		pausedDecision := at.buildPausedAIDecision()
		pausedReason := at.buildPausedDecisionReason(pauseReason)
		pausedDetails := at.buildPausedExecutionDetails()
		at.savePausedGridDecisionRecord(pausedReason, pausedDecision, pausedDetails)
		return nil
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	lang := at.config.StrategyConfig.Language
	if lang == "" {
		lang = "en"
	}

	// Build grid context
	gridCtx, err := at.buildGridContext()
	if err != nil {
		return fmt.Errorf("failed to build grid context: %w", err)
	}

	// Grid traders also need continuous equity snapshots for the dashboard curve.
	if at.store != nil {
		snapshot := &store.EquitySnapshot{
			TraderID:      at.id,
			Timestamp:     time.Now().UTC(),
			TotalEquity:   gridCtx.TotalEquity,
			Balance:       gridCtx.TotalEquity - gridCtx.UnrealizedPnL,
			UnrealizedPnL: gridCtx.UnrealizedPnL,
			PositionCount: int(math.Abs(gridCtx.CurrentLongPosition) + math.Abs(gridCtx.CurrentShortPosition)),
			MarginUsedPct: 0,
		}
		if err := at.store.Equity().Save(snapshot); err != nil {
			logger.Infof("⚠️ Failed to save grid equity snapshot: %v", err)
		}
	}

	// Get AI decisions
	decision, err := kernel.GetGridDecisions(gridCtx, at.mcpClient, gridConfig, lang)
	if err != nil {
		at.saveGridCycleFailureRecord("pause_grid", fmt.Sprintf("failed to get grid decisions: %v", err), []string{
			"Grid cycle failed before execution",
			fmt.Sprintf("AI decision request failed: %v", err),
		})
		return fmt.Errorf("failed to get grid decisions: %w", err)
	}

	// Check if trader is stopped before executing any decisions (prevent trades after Stop())
	at.isRunningMutex.RLock()
	running = at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("[Grid] Trader stopped before decision execution, aborting grid cycle")
		return nil
	}

	// Execute decisions
	actionRecords := make([]store.DecisionAction, 0, len(decision.Decisions))
	executionLog := make([]string, 0, len(decision.Decisions)+1)
	cycleSuccess := true
	pauseIssued := false
	for _, d := range decision.Decisions {
		// Check if trader is still running before each decision
		at.isRunningMutex.RLock()
		running := at.isRunning
		at.isRunningMutex.RUnlock()
		if !running {
			logger.Infof("[Grid] Trader stopped, skipping remaining %d decisions", len(decision.Decisions))
			executionLog = append(executionLog, "Trader stopped during grid execution, remaining decisions skipped")
			cycleSuccess = false
			break
		}

		actionRecord := store.DecisionAction{
			Action:     d.Action,
			Symbol:     d.Symbol,
			Quantity:   d.Quantity,
			Leverage:   d.Leverage,
			Price:      d.Price,
			StopLoss:   d.StopLoss,
			TakeProfit: d.TakeProfit,
			Confidence: d.Confidence,
			Reasoning:  d.Reasoning,
			Timestamp:  time.Now().UTC(),
			Success:    false,
		}

		if pauseIssued && d.Action == "cancel_all_orders" {
			actionRecord.Success = true
			actionRecord.Reasoning = strings.TrimSpace(strings.Join([]string{
				actionRecord.Reasoning,
				"Skipped redundant cancel_all_orders after pause_grid to preserve reduce-only exits.",
			}, " "))
			executionLog = append(executionLog, fmt.Sprintf("✓ %s %s skipped after pause_grid to preserve exit orders", d.Symbol, d.Action))
			actionRecords = append(actionRecords, actionRecord)
			continue
		}

		if err := at.executeGridDecision(&d); err != nil {
			logger.Warnf("[Grid] Failed to execute decision %s: %v", d.Action, err)
			actionRecord.Error = err.Error()
			executionLog = append(executionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
			cycleSuccess = false
		} else {
			actionRecord.Success = true
			executionLog = append(executionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			if d.Action == "pause_grid" {
				pauseIssued = true
			}
		}
		actionRecords = append(actionRecords, actionRecord)
	}

	// Sync state with exchange
	at.syncGridState()
	at.ensurePairedExitOrdersForAllFilledLevels()
	if ctx, err := at.buildGridContext(); err != nil {
		logger.Warnf("%s Failed to build context for post-cycle auto-heal: %v", at.gridLogPrefix(), err)
	} else if err := at.autoSeedMissingEntryOrdersForRangingGrid(ctx); err != nil {
		logger.Warnf("%s Failed to auto-seed missing entry orders after cycle sync: %v", at.gridLogPrefix(), err)
	}

	// Save decision record
	executionLog = append(executionLog, fmt.Sprintf("Grid cycle completed with %d decisions", len(decision.Decisions)))
	at.saveGridDecisionRecord(decision, actionRecords, executionLog, cycleSuccess)

	return nil
}

// buildGridContext builds the context for AI grid decisions
func (at *AutoTrader) buildGridContext() (*kernel.GridContext, error) {
	gridConfig := at.config.StrategyConfig.GridConfig

	// Get market data
	mktData, err := market.GetWithTimeframes(gridConfig.Symbol, []string{"5m", "4h"}, "5m", 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get market data: %w", err)
	}

	// Build base context from market data
	ctx := kernel.BuildGridContextFromMarketData(mktData, gridConfig)

	// Add grid state
	at.gridState.mu.RLock()
	ctx.Levels = at.gridState.Levels
	ctx.UpperPrice = at.gridState.UpperPrice
	ctx.LowerPrice = at.gridState.LowerPrice
	ctx.GridSpacing = at.gridState.GridSpacing
	ctx.IsPaused = at.gridState.IsPaused
	ctx.TotalProfit = at.gridState.TotalProfit
	ctx.TotalTrades = at.gridState.TotalTrades
	ctx.WinningTrades = at.gridState.WinningTrades
	ctx.MaxDrawdown = at.gridState.MaxDrawdown
	ctx.DailyPnL = at.gridState.DailyPnL

	// Count filled levels from local state. Pending order counts will be corrected
	// later using real exchange open orders to avoid phantom orders in AI context.
	for _, level := range at.gridState.Levels {
		if level.State == "pending" {
		} else if level.State == "filled" {
			ctx.FilledLevelCount++
			positionSide := normalizeGridPositionSide(level.PositionSide, level.Side)
			if positionSide == "LONG" {
				ctx.FilledLongLevelCount++
			} else {
				ctx.FilledShortLevelCount++
			}
		}
	}
	at.gridState.mu.RUnlock()

	// Get account info
	balance, err := at.trader.GetBalance()
	if err == nil {
		if equity, ok := balance["total_equity"].(float64); ok {
			ctx.TotalEquity = equity
		} else if total, ok := balance["totalWalletBalance"].(float64); ok {
			ctx.TotalEquity = total
			if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
				ctx.TotalEquity += unrealized
			}
		}
		if available, ok := balance["availableBalance"].(float64); ok {
			ctx.AvailableBalance = available
		}
		if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
			ctx.UnrealizedPnL = unrealized
		}
	}

	// Get current position
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == gridConfig.Symbol {
				if size, ok := pos["positionAmt"].(float64); ok {
					ctx.CurrentPosition += size
					side, _ := pos["side"].(string)
					positionSide := normalizeGridPositionSide(side, "")
					if positionSide == "LONG" {
						ctx.CurrentLongPosition += math.Abs(size)
					} else {
						ctx.CurrentShortPosition += math.Abs(size)
					}
				}
			}
		}
	}

	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err == nil {
		ctx.AggregatedOpenOrders = aggregateGridPromptOpenOrders(at.gridState.Levels, at.gridState.OrderBook, openOrders)
		ctx.ActiveOrderCount = 0
		ctx.ActiveLongOrderCount = 0
		ctx.ActiveShortOrderCount = 0
		ctx.ReduceOnlyOrderCount = 0
		for _, summary := range ctx.AggregatedOpenOrders {
			ctx.ActiveOrderCount += summary.OrderCount
			switch summary.Bucket {
			case "long_entry":
				ctx.ActiveLongOrderCount += summary.OrderCount
			case "short_entry":
				ctx.ActiveShortOrderCount += summary.OrderCount
			case "long_exit", "short_exit":
				ctx.ReduceOnlyOrderCount += summary.OrderCount
			}
		}
	}
	ctx.CollapsedLevelWarnings = detectCollapsedGridLevelWarnings(at.gridState.Levels)

	at.gridState.mu.Lock()
	at.gridState.LastContextATR14 = ctx.ATR14
	at.gridState.LastContextPrice = ctx.CurrentPrice
	at.gridState.LastContextBuiltAt = time.Now()
	at.gridState.mu.Unlock()

	return ctx, nil
}

func aggregateGridPromptOpenOrders(levels []kernel.GridLevelInfo, orderBook map[string]int, openOrders []OpenOrder) []kernel.GridOrderSummary {
	type aggregate struct {
		price              float64
		totalQuantity      float64
		totalNotional      float64
		orderCount         int
		levelIndexes       []int
		linkedLevelIndexes []int
		sourceEntryPrices  []float64
	}

	grouped := make(map[string]*aggregate)

	for _, order := range openOrders {
		positionSide := normalizeGridPositionSide(order.PositionSide, order.Side)
		isReduceOnly := (strings.EqualFold(order.Side, "SELL") && positionSide == "LONG") ||
			(strings.EqualFold(order.Side, "BUY") && positionSide == "SHORT")

		orderPrice := order.Price
		if orderPrice <= 0 {
			orderPrice = order.StopPrice
		}
		if orderPrice <= 0 {
			continue
		}

		bucket := "short_entry"
		switch {
		case isReduceOnly && positionSide == "LONG":
			bucket = "long_exit"
		case isReduceOnly && positionSide == "SHORT":
			bucket = "short_exit"
		case !isReduceOnly && positionSide == "LONG":
			bucket = "long_entry"
		}

		priceLabel := formatGridLogPrice(orderPrice)
		key := bucket + "|" + priceLabel
		current := grouped[key]
		if current == nil {
			current = &aggregate{price: orderPrice}
			grouped[key] = current
		}

		current.totalQuantity += order.Quantity
		current.totalNotional += math.Abs(order.Quantity * orderPrice)
		current.orderCount++

		if levelIndex, ok := orderBook[order.OrderID]; ok && levelIndex >= 0 {
			current.levelIndexes = append(current.levelIndexes, levelIndex)
			if levelIndex < len(levels) {
				linkedLevelIndex := levels[levelIndex].LinkedLevelIndex
				if linkedLevelIndex >= 0 {
					current.linkedLevelIndexes = append(current.linkedLevelIndexes, linkedLevelIndex)
					if linkedLevelIndex < len(levels) && levels[linkedLevelIndex].PositionEntry > 0 {
						current.sourceEntryPrices = append(current.sourceEntryPrices, levels[linkedLevelIndex].PositionEntry)
					}
				}
			}
		}
	}

	summaries := make([]kernel.GridOrderSummary, 0, len(grouped))
	for key, item := range grouped {
		parts := strings.SplitN(key, "|", 2)
		summaries = append(summaries, kernel.GridOrderSummary{
			Bucket:             parts[0],
			Price:              item.price,
			TotalQuantity:      item.totalQuantity,
			TotalNotional:      item.totalNotional,
			OrderCount:         item.orderCount,
			LevelIndexes:       uniqueSortedLevelIndexes(item.levelIndexes),
			LinkedLevelIndexes: uniqueSortedLevelIndexes(item.linkedLevelIndexes),
			SourceEntryPrices:  uniqueSortedPrices(item.sourceEntryPrices),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Bucket == summaries[j].Bucket {
			return summaries[i].Price > summaries[j].Price
		}
		return summaries[i].Bucket < summaries[j].Bucket
	})

	return summaries
}

func detectCollapsedGridLevelWarnings(levels []kernel.GridLevelInfo) []kernel.GridCollapsedLevelWarning {
	grouped := make(map[string][]int)
	priceLookup := make(map[string]float64)
	for _, level := range levels {
		priceLabel := formatGridLogPrice(level.Price)
		grouped[priceLabel] = append(grouped[priceLabel], level.Index)
		if _, ok := priceLookup[priceLabel]; !ok {
			priceLookup[priceLabel] = level.Price
		}
	}

	warnings := make([]kernel.GridCollapsedLevelWarning, 0)
	for priceLabel, levelIndexes := range grouped {
		if len(levelIndexes) < 2 {
			continue
		}
		sort.Ints(levelIndexes)
		warnings = append(warnings, kernel.GridCollapsedLevelWarning{
			Price:        priceLookup[priceLabel],
			LevelIndexes: append([]int(nil), levelIndexes...),
		})
	}

	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].Price > warnings[j].Price
	})
	return warnings
}

func uniqueSortedLevelIndexes(levelIndexes []int) []int {
	seen := make(map[int]struct{}, len(levelIndexes))
	unique := make([]int, 0, len(levelIndexes))
	for _, levelIndex := range levelIndexes {
		if levelIndex < 0 {
			continue
		}
		if _, ok := seen[levelIndex]; ok {
			continue
		}
		seen[levelIndex] = struct{}{}
		unique = append(unique, levelIndex)
	}
	sort.Ints(unique)
	return unique
}

func uniqueSortedPrices(prices []float64) []float64 {
	seen := make(map[string]struct{}, len(prices))
	unique := make([]float64, 0, len(prices))
	for _, price := range prices {
		if price <= 0 {
			continue
		}
		key := formatGridLogPrice(price)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, price)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i] > unique[j]
	})
	return unique
}

func isExplicitTrendProtectionDecision(d *kernel.Decision) bool {
	if d == nil {
		return false
	}
	if d.Action != "pause_grid" && d.Action != "cancel_all_orders" && d.Action != "adjust_grid" {
		return false
	}
	reasoning := strings.ToLower(d.Reasoning)
	if reasoning == "" {
		return false
	}
	trendSignals := []string{
		"趋势市场",
		"明显下跌趋势",
		"明显上涨趋势",
		"趋势行情",
		"breakout",
		"trending market",
		"clear downtrend",
		"clear uptrend",
		"price has broken",
		"price broke",
		"跌破网格",
		"突破网格",
		"跌破布林带",
		"突破箱体",
		"outside grid",
		"below grid",
		"above grid",
	}
	for _, signal := range trendSignals {
		if strings.Contains(reasoning, signal) {
			return true
		}
	}
	return false
}

// executeGridDecision executes a single grid decision
func (at *AutoTrader) executeGridDecision(d *kernel.Decision) error {
	if shouldProtect, ctx, reason := at.shouldProtectRangingEntryCoverage(); shouldProtect {
		switch d.Action {
		case "cancel_all_orders", "pause_grid", "adjust_grid":
			if isExplicitTrendProtectionDecision(d) {
				logger.Infof("%s Allowing %s despite ranging protection because decision explicitly indicates trend/breakout risk", at.gridLogPrefix(), d.Action)
				break
			}
			logger.Warnf("%s Blocked destructive action %s during ranging protection: %s", at.gridLogPrefix(), d.Action, reason)
			if ctx != nil {
				if err := at.autoSeedMissingEntryOrdersForRangingGrid(ctx); err != nil {
					logger.Warnf("%s Failed to auto-seed missing entry orders after blocking %s: %v", at.gridLogPrefix(), d.Action, err)
				}
			}
			return nil
		}
	}

	switch d.Action {
	case "place_buy_limit":
		return at.placeGridLimitOrder(d, "BUY")
	case "place_sell_limit":
		return at.placeGridLimitOrder(d, "SELL")
	case "cancel_order":
		return at.cancelGridOrder(d)
	case "cancel_all_orders":
		return at.cancelGridOrdersForRiskPause("cancel_all_orders")
	case "pause_grid":
		return at.pauseGrid(d.Reasoning)
	case "resume_grid":
		return at.resumeGrid()
	case "adjust_grid":
		return at.adjustGrid(d)
	case "hold":
		logger.Infof("%s Holding current state: %s", at.gridLogPrefix(), d.Reasoning)
		if ctx, err := at.buildGridContext(); err != nil {
			logger.Warnf("%s Failed to build context for hold auto-heal: %v", at.gridLogPrefix(), err)
		} else {
			if ctx.FilledLevelCount > 0 && ctx.ReduceOnlyOrderCount == 0 && (ctx.CurrentLongPosition > 0 || ctx.CurrentShortPosition > 0) {
				logger.Warnf("%s Grid hold detected live positions without any reduce-only exit orders (filledLevels=%d long=%.4f short=%.4f activeOrders=%d). Exchange-specific exit seeding may have failed.", at.gridLogPrefix(), ctx.FilledLevelCount, ctx.CurrentLongPosition, ctx.CurrentShortPosition, ctx.ActiveOrderCount)
			}
			if err := at.autoSeedMissingEntryOrdersForRangingGrid(ctx); err != nil {
				logger.Warnf("%s Failed to auto-seed missing entry orders during hold: %v", at.gridLogPrefix(), err)
			}
		}
		return nil
	case "close_long":
		logger.Warnf("%s Blocked AI direct close_long decision in grid mode; use adjacent reduce-only exits or explicit risk controls instead", at.gridLogPrefix())
		return nil
	case "close_short":
		logger.Warnf("%s Blocked AI direct close_short decision in grid mode; use adjacent reduce-only exits or explicit risk controls instead", at.gridLogPrefix())
		return nil
	default:
		logger.Warnf("[Grid] Unknown action: %s", d.Action)
		return nil
	}
}

func (at *AutoTrader) gridLogPrefix() string {
	return fmt.Sprintf("[Grid][%s|%s]", at.name, at.id)
}

func (at *AutoTrader) isInsideMidBox(ctx *kernel.GridContext) bool {
	if ctx == nil || ctx.BoxData == nil {
		return false
	}
	return ctx.CurrentPrice >= ctx.BoxData.MidLower && ctx.CurrentPrice <= ctx.BoxData.MidUpper
}

func (at *AutoTrader) isInsideGridBounds(ctx *kernel.GridContext) bool {
	if ctx == nil {
		return false
	}
	if at.gridState == nil {
		return false
	}
	return ctx.CurrentPrice >= at.gridState.LowerPrice && ctx.CurrentPrice <= at.gridState.UpperPrice
}

const (
	gridPauseResumeCooldown      = 6 * time.Minute
	gridPauseResumeConfirmations = 3
	gridResumeNarrowBollingerMax = 3.0
	gridResumeLaggingEMAUpper    = 8.0
	gridResumeNeutralRSILower    = 35.0
	gridResumeNeutralRSIUpper    = 70.0
)

func (at *AutoTrader) noteGridPaused(reason string) {
	if at.gridState == nil {
		return
	}
	// While already paused, keep the original pause anchor instead of
	// restarting cooldown/confirmation state on every re-asserted pause.
	if at.gridState.IsPaused && !at.gridState.PausedAt.IsZero() {
		at.gridState.PauseReason = reason
		return
	}
	at.gridState.PausedAt = time.Now()
	at.gridState.ResumedAt = time.Time{}
	at.gridState.PauseReason = reason
	at.gridState.ResumeConfirmCount = 0
	at.gridState.RecoverySeedWaves = 0
}

func (at *AutoTrader) qualifiesForGridResume(ctx *kernel.GridContext) bool {
	if ctx == nil || at.gridState == nil {
		return false
	}
	if !ctx.IsPaused {
		return false
	}
	if time.Since(at.gridState.PausedAt) < gridPauseResumeCooldown {
		return false
	}
	if ctx.BollingerWidth >= 3.0 {
		return false
	}
	if at.isInsideMidBox(ctx) {
		return ctx.EMADistance < 1.5 || at.qualifiesForLaggingEMARanging(ctx)
	}
	if at.isInsideGridBounds(ctx) {
		return ctx.EMADistance < 4.0 || at.qualifiesForLaggingEMARanging(ctx)
	}
	return false
}

func (at *AutoTrader) qualifiesForLaggingEMARanging(ctx *kernel.GridContext) bool {
	if ctx == nil {
		return false
	}
	if !at.isInsideGridBounds(ctx) {
		return false
	}
	if ctx.BollingerWidth <= 0 || ctx.BollingerWidth > gridResumeNarrowBollingerMax {
		return false
	}
	if ctx.EMADistance < 4.0 || ctx.EMADistance > gridResumeLaggingEMAUpper {
		return false
	}
	if ctx.RSI14 <= 0 {
		return false
	}
	if ctx.RSI14 < gridResumeNeutralRSILower || ctx.RSI14 > gridResumeNeutralRSIUpper {
		return false
	}
	if ctx.BoxData != nil && (ctx.CurrentPrice < ctx.BoxData.LongLower || ctx.CurrentPrice > ctx.BoxData.LongUpper) {
		return false
	}
	return true
}

func (at *AutoTrader) gridResumeBlockReason(ctx *kernel.GridContext) string {
	if at.gridState == nil {
		return "grid state unavailable"
	}
	if ctx == nil {
		return "grid context unavailable"
	}
	if !ctx.IsPaused {
		return "grid is not paused"
	}
	if remaining := gridPauseResumeCooldown - time.Since(at.gridState.PausedAt); remaining > 0 {
		return fmt.Sprintf("resume cooldown active for %s", remaining.Round(time.Second))
	}
	if !at.isInsideMidBox(ctx) && !at.isInsideGridBounds(ctx) {
		if ctx.BoxData != nil {
			return fmt.Sprintf("resume blocked: current price %.6f outside medium box %.6f-%.6f and grid bounds %.6f-%.6f", ctx.CurrentPrice, ctx.BoxData.MidLower, ctx.BoxData.MidUpper, at.gridState.LowerPrice, at.gridState.UpperPrice)
		}
		return fmt.Sprintf("resume blocked: current price %.6f outside medium box and grid bounds", ctx.CurrentPrice)
	}
	if ctx.BollingerWidth >= 3.0 {
		return fmt.Sprintf("resume blocked: bollinger width %.2f >= 3.0", ctx.BollingerWidth)
	}
	if at.isInsideMidBox(ctx) {
		if ctx.EMADistance >= 1.5 {
			if at.qualifiesForLaggingEMARanging(ctx) {
				return at.gridResumeConfirmationReason()
			}
			return fmt.Sprintf("resume blocked: ema distance %.2f >= 1.5 inside medium box", ctx.EMADistance)
		}
	} else if at.isInsideGridBounds(ctx) {
		if ctx.EMADistance >= 4.0 {
			if at.qualifiesForLaggingEMARanging(ctx) {
				return at.gridResumeConfirmationReason()
			}
			return fmt.Sprintf("resume blocked: ema distance %.2f >= 4.0 inside grid bounds", ctx.EMADistance)
		}
	}
	return at.gridResumeConfirmationReason()
}

func (at *AutoTrader) gridResumeConfirmationReason() string {
	at.gridState.mu.RLock()
	confirmCount := at.gridState.ResumeConfirmCount
	at.gridState.mu.RUnlock()
	if confirmCount < gridPauseResumeConfirmations {
		return fmt.Sprintf("resume awaiting confirmations %d/%d", confirmCount, gridPauseResumeConfirmations)
	}
	return "resume conditions satisfied"
}

func (at *AutoTrader) buildPausedDecisionReason(baseReason string) string {
	reason := strings.TrimSpace(baseReason)
	if reason == "" {
		reason = "grid paused"
	}

	ctx, err := at.buildGridContext()
	if err != nil {
		return fmt.Sprintf("%s; resume status unavailable: %v", reason, err)
	}

	blockReason := at.gridResumeBlockReason(ctx)
	return fmt.Sprintf("%s; %s; current price %.6f, boll %.2f, ema %.2f", reason, blockReason, ctx.CurrentPrice, ctx.BollingerWidth, ctx.EMADistance)
}

func (at *AutoTrader) buildPausedExecutionDetails() []string {
	ctx, err := at.buildGridContext()
	if err != nil {
		return []string{fmt.Sprintf("Resume status unavailable: %v", err)}
	}
	if ctx == nil {
		return []string{"Resume status unavailable: empty grid context"}
	}

	blockReason := at.gridResumeBlockReason(ctx)
	details := []string{blockReason}
	if ctx.BoxData != nil {
		details = append(details, fmt.Sprintf("Medium box %.6f-%.6f; current price %.6f", ctx.BoxData.MidLower, ctx.BoxData.MidUpper, ctx.CurrentPrice))
	} else {
		details = append(details, fmt.Sprintf("Current price %.6f", ctx.CurrentPrice))
	}
	details = append(details, fmt.Sprintf("Bollinger width %.2f, EMA distance %.2f", ctx.BollingerWidth, ctx.EMADistance))
	return details
}

func (at *AutoTrader) buildPausedAIDecision() *kernel.FullDecision {
	if at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil
	}
	if at.mcpClient == nil {
		return nil
	}

	ctx, err := at.buildGridContext()
	if err != nil {
		logger.Warnf("%s Failed to build context for paused AI re-evaluation: %v", at.gridLogPrefix(), err)
		return nil
	}
	if ctx == nil {
		return nil
	}

	lang := at.config.StrategyConfig.Language
	if lang == "" {
		lang = "en"
	}

	decision, err := kernel.GetGridDecisions(ctx, at.mcpClient, at.config.StrategyConfig.GridConfig, lang)
	if err != nil {
		logger.Warnf("%s Failed paused AI re-evaluation: %v", at.gridLogPrefix(), err)
		return nil
	}
	return decision
}

func (at *AutoTrader) enforceRecentPauseGuard() {
	if at.gridState == nil || at.store == nil {
		return
	}

	records, err := at.store.Decision().GetLatestRecords(at.id, 1)
	if err != nil || len(records) == 0 {
		return
	}

	latest := records[0]
	if !latest.Success || len(latest.Decisions) == 0 {
		return
	}
	if latest.Decisions[0].Action != "pause_grid" {
		return
	}

	pausedAt := latest.Timestamp
	if pausedAt.IsZero() {
		return
	}
	if time.Since(pausedAt) >= gridPauseResumeCooldown {
		return
	}

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()
	if !at.gridState.ResumedAt.IsZero() && !at.gridState.ResumedAt.Before(pausedAt) {
		return
	}
	if at.gridState.IsPaused {
		if at.gridState.PausedAt.IsZero() || at.gridState.PausedAt.After(pausedAt) {
			at.gridState.PausedAt = pausedAt
		}
		return
	}

	at.gridState.IsPaused = true
	at.gridState.PausedAt = pausedAt
	at.gridState.ResumedAt = time.Time{}
	at.gridState.PauseReason = "recent_pause_guard"
	at.gridState.ResumeConfirmCount = 0
	at.gridState.RecoverySeedWaves = 0
	logger.Warnf("%s Reinstated paused state from latest persisted pause_grid decision at %s", at.gridLogPrefix(), pausedAt.UTC().Format(time.RFC3339))
}

func (at *AutoTrader) qualifiesAsProtectedRanging(ctx *kernel.GridContext) bool {
	if ctx == nil || ctx.IsPaused {
		return false
	}

	if ctx.BollingerWidth >= 3.0 {
		return false
	}

	// Inside the medium box is the primary ranging anchor. While price remains
	// inside that box, we allow a looser EMA spread before treating the market
	// as trending so early directional drift does not tear down the grid.
	if at.isInsideMidBox(ctx) {
		return ctx.EMADistance < 2.0 || at.qualifiesForLaggingEMARanging(ctx)
	}

	// If price is still inside the current grid working range, keep the same
	// looser threshold. This prevents AI drift from tearing down a healthy
	// ranging grid before price has actually left the active box structure.
	if at.isInsideGridBounds(ctx) {
		return ctx.EMADistance < 2.0 || at.qualifiesForLaggingEMARanging(ctx)
	}

	// Outside the medium box we require a tighter EMA spread before treating it
	// as clearly ranging.
	return ctx.EMADistance < 1.2
}

func (at *AutoTrader) shouldProtectRangingEntryCoverage() (bool, *kernel.GridContext, string) {
	if at.gridState == nil {
		return false, nil, ""
	}

	ctx, err := at.buildGridContext()
	if err != nil {
		logger.Warnf("[Grid] Failed to build context for ranging protection check: %v", err)
		return false, nil, ""
	}

	if ctx.IsPaused {
		return false, ctx, "grid paused"
	}

	if !at.qualifiesAsProtectedRanging(ctx) {
		return false, ctx, fmt.Sprintf("market not clearly ranging (boll=%.2f ema=%.2f)", ctx.BollingerWidth, ctx.EMADistance)
	}

	at.gridState.mu.RLock()
	currentPrice := ctx.CurrentPrice
	lower := at.gridState.LowerPrice
	upper := at.gridState.UpperPrice
	at.gridState.mu.RUnlock()

	if lower > 0 && upper > 0 && (currentPrice < lower || currentPrice > upper) {
		return false, ctx, fmt.Sprintf("current price %.6f outside grid bounds %.6f-%.6f", currentPrice, lower, upper)
	}

	return true, ctx, "ranging market with active grid protection"
}

// IsGridStrategy returns true if current strategy is grid trading
func (at *AutoTrader) IsGridStrategy() bool {
	if at.config.StrategyConfig == nil {
		return false
	}
	return at.config.StrategyConfig.StrategyType == "grid_trading" && at.config.StrategyConfig.GridConfig != nil
}

// saveGridDecisionRecord saves the grid decision to database
func (at *AutoTrader) saveGridDecisionRecord(decision *kernel.FullDecision, actionRecords []store.DecisionAction, executionLog []string, success bool) {
	if at.store == nil {
		return
	}

	at.cycleNumber++
	compoundingBase := at.currentGridInvestmentBase()
	if compoundingBase > 0 {
		executionLog = append([]string{
			fmt.Sprintf("Compounding base (live account equity): $%.2f", compoundingBase),
		}, executionLog...)
	}

	record := &store.DecisionRecord{
		TraderID:            at.id,
		CycleNumber:         at.cycleNumber,
		Timestamp:           time.Now().UTC(),
		SystemPrompt:        decision.SystemPrompt,
		InputPrompt:         decision.UserPrompt,
		CoTTrace:            decision.CoTTrace,
		RawResponse:         decision.RawResponse,
		AIRequestDurationMs: decision.AIRequestDurationMs,
		Success:             success,
	}

	if len(decision.Decisions) > 0 {
		decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
		record.DecisionJSON = string(decisionJSON)
	}

	record.Decisions = actionRecords
	record.ExecutionLog = executionLog
	if !success {
		for _, actionRecord := range actionRecords {
			if actionRecord.Error != "" {
				record.ErrorMessage = actionRecord.Error
				break
			}
		}
		if record.ErrorMessage == "" {
			record.ErrorMessage = "one or more grid decisions failed"
		}
	}

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Warnf("[Grid] Failed to save decision record: %v", err)
	}
}

func (at *AutoTrader) savePausedGridDecisionRecord(reason string, aiDecision *kernel.FullDecision, pauseDetails []string) {
	if at.store == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		reason = "grid paused; entry orders remain blocked and reduce-only exits are preserved"
	}

	decision := &kernel.FullDecision{
		Decisions: []kernel.Decision{
			{
				Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
				Action:     "pause_grid",
				Confidence: 100,
				Reasoning:  reason,
			},
		},
		Timestamp:           time.Now().UTC(),
		AIRequestDurationMs: 0,
	}

	if aiDecision != nil {
		decision.SystemPrompt = aiDecision.SystemPrompt
		decision.UserPrompt = aiDecision.UserPrompt
		decision.CoTTrace = aiDecision.CoTTrace
		decision.RawResponse = aiDecision.RawResponse
		decision.AIRequestDurationMs = aiDecision.AIRequestDurationMs
		if len(aiDecision.Decisions) > 0 {
			decision.Decisions[0].Confidence = aiDecision.Decisions[0].Confidence
		}
	}

	actionRecords := []store.DecisionAction{
		{
			Action:     "pause_grid",
			Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
			Confidence: decision.Decisions[0].Confidence,
			Reasoning:  reason,
			Timestamp:  time.Now().UTC(),
			Success:    true,
		},
	}

	executionLog := []string{
		"Grid remains paused",
		"Entry orders remain blocked",
		"Reduce-only exit orders preserved",
	}
	executionLog = append(executionLog, pauseDetails...)
	if aiDecision != nil && len(aiDecision.Decisions) > 0 {
		executionLog = append(executionLog, fmt.Sprintf("Paused AI re-evaluation: %s (%s, confidence %d)", aiDecision.Decisions[0].Action, aiDecision.Decisions[0].Reasoning, aiDecision.Decisions[0].Confidence))
	}

	at.saveGridDecisionRecord(decision, actionRecords, executionLog, true)
}

func (at *AutoTrader) saveGridCycleFailureRecord(action, errorMessage string, executionLog []string) {
	if at.store == nil {
		return
	}
	if strings.TrimSpace(action) == "" {
		action = "pause_grid"
	}
	if strings.TrimSpace(errorMessage) == "" {
		errorMessage = "grid cycle failed"
	}

	decision := &kernel.FullDecision{
		Decisions: []kernel.Decision{{
			Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
			Action:     action,
			Confidence: 0,
			Reasoning:  errorMessage,
		}},
		Timestamp: time.Now().UTC(),
	}

	actionRecords := []store.DecisionAction{{
		Action:     action,
		Symbol:     at.config.StrategyConfig.GridConfig.Symbol,
		Confidence: 0,
		Reasoning:  errorMessage,
		Timestamp:  time.Now().UTC(),
		Success:    false,
		Error:      errorMessage,
	}}

	if len(executionLog) == 0 {
		executionLog = []string{errorMessage}
	}

	at.saveGridDecisionRecord(decision, actionRecords, executionLog, false)
}

// GridRiskInfo contains risk information for frontend display
type GridRiskInfo struct {
	CurrentLeverage     int     `json:"current_leverage"`
	EffectiveLeverage   float64 `json:"effective_leverage"`
	RecommendedLeverage int     `json:"recommended_leverage"`
	CompoundingBase     float64 `json:"compounding_base"`

	CurrentPosition      float64 `json:"current_position"`
	MaxPosition          float64 `json:"max_position"`
	PositionPercent      float64 `json:"position_percent"`
	CurrentLongPosition  float64 `json:"current_long_position"`
	CurrentShortPosition float64 `json:"current_short_position"`
	CurrentLongValue     float64 `json:"current_long_value"`
	CurrentShortValue    float64 `json:"current_short_value"`
	EntryOrderCount      int     `json:"entry_order_count"`
	ReduceOnlyOrderCount int     `json:"reduce_only_order_count"`
	LongOrderCount       int     `json:"long_order_count"`
	ShortOrderCount      int     `json:"short_order_count"`
	LongFilledLevels     int     `json:"long_filled_levels"`
	ShortFilledLevels    int     `json:"short_filled_levels"`
	AverageOrderNotional float64 `json:"average_order_notional"`

	LiquidationPrice    float64 `json:"liquidation_price"`
	LiquidationDistance float64 `json:"liquidation_distance"`

	RegimeLevel string `json:"regime_level"`

	ShortBoxUpper      float64 `json:"short_box_upper"`
	ShortBoxLower      float64 `json:"short_box_lower"`
	MidBoxUpper        float64 `json:"mid_box_upper"`
	MidBoxLower        float64 `json:"mid_box_lower"`
	LongBoxUpper       float64 `json:"long_box_upper"`
	LongBoxLower       float64 `json:"long_box_lower"`
	CurrentPrice       float64 `json:"current_price"`
	GridUpperPrice     float64 `json:"grid_upper_price"`
	GridLowerPrice     float64 `json:"grid_lower_price"`
	GridBoundarySource string  `json:"grid_boundary_source"`
	GridSpacing        float64 `json:"grid_spacing"`
	MinGridSpacing     float64 `json:"min_grid_spacing"`
	MaxGridSpacing     float64 `json:"max_grid_spacing"`

	BreakoutLevel     string `json:"breakout_level"`
	BreakoutDirection string `json:"breakout_direction"`

	FirstEntryGuardThresholdPct  float64 `json:"first_entry_guard_threshold_pct"`
	FirstEntryGuardATRMultiplier float64 `json:"first_entry_guard_atr_multiplier"`
	FirstEntryGuardFixedBand     float64 `json:"first_entry_guard_fixed_band"`
	FirstEntryGuardATRBand       float64 `json:"first_entry_guard_atr_band"`
	FirstEntryGuardActiveSource  string  `json:"first_entry_guard_active_source"`
	ShortFirstEntryGuardPrice    float64 `json:"short_first_entry_guard_price"`
	LongFirstEntryGuardPrice     float64 `json:"long_first_entry_guard_price"`
	EffectiveATR14               float64 `json:"effective_atr14"`
	LongReferenceStopPrice       float64 `json:"long_reference_stop_price"`
	ShortReferenceStopPrice      float64 `json:"short_reference_stop_price"`

	// Grid direction
	CurrentGridDirection  string `json:"current_grid_direction"`
	DirectionChangeCount  int    `json:"direction_change_count"`
	EnableDirectionAdjust bool   `json:"enable_direction_adjust"`

	CurrentRiskState            string                `json:"current_risk_state"`
	PreviousRiskState           string                `json:"previous_risk_state"`
	CurrentRiskReason           string                `json:"current_risk_reason"`
	CurrentRiskSide             string                `json:"current_risk_side"`
	RiskStateChangedAt          int64                 `json:"risk_state_changed_at"`
	StopLossTriggerPrice        float64               `json:"stop_loss_trigger_price"`
	StopLossExecutionPrice      float64               `json:"stop_loss_execution_price"`
	StopLossEntryPrice          float64               `json:"stop_loss_entry_price"`
	StopLossPositionQty         float64               `json:"stop_loss_position_qty"`
	StopLossReducedQty          float64               `json:"stop_loss_reduced_qty"`
	StopLossRemainingQty        float64               `json:"stop_loss_remaining_qty"`
	StopLossPositionValue       float64               `json:"stop_loss_position_value"`
	StopLossReducedValue        float64               `json:"stop_loss_reduced_value"`
	StopLossUnrealizedLoss      float64               `json:"stop_loss_unrealized_loss"`
	StopLossUnrealizedLossPct   float64               `json:"stop_loss_unrealized_loss_pct"`
	StopLossUnrealizedLossEqPct float64               `json:"stop_loss_unrealized_loss_equity_pct"`
	RiskHistory                 []GridRiskHistoryItem `json:"risk_history"`

	GridLevels []GridRiskLevel `json:"grid_levels"`
	GridOrders []GridRiskOrder `json:"grid_orders"`
}

type GridRiskLevel struct {
	Index int     `json:"index"`
	Price float64 `json:"price"`
}

type GridRiskOrder struct {
	OrderID          string  `json:"order_id"`
	Price            float64 `json:"price"`
	Quantity         float64 `json:"quantity"`
	Notional         float64 `json:"notional"`
	Side             string  `json:"side"`
	PositionSide     string  `json:"position_side"`
	ReduceOnly       bool    `json:"reduce_only"`
	LevelIndex       int     `json:"level_index"`
	LinkedLevelIndex int     `json:"linked_level_index"`
	SourceEntryPrice float64 `json:"source_entry_price"`
	Bucket           string  `json:"bucket"`
	Status           string  `json:"status"`
}

type GridRiskHistoryItem struct {
	ID                  string                 `json:"id"`
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
	CreatedAt           int64                  `json:"created_at"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}
