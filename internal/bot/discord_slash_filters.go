package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/db"
	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
)

const (
	slashFilterRemoveButtonPrefix  = "poracle:filter:remove:"
	slashFilterRestoreButtonPrefix = "poracle:filter:restore:"
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
		return ""
	}
}

func slashTrackingTitle(tr *i18n.Translator, trackingType string) string {
	switch trackingType {
	case "pokemon":
		return translateOrDefault(tr, "Pokemon")
	case "rocket":
		return translateOrDefault(tr, "Rocket")
	case "pokestop-event":
		return translateOrDefault(tr, "Pokestop Event")
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
				return tr.TranslateFormat("Filter UID {0}", uid)
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
		if idx >= 5 {
			lines = append(lines, tr.TranslateFormat("And {0} more...", len(rows)-idx))
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

func (d *Discord) slashFilterMutationEmbed(i *discordgo.InteractionCreate, titleKey, profileKey string, trackingType string, rows []map[string]any, profileLabel string) *discordgo.MessageEmbed {
	tr := d.slashInteractionTranslator(i)
	fieldName := tr.Translate("Filters", false)
	if len(rows) == 1 {
		fieldName = tr.Translate("Filter", false)
	}
	description := ""
	if profileLabel != "" && profileKey != "" {
		description = tr.TranslateFormat(profileKey, profileLabel)
	}
	return &discordgo.MessageEmbed{
		Title:       tr.Translate(titleKey, false),
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   fieldName,
				Value:  d.slashFilterSummary(tr, trackingType, rows),
				Inline: false,
			},
			{
				Name:   tr.Translate("Type", false),
				Value:  slashTrackingTitle(tr, trackingType),
				Inline: true,
			},
			{
				Name:   tr.Translate("Profile", false),
				Value:  profileLabel,
				Inline: true,
			},
		},
	}
}

func (d *Discord) slashFilterMutationComponents(i *discordgo.InteractionCreate, trackingType, customID string, count int, restoring bool) []discordgo.MessageComponent {
	tr := d.slashInteractionTranslator(i)
	label := tr.Translate("Remove Filter", false)
	style := discordgo.DangerButton
	if count > 1 {
		label = tr.Translate("Remove Filters", false)
	}
	if restoring {
		label = tr.Translate("Restore Filter", false)
		style = discordgo.SuccessButton
		if count > 1 {
			label = tr.Translate("Restore Filters", false)
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
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		})
	case "removed":
		customID = slashFilterRestoreButtonPrefix + d.storeSlashFilterAction(&slashFilterActionState{
			UserID:       userID,
			Table:        table,
			TrackingType: trackingType,
			Rows:         rows,
			ProfileLabel: profileLabel,
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		})
	}
	if customID == "" {
		return nil, nil, false
	}
	embed := d.slashFilterMutationEmbed(i, slashFilterMutationTitleKey(action, len(rows)), slashFilterMutationProfileKey(action), trackingType, rows, profileLabel)
	return []*discordgo.MessageEmbed{embed}, d.slashFilterMutationComponents(i, trackingType, customID, len(rows), action == "removed"), true
}

func (d *Discord) handleSlashFilterRemoveAction(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	userID, _ := slashUser(i)
	action, errKey := d.slashFilterAction(id, userID)
	if errKey != "" {
		d.respondEphemeral(s, i, d.slashText(i, errKey))
		return
	}
	removed, err := d.deleteSlashTrackingRows(action.Table, action.Rows)
	if err != nil {
		d.respondEphemeral(s, i, err.Error())
		return
	}
	if removed == 0 {
		d.respondEphemeral(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	embeds, components, ok := d.slashFilterMutationResponse(i, "removed", action.TrackingType, action.Table, action.ProfileLabel, userID, action.Rows)
	if !ok {
		d.respondEphemeral(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", embeds, components)
}

func (d *Discord) handleSlashFilterRestoreAction(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	userID, _ := slashUser(i)
	action, errKey := d.slashFilterAction(id, userID)
	if errKey != "" {
		d.respondEphemeral(s, i, d.slashText(i, errKey))
		return
	}
	restored, err := d.restoreSlashTrackingRows(action.Table, action.Rows)
	if err != nil {
		d.respondEphemeral(s, i, err.Error())
		return
	}
	if restored == 0 {
		d.respondEphemeral(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	embeds, components, ok := d.slashFilterMutationResponse(i, "restored", action.TrackingType, action.Table, action.ProfileLabel, userID, action.Rows)
	if !ok {
		d.respondEphemeral(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", embeds, components)
}
