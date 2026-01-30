package bot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"poraclego/internal/community"
	"poraclego/internal/dts"
	"poraclego/internal/render"
)

func (t *Telegram) startReconciliation() {
	if t.bot == nil {
		return
	}
	enabled, _ := t.manager.cfg.GetBool("telegram.checkRole")
	intervalHours, _ := t.manager.cfg.GetInt("telegram.checkRoleInterval")
	if !enabled || intervalHours <= 0 {
		return
	}
	time.AfterFunc(30*time.Second, func() {
		t.runReconciliation()
	})
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	go func() {
		for range ticker.C {
			t.runReconciliation()
		}
	}()
}

func (t *Telegram) runReconciliation() {
	if t.bot == nil || t.manager == nil || t.manager.cfg == nil || t.manager.query == nil {
		return
	}
	updateNames, _ := t.manager.cfg.GetBool("reconciliation.telegram.updateUserNames")
	removeInvalid, _ := t.manager.cfg.GetBool("reconciliation.telegram.removeInvalidUsers")
	t.syncTelegramChannels()
	t.syncTelegramUsers(updateNames, removeInvalid)
}

func (t *Telegram) syncTelegramUser(userID int64, syncNames, removeInvalid bool) {
	if t.bot == nil || t.manager == nil || t.manager.cfg == nil || t.manager.query == nil {
		return
	}
	id := fmt.Sprintf("%d", userID)
	user, _ := t.manager.query.SelectOneQuery("humans", map[string]any{"id": id, "type": "telegram:user"})
	channelList := t.telegramChannelList()
	name, channels := t.loadTelegramChannels(userID, channelList)
	info := &telegramUserInfo{name: name, channels: channels}
	t.reconcileTelegramUser(id, user, info, syncNames, removeInvalid)
}

func (t *Telegram) syncTelegramUsers(syncNames, removeInvalid bool) {
	users, err := t.manager.query.SelectAllQuery("humans", map[string]any{"type": "telegram:user"})
	if err != nil {
		return
	}
	admins, _ := t.manager.cfg.GetStringSlice("telegram.admins")
	channelList := t.telegramChannelList()
	for _, row := range users {
		userID := getString(row["id"])
		if containsString(admins, userID) {
			continue
		}
		idNum, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			continue
		}
		name, channels := t.loadTelegramChannels(idNum, channelList)
		info := &telegramUserInfo{name: name, channels: channels}
		t.reconcileTelegramUser(userID, row, info, syncNames, removeInvalid)
	}
}

type telegramUserInfo struct {
	name     string
	channels []string
}

func (t *Telegram) reconcileTelegramUser(id string, user map[string]any, info *telegramUserInfo, syncNames, removeInvalid bool) {
	channelList := []string{}
	name := ""
	if info != nil {
		channelList = info.channels
		name = info.name
	}
	areaEnabled, _ := t.manager.cfg.GetBool("areaSecurity.enabled")
	if !areaEnabled {
		before := user != nil && toInt(user["admin_disable"], 0) == 0
		after := t.channelListHasAny(channelList)
		if !before && after {
			if user == nil {
				_, _ = t.manager.query.InsertOrUpdateQuery("humans", map[string]any{
					"id":                   id,
					"type":                 "telegram:user",
					"name":                 name,
					"area":                 "[]",
					"community_membership": "[]",
				})
				t.sendGreetingsTelegram(id)
			} else if toInt(user["admin_disable"], 0) == 1 && hasDisabledDate(user) {
				_, _ = t.manager.query.UpdateQuery("humans", map[string]any{
					"admin_disable": 0,
					"disabled_date": nil,
				}, map[string]any{"id": id})
				t.sendGreetingsTelegram(id)
			}
		}
		if before && !after && removeInvalid {
			t.disableTelegramUser(user)
		}
		if before && after && syncNames && getString(user["name"]) != name {
			_, _ = t.manager.query.UpdateQuery("humans", map[string]any{"name": name}, map[string]any{"id": id})
		}
		return
	}

	communityList := t.communitiesForTelegramChannels(channelList)
	before := user != nil && toInt(user["admin_disable"], 0) == 0
	after := len(communityList) > 0
	areaRestriction := community.CalculateLocationRestrictions(t.manager.cfg, communityList)
	if !before && after {
		if user == nil {
			_, _ = t.manager.query.InsertOrUpdateQuery("humans", map[string]any{
				"id":                   id,
				"type":                 "telegram:user",
				"name":                 name,
				"area":                 "[]",
				"area_restriction":     toJSON(areaRestriction),
				"community_membership": toJSON(communityList),
			})
			t.sendGreetingsTelegram(id)
		} else if toInt(user["admin_disable"], 0) == 1 && hasDisabledDate(user) {
			_, _ = t.manager.query.UpdateQuery("humans", map[string]any{
				"admin_disable":        0,
				"disabled_date":        nil,
				"area_restriction":     toJSON(areaRestriction),
				"community_membership": toJSON(communityList),
			}, map[string]any{"id": id})
			t.sendGreetingsTelegram(id)
		}
	}
	if before && !after && removeInvalid {
		t.disableTelegramUser(user)
	}
	if before && after {
		updates := map[string]any{}
		if syncNames && getString(user["name"]) != name {
			updates["name"] = name
		}
		if !sameContents(parseStringList(user["area_restriction"]), areaRestriction) {
			updates["area_restriction"] = toJSON(areaRestriction)
		}
		if !sameContents(parseStringList(user["community_membership"]), communityList) {
			updates["community_membership"] = toJSON(communityList)
		}
		if len(updates) > 0 {
			_, _ = t.manager.query.UpdateQuery("humans", updates, map[string]any{"id": id})
		}
	}
}

func (t *Telegram) telegramChannelList() []string {
	areaEnabled, _ := t.manager.cfg.GetBool("areaSecurity.enabled")
	if !areaEnabled {
		channels, _ := t.manager.cfg.GetStringSlice("telegram.channels")
		return channels
	}
	raw, ok := t.manager.cfg.Get("areaSecurity.communities")
	if !ok {
		return []string{}
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return []string{}
	}
	result := []string{}
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		telegram, ok := entryMap["telegram"].(map[string]any)
		if !ok {
			continue
		}
		channels, ok := telegram["channels"].([]any)
		if !ok {
			continue
		}
		for _, channel := range channels {
			result = append(result, getString(channel))
		}
	}
	return result
}

func (t *Telegram) loadTelegramChannels(userID int64, channelList []string) (string, []string) {
	valid := []string{}
	name := ""
	for _, channel := range channelList {
		chatID, err := strconv.ParseInt(channel, 10, 64)
		if err != nil {
			continue
		}
		member, err := t.bot.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: chatID,
				UserID: userID,
			},
		})
		if err != nil {
			continue
		}
		if name == "" && member.User != nil {
			fullName := member.User.FirstName
			if member.User.LastName != "" {
				fullName = fullName + " " + member.User.LastName
			}
			if member.User.UserName != "" {
				fullName = fullName + " [" + member.User.UserName + "]"
			}
			name = strings.TrimSpace(fullName)
		}
		if member.Status != "left" && member.Status != "kicked" {
			valid = append(valid, channel)
		}
	}
	return name, valid
}

func (t *Telegram) channelListHasAny(channels []string) bool {
	allowed := t.telegramChannelList()
	for _, channel := range channels {
		if containsString(allowed, channel) {
			return true
		}
	}
	return false
}

func (t *Telegram) communitiesForTelegramChannels(channels []string) []string {
	raw, ok := t.manager.cfg.Get("areaSecurity.communities")
	if !ok {
		return []string{}
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return []string{}
	}
	result := []string{}
	for name, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		telegram, ok := entryMap["telegram"].(map[string]any)
		if !ok {
			continue
		}
		rawChannels, ok := telegram["channels"].([]any)
		if !ok {
			continue
		}
		for _, channel := range rawChannels {
			if containsString(channels, getString(channel)) {
				result = append(result, strings.ToLower(name))
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

func (t *Telegram) syncTelegramChannels() {
	rows, err := t.manager.query.SelectAllQuery("humans", map[string]any{
		"type":          "telegram:channel",
		"admin_disable": 0,
	})
	if err != nil {
		return
	}
	groups, err := t.manager.query.SelectAllQuery("humans", map[string]any{
		"type":          "telegram:group",
		"admin_disable": 0,
	})
	if err == nil {
		rows = append(rows, groups...)
	}
	for _, row := range rows {
		if row["area_restriction"] == nil || row["community_membership"] == nil {
			continue
		}
		communities := parseStringList(row["community_membership"])
		restriction := community.CalculateLocationRestrictions(t.manager.cfg, communities)
		if !sameContents(parseStringList(row["area_restriction"]), restriction) {
			_, _ = t.manager.query.UpdateQuery("humans", map[string]any{
				"area_restriction": toJSON(restriction),
			}, map[string]any{"id": row["id"]})
		}
	}
}

func (t *Telegram) disableTelegramUser(user map[string]any) {
	mode, _ := t.manager.cfg.GetString("general.roleCheckMode")
	id := getString(user["id"])
	switch mode {
	case "disable-user":
		if toInt(user["admin_disable"], 0) == 0 {
			_, _ = t.manager.query.UpdateQuery("humans", map[string]any{
				"admin_disable": 1,
				"disabled_date": time.Now(),
			}, map[string]any{"id": id})
			t.sendGoodbyeTelegram(id)
		}
	case "delete":
		t.removeUserTracking(id)
		_, _ = t.manager.query.DeleteQuery("profiles", map[string]any{"id": id})
		_, _ = t.manager.query.DeleteQuery("humans", map[string]any{"id": id})
		t.sendGoodbyeTelegram(id)
	}
}

func (t *Telegram) removeUserTracking(id string) {
	_, _ = t.manager.query.DeleteQuery("egg", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("monsters", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("raid", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("quest", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("lures", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("gym", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("invasion", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("nests", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("forts", map[string]any{"id": id})
	_, _ = t.manager.query.DeleteQuery("weather", map[string]any{"id": id})
}

func (t *Telegram) sendGreetingsTelegram(userID string) {
	disableGreetings, _ := t.manager.cfg.GetBool("telegram.disableAutoGreetings")
	if disableGreetings || t.bot == nil {
		return
	}
	tpl := findGreetingTemplateTelegram(t.manager.templates)
	if tpl == nil {
		return
	}
	view := map[string]any{
		"prefix": "/",
	}
	payload := renderTelegramTemplateMessages(tpl.Template, view)
	if len(payload) == 0 {
		return
	}
	userNum, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}
	for _, message := range payload {
		_, _ = t.bot.Send(tgbotapi.NewMessage(userNum, message))
	}
}

func (t *Telegram) sendGoodbyeTelegram(userID string) {
	msg, ok := t.manager.cfg.GetString("telegram.botGoodbyeMessage")
	if !ok || msg == "" || t.bot == nil {
		return
	}
	userNum, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}
	_, _ = t.bot.Send(tgbotapi.NewMessage(userNum, msg))
}

func findGreetingTemplateTelegram(templates []dts.Template) *dts.Template {
	for _, tpl := range templates {
		if tpl.Type == "greeting" && tpl.Platform == "telegram" && tpl.Default {
			return &tpl
		}
	}
	return nil
}

func renderTelegramTemplateMessages(template any, view map[string]any) []string {
	raw, err := json.Marshal(template)
	if err != nil {
		return nil
	}
	text, err := render.RenderHandlebars(string(raw), view, nil)
	if err != nil {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	embed, ok := payload["embed"].(map[string]any)
	if !ok {
		return nil
	}
	fields, ok := embed["fields"].([]any)
	if !ok {
		return nil
	}
	var out []string
	var buf strings.Builder
	for _, field := range fields {
		entry, ok := field.(map[string]any)
		if !ok {
			continue
		}
		name := getString(entry["name"])
		value := getString(entry["value"])
		chunk := fmt.Sprintf("\n\n%s\n\n%s", name, value)
		if buf.Len()+len(chunk) > 1024 {
			out = append(out, buf.String())
			buf.Reset()
		}
		buf.WriteString(chunk)
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}
