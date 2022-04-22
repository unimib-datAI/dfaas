#!/bin/bash

declare HEALTHZ_ENDPOINT="http://localhost:8080/healthz"
declare MAX_TRIES=20
declare TRIES=1

until [[ "$(curl -s -w '%{http_code}' -o /dev/null ${HEALTHZ_ENDPOINT})" -eq 200 || $TRIES -eq $MAX_TRIES ]]
do
  sleep 10;
  ((TRIES+=1));
done

if [[ $TRIES -eq $MAX_TRIES ]]; then
    exit 1;
fi

faas-cli login --password admin

faas-cli store deploy ocr --label dfaas.maxrate=10
faas-cli store deploy shasum --label dfaas.maxrate=20
faas-cli store deploy figlet --label dfaas.maxrate=30

exit 0;