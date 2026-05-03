package trader

import (
	"testing"
	"time"

	"nofx/kernel"
	"nofx/store"
)

func newTestGridStore(t *testing.T) *store.Store {
	t.Helper()
	st := newTestStore(t)
	if err := st.Grid().InitTables(); err != nil {
		t.Fatalf("init grid tables: %v", err)
	}
	return st
}

func TestUpsertGridInventoryLotPreservesDistinctEntryOrders(t *testing.T) {
	st := newTestGridStore(t)
	at := &AutoTrader{
		id:    "trader-1",
		store: st,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				GridConfig: &store.GridStrategyConfig{Symbol: "DOGEUSDT"},
			},
		},
	}

	if err := at.upsertGridInventoryLot(8, "LONG", 26153.5566, 0.09559, "entry-order-1"); err != nil {
		t.Fatalf("upsert first lot: %v", err)
	}
	if err := at.upsertGridInventoryLot(8, "LONG", 26154.4434, 0.09559, "entry-order-2"); err != nil {
		t.Fatalf("upsert second lot: %v", err)
	}

	lots, err := st.Grid().LoadOpenInventoryLots("trader-1", "DOGEUSDT")
	if err != nil {
		t.Fatalf("load lots: %v", err)
	}
	if len(lots) != 2 {
		t.Fatalf("expected 2 open lots, got %d", len(lots))
	}

	totalQty := 0.0
	seenOrders := map[string]bool{}
	for _, lot := range lots {
		totalQty += lot.RemainingQty
		seenOrders[lot.EntryOrderID] = true
	}
	if !seenOrders["entry-order-1"] || !seenOrders["entry-order-2"] {
		t.Fatalf("expected both entry orders to remain distinct, got %+v", seenOrders)
	}
	if totalQty < 52307.9 || totalQty > 52308.1 {
		t.Fatalf("expected total qty around 52308, got %.4f", totalQty)
	}
}

func TestReconcileGridInventoryLotsDoesNotOverwriteRealLotWithSyntheticResidual(t *testing.T) {
	st := newTestGridStore(t)
	now := time.Now()
	if err := st.Grid().SaveInventoryLot(&store.GridInventoryLotModel{
		TraderID:         "trader-1",
		Symbol:           "DOGEUSDT",
		PositionSide:     "LONG",
		SourceLevelIndex: 8,
		ExitLevelIndex:   9,
		EntryPrice:       0.09559,
		EntryQuantity:    26153.5566,
		RemainingQty:     26153.5566,
		EntryOrderID:     "entry-order-1",
		Status:           "OPEN",
		OpenedAt:         now,
	}); err != nil {
		t.Fatalf("seed real lot: %v", err)
	}

	at := &AutoTrader{
		id:    "trader-1",
		store: st,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				GridConfig: &store.GridStrategyConfig{Symbol: "DOGEUSDT"},
			},
		},
		gridState: NewGridState(&store.GridStrategyConfig{Symbol: "DOGEUSDT"}),
	}
	at.gridState.Levels = []kernel.GridLevelInfo{
		{Price: 0.09438},
		{Price: 0.09499},
		{Price: 0.09559},
		{Price: 0.09618},
	}

	positions := []map[string]interface{}{
		{
			"symbol":      "DOGEUSDT",
			"side":        "LONG",
			"positionAmt": 52308.0,
			"entryPrice":  0.09559,
		},
	}

	at.reconcileGridInventoryLotsWithExchangePositions("DOGEUSDT", positions)

	lots, err := st.Grid().LoadOpenInventoryLots("trader-1", "DOGEUSDT")
	if err != nil {
		t.Fatalf("load lots after reconcile: %v", err)
	}
	if len(lots) != 2 {
		t.Fatalf("expected 2 open lots after reconcile, got %d", len(lots))
	}

	realLotCount := 0
	syntheticLotCount := 0
	totalQty := 0.0
	for _, lot := range lots {
		totalQty += lot.RemainingQty
		if lot.EntryOrderID == "" {
			syntheticLotCount++
		} else if lot.EntryOrderID == "entry-order-1" {
			realLotCount++
		}
	}

	if realLotCount != 1 || syntheticLotCount != 1 {
		t.Fatalf("expected one real lot and one synthetic lot, got real=%d synthetic=%d", realLotCount, syntheticLotCount)
	}
	if totalQty < 52307.9 || totalQty > 52308.1 {
		t.Fatalf("expected total qty around 52308 after reconcile, got %.4f", totalQty)
	}
}
