class Configuration:
    def __init__(self):
        pass
        # self.node = node
        # self.funca_num = funca_num
        # self.funcb_num = funcb_num
        # self.funcc_num = funcc_num
        # self.funca_wl = funca_wl
        # self.funcb_wl = funcb_wl
        # self.funcc_wl = funcc_wl

    def set_node(self, node):
        self.node = node
        return self

    def set_funca_replicas(self, funca_num):
        self.funca_num = funca_num
        return self

    def set_funcb_replicas(self, funcb_num):
        self.funcb_num = funcb_num
        return self

    def set_funcc_replicas(self, funcc_num):
        self.funcc_num = funcc_num
        return self

    def set_funca_wl(self, funca_wl):
        self.funca_wl = funca_wl
        return self

    def set_funcb_wl(self, funcb_wl):
        self.funcb_wl = funcb_wl
        return self
  
    def set_funcc_wl(self, funcc_wl):
        self.funcc_wl = funcc_wl
        return self

    def __str__(self):
        return "Node: " + str(self.node) + " -- #Funca: " + str(self.funca_num) + " -- #Funcb: " + str(self.funcb_num) + \
                " -- #Funcc: " + str(self.funcc_num) + " -- FuncaWL: " + str(self.funca_wl) + " -- FuncbWL: " + \
                str(self.funcb_wl) + " -- FunccWL: " + str(self.funcc_wl)

    def __eq__(self, other):
        """Overrides the default implementation"""
        if isinstance(other, Configuration):
            return self.node == other.node and self.funca_num == other.funca_num and self.funcb_num == other.funcb_num and \
                self.funcc_num == other.funcc_num and self.funca_wl == other.funca_wl and \
                self.funcb_wl == other.funcb_wl and self.funcc_wl == other.funcc_wl
        return False

