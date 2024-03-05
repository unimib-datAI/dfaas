#!/bin/bash

declare HEALTHZ_ENDPOINT="http://192.168.49.2:31112/healthz"
declare MAX_TRIES=20
declare TRIES=1
declare OPENFAAS_SERVICE_IP="http://192.168.49.2:31112"
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
password_command="kubectl get secret -n openfaas basic-auth -o jsonpath={.data.basic-auth-password} | base64 --decode"
password=$(eval $password_command)

# Use the obtained password to log in using faas-cli
faas_cli_command="echo -n $password | faas-cli login --username admin --password-stdin --gateway $OPENFAAS_SERVICE_IP"
eval $faas_cli_command

for function in ${functions[@]}
do
if [[ "$function" == *"/"* ]]; then
    faas-cli deploy --image $function --name ${function##*/} --gateway ${OPENFAAS_SERVICE_IP}
else
    faas-cli store deploy $function --gateway ${OPENFAAS_SERVICE_IP}
fi
done


exit 0;
