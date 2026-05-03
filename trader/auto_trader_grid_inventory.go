package trader

import (
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

const gridLotQtyTolerance = 0.0000001

func (at *AutoTrader) restoreGridInventoryLotsToState() int {
	if at.store == nil || at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return 0
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, gridConfig.Symbol)
	if err != nil || len(lots) == 0 {
		return 0
	}

	sort.SliceStable(lots, func(i, j int) bool {
		if lots[i].OpenedAt.Equal(lots[j].OpenedAt) {
			return lots[i].SourceLevelIndex < lots[j].SourceLevelIndex
		}
		return lots[i].OpenedAt.Before(lots[j].OpenedAt)
	})

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	restored := 0
	for _, lot := range lots {
		if lot.RemainingQty <= 0 {
			continue
		}

		targetIdx := -1
		if lot.SourceLevelIndex >= 0 && lot.SourceLevelIndex < len(at.gridState.Levels) && at.gridState.Levels[lot.SourceLevelIndex].State == "empty" {
			targetIdx = lot.SourceLevelIndex
		}
		if targetIdx < 0 {
			targetIdx = at.findClosestGridLevelIndexLocked(lot.EntryPrice, func(level kernel.GridLevelInfo) bool {
				return level.State == "empty"
			})
		}
		if targetIdx < 0 {
			continue
		}

		level := &at.gridState.Levels[targetIdx]
		level.State = "filled"
		level.PositionEntry = lot.EntryPrice
		level.PositionSize = lot.RemainingQty
		level.PositionSide = lot.PositionSide
		level.OrderID = ""
		level.OrderQuantity = 0
		level.OrderPositionSide = ""
		level.OrderReduceOnly = false
		level.LinkedLevelIndex = 0
		restored++
	}

	if restored > 0 {
		logger.Infof("[Grid] Restored %d open inventory lots from persistent grid ledger", restored)
	}

	return restored
}

func (at *AutoTrader) upsertGridInventoryLot(levelIdx int, positionSide string, quantity float64, entryPrice float64, entryOrderID string) error {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return nil
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	if entryOrderID != "" {
		existingByOrder, err := at.store.Grid().FindOpenInventoryLotByEntryOrder(at.id, gridConfig.Symbol, positionSide, entryOrderID)
		if err != nil {
			return err
		}
		if existingByOrder != nil {
			existingByOrder.EntryPrice = entryPrice
			existingByOrder.EntryQuantity = quantity
			existingByOrder.RemainingQty = quantity
			existingByOrder.Status = "OPEN"
			return at.store.Grid().SaveInventoryLot(existingByOrder)
		}
	}

	if entryOrderID == "" {
		existingSynthetic, err := at.store.Grid().FindSyntheticOpenInventoryLot(at.id, gridConfig.Symbol, positionSide, levelIdx)
		if err != nil {
			return err
		}
		if existingSynthetic != nil {
			existingSynthetic.EntryPrice = entryPrice
			existingSynthetic.EntryQuantity = quantity
			existingSynthetic.RemainingQty = quantity
			existingSynthetic.Status = "OPEN"
			return at.store.Grid().SaveInventoryLot(existingSynthetic)
		}
	}

	return at.store.Grid().SaveInventoryLot(&store.GridInventoryLotModel{
		TraderID:         at.id,
		Symbol:           gridConfig.Symbol,
		PositionSide:     positionSide,
		SourceLevelIndex: levelIdx,
		ExitLevelIndex:   -1,
		EntryPrice:       entryPrice,
		EntryQuantity:    quantity,
		RemainingQty:     quantity,
		EntryOrderID:     entryOrderID,
		Status:           "OPEN",
		OpenedAt:         time.Now(),
	})
}

func (at *AutoTrader) applyGridExitToInventory(sourceLevelIdx int, closedQty float64, exitLevelIdx int, exitOrderID string) {
	if at.store == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return
	}

	if sourceLevelIdx < 0 || closedQty <= 0 {
		return
	}

	gridConfig := at.config.StrategyConfig.GridConfig

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	if sourceLevelIdx >= len(at.gridState.Levels) {
		return
	}

	sourceLevel := &at.gridState.Levels[sourceLevelIdx]
	positionSide := normalizeGridPositionSide(sourceLevel.PositionSide, sourceLevel.Side)

	lot, err := at.store.Grid().FindOpenInventoryLot(at.id, gridConfig.Symbol, positionSide, sourceLevelIdx)
	if err != nil || lot == nil {
		if sourceLevel.PositionSize <= closedQty+gridLotQtyTolerance {
			clearGridFilledPosition(sourceLevel)
		} else {
			sourceLevel.PositionSize = math.Max(0, sourceLevel.PositionSize-closedQty)
		}
		return
	}

	remainingQty := lot.RemainingQty - closedQty
	if remainingQty < gridLotQtyTolerance {
		remainingQty = 0
	}
	if err := at.store.Grid().UpdateInventoryLotAfterClose(lot.ID, remainingQty, exitLevelIdx, exitOrderID); err != nil {
		logger.Warnf("[Grid] Failed to update persistent lot %s after exit: %v", lot.ID, err)
	}

	if remainingQty <= 0 {
		clearGridFilledPosition(sourceLevel)
	} else {
		sourceLevel.PositionSize = remainingQty
	}
}

func (at *AutoTrader) reconcileGridInventoryLotsWithExchangePositions(symbol string, positions []map[string]interface{}) {
	if at.store == nil || at.gridState == nil || at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return
	}

	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, symbol)
	if err != nil {
		return
	}

	type actualPosition struct {
		qty        float64
		entryPrice float64
	}

	actualBySide := map[string]actualPosition{
		"LONG":  {},
		"SHORT": {},
	}
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
		normalizedSide := normalizeGridPositionSide(side, "")
		entryPrice, _ := pos["entryPrice"].(float64)
		actualBySide[normalizedSide] = actualPosition{
			qty:        math.Abs(size),
			entryPrice: entryPrice,
		}
	}

	lotsBySide := map[string][]store.GridInventoryLotModel{
		"LONG":  make([]store.GridInventoryLotModel, 0),
		"SHORT": make([]store.GridInventoryLotModel, 0),
	}
	for _, lot := range lots {
		side := strings.ToUpper(strings.TrimSpace(lot.PositionSide))
		lotsBySide[side] = append(lotsBySide[side], lot)
	}

	reconciled := false
	for _, side := range []string{"LONG", "SHORT"} {
		sideLots := lotsBySide[side]
		sort.SliceStable(sideLots, func(i, j int) bool {
			if sideLots[i].OpenedAt.Equal(sideLots[j].OpenedAt) {
				return sideLots[i].SourceLevelIndex > sideLots[j].SourceLevelIndex
			}
			return sideLots[i].OpenedAt.After(sideLots[j].OpenedAt)
		})

		targetQty := actualBySide[side].qty
		totalLotQty := 0.0
		for _, lot := range sideLots {
			totalLotQty += lot.RemainingQty
		}

		if len(sideLots) > 0 && math.Abs(totalLotQty-targetQty) > gridLotQtyTolerance {
			for _, lot := range sideLots {
				switch {
				case targetQty <= gridLotQtyTolerance:
					if err := at.store.Grid().UpdateInventoryLotAfterClose(lot.ID, 0, lot.ExitLevelIndex, lot.ExitOrderID); err != nil {
						logger.Warnf("[Grid] Failed to close stale inventory lot %s during position reconciliation: %v", lot.ID, err)
						continue
					}
					reconciled = true
				case lot.RemainingQty <= targetQty+gridLotQtyTolerance:
					targetQty -= lot.RemainingQty
				default:
					if err := at.store.Grid().UpdateInventoryLotAfterClose(lot.ID, targetQty, lot.ExitLevelIndex, lot.ExitOrderID); err != nil {
						logger.Warnf("[Grid] Failed to trim inventory lot %s during position reconciliation: %v", lot.ID, err)
						continue
					}
					targetQty = 0
					reconciled = true
				}
			}
		}

		if targetQty > gridLotQtyTolerance {
			entryPrice := actualBySide[side].entryPrice
			if entryPrice <= 0 {
				continue
			}

			at.gridState.mu.Lock()
			sourceLevelIdx := at.findClosestGridLevelIndexLocked(entryPrice, nil)
			at.gridState.mu.Unlock()
			if sourceLevelIdx < 0 {
				continue
			}

			existingLot, findErr := at.store.Grid().FindSyntheticOpenInventoryLot(at.id, symbol, side, sourceLevelIdx)
			if findErr != nil {
				logger.Warnf("[Grid] Failed to look up synthetic inventory lot for %s %s during position reconciliation: %v", symbol, side, findErr)
				continue
			}
			if existingLot != nil {
				existingLot.EntryPrice = entryPrice
				existingLot.EntryQuantity = targetQty
				existingLot.RemainingQty = targetQty
				existingLot.Status = "OPEN"
				if err := at.store.Grid().SaveInventoryLot(existingLot); err != nil {
					logger.Warnf("[Grid] Failed to refresh synthetic inventory lot for %s %s during position reconciliation: %v", symbol, side, err)
					continue
				}
				logger.Warnf("[Grid] Refreshed synthetic inventory lot for %s %s qty=%.4f entry=%.8f during position reconciliation", symbol, side, targetQty, entryPrice)
			} else {
				if err := at.store.Grid().SaveInventoryLot(&store.GridInventoryLotModel{
					TraderID:         at.id,
					Symbol:           symbol,
					PositionSide:     side,
					SourceLevelIndex: sourceLevelIdx,
					ExitLevelIndex:   -1,
					EntryPrice:       entryPrice,
					EntryQuantity:    targetQty,
					RemainingQty:     targetQty,
					EntryOrderID:     "",
					Status:           "OPEN",
					OpenedAt:         time.Now(),
				}); err != nil {
					logger.Warnf("[Grid] Failed to synthesize inventory lot for %s %s during position reconciliation: %v", symbol, side, err)
					continue
				}
				logger.Warnf("[Grid] Synthesized inventory lot for %s %s qty=%.4f entry=%.8f during position reconciliation", symbol, side, targetQty, entryPrice)
			}
			reconciled = true
		}
	}

	if reconciled {
		at.rebuildFilledGridLevelsFromInventoryLots(symbol)
		logger.Warnf("[Grid] Reconciled persistent inventory lots against live exchange positions for %s", symbol)
	}
}

func (at *AutoTrader) rebuildFilledGridLevelsFromInventoryLots(symbol string) {
	if at.store == nil || at.gridState == nil {
		return
	}

	lots, err := at.store.Grid().LoadOpenInventoryLots(at.id, symbol)
	if err != nil {
		return
	}

	sort.SliceStable(lots, func(i, j int) bool {
		if lots[i].OpenedAt.Equal(lots[j].OpenedAt) {
			return lots[i].SourceLevelIndex < lots[j].SourceLevelIndex
		}
		return lots[i].OpenedAt.Before(lots[j].OpenedAt)
	})

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	for i := range at.gridState.Levels {
		if at.gridState.Levels[i].State == "filled" {
			clearGridFilledPosition(&at.gridState.Levels[i])
		}
	}

	for _, lot := range lots {
		if lot.RemainingQty <= 0 {
			continue
		}

		targetIdx := -1
		if lot.SourceLevelIndex >= 0 && lot.SourceLevelIndex < len(at.gridState.Levels) && at.gridState.Levels[lot.SourceLevelIndex].State == "empty" {
			targetIdx = lot.SourceLevelIndex
		}
		if targetIdx < 0 {
			targetIdx = at.findClosestGridLevelIndexLocked(lot.EntryPrice, func(level kernel.GridLevelInfo) bool {
				return level.State == "empty"
			})
		}
		if targetIdx < 0 {
			continue
		}

		level := &at.gridState.Levels[targetIdx]
		level.State = "filled"
		level.PositionEntry = lot.EntryPrice
		level.PositionSize = lot.RemainingQty
		level.PositionSide = lot.PositionSide
		level.OrderID = ""
		level.OrderQuantity = 0
		level.OrderPositionSide = ""
		level.OrderReduceOnly = false
		level.LinkedLevelIndex = 0
	}
}
