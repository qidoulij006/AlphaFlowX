package sim

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEngineFillsRestingBuyOnSellAggressor(t *testing.T) {
	engine := NewEngine(EngineConfig{
		Symbol:         "DOGEUSDT",
		InitialBalance: 1000,
		MakerFeeRate:   0.0002,
		TakerFeeRate:   0.0004,
		QueueModel:     QueueModelTouch,
	})

	_, err := engine.SubmitLimit(OrderIntent{
		Time:         1000,
		Symbol:       "DOGEUSDT",
		Type:         OrderTypeLimit,
		Side:         SideBuy,
		PositionSide: PositionLong,
		Price:        0.10,
		Quantity:     100,
	})
	if err != nil {
		t.Fatalf("submit limit: %v", err)
	}

	fills := engine.OnTrade(AggTradeEvent{
		Type:      "trade",
		Symbol:    "DOGEUSDT",
		Time:      1100,
		Price:     0.10,
		Quantity:  50,
		TakerSide: SideSell,
	})
	if len(fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(fills))
	}
	if fills[0].Quantity != 50 {
		t.Fatalf("expected partial fill quantity 50, got %.8f", fills[0].Quantity)
	}

	report := engine.Report()
	if len(report.OpenOrders) != 1 {
		t.Fatalf("expected one open order after partial fill, got %d", len(report.OpenOrders))
	}
	if report.Positions[0].Quantity != 50 {
		t.Fatalf("expected long position quantity 50, got %.8f", report.Positions[0].Quantity)
	}
}

func TestEngineReduceOnlyCapsCloseQuantity(t *testing.T) {
	engine := NewEngine(EngineConfig{
		Symbol:         "DOGEUSDT",
		InitialBalance: 1000,
		TakerFeeRate:   0,
		QueueModel:     QueueModelTouch,
	})
	engine.lastPrice = 0.10

	_, err := engine.SubmitMarket(OrderIntent{
		Time:         1000,
		Symbol:       "DOGEUSDT",
		Type:         OrderTypeMarket,
		Side:         SideBuy,
		PositionSide: PositionLong,
		Quantity:     100,
	})
	if err != nil {
		t.Fatalf("open market: %v", err)
	}
	engine.lastPrice = 0.11
	fills, err := engine.SubmitMarket(OrderIntent{
		Time:         2000,
		Symbol:       "DOGEUSDT",
		Type:         OrderTypeMarket,
		Side:         SideSell,
		PositionSide: PositionLong,
		Quantity:     150,
		ReduceOnly:   true,
	})
	if err != nil {
		t.Fatalf("close market: %v", err)
	}
	if len(fills) != 1 {
		t.Fatalf("expected one close fill, got %d", len(fills))
	}
	if fills[0].Quantity != 100 {
		t.Fatalf("expected capped close quantity 100, got %.8f", fills[0].Quantity)
	}
	if fills[0].RealizedPnL < 0.999999 || fills[0].RealizedPnL > 1.000001 {
		t.Fatalf("expected realized pnl 1, got %.8f", fills[0].RealizedPnL)
	}
	if engine.Report().Positions[0].Quantity != 0 {
		t.Fatalf("expected flat long position")
	}
}

func TestEngineSeedPositionAllowsReduceOnlyClose(t *testing.T) {
	engine := NewEngine(EngineConfig{
		Symbol:         "DOGEUSDT",
		InitialBalance: 1000,
		MakerFeeRate:   0,
		QueueModel:     QueueModelTouch,
	})

	if err := engine.SeedPosition("DOGEUSDT", PositionShort, 10, 0.10, 1000); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	_, err := engine.SubmitLimit(OrderIntent{
		Time:         1100,
		Symbol:       "DOGEUSDT",
		Type:         OrderTypeLimit,
		Side:         SideBuy,
		PositionSide: PositionShort,
		Price:        0.09,
		Quantity:     15,
		ReduceOnly:   true,
	})
	if err != nil {
		t.Fatalf("submit reduce-only close: %v", err)
	}

	fills := engine.OnTrade(AggTradeEvent{
		Type:      "trade",
		Symbol:    "DOGEUSDT",
		Time:      1200,
		Price:     0.09,
		Quantity:  20,
		TakerSide: SideSell,
	})
	if len(fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(fills))
	}
	if fills[0].Quantity != 10 {
		t.Fatalf("expected reduce-only fill capped to seeded position, got %.8f", fills[0].Quantity)
	}
	if fills[0].RealizedPnL < 0.099999 || fills[0].RealizedPnL > 0.100001 {
		t.Fatalf("expected realized pnl 0.1, got %.8f", fills[0].RealizedPnL)
	}
	if engine.Report().Positions[1].Quantity != 0 {
		t.Fatalf("expected flat short position")
	}
}

func TestReadOrderIntentsPrioritizesSeedPosition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.ndjson")
	content := `{"time":1000,"action":"submit","client_id":"close-short","symbol":"DOGEUSDT","side":"BUY","position_side":"SHORT","price":0.09,"quantity":10,"reduce_only":true}
{"time":1000,"action":"seed_position","symbol":"DOGEUSDT","position_side":"SHORT","price":0.10,"quantity":10}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write intents: %v", err)
	}

	intents, err := ReadOrderIntents(path)
	if err != nil {
		t.Fatalf("read intents: %v", err)
	}
	if len(intents) != 2 {
		t.Fatalf("expected two intents, got %d", len(intents))
	}
	if intents[0].Action != "seed_position" {
		t.Fatalf("expected seed intent first, got %q", intents[0].Action)
	}
	if intents[0].Type != "" {
		t.Fatalf("seed intent should not default to limit type, got %q", intents[0].Type)
	}
	if intents[1].Type != OrderTypeLimit {
		t.Fatalf("submit intent should default to limit type, got %q", intents[1].Type)
	}
}
