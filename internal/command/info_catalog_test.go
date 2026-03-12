package command

import (
	"testing"
	"time"
)

func TestFormatCatalogEntriesUsesSecondaryLabelForDuplicateMoves(t *testing.T) {
	entries := []catalogEntry{
		{Name: "Thunder Shock", Secondary: "Electric"},
		{Name: "Thunder Shock", Secondary: "Ghost"},
		{Name: "Potion"},
	}

	got := formatCatalogEntries(nil, "Recognised entries:", entries, func(entry catalogEntry) bool {
		return entry.Secondary != ""
	})

	want := "Recognised entries:\nPotion\nThunder\\_Shock/Electric\nThunder\\_Shock/Ghost"
	if got != want {
		t.Fatalf("formatCatalogEntries()=%q, want %q", got, want)
	}
}

func TestFormatUptimeAndFormatNumber(t *testing.T) {
	if got := formatUptime(51 * time.Second); got != "00:00:00:51" {
		t.Fatalf("formatUptime(51s)=%q", got)
	}
	if got := formatNumber("en", 1234567); got != "1,234,567" {
		t.Fatalf("formatNumber()=%q", got)
	}
}
