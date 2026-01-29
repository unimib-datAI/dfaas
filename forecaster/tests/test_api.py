# SPDX-License-Identifier: AGPL-3.0-or-later

import pytest
from fastapi.testclient import TestClient

import main
from model import config_constants


def _make_client():
    return TestClient(main.app)


def test_root_ready(monkeypatch):
    monkeypatch.setattr(main, "load_models", lambda: None)
    client = _make_client()
    response = client.get("/")
    assert response.status_code == 200
    assert response.json() == "DFaaS Forecaster ready."


def test_node_usage_stub(monkeypatch):
    monkeypatch.setattr(main, "load_models", lambda: None)
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
    monkeypatch.setattr(main, "load_models", lambda: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: 0)
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
    }


def test_readiness_check(monkeypatch):
    monkeypatch.setattr(main, "load_models", lambda: None)
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
    monkeypatch.setattr(main, "load_models", lambda: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: models_loaded)
    client = _make_client()
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload["ready"] is expected_ready
    assert payload["models_expected"] == len(config_constants.METRICS)


def test_health_payload_when_ready(monkeypatch):
    monkeypatch.setattr(main, "load_models", lambda: None)
    monkeypatch.setattr(
        main.model_proxy, "models_loaded_count", lambda: len(config_constants.METRICS)
    )
    client = _make_client()
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "ok"
    assert payload["ready"] is True
    assert payload["models_loaded"] == len(config_constants.METRICS)
    assert payload["models_expected"] == len(config_constants.METRICS)


@pytest.mark.parametrize(
    ("models_loaded", "expected_ready"),
    [
        (0, False),
        (len(config_constants.METRICS) - 1, False),
        (len(config_constants.METRICS), True),
    ],
)
def test_ready_endpoint_threshold(monkeypatch, models_loaded, expected_ready):
    monkeypatch.setattr(main, "load_models", lambda: None)
    monkeypatch.setattr(main.model_proxy, "models_loaded_count", lambda: models_loaded)
    client = _make_client()
    response = client.get("/ready")
    assert response.status_code == 200
    assert response.json() == {"ready": expected_ready}


def test_metric_endpoints_stub(monkeypatch):
    monkeypatch.setattr(main, "load_models", lambda: None)

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


def test_load_models_calls_create_model(monkeypatch):
    calls = []

    def _create_model(metric):
        calls.append(metric)

    monkeypatch.setattr(main.model_proxy, "get_model_type", lambda: "regression")
    monkeypatch.setattr(main.model_proxy, "create_model", _create_model)
    main.load_models()
    assert calls == list(config_constants.METRICS)


def test_load_models_requires_model_type(monkeypatch):
    monkeypatch.setattr(main.model_proxy, "get_model_type", lambda: None)
    try:
        main.load_models()
        assert False, "Expected RuntimeError when MODELS_TYPE is missing."
    except RuntimeError as exc:
        assert "MODELS_TYPE" in str(exc)
