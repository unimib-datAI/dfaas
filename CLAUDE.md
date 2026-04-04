# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DFaaS is a decentralized Function-as-a-Service architecture for federated edge computing. It uses a P2P overlay network (libp2p with Kademlia DHT) to autonomously balance traffic load across edge nodes. Each node runs: a **DFaaS Agent** (Go), a **Forecaster** (Python/FastAPI), **HAProxy** (reverse proxy), a FaaS platform (**OpenFaaS CE** or **Apache OpenWhisk**), and **Prometheus** (metrics). Deployment target is Kubernetes (K3S on Ubuntu).

## Repository Structure

- `dfaasagent/` — Go agent (the core P2P load balancing daemon)
- `forecaster/` — Python FastAPI service that predicts CPU/RAM/power usage for the Node Margin strategy
- `simulation/` — Python simulator for testing load balancing strategies offline (uses SQLite data)
- `framework/` — Evolved Python simulator that incorporates ML model predictions
- `k6/` — Load testing scripts (JavaScript) and analysis tools (Python)
- `metrics_predictions/` — ML pipeline for training forecasting models (Sampler Generator → System Forecaster)
- `k8s/` — Helm chart for the agent (`k8s/charts/agent/`) and values files for HAProxy, OpenFaaS, OpenWhisk, Prometheus
- `docs/` — Architecture docs: `overview.md`, `commands.md`, `agent-strategies.md`, `agent-startup.md`, `timeouts.md`

## Building and Running

### DFaaS Agent (Go)

```bash
# Build (run from repo root)
go build -C dfaasagent

# Run tests
go test -C dfaasagent ./...

# Run a single test package
go test -C dfaasagent ./agent/utils/maddrhelp/...
```

Go module: `github.com/unimib-datAI/dfaas/dfaasagent`, requires Go 1.24+.

### Forecaster (Python)

```bash
cd forecaster
pip install -r requirements.txt
uvicorn main:app
```

The forecaster exposes endpoints: `/cpu_usage_node`, `/ram_usage_node`, `/power_usage_node`, `/node_usage`. The `MODELS_TYPE` env var selects the model type (regression/quantile005/quantile095).

### Simulation

```bash
cd simulation
python simulation_controller.py --nodesnum 10 --edgeprob 0.3 --seed 701
# Or run stages independently:
python instance_generator.py --nodesnum 5 --edgeprob 0.3 --seed 711
python simulation.py
python analyzer.py
```

### Framework (evolved simulator with ML models)

```bash
cd framework
python simulation_controller.py --nodesnum 10 --edgeprob 0.3 --overloaded 30 60 --expnum 1 --modeltype regression --seed 701
```

### Deploying to Kubernetes

```bash
# Deploy all components (OpenFaaS — default)
sudo helm install haproxy haproxytech/haproxy --values k8s/charts/values-haproxy.yaml --version 1.26.1
sudo helm install prometheus prometheus-community/prometheus --values k8s/charts/values-prometheus.yaml --version 27.37.0
sudo helm install openfaas openfaas/openfaas --values k8s/charts/values-openfaas.yaml --version 14.2.128
sudo helm install dfaas-agent ./k8s/charts/agent --values values.yaml

# Set strategy via values.yaml:
# config:
#   AGENT_STRATEGY: "staticstrategy"

sudo helm uninstall dfaas-agent  # before reinstalling
```

For **OpenWhisk** instead of OpenFaaS, see `docs/commands.md`. The agent is configured via:
- `AGENT_FAAS_PLATFORM`: `"openfaas"` (default) or `"openwhisk"`
- `AGENT_OPENWHISK_NAMESPACE`: OpenWhisk namespace (default `"guest"`)
- `AGENT_OPENWHISK_API_KEY`: OpenWhisk API key in `"uuid:key"` format

### Load Testing with k6

```bash
k6 run k6/single_node.js --out csv=results.csv.gz

# With live dashboard and trace input:
K6_WEB_DASHBOARD=true K6_WEB_DASHBOARD_PORT=30665 k6 run k6/single_trace.js \
  --env TRACE_PATH=traces.json --out csv=k6_results.csv.gz
```

## Agent Architecture

The agent (`dfaasagent/agent/`) initializes and runs these concurrent goroutines:
- **Kademlia DHT** (`discovery/kademlia/`) — peer discovery via libp2p
- **PubSub** (`communication/`) — libp2p GossipSub for inter-agent control messages
- **Strategy** (`loadbalancer/`) — periodic weight recalculation loop
- **HTTP server** (`httpserver/`) — exposes agent state/metrics

Key packages:
- `agent/config/config.go` — all configuration via env vars (`AGENT_*`)
- `agent/faasprovider/` — `FaaSProvider` interface + `openfaas` and `openwhisk` implementations
- `agent/loadbalancer/strategyfactory.go` — entry point for strategy selection
- `agent/nodestbl/` — tracks neighboring node states
- `agent/hacfgupd/` — updates HAProxy configuration via Data Plane API

## Offloading Strategies

Set via `AGENT_STRATEGY` env var. See `docs/agent-strategies.md` for full details.

The FaaS platform is selected via `AGENT_FAAS_PLATFORM` (`"openfaas"` default, `"openwhisk"`). When using OpenWhisk, also set `AGENT_OPENWHISK_NAMESPACE` and `AGENT_OPENWHISK_API_KEY`. All four strategies are platform-agnostic — they use the `FaaSProvider` interface, which abstracts all platform-specific API and Prometheus calls.

| Strategy | Env var value | Status |
|---|---|---|
| Recalc | `recalcstrategy` (default) | Partially working (issues #45, #48) |
| Node Margin | `nodemarginstrategy` | Partially working (forecasting accuracy) |
| Static | `staticstrategy` | Working |
| All Local | `alllocalstrategy` | Working |

- **Recalc** requires a `dfaas.maxrate` label on each deployed function (requests/sec limit).
- **Node Margin** requires the `dfaas-forecaster` sidecar to be running.
- **Static** splits 60% local / 40% forwarded equally among neighbors.

## HAProxy Interaction

HAProxy is controlled via its Data Plane API (default: `localhost:30555`, credentials `admin:admin`).

```bash
# Push new config
curl -X POST -u admin:admin -H "Content-Type: text/plain" \
  --data-binary @config.cfg 'http://localhost:30555/v3/services/haproxy/configuration/raw?skip_version=true&force_reload=true'

# Validate a config file
podman run --rm -v ./config.cfg:/etc/haproxy.cfg docker.io/library/haproxy:latest haproxy -c -f /etc/haproxy.cfg
```

HAProxy config templates live in `dfaasagent/agent/loadbalancer/haproxycfg*.tmpl`.

## Troubleshooting

```bash
# Follow agent logs on the running cluster
POD=$(sudo kubectl get pods -l app=dfaas-agent -o jsonpath="{.items[0].metadata.name}")
sudo kubectl logs --follow $POD

# Invoke a function directly via HAProxy (port 30080)
curl -i http://127.0.0.1:30080/function/figlet -d 'Hello DFaaS world!'
```
