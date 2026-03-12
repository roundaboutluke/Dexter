package webhook

import "time"

func (w *WeatherTracker) TrackCare(cellID string, target alertTarget, caresUntil int64, clean bool, ping string, pokemon *caredPokemon) {
	if w == nil || cellID == "" || target.ID == "" {
		return
	}
	if caresUntil == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.ensureCareCell(cellID)
	entry := cell.Cares[target.ID]
	if entry == nil {
		entry = &weatherCareEntry{ID: target.ID}
		cell.Cares[target.ID] = entry
	}
	entry.Name = target.Name
	entry.Type = target.Type
	entry.Template = target.Template
	entry.Language = target.Language
	entry.Clean = clean
	entry.Ping = ping
	entry.CaresUntil = maxInt64(entry.CaresUntil, caresUntil)
	if !getBoolFromConfig(w.cfg, "weather.showAlteredPokemon", false) {
		entry.CaredPokemons = nil
		return
	}
	if pokemon != nil && !hasCaredPokemon(entry.CaredPokemons, *pokemon) {
		entry.CaredPokemons = append(entry.CaredPokemons, *pokemon)
	}
}

func (w *WeatherTracker) EligibleTargets(cellID string, weatherID int, showAltered bool) []string {
	if w == nil || cellID == "" {
		return nil
	}
	now := time.Now().Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return nil
	}
	targets := []string{}
	for id, entry := range cell.Cares {
		if entry.CaresUntil < now {
			delete(cell.Cares, id)
			continue
		}
		entry.CaredPokemons = filterActivePokemons(entry.CaredPokemons, now)
		if showAltered && !hasAlteredPokemon(entry.CaredPokemons, weatherID) {
			continue
		}
		targets = append(targets, id)
	}
	return targets
}

func (w *WeatherTracker) CareEntry(cellID, targetID string) *weatherCareEntry {
	if w == nil || cellID == "" || targetID == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return nil
	}
	entry := cell.Cares[targetID]
	if entry == nil {
		return nil
	}
	copy := *entry
	copy.CaredPokemons = append([]caredPokemon(nil), entry.CaredPokemons...)
	return &copy
}

func (w *WeatherTracker) ShouldSendWeather(cellID, targetID string, hour int64) bool {
	if w == nil || cellID == "" || targetID == "" {
		return false
	}
	now := time.Now().Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return false
	}
	entry := cell.Cares[targetID]
	if entry == nil {
		return false
	}
	if entry.CaresUntil < now {
		delete(cell.Cares, targetID)
		return false
	}
	if entry.LastChangeAlert == hour {
		return false
	}
	entry.LastChangeAlert = hour
	return true
}

func (w *WeatherTracker) ActivePokemons(cellID, targetID string, weatherID int, maxCount int) []caredPokemon {
	if w == nil || cellID == "" || targetID == "" {
		return nil
	}
	now := time.Now().Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return nil
	}
	entry := cell.Cares[targetID]
	if entry == nil {
		return nil
	}
	entry.CaredPokemons = filterActivePokemons(entry.CaredPokemons, now)
	out := make([]caredPokemon, 0, len(entry.CaredPokemons))
	for _, mon := range entry.CaredPokemons {
		if weatherID > 0 && !containsInt(mon.AlteringWeathers, weatherID) {
			continue
		}
		out = append(out, mon)
		if maxCount > 0 && len(out) >= maxCount {
			break
		}
	}
	return out
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func hasCaredPokemon(list []caredPokemon, candidate caredPokemon) bool {
	for _, item := range list {
		if item.PokemonID == candidate.PokemonID && item.Form == candidate.Form && item.DisappearTime == candidate.DisappearTime {
			return true
		}
	}
	return false
}

func filterActivePokemons(list []caredPokemon, now int64) []caredPokemon {
	if len(list) == 0 {
		return list
	}
	out := make([]caredPokemon, 0, len(list))
	for _, item := range list {
		if item.DisappearTime <= now {
			continue
		}
		out = append(out, item)
	}
	return out
}

func hasAlteredPokemon(list []caredPokemon, weatherID int) bool {
	if weatherID == 0 {
		return len(list) > 0
	}
	for _, item := range list {
		if containsInt(item.AlteringWeathers, weatherID) {
			return true
		}
	}
	return false
}
