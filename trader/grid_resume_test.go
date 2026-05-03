package trader

import (
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
)

func TestQualifiesForGridResumeAllowsLaggingEMANarrowRange(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			IsPaused:   true,
			PausedAt:   time.Now().Add(-10 * time.Minute),
			LowerPrice: 0.093,
			UpperPrice: 0.112,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10708,
		IsPaused:       true,
		BollingerWidth: 0.65,
		EMADistance:    6.84,
		RSI14:          59.7,
		BoxData: &market.BoxData{
			MidLower:     0.096,
			MidUpper:     0.106,
			LongLower:    0.090,
			LongUpper:    0.115,
			CurrentPrice: 0.10708,
		},
	}

	if !at.qualifiesForGridResume(ctx) {
		t.Fatalf("expected narrow range with lagging EMA to qualify for resume")
	}
	reason := at.gridResumeBlockReason(ctx)
	if strings.Contains(reason, "ema distance") {
		t.Fatalf("expected EMA lag not to be reported as blocking, got %q", reason)
	}
	if !strings.Contains(reason, "resume awaiting confirmations") {
		t.Fatalf("expected resume to await confirmations, got %q", reason)
	}
}

func TestQualifiesForGridResumeAllowsBorderlineNarrowRange(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			IsPaused:   true,
			PausedAt:   time.Now().Add(-10 * time.Minute),
			LowerPrice: 0.093,
			UpperPrice: 0.112,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10750,
		IsPaused:       true,
		BollingerWidth: 1.03,
		EMADistance:    7.01,
		RSI14:          66.9,
		BoxData: &market.BoxData{
			MidLower:     0.096,
			MidUpper:     0.106,
			LongLower:    0.090,
			LongUpper:    0.115,
			CurrentPrice: 0.10750,
		},
	}

	if !at.qualifiesForGridResume(ctx) {
		t.Fatalf("expected borderline narrow range with lagging EMA to qualify for resume")
	}
}

func TestQualifiesForGridResumeBlocksWideLaggingEMARange(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			IsPaused:   true,
			PausedAt:   time.Now().Add(-10 * time.Minute),
			LowerPrice: 0.093,
			UpperPrice: 0.112,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10750,
		IsPaused:       true,
		BollingerWidth: 3.05,
		EMADistance:    7.01,
		RSI14:          66.9,
	}

	if at.qualifiesForGridResume(ctx) {
		t.Fatalf("expected range wider than lagging-EMA resume threshold to remain blocked")
	}
}

func TestQualifiesForGridResumeStillBlocksHighEMAWideRange(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			IsPaused:   true,
			PausedAt:   time.Now().Add(-10 * time.Minute),
			LowerPrice: 0.093,
			UpperPrice: 0.112,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10708,
		IsPaused:       true,
		BollingerWidth: 3.05,
		EMADistance:    6.84,
		RSI14:          59.7,
	}

	if at.qualifiesForGridResume(ctx) {
		t.Fatalf("expected wide range with high EMA distance to remain blocked")
	}
	reason := at.gridResumeBlockReason(ctx)
	if !strings.Contains(reason, "bollinger width 3.05 >= 3.0") {
		t.Fatalf("expected Bollinger block reason, got %q", reason)
	}
}

func TestQualifiesForGridResumeBlocksLaggingEMAWhenRSIExtreme(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			IsPaused:   true,
			PausedAt:   time.Now().Add(-10 * time.Minute),
			LowerPrice: 0.093,
			UpperPrice: 0.112,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10708,
		IsPaused:       true,
		BollingerWidth: 0.65,
		EMADistance:    6.84,
		RSI14:          74,
	}

	if at.qualifiesForGridResume(ctx) {
		t.Fatalf("expected overbought lagging EMA setup to remain blocked")
	}
}

func TestQualifiesAsProtectedRangingAllowsLaggingEMAForAutoSeed(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			LowerPrice: 0.09434,
			UpperPrice: 0.11200,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10643,
		BollingerWidth: 1.97,
		EMADistance:    6.96,
		RSI14:          41,
		BoxData: &market.BoxData{
			MidLower:     0.104,
			MidUpper:     0.108,
			LongLower:    0.094,
			LongUpper:    0.112,
			CurrentPrice: 0.10643,
		},
	}

	if !at.qualifiesAsProtectedRanging(ctx) {
		t.Fatalf("expected lagging EMA narrow-range context to qualify for auto-seed")
	}
}

func TestQualifiesAsProtectedRangingBlocksLaggingEMAWhenTooWide(t *testing.T) {
	at := &AutoTrader{
		gridState: &GridState{
			LowerPrice: 0.09434,
			UpperPrice: 0.11200,
		},
	}
	ctx := &kernel.GridContext{
		CurrentPrice:   0.10643,
		BollingerWidth: 3.05,
		EMADistance:    6.96,
		RSI14:          41,
	}

	if at.qualifiesAsProtectedRanging(ctx) {
		t.Fatalf("expected wider lagging EMA context to remain outside protected ranging")
	}
}
