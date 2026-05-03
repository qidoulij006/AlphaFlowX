#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/data/data.db}"
DEFAULT_TRADER_ID="62ce7c57_32162195-6112-4e49-aba3-cd5a2baf7b4e_qwen_1774115363"
DISPLAY_TZ="${DISPLAY_TZ:-Asia/Shanghai}"

TRADER_ID="${1:-$DEFAULT_TRADER_ID}"
ROUND_NUMBER="${2:-1}"

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

printf 'display_timezone = %s\n' "$DISPLAY_TZ"

sqlite3 -line "$DB_PATH" "
SELECT
  round_number,
  status,
  scheduled_at,
  started_at,
  completed_at,
  next_run_at,
  deferred_until,
  restart_triggered,
  open_position_count,
  equity_before,
  equity_change_since_last,
  pnl_pct_before,
  pnl_pct_change_since_last,
  profit_summary,
  change_summary,
  analysis_summary,
  error_message,
  metrics_json
FROM trader_prompt_optimization_runs
WHERE trader_id = '$TRADER_ID'
  AND round_number = $ROUND_NUMBER;
" | while IFS= read -r line; do
  case "$line" in
    *"scheduled_at = "*)
      printf '             scheduled_at = %s\n' "$(format_ts "${line#*= }")"
      ;;
    *"started_at = "*)
      printf '               started_at = %s\n' "$(format_ts "${line#*= }")"
      ;;
    *"completed_at = "*)
      printf '             completed_at = %s\n' "$(format_ts "${line#*= }")"
      ;;
    *"next_run_at = "*)
      printf '              next_run_at = %s\n' "$(format_ts "${line#*= }")"
      ;;
    *"deferred_until = "*)
      printf '           deferred_until = %s\n' "$(format_ts "${line#*= }")"
      ;;
    *)
      printf '%s\n' "$line"
      ;;
  esac
done
