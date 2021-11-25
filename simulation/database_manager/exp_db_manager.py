import datetime
import sqlite3
import time

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
    __exp_ids_for_func = {}

    def __init__(self, db_path) -> None:
        super().__init__(db_path)

    def __execute_insert_create_query(self, query: str):
        """
        Method used to execute a generic query
        :query: string representation of query to be executed
        """
        conn = sqlite3.connect(self._DbManager__path)
        cursor = conn.cursor()
        cursor.execute(query)
        conn.commit()
        return cursor.lastrowid

    def __select_exp_ids_for_func(self):
        """
        Method used to populate a dict that maps for each function a list of experiment ids that contain this function
        """
        for func in self.__config_manager.FUNCTION_NAMES:
            query = Path(self.__config_manager.SQL_FILE_PATH_DIR +
                         "select_exp_ids_for_func.sql").read_text().format(func)

            conn = sqlite3.connect(self._DbManager__path)
            c = conn.cursor()
            c.execute(query)

            # Get list of ids for next queries
            id_list = [str(el[0]) for el in c.fetchall()]
            self.__exp_ids_for_func[func] = id_list


    def create_tables(self) -> None:
        """ Create tables for experiment database """
        self.__execute_insert_create_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_node.sql").read_text())
        self.__execute_insert_create_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_function.sql").read_text())
        self.__execute_insert_create_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_exp_instant.sql").read_text())
        self.__execute_insert_create_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_metric.sql").read_text())
        self.__execute_insert_create_query(Path(self.__config_manager.SQL_FILE_PATH_DIR + "create_table_deploy.sql").read_text())

    def insert_node(self, name: str, ram: float, cpu: float) -> int:
        """
        Insert a node into NODE table
        :name: node's name, with unique constraint
        :ram: ram available on this node type
        :cpu: cpu available on this node type
        :return: last inserted id
        """
        last_id = self.__execute_insert_create_query(
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
        last_id = self.__execute_insert_create_query(
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
        last_id = self.__execute_insert_create_query(
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
            last_id = self.__execute_insert_create_query(
                Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_metric_func.sql").read_text().format(name, type, unit, val, desc, exp_instant_id, function_id)
            )
        elif node_id is not None and function_id is None:
            last_id = self.__execute_insert_create_query(
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
        last_id = self.__execute_insert_create_query(
            Path(self.__config_manager.SQL_FILE_PATH_DIR + "insert_deploy.sql").read_text().format(exp_instant_id, function_id, max_rate, num_replicas, wl, margin, state)
        )
        return last_id

    # TODO: delete this method
    def select_example(self):
        """
        Dummy select query example
        Used to verify that everything works
        Used for various type of experiments
        """
        # Metodo1:
        #   1) Selezione exp id con numero funzioni deployate = 2
        #   2) Tra questi id selezione dei record corrispondenti a ConfigRequest e selezioni di quelli con solo due funzioni rimaste (esattamente quelle due)
        #   3) Selezione delle metriche a partire dagli ultimi ID

        start = time.perf_counter()

        conn = sqlite3.connect(self._DbManager__path)
        c = conn.cursor()

        c.execute('''
                SELECT e.ID
                FROM EXPERIMENT_INSTANT e
                JOIN DEPLOY             d ON e.ID = d.ExpInstantID
                GROUP BY e.ID
                HAVING COUNT(d.FunctionID) = 2
        ''')

        fetch_data = c.fetchall()
        experiments_id_list = [str(el[0]) for el in fetch_data]
        #print(type(fetch_data)) # List type
        #print(fetch_data)

        df = pd.DataFrame(fetch_data, columns=["e.ID"])
        print(df)
        df.to_csv("prova.csv")

        c.execute('''
            SELECT e.ID
            FROM NODE n
            JOIN EXPERIMENT_INSTANT e ON n.ID = e.NodeID
            JOIN DEPLOY             d ON e.ID = d.ExpInstantID
            JOIN FUNCTION           f ON d.FunctionID = f.ID
            WHERE e.ID IN ({})  AND
            n.Name = 'node_1' AND
            (
                (f.Name = 'ocr'    AND d.Workload = 0  AND d.NumReplicas = 1)
                OR
                ( f.Name = 'qrcode' AND d.Workload = 0  AND d.NumReplicas = 1)
            )
            GROUP BY e.ID
            HAVING COUNT(f.Name) = 2
        '''.format(",".join(experiments_id_list)))

        fetch_data = c.fetchall()
        df = pd.DataFrame(fetch_data, columns=["e.ID"])
        print(df)
        experiments_id_list = [str(el[0]) for el in fetch_data]

        c.execute('''
            SELECT  m.Name, m.type, f.Name, n.Name, AVG(m.Value), d.MaxRate, d.NumReplicas, d.Margin, 
                    d.State
            FROM METRIC m
            LEFT JOIN FUNCTION   f ON m.FunctionID = f.ID
            LEFT JOIN NODE       n ON m.NodeID = n.ID
            LEFT JOIN DEPLOY     d ON m.ExpInstantID = d.ExpInstantID and
                                      f.ID = d.FunctionID
            WHERE m.ExpInstantID IN ({})
            GROUP BY m.Name, m.type, f.Name, d.MaxRate, d.NumReplicas, d.Margin, d.State, n.Name
        '''.format(",".join(experiments_id_list)))

        fetch_data = c.fetchall()
        # print(type(fetch_data)) # List type
        # print(fetch_data)

        df = pd.DataFrame(fetch_data, columns=["m.Name", "m.type", "f.Name", "n.Name", "AVG(m.Value)",
                                               "d.MaxRate", "d.NumReplicas", "d.Margin", "d.State"])
        print(df)

        end = time.perf_counter()
        execution = end - start
        print("Execution time for METHOD 1: ", execution)
        start = time.perf_counter()

        # Metodo2:
        #   1) Selezione delle funzioni che hanno quella determinata ConfigRequets e count = 2
        #   2) Selezione di tutti gli exp ID che comprendono anche funca
        #   3) Rimozione degli ID dai primi che contengono ancge funca e selezione delle metriche con ID rimasti
        conn = sqlite3.connect(self._DbManager__path)
        c = conn.cursor()

        c.execute('''
                    SELECT e.ID
                    FROM NODE n
                    JOIN EXPERIMENT_INSTANT e ON n.ID = e.NodeID
                    JOIN DEPLOY             d ON e.ID = d.ExpInstantID
                    JOIN FUNCTION           f ON d.FunctionID = f.ID
                    WHERE n.Name = 'node_1' AND
                    (
                        (f.Name = 'ocr'    AND d.Workload = 0  AND d.NumReplicas = 1)
                        OR
                        ( f.Name = 'qrcode' AND d.Workload = 0  AND d.NumReplicas = 1)
                    )
                    GROUP BY e.ID
                    HAVING COUNT(f.Name) = 2
                ''')

        fetch_data = c.fetchall()
        experiments_id_list = [str(el[0]) for el in fetch_data]
        # print(type(fetch_data)) # List type
        # print(fetch_data)

        df = pd.DataFrame(fetch_data, columns=["e.ID"])
        print(df)
        df.to_csv("prova.csv")

        c.execute('''
                    SELECT d.ExpInstantID
                    FROM DEPLOY             d 
                    JOIN FUNCTION           f ON d.FunctionID = f.ID
                    WHERE f.Name = "funca" 
                ''')

        fetch_data = c.fetchall()
        df = pd.DataFrame(fetch_data, columns=["e.ID"])
        print(df)
        experiments_id_list_funca = [str(el[0]) for el in fetch_data]

        final_list = [el for el in experiments_id_list if el not in experiments_id_list_funca]

        print(final_list)

        c.execute('''
                    SELECT  m.Name, m.type, f.Name, n.Name, AVG(m.Value), d.MaxRate, d.NumReplicas, d.Margin, 
                            d.State
                    FROM METRIC m
                    LEFT JOIN FUNCTION   f ON m.FunctionID = f.ID
                    LEFT JOIN NODE       n ON m.NodeID = n.ID
                    LEFT JOIN DEPLOY     d ON m.ExpInstantID = d.ExpInstantID and
                                              f.ID = d.FunctionID
                    WHERE m.ExpInstantID IN ({})
                    GROUP BY m.Name, m.type, f.Name, d.MaxRate, d.NumReplicas, d.Margin, d.State, n.Name
                '''.format(",".join(final_list)))

        fetch_data = c.fetchall()
        # print(type(fetch_data)) # List type
        # print(fetch_data)

        df = pd.DataFrame(fetch_data, columns=["m.Name", "m.type", "f.Name", "n.Name", "AVG(m.Value)",
                                               "d.MaxRate", "d.NumReplicas", "d.Margin", "d.State"])
        print(df)

        end = time.perf_counter()
        execution = end - start
        print("Execution time for METHOD 2: ", execution)

    def get_metrics(self, conf_request: ConfigRequest) -> pd.DataFrame and pd.DataFrame:
        """
        Method used to get metrics related to a specific configuration request
        :conf_request: configuration request
        :return: two dataframe, one for node's metrics and another for function's metrics
        """

        if not self.__exp_ids_for_func:
            self.__select_exp_ids_for_func()

        func_count = len(conf_request.get_functions())
        node_type = conf_request.get_node_type()

        # Select experiment IDs for this specific config request
        where_condition = "( n.Name = '{}' ) ".format(node_type)

        if func_count > 0:
            where_condition += "AND ( "

            for idx, func_req in enumerate(conf_request.get_functions()):
                where_condition += "(       \
                    f.Name = '{}'      AND \
                    d.Workload = {}    AND \
                    d.NumReplicas = {}     \
                )".format(func_req.get_name(), func_req.get_wl(), func_req.get_replicas_num())

                if idx != func_count - 1:
                    where_condition += " OR "

            where_condition += ")"

        query = Path(self.__config_manager.SQL_FILE_PATH_DIR +
                     "select_exp_id_for_config.sql").read_text().format(where_condition, func_count)

        conn = sqlite3.connect(self._DbManager__path)
        c = conn.cursor()
        c.execute(query)

        # Get list of ids for next queries
        experiments_id_list = [str(el[0]) for el in c.fetchall()]

        # Up to this point there is a tricky problem:
        #   Selected IDs are referred to experiment instants that has "func_count" or more deployed functions
        #   Filtering for HAVING COUNT() == 2 does not exclude exp IDs with more functions deployed
        # Solution: exclude experiments id's referred to functions not included in the ConfigRequest
        for func in self.__config_manager.FUNCTION_NAMES:
            if func not in [f.get_name() for f in conf_request.get_functions()]:
                experiments_id_list = [el for el in experiments_id_list if el not in self.__exp_ids_for_func[func]]

        # Select all metrics for this specific config request
        query = Path(self.__config_manager.SQL_FILE_PATH_DIR +
                     "select_metrics_by_exp_ids.sql").read_text().format(",".join(experiments_id_list))
        c.execute(query)

        print("Metrics for Experiments {}".format(experiments_id_list))
        print("------------------------------------------------------------------------------")
        df = pd.DataFrame(c.fetchall(),
                          columns=["MetricName", "Type", "FunctionName", "NodeName",
                                   "AVG(Value)", "MaxRate", "NumReplicas", "Margin", "State"])

        df_node_metrics = df[df["Type"] == "node"].drop(columns=["FunctionName", "MaxRate",
                                                                 "NumReplicas", "Margin", "State"])
        df_func_metrics = df[df["Type"] == "func"].drop(columns=["NodeName"])

        for col in ["MaxRate", "NumReplicas", "Margin"]:
            df_func_metrics[col] = df_func_metrics[col].astype(int)

        print(df_node_metrics)
        print("------------------------------------------------------------------------------")
        print(df_func_metrics)

        return df_node_metrics, df_func_metrics
