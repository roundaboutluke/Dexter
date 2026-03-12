package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"poraclego/internal/alertstate"
	"poraclego/internal/db"
)

// BackupCommand writes tracking backups (admin only).
type BackupCommand struct{}

func (c *BackupCommand) Name() string { return "backup" }

func (c *BackupCommand) Handle(ctx *Context, args []string) (string, error) {
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return tr.TranslateFormat("Your backup needs a name, please run `{0}backup <name>`", ctx.Prefix), nil
	}
	if containsWord(args, "remove") {
		args = removeWord(args, "remove")
		if len(args) == 0 {
			return tr.Translate("Include the name of the backup you want to remove", false), nil
		}
		name := args[0]
		return removeBackup(ctx.Root, name), nil
	}
	if containsWord(args, "list") {
		return tr.TranslateFormat("To list existing backups, run `{0}restore list`", ctx.Prefix), nil
	}
	name := args[0]
	backup, err := buildBackup(ctx, result.TargetID, result.ProfileNo)
	if err != nil {
		return "", err
	}
	if err := saveBackup(ctx.Root, name, backup); err != nil {
		return "", err
	}
	return "✅", nil
}

// RestoreCommand restores tracking backups.
type RestoreCommand struct{}

func (c *RestoreCommand) Name() string { return "restore" }

func (c *RestoreCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return tr.Translate("Specify a backup name.", false), nil
	}
	if containsWord(args, "list") {
		files := listBackups(ctx.Root)
		return fmt.Sprintf("%s ```\n%s\n```", tr.Translate("Available backups are", false), strings.Join(files, ",\n")), nil
	}

	backup, err := loadBackup(ctx.Root, args[0])
	if err != nil {
		files := listBackups(ctx.Root)
		return fmt.Sprintf("%s %s\n%s ```\n%s\n```", args[0], tr.Translate("is not an existing backup", false), tr.Translate("Available backups are", false), strings.Join(files, ",\n")), nil
	}
	if err := restoreBackup(ctx, backup, result.TargetID, result.ProfileNo); err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
	return "✅", nil
}

type backupPayload map[string][]map[string]any

func buildBackup(ctx *Context, targetID string, profileNo int) (backupPayload, error) {
	categories := alertstate.TrackedTables()
	backup := backupPayload{}
	for _, category := range categories {
		rows, err := ctx.Query.SelectAllQuery(category, map[string]any{"id": targetID, "profile_no": profileNo})
		if err != nil {
			return nil, err
		}
		sanitized := sanitizeRows(rows)
		backup[category] = sanitized
	}
	return backup, nil
}

func sanitizeRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		copy := map[string]any{}
		for key, value := range row {
			if key == "uid" {
				continue
			}
			if key == "id" || key == "profile_no" {
				continue
			}
			copy[key] = value
		}
		out = append(out, copy)
	}
	return out
}

func saveBackup(root, name string, payload backupPayload) error {
	dir := filepath.Join(root, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.json", name))
	raw, err := json.MarshalIndent(payload, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func removeBackup(root, name string) string {
	path := filepath.Join(root, "backups", fmt.Sprintf("%s.json", name))
	if err := os.Remove(path); err != nil {
		return "👌"
	}
	return "✅"
}

func listBackups(root string) []string {
	dir := filepath.Join(root, "backups")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".json") {
			names = append(names, strings.TrimSuffix(name, ".json"))
		}
	}
	return names
}

func loadBackup(root, name string) (backupPayload, error) {
	path := filepath.Join(root, "backups", fmt.Sprintf("%s.json", name))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload backupPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func restoreBackup(ctx *Context, payload backupPayload, targetID string, profileNo int) error {
	return withAlertStateTx(ctx, func(query *db.Query) error {
		for category, rows := range payload {
			for _, row := range rows {
				row["id"] = targetID
				row["profile_no"] = profileNo
			}
			if len(rows) == 0 {
				continue
			}
			if _, err := query.DeleteQuery(category, map[string]any{"id": targetID, "profile_no": profileNo}); err != nil {
				return err
			}
			if _, err := query.InsertOrUpdateQuery(category, rows); err != nil {
				return err
			}
		}
		return nil
	})
}

func removeWord(args []string, target string) []string {
	out := []string{}
	for _, arg := range args {
		if strings.EqualFold(arg, target) {
			continue
		}
		out = append(out, arg)
	}
	return out
}
