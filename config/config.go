package config

import (
	"crypto/rand"
	"encoding/base64"
	"nofx/mcp"
	"nofx/telemetry"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Global configuration instance
var global *Config

// Config is the global configuration (loaded from .env)
// Only contains truly global config, trading related config is at trader/strategy level
type Config struct {
	// Service configuration
	APIServerPort           int
	JWTSecret               string
	CORSAllowedOrigins      []string
	AllowPublicRegistration bool
	AdminMode               bool
	AdminPassword           string
	NofxOSEnabled           bool

	// Database configuration
	DBType     string // sqlite or postgres
	DBPath     string // SQLite database file path
	DBHost     string // PostgreSQL host
	DBPort     int    // PostgreSQL port
	DBUser     string // PostgreSQL user
	DBPassword string // PostgreSQL password
	DBName     string // PostgreSQL database name
	DBSSLMode  string // PostgreSQL SSL mode

	// Security configuration
	// TransportEncryption enables browser-side encryption for API keys
	// Requires HTTPS or localhost. Set to false for HTTP access via IP.
	TransportEncryption bool

	// Experience improvement (anonymous usage statistics)
	// Helps us understand product usage and improve the experience
	// Set EXPERIENCE_IMPROVEMENT=false to disable
	ExperienceImprovement bool

	// Market data provider API keys
	AlpacaAPIKey    string // Alpaca API key for US stocks
	AlpacaSecretKey string // Alpaca secret key
	TwelveDataKey   string // TwelveData API key for forex & metals

}

// Init initializes global configuration (from .env)
func Init() {
	cfg := &Config{
		APIServerPort:           8080,
		ExperienceImprovement:   false, // Default: disabled for privacy
		TransportEncryption:     true,  // Default: enabled for safer credential submission
		AllowPublicRegistration: false, // Default: disabled unless explicitly enabled
		AdminMode:               false,
		NofxOSEnabled:           true,
		CORSAllowedOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
		},
		// Database defaults
		DBType:    "sqlite",
		DBPath:    "data/data.db",
		DBHost:    "localhost",
		DBPort:    5432,
		DBUser:    "postgres",
		DBName:    "nofx",
		DBSSLMode: "disable",
	}

	// Load from environment variables
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = strings.TrimSpace(v)
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = loadOrCreateJWTSecret()
	}

	if v := os.Getenv("API_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.APIServerPort = port
		}
	}

	// Transport encryption defaults to enabled for safer deployment.
	// Set TRANSPORT_ENCRYPTION=false only for trusted local debugging over HTTP.
	if v := os.Getenv("TRANSPORT_ENCRYPTION"); v != "" {
		cfg.TransportEncryption = strings.ToLower(v) == "true"
	}

	if v := os.Getenv("ALLOW_PUBLIC_REGISTRATION"); v != "" {
		cfg.AllowPublicRegistration = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("ADMIN_MODE"); v != "" {
		cfg.AdminMode = strings.ToLower(strings.TrimSpace(v)) == "true"
	}
	if v := os.Getenv("NOFX_ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("NOFXOS_ENABLED"); v != "" {
		cfg.NofxOSEnabled = strings.ToLower(strings.TrimSpace(v)) == "true"
	}

	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.CORSAllowedOrigins = parseCSV(v)
	}

	// Experience improvement: anonymous usage statistics
	// Default disabled, set EXPERIENCE_IMPROVEMENT=true to opt in
	if v := os.Getenv("EXPERIENCE_IMPROVEMENT"); v != "" {
		cfg.ExperienceImprovement = strings.ToLower(v) != "false"
	}

	// Market data provider API keys
	cfg.AlpacaAPIKey = os.Getenv("ALPACA_API_KEY")
	cfg.AlpacaSecretKey = os.Getenv("ALPACA_SECRET_KEY")
	cfg.TwelveDataKey = os.Getenv("TWELVEDATA_API_KEY")

	// Database configuration
	if v := os.Getenv("DB_TYPE"); v != "" {
		cfg.DBType = strings.ToLower(v)
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.DBPort = port
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("DB_SSLMODE"); v != "" {
		cfg.DBSSLMode = v
	}

	global = cfg

	// Initialize experience improvement (installation ID will be set after database init)
	telemetry.Init(cfg.ExperienceImprovement, "")

	// Set up AI token usage tracking callback
	mcp.TokenUsageCallback = func(usage mcp.TokenUsage) {
		telemetry.TrackAIUsage(telemetry.AIUsageEvent{
			ModelProvider: usage.Provider,
			ModelName:     usage.Model,
			Channel:       usage.Channel(),
			InputTokens:   usage.PromptTokens,
			OutputTokens:  usage.CompletionTokens,
		})
	}
}

// Get returns the global configuration
func Get() *Config {
	if global == nil {
		Init()
	}
	return global
}

func loadOrCreateJWTSecret() string {
	const secretPath = "data/jwt_secret"

	if existing, err := os.ReadFile(secretPath); err == nil {
		if secret := strings.TrimSpace(string(existing)); secret != "" {
			return secret
		}
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic("failed to generate JWT secret: " + err.Error())
	}

	secret := base64.StdEncoding.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(secretPath), 0o700); err != nil {
		panic("failed to create JWT secret directory: " + err.Error())
	}
	if err := os.WriteFile(secretPath, []byte(secret+"\n"), 0o600); err != nil {
		panic("failed to persist JWT secret: " + err.Error())
	}

	return secret
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
