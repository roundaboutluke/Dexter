package tileserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

// TileOptions configures a tileservercache request.
type TileOptions struct {
	Type         string
	Width        int
	Height       int
	Zoom         float64
	Pregenerate  bool
	IncludeStops bool
}

// Client handles tileservercache requests.
type Client struct {
	cfg    *config.Config
	client *http.Client
}

// NewClient creates a tileserver client.
func NewClient(cfg *config.Config) *Client {
	timeout := 10 * time.Second
	if cfg != nil {
		if ms, ok := cfg.GetInt("tuning.tileserverTimeout"); ok && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

// GetMapURL returns a tileserver URL (pregenerated or dynamic).
func (c *Client) GetMapURL(templateType string, data map[string]any, options TileOptions) (string, error) {
	if c == nil || c.cfg == nil {
		return "", fmt.Errorf("tileserver client missing config")
	}
	baseURL, _ := c.cfg.GetString("geocoding.staticProviderURL")
	if baseURL == "" {
		return "", fmt.Errorf("staticProviderURL not set")
	}
	if strings.ToLower(options.Type) == "multistaticmap" {
		return c.getURL(baseURL, "multistaticmap", "multi-"+templateType, data, options.Pregenerate)
	}
	return c.getURL(baseURL, "staticmap", templateType, data, options.Pregenerate)
}

// GetOptions returns configured tileserver options for a map type.
func GetOptions(cfg *config.Config, mapType string) TileOptions {
	opts := TileOptions{
		Type:         "staticMap",
		Width:        500,
		Height:       250,
		Zoom:         15,
		Pregenerate:  true,
		IncludeStops: false,
	}
	if cfg == nil {
		return opts
	}
	configType := mapType
	if mapType == "monster" {
		configType = "pokemon"
	}

	if raw, ok := cfg.Get("geocoding.tileserverSettings.default"); ok {
		applyOptions(&opts, raw)
	}
	if raw, ok := cfg.Get(fmt.Sprintf("geocoding.staticMapType.%s", configType)); ok {
		if value, ok := raw.(string); ok && value != "" {
			if strings.HasPrefix(value, "*") {
				opts.Type = strings.TrimPrefix(value, "*")
				opts.Pregenerate = false
			} else {
				opts.Type = value
				opts.Pregenerate = true
			}
		}
	}
	if raw, ok := cfg.Get(fmt.Sprintf("geocoding.tileserverSettings.%s", mapType)); ok {
		applyOptions(&opts, raw)
	}
	return opts
}

func (c *Client) getURL(baseURL, mapType, template string, data map[string]any, pregenerate bool) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if pregenerate {
		start := time.Now()
		endpoint := fmt.Sprintf("%s/%s/poracle-%s?pregenerate=true&regeneratable=true", baseURL, strings.ToLower(mapType), template)
		body, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		resp, err := c.client.Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			if logger := logging.Get().General; logger != nil {
				logger.Warnf("tileserver pregenerate failed (%s/%s): http %d", mapType, template, resp.StatusCode)
			}
			return "", fmt.Errorf("tileserver http %d", resp.StatusCode)
		}
		response := strings.TrimSpace(string(raw))
		if contentType := strings.ToLower(resp.Header.Get("Content-Type")); strings.Contains(contentType, "json") {
			var decoded any
			if err := json.Unmarshal(raw, &decoded); err == nil {
				switch value := decoded.(type) {
				case string:
					response = strings.TrimSpace(value)
				default:
					if logger := logging.Get().General; logger != nil {
						logger.Warnf("tileserver pregenerate invalid response (%s/%s)", mapType, template)
					}
					return "", fmt.Errorf("tileserver invalid response")
				}
			}
		}
		if strings.Contains(response, "<") {
			if logger := logging.Get().General; logger != nil {
				logger.Warnf("tileserver pregenerate returned HTML (%s/%s)", mapType, template)
			}
			return "", fmt.Errorf("tileserver invalid response")
		}
		if logger := logging.Get().General; logger != nil {
			logger.Logf(logging.TimingLevel(c.cfg), "tileserver generated %s/%s (%d ms)", mapType, template, time.Since(start).Milliseconds())
		}
		if strings.HasPrefix(response, "http") {
			return response, nil
		}
		return fmt.Sprintf("%s/%s/pregenerated/%s", baseURL, strings.ToLower(mapType), response), nil
	}
	endpoint := fmt.Sprintf("%s/%s/poracle-%s", baseURL, strings.ToLower(mapType), template)
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for key, value := range data {
		q.Set(key, encodeQueryValue(value))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func encodeQueryValue(value any) string {
	switch v := value.(type) {
	case []string:
		return strings.Join(v, ",")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ",")
	case []int:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%d", item))
		}
		return strings.Join(parts, ",")
	case []float64:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", value)
	}
}

func applyOptions(opts *TileOptions, raw any) {
	entry, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if value, ok := entry["type"].(string); ok && value != "" {
		opts.Type = value
	}
	if value, ok := entry["width"].(float64); ok && value > 0 {
		opts.Width = int(value)
	}
	if value, ok := entry["height"].(float64); ok && value > 0 {
		opts.Height = int(value)
	}
	if value, ok := entry["zoom"].(float64); ok && value > 0 {
		opts.Zoom = value
	}
	if value, ok := entry["pregenerate"].(bool); ok {
		opts.Pregenerate = value
	}
	if value, ok := entry["includeStops"].(bool); ok {
		opts.IncludeStops = value
	}
}
