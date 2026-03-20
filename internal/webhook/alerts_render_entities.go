package webhook

import (
	"fmt"
	"strings"

	"dexter/internal/i18n"
)

type renderDataContext struct {
	p         *Processor
	hook      *Hook
	match     alertMatch
	data      map[string]any
	lat       float64
	lon       float64
	weatherID int
	tr        *i18n.Translator
}

func applyInitialNameRenderData(ctx *renderDataContext) {
	pokemonName := nameOrID(ctx.p, ctx.hook, "pokemon_id")
	ctx.data["pokemon_name"] = translateMaybe(ctx.tr, pokemonName)
	ctx.data["raid_pokemon_name"] = translateMaybe(ctx.tr, pokemonName)
	ctx.data["fullNameEng"] = pokemonName
	ctx.data["fullName"] = translateMaybe(ctx.tr, pokemonName)
	ctx.data["name"] = ctx.data["fullName"]
	ctx.data["nameEng"] = pokemonName
}

func applyPokemonRenderData(ctx *renderDataContext) {
	if ctx.hook.Type != "pokemon" {
		return
	}
	trackDistanceRaw, hasTrackDistance := ctx.match.Row["distance"]
	trackDistance := getInt(trackDistanceRaw)
	ctx.data["trackDistanceM"] = trackDistance
	ctx.data["isDistanceTrack"] = trackDistance > 0
	ctx.data["isAreaTrack"] = hasTrackDistance && trackDistance == 0

	var distance any = ""
	hasUserDistance := false
	userDistanceM := 0
	bearing := ""
	bearingEmoji := ""
	if ctx.match.Target.Lat != 0 || ctx.match.Target.Lon != 0 {
		hasUserDistance = true
		userDistanceM = distanceMeters(ctx.match.Target.Lat, ctx.match.Target.Lon, ctx.lat, ctx.lon)
		distance = userDistanceM
		brng := bearingDegrees(ctx.match.Target.Lat, ctx.match.Target.Lon, ctx.lat, ctx.lon)
		bearing = fmt.Sprintf("%.0f", brng)
		if emojiKey := bearingEmojiKey(brng); emojiKey != "" {
			bearingEmoji = translateMaybe(ctx.tr, lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform))
		}
	}
	ctx.data["distance"] = distance
	ctx.data["hasUserDistance"] = hasUserDistance
	ctx.data["userDistanceM"] = userDistanceM
	ctx.data["bearing"] = bearing
	ctx.data["bearingEmoji"] = bearingEmoji

	formID := hookFormID(ctx.hook.Message)
	ctx.data["formId"] = formID
	if formName := monsterFormName(ctx.p, ctx.data["pokemonId"].(int), formID); formName != "" {
		ctx.data["formNameEng"] = formName
		ctx.data["formName"] = translateMaybe(ctx.tr, formName)
		ctx.data["formname"] = ctx.data["formName"]
	}
	if gen, name, roman := monsterGeneration(ctx.p, ctx.data["pokemonId"].(int), formID); gen != "" {
		ctx.data["generation"] = gen
		ctx.data["generationNameEng"] = name
		ctx.data["generationRoman"] = roman
	}
	d := ctx.p.getData()
	monster := lookupMonster(d, fmt.Sprintf("%d_%d", ctx.data["pokemonId"].(int), formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(d, fmt.Sprintf("%d_0", ctx.data["pokemonId"].(int)))
	}
	if monster == nil {
		monster = lookupMonster(d, fmt.Sprintf("%d", ctx.data["pokemonId"].(int)))
	}
	if monster != nil {
		if stats, ok := monster["stats"].(map[string]any); ok {
			ctx.data["baseStats"] = stats
		}
		applyPokemonEvolutions(ctx.p, ctx.data, ctx.data["pokemonId"].(int), formID, ctx.match.Target.Platform, ctx.tr)
	}
	displayID := getInt(ctx.hook.Message["display_pokemon_id"])
	if displayID > 0 && displayID != ctx.data["pokemonId"].(int) {
		displayForm := getInt(ctx.hook.Message["display_form"])
		displayMonster := lookupMonster(d, fmt.Sprintf("%d_%d", displayID, displayForm))
		if displayMonster == nil && displayForm != 0 {
			displayMonster = lookupMonster(d, fmt.Sprintf("%d_0", displayID))
		}
		if displayMonster == nil {
			displayMonster = lookupMonster(d, fmt.Sprintf("%d", displayID))
		}
		if displayMonster != nil {
			if name := getString(displayMonster["name"]); name != "" {
				ctx.data["disguisePokemonNameEng"] = name
				ctx.data["disguisePokemonName"] = translateMaybe(ctx.tr, name)
			}
			if form, ok := displayMonster["form"].(map[string]any); ok {
				if name := getString(form["name"]); name != "" {
					ctx.data["disguideFormNameEng"] = name
					ctx.data["disguiseFormNameEng"] = name
					ctx.data["disguiseFormName"] = translateMaybe(ctx.tr, name)
				}
			}
		}
	}
	types := monsterTypes(ctx.p, ctx.data["pokemonId"].(int), formID)
	if len(types) > 0 {
		ctx.data["types"] = types
		ctx.data["alteringWeathers"] = alteringWeathers(ctx.p, types, ctx.weatherID)
		boostingWeathers := boostingWeathersForTypes(ctx.p, types)
		if len(boostingWeathers) > 0 {
			ctx.data["boostingWeathers"] = boostingWeathers
			nonBoosting := []int{}
			for id := 1; id <= 7; id++ {
				if !containsInt(boostingWeathers, id) {
					nonBoosting = append(nonBoosting, id)
				}
			}
			ctx.data["nonBoostingWeathers"] = nonBoosting
		}
		typeNames := monsterTypeNames(ctx.p, ctx.data["pokemonId"].(int), formID)
		if len(typeNames) > 0 {
			translated := make([]string, 0, len(typeNames))
			emojis := make([]string, 0, len(typeNames))
			for _, typeName := range typeNames {
				translated = append(translated, translateMaybe(ctx.tr, typeName))
				if _, emojiKey := typeStyle(ctx.p, typeName); emojiKey != "" {
					if emoji := lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform); emoji != "" {
						emojis = append(emojis, translateMaybe(ctx.tr, emoji))
					}
				}
			}
			ctx.data["emoji"] = emojis
			ctx.data["typeNameEng"] = typeNames
			ctx.data["typeName"] = strings.Join(translated, ", ")
			ctx.data["emojiString"] = strings.Join(emojis, "")
			ctx.data["typeEmoji"] = ctx.data["emojiString"]
		}
	}
	if group := rarityGroupForPokemon(ctx.p.stats, ctx.data["pokemonId"].(int)); group >= 0 {
		ctx.data["rarityGroup"] = group
		if name := rarityNameEng(ctx.p, group); name != "" {
			ctx.data["rarityNameEng"] = name
		}
	}
	if size := getInt(ctx.hook.Message["size"]); size > 0 {
		if name := sizeNameEng(ctx.p, size); name != "" {
			ctx.data["sizeNameEng"] = name
		}
	}
	if shiny := shinyStatsForPokemon(ctx.p.stats, ctx.data["pokemonId"].(int)); shiny != nil {
		ctx.data["shinyStats"] = shiny
	}
	if nameEng := getString(ctx.data["nameEng"]); nameEng != "" {
		formNameEng := getString(ctx.data["formNameEng"])
		formNormalisedEng := formNameEng
		if strings.EqualFold(formNormalisedEng, "Normal") {
			formNormalisedEng = ""
		}
		ctx.data["formNormalisedEng"] = formNormalisedEng
		ctx.data["formNormalised"] = translateMaybe(ctx.tr, formNormalisedEng)
		fullNameEng := nameEng
		if formNormalisedEng != "" {
			fullNameEng = fmt.Sprintf("%s %s", nameEng, formNormalisedEng)
		}
		ctx.data["fullNameEng"] = fullNameEng
		fullName := translateMaybe(ctx.tr, nameEng)
		if formNormalised := getString(ctx.data["formNormalised"]); formNormalised != "" {
			fullName = fmt.Sprintf("%s %s", fullName, formNormalised)
		}
		ctx.data["fullName"] = fullName
		ctx.data["name"] = translateMaybe(ctx.tr, nameEng)
	}
	ctx.data["rarityName"] = translateMaybe(ctx.tr, getString(ctx.data["rarityNameEng"]))
	ctx.data["sizeName"] = translateMaybe(ctx.tr, getString(ctx.data["sizeNameEng"]))
	if shinyPossible, ok := ctx.data["shinyPossible"].(bool); ok && shinyPossible {
		ctx.data["shinyPossibleEmoji"] = translateMaybe(ctx.tr, lookupEmojiForPlatform(ctx.p, "shiny", ctx.match.Target.Platform))
	} else {
		ctx.data["shinyPossibleEmoji"] = ""
	}
}

func applyRaidEggMaxBattleRenderData(ctx *renderDataContext) {
	if ctx.hook.Type != "raid" && ctx.hook.Type != "egg" && ctx.hook.Type != "max_battle" {
		return
	}
	pokemonID := getInt(ctx.hook.Message["pokemon_id"])
	if pokemonID <= 0 {
		return
	}
	formID := hookFormID(ctx.hook.Message)
	ctx.data["pokemonId"] = pokemonID
	ctx.data["formId"] = formID
	if ctx.p != nil && ctx.p.shinyPossible != nil {
		ctx.data["shinyPossible"] = ctx.p.shinyPossible.IsPossible(pokemonID, formID)
	} else {
		ctx.data["shinyPossible"] = false
	}
	if shinyPossible, ok := ctx.data["shinyPossible"].(bool); ok && shinyPossible {
		ctx.data["shinyPossibleEmoji"] = translateMaybe(ctx.tr, lookupEmojiForPlatform(ctx.p, "shiny", ctx.match.Target.Platform))
	} else {
		ctx.data["shinyPossibleEmoji"] = ""
	}
	if formName := monsterFormName(ctx.p, pokemonID, formID); formName != "" {
		ctx.data["formNameEng"] = formName
		ctx.data["formName"] = translateMaybe(ctx.tr, formName)
		ctx.data["formname"] = ctx.data["formNameEng"]
	}
	if gen, name, roman := monsterGeneration(ctx.p, pokemonID, formID); gen != "" {
		ctx.data["generation"] = gen
		ctx.data["generationNameEng"] = name
		ctx.data["generationRoman"] = roman
		ctx.data["generationName"] = translateMaybe(ctx.tr, name)
	}
	d2 := ctx.p.getData()
	monster := lookupMonster(d2, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(d2, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(d2, fmt.Sprintf("%d", pokemonID))
	}
	if monster != nil {
		if stats, ok := monster["stats"].(map[string]any); ok {
			ctx.data["baseStats"] = stats
		}
		if name := getString(monster["name"]); name != "" {
			ctx.data["nameEng"] = name
		}
	}
	evolutionID := getInt(ctx.hook.Message["evolution"])
	if evolutionID == 0 {
		evolutionID = getInt(ctx.hook.Message["evolution_id"])
	}
	if evolutionID > 0 {
		ctx.data["evolution"] = evolutionID
	}
	ctx.data["evolutionNameEng"] = evolutionName(ctx.p, evolutionID)
	ctx.data["evolutionName"] = translateMaybe(ctx.tr, getString(ctx.data["evolutionNameEng"]))
	ctx.data["evolutionname"] = ctx.data["evolutionNameEng"]
	ctx.data["quickMoveId"] = getInt(ctx.hook.Message["move_1"])
	ctx.data["chargeMoveId"] = getInt(ctx.hook.Message["move_2"])
	ctx.data["move_1"] = ctx.data["quickMoveId"]
	ctx.data["move_2"] = ctx.data["chargeMoveId"]
	ctx.data["move1"] = ctx.data["quickMoveName"]
	ctx.data["move2"] = ctx.data["chargeMoveName"]

	nameEng := getString(ctx.data["nameEng"])
	formNameEng := getString(ctx.data["formNameEng"])
	formNormalisedEng := formNameEng
	if strings.EqualFold(formNormalisedEng, "Normal") {
		formNormalisedEng = ""
	}
	ctx.data["formNormalisedEng"] = formNormalisedEng
	ctx.data["formNormalised"] = translateMaybe(ctx.tr, formNormalisedEng)
	fullNameEng := nameEng
	if formNormalisedEng != "" {
		fullNameEng = fmt.Sprintf("%s %s", nameEng, formNormalisedEng)
	}
	fullName := translateMaybe(ctx.tr, nameEng)
	if formNormalised := getString(ctx.data["formNormalised"]); formNormalised != "" {
		fullName = fmt.Sprintf("%s %s", fullName, formNormalised)
	}
	if evolutionID > 0 {
		if format := megaNameFormat(ctx.p, evolutionID); format != "" {
			fullNameEng = formatTemplate(format, nameEng)
			fullName = formatTemplate(format, translateMaybe(ctx.tr, nameEng))
			ctx.data["megaName"] = fullName
		}
	}
	ctx.data["fullNameEng"] = fullNameEng
	ctx.data["fullName"] = fullName
	ctx.data["name"] = translateMaybe(ctx.tr, nameEng)
	ctx.data["pokemonName"] = ctx.data["fullName"]

	types := monsterTypes(ctx.p, pokemonID, formID)
	typeNames := monsterTypeNames(ctx.p, pokemonID, formID)
	if len(types) > 0 {
		ctx.data["types"] = types
	}
	if len(typeNames) > 0 {
		translated := make([]string, 0, len(typeNames))
		emojis := make([]string, 0, len(typeNames))
		for _, typeName := range typeNames {
			translated = append(translated, translateMaybe(ctx.tr, typeName))
			if _, emojiKey := typeStyle(ctx.p, typeName); emojiKey != "" {
				if emoji := lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform); emoji != "" {
					emojis = append(emojis, translateMaybe(ctx.tr, emoji))
				}
			}
		}
		ctx.data["typeNameEng"] = typeNames
		ctx.data["emoji"] = emojis
		ctx.data["typeName"] = strings.Join(translated, ", ")
		ctx.data["typeEmoji"] = strings.Join(emojis, "")
	}
	boostingWeathers := boostingWeathersForTypes(ctx.p, types)
	ctx.data["boostingWeathers"] = boostingWeathers
	weatherEmojis := []string{}
	for _, weatherID := range boostingWeathers {
		_, emojiKey := weatherEntry(ctx.p, weatherID)
		if emojiKey != "" {
			if emoji := lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform); emoji != "" {
				weatherEmojis = append(weatherEmojis, translateMaybe(ctx.tr, emoji))
			}
		}
	}
	ctx.data["boostingWeathersEmoji"] = strings.Join(weatherEmojis, "")
	ctx.data["boosted"] = containsInt(boostingWeathers, ctx.weatherID)
	if ctx.data["boosted"].(bool) {
		name, emojiKey := weatherEntry(ctx.p, ctx.weatherID)
		ctx.data["boostWeatherNameEng"] = name
		ctx.data["boostWeatherId"] = ctx.weatherID
		ctx.data["boostWeatherName"] = translateMaybe(ctx.tr, name)
		ctx.data["boostWeatherEmoji"] = translateMaybe(ctx.tr, lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform))
	} else {
		ctx.data["boostWeatherNameEng"] = ""
		ctx.data["boostWeatherId"] = 0
		ctx.data["boostWeatherName"] = ""
		ctx.data["boostWeatherEmoji"] = ""
	}
	weaknessList, weaknessEmoji := weaknessListForTypes(ctx.p, typeNames, ctx.match.Target.Platform, ctx.tr)
	if len(weaknessList) > 0 {
		ctx.data["weaknessList"] = weaknessList
		ctx.data["weaknessEmoji"] = weaknessEmoji
	}
}

func applyPokemonPvpRenderData(ctx *renderDataContext) {
	if ctx.hook.Type != "pokemon" {
		return
	}
	filters := pvpFiltersFromRow(ctx.match.Row)
	filterByTrack := getBoolFromConfig(ctx.p.cfg, "pvp.filterByTrack", false)
	displayMaxRank := getIntFromConfig(ctx.p.cfg, "pvp.pvpDisplayMaxRank", 10)
	displayGreatMin := getIntFromConfig(ctx.p.cfg, "pvp.pvpDisplayGreatMinCP", 0)
	displayUltraMin := getIntFromConfig(ctx.p.cfg, "pvp.pvpDisplayUltraMinCP", 0)
	displayLittleMin := getIntFromConfig(ctx.p.cfg, "pvp.pvpDisplayLittleMinCP", 0)
	ctx.data["pvpDisplayMaxRank"] = displayMaxRank
	ctx.data["pvpDisplayGreatMinCP"] = displayGreatMin
	ctx.data["pvpDisplayUltraMinCP"] = displayUltraMin
	ctx.data["pvpDisplayLittleMinCP"] = displayLittleMin
	ctx.data["pvpGreat"] = pvpDisplayList(ctx.p, ctx.hook.Message["pvp_rankings_great_league"], 1500, displayMaxRank, displayGreatMin, filters, filterByTrack, ctx.tr)
	ctx.data["pvpUltra"] = pvpDisplayList(ctx.p, ctx.hook.Message["pvp_rankings_ultra_league"], 2500, displayMaxRank, displayUltraMin, filters, filterByTrack, ctx.tr)
	ctx.data["pvpLittle"] = pvpDisplayList(ctx.p, ctx.hook.Message["pvp_rankings_little_league"], 500, displayMaxRank, displayLittleMin, filters, filterByTrack, ctx.tr)
	ctx.data["pvpGreatBest"] = pvpBestInfo(ctx.data["pvpGreat"])
	ctx.data["pvpUltraBest"] = pvpBestInfo(ctx.data["pvpUltra"])
	ctx.data["pvpLittleBest"] = pvpBestInfo(ctx.data["pvpLittle"])
	ctx.data["pvpAvailable"] = ctx.data["pvpGreat"] != nil || ctx.data["pvpUltra"] != nil || ctx.data["pvpLittle"] != nil
	ctx.data["userHasPvpTracks"] = len(filters) > 0
	userRanking := getInt(ctx.match.Row["pvp_ranking_worst"])
	if userRanking == 4096 {
		userRanking = 0
	}
	ctx.data["pvpUserRanking"] = userRanking

	capsConsidered := pvpCapsFromConfig(ctx.p.cfg)
	ctx.data["pvpBestRank"] = map[string]any{}
	ctx.data["pvpEvolutionData"] = map[string]any{}
	maxRank := getIntFromConfig(ctx.p.cfg, "pvp.pvpFilterMaxRank", 4096)
	greatMin := getIntFromConfig(ctx.p.cfg, "pvp.pvpFilterGreatMinCP", 0)
	ultraMin := getIntFromConfig(ctx.p.cfg, "pvp.pvpFilterUltraMinCP", 0)
	littleMin := getIntFromConfig(ctx.p.cfg, "pvp.pvpFilterLittleMinCP", 0)
	evoEnabled, _ := ctx.p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")

	pvpBestRank := ctx.data["pvpBestRank"].(map[string]any)
	pvpEvolution := ctx.data["pvpEvolutionData"].(map[string]any)
	pokemonID := ctx.data["pokemonId"].(int)

	bestUltra, bestUltraCP := pvpRankSummary(capsConsidered, 2500, ctx.hook.Message["pvp_rankings_ultra_league"], pokemonID, evoEnabled, ultraMin, maxRank, pvpEvolution)
	bestGreat, bestGreatCP := pvpRankSummary(capsConsidered, 1500, ctx.hook.Message["pvp_rankings_great_league"], pokemonID, evoEnabled, greatMin, maxRank, pvpEvolution)
	bestLittle, bestLittleCP := pvpRankSummary(capsConsidered, 500, ctx.hook.Message["pvp_rankings_little_league"], pokemonID, evoEnabled, littleMin, maxRank, pvpEvolution)

	pvpBestRank["2500"] = bestUltra
	pvpBestRank["1500"] = bestGreat
	pvpBestRank["500"] = bestLittle
	ctx.data["bestUltraLeagueRank"] = bestUltraCP.rank
	ctx.data["bestUltraLeagueRankCP"] = bestUltraCP.cp
	ctx.data["bestGreatLeagueRank"] = bestGreatCP.rank
	ctx.data["bestGreatLeagueRankCP"] = bestGreatCP.cp
	ctx.data["bestLittleLeagueRank"] = bestLittleCP.rank
	ctx.data["bestLittleLeagueRankCP"] = bestLittleCP.cp
}
