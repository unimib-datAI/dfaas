#!/usr/bin/env bash
#
# Small utility to build the DFaaS Agent and DFaaS Forecaster Docker images and
# to push them to the local K3S instance.
#
# Usage:
#   ./build-image.sh <image_name>
#
# Must be run on the project's root directory!

set -e

if [[ -z "$1" ]]; then
  echo "Usage: $0 <image_name>"
  exit 1
fi

IMAGE_NAME="$1"
IMAGE_TAG="${IMAGE_NAME}:dev"
TAR_FILE="${IMAGE_NAME}.tar"
DOCKERFILE="k8s/docker/Dockerfile.${IMAGE_NAME}"

echo "-- Removing ${TAR_FILE} if it exists..."
echo "-- Command: rm -f \"${TAR_FILE}\""
rm -f "${TAR_FILE}"

echo "-- Building image ${IMAGE_TAG} from ${DOCKERFILE}..."
echo "-- Command: buildah build -f \"${DOCKERFILE}\" -t \"${IMAGE_TAG}\" ."
buildah build -f "${DOCKERFILE}" -t "${IMAGE_TAG}" .

echo "-- Pushing image ${IMAGE_TAG} to docker-archive:${TAR_FILE}..."
echo "-- Command: buildah push \"${IMAGE_TAG}\" \"docker-archive:./${TAR_FILE}\""
buildah push "${IMAGE_TAG}" "docker-archive:./${TAR_FILE}"

echo "-- Importing ${TAR_FILE} into k3s container runtime..."
echo "-- Command: sudo k3s ctr images import \"${TAR_FILE}\""
sudo k3s ctr images import "${TAR_FILE}"

echo "-- Done!"
