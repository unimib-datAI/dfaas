# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Recipes to build the container images of the DFaaS components.
#
# Each component has a Containerfile in its root directory, which is also used
# as build context.

set shell := ["bash", "-euo", "pipefail", "-c"]

registry := "ghcr.io/unimib-datai"

# List the available recipes.
default:
    @just --list

# Build the DFaaS Agent image, then run the given actions (k3s, push).
agent *actions: (build "agent" "dfaasagent" actions)

# Build the DFaaS Forecaster image, then run the given actions (k3s, push).
forecaster *actions: (build "forecaster" "forecaster" actions)

# Build NAME:dev from DIR/Containerfile, then run the given actions (k3s, push).
build name dir *actions:
    #!/usr/bin/env bash
    set -euo pipefail
    for action in {{ actions }}; do
      if [[ "$action" != "k3s" && "$action" != "push" ]]; then
        echo "Unknown action: $action (valid actions: k3s, push)" >&2
        exit 1
      fi
    done
    set -x
    buildah build -f "{{ dir }}/Containerfile" -t "{{ name }}:dev" "{{ dir }}"
    for action in {{ actions }}; do
      "{{ just_executable() }}" --justfile "{{ justfile() }}" "$action" "{{ name }}"
    done

# Import the already built NAME:dev image into the local k3s instance.
k3s name:
    rm -f "{{ name }}.tar"
    buildah push "{{ name }}:dev" "docker-archive:./{{ name }}.tar"
    sudo k3s ctr images import "{{ name }}.tar"
    rm -f "{{ name }}.tar"

# Push the already built NAME:dev image to GHCR (buildah must be logged in).
push name:
    buildah tag "{{ name }}:dev" "{{ registry }}/dfaas-{{ name }}:dev"
    buildah push "{{ registry }}/dfaas-{{ name }}:dev"
