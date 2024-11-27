#!/bin/bash

MAX_TRIES=25
MAX_TRIES_DEPLOY=10
MAX_TRIES_DELETE=10
DEPLOY_FAILED=0
NODES_DOWN=0

IFS=: read -r -a nodes <<< "$NODES"
IFS=: read -r -a funcs_names <<< "$FUNCS_NAMES"
IFS=: read -r -a funcs_images <<< "$FUNCS_IMAGES"
IFS=: read -r -a funcs_limits <<< "$FUNCS_LIMITS"
IFS=: read -r -a attacks_names <<< "$ATTACKS_NAME"
IFS=: read -r -a delays <<< "$ATTACKS_DELAY"
IFS=: read -r -a targets <<< "$ATTACKS_TARGET"
IFS=: read -r -a methods <<< "$ATTACKS_METHOD"
IFS=: read -r -a bodies <<< "$ATTACKS_BODY"
IFS=: read -r -a rates <<< "$ATTACKS_RATE"
IFS=: read -r -a durations <<< "$ATTACKS_DURATION"

# Function to perform a Vegeta attack
vegeta_attack () {
  mkdir -p $VEGFOLDER/"$1"
  sleep "$2"

  jq -ncM "{method: \"$4\", url: \"http://$3\", body: \"$5\" | @base64, header: {\"Content-Type\": [\"text/plain\"]}}" | \
  vegeta attack -name="$1" -duration=""$7"m" -rate="$6" -format=json | \
  tee $VEGFOLDER/"$1"/results.bin | \
  vegeta report -every=200ms

  # Plot attack's results
  cat $VEGFOLDER/"$1"/results.bin | vegeta report -output $VEGFOLDER/"$1"/report.txt
  cat $VEGFOLDER/"$1"/results.bin | vegeta encode > $VEGFOLDER/"$1"/results.json
  cat $VEGFOLDER/"$1"/results.bin | vegeta plot > $VEGFOLDER/"$1"/plot.html
  /plot-results.py $VEGFOLDER/"$1"/results.json $VEGFOLDER/"$1" "$6" False
}


# Perform health check on each node
for j in "${!nodes[@]}"
do
  echo -e "Checking if node ${nodes[$j]} is healthy...\n";
  HEALTHZ_ENDPOINT="http://${nodes[$j]}/healthz"
  TRIES=1

  until [[ "$(curl -s -w '%{http_code}' -o /dev/null ${HEALTHZ_ENDPOINT})" -eq 200 || $TRIES -eq $MAX_TRIES ]]
  do
    echo -e "Node ${nodes[$j]} not ready yet.\nRetrying health check in 10 seconds...\n";
    sleep 10;
    ((TRIES+=1));
  done

  if [[ $TRIES -eq $MAX_TRIES ]]
  then
    echo -e "Node ${nodes[$j]} is down.\n\n";
    ((NODES_DOWN+=1));
  fi
done

# Proceeds only if all nodes are healthy
if [[ $NODES_DOWN -eq 0 ]]
then
  if [[ $DEPLOY_MODE == "true" ]]
  then
    LIMITS_INDEX=0
    for j in "${!nodes[@]}"
    do
      echo -e "Deploying functions to node ${nodes[$j]}...\n";

      for i in "${!funcs_names[@]}"
      do
        FUNCS_ENDPOINT="http://${OPENFAAS_GATEWAY_USER}:${OPENFAAS_GATEWAY_PWD}@${nodes[$j]}/system/functions"
        TRIES_DEPLOY=0
        echo -e "Deploying function "${funcs_names[$i]}"...\n";

        if [[ $DEPLOY_MAXRATES == "true" ]]
        then
          DEPLOY_REQ_BODY="{\"service\":\""${funcs_names[$i]}"\",\"image\":\""${funcs_images[$i]}"\",\"labels\":{\"dfaas.maxrate\":\""${funcs_limits[$LIMITS_INDEX]}"\"}}"
          ((LIMITS_INDEX+=1));
        else
          DEPLOY_REQ_BODY="{\"service\":\""${funcs_names[$i]}"\",\"image\":\""${funcs_images[$i]}"\"}"
        fi

        until [[ "$(curl -s -w '%{http_code}' -o /dev/null -H "Content-Type: application/json" -X POST -d "${DEPLOY_REQ_BODY}" "${FUNCS_ENDPOINT}")" -eq 200 || $TRIES_DEPLOY -eq $MAX_TRIES_DEPLOY ]]
        do
          echo -e "Failed to deploy function "${funcs_names[$i]}" to node ${nodes[$j]}. Retrying in 10 seconds...\n";
          sleep 10;
          ((TRIES_DEPLOY+=1));
        done

        if [[ $TRIES_DEPLOY -eq $MAX_TRIES_DEPLOY ]]
        then
            echo -e "Failed to deploy function "${funcs_names[$i]}" to node ${nodes[$j]}.\n";
            ((DEPLOY_FAILED+=1));
        else
          echo -e "Function "${funcs_names[$i]}" successfully deployed to node ${nodes[$j]}.\n";
        fi
      done
    done
  fi

  # Test phase
  if [[ $TEST_MODE == "true" ]]
  then
    # Start test phase only if no errors occurred during functions' deploy
    if [[ $DEPLOY_FAILED -eq 0 ]]
    then
      # Wait for the proxy to reconfigure if new functions were deployed
      if [[ $DEPLOY_MODE == "true" ]]
      then
        echo -e "Wait for the proxy to reconfigure after deploying function for "$WAIT_TIME" seconds... ";
        sleep $WAIT_TIME;
      fi

      # Execute Vegeta attacks
      export VEGFOLDER="/tests/vegeta-results/$(date +%Y-%m-%d-%H%M%S)"
      mkdir -p $VEGFOLDER

      for i in "${!attacks_names[@]}"
      do
        echo -e "Attack "${attacks_names[$i]}" will start in "${delays[$i]}" seconds.\n";
        vegeta_attack "${attacks_names[$i]}" "${delays[$i]}" "${targets[$i]}" "${methods[$i]}" "${bodies[$i]}" "${rates[$i]}" "${durations[$i]}" &
      done

      wait
      echo -e "All attacks completed.";
      
      # Aggregate attack's results
      echo -e "Merging attacks results...\n";
      RESULTS_FILES=""

      for i in "${!attacks_names[@]}"
      do
        RESULTS_FILES="${RESULTS_FILES} $VEGFOLDER/"${attacks_names[$i]}"/results.json"
      done

      mkdir -p $VEGFOLDER/merged-results
      cat $RESULTS_FILES > $VEGFOLDER/merged-results/merged-results.json

      /plot-results.py $VEGFOLDER/merged-results/merged-results.json $VEGFOLDER/merged-results 0 True

      echo -e "Results merged successfully.";
    else
      echo -e "Cannot run tests. Failed to deploy ${DEPLOY_FAILED} functions."
    fi

    # Delete deployed functions
    if [[ $DELETE_FUNCTIONS == "true" ]]
    then
      echo -e "Deleting functions from nodes...\n";

      for k in "${!funcs_names[@]}"
      do
        for node in "${nodes[@]}"
        do
          echo -e "Deleting function "${funcs_names[$k]}" from node "$node"...\n";
          TRIES_DELETE=0
          FUNCS_ENDPOINT="http://${OPENFAAS_GATEWAY_USER}:${OPENFAAS_GATEWAY_PWD}@${node}/system/functions"
          
          until [[ "$(curl -s -w '%{http_code}' -o /dev/null -H "Content-Type: application/json" -X DELETE -d "{\"functionName\":\""${funcs_names[$k]}"\"}" "${FUNCS_ENDPOINT}")" -eq 200 || $TRIES_DELETE -eq $MAX_TRIES_DELETE ]]
          do
            echo -e "Failed to delete function "${funcs_names[$k]}" from node "${node}". Retrying in 10 seconds...\n";
            sleep 10;
            ((TRIES_DELETE+=1));
          done
          
          if [[ $TRIES_DELETE -eq $MAX_TRIES_DELETE ]]
          then
            echo -e "Failed to delete function "${funcs_names[$k]}" from node "${node}".\n";
          else
            echo -e "Function "${funcs_names[$k]}" successfully deleted from node "${node}".\n";
          fi
        done
      done
    fi
  fi
fi

exit 0;
