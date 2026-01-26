#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
set -euo pipefail

# Show help message if requested.
if [[ $# -gt 0 && ( "$1" == "-h" || "$1" == "--help" ) ]]; then
    echo "Usage: $0 [FUNCTION ...]"
    echo
    echo "Deploy basic OpenFaaS functions to the local k3s cluster."
    echo
    echo "Options:"
    echo "  -h, --help    Show this help message and exit."
    echo
    echo "Arguments:"
    echo "  FUNCTION      List of functions to deploy. If none are provided,"
    echo "                defaults (figlet, shasum, ocr) will be used."
    echo
    echo "Warning: the label 'dfaas.maxrate=100' is added to all deployed"
    echo "functions."
    exit 0
fi

# Get function from CLI or defaults.
if [[ $# -eq 0 ]]; then
    FUNCTIONS=(figlet shasum ocr)
else
    FUNCTIONS=("$@")
fi

# Iterate over the default functions.
for func in "${FUNCTIONS[@]}"; do
    # Check if it is already deployed.
    if [[ $(faas-cli list --quiet | grep -x "$func") ]]; then
        echo "Function '$func' is already deployed. Skipping."
        continue
    fi

    # Deploy function.
    #
    # Note: the custom label is used by the DFaaS Agent with the Recalc
    # Strategy. It is the max number of requests per second this function can
    # handle.
    echo "faas-cli store deploy '$func' --label dfaas.maxrate=100"
    faas-cli store deploy "$func" --label dfaas.maxrate=100
done
