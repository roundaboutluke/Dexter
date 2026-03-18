package bot

import (
	"testing"

	"dexter/internal/command"
)

func TestInsertAutocreateTargetFailureSkipsCommandsAndRefresh(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{}, 1)
	commandCount := 0
	env.discord.manager.registry = command.NewRegistry()
	env.discord.manager.registry.Register(autocreateStubHandler{
		name: "autocreatetest",
		fn: func(ctx *command.Context, _ []string) (string, error) {
			commandCount++
			return "ok", nil
		},
	})

	ctx := &command.Context{
		Query:             env.query,
		RefreshAlertCache: env.discord.manager.RefreshAlertState,
	}
	target := commandTarget{ID: "chan-1", Type: "discord:channel", Name: "alerts"}

	dirty := false
	if env.discord.insertAutocreateTarget(ctx, target) {
		dirty = true
		clone := *ctx
		clone.TargetOverride = &command.Target{ID: target.ID, Type: target.Type, Name: target.Name}
		_, _ = env.discord.manager.Registry().Execute(&clone, "autocreatetest")
	}
	if dirty {
		env.discord.manager.RefreshAlertState()
	}

	env.assertNoRefresh(t)
	if commandCount != 0 {
		t.Fatalf("commandCount=%d, want 0", commandCount)
	}
	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": target.ID})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if row != nil {
		t.Fatalf("row=%v, want nil", row)
	}
}

func TestInsertAutocreateTargetSuccessExecutesCommandsAndRefreshesOnce(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{}, 0)
	commandCount := 0
	env.discord.manager.registry = command.NewRegistry()
	env.discord.manager.registry.Register(autocreateStubHandler{
		name: "autocreatetest",
		fn: func(ctx *command.Context, _ []string) (string, error) {
			commandCount++
			if ctx.TargetOverride == nil || ctx.TargetOverride.ID != "chan-1" {
				t.Fatalf("target override=%v, want chan-1", ctx.TargetOverride)
			}
			return "ok", nil
		},
	})

	ctx := &command.Context{
		Query:             env.query,
		RefreshAlertCache: env.discord.manager.RefreshAlertState,
	}
	target := commandTarget{ID: "chan-1", Type: "discord:channel", Name: "alerts"}

	dirty := false
	if env.discord.insertAutocreateTarget(ctx, target) {
		dirty = true
		clone := *ctx
		clone.TargetOverride = &command.Target{ID: target.ID, Type: target.Type, Name: target.Name}
		_, _ = env.discord.manager.Registry().Execute(&clone, "autocreatetest")
	}
	if dirty {
		env.discord.manager.RefreshAlertState()
	}

	env.waitForRefresh(t, 1)
	if commandCount != 1 {
		t.Fatalf("commandCount=%d, want 1", commandCount)
	}
	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": target.ID})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if row == nil {
		t.Fatalf("expected inserted human row")
	}
}

func TestInsertAutocreateTargetsMixedResultsRefreshOnceAfterAnySuccess(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{}, 2)
	commandCount := 0
	env.discord.manager.registry = command.NewRegistry()
	env.discord.manager.registry.Register(autocreateStubHandler{
		name: "autocreatetest",
		fn: func(ctx *command.Context, _ []string) (string, error) {
			commandCount++
			return "ok", nil
		},
	})

	ctx := &command.Context{
		Query:             env.query,
		RefreshAlertCache: env.discord.manager.RefreshAlertState,
	}
	targets := []commandTarget{
		{ID: "chan-1", Type: "discord:channel", Name: "alerts-1"},
		{ID: "chan-2", Type: "discord:channel", Name: "alerts-2"},
	}

	dirty := false
	for _, target := range targets {
		if !env.discord.insertAutocreateTarget(ctx, target) {
			continue
		}
		dirty = true
		clone := *ctx
		clone.TargetOverride = &command.Target{ID: target.ID, Type: target.Type, Name: target.Name}
		_, _ = env.discord.manager.Registry().Execute(&clone, "autocreatetest")
	}
	if dirty {
		env.discord.manager.RefreshAlertState()
	}

	env.waitForRefresh(t, 1)
	if commandCount != 1 {
		t.Fatalf("commandCount=%d, want 1", commandCount)
	}
	first, err := env.query.SelectOneQuery("humans", map[string]any{"id": "chan-1"})
	if err != nil {
		t.Fatalf("select first human: %v", err)
	}
	if first == nil {
		t.Fatalf("expected first inserted human row")
	}
	second, err := env.query.SelectOneQuery("humans", map[string]any{"id": "chan-2"})
	if err != nil {
		t.Fatalf("select second human: %v", err)
	}
	if second != nil {
		t.Fatalf("second=%v, want nil", second)
	}
}

type autocreateStubHandler struct {
	name string
	fn   func(*command.Context, []string) (string, error)
}

func (h autocreateStubHandler) Name() string {
	return h.name
}

func (h autocreateStubHandler) Handle(ctx *command.Context, args []string) (string, error) {
	if h.fn != nil {
		return h.fn(ctx, args)
	}
	return "", nil
}
