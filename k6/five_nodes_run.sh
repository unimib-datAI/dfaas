#!/usr/bin/env bash

# Exit on error.
set -euo pipefail

# Make sure the working directory is where this script is located!
cd "$(dirname "$0")"

TRACE_PATH="data/input_requests/mlimage/rlstrategy/scaled_pwr_5.json"

run_job() {
  local NODE_NAME="$1"
  local IP="$2"
  local NODE_ID="$3"
  local PORT="$4"

  echo "[START] $NODE_NAME (IP=$IP, NODE=$NODE_ID, PORT=$PORT)"

  export K6_WEB_DASHBOARD_PORT="$PORT"
  export K6_WEB_DASHBOARD_EXPORT="data/k6/$NODE_NAME/k6_report.html"

  mkdir -p "data/k6/$NODE_NAME"

  # Notes:
  #   - We disabled the API server.
  #
  # Output:
  #   1. Stdout and stderr (with final summary) -> data/k6/X/k6_stdout.logs
  #   2. Console logs (optional) -> data/k6/X/k6_console.logs
  #   3. CSV output -> data/k6/X/k6_results.csv.gz
  #   4. HTML summary -> data/k6/X/k6_report.html
  k6 run single_trace.js > "data/k6/$NODE_NAME/k6_stdout.logs" 2>&1 \
    --quiet \
    --address "" \
    --out csv="data/k6/$NODE_NAME/k6_results.csv.gz" \
    --console-output "data/k6/$NODE_NAME/k6_console.logs" \
    --env IP_SERVER="$IP" \
    --env TRACE_PATH="$TRACE_PATH" \
    --env FUNCTION=0 \
    --env NODE="$NODE_ID" \
    --env LIMIT=60 &

  echo "[LAUNCHED] $NODE_NAME (pid=$!)"
}

# Format: "name ip node_id port"a.
JOBS=(
  "node_c 10.12.68.2 0 30665"
  "node_d 10.12.68.3 1 30666"
  "node_e 10.12.68.4 2 30667"
  "node_f 10.12.68.5 3 30668"
  "node_g 10.12.68.6 4 30669"
)

# Launch all jobs.
for job in "${JOBS[@]}"; do
  run_job $job
done

echo "[INFO] Waiting for all jobs..."

# Track failures while waiting.
failures=0
for pid in $(jobs -rp); do
  if ! wait "$pid"; then
    echo "[ERROR] Job with PID $pid failed"
    ((failures++))
  else
    echo "[DONE] Job with PID $pid completed"
  fi
done

echo "[INFO] All jobs finished. Failures: $failures"

exit $failures
