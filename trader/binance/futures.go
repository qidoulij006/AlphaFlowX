package binance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"nofx/hook"
	"nofx/logger"
	"nofx/store"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// getBrOrderID generates unique order ID (for futures contracts)
// Format: x-{BR_ID}{TIMESTAMP}{RANDOM}
// Futures limit is 32 characters, use this limit consistently
// Uses nanosecond timestamp + random number to ensure global uniqueness (collision probability < 10^-20)
func getBrOrderID() string {
	brID := "KzrpZaP9" // Futures br ID

	// Calculate available space: 32 - len("x-KzrpZaP9") = 32 - 11 = 21 characters
	// Allocation: 13-digit timestamp + 8-digit random = 21 characters (perfect utilization)
	timestamp := time.Now().UnixNano() % 10000000000000 // 13-digit nanosecond timestamp

	// Generate 4-byte random number (8 hex digits)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	// Format: x-KzrpZaP9{13-digit timestamp}{8-digit random}
	// Example: x-KzrpZaP91234567890123abcdef12 (exactly 31 characters)
	orderID := fmt.Sprintf("x-%s%d%s", brID, timestamp, randomHex)

	// Ensure not exceeding 32-character limit (theoretically exactly 31 characters)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	return orderID
}

// FuturesTrader Binance futures trader
type FuturesTrader struct {
	client *futures.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Cache validity period (15 seconds)
	cacheDuration time.Duration

	cooldownUntil time.Time
	cooldownMutex sync.RWMutex
	store         *store.Store
	exchangeID    string

	userStreamMutex         sync.Mutex
	userStreamRunning       bool
	userStreamStopCh        chan struct{}
	userStreamDoneCh        chan struct{}
	userStreamSnapshotReady bool
	userStreamLastEventTime time.Time
	userStreamStateMutex    sync.RWMutex
	userStreamBalances      map[string]userStreamBalanceState
	userStreamPositions     map[string]userStreamPositionState
}

// NewFuturesTrader creates futures trader.
// exchangeID is optional and enables per-exchange runtime hooks such as proxy injection.
func NewFuturesTrader(apiKey, secretKey string, userId string, exchangeIDs ...string) *FuturesTrader {
	client := futures.NewClient(apiKey, secretKey)
	exchangeID := ""
	if len(exchangeIDs) > 0 {
		exchangeID = exchangeIDs[0]
	}

	hookRes := hook.HookExec[hook.NewBinanceTraderResult](hook.NEW_BINANCE_TRADER, userId, exchangeID, client)
	if hookRes != nil && hookRes.GetResult() != nil {
		client = hookRes.GetResult()
	}

	trader := &FuturesTrader{
		client:              client,
		cacheDuration:       5 * time.Second, // 5-second cache for fresher dashboard positions/account
		userStreamBalances:  make(map[string]userStreamBalanceState),
		userStreamPositions: make(map[string]userStreamPositionState),
	}

	return trader
}

// InitializeSession performs best-effort Binance REST setup after cooldown state is restored.
func (t *FuturesTrader) InitializeSession() {
	if err := t.checkCooldown(); err != nil {
		logger.Infof("⏸ Skipping Binance REST session initialization: %v", err)
		return
	}

	// Sync time to avoid "Timestamp ahead" errors on signed requests.
	if err := t.syncBinanceServerTime(); err != nil {
		logger.Infof("⚠️ Failed to sync Binance server time: %v", err)
	}

	// This is required because the code uses PositionSide (LONG/SHORT).
	if err := t.setDualSidePosition(); err != nil {
		logger.Infof("⚠️ Failed to set dual-side position mode: %v (ignore this warning if already in dual-side mode)", err)
	}
}

// setDualSidePosition sets dual-side position mode (called during initialization)
func (t *FuturesTrader) setDualSidePosition() error {
	if err := t.checkCooldown(); err != nil {
		return err
	}

	// Try to set dual-side position mode
	err := t.client.NewChangePositionModeService().
		DualSide(true). // true = dual-side position (Hedge Mode)
		Do(context.Background())
	err = t.recordRESTError(err)

	if err != nil {
		// If error message contains "No need to change", it means already in dual-side position mode
		if strings.Contains(err.Error(), "No need to change position side") {
			logger.Infof("  ✓ Account is already in dual-side position mode (Hedge Mode)")
			return nil
		}
		// Other errors are returned (but won't interrupt initialization in the caller)
		return err
	}

	logger.Infof("  ✓ Account has been switched to dual-side position mode (Hedge Mode)")
	logger.Infof("  ℹ️  Dual-side position mode allows holding both long and short positions simultaneously")
	return nil
}

// syncBinanceServerTime syncs Binance server time to ensure request timestamps are valid.
func (t *FuturesTrader) syncBinanceServerTime() error {
	if err := t.checkCooldown(); err != nil {
		return err
	}

	serverTime, err := t.client.NewServerTimeService().Do(context.Background())
	err = t.recordRESTError(err)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	offset := now - serverTime
	t.client.TimeOffset = offset
	logger.Infof("⏱ Binance server time synced, offset %dms", offset)
	return nil
}

var binanceBanUntilPattern = regexp.MustCompile(`banned until (\d+)`)

func parseBinanceBanUntil(err error) time.Time {
	if err == nil {
		return time.Time{}
	}

	matches := binanceBanUntilPattern.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return time.Time{}
	}

	untilMs, parseErr := strconv.ParseInt(matches[1], 10, 64)
	if parseErr != nil || untilMs <= 0 {
		return time.Time{}
	}

	return time.UnixMilli(untilMs).UTC()
}

func (t *FuturesTrader) setCooldownUntil(until time.Time) {
	if until.IsZero() {
		return
	}

	t.cooldownMutex.Lock()
	defer t.cooldownMutex.Unlock()
	if until.After(t.cooldownUntil) {
		t.cooldownUntil = until
	}
}

func (t *FuturesTrader) GetCooldownUntil() time.Time {
	return t.getCooldownUntil()
}

func (t *FuturesTrader) getCooldownUntil() time.Time {
	t.cooldownMutex.RLock()
	defer t.cooldownMutex.RUnlock()
	return t.cooldownUntil
}

func (t *FuturesTrader) cooldownConfigKey() string {
	if t.exchangeID == "" {
		return ""
	}
	return fmt.Sprintf("binance_cooldown_until:%s", t.exchangeID)
}

func (t *FuturesTrader) persistCooldownUntil(until time.Time) {
	if t.store == nil {
		return
	}
	key := t.cooldownConfigKey()
	if key == "" {
		return
	}

	value := ""
	if !until.IsZero() {
		value = strconv.FormatInt(until.UTC().UnixMilli(), 10)
	}
	if err := t.store.SetSystemConfig(key, value); err != nil {
		logger.Infof("⚠️ Failed to persist Binance cooldown for %s: %v", t.exchangeID, err)
	}
}

func (t *FuturesTrader) BindCooldownStore(st *store.Store, exchangeID string) {
	t.store = st
	t.exchangeID = exchangeID
	if st == nil || exchangeID == "" {
		return
	}

	value, err := st.GetSystemConfig(t.cooldownConfigKey())
	if err != nil {
		logger.Infof("⚠️ Failed to load Binance cooldown for %s: %v", exchangeID, err)
		return
	}
	if strings.TrimSpace(value) == "" {
		return
	}

	untilMs, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || untilMs <= 0 {
		return
	}

	until := time.UnixMilli(untilMs).UTC()
	if until.After(time.Now().UTC()) {
		t.setCooldownUntil(until)
		logger.Infof("⏸ Restored Binance REST cooldown for %s until %s", exchangeID, until.Format(time.RFC3339))
		return
	}

	t.persistCooldownUntil(time.Time{})
}

func (t *FuturesTrader) checkCooldown() error {
	until := t.getCooldownUntil()
	if until.IsZero() {
		return nil
	}

	now := time.Now().UTC()
	if now.Before(until) {
		return fmt.Errorf("binance rest cooldown active until %s UTC", until.Format("2006-01-02 15:04:05"))
	}

	t.cooldownMutex.Lock()
	if !t.cooldownUntil.IsZero() && !time.Now().UTC().Before(t.cooldownUntil) {
		t.cooldownUntil = time.Time{}
		t.persistCooldownUntil(time.Time{})
	}
	t.cooldownMutex.Unlock()
	return nil
}

func (t *FuturesTrader) recordRESTError(err error) error {
	if err == nil {
		return nil
	}

	if banUntil := parseBinanceBanUntil(err); !banUntil.IsZero() {
		t.setCooldownUntil(banUntil)
		t.persistCooldownUntil(banUntil)
	}
	return err
}

func isBinanceRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "too many requests") ||
		strings.Contains(errMsg, "way too many requests") ||
		strings.Contains(errMsg, "banned until") ||
		strings.Contains(errMsg, "ip banned") ||
		strings.Contains(errMsg, "code=-1003")
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// calculatePrecision calculates precision from stepSize
func calculatePrecision(stepSize string) int {
	// Remove trailing zeros
	stepSize = trimTrailingZeros(stepSize)

	// Find decimal point
	dotIndex := -1
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] == '.' {
			dotIndex = i
			break
		}
	}

	// If no decimal point or decimal point is at the end, precision is 0
	if dotIndex == -1 || dotIndex == len(stepSize)-1 {
		return 0
	}

	// Return number of digits after decimal point
	return len(stepSize) - dotIndex - 1
}

// trimTrailingZeros removes trailing zeros
func trimTrailingZeros(s string) string {
	// If no decimal point, return directly
	if !stringContains(s, ".") {
		return s
	}

	// Iterate backwards to remove trailing zeros
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}

	// If last character is decimal point, remove it too
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	return s
}
