# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

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

    def __str__(self):
        return "Node type: {} -- [ {} ]".format(self.__node_type, self.__func_requests)
