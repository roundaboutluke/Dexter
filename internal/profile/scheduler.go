package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"poraclego/internal/config"
	"poraclego/internal/db"
	"poraclego/internal/digest"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/render"
	"poraclego/internal/tz"
)

// Scheduler checks active hours and switches profiles as needed.
type Scheduler struct {
	cfg           *config.Config
	query         *db.Query
	i18n          *i18n.Factory
	tzLocator     *tz.Locator
	discordQueue  *dispatch.Queue
	telegramQueue *dispatch.Queue
	questDigests  *digest.Store
	templates     []dts.Template
}

// NewScheduler creates a profile scheduler.
func NewScheduler(cfg *config.Config, query *db.Query, i18nFactory *i18n.Factory, tzLocator *tz.Locator, discordQueue, telegramQueue *dispatch.Queue, digestStore *digest.Store, templates []dts.Template) *Scheduler {
	return &Scheduler{
		cfg:           cfg,
		query:         query,
		i18n:          i18nFactory,
		tzLocator:     tzLocator,
		discordQueue:  discordQueue,
		telegramQueue: telegramQueue,
		questDigests:  digestStore,
		templates:     templates,
	}
}

// Start begins the periodic profile checks.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Scheduler) run(ctx context.Context) {
	s.waitForBoundary(ctx, time.Minute)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.check()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) waitForBoundary(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	now := time.Now()
	next := now.Truncate(interval).Add(interval)
	delay := time.Until(next)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (s *Scheduler) check() {
	if s == nil || s.query == nil {
		return
	}
	profiles, err := s.query.MysteryQuery("SELECT * FROM profiles WHERE LENGTH(active_hours)>5 ORDER BY id, profile_no")
	if err != nil {
		s.logf("profile schedule: load profiles failed: %v", err)
		return
	}
	if len(profiles) == 0 {
		return
	}
	ids := make([]any, 0, len(profiles))
	seenIDs := map[string]struct{}{}
	for _, row := range profiles {
		id := getString(row["id"])
		if id == "" {
			continue
		}
		if _, ok := seenIDs[id]; ok {
			continue
		}
		seenIDs[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	humans, err := s.query.SelectWhereInQuery("humans", ids, "id")
	if err != nil {
		s.logf("profile schedule: load humans failed: %v", err)
		return
	}
	humanByID := map[string]map[string]any{}
	for _, row := range humans {
		id := getString(row["id"])
		if id != "" {
			if toInt(row["enabled"]) != 1 || toInt(row["admin_disable"]) != 0 {
				continue
			}
			humanByID[id] = row
		}
	}
	profilesByID := map[string][]map[string]any{}
	for _, row := range profiles {
		id := getString(row["id"])
		if id == "" {
			continue
		}
		profilesByID[id] = append(profilesByID[id], row)
	}
	for id, rows := range profilesByID {
		human := humanByID[id]
		if human == nil {
			continue
		}
		if toInt(human["schedule_disabled"]) == 1 {
			continue
		}
		if hasRangeSchedules(rows) {
			active := []map[string]any{}
			for _, row := range rows {
				ranges := parseScheduleRanges(row["active_hours"])
				if len(ranges) == 0 {
					continue
				}
				now := s.nowForProfile(human, row)
				if inScheduleRange(now, ranges) {
					active = append(active, row)
				}
			}
			if len(active) > 1 {
				s.logf("profile schedule: overlap detected for %s", id)
				continue
			}
			current := toInt(human["current_profile_no"])
			if len(active) == 0 {
				if current != 0 {
					if _, err := s.query.UpdateQuery("humans", map[string]any{"current_profile_no": 0}, map[string]any{"id": id}); err != nil {
						s.logf("profile schedule: clear failed for %s: %v", id, err)
					} else {
						s.notifyQuiet(human, rows)
					}
				} else if s.questDigests != nil {
					// Service restarts during quiet hours won't trigger notifyQuiet, but should still keep
					// a stable digest cycle key until active hours resume.
					targetID := getString(human["id"])
					s.questDigests.BeginQuiet(targetID)
				}
				continue
			}
			target := active[0]
			profileNo := toInt(target["profile_no"])
			if profileNo == 0 || profileNo == current {
				continue
			}
			update := map[string]any{
				"current_profile_no": profileNo,
				"area":               target["area"],
				"latitude":           target["latitude"],
				"longitude":          target["longitude"],
			}
			if _, err := s.query.UpdateQuery("humans", update, map[string]any{"id": id}); err != nil {
				s.logf("profile schedule: update failed for %s: %v", id, err)
				continue
			}
			if current == 0 {
				s.notifyResume(human, target)
			} else {
				s.notifySwitch(human, target)
			}
			s.logf("profile schedule: set %s to profile %d", id, profileNo)
			continue
		}
		for _, row := range rows {
			times := parseActiveHours(row["active_hours"])
			if len(times) == 0 {
				continue
			}
			now := s.nowForProfile(human, row)
			if !matchesActiveHours(now, times) {
				continue
			}
			current := toInt(human["current_profile_no"])
			if current == 0 {
				current = 1
			}
			profileNo := toInt(row["profile_no"])
			if profileNo == 0 || profileNo == current {
				break
			}
			update := map[string]any{
				"current_profile_no": profileNo,
				"area":               row["area"],
				"latitude":           row["latitude"],
				"longitude":          row["longitude"],
			}
			if _, err := s.query.UpdateQuery("humans", update, map[string]any{"id": id}); err != nil {
				s.logf("profile schedule: update failed for %s: %v", id, err)
				break
			}
			s.notifySwitch(human, row)
			s.logf("profile schedule: set %s to profile %d", id, profileNo)
			break
		}
	}
}

func (s *Scheduler) nowForHuman(human map[string]any) time.Time {
	now := time.Now()
	lat := getFloat(human["latitude"])
	lon := getFloat(human["longitude"])
	if (lat != 0 || lon != 0) && s.tzLocator != nil {
		if loc, ok := s.tzLocator.Location(lat, lon); ok && loc != nil {
			return now.In(loc)
		}
	}
	return now
}

func (s *Scheduler) nowForProfile(human map[string]any, profile map[string]any) time.Time {
	now := time.Now()
	lat := getFloat(profile["latitude"])
	lon := getFloat(profile["longitude"])
	if (lat != 0 || lon != 0) && s.tzLocator != nil {
		if loc, ok := s.tzLocator.Location(lat, lon); ok && loc != nil {
			return now.In(loc)
		}
	}
	return s.nowForHuman(human)
}

func matchesActiveHours(now time.Time, times []activeHour) bool {
	nowHour := now.Hour()
	nowMinutes := now.Minute()
	nowDow := isoWeekday(now.Weekday())
	yesterdayDow := 7
	if nowDow > 1 {
		yesterdayDow = nowDow - 1
	}
	for _, entry := range times {
		rowDay := entry.Day
		rowHours := entry.Hours
		rowMins := entry.Mins
		if rowDay == nowDow && rowHours == nowHour && nowMinutes >= rowMins && (nowMinutes-rowMins) < 10 {
			return true
		}
		if nowMinutes < 10 && rowDay == nowDow && rowHours == nowHour-1 && rowMins > 50 {
			return true
		}
		if nowHour == 0 && nowMinutes < 10 && rowDay == yesterdayDow && rowHours == 23 && rowMins > 50 {
			return true
		}
	}
	return false
}

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
		message = tr.TranslateFormat("Quiet hours enabled. Alerts paused until {0}. Adjust schedules with /profile.", formatScheduleTime(next))
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
	if profileName == "" {
		profileName = fmt.Sprintf("Profile %d", toInt(profile["profile_no"]))
	}
	lang := getString(human["language"])
	if s.cfg != nil && lang == "" {
		if fallback, ok := s.cfg.GetString("general.locale"); ok {
			lang = fallback
		}
	}
	tr := s.i18n.Translator(lang)
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
	if stops := questDigestStops(summary); len(stops) > 0 {
		lines = questDigestStopLines(stops)
	} else {
		lines = digest.RewardsWithStops(summary.Rewards, summary.Stops)
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
	current := ""
	push := func() {
		if strings.TrimSpace(current) != "" {
			pages = append(pages, current)
		}
		current = ""
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		for len(line) > maxLen {
			part := line[:maxLen]
			line = line[maxLen:]
			if current != "" {
				push()
			}
			pages = append(pages, part)
		}
		if current == "" {
			current = line
			continue
		}
		if len(current)+1+len(line) <= maxLen {
			current = current + "\n" + line
			continue
		}
		push()
		current = line
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
	stops := questDigestStops(summary)
	if len(stops) > 0 {
		lines = append(lines, questDigestStopLines(stops)...)
	} else {
		rewards := digest.RewardsWithStops(summary.Rewards, summary.Stops)
		lines = append(lines, rewards...)
	}
	return strings.Join(lines, "\n")
}

func (s *Scheduler) renderQuestDigestTemplate(targetType, language, profileName string, summary *digest.QuestDigestSummary) (map[string]any, string) {
	if summary == nil || summary.Total == 0 || len(s.templates) == 0 {
		return nil, ""
	}
	platform := platformFromType(targetType)
	template := selectTemplatePayload(s.templates, platform, "questdigest", language)
	if template == nil {
		return nil, ""
	}
	rewards := digest.RewardsWithStops(summary.Rewards, summary.Stops)
	stops := questDigestStops(summary)
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

func questDigestStops(summary *digest.QuestDigestSummary) []map[string]any {
	if summary == nil || len(summary.Stops) == 0 {
		return nil
	}
	type stopEntry struct {
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
		for stop := range stops {
			if strings.TrimSpace(stop) == "" {
				continue
			}
			entry := byStop[stop]
			if entry == nil {
				entry = &stopEntry{
					NoAR:   map[string]bool{},
					WithAR: map[string]bool{},
				}
				byStop[stop] = entry
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
	stopNames := make([]string, 0, len(byStop))
	for stop := range byStop {
		stopNames = append(stopNames, stop)
	}
	sort.Strings(stopNames)
	out := make([]map[string]any, 0, len(stopNames))
	for _, stop := range stopNames {
		entry := byStop[stop]
		if entry == nil {
			continue
		}
		noAR := mapKeysSorted(entry.NoAR)
		withAR := mapKeysSorted(entry.WithAR)
		out = append(out, map[string]any{
			"stop":        stop,
			"noAR":        noAR,
			"noARCount":   len(noAR),
			"withAR":      withAR,
			"withARCount": len(withAR),
		})
	}
	return out
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

type activeHour struct {
	Day   int
	Hours int
	Mins  int
}

type scheduleRange struct {
	Day      int
	StartMin int
	EndMin   int
}

func parseActiveHours(raw any) []activeHour {
	text := rawJSONText(raw)
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := make([]activeHour, 0, len(rows))
	for _, row := range rows {
		day := toInt(row["day"])
		if day < 1 || day > 7 {
			continue
		}
		out = append(out, activeHour{
			Day:   day,
			Hours: toInt(row["hours"]),
			Mins:  toInt(row["mins"]),
		})
	}
	return out
}

func parseScheduleRanges(raw any) []scheduleRange {
	text := rawJSONText(raw)
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := []scheduleRange{}
	for _, row := range rows {
		day := toInt(row["day"])
		if day < 1 || day > 7 {
			continue
		}
		startHours, ok := row["start_hours"]
		if !ok {
			continue
		}
		startMins := toInt(row["start_mins"])
		endHours := toInt(row["end_hours"])
		endMins := toInt(row["end_mins"])
		start := toInt(startHours)*60 + startMins
		end := endHours*60 + endMins
		if end <= start {
			continue
		}
		out = append(out, scheduleRange{Day: day, StartMin: start, EndMin: end})
	}
	return out
}

func rawJSONText(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case json.RawMessage:
		return strings.TrimSpace(string(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
}

func hasRangeSchedules(rows []map[string]any) bool {
	for _, row := range rows {
		if len(parseScheduleRanges(row["active_hours"])) > 0 {
			return true
		}
	}
	return false
}

func inScheduleRange(now time.Time, ranges []scheduleRange) bool {
	if len(ranges) == 0 {
		return false
	}
	nowDow := isoWeekday(now.Weekday())
	nowMin := now.Hour()*60 + now.Minute()
	for _, entry := range ranges {
		if entry.Day != nowDow {
			continue
		}
		if nowMin >= entry.StartMin && nowMin < entry.EndMin {
			return true
		}
	}
	return false
}

func (s *Scheduler) nextScheduleTime(human map[string]any, profiles []map[string]any) (time.Time, bool) {
	var next time.Time
	found := false
	for _, profile := range profiles {
		ranges := parseScheduleRanges(profile["active_hours"])
		if len(ranges) == 0 {
			continue
		}
		now := s.nowForProfile(human, profile)
		if candidate, ok := nextScheduleStart(now, ranges); ok {
			if !found || candidate.UTC().Before(next.UTC()) {
				next = candidate
				found = true
			}
		}
	}
	return next, found
}

func nextScheduleStart(now time.Time, ranges []scheduleRange) (time.Time, bool) {
	if len(ranges) == 0 {
		return time.Time{}, false
	}
	nowDow := isoWeekday(now.Weekday())
	nowMin := now.Hour()*60 + now.Minute()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var next time.Time
	found := false
	for _, entry := range ranges {
		deltaDays := (entry.Day - nowDow + 7) % 7
		if deltaDays == 0 && entry.StartMin <= nowMin {
			deltaDays = 7
		}
		candidate := startOfDay.AddDate(0, 0, deltaDays).Add(time.Duration(entry.StartMin) * time.Minute)
		if !found || candidate.Before(next) {
			next = candidate
			found = true
		}
	}
	return next, found
}

func formatScheduleTime(t time.Time) string {
	return t.Format("Mon 15:04")
}

func isoWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func getString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func getFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		var out float64
		_, _ = fmt.Sscanf(strings.TrimSpace(v), "%f", &out)
		return out
	default:
		return 0
	}
}

func (s *Scheduler) logf(format string, args ...any) {
	logger := logging.Get().General
	if logger == nil {
		return
	}
	logger.Infof(format, args...)
}
