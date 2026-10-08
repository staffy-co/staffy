package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Canonical Channel Event (Story 2.1, first slice)
// ---------------------------------------------------------------------------

// The event direction dictionary: this slice receives inbound events only —
// one admitted value is an honest boundary, not an omission (outbound
// replies are the later stories of the case contour).
const DirectionInbound = "inbound"

// The event modality dictionary mirrors the profile matrix modalities.
const (
	ModalityVoice = "voice"
	ModalityText  = "text"
)

// Event states: an accepted event is material; a quarantined delivery is
// kept for the human without ever rewriting the accepted record.
const (
	EventStateAccepted    = "accepted"
	EventStateQuarantined = "quarantined"
)

// ChannelEvent is the canonical domain event. A provider DTO never becomes
// this model directly: the ingress contract is the only translator, and the
// payload lives sealed beside the record (digest + HMAC), never inline.
type ChannelEvent struct {
	TenantID          id.UUID
	CaseID            id.UUID
	ConversationID    id.UUID
	EventID           id.UUID
	ConnectionID      id.UUID
	Provider          string
	ProviderEventID   string
	ProviderSessionID string
	Direction         string
	Modality          string
	Channel           string
	CorrelationID     id.UUID
	ProviderTimestamp time.Time
	ReceivedAt        time.Time
	PayloadDigest     string
	IdempotencyKey    string
	State             string
	CreatedAt         time.Time
}

// ---------------------------------------------------------------------------
// Ingress envelope (the versioned machine contract, schema v1)
// ---------------------------------------------------------------------------

// EnvelopeSchemaVersion is the only envelope schema this build reads. A
// contract of another version is an honest refusal, never a best-effort
// decode.
const EnvelopeSchemaVersion = 1

// Envelope is the decoded canonical ingress envelope. Payload stays raw
// canonical JSON bytes: the domain never interprets provider content.
type Envelope struct {
	SchemaVersion     int
	TenantClaim       id.UUID
	ConnectionID      id.UUID
	Provider          string
	ProviderEventID   string
	ProviderSessionID string
	Direction         string
	Modality          string
	Channel           string
	ProviderTimestamp time.Time
	Payload           []byte
	PayloadDigest     string
	EnvelopeBytes     int
}

// EnvelopeError is the honest malformed-envelope refusal: the field, the
// safe reason. Payload content never travels inside the reason.
type EnvelopeError struct {
	Field  string
	Reason string
}

func (e EnvelopeError) Error() string {
	return fmt.Sprintf("ingress envelope: %s: %s", e.Field, e.Reason)
}

// ErrEnvelopeMalformed marks every structural refusal uniformly.
var ErrEnvelopeMalformed = errors.New("ingress envelope is malformed")

// maxProviderClockSkew bounds how far ahead of the receive time the
// provider timestamp may claim to be; the receive time is never taken from
// the provider.
const maxProviderClockSkew = 48 * time.Hour

// ValidateEnvelope checks the structural dictionary of the decoded
// envelope: schema version, identifiers, direction, channel, modality and
// the channel-honest modality agreement. Sizes are NOT refused here — the
// admission ladder bounds them honestly with counters (envelope_exceeded);
// the transport keeps only a hard DoS cap above the dictionary limit.
func ValidateEnvelope(envelope Envelope, receivedAt time.Time) (Envelope, error) {
	if envelope.SchemaVersion != EnvelopeSchemaVersion {
		return envelope, EnvelopeError{Field: "schema_version", Reason: fmt.Sprintf("Поддерживается только версия %d; обновите отправителя.", EnvelopeSchemaVersion)}
	}
	if !envelope.TenantClaim.Valid() {
		return envelope, EnvelopeError{Field: "tenant_id", Reason: "Требуется UUID пространства."}
	}
	if !envelope.ConnectionID.Valid() {
		return envelope, EnvelopeError{Field: "connection_id", Reason: "Требуется UUID подключения."}
	}
	if err := validateEnvelopeToken("provider", envelope.Provider, 64); err != nil {
		return envelope, *err
	}
	if err := validateEnvelopeToken("provider_event_id", envelope.ProviderEventID, 128); err != nil {
		return envelope, *err
	}
	if err := validateEnvelopeToken("provider_session_id", envelope.ProviderSessionID, 128); err != nil {
		return envelope, *err
	}
	if envelope.Direction != DirectionInbound {
		return envelope, EnvelopeError{Field: "direction", Reason: "Срез принимает только inbound-события."}
	}
	if !AllowedChannels[envelope.Channel] {
		return envelope, EnvelopeError{Field: "channel", Reason: "Канал не входит в словарь каналов профиля."}
	}
	if envelope.Modality != ModalityOfChannel(envelope.Channel) {
		return envelope, EnvelopeError{Field: "modality", Reason: fmt.Sprintf("Модальность канала %s — %s.", envelope.Channel, ModalityOfChannel(envelope.Channel))}
	}
	if envelope.ProviderTimestamp.IsZero() {
		return envelope, EnvelopeError{Field: "provider_timestamp", Reason: "Требуется время провайдера (RFC3339)."}
	}
	if envelope.ProviderTimestamp.After(receivedAt.Add(maxProviderClockSkew)) {
		return envelope, EnvelopeError{Field: "provider_timestamp", Reason: "Время провайдера слишком далеко в будущем."}
	}
	if len(envelope.Payload) == 0 {
		return envelope, EnvelopeError{Field: "payload", Reason: "Событие без материального payload не принимается."}
	}
	envelope.PayloadDigest = PayloadDigest(envelope.Payload)
	return envelope, nil
}

func validateEnvelopeToken(field, value string, maxRunes int) *EnvelopeError {
	if value == "" {
		return &EnvelopeError{Field: field, Reason: "Поле обязательно."}
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return &EnvelopeError{Field: field, Reason: "Допустимы только видимые ASCII-символы без пробелов."}
		}
	}
	if len(value) > maxRunes {
		return &EnvelopeError{Field: field, Reason: fmt.Sprintf("Максимум %d символов.", maxRunes)}
	}
	return nil
}

// PayloadDigest is the sha256 of the canonical payload bytes — the material
// identity of the delivered content.
func PayloadDigest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Source verification (the port result — the ladder input)
// ---------------------------------------------------------------------------

// SourceVerification is the outcome of the source check: either a verified
// connection snapshot (trusted metadata for the tenant scope) or the honest
// refusal kind that fired before any durable record.
type SourceVerification struct {
	Verified bool
	Kind     string
	Text     string
	Snapshot ConnectionSnapshot
}

// ConnectionSnapshot is the trusted metadata of the verified connection:
// the tenant the scope is built from, the connector identity and the
// connection state as read by the verifier.
type ConnectionSnapshot struct {
	TenantID      id.UUID
	ConnectionID  id.UUID
	ConnectorID   string
	ConnectorType string
	Environment   string
	Status        string
}
