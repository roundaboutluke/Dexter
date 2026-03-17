package bot

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
)

func normalizeRaidType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "level") {
		return "level" + strings.TrimPrefix(lower, "level")
	}
	if _, err := strconv.Atoi(lower); err == nil {
		return "level" + lower
	}
	return trimmed
}

func optionalInt(options []*discordgo.ApplicationCommandInteractionDataOption, name string) *int {
	if value, ok := optionInt(options, name); ok {
		return &value
	}
	return nil
}

func parseRemoveSelection(optionType, value string) (string, string) {
	if strings.Contains(value, "|") {
		parts := strings.SplitN(value, "|", 2)
		return strings.ToLower(parts[0]), parts[1]
	}
	return strings.ToLower(optionType), value
}

func removeTrackingTable(trackingType string) string {
	switch strings.ToLower(trackingType) {
	case "pokemon":
		return "monsters"
	case "raid":
		return "raid"
	case "egg":
		return "egg"
	case "gym":
		return "gym"
	case "maxbattle":
		return "maxbattle"
	case "incident", "invasion", "rocket", "pokestop-event":
		return "invasion"
	case "quest":
		return "quest"
	case "weather":
		return "weather"
	case "lure":
		return "lures"
	case "nest":
		return "nests"
	case "fort":
		return "forts"
	default:
		return ""
	}
}

const (
	slashProfileScopeEffective = "effective"
	slashProfileScopeSpecific  = "specific"
	slashProfileScopeAll       = "all"
)

type slashProfileSelection struct {
	UserID      string
	Human       map[string]any
	Profiles    []map[string]any
	EffectiveNo int
	Mode        string
	ProfileNo   int
}

func (s slashProfileSelection) ProfileRow() map[string]any {
	if s.Mode == slashProfileScopeAll {
		return nil
	}
	return profileRowByNo(s.Profiles, s.ProfileNo)
}

func (s slashProfileSelection) TargetLabel() string {
	if s.Mode == slashProfileScopeAll {
		return "all profiles"
	}
	if row := s.ProfileRow(); row != nil {
		return profileDisplayName(row)
	}
	if s.ProfileNo > 0 {
		return fmt.Sprintf("Profile %d", s.ProfileNo)
	}
	return "your current profile"
}

func (s slashProfileSelection) TargetLabelLocalized(tr *i18n.Translator) string {
	if s.Mode == slashProfileScopeAll {
		return translateOrDefault(tr, "All profiles")
	}
	if row := s.ProfileRow(); row != nil {
		return localizedProfileDisplayName(tr, row)
	}
	if s.ProfileNo > 0 {
		return localizedProfileLabel(tr, s.ProfileNo)
	}
	return translateOrDefault(tr, "your current profile")
}

func (s slashProfileSelection) LogValue() string {
	if s.Mode == slashProfileScopeAll {
		return "all"
	}
	if s.ProfileNo > 0 {
		return fmt.Sprintf("p%d", s.ProfileNo)
	}
	return s.Mode
}

func parseUID(value string) any {
	if value == "" {
		return value
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return value
}

func effectiveProfileNoFromHuman(human map[string]any) int {
	if human == nil {
		return 1
	}
	current := toInt(human["current_profile_no"], 1)
	if current > 0 {
		return current
	}
	if preferred := toInt(human["preferred_profile_no"], 0); preferred > 0 {
		return preferred
	}
	return 1
}

func (d *Discord) loadSlashProfileContext(i *discordgo.InteractionCreate) (string, map[string]any, []map[string]any, string) {
	if d == nil || d.manager == nil || d.manager.query == nil {
		return "", nil, nil, d.slashText(i, "Target is not registered.")
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return "", nil, nil, d.slashText(i, "Target is not registered.")
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		return "", nil, nil, d.slashText(i, "Target is not registered.")
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return "", nil, nil, d.slashText(i, "Unable to load profiles.")
	}
	sort.Slice(profiles, func(i, j int) bool {
		return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0)
	})
	return userID, human, profiles, ""
}

func (d *Discord) resolveSlashProfileSelection(i *discordgo.InteractionCreate, token string) (slashProfileSelection, string) {
	userID, human, profiles, errText := d.loadSlashProfileContext(i)
	if errText != "" {
		return slashProfileSelection{}, errText
	}
	effectiveNo := effectiveProfileNoFromHuman(human)
	token = strings.TrimSpace(token)
	if token == "" || strings.EqualFold(token, "effective") {
		return slashProfileSelection{
			UserID:      userID,
			Human:       human,
			Profiles:    profiles,
			EffectiveNo: effectiveNo,
			Mode:        slashProfileScopeEffective,
			ProfileNo:   effectiveNo,
		}, ""
	}
	if strings.EqualFold(token, "all") {
		return slashProfileSelection{
			UserID:      userID,
			Human:       human,
			Profiles:    profiles,
			EffectiveNo: effectiveNo,
			Mode:        slashProfileScopeAll,
		}, ""
	}
	row := profileRowByToken(profiles, token)
	if row == nil {
		return slashProfileSelection{}, d.slashText(i, "Profile not found.")
	}
	return slashProfileSelection{
		UserID:      userID,
		Human:       human,
		Profiles:    profiles,
		EffectiveNo: effectiveNo,
		Mode:        slashProfileScopeSpecific,
		ProfileNo:   toInt(row["profile_no"], 0),
	}, ""
}

func (d *Discord) userProfileNo(userID string) int {
	if d.manager == nil || d.manager.query == nil || userID == "" {
		return 1
	}
	row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || row == nil {
		return 1
	}
	current := toInt(row["current_profile_no"], 1)
	if current > 0 {
		return current
	}
	if preferred := toInt(row["preferred_profile_no"], 0); preferred > 0 {
		return preferred
	}
	return 1
}

func (d *Discord) userLanguage(userID string) string {
	if d.manager == nil || d.manager.query == nil || userID == "" {
		return d.configuredDefaultLocale()
	}
	row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err == nil && row != nil {
		if lang, ok := row["language"].(string); ok && lang != "" {
			return lang
		}
	}
	return d.configuredDefaultLocale()
}

func floatPtr(value float64) *float64 {
	return &value
}

func toFloat(value any) float64 {
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
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return parsed
		}
	}
	return 0
}

var latLonStringRe = regexp.MustCompile(`^([-+]?(?:[1-8]?\d(?:\.\d+)?|90(?:\.0+)?)),\s*([-+]?(?:180(\.0+)?|(?:(?:1[0-7]\d)|(?:[1-9]?\d))(?:\.\d+)?))$`)

func parseLatLonString(value string) (float64, float64, bool) {
	match := latLonStringRe.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) < 3 {
		return 0, 0, false
	}
	lat, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, 0, false
	}
	lon, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

func formatFloat(value float64) string {
	if value == 0 {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.7f", value), "0"), ".")
}

func parseAreaListFromHuman(human map[string]any) []string {
	return tracking.ParseAreaList(human)
}

func fenceCentroid(fence geofence.Fence) (float64, float64, bool) {
	path := fence.Path
	if len(path) == 0 && len(fence.MultiPath) > 0 {
		longest := fence.MultiPath[0]
		for _, candidate := range fence.MultiPath[1:] {
			if len(candidate) > len(longest) {
				longest = candidate
			}
		}
		path = longest
	}
	if len(path) == 0 {
		return 0, 0, false
	}
	var sumLat, sumLon float64
	for _, point := range path {
		if len(point) < 2 {
			continue
		}
		sumLat += point[0]
		sumLon += point[1]
	}
	count := float64(len(path))
	if count == 0 {
		return 0, 0, false
	}
	return sumLat / count, sumLon / count, true
}

func selectableAreaNames(fences []geofence.Fence) []string {
	names := []string{}
	for _, fence := range fences {
		selectable := true
		if fence.UserSelectable != nil {
			selectable = *fence.UserSelectable
		}
		if !selectable {
			continue
		}
		name := strings.TrimSpace(fence.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}
