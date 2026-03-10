package logging

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"poraclego/internal/config"
)

func TestInitCreatesCategoryFilesAndAliases(t *testing.T) {
	root := t.TempDir()
	loggers := initTestLoggers(t, root, testConfig(true, true, true, false, "silly"))

	if loggers.Commands != loggers.Command {
		t.Fatalf("commands alias mismatch")
	}

	loggers.General.Infof("general ready")
	loggers.General.Warnf("warn mirror")
	loggers.Webhooks.Infof("webhook ready")
	loggers.Discord.Infof("discord ready")
	loggers.Telegram.Infof("telegram ready")
	loggers.Commands.Infof("command ready")
	loggers.Controller.Infof("controller ready")

	if err := Close(); err != nil {
		t.Fatalf("close loggers: %v", err)
	}

	generalName, generalBody := readLogFile(t, root, "general")
	if !regexp.MustCompile(`^general-\d{4}-\d{2}-\d{2}\.log$`).MatchString(generalName) {
		t.Fatalf("unexpected general log filename %q", generalName)
	}
	if !strings.Contains(generalBody, "general ready") {
		t.Fatalf("general log missing message: %s", generalBody)
	}

	webhooksName, webhooksBody := readLogFile(t, root, "webhooks")
	if !regexp.MustCompile(`^webhooks-\d{4}-\d{2}-\d{2}-\d{2}\.log$`).MatchString(webhooksName) {
		t.Fatalf("unexpected webhooks log filename %q", webhooksName)
	}
	if !strings.Contains(webhooksBody, "webhook ready") {
		t.Fatalf("webhooks log missing message: %s", webhooksBody)
	}

	assertLogContains(t, root, "discord", "discord ready")
	assertLogContains(t, root, "telegram", "telegram ready")
	assertLogContains(t, root, "commands", "command ready")
	assertLogContains(t, root, "controller", "controller ready")
	assertLogContains(t, root, "errors", "warn mirror")
}

func TestWarnAndErrorMirrorToErrorsLog(t *testing.T) {
	root := t.TempDir()
	loggers := initTestLoggers(t, root, testConfig(true, true, true, false, "silly"))

	loggers.General.Infof("info only")
	loggers.General.Warnf("warn only")
	loggers.Discord.Errorf("discord error")

	if err := Close(); err != nil {
		t.Fatalf("close loggers: %v", err)
	}

	errorsBody := readLogBody(t, root, "errors")
	if strings.Contains(errorsBody, "info only") {
		t.Fatalf("errors log should not contain info-only lines: %s", errorsBody)
	}
	if !strings.Contains(errorsBody, "warn only") {
		t.Fatalf("errors log missing warn line: %s", errorsBody)
	}
	if !strings.Contains(errorsBody, "discord error") {
		t.Fatalf("errors log missing error line: %s", errorsBody)
	}
}

func TestDisabledCategoryFilesFallBackToWarn(t *testing.T) {
	root := t.TempDir()
	loggers := initTestLoggers(t, root, testConfig(false, false, false, false, "silly"))

	loggers.Webhooks.Infof("webhook info")
	loggers.Webhooks.Warnf("webhook warn")
	loggers.Discord.Infof("discord info")
	loggers.Discord.Warnf("discord warn")
	loggers.Telegram.Infof("telegram info")
	loggers.Telegram.Warnf("telegram warn")

	if err := Close(); err != nil {
		t.Fatalf("close loggers: %v", err)
	}

	webhooksBody := readLogBody(t, root, "webhooks")
	if strings.Contains(webhooksBody, "webhook info") {
		t.Fatalf("webhooks info should be suppressed when disabled: %s", webhooksBody)
	}
	if !strings.Contains(webhooksBody, "webhook warn") {
		t.Fatalf("webhooks warn missing: %s", webhooksBody)
	}

	discordBody := readLogBody(t, root, "discord")
	if strings.Contains(discordBody, "discord info") {
		t.Fatalf("discord info should be suppressed when disabled: %s", discordBody)
	}
	if !strings.Contains(discordBody, "discord warn") {
		t.Fatalf("discord warn missing: %s", discordBody)
	}

	telegramBody := readLogBody(t, root, "telegram")
	if strings.Contains(telegramBody, "telegram info") {
		t.Fatalf("telegram info should be suppressed when disabled: %s", telegramBody)
	}
	if !strings.Contains(telegramBody, "telegram warn") {
		t.Fatalf("telegram warn missing: %s", telegramBody)
	}
}

func TestEnabledHonorsConfiguredSinks(t *testing.T) {
	dir := t.TempDir()

	logger := newLogger("test", LevelVerbose, LevelInfo, nil, dir, "daily", 7)
	logger.console = io.Discard
	if logger.Enabled(LevelDebug) {
		t.Fatalf("debug should be disabled when below console and file thresholds")
	}
	if !logger.Enabled(LevelVerbose) {
		t.Fatalf("verbose should be enabled through console threshold")
	}
	if !logger.Enabled(LevelInfo) {
		t.Fatalf("info should be enabled through file threshold")
	}

	warnLogger := newLogger("warn", LevelError, LevelError, newRotatingWriter(dir, "errors", "daily", 7), dir, "daily", 7)
	warnLogger.console = io.Discard
	if !warnLogger.Enabled(LevelWarn) {
		t.Fatalf("warn should be enabled through error mirroring")
	}
	if warnLogger.Enabled(LevelInfo) {
		t.Fatalf("info should not be enabled without console/file support")
	}
}

func TestTimingLevelAndPvpToggle(t *testing.T) {
	cfg := config.New(map[string]any{
		"logger": map[string]any{
			"timingStats": true,
			"enableLogs": map[string]any{
				"pvp": true,
			},
		},
	})

	if got := TimingLevel(cfg); got != LevelVerbose {
		t.Fatalf("timing level=%v, want verbose", got)
	}
	if !PvpEnabled(cfg) {
		t.Fatalf("expected PvP logging to be enabled")
	}

	if got := TimingLevel(config.New(nil)); got != LevelDebug {
		t.Fatalf("default timing level=%v, want debug", got)
	}
	if PvpEnabled(config.New(nil)) {
		t.Fatalf("expected PvP logging to default to disabled")
	}
}

func initTestLoggers(t *testing.T, root string, cfg *config.Config) Loggers {
	t.Helper()
	if err := Init(cfg, root); err != nil {
		t.Fatalf("init loggers: %v", err)
	}
	t.Cleanup(func() {
		_ = Close()
		global = Loggers{}
	})
	return Get()
}

func testConfig(webhooks, discord, telegram, pvp bool, logLevel string) *config.Config {
	return config.New(map[string]any{
		"logger": map[string]any{
			"consoleLogLevel": "error",
			"logLevel":        logLevel,
			"dailyLogLimit":   7,
			"webhookLogLimit": 12,
			"enableLogs": map[string]any{
				"webhooks": webhooks,
				"discord":  discord,
				"telegram": telegram,
				"pvp":      pvp,
			},
		},
	})
}

func assertLogContains(t *testing.T, root, prefix, want string) {
	t.Helper()
	body := readLogBody(t, root, prefix)
	if !strings.Contains(body, want) {
		t.Fatalf("%s log missing %q: %s", prefix, want, body)
	}
}

func readLogBody(t *testing.T, root, prefix string) string {
	t.Helper()
	_, body := readLogFile(t, root, prefix)
	return body
}

func readLogFile(t *testing.T, root, prefix string) (string, string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "logs", prefix+"-*.log"))
	if err != nil {
		t.Fatalf("glob %s logs: %v", prefix, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %s log file, got %d", prefix, len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s log: %v", prefix, err)
	}
	return filepath.Base(matches[0]), string(data)
}
