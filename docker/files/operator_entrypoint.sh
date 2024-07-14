#!/bin/bash

MAX_TRIES=25
MAX_TRIES_DEPLOY=10
DEPLOY_FAILED=0

IFS=: read -r -a mode <<< "$MODE"
IFS=: read -r -a nodes <<< "$NODES"
IFS=: read -r -a funcs_names <<< "$FUNCS_NAMES"
IFS=: read -r -a funcs_images <<< "$FUNCS_IMAGES"

for node in "${nodes[@]}"
do
  echo -e "Checking if node ${node} is healthy...\n";
  HEALTHZ_ENDPOINT="http://${node}/healthz"
  FUNCS_ENDPOINT="http://${OPENFAAS_GATEWAY_USER}:${OPENFAAS_GATEWAY_PWD}@${node}/system/functions"
  TRIES=1

  until [[ "$(curl -s -w '%{http_code}' -o /dev/null ${HEALTHZ_ENDPOINT})" -eq 200 || $TRIES -eq $MAX_TRIES ]]
  do
    echo -e "Node ${node} not ready yet.\nRetrying health check in 10 seconds...\n";
    sleep 10;
    ((TRIES+=1));
  done

  if [[ $TRIES -eq $MAX_TRIES ]]
  then
    echo -e "Node ${node} is down.\n\n";
  else
    echo -e "Deploying functions to node ${node}...\n";

    for i in "${!funcs_names[@]}"
    do
      TRIES_DEPLOY=0
      echo -e "Deploying function "${funcs_names[$i]}"...\n";

      until [[ "$(curl -s -w '%{http_code}' -o /dev/null -H "Content-Type: application/json" -X POST -d "{\"service\":\""${funcs_names[$i]}"\",\"image\":\""${funcs_images[$i]}"\"}" "${FUNCS_ENDPOINT}")" -eq 200 || $TRIES_DEPLOY -eq $MAX_TRIES_DEPLOY ]]
      do
        echo -e "Failed to deploy function "${funcs_names[$i]}" to node ${node}. Retrying in 2 seconds...\n";
        sleep 2;
        ((TRIES_DEPLOY+=1));
      done
      if [[ $TRIES_DEPLOY -eq $MAX_TRIES_DEPLOY ]]
      then
        echo -e "Failed to deploy function "${funcs_names[$i]}" to node ${node}.\n";
        ((DEPLOY_FAILED+=1));
      else
        echo -e "Function "${funcs_names[$i]}" successfully deployed to node ${node}.\n";
      fi
    done
  fi
done

if [[ $mode = "test" ]]
then
  if [[ $DEPLOY_FAILED -eq 0 ]]
  then
    export VEGFOLDER="/vegeta-results/$(date +%Y-%m-%d-%H%M%S)"
    mkdir -p $VEGFOLDER

    jq -ncM '{method: "GET", url: "http://172.16.238.10/function/figlet", body: "Hello DFaaS world!" | @base64, header: {"Content-Type": ["text/plain"]}}' | \
      vegeta attack -duration=3m -rate=600 -format=json | \
      tee $VEGFOLDER/results.bin | \
      vegeta report -every=200ms

    # Encode results as JSON
    cat $VEGFOLDER/results.bin | vegeta encode > $VEGFOLDER/results.json

    # Create plot with vegeta
    cat $VEGFOLDER/results.bin | vegeta plot > $VEGFOLDER/plot.html

    # 1st arg: path int results.json
    # 2nd arg: path output plot
    # 3rd arg: rate req/s used for the attack
    #./utils/plot.py $VEGFOLDER/results.json $VEGFOLDER/plot.png 50
  else
    echo -e "Cannot run tests. Failed to deploy ${DEPLOY_FAILED} functions."
  fi
fi

exit 0;
