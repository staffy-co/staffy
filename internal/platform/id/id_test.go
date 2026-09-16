package id

import (
	"testing"
	"time"
)

func TestNewV7(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	got, err := NewV7(now)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Valid() {
		t.Fatalf("generated invalid UUID: %q", got)
	}
	if got[14] != '7' {
		t.Fatalf("version nibble = %q, want 7", got[14])
	}
}

func TestParseRejectsNilWrongVersionAndWrongVariant(t *testing.T) {
	for _, value := range []string{
		"00000000-0000-0000-0000-000000000000",
		"01994f64-6c00-4000-8000-000000000001",
		"01994f64-6c00-7000-0000-000000000001",
	} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) succeeded", value)
		}
	}
}
