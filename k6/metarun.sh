#!/usr/bin/env bash

# Exit on error.
set -euo pipefail

# Make sure the working directory is where this script is located!
cd "$(dirname "$0")"

EXP_NAME=input_trace_sinusoidal_78_mlimage_scaled TRACE_PATH=input_trace_sinusoidal_78_mlimage_scaled.json ./run.sh
echo
echo
echo "Waiting 5 minutes..."
sleep 300
echo "Waiting done!"
echo

EXP_NAME=input_trace_sinusoidal_78_mlimage_scaled_pwr_75 TRACE_PATH=input_trace_sinusoidal_78_mlimage_scaled_pwr_75.json ./run.sh
echo
echo
echo "Waiting 5 minutes..."
sleep 300
echo "Waiting done!"
echo

EXP_NAME=input_trace_sinusoidal_78_mlimage_scaled_pwr_50 TRACE_PATH=input_trace_sinusoidal_78_mlimage_scaled_pwr_50.json ./run.sh
echo
echo
echo "Waiting 5 minutes..."
sleep 300
echo "Waiting done!"
echo

EXP_NAME=input_trace_sinusoidal_78_mlimage_scaled_pwr_25 TRACE_PATH=input_trace_sinusoidal_78_mlimage_scaled_pwr_25.json ./run.sh
echo
echo
echo "Waiting 5 minutes..."
sleep 300
echo "Waiting done!"
echo
