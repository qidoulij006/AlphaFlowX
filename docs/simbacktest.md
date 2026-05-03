# Independent Matching Simulation

`cmd/simbacktest` is an offline test system for exchange-data replay. It does not connect to the live `AutoTrader` execution loop and does not place real orders.

## Download Binance Futures aggregate trades

```bash
go run ./cmd/simbacktest \
  -mode download \
  -symbol DOGEUSDT \
  -start "2026-04-23T14:52:32+08:00" \
  -end "2026-04-30T14:52:32+08:00" \
  -out data/sim/DOGEUSDT-7d-aggtrades.ndjson \
  -request-delay 1s
```

The output is newline-delimited JSON `trade` events. `taker_side` is inferred from Binance `buyer_maker`: if the buyer was maker, the taker was a seller; otherwise the taker was a buyer.

The downloader writes to `*.tmp` first and renames it only after a complete run. It retries Binance `429` and `5xx` responses with exponential backoff, but long windows can still need a larger `-request-delay` under shared-IP rate limits.

## Replay order intents

Create an order-intent NDJSON file:

```json
{"time":1776927152000,"action":"seed_position","symbol":"DOGEUSDT","position_side":"SHORT","price":0.09576,"quantity":238993}
{"time":1777449094000,"action":"submit","client_id":"short-exit-1","symbol":"DOGEUSDT","type":"LIMIT","side":"BUY","position_side":"SHORT","price":0.1096,"quantity":10000,"reduce_only":true}
{"time":1777449500000,"action":"cancel","client_id":"short-exit-1","symbol":"DOGEUSDT"}
{"time":1777449600000,"action":"submit","client_id":"risk-market-1","symbol":"DOGEUSDT","type":"MARKET","side":"BUY","position_side":"SHORT","quantity":5000,"reduce_only":true}
```

`export-orders` reconstructs the position at `-start` from earlier fills and emits `seed_position` intents before replaying orders inside the window. This prevents opening reduce-only stop orders against an empty simulated book.

You can also export historical order intents from the local SQLite database:

```bash
go run ./cmd/simbacktest \
  -mode export-orders \
  -db data/data.db \
  -trader-id "3885bf4e_32162195-6112-4e49-aba3-cd5a2baf7b4e_deepseek_1775547230" \
  -symbol DOGEUSDT \
  -start "2026-04-23T14:52:32+08:00" \
  -end "2026-04-30T14:52:32+08:00" \
  -out data/sim/DOGEUSDT-7d-orders.ndjson
```

Run the replay:

```bash
go run ./cmd/simbacktest \
  -mode replay \
  -symbol DOGEUSDT \
  -data data/sim/DOGEUSDT-7d-aggtrades.ndjson \
  -orders data/sim/DOGEUSDT-7d-orders.ndjson \
  -initial-balance 10000 \
  -maker-fee 0.0002 \
  -taker-fee 0.0004 \
  -queue-model trade_through \
  -record-fills \
  -report data/sim/report.json
```

## Queue models

`trade_through` is the default conservative model. A resting buy fills only when the market trades below its limit price, and a resting sell fills only when the market trades above its limit price. This avoids overestimating maker fills when historical queue depth is unavailable.

`touch` fills when the market trades at or through the order price. This is optimistic with aggregate trades because it assumes the local order had enough queue priority at that price.

## Current limits

This first version uses Binance Futures aggregate trades from REST. It is useful for deterministic execution tests and conservative maker-fill estimates, but it is not full L2 queue simulation yet.

To make it closer to exchange matching, add historical depth/book-ticker input and extend `sim.Engine` with a queue-position model. Binance REST depth is current-only; historical depth has to come from recorded websocket data or an exchange data archive.
