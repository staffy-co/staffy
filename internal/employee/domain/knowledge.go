package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

type KnowledgeSource struct {
	ChecksumKey        []byte     `json:"-"`
	TenantID           id.UUID    `json:"tenant_id"`
	EmployeeID         id.UUID    `json:"employee_id"`
	SourceID           id.UUID    `json:"source_id"`
	Type               string     `json:"type"`
	Title              string     `json:"title"`
	Owner              string     `json:"owner"`
	Provenance         string     `json:"provenance"`
	Origin             string     `json:"origin,omitempty"`
	IngestReason       string     `json:"ingest_reason,omitempty"`
	Scope              string     `json:"scope"`
	Priority           int        `json:"priority"`
	Language           string     `json:"language"`
	Status             string     `json:"status"`
	Version            int64      `json:"version"`
	CurrentVersion     int64      `json:"current_version"`
	ConflictNote       string     `json:"conflict_note,omitempty"`
	ConflictProvenance string     `json:"conflict_provenance,omitempty"`
	FreshUntil         *time.Time `json:"fresh_until,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type KnowledgeChunk struct {
	SourceID   id.UUID   `json:"source_id"`
	Version    int64     `json:"version"`
	Position   int       `json:"position"`
	Text       string    `json:"text"`
	Checksum   string    `json:"checksum"`
	Origin     string    `json:"origin,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
}

type KnowledgeVersion struct {
	SourceID            id.UUID          `json:"source_id"`
	Version             int64            `json:"version"`
	Checksum            string           `json:"checksum"`
	Text                string           `json:"text"`
	Chunks              []KnowledgeChunk `json:"chunks"`
	RemovedFromPrevious string           `json:"removed_from_previous,omitempty"`
	AddedFromPrevious   string           `json:"added_from_previous,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	Origin              string           `json:"origin,omitempty"`
}

func KnowledgeDiff(previous, current string) (string, string) {
	a, b := []rune(previous), []rune(current)
	start := 0
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	return string(a[start:endA]), string(b[start:endB])
}

func (s KnowledgeSource) EffectiveStatus(now time.Time) string {
	if s.Status == "retired" {
		return "retired"
	}
	if s.ConflictNote != "" {
		return "conflict"
	}
	if s.FreshUntil != nil && !now.Before(*s.FreshUntil) {
		return "stale"
	}
	return s.Status
}

func (s KnowledgeSource) Valid() bool {
	switch s.Type {
	case "faq", "regulation", "dialogue", "website", "document":
	default:
		return false
	}
	return length(s.Title, 1, 160) && length(s.Owner, 1, 160) && length(s.Provenance, 1, 500) && len(s.Origin) <= 2048 && length(s.Scope, 1, 160) && length(s.Language, 2, 32) && s.Priority >= 0 && s.Priority <= 100
}

func ValidKnowledgeText(value string) bool {
	if len(value) == 0 || len(value) > 256<<10 || !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(lower, "://") || strings.Contains(lower, "<") || strings.Contains(lower, ">") {
		return false
	}
	return true
}

func KnowledgeChecksum(key []byte, value string) string {
	if len(key) < 32 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func KnowledgeChunks(key []byte, value string) []KnowledgeChunk {
	const maxBytes = 2048
	chunks := []KnowledgeChunk{}
	for len(value) > 0 {
		end := len(value)
		if end > maxBytes {
			end = maxBytes
			for end > 0 && !utf8.RuneStart(value[end]) {
				end--
			}
		}
		part := value[:end]
		chunks = append(chunks, KnowledgeChunk{Position: len(chunks), Text: part, Checksum: KnowledgeChecksum(key, part)})
		value = value[end:]
	}
	return chunks
}
