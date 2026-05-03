# Ansible OpenWhisk Support + Provision Script Design

## Goal

Make the DFaaS node provisioning support both OpenFaaS (default) and OpenWhisk via a single variable, and automate VM creation + Ansible execution with a self-contained script in `scripts/`.

## Architecture

A single Ansible playbook with a `faas_platform` variable drives which FaaS stack is installed. A new `provision.sh` script uses the existing `scripts/lib/e2e-k3s-common.sh` library to create a Multipass VM and then invoke the playbook against it. All artifacts live under `scripts/` to keep it self-contained.

## Files

| File | Action |
|------|--------|
| `scripts/ansible/setup_playbook.yaml` | Modified: add `faas_platform` var and conditionals |
| `scripts/ansible/charts/values-agent-openwhisk.yaml` | New: agent values overlay for OpenWhisk |
| `scripts/provision.sh` | New: VM creation + Ansible execution script |

---

## Section 1: `setup_playbook.yaml` changes

### New variables

```yaml
vars:
  faas_platform: "openfaas"   # or "openwhisk"
  openfaas_url: "http://127.0.0.1:31112"
  user_home: "{{ ansible_env.HOME }}"
```

### Conditional tasks

| Task | Condition |
|------|-----------|
| Install `faas-cli` block | `when: faas_platform == "openfaas"` |
| Add OpenFaaS Helm repo | `when: faas_platform == "openfaas"` |
| Add `owdev` OpenWhisk Helm repo (new) | `when: faas_platform == "openwhisk"` |
| `helm install prometheus` (openfaas variant) | `when: faas_platform == "openfaas"` |
| `helm install prometheus` (openwhisk variant, double `--values`) | `when: faas_platform == "openwhisk"` (new) |
| `helm install openfaas` | `when: faas_platform == "openfaas"` |
| `helm install owdev openwhisk` (new) | `when: faas_platform == "openwhisk"` |
| Configure OpenFaaS (`OPENFAAS_URL` + `deploy_functions.sh`) | `when: faas_platform == "openfaas"` |
| Retrieve OpenWhisk API key via `wskadmin` (new) | `when: faas_platform == "openwhisk"` |

HAProxy, K3S, Helm install, image builds, and Forecaster are always executed regardless of platform.

### OpenWhisk Helm install command (new task)

```yaml
- name: Install OpenWhisk on K3S
  ansible.builtin.command: >
    helm install owdev owdev/openwhisk
    --version 1.0.1
    --values {{ playbook_dir }}/charts/values-openwhisk.yaml
    --namespace openwhisk
    --create-namespace
  when: faas_platform == "openwhisk"
  environment:
    KUBECONFIG: /etc/rancher/k3s/k3s.yaml
```

### OpenWhisk repo (new task)

```yaml
- name: Add OpenWhisk Helm repository
  ansible.builtin.command: helm repo add owdev https://openwhisk.apache.org/charts
  when: faas_platform == "openwhisk"
```

### Retrieve OpenWhisk API key (new task)

```yaml
- name: Retrieve OpenWhisk API key
  ansible.builtin.command: >
    kubectl -n openwhisk exec deploy/owdev-wskadmin
    -- wskadmin user get guest
  register: openwhisk_api_key
  when: faas_platform == "openwhisk"
  environment:
    KUBECONFIG: /etc/rancher/k3s/k3s.yaml

- name: Display OpenWhisk API key
  ansible.builtin.debug:
    msg: "OpenWhisk API key: {{ openwhisk_api_key.stdout }}"
  when: faas_platform == "openwhisk"
```

---

## Section 2: `scripts/ansible/charts/values-agent-openwhisk.yaml`

Agent values overlay for OpenWhisk. Used alongside the base `values.yaml`:

```yaml
# Agent values overlay for OpenWhisk platform.
# Pass alongside your base values when installing the agent:
#   helm install dfaas-agent ./k8s/charts/agent \
#     --values my-values.yaml \
#     --values scripts/ansible/charts/values-agent-openwhisk.yaml
config:
  AGENT_FAAS_PLATFORM: "openwhisk"
  AGENT_FAAS_HOST: "owdev-nginx.openwhisk"
  AGENT_FAAS_PORT: 80
  AGENT_OPENWHISK_NAMESPACE: "guest"
  AGENT_OPENWHISK_API_KEY: ""  # Set after retrieving via wskadmin
```

---

## Section 3: `scripts/provision.sh`

### Purpose

Self-contained script that:
1. Sources `scripts/lib/e2e-k3s-common.sh`
2. Creates/ensures a Multipass VM is running
3. Generates a temporary Ansible inventory with VM IP + SSH key
4. Runs `ansible-playbook scripts/ansible/setup_playbook.yaml`
5. Cleans up the temporary inventory on exit

### Interface

```bash
# Default (OpenFaaS, VM name dfaas-node)
./scripts/provision.sh

# OpenWhisk
./scripts/provision.sh --faas-platform openwhisk

# Custom VM
./scripts/provision.sh --vm-name dfaas-node-1 --cpus 4 --memory 8G --disk 30G --faas-platform openwhisk
```

Environment variable overrides (consistent with the library):
- `VM_NAME` (default: `dfaas-node`)
- `FAAS_PLATFORM` (default: `openfaas`) — overridden by `--faas-platform`
- `KEEP_VM` — passed through to library

### Behavior

- Uses `e2e_ensure_vm_running` for idempotent VM management
- Generates inventory at a temp path via `e2e_mktemp_file`
- Detects the Multipass SSH key via `e2e_get_ssh_identity_opt`
- Passes `faas_platform` as Ansible extra-var (`-e faas_platform=...`)
- Traps EXIT to remove temp inventory
- Prints SSH access info on completion

### Generated inventory

```ini
[all]
dfaas-node ansible_host=192.168.64.10 ansible_user=ubuntu ansible_ssh_private_key_file=/path/to/key ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'
```
