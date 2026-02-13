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
# - rclone is installed on the remote host at: ~/.local/bin/rclone and is
#   properly configured with access to the target Google Drive.
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

export TRACE_PATH=input_requests_scaled_traces.json
#export LIMIT=2

# The DFaaS server.
readonly SSH_SERVER="emanuele@10.0.2.38"

readonly DATE="$(date +%Y%m%d)"

readonly -a RESULT_FILES=(k6_results.csv.gz k6_report.html "$TRACE_PATH")

# Extract nodes from function "0" in the trace JSON.
# The JSON has structure: {"0": {"node0": ..., "node1": ...}, "1": ...}
readarray -t NODES < <(jq -r '.["0"] | keys[]' "$TRACE_PATH")
readonly -a NODES
if [ ${#NODES[@]} -eq 0 ]; then
    echo "Error: No nodes found in function '0' of $TRACE_PATH" >&2
    exit 1
fi
echo "Discovered ${#NODES[@]} node(s) from trace: ${NODES[*]}"

# Loop over all nodes extracted from the trace.
for NODE in "${NODES[@]}"; do
    DIRNAME="${DATE}_auto_test_node_${NODE}"
    UPLOAD_PATH="gdrive:archive/dfaas/load_tests/$DIRNAME"

    echo "Running k6 load test on node $NODE..."
    k6 run single_trace.js --out csv=k6_results.csv.gz --env NODE=$NODE
    echo "k6 load test completed."

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

    # We suppose rclone is installed and configured like in the local host.
    echo "Uploading Prometheus metrics to remote storage..."
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" \
        .local/bin/rclone copy dfaas/k8s/scripts/prometheus2csv/prometheus_metrics.csv.gz "$UPLOAD_PATH"
    ssh -i ~/.ssh/id_ed25519 "$SSH_SERVER" \
        .local/bin/rclone copy dfaas/k8s/scripts/prometheus2csv/metrics.csv "$UPLOAD_PATH"
    echo "Prometheus metrics uploaded."

    # Let DFaaS node go back to normal operations.
    echo "Waiting 120 seconds for DFaaS node to return to normal operations..."
    sleep 120s
    echo "Done. Test for node $NODE completed successfully."
done

echo "All tests completed successfully."
