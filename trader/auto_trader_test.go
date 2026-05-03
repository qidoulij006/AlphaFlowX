package trader

import "testing"

func TestSanitizeRuntimeConfigClearsSensitiveFields(t *testing.T) {
	cfg := AutoTraderConfig{
		BinanceAPIKey:           "binance-key",
		BinanceSecretKey:        "binance-secret",
		DeepSeekKey:             "deepseek-key",
		CustomAPIKey:            "custom-key",
		HyperliquidPrivateKey:   "hyper-key",
		LighterAPIKeyPrivateKey: "lighter-key",
		CustomAPIURL:            "https://api.example.com",
		CustomModelName:         "model-a",
	}

	sanitizeRuntimeConfig(&cfg)

	if cfg.BinanceAPIKey != "" || cfg.BinanceSecretKey != "" {
		t.Fatal("expected exchange credentials to be cleared")
	}
	if cfg.DeepSeekKey != "" || cfg.CustomAPIKey != "" {
		t.Fatal("expected AI credentials to be cleared")
	}
	if cfg.HyperliquidPrivateKey != "" || cfg.LighterAPIKeyPrivateKey != "" {
		t.Fatal("expected private keys to be cleared")
	}
	if cfg.CustomAPIURL == "" || cfg.CustomModelName == "" {
		t.Fatal("expected non-sensitive custom endpoint metadata to be preserved")
	}
}

func TestSanitizeCustomAPIURL(t *testing.T) {
	if got := sanitizeCustomAPIURL("https://api.example.com/v1"); got == "" {
		t.Fatal("expected public custom API URL to be preserved")
	}

	if got := sanitizeCustomAPIURL("http://127.0.0.1:8080"); got != "" {
		t.Fatal("expected loopback custom API URL to be rejected")
	}

	if got := sanitizeCustomAPIURL("https://api.example.com/v1#"); got == "" {
		t.Fatal("expected full custom API URL marker to be preserved when safe")
	}
}
