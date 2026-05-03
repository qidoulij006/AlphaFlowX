#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/.env}"
DB_PATH="${DB_PATH:-$ROOT_DIR/data/data.db}"
SERVICE_NAME="${SERVICE_NAME:-nofx}"

if [[ -f "$ENV_FILE" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" != *=* ]] && continue
    key="${line%%=*}"
    value="${line#*=}"
    if [[ "$value" =~ ^\".*\"$ ]]; then
      value="${value:1:${#value}-2}"
    elif [[ "$value" =~ ^\'.*\'$ ]]; then
      value="${value:1:${#value}-2}"
    fi
    export "$key=$value"
  done < "$ENV_FILE"
fi

if [[ -n "${NOFX_TIMEZONE:-}" && -z "${PROMPT_OPT_DISPLAY_TIMEZONE:-}" ]]; then
  export PROMPT_OPT_DISPLAY_TIMEZONE="$NOFX_TIMEZONE"
fi
if [[ -n "${TZ:-}" && -z "${PROMPT_OPT_DISPLAY_TIMEZONE:-}" ]]; then
  export PROMPT_OPT_DISPLAY_TIMEZONE="$TZ"
fi

"$ROOT_DIR/bin/promptopt" \
  --once \
  --db "$DB_PATH" \
  --compose-dir "$ROOT_DIR" \
  --service "$SERVICE_NAME" \
  "$@"
