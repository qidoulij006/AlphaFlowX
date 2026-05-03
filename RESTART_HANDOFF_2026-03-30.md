# NOFX Restart Handoff

Date: 2026-03-30
Trader: `qidou02-智能网格`
Issue: Grid AI keeps pausing with messages like:
- all grid levels show `$0.09`
- grid spacing shows `$0.00`
- `EMA50` shows `$0.00`

## What Was Confirmed

The online service was repeatedly writing old-style grid prompts into `decision_records`.

Latest observed records before handoff:
- `decision_records.id = 3871`
- `decision_records.id = 3870`

Those prompts still showed:
- `当前价格: $0.09`
- `布林带: 上轨 $0.09, 中轨 $0.09, 下轨 $0.09`
- `EMA50: $0.00`
- `总权益: $0.00`

So the root problem is not just AI judgment. The running backend is still using the old code path.

## Code Changes Already Made Locally

These source edits are already present in the workspace:

1. `market/data.go`
- `GetWithTimeframes()` now keeps the longest non-primary timeframe as `LongerTermContext`.
- This is meant to restore valid `EMA50` / longer-term ATR data for grid prompts.

2. `kernel/grid_engine.go`
- Added dynamic price formatting for low-priced assets like `DOGEUSDT`.
- Goal: stop collapsing different prices into the same displayed `$0.09`.
- Grid range, spacing, Bollinger bands, EMA values, and per-level prices now format with more precision for small prices.

3. `trader/auto_trader_grid.go`
- Added fallback equity parsing from Binance-style fields:
  - `totalWalletBalance`
  - `totalUnrealizedProfit`
- Goal: stop showing `总权益: $0.00` when account balance is actually nonzero.

4. `docker/Dockerfile.backend`
- Build step was changed to lower concurrency:
  - `GOFLAGS="-p=1"`
  - `GOMAXPROCS=1`
- Goal: reduce memory pressure during Docker image build.

## What Was Verified

Local package tests passed against source code:

```bash
go test ./kernel ./market ./trader
```

That verifies the source changes compile in the local environment.

## What Failed During Deployment

### 1. Docker local rebuild still failed

Command attempted:

```bash
docker compose -f docker-compose.yml -f docker-compose.localbuild.yml up -d --build nofx
```

Failure:

```text
github.com/ugorji/go/codec: /usr/local/go/pkg/tool/linux_amd64/compile: signal: killed
```

Interpretation:
- This is an OOM / memory-pressure kill during Docker build.
- It is not a source-code compile error.

### 2. Host-built binary hot-swap was not viable

Host build succeeded with:

```bash
env GOCACHE=/tmp/go-build-cache go build -trimpath -ldflags='-s -w' -o /tmp/nofx-hotfix .
```

But hot-swapping `/tmp/nofx-hotfix` into the Alpine container failed:
- first: `exec ./nofx: no such file or directory`
- then after compatibility packages: `Error relocating /app/nofx: fcntl64: symbol not found`

Interpretation:
- Host binary is glibc-linked
- Runtime container is Alpine/musl-based
- Compatibility layer was insufficient

## Current Safe State Before Reboot

Backend service was restored to the existing image and is healthy again.

Confirmed:

```bash
curl -I http://127.0.0.1:8080/api/health
```

returned `HTTP/1.1 200 OK`.

Important:
- data was preserved
- `./data` mount was not deleted
- trader / exchange / strategy records remain in `data/data.db`

## Current Deployment Status

The service is running again, but still on the old backend image.

That means:
- online behavior is still old
- the grid prompt bug is not deployed yet
- logs still show old formatting like:
  - `Initialized: 10 levels, $0.09 - $0.09, spacing $0.00`
  - `EMA50: $0.00`

## Recommended Next Step After Server Upgrade

After memory is upgraded, continue with this exact step first:

```bash
cd /root/codex-workspace/nofx
docker compose -f docker-compose.yml -f docker-compose.localbuild.yml up -d --build nofx
```

If rebuild succeeds:
- wait for health:

```bash
curl -I http://127.0.0.1:8080/api/health
```

- then verify new decision prompt:

```bash
sqlite3 -header -column data/data.db "select id,timestamp,substr(input_prompt,1,2200) as input_prompt from decision_records where trader_id=(select id from traders where name='qidou02-智能网格') order by timestamp desc limit 3;"
```

## Expected Signs The Fix Is Really Deployed

In new `input_prompt`, `DOGEUSDT` should no longer collapse to two decimals everywhere.

Expected improvements:
- grid prices should show more precision, not all `$0.09`
- spacing should not display as `$0.00` if nonzero
- `EMA50` should no longer be `0.00` if 4h context is available
- `总权益` should no longer be `$0.00` when Binance account balance is nonzero

## Useful Verification Commands

Check backend health:

```bash
curl -I http://127.0.0.1:8080/api/health
```

Check backend logs:

```bash
docker compose logs --tail=80 nofx
```

Check latest grid decisions:

```bash
sqlite3 -header -column data/data.db "select id,timestamp,substr(input_prompt,1,2200) as input_prompt, substr(decisions,1,500) as decisions from decision_records where trader_id=(select id from traders where name='qidou02-智能网格') order by timestamp desc limit 3;"
```

## Notes About Data Safety

Deleting and recreating only the `nofx-trading` container does not remove trader data, because compose mounts:

`./data:/app/data`

So records survive container recreation unless `./data` is deleted or replaced.
