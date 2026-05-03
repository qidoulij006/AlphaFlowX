package trader

import (
	"nofx/store"
	"testing"
)

func TestShouldManageExchangeTPSL_DefaultsToTrue(t *testing.T) {
	at := &AutoTrader{}
	if !at.shouldManageExchangeTPSL() {
		t.Fatalf("expected TP/SL management to be enabled without strategy config")
	}
}

func TestShouldManageExchangeTPSL_DisablesForGridStrategy(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				StrategyType: "grid_trading",
				GridConfig:   &store.GridStrategyConfig{Symbol: "DOGEUSDT"},
			},
		},
	}
	if at.shouldManageExchangeTPSL() {
		t.Fatalf("expected TP/SL management to be disabled for grid strategy")
	}
}

func TestShouldManageExchangeTPSL_EnabledForNonGridStrategy(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				StrategyType: "ai_trading",
			},
		},
	}
	if !at.shouldManageExchangeTPSL() {
		t.Fatalf("expected TP/SL management to remain enabled for non-grid strategy")
	}
}
