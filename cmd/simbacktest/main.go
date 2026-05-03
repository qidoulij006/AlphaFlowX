package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nofx/sim"

	_ "modernc.org/sqlite"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "simbacktest: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "replay", "mode: download, export-orders, or replay")
	symbol := flag.String("symbol", "DOGEUSDT", "futures symbol")
	start := flag.String("start", "", "start time, RFC3339, e.g. 2026-04-23T14:52:32+08:00")
	end := flag.String("end", "", "end time, RFC3339")
	out := flag.String("out", "", "download output path")
	data := flag.String("data", "", "market data NDJSON path for replay")
	orders := flag.String("orders", "", "order intents NDJSON path for replay")
	dbPath := flag.String("db", "./data/data.db", "sqlite database path for export-orders")
	traderID := flag.String("trader-id", "", "trader id for export-orders")
	reportPath := flag.String("report", "", "optional replay report JSON output path")
	initialBalance := flag.Float64("initial-balance", 10000, "initial simulated account balance")
	makerFee := flag.Float64("maker-fee", 0.0002, "maker fee rate")
	takerFee := flag.Float64("taker-fee", 0.0004, "taker fee rate")
	queueModel := flag.String("queue-model", string(sim.QueueModelTradeThrough), "queue model: trade_through or touch")
	recordFills := flag.Bool("record-fills", false, "include fills in replay report")
	requestDelay := flag.Duration("request-delay", 150*time.Millisecond, "delay between download requests")
	flag.Parse()

	switch strings.ToLower(*mode) {
	case "download":
		return runDownload(*symbol, *start, *end, *out, *requestDelay)
	case "replay":
		return runReplay(replayConfig{
			symbol:         *symbol,
			dataPath:       *data,
			ordersPath:     *orders,
			reportPath:     *reportPath,
			initialBalance: *initialBalance,
			makerFee:       *makerFee,
			takerFee:       *takerFee,
			queueModel:     sim.QueueModel(*queueModel),
			recordFills:    *recordFills,
		})
	case "export-orders":
		return runExportOrders(*dbPath, *traderID, *symbol, *start, *end, *out)
	default:
		return fmt.Errorf("unsupported mode %q", *mode)
	}
}

func runExportOrders(dbPath, traderID, symbol, startRaw, endRaw, out string) error {
	if traderID == "" {
		return fmt.Errorf("trader-id is required")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	start, err := parseRequiredTime("start", startRaw)
	if err != nil {
		return err
	}
	end, err := parseRequiredTime("end", endRaw)
	if err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join("data", "sim", fmt.Sprintf("%s-orders-%d-%d.ndjson", strings.ToUpper(symbol), start.UnixMilli(), end.UnixMilli()))
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	seedIntents, err := buildSeedPositionIntents(db, traderID, symbol, start.UnixMilli())
	if err != nil {
		return err
	}

	rows, err := db.Query(`
SELECT created_at, updated_at, exchange_order_id, client_order_id, symbol, side, position_side, type, quantity, price, reduce_only, status
FROM trader_orders
WHERE trader_id = ?
  AND symbol = ?
  AND created_at >= ?
  AND created_at <= ?
ORDER BY created_at, id`, traderID, strings.ToUpper(symbol), start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return err
	}
	defer rows.Close()

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	file, err := os.Create(out)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)

	count := 0
	for _, intent := range seedIntents {
		if err := encoder.Encode(intent); err != nil {
			return err
		}
		count++
	}
	for rows.Next() {
		var createdAt, updatedAt int64
		var exchangeOrderID, clientOrderID, rowSymbol, side, positionSide, orderType, status string
		var quantity, price float64
		var reduceOnlyRaw int64
		if err := rows.Scan(&createdAt, &updatedAt, &exchangeOrderID, &clientOrderID, &rowSymbol, &side, &positionSide, &orderType, &quantity, &price, &reduceOnlyRaw, &status); err != nil {
			return err
		}
		clientID := clientOrderID
		if clientID == "" {
			clientID = exchangeOrderID
		}
		intent := sim.OrderIntent{
			Time:         createdAt,
			Action:       "submit",
			OrderID:      exchangeOrderID,
			ClientID:     clientID,
			Symbol:       strings.ToUpper(rowSymbol),
			Type:         sim.OrderType(strings.ToUpper(orderType)),
			Side:         sim.Side(strings.ToUpper(side)),
			PositionSide: sim.PositionSide(strings.ToUpper(positionSide)),
			Price:        price,
			Quantity:     quantity,
			ReduceOnly:   reduceOnlyRaw != 0,
		}
		if err := encoder.Encode(intent); err != nil {
			return err
		}
		count++
		if isCanceledStatus(status) && updatedAt > createdAt {
			cancelIntent := sim.OrderIntent{
				Time:     updatedAt,
				Action:   "cancel",
				OrderID:  exchangeOrderID,
				ClientID: clientID,
				Symbol:   strings.ToUpper(rowSymbol),
			}
			if err := encoder.Encode(cancelIntent); err != nil {
				return err
			}
			count++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Printf("exported_order_intents: %d\n", count)
	fmt.Printf("orders: %s\n", out)
	return nil
}

type positionAccumulator struct {
	quantity float64
	avgPrice float64
}

func buildSeedPositionIntents(db *sql.DB, traderID, symbol string, startMs int64) ([]sim.OrderIntent, error) {
	rows, err := db.Query(`
SELECT COALESCE(o.side, f.side), COALESCE(o.position_side, ''), COALESCE(o.order_action, ''), f.price, f.quantity
FROM trader_fills f
JOIN trader_orders o ON o.id = f.order_id
WHERE f.trader_id = ?
  AND f.symbol = ?
  AND f.created_at < ?
ORDER BY f.created_at, f.id`, traderID, symbol, startMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := map[sim.PositionSide]*positionAccumulator{
		sim.PositionLong:  {},
		sim.PositionShort: {},
	}
	for rows.Next() {
		var sideRaw, positionSideRaw, actionRaw string
		var price, quantity float64
		if err := rows.Scan(&sideRaw, &positionSideRaw, &actionRaw, &price, &quantity); err != nil {
			return nil, err
		}
		side, positionSide, err := normalizeFillDirection(sideRaw, positionSideRaw, actionRaw)
		if err != nil {
			return nil, err
		}
		position := positions[positionSide]
		if fillOpensPosition(side, positionSide) {
			addSeedPosition(position, price, quantity)
		} else {
			reduceSeedPosition(position, quantity)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	intents := make([]sim.OrderIntent, 0, 2)
	for _, positionSide := range []sim.PositionSide{sim.PositionLong, sim.PositionShort} {
		position := positions[positionSide]
		if position.quantity <= 1e-12 {
			continue
		}
		intents = append(intents, sim.OrderIntent{
			Time:         startMs,
			Action:       "seed_position",
			Symbol:       symbol,
			PositionSide: positionSide,
			Price:        position.avgPrice,
			Quantity:     position.quantity,
		})
	}
	return intents, nil
}

func normalizeFillDirection(sideRaw, positionSideRaw, actionRaw string) (sim.Side, sim.PositionSide, error) {
	side := sim.Side(strings.ToUpper(strings.TrimSpace(sideRaw)))
	positionSide := sim.PositionSide(strings.ToUpper(strings.TrimSpace(positionSideRaw)))
	action := strings.ToLower(strings.TrimSpace(actionRaw))

	if positionSide == "" {
		switch {
		case strings.Contains(action, "long"):
			positionSide = sim.PositionLong
		case strings.Contains(action, "short"):
			positionSide = sim.PositionShort
		}
	}
	if side == "" {
		switch action {
		case "open_long":
			side = sim.SideBuy
		case "close_long":
			side = sim.SideSell
		case "open_short":
			side = sim.SideSell
		case "close_short":
			side = sim.SideBuy
		}
	}
	if side != sim.SideBuy && side != sim.SideSell {
		return "", "", fmt.Errorf("unsupported fill side %q", sideRaw)
	}
	if positionSide != sim.PositionLong && positionSide != sim.PositionShort {
		return "", "", fmt.Errorf("unsupported fill position side %q", positionSideRaw)
	}
	return side, positionSide, nil
}

func fillOpensPosition(side sim.Side, positionSide sim.PositionSide) bool {
	return (positionSide == sim.PositionLong && side == sim.SideBuy) ||
		(positionSide == sim.PositionShort && side == sim.SideSell)
}

func addSeedPosition(position *positionAccumulator, price, quantity float64) {
	if quantity <= 0 {
		return
	}
	if position.quantity <= 0 {
		position.quantity = quantity
		position.avgPrice = price
		return
	}
	position.avgPrice = (position.avgPrice*position.quantity + price*quantity) / (position.quantity + quantity)
	position.quantity += quantity
}

func reduceSeedPosition(position *positionAccumulator, quantity float64) {
	if quantity <= 0 || position.quantity <= 0 {
		return
	}
	position.quantity -= quantity
	if position.quantity <= 1e-12 {
		position.quantity = 0
		position.avgPrice = 0
	}
}

func isCanceledStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CANCELED", "EXPIRED", "REJECTED":
		return true
	default:
		return false
	}
}

func runDownload(symbol, startRaw, endRaw, out string, delay time.Duration) error {
	start, err := parseRequiredTime("start", startRaw)
	if err != nil {
		return err
	}
	end, err := parseRequiredTime("end", endRaw)
	if err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join("data", "sim", fmt.Sprintf("%s-aggtrades-%d-%d.ndjson", strings.ToUpper(symbol), start.UnixMilli(), end.UnixMilli()))
	}
	result, err := sim.DownloadBinanceAggTrades(context.Background(), sim.BinanceDownloadConfig{
		Symbol:       symbol,
		Start:        start,
		End:          end,
		OutputPath:   out,
		RequestDelay: delay,
	})
	if err != nil {
		return err
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
	return nil
}

type replayConfig struct {
	symbol         string
	dataPath       string
	ordersPath     string
	reportPath     string
	initialBalance float64
	makerFee       float64
	takerFee       float64
	queueModel     sim.QueueModel
	recordFills    bool
}

func runReplay(cfg replayConfig) error {
	if cfg.dataPath == "" {
		return fmt.Errorf("data path is required")
	}
	if cfg.ordersPath == "" {
		return fmt.Errorf("orders path is required")
	}
	trades, err := sim.ReadAggTradeEvents(cfg.dataPath)
	if err != nil {
		return err
	}
	intents, err := sim.ReadOrderIntents(cfg.ordersPath)
	if err != nil {
		return err
	}
	sort.Slice(intents, func(i, j int) bool {
		return intents[i].Time < intents[j].Time
	})

	engine := sim.NewEngine(sim.EngineConfig{
		Symbol:         strings.ToUpper(cfg.symbol),
		InitialBalance: cfg.initialBalance,
		MakerFeeRate:   cfg.makerFee,
		TakerFeeRate:   cfg.takerFee,
		QueueModel:     cfg.queueModel,
		RecordFills:    cfg.recordFills,
	})

	intentIdx := 0
	for _, trade := range trades {
		for intentIdx < len(intents) && intents[intentIdx].Time <= trade.Time {
			if err := applyIntent(engine, intents[intentIdx]); err != nil {
				return err
			}
			intentIdx++
		}
		engine.OnTrade(trade)
	}
	for intentIdx < len(intents) {
		if err := applyIntent(engine, intents[intentIdx]); err != nil {
			return err
		}
		intentIdx++
	}

	report := engine.Report()
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if cfg.reportPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.reportPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(cfg.reportPath, append(encoded, '\n'), 0o644); err != nil {
			return err
		}
	}
	printReplaySummary(report)
	if cfg.reportPath != "" {
		fmt.Printf("report: %s\n", cfg.reportPath)
	}
	return nil
}

func applyIntent(engine *sim.Engine, intent sim.OrderIntent) error {
	switch strings.ToLower(intent.Action) {
	case "seed_position":
		return engine.SeedPosition(intent.Symbol, intent.PositionSide, intent.Quantity, intent.Price, intent.Time)
	case "cancel":
		orderID := intent.OrderID
		if orderID == "" {
			orderID = intent.ClientID
		}
		return engine.CancelOrder(orderID, intent.Time)
	case "submit", "place", "":
		if intent.Type == sim.OrderTypeMarket {
			_, err := engine.SubmitMarket(intent)
			return err
		}
		_, err := engine.SubmitLimit(intent)
		return err
	default:
		return fmt.Errorf("unsupported intent action %q", intent.Action)
	}
}

func parseRequiredTime(name, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is required", name)
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func printReplaySummary(report sim.Report) {
	fmt.Printf("symbol: %s\n", report.Symbol)
	fmt.Printf("window: %s -> %s\n", formatMs(report.StartedAt), formatMs(report.EndedAt))
	fmt.Printf("balance: %.8f\n", report.Balance)
	fmt.Printf("equity: %.8f\n", report.Equity)
	fmt.Printf("realized_pnl: %.8f\n", report.RealizedPnL)
	fmt.Printf("fees: %.8f\n", report.Fees)
	fmt.Printf("unrealized_pnl: %.8f\n", report.UnrealizedPnL)
	fmt.Printf("fills: %d\n", report.FillCount)
	fmt.Printf("open_orders: %d\n", len(report.OpenOrders))
	for _, position := range report.Positions {
		if position.Quantity == 0 {
			continue
		}
		fmt.Printf("position %s qty=%.8f avg=%.8f realized=%.8f fees=%.8f\n",
			position.Side, position.Quantity, position.AvgPrice, position.RealizedPnL, position.Fees)
	}
}

func formatMs(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
