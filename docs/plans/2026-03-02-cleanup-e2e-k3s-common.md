# Cleanup e2e-k3s-common.sh Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove all NanoFaaS and control-plane references from `scripts/lib/e2e-k3s-common.sh`, leaving a generic VM/k3s management library.

**Architecture:** Pure deletion + targeted string replacements. No new abstractions. The file goes from ~1450 lines to ~750 lines by removing 5 distinct sections and fixing 6 string references.

**Tech Stack:** bash

---

### Task 1: Remove `e2e_resolve_nanofaas_url`

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh:269-276`

**Step 1: Delete the function and its blank line**

Remove lines 269–276 (the `e2e_resolve_nanofaas_url` function):

```
e2e_resolve_nanofaas_url() {
    local port=${1:-30080}
    if [[ -n "${NANOFAAS_URL:-}" ]]; then echo "${NANOFAAS_URL}"; return; fi
    local vm_ip
    vm_ip=$(e2e_get_vm_ip) || true
    if [[ -z "${vm_ip}" ]]; then error "Cannot determine VM IP for '${VM_NAME}'"; return 1; fi
    echo "http://${vm_ip}:${port}"
}
```

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: remove e2e_resolve_nanofaas_url"
```

---

### Task 2: Fix `e2e_auto_detect_vm` — remove nanofaas references

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh` (the `e2e_auto_detect_vm` function)

The function has two nanofaas references:
- Default VM name: `nanofaas-e2e` → `e2e-vm`
- `grep -i "nanofaas"` filter → remove (detect any running VM)

**Step 1: Replace the function body**

Old:
```bash
e2e_auto_detect_vm() {
    local default_name=${1:-nanofaas-e2e}
    if [[ -n "${VM_NAME:-}" ]]; then echo "${VM_NAME}"; return; fi
    if command -v multipass &>/dev/null; then
        local detected
        detected=$(multipass list --format csv 2>/dev/null | tail -n +2 | grep -i "nanofaas" | grep "Running" | head -1 | cut -d, -f1) || true
        if [[ -n "${detected}" ]]; then echo "${detected}"; return; fi
    fi
    echo "${default_name}"
}
```

New:
```bash
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
```

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: remove nanofaas references from e2e_auto_detect_vm"
```

---

### Task 3: Fix nanofaas string literals in `e2e_create_vm` and `e2e_sync_project_to_vm`

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh`

Four occurrences:

| Line | Old | New |
|------|-----|-----|
| `e2e_create_vm` cloud-init temp | `"nanofaas-cloud-init"` | `"e2e-cloud-init"` |
| `e2e_sync_project_to_vm` default remote_dir | `/home/ubuntu/nanofaas` | `/home/ubuntu/project` |
| `e2e_sync_project_to_vm` sync_tar | `/tmp/nanofaas-e2e-sync.tar` | `/tmp/e2e-sync.tar` |
| `e2e_sync_project_to_vm` tmp_tar mktemp prefix | `"nanofaas-e2e-sync"` | `"e2e-sync"` |

**Step 1: Apply all four replacements**

In `e2e_create_vm`:
```bash
# old
cloud_init=$(e2e_mktemp_file "nanofaas-cloud-init" ".yaml")
# new
cloud_init=$(e2e_mktemp_file "e2e-cloud-init" ".yaml")
```

In `e2e_sync_project_to_vm`:
```bash
# old
local remote_dir=${3:-/home/ubuntu/nanofaas}
local sync_tar=/tmp/nanofaas-e2e-sync.tar
# new
local remote_dir=${3:-/home/ubuntu/project}
local sync_tar=/tmp/e2e-sync.tar
```
```bash
# old
tmp_tar=$(e2e_mktemp_file "nanofaas-e2e-sync" ".tar")
# new
tmp_tar=$(e2e_mktemp_file "e2e-sync" ".tar")
```

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: replace nanofaas string literals with generic names"
```

---

### Task 4: Fix nanofaas container name default in `e2e_setup_local_registry`

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh`

**Step 1: Replace default container name**

```bash
# old
local container_name=${2:-nanofaas-e2e-registry}
# new
local container_name=${2:-e2e-registry}
```

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: rename nanofaas-e2e-registry default to e2e-registry"
```

---

### Task 5: Remove build pipeline section (lines ~564–731)

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh:564-731`

**Step 1: Delete the entire block** from `e2e_runtime_kind` through `e2e_build_control_plane_image` (inclusive), which contains:
- `e2e_runtime_kind`
- `e2e_is_rust_runtime`
- `e2e_build_core_jars`
- `e2e_build_core_images`
- `e2e_build_function_runtime_jar`
- `e2e_build_function_runtime_image`
- `e2e_build_rust_control_plane_image`
- `e2e_build_control_plane_artifacts`
- `e2e_build_control_plane_image`

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: remove NanoFaaS build pipeline functions"
```

---

### Task 6: Remove NanoFaaS deploy section (lines ~733–1011)

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh:733-1011`

**Step 1: Delete the entire block** containing:
- `e2e_render_control_plane_sync_env`
- `e2e_deploy_control_plane`
- `e2e_deploy_function_runtime`
- `e2e_wait_for_deployment` — **KEEP** (generic k8s helper)
- `e2e_verify_core_pods_running`
- `e2e_verify_control_plane_health`
- `e2e_dump_core_pod_logs`

Note: `e2e_wait_for_deployment` is a generic helper — keep it. Remove all others.

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: remove NanoFaaS-specific deploy/verify/dump functions"
```

---

### Task 7: Remove function API and JSON extraction section (lines ~1013–1331)

**Files:**
- Modify: `scripts/lib/e2e-k3s-common.sh:1013-1331`

**Step 1: Delete the entire block** containing:
- `e2e_register_pool_function`
- `e2e_kubectl_curl_control_plane`
- `e2e_extract_json_by_field`
- `e2e_extract_execution_id`
- `e2e_extract_execution_status`
- `e2e_extract_bool_field`
- `e2e_extract_numeric_field`
- `e2e_invoke_sync_message`
- `e2e_enqueue_message`
- `e2e_fetch_execution`
- `e2e_wait_execution_success`
- `e2e_enqueue_message_burst`
- `e2e_get_control_plane_pod_name`
- `e2e_fetch_control_plane_prometheus`

**Step 2: Commit**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: remove NanoFaaS function API and JSON extraction helpers"
```

---

### Task 8: Final verification

**Step 1: Confirm no nanofaas/control-plane references remain**

```bash
grep -in "nanofaas\|control.plane\|control_plane\|function.runtime\|function_runtime\|sync_queue\|executionId\|enqueue\|POOL" \
    scripts/lib/e2e-k3s-common.sh
```

Expected: no output.

**Step 2: Confirm file size is reasonable**

```bash
wc -l scripts/lib/e2e-k3s-common.sh
```

Expected: ~700–800 lines.

**Step 3: Syntax check**

```bash
bash -n scripts/lib/e2e-k3s-common.sh && echo "syntax OK"
```

Expected: `syntax OK`

**Step 4: Final commit if needed**

```bash
git add scripts/lib/e2e-k3s-common.sh
git commit -m "refactor: clean up e2e-k3s-common.sh - remove NanoFaaS and control-plane references"
```
