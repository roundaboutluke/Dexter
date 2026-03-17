package bot

import (
	"fmt"
	"sort"
	"strings"

	"poraclego/internal/db"
	"poraclego/internal/i18n"
)

func (d *Discord) persistSlashHumanUpdate(userID string, update map[string]any) error {
	if d == nil || d.manager == nil {
		return nil
	}
	return d.manager.withAlertStateTx(func(query *db.Query) error {
		_, err := query.UpdateQuery("humans", update, map[string]any{"id": userID})
		return err
	})
}

func (d *Discord) persistSlashScheduleUpdates(userID string, updates map[int][]scheduleEntry) error {
	if d == nil || d.manager == nil || len(updates) == 0 {
		return nil
	}
	profileNos := make([]int, 0, len(updates))
	for profileNo := range updates {
		if profileNo > 0 {
			profileNos = append(profileNos, profileNo)
		}
	}
	sort.Ints(profileNos)
	return d.manager.withAlertStateTx(func(query *db.Query) error {
		for _, profileNo := range profileNos {
			if _, err := query.UpdateQuery("profiles", map[string]any{
				"active_hours": encodeScheduleEntries(updates[profileNo]),
			}, map[string]any{
				"id":         userID,
				"profile_no": profileNo,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func buildScheduleEditAssignUpdates(profiles []map[string]any, selected map[string]any, original scheduleEntry, day, startMin, endMin int) (map[int][]scheduleEntry, string) {
	return buildScheduleEditAssignUpdatesLocalized(nil, profiles, selected, original, day, startMin, endMin)
}

func buildScheduleEditAssignUpdatesLocalized(tr *i18n.Translator, profiles []map[string]any, selected map[string]any, original scheduleEntry, day, startMin, endMin int) (map[int][]scheduleEntry, string) {
	if selected == nil {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	selectedNo := toInt(selected["profile_no"], 0)
	if selectedNo == 0 {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	if conflicts := scheduleConflictsLocalized(tr, profiles, day, startMin, endMin, original.ProfileNo, original); len(conflicts) > 0 {
		if tr != nil {
			return nil, tr.TranslateFormat("That overlaps with existing schedules: {0}", strings.Join(conflicts, ", "))
		}
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(conflicts, ", "))
	}

	newEntry := scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin}
	updates := map[int][]scheduleEntry{}

	if original.ProfileNo != 0 && original.ProfileNo == selectedNo {
		entries := removeScheduleEntry(scheduleEntriesFromRaw(selected["active_hours"]), scheduleEntryValue(original))
		entries = append(entries, newEntry)
		sortScheduleEntries(entries)
		updates[selectedNo] = entries
		return updates, ""
	}

	targetEntries := scheduleEntriesFromRaw(selected["active_hours"])
	targetEntries = append(targetEntries, newEntry)
	sortScheduleEntries(targetEntries)
	updates[selectedNo] = targetEntries

	if original.ProfileNo != 0 {
		if old := profileRowByNo(profiles, original.ProfileNo); old != nil {
			sourceEntries := removeScheduleEntry(scheduleEntriesFromRaw(old["active_hours"]), scheduleEntryValue(original))
			updates[original.ProfileNo] = sourceEntries
		}
	}

	return updates, ""
}

func sortScheduleEntries(entries []scheduleEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
}
