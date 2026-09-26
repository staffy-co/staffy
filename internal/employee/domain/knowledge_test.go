package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestKnowledgeTextValidationAndChunks(t *testing.T) {
	for _, bad := range []string{"", "https://example.com", "FAQ https://example.com", "ftp://example.com/faq", "file:///tmp/faq.txt", "FAQ FILE://server/share", "<html>bad</html>", strings.Repeat("x", 256<<10+1), string([]byte{0xff})} {
		if ValidKnowledgeText(bad) {
			t.Fatalf("accepted invalid input of %d bytes", len(bad))
		}
	}
	value := strings.Repeat("Пример записи. ", 200)
	if !ValidKnowledgeText(value) {
		t.Fatal("valid text rejected")
	}
	key := []byte("a-32-byte-secret-for-tests-00000001")
	chunks := KnowledgeChunks(key, value)
	if len(chunks) < 2 {
		t.Fatal("expected chunks")
	}
	var joined string
	for i, c := range chunks {
		if c.Position != i || len(c.Text) > 2048 || c.Checksum != KnowledgeChecksum(key, c.Text) {
			t.Fatalf("bad chunk %d", i)
		}
		joined += c.Text
	}
	if joined != value {
		t.Fatal("chunking changed content")
	}
}
func TestKnowledgeChecksumUsesSourceKey(t *testing.T) {
	short := "Да"
	raw := sha256.Sum256([]byte(short))
	a := KnowledgeChecksum([]byte("a-32-byte-secret-for-tests-00000001"), short)
	b := KnowledgeChecksum([]byte("b-32-byte-secret-for-tests-00000002"), short)
	if a == hex.EncodeToString(raw[:]) || a == b {
		t.Fatalf("checksum exposes raw short-text hash or ignores key")
	}
}
func TestKnowledgeEffectiveStatus(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Second)
	s := KnowledgeSource{Status: "ready", FreshUntil: &past}
	if got := s.EffectiveStatus(now); got != "stale" {
		t.Fatal(got)
	}
	s.ConflictNote = "Разные сведения"
	if got := s.EffectiveStatus(now); got != "conflict" {
		t.Fatal(got)
	}
	s.Status = "retired"
	if got := s.EffectiveStatus(now); got != "retired" {
		t.Fatal(got)
	}
}
