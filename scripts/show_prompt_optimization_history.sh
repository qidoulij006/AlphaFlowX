#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/data/data.db}"
DEFAULT_TRADER_ID="62ce7c57_32162195-6112-4e49-aba3-cd5a2baf7b4e_qwen_1774115363"
DISPLAY_TZ="${DISPLAY_TZ:-Asia/Shanghai}"

TRADER_ID="${1:-$DEFAULT_TRADER_ID}"
LIMIT="${2:-10}"

format_ts() {
  local raw="${1:-}"
  if [[ -z "$raw" ]]; then
    printf '%s' ""
    return
  fi
  if [[ "$raw" == *" UTC" ]]; then
    raw="${raw% UTC}"
  elif [[ ! "$raw" =~ [+-][0-9]{4}$ ]]; then
    raw="$raw +0000"
  fi
  TZ="$DISPLAY_TZ" date -d "$raw" '+%Y-%m-%d %H:%M:%S %Z'
}

printf 'display_timezone: %s\n\n' "$DISPLAY_TZ"
printf '%-6s %-18s %-22s %-22s %-22s %-7s %-12s %-12s %-10s %-10s %-8s %-40s %-40s %-s\n' \
  "round" "status" "scheduled_at" "completed_at" "next_run_at" "restart" "equity" "equity_delta" "pnl_pct" "pnl_delta" "open" "profit_summary" "change_summary" "error"

sqlite3 -separator $'\t' "$DB_PATH" "
SELECT
  round_number,
  status,
  scheduled_at,
  completed_at,
  next_run_at,
  restart_triggered,
  round(equity_before, 4) AS equity_before,
  round(equity_change_since_last, 4) AS equity_delta,
  round(pnl_pct_before, 2) AS pnl_pct_before,
  round(pnl_pct_change_since_last, 2) AS pnl_pct_delta,
  open_position_count,
  substr(replace(profit_summary, char(10), ' '), 1, 120) AS profit_summary,
  substr(replace(change_summary, char(10), ' '), 1, 120) AS change_summary,
  replace(error_message, char(10), ' ') AS error_message
FROM trader_prompt_optimization_runs
WHERE trader_id = '$TRADER_ID'
ORDER BY round_number DESC
LIMIT $LIMIT;
" | while IFS=$'\t' read -r round status scheduled completed next_run restart equity equity_delta pnl_pct pnl_delta open_count profit change error; do
  printf '%-6s %-18s %-22s %-22s %-22s %-7s %-12s %-12s %-10s %-10s %-8s %-40s %-40s %-s\n' \
    "$round" \
    "$status" \
    "$(format_ts "$scheduled")" \
    "$(format_ts "$completed")" \
    "$(format_ts "$next_run")" \
    "$restart" \
    "$equity" \
    "$equity_delta" \
    "$pnl_pct" \
    "$pnl_delta" \
    "$open_count" \
    "$profit" \
    "$change" \
    "$error"
done
