package command

// IncidentCommand is an alias of invasion.
type IncidentCommand struct{}

func (c *IncidentCommand) Name() string { return "incident" }

func (c *IncidentCommand) Handle(ctx *Context, args []string) (string, error) {
	inner := &InvasionCommand{}
	return inner.Handle(ctx, args)
}
