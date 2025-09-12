#!/usr/bin/env bash
#
# Utility to build DFaaS Agent and DFaaS Forecaster Docker images.
#
# Must be run from the project's root directory!

# Exit on error.
set -e

# Help message.
if [[ -z "$1" || -z "$2" ]]; then
  echo "Usage: $0 <image_name> [<mode> ...] [--dockerfile <path>]"
  echo ""
  echo "Arguments:"
  echo "  <image_name>   The image name (e.g., agent, forecaster)."
  echo "  <mode>         'k3s' to build and import into local k3s,"
  echo "                 'push' to build and push to GHCR remote registry,"
  echo "                 'none' to only build the image."
  echo "  --dockerfile   Optional flag to specify the Dockerfile path. Useful"
  echo "                 only for the operator component"
  echo ""
  echo "The image will be automatically tagged with 'dev'. Accepts multiple"
  echo "modes (eg. 'push k3s')."
  echo ""
  echo "If image is operator, be sure to specify Dockerfile and to not use k3s"
  echo "mode, only push."
  exit 1
fi

# Exit on unknown used variables.
set -u

IMAGE_NAME=""
DOCKERFILE=""
MODES=()

# Parse arguments, looking for --dockerfile
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dockerfile)
      shift
      if [[ -z "$1" ]]; then
        echo "Error: --dockerfile requires a path argument."
        exit 3
      fi
      DOCKERFILE="$1"
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

echo "-- Building image ${IMAGE_TAG} from ${DOCKERFILE}..."
echo "-- Command: buildah build -f \"${DOCKERFILE}\" -t \"${IMAGE_TAG}\" ."
buildah build -f "${DOCKERFILE}" -t "${IMAGE_TAG}" .

for MODE in "${MODES[@]}"; do
  if [[ "$MODE" == "k3s" ]]; then
    TAR_FILE="${IMAGE_NAME}.tar"
    echo "-- Removing ${TAR_FILE} if it exists..."
    echo "-- Command: rm -f \"${TAR_FILE}\""
    rm -f "${TAR_FILE}"

    echo "-- Pushing image ${IMAGE_TAG} to docker-archive:${TAR_FILE}..."
    echo "-- Command: buildah push \"${IMAGE_TAG}\" \"docker-archive:./${TAR_FILE}\""
    buildah push "${IMAGE_TAG}" "docker-archive:./${TAR_FILE}"

    echo "-- Importing ${TAR_FILE} into k3s container runtime..."
    echo "-- Command: sudo k3s ctr images import \"${TAR_FILE}\""
    sudo k3s ctr images import "${TAR_FILE}"
  elif [[ "$MODE" == "push" ]]; then
    GHCR_TAG="ghcr.io/unimib-datai/dfaas-${IMAGE_NAME}:dev"
    echo "-- Tagging image as ${GHCR_TAG}..."
    echo "-- Command: buildah tag \"${IMAGE_TAG}\" \"${GHCR_TAG}\""
    buildah tag "${IMAGE_TAG}" "${GHCR_TAG}"

    echo "-- Pushing image to remote registry: ${GHCR_TAG}..."
    echo "-- Command: buildah push \"${GHCR_TAG}\""
    buildah push "${GHCR_TAG}"
  elif [[ "$MODE" == "none" ]]; then
    echo "-- Only building image, no further action for mode 'none'."
  else
    echo "Unknown mode: ${MODE}"
    echo "Valid modes are: k3s, push, none"
    exit 2
  fi
done

echo "-- Done!"
