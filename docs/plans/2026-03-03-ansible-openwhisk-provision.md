# Ansible OpenWhisk Support + Provision Script Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add OpenWhisk support to the DFaaS Ansible playbook via a `faas_platform` variable, and create a `provision.sh` script that automates VM creation and playbook execution.

**Architecture:** Single playbook with `faas_platform: openfaas|openwhisk` variable drives which FaaS stack is installed. `provision.sh` sources `scripts/lib/e2e-k3s-common.sh`, creates a Multipass VM, syncs the project, generates a temporary Ansible inventory, and runs the playbook. A new agent values overlay `scripts/ansible/charts/values-agent-openwhisk.yaml` captures the OpenWhisk-specific agent config.

**Tech Stack:** Bash, Ansible (YAML playbook), Helm, Multipass

---

### Task 1: Create `scripts/ansible/charts/values-agent-openwhisk.yaml`

**Files:**
- Create: `scripts/ansible/charts/values-agent-openwhisk.yaml`

**Step 1: Create directory and file**

```bash
mkdir -p /path/to/repo/scripts/ansible/charts
```

Then create `scripts/ansible/charts/values-agent-openwhisk.yaml` with this exact content:

```yaml
# Agent Helm values overlay for OpenWhisk platform.
#
# Pass alongside your base values when installing the agent chart:
#   helm install dfaas-agent ./k8s/charts/agent \
#     --values my-values.yaml \
#     --values scripts/ansible/charts/values-agent-openwhisk.yaml
#
# AGENT_OPENWHISK_API_KEY must be filled in after OpenWhisk is deployed.
# Retrieve it with:
#   kubectl -n openwhisk exec deploy/owdev-wskadmin -- wskadmin user get guest

config:
  AGENT_FAAS_PLATFORM: "openwhisk"
  AGENT_FAAS_HOST: "owdev-nginx.openwhisk"
  AGENT_FAAS_PORT: 80
  AGENT_OPENWHISK_NAMESPACE: "guest"
  AGENT_OPENWHISK_API_KEY: ""  # Set after retrieving via wskadmin
```

**Step 2: Verify file exists**

```bash
cat scripts/ansible/charts/values-agent-openwhisk.yaml
```

Expected: file content printed, no error.

**Step 3: Commit**

```bash
git add scripts/ansible/charts/values-agent-openwhisk.yaml
git commit -m "feat: add agent values overlay for OpenWhisk platform"
```

---

### Task 2: Rewrite `scripts/ansible/setup_playbook.yaml`

**Files:**
- Modify: `scripts/ansible/setup_playbook.yaml`

**Context:**
- The existing playbook installs OpenFaaS only, references `{{ playbook_dir }}` for chdir (which breaks when run remotely).
- We add `faas_platform` (default `openfaas`) and `project_dir` (default `{{ playbook_dir }}`) variables.
- We split the Helm repos block and Helm charts block to allow per-platform `when:` conditions.
- We add OpenWhisk-specific tasks (repo, install, API key retrieval).
- We guard OpenFaaS-specific tasks (faas-cli, Configure OpenFaaS block) with `when: faas_platform == "openfaas"`.
- We fix a pre-existing YAML indentation bug in the last task (`args:` was incorrectly indented under `command:`).
- All `chdir: "{{ playbook_dir }}"` become `chdir: "{{ project_dir }}"`.

**Step 1: Replace the entire file** with the following content:

```yaml
---
- name: DFaaS node setup from scratch
  hosts: all
  become: true
  vars:
    # FaaS platform to install. Accepted values: "openfaas" (default), "openwhisk".
    faas_platform: "openfaas"
    openfaas_url: "http://127.0.0.1:31112"
    user_home: "{{ ansible_env.HOME }}"
    # Root directory of the DFaaS project on the target host.
    # When running via provision.sh this is set to /home/ubuntu/dfaas.
    # When running locally (hosts: localhost) leave as-is.
    project_dir: "{{ playbook_dir }}"

  tasks:
    - name: Install buildah
      ansible.builtin.apt:
        name: buildah
        state: present
        update_cache: yes

    - name: Disable UFW
      community.general.ufw:
        state: disabled

    - name: Increase number of allowed file descriptors and processes (for HAProxy)
      ansible.builtin.blockinfile:
        path: /etc/security/limits.conf
        marker: "# {mark} ANSIBLE MANAGED BLOCK: unlimited nofile/nproc"
        block: |
          * soft nofile unlimited
          * hard nofile unlimited
          * soft nproc unlimited
          * hard nproc unlimited

    - name: Install K3S
      ansible.builtin.shell: curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -
      args:
        executable: /bin/bash
        creates: /usr/local/bin/k3s

    - name: Ensure k3s service is running
      ansible.builtin.systemd_service:
        name: k3s
        state: started
        enabled: yes

    - name: Build DFaaS agent and forecaster images and push to K3S
      block:
        - name: Build agent image
          ansible.builtin.command: ./k8s/scripts/build-image.sh agent
          args:
            chdir: "{{ project_dir }}"

        - name: Build forecaster image
          ansible.builtin.command: ./k8s/scripts/build-image.sh forecaster
          args:
            chdir: "{{ project_dir }}"
      become: false

    - name: Add Helm apt repository
      ansible.builtin.deb822_repository:
        name: helm-stable-debian
        uris: https://packages.buildkite.com/helm-linux/helm-debian/any/
        suites: any
        components: [main]
        architectures: [amd64]
        signed_by: https://packages.buildkite.com/helm-linux/helm-debian/gpgkey

    - name: Install Helm
      ansible.builtin.apt:
        name: helm
        state: present
        update_cache: yes

    - name: Add KUBECONFIG to .bashrc
      ansible.builtin.lineinfile:
        path: "{{ user_home }}/.bashrc"
        line: 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml'
        state: present
      become: false

    - name: Configure sudo to preserve KUBECONFIG
      ansible.builtin.copy:
        content: |
          # Preserve KUBECONFIG env variable.
          Defaults:%sudo env_keep += "KUBECONFIG"
          Defaults !always_set_home
        dest: /etc/sudoers.d/50-helm
        mode: '0440'

    - name: Install faas-cli
      block:
        - name: Create ~/.local/bin directory
          ansible.builtin.file:
            path: "{{ user_home }}/.local/bin"
            state: directory
            mode: '0755'

        - name: Download faas-cli
          ansible.builtin.get_url:
            url: https://github.com/openfaas/faas-cli/releases/download/0.17.8/faas-cli
            dest: "{{ user_home }}/.local/bin/faas-cli"
            mode: '0755'

        - name: Add ~/.local/bin to PATH in .bashrc
          ansible.builtin.lineinfile:
            path: "{{ user_home }}/.bashrc"
            line: 'export PATH="{{ user_home }}/.local/bin:$PATH"'
            state: present
      become: false
      when: faas_platform == "openfaas"

    - name: Add common Helm repositories
      block:
        - name: Add HAProxy Helm repository
          ansible.builtin.command: helm repo add haproxytech https://haproxytech.github.io/helm-charts

        - name: Add Prometheus Helm repository
          ansible.builtin.command: helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml

    - name: Add OpenFaaS Helm repository
      ansible.builtin.command: helm repo add openfaas https://openfaas.github.io/faas-netes/
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openfaas"

    - name: Add OpenWhisk Helm repository
      ansible.builtin.command: helm repo add owdev https://openwhisk.apache.org/charts
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openwhisk"

    - name: Update Helm repositories
      ansible.builtin.command: helm repo update
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml

    - name: Install HAProxy on K3S
      ansible.builtin.command: >
        helm install haproxy haproxytech/haproxy
        --version 1.26.1
        --values {{ project_dir }}/k8s/charts/values-haproxy.yaml
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml

    - name: Install Prometheus on K3S (OpenFaaS)
      ansible.builtin.command: >
        helm install prometheus prometheus-community/prometheus
        --version 27.37.0
        --values {{ project_dir }}/k8s/charts/values-prometheus.yaml
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openfaas"

    - name: Install Prometheus on K3S (OpenWhisk)
      ansible.builtin.command: >
        helm install prometheus prometheus-community/prometheus
        --version 27.37.0
        --values {{ project_dir }}/k8s/charts/values-prometheus.yaml
        --values {{ project_dir }}/k8s/charts/values-prometheus-openwhisk.yaml
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openwhisk"

    - name: Install OpenFaaS on K3S
      ansible.builtin.command: >
        helm install openfaas openfaas/openfaas
        --version 14.2.124
        --values {{ project_dir }}/k8s/charts/values-openfaas.yaml
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openfaas"

    - name: Install OpenWhisk on K3S
      ansible.builtin.command: >
        helm install owdev owdev/openwhisk
        --version 1.0.1
        --values {{ project_dir }}/k8s/charts/values-openwhisk.yaml
        --namespace openwhisk
        --create-namespace
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openwhisk"

    - name: Configure OpenFaaS
      block:
        - name: Add OPENFAAS_URL to .bashrc
          ansible.builtin.lineinfile:
            path: "{{ user_home }}/.bashrc"
            line: 'export OPENFAAS_URL={{ openfaas_url }}'
            state: present

        - name: Deploy OpenFaaS functions
          ansible.builtin.shell: ./k8s/scripts/deploy_functions.sh
          args:
            chdir: "{{ project_dir }}"
          environment:
            OPENFAAS_URL: "{{ openfaas_url }}"
            KUBECONFIG: /etc/rancher/k3s/k3s.yaml
            PATH: "{{ ansible_env.PATH }}:{{ user_home }}/.local/bin"
      become: false
      when: faas_platform == "openfaas"

    - name: Retrieve OpenWhisk API key
      ansible.builtin.command: >
        kubectl -n openwhisk exec deploy/owdev-wskadmin
        -- wskadmin user get guest
      register: openwhisk_api_key
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
      when: faas_platform == "openwhisk"

    - name: Display OpenWhisk API key
      ansible.builtin.debug:
        msg: "OpenWhisk API key (set AGENT_OPENWHISK_API_KEY to this value): {{ openwhisk_api_key.stdout }}"
      when: faas_platform == "openwhisk"

    - name: Install DFaaS Forecaster
      ansible.builtin.command: helm install forecaster ./k8s/charts/forecaster
      args:
        chdir: "{{ project_dir }}"
      environment:
        KUBECONFIG: /etc/rancher/k3s/k3s.yaml
```

**Step 2: Verify YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('scripts/ansible/setup_playbook.yaml'))" && echo "YAML OK"
```

Expected: `YAML OK`

**Step 3: Commit**

```bash
git add scripts/ansible/setup_playbook.yaml
git commit -m "feat: add faas_platform variable and OpenWhisk support to setup playbook"
```

---

### Task 3: Create `scripts/provision.sh`

**Files:**
- Create: `scripts/provision.sh`

**Context:**
- Sources `scripts/lib/e2e-k3s-common.sh` for VM lifecycle functions.
- Uses `e2e_ensure_vm_running` to create/start the VM idempotently.
- Uses `e2e_sync_project_to_vm` to copy the repo to `/home/ubuntu/dfaas` on the VM (needed because Ansible commands run on the remote host and need the values files).
- Generates a temporary Ansible inventory file, cleaned up on EXIT.
- Passes `project_dir=/home/ubuntu/dfaas` to the playbook so it uses the synced copy.

**Step 1: Create `scripts/provision.sh`** with this exact content:

```bash
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
    log "  Next step:    set AGENT_OPENWHISK_API_KEY in your agent values"
fi
```

**Step 2: Make executable**

```bash
chmod +x scripts/provision.sh
```

**Step 3: Verify it sources correctly (dry syntax check)**

```bash
bash -n scripts/provision.sh && echo "syntax OK"
```

Expected: `syntax OK`

**Step 4: Commit**

```bash
git add scripts/provision.sh
git commit -m "feat: add provision.sh script for VM creation and Ansible provisioning"
```
