package command

import (
	"fmt"
	"regexp"
	"strings"

	"dexter/internal/util"
)

type Target struct {
	ID   string
	Name string
	Type string
}

type humanContext struct {
	human          map[string]any
	currentProfile int
	language       string
}

func loadHumanContext(ctx *Context) (*humanContext, error) {
	row, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": ctx.UserID})
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	profile := toInt(row["current_profile_no"], 1)
	lang := ctx.Language
	if l, ok := row["language"].(string); ok && l != "" {
		lang = l
	}
	return &humanContext{human: row, currentProfile: profile, language: lang}, nil
}

func resolveTarget(ctx *Context, args []string, re *RegexSet) (Target, []string) {
	t := Target{ID: ctx.UserID, Name: ctx.UserName, Type: ctx.Platform + ":user"}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case re.Name.MatchString(arg):
			match := re.Name.FindStringSubmatch(arg)
			if len(match) > 2 {
				t.Name = match[2]
				t.Type = "webhook"
				t.ID = match[2]
			}
		case re.Channel.MatchString(arg):
			match := re.Channel.FindStringSubmatch(arg)
			if len(match) > 2 {
				t.Type = ctx.Platform + ":channel"
				t.ID = match[2]
				t.Name = match[2]
			}
		case re.User.MatchString(arg):
			match := re.User.FindStringSubmatch(arg)
			if len(match) > 2 {
				t.Type = ctx.Platform + ":user"
				t.ID = match[2]
				t.Name = match[2]
			}
		default:
			out = append(out, arg)
		}
	}
	return t, out
}

func parseDistance(args []string, re *RegexSet) (int, []string) {
	distance := 0
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if re.Distance.MatchString(arg) {
			match := re.Distance.FindStringSubmatch(arg)
			if len(match) > 2 {
				distance = toInt(match[2], 0)
			}
			continue
		}
		out = append(out, arg)
	}
	return distance, out
}

func parseTemplate(args []string, re *RegexSet) (string, []string) {
	template := ""
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if re.Template.MatchString(arg) {
			match := re.Template.FindStringSubmatch(arg)
			if len(match) > 2 {
				template = match[2]
			}
			continue
		}
		out = append(out, arg)
	}
	return template, out
}

func parseMinSpawn(args []string, re *RegexSet) (int, []string) {
	minSpawn := 0
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if re.MinSpawn.MatchString(arg) {
			match := re.MinSpawn.FindStringSubmatch(arg)
			if len(match) > 2 {
				minSpawn = toInt(match[2], 0)
			}
			continue
		}
		out = append(out, arg)
	}
	return minSpawn, out
}

func parseClean(args []string) (bool, []string) {
	clean := false
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.EqualFold(arg, "clean") {
			clean = true
			continue
		}
		out = append(out, arg)
	}
	return clean, out
}

func parseRange(arg string, re *regexp.Regexp) (int, int, bool) {
	if !re.MatchString(arg) {
		return 0, 0, false
	}
	match := re.FindStringSubmatch(arg)
	if len(match) < 3 {
		return 0, 0, false
	}
	values := strings.Split(match[2], "-")
	if len(values) == 1 {
		val := toInt(values[0], 0)
		return val, val, true
	}
	return toInt(values[0], 0), toInt(values[1], 0), true
}

func containsPhrase(args []string, phrase string) bool {
	joined := strings.ToLower(strings.Join(args, " "))
	return strings.Contains(joined, strings.ToLower(phrase))
}

func lookupMonsterIDs(ctx *Context, tokens []string) []int {
	ids := []int{}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if num := toInt(token, -1); num > 0 {
			ids = append(ids, num)
			continue
		}
		for _, raw := range ctx.Data.Monsters {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := fmt.Sprintf("%v", m["name"])
			if strings.EqualFold(name, token) {
				ids = append(ids, toInt(m["id"], 0))
			}
		}
	}
	return ids
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

var toInt = util.ToInt
