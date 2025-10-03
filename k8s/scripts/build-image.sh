#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Utility to build DFaaS Agent and DFaaS Forecaster Docker images.
#
# Must be run from the project's root directory!

# Exit on command errors.
set -euo pipefail

# Show help message if no arguments are given or on -h|--help.
if [[ $# -eq 0 || "$1" == "-h" || "$1" == "--help" ]]; then
  echo "Usage: $0 <image_name> [<mode> ...] [--dockerfile <path>] [--skip-build]"
  echo
  echo "Arguments:"
  echo "  <image_name>   The image name (e.g., agent, forecaster)."
  echo "  <mode>         'k3s' to build, tag and import into local k3s,"
  echo "                 'push' to build, tag and push to GHCR remote registry,"
  echo "                 'tag' to only build and tag the image."
  echo
  echo "Optional flags:"
  echo "  --dockerfile   Specify the Dockerfile path. If omitted, the default"
  echo "                 directory is k8s/docker/."
  echo "  --skip-build   Skip the build step."
  echo
  echo "After the build, the image is tagged as 'NAME:dev' in the local"
  echo "registry. If push is specified, it is also tagged as"
  echo "'ghcr.io/unimit-datai/dfaas-NAME:dev'."
  echo
  echo "When imported into the local k3s instance, the tag remains 'NAME:dev.'"
  echo
  echo "If 'push' is used, make sure that buildah is logged in to GHCR!"
  echo
  echo "For the operator component, do not push to the local k3s instance, as it"
  echo "is an external component."
  exit 0
fi

IMAGE_NAME=""
DOCKERFILE=""
MODES=()
SKIP_BUILD=false

# Parse arguments and options.
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dockerfile)
      shift
      if [[ -z "$1" ]]; then
        echo "Error: --dockerfile requires a path argument."
        exit 1
      fi
      DOCKERFILE="$1"
      shift
      ;;
    --skip-build)
      SKIP_BUILD=true
      shift
      ;;
    *)
      if [[ -z "$IMAGE_NAME" ]]; then
        IMAGE_NAME="$1"
      else
        MODES+=("$1")
      fi
      shift
      ;;
  esac
done

if [[ -z "$DOCKERFILE" ]]; then
  DOCKERFILE="k8s/docker/Dockerfile.${IMAGE_NAME}"
fi

IMAGE_TAG="${IMAGE_NAME}:dev"

if [[ "$SKIP_BUILD" == false ]]; then
  echo "-- Command: buildah build -f \"${DOCKERFILE}\" -t \"${IMAGE_TAG}\" ."
  buildah build -f "${DOCKERFILE}" -t "${IMAGE_TAG}" .
else
  echo "-- Skipping build step (--skip-build)."
fi

for MODE in "${MODES[@]}"; do
  if [[ "$MODE" == "k3s" ]]; then
    TAR_FILE="${IMAGE_NAME}.tar"
    echo "-- Command: rm -f \"${TAR_FILE}\""
    rm -f "${TAR_FILE}"

    echo "-- Command: buildah push \"${IMAGE_TAG}\" \"docker-archive:./${TAR_FILE}\""
    buildah push "${IMAGE_TAG}" "docker-archive:./${TAR_FILE}"

    echo "-- Command: sudo k3s ctr images import \"${TAR_FILE}\""
    sudo k3s ctr images import "${TAR_FILE}"
  elif [[ "$MODE" == "push" ]]; then
    GHCR_TAG="ghcr.io/unimib-datai/dfaas-${IMAGE_NAME}:dev"
    echo "-- Command: buildah tag \"${IMAGE_TAG}\" \"${GHCR_TAG}\""
    buildah tag "${IMAGE_TAG}" "${GHCR_TAG}"

    echo "-- Command: buildah push \"${GHCR_TAG}\""
    buildah push "${GHCR_TAG}"
  elif [[ "$MODE" == "tag" ]]; then
    echo "-- Only building and tagging image, no further action."
  else
    echo "Unknown mode: ${MODE}"
    echo "Valid modes are: k3s, push, tag"
    exit 1
  fi
done

echo "-- Done!"
