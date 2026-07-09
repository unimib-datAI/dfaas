#!/bin/sh
set -eu

MODELS_ROOT="${MODELS_ROOT:-/models}"
STAGING_DIR="${STAGING_DIR:-$MODELS_ROOT/staging}"
CURRENT_LINK="${CURRENT_LINK:-$MODELS_ROOT/current}"
MANIFEST_NAME="${MANIFEST_NAME:-manifest.json}"
POLL_DEFAULT="${POLL_DEFAULT:-30}"
POLL_OVERRIDE=""
RELOAD_URL="${RELOAD_URL:-http://127.0.0.1:8000/reload}"
RELOAD_TOKEN="${RELOAD_TOKEN:-}"

log() {
  printf '%s\n' "$*"
}

read_poll_interval() {
  manifest_path="$1"
  if [ -n "$POLL_OVERRIDE" ]; then
    echo "$POLL_OVERRIDE"
    return
  fi

  if [ -f "$manifest_path" ] && command -v jq >/dev/null 2>&1; then
    value="$(jq -r '.sidecar.poll_interval_seconds // empty' "$manifest_path")"
    case "$value" in
      ''|*[!0-9]*)
        ;;
      *)
        if [ "$value" -gt 0 ]; then
          echo "$value"
          return
        fi
        ;;
    esac
  fi
  echo "$POLL_DEFAULT"
}

trigger_reload() {
  if command -v curl >/dev/null 2>&1; then
    if [ -n "$RELOAD_TOKEN" ]; then
      curl -fsS -H "x-reload-token: $RELOAD_TOKEN" -X POST "$RELOAD_URL" >/dev/null || true
    else
      curl -fsS -X POST "$RELOAD_URL" >/dev/null || true
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "$RELOAD_TOKEN" ]; then
      wget -qO- --header="x-reload-token: $RELOAD_TOKEN" "$RELOAD_URL" >/dev/null || true
    else
      wget -qO- "$RELOAD_URL" >/dev/null || true
    fi
  fi
}

usage() {
  cat <<'EOF'
Usage: sidecar.sh [--poll-interval SECONDS]

Options:
  --poll-interval  Override poll interval (seconds).
  -h, --help       Show this help.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --poll-interval)
      shift
      case "${1:-}" in
        ''|*[!0-9]*)
          echo "Invalid --poll-interval value."
          exit 1
          ;;
        *)
          POLL_OVERRIDE="$1"
          shift
          ;;
      esac
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

while true; do
  if [ -f "$STAGING_DIR/READY" ]; then
    ts="$(date -u +%Y%m%dT%H%M%SZ)"
    target="$MODELS_ROOT/$ts"
    mv "$STAGING_DIR" "$target"
    ln -sfn "$target" "$CURRENT_LINK"
    log "Activated model bundle $target"
    trigger_reload
  fi

  interval="$(read_poll_interval "$CURRENT_LINK/$MANIFEST_NAME")"
  sleep "$interval"
done
