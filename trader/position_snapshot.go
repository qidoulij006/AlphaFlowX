package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"time"
)

// CreatePositionSnapshot gets current real positions from exchange and creates snapshot positions
// This function will:
// 1. Delete all OPEN old positions from database
// 2. Get current real positions from exchange
// 3. Create a "snapshot" record for each real position
func CreatePositionSnapshot(traderID, exchangeID, exchangeType string, trader Trader, st *store.Store) error {
	logger.Infof("📸 Creating position snapshot for trader %s (%s)...", traderID, exchangeType)

	positionStore := st.Position()

	// Step 1: Delete all OPEN positions
	logger.Infof("🗑️  Deleting all OPEN positions from database...")
	if err := positionStore.DeleteAllOpenPositions(traderID); err != nil {
		return fmt.Errorf("failed to delete open positions: %w", err)
	}
	logger.Infof("✅ Deleted all OPEN positions")

	// Step 2: Get current positions from exchange
	logger.Infof("📡 Fetching current positions from exchange...")
	positions, err := trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions from exchange: %w", err)
	}

	if len(positions) == 0 {
		logger.Infof("✅ No open positions on exchange, snapshot complete")
		return nil
	}

	logger.Infof("📥 Found %d positions on exchange", len(positions))

	// Step 3: Create snapshot record for each position
	nowMs := time.Now().UnixMilli()
	createdCount := 0

	for _, posMap := range positions {
		// Parse position data
		rawSymbol, _ := posMap["symbol"].(string)
		symbol := market.Normalize(rawSymbol)
		sideStr, _ := posMap["side"].(string)
		positionAmt, _ := posMap["positionAmt"].(float64)
		entryPrice, _ := posMap["entryPrice"].(float64)
		markPrice, _ := posMap["markPrice"].(float64)
		leverage, _ := posMap["leverage"].(float64)

		// Skip positions with 0 quantity
		if positionAmt == 0 {
			continue
		}

		// Determine position side
		side := "LONG"
		if sideStr == "short" {
			side = "SHORT"
		}

		// Use current mark price as entry price (approximation)
		// If entryPrice is 0, use markPrice
		if entryPrice == 0 {
			entryPrice = markPrice
		}

		snapshotPosition := &store.TraderPosition{
			TraderID:           traderID,
			ExchangeID:         exchangeID,
			ExchangeType:       exchangeType,
			ExchangePositionID: fmt.Sprintf("snapshot_%s_%s_%d", symbol, side, nowMs),
			Symbol:             symbol,
			Side:               side,
			Quantity:           positionAmt,
			EntryPrice:         entryPrice,
			EntryOrderID:       "snapshot", // Mark as snapshot
			EntryTime:          nowMs,
			Leverage:           int(leverage),
			Status:             "OPEN",
			Source:             "snapshot", // Mark source as snapshot
			CreatedAt:          nowMs,
			UpdatedAt:          nowMs,
		}

		if err := positionStore.CreateOpenPosition(snapshotPosition); err != nil {
			logger.Infof("  ⚠️ Failed to create snapshot position for %s %s: %v", symbol, side, err)
			continue
		}

		logger.Infof("  ✅ Created snapshot: %s %s %.6f @ %.2f (leverage: %dx)",
			symbol, side, positionAmt, entryPrice, int(leverage))
		createdCount++
	}

	logger.Infof("✅ Position snapshot complete: %d positions created", createdCount)
	return nil
}

// SyncPositionSnapshotFromMaps refreshes current OPEN positions using already-fetched live positions.
// Historical CLOSED positions are kept intact, and fill-derived OPEN positions are preserved.
func SyncPositionSnapshotFromMaps(traderID, exchangeID, exchangeType string, positions []map[string]interface{}, st *store.Store) error {
	positionStore := st.Position()

	existingOpenPositions, err := positionStore.GetOpenPositions(traderID)
	if err != nil {
		return fmt.Errorf("failed to load existing open positions: %w", err)
	}

	type livePosition struct {
		symbol     string
		side       string
		quantity   float64
		entryPrice float64
		leverage   int
	}

	liveByKey := make(map[string]livePosition)
	nowMs := time.Now().UTC().UnixMilli()
	for _, posMap := range positions {
		rawSymbol, _ := posMap["symbol"].(string)
		symbol := market.Normalize(rawSymbol)
		if symbol == "" {
			continue
		}

		sideStr, _ := posMap["side"].(string)
		positionAmt, _ := posMap["positionAmt"].(float64)
		entryPrice, _ := posMap["entryPrice"].(float64)
		markPrice, _ := posMap["markPrice"].(float64)
		leverage, _ := posMap["leverage"].(float64)
		if positionAmt == 0 {
			continue
		}

		side := "LONG"
		if sideStr == "short" {
			side = "SHORT"
		}
		if entryPrice == 0 {
			entryPrice = markPrice
		}
		size := math.Abs(positionAmt)
		liveByKey[symbol+"|"+side] = livePosition{
			symbol:     symbol,
			side:       side,
			quantity:   size,
			entryPrice: entryPrice,
			leverage:   int(leverage),
		}
	}

	hasNormalOpenByKey := make(map[string]bool)
	snapshotOpenByKey := make(map[string]*store.TraderPosition)
	for _, pos := range existingOpenPositions {
		key := pos.Symbol + "|" + pos.Side
		if pos.Source == "snapshot" {
			snapshotOpenByKey[key] = pos
			continue
		}
		hasNormalOpenByKey[key] = true
	}

	for key, live := range liveByKey {
		if hasNormalOpenByKey[key] {
			continue
		}

		snapshotPosition := &store.TraderPosition{
			TraderID:           traderID,
			ExchangeID:         exchangeID,
			ExchangeType:       exchangeType,
			ExchangePositionID: fmt.Sprintf("snapshot_%s_%s_%d", live.symbol, live.side, nowMs),
			Symbol:             live.symbol,
			Side:               live.side,
			EntryQuantity:      live.quantity,
			Quantity:           live.quantity,
			EntryPrice:         live.entryPrice,
			EntryOrderID:       "snapshot",
			EntryTime:          nowMs,
			Leverage:           live.leverage,
			Status:             "OPEN",
			Source:             "snapshot",
			CreatedAt:          nowMs,
			UpdatedAt:          nowMs,
		}
		if err := positionStore.UpsertSnapshotOpenPosition(snapshotPosition); err != nil {
			return fmt.Errorf("failed to upsert snapshot position for %s %s: %w", live.symbol, live.side, err)
		}
	}

	for key, snapshot := range snapshotOpenByKey {
		if _, stillLive := liveByKey[key]; stillLive {
			continue
		}
		if err := positionStore.CloseOpenSnapshotPosition(snapshot.ID, "snapshot_sync"); err != nil {
			return fmt.Errorf("failed to close stale snapshot position for %s: %w", key, err)
		}
	}

	return nil
}
