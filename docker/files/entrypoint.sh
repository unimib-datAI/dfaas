#!/bin/sh
# Generate dfaasagent.env file with all the env vars prefixed with `AGENT_`.
# This file is used by the systemd unit dfaasagent.service.
env | grep ^AGENT_ > /agent/dfaasagent.env

/sbin/init --log-level=err

k3d cluster create --k3s-arg '--kubelet-arg=feature-gates=KubeletInUserNamespace=true@server:*' single-node-cluster
sleep 10

arkade install openfaas

kubectl port-forward -n openfaas svc/gateway 8080:8080 &
kubectl port-forward -n openfaas svc/prometheus 9090:9090 &

PASSWORD=$(kubectl get secret -n openfaas basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode; echo)
echo -n $PASSWORD | faas-cli login --username admin --password-stdin

exec "@"