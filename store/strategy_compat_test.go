package store

import (
	"encoding/json"
	"testing"
)

func TestIndicatorConfigUnmarshalLegacyKey(t *testing.T) {
	var cfg IndicatorConfig
	if err := json.Unmarshal([]byte(`{"nofxos_api_key":"legacy-key"}`), &cfg); err != nil {
		t.Fatalf("unmarshal legacy key: %v", err)
	}

	if cfg.NofxOSAPIKey != "legacy-key" {
		t.Fatalf("expected legacy key to be preserved, got %q", cfg.NofxOSAPIKey)
	}
	if cfg.ExternalDataKey != "legacy-key" {
		t.Fatalf("expected external data key to mirror legacy key, got %q", cfg.ExternalDataKey)
	}
}

func TestIndicatorConfigUnmarshalNewKey(t *testing.T) {
	var cfg IndicatorConfig
	if err := json.Unmarshal([]byte(`{"external_data_key":"new-key"}`), &cfg); err != nil {
		t.Fatalf("unmarshal new key: %v", err)
	}

	if cfg.ExternalDataKey != "new-key" {
		t.Fatalf("expected new key to be preserved, got %q", cfg.ExternalDataKey)
	}
	if cfg.NofxOSAPIKey != "new-key" {
		t.Fatalf("expected legacy key to mirror new key, got %q", cfg.NofxOSAPIKey)
	}
}

func TestIndicatorConfigMarshalWritesBothKeys(t *testing.T) {
	cfg := IndicatorConfig{
		ExternalDataKey: "shared-key",
	}

	body, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode marshaled config: %v", err)
	}

	if decoded["external_data_key"] != "shared-key" {
		t.Fatalf("expected external_data_key to be written, got %q", decoded["external_data_key"])
	}
	if decoded["nofxos_api_key"] != "shared-key" {
		t.Fatalf("expected nofxos_api_key to be written for compatibility, got %q", decoded["nofxos_api_key"])
	}
}

func TestNormalizeStrategyConfigJSONBackfillsNewField(t *testing.T) {
	raw := `{"language":"zh","indicators":{"nofxos_api_key":"legacy-key","enable_quant_data":true},"custom":"keep"}`

	normalized, changed, err := normalizeStrategyConfigJSON(raw)
	if err != nil {
		t.Fatalf("normalize strategy json: %v", err)
	}
	if !changed {
		t.Fatalf("expected config to be changed")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(normalized), &decoded); err != nil {
		t.Fatalf("decode normalized config: %v", err)
	}

	indicators := decoded["indicators"].(map[string]any)
	if indicators["external_data_key"] != "legacy-key" {
		t.Fatalf("expected external_data_key to be backfilled, got %v", indicators["external_data_key"])
	}
	if decoded["custom"] != "keep" {
		t.Fatalf("expected unrelated fields to be preserved, got %v", decoded["custom"])
	}
}

func TestNormalizeStrategyConfigJSONBackfillsLegacyField(t *testing.T) {
	raw := `{"indicators":{"external_data_key":"new-key"}}`

	normalized, changed, err := normalizeStrategyConfigJSON(raw)
	if err != nil {
		t.Fatalf("normalize strategy json: %v", err)
	}
	if !changed {
		t.Fatalf("expected config to be changed")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(normalized), &decoded); err != nil {
		t.Fatalf("decode normalized config: %v", err)
	}

	indicators := decoded["indicators"].(map[string]any)
	if indicators["nofxos_api_key"] != "new-key" {
		t.Fatalf("expected nofxos_api_key to be backfilled, got %v", indicators["nofxos_api_key"])
	}
}
