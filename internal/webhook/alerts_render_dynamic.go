package webhook

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/geo/s2"

	"poraclego/internal/geo"
)

func applyDynamicWeatherRenderData(ctx *renderDataContext) {
	if ctx.hook.Type == "invasion" || ctx.hook.Type == "pokestop" {
		weatherCellID := geo.WeatherCellID(ctx.lat, ctx.lon)
		if weatherCellID != "" {
			ctx.data["weatherCellId"] = weatherCellID
			if ctx.p != nil && ctx.p.weatherData != nil {
				if cell := ctx.p.weatherData.WeatherInfo(weatherCellID); cell != nil {
					now := time.Now().Unix()
					currentHour := now - (now % 3600)
					if current := cell.Data[currentHour]; current > 0 {
						nameEng, emojiKey := weatherEntry(ctx.p, current)
						ctx.data["gameWeatherId"] = current
						ctx.data["gameWeatherNameEng"] = nameEng
						ctx.data["gameWeatherName"] = translateMaybe(ctx.tr, nameEng)
						ctx.data["gameWeatherEmoji"] = translateMaybe(ctx.tr, lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform))
						ctx.data["gameweather"] = ctx.data["gameWeatherName"]
						ctx.data["gameweatheremoji"] = ctx.data["gameWeatherEmoji"]
					}
				}
			}
		}
	}
	ctx.data["now"] = time.Now()
	ctx.data["nowISO"] = time.Now().UTC().Format(time.RFC3339)
	ctx.data["weatherChange"] = ""
	ctx.data["futureEvent"] = false
	if ctx.hook.Type == "pokemon" {
		applyPokemonWeatherRenderData(ctx)
	}
	if ctx.hook.Type == "raid" || ctx.hook.Type == "egg" {
		applyRaidEggWeatherRenderData(ctx)
	}
	if ctx.p != nil && ctx.p.eventParser != nil {
		start := time.Now().Unix()
		expire := hookExpiryUnix(ctx.hook)
		if expire > 0 {
			var event *EventChange
			switch ctx.hook.Type {
			case "pokemon":
				event = ctx.p.eventParser.EventChangesSpawn(start, expire, ctx.lat, ctx.lon, ctx.p.tzLocator)
			case "quest":
				event = ctx.p.eventParser.EventChangesQuest(start, expire, ctx.lat, ctx.lon, ctx.p.tzLocator)
			}
			if event != nil {
				ctx.data["futureEvent"] = true
				ctx.data["futureEventTime"] = event.Time
				ctx.data["futureEventName"] = event.Name
				ctx.data["futureEventTrigger"] = event.Reason
			}
		}
	}
	if ctx.p != nil && ctx.p.cfg != nil {
		if rdmURL := buildRdmURL(ctx.p.cfg, ctx.hook, ctx.lat, ctx.lon); rdmURL != "" {
			ctx.data["rdmUrl"] = rdmURL
		}
		if rocketMad := rocketMadURL(ctx.p.cfg, ctx.lat, ctx.lon); rocketMad != "" {
			ctx.data["rocketMadUrl"] = rocketMad
		}
	}
	if timeStr, ok := ctx.data["time"].(string); ok && timeStr != "" {
		ctx.data["disappearTime"] = timeStr
		ctx.data["distime"] = timeStr
		ctx.data["disTime"] = timeStr
	}
	ctx.data["ivcolor"] = ctx.data["ivColor"]
	if ctx.hook.Type == "pokemon" {
		applyPokemonSeenTypeRenderData(ctx)
		applyPokemonWeatherChangeRenderData(ctx)
		applyMonsterChangeRenderData(ctx)
	}
	if ctx.hook.Type == "raid" {
		applyRaidWeatherChangeRenderData(ctx)
	}
	if ctx.p != nil && ctx.p.cfg != nil {
		staticMap := staticMapURL(ctx.p, ctx.hook, ctx.data)
		if staticMap == "" {
			staticMap = getStringFromConfig(ctx.p.cfg, "fallbacks.staticMap", "")
		}
		if staticMap != "" {
			ctx.data["staticMap"] = staticMap
			ctx.data["staticmap"] = staticMap
		}
	}
}

func applyPokemonWeatherRenderData(ctx *renderDataContext) {
	weatherCellID := getString(ctx.hook.Message["s2_cell_id"])
	if weatherCellID == "" {
		weatherCellID = geo.WeatherCellID(ctx.lat, ctx.lon)
	}
	if weatherCellID != "" {
		ctx.data["weatherCellId"] = weatherCellID
	}
	currentCellWeather := 0
	if ctx.p != nil && ctx.p.weatherData != nil && weatherCellID != "" {
		if cell := ctx.p.weatherData.WeatherInfo(weatherCellID); cell != nil {
			now := time.Now().Unix()
			currentHour := now - (now % 3600)
			currentCellWeather = cell.Data[currentHour]
		}
	}
	if currentCellWeather > 0 {
		ctx.data["gameWeatherId"] = currentCellWeather
		name, emoji := weatherInfo(ctx.p, currentCellWeather, ctx.match.Target.Platform, ctx.tr)
		ctx.data["gameWeatherName"] = name
		ctx.data["gameWeatherEmoji"] = emoji
		ctx.data["gameweather"] = name
		ctx.data["gameweatheremoji"] = emoji
	} else {
		ctx.data["gameWeatherId"] = ""
		ctx.data["gameWeatherName"] = ""
		ctx.data["gameWeatherEmoji"] = ""
		ctx.data["gameweather"] = ""
		ctx.data["gameweatheremoji"] = ""
	}

	if ctx.p == nil || ctx.p.cfg == nil || ctx.p.weatherData == nil || weatherCellID == "" {
		return
	}
	enabled, _ := ctx.p.cfg.GetBool("weather.enableWeatherForecast")
	if !enabled {
		return
	}
	expire := hookExpiryUnix(ctx.hook)
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	nextHour := currentHour + 3600
	if expire <= nextHour {
		return
	}
	cell := ctx.p.weatherData.EnsureForecast(weatherCellID, ctx.lat, ctx.lon)
	if cell == nil {
		return
	}
	weatherCurrent := cell.Data[currentHour]
	weatherNext := cell.Data[nextHour]
	types, _ := ctx.data["types"].([]int)
	pokemonShouldBeBoosted := weatherBoostsTypes(ctx.p, weatherCurrent, types)
	pokemonWillBeBoosted := weatherBoostsTypes(ctx.p, weatherNext, types)
	if weatherNext <= 0 || ((ctx.weatherID <= 0 || weatherNext == ctx.weatherID) && (weatherCurrent <= 0 || weatherNext == weatherCurrent) && !(pokemonShouldBeBoosted && ctx.weatherID == 0)) {
		return
	}
	changeTime := expire - (expire % 3600)
	layout := "15:04:05"
	if format := getStringFromConfig(ctx.p.cfg, "locale.time", ""); format != "" {
		layout = momentFormatToGoLayout(format)
	}
	weatherChangeTime := trimWeatherChangeTime(formatUnixInHookLocation(ctx.p, ctx.hook, changeTime, layout))
	if (ctx.weatherID > 0 && !pokemonWillBeBoosted) || (ctx.weatherID == 0 && pokemonWillBeBoosted) {
		if ctx.weatherID > 0 {
			weatherCurrent = ctx.weatherID
		}
		if pokemonShouldBeBoosted && ctx.weatherID == 0 {
			ctx.data["weatherCurrent"] = 0
		} else {
			ctx.data["weatherCurrent"] = weatherCurrent
		}
		ctx.data["weatherChangeTime"] = weatherChangeTime
		ctx.data["weatherNext"] = weatherNext
	}
}

func applyRaidEggWeatherRenderData(ctx *renderDataContext) {
	weatherCellID := geo.WeatherCellID(ctx.lat, ctx.lon)
	if weatherCellID != "" {
		ctx.data["weatherCellId"] = weatherCellID
	}
	if ctx.weatherID > 0 {
		nameEng, emojiKey := weatherEntry(ctx.p, ctx.weatherID)
		ctx.data["gameWeatherId"] = ctx.weatherID
		ctx.data["gameWeatherNameEng"] = nameEng
		ctx.data["gameWeatherName"] = translateMaybe(ctx.tr, nameEng)
		ctx.data["gameWeatherEmoji"] = translateMaybe(ctx.tr, lookupEmojiForPlatform(ctx.p, emojiKey, ctx.match.Target.Platform))
		ctx.data["gameweather"] = ctx.data["gameWeatherName"]
		ctx.data["gameweatheremoji"] = ctx.data["gameWeatherEmoji"]
	} else {
		ctx.data["gameWeatherId"] = ""
		ctx.data["gameWeatherNameEng"] = ""
		ctx.data["gameWeatherName"] = ""
		ctx.data["gameWeatherEmoji"] = ""
		ctx.data["gameweather"] = ""
		ctx.data["gameweatheremoji"] = ""
	}
	if ctx.p == nil || ctx.p.cfg == nil || ctx.p.weatherData == nil || weatherCellID == "" {
		return
	}
	enabled, _ := ctx.p.cfg.GetBool("weather.enableWeatherForecast")
	if !enabled {
		return
	}
	expire := hookExpiryUnix(ctx.hook)
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	nextHour := currentHour + 3600
	if expire <= nextHour {
		return
	}
	cell := ctx.p.weatherData.EnsureForecast(weatherCellID, ctx.lat, ctx.lon)
	if cell == nil {
		return
	}
	weatherCurrent := cell.Data[currentHour]
	weatherNext := cell.Data[nextHour]
	types, _ := ctx.data["types"].([]int)
	pokemonShouldBeBoosted := weatherBoostsTypes(ctx.p, weatherCurrent, types)
	pokemonWillBeBoosted := weatherBoostsTypes(ctx.p, weatherNext, types)
	if weatherNext <= 0 || ((ctx.weatherID <= 0 || weatherNext == ctx.weatherID) && (weatherCurrent <= 0 || weatherNext == weatherCurrent) && !(pokemonShouldBeBoosted && ctx.weatherID == 0)) {
		return
	}
	changeTime := expire - (expire % 3600)
	layout := "15:04:05"
	if format := getStringFromConfig(ctx.p.cfg, "locale.time", ""); format != "" {
		layout = momentFormatToGoLayout(format)
	}
	weatherChangeTime := trimWeatherChangeTime(formatUnixInHookLocation(ctx.p, ctx.hook, changeTime, layout))
	if (ctx.weatherID > 0 && !pokemonWillBeBoosted) || (ctx.weatherID == 0 && pokemonWillBeBoosted) {
		if ctx.weatherID > 0 {
			weatherCurrent = ctx.weatherID
		}
		if pokemonShouldBeBoosted && ctx.weatherID == 0 {
			ctx.data["weatherCurrent"] = 0
		} else {
			ctx.data["weatherCurrent"] = weatherCurrent
		}
		ctx.data["weatherChangeTime"] = weatherChangeTime
		ctx.data["weatherNext"] = weatherNext
	}
}

func applyPokemonSeenTypeRenderData(ctx *renderDataContext) {
	seenType := getString(ctx.hook.Message["seen_type"])
	if seenType != "" {
		switch seenType {
		case "nearby_stop":
			ctx.data["seenType"] = "pokestop"
		case "nearby_cell":
			ctx.data["seenType"] = "cell"
		case "lure", "lure_wild":
			ctx.data["seenType"] = "lure"
		case "lure_encounter", "encounter", "wild":
			ctx.data["seenType"] = seenType
		}
	} else {
		stopID := getString(ctx.hook.Message["pokestop_id"])
		spawnID := getString(ctx.hook.Message["spawnpoint_id"])
		if stopID == "None" && spawnID == "None" {
			ctx.data["seenType"] = "cell"
		} else if stopID == "None" {
			if getString(ctx.data["iv"]) != "-1" {
				ctx.data["seenType"] = "encounter"
			} else {
				ctx.data["seenType"] = "wild"
			}
		}
	}
	if ctx.data["seenType"] == "cell" {
		if _, ok := ctx.data["cell_coords"]; !ok {
			cellID := s2.CellIDFromLatLng(s2.LatLngFromDegrees(ctx.lat, ctx.lon)).Parent(15)
			cell := s2.CellFromCellID(cellID)
			coords := make([][]float64, 0, 4)
			for i := 0; i < 4; i++ {
				ll := s2.LatLngFromPoint(cell.Vertex(i))
				coords = append(coords, []float64{ll.Lat.Degrees(), ll.Lng.Degrees()})
			}
			ctx.data["cell_coords"] = coords
		}
	}
	ctx.data["pvpPokemonId"] = ctx.data["pokemonId"]
	ctx.data["pvpFormId"] = ctx.data["formId"]
}

func applyPokemonWeatherChangeRenderData(ctx *renderDataContext) {
	weatherNext := getInt(ctx.data["weatherNext"])
	if weatherNext <= 0 {
		return
	}
	weatherCurrent := getInt(ctx.data["weatherCurrent"])
	weatherNextName, weatherNextEmoji := weatherInfo(ctx.p, weatherNext, ctx.match.Target.Platform, ctx.tr)
	ctx.data["weatherNextName"] = weatherNextName
	ctx.data["weatherNextEmoji"] = weatherNextEmoji
	changeTime := getString(ctx.data["weatherChangeTime"])
	if weatherCurrent <= 0 {
		ctx.data["weatherCurrentName"] = translateMaybe(ctx.tr, "unknown")
		ctx.data["weatherCurrentEmoji"] = "❓"
		ctx.data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : ➡️ %s %s", translateMaybe(ctx.tr, "Possible weather change at"), changeTime, weatherNextName, weatherNextEmoji)
		return
	}
	weatherCurrentName, weatherCurrentEmoji := weatherInfo(ctx.p, weatherCurrent, ctx.match.Target.Platform, ctx.tr)
	ctx.data["weatherCurrentName"] = weatherCurrentName
	ctx.data["weatherCurrentEmoji"] = weatherCurrentEmoji
	ctx.data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : %s %s ➡️ %s %s", translateMaybe(ctx.tr, "Possible weather change at"), changeTime, weatherCurrentName, weatherCurrentEmoji, weatherNextName, weatherNextEmoji)
}

func applyRaidWeatherChangeRenderData(ctx *renderDataContext) {
	weatherNext := getInt(ctx.data["weatherNext"])
	if weatherNext <= 0 {
		return
	}
	weatherCurrent := getInt(ctx.data["weatherCurrent"])
	weatherNextName, weatherNextEmoji := weatherInfo(ctx.p, weatherNext, ctx.match.Target.Platform, ctx.tr)
	ctx.data["weatherNextName"] = weatherNextName
	ctx.data["weatherNextEmoji"] = weatherNextEmoji
	changeTime := getString(ctx.data["weatherChangeTime"])
	if weatherCurrent <= 0 {
		ctx.data["weatherCurrentName"] = translateMaybe(ctx.tr, "unknown")
		ctx.data["weatherCurrentEmoji"] = "❓"
		ctx.data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : ➡️ %s %s", translateMaybe(ctx.tr, "Possible weather change at"), changeTime, weatherNextName, weatherNextEmoji)
		return
	}
	weatherCurrentName, weatherCurrentEmoji := weatherInfo(ctx.p, weatherCurrent, ctx.match.Target.Platform, ctx.tr)
	ctx.data["weatherCurrentName"] = weatherCurrentName
	ctx.data["weatherCurrentEmoji"] = weatherCurrentEmoji
	ctx.data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : %s %s ➡️ %s %s", translateMaybe(ctx.tr, "Possible weather change at"), changeTime, weatherCurrentName, weatherCurrentEmoji, weatherNextName, weatherNextEmoji)
}

func applyMonsterChangeRenderData(ctx *renderDataContext) {
	if !getBool(ctx.hook.Message["_monsterChange"]) && !getBool(ctx.hook.Message["monster_change"]) && !getBool(ctx.hook.Message["monsterChange"]) && !getBool(ctx.hook.Message["pokemon_change"]) && !getBool(ctx.hook.Message["pokemonChange"]) {
		return
	}
	oldPokemonID := getInt(ctx.hook.Message["oldPokemonId"])
	if oldPokemonID == 0 {
		oldPokemonID = getInt(ctx.hook.Message["old_pokemon_id"])
	}
	oldFormID := getInt(ctx.hook.Message["oldFormId"])
	if oldFormID == 0 {
		oldFormID = getInt(ctx.hook.Message["old_form_id"])
	}
	if oldPokemonID > 0 {
		oldNameEng := monsterName(ctx.p, oldPokemonID)
		oldFormNameEng := monsterFormName(ctx.p, oldPokemonID, oldFormID)
		oldFullNameEng := oldNameEng
		if oldFormNameEng != "" && !strings.EqualFold(oldFormNameEng, "Normal") {
			oldFullNameEng = fmt.Sprintf("%s %s", oldNameEng, oldFormNameEng)
		}
		ctx.data["oldFullNameEng"] = oldFullNameEng
		oldFullName := translateMaybe(ctx.tr, oldNameEng)
		if oldFormNameEng != "" && !strings.EqualFold(oldFormNameEng, "Normal") {
			oldFullName = fmt.Sprintf("%s %s", oldFullName, translateMaybe(ctx.tr, oldFormNameEng))
		}
		ctx.data["oldFullName"] = oldFullName
	}
	if _, ok := ctx.data["oldCp"]; !ok {
		ctx.data["oldCp"] = getInt(ctx.hook.Message["oldCp"])
	}
	if _, ok := ctx.data["oldIv"]; !ok {
		ctx.data["oldIv"] = getFloat(ctx.hook.Message["oldIv"])
	}
	if _, ok := ctx.data["oldIvKnown"]; !ok {
		ctx.data["oldIvKnown"] = getFloat(ctx.data["oldIv"]) >= 0
	}
}
