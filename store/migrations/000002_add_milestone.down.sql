BEGIN;

SET @dbName = DATABASE();
SET @tableName = "PullRequests";
SET @columnName = "MilestoneNumber";
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE
      (table_name = @tableName)
      AND (table_schema = @dbName)
      AND (column_name = @columnName)
  ) > 0,
  CONCAT("ALTER TABLE ", @tableName, " DROP ", @columnName, ";"),
  "SELECT 1"
));
PREPARE alterIfExists FROM @preparedStatement;
EXECUTE alterIfExists;

DEALLOCATE PREPARE alterIfExists;

SET @columnName = "MilestoneTitle";
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE
      (table_name = @tableName)
      AND (table_schema = @dbName)
      AND (column_name = @columnName)
  ) > 0,
  CONCAT("ALTER TABLE ", @tableName, " DROP ", @columnName, ";"),
  "SELECT 1"
));
PREPARE alterIfExists FROM @preparedStatement;
EXECUTE alterIfExists;

DEALLOCATE PREPARE alterIfExists;
COMMIT;