DROP INDEX monsters_pokemon_id_min_iv_index ON monsters;
ALTER TABLE monsters
  ADD INDEX monsters_pvp_ranking_league_pokemon_id_min_iv_index (pvp_ranking_league, pokemon_id, min_iv),
  ADD INDEX monsters_pvp_ranking_league_pokemon_id_pvp_ranking_worst_index (pvp_ranking_league, pokemon_id, pvp_ranking_worst);
