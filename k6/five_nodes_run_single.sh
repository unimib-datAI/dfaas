#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
#
# TODO: Migrate this script from Bash to Python.

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
  -o IdentitiesOnly=yes
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=QUIET
)

DEFAULT_TRACE_PATH="../data/input_requests/mlimage/rlstrategy/scaled_pwr_40_only_0_other_constant_4.json"
DEFAULT_OUTPUT_BASE_DIR="../data/20260525_one_rl_agent"
DEFAULT_STAGE_BUILDER="OneMinuteWindow"

# Allow to overwrite these options via environment variables. See
# single_trace.js for more information about the env vars.
TRACE_PATH="${TRACE_PATH:-$DEFAULT_TRACE_PATH}"
OUTPUT_BASE_DIR="${OUTPUT_BASE_DIR:-$DEFAULT_OUTPUT_BASE_DIR}"
STAGE_BUILDER="${STAGE_BUILDER:-$DEFAULT_STAGE_BUILDER}"

run_job() {
  local NODE_NAME="$1"
  local IP="$2"
  local PORT="$3"

  echo "[START] $NODE_NAME (IP=$IP, PORT=$PORT)"

  export K6_WEB_DASHBOARD_PORT="$PORT"
  export K6_WEB_DASHBOARD_EXPORT="$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_report.html"

  mkdir -p "$OUTPUT_BASE_DIR/k6/$NODE_NAME"

  # Notes:
  #   - We disabled the API server.
  #
  # Output:
  #   1. Stdout and stderr (with final summary) -> data/k6/X/k6_stdout.logs
  #   3. CSV output -> data/k6/X/k6_results.csv.gz
  #   4. HTML summary -> data/k6/X/k6_report.html
  k6 run single_trace.js > "$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_stdout.logs" 2>&1 \
    --quiet \
    --no-color \
    --address "" \
    --out csv="$OUTPUT_BASE_DIR/k6/$NODE_NAME/k6_results.csv.gz" \
    --env ENDPOINT_IP="$IP" \
    --env TRACE_PATH="$TRACE_PATH" \
    --env FUNCTION=0 \
    --env NODE="$NODE_NAME" \
    --env STAGE_BUILDER="$STAGE_BUILDER" &

  echo "[LAUNCHED] $NODE_NAME (pid=$!)"
}

TRACE_NAME="$(basename "$TRACE_PATH")"
echo "[INFO] Using trace $TRACE_NAME from $(realpath "$TRACE_PATH")"

# Save a copy of the input trace in the result dir.
mkdir -p "$OUTPUT_BASE_DIR/k6/"
cp -a "$TRACE_PATH" "$OUTPUT_BASE_DIR/k6/"
echo "[INFO] Saved trace path to $(realpath "$OUTPUT_BASE_DIR/k6/$TRACE_NAME")"

# Format: "node_name ip port".
JOBS=(
  "node_a dfaas-node-a.local  30665"
  "node_b dfaas-node-b.local  30666"
  "node_c dfaas-node-c.local  30667"
  "node_f dfaas-node-f.local  30668"
  "node_g dfaas-node-g.local  30669"
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

# Wait before collecting data.
sleep 10s

collect_agent_logs() {
  local NODE_NAME="$1"
  local IP="$2"

  local readonly AGENT_LOG_PATH_SRC="dfaas-rl/dfaasagent/agent.log"
  local readonly AGENT_LOG_PATH_DST="$OUTPUT_BASE_DIR/other/$NODE_NAME/agent.log"

  # $AGENT_LOG_PATH_SRC may not exist, so we must check that case.
  if ssh "${SSH_OPTS[@]}" user@"$IP" "test -f '$AGENT_LOG_PATH_SRC'"; then
    mkdir -p "$OUTPUT_BASE_DIR/other/$NODE_NAME"

    if scp "${SSH_OPTS[@]}" user@"$IP:$AGENT_LOG_PATH_SRC" "$AGENT_LOG_PATH_DST"; then
      echo "[INFO] Collected DFaaS agent logs from $NODE_NAME (IP=$IP)"
    else
      echo "[WARN] Failed to copy DFaaS agent logs from $NODE_NAME (IP=$IP)"
    fi
  else
    echo "[INFO] No DFaaS agent logs found on $NODE_NAME (IP=$IP)"
  fi
}

echo "[INFO] Collecting DFaaS agent logs from DFaaS nodes..."
for job in "${JOBS[@]}"; do
  collect_agent_logs $job
done
echo "[INFO] All available DFaaS agent logs collected."

collect_prometheus_snapshot() {
  local readonly NODE_NAME="$1"
  local readonly IP="$2"

  mkdir -p "$OUTPUT_BASE_DIR/prom/$NODE_NAME"

  if ./take_prometheus_snapshot.sh "$IP" "$OUTPUT_BASE_DIR/prom/$NODE_NAME"; then
    echo "[INFO] Collected Prometheus snapshot from $NODE_NAME (IP=$IP)"
  else
    echo "[WARN] Failed to collect Prometheus snapshot from $NODE_NAME (IP=$IP)"
  fi
}

echo "[INFO] Collecting Prometheus snapshot from DFaaS nodes..."

prom_failures=0

# Launch and wait all snapshot collections in parallel, same as for k6 jobs.
for job in "${JOBS[@]}"; do
  collect_prometheus_snapshot $job &
done

for pid in $(jobs -rp); do
  if ! wait "$pid"; then
    echo "[WARN] Prometheus snapshot job with PID $pid failed"
    ((prom_failures++))
  fi
done

echo "[INFO] Prometheus snapshots collected. Failures: $prom_failures"

echo "[INFO] End at $(date '+%Y-%m-%dT%H:%M:%S%z [%Z] epoch=%s')"

exit $(( failures > 0 ? 1 : 0 ))
