package command

import (
	"fmt"
	"strings"
)

// ApplyCommand runs multiple commands as other targets (admin only).
type ApplyCommand struct{}

func (c *ApplyCommand) Name() string { return "apply" }

func (c *ApplyCommand) Handle(ctx *Context, _ []string) (string, error) {
	tr := ctx.I18n.Translator(ctx.Language)
	if !commandAllowed(ctx, "apply") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	raw := strings.TrimSpace(ctx.RawLine)
	if raw == "" {
		return tr.Translate("No commands provided.", false), nil
	}

	segments := splitSegments(raw)
	if len(segments) < 2 {
		return tr.Translate("No commands provided.", false), nil
	}

	first := strings.TrimSpace(segments[0])
	if strings.HasPrefix(strings.ToLower(first), "apply") {
		first = strings.TrimSpace(first[len("apply"):])
	}
	targetTokens := parseArgs(first)
	if len(targetTokens) == 0 {
		return tr.Translate("No targets provided.", false), nil
	}

	targets, err := resolveApplyTargets(ctx, targetTokens)
	if err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return tr.Translate("No matching targets found.", false), nil
	}

	registry := NewRegistry()
	output := []string{}
	for _, tgt := range targets {
		output = append(output, fmt.Sprintf(">>> Executing as %s / %s %s", tgt.Type, tgt.Name, tgt.ID))
		for _, cmdLine := range segments[1:] {
			line := strings.TrimSpace(cmdLine)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, ctx.Prefix) {
				line = strings.TrimSpace(strings.TrimPrefix(line, ctx.Prefix))
			}
			if strings.HasPrefix(strings.ToLower(line), "apply") {
				output = append(output, ">> Skipping nested apply command")
				continue
			}
			output = append(output, ">> "+formatApplyCommand(line))
			clone := *ctx
			clone.RawLine = line
			clone.TargetOverride = &Target{ID: tgt.ID, Name: tgt.Name, Type: tgt.Type}
			reply, err := registry.Execute(&clone, line)
			if err != nil {
				output = append(output, ">> Error executing command")
				continue
			}
			if reply != "" {
				output = append(output, ">> "+reply)
			}
		}
	}

	return strings.Join(output, "\n"), nil
}

func splitSegments(input string) []string {
	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	segments := []string{}
	var buf strings.Builder
	inQuote := false
	for _, r := range normalized {
		switch r {
		case '"':
			inQuote = !inQuote
			buf.WriteRune(r)
		case '|', '\n':
			if inQuote {
				buf.WriteRune(r)
			} else {
				segment := strings.TrimSpace(buf.String())
				if segment != "" {
					segments = append(segments, segment)
				}
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if tail := strings.TrimSpace(buf.String()); tail != "" {
		segments = append(segments, tail)
	}
	return segments
}

func parseArgs(input string) []string {
	args := []string{}
	var buf strings.Builder
	inQuote := false
	for _, r := range input {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t', '\n':
			if inQuote {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}

func formatApplyCommand(line string) string {
	tokens := parseArgs(line)
	if len(tokens) == 0 {
		return line
	}
	for i, token := range tokens {
		tokens[i] = strings.ReplaceAll(token, " ", "_")
	}
	return strings.Join(tokens, " ")
}

func resolveApplyTargets(ctx *Context, tokens []string) ([]Target, error) {
	targets := []Target{}
	seen := map[string]bool{}
	typeFilter := []any{"webhook", "discord:channel", "telegram:channel", "telegram:group"}

	for _, token := range tokens {
		if token == "" {
			continue
		}
		row, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": token})
		if err != nil {
			return nil, err
		}
		if row != nil {
			targets = addTargetRow(targets, seen, row)
		}
		rows, err := ctx.Query.SelectWhereInLikeQuery("humans", "name", token, "type", typeFilter)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			targets = addTargetRow(targets, seen, row)
		}
	}
	return targets, nil
}

func addTargetRow(targets []Target, seen map[string]bool, row map[string]any) []Target {
	id := fmt.Sprintf("%v", row["id"])
	name := fmt.Sprintf("%v", row["name"])
	tp := fmt.Sprintf("%v", row["type"])
	key := tp + ":" + id
	if seen[key] {
		return targets
	}
	seen[key] = true
	return append(targets, Target{ID: id, Name: name, Type: tp})
}
