package command

func buildTrackRows(
	targetID string,
	profileNo int,
	ping string,
	template string,
	distance int,
	clean bool,
	opt trackOptions,
	monsters []trackMonster,
) []map[string]any {
	minIV := opt.MinIV
	if minIV == -1 && minimumIvZero(&opt) {
		minIV = 0
	}

	rows := make([]map[string]any, 0, len(monsters))
	for _, mon := range monsters {
		row := map[string]any{
			"id":                 targetID,
			"profile_no":         profileNo,
			"pokemon_id":         mon.ID,
			"ping":               ping,
			"template":           template,
			"distance":           distance,
			"min_iv":             minIV,
			"max_iv":             opt.MaxIV,
			"min_cp":             opt.MinCP,
			"max_cp":             opt.MaxCP,
			"min_level":          opt.MinLevel,
			"max_level":          opt.MaxLevel,
			"atk":                opt.MinAtk,
			"def":                opt.MinDef,
			"sta":                opt.MinSta,
			"min_weight":         opt.MinWeight,
			"max_weight":         opt.MaxWeight,
			"form":               mon.FormID,
			"max_atk":            opt.MaxAtk,
			"max_def":            opt.MaxDef,
			"max_sta":            opt.MaxSta,
			"gender":             opt.Gender,
			"clean":              boolToInt(clean),
			"min_time":           opt.MinTime,
			"rarity":             opt.MinRarity,
			"max_rarity":         opt.MaxRarity,
			"size":               opt.MinSize,
			"max_size":           opt.MaxSize,
			"pvp_ranking_league": opt.PvpLeague,
			"pvp_ranking_best":   opt.PvpBest,
			"pvp_ranking_worst":  opt.PvpWorst,
			"pvp_ranking_min_cp": opt.PvpMinCP,
			"pvp_ranking_cap":    opt.PvpCap,
		}
		rows = append(rows, row)
	}
	return rows
}
