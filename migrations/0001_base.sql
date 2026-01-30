CREATE TABLE IF NOT EXISTS humans (
  id VARCHAR(255) NOT NULL PRIMARY KEY,
  type VARCHAR(255) NOT NULL,
  name VARCHAR(255) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  area TEXT NOT NULL DEFAULT '[]',
  latitude FLOAT(14,10) NOT NULL DEFAULT 0,
  longitude FLOAT(14,10) NOT NULL DEFAULT 0,
  fails INT NOT NULL DEFAULT 0,
  last_checked DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS monsters (
  id VARCHAR(255) NOT NULL,
  ping TEXT NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  pokemon_id INT NOT NULL,
  distance INT NOT NULL,
  min_iv INT NOT NULL,
  max_iv INT NOT NULL,
  min_cp INT NOT NULL,
  max_cp INT NOT NULL,
  min_level INT NOT NULL,
  max_level INT NOT NULL,
  atk INT NOT NULL,
  def INT NOT NULL,
  sta INT NOT NULL,
  template INT NOT NULL,
  min_weight INT NOT NULL,
  max_weight INT NOT NULL,
  form INT NOT NULL,
  max_atk INT NOT NULL,
  max_def INT NOT NULL,
  max_sta INT NOT NULL,
  gender INT NOT NULL,
  CONSTRAINT monsters_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY monsters_tracking (id, pokemon_id, min_iv, max_iv, min_level, max_level, atk, def, sta, min_weight, max_weight, form, max_atk, max_def, max_sta, gender)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS raid (
  id VARCHAR(255) NOT NULL,
  ping TEXT NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  pokemon_id INT NOT NULL,
  exclusive TINYINT(1) NOT NULL DEFAULT 0,
  template INT NOT NULL,
  distance INT NOT NULL,
  team INT NOT NULL,
  level INT NOT NULL,
  form INT NOT NULL,
  CONSTRAINT raid_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY raid_tracking (id, pokemon_id, exclusive, level, team)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS egg (
  id VARCHAR(255) NOT NULL,
  ping TEXT NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  exclusive TINYINT(1) NOT NULL DEFAULT 0,
  template INT NOT NULL,
  distance INT NOT NULL,
  team INT NOT NULL,
  level INT NOT NULL,
  CONSTRAINT egg_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY egg_tracking (id, team, exclusive, level)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS quest (
  id VARCHAR(255) NOT NULL,
  ping TEXT NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  reward INT NOT NULL,
  template INT NOT NULL,
  shiny TINYINT(1) NOT NULL DEFAULT 0,
  reward_type INT NOT NULL,
  distance INT NOT NULL,
  CONSTRAINT quest_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY quest_tracking (id, reward_type, reward)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS invasion (
  id VARCHAR(255) NOT NULL,
  ping VARCHAR(255) NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  distance INT NOT NULL,
  template INT NOT NULL,
  gender INT NOT NULL,
  grunt_type VARCHAR(255) NOT NULL,
  CONSTRAINT invasion_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY invasion_tracking (id, gender, grunt_type)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS weather (
  id VARCHAR(255) NOT NULL,
  ping TEXT NOT NULL,
  template INT NOT NULL,
  clean TINYINT(1) NOT NULL DEFAULT 0,
  `condition` INT NOT NULL,
  cell VARCHAR(255) NOT NULL,
  CONSTRAINT weather_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE,
  UNIQUE KEY weather_tracking (id, `condition`, cell)
) ENGINE=InnoDB;
