package tracking

import (
	"fmt"
	"strings"

	"dexter/internal/i18n"
)

// RuleScope identifies the tracked user/profile pair for a normalized rule.
type RuleScope struct {
	UserID    string
	ProfileNo int
}

// UpsertPlan captures the outcome of comparing incoming rule rows with the
// currently persisted rows for the same tracking category.
type UpsertPlan struct {
	Unchanged []map[string]any
	Updates   []map[string]any
	Inserts   []map[string]any
}

// Total returns the number of rows represented by the plan.
func (p UpsertPlan) Total() int {
	return len(p.Unchanged) + len(p.Updates) + len(p.Inserts)
}

// CloneRow makes a shallow copy of a DB row map.
func CloneRow(row map[string]any) map[string]any {
	if row == nil {
		return nil
	}
	cloned := make(map[string]any, len(row))
	for key, value := range row {
		cloned[key] = value
	}
	return cloned
}

// DiffKeys compares the candidate keys against an existing row and returns the
// changed keys, ignoring uid.
func DiffKeys(candidate, existing map[string]any) []string {
	keys := []string{}
	for key, value := range candidate {
		if key == "uid" {
			continue
		}
		if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", existing[key]) {
			keys = append(keys, key)
		}
	}
	return keys
}

// DesiredDiffKeys mirrors the command-side comparison semantics where the
// existing row uid is treated as an ignorable difference.
func DesiredDiffKeys(existing, desired map[string]any) []string {
	diffs := []string{}
	if _, ok := existing["uid"]; ok {
		diffs = append(diffs, "uid")
	}
	for key, desiredValue := range desired {
		if key == "uid" {
			continue
		}
		if fmt.Sprintf("%v", existing[key]) != fmt.Sprintf("%v", desiredValue) {
			diffs = append(diffs, key)
		}
	}
	return diffs
}

// IsSingleMutableFieldUpdate reports whether the diff set only changes uid plus
// one allowed mutable field.
func IsSingleMutableFieldUpdate(diffs []string, allowed ...string) bool {
	if len(diffs) != 2 {
		return false
	}
	if diffs[0] != "uid" && diffs[1] != "uid" {
		return false
	}
	other := diffs[0]
	if other == "uid" {
		other = diffs[1]
	}
	for _, allowedKey := range allowed {
		if other == allowedKey {
			return true
		}
	}
	return false
}

// HasOnlyAllowedDiffs reports whether every diff key is in the allowlist.
func HasOnlyAllowedDiffs(diffs []string, allowed ...string) bool {
	if len(diffs) == 0 {
		return false
	}
	allowedSet := map[string]struct{}{}
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for _, diff := range diffs {
		if _, ok := allowedSet[diff]; !ok {
			return false
		}
	}
	return true
}

// PlanUpsert compares desired rows to existing rows and classifies them as
// unchanged, updates, or inserts.
func PlanUpsert(
	desired []map[string]any,
	existing []map[string]any,
	sameIdentity func(candidate, existing map[string]any) bool,
	mutableFields ...string,
) UpsertPlan {
	plan := UpsertPlan{
		Unchanged: make([]map[string]any, 0),
		Updates:   make([]map[string]any, 0),
		Inserts:   append([]map[string]any{}, desired...),
	}
	for i := len(plan.Inserts) - 1; i >= 0; i-- {
		candidate := plan.Inserts[i]
		var updateSource map[string]any
		for _, row := range existing {
			if !sameIdentity(candidate, row) {
				continue
			}
			diffKeys := DiffKeys(candidate, row)
			switch {
			case len(diffKeys) == 0:
				plan.Unchanged = append(plan.Unchanged, candidate)
				plan.Inserts = append(plan.Inserts[:i], plan.Inserts[i+1:]...)
				updateSource = nil
				goto nextCandidate
			case updateSource == nil && HasOnlyAllowedDiffs(diffKeys, mutableFields...):
				updateSource = row
			}
		}
		if updateSource != nil {
			updated := CloneRow(candidate)
			updated["uid"] = updateSource["uid"]
			plan.Updates = append(plan.Updates, updated)
			plan.Inserts = append(plan.Inserts[:i], plan.Inserts[i+1:]...)
		}
	nextCandidate:
	}
	return plan
}

// ChangeMessage renders a human-readable summary of a tracking upsert plan.
func ChangeMessage(
	tr *i18n.Translator,
	prefix string,
	trackedLabel string,
	plan UpsertPlan,
	rowText func(map[string]any) string,
) string {
	translate := func(text string) string {
		if tr != nil {
			return tr.Translate(text, false)
		}
		return text
	}
	if trackedLabel == "" {
		trackedLabel = "tracked"
	}
	if plan.Total() > 50 {
		if tr != nil {
			return tr.TranslateFormat("I have made a lot of changes. See {0}{1} for details", prefix, trackedLabel)
		}
		return fmt.Sprintf("I have made a lot of changes. See %s%s for details", prefix, trackedLabel)
	}
	lines := make([]string, 0, plan.Total())
	appendSection := func(label string, rows []map[string]any) {
		for _, row := range rows {
			lines = append(lines, translate(label)+rowText(row))
		}
	}
	appendSection("Unchanged: ", plan.Unchanged)
	appendSection("Updated: ", plan.Updates)
	appendSection("New: ", plan.Inserts)
	return strings.Join(lines, "\n")
}

func defaultTemplate(cfgValue string) string {
	if strings.TrimSpace(cfgValue) == "" {
		return "1"
	}
	return cfgValue
}
