package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

func (s *Sender) postJSONRaw(endpoint string, payload any, headers map[string]string) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	return resp.StatusCode, string(bodyBytes), nil
}

func (s *Sender) patchJSONRaw(endpoint string, payload any, headers map[string]string) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	return resp.StatusCode, string(bodyBytes), nil
}

func (s *Sender) postJSON(url string, payload any, headers map[string]string) error {
	_, err := s.postJSONWithResponse(url, payload, headers)
	return err
}

func (s *Sender) postJSONWithResponse(url string, payload any, headers map[string]string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return bodyBytes, nil
}

func (s *Sender) postRawWithResponse(url string, body io.Reader, contentType string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return bodyBytes, nil
}

func (s *Sender) discordRequestWithRetries(method, endpoint string, body []byte, contentType string, headers map[string]string, maxRetries int) ([]byte, error) {
	return s.discordRequestWithRetryOptions(method, endpoint, body, contentType, headers, maxRetries, true)
}

func (s *Sender) discordRequestWithRetryOptions(method, endpoint string, body []byte, contentType string, headers map[string]string, maxRetries int, retryTimeouts bool) ([]byte, error) {
	const maxTimeoutRetries = 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		status, respHeaders, respBody, err := s.doRawRequest(method, endpoint, body, contentType, headers)
		if err != nil {
			if retryTimeouts && isTimeoutErr(err) && attempt < maxTimeoutRetries {
				if logger := logging.Get().Discord; logger != nil {
					logger.Warnf("discord timeout endpoint=%s attempt=%d", endpoint, attempt+1)
				}
				time.Sleep(discordTimeoutBackoff(endpoint, attempt))
				continue
			}
			return nil, err
		}
		if status == http.StatusTooManyRequests {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("discord 429 rate limit endpoint=%s attempt=%d retry_after=%s", endpoint, attempt+1, discordRetryAfter(respHeaders, respBody))
			}
			if attempt == maxRetries {
				return nil, fmt.Errorf("discord rate limited after %d retries", maxRetries)
			}
			delay := discordRetryAfter(respHeaders, respBody)
			time.Sleep(delay + discordRetryJitter(endpoint, attempt))
			continue
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("http %d: %s", status, strings.TrimSpace(string(respBody)))
		}
		return respBody, nil
	}
	return nil, fmt.Errorf("discord request exceeded retries")
}

func (s *Sender) doRawRequest(method, endpoint string, body []byte, contentType string, headers map[string]string) (int, http.Header, []byte, error) {
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	return resp.StatusCode, resp.Header, bodyBytes, nil
}

func discordRetryAfter(headers http.Header, body []byte) time.Duration {
	type discordRate struct {
		RetryAfter float64 `json:"retry_after"`
		Parameters struct {
			RetryAfter float64 `json:"retry_after"`
		} `json:"parameters"`
	}
	delaySeconds := 0.0
	var parsed discordRate
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err == nil {
			if parsed.RetryAfter > 0 {
				delaySeconds = parsed.RetryAfter
			} else if parsed.Parameters.RetryAfter > 0 {
				delaySeconds = parsed.Parameters.RetryAfter
			}
		}
	}
	if delaySeconds > 0 {
		return time.Duration(delaySeconds * float64(time.Second))
	}

	raw := strings.TrimSpace(headers.Get("Retry-After"))
	if raw == "" {
		raw = strings.TrimSpace(headers.Get("retry-after"))
	}
	if raw == "" {
		return 5 * time.Second
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
		if v > 1000 {
			return time.Duration(v) * time.Millisecond
		}
		return time.Duration(v * float64(time.Second))
	}
	return 5 * time.Second
}

func discordRetryJitter(endpoint string, attempt int) time.Duration {
	h := hashString(endpoint + string(rune(attempt)))
	return time.Duration(h%5000) * time.Millisecond
}

func discordTimeoutBackoff(endpoint string, attempt int) time.Duration {
	base := 2500*time.Millisecond + discordRetryJitter(endpoint, attempt)
	extra := time.Duration((attempt%3)*2500) * time.Millisecond
	return base + extra
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (s *Sender) postDiscordPayload(url string, payload map[string]any, headers map[string]string, webhook bool) ([]byte, error) {
	const maxRetries = 10
	if s.shouldUploadEmbedImages() {
		if body, contentType, used, err := s.buildDiscordMultipartBytes(payload, webhook); err == nil && used {
			return s.discordRequestWithRetryOptions(http.MethodPost, url, body, contentType, headers, maxRetries, false)
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return s.discordRequestWithRetryOptions(http.MethodPost, url, raw, "application/json", headers, maxRetries, false)
}

func (s *Sender) shouldUploadEmbedImages() bool {
	if s.cfg == nil {
		return false
	}
	value, ok := s.cfg.GetBool("discord.uploadEmbedImages")
	return ok && value
}

func (s *Sender) buildDiscordMultipartBytes(payload map[string]any, webhook bool) ([]byte, string, bool, error) {
	imageURL := extractEmbedImageURL(payload)
	if imageURL == "" {
		return nil, "", false, nil
	}
	clone, err := clonePayload(payload)
	if err != nil {
		return nil, "", false, err
	}
	if !setEmbedImageURL(clone, "attachment://map.png") {
		return nil, "", false, nil
	}
	resp, err := s.client.Get(imageURL)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", false, fmt.Errorf("image http %d", resp.StatusCode)
	}
	imageBytes, err := ioReadAll(resp.Body)
	if err != nil {
		return nil, "", false, err
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	payloadJSON, err := json.Marshal(clone)
	if err != nil {
		return nil, "", false, err
	}
	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		return nil, "", false, err
	}
	fieldName := "file"
	if !webhook {
		fieldName = "files[0]"
	}
	part, err := writer.CreateFormFile(fieldName, "map.png")
	if err != nil {
		return nil, "", false, err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, "", false, err
	}
	if err := writer.Close(); err != nil {
		return nil, "", false, err
	}
	return buf.Bytes(), writer.FormDataContentType(), true, nil
}

func ioReadAll(r io.Reader) ([]byte, error) {
	buf := &bytes.Buffer{}
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

func getIntConfig(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	if value, ok := cfg.GetInt(path); ok {
		return value
	}
	return fallback
}
