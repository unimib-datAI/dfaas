#!/bin/sh

k3d cluster create --k3s-arg '--kubelet-arg=feature-gates=KubeletInUserNamespace=true@server:*' single-node-cluster
sleep 10

arkade install openfaas

kubectl port-forward -n openfaas svc/gateway 8080:8080 &
kubectl port-forward -n openfaas svc/prometheus 9090:9090 &

PASSWORD=$(kubectl get secret -n openfaas basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode; echo)
echo -n $PASSWORD | faas-cli login --username admin --password-stdin