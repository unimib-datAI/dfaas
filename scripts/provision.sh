#!/usr/bin/env bash
# provision.sh — Creates a Multipass VM and provisions it with the DFaaS Ansible playbook.
#
# Usage:
#   ./scripts/provision.sh [OPTIONS]
#
# Options:
#   --vm-name NAME          VM name (default: dfaas-node, env: VM_NAME)
#   --faas-platform PLAT    FaaS platform: openfaas|openwhisk (default: openfaas, env: FAAS_PLATFORM)
#   --cpus N                vCPUs (default: 4, env: VM_CPUS)
#   --memory SIZE           RAM (default: 8G, env: VM_MEMORY)
#   --disk SIZE             Disk size (default: 30G, env: VM_DISK)
#   --skip-sync             Skip project sync to VM (useful if already synced)
#   -h, --help              Show this help
#
# Examples:
#   ./scripts/provision.sh
#   ./scripts/provision.sh --faas-platform openwhisk
#   ./scripts/provision.sh --vm-name dfaas-1 --cpus 4 --memory 8G --faas-platform openwhisk

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e-k3s-common.sh
source "${SCRIPT_DIR}/lib/e2e-k3s-common.sh"

e2e_set_log_prefix "provision"

# ── Defaults (overridable via environment variables) ──────────────────────
VM_NAME="${VM_NAME:-dfaas-node}"
FAAS_PLATFORM="${FAAS_PLATFORM:-openfaas}"
VM_CPUS="${VM_CPUS:-4}"
VM_MEMORY="${VM_MEMORY:-8G}"
VM_DISK="${VM_DISK:-30G}"
SKIP_SYNC="${SKIP_SYNC:-false}"

REMOTE_PROJECT_DIR="/home/ubuntu/dfaas"

# ── Argument parsing ──────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --vm-name)        VM_NAME="$2";       shift 2 ;;
        --faas-platform)  FAAS_PLATFORM="$2"; shift 2 ;;
        --cpus)           VM_CPUS="$2";       shift 2 ;;
        --memory)         VM_MEMORY="$2";     shift 2 ;;
        --disk)           VM_DISK="$2";       shift 2 ;;
        --skip-sync)      SKIP_SYNC=true;     shift   ;;
        -h|--help)
            grep '^#' "$0" | grep -v '^#!/' | sed 's/^# \?//'
            exit 0
            ;;
        *)
            error "Unknown argument: $1"
            exit 1
            ;;
    esac
done

# ── Validate ──────────────────────────────────────────────────────────────
case "${FAAS_PLATFORM}" in
    openfaas|openwhisk) ;;
    *)
        error "faas_platform must be 'openfaas' or 'openwhisk' (got '${FAAS_PLATFORM}')"
        exit 1
        ;;
esac

e2e_require_multipass || exit 1

if ! command -v ansible-playbook >/dev/null 2>&1; then
    error "ansible-playbook not found. Install Ansible: pip install ansible"
    exit 1
fi

# ── Create / start VM ─────────────────────────────────────────────────────
log "Ensuring VM '${VM_NAME}' is running..."
e2e_ensure_vm_running "${VM_NAME}" "${VM_CPUS}" "${VM_MEMORY}" "${VM_DISK}"

VM_IP=$(e2e_get_vm_ip)
if [[ -z "${VM_IP}" ]]; then
    error "Cannot determine IP for VM '${VM_NAME}'"
    exit 1
fi
log "VM '${VM_NAME}' is up at ${VM_IP}"

# ── Sync project to VM ────────────────────────────────────────────────────
if [[ "${SKIP_SYNC}" != "true" ]]; then
    PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
    e2e_sync_project_to_vm "${PROJECT_ROOT}" "${VM_NAME}" "${REMOTE_PROJECT_DIR}"
fi

# ── Generate temporary Ansible inventory ─────────────────────────────────
INVENTORY=$(e2e_mktemp_file "dfaas-inventory" ".ini")
trap 'rm -f "${INVENTORY}"' EXIT

SSH_KEY_FILE="${E2E_SSH_KEY:-}"
if [[ -z "${SSH_KEY_FILE}" ]]; then
    SSH_KEY_FILE=$(e2e_detect_multipass_ssh_key || true)
fi

SSH_KEY_PARAM=""
if [[ -n "${SSH_KEY_FILE}" ]]; then
    SSH_KEY_PARAM=" ansible_ssh_private_key_file=${SSH_KEY_FILE}"
fi

cat > "${INVENTORY}" <<EOF
[all]
${VM_NAME} ansible_host=${VM_IP} ansible_user=${E2E_VM_USER:-ubuntu}${SSH_KEY_PARAM} ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'
EOF

# ── Run Ansible playbook ──────────────────────────────────────────────────
log "Running Ansible playbook (faas_platform=${FAAS_PLATFORM})..."
ansible-playbook \
    -i "${INVENTORY}" \
    -e "faas_platform=${FAAS_PLATFORM}" \
    -e "project_dir=${REMOTE_PROJECT_DIR}" \
    "${SCRIPT_DIR}/ansible/setup_playbook.yaml"

# ── Done ─────────────────────────────────────────────────────────────────
log "Provisioning complete!"
log "  VM:           ${VM_NAME}"
log "  IP:           ${VM_IP}"
log "  FaaS:         ${FAAS_PLATFORM}"
log "  SSH:          ssh ${E2E_VM_USER:-ubuntu}@${VM_IP}"
if [[ "${FAAS_PLATFORM}" == "openwhisk" ]]; then
    log "  OpenWhisk API key was retrieved and injected into the agent automatically."
fi
