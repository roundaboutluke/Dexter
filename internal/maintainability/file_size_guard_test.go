package maintainability_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHotspotFilesStayManageable(t *testing.T) {
	root := repoRoot(t)

	assertMaxLines(t, root, "internal/server/routes.go", 1500)
	assertMaxLines(t, root, "internal/bot/discord_slash.go", 1500)
	assertMaxLines(t, root, "internal/bot/discord_reconcile.go", 1500)
	assertMaxLines(t, root, "internal/dispatch/sender.go", 1500)
	assertMaxLines(t, root, "internal/profile/scheduler.go", 1500)
	assertMaxLines(t, root, "internal/render/helpers.go", 1500)
	assertMaxLines(t, root, "internal/webhook/alerts.go", 1500)
	assertMaxLines(t, root, "internal/webhook/geocoder.go", 1500)
	assertMaxLines(t, root, "internal/webhook/processor.go", 1500)
	assertMaxLines(t, root, "internal/webhook/weather_tracker.go", 1500)

	assertGlobMaxLines(t, root, "internal/server/routes_*.go", 800)
	assertGlobMaxLines(t, root, "internal/bot/discord_slash_*.go", 800)
	assertGlobMaxLines(t, root, "internal/bot/discord_reconcile_*.go", 800)
	assertGlobMaxLines(t, root, "internal/dispatch/sender_*.go", 800)
	assertGlobMaxLines(t, root, "internal/profile/scheduler_*.go", 800)
	assertGlobMaxLines(t, root, "internal/render/helpers_*.go", 800)
	assertGlobMaxLines(t, root, "internal/webhook/geocoder_*.go", 800)
	assertGlobMaxLines(t, root, "internal/webhook/alerts_*.go", 800)
	assertGlobMaxLines(t, root, "internal/webhook/processor_*.go", 800)
	assertGlobMaxLines(t, root, "internal/webhook/weather_tracker_*.go", 800)
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func assertGlobMaxLines(t *testing.T, root, pattern string, max int) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(matches) == 0 {
		t.Fatalf("glob %s matched no files", pattern)
	}
	for _, match := range matches {
		if strings.HasSuffix(match, "_test.go") {
			continue
		}
		assertMaxLinesAbs(t, root, match, max)
	}
}

func assertMaxLines(t *testing.T, root, rel string, max int) {
	t.Helper()
	assertMaxLinesAbs(t, root, filepath.Join(root, rel), max)
}

func assertMaxLinesAbs(t *testing.T, root, abs string, max int) {
	t.Helper()

	content, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	lines := 0
	if len(content) > 0 {
		lines = 1 + strings.Count(string(content), "\n")
		if content[len(content)-1] == '\n' {
			lines--
		}
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		rel = abs
	}
	if lines > max {
		t.Fatalf("%s has %d lines; max allowed is %d", rel, lines, max)
	}
}
