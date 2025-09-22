#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# This is the DFaaS Operator script. It runs Vegeta to perform load testing on
# DFaaS nodes, and then invokes a Python script to generate plots.
#
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

MAX_TRIES=25
NODES_DOWN=0

IFS=';' read -r -a nodes <<< "$NODES"
IFS=';' read -r -a funcs_names <<< "$FUNCS_NAMES"
IFS=';' read -r -a funcs_images <<< "$FUNCS_IMAGES"
IFS=';' read -r -a funcs_limits <<< "$FUNCS_LIMITS"
IFS=';' read -r -a attacks_names <<< "$ATTACKS_NAME"
IFS=';' read -r -a delays <<< "$ATTACKS_DELAY"
IFS=';' read -r -a targets <<< "$ATTACKS_TARGET"
IFS=';' read -r -a methods <<< "$ATTACKS_METHOD"
IFS=';' read -r -a bodies <<< "$ATTACKS_BODY"
IFS=';' read -r -a rates <<< "$ATTACKS_RATE"
IFS=';' read -r -a durations <<< "$ATTACKS_DURATION"

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


# Perform health check on each node.
for node in "${nodes[@]}"; do
  echo -e "Checking if node $node is healthy...\n"
  HEALTHZ_ENDPOINT="http://$node/healthz"
  TRIES=0

  while [[ $TRIES -lt $MAX_TRIES ]]; do
    STATUS_CODE=$(curl --max-time 10 -s -w '%{http_code}' -o /dev/null "$HEALTHZ_ENDPOINT")
    if [[ "$STATUS_CODE" -eq 200 ]]; then
      echo "Node $node is ready (status: $STATUS_CODE)"
      break
    else
      echo -e "Node $node not ready yet (status: $STATUS_CODE).\nRetrying health check in 10 seconds...\n"
      sleep 10
      ((TRIES++))
    fi
  done

  if [[ "$STATUS_CODE" -ne 200 ]]; then
    echo "Node $node did not become ready after $MAX_TRIES tries (last status: $STATUS_CODE)."
    ((NODES_DOWN++))
  fi
done

# Proceeds only if all nodes are healthy.
if (( NODES_DOWN == 0 )); then
  VEGFOLDER="/tests/vegeta-results/$(date +%Y-%m-%d-%H%M%S)"
  mkdir -p "$VEGFOLDER"

  for i in "${!attacks_names[@]}"; do
    echo -e "Attack ${attacks_names[i]} will start in ${delays[i]} seconds.\n"
    vegeta_attack "${attacks_names[i]}" "${delays[i]}" "${targets[i]}" "${methods[i]}" "${bodies[i]}" "${rates[i]}" "${durations[i]}" &
  done

  wait
  echo -e "All attacks completed."

  echo -e "Merging attacks results...\n"
  results_files=()
  for name in "${attacks_names[@]}"; do
    results_files+=("$VEGFOLDER/$name/results.json")
  done

  mkdir -p "$VEGFOLDER/merged-results"
  cat "${results_files[@]}" > "$VEGFOLDER/merged-results/merged-results.json"

  /plot-results.py "$VEGFOLDER/merged-results/merged-results.json" "$VEGFOLDER/merged-results" 0 True

  echo -e "Results merged successfully."
fi

exit 0
