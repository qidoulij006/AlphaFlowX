package trader

import (
	"testing"

	"nofx/store"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	st, err := store.NewFromGorm(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := st.Position().InitTables(); err != nil {
		t.Fatalf("init position tables: %v", err)
	}
	return st
}

func TestSyncPositionSnapshotPreservesNormalOpenPositions(t *testing.T) {
	st := newTestStore(t)
	positionStore := st.Position()

	err := positionStore.CreateOpenPosition(&store.TraderPosition{
		TraderID:     "trader-1",
		ExchangeID:   "ex-1",
		ExchangeType: "binance",
		Symbol:       "DOGEUSDT",
		Side:         "SHORT",
		Quantity:     120,
		EntryPrice:   0.095,
		EntryOrderID: "fill-order-1",
		EntryTime:    1000,
		Status:       "OPEN",
		Source:       "system",
		CreatedAt:    1000,
		UpdatedAt:    1000,
	})
	if err != nil {
		t.Fatalf("create normal open position: %v", err)
	}

	err = positionStore.CreateOpenPosition(&store.TraderPosition{
		TraderID:           "trader-1",
		ExchangeID:         "ex-1",
		ExchangeType:       "binance",
		ExchangePositionID: "snapshot_old",
		Symbol:             "BTCUSDT",
		Side:               "LONG",
		Quantity:           1,
		EntryQuantity:      1,
		EntryPrice:         65000,
		EntryOrderID:       "snapshot",
		EntryTime:          1001,
		Status:             "OPEN",
		Source:             "snapshot",
		CreatedAt:          1001,
		UpdatedAt:          1001,
	})
	if err != nil {
		t.Fatalf("create snapshot open position: %v", err)
	}

	livePositions := []map[string]interface{}{
		{
			"symbol":      "DOGEUSDT",
			"side":        "short",
			"positionAmt": 120.0,
			"entryPrice":  0.095,
			"markPrice":   0.0951,
			"leverage":    5.0,
		},
		{
			"symbol":      "ETHUSDT",
			"side":        "long",
			"positionAmt": 2.0,
			"entryPrice":  3200.0,
			"markPrice":   3201.0,
			"leverage":    3.0,
		},
	}

	if err := SyncPositionSnapshotFromMaps("trader-1", "ex-1", "binance", livePositions, st); err != nil {
		t.Fatalf("sync position snapshot: %v", err)
	}

	openPositions, err := positionStore.GetOpenPositions("trader-1")
	if err != nil {
		t.Fatalf("get open positions: %v", err)
	}
	if len(openPositions) != 2 {
		t.Fatalf("expected 2 open positions after sync, got %d", len(openPositions))
	}

	var dogeSystem, ethSnapshot bool
	for _, pos := range openPositions {
		switch pos.Symbol + "|" + pos.Side {
		case "DOGEUSDT|SHORT":
			if pos.Source != "system" {
				t.Fatalf("expected DOGE short to remain system position, got source=%s", pos.Source)
			}
			dogeSystem = true
		case "ETHUSDT|LONG":
			if pos.Source != "snapshot" {
				t.Fatalf("expected ETH long to be recovered as snapshot, got source=%s", pos.Source)
			}
			ethSnapshot = true
		case "BTCUSDT|LONG":
			t.Fatalf("stale BTC snapshot should have been closed, still open")
		}
	}
	if !dogeSystem || !ethSnapshot {
		t.Fatalf("missing expected open positions after sync: dogeSystem=%v ethSnapshot=%v", dogeSystem, ethSnapshot)
	}
}

func TestFallbackExitMeetsMinNetProfit(t *testing.T) {
	at := &AutoTrader{}

	if at.fallbackExitMeetsMinNetProfit("LONG", 100, 100.05, 1) {
		t.Fatalf("expected low-edge long fallback exit to be rejected")
	}
	if !at.fallbackExitMeetsMinNetProfit("LONG", 100, 100.25, 1) {
		t.Fatalf("expected sufficiently profitable long fallback exit to be accepted")
	}
	if at.fallbackExitMeetsMinNetProfit("SHORT", 100, 99.95, 1) {
		t.Fatalf("expected low-edge short fallback exit to be rejected")
	}
	if !at.fallbackExitMeetsMinNetProfit("SHORT", 100, 99.7, 1) {
		t.Fatalf("expected sufficiently profitable short fallback exit to be accepted")
	}
}
