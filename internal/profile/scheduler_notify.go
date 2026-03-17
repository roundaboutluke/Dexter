package profile

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"poraclego/internal/config"
	"poraclego/internal/digest"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/i18n"
	"poraclego/internal/render"
)

func (s *Scheduler) notifySwitch(human map[string]any, profile map[string]any) {
	if human == nil || profile == nil {
		return
	}
	targetType := getString(human["type"])
	targetID := getString(human["id"])
	if targetType == "" || targetID == "" {
		return
	}
	name := getString(human["name"])
	profileName := getString(profile["name"])
	lang := getString(human["language"])
	if s.cfg != nil && lang == "" {
		if fallback, ok := s.cfg.GetString("general.locale"); ok {
			lang = fallback
		}
	}
	tr := s.i18n.Translator(lang)
	message := tr.TranslateFormat("I have set your profile to: {0}", profileName)
	job := dispatch.MessageJob{
		Type:    targetType,
		Target:  targetID,
		Name:    name,
		Message: message,
		TTH:     dispatch.TimeToHide{Hours: 1},
	}
	if strings.HasPrefix(targetType, "telegram") {
		if s.telegramQueue != nil {
			s.telegramQueue.Push(job)
		}
	} else if s.discordQueue != nil {
		s.discordQueue.Push(job)
	}
	s.sendQuestDigest(targetType, targetID, name, lang, tr, profileName, profile)
}

func (s *Scheduler) notifyQuiet(human map[string]any, profiles []map[string]any) {
	if human == nil {
		return
	}
	targetType := getString(human["type"])
	targetID := getString(human["id"])
	if targetType == "" || targetID == "" {
		return
	}
	name := getString(human["name"])
	lang := getString(human["language"])
	if s.cfg != nil && lang == "" {
		if fallback, ok := s.cfg.GetString("general.locale"); ok {
			lang = fallback
		}
	}
	tr := s.i18n.Translator(lang)
	next, ok := s.nextScheduleTime(human, profiles)
	message := ""
	if ok {
		message = tr.TranslateFormat("Quiet hours enabled. Alerts paused until {0}. Adjust schedules with /profile.", formatScheduleTime(tr, next))
	} else {
		message = tr.Translate("Quiet hours enabled. Alerts paused. Adjust schedules with /profile.", false)
	}
	job := dispatch.MessageJob{
		Type:    targetType,
		Target:  targetID,
		Name:    name,
		Message: message,
		TTH:     dispatch.TimeToHide{Hours: 1},
	}
	if strings.HasPrefix(targetType, "telegram") {
		if s.telegramQueue != nil {
			s.telegramQueue.Push(job)
		}
	} else if s.discordQueue != nil {
		s.discordQueue.Push(job)
	}
	if s.questDigests != nil {
		s.questDigests.BeginQuiet(targetID)
	}
}

func (s *Scheduler) notifyResume(human map[string]any, profile map[string]any) {
	if human == nil || profile == nil {
		return
	}
	targetType := getString(human["type"])
	targetID := getString(human["id"])
	if targetType == "" || targetID == "" {
		return
	}
	name := getString(human["name"])
	profileName := getString(profile["name"])
	lang := getString(human["language"])
	if s.cfg != nil && lang == "" {
		if fallback, ok := s.cfg.GetString("general.locale"); ok {
			lang = fallback
		}
	}
	tr := s.i18n.Translator(lang)
	if profileName == "" {
		profileName = tr.TranslateFormat("Profile {0}", toInt(profile["profile_no"]))
	}
	message := tr.TranslateFormat("Active hours started. Alerts resumed on {0}. Adjust schedules with /profile.", profileName)
	job := dispatch.MessageJob{
		Type:    targetType,
		Target:  targetID,
		Name:    name,
		Message: message,
		TTH:     dispatch.TimeToHide{Hours: 1},
	}
	if strings.HasPrefix(targetType, "telegram") {
		if s.telegramQueue != nil {
			s.telegramQueue.Push(job)
		}
	} else if s.discordQueue != nil {
		s.discordQueue.Push(job)
	}
	if s.questDigests != nil {
		s.questDigests.EndQuiet(targetID)
	}
	s.sendQuestDigest(targetType, targetID, name, lang, tr, profileName, profile)
}

func (s *Scheduler) sendQuestDigest(targetType, targetID, name, language string, tr *i18n.Translator, profileName string, profile map[string]any) {
	if s.questDigests == nil || profile == nil {
		return
	}
	profileNo := toInt(profile["profile_no"])
	if digest, ok := s.questDigests.Consume(targetID, profileNo); ok {
		s.logf("quest digest consume: user=%s profile=%d total=%d", targetID, profileNo, digest.Total)
		templatePayload, templateMsg := s.renderQuestDigestTemplate(targetType, language, profileName, digest)
		if templateMsg != "" || templatePayload != nil {
			if !questDigestExceedsLimits(targetType, templatePayload, templateMsg) {
				s.pushQuestDigestJob(dispatch.MessageJob{
					Type:    targetType,
					Target:  targetID,
					Name:    name,
					Message: templateMsg,
					Payload: templatePayload,
					TTH:     dispatch.TimeToHide{Hours: 2},
				})
				return
			}
		}

		// Chunked output keeps clickable stop links and avoids hard message limits.
		for _, job := range s.chunkQuestDigestJobs(targetType, targetID, name, tr, profileName, digest) {
			s.pushQuestDigestJob(job)
		}
	}
}

func (s *Scheduler) pushQuestDigestJob(job dispatch.MessageJob) {
	if strings.HasPrefix(job.Type, "telegram") {
		if s.telegramQueue != nil {
			s.telegramQueue.Push(job)
		}
		return
	}
	if s.discordQueue != nil {
		s.discordQueue.Push(job)
	}
}

func questDigestExceedsLimits(targetType string, payload map[string]any, message string) bool {
	const (
		telegramMax         = 4096
		discordContentMax   = 2000
		discordEmbedDescMax = 4096
	)

	if strings.HasPrefix(targetType, "telegram") {
		return len(message) > telegramMax
	}

	if payload == nil {
		return len(message) > discordContentMax
	}
	if embeds, ok := payload["embeds"].([]any); ok && len(embeds) > 0 {
		for _, raw := range embeds {
			embed, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if desc, ok := embed["description"].(string); ok && len(desc) > discordEmbedDescMax {
				return true
			}
		}
	}
	if embed, ok := payload["embed"].(map[string]any); ok {
		if desc, ok := embed["description"].(string); ok && len(desc) > discordEmbedDescMax {
			return true
		}
	}
	if content, ok := payload["content"].(string); ok && len(content) > discordContentMax {
		return true
	}
	return false
}

func (s *Scheduler) chunkQuestDigestJobs(targetType, targetID, name string, tr *i18n.Translator, profileName string, summary *digest.QuestDigestSummary) []dispatch.MessageJob {
	if summary == nil || summary.Total == 0 {
		return nil
	}
	isTelegram := strings.HasPrefix(targetType, "telegram")
	limit := 4096

	headerBase := tr.TranslateFormat("Quest digest for {0}: {1} quests missed during quiet hours.", profileName, summary.Total)
	reserve := len(headerBase) + len(" (999/999)") + 1
	bodyLimit := limit - reserve
	if bodyLimit < 800 {
		bodyLimit = 800
	}

	lines := []string{}
	if stops := questDigestStops(s.cfg, summary); len(stops) > 0 {
		lines = questDigestStopLines(stops)
	} else {
		lines = digest.RewardsWithStops(summary.Rewards, summary.Stops, summary.StopNames)
	}

	bodyPages := chunkLines(lines, bodyLimit)
	if len(bodyPages) == 0 {
		bodyPages = []string{""}
	}
	pageCount := len(bodyPages)

	nowISO := time.Now().Format(time.RFC3339)
	jobs := make([]dispatch.MessageJob, 0, pageCount)
	for i, body := range bodyPages {
		header := headerBase
		if pageCount > 1 {
			header = fmt.Sprintf("%s (%d/%d)", headerBase, i+1, pageCount)
		}
		text := header
		if strings.TrimSpace(body) != "" {
			text = header + "\n" + body
		}

		job := dispatch.MessageJob{
			Type:   targetType,
			Target: targetID,
			Name:   name,
			TTH:    dispatch.TimeToHide{Hours: 2},
		}
		if isTelegram {
			job.Message = text
			job.Payload = map[string]any{
				"parse_mode":      "Markdown",
				"webpage_preview": true,
			}
		} else {
			job.Message = ""
			job.Payload = map[string]any{
				"embeds": []any{map[string]any{
					"title":       "Quest Digest",
					"description": text,
					"timestamp":   nowISO,
				}},
			}
		}
		jobs = append(jobs, job)
	}
	return jobs
}

func chunkLines(lines []string, maxLen int) []string {
	if maxLen <= 0 {
		return nil
	}
	pages := []string{}
	var buf strings.Builder
	push := func() {
		if s := buf.String(); strings.TrimSpace(s) != "" {
			pages = append(pages, s)
		}
		buf.Reset()
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		for len(line) > maxLen {
			part := line[:maxLen]
			line = line[maxLen:]
			if buf.Len() > 0 {
				push()
			}
			pages = append(pages, part)
		}
		if buf.Len() == 0 {
			buf.WriteString(line)
			continue
		}
		if buf.Len()+1+len(line) <= maxLen {
			buf.WriteByte('\n')
			buf.WriteString(line)
			continue
		}
		push()
		buf.WriteString(line)
	}
	push()
	return pages
}

func (s *Scheduler) questDigestMessage(tr *i18n.Translator, profileName string, summary *digest.QuestDigestSummary) string {
	if summary == nil || summary.Total == 0 {
		return ""
	}
	lines := []string{
		tr.TranslateFormat("Quest digest for {0}: {1} quests missed during quiet hours.", profileName, summary.Total),
	}
	stops := questDigestStops(s.cfg, summary)
	if len(stops) > 0 {
		lines = append(lines, questDigestStopLines(stops)...)
	} else {
		rewards := digest.RewardsWithStops(summary.Rewards, summary.Stops, summary.StopNames)
		lines = append(lines, rewards...)
	}
	return strings.Join(lines, "\n")
}

func (s *Scheduler) renderQuestDigestTemplate(targetType, language, profileName string, summary *digest.QuestDigestSummary) (map[string]any, string) {
	if summary == nil || summary.Total == 0 {
		return nil, ""
	}
	s.templatesMu.RLock()
	templates := append([]dts.Template(nil), s.templates...)
	s.templatesMu.RUnlock()
	if len(templates) == 0 {
		return nil, ""
	}
	platform := platformFromType(targetType)
	template := selectTemplatePayload(templates, platform, "questdigest", language)
	if template == nil {
		return nil, ""
	}
	rewards := digest.RewardsWithStops(summary.Rewards, summary.Stops, summary.StopNames)
	stops := questDigestStops(s.cfg, summary)
	data := map[string]any{
		"profileName":  profileName,
		"total":        summary.Total,
		"rewards":      rewards,
		"rewardsCount": len(rewards),
		"stops":        stops,
		"stopsCount":   len(stops),
		"nowISO":       time.Now().Format(time.RFC3339),
	}
	meta := map[string]any{
		"language": language,
		"platform": platform,
	}
	rendered := renderTemplateAny(template, data, meta)
	if payload, ok := rendered.(map[string]any); ok {
		message := ""
		if content, ok := payload["content"].(string); ok {
			message = content
		} else if embed, ok := payload["embed"].(map[string]any); ok {
			if desc, ok := embed["description"].(string); ok {
				message = desc
			}
		} else if embeds, ok := payload["embeds"].([]any); ok && len(embeds) > 0 {
			if first, ok := embeds[0].(map[string]any); ok {
				if desc, ok := first["description"].(string); ok {
					message = desc
				}
			}
		}
		return payload, message
	}
	if text, ok := rendered.(string); ok {
		return nil, text
	}
	return nil, ""
}

func questDigestStops(cfg *config.Config, summary *digest.QuestDigestSummary) []map[string]any {
	if summary == nil || len(summary.Stops) == 0 {
		return nil
	}
	type stopEntry struct {
		Key    string
		Name   string
		NoAR   map[string]bool
		WithAR map[string]bool
	}
	byStop := map[string]*stopEntry{}
	for rewardKey, stops := range summary.Stops {
		if rewardKey == "" || len(stops) == 0 {
			continue
		}
		mode := "any"
		rewardText := rewardKey
		if strings.HasPrefix(rewardKey, "With AR: ") {
			mode = "with"
			rewardText = strings.TrimPrefix(rewardKey, "With AR: ")
		} else if strings.HasPrefix(rewardKey, "No AR: ") {
			mode = "no"
			rewardText = strings.TrimPrefix(rewardKey, "No AR: ")
		}
		rewardText = strings.TrimSpace(rewardText)
		for stopKey := range stops {
			stopKey = strings.TrimSpace(stopKey)
			if stopKey == "" {
				continue
			}
			entry := byStop[stopKey]
			if entry == nil {
				entry = &stopEntry{
					Key:    stopKey,
					Name:   questDigestStopName(summary, stopKey),
					NoAR:   map[string]bool{},
					WithAR: map[string]bool{},
				}
				byStop[stopKey] = entry
			} else if entry.Name == "" {
				entry.Name = questDigestStopName(summary, stopKey)
			}
			switch mode {
			case "with":
				entry.WithAR[rewardText] = true
			case "no":
				entry.NoAR[rewardText] = true
			default:
				entry.NoAR[rewardText] = true
			}
		}
	}
	stopKeys := make([]string, 0, len(byStop))
	for key := range byStop {
		stopKeys = append(stopKeys, key)
	}
	sort.Slice(stopKeys, func(i, j int) bool {
		left := byStop[stopKeys[i]]
		right := byStop[stopKeys[j]]
		leftName := ""
		rightName := ""
		if left != nil {
			leftName = left.Name
		}
		if right != nil {
			rightName = right.Name
		}
		if leftName == rightName {
			return stopKeys[i] < stopKeys[j]
		}
		return leftName < rightName
	})
	out := make([]map[string]any, 0, len(stopKeys))
	for _, stopKey := range stopKeys {
		entry := byStop[stopKey]
		if entry == nil {
			continue
		}
		noAR := mapKeysSorted(entry.NoAR)
		withAR := mapKeysSorted(entry.WithAR)
		reactMap := questDigestReactMapURL(cfg, entry.Key)
		diadem := questDigestDiademURL(cfg, entry.Key)
		mapURL := diadem
		if mapURL == "" {
			mapURL = reactMap
		}
		stopName := strings.TrimSpace(entry.Name)
		if stopName == "" {
			stopName = entry.Key
		}
		out = append(out, map[string]any{
			"stop":        stopName,
			"stopKey":     entry.Key,
			"noAR":        noAR,
			"noARCount":   len(noAR),
			"withAR":      withAR,
			"withARCount": len(withAR),
			"reactMapUrl": reactMap,
			"diademUrl":   diadem,
			"stopUrl":     mapURL,
		})
	}
	return out
}

func questDigestStopName(summary *digest.QuestDigestSummary, stopKey string) string {
	stopKey = strings.TrimSpace(stopKey)
	if stopKey == "" || summary == nil {
		return stopKey
	}
	if summary.StopNames != nil {
		if stopName := strings.TrimSpace(summary.StopNames[stopKey]); stopName != "" {
			return stopName
		}
	}
	return stopKey
}

func questDigestReactMapURL(cfg *config.Config, stopKey string) string {
	if cfg == nil {
		return ""
	}
	stopID := questDigestStopID(stopKey)
	if stopID == "" {
		return ""
	}
	base, ok := cfg.GetString("general.reactMapURL")
	if !ok {
		return ""
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + "id/pokestops/" + stopID
}

func questDigestDiademURL(cfg *config.Config, stopKey string) string {
	if cfg == nil {
		return ""
	}
	stopID := questDigestStopID(stopKey)
	if stopID == "" {
		return ""
	}
	base, ok := cfg.GetString("general.diademURL")
	if !ok {
		return ""
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + "pokestop/" + stopID
}

func questDigestStopID(stopKey string) string {
	stopKey = strings.TrimSpace(stopKey)
	if stopKey == "" {
		return ""
	}
	// Coordinate fallback keys ("lat,lon") cannot be converted to provider stop URLs.
	if strings.Contains(stopKey, ",") {
		return ""
	}
	return stopKey
}

func questDigestStopLines(stops []map[string]any) []string {
	lines := []string{}
	for _, entry := range stops {
		stop := getString(entry["stop"])
		noAR, _ := entry["noAR"].([]string)
		withAR, _ := entry["withAR"].([]string)
		if strings.TrimSpace(stop) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("At %s", stop))
		lines = append(lines, questDigestRewardLines("With AR: ", withAR)...)
		lines = append(lines, questDigestRewardLines("No AR: ", noAR)...)
	}
	return lines
}

func questDigestRewardLines(prefix string, rewards []string) []string {
	if len(rewards) == 0 {
		return nil
	}
	const perLine = 10
	out := []string{}
	for i := 0; i < len(rewards); i += perLine {
		end := i + perLine
		if end > len(rewards) {
			end = len(rewards)
		}
		segment := strings.Join(rewards[i:end], ", ")
		if strings.TrimSpace(segment) == "" {
			continue
		}
		if i == 0 {
			out = append(out, prefix+segment)
		} else {
			out = append(out, "  "+segment)
		}
	}
	return out
}

func mapKeysSorted(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func renderTemplateAny(value any, data map[string]any, meta map[string]any) any {
	switch v := value.(type) {
	case string:
		if rendered, err := render.RenderHandlebars(v, data, meta); err == nil {
			return rendered
		}
		return v
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, renderTemplateAny(item, data, meta))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = renderTemplateAny(item, data, meta)
		}
		return out
	default:
		return value
	}
}

func selectTemplatePayload(templates []dts.Template, platform, templateType, language string) any {
	var fallback any
	for _, tpl := range templates {
		if tpl.Hidden || tpl.Platform != platform || tpl.Type != templateType {
			continue
		}
		if language != "" && tpl.Language != nil && *tpl.Language != language {
			continue
		}
		fallback = tpl.Template
		if tpl.Default {
			return tpl.Template
		}
	}
	if fallback != nil {
		return fallback
	}
	for _, tpl := range templates {
		if tpl.Hidden || tpl.Platform != platform || tpl.Type != templateType {
			continue
		}
		if tpl.Default {
			return tpl.Template
		}
	}
	return nil
}

func platformFromType(targetType string) string {
	if strings.HasPrefix(targetType, "telegram") {
		return "telegram"
	}
	if targetType == "webhook" {
		return "discord"
	}
	return "discord"
}
