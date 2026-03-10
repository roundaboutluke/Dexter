package dispatch

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/logging"
)

func TestDiscordRequestWithRetriesLogsRateLimits(t *testing.T) {
	root := t.TempDir()
	initDispatchLogging(t, root)

	sender := &Sender{
		client: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(`{"retry_after":0.001}`)),
				}, nil
			}),
		},
	}
	endpoint := "https://discord.test/api/messages"
	if _, err := sender.discordRequestWithRetries(http.MethodPost, endpoint, []byte(`{}`), "application/json", nil, 0); err == nil {
		t.Fatalf("expected discord rate-limit error")
	}

	body := readDispatchLog(t, root, "discord")
	if !strings.Contains(body, "discord 429 rate limit endpoint="+endpoint) {
		t.Fatalf("discord log missing rate-limit line: %s", body)
	}
	if !strings.Contains(body, "attempt=1") {
		t.Fatalf("discord log missing attempt counter: %s", body)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func initDispatchLogging(t *testing.T, root string) {
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

func readDispatchLog(t *testing.T, root, prefix string) string {
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
