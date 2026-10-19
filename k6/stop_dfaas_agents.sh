#!/usr/bin/env bash
set -euo pipefail

SSH_OPTS=(
  -i ~/.ssh/id_ed25519
  -o IdentitiesOnly=yes
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR
)

while [[ $# -gt 0 ]]; do
  case "$1" in
    --nodes)
      IFS=',' read -ra NODES <<< "$2"
      for node in "${NODES[@]}"; do
        ssh "${SSH_OPTS[@]}" "cloud-ateneo-dfaas-node-$node" "tmux send-keys -t agent C-c"
      done
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 --nodes NODE[,NODE...]"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done
