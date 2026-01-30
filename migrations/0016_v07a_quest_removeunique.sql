ALTER TABLE quest DROP FOREIGN KEY quest_id_foreign;
DROP INDEX quest_tracking ON quest;
ALTER TABLE quest
  ADD CONSTRAINT quest_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;
