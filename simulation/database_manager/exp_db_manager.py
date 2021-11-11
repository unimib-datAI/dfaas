import sqlite3
from sqlite3 import Error
from simulation.database_manager.db_manager import DbManager

class ExpDbManager(DbManager):
    def __init__(self, db_path) -> None:
        super().__init__(db_path)

    def create_tables(self):
        """ Create tables for experiment database """
        cursor = self._conn.cursor()

        # Create node table
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS `db_prova`.`NODE` (
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
            CREATE TABLE IF NOT EXISTS `db_prova`.`FUNCTION` (
                `ID` INT NOT NULL AUTO_INCREMENT,
                `Name` VARCHAR(20) NOT NULL,
                `Description` VARCHAR(100) NULL,
                PRIMARY KEY (`ID`)
            );
        ''')

        # Create experiment instant table
        cursor.execute('''
            CREATE TABLE `db_prova`.`EXPERIMENT_INSTANT` (
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
            CREATE TABLE IF NOT EXISTS `db_prova`.`DEPLOY` (
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

    
