class ConfigRequest:
    """
    Class that represents a configuration request by the simulator.
    A configuration request is composed by a node_type and a list of
    FunctionRequest
    """

    def __init__(self, node_type, func_requests):
        self.__node_type = node_type
        self.__func_requests = func_requests

    def get_node_type(self):
        """
        Get node type
        """
        return self.__node_type

    def get_functions(self):
        """
        Get functions list
        """
        return self.__func_requests
