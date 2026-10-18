package packresolver

import "testing"

// TestGetResolvesPublishedPackVersions — точные пин-пары опубликованных
// паков резолвятся (живой каталог employee).
func TestGetResolvesPublishedPackVersions(t *testing.T) {
	adapter := New()
	for _, pin := range []struct{ id, version string }{
		{"auto-parts", "1.0.0"},
		{"auto-parts", "2.0.0"},
		{"montagio-services", "1.0.0"},
	} {
		if !adapter.Get(pin.id, pin.version) {
			t.Fatalf("опубликованный пак %s@%s обязан резолвиться", pin.id, pin.version)
		}
	}
}

// TestGetRefusesUnknownPackPins — неизвестные пины и несуществующие версии
// не резолвятся (честный отказ, никогда не догадка).
func TestGetRefusesUnknownPackPins(t *testing.T) {
	adapter := New()
	for _, pin := range []struct{ id, version string }{
		{"auto-parts", "9.9.9"},
		{"novofon-parts", "1.0.0"},
		{"", "1.0.0"},
		{"auto-parts", ""},
	} {
		if adapter.Get(pin.id, pin.version) {
			t.Fatalf("пин %q@%q не обязан резолвиться", pin.id, pin.version)
		}
	}
}
