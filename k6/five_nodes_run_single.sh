#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:

# Exit on error.
set -euo pipefail

# Make sure the working directory is where this script is located!
cd "$(dirname "$0")"

echo "[INFO] Start at $(date '+%Y-%m-%dT%H:%M:%S%z [%Z] epoch=%s')"

# SSH connection info. It assumes that each DFaaS node can be accessed using the
# provided private key, and that the corresponding public key has been added to
# the authorized_keys file for the "user" account.
SSH_OPTS=(
  -i ~/.ssh/id_ed25519
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=QUIET
)

DEFAULT_TRACE_PATH="../data/input_requests/mlimage/rlstrategy/scaled_pwr_40_only_0_other_constant_4.json"
DEFAULT_OUTPUT_BASE_DIR="../data/20260525_one_rl_agent"

# Allow to overwrite these options via environment variables.
TRACE_PATH="${TRACE_PATH:-$DEFAULT_TRACE_PATH}"
OUTPUT_BASE_DIR="${OUTPUT_BASE_DIR:-$DEFAULT_OUTPUT_BASE_DIR}"

run_job() {
  local NODE_NAME="$1"
  local IP="$2"
  local NODE_ID="$3"
  local PORT="$4"

  local readonly RL_MODEL_PATH_SRC="dfaas-rl/dfaasagent/rl_model.log"
  ssh "${SSH_OPTS[@]}" user@$IP "rm --force '$RL_MODEL_PATH_SRC'"
  echo "[INFO] $NODE_NAME Deleted $RL_MODEL_PATH_SRC (if exists)"

  echo "[START] $NODE_NAME (IP=$IP, NODE=$NODE_ID, PORT=$PORT)"

  export K6_WEB_DASHBOARD_PORT="$PORT"
  export K6_WEB_DASHBOARD_EXPORT="$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_report.html"

  mkdir -p "$OUTPUT_BASE_DIR/k6/$NODE_NAME"

  # Notes:
  #   - We disabled the API server.
  #
  # Output:
  #   1. Stdout and stderr (with final summary) -> data/k6/X/k6_stdout.logs
  #   2. Console logs (optional) -> data/k6/X/k6_console.logs
  #   3. CSV output -> data/k6/X/k6_results.csv.gz
  #   4. HTML summary -> data/k6/X/k6_report.html
  k6 run single_trace.js > "$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_stdout.logs" 2>&1 \
    --quiet \
    --address "" \
    --out csv="$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_results.csv.gz" \
    --console-output "$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_console.logs" \
    --env IP_SERVER="$IP" \
    --env TRACE_PATH="$TRACE_PATH" \
    --env FUNCTION=0 \
    --env NODE="$NODE_ID" \
    --env LIMIT=40 &

  echo "[LAUNCHED] $NODE_NAME (pid=$!)"
}

TRACE_NAME="$(basename "$TRACE_PATH")"
echo "[INFO] Using trace $TRACE_NAME from $(realpath "$TRACE_PATH")"

# Save a copy of the input trace in the result dir.
mkdir -p "$OUTPUT_BASE_DIR/k6/"
cp -a "$TRACE_PATH" "$OUTPUT_BASE_DIR/k6/"
echo "[INFO] Saved trace path to $(realpath "$OUTPUT_BASE_DIR/k6/$TRACE_NAME")"

# Format: "name ip node_id port".
JOBS=(
  "node_c 10.12.68.2 0 30665"
  "node_d 10.12.68.3 1 30666"
  "node_e 10.12.68.4 2 30667"
  "node_f 10.12.68.5 3 30668"
  "node_g 10.12.68.6 4 30669"
)

echo "[INFO] Saving results to $(realpath "$OUTPUT_BASE_DIR")"

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

echo "[INFO] Collecting rl_model.log file from DFaaS nodes..."

collect_rl_model() {
  local NODE_NAME="$1"
  local IP="$2"

  local readonly RL_MODEL_PATH_SRC="dfaas-rl/dfaasagent/rl_model.log"
  local readonly RL_MODEL_PATH_DST="$OUTPUT_BASE_DIR/other/$NODE_NAME/rl_model.log"

  mkdir -p "$OUTPUT_BASE_DIR/other/$NODE_NAME"

  # $RL_MODEL_PATH_SRC may not exist because that DFaaS agent is not executing
  # the RL Agent strategy.
  if ssh "${SSH_OPTS[@]}" user@"$IP" "test -f '$RL_MODEL_PATH_SRC'"; then
    if scp "${SSH_OPTS[@]}" user@"$IP:$RL_MODEL_PATH_SRC" "$RL_MODEL_PATH_DST"; then
      echo "[INFO] Collected rl_model.log from $NODE_NAME (IP=$IP)"
    else
      echo "[WARN] Failed to copy rl_model.log from $NODE_NAME (IP=$IP)"
    fi
  else
    echo "[INFO] No rl_model.log found on $NODE_NAME (IP=$IP)"
  fi
}

for job in "${JOBS[@]}"; do
  collect_rl_model $job
done

echo "[INFO] All (if available) rl_model.log files collected."

echo "[INFO] End at $(date '+%Y-%m-%dT%H:%M:%S%z [%Z] epoch=%s')"

exit $(( failures > 0 ? 1 : 0 ))
