package trader

import (
	"errors"
	"testing"
	"time"
)

type rateLimitTestTrader struct {
	balanceErr error
	positions  []map[string]interface{}
}

func (t *rateLimitTestTrader) GetBalance() (map[string]interface{}, error) {
	if t.balanceErr != nil {
		return nil, t.balanceErr
	}
	return map[string]interface{}{
		"totalWalletBalance":    100.0,
		"totalUnrealizedProfit": 5.0,
		"availableBalance":      80.0,
	}, nil
}

func (t *rateLimitTestTrader) GetPositions() ([]map[string]interface{}, error) {
	return t.positions, nil
}

func (t *rateLimitTestTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}

func (t *rateLimitTestTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}

func (t *rateLimitTestTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}

func (t *rateLimitTestTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}

func (t *rateLimitTestTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (t *rateLimitTestTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (t *rateLimitTestTrader) GetMarketPrice(symbol string) (float64, error) {
	return 0, nil
}

func (t *rateLimitTestTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (t *rateLimitTestTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (t *rateLimitTestTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (t *rateLimitTestTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (t *rateLimitTestTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (t *rateLimitTestTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (t *rateLimitTestTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}

func (t *rateLimitTestTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}

func (t *rateLimitTestTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}

func (t *rateLimitTestTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	return nil, nil
}

func TestGetCachedAccountInfoOnRateLimit(t *testing.T) {
	at := &AutoTrader{
		name: "rate-limit-test",
		accountInfoCache: map[string]interface{}{
			"total_equity":      105.0,
			"wallet_balance":    100.0,
			"unrealized_profit": 5.0,
			"available_balance": 80.0,
		},
		accountInfoCacheTime: time.Now().Add(-5 * time.Second),
	}

	cached, ok := at.getCachedAccountInfoOnRateLimit(
		errors.New("<APIError> code=-1003, msg=Way too many requests; IP banned until 1774624436940"),
		"test",
	)
	if !ok {
		t.Fatalf("expected cached snapshot for rate-limited error")
	}
	if cached["total_equity"] != 105.0 {
		t.Fatalf("expected cached total_equity, got %#v", cached["total_equity"])
	}
}

func TestGetAccountInfoFallsBackToCachedSnapshotOnRateLimit(t *testing.T) {
	at := &AutoTrader{
		name:           "rate-limit-test",
		trader:         &rateLimitTestTrader{balanceErr: errors.New("<APIError> code=-1003, msg=Way too many requests")},
		initialBalance: 100.0,
		accountInfoCache: map[string]interface{}{
			"total_equity":      105.0,
			"wallet_balance":    100.0,
			"unrealized_profit": 5.0,
			"available_balance": 80.0,
			"total_pnl":         5.0,
			"total_pnl_pct":     5.0,
			"position_count":    1,
			"margin_used":       10.0,
			"margin_used_pct":   9.5,
		},
		accountInfoCacheTime: time.Now().Add(-10 * time.Second),
	}

	info, err := at.GetAccountInfo()
	if err != nil {
		t.Fatalf("expected cached account info fallback, got error: %v", err)
	}
	if info["total_equity"] != 105.0 {
		t.Fatalf("expected cached total_equity, got %#v", info["total_equity"])
	}
	if info["position_count"] != 1 {
		t.Fatalf("expected cached position_count, got %#v", info["position_count"])
	}
}
