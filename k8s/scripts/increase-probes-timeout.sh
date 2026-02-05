#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Run with --help for more information.

show_help() {
  echo "Usage: $0 <function_name> [timeoutSeconds] [periodSeconds] [initialDelaySeconds]"
  echo ""
  echo "Patch a Kubernetes deployment (<function_name>) to adjust liveness and readiness"
  echo "probes."
  echo ""
  echo "Required after deploying an OpenFaaS function with faas-cli because the default"
  echo "probe timeouts are too short and may cause issues under high load. This"
  echo "workaround is needed only with OpenFaaS Community Edition."
  echo ""
  echo "Run this script on the same host where the Kubernetes cluster is running!"
  echo ""
  echo "Default values:"
  echo "  timeoutSeconds:      10"
  echo "  periodSeconds:       15"
  echo "  initialDelaySeconds:  5"
  echo ""
  echo "Options:"
  echo "  -h, --help            Show this help message and exit"
}

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
  show_help
  exit 0
fi

if [[ -z "$1" ]]; then
  show_help
  exit 1
fi

FUNCTION_NAME="$1"
TIMEOUT="${2:-10}"
PERIOD="${3:-15}"
INITIAL_DELAY="${4:-5}"

# We suppose that the deployment is on the default namespace.
kubectl patch deployment "$FUNCTION_NAME" --type='json' --patch "$(cat <<EOF
[
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/timeoutSeconds", "value": $TIMEOUT },
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/periodSeconds", "value": $PERIOD },
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/initialDelaySeconds", "value": $INITIAL_DELAY },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/timeoutSeconds", "value": $TIMEOUT },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/periodSeconds", "value": $PERIOD },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/initialDelaySeconds", "value": $INITIAL_DELAY }
]
EOF
)"
