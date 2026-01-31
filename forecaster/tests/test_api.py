# SPDX-License-Identifier: AGPL-3.0-or-later

import pytest
from fastapi.testclient import TestClient

import main
from model import config_constants
from model.runtime_config import RuntimeConfig


def _make_client():
    return TestClient(main.app)


def test_root_ready(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    client = _make_client()
    response = client.get("/")
    assert response.status_code == 200
    assert response.json() == "DFaaS Forecaster ready."


def test_node_usage_stub(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    expected = [
        {
            "cpu_usage_node": 1.23,
            "ram_usage_node": 4.56,
            "power_usage_node": 7.89,
        }
    ]
    monkeypatch.setattr(main.model_proxy, "get_node_predictions",
                        lambda _input: expected)
    client = _make_client()
    response = client.post("/node_usage", json={"any": "value"})
    assert response.status_code == 200
    assert response.json() == expected


def test_health_check(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: 0)
    monkeypatch.setattr(main.model_proxy, "model_version", lambda: None)
    monkeypatch.setattr(main.model_proxy, "get_model_type", lambda: "regression")
    monkeypatch.setattr(main.model_proxy, "manifest_ok", lambda: False)
    monkeypatch.setattr(main.model_proxy, "last_error", lambda: "missing models")
    client = _make_client()
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "ok"
    assert payload["ready"] is False
    assert payload["models_loaded"] == 0
    assert payload["models_expected"] == len(config_constants.METRICS)
    assert set(payload.keys()) == {
        "status",
        "ready",
        "models_loaded",
        "models_expected",
        "model_version",
        "models_type",
        "manifest_ok",
        "last_error",
    }


def test_readiness_check(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: 3)
    client = _make_client()
    response = client.get("/ready")
    assert response.status_code == 200
    assert response.json() == {"ready": True}


@pytest.mark.parametrize(
    ("models_loaded", "expected_ready"),
    [
        (0, False),
        (1, False),
        (len(config_constants.METRICS) - 1, False),
        (len(config_constants.METRICS), True),
        (len(config_constants.METRICS) + 1, True),
    ],
)
def test_health_ready_threshold(monkeypatch, models_loaded, expected_ready):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: models_loaded)
    client = _make_client()
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload["ready"] is expected_ready
    assert payload["models_expected"] == len(config_constants.METRICS)


def test_health_payload_when_ready(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        main.model_proxy, "models_loaded_count", lambda: len(config_constants.METRICS)
    )
    monkeypatch.setattr(main.model_proxy, "model_version", lambda: "v1")
    monkeypatch.setattr(main.model_proxy, "get_model_type", lambda: "regression")
    monkeypatch.setattr(main.model_proxy, "manifest_ok", lambda: True)
    monkeypatch.setattr(main.model_proxy, "last_error", lambda: None)
    client = _make_client()
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "ok"
    assert payload["ready"] is True
    assert payload["models_loaded"] == len(config_constants.METRICS)
    assert payload["models_expected"] == len(config_constants.METRICS)
    assert payload["model_version"] == "v1"
    assert payload["models_type"] == "regression"
    assert payload["manifest_ok"] is True
    assert payload["last_error"] is None


@pytest.mark.parametrize(
    ("models_loaded", "expected_ready"),
    [
        (0, False),
        (len(config_constants.METRICS) - 1, False),
        (len(config_constants.METRICS), True),
    ],
)
def test_ready_endpoint_threshold(monkeypatch, models_loaded, expected_ready):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: models_loaded)
    client = _make_client()
    response = client.get("/ready")
    assert response.status_code == 200
    assert response.json() == {"ready": expected_ready}


def test_metric_endpoints_stub(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)

    def _stub_predict(_input, metric, _to_json):
        return [{"metric": metric}]

    monkeypatch.setattr(main.model_proxy, "get_predictions", _stub_predict)
    client = _make_client()

    for path, metric in [
        ("/cpu_usage_node", config_constants.CPU_USAGE_METRIC),
        ("/ram_usage_node", config_constants.RAM_USAGE_METRIC),
        ("/power_usage_node", config_constants.POWER_USAGE_METRIC),
    ]:
        response = client.post(path, json={"any": "value"})
        assert response.status_code == 200
        assert response.json() == [{"metric": metric}]


def test_load_models_calls_proxy(monkeypatch):
    calls = []

    def _load_models(strict=True):
        calls.append(strict)

    monkeypatch.setattr(main.model_proxy, "load_models", _load_models)
    main.load_models()
    assert calls == [True]


def test_reload_endpoint_disabled(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        main,
        "runtime_config",
        RuntimeConfig(
            models_base_dir=main.runtime_config.models_base_dir,
            models_type=main.runtime_config.models_type,
            manifest_filename=main.runtime_config.manifest_filename,
            reload_mode="none",
            reload_interval_seconds=main.runtime_config.reload_interval_seconds,
            reload_token=None,
        ),
    )
    client = _make_client()
    response = client.post("/reload")
    assert response.status_code == 404


def test_reload_endpoint_token_rejected(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        main,
        "runtime_config",
        RuntimeConfig(
            models_base_dir=main.runtime_config.models_base_dir,
            models_type=main.runtime_config.models_type,
            manifest_filename=main.runtime_config.manifest_filename,
            reload_mode="endpoint",
            reload_interval_seconds=main.runtime_config.reload_interval_seconds,
            reload_token="secret",
        ),
    )
    client = _make_client()
    response = client.post("/reload", headers={"x-reload-token": "wrong"})
    assert response.status_code == 403


def test_reload_endpoint_success(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        main,
        "runtime_config",
        RuntimeConfig(
            models_base_dir=main.runtime_config.models_base_dir,
            models_type=main.runtime_config.models_type,
            manifest_filename=main.runtime_config.manifest_filename,
            reload_mode="endpoint",
            reload_interval_seconds=main.runtime_config.reload_interval_seconds,
            reload_token="secret",
        ),
    )

    def _load_models(strict=False):
        return object()

    monkeypatch.setattr(main.model_proxy, "load_models", _load_models)
    monkeypatch.setattr(main.model_proxy, "model_version", lambda: "v1")
    monkeypatch.setattr(main.model_proxy, "get_model_type", lambda: "regression")
    client = _make_client()
    response = client.post("/reload", headers={"x-reload-token": "secret"})
    assert response.status_code == 200
    assert response.json() == {
        "status": "ok",
        "model_version": "v1",
        "models_type": "regression",
    }


def test_reload_endpoint_failure(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "load_models", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        main,
        "runtime_config",
        RuntimeConfig(
            models_base_dir=main.runtime_config.models_base_dir,
            models_type=main.runtime_config.models_type,
            manifest_filename=main.runtime_config.manifest_filename,
            reload_mode="endpoint",
            reload_interval_seconds=main.runtime_config.reload_interval_seconds,
            reload_token=None,
        ),
    )

    monkeypatch.setattr(main.model_proxy, "load_models", lambda strict=False: None)
    monkeypatch.setattr(main.model_proxy, "last_error", lambda: "boom")
    client = _make_client()
    response = client.post("/reload")
    assert response.status_code == 500
    assert response.json()["detail"] == "boom"
