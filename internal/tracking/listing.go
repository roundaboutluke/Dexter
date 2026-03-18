package tracking

import (
	"encoding/json"
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/geofence"
	"dexter/internal/i18n"
	"dexter/internal/scanner"
)

// RowSource is the minimal query surface needed to build tracked summaries.
type RowSource interface {
	SelectAllQuery(table string, where map[string]any) ([]map[string]any, error)
	CountGroupedQuery(table string, conditions map[string]any, groupBy string) (map[int]int64, error)
}

// ListingContext carries the shared dependencies needed to render tracked
// summaries across commands and bot entrypoints.
type ListingContext struct {
	Config   *config.Config
	Query    RowSource
	Data     *data.GameData
	GymNames scanner.GymNameResolver
}

// CountSpec describes one tracked category for summary counts.
type CountSpec struct {
	Key         string
	Singular    string
	Plural      string
	Table       string
	DisabledKey string
	BlockedKeys []string
}

type listingSectionSpec struct {
	Table       string
	DisabledKey string
	BlockedText string
	EmptyText   string
	Heading     string
	BlockedKeys []string
	RowText     func(ListingContext, *i18n.Translator, map[string]any) string
}

// ParseAreaList returns the selected area list stored on a human/profile row.
func ParseAreaList(human map[string]any) []string {
	areas := []string{}
	if raw, ok := human["area"].(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &areas)
	}
	return areas
}

// BlockedAlerts returns the blocked alert list stored on a human row.
func BlockedAlerts(human map[string]any) []string {
	blocked := []string{}
	if raw, ok := human["blocked_alerts"].(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &blocked)
	}
	return blocked
}

// AreaText renders the selected areas using loaded fence display names when
// available.
func AreaText(tr *i18n.Translator, fences []geofence.Fence, selected []string) string {
	return areaText(tr, fences, selected, false)
}

// AreaTextWithFallback matches the Discord tracked view behaviour by falling
// back to the raw selected area names when fence display names are unavailable.
func AreaTextWithFallback(tr *i18n.Translator, fences []geofence.Fence, selected []string) string {
	return areaText(tr, fences, selected, true)
}

func areaText(tr *i18n.Translator, fences []geofence.Fence, selected []string, fallbackToRaw bool) string {
	if len(selected) == 0 {
		return translateListingText(tr, "You have not selected any area yet")
	}
	names := []string{}
	for _, fence := range fences {
		name := strings.ToLower(strings.TrimSpace(fence.Name))
		if containsFold(selected, name) {
			names = append(names, fence.Name)
		}
	}
	if len(names) == 0 {
		if !fallbackToRaw {
			return translateListingText(tr, "You have not selected any area yet")
		}
		names = append(names, selected...)
	}
	return fmt.Sprintf("%s %s", translateListingText(tr, "You are currently set to receive alarms in"), strings.Join(names, ", "))
}

// CategoryDetails renders the full tracked-category listing for one
// user/profile pair.
func CategoryDetails(ctx ListingContext, tr *i18n.Translator, userID string, profileNo int, blocked []string) string {
	sections := []string{}

	if section := monsterCategoryDetails(ctx, tr, userID, profileNo, blocked); section != "" {
		sections = append(sections, section)
	}
	if section := raidCategoryDetails(ctx, tr, userID, profileNo, blocked); section != "" {
		sections = append(sections, section)
	}

	for _, spec := range listingSectionSpecs() {
		if section := simpleCategoryDetails(ctx, tr, userID, profileNo, blocked, spec); section != "" {
			sections = append(sections, section)
		}
	}

	if len(sections) == 0 {
		return translateListingText(tr, "No tracking entries found.")
	}
	return strings.Join(sections, "\n\n")
}

// CountSpecs returns the tracked categories included in count summaries.
func CountSpecs() []CountSpec {
	return []CountSpec{
		{Key: "monsters", Singular: "monster", Plural: "monsters", Table: "monsters", DisabledKey: "general.disablePokemon", BlockedKeys: []string{"monster"}},
		{Key: "raids", Singular: "raid", Plural: "raids", Table: "raid", DisabledKey: "general.disableRaid", BlockedKeys: []string{"raid"}},
		{Key: "eggs", Singular: "egg", Plural: "eggs", Table: "egg", DisabledKey: "general.disableRaid", BlockedKeys: []string{"egg"}},
		{Key: "maxbattles", Singular: "max battle", Plural: "max battles", Table: "maxbattle", DisabledKey: "general.disableMaxBattle", BlockedKeys: []string{"maxbattle"}},
		{Key: "quests", Singular: "quest", Plural: "quests", Table: "quest", DisabledKey: "general.disableQuest", BlockedKeys: []string{"quest"}},
		{Key: "invasions", Singular: "invasion", Plural: "invasions", Table: "invasion", DisabledKey: "general.disableInvasion", BlockedKeys: []string{"invasion"}},
		{Key: "lures", Singular: "lure", Plural: "lures", Table: "lures", DisabledKey: "general.disableLure", BlockedKeys: []string{"lure"}},
		{Key: "weather", Singular: "weather alert", Plural: "weather alerts", Table: "weather", DisabledKey: "general.disableWeather", BlockedKeys: []string{"weather"}},
		{Key: "gyms", Singular: "gym alert", Plural: "gym alerts", Table: "gym", DisabledKey: "general.disableGym", BlockedKeys: []string{"gym"}},
		{Key: "nests", Singular: "nest", Plural: "nests", Table: "nests", DisabledKey: "general.disableNest", BlockedKeys: []string{"nest"}},
		{Key: "forts", Singular: "fort change", Plural: "fort changes", Table: "forts", DisabledKey: "general.disableFortUpdate", BlockedKeys: []string{"forts"}},
	}
}

// ProfileCounts returns tracked-category counts grouped by profile number.
func ProfileCounts(ctx ListingContext, userID string, blocked []string) map[int]map[string]int {
	counts := map[int]map[string]int{}
	if ctx.Query == nil || userID == "" {
		return counts
	}
	for _, spec := range CountSpecs() {
		if trackingDisabled(ctx.Config, spec.DisabledKey) || containsFold(blocked, spec.BlockedKeys...) {
			continue
		}
		grouped, err := ctx.Query.CountGroupedQuery(spec.Table, map[string]any{"id": userID}, "profile_no")
		if err != nil {
			continue
		}
		for profileNo, count := range grouped {
			if profileNo <= 0 {
				profileNo = 1
			}
			if counts[profileNo] == nil {
				counts[profileNo] = map[string]int{}
			}
			counts[profileNo][spec.Key] += int(count)
		}
	}
	return counts
}

// PluralCount renders a singular/plural tracking count label.
func PluralCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func monsterCategoryDetails(ctx ListingContext, tr *i18n.Translator, userID string, profileNo int, blocked []string) string {
	if trackingDisabled(ctx.Config, "general.disablePokemon") {
		return ""
	}
	if containsFold(blocked, "monster") {
		return translateListingText(tr, "You do not have permission to track monsters")
	}
	rows, err := selectListingRows(ctx.Query, "monsters", userID, profileNo)
	if err != nil {
		return ""
	}
	if len(rows) == 0 {
		return translateListingText(tr, "You're not tracking any monsters")
	}
	lines := []string{translateListingText(tr, "You're tracking the following monsters:")}
	for _, row := range rows {
		lines = append(lines, MonsterRowText(ctx.Config, tr, ctx.Data, row))
	}
	if containsFold(blocked, "pvp") {
		lines = append(lines, translateListingText(tr, "Your permission level means you will not get results from PVP tracking"))
	}
	return strings.Join(lines, "\n")
}

func raidCategoryDetails(ctx ListingContext, tr *i18n.Translator, userID string, profileNo int, blocked []string) string {
	if trackingDisabled(ctx.Config, "general.disableRaid") {
		return ""
	}
	if containsFold(blocked, "raid") {
		return translateListingText(tr, "You do not have permission to track raids")
	}
	if containsFold(blocked, "egg") {
		return translateListingText(tr, "You do not have permission to track eggs")
	}
	raids, _ := selectListingRows(ctx.Query, "raid", userID, profileNo)
	eggs, _ := selectListingRows(ctx.Query, "egg", userID, profileNo)
	if len(raids) == 0 && len(eggs) == 0 {
		return translateListingText(tr, "You're not tracking any raids")
	}
	lines := []string{translateListingText(tr, "You're tracking the following raids:")}
	for _, row := range raids {
		lines = append(lines, RaidRowText(ctx.Config, tr, ctx.Data, row, ctx.GymNames))
	}
	for _, row := range eggs {
		lines = append(lines, EggRowText(ctx.Config, tr, ctx.Data, row, ctx.GymNames))
	}
	return strings.Join(lines, "\n")
}

func simpleCategoryDetails(ctx ListingContext, tr *i18n.Translator, userID string, profileNo int, blocked []string, spec listingSectionSpec) string {
	if trackingDisabled(ctx.Config, spec.DisabledKey) {
		return ""
	}
	if containsFold(blocked, spec.BlockedKeys...) {
		return spec.BlockedText
	}
	rows, err := selectListingRows(ctx.Query, spec.Table, userID, profileNo)
	if err != nil {
		return ""
	}
	if len(rows) == 0 {
		return spec.EmptyText
	}
	lines := []string{spec.Heading}
	for _, row := range rows {
		lines = append(lines, spec.RowText(ctx, tr, row))
	}
	return strings.Join(lines, "\n")
}

func listingSectionSpecs() []listingSectionSpec {
	return []listingSectionSpec{
		{
			Table:       "maxbattle",
			DisabledKey: "general.disableMaxBattle",
			BlockedText: "You do not have permission to track max battles",
			EmptyText:   "You're not tracking any max battles",
			Heading:     "You're tracking the following max battles:",
			BlockedKeys: []string{"maxbattle"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return MaxbattleRowText(ctx.Config, tr, ctx.Data, row)
			},
		},
		{
			Table:       "quest",
			DisabledKey: "general.disableQuest",
			BlockedText: "You do not have permission to track quests",
			EmptyText:   "You're not tracking any quests",
			Heading:     "You're tracking the following quests:",
			BlockedKeys: []string{"quest"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return QuestRowText(ctx.Config, tr, ctx.Data, row)
			},
		},
		{
			Table:       "invasion",
			DisabledKey: "general.disableInvasion",
			BlockedText: "You do not have permission to track invasions",
			EmptyText:   "You're not tracking any invasions",
			Heading:     "You're tracking the following invasions:",
			BlockedKeys: []string{"invasion"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return InvasionRowText(ctx.Config, tr, ctx.Data, row)
			},
		},
		{
			Table:       "lures",
			DisabledKey: "general.disableLure",
			BlockedText: "You do not have permission to track lures",
			EmptyText:   "You're not tracking any lures",
			Heading:     "You're tracking the following lures:",
			BlockedKeys: []string{"lure"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return LureRowText(ctx.Config, tr, ctx.Data, row)
			},
		},
		{
			Table:       "nests",
			DisabledKey: "general.disableNest",
			BlockedText: "You do not have permission to track nests",
			EmptyText:   "You're not tracking any nests",
			Heading:     "You're tracking the following nests:",
			BlockedKeys: []string{"nest"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return NestRowText(ctx.Config, tr, ctx.Data, row)
			},
		},
		{
			Table:       "gym",
			DisabledKey: "general.disableGym",
			BlockedText: "You do not have permission to track gyms",
			EmptyText:   "You're not tracking any gyms",
			Heading:     "You're tracking the following gyms:",
			BlockedKeys: []string{"gym"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return GymRowText(ctx.Config, tr, ctx.Data, row, ctx.GymNames)
			},
		},
		{
			Table:       "forts",
			DisabledKey: "general.disableFortUpdate",
			BlockedText: "You do not have permission to track fort changes",
			EmptyText:   "You're not tracking any fort changes",
			Heading:     "You're tracking the following fort changes:",
			BlockedKeys: []string{"forts"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return FortUpdateRowText(ctx.Config, tr, ctx.Data, row)
			},
		},
		{
			Table:       "weather",
			DisabledKey: "general.disableWeather",
			BlockedText: "You do not have permission to track weather",
			EmptyText:   "You're not tracking any weather",
			Heading:     "You're tracking the following weather:",
			BlockedKeys: []string{"weather"},
			RowText: func(ctx ListingContext, tr *i18n.Translator, row map[string]any) string {
				return WeatherRowText(tr, ctx.Data, row)
			},
		},
	}
}

func selectListingRows(source RowSource, table, userID string, profileNo int) ([]map[string]any, error) {
	if source == nil {
		return nil, nil
	}
	return source.SelectAllQuery(table, map[string]any{"id": userID, "profile_no": profileNo})
}

func trackingDisabled(cfg *config.Config, key string) bool {
	if cfg == nil {
		return false
	}
	disabled, _ := cfg.GetBool(key)
	return disabled
}

func containsFold(values []string, targets ...string) bool {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		for _, target := range targets {
			if strings.EqualFold(trimmed, strings.TrimSpace(target)) {
				return true
			}
		}
	}
	return false
}

func translateListingText(tr *i18n.Translator, text string) string {
	if tr == nil {
		return text
	}
	return tr.Translate(text, false)
}
