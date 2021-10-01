from behaviour.empirical_strategy import EmpiricalStrategy
from behaviour.random_strategy import RandomStrategy
from behaviour.base_strategy import BaseStrategy
from config_manager import ConfigManager

class StrategyFactory():
    @staticmethod
    def create_strategy(strategy_type, config_file):
        config_manager = ConfigManager()
        if strategy_type == config_manager.BASE_STRATEGY:
            return BaseStrategy("", False, config_file)
        elif strategy_type == config_manager.RANDOM_STRATEGY:
            return RandomStrategy("", False, config_file)
        elif strategy_type == config_manager.EMPIRICAL_STRATEGY:
            return EmpiricalStrategy("", False, config_file)
        else: # Default behaviour
            return BaseStrategy("", False, config_file)
