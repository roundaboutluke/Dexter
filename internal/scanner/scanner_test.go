package scanner

import (
	"testing"

	"dexter/internal/config"
)

func TestNew_NilConfig(t *testing.T) {
	client, err := New(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("expected nil client for nil config")
	}
}

func TestNew_ScannerTypeNone(t *testing.T) {
	cfg := config.New(map[string]any{
		"database": map[string]any{
			"scannerType": "none",
		},
	})
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("expected nil client for scannerType=none")
	}
}

func TestNew_InvalidScannerType(t *testing.T) {
	cfg := config.New(map[string]any{
		"database": map[string]any{
			"scannerType": "invalid",
		},
	})
	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for invalid scannerType")
	}
}
