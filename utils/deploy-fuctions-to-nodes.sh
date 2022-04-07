#!/bin/bash

set -e
NODES=$1
for i in {1..$NODES} ; do
  CONTAINER_NAME="dfaas-node-$i-1"
  docker exec -it ${CONTAINER_NAME} ./deploy_functions.sh # it may requires few minutes due to functions images download
done

for i in {1..$NODES} ; do
  CONTAINER_NAME="dfaas-node-$i-1"
  docker exec -it ${CONTAINER_NAME} "systemctl restart dfaasagent"
done