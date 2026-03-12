package profile

import "time"

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
					update := map[string]any{"current_profile_no": 0}
					if current > 0 {
						update["preferred_profile_no"] = current
					}
					if _, err := s.query.UpdateQuery("humans", update, map[string]any{"id": id}); err != nil {
						s.logf("profile schedule: clear failed for %s: %v", id, err)
					} else {
						s.refreshAlertState()
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
			s.refreshAlertState()
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
			s.refreshAlertState()
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
