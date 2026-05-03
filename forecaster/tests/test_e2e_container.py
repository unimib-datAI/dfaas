# SPDX-License-Identifier: AGPL-3.0-or-later

from __future__ import annotations

import shutil
import socket
import subprocess
import time
from pathlib import Path

import httpx
import joblib
import pytest


E2E_IMAGE = "forecaster-e2e:latest"


def _run(cmd: list[str], cwd: Path | None = None) -> subprocess.CompletedProcess:
    return subprocess.run(
        cmd,
        check=False,
        cwd=str(cwd) if cwd else None,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )


def _ensure_container_system() -> None:
    result = _run(["container", "system", "start"])
    if result.returncode != 0:
        message = (result.stderr or result.stdout).strip()
        pytest.skip(f"container system not available: {message}")


def _find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("", 0))
        return sock.getsockname()[1]


def _wait_ready(base_url: str, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            response = httpx.get(f"{base_url}/ready", timeout=2.0)
            if response.status_code == 200 and response.json().get("ready") is True:
                return
        except Exception:
            pass
        time.sleep(1.0)
    raise AssertionError("Service did not become ready in time.")


def _build_payload(repo_root: Path) -> dict[str, float]:
    scaler = joblib.load(repo_root / "scalers" / "scaler_x" / "features.joblib")
    feature_names = getattr(scaler, "feature_names_in_", None)
    if feature_names is None:
        n_features = getattr(scaler, "n_features_in_", None)
        if n_features is None:
            raise AssertionError("Unable to derive input feature names for E2E.")
        feature_names = [f"f{i}" for i in range(n_features)]

    payload = {name: 0.0 for name in feature_names}
    return payload


@pytest.mark.e2e
def test_e2e_container_predictions():
    if not shutil.which("container"):
        pytest.skip("apple/container CLI not available.")

    repo_root = Path(__file__).resolve().parents[1]
    payload = _build_payload(repo_root)

    _ensure_container_system()

    build_result = _run(
        ["container", "build", "-t", E2E_IMAGE, "-f", "Dockerfile", "."],
        cwd=repo_root,
    )
    if build_result.returncode != 0:
        message = (build_result.stderr or build_result.stdout).strip()
        raise AssertionError(f"container build failed: {message}")

    port = _find_free_port()
    container_name = f"forecaster-e2e-{port}"

    run_result = _run(
        [
            "container",
            "run",
            "--rm",
            "-d",
            "--name",
            container_name,
            "-p",
            f"{port}:8000",
            E2E_IMAGE,
        ]
    )
    if run_result.returncode != 0:
        message = (run_result.stderr or run_result.stdout).strip()
        raise AssertionError(f"container run failed: {message}")

    base_url = f"http://127.0.0.1:{port}"
    try:
        _wait_ready(base_url)

        root = httpx.get(f"{base_url}/", timeout=5.0)
        assert root.status_code == 200

        health = httpx.get(f"{base_url}/health", timeout=5.0)
        assert health.status_code == 200
        assert health.json().get("ready") is True

        for path, key in [
            ("/cpu_usage_node", "cpu_usage_node"),
            ("/ram_usage_node", "ram_usage_node"),
            ("/power_usage_node", "power_usage_node"),
        ]:
            response = httpx.post(f"{base_url}{path}", json=payload, timeout=10.0)
            assert response.status_code == 200
            data = response.json()
            assert isinstance(data, list)
            assert key in data[0]

        node_response = httpx.post(
            f"{base_url}/node_usage", json=payload, timeout=10.0
        )
        assert node_response.status_code == 200
        data = node_response.json()
        assert isinstance(data, list)
        assert all(
            key in data[0]
            for key in ["cpu_usage_node", "ram_usage_node", "power_usage_node"]
        )
    finally:
        subprocess.run(
            ["container", "rm", "-f", container_name],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
