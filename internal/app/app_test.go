package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/logging"
)

func TestPreloadAlertStateSuccess(t *testing.T) {
	preloader := &stubAlertStatePreloader{}

	if err := preloadAlertState(preloader, false); err != nil {
		t.Fatalf("preloadAlertState err=%v, want nil", err)
	}
	if preloader.calls != 1 {
		t.Fatalf("calls=%d, want 1", preloader.calls)
	}
}

func TestPreloadAlertStateFailureWithoutForcedMigrationReturnsError(t *testing.T) {
	preloader := &stubAlertStatePreloader{err: errors.New("boom")}

	err := preloadAlertState(preloader, false)
	if err == nil {
		t.Fatalf("expected preload error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%v, want boom", err)
	}
}

func TestPreloadAlertStateFailureAfterForcedMigrationContinuesAndLogsWarning(t *testing.T) {
	root := t.TempDir()
	initAppLogging(t, root)

	preloader := &stubAlertStatePreloader{err: errors.New("boom")}

	if err := preloadAlertState(preloader, true); err != nil {
		t.Fatalf("preloadAlertState err=%v, want nil", err)
	}

	body := readGeneralLog(t, root)
	if !strings.Contains(body, "alert state preload failed after force-tolerated migration error; continuing without preloaded snapshot: boom") {
		t.Fatalf("general log missing degraded-start warning: %s", body)
	}
	if !strings.Contains(body, "webhook matching will use the DB fallback until a later alert-state refresh succeeds") {
		t.Fatalf("general log missing fallback warning: %s", body)
	}
}

type stubAlertStatePreloader struct {
	calls int
	err   error
}

func (s *stubAlertStatePreloader) RefreshAlertCacheSync() error {
	s.calls++
	return s.err
}

func initAppLogging(t *testing.T, root string) {
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

func readGeneralLog(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "logs", "general-*.log"))
	if err != nil {
		t.Fatalf("glob general log: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one general log file, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read general log: %v", err)
	}
	return string(data)
}
