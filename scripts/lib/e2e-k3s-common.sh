#!/usr/bin/env bash

# Shared helpers for k3s-based E2E scripts.
# Source this file and then call e2e_set_log_prefix to set your script's prefix.

# ─── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ─── Logging with configurable prefix ────────────────────────────────────────
E2E_LOG_PREFIX="e2e"
e2e_set_log_prefix() { E2E_LOG_PREFIX="$1"; }

log()   { echo -e "${GREEN}[${E2E_LOG_PREFIX}]${NC} $*"; }
warn()  { echo -e "${YELLOW}[${E2E_LOG_PREFIX}]${NC} $*"; }
info()  { echo -e "${CYAN}[${E2E_LOG_PREFIX}]${NC} $*"; }
error() { echo -e "${RED}[${E2E_LOG_PREFIX}]${NC} $*" >&2; }
err()   { error "$@"; }

# Backward-compat aliases
e2e_log() { log "$@"; }
e2e_error() { error "$@"; }
e2e_warn_stderr() { echo -e "${YELLOW}[${E2E_LOG_PREFIX}]${NC} $*" >&2; }

# ─── Temp file helpers ───────────────────────────────────────────────────────
e2e_mktemp_file() {
    local prefix=${1:?prefix is required}
    local suffix=${2:-}
    local tmp

    tmp=$(mktemp "/tmp/${prefix}.XXXXXX")
    if [[ -n "${suffix}" ]]; then
        local with_suffix="${tmp}${suffix}"
        mv "${tmp}" "${with_suffix}"
        tmp="${with_suffix}"
    fi
    echo "${tmp}"
}

# ─── Multipass purge policy ──────────────────────────────────────────────────
# MULTIPASS_PURGE:
#   - always|true|1|yes  -> always run multipass purge
#   - never|false|0|no   -> never run multipass purge
#   - auto (default)     -> run purge only in CI
e2e_should_purge() {
    local mode=${MULTIPASS_PURGE:-auto}
    mode=$(echo "${mode}" | tr '[:upper:]' '[:lower:]')

    case "${mode}" in
        always|true|1|yes)
            return 0
            ;;
        never|false|0|no)
            return 1
            ;;
        auto|"")
            [[ "${CI:-}" == "true" || "${CI:-}" == "1" ]]
            return
            ;;
        *)
            warn "Unknown MULTIPASS_PURGE='${MULTIPASS_PURGE}'. Using 'auto'."
            [[ "${CI:-}" == "true" || "${CI:-}" == "1" ]]
            return
            ;;
    esac
}

# ─── VM transport backend ────────────────────────────────────────────────────
# E2E_VM_BACKEND:
#   - ssh  -> use ssh/scp for exec and copy
#   - auto -> alias of ssh (default)
#   - multipass -> legacy alias, coerced to ssh for portability
e2e_get_vm_backend() {
    local backend=${E2E_VM_BACKEND:-auto}
    backend=$(echo "${backend}" | tr '[:upper:]' '[:lower:]')
    case "${backend}" in
        ssh|auto|"")
            if command -v ssh >/dev/null 2>&1; then
                echo "ssh"
            else
                e2e_error "ssh not found. Install OpenSSH client."
                return 1
            fi
            ;;
        multipass)
            e2e_warn_stderr "E2E_VM_BACKEND=multipass is deprecated for command transport; using ssh."
            if command -v ssh >/dev/null 2>&1; then
                echo "ssh"
            else
                e2e_error "ssh not found. Install OpenSSH client."
                return 1
            fi
            ;;
        *)
            warn "Unknown E2E_VM_BACKEND='${E2E_VM_BACKEND}'. Using 'auto'."
            if command -v ssh >/dev/null 2>&1; then
                echo "ssh"
            else
                e2e_error "ssh not found. Install OpenSSH client."
                return 1
            fi
            ;;
    esac
}

e2e_detect_multipass_ssh_key() {
    local candidates=(
        "${HOME}/Library/Application Support/multipassd/ssh-keys/id_rsa"
        "/var/snap/multipass/common/data/multipassd/ssh-keys/id_rsa"
        "${HOME}/snap/multipass/common/data/multipassd/ssh-keys/id_rsa"
    )
    local key
    for key in "${candidates[@]}"; do
        if [[ -f "${key}" ]]; then
            echo "${key}"
            return 0
        fi
    done
    return 1
}

e2e_get_ssh_identity_opt() {
    local key=${E2E_SSH_KEY:-}
    if [[ -z "${key}" ]]; then
        key=$(e2e_detect_multipass_ssh_key || true)
    fi
    if [[ -n "${key}" ]]; then
        printf -- "-i %q" "${key}"
    fi
}

e2e_get_host_pubkey() {
    local key
    for key in "${HOME}/.ssh/id_ed25519.pub" "${HOME}/.ssh/id_rsa.pub"; do
        if [[ -f "${key}" ]]; then
            cat "${key}"
            return 0
        fi
    done
    return 1
}

e2e_ssh_exec() {
    local vm_ip=${1:?vm_ip is required}
    local remote_cmd=${2:?remote_cmd is required}
    local user=${E2E_VM_USER:-ubuntu}
    local connect_timeout=${E2E_SSH_CONNECT_TIMEOUT:-15}
    # Commands like gradle test/image build can legitimately run for minutes.
    local cmd_timeout=${E2E_SSH_CMD_TIMEOUT:-900}
    local retries=${E2E_SSH_RETRIES:-6}
    local retry_delay=${E2E_SSH_RETRY_DELAY_SECONDS:-2}
    local identity_opt
    local attempt rc
    identity_opt=$(e2e_get_ssh_identity_opt || true)

    for ((attempt = 1; attempt <= retries; attempt++)); do
        if [[ "${cmd_timeout}" =~ ^[0-9]+$ ]] && (( cmd_timeout > 0 )) && command -v perl >/dev/null 2>&1; then
            # shellcheck disable=SC2086
            perl -e 'alarm shift @ARGV; exec @ARGV' "${cmd_timeout}" \
                ssh -n ${identity_opt} \
                -o BatchMode=yes \
                -o StrictHostKeyChecking=no \
                -o UserKnownHostsFile=/dev/null \
                -o LogLevel=ERROR \
                -o ConnectTimeout="${connect_timeout}" \
                "${user}@${vm_ip}" "${remote_cmd}"
            rc=$?
        else
            # shellcheck disable=SC2086
            ssh -n ${identity_opt} \
                -o BatchMode=yes \
                -o StrictHostKeyChecking=no \
                -o UserKnownHostsFile=/dev/null \
                -o LogLevel=ERROR \
                -o ConnectTimeout="${connect_timeout}" \
                "${user}@${vm_ip}" "${remote_cmd}"
            rc=$?
        fi

        if [[ "${rc}" -eq 0 ]]; then
            return 0
        fi

        # Retry only transport-level failures (e.g. VM still booting SSH).
        if [[ "${rc}" -eq 255 && "${attempt}" -lt "${retries}" ]]; then
            sleep "${retry_delay}"
            continue
        fi
        return "${rc}"
    done

    return 1
}

e2e_scp_to_vm() {
    local src=${1:?src is required}
    local vm_ip=${2:?vm_ip is required}
    local dest=${3:?dest is required}
    local user=${E2E_VM_USER:-ubuntu}
    local connect_timeout=${E2E_SSH_CONNECT_TIMEOUT:-15}
    local identity_opt
    identity_opt=$(e2e_get_ssh_identity_opt || true)

    # shellcheck disable=SC2086
    scp ${identity_opt} \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o LogLevel=ERROR \
        -o ConnectTimeout="${connect_timeout}" \
        "${src}" "${user}@${vm_ip}:${dest}"
}

e2e_scp_from_vm() {
    local vm_ip=${1:?vm_ip is required}
    local src=${2:?src is required}
    local dest=${3:?dest is required}
    local user=${E2E_VM_USER:-ubuntu}
    local connect_timeout=${E2E_SSH_CONNECT_TIMEOUT:-15}
    local identity_opt
    identity_opt=$(e2e_get_ssh_identity_opt || true)

    # shellcheck disable=SC2086
    scp ${identity_opt} \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o LogLevel=ERROR \
        -o ConnectTimeout="${connect_timeout}" \
        "${user}@${vm_ip}:${src}" "${dest}"
}

# ─── Standard vm_exec ────────────────────────────────────────────────────────
e2e_vm_exec() {
    local backend
    backend=$(e2e_get_vm_backend) || return 1
    local cmd_string="export KUBECONFIG=/home/ubuntu/.kube/config; $*"

    if [[ "${backend}" != "ssh" ]]; then
        e2e_error "Unsupported vm backend '${backend}'"
        return 1
    fi

    local vm_ip quoted
    vm_ip=$(e2e_get_vm_ip) || true
    if [[ -z "${vm_ip}" ]]; then
        e2e_error "Cannot determine VM IP for '${VM_NAME}'"
        return 1
    fi
    quoted=$(printf '%q' "${cmd_string}")
    if e2e_ssh_exec "${vm_ip}" "bash -lc ${quoted}"; then
        return
    fi
    e2e_error "SSH exec failed for VM '${VM_NAME}'"
    return 1
}

# ─── VM IP resolution ────────────────────────────────────────────────────────
e2e_get_vm_ip() {
    if [[ -n "${VM_IP:-}" ]]; then
        echo "${VM_IP}"
        return 0
    fi
    multipass info "${VM_NAME}" --format csv 2>/dev/null | tail -1 | cut -d, -f3
}

# ─── VM auto-detect ──────────────────────────────────────────────────────────
e2e_auto_detect_vm() {
    local default_name=${1:-e2e-vm}
    if [[ -n "${VM_NAME:-}" ]]; then echo "${VM_NAME}"; return; fi
    if command -v multipass &>/dev/null; then
        local detected
        detected=$(multipass list --format csv 2>/dev/null | tail -n +2 | grep "Running" | head -1 | cut -d, -f1) || true
        if [[ -n "${detected}" ]]; then echo "${detected}"; return; fi
    fi
    echo "${default_name}"
}

# ─── Test counters ───────────────────────────────────────────────────────────
E2E_PASS=0
E2E_FAIL=0
E2E_TESTS_RUN=()

e2e_test_init() { E2E_PASS=0; E2E_FAIL=0; E2E_TESTS_RUN=(); }
e2e_pass() { ((E2E_PASS++)); E2E_TESTS_RUN+=("[PASS] $1"); log "  PASS: $*"; }
e2e_fail() { ((E2E_FAIL++)); E2E_TESTS_RUN+=("[FAIL] $1"); error "  FAIL: $*"; }

# ─── Deployment replica queries ──────────────────────────────────────────────
e2e_get_ready_replicas() {
    local ns=${1:?} deploy=${2:?}
    local val
    val=$(e2e_vm_exec "kubectl get deployment ${deploy} -n ${ns} -o jsonpath='{.status.readyReplicas}' 2>/dev/null") || true
    [[ -z "${val}" || "${val}" == "null" ]] && echo "0" || echo "${val}"
}

e2e_get_desired_replicas() {
    local ns=${1:?} deploy=${2:?}
    local val
    val=$(e2e_vm_exec "kubectl get deployment ${deploy} -n ${ns} -o jsonpath='{.spec.replicas}' 2>/dev/null") || true
    [[ -z "${val}" || "${val}" == "null" ]] && echo "0" || echo "${val}"
}

# ─── Cleanup helper ──────────────────────────────────────────────────────────
e2e_cleanup_vm() {
    if [[ "${KEEP_VM:-false}" == "true" ]]; then
        warn "KEEP_VM=true — VM '${VM_NAME}' preserved"
        local vm_ip
        vm_ip=$(e2e_get_vm_ip || true)
        if [[ -n "${vm_ip}" ]]; then
            warn "  SSH:    ssh ${E2E_VM_USER:-ubuntu}@${vm_ip}"
        else
            warn "  Shell:  multipass shell ${VM_NAME}"
        fi
        warn "  Delete: multipass delete ${VM_NAME}"
        return
    fi
    log "Cleaning up VM ${VM_NAME}..."
    multipass delete "${VM_NAME}" 2>/dev/null || true
    if e2e_should_purge; then
        info "Running multipass purge (MULTIPASS_PURGE=${MULTIPASS_PURGE:-auto})"
        multipass purge 2>/dev/null || true
    else
        info "Skipping multipass purge (MULTIPASS_PURGE=${MULTIPASS_PURGE:-auto})"
    fi
}

e2e_require_multipass() {
    if ! command -v multipass >/dev/null 2>&1; then
        e2e_error "multipass not found. Install: brew install multipass"
        return 1
    fi
}

e2e_require_vm_exec() {
    if ! declare -F vm_exec >/dev/null 2>&1; then
        e2e_error "vm_exec function not defined by caller"
        return 1
    fi
}

e2e_vm_has_command() {
    local cmd=${1:?command name is required}
    e2e_vm_exec "if command -v ${cmd} >/dev/null 2>&1; then echo yes; else echo no; fi"
}

e2e_create_vm() {
    local vm_name=${1:?vm_name is required}
    local cpus=${2:-4}
    local memory=${3:-8G}
    local disk=${4:-30G}

    if multipass list 2>/dev/null | awk '{print $1}' | grep -q "^${vm_name}$"; then
        e2e_log "VM ${vm_name} already exists, ensuring it is running..."
        multipass start "${vm_name}" >/dev/null 2>&1 || true
        return 0
    fi

    e2e_log "Creating VM ${vm_name} (cpus=${cpus}, memory=${memory}, disk=${disk})..."
    local host_pubkey cloud_init
    host_pubkey=$(e2e_get_host_pubkey || true)
    if [[ -n "${host_pubkey}" ]]; then
        cloud_init=$(e2e_mktemp_file "e2e-cloud-init" ".yaml")
        cat > "${cloud_init}" <<EOF
#cloud-config
ssh_authorized_keys:
  - ${host_pubkey}
EOF
        if ! multipass launch --name "${vm_name}" --cpus "${cpus}" --memory "${memory}" --disk "${disk}" --cloud-init "${cloud_init}"; then
            rm -f "${cloud_init}"
            return 1
        fi
        rm -f "${cloud_init}"
    else
        e2e_warn_stderr "No host SSH public key found (~/.ssh/id_ed25519.pub or id_rsa.pub); SSH transport may fail."
        multipass launch --name "${vm_name}" --cpus "${cpus}" --memory "${memory}" --disk "${disk}"
    fi
    e2e_log "VM created successfully"
}

e2e_ensure_vm_running() {
    local vm_name=${1:?vm_name is required}
    local cpus=${2:-4}
    local memory=${3:-8G}
    local disk=${4:-30G}

    if multipass info "${vm_name}" &>/dev/null; then
        local state
        state=$(multipass info "${vm_name}" --format csv | tail -1 | cut -d, -f2)
        if [[ "${state}" == "Running" ]]; then
            e2e_log "VM '${vm_name}' already running, reusing..."
            return 0
        fi
        if [[ "${state}" == "Deleted" ]]; then
            e2e_warn_stderr "VM '${vm_name}' is in state 'Deleted'."
            e2e_log "Attempting to recover VM '${vm_name}'..."
            if multipass recover "${vm_name}" >/dev/null 2>&1; then
                e2e_log "Recover succeeded for '${vm_name}', starting..."
                multipass start "${vm_name}"
                return 0
            fi
            e2e_warn_stderr "Recover failed for '${vm_name}', purging deleted instances and recreating VM..."
            multipass purge >/dev/null 2>&1 || true
            e2e_create_vm "${vm_name}" "${cpus}" "${memory}" "${disk}"
            return 0
        fi
        e2e_log "VM '${vm_name}' exists with state '${state}', starting..."
        multipass start "${vm_name}"
        return 0
    fi

    e2e_create_vm "${vm_name}" "${cpus}" "${memory}" "${disk}"
}

e2e_install_vm_dependencies() {
    e2e_require_vm_exec || return 1
    local install_helm=${1:-false}
    local helm_version=${HELM_VERSION:-3.16.4}
    helm_version=${helm_version#v}
    local helm_label=""
    if [[ "${install_helm}" == "true" ]]; then
        helm_label=", helm"
    fi

    e2e_log "Installing VM dependencies (docker, jdk21${helm_label})..."
    vm_exec "sudo apt-get update -y"
    vm_exec "sudo DEBIAN_FRONTEND=noninteractive apt-get install -y curl ca-certificates tar unzip openjdk-21-jdk-headless docker.io"
    vm_exec "sudo systemctl enable --now docker"
    vm_exec "sudo usermod -aG docker ubuntu || true"

    if [[ "${install_helm}" == "true" ]]; then
        vm_exec "if ! command -v helm >/dev/null 2>&1; then
            arch=\$(uname -m)
            case \"\${arch}\" in
                x86_64|amd64) helm_arch=amd64 ;;
                aarch64|arm64) helm_arch=arm64 ;;
                *) echo \"Unsupported architecture for helm: \${arch}\" >&2; exit 1 ;;
            esac
            archive=/tmp/helm-v${helm_version}-linux-\${helm_arch}.tar.gz
            curl -fsSL -o \"\${archive}\" \"https://get.helm.sh/helm-v${helm_version}-linux-\${helm_arch}.tar.gz\"
            tar -xzf \"\${archive}\" -C /tmp
            sudo install -m 0755 /tmp/linux-\${helm_arch}/helm /usr/local/bin/helm
            rm -rf /tmp/linux-\${helm_arch} \"\${archive}\"
        fi"
    fi
    e2e_log "VM dependencies installed"
}

e2e_copy_to_vm() {
    local src=${1:?src is required}
    local vm_name=${2:?vm_name is required}
    local dest=${3:?dest is required}
    local backend
    backend=$(e2e_get_vm_backend) || return 1
    if [[ "${backend}" != "ssh" ]]; then
        e2e_error "Unsupported vm backend '${backend}'"
        return 1
    fi

    local vm_ip
    vm_ip=$(e2e_get_vm_ip) || true
    if [[ -z "${vm_ip}" ]]; then
        e2e_error "Cannot determine VM IP for '${vm_name}'"
        return 1
    fi
    if e2e_scp_to_vm "${src}" "${vm_ip}" "${dest}"; then
        return
    fi
    e2e_error "SCP upload failed for VM '${vm_name}'"
    return 1
}

e2e_copy_from_vm() {
    local vm_name=${1:?vm_name is required}
    local src=${2:?src is required}
    local dest=${3:?dest is required}
    local backend
    backend=$(e2e_get_vm_backend) || return 1
    if [[ "${backend}" != "ssh" ]]; then
        e2e_error "Unsupported vm backend '${backend}'"
        return 1
    fi

    local vm_ip
    vm_ip=$(e2e_get_vm_ip) || true
    if [[ -z "${vm_ip}" ]]; then
        e2e_error "Cannot determine VM IP for '${vm_name}'"
        return 1
    fi
    if e2e_scp_from_vm "${vm_ip}" "${src}" "${dest}"; then
        return
    fi
    e2e_error "SCP download failed for VM '${vm_name}'"
    return 1
}

e2e_sync_project_to_vm() {
    local project_root=${1:?project_root is required}
    local vm_name=${2:?vm_name is required}
    local remote_dir=${3:-/home/ubuntu/project}
    local sync_tar=/tmp/e2e-sync.tar

    e2e_log "Syncing project to VM (${remote_dir})..."
    local tmp_tar
    local -a tar_flags=()
    if tar --help 2>&1 | grep -q -- '--disable-copyfile'; then tar_flags+=(--disable-copyfile); fi
    if tar --help 2>&1 | grep -q -- '--no-mac-metadata'; then tar_flags+=(--no-mac-metadata); fi
    if tar --help 2>&1 | grep -q -- '--no-xattrs'; then tar_flags+=(--no-xattrs); fi
    if tar --help 2>&1 | grep -q -- '--no-acls'; then tar_flags+=(--no-acls); fi
    tmp_tar=$(e2e_mktemp_file "e2e-sync" ".tar")
    if [[ ${#tar_flags[@]} -gt 0 ]]; then
        COPYFILE_DISABLE=1 tar "${tar_flags[@]}" -C "${project_root}" \
            --exclude='.git' \
            --exclude='.gradle' \
            --exclude='.idea' \
            --exclude='.worktrees' \
            --exclude='.DS_Store' \
            --exclude='build' \
            --exclude='*/build' \
            --exclude='target' \
            --exclude='*/target' \
            -cf "${tmp_tar}" .
    else
        COPYFILE_DISABLE=1 tar -C "${project_root}" \
            --exclude='.git' \
            --exclude='.gradle' \
            --exclude='.idea' \
            --exclude='.worktrees' \
            --exclude='.DS_Store' \
            --exclude='build' \
            --exclude='*/build' \
            --exclude='target' \
            --exclude='*/target' \
            -cf "${tmp_tar}" .
    fi

    if declare -F vm_exec >/dev/null 2>&1; then
        vm_exec "rm -rf ${remote_dir} && mkdir -p ${remote_dir}"
    else
        e2e_vm_exec "rm -rf ${remote_dir} && mkdir -p ${remote_dir}"
    fi
    e2e_copy_to_vm "${tmp_tar}" "${vm_name}" "${sync_tar}"
    local extract_sync_cmd
    extract_sync_cmd="if tar --help 2>&1 | grep -q -- '--warning'; then tar --warning=no-unknown-keyword -xf ${sync_tar} -C ${remote_dir}; else tar -xf ${sync_tar} -C ${remote_dir}; fi && rm -f ${sync_tar}"
    if declare -F vm_exec >/dev/null 2>&1; then
        vm_exec "${extract_sync_cmd}"
    else
        e2e_vm_exec "${extract_sync_cmd}"
    fi
    rm -f "${tmp_tar}"
    e2e_log "Project synced"
}

e2e_create_namespace() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    e2e_log "Creating namespace ${namespace}..."
    vm_exec "kubectl create namespace ${namespace} --dry-run=client -o yaml | kubectl apply -f -"
    e2e_log "Namespace ready"
}

e2e_wait_for_deployment() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local name=${2:?deployment name is required}
    local timeout=${3:-180}

    e2e_log "Waiting for deployment ${name} to be ready (timeout: ${timeout}s)..."
    vm_exec "kubectl rollout status deployment/${name} -n ${namespace} --timeout=${timeout}s"
    e2e_log "Deployment ${name} is ready"
}

e2e_install_k3s() {
    e2e_require_vm_exec || return 1
    local k3s_version=${K3S_VERSION:-v1.32.2+k3s1}

    e2e_log "Installing k3s (${k3s_version})..."
    vm_exec "curl -sfL https://get.k3s.io | sudo INSTALL_K3S_VERSION='${k3s_version}' sh -s - --disable traefik"

    vm_exec "for i in \$(seq 1 60); do
        if sudo k3s kubectl get nodes --no-headers 2>/dev/null | grep -q ' Ready'; then
            echo 'k3s node ready'
            exit 0
        fi
        sleep 2
    done
    echo 'k3s node not ready after 120s' >&2
    exit 1"

    vm_exec "mkdir -p /home/ubuntu/.kube"
    vm_exec "sudo cp /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config"
    vm_exec "sudo chown ubuntu:ubuntu /home/ubuntu/.kube/config"
    vm_exec "chmod 600 /home/ubuntu/.kube/config"

    e2e_log "k3s installed and ready"
}

e2e_setup_local_registry() {
    e2e_require_vm_exec || return 1

    local registry=${1:-localhost:5000}
    local host=${registry%:*}
    local port=${registry##*:}
    local container_name=${2:-e2e-registry}

    if [[ -z "${host}" || -z "${port}" || "${host}" == "${registry}" ]]; then
        e2e_error "Registry must include host:port (got '${registry}')"
        return 1
    fi

    e2e_log "Starting local registry ${registry}..."
    vm_exec "sudo docker rm -f ${container_name} >/dev/null 2>&1 || true"
    vm_exec "sudo docker run -d --restart unless-stopped --name ${container_name} -p ${port}:5000 registry:2 >/dev/null"

    vm_exec "for i in \$(seq 1 30); do
        if curl -fsS http://${registry}/v2/ >/dev/null 2>&1; then
            echo 'registry ready'
            exit 0
        fi
        sleep 1
    done
    echo 'registry not ready after 30s' >&2
    exit 1"

    e2e_log "Configuring k3s to pull from ${registry}..."
    vm_exec "cat <<EOF | sudo tee /etc/rancher/k3s/registries.yaml >/dev/null
mirrors:
  \"${registry}\":
    endpoint:
      - \"http://${registry}\"
configs:
  \"${registry}\":
    tls:
      insecure_skip_verify: true
EOF"
    vm_exec "sudo systemctl restart k3s"
    vm_exec "for i in \$(seq 1 60); do
        if sudo k3s kubectl get nodes --no-headers 2>/dev/null | grep -q ' Ready'; then
            echo 'k3s node ready'
            exit 0
        fi
        sleep 2
    done
    echo 'k3s node not ready after k3s restart' >&2
    exit 1"

    e2e_log "Local registry configured for k3s"
}

e2e_push_images_to_registry() {
    e2e_require_vm_exec || return 1
    if [[ "$#" -eq 0 ]]; then
        e2e_error "e2e_push_images_to_registry requires at least one image"
        return 1
    fi

    local img
    for img in "$@"; do
        vm_exec "sudo docker push ${img}"
    done
    e2e_log "Pushed images to registry"
}

e2e_cleanup_host_resources() {
    local pid=${1:-} container=${2:-} tmpdir=${3:-}
    if [[ -n "${pid}" ]]; then kill "${pid}" >/dev/null 2>&1 || true; wait "${pid}" 2>/dev/null || true; fi
    if [[ -n "${container}" ]]; then docker rm -f "${container}" >/dev/null 2>&1 || true; fi
    if [[ -n "${tmpdir}" && -d "${tmpdir}" ]]; then rm -rf "${tmpdir}" || true; fi
}

e2e_import_images_to_k3s() {
    e2e_require_vm_exec || return 1
    if [[ "$#" -eq 0 ]]; then
        e2e_error "e2e_import_images_to_k3s requires at least one image"
        return 1
    fi

    e2e_log "Importing images to k3s..."
    local img
    for img in "$@"; do
        local tarname
        tarname=$(echo "${img}" | tr '/:' '_')
        vm_exec "sudo docker save ${img} -o /tmp/${tarname}.tar"
        vm_exec "sudo k3s ctr images import /tmp/${tarname}.tar"
        vm_exec "sudo rm -f /tmp/${tarname}.tar"
    done
    e2e_log "Images imported to k3s"
}
