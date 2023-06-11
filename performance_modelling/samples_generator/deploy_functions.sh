#!/bin/bash

declare HEALTHZ_ENDPOINT="http://192.168.49.2:31112/healthz"
declare MAX_TRIES=20
declare TRIES=1
maxrate=$1; shift
functions=("$@")

until [[ "$(curl -s -w '%{http_code}' -o /dev/null ${HEALTHZ_ENDPOINT})" -eq 200 || $TRIES -eq $MAX_TRIES ]]
do
  sleep 10;
  ((TRIES+=1));
done

if [[ $TRIES -eq $MAX_TRIES ]]; then
    exit 1;
fi

faas-cli login --password q037IuiRL4P0VVW8M4PW93j4O --gateway http://192.168.49.2:31112

for function in ${functions[@]}
do
  faas-cli store deploy $function --gateway http://192.168.49.2:31112
done

exit 0;