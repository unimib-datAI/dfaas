#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
# 
# This script automatically starts DFaaS agents on the testbed VMs. Agents are
# started by issuing commands via tmux using a hardcoded session name. The VMs
# are referenced by their hardcoded SSH config names. Make sure to check and
# preconfigure the VMs beforehand (SSH access, tmux session, agent configuration
# files, etc.).
#
# Example usage: ./setup_dfaas_agents.sh --main c --al c --al a --al b --rl f --rl g
set -euo pipefail

TMUX_SESSION="agent"

AL_CONFIG="node_config_al.env"
RL_CONFIG="node_config_rl.env"

AL_NODES=()
RL_NODES=()
AGENT_MASTER=""

# SSH options: we disable strict host key checking because the remote hosts are
# VMs that are frequently recreated, and we operate in an isolated test bed
# environment.
SSH_OPTS=(
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR
)

while [[ $# -gt 0 ]]; do
    case "$1" in
        # "All Local Strategy".
        --al)
            AL_NODES+=("cloud-ateneo-dfaas-node-$2")
            shift 2
            ;;
        # "RL Agent Strategy".
        --rl)
            RL_NODES+=("cloud-ateneo-dfaas-node-$2")
            shift 2
            ;;
        # Master node, it will be started as first.
        --main)
            AGENT_MASTER="cloud-ateneo-dfaas-node-$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 --main NODE [--al NODE]... [--rl NODE]..."
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            exit 1
            ;;
    esac
done

: "${AGENT_MASTER:?Error: --main is mandatory}"

# Master must belong to either AL or RL nodes.
if [[ " ${AL_NODES[*]} " == *" $AGENT_MASTER "* ]]; then
    MASTER_CONFIG="$AL_CONFIG"
elif [[ " ${RL_NODES[*]} " == *" $AGENT_MASTER "* ]]; then
    MASTER_CONFIG="$RL_CONFIG"
else
    echo "Error: master '$AGENT_MASTER' must be specified with --al or --rl" >&2
    exit 1
fi

stop() {
    ssh "${SSH_OPTS[@]}" "$1" "tmux send-keys -t '$TMUX_SESSION' C-c"
}

start() {
    ssh "${SSH_OPTS[@]}" "$1" "tmux send-keys -t '$TMUX_SESSION' './dfaasagent -config $2' Enter"
}

# Stop all nodes.
for node in "${AL_NODES[@]}" "${RL_NODES[@]}"; do
    stop "$node"
done

# Start master first.
start "$AGENT_MASTER" "$MASTER_CONFIG"
sleep 5 # Wait to let workers connect immediately.

# Start AL workers.
for node in "${AL_NODES[@]}"; do
    [[ "$node" == "$AGENT_MASTER" ]] && continue
    start "$node" "$AL_CONFIG"
done

# Start RL workers.
for node in "${RL_NODES[@]}"; do
    [[ "$node" == "$AGENT_MASTER" ]] && continue
    start "$node" "$RL_CONFIG"
done

# Sleep of at least 1 minute is necessary because the RL strategy effectively
# starts after this time, as it waits for all other nodes.
sleep 2m
