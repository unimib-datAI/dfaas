#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="${MODELS_BASE_DIR:-.}"
MODELS_TYPE="${MODELS_TYPE:-}"
MODEL_VERSION="${MODEL_VERSION:-$(date -u +%Y-%m-%dT%H%M%SZ)}"
MODEL_CREATED_AT="${MODEL_CREATED_AT:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
OUTPUT="${OUTPUT:-manifest.json}"
SIDECAR_POLL_INTERVAL_SECONDS="${SIDECAR_POLL_INTERVAL_SECONDS:-}"

if [[ -z "$MODELS_TYPE" ]]; then
  if [[ -f "$BASE_DIR/config/runtime.json" ]]; then
    MODELS_TYPE="$(jq -r '.models_type // empty' "$BASE_DIR/config/runtime.json")"
  fi
fi

if [[ -z "$MODELS_TYPE" ]]; then
  echo "MODELS_TYPE is required (env or config/runtime.json)."
  exit 1
fi

if [[ -z "$SIDECAR_POLL_INTERVAL_SECONDS" && -f "$BASE_DIR/config/runtime.json" ]]; then
  SIDECAR_POLL_INTERVAL_SECONDS="$(jq -r '.sidecar_poll_interval_seconds // empty' \
    "$BASE_DIR/config/runtime.json")"
fi

metrics=("cpu_usage_node" "ram_usage_node" "power_usage_node")

sha256='{}'
add_hash() {
  local rel_path="$1"
  local abs_path="$BASE_DIR/$rel_path"
  if [[ ! -f "$abs_path" ]]; then
    echo "Required file not found: $rel_path"
    exit 1
  fi
  local hash
  hash="$(shasum -a 256 "$abs_path" | awk '{print $1}')"
  sha256="$(jq -c --arg path "$rel_path" --arg hash "$hash" '. + {($path): $hash}' <<<"$sha256")"
}

add_hash "scalers/scaler_x/features.joblib"
for metric in "${metrics[@]}"; do
  add_hash "scalers/scaler_y/${metric}.joblib"
  add_hash "models/${metric}/${MODELS_TYPE}/model.joblib"
done

manifest="$(jq -n \
  --arg model_version "$MODEL_VERSION" \
  --arg models_type "$MODELS_TYPE" \
  --arg created_at "$MODEL_CREATED_AT" \
  --argjson sha256 "$sha256" \
  --arg sidecar_poll_interval "$SIDECAR_POLL_INTERVAL_SECONDS" \
  '{
    model_version:$model_version,
    models_type:$models_type,
    created_at:$created_at,
    sha256:$sha256
  } + ( ($sidecar_poll_interval | tonumber?) as $poll
        | if $poll != null then {sidecar:{poll_interval_seconds:$poll}} else {} end )')"

output_path="$OUTPUT"
if [[ "$output_path" != /* ]]; then
  output_path="$BASE_DIR/$output_path"
fi

printf '%s\n' "$manifest" > "$output_path"
echo "Wrote $output_path"
