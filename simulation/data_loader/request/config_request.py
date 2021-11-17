class ConfigRequest:

    def __init__(self, node_type, func_requests):
        self._node_type = node_type
        self._func_requests = func_requests

    def get_node_type(self):
        return self._node_type

    def get_functions(self):
        return self._func_requests
