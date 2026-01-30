ALTER TABLE monsters
  ADD COLUMN great_league_ranking INT NOT NULL DEFAULT 4096,
  ADD COLUMN great_league_ranking_min_cp INT NOT NULL DEFAULT 0,
  ADD COLUMN ultra_league_ranking INT NOT NULL DEFAULT 4096,
  ADD COLUMN ultra_league_ranking_min_cp INT NOT NULL DEFAULT 0;

ALTER TABLE monsters DROP FOREIGN KEY monsters_id_foreign;
ALTER TABLE monsters DROP INDEX monsters_tracking;

ALTER TABLE monsters
  ADD UNIQUE KEY monsters_tracking (
    id,
    pokemon_id,
    min_iv,
    max_iv,
    min_level,
    max_level,
    atk,
    def,
    sta,
    form,
    gender,
    min_weight,
    great_league_ranking,
    great_league_ranking_min_cp,
    ultra_league_ranking,
    ultra_league_ranking_min_cp
  );

ALTER TABLE monsters
  ADD CONSTRAINT monsters_id_foreign FOREIGN KEY (id) REFERENCES humans(id) ON DELETE CASCADE;
