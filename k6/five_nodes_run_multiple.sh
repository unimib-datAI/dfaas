#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
set -euo pipefail

SCRIPT="five_nodes_run_single.sh"

export FUNCTION_NAME="mlimage"
export STAGE_BUILDER="OneMinuteWindow"

# Same trace for ALL experiments.
export TRACE_PATH="/home/emanuele/dfaas/data/input_requests/mlimage/rlstrategy/fixed_high_load/high_a_b_c_f_g_deduplicated.json"

ALL_NODES="dfaas-node-a,dfaas-node-b,dfaas-node-c,dfaas-node-f,dfaas-node-g"
RL_NODES="dfaas-node-a,dfaas-node-b,dfaas-node-c"
RECALC_NODES="dfaas-node-f,dfaas-node-g"
MAIN_NODE="dfaas-node-c"

# Set up functions.
./stop_dfaas_agents.sh --nodes $ALL_NODES
./update_maxrate.sh --nodes $ALL_NODES --max-rate-rps 15

# EXP

./start_dfaas_agents.py --main-node $MAIN_NODE --strategy-recalc $RECALC_NODES --strategy-rl $RL_NODES

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/hybrid/v1"
./"$SCRIPT"
