import datetime
import sqlite3
import pandas as pd
from pathlib import Path
from .db_manager import DbManager
from data_loader.request.config_request import ConfigRequest
from config_manager import ConfigManager


class ExpDbManager(DbManager):
    """
    Class that represents Experiment database,
    with all SQLLite statements to create table,
    insert data and get metrics
    """

    __config_manager = ConfigManager()

    def __init__(self, db_path) -> None:
        super().__init__(db_path)

    def __execute_query(self, query: str):
        """
        Method used to execute a generic query
        :query: string representation of query to be executed
        """
        conn = sqlite3.connect(self._DbManager__path)
        cursor = conn.cursor()
        cursor.execute(query)
        conn.commit()
        return cursor.lastrowid

    def create_tables(self) -> None:
        """ Create tables for experiment database """
        self.__execute_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_node.sql").read_text())
        self.__execute_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_function.sql").read_text())
        self.__execute_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_exp_instant.sql").read_text())
        self.__execute_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_metric.sql").read_text())
        self.__execute_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_deploy.sql").read_text())

    def insert_node(self, name: str, ram: float, cpu: float) -> int:
        """
        Insert a node into NODE table
        :name: node's name, with unique constraint
        :ram: ram available on this node type
        :cpu: cpu available on this node type
        :return: last inserted id
        """
        last_id = self.__execute_query(
            Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_node.sql").read_text().format(name, ram, cpu)
        )
        return last_id

    def insert_function(self, name: str, description: str) -> int:
        """
        Insert a function to FUNCTION table
        :name: function name, with unique constraint
        :description: function description
        :return: last inserted id
        """
        last_id = self.__execute_query(
            Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_function.sql").read_text().format(name, description)
        )
        return last_id

    def insert_exp_instant(self, ts: datetime, node_id: int) -> int:
        """
        Insert experiment_instant to EXPERIMENT_INSTANT table
        :ts: timestamp of experiment instant
        :node_id: id of the node which experiment_instant is related to
        :return: last inserted id
        """
        last_id = self.__execute_query(
            Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_exp_instant.sql").read_text().format(ts, node_id)
        )
        return last_id

    def insert_metric(self, name: str, type: str, unit: str, val: float, desc: str,
                      exp_instant_id: int, function_id = None, node_id = None) -> int:
        """
        Insert metric to METRIC table
        :name: metric name
        :type: metric type (func or node)
        :unit: metric unit of measure
        :val: metric value
        :desc: metric description
        :exp_instant_id: experiment instant id which metric is related to
        :function_id: (optional) id of function which metric is related to
        :node_id: (optional) id of node which metric is related to
        :return: last inserted id
        """
        if function_id is not None and node_id is None:
            last_id = self.__execute_query(
                Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_metric_func.sql").read_text().format(name, type, unit, val, desc, exp_instant_id, function_id)
            )
        elif node_id is not None and function_id is None:
            last_id = self.__execute_query(
                Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_metric_node.sql").read_text().format(name, type, unit, val, desc, exp_instant_id, node_id)
            )
        else:
            print("Params function_id and node_id cannot be both not None")

        return last_id

    def insert_deploy(self, exp_instant_id: int, function_id: int, max_rate: int,
                      num_replicas: int, wl: int, margin: int, state: str) -> None:
        """
        Insert deploy information on DEPLOY table
        :exp_instant_id: experiment instant id
        :function_id: function id
        :max_rate: max_rate of function deployed in this experiment instant (req/s)
        :num_replicas: number of function replicas deployed in this experiment instant
        :wl: invocation rate to this function deployed in this experiment instant (req/s)
        :margin: margin of requests for this function deployed in this experiment instant (req/s)
        :state: state ("Overloaded", "Underloaded") for this function deployed in this experiment instant
        """
        last_id = self.__execute_query(
            Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_deploy.sql").read_text().format(exp_instant_id, function_id, max_rate, num_replicas, wl, margin, state)
        )
        return last_id

    def select_example(self):
        """
        Dummy select query example
        Used to verify that everything works
        """
        conn = sqlite3.connect(self._DbManager__path)
        c = conn.cursor()

        c.execute('''
                SELECT a.ExpInstantID, a.MaxRate, a.NumReplicas, a.Workload, b.Name, b.Description
                FROM DEPLOY a
                JOIN FUNCTION b ON a.FunctionID = b.ID
                WHERE b.Name == "qrcode"
                ORDER BY a.Workload DESC
                LIMIT 5
        ''')

        fetch_data = c.fetchall()
        print(type(fetch_data)) # List type
        print(fetch_data)

        df = pd.DataFrame(fetch_data, columns=["ExpInstantID", "FunctionID", "MaxRate", "NumReplicas", "Workload", "Name", "Description"])
        print(df)

    def get_metrics(self, conf_request: ConfigRequest) -> pd.DataFrame and pd.DataFrame:
        """
        Method used to get metrics related to a specific configuration request
        :conf_request: configuration request
        :return: two dataframe, one for node's metrics and another for function's metrics
        """
        func_count = len(conf_request.get_functions())
        node_type = conf_request.get_node_type()

        # Select experiment IDs for this specific config request
        where_condition = "( n.Name == '{}' ) AND ".format(node_type)
        where_condition += "( "

        for idx, func_req in enumerate(conf_request.get_functions()):
            where_condition += "(       \
                f.Name == '{}'      AND \
                d.Workload == {}    AND \
                d.NumReplicas == {}     \
            ) ".format(func_req.get_name(), func_req.get_wl(), func_req.get_replicas_num())

            if idx != func_count - 1:
                where_condition += "OR"

        where_condition += ")"

        query = Path(self.__config_manager.SQL_FILE_PATH_DIR +
                     "select_exp_id_for_config.sql").read_text().format(where_condition, func_count)

        conn = sqlite3.connect(self._DbManager__path)
        c = conn.cursor()
        c.execute(query)

        experiments_id_list = [str(el[0]) for el in c.fetchall()]

        #print("Intermediate result: DF with experiments IDs")
        #df = pd.DataFrame(experiments_id_list,
        #                  columns=["ExpInstantID"])
        #print(df)

        # Select all metrics for this specific config request
        query = Path(self.__config_manager.SQL_FILE_PATH_DIR +
                     "select_metrics_by_exp_ids.sql").read_text().format(",".join(experiments_id_list))

        c.execute(query)

        print("Metrics for Experiments {}".format(experiments_id_list))
        print("------------------------------------------------------------------------------")
        df = pd.DataFrame(c.fetchall(),
                          columns=["ID", "Type", "Name", "Value", "Unit", "Description",
                                   "ExpInstantID", "NodeID", "NodeName", "FunctionID",
                                   "FunctionName"])

        df_node_metrics = df[df["Type"] == "node"].drop(columns=["FunctionID", "FunctionName"])
        df_func_metrics = df[df["Type"] == "func"].drop(columns=["NodeID", "NodeName"])

        print(df_node_metrics)
        print("------------------------------------------------------------------------------")
        print(df_func_metrics)

        return df_node_metrics, df_func_metrics
