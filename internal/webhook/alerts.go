package webhook

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/golang/geo/s2"

	"dexter/internal/geo"
	"dexter/internal/render"
)

func buildRenderData(p *Processor, hook *Hook, match alertMatch) map[string]any {
	data := make(map[string]any, 128)
	base := applyBaseRenderData(p, hook, match, data)
	ctx := renderDataContext{
		p:         p,
		hook:      hook,
		match:     match,
		data:      data,
		lat:       base.lat,
		lon:       base.lon,
		weatherID: base.weatherID,
		tr:        translatorFor(p, match.Target.Language),
	}
	lat, lon := ctx.lat, ctx.lon
	weatherID := ctx.weatherID
	tr := ctx.tr
	applyInitialNameRenderData(&ctx)
	applyPokemonRenderData(&ctx)
	applyRaidEggMaxBattleRenderData(&ctx)
	if _, ok := data["color"]; !ok || getString(data["color"]) == "" {
		if color := pokemonTypeColor(p, hook); color != "" {
			data["color"] = color
		}
	}
	data["mapurl"] = data["googleMap"]
	data["googleMapUrl"] = data["googleMap"]
	data["appleMapUrl"] = appleMapURL(lat, lon)
	data["applemap"] = data["appleMapUrl"]
	data["wazeMapUrl"] = wazeMapURL(lat, lon)
	data["time"] = hookTime(p, hook)
	expireForTTH := hookExpiryUnix(hook)
	if hook.Type == "egg" {
		start := hookEggStart(hook.Message)
		if start > 0 {
			expireForTTH = start
		}
	}
	if (hook.Type == "gym" || hook.Type == "gym_details") && expireForTTH <= 0 {
		expireForTTH = time.Now().Unix() + 3600
	}
	applyTTH(data, expireForTTH)
	if hook.Type == "egg" {
		start := hookEggStart(hook.Message)
		if start > 0 {
			data["hatchTime"] = formatUnixInHookLocation(p, hook, start, configTimeLayout(p))
			data["hatchtime"] = data["hatchTime"]
			data["time"] = data["hatchTime"]
			applyTTH(data, start)
		}
	}
	if value := getInt64(hook.Message["disappear_time"]); value > 0 {
		data["disappear_time"] = value
	}
	if p != nil && p.cfg != nil {
		data["reactMapUrl"] = reactMapURL(p.cfg, hook)
		data["diademUrl"] = diademURL(p.cfg, hook)
		data["nightTime"] = false
		data["dawnTime"] = false
		data["duskTime"] = false
		data["style"] = getStringFromConfig(p.cfg, "geocoding.dayStyle", "klokantech-basic")
	}
	applyNightTime(p, hook, data)
	if p != nil && p.geocoder != nil {
		data["intersection"] = p.geocoder.Intersection(lat, lon)
		if details := p.geocoder.ReverseDetails(lat, lon); details != nil {
			data["address"] = details.FormattedAddress
			data["addr"] = details.FormattedAddress
			data["formattedAddress"] = details.FormattedAddress
			data["streetName"] = details.StreetName
			data["streetNumber"] = details.StreetNumber
			data["city"] = details.City
			data["country"] = details.Country
			data["state"] = details.State
			data["zipcode"] = details.Zipcode
			data["countryCode"] = details.CountryCode
			data["neighbourhood"] = details.Neighbourhood
			data["suburb"] = details.Suburb
			data["town"] = details.Town
			data["village"] = details.Village
			data["shop"] = details.Shop
			data["flag"] = countryCodeEmoji(details.CountryCode)
		} else {
			address := p.geocoder.Reverse(lat, lon)
			data["address"] = address
			data["addr"] = data["address"]
			data["formattedAddress"] = address
		}
	}
	if _, ok := data["flag"]; !ok {
		data["flag"] = ""
	}
	if getString(data["addr"]) == "" {
		data["addr"] = "Unknown"
	}
	if getString(data["address"]) == "" {
		data["address"] = getString(data["addr"])
	}
	if _, ok := data["formattedAddress"]; !ok {
		data["formattedAddress"] = getString(data["address"])
	}
	if p != nil && p.cfg != nil {
		if format := getStringFromConfig(p.cfg, "locale.addressFormat", ""); format != "" {
			if rendered, err := render.RenderHandlebars(format, data, nil); err == nil && strings.TrimSpace(rendered) != "" {
				data["addr"] = rendered
			}
		}
	}
	if hook.Type == "weather" && p != nil && p.weather != nil {
		data["weather_summary"] = p.weather.Summary(lat, lon)
	}
	encountered := hasNumeric(hook.Message["individual_attack"]) && hasNumeric(hook.Message["individual_defense"]) && hasNumeric(hook.Message["individual_stamina"])
	ivString := "-1"
	if encountered {
		if ivValue := computeIV(hook); ivValue >= 0 {
			ivString = fmt.Sprintf("%.2f", ivValue)
		}
	}
	data["iv"] = ivString
	ivPercent := normalizeIV(hook, ivString)
	data["ivPercent"] = ivPercent
	if ivPercent != "-1" {
		if ivInt, err := strconv.Atoi(ivPercent); err == nil {
			data["ivPercent"] = ivInt
		}
	}
	if encountered {
		data["atk"] = getInt(hook.Message["individual_attack"])
		data["def"] = getInt(hook.Message["individual_defense"])
		data["sta"] = getInt(hook.Message["individual_stamina"])
		data["cp"] = getInt(hook.Message["cp"])
		data["quickMoveId"] = getInt(hook.Message["move_1"])
		data["chargeMoveId"] = getInt(hook.Message["move_2"])
		if level := getInt(hook.Message["pokemon_level"]); level > 0 {
			data["level"] = level
		} else {
			data["level"] = 0
		}
	} else {
		data["atk"] = 0
		data["def"] = 0
		data["sta"] = 0
		data["cp"] = 0
		data["quickMoveId"] = 0
		data["chargeMoveId"] = 0
		data["level"] = 0
	}
	data["ivColor"] = ivColor(data["iv"])
	data["verified"] = getBool(hook.Message["verified"]) || getBool(hook.Message["disappear_time_verified"]) || getBool(hook.Message["confirmed"])
	data["disappear_time_verified"] = data["verified"]
	data["confirmed"] = data["verified"]
	data["confirmedTime"] = data["verified"]
	data["imgUrl"] = selectImageURL(p, hook)
	if getString(data["imgUrl"]) == "" && p != nil && p.cfg != nil {
		data["imgUrl"] = getStringFromConfig(p.cfg, "fallbacks.imgUrl", "")
	}
	data["imgUrlAlt"] = selectImageURLAlt(p, hook)
	data["stickerUrl"] = selectStickerURL(p, hook)
	data["quickMoveName"] = translateMaybe(tr, moveName(p, getInt(hook.Message["move_1"])))
	data["chargeMoveName"] = translateMaybe(tr, moveName(p, getInt(hook.Message["move_2"])))
	data["quickMoveNameEng"] = moveName(p, getInt(hook.Message["move_1"]))
	data["chargeMoveNameEng"] = moveName(p, getInt(hook.Message["move_2"]))
	data["quickMoveEmoji"] = moveEmoji(p, getInt(hook.Message["move_1"]), match.Target.Platform, tr)
	data["chargeMoveEmoji"] = moveEmoji(p, getInt(hook.Message["move_2"]), match.Target.Platform, tr)
	data["quickMove"] = data["quickMoveName"]
	data["chargeMove"] = data["chargeMoveName"]
	data["move1emoji"] = data["quickMoveEmoji"]
	data["move2emoji"] = data["chargeMoveEmoji"]
	data["individual_attack"] = data["atk"]
	data["individual_defense"] = data["def"]
	data["individual_stamina"] = data["sta"]
	data["pokemon_level"] = data["level"]
	data["move_1"] = data["quickMoveId"]
	data["move_2"] = data["chargeMoveId"]
	if height := getFloat(hook.Message["height"]); height > 0 && encountered {
		data["height"] = fmt.Sprintf("%.2f", height)
	} else {
		data["height"] = "0"
	}
	if weight := getFloat(hook.Message["weight"]); weight > 0 && encountered {
		data["weight"] = fmt.Sprintf("%.2f", weight)
	} else {
		data["weight"] = "0"
	}
	data["size"] = getInt(hook.Message["size"])
	if encountered {
		if baseCatch := getFloat(hook.Message["base_catch"]); baseCatch > 0 {
			data["capture_1"] = baseCatch
			data["catchBase"] = fmt.Sprintf("%.2f", baseCatch*100)
		} else {
			data["catchBase"] = "0"
		}
		if greatCatch := getFloat(hook.Message["great_catch"]); greatCatch > 0 {
			data["capture_2"] = greatCatch
			data["catchGreat"] = fmt.Sprintf("%.2f", greatCatch*100)
		} else {
			data["catchGreat"] = "0"
		}
		if ultraCatch := getFloat(hook.Message["ultra_catch"]); ultraCatch > 0 {
			data["capture_3"] = ultraCatch
			data["catchUltra"] = fmt.Sprintf("%.2f", ultraCatch*100)
		} else {
			data["catchUltra"] = "0"
		}
	} else {
		data["catchBase"] = "0"
		data["catchGreat"] = "0"
		data["catchUltra"] = "0"
	}
	data["gymName"] = gymName(hook)
	data["pokestopName"] = getString(hook.Message["pokestop_name"])
	if hook.Type == "gym" || hook.Type == "gym_details" {
		if name := getString(data["gymName"]); name != "" {
			data["name"] = name
		}
	}
	if hook.Type == "quest" || hook.Type == "lure" || hook.Type == "invasion" || hook.Type == "pokestop" {
		if name := getString(data["pokestopName"]); name != "" {
			data["name"] = name
		}
	}
	if hook.Type == "quest" || hook.Type == "lure" || hook.Type == "pokestop" {
		pokestopURL := getString(hook.Message["pokestop_url"])
		if pokestopURL == "" {
			pokestopURL = getString(hook.Message["url"])
		}
		if pokestopURL == "" && p != nil && p.cfg != nil {
			pokestopURL = getStringFromConfig(p.cfg, "fallbacks.pokestopUrl", "")
		}
		if pokestopURL != "" {
			data["pokestopUrl"] = pokestopURL
			data["pokestop_url"] = pokestopURL
			data["url"] = pokestopURL
		}
	}
	if hook.Type == "pokemon" && getString(data["pokestopName"]) == "" && p != nil && p.cfg != nil {
		if enabled, _ := p.cfg.GetBool("general.populatePokestopName"); enabled && p.scanner != nil {
			stopID := getString(hook.Message["pokestop_id"])
			if stopID != "" {
				if name, err := p.scanner.GetPokestopName(stopID); err == nil && name != "" {
					data["pokestopName"] = name
				}
			}
		}
	}
	data["teamName"], data["teamColor"] = teamInfo(teamFromHookMessage(hook.Message))
	data["slotsAvailable"] = getInt(hook.Message["slots_available"])
	data["previousControlName"], _ = teamInfo(getInt(hook.Message["old_team_id"]))
	data["gymColor"] = data["teamColor"]
	level := getInt(hook.Message["level"])
	if level == 0 {
		level = getInt(hook.Message["raid_level"])
	}
	if level > 0 {
		data["level"] = level
	}
	data["levelName"] = fmt.Sprintf("Level %d", level)
	if hook.Type == "raid" || hook.Type == "egg" || hook.Type == "gym" || hook.Type == "gym_details" {
		teamID := teamFromHookMessage(hook.Message)
		data["team_id"] = teamID
		if getString(data["gym_name"]) == "" {
			data["gym_name"] = getString(data["gymName"])
		}
		teamNameEng, teamEmojiKey, teamColor := teamDetails(p, teamID)
		if hook.Type == "egg" && teamID == 0 {
			teamNameEng = "Harmony"
		}
		if teamNameEng != "" {
			data["teamNameEng"] = teamNameEng
			data["teamName"] = translateMaybe(tr, teamNameEng)
		}
		if teamEmojiKey != "" {
			data["teamEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, teamEmojiKey, match.Target.Platform))
		}
		if teamColor != 0 {
			data["gymColor"] = teamColor
		}
		if hook.Type == "raid" || hook.Type == "egg" {
			data["color"] = data["gymColor"]
		}
		if hook.Type == "gym" || hook.Type == "gym_details" {
			data["gymId"] = getString(hook.Message["id"])
			if data["gymId"] == "" {
				data["gymId"] = getString(hook.Message["gym_id"])
			}
			data["oldTeamId"] = getInt(hook.Message["old_team_id"])
			data["previousControlId"] = getInt(hook.Message["last_owner_id"])
			data["oldSlotsAvailable"] = getInt(hook.Message["old_slots_available"])
			data["trainerCount"] = 6 - getInt(hook.Message["slots_available"])
			data["oldTrainerCount"] = 6 - getInt(hook.Message["old_slots_available"])
			data["inBattle"] = gymInBattle(hook.Message)
			oldTeamNameEng, _, _ := teamDetails(p, getInt(data["oldTeamId"]))
			prevTeamNameEng, prevTeamEmojiKey, _ := teamDetails(p, getInt(data["previousControlId"]))
			data["teamNameEng"] = teamNameEng
			data["teamName"] = translateMaybe(tr, teamNameEng)
			data["teamEmojiEng"] = lookupEmojiForPlatform(p, teamEmojiKey, match.Target.Platform)
			data["teamEmoji"] = translateMaybe(tr, getString(data["teamEmojiEng"]))
			data["oldTeamNameEng"] = oldTeamNameEng
			data["oldTeamName"] = translateMaybe(tr, oldTeamNameEng)
			data["previousControlNameEng"] = prevTeamNameEng
			data["previousControlName"] = translateMaybe(tr, prevTeamNameEng)
			data["previousControlTeamEmojiEng"] = lookupEmojiForPlatform(p, prevTeamEmojiKey, match.Target.Platform)
			data["previousControlTeamEmoji"] = translateMaybe(tr, getString(data["previousControlTeamEmojiEng"]))
			data["color"] = data["gymColor"]
			if loc := hookLocation(p, hook); loc != nil {
				data["conqueredTime"] = time.Now().In(loc).Format(configTimeLayout(p))
			} else {
				data["conqueredTime"] = time.Now().Format("15:04:05")
			}
			if getInt(data["tthSeconds"]) == 0 {
				data["tthh"] = 1
				data["tthm"] = 0
				data["tths"] = 0
				data["tth"] = map[string]any{"hours": 1, "minutes": 0, "seconds": 0}
				data["tthSeconds"] = 3600
			}
		}
		if level > 0 {
			levelNameEng := raidLevelName(p, level)
			data["levelNameEng"] = levelNameEng
			data["levelName"] = translateMaybe(tr, levelNameEng)
		}
	}
	if hook.Type == "max_battle" {
		level := getInt(hook.Message["battle_level"])
		if level == 0 {
			level = getInt(hook.Message["level"])
		}
		if level > 0 {
			levelNameEng := maxbattleLevelName(p, level)
			data["levelNameEng"] = levelNameEng
			data["levelName"] = translateMaybe(tr, levelNameEng)
		}
	}
	data["weatherName"], data["weatherEmoji"] = weatherInfo(p, weatherID, match.Target.Platform, tr)
	data["boosted"] = weatherID > 0
	if weatherID > 0 {
		data["boostWeatherId"] = weatherID
		data["boostWeatherName"] = data["weatherName"]
		data["boostWeatherEmoji"] = data["weatherEmoji"]
		data["boost"] = data["boostWeatherName"]
		data["boostemoji"] = data["boostWeatherEmoji"]
	} else {
		data["boostWeatherId"] = ""
		data["boostWeatherName"] = ""
		data["boostWeatherEmoji"] = ""
		data["boost"] = ""
		data["boostemoji"] = ""
	}
	data["oldWeatherName"] = ""
	data["oldWeatherEmoji"] = ""
	if hook.Type == "weather" {
		// PoracleJS exposes `condition` (usually sourced from `gameplay_condition`) for templates.
		data["condition"] = weatherID
		cellID := getString(hook.Message["s2_cell_id"])
		if cellID == "" {
			cellID = geo.WeatherCellID(lat, lon)
		}
		if cellID != "" {
			data["weatherCellId"] = cellID
		}
		data["weatherId"] = weatherID
		data["weatherNameEng"] = ""
		data["weatherEmojiEng"] = ""
		if weatherID > 0 {
			nameEng, emojiKey := weatherEntry(p, weatherID)
			data["weatherNameEng"] = nameEng
			data["weatherName"] = translateMaybe(tr, nameEng)
			if emojiKey != "" {
				data["weatherEmojiEng"] = lookupEmojiForPlatform(p, emojiKey, match.Target.Platform)
				data["weatherEmoji"] = translateMaybe(tr, getString(data["weatherEmojiEng"]))
			}
		}
		oldWeather := 0
		if p != nil && p.weatherData != nil && cellID != "" {
			timestamp := getInt64(hook.Message["time_changed"])
			if timestamp == 0 {
				timestamp = getInt64(hook.Message["updated"])
			}
			if timestamp == 0 {
				timestamp = time.Now().Unix()
			}
			updateHour := timestamp - (timestamp % 3600)
			prevHour := updateHour - 3600
			if cell := p.weatherData.WeatherInfo(cellID); cell != nil {
				oldWeather = cell.Data[prevHour]
			}
		}
		if oldWeather > 0 {
			data["oldWeatherId"] = oldWeather
			oldNameEng, oldEmojiKey := weatherEntry(p, oldWeather)
			data["oldWeatherNameEng"] = oldNameEng
			data["oldWeatherName"] = translateMaybe(tr, oldNameEng)
			if oldEmojiKey != "" {
				data["oldWeatherEmojiEng"] = lookupEmojiForPlatform(p, oldEmojiKey, match.Target.Platform)
				data["oldWeatherEmoji"] = translateMaybe(tr, getString(data["oldWeatherEmojiEng"]))
			}
		} else {
			data["oldWeatherId"] = ""
			data["oldWeatherNameEng"] = ""
			data["oldWeatherEmojiEng"] = ""
		}
		data["weather"] = data["weatherName"]
		data["weatheremoji"] = data["weatherEmoji"]
		data["oldweather"] = data["oldWeatherName"]
		data["oldweatheremoji"] = data["oldWeatherEmoji"]
		if p != nil && p.cfg != nil && p.weatherData != nil {
			showAltered := getBoolFromConfig(p.cfg, "weather.showAlteredPokemon", false)
			if showAltered && weatherID > 0 {
				maxCount := getIntFromConfig(p.cfg, "weather.showAlteredPokemonMaxCount", 0)
				active := p.weatherData.ActivePokemons(cellID, match.Target.ID, weatherID, maxCount)
				base := imageBaseURL(p.cfg, "pokemon", "general.images", "general.imgUrl")
				client := uiconsClient(base, "png")
				activeViews := make([]map[string]any, 0, len(active))
				for _, mon := range active {
					formNormalisedEng := mon.FormName
					if strings.EqualFold(formNormalisedEng, "Normal") {
						formNormalisedEng = ""
					}
					fullNameEng := joinNonEmpty([]string{mon.Name, formNormalisedEng})
					entry := map[string]any{
						"pokemon_id":        mon.PokemonID,
						"form":              mon.Form,
						"nameEng":           mon.Name,
						"name":              translateMaybe(tr, mon.Name),
						"formNameEng":       mon.FormName,
						"formName":          translateMaybe(tr, mon.FormName),
						"formNormalisedEng": formNormalisedEng,
						"formNormalised":    translateMaybe(tr, formNormalisedEng),
						"fullNameEng":       fullNameEng,
						"iv":                mon.IV,
						"cp":                mon.CP,
						"latitude":          mon.Latitude,
						"longitude":         mon.Longitude,
						"disappear_time":    mon.DisappearTime,
						"alteringWeathers":  mon.AlteringWeathers,
					}
					if client != nil {
						if url, ok := client.PokemonIcon(mon.PokemonID, mon.Form, 0, 0, 0, 0, false, 0); ok {
							entry["imgUrl"] = url
						}
					}
					activeViews = append(activeViews, entry)
				}
				data["activePokemons"] = activeViews
			}
		}
		if cellID != "" {
			if cellInt, err := strconv.ParseUint(cellID, 10, 64); err == nil {
				cell := s2.CellFromCellID(s2.CellID(cellInt))
				coords := make([][]float64, 0, 4)
				for i := 0; i < 4; i++ {
					ll := s2.LatLngFromPoint(cell.Vertex(i))
					coords = append(coords, []float64{ll.Lat.Degrees(), ll.Lng.Degrees()})
				}
				data["coords"] = coords
				data["cell_coords"] = coords
			}
		}
	}
	data["pokemonSpawnAvg"] = getFloat(hook.Message["average_spawns"])
	if data["pokemonSpawnAvg"] == 0 {
		data["pokemonSpawnAvg"] = getFloat(hook.Message["pokemon_spawn_avg"])
	}
	if data["pokemonSpawnAvg"] == 0 {
		data["pokemonSpawnAvg"] = getFloat(hook.Message["pokemon_avg"])
	}
	if count := getInt(hook.Message["pokemon_count"]); count > 0 {
		data["pokemonCount"] = count
	}
	if hook.Type == "raid" || hook.Type == "egg" {
		rsvps := normalizeRSVPList(hook.Message["rsvps"])
		if len(rsvps) > 0 {
			nowMs := time.Now().UnixMilli()
			out := make([]map[string]any, 0, len(rsvps))
			layout := configTimeLayout(p)
			for _, entry := range rsvps {
				timeslot := getInt64(entry["timeslot"])
				if timeslot == 0 || timeslot <= nowMs {
					continue
				}
				entry["timeSlot"] = int64(math.Ceil(float64(timeslot) / 1000))
				entry["time"] = formatUnixInHookLocation(p, hook, timeslot/1000, layout)
				entry["goingCount"] = getInt(entry["going_count"])
				entry["maybeCount"] = getInt(entry["maybe_count"])
				out = append(out, entry)
			}
			data["rsvps"] = out
		}
	}
	if hook.Type == "nest" {
		pokemonID := getInt(hook.Message["pokemon_id"])
		if pokemonID > 0 {
			formID := hookFormID(hook.Message)
			data["pokemonId"] = pokemonID
			data["formId"] = formID
			if monster := lookupMonster(p.getData(), fmt.Sprintf("%d_%d", pokemonID, formID)); monster != nil {
				if name := getString(monster["name"]); name != "" {
					data["nameEng"] = name
				}
			}
			if formName := monsterFormName(p, pokemonID, formID); formName != "" {
				data["formNameEng"] = formName
				data["formName"] = translateMaybe(tr, formName)
			}
			data["name"] = translateMaybe(tr, getString(data["nameEng"]))
			fullNameEng := getString(data["nameEng"])
			if formNameEng := getString(data["formNameEng"]); formNameEng != "" && !strings.EqualFold(formNameEng, "Normal") {
				fullNameEng = fmt.Sprintf("%s %s", fullNameEng, formNameEng)
			}
			data["fullNameEng"] = fullNameEng
			fullName := translateMaybe(tr, getString(data["nameEng"]))
			if formName := getString(data["formName"]); formName != "" && !strings.EqualFold(formName, "Normal") {
				fullName = fmt.Sprintf("%s %s", fullName, formName)
			}
			data["fullName"] = fullName
			if shinyPossible, ok := data["shinyPossible"].(bool); ok && shinyPossible {
				data["shinyPossibleEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, "shiny", match.Target.Platform))
			}
		}
	}
	if reset := getInt64(hook.Message["reset_time"]); reset > 0 {
		dateLayout := "2006-01-02"
		timeLayout := "15:04:05"
		if p != nil && p.cfg != nil {
			if format := getStringFromConfig(p.cfg, "locale.date", ""); format != "" {
				dateLayout = momentFormatToGoLayout(format)
			}
			if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
				timeLayout = momentFormatToGoLayout(format)
			}
		}
		data["resetDate"] = formatUnixInHookLocation(p, hook, reset, dateLayout)
		if hook.Type == "nest" || hook.Type == "fort_update" {
			data["resetTime"] = formatUnixInHookLocation(p, hook, reset, timeLayout)
			expire := reset + 7*24*60*60
			data["disappearDate"] = formatUnixInHookLocation(p, hook, expire, dateLayout)
		}
	}
	data["questStringEng"] = questString(p, hook, "en", nil)
	data["questString"] = questString(p, hook, match.Target.Language, tr)
	data["rewardStringEng"] = rewardString(p, hook, nil)
	data["rewardString"] = rewardString(p, hook, tr)
	if hook.Type == "quest" {
		rewardDataNoAR := questRewardData(p, hook)
		rewardDataAR := questRewardDataAR(p, hook)
		primaryRewardData := rewardDataNoAR
		if questRewardDataIsEmpty(primaryRewardData) && !questRewardDataIsEmpty(rewardDataAR) {
			primaryRewardData = rewardDataAR
		}
		data["withAR"] = getBool(hook.Message["with_ar"])
		data["rewardData"] = primaryRewardData
		data["rewardDataNoAR"] = rewardDataNoAR
		data["rewardDataAR"] = rewardDataAR
		data["rewardStringNoAREng"] = questRewardStringFromData(p, rewardDataNoAR, nil)
		data["rewardStringNoAR"] = questRewardStringFromData(p, rewardDataNoAR, tr)
		data["rewardStringAREng"] = questRewardStringFromData(p, rewardDataAR, nil)
		data["rewardStringAR"] = questRewardStringFromData(p, rewardDataAR, tr)
		data["hasQuestNoAR"] = strings.TrimSpace(getString(data["rewardStringNoAR"])) != ""
		data["hasQuestAR"] = strings.TrimSpace(getString(data["rewardStringAR"])) != ""
		applyQuestRewardDetails(p, data, primaryRewardData, match.Target.Platform, tr)
		applyQuestRewardImages(p, data, primaryRewardData)
	}
	if hook.Type == "fort_update" {
		applyFortUpdateFields(data, hook)
	}
	if hook.Type == "invasion" {
		applyInvasionData(p, hook, data, match.Target.Platform, tr)
	} else {
		data["gruntType"] = getString(hook.Message["grunt_type"])
		data["gruntTypeEmoji"] = gruntTypeEmoji(p, data["gruntType"], match.Target.Platform)
		data["gruntTypeColor"] = gruntTypeColor(data["gruntType"])
		data["gruntRewardsList"] = gruntRewardsList(p, data["gruntType"], tr)
	}
	genderValue := getInt(hook.Message["gender"])
	if hook.Type == "invasion" {
		genderValue = getInt(data["gender"])
	}
	data["genderData"] = genderData(p, genderValue, match.Target.Platform, tr)
	if gender, ok := data["genderData"].(map[string]any); ok {
		data["genderName"] = getString(gender["name"])
		data["genderEmoji"] = getString(gender["emoji"])
	}
	if genderEng := genderDataEng(p, getInt(hook.Message["gender"])); genderEng != nil {
		data["genderDataEng"] = genderEng
		if name, ok := genderEng["name"].(string); ok {
			data["genderNameEng"] = name
		}
	}
	data["lureTypeId"] = getInt(hook.Message["lure_id"])
	if data["lureTypeId"].(int) == 0 {
		data["lureTypeId"] = getInt(hook.Message["lure_type"])
	}
	lureID := 0
	if value, ok := data["lureTypeId"].(int); ok {
		lureID = value
	}
	lureName, lureEmojiKey, lureColor := lureTypeDetails(p, lureID)
	if lureName == "" {
		lureName, lureColor = lureTypeInfo(lureID)
	}
	data["lureTypeNameEng"] = lureName
	data["lureTypeName"] = translateMaybe(tr, lureName)
	data["lureType"] = data["lureTypeName"]
	data["lureTypeColor"] = lureColor
	if lureEmojiKey != "" {
		data["lureTypeEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, lureEmojiKey, match.Target.Platform))
	}
	data["gymUrl"] = getString(hook.Message["gym_url"])
	if data["gymUrl"] == "" {
		data["gymUrl"] = getString(hook.Message["url"])
	}
	if hook.Type == "raid" || hook.Type == "egg" || hook.Type == "gym" || hook.Type == "gym_details" {
		campfireGymID := any(nil)
		if hook != nil {
			campfireGymID = hook.Message["gym_id"]
		}
		campfire := campfireLink(lat, lon, campfireGymID, data["gymName"], data["gymUrl"])
		if campfire != "" {
			data["campfireLink"] = campfire
			data["campfireUrl"] = campfire
		}
	}
	applyPokemonPvpRenderData(&ctx)
	applyDynamicWeatherRenderData(&ctx)
	return data
}
