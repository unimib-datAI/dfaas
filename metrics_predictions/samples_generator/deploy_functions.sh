#!/bin/bash

declare HEALTHZ_ENDPOINT="http://10.12.38.4:31112/healthz"
declare MAX_TRIES=20
declare TRIES=1
declare OPENFAAS_SERVICE_IP="http://10.12.38.4:31112"
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

# Execute kubectl command to get the OpenFaaS basic-auth-password from the secret
password_command="kubectl --context=mid get secret -n openfaas basic-auth -o jsonpath={.data.basic-auth-password} | base64 --decode" # NB CONTEXT NEEDS TO BE ADAPTED BASED ON THE RECEIVER NODE (CHECK THE GENERATOR CONFIG FILE)
password=$(eval $password_command)

# Use the obtained password to log in using faas-cli
faas_cli_command="echo -n $password | faas-cli login --username admin --password-stdin --gateway $OPENFAAS_SERVICE_IP"
eval $faas_cli_command

for function in "${functions[@]}"
do
    if [[ "$function" == "openfaas-youtube-dl" ]]; then
        faas-cli deploy --image ghcr.io/ema-pe/openfaas-youtube-dl --name openfaas-youtube-dl --gateway "${OPENFAAS_SERVICE_IP}"
    elif [[ "$function" == "openfaas-text-to-speech" ]]; then
        faas-cli deploy --image ghcr.io/ema-pe/openfaas-text-to-speech --name openfaas-text-to-speech --gateway "${OPENFAAS_SERVICE_IP}"
    else
        faas-cli store deploy "$function" --gateway "${OPENFAAS_SERVICE_IP}"
    fi
done

exit 0;
