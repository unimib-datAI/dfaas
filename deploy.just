# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Recipes to deploy DFaaS on the local k3s node.
#
# There are three deployment modes:
#
#   just deploy-all CONFIG    Agent, HAProxy, Prometheus, FaaS platform, functions
#   just deploy-no-agent      HAProxy, Prometheus, FaaS platform, functions
#   just deploy-faas          Prometheus, FaaS platform, functions
#
# All recipes can be run again to update an existing deployment. Variables can
# be overridden from the command line, before the recipe name:
#
#   just platform=openwhisk deploy-faas
#   just function_replica_timeout=60s function_gateway_timeout=61s deploy-faas

# FaaS platform to deploy: "openfaas" or "openwhisk".
platform := "openfaas"

# OpenFaaS only: timeouts and limits, shared by all functions. The timeouts must
# be increasing, see docs/timeouts.md. The gateway timeout is global and it must
# stay below the gateway read and write timeouts (65s by default).
#
# With these values, memstress must be called with "seconds" below 4 (its
# default is 5) or the request is cut off. Increase them if you need more.
function_replica_timeout := "4s"
function_gateway_timeout := "5s"
function_proxy_timeout_ms := "6000"
function_max_inflight := "400"

# Maximum rate of the functions in requests/s, required by the Recalc strategy.
function_max_rate := "100"

# OpenWhisk only: IP address of the node, as reached by wsk clients.
node_ip := `hostname -I | cut -d' ' -f1`

# OpenWhisk only: fixed administrator credentials set in the chart's values.
openwhisk_auth := "789c46b1-71f6-4ed5-8c54-816aa4f8c502:abczO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP"

# Installed in /opt/bin by the setup, which may not be in PATH.
faas_cli := "/opt/bin/faas-cli"
wsk := "/opt/bin/wsk"

charts := justfile_directory() / "k8s/charts"
scripts := justfile_directory() / "k8s/scripts"
functions := justfile_directory() / "functions"

sudo_k := "sudo env KUBECONFIG=/etc/rancher/k3s/k3s.yaml"

# Wait for the pods, so a failed deploy stops here and the next recipes (like
# the functions, that need the gateway) can rely on what has been deployed.
helm_deploy := sudo_k + " helm upgrade --install --namespace default --wait --timeout 10m"

# Deploy the whole DFaaS node. CONFIG is the Helm values file of the agent.
[group('deploy')]
deploy-all config: (_check-agent-config config) deploy-no-agent (deploy-agent config)

# Deploy everything except the agent. HAProxy replies 503 until it is configured.
[group('deploy')]
deploy-no-agent: _check-platform deploy-prometheus deploy-haproxy deploy-rejector deploy-faas

# Deploy only the FaaS platform, the functions and Prometheus, which both platforms
# require. Call the functions on the platform's port.
[group('deploy')]
deploy-faas: _check-platform deploy-prometheus deploy-platform deploy-functions

# Deploy the agent and restart it, since it reads its config only at startup.
[group('deploy components')]
deploy-agent config: (_check-agent-config config)
    {{ sudo_k }} helm upgrade --install agent "{{ charts }}/agent" \
        --namespace default \
        --values "{{ join(invocation_directory(), config) }}"
    {{ sudo_k }} kubectl rollout restart deployment/dfaas-agent
    {{ sudo_k }} kubectl rollout status deployment/dfaas-agent --timeout=5m

# Deploy HAProxy and the Service of its metrics exporter sidecar.
[group('deploy components')]
deploy-haproxy: _helm-repos
    {{ helm_deploy }} haproxy haproxytech/haproxy \
        --version 1.28.1 \
        --values "{{ charts }}/values-haproxy.yaml"
    {{ sudo_k }} kubectl apply -f "{{ scripts }}/haproxy-metrics-exporter/service.yaml"

# Deploy the rejector, used by the HAProxy configs that can reject requests.
[group('deploy components')]
deploy-rejector:
    {{ helm_deploy }} rejector "{{ scripts }}/rejector/helm" \
        --values "{{ charts }}/values-rejector.yaml"

# Deploy Prometheus.
[group('deploy components')]
deploy-prometheus: _helm-repos
    {{ helm_deploy }} prometheus prometheus-community/prometheus \
        --version 27.37.0 \
        --values "{{ charts }}/values-prometheus.yaml"

# Deploy the FaaS platform selected with the "platform" variable.
[group('deploy components')]
deploy-platform: _check-platform _helm-repos
    #!/usr/bin/env bash
    set -euo pipefail
    set -x
    if [[ "{{ platform }}" == "openfaas" ]]; then
      {{ helm_deploy }} openfaas openfaas/openfaas \
          --version 14.2.124 \
          --values "{{ charts }}/values-openfaas.yaml" \
          --set gateway.upstreamTimeout={{ function_gateway_timeout }}
    else
      # Do not use --wait: OpenWhisk is ready only when this job has completed,
      # and it requires a healthy controller and invoker.
      {{ sudo_k }} helm upgrade --install openwhisk "{{ charts }}/openwhisk" \
          --namespace default \
          --set whisk.apiHostName={{ node_ip }}
      {{ sudo_k }} kubectl wait --for=condition=complete \
          job/openwhisk-install-packages --timeout=20m
    fi

# Create or update the functions in functions/ on the selected FaaS platform.
[group('deploy components')]
deploy-functions: _check-platform
    #!/usr/bin/env bash
    set -euo pipefail
    set -x
    if [[ "{{ platform }}" == "openfaas" ]]; then
      export OPENFAAS_URL=http://127.0.0.1:31112
      for function in mlimage:image-classification memstress:memstress noop:noop; do
        {{ faas_cli }} deploy \
            --name="${function%%:*}" \
            --image="{{ registry }}/dfaas-openfaas-${function##*:}:dev" \
            --env exec_timeout={{ function_replica_timeout }} \
            --env max_inflight={{ function_max_inflight }} \
            --label dfaas.maxrate={{ function_max_rate }} \
            --label dfaas.timeout_ms={{ function_proxy_timeout_ms }}
      done
    else
      wsk=({{ wsk }} --apihost "http://{{ node_ip }}:31001" --auth "{{ openwhisk_auth }}")
      common=(--web true --memory 512 --annotation dfaas.maxrate {{ function_max_rate }})

      # PyTorch is too large for a zip action, so the dependencies are in a custom
      # runtime image, and only the code is packaged.
      workdir=$(mktemp -d)
      trap 'rm -rf "$workdir"' EXIT
      zip -q -j "$workdir/action.zip" \
          "{{ functions }}/openwhisk/image-classification/__main__.py" \
          "{{ functions }}/openwhisk/image-classification/function.py"
      "${wsk[@]}" action update mlimage "$workdir/action.zip" "${common[@]}" \
          --docker {{ registry }}/dfaas-openwhisk-image-classification:dev \
          --timeout 120000

      "${wsk[@]}" action update memstress "{{ functions }}/openwhisk/memstress/main.go" \
          "${common[@]}" --kind go:1.23 --timeout 60000
      "${wsk[@]}" action update noop "{{ functions }}/openwhisk/noop/main.go" \
          "${common[@]}" --kind go:1.23
    fi

# Fail before changing the cluster if the deploy cannot work.
_check-platform:
    @case "{{ platform }}" in openfaas|openwhisk) ;; *) echo "Unknown platform: {{ platform }} (valid platforms: openfaas, openwhisk)" >&2; exit 1 ;; esac

# Fail before changing the cluster if the agent's config is not usable. Checks
# the rendered chart, because the config may rely on the chart's default values.
_check-agent-config config: _check-platform
    #!/usr/bin/env bash
    set -euo pipefail
    config="{{ join(invocation_directory(), config) }}"
    if [[ ! -f "$config" ]]; then
      echo "Agent config not found: $config" >&2
      exit 1
    fi

    rendered=$(helm template agent "{{ charts }}/agent" --values "$config")

    configured=$(sed -n 's/^ *AGENT_FAAS_PLATFORM: "\(.*\)"$/\1/p' <<< "$rendered")
    if [[ "$configured" != "{{ platform }}" ]]; then
      echo "The agent config sets AGENT_FAAS_PLATFORM to \"$configured\", but the platform to deploy is \"{{ platform }}\"." >&2
      echo "Fix the config or run: just platform=$configured ..." >&2
      exit 1
    fi

    # The chart defaults to local images that Kubernetes never pulls, without
    # them the pods would stay in ErrImageNeverPull.
    images=$(sed -n 's/^ *image: \(localhost\/.*\)$/\1/p' <<< "$rendered")
    if [[ -n "$images" ]]; then
      imported=$(sudo k3s ctr images ls -q)
      while IFS= read -r image; do
        if ! grep -qFx "$image" <<< "$imported"; then
          echo "Image $image is not imported in k3s." >&2
          echo "Build it (just agent k3s, just forecaster k3s) or set a remote image in the config." >&2
          exit 1
        fi
      done <<< "$images"
    fi

# Add the Helm repositories (as root, the same user that deploys the charts).
_helm-repos:
    sudo helm repo add haproxytech https://haproxytech.github.io/helm-charts
    sudo helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    sudo helm repo add openfaas https://openfaas.github.io/faas-netes/
    sudo helm repo update
