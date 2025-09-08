#!/usr/bin/env bash
#
# Small utility to deploy three OpenFaaS functions to the local k3s cluster.
set -euo pipefail

# Read credentials from the 'basic-auth' Kubernetes secret.
PASSWORD=$(sudo kubectl get secret basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode)

# Login using faas-cli.
echo "faas-cli login --password $PASSWORD"
faas-cli login --password "$PASSWORD"
echo

# Iterate over the default functions.
FUNCTIONS=(figlet shasum ocr)
for func in "${FUNCTIONS[@]}"; do
    # Check if it is already deployed.
    if [[ $(faas-cli list --quiet | grep -x "$func") ]]; then
        echo "Function '$func' is already deployed. Skipping."
        continue
    fi

    # Deploy function.
    echo "faas-cli store deploy '$func' --label dfaas.maxrate=100"
    faas-cli store deploy "$func" --label dfaas.maxrate=100
done
