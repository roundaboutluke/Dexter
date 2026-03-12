package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) handleProfileScheduleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken string) {
	if strings.EqualFold(strings.TrimSpace(profileToken), "all") {
		d.handleProfileScheduleAddGlobal(s, i)
		return
	}
	embed, components, errText := d.buildProfileScheduleDayPayload(i, profileToken)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleDay(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken, dayValue string) {
	day := parseDayValue(dayValue)
	if day == 0 {
		d.respondEphemeral(s, i, "Please select a day.")
		return
	}
	customID := fmt.Sprintf("%s%s:%d", slashProfileScheduleTime, strings.TrimSpace(profileToken), day)
	d.respondWithScheduleModal(s, i, customID, "Start time", "End time", "", "")
}

func (d *Discord) handleProfileScheduleTime(s *discordgo.Session, i *discordgo.InteractionCreate, payload string, data discordgo.ModalSubmitInteractionData) {
	parts := strings.Split(payload, ":")
	if len(parts) < 2 {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	profileToken := strings.TrimSpace(parts[0])
	dayPart := strings.TrimSpace(parts[1])
	day := toInt(dayPart, 0)
	if day < 1 || day > 7 {
		day = 0
	}
	startText := strings.TrimSpace(modalTextValue(data, "start"))
	endText := strings.TrimSpace(modalTextValue(data, "end"))
	startMin, ok := parseClockMinutes(startText)
	if !ok {
		d.respondEphemeral(s, i, "Start time must be in HH:MM.")
		return
	}
	endMin, ok := parseClockMinutes(endText)
	if !ok {
		d.respondEphemeral(s, i, "End time must be in HH:MM.")
		return
	}
	if endMin <= startMin {
		d.respondEphemeral(s, i, "End time must be after start time.")
		return
	}
	if strings.EqualFold(profileToken, "all") {
		days := parseDayList(dayPart)
		if len(days) == 0 {
			d.respondEphemeral(s, i, "Invalid day selected.")
			return
		}
		embed, components, errText := d.buildProfileScheduleAssignPayload(i, days, startMin, endMin)
		if errText != "" {
			d.respondEphemeral(s, i, errText)
			return
		}
		d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
		return
	}
	if day == 0 {
		d.respondEphemeral(s, i, "Invalid day selected.")
		return
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries, errText := addScheduleEntry(profiles, selected, day, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if err := d.persistSlashScheduleUpdates(userID, map[int][]scheduleEntry{
		toInt(selected["profile_no"], 0): entries,
	}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken, value string) {
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	updated := removeScheduleEntry(entries, value)
	if len(updated) == len(entries) {
		d.respondEphemeral(s, i, "Schedule entry not found.")
		return
	}
	if err := d.persistSlashScheduleUpdates(userID, map[int][]scheduleEntry{
		toInt(selected["profile_no"], 0): updated,
	}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleRemoveGlobal(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	profileNo, entry, ok := parseGlobalScheduleValue(value)
	if !ok {
		d.respondEphemeral(s, i, "Schedule entry not found.")
		return
	}
	selected := profileRowByNo(profiles, profileNo)
	removed := false
	updates := map[int][]scheduleEntry{}
	if selected != nil {
		entries := scheduleEntriesFromRaw(selected["active_hours"])
		updated := removeScheduleEntry(entries, entry)
		if len(updated) != len(entries) {
			updates[toInt(selected["profile_no"], 0)] = updated
			removed = true
		}
	}
	if !removed {
		for _, row := range profiles {
			entries := scheduleEntriesFromRaw(row["active_hours"])
			updated := removeScheduleEntry(entries, entry)
			if len(updated) == len(entries) {
				continue
			}
			updates[toInt(row["profile_no"], 0)] = updated
			removed = true
			break
		}
	}
	if !removed {
		d.respondEphemeral(s, i, "Schedule entry not found.")
		return
	}
	if err := d.persistSlashScheduleUpdates(userID, updates); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleClear(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken string) {
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	if err := d.persistSlashScheduleUpdates(userID, map[int][]scheduleEntry{
		toInt(selected["profile_no"], 0): {},
	}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleAddGlobal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed, components, errText := d.buildProfileScheduleDayPayloadGlobal(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleDayGlobal(s *discordgo.Session, i *discordgo.InteractionCreate, dayValues []string) {
	days := parseDayValues(dayValues)
	if len(days) == 0 {
		d.respondEphemeral(s, i, "Please select at least one day.")
		return
	}
	customID := fmt.Sprintf("%sall:%s", slashProfileScheduleTime, joinDayList(days))
	d.respondWithScheduleModal(s, i, customID, "Start time", "End time", "", "")
}

func (d *Discord) handleProfileScheduleOverview(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleAssign(s *discordgo.Session, i *discordgo.InteractionCreate, payload, profileValue string) {
	days, startMin, endMin, ok := parseAssignPayloadDays(payload)
	if !ok || len(days) == 0 {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	selected := profileRowByToken(profiles, profileValue)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries, errText := addScheduleEntriesForDays(profiles, selected, days, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if err := d.persistSlashScheduleUpdates(userID, map[int][]scheduleEntry{
		toInt(selected["profile_no"], 0): entries,
	}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleEditSelect(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	entry, ok := parseScheduleValue(value)
	if !ok || entry.Legacy {
		d.respondEphemeral(s, i, "That entry cannot be edited.")
		return
	}
	embed, components, errText := d.buildProfileScheduleEditDayPayload(entry)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleEditDay(s *discordgo.Session, i *discordgo.InteractionCreate, payload, dayValue string) {
	entry, ok := parseScheduleValue(payload)
	if !ok {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	day := parseDayValue(dayValue)
	if day == 0 {
		d.respondEphemeral(s, i, "Please select a day.")
		return
	}
	startValue := fmt.Sprintf("%02d:%02d", entry.StartMin/60, entry.StartMin%60)
	endValue := fmt.Sprintf("%02d:%02d", entry.EndMin/60, entry.EndMin%60)
	customID := fmt.Sprintf("%s%d|%s:%d", slashProfileScheduleEditTime, entry.ProfileNo, scheduleEntryValue(entry), day)
	d.respondWithScheduleModal(s, i, customID, "Start time", "End time", startValue, endValue)
}

func (d *Discord) handleProfileScheduleEditTime(s *discordgo.Session, i *discordgo.InteractionCreate, payload string, data discordgo.ModalSubmitInteractionData) {
	parts := strings.Split(payload, ":")
	if len(parts) < 2 {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	original, ok := parseScheduleValue(parts[0])
	if !ok {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	day := toInt(parts[1], 0)
	if day < 1 || day > 7 {
		d.respondEphemeral(s, i, "Invalid day selected.")
		return
	}
	startText := strings.TrimSpace(modalTextValue(data, "start"))
	endText := strings.TrimSpace(modalTextValue(data, "end"))
	startMin, ok := parseClockMinutes(startText)
	if !ok {
		d.respondEphemeral(s, i, "Start time must be in HH:MM.")
		return
	}
	endMin, ok := parseClockMinutes(endText)
	if !ok {
		d.respondEphemeral(s, i, "End time must be in HH:MM.")
		return
	}
	if endMin <= startMin {
		d.respondEphemeral(s, i, "End time must be after start time.")
		return
	}
	embed, components, errText := d.buildProfileScheduleEditAssignPayload(i, original, day, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleEditAssign(s *discordgo.Session, i *discordgo.InteractionCreate, payload, profileValue string) {
	original, day, startMin, endMin, ok := parseEditAssignPayload(payload)
	if !ok {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	selected := profileRowByToken(profiles, profileValue)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	updates, errText := buildScheduleEditAssignUpdates(profiles, selected, original, day, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if err := d.persistSlashScheduleUpdates(userID, updates); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleToggle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	userID, _ := slashUser(i)
	if userID == "" {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	disabled := toInt(human["schedule_disabled"], 0) == 1
	update := map[string]any{"schedule_disabled": 0}
	if !disabled {
		update["schedule_disabled"] = 1
		if current := toInt(human["current_profile_no"], 0); current == 0 {
			profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
			if err == nil && len(profiles) > 0 {
				sort.Slice(profiles, func(i, j int) bool {
					return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0)
				})
				fallback := toInt(profiles[0]["profile_no"], 1)
				update["current_profile_no"] = fallback
				if toInt(human["preferred_profile_no"], 0) == 0 {
					update["preferred_profile_no"] = fallback
				}
			} else {
				update["current_profile_no"] = 1
				if toInt(human["preferred_profile_no"], 0) == 0 {
					update["preferred_profile_no"] = 1
				}
			}
		}
	}
	if err := d.persistSlashHumanUpdate(userID, update); err != nil {
		d.respondEphemeral(s, i, "Unable to update scheduler.")
		return
	}
	d.handleProfileShow(s, i)
}
