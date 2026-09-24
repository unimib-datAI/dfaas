#!/usr/bin/env python3
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
#
# This script transfers Prometheus snapshots to DFaaS RL nodes.
#
# The snapshots contain the Prometheus TSDB data collected during an experiment.
# Restoring this data on the RL nodes allows the agent to access past
# observations and metrics after the experiment has finished, without requiring
# the original Prometheus instance to remain active. Nodes are restored in
# parallel.
#
# Expected experiment layout:
#
#   experiment/
#   ├── k6/
#   │   ├── node_a/
#   │   │   └── iteration_timestamps.csv
#   │   └── ...
#   └── prom/
#       ├── node_a/
#       │   └── tsdb.tar.zst
#       └── ...
#
# This script assumes:
#
#   - The local machine has Python 3, ssh and scp installed.
#   - SSH key-based authentication is configured for the remote user with the
#     private key at ~/.ssh/id_ed25519 with no password.
#   - The remote user is called "user" and can execute systemctl commands using
#     sudo without interactive authentication.
#   - Prometheus data is stored under /usr/local/share/prometheus.
#   - The remote machine has the "prometheus-history" systemd service installed.
#

import argparse
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

# FIXME: This should not be hardcoded!
NODE_IPS = {
    "node_a": "10.12.68.9",
    "node_b": "10.12.68.10",
    "node_c": "10.12.68.2",
    "node_d": "10.12.68.3",
    "node_e": "10.12.68.11",
    "node_f": "10.12.68.5",
    "node_g": "10.12.68.6",
}

SSH_USER = "user"

SSH_OPTS = [
    "-i",
    str(Path.home() / ".ssh/id_ed25519"),
    "-o",
    "IdentitiesOnly=yes",
    "-o",
    "StrictHostKeyChecking=no",
    "-o",
    "UserKnownHostsFile=/dev/null",
    "-o",
    "LogLevel=ERROR",
]

REMOTE_PROM_DIR = "/usr/local/share/prometheus"


def run(cmd):
    subprocess.run(cmd, check=True)


def restore_node(node, ip, experiment):
    csv = experiment / "k6" / node / "iteration_timestamps.csv"
    tsdb = experiment / "prom" / node / "tsdb.tar.zst"

    if not csv.exists() or not tsdb.exists():
        missing = []

        if not csv.exists():
            missing.append(str(csv))

        if not tsdb.exists():
            missing.append(str(tsdb))

        raise RuntimeError("missing files:\n  " + "\n  ".join(missing))

    ssh = ["ssh", *SSH_OPTS, f"{SSH_USER}@{ip}"]
    scp = ["scp", *SSH_OPTS]

    remote_csv_tmp = "/tmp/iteration_timestamps.csv"
    remote_tsdb_tmp = "/tmp/tsdb.tar.zst"

    print(f"[{node}] Stopping prometheus-history.service")
    run(ssh + ["sudo systemctl stop prometheus-history.service"])

    print(f"[{node}] Uploading iteration timestamps")
    run(scp + [str(csv), f"{SSH_USER}@{ip}:{remote_csv_tmp}"])
    run(ssh + [f"sudo mv {remote_csv_tmp} {REMOTE_PROM_DIR}/iteration_timestamps.csv"])

    print(f"[{node}] Uploading Prometheus snapshot")
    run(scp + [str(tsdb), f"{SSH_USER}@{ip}:{remote_tsdb_tmp}"])
    run(ssh + [f"sudo mv {remote_tsdb_tmp} {REMOTE_PROM_DIR}/tsdb.tar.zst"])

    print(f"[{node}] Cleaning previous TSDB and extracting snapshot")
    run(
        ssh
        + [
            f"set -e; "
            f"sudo rm -rf {REMOTE_PROM_DIR}/tsdb; "
            f"sudo tar --zstd -xf {REMOTE_PROM_DIR}/tsdb.tar.zst "
            f"-C {REMOTE_PROM_DIR}; "
            f"sudo rm -f {REMOTE_PROM_DIR}/tsdb.tar.zst"
        ]
    )

    print(f"[{node}] Starting prometheus-history.service")
    run(ssh + ["sudo systemctl start prometheus-history.service"])


def main():
    parser = argparse.ArgumentParser(
        description="Restore Prometheus history data on DFaaS nodes"
    )
    parser.add_argument("experiment", type=Path, help="Experiment directory")
    args = parser.parse_args()

    experiment = args.experiment.resolve()

    if not experiment.exists():
        print(f"Experiment directory does not exist: {experiment}", file=sys.stderr)
        sys.exit(1)

    nodes = [node for node in NODE_IPS if (experiment / "prom" / node).exists()]

    if not nodes:
        print("No Prometheus archives found in experiment directory", file=sys.stderr)
        sys.exit(1)

    print(f"Restoring Prometheus snapshots on nodes: {', '.join(nodes)}")

    failures = []

    with ThreadPoolExecutor(max_workers=len(nodes)) as executor:
        futures = {
            executor.submit(restore_node, node, NODE_IPS[node], experiment): node
            for node in nodes
        }

        for future in as_completed(futures):
            node = futures[future]

            try:
                future.result()
            except Exception as exc:
                print(f"[{node}] ERROR: {exc}", file=sys.stderr)
                failures.append(node)

    if failures:
        print("\nFailed nodes: " + ", ".join(failures), file=sys.stderr)
        sys.exit(1)

    print("\nPrometheus restore completed successfully.")


if __name__ == "__main__":
    main()
