# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from database_manager.exp_db_manager import ExpDbManager
from data_loader.data_loader import DataLoader
from data_loader.request.config_request import ConfigRequest
from data_loader.request.function_request import FunctionRequest
import datetime

#db_manager = ExpDbManager("database_design/experiment_db")
#db_manager.create_connection()
#db_manager.create_tables()

#db_manager.insert_node("node_1", 2, 1)
#db_manager.insert_function("funca", "Very simple function")
#db_manager.insert_function("qrcode", "Qrcode generation function")
#db_manager.insert_exp_instant(datetime.datetime.now(), 1)
#db_manager.insert_metric("ram_usage", "node", "%", 34, "RAM usage for a node", 1, node_id = 1)
#db_manager.insert_metric("afet", "func", "sec", 0.000000005, "Average Function Execution time", 1, function_id = 1)
#db_manager.insert_deploy(1, 1, 150, 5, 50, 100, "Underload")
#db_manager.insert_deploy(1, 2, 20, 2, 10, 10, "Underload")

#db_manager.select_example()

#db_manager.get_metrics(
#    ConfigRequest(
#        "node_1",
#        [
#            FunctionRequest("funca", 5, 50),
#            FunctionRequest("qrcode", 2, 10)
#        ]
#    )
#)

dl = DataLoader()
#dl.select_example()

request = ConfigRequest("node_1",
    [
        FunctionRequest("qrcode", 1, 0),
        FunctionRequest("ocr", 1, 0)
    ]
)

df_node, df_func = dl.get_metric_for_configuration(request)

#dl.select_example()


print("END")
