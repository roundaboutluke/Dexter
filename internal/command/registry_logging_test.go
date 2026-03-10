package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/logging"
)

func TestRegistryExecuteLogsIngressAndCompletion(t *testing.T) {
	root := t.TempDir()
	initCommandLogging(t, root)

	registry := &Registry{handlers: map[string]Handler{
		"ping": stubHandler{name: "ping", reply: "pong"},
	}}
	ctx := &Context{
		Logs:        logging.Get(),
		Platform:    "discord",
		ChannelID:   "chan1",
		ChannelName: "alerts",
		UserID:      "user1",
		UserName:    "alice",
	}

	reply, err := registry.Execute(ctx, "ping alpha beta")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if reply != "pong" {
		t.Fatalf("reply=%q, want pong", reply)
	}

	body := readCommandLog(t, root)
	if !strings.Contains(body, "alerts/discord:channel-chan1: ping alpha beta") {
		t.Fatalf("commands log missing ingress line: %s", body)
	}
	if !strings.Contains(body, "alerts/discord:channel-chan1: ping alpha beta completed") {
		t.Fatalf("commands log missing completion line: %s", body)
	}
}

func TestRegistryExecuteLogsUnknownCommand(t *testing.T) {
	root := t.TempDir()
	initCommandLogging(t, root)

	registry := &Registry{handlers: map[string]Handler{}}
	ctx := &Context{
		Logs:     logging.Get(),
		Platform: "telegram",
		UserID:   "100",
		UserName: "bob",
		IsDM:     true,
	}

	if _, err := registry.Execute(ctx, "missing"); err == nil {
		t.Fatalf("expected unknown command error")
	}

	body := readCommandLog(t, root)
	if !strings.Contains(body, "bob/telegram:user-100: missing unknown command") {
		t.Fatalf("commands log missing unknown-command warning: %s", body)
	}
}

type stubHandler struct {
	name  string
	reply string
	err   error
}

func (h stubHandler) Name() string {
	return h.name
}

func (h stubHandler) Handle(*Context, []string) (string, error) {
	return h.reply, h.err
}

func initCommandLogging(t *testing.T, root string) {
	t.Helper()
	cfg := config.New(map[string]any{
		"logger": map[string]any{
			"consoleLogLevel": "error",
			"logLevel":        "debug",
			"dailyLogLimit":   7,
			"webhookLogLimit": 12,
			"enableLogs": map[string]any{
				"webhooks": true,
				"discord":  true,
				"telegram": true,
			},
		},
	})
	if err := logging.Init(cfg, root); err != nil {
		t.Fatalf("init logging: %v", err)
	}
	t.Cleanup(func() {
		_ = logging.Close()
	})
}

func readCommandLog(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "logs", "commands-*.log"))
	if err != nil {
		t.Fatalf("glob commands log: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one commands log file, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read commands log: %v", err)
	}
	return string(data)
}
