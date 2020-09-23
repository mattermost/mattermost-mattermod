BEGIN;

SET @dbName = DATABASE();
SET @tableName = "PullRequests";
SET @columnName = "MilestoneNumber";
SET @columnType = "INT";
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE
      (table_name = @tableName)
      AND (table_schema = @dbName)
      AND (column_name = @columnName)
  ) > 0,
  "SELECT 1",
  CONCAT("ALTER TABLE ", @tableName, " ADD ", @columnName, " ", @columnType, ";")
));
PREPARE alterIfNotExists FROM @preparedStatement;
EXECUTE alterIfNotExists;

DEALLOCATE PREPARE alterIfNotExists;

SET @columnName = "MilestoneTitle";
SET @columnType = "VARCHAR(255)";
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE
      (table_name = @tableName)
      AND (table_schema = @dbName)
      AND (column_name = @columnName)
  ) > 0,
  "SELECT 1",
  CONCAT("ALTER TABLE ", @tableName, " ADD ", @columnName, " ", @columnType, ";")
));
PREPARE alterIfNotExists FROM @preparedStatement;
EXECUTE alterIfNotExists;

DEALLOCATE PREPARE alterIfNotExists;
COMMIT;