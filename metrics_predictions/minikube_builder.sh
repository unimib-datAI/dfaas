#!/usr/bin/env bash

echo "Checking if Minikube is running..."
STATUS=$(minikube status --format='{{.Host}}')
if [ "$STATUS" = "Running" ]; then
  echo A Minikube instance is running.
  echo Killing Minikube instance...
  minikube delete
  if [ $? -eq 0 ]; then
    echo Minikube deleted
  else
    echo Unable to delete Minikube instance
    exit 1
  fi
else
  echo Minikube is not running
fi

echo Building Minikube instance...

minikube start --memory=max --cpus=max  --apiserver-ips=10.12.38.4

if [ $? -eq 0 ]; then
  echo Minikube is up and running
else
  echo Unable to start Minikube
  exit 1
fi

# Loading pre pulled docker images
#
# TODO: Skip this for now.

# Folder containing the .tar files from pull-and-save-images.sh
#IMAGE_DIR=~/openfaas-image-cache

# Check the directory exists
#if [ ! -d "$IMAGE_DIR" ]; then
#  echo " Error: $IMAGE_DIR does not exist. Run the pull script first."
#  exit 1
#fi

#echo "[*] Loading saved images into Minikube from $IMAGE_DIR..."

#for TAR in "$IMAGE_DIR"/*.tar; do
#  echo "-> Loading $TAR into Minikube"
#  minikube image load "$TAR"
#done

#echo "All .tar images successfully loaded into Minikube"

echo Installing openfaas on the Minikube instance...
arkade install openfaas-ce
if [ $? -eq 0 ]; then
  echo Setting up openfaas...
  # From official OpenFaaS docs
  kubectl rollout status -n openfaas deploy/gateway
  kubectl port-forward -n openfaas svc/gateway 8080:8080 >/dev/null 2>&1 &

  # Wait until the gateway is up
  until curl -s http://127.0.0.1:8080 >/dev/null; do
    echo "Waiting for gateway..."
    sleep 2
  done
  echo "OpenFaaS gateway is available at http://127.0.0.1:8080"

  PASSWORD=$(kubectl -n openfaas get secret basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode; echo)
  echo -n $PASSWORD | faas-cli login --username admin --password-stdin --tls-no-verify

  if [ $? -eq 0 ]; then
    echo openfaas configured
  else
    echo Unable to configure openfaas
    exit 1
  fi
else
  echo Unable to install openfaas
  exit 1
fi

# Start building the cluster using the infrastructure dir

echo Building node infrastructure...
kubectl apply -f ~/dfaas/metrics_predictions/infrastructure
if [ $? -eq 0 ]; then
  echo Node has been built succesfully!
else
  echo Unable to build the node
  exit 1
fi

sleep 20

echo "Restating Promtheus pod..."
POD=$(kubectl get pods -n openfaas -l app=prometheus -o jsonpath="{.items[0].metadata.name}")
kubectl delete pod $POD -n openfaas
if [ $? -eq 0 ]; then
  echo Prometheus pod restarted
else
  echo Unable to restart Prometheus pod
  exit 1
fi

echo "Copying find-pid.py script into the Minikube container..."
docker cp ~/dfaas/metrics_predictions/samples_generator/find-pid.py minikube:/etc/

if [ $? -eq 0 ]; then
  echo find-pid.py copied
else
  echo Unable to copy find-pid.py script
  exit 1
fi

echo The node is up and running!
exit 0
