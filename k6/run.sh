#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This script runs a k6 load test against a single DFaaS node, then
# automatically collects, converts, and uploads the resulting metrics to Google
# Drive using rclone.
#
# Assumptions:
#
# - The DFaaS node is reachable via SSH using the private key: ~/.ssh/id_ed25519
#   (on the local machine).
# - The DFaaS repository is located at: ~/dfaas (on the remote host).
# - A Python virtual environment is already set up on the remote host with
#   prometheus2csv installed and ready to use.
# - rclone is installed and configured on the local host with access to Google
#   Drive.
#
# IMPORTANT: You will almost certainly need to modify the DFaaS SSH connection
# string (SSH_SERVER) in this script to match your own username and server IP
# address.

# Exit on error.
set -euo pipefail

# Make sure the working directory is where this script is located!
cd "$(dirname "$0")"

# Enable Web dashboard only to save the final report as HTML format. Warning: do
# not open the Web page, the browser will crash!
export K6_WEB_DASHBOARD=true
export K6_WEB_DASHBOARD_PORT=30665
export K6_WEB_DASHBOARD_EXPORT=k6_report.html
export K6_WEB_DASHBOARD_PERIOD=10s

# The trace as JSON format. The JSON has the following structure: {"0":
# {"node0": ..., "node1": ...}, "1": ...}, where the first level is the ID of
# the function and the second level is the ID of each node. The values are an
# array with one element (integer) for each iteration, that is the number of
# requests per second for that iteration.
export TRACE_PATH="${TRACE_PATH:-input_requests_traces.json}"

# Optionally the trace can be limited to the fist X steps and not to the full
# length.
#export LIMIT=2

# Function name, used also in the URL.
export FUNCTION_NAME="${FUNCTION_NAME:-mlimage}"

# For each k6 run, results are stored in a directory named:
#
#   ${DATE}_${EXP_NAME}_${NODE}
#
# Where: DATE is launch date of the k6 job, NODE is the ID of the evaluated
# node, and EXP_NAME is a custom string. You can control only DIRNAME_STR, and
# it must always be provided.
if ! [[ -v EXP_NAME ]]; then
  echo "Error: EXP_NAME is not set"
  exit 1
fi

# The DFaaS server.
readonly IP_SERVER="10.12.68.9"
readonly SSH_SERVER="emanuele@$IP_SERVER"

readonly DATE="$(date +%Y%m%d)"

readonly -a RESULT_FILES=(k6_results.csv.gz k6_report.html "$TRACE_PATH")

# Extract nodes from function "0" in the trace JSON.
readarray -t NODES < <(jq -r '.["0"] | keys[]' "$TRACE_PATH")
readonly -a NODES
if [ ${#NODES[@]} -eq 0 ]; then
    echo "Error: No nodes found in function '0' of $TRACE_PATH" >&2
    exit 1
fi
echo "Discovered ${#NODES[@]} node(s) from trace $TRACE_PATH: ${NODES[*]}"

# Loop over all nodes extracted from the trace.
for NODE in "${NODES[@]}"; do
    DIRNAME="${DATE}_${EXP_NAME}_node_${NODE}"
    UPLOAD_PATH="gdrive:archive/dfaas/load_tests/$DIRNAME"
    echo "Upload path (at the end): $UPLOAD_PATH"

    echo "Running k6 load test on node $NODE..."
    k6 run single_trace.js --out csv=k6_results.csv --env NODE=$NODE --env IP_SERVER=$IP_SERVER --env FUNCTION_NAME=$FUNCTION_NAME --no-thresholds --summary-mode=disabled
    echo "k6 load test completed."

    echo "Compress k6 results CSV file with gzip..."
    gzip --force k6_results.csv
    echo "CSV file compressed."

    echo "Creating remote upload directory: $UPLOAD_PATH..."
    rclone mkdir "$UPLOAD_PATH"
    echo "Remote directory created."

    echo "Uploading result files to remote storage..."
    for file in "${RESULT_FILES[@]}"; do
        rclone copy "$file" "$UPLOAD_PATH"
    done
    echo "All result files uploaded."

    # Wee need start and end time to download metrics as CSV from Prometheus.
    echo "Extracting start and end time from k6 results..."
    read start end < <(
        .env/bin/python extract_start_end_time.py k6_results.csv.gz | awk -F': ' '
          /Start:/ {start = $2}
          /End:/ {end = $2}
          END {
            cmd = "date -d \"" start " 1 minute ago\" +%Y-%m-%dT%H:%M:%S"
            cmd | getline s1; close(cmd)
            cmd = "date -d \"" end " 1 minute\" +%Y-%m-%dT%H:%M:%S"
            cmd | getline e1; close(cmd)
            print s1, e1
          }
        '
    )

    echo "Running Prometheus metrics export on remote server..."
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" \
        dfaas/k8s/scripts/prometheus2csv/.env/bin/python \
        dfaas/k8s/scripts/prometheus2csv/main.py \
        --metrics-file dfaas/k8s/scripts/prometheus2csv/metrics.csv \
        --start "$start" \
        --end "$end"
    echo "Prometheus metrics export completed."

    # We can directly call the HTTP Web server because it is exposed as NodePort
    # service at 30808 port. This is the stick-table-exporter service, see
    # k8s/scripts/stick-table-exporter for more information.
    #
    # This service only accepts UNIX timestamps!
    start_unix=$(date -d "$start" +%s)
    end_unix=$(date -d "$end" +%s)
    echo "Extracting k6 stages on remote server..."
    curl --silent --show-error --header "Accept: text/csv" \
        --output dfaas_node_k6_stages.csv \
        "http://$IP_SERVER:30808/table?start=$start_unix&end=$end_unix"
    echo "k6 stages on remote server extracted"

    echo "Downloading Prometheus metrics and k6 stages from remote server..."
    scp -i ~/.ssh/id_ed25519 "$SSH_SERVER:prometheus_metrics.csv.gz" prometheus_metrics.csv.gz
    scp -i ~/.ssh/id_ed25519 "$SSH_SERVER:dfaas/k8s/scripts/prometheus2csv/metrics.csv" metrics.csv
    echo "Prometheus metrics and k6 stages downloaded."

    echo "Uploading Prometheus metrics and k6 stages to remote storage..."
    rclone copy prometheus_metrics.csv.gz "$UPLOAD_PATH"
    rclone copy metrics.csv "$UPLOAD_PATH"
    rclone copy dfaas_node_k6_stages.csv "$UPLOAD_PATH"
    echo "Prometheus metrics and k6 stages uploaded."

    # Let DFaaS node go back to normal operations.
    echo "Restarting OpenFaaS Gateway..."
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" sudo kubectl rollout restart deployment gateway
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" sudo kubectl rollout status deployment gateway --watch --timeout=1m
    echo "OpenFaaS Gateway restarted."

    echo "Restarting $FUNCTION_NAME pods.."
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" sudo kubectl rollout restart deployment "$FUNCTION_NAME"
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" sudo kubectl rollout status deployment "$FUNCTION_NAME" --watch --timeout=1m
    echo "$FUNCTION_NAME pods restarted."

    echo "Waiting 120 seconds for DFaaS node to return to normal operations..."
    sleep 120s
    echo "Done. Test for node $NODE completed successfully."
    echo
done

echo "All tests completed successfully."
