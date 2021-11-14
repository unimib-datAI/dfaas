class FunctionRequest:

    def __init__(self, name, replicas, wl):
        self._name = name
        self._replicas_num = replicas
        self._wl = wl

    def get_name(self):
        return self._name

    def get_replicas_num(self):
        return self._replicas_num

    def get_wl(self):
        return self._wl
