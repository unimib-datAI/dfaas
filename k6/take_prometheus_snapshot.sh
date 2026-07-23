#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This script automatically creates a Prometheus snapshot on a remote host,
# whose IP address is provided as an argument, and downloads it to the local
# host at the directory specified as another argument.
#
# The main purpose of this script is to preserve a copy of the Prometheus
# metrics collected during an experiment, avoiding concerns about data retention
# limits. This allows the data to be analyzed freely (e.g., with Grafana) while
# the remote host is running another experiment.
#
# This script assumes:
#
#   - The local host have bash, curl, jq, and scp installed.
#   - The local host can connect, without interaction, to the remote host at the
#   given IP address with the user "user" (with a key-based auth).
#   - The local host can reach the remote host with SSH (port 22) and Prometheus
#   API (port 30909).
#   - The remote host has a Kubernetes cluster accessible via kubectl.
#   - The remote host user' can run sudo without interaction.
#   - The Prometheus service on the remote host has the admin API enabled
#   (option --web.enable-admin-api) and exposed on the remote host'IP at port
#   30909. The `values.yaml` file provided with DFaaS for the Prometheus Helm
#   chart already enables the admin API.

# Exit on error.
set -euo pipefail

usage() {
    cat <<EOF
Usage:
  $0 REMOTE_IP DEST_DIR

Arguments:
  REMOTE_IP   IP address of the remote host
  DEST_DIR    Local absolure or relative directory where tsdb.tar.zst will be stored

Examples:
  $0 192.168.1.10 /backups
EOF
}

# Help requested.
if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
fi

# Require exactly two positional arguments.
if [[ $# -ne 2 ]]; then
    usage
    exit 1
fi

REMOTE_IP="$1"
DEST_DIR="$2"

mkdir -p "$DEST_DIR"

SSH_USER="user"
SSH="${SSH_USER}@${REMOTE_IP}"

# SSH options: we disable strict host key checking because the remote hosts are
# VMs that are frequently recreated, and we operate in an isolated test bed
# environment.
SSH_OPTS=(
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
)

echo "Creating Prometheus snapshot..."
SNAPSHOT_JSON=$(curl --fail --silent --show-error --request POST "http://${REMOTE_IP}:30909/api/v1/admin/tsdb/snapshot")

SNAPSHOT_NAME=$(printf '%s\n' "$SNAPSHOT_JSON" | jq --raw-output '.data.name')
if [[ -z "$SNAPSHOT_NAME" || "$SNAPSHOT_NAME" == "null" ]]; then
    echo "Prometheus did not return a snapshot name."
    printf '%s\n' "$SNAPSHOT_JSON"
    exit 1
fi
echo "Snapshot: $SNAPSHOT_NAME"

echo "Finding Prometheus pod..."
POD_NAME=$(ssh "${SSH_OPTS[@]}" "$SSH" "sudo kubectl get pods --no-headers -o custom-columns=:metadata.name | grep '^prometheus-' | head -n1")
if [[ -z "$POD_NAME" ]]; then
    echo "Could not determine Prometheus pod."
    exit 1
fi
echo "Pod: $POD_NAME"

# Wait some seconds to let snapshost creation.
sleep 60s

# Make sure to remove any existing older snapshot from the user's home
# directory. This is why the `rm` command is required before running `kubectl
# cp`.
#
# sudo is required also for rm because kubectl cp preserve root permissions.
echo "Copying snapshot from pod to remote host..."
ssh "${SSH_OPTS[@]}" "$SSH" "set -e
    sudo rm --recursive --force tsdb tsdb.tar.zst
    sudo kubectl cp --container=prometheus-server ${POD_NAME}:/data/snapshots/${SNAPSHOT_NAME} tsdb"

# Required to prevent the volume inside the pod from filling up. Note the rm
# command inside the pod is the BusyBox implementation, it supports only short
# arguments!
echo "Removing snapshot from Prometheus pod..."
ssh "${SSH_OPTS[@]}" "$SSH" "sudo kubectl exec --container=prometheus-server ${POD_NAME} -- rm -rf /data/snapshots/${SNAPSHOT_NAME}"

echo "Compressing snapshot on remote host..."
ssh "${SSH_OPTS[@]}" "$SSH" "tar --create --zstd --file=tsdb.tar.zst tsdb"

echo "Downloading archive..."
scp "$SSH:tsdb.tar.zst" "${DEST_DIR}/tsdb.tar.zst"

echo "Cleaning remote files..."
ssh "${SSH_OPTS[@]}" "$SSH" "sudo rm --recursive --force tsdb tsdb.tar.zst"

echo
echo "Completed. Prometheus TSDB archive stored at: $(realpath "${DEST_DIR}/tsdb.tar.zst")"

