package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"poraclego/internal/config"
)

type urlShortener interface {
	Shorten(raw string) (string, error)
}

func newShortener(cfg *config.Config) urlShortener {
	provider, _ := cfg.GetString("geocoding.shortlinkProvider")
	provider = strings.ToLower(provider)
	switch provider {
	case "hideuri":
		return &hideuriShortener{client: httpClient()}
	case "yourls":
		uri, _ := cfg.GetString("geocoding.shortlinkProviderURL")
		key, _ := cfg.GetString("geocoding.shortlinkProviderKey")
		return &yourlsShortener{client: httpClient(), baseURL: uri, signature: key}
	case "shlink":
		uri, _ := cfg.GetString("geocoding.shortlinkProviderURL")
		key, _ := cfg.GetString("geocoding.shortlinkProviderKey")
		domain, _ := cfg.GetString("geocoding.shortlinkProviderDomain")
		return &shlinkShortener{client: httpClient(), baseURL: uri, apiKey: key, domain: domain}
	default:
		return nil
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second}
}

type hideuriShortener struct {
	client *http.Client
}

func (s *hideuriShortener) Shorten(raw string) (string, error) {
	data := url.Values{}
	data.Set("url", raw)
	resp, err := s.client.Post("https://hideuri.com/api/v1/shorten", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var payload struct {
		ResultURL string `json:"result_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.ResultURL, nil
}

type yourlsShortener struct {
	client    *http.Client
	baseURL   string
	signature string
}

func (s *yourlsShortener) Shorten(raw string) (string, error) {
	if s.baseURL == "" || s.signature == "" {
		return "", fmt.Errorf("yourls config missing")
	}
	base := strings.TrimRight(s.baseURL, "/") + "/yourls-api.php"
	reqURL := fmt.Sprintf("%s?signature=%s&action=shorturl&format=json&url=%s", base, url.QueryEscape(s.signature), url.QueryEscape(raw))
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var payload struct {
		ShortURL string `json:"shorturl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.ShortURL, nil
}

type shlinkShortener struct {
	client  *http.Client
	baseURL string
	apiKey  string
	domain  string
}

func (s *shlinkShortener) Shorten(raw string) (string, error) {
	if s.baseURL == "" || s.apiKey == "" {
		return "", fmt.Errorf("shlink config missing")
	}
	body := map[string]any{"longUrl": raw}
	if s.domain != "" {
		body["domain"] = s.domain
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.baseURL, "/")+"/rest/v2/short-urls", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", s.apiKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var response struct {
		ShortURL string `json:"shortUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	return response.ShortURL, nil
}
