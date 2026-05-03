package api

import (
	"reflect"
	"strings"
)

// MaskSensitiveString Mask sensitive strings, showing only first 4 and last 4 characters
// Used to mask API Key, Secret Key, Private Key and other sensitive information
func MaskSensitiveString(s string) string {
	if s == "" {
		return ""
	}
	length := len(s)
	if length <= 8 {
		return "****" // String too short, hide everything
	}
	return s[:4] + "****" + s[length-4:]
}

// SanitizeModelConfigForLog sanitizes model configuration for log output.
func SanitizeModelConfigForLog(models interface{}) map[string]interface{} {
	safe := make(map[string]interface{})

	rv := reflect.ValueOf(models)
	if rv.Kind() != reflect.Map {
		return safe
	}

	for _, key := range rv.MapKeys() {
		cfg := rv.MapIndex(key)
		if !cfg.IsValid() {
			continue
		}
		if cfg.Kind() == reflect.Interface || cfg.Kind() == reflect.Pointer {
			if cfg.IsNil() {
				continue
			}
			cfg = cfg.Elem()
		}
		if cfg.Kind() != reflect.Struct {
			continue
		}

		safe[key.String()] = map[string]interface{}{
			"name":              readStructStringField(cfg, "Name"),
			"provider":          readStructStringField(cfg, "Provider"),
			"enabled":           readStructBoolField(cfg, "Enabled"),
			"api_key":           MaskSensitiveString(readStructStringField(cfg, "APIKey")),
			"custom_api_url":    readStructStringField(cfg, "CustomAPIURL"),
			"custom_model_name": readStructStringField(cfg, "CustomModelName"),
		}
	}
	return safe
}

// SanitizeExchangeConfigForLog Sanitize exchange configuration for log output
func SanitizeExchangeConfigForLog(exchanges interface{}) map[string]interface{} {
	safe := make(map[string]interface{})
	rv := reflect.ValueOf(exchanges)
	if rv.Kind() != reflect.Map {
		return safe
	}

	for _, key := range rv.MapKeys() {
		cfgValue := rv.MapIndex(key)
		if !cfgValue.IsValid() {
			continue
		}
		cfg := cfgValue
		if cfg.Kind() == reflect.Interface || cfg.Kind() == reflect.Pointer {
			if cfg.IsNil() {
				continue
			}
			cfg = cfg.Elem()
		}
		if cfg.Kind() != reflect.Struct {
			continue
		}

		safeExchange := map[string]interface{}{
			"enabled": readStructBoolField(cfg, "Enabled"),
			"testnet": readStructBoolField(cfg, "Testnet"),
		}

		// Only add masked sensitive fields when they have values
		if value := readStructStringField(cfg, "APIKey"); value != "" {
			safeExchange["api_key"] = MaskSensitiveString(value)
		}
		if value := readStructStringField(cfg, "SecretKey"); value != "" {
			safeExchange["secret_key"] = MaskSensitiveString(value)
		}
		if value := readStructStringField(cfg, "Passphrase"); value != "" {
			safeExchange["passphrase"] = MaskSensitiveString(value)
		}
		if value := readStructStringField(cfg, "AsterPrivateKey"); value != "" {
			safeExchange["aster_private_key"] = MaskSensitiveString(value)
		}
		if value := readStructStringField(cfg, "LighterPrivateKey"); value != "" {
			safeExchange["lighter_private_key"] = MaskSensitiveString(value)
		}
		if value := readStructStringField(cfg, "LighterAPIKeyPrivateKey"); value != "" {
			safeExchange["lighter_api_key_private_key"] = MaskSensitiveString(value)
		}

		// Add non-sensitive fields directly
		if value := readStructStringField(cfg, "HyperliquidWalletAddr"); value != "" {
			safeExchange["hyperliquid_wallet_addr"] = value
		}
		if value := readStructStringField(cfg, "AsterUser"); value != "" {
			safeExchange["aster_user"] = value
		}
		if value := readStructStringField(cfg, "AsterSigner"); value != "" {
			safeExchange["aster_signer"] = value
		}
		if value := readStructStringField(cfg, "LighterWalletAddr"); value != "" {
			safeExchange["lighter_wallet_addr"] = value
		}

		safe[key.String()] = safeExchange
	}
	return safe
}

func readStructStringField(v reflect.Value, fieldName string) string {
	field := v.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func readStructBoolField(v reflect.Value, fieldName string) bool {
	field := v.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Bool {
		return false
	}
	return field.Bool()
}

// MaskEmail Mask email address, keeping first 2 characters and domain part
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****" // Incorrect format
	}
	username := parts[0]
	domain := parts[1]
	if len(username) <= 2 {
		return "**@" + domain
	}
	return username[:2] + "****@" + domain
}
