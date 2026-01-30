ALTER TABLE monsters DROP FOREIGN KEY monsters_id_foreign;

SET @poracle_monster_tracking := (
  SELECT index_name
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'monsters'
    AND index_name = 'monsters_tracking'
  LIMIT 1
);
SET @poracle_monster_drop := IF(@poracle_monster_tracking IS NULL, 'SELECT 1', CONCAT('DROP INDEX ', @poracle_monster_tracking, ' ON monsters'));
PREPARE poracle_stmt FROM @poracle_monster_drop;
EXECUTE poracle_stmt;
DEALLOCATE PREPARE poracle_stmt;

ALTER TABLE monsters
  ADD CONSTRAINT monsters_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;
