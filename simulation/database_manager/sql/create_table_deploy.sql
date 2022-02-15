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
        ON UPDATE NO ACTION
);