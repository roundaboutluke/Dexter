package command

import (
	"dexter/internal/profile"
)

// ProfileCommand manages user profiles.
type ProfileCommand struct{}

func (c *ProfileCommand) Name() string { return "profile" }

func (c *ProfileCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "profile") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}

	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	logic := profile.New(ctx.Query, result.TargetID)
	logic.SetRefreshAlertState(ctx.MarkAlertStateDirty)
	if err := logic.Init(); err != nil {
		return "", err
	}

	return handleProfileAction(ctx, tr, logic, result, args)
}
