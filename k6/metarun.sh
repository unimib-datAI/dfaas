#!/usr/bin/env bash

# Exit on error.
set -euo pipefail

# Make sure the working directory is where this script is located!
cd "$(dirname "$0")"

EXP_NAME=auto_scaled TRACE_PATH=input_requests_scaled_traces.json ./run.sh
echo
echo

EXP_NAME=auto_scaled_power_2 TRACE_PATH=input_requests_scaled_power_2_traces.json ./run.sh
echo
echo

EXP_NAME=auto_scaled_power_1 TRACE_PATH=input_requests_scaled_power_1_traces.json ./run.sh
echo
echo
