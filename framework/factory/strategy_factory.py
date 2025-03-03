# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from behaviour.equal_strategy import EqualStrategy
from behaviour.base_strategy import BaseStrategy
from behaviour.node_margin_strategy import NodeMarginStrategy
from behaviour.power_saving_strategy import PowerSavingStrategy
from configuration.config_manager import ConfigManager


class StrategyFactory():
    @staticmethod
    def create_strategy(strategy_type, node_key, final_config):
        """
        Create strategy based on type passed as first parameter
        """
        # Important: the list of neighbours is supposed to be already present in a node in the reality
        # So it is not a message to forward between the nodes
        
        config_manager = ConfigManager()
        if strategy_type == config_manager.BASE_STRATEGY:
            # It only needs its configuration and a list of neighbours
            return BaseStrategy(final_config[node_key])
        
        if strategy_type == config_manager.EQUAL_STRATEGY:
            # It only needs its configuration and a list of neighbours
            return EqualStrategy(final_config[node_key])
        
        if strategy_type == config_manager.NODE_MARGIN_STRATEGY:
            # It needs the configuration of all the nodes in order to calculate phase 1 correctly
            return NodeMarginStrategy(final_config)
        
        if strategy_type == config_manager.POWER_SAVING_STRATEGY:
            # It needs the configuration of all the nodes in order to calculate phase 1 correctly
            return PowerSavingStrategy(final_config)
