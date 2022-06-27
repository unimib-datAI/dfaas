#!/bin/bash

k3d cluster create --k3s-arg '--kubelet-arg=feature-gates=KubeletInUserNamespace=true@server:*' single-node-cluster
sleep 10

kubectl apply -f https://raw.githubusercontent.com/openfaas/faas-netes/master/namespaces.yml

helm repo add openfaas https://openfaas.github.io/faas-netes/
helm repo update \
 && helm upgrade openfaas --install openfaas/openfaas \
    --namespace openfaas  \
    --set functionNamespace=openfaas-fn \
    --set generateBasicAuth=true

kubectl port-forward -n openfaas svc/gateway 8080:8080 &
kubectl port-forward -n openfaas svc/prometheus 9090:9090 &

PASSWORD=$(kubectl get secret -n openfaas basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode; echo)

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

echo -n $PASSWORD | faas-cli login --username admin --password-stdin