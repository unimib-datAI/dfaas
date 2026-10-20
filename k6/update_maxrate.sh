#!/usr/bin/env bash
#
# Update the maxrate of the mlimage OpenFaaS function on remote nodes.
#
# Example:
#   ./update_maxrate.sh \
#       --nodes "dfaas-node-a,dfaas-node-b,dfaas-node-c" \
#       --max-rate-rps 10

set -euo pipefail

FUNCTION_NAME="mlimage"
FUNCTION_IMAGE="ghcr.io/unimib-datai/dfaas-openfaas-image-classification:dev"

FUNCTION_TIMEOUT="4s"
FUNCTION_MAX_INFLIGHT="400"
FUNCTION_TIMEOUT_MS="6000"

SSH_KEY="${HOME}/.ssh/id_ed25519"

usage() {
    echo "Usage: $0 --nodes node1,node2,... --max-rate-rps RATE"
    exit 1
}

NODES=""
MAX_RATE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --nodes)
            [[ $# -ge 2 ]] || usage
            NODES="$2"
            shift 2
            ;;
        --max-rate-rps)
            [[ $# -ge 2 ]] || usage
            MAX_RATE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown argument: $1" >&2
            usage
            ;;
    esac
done

if [[ -z "$NODES" || -z "$MAX_RATE" ]]; then
    usage
fi

if [[ ! "$MAX_RATE" =~ ^[0-9]+$ ]] || (( MAX_RATE <= 0 )); then
    echo "Error: --max-rate-rps must be a positive integer." >&2
    exit 1
fi

SSH_OPTIONS=(
    -i "$SSH_KEY"
    -o IdentitiesOnly=yes
    -o StrictHostKeyChecking=no
    -o UserKnownHostsFile=/dev/null
    -o LogLevel=ERROR
)

# Parse and deduplicate the comma-separated node list.
IFS=',' read -r -a RAW_NODES <<< "$NODES"
UNIQUE_NODES=()
declare -A SEEN=()

for node in "${RAW_NODES[@]}"; do
    node="${node//[[:space:]]/}"

    [[ -z "$node" ]] && continue

    if [[ -z "${SEEN[$node]+x}" ]]; then
        UNIQUE_NODES+=("$node")
        SEEN["$node"]=1
    fi
done

if [[ ${#UNIQUE_NODES[@]} -eq 0 ]]; then
    echo "Error: --nodes must contain at least one node." >&2
    exit 1
fi

update_maxrate() {
    local node="$1"

    echo "[$node] Removing OpenFaaS function '$FUNCTION_NAME'..."

    ssh "${SSH_OPTIONS[@]}" "$node" \
        "OPENFAAS_URL=http://127.0.0.1:31112 /opt/bin/faas-cli remove '$FUNCTION_NAME'"

    sleep 5

    echo "[$node] Deploying '$FUNCTION_NAME' with maxrate=${MAX_RATE}..."

    ssh "${SSH_OPTIONS[@]}" "$node" \
        "OPENFAAS_URL=http://127.0.0.1:31112 /opt/bin/faas-cli deploy \
        --image='$FUNCTION_IMAGE' \
        --name='$FUNCTION_NAME' \
        --env 'exec_timeout=$FUNCTION_TIMEOUT' \
        --env 'max_inflight=$FUNCTION_MAX_INFLIGHT' \
        --label 'dfaas.timeout_ms=$FUNCTION_TIMEOUT_MS' \
        --label 'dfaas.maxrate=$MAX_RATE'"

    echo "[$node] Done."
}

echo "Updating maxrate to ${MAX_RATE} requests/s on ${#UNIQUE_NODES[@]} nodes..."

for node in "${UNIQUE_NODES[@]}"; do
    update_maxrate "$node"
done

echo "All nodes updated successfully."
