package webhook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/logging"
)

func TestProcessorLogsDisabledPokemonHooks(t *testing.T) {
	root := t.TempDir()
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
		"general": map[string]any{
			"disablePokemon": true,
		},
	})
	if err := logging.Init(cfg, root); err != nil {
		t.Fatalf("init logging: %v", err)
	}
	t.Cleanup(func() {
		_ = logging.Close()
	})

	processor := &Processor{cfg: cfg}
	processor.handle(map[string]any{
		"type": "pokemon",
		"message": map[string]any{
			"encounter_id": "enc1",
		},
	})

	if err := logging.Close(); err != nil {
		t.Fatalf("close logging: %v", err)
	}

	controller := readWebhookLog(t, root, "controller")
	if !strings.Contains(controller, "enc1: wild encounter was received but set to be ignored in config") {
		t.Fatalf("controller log missing disablePokemon decision: %s", controller)
	}

	webhooks := readWebhookLog(t, root, "webhooks")
	if !strings.Contains(webhooks, "pokemon {\"encounter_id\":\"enc1\"}") {
		t.Fatalf("webhooks log missing inbound payload: %s", webhooks)
	}
}

func readWebhookLog(t *testing.T, root, prefix string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "logs", prefix+"-*.log"))
	if err != nil {
		t.Fatalf("glob %s log: %v", prefix, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one %s log file, got %d", prefix, len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s log: %v", prefix, err)
	}
	return string(data)
}
