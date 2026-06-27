# SPDX-License-Identifier: AGPL-3.0-or-later

from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import time
from pathlib import Path

import joblib
import pytest


@pytest.mark.e2e_k3s
def test_k3s_vm_end_to_end():
    if not shutil.which("multipass"):
        pytest.skip("multipass not available")

    _log("Starting E2E test with in-VM image build")
    vm = _VM()
    vm.start()

    try:
        # Setup build environment
        _install_docker(vm)
        forecaster_ctx, sidecar_ctx = _stage_build_context(vm)

        # Build images in VM
        _build_images_in_vm(vm, forecaster_ctx, sidecar_ctx)

        # Setup k3s and import images
        _install_k3s(vm)
        _import_images_to_k3s(vm)

        # Deploy and test
        _stage_model_bundle(vm)
        _deploy_manifest(vm)
        _wait_for_ready(vm)
        _exercise_endpoints(vm)
        _trigger_sidecar_reload(vm)
    finally:
        vm.delete()


class _VM:
    def __init__(self) -> None:
        self.name = f"forecaster-e2e-{int(time.time())}"
        self.user = "ubuntu"
        self.home = "/home/ubuntu"

    def start(self) -> None:
        _log(f"Starting multipass VM: {self.name}")
        self._run([
            "multipass", "launch",
            "--name", self.name,
            "--cpus", "2",
            "--memory", "4G",
            "--disk", "20G",
        ])

    def delete(self) -> None:
        _log(f"Deleting multipass VM: {self.name}")
        subprocess.run(["multipass", "delete", self.name], check=False)
        subprocess.run(["multipass", "purge"], check=False)

    def exec(self, command: str) -> str:
        _log(f"[vm exec] {command}")
        return self._run(["multipass", "exec", self.name, "--", "bash", "-lc", command])

    def copy(self, src: Path, dest: str) -> None:
        _log(f"[vm copy] {src} -> {dest}")
        dest_dir = os.path.dirname(dest)
        if dest_dir:
            self.exec(f"mkdir -p {dest_dir}")

        if src.is_dir():
            # multipass transfer doesn't support directories, use tar
            with tempfile.NamedTemporaryFile(suffix=".tar.gz", delete=False) as tmp:
                tmp_path = Path(tmp.name)
            try:
                subprocess.run(
                    ["tar", "-czf", str(tmp_path), "-C", str(src.parent), src.name],
                    check=True
                )
                self._run(["multipass", "transfer", str(tmp_path), f"{self.name}:{dest}.tar.gz"])
                self.exec(f"tar -xzf {dest}.tar.gz -C {dest_dir} && rm {dest}.tar.gz")
            finally:
                tmp_path.unlink(missing_ok=True)
        else:
            self._run(["multipass", "transfer", str(src), f"{self.name}:{dest}"])

    def _run(self, cmd: list[str]) -> str:
        result = subprocess.run(
            cmd,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        return result.stdout


def _install_docker(vm: _VM) -> None:
    """Install Docker in the VM."""
    _log("Installing Docker...")
    vm.exec("curl -fsSL https://get.docker.com | sudo sh")
    vm.exec("sudo usermod -aG docker $USER")
    vm.exec("sudo docker version")
    _log("Docker installed successfully.")


def _stage_build_context(vm: _VM) -> tuple[str, str]:
    """Copy source files to VM for image building."""
    repo_root = Path(__file__).resolve().parents[1]

    with tempfile.TemporaryDirectory() as tmp_dir:
        tmp_path = Path(tmp_dir)

        # Create forecaster build context
        forecaster_ctx = tmp_path / "forecaster"
        forecaster_ctx.mkdir()
        shutil.copy(repo_root / "Dockerfile", forecaster_ctx)
        shutil.copy(repo_root / "pyproject.toml", forecaster_ctx)
        shutil.copy(repo_root / "uv.lock", forecaster_ctx)
        shutil.copy(repo_root / "main.py", forecaster_ctx)
        shutil.copy(repo_root / "manifest.json", forecaster_ctx)
        if (repo_root / ".dockerignore").exists():
            shutil.copy(repo_root / ".dockerignore", forecaster_ctx)
        shutil.copytree(repo_root / "model", forecaster_ctx / "model")
        shutil.copytree(repo_root / "models", forecaster_ctx / "models")
        shutil.copytree(repo_root / "scalers", forecaster_ctx / "scalers")
        shutil.copytree(repo_root / "config", forecaster_ctx / "config")

        # Create sidecar build context
        sidecar_ctx = tmp_path / "sidecar"
        shutil.copytree(repo_root / "sidecar", sidecar_ctx)

        # Create tarball and transfer
        tarball = tmp_path / "build-context.tar.gz"
        subprocess.run(
            ["tar", "-czf", str(tarball), "-C", str(tmp_path), "forecaster", "sidecar"],
            check=True
        )

        _log("Copying build context to VM...")
        vm.copy(tarball, f"{vm.home}/build-context.tar.gz")
        vm.exec(f"cd {vm.home} && tar -xzf build-context.tar.gz && rm build-context.tar.gz")

    return (f"{vm.home}/forecaster", f"{vm.home}/sidecar")


def _build_images_in_vm(vm: _VM, forecaster_ctx: str, sidecar_ctx: str) -> None:
    """Build Docker images inside the VM."""
    _log("Building forecaster image in VM...")
    vm.exec(f"sudo docker build -t forecaster:latest {forecaster_ctx}")

    _log("Building sidecar image in VM...")
    vm.exec(f"sudo docker build -t forecaster-sidecar:latest {sidecar_ctx}")

    _log("Images built successfully.")


def _install_k3s(vm: _VM) -> None:
    _log("Installing k3s...")
    vm.exec("curl -sfL https://get.k3s.io | sudo sh -s - --disable traefik")
    deadline = time.time() + 180
    while time.time() < deadline:
        output = vm.exec("sudo k3s kubectl get nodes --no-headers || true")
        if " Ready" in output or "\tReady" in output:
            _log("k3s node is Ready.")
            return
        time.sleep(5)
    raise AssertionError("k3s node not ready in time")


def _import_images_to_k3s(vm: _VM) -> None:
    """Export images from Docker and import into k3s containerd."""
    _log("Exporting images from Docker...")
    vm.exec(f"sudo docker save forecaster:latest -o {vm.home}/forecaster.tar")
    vm.exec(f"sudo docker save forecaster-sidecar:latest -o {vm.home}/sidecar.tar")

    _log("Importing images into k3s containerd...")
    vm.exec(f"sudo k3s ctr images import {vm.home}/forecaster.tar")
    vm.exec(f"sudo k3s ctr images import {vm.home}/sidecar.tar")

    vm.exec(f"rm -f {vm.home}/forecaster.tar {vm.home}/sidecar.tar")
    _log("Images imported into k3s.")


def _stage_model_bundle(vm: _VM) -> None:
    repo_root = Path(__file__).resolve().parents[1]
    _log("Staging model bundle on VM...")
    vm.exec("sudo mkdir -p /var/lib/forecaster-models/bundle")
    vm.copy(repo_root / "models", f"{vm.home}/models")
    vm.copy(repo_root / "scalers", f"{vm.home}/scalers")
    vm.copy(repo_root / "manifest.json", f"{vm.home}/manifest.json")

    vm.exec("sudo rm -rf /var/lib/forecaster-models/bundle/models")
    vm.exec("sudo rm -rf /var/lib/forecaster-models/bundle/scalers")
    vm.exec(f"sudo mv {vm.home}/models /var/lib/forecaster-models/bundle/")
    vm.exec(f"sudo mv {vm.home}/scalers /var/lib/forecaster-models/bundle/")
    vm.exec(f"sudo mv {vm.home}/manifest.json /var/lib/forecaster-models/bundle/")

    vm.exec("cd /var/lib/forecaster-models && sudo ln -sfn bundle current")

    _log("Tweaking manifest for faster sidecar polling...")
    vm.exec(
        "python3 - <<'PY'\n"
        "import json\n"
        "path = '/var/lib/forecaster-models/bundle/manifest.json'\n"
        "data = json.load(open(path))\n"
        "data.setdefault('sidecar', {})['poll_interval_seconds'] = 5\n"
        "json.dump(data, open(path, 'w'), indent=2)\n"
        "PY"
    )


def _deploy_manifest(vm: _VM) -> None:
    repo_root = Path(__file__).resolve().parents[1]
    _log("Deploying k8s manifest...")
    vm.copy(repo_root / "k8s" / "forecaster-sidecar.yaml", f"{vm.home}/forecaster-sidecar.yaml")
    vm.exec(f"sudo k3s kubectl apply -f {vm.home}/forecaster-sidecar.yaml")
    _log("Waiting for deployment rollout...")
    try:
        vm.exec("sudo k3s kubectl rollout status deployment/forecaster --timeout=180s")
    except subprocess.CalledProcessError:
        _dump_k3s_debug(vm, "Deployment failed")
        raise


def _wait_for_ready(vm: _VM) -> None:
    _log("Waiting for service readiness...")
    deadline = time.time() + 180
    last_payload: dict[str, object] | None = None
    last_raw: str | None = None
    last_error: str | None = None
    while time.time() < deadline:
        try:
            health = _curl(vm, "http://forecaster:8000/health")
        except subprocess.CalledProcessError as exc:
            last_raw = (exc.stdout or "").strip()
            time.sleep(5)
            continue
        try:
            payload = json.loads(health)
            if payload.get("ready") is True:
                _log("Service reported ready.")
                return
            last_payload = payload
            last_error = payload.get("last_error") if isinstance(payload, dict) else None
        except json.JSONDecodeError:
            last_raw = health.strip() or None
        time.sleep(5)
    _dump_k3s_debug(vm, "Service did not become ready in time")
    details = []
    if last_payload is not None:
        details.append(f"last health payload: {last_payload}")
    if last_error:
        details.append(f"last_error: {last_error}")
    if last_raw and not details:
        details.append(f"last raw response: {last_raw}")
    suffix = f" ({'; '.join(details)})" if details else ""
    raise AssertionError(f"Service did not become ready in time{suffix}")


def _exercise_endpoints(vm: _VM) -> None:
    _log("Exercising service endpoints...")
    payload = _build_payload()
    payload_json = json.dumps(payload)

    health = json.loads(_curl(vm, "http://forecaster:8000/health"))
    assert health.get("status") == "ok"

    ready = json.loads(_curl(vm, "http://forecaster:8000/ready"))
    assert ready.get("ready") is True

    for path, key in [
        ("/cpu_usage_node", "cpu_usage_node"),
        ("/ram_usage_node", "ram_usage_node"),
        ("/power_usage_node", "power_usage_node"),
    ]:
        response = _curl(vm, f"http://forecaster:8000{path}", payload_json)
        data = json.loads(response)
        assert isinstance(data, list)
        assert key in data[0]

    node_resp = json.loads(_curl(vm, "http://forecaster:8000/node_usage", payload_json))
    assert all(
        key in node_resp[0]
        for key in ["cpu_usage_node", "ram_usage_node", "power_usage_node"]
    )


def _trigger_sidecar_reload(vm: _VM) -> None:
    _log("Triggering sidecar promotion...")
    vm.exec("sudo rm -rf /var/lib/forecaster-models/staging")
    vm.exec("sudo cp -r /var/lib/forecaster-models/bundle /var/lib/forecaster-models/staging")
    vm.exec(
        "sudo python3 - <<'PY'\n"
        "import json\n"
        "path = '/var/lib/forecaster-models/staging/manifest.json'\n"
        "data = json.load(open(path))\n"
        "data['model_version'] = 'e2e-v2'\n"
        "json.dump(data, open(path, 'w'), indent=2)\n"
        "PY"
    )
    vm.exec("sudo touch /var/lib/forecaster-models/staging/READY")

    deadline = time.time() + 120
    while time.time() < deadline:
        health = _curl(vm, "http://forecaster:8000/health")
        try:
            payload = json.loads(health)
            if payload.get("model_version") == "e2e-v2":
                _log("Sidecar promotion succeeded.")
                return
        except json.JSONDecodeError:
            pass
        time.sleep(5)
    raise AssertionError("Sidecar promotion did not update model_version")


def _curl(vm: _VM, url: str, payload: str | None = None) -> str:
    pod_name = f"curl-{int(time.time() * 1000)}"
    if payload is None:
        cmd = (
            f"sudo k3s kubectl run {pod_name} --quiet --rm -i --restart=Never "
            "--image=curlimages/curl:8.6.0 -- "
            f"curl -s --max-time 5 {url}"
            " 2>/dev/null"
        )
    else:
        cmd = (
            f"sudo k3s kubectl run {pod_name} --quiet --rm -i --restart=Never "
            "--image=curlimages/curl:8.6.0 -- "
            f"curl -s --max-time 5 -H 'Content-Type: application/json' "
            f"-d '{payload}' {url}"
            " 2>/dev/null"
        )
    output = vm.exec(cmd)
    # kubectl may print "pod ... deleted" after the command; drop those lines.
    cleaned = []
    for line in output.splitlines():
        if line.startswith("pod ") and " deleted" in line:
            continue
        cleaned.append(line)
    return "\n".join(cleaned).strip()


def _dump_k3s_debug(vm: _VM, header: str) -> None:
    _log(f"{header}! Gathering debug info...")
    _log("--- Pod status ---")
    print(vm.exec("sudo k3s kubectl get pods -o wide || true"), flush=True)
    _log("--- Pod describe ---")
    print(vm.exec("sudo k3s kubectl describe pods -l app=forecaster || true"), flush=True)
    _log("--- Pod logs (current) ---")
    print(vm.exec("sudo k3s kubectl logs -l app=forecaster --all-containers --tail=200 || true"), flush=True)
    _log("--- Pod logs (previous) ---")
    print(vm.exec("sudo k3s kubectl logs -l app=forecaster --all-containers --previous --tail=200 || true"), flush=True)
    _log("--- Events ---")
    print(vm.exec("sudo k3s kubectl get events --sort-by='.lastTimestamp' | tail -30 || true"), flush=True)


def _build_payload() -> dict[str, float]:
    repo_root = Path(__file__).resolve().parents[1]
    scaler = joblib.load(repo_root / "scalers" / "scaler_x" / "features.joblib")
    feature_names = getattr(scaler, "feature_names_in_", None)
    if feature_names is None:
        n_features = getattr(scaler, "n_features_in_", None)
        if n_features is None:
            raise AssertionError("Unable to derive input feature names for E2E.")
        feature_names = [f"f{i}" for i in range(n_features)]
    return {name: 0.0 for name in feature_names}


def _log(message: str) -> None:
    print(f"[e2e_k3s] {message}", flush=True)


def _env_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None:
        return default
    try:
        return int(value)
    except ValueError:
        return default
