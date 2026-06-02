#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
set -euo pipefail

SCRIPT="five_nodes_run_single.sh"

export TRACE_PATH=../data/input_requests/mlimage/rlstrategy/sinusoidal_78_scaled_pwr_25.json
export OUTPUT_BASE_DIR=../data/20260601_one_rl_agent_sinusoidal
./"$SCRIPT"

sleep 200
echo
echo
echo

export TRACE_PATH=../data/input_requests/mlimage/rlstrategy/scaled_pwr_3.json
export OUTPUT_BASE_DIR=../data/20260601_one_rl_agent_pwr_3
./"$SCRIPT"

sleep 200
echo
echo
echo

export TRACE_PATH=../data/input_requests/mlimage/rlstrategy/scaled_pwr_5.json
export OUTPUT_BASE_DIR=../data/20260601_one_rl_agent_pwr_5
./"$SCRIPT"

sleep 200
echo
echo
echo

export TRACE_PATH=../data/input_requests/mlimage/rlstrategy/scaled_pwr_40_only_0_other_constant_4.json
export OUTPUT_BASE_DIR=../data/20260601_one_rl_agent_pwr_4
./"$SCRIPT"
