ALTER TABLE humans MODIFY COLUMN disabled_date DATETIME NULL;
UPDATE humans SET disabled_date = NULL;
