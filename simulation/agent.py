from behaviour.strategy import Strategy

class Agent(): # Inherit by Thread in () bratches

    _json_path = ""
    
    def __init__(self, id, logger, behaviour: Strategy):
        super().__init__()
        self._id = id
        self._behaviour = behaviour
        self._behaviour.set_id(id)
        self._behaviour.set_logger(logger)

    # Used when this class extends Thread
    def run(self) -> dict:
        #self.loop()
        w = self._behaviour.run()
        print(w)
        return w

    @property
    def strategy(self) -> Strategy:
        """
        The Context maintains a reference to one of the Strategy objects. The
        Context does not know the concrete class of a strategy. It should work
        with all strategies via the Strategy interface.
        """

        return self._behaviour

    @strategy.setter
    def strategy(self, _behaviour: Strategy) -> None:
        """
        Usually, the Context allows replacing a Strategy object at runtime.
        """

        self._behaviour = _behaviour

    def disable_logging(self):
        #self._logger.isEnabledFor(50) # Used to do not print and gain in speed.
        self._logger.disabled = True
