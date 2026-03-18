package webhook

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"dexter/internal/i18n"
)

func applyFortUpdateFields(data map[string]any, hook *Hook) {
	if data == nil || hook == nil {
		return
	}
	changeType := getString(hook.Message["change_type"])
	editTypes := []string{}
	switch v := hook.Message["edit_types"].(type) {
	case []string:
		editTypes = append(editTypes, v...)
	case []any:
		for _, item := range v {
			if entry, ok := item.(string); ok {
				editTypes = append(editTypes, entry)
			}
		}
	case string:
		var decoded []string
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			editTypes = append(editTypes, decoded...)
		}
	}
	oldEntry := mapFromAny(hook.Message["old"])
	newEntry := mapFromAny(hook.Message["new"])
	if changeType == "edit" && (getStringFromAnyMap(oldEntry, "name") == "" && getStringFromAnyMap(oldEntry, "description") == "") {
		changeType = "new"
		editTypes = nil
	}
	data["change_type"] = changeType
	if editTypes != nil {
		data["edit_types"] = editTypes
	} else {
		data["edit_types"] = nil
	}
	changeTypes := append([]string{}, editTypes...)
	if changeType != "" {
		changeTypes = append(changeTypes, changeType)
	}
	data["changeTypes"] = changeTypes
	if oldEntry != nil {
		data["old"] = oldEntry
	}
	if newEntry != nil {
		data["new"] = newEntry
	}
	isEmpty := true
	if newEntry != nil && (getString(newEntry["name"]) != "" || getString(newEntry["description"]) != "") {
		isEmpty = false
	}
	if oldEntry != nil && getString(oldEntry["name"]) != "" {
		isEmpty = false
	}
	data["isEmpty"] = isEmpty

	data["isEdit"] = changeType == "edit"
	data["isNew"] = changeType == "new"
	data["isRemoval"] = changeType == "removal"

	data["isEditLocation"] = containsString(changeTypes, "location")
	data["isEditName"] = containsString(changeTypes, "name")
	data["isEditDescription"] = containsString(changeTypes, "description")
	data["isEditImageUrl"] = containsString(changeTypes, "image_url")
	data["isEditImgUrl"] = data["isEditImageUrl"]

	oldName := getStringFromAnyMap(oldEntry, "name")
	oldDescription := getStringFromAnyMap(oldEntry, "description")
	oldImageURL := getStringFromAnyMap(oldEntry, "image_url")
	if oldImageURL == "" {
		oldImageURL = getStringFromAnyMap(oldEntry, "imageUrl")
	}
	oldLat, oldLon, _ := extractLocation(oldEntry)

	newName := getStringFromAnyMap(newEntry, "name")
	newDescription := getStringFromAnyMap(newEntry, "description")
	newImageURL := getStringFromAnyMap(newEntry, "image_url")
	if newImageURL == "" {
		newImageURL = getStringFromAnyMap(newEntry, "imageUrl")
	}
	newLat, newLon, _ := extractLocation(newEntry)

	data["oldName"] = oldName
	data["oldDescription"] = oldDescription
	data["oldImageUrl"] = oldImageURL
	data["oldImgUrl"] = oldImageURL
	data["oldLatitude"] = oldLat
	data["oldLongitude"] = oldLon

	data["newName"] = newName
	data["newDescription"] = newDescription
	data["newImageUrl"] = newImageURL
	data["newImgUrl"] = newImageURL
	data["newLatitude"] = newLat
	data["newLongitude"] = newLon

	fortType := getString(hook.Message["fort_type"])
	if fortType == "" {
		fortType = getString(hook.Message["fortType"])
	}
	if fortType == "" {
		fortType = getStringFromAnyMap(newEntry, "type")
		if fortType == "" {
			fortType = getStringFromAnyMap(oldEntry, "type")
		}
	}
	fortType = strings.ToLower(strings.TrimSpace(fortType))
	if fortType == "" {
		fortType = "unknown"
	}
	data["fortType"] = fortType
	if fortType == "pokestop" {
		data["fortTypeText"] = "Pokestop"
	} else {
		data["fortTypeText"] = "Gym"
	}
	switch changeType {
	case "edit":
		data["changeTypeText"] = "Edit"
	case "removal":
		data["changeTypeText"] = "Removal"
	case "new":
		data["changeTypeText"] = "New"
	}

	name := newName
	if name == "" {
		name = oldName
	}
	if name == "" {
		name = "unknown"
	}
	description := newDescription
	if description == "" {
		description = oldDescription
	}
	if description == "" {
		description = "unknown"
	}
	imgURL := newImageURL
	if imgURL == "" {
		imgURL = oldImageURL
	}
	data["name"] = name
	data["description"] = description
	data["imgUrl"] = imgURL

	if oldEntry != nil {
		oldEntry["imgUrl"] = oldImageURL
		oldEntry["imageUrl"] = oldImageURL
	}
	if newEntry != nil {
		newEntry["imgUrl"] = newImageURL
		newEntry["imageUrl"] = newImageURL
	}
}

func applyNightTime(p *Processor, hook *Hook, data map[string]any) {
	if p == nil || p.cfg == nil || hook == nil || data == nil {
		return
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 && lon == 0 {
		return
	}
	checkTime, ok := nightTimeReference(hook)
	if !ok {
		return
	}
	loc := hookLocation(p, hook)
	if loc == nil {
		loc = time.Local
	}
	checkTime = checkTime.In(loc)
	sunrise, sunset, ok := sunriseSunset(checkTime, lat, lon, loc)
	if !ok {
		return
	}
	night := !(checkTime.After(sunrise) && checkTime.Before(sunset))
	dawn := checkTime.After(sunrise) && checkTime.Before(sunrise.Add(time.Hour))
	dusk := checkTime.After(sunset.Add(-time.Hour)) && checkTime.Before(sunset)
	data["nightTime"] = night
	data["dawnTime"] = dawn
	data["duskTime"] = dusk

	style := getStringFromConfig(p.cfg, "geocoding.dayStyle", "klokantech-basic")
	if dawn {
		if value := getStringFromConfig(p.cfg, "geocoding.dawnStyle", ""); value != "" {
			style = value
		}
	} else if dusk {
		if value := getStringFromConfig(p.cfg, "geocoding.duskStyle", ""); value != "" {
			style = value
		}
	} else if night {
		if value := getStringFromConfig(p.cfg, "geocoding.nightStyle", ""); value != "" {
			style = value
		}
	}
	data["style"] = style
}

func nightTimeReference(hook *Hook) (time.Time, bool) {
	if hook == nil {
		return time.Time{}, false
	}
	switch hook.Type {
	case "egg":
		start := getInt64(hook.Message["start"])
		if start == 0 {
			start = getInt64(hook.Message["hatch_time"])
		}
		if start > 0 {
			return time.Unix(start, 0), true
		}
	case "gym", "gym_details", "weather":
		return time.Now(), true
	default:
		if expire := hookExpiryUnix(hook); expire > 0 {
			return time.Unix(expire, 0), true
		}
	}
	return time.Time{}, false
}

func sunriseSunset(day time.Time, lat, lon float64, loc *time.Location) (time.Time, time.Time, bool) {
	if loc == nil {
		loc = time.Local
	}
	year, month, dayOfMonth := day.Date()
	n := dayOfYear(year, int(month), dayOfMonth)
	sunrise, ok1 := solarEventUTC(n, lat, lon, true)
	sunset, ok2 := solarEventUTC(n, lat, lon, false)
	if !ok1 || !ok2 {
		return time.Time{}, time.Time{}, false
	}
	base := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
	sunriseTime := base.Add(time.Duration(sunrise * float64(time.Hour))).In(loc)
	sunsetTime := base.Add(time.Duration(sunset * float64(time.Hour))).In(loc)
	return sunriseTime, sunsetTime, true
}

func dayOfYear(year, month, day int) int {
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	current := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(current.Sub(start).Hours()/24) + 1
}

func solarEventUTC(dayOfYear int, lat, lon float64, sunrise bool) (float64, bool) {
	if lat > 89.8 {
		lat = 89.8
	}
	if lat < -89.8 {
		lat = -89.8
	}
	zenith := 90.833
	lngHour := lon / 15.0
	t := float64(dayOfYear) + ((6.0 - lngHour) / 24.0)
	if !sunrise {
		t = float64(dayOfYear) + ((18.0 - lngHour) / 24.0)
	}
	m := (0.9856 * t) - 3.289
	l := m + (1.916 * math.Sin(degToRad(m))) + (0.020 * math.Sin(2*degToRad(m))) + 282.634
	l = math.Mod(l+360, 360)
	ra := radToDeg(math.Atan(0.91764 * math.Tan(degToRad(l))))
	ra = math.Mod(ra+360, 360)
	lQuadrant := math.Floor(l/90.0) * 90.0
	raQuadrant := math.Floor(ra/90.0) * 90.0
	ra = ra + (lQuadrant - raQuadrant)
	ra = ra / 15.0

	sinDec := 0.39782 * math.Sin(degToRad(l))
	cosDec := math.Cos(math.Asin(sinDec))

	cosH := (math.Cos(degToRad(zenith)) - (sinDec * math.Sin(degToRad(lat)))) / (cosDec * math.Cos(degToRad(lat)))
	if cosH > 1 || cosH < -1 {
		return 0, false
	}
	var h float64
	if sunrise {
		h = 360 - radToDeg(math.Acos(cosH))
	} else {
		h = radToDeg(math.Acos(cosH))
	}
	h = h / 15.0
	tVal := h + ra - (0.06571 * t) - 6.622
	ut := tVal - lngHour
	ut = math.Mod(ut+24, 24)
	return ut, true
}

func degToRad(deg float64) float64 {
	return deg * (math.Pi / 180.0)
}

func radToDeg(rad float64) float64 {
	return rad * (180.0 / math.Pi)
}

func mapFromAny(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case map[string]any:
		return v
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return decoded
		}
	}
	return nil
}

func getStringFromAnyMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return getString(m[key])
}

func itemRewardString(p *Processor, itemID, amount int, tr *i18n.Translator) string {
	if itemID == 0 {
		return ""
	}
	name := ""
	if p != nil {
		d := p.getData()
		if d != nil {
			if raw, ok := d.Items[fmt.Sprintf("%d", itemID)]; ok {
				if m, ok := raw.(map[string]any); ok {
					if text, ok := m["name"].(string); ok {
						name = text
					}
				}
			}
		}
	}
	if name == "" {
		name = fmt.Sprintf("Item %d", itemID)
	}
	if amount > 1 {
		return fmt.Sprintf("%d %s", amount, translateMaybe(tr, name))
	}
	return translateMaybe(tr, name)
}

func monsterName(p *Processor, pokemonID int) string {
	if pokemonID == 0 || p == nil {
		return ""
	}
	d := p.getData()
	if d == nil || d.Monsters == nil {
		return ""
	}
	if raw, ok := d.Monsters[fmt.Sprintf("%d_0", pokemonID)]; ok {
		if m, ok := raw.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	if raw, ok := d.Monsters[fmt.Sprintf("%d", pokemonID)]; ok {
		if m, ok := raw.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	for _, raw := range d.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if getInt(m["id"]) == pokemonID {
				if name, ok := m["name"].(string); ok && name != "" {
					return name
				}
			}
		}
	}
	return fmt.Sprintf("Pokemon %d", pokemonID)
}

func monsterInfo(p *Processor, pokemonID, formID int) (string, string) {
	if p == nil {
		return "", ""
	}
	d := p.getData()
	if d == nil {
		return "", ""
	}
	monster := lookupMonster(d, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(d, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(d, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return "", ""
	}
	name := getString(monster["name"])
	formName := ""
	if form, ok := monster["form"].(map[string]any); ok {
		formName = getString(form["name"])
	}
	return name, formName
}

func monsterFormName(p *Processor, pokemonID, formID int) string {
	if p == nil {
		return ""
	}
	d := p.getData()
	if d == nil {
		return ""
	}
	key := fmt.Sprintf("%d_%d", pokemonID, formID)
	monster := lookupMonster(d, key)
	if monster == nil && formID != 0 {
		monster = lookupMonster(d, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(d, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return ""
	}
	if form, ok := monster["form"].(map[string]any); ok {
		if name, ok := form["name"].(string); ok {
			return name
		}
	}
	return ""
}

func monsterGeneration(p *Processor, pokemonID, formID int) (string, string, string) {
	if p == nil || pokemonID <= 0 {
		return "", "", ""
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return "", "", ""
	}
	if exceptions, ok := d.UtilData["genException"].(map[string]any); ok {
		key := fmt.Sprintf("%d_%d", pokemonID, formID)
		if value, ok := exceptions[key]; ok {
			gen := fmt.Sprintf("%v", value)
			if name, roman := genDetails(p, gen); name != "" {
				return gen, name, roman
			}
			return gen, "", ""
		}
	}
	if genData, ok := d.UtilData["genData"].(map[string]any); ok {
		for genKey, raw := range genData {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			min := getInt(entry["min"])
			max := getInt(entry["max"])
			if pokemonID >= min && pokemonID <= max {
				name, _ := entry["name"].(string)
				roman, _ := entry["roman"].(string)
				return genKey, name, roman
			}
		}
	}
	return "", "", ""
}

func genDetails(p *Processor, gen string) (string, string) {
	if p == nil {
		return "", ""
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return "", ""
	}
	genData, ok := d.UtilData["genData"].(map[string]any)
	if !ok {
		return "", ""
	}
	entry, ok := genData[gen].(map[string]any)
	if !ok {
		return "", ""
	}
	name, _ := entry["name"].(string)
	roman, _ := entry["roman"].(string)
	return name, roman
}

func gruntTypeEmoji(p *Processor, gruntType any, platform string) string {
	// PoracleJS defaults gruntTypeEmoji to "grunt-unknown" and then overrides it when more specific
	// information is available (type emoji, event invasion emoji, etc.).
	_ = gruntType
	return lookupEmojiForPlatform(p, "grunt-unknown", platform)
}
