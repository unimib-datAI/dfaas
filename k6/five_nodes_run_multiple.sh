#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:
set -euo pipefail

SCRIPT="five_nodes_run_single.sh"

export FUNCTION_NAME="mlimage"
export STAGE_BUILDER="OneMinuteWindow"


# ----
# 0 RL
# ----

# Since there are no RL agents we can use the trace in the deduplicated version.
export TRACE_PATH="/home/emanuele/dfaas/data/input_requests/mlimage/rlstrategy/fixed_high_load/high_a_b_c_f_g_deduplicated.json"

./setup_dfaas_agents.sh --main c --al a --al b --al c --al f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/0_rl"
./"$SCRIPT"

# Exit here to check.
exit 0

sleep 10m
echo
echo
echo

# ----
# 1 RL
# ----
#
export TRACE_PATH="/home/emanuele/dfaas/data/input_requests/mlimage/rlstrategy/fixed_high_load/high_a_b_c_f_g.json"

# RL node: A
./setup_dfaas_agents.sh --main c --rl a --al b --al c --al f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/1_rl/a"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL node: B
./setup_dfaas_agents.sh --main c --al a --rl b --al c --al f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/1_rl/b"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL node: C
./setup_dfaas_agents.sh --main c --al a --al b --rl c --al f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/1_rl/c"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL node: F
./setup_dfaas_agents.sh --main c --al a --al b --al c --rl f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/1_rl/f"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL node: G
./setup_dfaas_agents.sh --main c --al a --al b --al c --al f --rl g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/1_rl/g"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# ----
# 2 RL
# ----

# RL nodes: A, B
./setup_dfaas_agents.sh --main c --rl a --rl b --al c --al f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/2_rl/a_b"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL nodes: C, F
./setup_dfaas_agents.sh --main c --al a --al b --rl c --rl f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/2_rl/c_f"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL nodes: G, F
./setup_dfaas_agents.sh --main c --al a --al b --al c --rl f --rl g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/2_rl/f_g"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# ----
# 3 RL
# ----

# RL nodes: A, B, C
./setup_dfaas_agents.sh --main c --rl a --rl b --rl c --al f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/3_rl/a_b_c"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL nodes: A, C, F
./setup_dfaas_agents.sh --main c --rl a --al b --rl c --rl f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/3_rl/a_c_f"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL nodes: C, F, G
./setup_dfaas_agents.sh --main c --al a --al b --rl c --rl f --rl g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/3_rl/c_f_g"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# ----
# 4 RL
# ----

# RL nodes: A, B, C, F
./setup_dfaas_agents.sh --main c --rl a --rl b --rl c --rl f --al g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/4_rl/a_b_c_f"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL nodes: A, B, F, G
./setup_dfaas_agents.sh --main c --rl a --rl b --al c --rl f --rl g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/4_rl/a_b_f_g"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# RL nodes: B, C, F, G
./setup_dfaas_agents.sh --main c --al a --rl b --rl c --rl f --rl g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/4_rl/b_c_f_g"
./"$SCRIPT"

sleep 10m
echo
echo
echo

# ----
# 5 RL
# ----

# RL nodes: A, B, C, F, G
./setup_dfaas_agents.sh --main c --rl a --rl b --rl c --rl f --rl g

export OUTPUT_BASE_DIR="/home/emanuele/dfaas/data/rl_paper/v3/5_rl"
./"$SCRIPT"
