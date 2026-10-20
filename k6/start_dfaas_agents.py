#!/usr/bin/env python3
#
# Stop and start DFaaS agents for the specified nodes and strategies.
#
# Example:
#   ./setup_dfaas_agents.py \
#       --strategy-recalc "dfaas-node-a,dfaas-node-b" \
#       --strategy-all-local "dfaas-node-c,dfaas-node-d" \
#       --strategy-rl "dfaas-node-f,dfaas-node-g" \
#       --main-node "dfaas-node-c"

import argparse
import shlex
import subprocess
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

# Both the tmux session and the configuration files must exist beforehand.
TMUX_SESSION = "agent"

STRATEGY_CONFIGS = {
    "recalc": "../node_config_recalc.env",
    "all-local": "../node_config_all_local.env",
    "rl": "../node_config_rl.env",
}

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


def run_concurrently(nodes, function, *args):
    """Execute a function on all nodes concurrently and report failures."""
    if not nodes:
        return

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


def stop_agent(node):
    """Stop the DFaaS agent running in tmux on a node."""
    command = f"tmux send-keys -t {shlex.quote(TMUX_SESSION)} C-c"
    subprocess.run(
        ["ssh", *SSH_OPTIONS, node, command],
        check=True,
    )


def start_agent(node, config):
    """Start the DFaaS agent using the specified configuration."""
    agent_command = f"./dfaasagent -config {shlex.quote(config)}"
    command = (
        f"tmux send-keys -t {shlex.quote(TMUX_SESSION)} "
        f"{shlex.quote(agent_command)} Enter"
    )
    subprocess.run(
        ["ssh", *SSH_OPTIONS, node, command],
        check=True,
    )


def main():
    parser = argparse.ArgumentParser(
        description="Stop and restart DFaaS agents using the selected strategies."
    )

    parser.add_argument(
        "--strategy-recalc",
        default="",
        help="Comma-separated SSH hostnames using the Recalc strategy.",
    )
    parser.add_argument(
        "--strategy-all-local",
        default="",
        help="Comma-separated SSH hostnames using the All-Local strategy.",
    )
    parser.add_argument(
        "--strategy-rl",
        default="",
        help="Comma-separated SSH hostnames using the RL strategy.",
    )
    parser.add_argument(
        "--main-node",
        required=True,
        help="SSH hostname of the main node. Must belong to one strategy.",
    )

    args = parser.parse_args()

    parse_nodes = lambda value: list(
        dict.fromkeys(node.strip() for node in value.split(",") if node.strip())
    )
    strategy_nodes = {
        "recalc": parse_nodes(args.strategy_recalc),
        "all-local": parse_nodes(args.strategy_all_local),
        "rl": parse_nodes(args.strategy_rl),
    }

    # Build the complete node list while preserving strategy order.
    all_nodes = [node for nodes in strategy_nodes.values() for node in nodes]

    if not all_nodes:
        parser.error(
            "Specify at least one node using --strategy-recalc, --strategy-all-local, or --strategy-rl."
        )

    # A node must belong to exactly one strategy.
    seen = set()
    duplicates = set()

    for nodes in strategy_nodes.values():
        for node in nodes:
            if node in seen:
                duplicates.add(node)
            seen.add(node)

    if duplicates:
        parser.error(
            "Nodes cannot belong to multiple strategies: "
            + ", ".join(sorted(duplicates))
        )

    if args.main_node not in all_nodes:
        parser.error("--main-node must be included in one of the strategy node lists.")

    # Determine the configuration file for each node.
    node_configs = {
        node: STRATEGY_CONFIGS[strategy]
        for strategy, nodes in strategy_nodes.items()
        for node in nodes
    }

    print("Stopping all DFaaS agents", flush=True)
    run_concurrently(all_nodes, stop_agent)

    print("Starting main DFaaS agent", flush=True)
    start_agent(args.main_node, node_configs[args.main_node])
    time.sleep(5)

    workers = [node for node in all_nodes if node != args.main_node]

    print("Starting all other DFaaS agents concurrently", flush=True)
    run_concurrently(workers, lambda node: start_agent(node, node_configs[node]))

    print("Waiting 60 seconds for the agents to initialize...", flush=True)
    time.sleep(60)
    print("All agents started successfully.")


if __name__ == "__main__":
    main()
