DROP INDEX monsters_pokemon_id_index ON monsters;
ALTER TABLE monsters
  ADD INDEX monsters_pokemon_id_min_iv_index (pokemon_id, min_iv);
