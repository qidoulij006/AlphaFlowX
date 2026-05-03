package binance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/stretchr/testify/assert"
	"nofx/trader/testutil"
	"nofx/trader/types"
)

// ============================================================
// 1. BinanceFuturesTestSuite - Inherits base test suite
// ============================================================

// BinanceFuturesTestSuite Binance Futures trader test suite
// Inherits TraderTestSuite and adds Binance Futures specific mock logic
type BinanceFuturesTestSuite struct {
	*testutil.TraderTestSuite // Embeds base test suite
	mockServer                *httptest.Server
}

// NewBinanceFuturesTestSuite Creates Binance Futures test suite
func NewBinanceFuturesTestSuite(t *testing.T) *BinanceFuturesTestSuite {
	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different mock responses based on URL path
		path := r.URL.Path

		var respBody interface{}

		switch {
		// Mock GetBalance - /fapi/v2/balance
		case path == "/fapi/v2/balance":
			respBody = []map[string]interface{}{
				{
					"accountAlias":       "test",
					"asset":              "USDT",
					"balance":            "10000.00",
					"crossWalletBalance": "10000.00",
					"crossUnPnl":         "100.50",
					"availableBalance":   "8000.00",
					"maxWithdrawAmount":  "8000.00",
				},
			}

		// Mock GetAccount - /fapi/v2/account
		case path == "/fapi/v2/account":
			respBody = map[string]interface{}{
				"totalWalletBalance":    "10000.00",
				"availableBalance":      "8000.00",
				"totalUnrealizedProfit": "100.50",
				"assets": []map[string]interface{}{
					{
						"asset":                  "USDT",
						"walletBalance":          "10000.00",
						"unrealizedProfit":       "100.50",
						"marginBalance":          "10100.50",
						"maintMargin":            "200.00",
						"initialMargin":          "2000.00",
						"positionInitialMargin":  "2000.00",
						"openOrderInitialMargin": "0.00",
						"crossWalletBalance":     "10000.00",
						"crossUnPnl":             "100.50",
						"availableBalance":       "8000.00",
						"maxWithdrawAmount":      "8000.00",
					},
				},
			}

		// Mock GetPositions - /fapi/v2/positionRisk
		case path == "/fapi/v2/positionRisk":
			respBody = []map[string]interface{}{
				{
					"symbol":           "BTCUSDT",
					"positionAmt":      "0.5",
					"entryPrice":       "50000.00",
					"markPrice":        "50500.00",
					"unRealizedProfit": "250.00",
					"liquidationPrice": "45000.00",
					"leverage":         "10",
					"positionSide":     "LONG",
				},
			}

		// Mock GetMarketPrice - /fapi/v1/ticker/price and /fapi/v2/ticker/price
		case path == "/fapi/v1/ticker/price" || path == "/fapi/v2/ticker/price":
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				// Return all prices
				respBody = []map[string]interface{}{
					{"Symbol": "BTCUSDT", "Price": "50000.00", "Time": 1234567890},
					{"Symbol": "ETHUSDT", "Price": "3000.00", "Time": 1234567890},
				}
			} else if symbol == "INVALIDUSDT" {
				// Return error
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1121,
					"msg":  "Invalid symbol.",
				})
				return
			} else {
				// Return single price (note: even with symbol parameter, return array)
				price := "50000.00"
				if symbol == "ETHUSDT" {
					price = "3000.00"
				}
				respBody = []map[string]interface{}{
					{
						"Symbol": symbol,
						"Price":  price,
						"Time":   1234567890,
					},
				}
			}

		// Mock ExchangeInfo - /fapi/v1/exchangeInfo
		case path == "/fapi/v1/exchangeInfo":
			respBody = map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol":             "BTCUSDT",
						"status":             "TRADING",
						"baseAsset":          "BTC",
						"quoteAsset":         "USDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"minPrice":   "0.01",
								"maxPrice":   "1000000",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"minQty":     "0.001",
								"maxQty":     "10000",
								"stepSize":   "0.001",
							},
						},
					},
					{
						"symbol":             "ETHUSDT",
						"status":             "TRADING",
						"baseAsset":          "ETH",
						"quoteAsset":         "USDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"minPrice":   "0.01",
								"maxPrice":   "100000",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"minQty":     "0.001",
								"maxQty":     "10000",
								"stepSize":   "0.001",
							},
						},
					},
				},
			}

		// Mock CreateOrder - /fapi/v1/order (POST)
		case path == "/fapi/v1/order" && r.Method == "POST":
			symbol := r.FormValue("symbol")
			if symbol == "" {
				symbol = "BTCUSDT"
			}
			respBody = map[string]interface{}{
				"orderId":       123456,
				"symbol":        symbol,
				"status":        "FILLED",
				"clientOrderId": r.FormValue("newClientOrderId"),
				"price":         r.FormValue("price"),
				"avgPrice":      r.FormValue("price"),
				"origQty":       r.FormValue("quantity"),
				"executedQty":   r.FormValue("quantity"),
				"cumQty":        r.FormValue("quantity"),
				"cumQuote":      "1000.00",
				"timeInForce":   r.FormValue("timeInForce"),
				"type":          r.FormValue("type"),
				"reduceOnly":    r.FormValue("reduceOnly") == "true",
				"side":          r.FormValue("side"),
				"positionSide":  r.FormValue("positionSide"),
				"stopPrice":     r.FormValue("stopPrice"),
				"workingType":   r.FormValue("workingType"),
			}

		// Mock CancelOrder - /fapi/v1/order (DELETE)
		case path == "/fapi/v1/order" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  r.URL.Query().Get("symbol"),
				"status":  "CANCELED",
			}

		// Mock ListOpenOrders - /fapi/v1/openOrders
		case path == "/fapi/v1/openOrders":
			respBody = []map[string]interface{}{}

		// Mock CancelAllOrders - /fapi/v1/allOpenOrders (DELETE)
		case path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "The operation of cancel all open order is done.",
			}

		// Mock SetLeverage - /fapi/v1/leverage
		case path == "/fapi/v1/leverage":
			// Convert string to integer
			leverageStr := r.FormValue("leverage")
			leverage := 10 // default value
			if leverageStr != "" {
				// Note: here we return an integer directly, not a string
				fmt.Sscanf(leverageStr, "%d", &leverage)
			}
			respBody = map[string]interface{}{
				"leverage":         leverage,
				"maxNotionalValue": "1000000",
				"symbol":           r.FormValue("symbol"),
			}

		// Mock SetMarginType - /fapi/v1/marginType
		case path == "/fapi/v1/marginType":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Mock ChangePositionMode - /fapi/v1/positionSide/dual
		case path == "/fapi/v1/positionSide/dual":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Mock ServerTime - /fapi/v1/time
		case path == "/fapi/v1/time":
			respBody = map[string]interface{}{
				"serverTime": 1234567890000,
			}

		// Default: empty response
		default:
			respBody = map[string]interface{}{}
		}

		// Serialize response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// Create futures.Client and configure to use mock server
	client := futures.NewClient("test_api_key", "test_secret_key")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// Create FuturesTrader
	traderInstance := &FuturesTrader{
		client:        client,
		cacheDuration: 0, // disable cache for testing
	}

	// Create base suite
	baseSuite := testutil.NewTraderTestSuite(t, traderInstance)

	return &BinanceFuturesTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup cleans up resources
func (s *BinanceFuturesTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 2. Run common tests using BinanceFuturesTestSuite
// ============================================================

// TestFuturesTrader_InterfaceCompliance tests interface compliance
func TestFuturesTrader_InterfaceCompliance(t *testing.T) {
	var _ types.Trader = (*FuturesTrader)(nil)
}

// TestFuturesTrader_CommonInterface runs all common interface tests using test suite
func TestFuturesTrader_CommonInterface(t *testing.T) {
	// Create test suite
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	// Run all common interface tests
	suite.RunAllTests()
}

// ============================================================
// 3. Binance Futures specific unit tests
// ============================================================

// TestNewFuturesTrader tests creating Binance Futures trader
func TestNewFuturesTrader(t *testing.T) {
	t1 := NewFuturesTrader("test_api_key", "test_secret_key", "test_user")

	assert.NotNil(t, t1)
	assert.NotNil(t, t1.client)
	assert.Equal(t, 5*time.Second, t1.cacheDuration)
}

func TestInitializeSessionSkipsRESTDuringCooldown(t *testing.T) {
	requestCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer mockServer.Close()

	ft := NewFuturesTrader("test_api_key", "test_secret_key", "test_user")
	ft.client.BaseURL = mockServer.URL
	ft.client.HTTPClient = mockServer.Client()
	ft.cooldownUntil = time.Now().UTC().Add(1 * time.Minute)

	ft.InitializeSession()

	assert.Equal(t, 0, requestCount, "InitializeSession should not call REST while cooldown is active")
}

func TestApplyAccountUpdateBuildsUserStreamSnapshots(t *testing.T) {
	ft := NewFuturesTrader("test_api_key", "test_secret_key", "test_user")
	ft.cachedPositions = []map[string]interface{}{
		{
			"symbol":           "BTCUSDT",
			"side":             "long",
			"leverage":         20.0,
			"liquidationPrice": 45000.0,
		},
	}

	ft.applyAccountUpdate(futures.WsAccountUpdate{
		Balances: []futures.WsBalance{
			{Asset: "USDT", Balance: "100.5", CrossWalletBalance: "80.25"},
		},
		Positions: []futures.WsPosition{
			{
				Symbol:        "BTCUSDT",
				Side:          futures.PositionSideTypeLong,
				Amount:        "0.5",
				EntryPrice:    "50000",
				MarkPrice:     "50500",
				UnrealizedPnL: "250",
			},
		},
	})

	balance, ok := ft.getUserStreamBalanceSnapshot()
	assert.True(t, ok)
	assert.Equal(t, 100.5, balance["totalWalletBalance"])
	assert.Equal(t, 80.25, balance["availableBalance"])
	assert.Equal(t, 250.0, balance["totalUnrealizedProfit"])

	positions, ok := ft.getUserStreamPositionSnapshot()
	assert.True(t, ok)
	if assert.Len(t, positions, 1) {
		assert.Equal(t, "BTCUSDT", positions[0]["symbol"])
		assert.Equal(t, "long", positions[0]["side"])
		assert.Equal(t, 20.0, positions[0]["leverage"])
		assert.Equal(t, 45000.0, positions[0]["liquidationPrice"])
	}
}

func TestGetBalanceAndPositionsPreferUserStreamSnapshots(t *testing.T) {
	ft := NewFuturesTrader("test_api_key", "test_secret_key", "test_user")
	ft.applyAccountUpdate(futures.WsAccountUpdate{
		Balances: []futures.WsBalance{
			{Asset: "USDT", Balance: "120", CrossWalletBalance: "90"},
		},
		Positions: []futures.WsPosition{
			{
				Symbol:        "ETHUSDT",
				Side:          futures.PositionSideTypeShort,
				Amount:        "-1.25",
				EntryPrice:    "3200",
				MarkPrice:     "3100",
				UnrealizedPnL: "125",
			},
		},
	})

	balance, err := ft.GetBalance()
	assert.NoError(t, err)
	assert.Equal(t, 120.0, balance["totalWalletBalance"])

	positions, err := ft.GetPositions()
	assert.NoError(t, err)
	if assert.Len(t, positions, 1) {
		assert.Equal(t, "ETHUSDT", positions[0]["symbol"])
		assert.Equal(t, "short", positions[0]["side"])
	}
}

func TestPlaceLimitOrderDoesNotSendReduceOnlyParam(t *testing.T) {
	var reduceOnlyValues []string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/fapi/v1/leverage":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"leverage":         10,
				"maxNotionalValue": "1000000",
				"symbol":           r.FormValue("symbol"),
			})
		case path == "/fapi/v1/order" && r.Method == "POST":
			reduceOnlyValues = append(reduceOnlyValues, r.FormValue("reduceOnly"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId":       123456,
				"symbol":        r.FormValue("symbol"),
				"status":        "NEW",
				"clientOrderId": r.FormValue("newClientOrderId"),
				"side":          r.FormValue("side"),
				"positionSide":  r.FormValue("positionSide"),
			})
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer mockServer.Close()

	client := futures.NewClient("test_api_key", "test_secret_key")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	ft := &FuturesTrader{
		client:        client,
		cacheDuration: 0,
	}

	_, err := ft.PlaceLimitOrder(&types.LimitOrderRequest{
		Symbol:       "BTCUSDT",
		Side:         "SELL",
		PositionSide: "LONG",
		Price:        50000,
		Quantity:     0.01,
		Leverage:     10,
		ReduceOnly:   true,
		ClientID:     "grid-test",
	})

	assert.NoError(t, err)
	if assert.Len(t, reduceOnlyValues, 1) {
		assert.Equal(t, "", reduceOnlyValues[0], "hedge-mode Binance limit orders must not send reduceOnly")
	}
}

// TestCalculatePositionSize tests position size calculation
func TestCalculatePositionSize(t *testing.T) {
	ft := &FuturesTrader{}

	tests := []struct {
		name         string
		balance      float64
		riskPercent  float64
		price        float64
		leverage     int
		wantQuantity float64
	}{
		{
			name:         "normal calculation",
			balance:      10000,
			riskPercent:  2,
			price:        50000,
			leverage:     10,
			wantQuantity: 0.04, // (10000 * 0.02 * 10) / 50000 = 0.04
		},
		{
			name:         "high leverage",
			balance:      10000,
			riskPercent:  1,
			price:        3000,
			leverage:     20,
			wantQuantity: 0.6667, // (10000 * 0.01 * 20) / 3000 = 0.6667
		},
		{
			name:         "low risk",
			balance:      5000,
			riskPercent:  0.5,
			price:        50000,
			leverage:     5,
			wantQuantity: 0.0025, // (5000 * 0.005 * 5) / 50000 = 0.0025
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quantity := ft.CalculatePositionSize(tt.balance, tt.riskPercent, tt.price, tt.leverage)
			assert.InDelta(t, tt.wantQuantity, quantity, 0.0001, "calculated position size is incorrect")
		})
	}
}

// TestGetBrOrderID tests order ID generation
func TestGetBrOrderID(t *testing.T) {
	// Test 3 times to ensure each generated ID is unique
	ids := make(map[string]bool)
	for i := 0; i < 3; i++ {
		id := getBrOrderID()

		// Check format
		assert.True(t, strings.HasPrefix(id, "x-KzrpZaP9"), "order ID should start with x-KzrpZaP9")

		// Check length (should be <= 32)
		assert.LessOrEqual(t, len(id), 32, "order ID length should not exceed 32 characters")

		// Check uniqueness
		assert.False(t, ids[id], "order ID should be unique")
		ids[id] = true
	}
}
