#!/bin/bash

set -e  # Exit immediately on error
trap 'echo "[ERROR] Script failed at line $LINENO: $BASH_COMMAND"' ERR

# Install buildah via apt.
sudo apt install buildah

# Install K3S
# See: https://docs.k3s.io/installation
sudo ufw disable
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -

# Build local images and push to k3s.
./k8s/scripts/build-image.sh agent
./k8s/scripts/build-image.sh forecaster

# Install Helm
# See: https://helm.sh/docs/intro/install/#from-apt-debianubuntu
sudo apt-get install curl gpg apt-transport-https --yes
curl -fsSL https://packages.buildkite.com/helm-linux/helm-debian/gpgkey | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg > /dev/null
echo "deb [signed-by=/usr/share/keyrings/helm.gpg] https://packages.buildkite.com/helm-linux/helm-debian/any/ any main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update
sudo apt-get install helm

# Append KUBECONFIG export to .bashrc if not already present. Needed by Helm
if ! grep -q 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml' ~/.bashrc; then
  echo 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml' >> ~/.bashrc
fi

# Preserve KUBECONFIG when using sudo (needed by Helm).
sudo sh -c 'echo "# Preserve KUBECONFIG env variable.
Defaults:%sudo env_keep += \"KUBECONFIG\"
Defaults !always_set_home" > /etc/sudoers.d/50-helm'
sudo chmod 0440 /etc/sudoers.d/50-helm

# Reload .bashrc for current shell
source ~/.bashrc

# Install faas-cli
# Download faas-cli to ~/.local/bin
mkdir -p ~/.local/bin
curl -sSL https://github.com/openfaas/faas-cli/releases/download/0.17.8/faas-cli -o ~/.local/bin/faas-cli
chmod +x ~/.local/bin/faas-cli
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Install HAProxy on K3S.
sudo helm repo add haproxytech https://haproxytech.github.io/helm-charts
sudo helm repo update
sudo helm install haproxy haproxytech/haproxy --version 1.25.0 --values k8s/charts/values-haproxy.yaml

# Install Prometheus (plus node-exporter and alertmanager) on K3S.
sudo helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
sudo helm repo update
sudo helm install prometheus prometheus-community/prometheus --version 27.37.0 --values k8s/charts/values-prometheus.yaml

# Install OpenFaaS on K3S and deploy functions.
sudo helm repo add openfaas https://openfaas.github.io/faas-netes/ # OpenFaaS
sudo helm repo update
sudo helm install openfaas openfaas/openfaas --version 14.2.124 --values ./k8s/charts/values-openfaas.yaml

OPENFAAS_PWD="$(sudo kubectl get secret basic-auth -o jsonpath={.data.basic-auth-password} | base64 --decode)"

export OPENFAAS_URL=http://127.0.0.1:31112
if ! grep -q 'export OPENFAAS_URL="http://127.0.0.1:31112"' ~/.bashrc; then
  echo 'export "OPENFAAS_URL=http://127.0.0.1:31112"' >> ~/.bashrc
  source ~/.bashrc
fi

faas-login --password "$OPENFAAS_PWD"

./k8s/scripts/deploy_functions.sh

# Install Forecaster in K3S.
sudo helm install forecaster ./k8s/charts/forecaster
sudo helm install dfaas-extra-setup ./k8s/charts/extra-setup/

# Do not install DFaaS Agent, this is manual.
