import datetime

import pandas as pd
import os
import json

from config_manager import ConfigManager
from database_manager.exp_db_manager import ExpDbManager


class DataLoader:

    _config_manager = ConfigManager()
    _nodes_ids = {}
    _functions_ids = {}

    def __init__(self):
        self._db_manager = ExpDbManager(self._config_manager.EXPERIMENT_DB_PATH)

        if not os.path.exists(self._config_manager.EXPERIMENT_DB_PATH):
            print("Creating connection to database...")
            self._db_manager.create_connection()
            print("Done")
            print("Creating tables...")
            self._db_manager.create_tables()
            print("Done")

            print("Loading data from disk...")
            self._load_static_data()
            self._load_data()
            print("Done")
        else:
            print("DB file already exist")

    def get_metric_for_configuration(self, config_request) -> pd.DataFrame and pd.DataFrame:
        df_node, df_func = self._db_manager.get_metrics(config_request)

        if df_node.empty and df_func.empty:
            print("Experiment with this type of configuration does not exist in the database...")

        return df_node, df_func

    def _parse_file(self, json_file) -> None:
        # Parse input section
        # Note: all other data from "input" section are discarded
        # because replicas and wl can be obtained from single minute metrics
        node_type = json_file["input"]["node"]

        # Parse output section
        for idx, exp_instant in enumerate(json_file["output"]):
            node_ram_usage = json_file["output"][idx]["ram_usage"]
            node_cpu_usage = json_file["output"][idx]["cpu_usage"]

            if "timestamp" in json_file["output"][idx]:
                timestamp = json_file["output"][idx]["timestamp"]
            else:
                now = datetime.datetime.now()
                timestamp = datetime.datetime(now.year, now.month, now.day, 0, 0, idx)

            exp_instant_id = self._db_manager.insert_exp_instant(timestamp, self._nodes_ids[node_type])

            self._db_manager.insert_metric("ram_usage", "node", self._config_manager.NODES_METRICS_UNIT["ram_usage"],
                                           round(node_ram_usage*100, 2),
                                           self._config_manager.NODES_METRICS["ram_usage"], exp_instant_id,
                                           node_id=self._nodes_ids[node_type])
            self._db_manager.insert_metric("cpu_usage", "node", self._config_manager.NODES_METRICS_UNIT["cpu_usage"],
                                           round(node_cpu_usage * 100, 2),
                                           self._config_manager.NODES_METRICS["cpu_usage"], exp_instant_id,
                                           node_id=self._nodes_ids[node_type])

            for _, func in enumerate(json_file["output"][idx]["functions"]):
                func_name = func["name"]
                if func_name in self._config_manager.FUNCTION_NAMES:
                    self._db_manager.insert_deploy(exp_instant_id, self._functions_ids[func_name], func["max_rate"], func["service_count"], func["invoc_rate"], func["margin"], func["state"])
                    for key, value in func.items():
                        if key != "name" and key not in self._config_manager.DEPLOY_DATAS:
                            if key in ["ram_xfunc", "cpu_xfunc"]:
                                value = round(value * 100, 2)

                            self._db_manager.insert_metric(key, "func", self._config_manager.FUNCTION_METRICS_UNIT[key],
                                                           value, self._config_manager.FUNCTION_METRICS[key], exp_instant_id,
                                                           function_id=self._functions_ids[func_name])

    def _load_data(self) -> None:
        data_dir = self._config_manager.DATA_DIR
        for node_type in self._config_manager.NODES_TYPE:
            for path, _, files in os.walk(os.path.join(data_dir, node_type)):
                for config in files:
                    f = open(os.path.join(path, config))
                    json_doc = json.load(f)  # Return json file as a dictionary
                    self._parse_file(json_doc)

    def _load_static_data(self) -> None:
        # Load node's data
        for node_type in self._config_manager.NODES_TYPE:
            node_id = self._db_manager.insert_node(
                node_type,
                self._config_manager.NODE_CONFIGURATIONS[node_type]["ram"],
                self._config_manager.NODE_CONFIGURATIONS[node_type]["cpu"]
            )
            self._nodes_ids[node_type] = node_id

        # Load function's data
        for func_name in self._config_manager.FUNCTION_NAMES:
            function_id = self._db_manager.insert_function(
                func_name,
                self._config_manager.FUNCTIONS_DESCRPTION[func_name]
            )
            self._functions_ids[func_name] = function_id

    def select_example(self):
        self._db_manager.select_example()
