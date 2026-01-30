ALTER TABLE monsters
  ADD COLUMN pvp_ranking_worst INT NOT NULL DEFAULT 4096,
  ADD COLUMN pvp_ranking_best INT NOT NULL DEFAULT 1,
  ADD COLUMN pvp_ranking_min_cp INT NOT NULL DEFAULT 1,
  ADD COLUMN pvp_ranking_league INT NOT NULL DEFAULT 0;

UPDATE monsters
  SET pvp_ranking_league = 1500,
      pvp_ranking_worst = great_league_ranking,
      pvp_ranking_min_cp = great_league_ranking_min_cp
  WHERE great_league_ranking < 4096;

UPDATE monsters
  SET pvp_ranking_league = 2500,
      pvp_ranking_worst = ultra_league_ranking,
      pvp_ranking_min_cp = ultra_league_ranking_min_cp
  WHERE ultra_league_ranking < 4096;
