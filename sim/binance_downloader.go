package sim

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	BinanceFuturesBaseURL        = "https://fapi.binance.com"
	binanceAggTradeLimit         = 1000
	binanceAggTradeWindow        = time.Hour
	defaultBinanceMaxRetries     = 6
	defaultBinanceRetryBaseDelay = 2 * time.Second
)

type BinanceDownloadConfig struct {
	Symbol       string
	Start        time.Time
	End          time.Time
	OutputPath   string
	BaseURL      string
	HTTPClient   *http.Client
	RequestDelay time.Duration
	MaxRetries   int
	RetryDelay   time.Duration
}

type DownloadResult struct {
	Symbol     string `json:"symbol"`
	OutputPath string `json:"output_path"`
	StartedAt  int64  `json:"started_at"`
	EndedAt    int64  `json:"ended_at"`
	Events     int    `json:"events"`
	FirstEvent int64  `json:"first_event"`
	LastEvent  int64  `json:"last_event"`
}

type binanceAggTrade struct {
	AggID      int64  `json:"a"`
	Price      string `json:"p"`
	Quantity   string `json:"q"`
	FirstTrade int64  `json:"f"`
	LastTrade  int64  `json:"l"`
	Time       int64  `json:"T"`
	BuyerMaker bool   `json:"m"`
	BestMatch  bool   `json:"M"`
}

func DownloadBinanceAggTrades(ctx context.Context, cfg BinanceDownloadConfig) (DownloadResult, error) {
	if cfg.Symbol == "" {
		return DownloadResult{}, fmt.Errorf("symbol is required")
	}
	if !cfg.End.After(cfg.Start) {
		return DownloadResult{}, fmt.Errorf("end must be after start")
	}
	if cfg.OutputPath == "" {
		return DownloadResult{}, fmt.Errorf("output path is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = BinanceFuturesBaseURL
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return DownloadResult{}, err
	}
	tmpPath := cfg.OutputPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return DownloadResult{}, err
	}
	downloadOK := false
	defer func() {
		if !downloadOK {
			_ = file.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	writer := bufio.NewWriter(file)

	result := DownloadResult{
		Symbol:     strings.ToUpper(cfg.Symbol),
		OutputPath: cfg.OutputPath,
		StartedAt:  cfg.Start.UnixMilli(),
		EndedAt:    cfg.End.UnixMilli(),
	}
	cursor := cfg.Start.UnixMilli()
	endMs := cfg.End.UnixMilli()

	for cursor <= endMs {
		windowEnd := cursor + binanceAggTradeWindow.Milliseconds() - 1
		if windowEnd > endMs {
			windowEnd = endMs
		}
		batch, err := fetchBinanceAggTrades(ctx, client, baseURL, result.Symbol, cursor, windowEnd, cfg.MaxRetries, cfg.RetryDelay)
		if err != nil {
			return result, err
		}
		if len(batch) == 0 {
			cursor = windowEnd + 1
			if cfg.RequestDelay > 0 {
				if err := sleepOrDone(ctx, cfg.RequestDelay); err != nil {
					return result, err
				}
			}
			continue
		}

		wrote := 0
		for _, raw := range batch {
			if raw.Time < cfg.Start.UnixMilli() {
				continue
			}
			if raw.Time > windowEnd || raw.Time > endMs {
				return result, nil
			}
			price, err := strconv.ParseFloat(raw.Price, 64)
			if err != nil {
				return result, fmt.Errorf("parse agg trade price %q: %w", raw.Price, err)
			}
			qty, err := strconv.ParseFloat(raw.Quantity, 64)
			if err != nil {
				return result, fmt.Errorf("parse agg trade quantity %q: %w", raw.Quantity, err)
			}
			takerSide := SideBuy
			if raw.BuyerMaker {
				takerSide = SideSell
			}
			event := AggTradeEvent{
				Type:       "trade",
				Symbol:     result.Symbol,
				Time:       raw.Time,
				AggID:      raw.AggID,
				Price:      price,
				Quantity:   qty,
				TakerSide:  takerSide,
				BuyerMaker: raw.BuyerMaker,
			}
			encoded, err := json.Marshal(event)
			if err != nil {
				return result, err
			}
			if _, err := writer.Write(append(encoded, '\n')); err != nil {
				return result, err
			}
			if result.Events == 0 {
				result.FirstEvent = event.Time
			}
			result.LastEvent = event.Time
			result.Events++
			wrote++
		}

		last := batch[len(batch)-1]
		if wrote == 0 && last.Time >= windowEnd {
			cursor = windowEnd + 1
		} else if len(batch) >= binanceAggTradeLimit {
			cursor = last.Time + 1
		} else {
			cursor = windowEnd + 1
		}
		if last.Time >= endMs {
			break
		}
		if cfg.RequestDelay > 0 {
			if err := sleepOrDone(ctx, cfg.RequestDelay); err != nil {
				return result, err
			}
		}
	}

	if err := writer.Flush(); err != nil {
		return result, err
	}
	if err := file.Close(); err != nil {
		return result, err
	}
	if err := os.Rename(tmpPath, cfg.OutputPath); err != nil {
		return result, err
	}
	downloadOK = true
	return result, nil
}

func fetchBinanceAggTrades(ctx context.Context, client *http.Client, baseURL, symbol string, startMs, endMs int64, maxRetries int, retryDelay time.Duration) ([]binanceAggTrade, error) {
	if maxRetries <= 0 {
		maxRetries = defaultBinanceMaxRetries
	}
	if retryDelay <= 0 {
		retryDelay = defaultBinanceRetryBaseDelay
	}
	var lastErr error
	delay := retryDelay
	for attempt := 0; attempt <= maxRetries; attempt++ {
		trades, err := fetchBinanceAggTradesOnce(ctx, client, baseURL, symbol, startMs, endMs)
		if err == nil {
			return trades, nil
		}
		lastErr = err
		httpErr, ok := err.(*binanceHTTPError)
		if !ok || !httpErr.retryable() || attempt == maxRetries {
			return nil, err
		}
		wait := delay
		if httpErr.RetryAfter > wait {
			wait = httpErr.RetryAfter
		}
		if err := sleepOrDone(ctx, wait); err != nil {
			return nil, err
		}
		delay *= 2
	}
	return nil, lastErr
}

func fetchBinanceAggTradesOnce(ctx context.Context, client *http.Client, baseURL, symbol string, startMs, endMs int64) ([]binanceAggTrade, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/fapi/v1/aggTrades", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("symbol", strings.ToUpper(symbol))
	q.Set("limit", strconv.Itoa(binanceAggTradeLimit))
	q.Set("startTime", strconv.FormatInt(startMs, 10))
	q.Set("endTime", strconv.FormatInt(endMs, 10))
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &binanceHTTPError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	var trades []binanceAggTrade
	if err := json.Unmarshal(body, &trades); err != nil {
		return nil, err
	}
	return trades, nil
}

type binanceHTTPError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *binanceHTTPError) Error() string {
	return fmt.Sprintf("binance aggTrades status %d: %s", e.StatusCode, e.Body)
}

func (e *binanceHTTPError) retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests ||
		e.StatusCode == http.StatusTeapot ||
		e.StatusCode >= http.StatusInternalServerError
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil && seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	retryAt, err := http.ParseTime(raw)
	if err != nil {
		return 0
	}
	delay := time.Until(retryAt)
	if delay < 0 {
		return 0
	}
	return delay
}

func sleepOrDone(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}
