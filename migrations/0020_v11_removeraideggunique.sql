ALTER TABLE raid DROP FOREIGN KEY raid_id_foreign;
DROP INDEX raid_tracking ON raid;
ALTER TABLE raid
  ADD CONSTRAINT raid_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

ALTER TABLE egg DROP FOREIGN KEY egg_id_foreign;
DROP INDEX egg_tracking ON egg;
ALTER TABLE egg
  ADD CONSTRAINT egg_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;
