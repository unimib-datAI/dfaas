#!/bin/bash

set -e

NODES=$1
NODE_PREFIX=$2
NODE_SUFFIX=$3

for ((i=1;i<=NODES;i++)); do
  CONTAINER_NAME="$NODE_PREFIX$i$NODE_SUFFIX"
  echo "Deploying functions to $CONTAINER_NAME..."
  docker exec -it ${CONTAINER_NAME} ./deploy_functions.sh # it may requires few minutes due to functions images download
done

for ((i=1;i<=NODES;i++)); do
  CONTAINER_NAME="$NODE_PREFIX$i$NODE_SUFFIX"
  echo "Restarting $CONTAINER_NAME..."
  docker exec -it ${CONTAINER_NAME} "systemctl restart dfaasagent"
done