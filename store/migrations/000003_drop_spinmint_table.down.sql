BEGIN;

CREATE TABLE IF NOT EXISTS `Spinmint`
  (
    `InstanceId` varchar(128) NOT NULL,
    `RepoOwner` varchar(255) DEFAULT NULL,
    `RepoName` varchar(255) DEFAULT NULL,
    `Number` int(11) DEFAULT NULL,
    `CreatedAt` bigint(20) DEFAULT NULL,
    PRIMARY KEY(`InstanceId`)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

COMMIT;