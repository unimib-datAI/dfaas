#!/usr/bin/env python3
#
# This script updates the maxrate of the mlimage OpenFaaS function on the
# specified DFaaS testbed nodes. Commands are executed remotely via SSH and each
# node is processed concurrently.
#
# This script is useful only for the Recalc strategy.
#
# Example usage:
#   ./update_maxrate.py --maxrate 10 --nodes "dfaas-node-a,dfaas-node-b"

import argparse
import shlex
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

def update_maxrate(maxrate, node):
    ssh_options = [
        "-i", str(Path.home() / ".ssh" / "id_ed25519"),
        "-o", "IdentitiesOnly=yes",
        "-o", "StrictHostKeyChecking=no",
        "-o", "UserKnownHostsFile=/dev/null",
        "-o", "LogLevel=ERROR",
    ]
    function_name = "mlimage"
    function_image = "ghcr.io/unimib-datai/dfaas-openfaas-image-classification:dev"

    function_timeout = "4s"
    function_max_inflight = "400"
    function_timeout_ms = "6000"
    wait_seconds = 5

    run_remote = lambda cmd: subprocess.run(["ssh", *ssh_options, node, f"OPENFAAS_URL=http://127.0.0.1:31112 {cmd}"], check=True)

    run_remote(f"/opt/bin/faas-cli remove {shlex.quote(function_name)}")

    time.sleep(wait_seconds)

    command = " ".join([
        "/opt/bin/faas-cli deploy",
        f"--image={shlex.quote(function_image)}",
        f"--name={shlex.quote(function_name)}",
        f"--env {shlex.quote(f'exec_timeout={function_timeout}')}",
        f"--env {shlex.quote(f'max_inflight={function_max_inflight}')}",
        f"--label {shlex.quote(f'dfaas.timeout_ms={function_timeout_ms}')}",
        f"--label {shlex.quote(f'dfaas.maxrate={maxrate}')}",
    ])
    run_remote(command)


def main():
    parser = argparse.ArgumentParser(description="Update mlimage maxrate on remote DFaaS nodes.")
    parser.add_argument(
        "--maxrate",
        type=int,
        required=True,
        help="Maximum request rate per second to configure.",
    )
    parser.add_argument(
        "--nodes",
        required=True,
        help="Comma-separated list of DFaaS nodes as SSH hostnames.",
    )
    args = parser.parse_args()

    if args.maxrate < 0:
        parser.error("--maxrate must be a non-negative integer")

    nodes = set([node.strip() for node in args.nodes.split(",")])
    if len(nodes) == 0:
        parser.error("--nodes must contain at least one node")

    failures = []
    with ThreadPoolExecutor(max_workers=len(nodes)) as executor:
        tasks = {executor.submit(update_maxrate, args.maxrate, node): node for node in nodes}
        for task in as_completed(tasks):
            node = tasks[task]
            try:
                task.result()
            except Exception as exception:
                failures.append(node)
                print(f"[{node}] ERROR: {exception}")

    if failures:
        sys.exit(f"Failed to update {len(failures)} nodes: {failures}")

    print("All nodes updated successfully.")


if __name__ == "__main__":
    main()
