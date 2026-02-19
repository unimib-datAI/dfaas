#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
UV_CACHE_DIR="${UV_CACHE_DIR:-$REPO_ROOT/.uv-cache}"
export UV_CACHE_DIR

HOST="${HOST:-0.0.0.0}"
PORT="${PORT:-8000}"
RELOAD="${RELOAD:-0}"

UVICORN_ARGS=(main:app --host "$HOST" --port "$PORT")
if [[ "$RELOAD" == "1" || "$RELOAD" == "true" ]]; then
  UVICORN_ARGS+=(--reload)
fi

uv run uvicorn "${UVICORN_ARGS[@]}"
