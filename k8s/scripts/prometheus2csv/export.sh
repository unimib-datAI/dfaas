#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

if [[ $# -ne 3 ]]; then
    echo "Usage: $0 <input_dir> <start date> <end date>"
    exit 1
fi

OUTPUT_BASE_DIR="$1"
START_DATE="$2"
END_DATE="$3"

PIDS=()

mkdir -p "$OUTPUT_BASE_DIR"

cp metrics.csv "$OUTPUT_BASE_DIR/metrics.csv"

run_job() {
  local NODE_NAME="$1"
  local IP="$2"
  local NODE_ID="$3"

  echo "[START] $NODE_NAME (IP=$IP, NODE=$NODE_ID)"

  mkdir "$OUTPUT_BASE_DIR/$NODE_NAME"

  /home/emanuele/ipython-env/env/bin/python main.py \
      --host "$IP" \
      --metrics-file metrics.csv \
      --start "$START_DATE" \
      --end "$END_DATE" \
      --output "$OUTPUT_BASE_DIR/$NODE_NAME" &

  local pid=$!
  PIDS+=("$pid")

  echo "[LAUNCHED] $NODE_NAME (pid=$pid)"
}

# Format: "name ip node_id"
# FIXME: Make node name and IP configurable!
JOBS=(
  "node_c 10.12.68.2 0"
  "node_d 10.12.68.3 1"
  "node_e 10.12.68.4 2"
  "node_f 10.12.68.5 3"
  "node_g 10.12.68.6 4"
)

for job in "${JOBS[@]}"; do
  read -r name ip node_id <<< "$job"
  run_job "$name" "$ip" "$node_id"
done

echo "[INFO] Waiting for all jobs..."

failures=0

for pid in "${PIDS[@]}"; do
  if ! wait "$pid"; then
    echo "[ERROR] Job with PID $pid failed"
    ((failures++))
  else
    echo "[DONE] Job with PID $pid completed"
  fi
done

echo "[INFO] All jobs finished. Failures: $failures"

exit "$failures"
