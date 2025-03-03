# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from behaviour.empirical_strategy import EmpiricalStrategy
from behaviour.random_strategy import RandomStrategy
from behaviour.base_strategy import BaseStrategy
from behaviour.dfaas_static_strategy import DFaasStaticStrategy
from configuration.config_manager import ConfigManager


class StrategyFactory():
    @staticmethod
    def create_strategy(strategy_type, config_file):
        """
        Create strategy based on type passed as first parameter
        """
        config_manager = ConfigManager()
        if strategy_type == config_manager.BASE_STRATEGY:
            return BaseStrategy(config_file)
        elif strategy_type == config_manager.RANDOM_STRATEGY:
            return RandomStrategy(config_file)
        elif strategy_type == config_manager.EMPIRICAL_STRATEGY:
            return EmpiricalStrategy(config_file)
        elif strategy_type == config_manager.DFAAS_STATIC_STRATEGY:
            return DFaasStaticStrategy(config_file)
        else: # Default behaviour
            return BaseStrategy(config_file)
