#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
set -euo pipefail

SCRIPT="five_nodes_run_single.sh"

export FUNCTION_NAME="sentimentanalysis"

export TRACE_PATH=../data/input_requests/sentimentanalysis/rlstrategy/rate_mean_8_only_0.json
export OUTPUT_BASE_DIR=../data/20260602_sentiment_one_rl_rate_08
./"$SCRIPT"

sleep 200
echo
echo
echo

export TRACE_PATH=../data/input_requests/sentimentanalysis/rlstrategy/rate_mean_10_only_0.json
export OUTPUT_BASE_DIR=../data/20260602_sentiment_one_rl_rate_10
./"$SCRIPT"

sleep 200
echo
echo
echo
