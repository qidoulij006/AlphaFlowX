package binance

import (
	"context"
	"fmt"
	"nofx/logger"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	binancefutures "github.com/adshao/go-binance/v2/futures"
)

const (
	userStreamRetryDelay     = 5 * time.Second
	userStreamKeepaliveEvery = 25 * time.Minute
)

type userStreamBalanceState struct {
	Asset              string
	WalletBalance      float64
	CrossWalletBalance float64
}

type userStreamPositionState struct {
	Symbol           string
	Side             string
	PositionAmt      float64
	EntryPrice       float64
	MarkPrice        float64
	UnrealizedProfit float64
	Leverage         float64
	LiquidationPrice float64
}

func (t *FuturesTrader) StartUserDataStream() {
	t.userStreamMutex.Lock()
	if t.userStreamRunning {
		t.userStreamMutex.Unlock()
		return
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	t.userStreamRunning = true
	t.userStreamStopCh = stopCh
	t.userStreamDoneCh = doneCh
	t.userStreamMutex.Unlock()

	go func() {
		defer close(doneCh)
		defer func() {
			t.userStreamMutex.Lock()
			t.userStreamRunning = false
			t.userStreamStopCh = nil
			t.userStreamDoneCh = nil
			t.userStreamMutex.Unlock()
		}()
		t.runUserDataStreamLoop(stopCh)
	}()
}

func (t *FuturesTrader) StopUserDataStream() {
	t.userStreamMutex.Lock()
	stopCh := t.userStreamStopCh
	doneCh := t.userStreamDoneCh
	if !t.userStreamRunning || stopCh == nil || doneCh == nil {
		t.userStreamMutex.Unlock()
		return
	}
	t.userStreamMutex.Unlock()

	select {
	case <-stopCh:
	default:
		close(stopCh)
	}

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		logger.Infof("⚠️ Timed out waiting for Binance user data stream to stop")
	}
}

func (t *FuturesTrader) runUserDataStreamLoop(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if err := t.checkCooldown(); err != nil {
			logger.Infof("⏸ Binance user data stream delayed by REST cooldown: %v", err)
			if !waitForStop(stopCh, userStreamRetryDelay) {
				return
			}
			continue
		}

		if err := t.runSingleUserDataSession(stopCh); err != nil {
			logger.Infof("⚠️ Binance user data stream ended: %v", err)
		}

		if !waitForStop(stopCh, userStreamRetryDelay) {
			return
		}
	}
}

func (t *FuturesTrader) runSingleUserDataSession(stopCh <-chan struct{}) error {
	listenKey, err := t.client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		err = t.recordRESTError(err)
		return fmt.Errorf("failed to start user stream: %w", err)
	}

	logger.Infof("🔌 Binance user data stream started for %s", t.exchangeID)

	errCh := make(chan error, 1)
	expiredCh := make(chan struct{}, 1)

	doneC, wsStopC, err := binancefutures.WsUserDataServe(listenKey, func(event *binancefutures.WsUserDataEvent) {
		t.handleUserDataEvent(event, expiredCh)
	}, func(wsErr error) {
		select {
		case errCh <- wsErr:
		default:
		}
	})
	if err != nil {
		_ = t.closeUserStreamListenKey(listenKey)
		return fmt.Errorf("failed to connect user stream websocket: %w", err)
	}

	keepaliveStopCh := make(chan struct{})
	var stopOnce sync.Once
	stopSession := func() {
		stopOnce.Do(func() {
			close(keepaliveStopCh)
			close(wsStopC)
		})
	}

	go t.keepaliveUserDataStream(listenKey, keepaliveStopCh, errCh)

	var sessionErr error
	select {
	case <-stopCh:
	case <-expiredCh:
		sessionErr = fmt.Errorf("listen key expired")
	case wsErr := <-errCh:
		sessionErr = wsErr
	case <-doneC:
		sessionErr = fmt.Errorf("user stream websocket closed")
	}

	stopSession()
	select {
	case <-doneC:
	case <-time.After(2 * time.Second):
	}

	if err := t.closeUserStreamListenKey(listenKey); err != nil {
		logger.Infof("⚠️ Failed to close Binance user stream listen key: %v", err)
	}

	return sessionErr
}

func (t *FuturesTrader) keepaliveUserDataStream(listenKey string, stopCh <-chan struct{}, errCh chan<- error) {
	ticker := time.NewTicker(userStreamKeepaliveEvery)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			err := t.client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background())
			if err != nil {
				err = t.recordRESTError(err)
				select {
				case errCh <- fmt.Errorf("failed to keepalive user stream: %w", err):
				default:
				}
				return
			}
		}
	}
}

func (t *FuturesTrader) closeUserStreamListenKey(listenKey string) error {
	if strings.TrimSpace(listenKey) == "" {
		return nil
	}

	if err := t.checkCooldown(); err != nil {
		return err
	}

	err := t.client.NewCloseUserStreamService().ListenKey(listenKey).Do(context.Background())
	return t.recordRESTError(err)
}

func (t *FuturesTrader) handleUserDataEvent(event *binancefutures.WsUserDataEvent, expiredCh chan<- struct{}) {
	switch event.Event {
	case binancefutures.UserDataEventTypeAccountUpdate:
		t.applyAccountUpdate(event.AccountUpdate)
	case binancefutures.UserDataEventTypeListenKeyExpired:
		select {
		case expiredCh <- struct{}{}:
		default:
		}
	default:
		t.userStreamStateMutex.Lock()
		t.userStreamLastEventTime = time.Now().UTC()
		t.userStreamStateMutex.Unlock()
	}
}

func (t *FuturesTrader) applyAccountUpdate(update binancefutures.WsAccountUpdate) {
	now := time.Now().UTC()

	t.userStreamStateMutex.Lock()
	for _, balance := range update.Balances {
		t.userStreamBalances[balance.Asset] = userStreamBalanceState{
			Asset:              balance.Asset,
			WalletBalance:      parseBinanceFloat(balance.Balance),
			CrossWalletBalance: parseBinanceFloat(balance.CrossWalletBalance),
		}
	}

	for _, position := range update.Positions {
		amt := parseBinanceFloat(position.Amount)
		side := normalizeUserStreamPositionSide(position.Side, amt)
		key := userStreamPositionKey(position.Symbol, side)
		if amt == 0 {
			delete(t.userStreamPositions, key)
			continue
		}

		existing := t.userStreamPositions[key]
		if existing.Symbol == "" {
			existing.Symbol = position.Symbol
			existing.Side = side
			existing.Leverage, existing.LiquidationPrice = t.getCachedPositionMetadata(position.Symbol, side)
			if existing.Leverage == 0 {
				existing.Leverage = 10
			}
		}
		existing.PositionAmt = amt
		existing.EntryPrice = parseBinanceFloat(position.EntryPrice)
		existing.MarkPrice = parseBinanceFloat(position.MarkPrice)
		existing.UnrealizedProfit = parseBinanceFloat(position.UnrealizedPnL)
		t.userStreamPositions[key] = existing
	}

	t.userStreamSnapshotReady = len(t.userStreamBalances) > 0
	t.userStreamLastEventTime = now

	balanceSnapshot := t.buildBalanceSnapshotLocked()
	positionSnapshot := t.buildPositionSnapshotLocked()
	t.userStreamStateMutex.Unlock()

	if balanceSnapshot != nil {
		t.balanceCacheMutex.Lock()
		t.cachedBalance = balanceSnapshot
		t.balanceCacheTime = now
		t.balanceCacheMutex.Unlock()
	}

	if len(positionSnapshot) > 0 || len(update.Positions) > 0 {
		t.positionsCacheMutex.Lock()
		t.cachedPositions = positionSnapshot
		t.positionsCacheTime = now
		t.positionsCacheMutex.Unlock()
	}
}

func (t *FuturesTrader) getUserStreamBalanceSnapshot() (map[string]interface{}, bool) {
	t.userStreamStateMutex.RLock()
	defer t.userStreamStateMutex.RUnlock()
	if !t.userStreamSnapshotReady {
		return nil, false
	}
	if !t.isUserStreamSnapshotFreshLocked() {
		return nil, false
	}
	if len(t.userStreamPositions) == 0 && t.positionsCacheTime.IsZero() {
		return nil, false
	}
	snapshot := t.buildBalanceSnapshotLocked()
	return snapshot, snapshot != nil
}

func (t *FuturesTrader) getUserStreamPositionSnapshot() ([]map[string]interface{}, bool) {
	t.userStreamStateMutex.RLock()
	defer t.userStreamStateMutex.RUnlock()
	if !t.userStreamSnapshotReady {
		return nil, false
	}
	if !t.isUserStreamSnapshotFreshLocked() {
		return nil, false
	}
	if len(t.userStreamPositions) == 0 && t.positionsCacheTime.IsZero() {
		return nil, false
	}
	return t.buildPositionSnapshotLocked(), true
}

func (t *FuturesTrader) isUserStreamSnapshotFreshLocked() bool {
	if t.userStreamLastEventTime.IsZero() {
		return false
	}
	// If user stream has been silent for too long, prefer REST refresh over stale websocket snapshots.
	return time.Since(t.userStreamLastEventTime) <= 15*time.Second
}

func (t *FuturesTrader) buildBalanceSnapshotLocked() map[string]interface{} {
	if len(t.userStreamBalances) == 0 {
		return nil
	}

	totalWalletBalance := 0.0
	availableBalance := 0.0
	totalUnrealizedProfit := 0.0

	for _, balance := range t.userStreamBalances {
		totalWalletBalance += balance.WalletBalance
		availableBalance += balance.CrossWalletBalance
	}

	for _, position := range t.userStreamPositions {
		totalUnrealizedProfit += position.UnrealizedProfit
	}

	return map[string]interface{}{
		"totalWalletBalance":    totalWalletBalance,
		"availableBalance":      availableBalance,
		"totalUnrealizedProfit": totalUnrealizedProfit,
	}
}

func (t *FuturesTrader) buildPositionSnapshotLocked() []map[string]interface{} {
	if len(t.userStreamPositions) == 0 {
		return []map[string]interface{}{}
	}

	keys := make([]string, 0, len(t.userStreamPositions))
	for key := range t.userStreamPositions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	positions := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		position := t.userStreamPositions[key]
		positions = append(positions, map[string]interface{}{
			"symbol":           position.Symbol,
			"side":             position.Side,
			"positionAmt":      position.PositionAmt,
			"entryPrice":       position.EntryPrice,
			"markPrice":        position.MarkPrice,
			"unRealizedProfit": position.UnrealizedProfit,
			"leverage":         position.Leverage,
			"liquidationPrice": position.LiquidationPrice,
		})
	}
	return positions
}

func (t *FuturesTrader) syncUserStreamPositionMetadata(positions []map[string]interface{}) {
	if len(positions) == 0 {
		return
	}

	t.userStreamStateMutex.Lock()
	defer t.userStreamStateMutex.Unlock()

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		if symbol == "" || side == "" {
			continue
		}

		key := userStreamPositionKey(symbol, side)
		state, ok := t.userStreamPositions[key]
		if !ok {
			continue
		}

		if leverage, ok := pos["leverage"].(float64); ok && leverage > 0 {
			state.Leverage = leverage
		}
		if liquidationPrice, ok := pos["liquidationPrice"].(float64); ok {
			state.LiquidationPrice = liquidationPrice
		}
		t.userStreamPositions[key] = state
	}
}

func (t *FuturesTrader) getCachedPositionMetadata(symbol, side string) (float64, float64) {
	t.positionsCacheMutex.RLock()
	defer t.positionsCacheMutex.RUnlock()

	for _, pos := range t.cachedPositions {
		posSymbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		if posSymbol != symbol || posSide != side {
			continue
		}

		leverage, _ := pos["leverage"].(float64)
		liquidationPrice, _ := pos["liquidationPrice"].(float64)
		return leverage, liquidationPrice
	}

	return 0, 0
}

func userStreamPositionKey(symbol, side string) string {
	return symbol + "_" + side
}

func normalizeUserStreamPositionSide(side binancefutures.PositionSideType, amount float64) string {
	switch strings.ToUpper(string(side)) {
	case "LONG":
		return "long"
	case "SHORT":
		return "short"
	default:
		if amount < 0 {
			return "short"
		}
		return "long"
	}
}

func parseBinanceFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func waitForStop(stopCh <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-stopCh:
		return false
	case <-timer.C:
		return true
	}
}
