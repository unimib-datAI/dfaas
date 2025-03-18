# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

# FIXME: Make the paths configurable.
MODELS_BASE_PATH = "/opt/forecaster/models/"
SCALERS_BASE_PATH = "/opt/forecaster/scalers/"
GROUPS_COLUMNS_NAMES = ["rate_group_HIGH_USAGE", "rate_group_LOW_USAGE", "rate_group_HIGH_USAGE"]
CPU_USAGE_METRIC = "cpu_usage_node"
RAM_USAGE_METRIC = "ram_usage_node"
POWER_USAGE_METRIC = "power_usage_node"
METRICS = [CPU_USAGE_METRIC, RAM_USAGE_METRIC, POWER_USAGE_METRIC]
