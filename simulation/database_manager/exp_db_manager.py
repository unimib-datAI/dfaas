import sqlite3
from sqlite3 import Error
from simulation.database_manager.db_manager import DbManager

class ExpDbManager(DbManager):
    def __init__(self, db_path) -> None:
        super().__init__(db_path)

    def create_tables(self) -> None:
        """ Create tables for experiment database """
        cursor = self._conn.cursor()

        # Create node table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `NODE` (
                `ID` INT NOT NULL AUTO_INCREMENT,
                `Name` VARCHAR(20) NOT NULL,
                `Ram` DECIMAL(4,1) NOT NULL,
                `Cpu` DECIMAL(4,1) NOT NULL,
                PRIMARY KEY (`ID`),
                UNIQUE INDEX `Name_UNIQUE` (`Name` ASC) VISIBLE
            );
        ''')

        # Creare function table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `FUNCTION` (
                `ID` INT NOT NULL AUTO_INCREMENT,
                `Name` VARCHAR(20) NOT NULL,
                `Description` VARCHAR(100) NULL,
                PRIMARY KEY (`ID`)
            );
        ''')

        # Create experiment instant table
        cursor.execute('''
            CREATE TABLE `EXPERIMENT_INSTANT` (
                `ID` INT NOT NULL AUTO_INCREMENT,
                `Timestamp` DATETIME NOT NULL,
                `NodeID` INT NOT NULL,
                PRIMARY KEY (`ID`),
                INDEX `NodeID_idx` (`NodeID` ASC) VISIBLE,
                CONSTRAINT `NodeID`
                    FOREIGN KEY (`NodeID`)
                    REFERENCES `db_prova`.`NODE` (`ID`)
                    ON DELETE NO ACTION
                    ON UPDATE NO ACTION
            );
        ''')

        # Create metric table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `METRIC` (
                `ID` int(11) NOT NULL AUTO_INCREMENT,
                `Type` enum('node','func') NOT NULL,
                `Name` varchar(20) NOT NULL,
                `Unit` varchar(20) NOT NULL,
                `Description` varchar(100) DEFAULT NULL,
                `ExpInstantID` int(11) NOT NULL,
                `NodeID` int(11) DEFAULT NULL,
                `FunctionID` int(11) DEFAULT NULL,
                PRIMARY KEY (`ID`),
                KEY `ExpInstantID_idx` (`ExpInstantID`),
                KEY `NodeID_idx` (`NodeID`),
                KEY `FunctionID_idx` (`FunctionID`),
                CONSTRAINT `ExpInstantID` FOREIGN KEY (`ExpInstantID`) REFERENCES `EXPERIMENT_INSTANT` (`ID`) ON DELETE NO ACTION ON UPDATE NO ACTION,
                CONSTRAINT `FunctionID` FOREIGN KEY (`FunctionID`) REFERENCES `FUNCTION` (`ID`) ON DELETE NO ACTION ON UPDATE NO ACTION,
                CONSTRAINT `NodeIDMetic` FOREIGN KEY (`NodeID`) REFERENCES `NODE` (`ID`) ON DELETE NO ACTION ON UPDATE NO ACTION
                ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
        ''')
        
        # Create deploy table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `DEPLOY` (
                `ExpInstantID` INT NOT NULL,
                `FunctionID` INT NOT NULL,
                `MaxRate` INT NOT NULL,
                `NumReplicas` INT NOT NULL,
                `Workload` INT NOT NULL,
                PRIMARY KEY(`ExpInstantID`, `FunctionID`),
                INDEX `FunctionIDdeploy_idx` (`FunctionID` ASC) VISIBLE,
                CONSTRAINT `ExpInstantIDdeploy`
                FOREIGN KEY(`ExpInstantID`)
                REFERENCES `db_prova`.`EXPERIMENT_INSTANT` (`ID`)
                ON DELETE NO ACTION
                ON UPDATE NO ACTION,
                CONSTRAINT `FunctionIDdeploy`
                FOREIGN KEY(`FunctionID`)
                REFERENCES `db_prova`.`FUNCTION` (`ID`)
                ON DELETE NO ACTION
                ON UPDATE NO ACTION)
        ''')

        self._conn.commit()
        
    def insert_node(self, name, ram, cpu) -> None:
        cursor = self._conn.cursor()
        
        cursor.execute('''
            INSERT INTO `NODE` (`Name`, `Ram`, `Cpu`)
            VALUES ( {}, {}, {} );
        '''.format(name, ram, cpu))

        self._conn.commit()

    def insert_function(self, name, description) -> None:
        cursor = self._conn.cursor()
        
        cursor.execute('''
            INSERT INTO `FUNCTION` ( `Name`, `Description`)
            VALUES ( {}, {} );
        '''.format(name, description))

        self._conn.commit()

    def insert_function(self, name, description) -> None:
        cursor = self._conn.cursor()

        cursor.execute('''
            INSERT INTO `FUNCTION` ( `Name`, `Description`)
            VALUES ( {}, {} );
        '''.format(name, description))

        self._conn.commit()

    def insert_exp_instant(self, ts, node_id) -> None:
        cursor = self._conn.cursor()

        cursor.execute('''
            INSERT INTO `EXPERIMENT_INSTANT`( `Timestamp`, `NodeID` )
            VALUES ( {}, {} );
        '''.format(ts, node_id))

        self._conn.commit()

    def insert_metric(self, name, type, unit, desc, exp_instant_id, function_id, node_id = None) -> None:
        cursor = self._conn.cursor()

        if function_id != None and node_id == None:
            cursor.execute('''
                INSERT INTO `METRIC` ( `Name`, `Type`, `Unit`, `Description`, `ExpInstantID`, `FunctionID`)
                VALUES ( {}, {}, {}, {}, {}, {});
            '''.format(name, type, unit, desc, exp_instant_id, function_id))
        elif node_id != None and function_id == None:
            cursor.execute('''
                INSERT INTO `METRIC` ( `Name`, `Type`, `Unit`, `Description`, `ExpInstantID`, `NodeID`)
                VALUES ( {}, {}, {}, {}, {}, {});
            '''.format(name, type, unit, desc, exp_instant_id, node_id))
        else:
            print("Params function_id and node_id cannot be both not None")

        self._conn.commit()
    
    def insert_exp_instant(self, ts, node_id) -> None:
        cursor = self._conn.cursor()

        cursor.execute('''
            INSERT INTO `EXPERIMENT_INSTANT`( `Timestamp`, `NodeID` )
            VALUES ( {}, {} );
        '''.format(ts, node_id))

        self._conn.commit()

    def insert_deploy(self, exp_instant_id, function_id, max_rate, num_replicas, wl) -> None:
        cursor = self._conn.cursor()

        cursor.execute('''
            INSERT INTO `DEPLOY` ( `ExpInstantID`, `FunctionID`, `MaxRate`, `NumReplicas`, `Workload` )
            VALUES ( {}, {}, {}, {}, {});
        '''.format(exp_instant_id, function_id, max_rate, num_replicas, wl))

        self._conn.commit()
