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

e2e_runtime_kind() {
    local runtime=${CONTROL_PLANE_RUNTIME:-java}
    runtime=$(echo "${runtime}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
    case "${runtime}" in
        rust)
            echo "rust"
            ;;
        java|"")
            echo "java"
            ;;
        *)
            echo "java"
            ;;
    esac
}

e2e_is_rust_runtime() {
    [[ "$(e2e_runtime_kind)" == "rust" ]]
}

e2e_build_core_jars() {
    e2e_require_vm_exec || return 1
    local remote_dir=${1:-/home/ubuntu/nanofaas}
    local quiet=${2:-true}
    local quiet_flag=""
    local q_remote_dir
    q_remote_dir=$(printf '%q' "${remote_dir}")
    if [[ "${quiet}" == "true" ]]; then
        quiet_flag="-q"
    fi

    e2e_log "Cleaning stale core build outputs..."
    vm_exec "cd ${q_remote_dir} && rm -rf control-plane/build function-runtime/build"

    e2e_log "Building core boot jars..."
    vm_exec "cd ${q_remote_dir} && ./gradlew :control-plane:bootJar :function-runtime:bootJar --no-daemon --rerun-tasks ${quiet_flag}"
    e2e_log "Core boot jars built"
}

e2e_build_core_images() {
    e2e_require_vm_exec || return 1
    local remote_dir=${1:-/home/ubuntu/nanofaas}
    local control_image=${2:?control_image is required}
    local runtime_image=${3:?runtime_image is required}
    local q_remote_dir q_control_image q_runtime_image
    q_remote_dir=$(printf '%q' "${remote_dir}")
    q_control_image=$(printf '%q' "${control_image}")
    q_runtime_image=$(printf '%q' "${runtime_image}")

    e2e_log "Building core images..."
    vm_exec "cd ${q_remote_dir} && sudo docker build -t ${q_control_image} -f control-plane/Dockerfile control-plane/"
    vm_exec "cd ${q_remote_dir} && sudo docker build -t ${q_runtime_image} -f function-runtime/Dockerfile function-runtime/"
    e2e_log "Core images built"
}

# Builds only the function-runtime boot JAR (used when the control-plane is built separately, e.g. Rust).
e2e_build_function_runtime_jar() {
    e2e_require_vm_exec || return 1
    local remote_dir=${1:-/home/ubuntu/nanofaas}
    local quiet=${2:-true}
    local quiet_flag=""
    local q_remote_dir
    q_remote_dir=$(printf '%q' "${remote_dir}")
    if [[ "${quiet}" == "true" ]]; then
        quiet_flag="-q"
    fi

    e2e_log "Cleaning stale function-runtime build output..."
    vm_exec "cd ${q_remote_dir} && rm -rf function-runtime/build"

    e2e_log "Building function-runtime boot jar..."
    vm_exec "cd ${q_remote_dir} && ./gradlew :function-runtime:bootJar --no-daemon --rerun-tasks ${quiet_flag}"
    e2e_log "Function-runtime boot jar built"
}

# Builds the function-runtime Docker image from its Java Dockerfile.
e2e_build_function_runtime_image() {
    e2e_require_vm_exec || return 1
    local remote_dir=${1:-/home/ubuntu/nanofaas}
    local runtime_image=${2:?runtime_image is required}
    local q_remote_dir q_runtime_image
    q_remote_dir=$(printf '%q' "${remote_dir}")
    q_runtime_image=$(printf '%q' "${runtime_image}")

    e2e_log "Building function-runtime image..."
    vm_exec "cd ${q_remote_dir} && sudo docker build -t ${q_runtime_image} -f function-runtime/Dockerfile function-runtime/"
    e2e_log "Function-runtime image built"
}

# Builds the Rust control-plane Docker image on the HOST and transfers it into the VM.
#
# Building on the host avoids two problems that occur when building inside a fresh
# ephemeral Multipass VM:
#   1. No Docker layer cache → every run re-downloads the rust-std musl target (~50 MB)
#      via `rustup target add`, which causes SSH connection drops mid-download.
#   2. The rust:bookworm base image (~1.5 GB) must be pulled from scratch every time.
#
# On the host, Docker layer cache persists across runs so only changed source layers
# are rebuilt.  The final image (~10 MB) is then exported with `docker save`, copied
# into the VM via SCP, and loaded with `docker load`.
#
# CONTROL_PLANE_RUST_DIR (env var) overrides the default path relative to PROJECT_ROOT.
e2e_build_rust_control_plane_image() {
    e2e_require_vm_exec || return 1
    local remote_dir=${1:-/home/ubuntu/nanofaas}    # kept for API compat; unused for host build
    local control_image=${2:?control_image is required}
    local rust_cp_dir="${CONTROL_PLANE_RUST_DIR:-experiments/control-plane-staging/versions/control-plane-rust-m3-20260222-200159/snapshot/control-plane-rust}"
    local host_rust_cp_dir="${PROJECT_ROOT:-$(pwd)}/${rust_cp_dir}"

    if [[ ! -f "${host_rust_cp_dir}/Dockerfile" ]]; then
        e2e_error "Rust control-plane Dockerfile not found at ${host_rust_cp_dir}/Dockerfile"
        return 1
    fi
    if ! command -v docker >/dev/null 2>&1; then
        e2e_error "docker not found on host; install Docker Desktop and retry."
        return 1
    fi

    local host_tag="nanofaas/control-plane-rust:e2e-host"
    local tmp_tar
    tmp_tar=$(mktemp "/tmp/nanofaas-rust-cp.XXXXXX.tar")

    e2e_log "Building Rust control-plane image on host (${rust_cp_dir})..."
    if ! docker build -t "${host_tag}" -f "${host_rust_cp_dir}/Dockerfile" "${host_rust_cp_dir}/"; then
        rm -f "${tmp_tar}"
        e2e_error "Host docker build failed for Rust control-plane"
        return 1
    fi

    e2e_log "Exporting image and transferring to VM..."
    docker save "${host_tag}" > "${tmp_tar}"
    e2e_copy_to_vm "${tmp_tar}" "${VM_NAME}" "/tmp/nanofaas-rust-cp.tar"
    rm -f "${tmp_tar}"
    docker rmi "${host_tag}" >/dev/null 2>&1 || true

    vm_exec "sudo docker load -i /tmp/nanofaas-rust-cp.tar \
        && sudo docker tag ${host_tag} ${control_image} \
        && sudo docker rmi ${host_tag} >/dev/null 2>&1 || true \
        && rm -f /tmp/nanofaas-rust-cp.tar"
    e2e_log "Rust control-plane image built and loaded into VM"
}

e2e_build_control_plane_artifacts() {
    local remote_dir=${1:-/home/ubuntu/nanofaas}
    if e2e_is_rust_runtime; then
        e2e_build_function_runtime_jar "${remote_dir}"
        return
    fi
    e2e_build_core_jars "${remote_dir}"
}

e2e_build_control_plane_image() {
    e2e_require_vm_exec || return 1
    local remote_dir=${1:-/home/ubuntu/nanofaas}
    local control_image=${2:?control_image is required}
    local q_remote_dir q_control_image
    q_remote_dir=$(printf '%q' "${remote_dir}")
    q_control_image=$(printf '%q' "${control_image}")

    if e2e_is_rust_runtime; then
        e2e_build_rust_control_plane_image "${remote_dir}" "${control_image}"
        return
    fi

    e2e_log "Building Java control-plane image..."
    vm_exec "cd ${q_remote_dir} && sudo docker build -t ${q_control_image} -f control-plane/Dockerfile control-plane/"
    e2e_log "Java control-plane image built"
}

e2e_render_control_plane_sync_env() {
    local enabled=${1:-}
    if [[ -z "${enabled}" ]]; then
        return 0
    fi

    cat <<EOF
        - name: SYNC_QUEUE_ENABLED
          value: "${enabled}"
        - name: NANOFAAS_SYNC_QUEUE_ENABLED
          value: "${enabled}"
EOF
}

e2e_create_namespace() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    e2e_log "Creating namespace ${namespace}..."
    vm_exec "kubectl create namespace ${namespace} --dry-run=client -o yaml | kubectl apply -f -"
    e2e_log "Namespace ready"
}

e2e_deploy_control_plane() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local image=${2:?image is required}
    local pod_namespace=${3:-${namespace}}
    local sync_queue_enabled=${4:-}
    local sync_env

    e2e_log "Deploying control-plane in namespace ${namespace}..."
    if [[ -n "${sync_queue_enabled}" ]]; then
        sync_env=$(e2e_render_control_plane_sync_env "${sync_queue_enabled}")
        vm_exec "cat <<'EOF' | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: control-plane
  namespace: ${namespace}
  labels:
    app: control-plane
spec:
  replicas: 1
  selector:
    matchLabels:
      app: control-plane
  template:
    metadata:
      labels:
        app: control-plane
    spec:
      containers:
      - name: control-plane
        image: ${image}
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: api
        - containerPort: 8081
          name: management
        env:
        - name: POD_NAMESPACE
          value: \"${pod_namespace}\"
${sync_env}
        readinessProbe:
          httpGet:
            path: /actuator/health/readiness
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /actuator/health/liveness
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
---
apiVersion: v1
kind: Service
metadata:
  name: control-plane
  namespace: ${namespace}
spec:
  selector:
    app: control-plane
  ports:
  - name: api
    port: 8080
    targetPort: 8080
  - name: management
    port: 8081
    targetPort: 8081
EOF"
    else
        vm_exec "cat <<'EOF' | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: control-plane
  namespace: ${namespace}
  labels:
    app: control-plane
spec:
  replicas: 1
  selector:
    matchLabels:
      app: control-plane
  template:
    metadata:
      labels:
        app: control-plane
    spec:
      containers:
      - name: control-plane
        image: ${image}
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: api
        - containerPort: 8081
          name: management
        env:
        - name: POD_NAMESPACE
          value: \"${pod_namespace}\"
        readinessProbe:
          httpGet:
            path: /actuator/health/readiness
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /actuator/health/liveness
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
---
apiVersion: v1
kind: Service
metadata:
  name: control-plane
  namespace: ${namespace}
spec:
  selector:
    app: control-plane
  ports:
  - name: api
    port: 8080
    targetPort: 8080
  - name: management
    port: 8081
    targetPort: 8081
EOF"
    fi
    # Force a rollout restart so the pod picks up the newly-pushed image
    # even when the deployment spec hasn't changed (same image tag, re-pushed).
    vm_exec "kubectl rollout restart deployment/control-plane -n ${namespace}" || true
    e2e_log "Control-plane deployed"
}

e2e_deploy_function_runtime() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local image=${2:?image is required}

    e2e_log "Deploying function-runtime in namespace ${namespace}..."
    vm_exec "cat <<'EOF' | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: function-runtime
  namespace: ${namespace}
  labels:
    app: function-runtime
spec:
  replicas: 1
  selector:
    matchLabels:
      app: function-runtime
  template:
    metadata:
      labels:
        app: function-runtime
    spec:
      containers:
      - name: function-runtime
        image: ${image}
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
        readinessProbe:
          httpGet:
            path: /actuator/health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: function-runtime
  namespace: ${namespace}
spec:
  selector:
    app: function-runtime
  ports:
  - name: http
    port: 8080
    targetPort: 8080
EOF"
    vm_exec "kubectl rollout restart deployment/function-runtime -n ${namespace}" || true
    e2e_log "Function-runtime deployed"
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

e2e_verify_core_pods_running() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}

    e2e_log "Verifying control-plane/function-runtime pods are running..."
    local cp_running
    cp_running=$(vm_exec "kubectl get pods -n ${namespace} -l app=control-plane --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' '")
    if [[ -z "${cp_running}" || "${cp_running}" -lt 1 ]]; then
        e2e_error "no Running control-plane pod found"
        vm_exec "kubectl describe pod -n ${namespace} -l app=control-plane"
        return 1
    fi

    local fr_running
    fr_running=$(vm_exec "kubectl get pods -n ${namespace} -l app=function-runtime --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' '")
    if [[ -z "${fr_running}" || "${fr_running}" -lt 1 ]]; then
        e2e_error "no Running function-runtime pod found"
        vm_exec "kubectl describe pod -n ${namespace} -l app=function-runtime"
        return 1
    fi
    e2e_log "Core pods running"
}

e2e_verify_control_plane_health() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local service_ip
    service_ip=$(vm_exec "kubectl get svc -n ${namespace} control-plane -o jsonpath='{.spec.clusterIP}'")

    e2e_log "Verifying control-plane health endpoints (service IP: ${service_ip})..."
    vm_exec "curl -sf http://${service_ip}:8081/actuator/health" | grep -q '"status":"UP"'
    vm_exec "curl -sf http://${service_ip}:8081/actuator/health/liveness" | grep -q '"status":"UP"'
    vm_exec "curl -sf http://${service_ip}:8081/actuator/health/readiness" | grep -q '"status":"UP"'
    e2e_log "Control-plane health endpoints are UP"
}

e2e_dump_core_pod_logs() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local tail_lines=${2:-50}

    e2e_log "--- control-plane logs (last ${tail_lines} lines) ---"
    vm_exec "kubectl logs -n ${namespace} -l app=control-plane --tail=${tail_lines}" 2>/dev/null || true
    e2e_log "--- function-runtime logs (last ${tail_lines} lines) ---"
    vm_exec "kubectl logs -n ${namespace} -l app=function-runtime --tail=${tail_lines}" 2>/dev/null || true
}

e2e_register_pool_function() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local name=${2:?function name is required}
    local image=${3:?image is required}
    local endpoint_url=${4:?endpoint_url is required}
    local timeout_ms=${5:-5000}
    local concurrency=${6:-2}
    local queue_size=${7:-20}
    local max_retries=${8:-3}
    local runner_name=${9:-curl-register}

    e2e_log "Registering function '${name}'..."
    local payload
    payload="{\"name\":\"${name}\",\"image\":\"${image}\",\"timeoutMs\":${timeout_ms},\"concurrency\":${concurrency},\"queueSize\":${queue_size},\"maxRetries\":${max_retries},\"executionMode\":\"POOL\",\"endpointUrl\":\"${endpoint_url}\"}"
    e2e_kubectl_curl_control_plane "${namespace}" "${runner_name}" "POST" "/v1/functions" "${payload}" "20" >/dev/null
    e2e_log "Function '${name}' registered"
}

e2e_kubectl_curl_control_plane() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local runner_name=${2:?runner_name is required}
    local method=${3:?method is required}
    local path=${4:?path is required}
    local body_json=${5:-}
    local max_time=${6:-35}

    local service_ip
    service_ip=$(vm_exec "kubectl get svc -n ${namespace} control-plane -o jsonpath='{.spec.clusterIP}'")

    if [[ -n "${body_json}" ]]; then
        local body_b64
        body_b64=$(printf '%s' "${body_json}" | base64 | tr -d '\n')
        vm_exec "echo '${body_b64}' | base64 -d | curl -s --max-time ${max_time} -X ${method} http://${service_ip}:8080${path} -H 'Content-Type: application/json' --data-binary @-"
        return
    fi

    vm_exec "curl -s --max-time ${max_time} -X ${method} http://${service_ip}:8080${path}"
}

e2e_extract_json_by_field() {
    local text=${1:-}
    local field=${2:-}
    python3 -c '
import json
import sys

field = sys.argv[1]
raw = sys.stdin.read()
if not raw.strip():
    sys.exit(0)

decoder = json.JSONDecoder()
obj = None
candidates = [raw.strip()] + [line.strip() for line in raw.splitlines() if line.strip()]
for candidate in candidates:
    try:
        parsed = json.loads(candidate)
        if isinstance(parsed, dict):
            obj = parsed
            break
    except Exception:
        continue
if obj is None:
    for idx, ch in enumerate(raw):
        if ch not in "{[":
            continue
        try:
            parsed, _ = decoder.raw_decode(raw[idx:])
            if isinstance(parsed, dict):
                obj = parsed
                break
        except Exception:
            continue

if isinstance(obj, dict):
    if not field or field in obj:
        print(json.dumps(obj, separators=(",", ":")))
' "${field}" <<< "${text}"
}

e2e_extract_execution_id() {
    local json_line=${1:-}
    python3 -c '
import json
import sys

raw = sys.stdin.read()
if not raw.strip():
    sys.exit(0)
decoder = json.JSONDecoder()
obj = None
try:
    obj = json.loads(raw.strip())
except Exception:
    for idx, ch in enumerate(raw):
        if ch != "{":
            continue
        try:
            parsed, _ = decoder.raw_decode(raw[idx:])
            if isinstance(parsed, dict):
                obj = parsed
                break
        except Exception:
            continue
try:
    if isinstance(obj, dict):
        val = obj.get("executionId")
        if isinstance(val, str):
            print(val)
except Exception:
    pass
' <<< "${json_line}"
}

e2e_extract_execution_status() {
    local json_line=${1:-}
    python3 -c '
import json
import sys

raw = sys.stdin.read()
if not raw.strip():
    sys.exit(0)
decoder = json.JSONDecoder()
obj = None
try:
    obj = json.loads(raw.strip())
except Exception:
    for idx, ch in enumerate(raw):
        if ch != "{":
            continue
        try:
            parsed, _ = decoder.raw_decode(raw[idx:])
            if isinstance(parsed, dict):
                obj = parsed
                break
        except Exception:
            continue
if isinstance(obj, dict):
    val = obj.get("status")
    if isinstance(val, str):
        print(val)
' <<< "${json_line}"
}

e2e_extract_bool_field() {
    local json_line=${1:-}
    local field=${2:?field is required}
    python3 -c '
import json
import sys

field = sys.argv[1]
raw = sys.stdin.read()
if not raw.strip():
    sys.exit(0)
decoder = json.JSONDecoder()
obj = None
try:
    obj = json.loads(raw.strip())
except Exception:
    for idx, ch in enumerate(raw):
        if ch != "{":
            continue
        try:
            parsed, _ = decoder.raw_decode(raw[idx:])
            if isinstance(parsed, dict):
                obj = parsed
                break
        except Exception:
            continue
if isinstance(obj, dict):
    val = obj.get(field)
    if isinstance(val, bool):
        print("true" if val else "false")
' "${field}" <<< "${json_line}"
}

e2e_extract_numeric_field() {
    local json_line=${1:-}
    local field=${2:?field is required}
    python3 -c '
import json
import sys

field = sys.argv[1]
raw = sys.stdin.read()
if not raw.strip():
    sys.exit(0)
decoder = json.JSONDecoder()
obj = None
try:
    obj = json.loads(raw.strip())
except Exception:
    for idx, ch in enumerate(raw):
        if ch != "{":
            continue
        try:
            parsed, _ = decoder.raw_decode(raw[idx:])
            if isinstance(parsed, dict):
                obj = parsed
                break
        except Exception:
            continue
if isinstance(obj, dict):
    val = obj.get(field)
    if isinstance(val, (int, float)):
        print(val)
' "${field}" <<< "${json_line}"
}

e2e_invoke_sync_message() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local function_name=${2:?function_name is required}
    local message=${3:?message is required}
    local runner_name=${4:-curl-invoke}

    local raw
    raw=$(e2e_kubectl_curl_control_plane \
        "${namespace}" \
        "${runner_name}" \
        "POST" \
        "/v1/functions/${function_name}:invoke" \
        "{\"input\": {\"message\": \"${message}\"}}")
    e2e_extract_json_by_field "${raw}"
}

e2e_enqueue_message() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local function_name=${2:?function_name is required}
    local message=${3:?message is required}
    local runner_name=${4:-curl-enqueue}

    local raw
    raw=$(e2e_kubectl_curl_control_plane \
        "${namespace}" \
        "${runner_name}" \
        "POST" \
        "/v1/functions/${function_name}:enqueue" \
        "{\"input\": {\"message\": \"${message}\"}}")
    e2e_extract_json_by_field "${raw}" "executionId"
}

e2e_fetch_execution() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local execution_id=${2:?execution_id is required}
    local runner_name=${3:-curl-status}

    local raw
    raw=$(e2e_kubectl_curl_control_plane \
        "${namespace}" \
        "${runner_name}" \
        "GET" \
        "/v1/executions/${execution_id}" \
        "" \
        "10")
    e2e_extract_json_by_field "${raw}" "executionId"
}

e2e_wait_execution_success() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local execution_id=${2:?execution_id is required}
    local attempts=${3:-20}
    local sleep_seconds=${4:-1}
    local runner_prefix=${5:-curl-poll}

    local i
    for i in $(seq 1 "${attempts}"); do
        local json status
        json=$(e2e_fetch_execution "${namespace}" "${execution_id}" "${runner_prefix}-${i}" || true)
        status=$(e2e_extract_execution_status "${json}")
        if [[ "${status}" == "success" ]]; then
            return 0
        fi
        if [[ "${status}" == "failed" || "${status}" == "error" || "${status}" == "timeout" ]]; then
            return 1
        fi
        sleep "${sleep_seconds}"
    done
    return 1
}

e2e_enqueue_message_burst() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local function_name=${2:?function_name is required}
    local message_prefix=${3:?message_prefix is required}
    local count=${4:?count is required}
    local runner_prefix=${5:-curl-queue}

    local i
    for i in $(seq 1 "${count}"); do
        e2e_enqueue_message \
            "${namespace}" \
            "${function_name}" \
            "${message_prefix}-${i}" \
            "${runner_prefix}-${i}" >/dev/null
    done
}

e2e_get_control_plane_pod_name() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    vm_exec "kubectl get pods -n ${namespace} -l app=control-plane -o jsonpath='{.items[0].metadata.name}'"
}

e2e_fetch_control_plane_prometheus() {
    e2e_require_vm_exec || return 1
    local namespace=${1:?namespace is required}
    local service_ip
    service_ip=$(vm_exec "kubectl get svc -n ${namespace} control-plane -o jsonpath='{.spec.clusterIP}'")
    vm_exec "curl -sf http://${service_ip}:8081/actuator/prometheus"
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
    local container_name=${2:-nanofaas-e2e-registry}

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
