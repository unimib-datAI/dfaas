#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Run with --help for more information.

show_help() {
  echo "Usage: $0 <function_name>"
  echo ""
  echo "Patch a Kubernetes deployment (<function_name>) to adjust liveness and readiness"
  echo "probes separately, including success and failure thresholds."
  echo ""
  echo "Required after deploying an OpenFaaS function with faas-cli because the default"
  echo "probe timeouts are too short and may cause issues under high load. This"
  echo "workaround is needed only with OpenFaaS Community Edition."
  echo ""
  echo "Run this script on the same host where the Kubernetes cluster is running!"
  echo ""
  echo "Environment variables for customization:"
  echo "  LIVENESS_TIMEOUT             (default: 55)"
  echo "  LIVENESS_PERIOD              (default: 60)"
  echo "  LIVENESS_INITIAL_DELAY       (default: 5)"
  echo "  LIVENESS_SUCCESS_THRESHOLD   (default: 1)"
  echo "  LIVENESS_FAILURE_THRESHOLD   (default: 3)"
  echo "  READINESS_TIMEOUT            (default: 6)"
  echo "  READINESS_PERIOD             (default: 10)"
  echo "  READINESS_INITIAL_DELAY      (default: 5)"
  echo "  READINESS_SUCCESS_THRESHOLD  (default: 3)"
  echo "  READINESS_FAILURE_THRESHOLD  (default: 3)"
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

# Liveness probe values (with defaults).
LIVENESS_TIMEOUT="${LIVENESS_TIMEOUT:-55}"
LIVENESS_PERIOD="${LIVENESS_PERIOD:-60}"
LIVENESS_INITIAL_DELAY="${LIVENESS_INITIAL_DELAY:-5}"
LIVENESS_SUCCESS_THRESHOLD="${LIVENESS_SUCCESS_THRESHOLD:-1}"
LIVENESS_FAILURE_THRESHOLD="${LIVENESS_FAILURE_THRESHOLD:-3}"

# Readiness probe values (with defaults).
READINESS_TIMEOUT="${READINESS_TIMEOUT:-6}"
READINESS_PERIOD="${READINESS_PERIOD:-10}"
READINESS_INITIAL_DELAY="${READINESS_INITIAL_DELAY:-5}"
READINESS_SUCCESS_THRESHOLD="${READINESS_SUCCESS_THRESHOLD:-3}"
READINESS_FAILURE_THRESHOLD="${READINESS_FAILURE_THRESHOLD:-3}"

kubectl patch deployment "$FUNCTION_NAME" --type='json' --patch "$(cat <<EOF
[
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/timeoutSeconds", "value": $LIVENESS_TIMEOUT },
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/periodSeconds", "value": $LIVENESS_PERIOD },
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/initialDelaySeconds", "value": $LIVENESS_INITIAL_DELAY },
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/successThreshold", "value": $LIVENESS_SUCCESS_THRESHOLD },
  { "op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/failureThreshold", "value": $LIVENESS_FAILURE_THRESHOLD },

  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/timeoutSeconds", "value": $READINESS_TIMEOUT },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/periodSeconds", "value": $READINESS_PERIOD },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/initialDelaySeconds", "value": $READINESS_INITIAL_DELAY },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/successThreshold", "value": $READINESS_SUCCESS_THRESHOLD },
  { "op": "replace", "path": "/spec/template/spec/containers/0/readinessProbe/failureThreshold", "value": $READINESS_FAILURE_THRESHOLD }
]
EOF
)"
