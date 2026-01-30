ALTER TABLE humans
  ADD COLUMN admin_disable TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN disabled_date DATETIME NOT NULL DEFAULT '2021-01-01 00:00:00',
  ADD COLUMN current_profile_no INT NOT NULL DEFAULT 1;

ALTER TABLE monsters
  ADD COLUMN uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  ADD COLUMN profile_no INT NOT NULL DEFAULT 1,
  ADD COLUMN min_time INT NOT NULL DEFAULT 0;
ALTER TABLE monsters DROP FOREIGN KEY monsters_id_foreign;
ALTER TABLE monsters DROP INDEX monsters_tracking;
ALTER TABLE monsters
  ADD CONSTRAINT monsters_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

ALTER TABLE raid
  ADD COLUMN uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  ADD COLUMN profile_no INT NOT NULL DEFAULT 1;
ALTER TABLE raid DROP FOREIGN KEY raid_id_foreign;
ALTER TABLE raid DROP INDEX raid_tracking;
ALTER TABLE raid
  ADD UNIQUE KEY raid_tracking (id, profile_no, pokemon_id, exclusive, level, team);
ALTER TABLE raid
  ADD CONSTRAINT raid_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

ALTER TABLE egg
  ADD COLUMN uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  ADD COLUMN profile_no INT NOT NULL DEFAULT 1;
ALTER TABLE egg DROP FOREIGN KEY egg_id_foreign;
ALTER TABLE egg DROP INDEX egg_tracking;
ALTER TABLE egg
  ADD UNIQUE KEY egg_tracking (id, profile_no, team, exclusive, level);
ALTER TABLE egg
  ADD CONSTRAINT egg_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

ALTER TABLE quest
  ADD COLUMN uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  ADD COLUMN profile_no INT NOT NULL DEFAULT 1;
ALTER TABLE quest DROP FOREIGN KEY quest_id_foreign;
ALTER TABLE quest DROP INDEX quest_tracking;
ALTER TABLE quest
  ADD UNIQUE KEY quest_tracking (id, profile_no, reward_type, reward);
ALTER TABLE quest
  ADD CONSTRAINT quest_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

ALTER TABLE invasion
  ADD COLUMN uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  ADD COLUMN profile_no INT NOT NULL DEFAULT 1;
ALTER TABLE invasion DROP FOREIGN KEY invasion_id_foreign;
ALTER TABLE invasion DROP INDEX invasion_tracking;
ALTER TABLE invasion
  ADD UNIQUE KEY invasion_tracking (id, profile_no, gender, grunt_type);
ALTER TABLE invasion
  ADD CONSTRAINT invasion_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

ALTER TABLE weather
  ADD COLUMN uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  ADD COLUMN profile_no INT NOT NULL DEFAULT 1;
ALTER TABLE weather DROP FOREIGN KEY weather_id_foreign;
ALTER TABLE weather DROP INDEX weather_tracking;
ALTER TABLE weather
  ADD UNIQUE KEY weather_tracking (id, profile_no, `condition`, cell);
ALTER TABLE weather
  ADD CONSTRAINT weather_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS profiles (
  uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  id VARCHAR(255) NOT NULL,
  profile_no INT NOT NULL DEFAULT 1,
  name VARCHAR(255) NOT NULL,
  area TEXT NOT NULL DEFAULT '[]',
  latitude FLOAT(14,10) NOT NULL DEFAULT 0,
  longitude FLOAT(14,10) NOT NULL DEFAULT 0,
  active_hours VARCHAR(255) NOT NULL DEFAULT '[]',
  CONSTRAINT profiles_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY profile_unique (id, profile_no)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS lures (
  uid INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  id VARCHAR(255) NOT NULL,
  profile_no INT NOT NULL DEFAULT 1,
  ping VARCHAR(255) NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  distance INT NOT NULL,
  template INT NOT NULL,
  lure_id INT NOT NULL,
  CONSTRAINT lures_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY lure_tracking (id, profile_no, lure_id)
) ENGINE=InnoDB;
