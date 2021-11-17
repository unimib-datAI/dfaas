import sqlite3
import pandas as pd
from .db_manager import DbManager
from data_loader.request.config_request import ConfigRequest


class ExpDbManager(DbManager):

    def __init__(self, db_path) -> None:
        super().__init__(db_path)

    def create_tables(self) -> None:
        """ Create tables for experiment database """
        conn = sqlite3.connect(self._path)
        cursor = conn.cursor()

        # Create node table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `NODE` (
                `ID` INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                `Name` TEXT NOT NULL UNIQUE,
                `Ram` REAL NOT NULL,
                `Cpu` REAL NOT NULL
            );
        ''')

        # Creare function table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `FUNCTION` (
                `ID` INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                `Name` TEXT NOT NULL,
                `Description` TEXT NULL
            );
        ''')

        # Create experiment instant table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `EXPERIMENT_INSTANT` (
                `ID` INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                `Timestamp` DATETIME NOT NULL,
                `NodeID` INTEGER NOT NULL,
                CONSTRAINT `NodeID`
                    FOREIGN KEY (`NodeID`)
                    REFERENCES `NODE` (`ID`)
                    ON DELETE NO ACTION
                    ON UPDATE NO ACTION
            );
        ''')

        # Create metric table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `METRIC` (
                `ID` INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                `Type` TEXT CHECK( Type IN ('node','func') ) NOT NULL,
                `Name` TEXT NOT NULL,
                `Value` REAL NULL,
                `Unit` TEXT NOT NULL,
                `Description` TEXT DEFAULT NULL,
                `ExpInstantID` INTEGER NOT NULL,
                `NodeID` INTEGER DEFAULT NULL,
                `FunctionID` INTEGER DEFAULT NULL,
                CONSTRAINT `ExpInstantID` FOREIGN KEY (`ExpInstantID`) REFERENCES `EXPERIMENT_INSTANT` (`ID`) ON DELETE NO ACTION ON UPDATE NO ACTION,
                CONSTRAINT `FunctionID` FOREIGN KEY (`FunctionID`) REFERENCES `FUNCTION` (`ID`) ON DELETE NO ACTION ON UPDATE NO ACTION,
                CONSTRAINT `NodeIDMetic` FOREIGN KEY (`NodeID`) REFERENCES `NODE` (`ID`) ON DELETE NO ACTION ON UPDATE NO ACTION
                );
        ''')
        
        # Create deploy table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `DEPLOY` (
                `ExpInstantID` INTEGER NOT NULL,
                `FunctionID` INTEGER NOT NULL,
                `MaxRate` INTEGER NOT NULL,
                `NumReplicas` INTEGER NOT NULL,
                `Workload` INTEGER NOT NULL,
                `Margin` INTEGER NOT NULL,
                `State` TEXT NOT NULL,
                PRIMARY KEY(`ExpInstantID`, `FunctionID`),
                CONSTRAINT `ExpInstantIDdeploy`
                FOREIGN KEY(`ExpInstantID`)
                REFERENCES `EXPERIMENT_INSTANT` (`ID`)
                ON DELETE NO ACTION
                ON UPDATE NO ACTION,
                CONSTRAINT `FunctionIDdeploy`
                FOREIGN KEY(`FunctionID`)
                REFERENCES `FUNCTION` (`ID`)
                ON DELETE NO ACTION
                ON UPDATE NO ACTION)
        ''')

        conn.commit()

    ############ Insert statements ############

    def insert_node(self, name, ram, cpu) -> int:
        conn = sqlite3.connect(self._path)
        cursor = conn.cursor()

        cursor.execute('''
            INSERT INTO `NODE` (`Name`, `Ram`, `Cpu`)
            VALUES ( "{}", {}, {} );
        '''.format(name, ram, cpu))

        conn.commit()
        return cursor.lastrowid

    def insert_function(self, name, description) -> int:
        conn = sqlite3.connect(self._path)
        cursor = conn.cursor()

        cursor.execute('''
            INSERT INTO `FUNCTION` ( `Name`, `Description`)
            VALUES ( "{}", "{}" );
        '''.format(name, description))

        conn.commit()
        return cursor.lastrowid

    def insert_exp_instant(self, ts, node_id) -> int:
        conn = sqlite3.connect(self._path)
        cursor = conn.cursor()

        cursor.execute('''
            INSERT INTO `EXPERIMENT_INSTANT`( `Timestamp`, `NodeID` )
            VALUES ( "{}", {} );
        '''.format(ts, node_id))

        conn.commit()
        return cursor.lastrowid

    def insert_metric(self, name, type, unit, val, desc, exp_instant_id, function_id = None, node_id = None) -> int:
        conn = sqlite3.connect(self._path)
        cursor = conn.cursor()

        if function_id != None and node_id == None:
            cursor.execute('''
                INSERT INTO `METRIC` ( `Name`, `Type`, `Unit`, `Value`, `Description`, `ExpInstantID`, `FunctionID`)
                VALUES ( "{}", "{}", "{}", {}, "{}", {}, {});
            '''.format(name, type, unit, val, desc, exp_instant_id, function_id))
        elif node_id != None and function_id == None:
            cursor.execute('''
                INSERT INTO `METRIC` ( `Name`, `Type`, `Unit`, `Value`, `Description`, `ExpInstantID`, `NodeID`)
                VALUES ( "{}", "{}", "{}", {}, "{}", {}, {});
            '''.format(name, type, unit, val, desc, exp_instant_id, node_id))
        else:
            print("Params function_id and node_id cannot be both not None")

        conn.commit()
        return cursor.lastrowid

    def insert_deploy(self, exp_instant_id, function_id, max_rate, num_replicas, wl, margin, state) -> None:
        conn = sqlite3.connect(self._path)
        cursor = conn.cursor()

        cursor.execute('''
            INSERT INTO `DEPLOY` ( `ExpInstantID`, `FunctionID`, `MaxRate`, `NumReplicas`, `Workload`, `Margin`, `State` )
            VALUES ( {}, {}, {}, {}, {}, {}, "{}");
        '''.format(exp_instant_id, function_id, max_rate, num_replicas, wl, margin, state))

        conn.commit()

    #######################################

    def select_example(self):
        conn = sqlite3.connect(self._path)
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

        query = '''
                    SELECT e.ID
                    FROM NODE n
                    JOIN EXPERIMENT_INSTANT e ON n.ID = e.NodeID 
                    JOIN DEPLOY             d ON e.ID = d.ExpInstantID
                    JOIN FUNCTION           f ON d.FunctionID == f.ID
                    WHERE {}
                    GROUP BY e.ID
                    HAVING COUNT(d.FunctionID) = {}
                '''.format(where_condition, func_count)

        #print(query)

        conn = sqlite3.connect(self._path)
        c = conn.cursor()
        c.execute(query)

        experiments_id_list = [str(el[0]) for el in c.fetchall()]

        print("Intermediate result: DF with experiments IDs")
        df = pd.DataFrame(experiments_id_list,
                          columns=["ExpInstantID"])
        print(df)

        # Select all metrics for this specific config request
        query = '''
                    SELECT  m.ID, m.Type, m.Name, m.Value, m.Unit, m.Description, m.ExpInstantID, 
                            m.NodeID, n.Name, m.FunctionID, f.Name
                    FROM METRIC m
                    LEFT JOIN FUNCTION   f ON m.FunctionID = f.ID
                    LEFT JOIN NODE       n ON m.NodeID = n.ID
                    WHERE m.ExpInstantID IN ({})
                '''.format(",".join(experiments_id_list))

        c.execute(query)

        print("Final result: Metrics for Experiments {}".format(experiments_id_list))
        df = pd.DataFrame(c.fetchall(),
                          columns=["ID", "Type", "Name", "Value", "Unit", "Description",
                                   "ExpInstantID", "NodeID", "NodeName", "FunctionID",
                                   "FunctionName"])
        #print(df)

        df_node_metrics = df[df["Type"] == "node"].drop(columns=["FunctionID", "FunctionName"])
        df_func_metrics = df[df["Type"] == "func"].drop(columns=["NodeID", "NodeName"])

        print(df_node_metrics)
        print(df_func_metrics)

        return df_node_metrics, df_func_metrics
