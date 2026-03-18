package command

import (
	"context"
	"fmt"
	"strings"

	"dexter/internal/db"
	"dexter/internal/i18n"
	"dexter/internal/profile"
)

func profileDefaultMessage(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult) string {
	profileName := currentProfileName(logic.Profiles(), result.ProfileNo)
	lines := []string{}
	if quietHoursActive(logic.Human()) {
		lines = append(lines, "**Quiet Hours Enabled**")
	}
	if profileName == "" {
		lines = append(lines, tr.Translate("You don't have a profile set", false))
	} else {
		lines = append(lines, fmt.Sprintf("%s %s", tr.Translate("Your profile is currently set to:", false), profileName))
	}
	lines = append(lines, tr.TranslateFormat("Valid commands are `{0}profile <name>`, `{0}profile list`, `{0}profile add <name>`, `{0}profile remove <name>`, `{0}profile settime <times>`, `{0}profile schedule <enable|disable|toggle>`, `{0}profile copyto <name>`", ctx.Prefix))
	lines = append(lines, tr.TranslateFormat("`{0}profile settime` supports switches (e.g. `mon0900 tue1300`) and ranges (e.g. `mon:08:00-12:00 weekday:18:00-23:00`). Ranges allow quiet hours.", ctx.Prefix))
	if helpLine := singleLineHelpText(ctx, "profile", result.Language, result.Target); helpLine != "" {
		lines = append(lines, helpLine)
	}
	return strings.Join(lines, "\n")
}

func handleProfileAction(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult, args []string) (string, error) {
	if len(args) == 0 {
		return profileDefaultMessage(ctx, tr, logic, result), nil
	}

	switch strings.ToLower(args[0]) {
	case "add":
		return handleProfileAdd(tr, logic, args[1:])
	case "remove":
		return handleProfileRemove(tr, logic, args[1:])
	case "list":
		return profileList(tr, logic, result.ProfileNo), nil
	case "copyto":
		return handleProfileCopyTo(ctx, tr, logic, result, args[1:])
	case "settime":
		payload, errText := parseProfileSettime(ctx, tr, logic, result, args[1:])
		if errText != "" {
			return errText, nil
		}
		if err := logic.UpdateHours(result.ProfileNo, payload); err != nil {
			return "", err
		}
		return tr.Translate("Profile active hours updated.", false), nil
	case "schedule":
		return handleProfileScheduleToggle(ctx, tr, logic, result, args[1:])
	default:
		return handleProfileSelect(ctx, tr, logic, result, args[0])
	}
}

func handleProfileAdd(tr *i18n.Translator, logic *profile.Logic, args []string) (string, error) {
	if len(args) == 0 || strings.EqualFold(args[0], "all") {
		return tr.Translate("That is not a valid profile name", false), nil
	}
	if profileNameExists(logic.Profiles(), args[0]) {
		return tr.Translate("That profile name already exists", false), nil
	}
	if err := logic.AddProfile(args[0], "{}"); err != nil {
		return "", err
	}
	return tr.Translate("Profile added.", false), nil
}

func handleProfileRemove(tr *i18n.Translator, logic *profile.Logic, args []string) (string, error) {
	if len(args) == 0 {
		return tr.Translate("That is not a valid profile name", false), nil
	}
	profileNo, errMsg := resolveProfileNumber(logic, args[0])
	if errMsg != "" {
		return tr.Translate(errMsg, false), nil
	}
	if err := logic.DeleteProfile(profileNo); err != nil {
		return "", err
	}
	return tr.Translate("Profile removed.", false), nil
}

func handleProfileCopyTo(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult, args []string) (string, error) {
	if len(args) == 0 {
		return tr.Translate("No profiles specified.", false), nil
	}
	currentName := profileNameByNo(logic.Profiles(), result.ProfileNo)
	valid := []string{}
	invalid := []string{}
	for _, arg := range args {
		if (strings.EqualFold(arg, "all") || profileNameExists(logic.Profiles(), arg)) && !strings.EqualFold(arg, currentName) {
			valid = append(valid, arg)
		} else if !strings.EqualFold(arg, "copyto") {
			invalid = append(invalid, arg)
		}
	}
	targetNumbers := resolveProfileTargets(logic, valid)
	if len(targetNumbers) > 0 {
		if err := ctx.Query.WithTx(context.Background(), func(query *db.Query) error {
			txLogic := profile.New(query, result.TargetID)
			txLogic.SetRefreshAlertState(ctx.MarkAlertStateDirty)
			if err := txLogic.Init(); err != nil {
				return err
			}
			for _, dest := range targetNumbers {
				if dest == result.ProfileNo {
					continue
				}
				if err := txLogic.CopyProfile(result.ProfileNo, dest); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return "", err
		}
	}
	message := ""
	if len(targetNumbers) > 0 {
		targetNames := []string{}
		for _, name := range valid {
			if strings.EqualFold(name, "all") {
				continue
			}
			targetNames = append(targetNames, name)
		}
		message = fmt.Sprintf("%s%s.", tr.Translate("Current profile copied to: ", false), strings.Join(targetNames, ", "))
		if containsString(valid, "all") {
			message += " (all)"
		}
	}
	if len(targetNumbers) == 0 {
		message = tr.Translate("No valid profiles specified.", false)
	}
	if len(invalid) > 0 {
		message = strings.TrimSpace(message + fmt.Sprintf("\n%s%s.", tr.Translate("These profiles were invalid: ", false), strings.Join(invalid, ", ")))
		if containsString(invalid, currentName) {
			message += "\n" + tr.Translate("Cannot copy over the currently active profile.", false)
		}
	}
	return strings.TrimSpace(message), nil
}

func handleProfileSelect(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult, token string) (string, error) {
	profileNo, errMsg := resolveProfileNumber(logic, token)
	if errMsg != "" {
		return tr.Translate("I can't find that profile", false), nil
	}
	selected := profileByNo(logic.Profiles(), profileNo)
	update := map[string]any{"preferred_profile_no": profileNo}
	if !quietHoursActive(logic.Human()) {
		update["current_profile_no"] = profileNo
	}
	if selected != nil {
		update["area"] = selected["area"]
		update["latitude"] = selected["latitude"]
		update["longitude"] = selected["longitude"]
	}
	if _, err := ctx.Query.UpdateQuery("humans", update, map[string]any{"id": result.TargetID}); err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
	return tr.Translate("Profile set.", false), nil
}
