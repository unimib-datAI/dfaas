from .strategy import Strategy

class BaseStrategy(Strategy):

    def __init__(self, config_json):
        super().__init__(config_json)

    def run(self) -> dict:
        return self.loop()


    def loop(self) -> dict:
        """
        This stategy does not perform any forward of request.
        In any case, the node keeps all the requests
        """
        weights = {}

        # Extract loads data of the node from the config file
        original_requests_groups, original_requests_functions = self._get_load_for_groups_and_functions(self._config_json)

        for fun in original_requests_functions.keys():
            weights[fun] = {}
            weights[fun][self._id] = 1
        
        for func, val in weights.items():
            self._logger.info("Weights for func {}: {}".format(func, val))

        return weights

    def set_id(self, id):
        self._id = self._config_manager.NODE_KEY_PREFIX + str(id)
        self._prefix = "THREAD: " + self._id

    def set_logger(self, logger):
        self._logger = logger
