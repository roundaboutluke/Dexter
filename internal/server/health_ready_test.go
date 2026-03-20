package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dexter/internal/config"
)

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1:5000"
	got := clientIP(r)
	if got != "192.168.1.1" {
		t.Errorf("clientIP() = %q, want %q", got, "192.168.1.1")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	got := clientIP(r)
	if got != "10.0.0.1" {
		t.Errorf("clientIP() = %q, want %q", got, "10.0.0.1")
	}
}

func TestHealthReady_NoDB(t *testing.T) {
	cfg := config.New(map[string]any{})
	srv, err := New(cfg, nil, nil, nil, nil, nil, nil, t.TempDir(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	w := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
