#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/data/data.db}"
TRADER_ID="${TRADER_ID:-${1:-6f655b09_32162195-6112-4e49-aba3-cd5a2baf7b4e_claw402_1774776897}}"
TARGET_CYCLES="${TARGET_CYCLES:-200}"
REPORT_EVERY="${REPORT_EVERY:-20}"
POLL_SECONDS="${POLL_SECONDS:-15}"
REPORT_DIR="${REPORT_DIR:-$ROOT_DIR/reports/grid-monitor}"
STATE_DIR="${STATE_DIR:-$ROOT_DIR/.monitor/grid-monitor}"
LOG_FILE="$STATE_DIR/monitor.log"
LOCK_FILE="$STATE_DIR/monitor.lock"
COMPOSE_ARGS=(-f "$ROOT_DIR/docker-compose.yml" -f "$ROOT_DIR/docker-compose.localbuild.yml")

mkdir -p "$REPORT_DIR" "$STATE_DIR"

if [[ -f "$LOCK_FILE" ]]; then
  existing_pid="$(cat "$LOCK_FILE" 2>/dev/null || true)"
  if [[ -n "$existing_pid" ]] && [[ -r "/proc/$existing_pid/cmdline" ]]; then
    existing_cmd="$(tr '\0' ' ' </proc/"$existing_pid"/cmdline 2>/dev/null || true)"
    if [[ "$existing_cmd" == *"monitor_grid_cycles.sh"* ]]; then
      echo "monitor already running with pid $existing_pid"
      exit 1
    fi
  fi
fi

echo "${BASHPID:-$$}" >"$LOCK_FILE"
trap 'rm -f "$LOCK_FILE"' EXIT

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$LOG_FILE"
}

sql() {
  sqlite3 -readonly "$DB_PATH" "$1"
}

write_report() {
  local start_cycle="$1"
  local end_cycle="$2"
  local report_seq="$3"
  local trader_name
  local report_file
  local success_count
  local failure_count
  local avg_ai_ms
  local cycle_rows
  local failure_rows
  local action_rows
  local open_order_rows
  local open_position_rows

  trader_name="$(sql "select name from traders where id = '$TRADER_ID' limit 1;")"
  success_count="$(sql "select count(*) from decision_records where trader_id = '$TRADER_ID' and cycle_number between $start_cycle and $end_cycle and success = 1;")"
  failure_count="$(sql "select count(*) from decision_records where trader_id = '$TRADER_ID' and cycle_number between $start_cycle and $end_cycle and success = 0;")"
  avg_ai_ms="$(sql "select printf('%.2f', coalesce(avg(ai_request_duration_ms), 0)) from decision_records where trader_id = '$TRADER_ID' and cycle_number between $start_cycle and $end_cycle;")"
  cycle_rows="$(sql "select cycle_number || '|' || timestamp || '|' || success || '|' || replace(coalesce(error_message,''), char(10), ' ') from decision_records where trader_id = '$TRADER_ID' and cycle_number between $start_cycle and $end_cycle order by cycle_number;")"
  failure_rows="$(sql "select replace(coalesce(error_message,'(empty)'), char(10), ' ') || '|' || count(*) from decision_records where trader_id = '$TRADER_ID' and cycle_number between $start_cycle and $end_cycle and success = 0 group by error_message order by count(*) desc limit 10;")"
  action_rows="$(sql "select coalesce(json_extract(j.value, '$.action'), '(none)') || '|' || count(*) from decision_records dr, json_each(dr.decisions) j where dr.trader_id = '$TRADER_ID' and dr.cycle_number between $start_cycle and $end_cycle group by json_extract(j.value, '$.action') order by count(*) desc;")"
  open_order_rows="$(sql "select coalesce(symbol,'') || '|' || coalesce(side,'') || '|' || coalesce(position_side,'') || '|' || printf('%.8f', coalesce(price,0)) || '|' || printf('%.4f', coalesce(quantity,0)) || '|' || coalesce(status,'') from trader_orders where trader_id = '$TRADER_ID' and status in ('NEW','PARTIALLY_FILLED') order by updated_at desc limit 12;")"
  open_position_rows="$(sql "select coalesce(symbol,'') || '|' || coalesce(side,'') || '|' || printf('%.4f', coalesce(quantity,0)) || '|' || printf('%.8f', coalesce(entry_price,0)) || '|' || coalesce(status,'') from trader_positions where trader_id = '$TRADER_ID' and status = 'OPEN' order by updated_at desc limit 12;")"

  report_file="$REPORT_DIR/report_$(printf '%03d' "$report_seq")_cycles_${start_cycle}_${end_cycle}.md"

  {
    echo "# Grid Monitor Report"
    echo
    echo "- Trader: ${trader_name:-unknown}"
    echo "- Trader ID: $TRADER_ID"
    echo "- Cycle window: $start_cycle -> $end_cycle"
    echo "- Generated at: $(date '+%Y-%m-%d %H:%M:%S %z')"
    echo "- Successful cycles: $success_count"
    echo "- Failed cycles: $failure_count"
    echo "- Average AI request duration: ${avg_ai_ms} ms"
    echo
    echo "## Cycle Details"
    if [[ -n "$cycle_rows" ]]; then
      while IFS='|' read -r cycle ts ok err; do
        status="success"
        if [[ "$ok" != "1" ]]; then
          status="failed"
        fi
        echo "- Cycle $cycle at $ts: $status${err:+ | error: $err}"
      done <<<"$cycle_rows"
    else
      echo "- No cycle records found in this window."
    fi
    echo
    echo "## Failure Breakdown"
    if [[ -n "$failure_rows" ]]; then
      while IFS='|' read -r reason count; do
        echo "- $count x $reason"
      done <<<"$failure_rows"
    else
      echo "- No failures in this window."
    fi
    echo
    echo "## Action Breakdown"
    if [[ -n "$action_rows" ]]; then
      while IFS='|' read -r action count; do
        echo "- $action: $count"
      done <<<"$action_rows"
    else
      echo "- No recorded actions in this window."
    fi
    echo
    echo "## Open Orders Snapshot"
    if [[ -n "$open_order_rows" ]]; then
      while IFS='|' read -r symbol side position_side price qty status; do
        echo "- $symbol $side/$position_side qty=$qty price=$price status=$status"
      done <<<"$open_order_rows"
    else
      echo "- No open orders."
    fi
    echo
    echo "## Open Positions Snapshot"
    if [[ -n "$open_position_rows" ]]; then
      while IFS='|' read -r symbol side qty entry_price status; do
        echo "- $symbol $side qty=$qty entry=$entry_price status=$status"
      done <<<"$open_position_rows"
    else
      echo "- No open positions."
    fi
  } >"$report_file"

  log "wrote report: $report_file"
}

check_live_pause_consistency() {
  local cycle="$1"
  local decision_json
  decision_json="$(sql "select coalesce(decision_json,'') from decision_records where trader_id = '$TRADER_ID' and cycle_number = $cycle limit 1;")"
  if [[ -z "$decision_json" ]]; then
    return 0
  fi

  python3 - <<'PYCHK' "$cycle" "$TRADER_ID" "$decision_json" "$LOG_FILE"
import json, sys, urllib.request
cycle, trader_id, decision_json, log_file = sys.argv[1:5]
try:
    decisions = json.loads(decision_json)
except Exception:
    raise SystemExit(0)
acts = {str(d.get('action','')) for d in decisions if isinstance(d, dict)}
if 'pause_grid' not in acts and 'cancel_all_orders' not in acts:
    raise SystemExit(0)
url = f'http://127.0.0.1:8080/api/open-orders?trader_id={trader_id}'
try:
    with urllib.request.urlopen(url, timeout=5) as r:
        payload = json.loads(r.read().decode())
except Exception:
    raise SystemExit(0)
orders = payload if isinstance(payload, list) else []
entry_orders = []
for o in orders:
    if not isinstance(o, dict):
        continue
    reduce_only = bool(o.get('reduceOnly', o.get('reduce_only', False)))
    if reduce_only:
        continue
    entry_orders.append(o.get('orderId') or o.get('order_id') or '?')
if entry_orders:
    with open(log_file, 'a', encoding='utf-8') as f:
        f.write(f"{cycle} pause-consistency mismatch live_entry_orders={','.join(map(str, entry_orders))}
")
PYCHK
}

auto_remediate_if_needed() {
  local cycle="$1"
  local err
  err="$(sql "select replace(coalesce(error_message,''), char(10), ' ') from decision_records where trader_id = '$TRADER_ID' and cycle_number = $cycle limit 1;")"
  if [[ "$err" == *"Parameter 'reduceonly' sent when not required"* ]]; then
    log "detected reduceonly regression at cycle $cycle; rebuilding backend"
    docker compose "${COMPOSE_ARGS[@]}" build nofx >>"$LOG_FILE" 2>&1 || true
    docker compose "${COMPOSE_ARGS[@]}" up -d --force-recreate nofx >>"$LOG_FILE" 2>&1 || true
    log "completed automatic backend recreate after reduceonly regression"
  fi
  check_live_pause_consistency "$cycle"
}

start_cycle="$(sql "select coalesce(max(cycle_number), 0) from decision_records where trader_id = '$TRADER_ID';")"
target_cycle=$((start_cycle + TARGET_CYCLES))
last_seen_cycle="$start_cycle"
last_report_cycle="$start_cycle"
report_seq=0

log "monitor start trader_id=$TRADER_ID start_cycle=$start_cycle target_cycle=$target_cycle report_every=$REPORT_EVERY poll_seconds=$POLL_SECONDS"

while (( last_seen_cycle < target_cycle )); do
  current_cycle="$(sql "select coalesce(max(cycle_number), 0) from decision_records where trader_id = '$TRADER_ID';")"

  if (( current_cycle > last_seen_cycle )); then
    for ((cycle=last_seen_cycle + 1; cycle<=current_cycle; cycle++)); do
      cycle_summary="$(sql "select cycle_number || '|' || timestamp || '|' || success || '|' || replace(coalesce(error_message,''), char(10), ' ') from decision_records where trader_id = '$TRADER_ID' and cycle_number = $cycle limit 1;")"
      if [[ -n "$cycle_summary" ]]; then
        IFS='|' read -r seen_cycle seen_ts seen_success seen_err <<<"$cycle_summary"
        if [[ "$seen_success" == "1" ]]; then
          log "observed cycle $seen_cycle at $seen_ts success"
        else
          log "observed cycle $seen_cycle at $seen_ts failure: ${seen_err:-unknown}"
          auto_remediate_if_needed "$seen_cycle"
        fi
      fi
    done

    last_seen_cycle="$current_cycle"
  fi

  if (( last_seen_cycle - last_report_cycle >= REPORT_EVERY )); then
    report_seq=$((report_seq + 1))
    write_report "$((last_report_cycle + 1))" "$((last_report_cycle + REPORT_EVERY))" "$report_seq"
    last_report_cycle=$((last_report_cycle + REPORT_EVERY))
  fi

  sleep "$POLL_SECONDS"
done

if (( last_report_cycle < last_seen_cycle )); then
  report_seq=$((report_seq + 1))
  write_report "$((last_report_cycle + 1))" "$last_seen_cycle" "$report_seq"
fi

log "monitor completed trader_id=$TRADER_ID final_cycle=$last_seen_cycle target_cycle=$target_cycle"
