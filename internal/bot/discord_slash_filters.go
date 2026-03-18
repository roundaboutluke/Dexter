package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"dexter/internal/db"
	"dexter/internal/i18n"
	"dexter/internal/logging"
	"dexter/internal/tracking"
)

const (
	slashFilterRemoveButtonPrefix  = "poracle:filter:remove:"
	slashFilterRestoreButtonPrefix = "poracle:filter:restore:"

	slashFilterCardColorConfirm  = 0x57F287 // Green: pre-save confirmation prompt
	slashFilterCardColorAdded    = 0x57F287 // Green: post-save success (intentionally same as confirm)
	slashFilterCardColorRemoved  = 0xED4245
	slashFilterCardColorRestored = 0x5865F2

	pokemonIDWildcard     = 9000
	defaultMaxCP          = 9000
	allLevelsSentinel     = 90
	filterActionExpiry    = 15 * time.Minute
	filterSummaryRowLimit = 5

	questRewardPokemon    = 7
	questRewardItem       = 2
	questRewardStardust   = 3
	questRewardMegaEnergy = 12
	questRewardCandy      = 4
	questRewardXLCandy    = 9
	questRewardExperience = 1
)

type slashFilterActionState struct {
	UserID       string
	Table        string
	TrackingType string
	Rows         []map[string]any
	ProfileLabel string
	ExpiresAt    time.Time
}

func cloneSlashFilterRows(rows []map[string]any) []map[string]any {
	cloned := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		copyRow := make(map[string]any, len(row))
		for key, value := range row {
			copyRow[key] = value
		}
		cloned = append(cloned, copyRow)
	}
	return cloned
}

func (d *Discord) nextSlashFilterActionID() string {
	d.filterMu.Lock()
	defer d.filterMu.Unlock()
	d.filterSeq++
	return fmt.Sprintf("%d", d.filterSeq)
}

func (d *Discord) storeSlashFilterAction(state *slashFilterActionState) string {
	if d == nil || state == nil {
		return ""
	}
	id := d.nextSlashFilterActionID()
	d.filterMu.Lock()
	defer d.filterMu.Unlock()
	if d.filterActions == nil {
		d.filterActions = map[string]*slashFilterActionState{}
	}
	now := time.Now()
	for key, action := range d.filterActions {
		if action == nil || action.ExpiresAt.Before(now) {
			delete(d.filterActions, key)
		}
	}
	d.filterActions[id] = &slashFilterActionState{
		UserID:       state.UserID,
		Table:        state.Table,
		TrackingType: state.TrackingType,
		Rows:         cloneSlashFilterRows(state.Rows),
		ProfileLabel: state.ProfileLabel,
		ExpiresAt:    state.ExpiresAt,
	}
	return id
}

func (d *Discord) slashFilterAction(id, userID string) (*slashFilterActionState, string) {
	if d == nil || id == "" {
		return nil, "That filter action is no longer available."
	}
	d.filterMu.Lock()
	defer d.filterMu.Unlock()
	state := d.filterActions[id]
	if state == nil {
		return nil, "That filter action is no longer available."
	}
	if state.ExpiresAt.Before(time.Now()) {
		delete(d.filterActions, id)
		return nil, "That filter action has expired."
	}
	if state.UserID != "" && userID != "" && state.UserID != userID {
		return nil, "That filter action belongs to another user."
	}
	delete(d.filterActions, id)
	return &slashFilterActionState{
		UserID:       state.UserID,
		Table:        state.Table,
		TrackingType: state.TrackingType,
		Rows:         cloneSlashFilterRows(state.Rows),
		ProfileLabel: state.ProfileLabel,
		ExpiresAt:    state.ExpiresAt,
	}, ""
}

func slashTrackingTypeFromCommand(command string) string {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "track", "pokemon":
		return "pokemon"
	case "raid":
		return "raid"
	case "egg":
		return "egg"
	case "maxbattle":
		return "maxbattle"
	case "quest":
		return "quest"
	case "invasion", "rocket":
		return "rocket"
	case "incident", "pokestop-event":
		return "pokestop-event"
	case "gym":
		return "gym"
	case "weather":
		return "weather"
	case "lure":
		return "lure"
	case "nest":
		return "nest"
	case "fort":
		return "fort"
	default:
		logging.Get().Discord.Debugf("unrecognized slash tracking command: %s", command)
		return ""
	}
}

func slashTrackingTitle(tr *i18n.Translator, trackingType string) string {
	switch trackingType {
	case "pokemon":
		return translateOrDefault(tr, "Pokemon")
	case "raid":
		return translateOrDefault(tr, "Raid")
	case "egg":
		return translateOrDefault(tr, "Egg")
	case "maxbattle":
		return translateOrDefault(tr, "Max Battle")
	case "quest":
		return translateOrDefault(tr, "Quest")
	case "rocket":
		return translateOrDefault(tr, "Rocket")
	case "pokestop-event":
		return translateOrDefault(tr, "Pokestop Event")
	case "gym":
		return translateOrDefault(tr, "Gym")
	case "fort":
		return translateOrDefault(tr, "Fort")
	case "nest":
		return translateOrDefault(tr, "Nest")
	case "weather":
		return translateOrDefault(tr, "Weather")
	case "lure":
		return translateOrDefault(tr, "Lure")
	default:
		return humanizeOptionName(trackingType)
	}
}

func (d *Discord) slashTrackingRowsForSelection(selection slashProfileSelection, trackingType string) ([]map[string]any, string) {
	if d == nil || d.manager == nil || d.manager.query == nil {
		return nil, ""
	}
	table := removeTrackingTable(trackingType)
	if table == "" {
		return nil, ""
	}
	where := map[string]any{"id": selection.UserID}
	if selection.Mode != slashProfileScopeAll && selection.ProfileNo > 0 {
		where["profile_no"] = selection.ProfileNo
	}
	rows, err := d.manager.query.SelectAllQuery(table, where)
	if err != nil {
		return nil, table
	}
	return d.filterRowsByTrackingType(rows, trackingType), table
}

func (d *Discord) filterRowsByTrackingType(rows []map[string]any, trackingType string) []map[string]any {
	trackingType = strings.ToLower(strings.TrimSpace(trackingType))
	if trackingType != "rocket" && trackingType != "pokestop-event" {
		return cloneSlashFilterRows(rows)
	}
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		gruntType := strings.TrimSpace(fmt.Sprintf("%v", row["grunt_type"]))
		isEvent := d.isPokestopEventType(gruntType)
		if trackingType == "rocket" && isEvent {
			continue
		}
		if trackingType == "pokestop-event" && !isEvent {
			continue
		}
		filtered = append(filtered, row)
	}
	return cloneSlashFilterRows(filtered)
}

func slashRowUID(row map[string]any) string {
	return strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
}

func slashRowFingerprint(row map[string]any) string {
	if row == nil {
		return ""
	}
	payload, _ := json.Marshal(row)
	return string(payload)
}

func slashChangedRows(beforeRows, afterRows []map[string]any) []map[string]any {
	beforeByUID := make(map[string]string, len(beforeRows))
	for _, row := range beforeRows {
		uid := slashRowUID(row)
		if uid == "" {
			continue
		}
		beforeByUID[uid] = slashRowFingerprint(row)
	}
	changed := []map[string]any{}
	for _, row := range afterRows {
		uid := slashRowUID(row)
		if uid == "" {
			continue
		}
		if beforeByUID[uid] != slashRowFingerprint(row) {
			changed = append(changed, row)
		}
	}
	return cloneSlashFilterRows(changed)
}

func slashFilterRowsByUID(rows []map[string]any, uid string) []map[string]any {
	if uid == "" {
		return nil
	}
	filtered := []map[string]any{}
	for _, row := range rows {
		if slashRowUID(row) == uid {
			filtered = append(filtered, row)
		}
	}
	return cloneSlashFilterRows(filtered)
}

func slashUIDValues(rows []map[string]any) []any {
	values := make([]any, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		uid := slashRowUID(row)
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		values = append(values, parseUID(uid))
	}
	return values
}

func (d *Discord) deleteSlashTrackingRows(table string, rows []map[string]any) (int64, error) {
	if d == nil || d.manager == nil || d.manager.query == nil || table == "" || len(rows) == 0 {
		return 0, nil
	}
	scope := map[string]any{}
	if id := strings.TrimSpace(fmt.Sprintf("%v", rows[0]["id"])); id != "" {
		scope["id"] = id
	}
	profiles := map[int]bool{}
	profileOrder := []int{}
	for _, row := range rows {
		profileNo := toInt(row["profile_no"], 0)
		if profiles[profileNo] {
			continue
		}
		profiles[profileNo] = true
		profileOrder = append(profileOrder, profileNo)
	}
	uidsByProfile := map[int][]any{}
	for _, row := range rows {
		profileNo := toInt(row["profile_no"], 0)
		uidsByProfile[profileNo] = append(uidsByProfile[profileNo], parseUID(slashRowUID(row)))
	}
	var removed int64
	err := d.manager.query.WithTx(context.Background(), func(tx *db.Query) error {
		for _, profileNo := range profileOrder {
			deleteScope := map[string]any{}
			for key, value := range scope {
				deleteScope[key] = value
			}
			if profileNo > 0 {
				deleteScope["profile_no"] = profileNo
			}
			affected, err := tx.DeleteWhereInQuery(table, deleteScope, uidsByProfile[profileNo], "uid")
			if err != nil {
				return err
			}
			removed += affected
		}
		tx.AfterCommit(func() {
			if d.manager != nil && d.manager.processor != nil {
				d.manager.processor.RefreshAlertCacheAsync()
			}
		})
		return nil
	})
	return removed, err
}

func (d *Discord) restoreSlashTrackingRows(table string, rows []map[string]any) (int64, error) {
	if d == nil || d.manager == nil || d.manager.query == nil || table == "" || len(rows) == 0 {
		return 0, nil
	}
	var inserted int64
	err := d.manager.query.WithTx(context.Background(), func(tx *db.Query) error {
		affected, err := tx.InsertQuery(table, cloneSlashFilterRows(rows))
		if err != nil {
			return err
		}
		inserted = affected
		tx.AfterCommit(func() {
			if d.manager != nil && d.manager.processor != nil {
				d.manager.processor.RefreshAlertCacheAsync()
			}
		})
		return nil
	})
	return inserted, err
}

func (d *Discord) slashFilterRowText(tr *i18n.Translator, trackingType string, row map[string]any) string {
	if d == nil || d.manager == nil {
		return ""
	}
	if d.manager.data == nil {
		if uid := slashRowUID(row); uid != "" {
			if tr != nil {
				return translateFormatOrDefault(tr, "Filter UID {0}", uid)
			}
			return fmt.Sprintf("Filter UID %s", uid)
		}
		return humanizeOptionName(trackingType)
	}
	switch trackingType {
	case "pokemon":
		return tracking.MonsterRowText(d.manager.cfg, tr, d.manager.data, row)
	case "raid":
		return tracking.RaidRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner)
	case "egg":
		return tracking.EggRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner)
	case "maxbattle":
		return tracking.MaxbattleRowText(d.manager.cfg, tr, d.manager.data, row)
	case "quest":
		return tracking.QuestRowText(d.manager.cfg, tr, d.manager.data, row)
	case "rocket", "pokestop-event":
		return tracking.InvasionRowText(d.manager.cfg, tr, d.manager.data, row)
	case "lure":
		return tracking.LureRowText(d.manager.cfg, tr, d.manager.data, row)
	case "weather":
		return tracking.WeatherRowText(tr, d.manager.data, row)
	case "gym":
		return tracking.GymRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner)
	case "nest":
		return tracking.NestRowText(d.manager.cfg, tr, d.manager.data, row)
	case "fort":
		return tracking.FortUpdateRowText(d.manager.cfg, tr, d.manager.data, row)
	default:
		return ""
	}
}

func (d *Discord) slashFilterSummary(tr *i18n.Translator, trackingType string, rows []map[string]any) string {
	lines := []string{}
	for idx, row := range rows {
		if idx >= filterSummaryRowLimit {
			lines = append(lines, translateFormatOrDefault(tr, "And {0} more...", len(rows)-idx))
			break
		}
		label := d.slashFilterRowText(tr, trackingType, row)
		if label == "" {
			continue
		}
		lines = append(lines, "- "+label)
	}
	return strings.Join(lines, "\n")
}

func slashCardDetailLine(label, value string) string {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return ""
	}
	return fmt.Sprintf("- %s: **%s**", label, value)
}

func slashCardHeading(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, ":")
	text = strings.TrimRight(text, "：")
	return strings.TrimSpace(text)
}

func slashCardDescription(headline, intro string, detailLines []string, summary string) string {
	sections := []string{}
	if intro = strings.TrimSpace(intro); intro != "" {
		sections = append(sections, intro)
	}
	if headline = strings.TrimSpace(headline); headline != "" {
		sections = append(sections, "## "+headline)
	}
	lines := []string{}
	for _, line := range detailLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		sections = append(sections, strings.Join(lines, "\n"))
	}
	if summary = strings.TrimSpace(summary); summary != "" {
		sections = append(sections, summary)
	}
	return strings.Join(sections, "\n\n")
}

func slashFilterCardColor(action string) int {
	switch action {
	case "added":
		return slashFilterCardColorAdded
	case "removed":
		return slashFilterCardColorRemoved
	case "restored":
		return slashFilterCardColorRestored
	default:
		return slashFilterCardColorConfirm
	}
}

func (d *Discord) slashFilterCardHeadline(tr *i18n.Translator, trackingType string, rows []map[string]any) string {
	if len(rows) == 1 {
		if label := strings.TrimSpace(d.slashFilterRowText(tr, trackingType, rows[0])); label != "" {
			return label
		}
	}
	if len(rows) > 1 {
		return fmt.Sprintf("%d %s", len(rows), strings.ToLower(translateOrDefault(tr, "Filters")))
	}
	return slashTrackingTitle(tr, trackingType)
}

func (d *Discord) slashFilterMutationEmbed(i *discordgo.InteractionCreate, action, trackingType string, rows []map[string]any, profileLabel string) *discordgo.MessageEmbed {
	tr := d.slashInteractionTranslator(i)
	headline := d.slashFilterTypedHeading(tr, trackingType, rows)
	detailLines := []string{}
	if profileLabel != "" {
		detailLines = append(detailLines, slashCardDetailLine(translateOrDefault(tr, "Profile"), profileLabel))
	}
	if len(rows) == 1 {
		detailLines = append(detailLines, slashFilterNonDefaultDetailLines(tr, trackingType, rows[0])...)
	} else if len(rows) > 1 {
		for idx, row := range rows {
			if idx >= filterSummaryRowLimit {
				detailLines = append(detailLines, fmt.Sprintf("*%s*", translateFormatOrDefault(tr, "And {0} more...", len(rows)-idx)))
				break
			}
			label := d.slashFilterRowText(tr, trackingType, row)
			if label != "" {
				detailLines = append(detailLines, "- "+label)
			}
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:       translateOrDefault(tr, slashFilterMutationTitleKey(action, len(rows))),
		Description: slashCardDescription(headline, "", detailLines, ""),
		Color:       slashFilterCardColor(action),
	}
	if iconURL := d.slashFilterIconURL(trackingType, rows); iconURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: iconURL}
	}
	return embed
}

func (d *Discord) slashFilterMutationComponents(i *discordgo.InteractionCreate, trackingType, customID string, count int, restoring bool) []discordgo.MessageComponent {
	tr := d.slashInteractionTranslator(i)
	label := translateOrDefault(tr, "Remove Filter")
	style := discordgo.DangerButton
	if count > 1 {
		label = translateOrDefault(tr, "Remove Filters")
	}
	if restoring {
		label = translateOrDefault(tr, "Restore Filter")
		style = discordgo.SuccessButton
		if count > 1 {
			label = translateOrDefault(tr, "Restore Filters")
		}
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: customID, Label: label, Style: style},
		}},
	}
}

func slashFilterMutationTitleKey(action string, count int) string {
	if count == 1 {
		switch action {
		case "added":
			return "Filter Added"
		case "removed":
			return "Filter Removed"
		case "restored":
			return "Filter Restored"
		}
	}
	switch action {
	case "added":
		return "Filters Added"
	case "removed":
		return "Filters Removed"
	case "restored":
		return "Filters Restored"
	default:
		return "Confirm Command:"
	}
}

func slashFilterMutationProfileKey(action string) string {
	switch action {
	case "added":
		return "Saved to {0}."
	case "removed":
		return "Removed from {0}."
	case "restored":
		return "Restored in {0}."
	default:
		return ""
	}
}

func (d *Discord) slashFilterMutationResponse(i *discordgo.InteractionCreate, action, trackingType, table, profileLabel, userID string, rows []map[string]any) ([]*discordgo.MessageEmbed, []discordgo.MessageComponent, bool) {
	if len(rows) == 0 {
		return nil, nil, false
	}
	customID := ""
	switch action {
	case "added", "restored":
		customID = slashFilterRemoveButtonPrefix + d.storeSlashFilterAction(&slashFilterActionState{
			UserID:       userID,
			Table:        table,
			TrackingType: trackingType,
			Rows:         rows,
			ProfileLabel: profileLabel,
			ExpiresAt:    time.Now().Add(filterActionExpiry),
		})
	case "removed":
		customID = slashFilterRestoreButtonPrefix + d.storeSlashFilterAction(&slashFilterActionState{
			UserID:       userID,
			Table:        table,
			TrackingType: trackingType,
			Rows:         rows,
			ProfileLabel: profileLabel,
			ExpiresAt:    time.Now().Add(filterActionExpiry),
		})
	}
	if customID == "" {
		return nil, nil, false
	}
	embed := d.slashFilterMutationEmbed(i, action, trackingType, rows, profileLabel)
	return []*discordgo.MessageEmbed{embed}, d.slashFilterMutationComponents(i, trackingType, customID, len(rows), action == "removed"), true
}

func (d *Discord) handleSlashFilterRemoveAction(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	d.handleSlashFilterMutationAction(s, i, id, "removed", d.deleteSlashTrackingRows)
}

func (d *Discord) handleSlashFilterRestoreAction(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	d.handleSlashFilterMutationAction(s, i, id, "restored", d.restoreSlashTrackingRows)
}

func (d *Discord) handleSlashFilterMutationAction(s *discordgo.Session, i *discordgo.InteractionCreate, id, actionLabel string, operationFn func(string, []map[string]any) (int64, error)) {
	userID, _ := slashUser(i)
	action, errKey := d.slashFilterAction(id, userID)
	if errKey != "" {
		d.respondEphemeralError(s, i, d.slashText(i, errKey))
		return
	}
	affected, err := operationFn(action.Table, action.Rows)
	if err != nil {
		d.respondEphemeralError(s, i, err.Error())
		return
	}
	if affected == 0 {
		d.respondEphemeralError(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	embeds, components, ok := d.slashFilterMutationResponse(i, actionLabel, action.TrackingType, action.Table, action.ProfileLabel, userID, action.Rows)
	if !ok {
		d.respondEphemeralError(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", embeds, components)
}
