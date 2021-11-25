class FunctionRequest:
    """
    This class represents the request of a specific function.
    A FunctionRequest is composed by the name of the function,
    the number of replicas and the workload
    """

    def __init__(self, name, replicas, wl):
        self.__name = name
        self.__replicas_num = replicas
        self.__wl = wl

    def get_name(self):
        """
        Get the function name
        """
        return self.__name

    def get_replicas_num(self):
        """
        Get number of replicas of function
        """
        return self.__replicas_num

    def get_wl(self):
        """
        Get workload of function (invocation rate) in terms
        of req/s
        """
        return self.__wl

    def __str__(self):
        return "( Name: {} -- Replicas: {} -- Workload: {} )".format(self.__name, self.__replicas_num, self.__wl)

    def __repr__(self):
        return str(self)
