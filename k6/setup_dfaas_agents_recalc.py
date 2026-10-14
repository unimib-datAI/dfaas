#!/usr/bin/env python3
#
# Stop and start DFaaS agents on the specified nodes and update the
# maxrate of the mlimage OpenFaaS function. For the recalc strategy.
#
# Example:
#   ./setup_dfaas_agents_recalc.py \
#       --nodes "dfaas-node-a,dfaas-node-b,dfaas-node-c,dfaas-node-f,dfaas-node-g" \
#       --main-node "dfaas-node-c" \
#       --max-rate-rps 10

import argparse
import shlex
import subprocess
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

# Both the tmux session and the configuration file must exist beforehand.
TMUX_SESSION = "agent"
AGENT_CONFIG = "../node_config_recalc.env"

SSH_OPTIONS = [
    "-i",
    str(Path.home() / ".ssh" / "id_ed25519"),
    "-o",
    "IdentitiesOnly=yes",
    "-o",
    "StrictHostKeyChecking=no",
    "-o",
    "UserKnownHostsFile=/dev/null",
    "-o",
    "LogLevel=ERROR",
]

FUNCTION_NAME = "mlimage"
FUNCTION_IMAGE = "ghcr.io/unimib-datai/dfaas-openfaas-image-classification:dev"

FUNCTION_TIMEOUT = "4s"
FUNCTION_MAX_INFLIGHT = "400"
FUNCTION_TIMEOUT_MS = "6000"


def update_maxrate(maxrate, node):
    """Recreate the mlimage function with the requested maxrate."""

    def run_remote(command):
        # The OPENFAAS_URL env var is required, otherwise faas-cli won't work
        # properly.
        remote_command = "OPENFAAS_URL=http://127.0.0.1:31112 " + command
        subprocess.run(
            ["ssh", *SSH_OPTIONS, node, remote_command],
            check=True,
        )

    run_remote(shlex.join(["/opt/bin/faas-cli", "remove", FUNCTION_NAME]))

    time.sleep(5)

    deploy_command = shlex.join(
        [
            "/opt/bin/faas-cli",
            "deploy",
            f"--image={FUNCTION_IMAGE}",
            f"--name={FUNCTION_NAME}",
            "--env",
            f"exec_timeout={FUNCTION_TIMEOUT}",
            "--env",
            f"max_inflight={FUNCTION_MAX_INFLIGHT}",
            "--label",
            f"dfaas.timeout_ms={FUNCTION_TIMEOUT_MS}",
            "--label",
            f"dfaas.maxrate={maxrate}",
        ]
    )
    run_remote(deploy_command)


def run_concurrently(nodes, function, *args):
    """Execute a function on all nodes concurrently and report failures."""
    failures = []

    with ThreadPoolExecutor(max_workers=len(nodes)) as executor:
        tasks = {executor.submit(function, *args, node): node for node in nodes}

        for task in as_completed(tasks):
            node = tasks[task]
            try:
                task.result()
            except Exception as exception:
                failures.append(node)
                print(f"[{node}] ERROR: {exception}", flush=True)

    if failures:
        raise RuntimeError(
            f"Operation failed on {len(failures)} nodes: {', '.join(sorted(failures))}"
        )


def main():
    parser = argparse.ArgumentParser(
        description="Configure the mlimage maxrate and restart Recalc DFaaS agents on remote nodes."
    )
    parser.add_argument(
        "--nodes", required=True, help="Comma-separated list of SSH hostnames."
    )
    parser.add_argument(
        "--main-node", required=True, help="SSH hostname of the master node."
    )
    parser.add_argument(
        "--max-rate-rps",
        type=int,
        required=True,
        help="Maximum request rate per second.",
    )
    args = parser.parse_args()

    if args.max_rate_rps <= 0:
        parser.error("--max-rate-rps must be positive")

    nodes = list(
        dict.fromkeys(node.strip() for node in args.nodes.split(",") if node.strip())
    )
    if not nodes:
        parser.error("--nodes must contain at least one node")
    if args.main_node not in nodes:
        parser.error("--main-node must be included in --nodes")

    print("Stopping all DFaaS agents", flush=True)
    for node in nodes:
        command = f"tmux send-keys -t {shlex.quote(TMUX_SESSION)} C-c"
        subprocess.run(
            ["ssh", *SSH_OPTIONS, node, command],
            check=True,
        )

    print(f"Updating OpenFaaS maxrate to {args.max_rate_rps} rps", flush=True)
    run_concurrently(nodes, update_maxrate, args.max_rate_rps)

    print("Starting main DFaaS agent", flush=True)
    agent_command = f"./dfaasagent -config {shlex.quote(AGENT_CONFIG)}"
    command = (
        f"tmux send-keys -t {shlex.quote(TMUX_SESSION)} "
        f"{shlex.quote(agent_command)} Enter"
    )
    subprocess.run(
        ["ssh", *SSH_OPTIONS, args.main_node, command],
        check=True,
    )
    time.sleep(5)

    print("Starting all other DFaaS agents concurrently", flush=True)
    workers = [node for node in nodes if node != args.main_node]
    run_concurrently(
        workers,
        lambda node: subprocess.run(
            [
                "ssh",
                *SSH_OPTIONS,
                node,
                (f"tmux send-keys -t {shlex.quote(TMUX_SESSION)} "
                f"{shlex.quote(agent_command)} Enter"),
            ],
            check=True,
        ),
    )

    print("Waiting 120 seconds for the agents to initialize...", flush=True)
    time.sleep(120)

    print("All agents started successfully.", flush=True)


if __name__ == "__main__":
    main()
