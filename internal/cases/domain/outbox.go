package domain

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Payload sealing (the knowledge-source precedent of Story 1-5)
// ---------------------------------------------------------------------------

// NewSealKey generates the random per-event seal key. The key never leaves
// the store: the outward surface of a payload is its digest only.
func NewSealKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// PayloadSeal is the HMAC-SHA256 print of the canonical payload bytes under
// the seal key — the read-time integrity guard of the sealed blob.
func PayloadSeal(key, payload []byte) string {
	if len(key) < 32 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// CanonicalPayload compacts the decoded provider payload into deterministic
// bytes: object keys are sorted by the encoder, so the same material
// content always yields the same digest regardless of the wire formatting.
func CanonicalPayload(raw json.RawMessage) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("payload не является JSON-значением: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("payload должен быть одним JSON-значением")
	}
	return json.Marshal(value)
}

// ---------------------------------------------------------------------------
// Outbox records (the first slice that actually executes the rule)
// ---------------------------------------------------------------------------

// OutboxEvent is one durable outbox record: the event type with its schema
// version, the aggregate, the canonical safe JSON (never the raw payload)
// and the dedup key. The initiating module commits the domain state and
// the outbox record in one transaction — the architecture rule claimed
// since the identity migration and first executed here.
type OutboxEvent struct {
	TenantID      id.UUID
	EventID       id.UUID
	EventType     string
	SchemaVersion int
	AggregateID   id.UUID
	CorrelationID id.UUID
	OccurredAt    time.Time
	Payload       []byte
	DedupKey      string
}

// AcceptedEventPayload is the safe canonical JSON of the accepted-event
// outbox record: identifiers, routes and digests only — the sealed payload
// content never enters the outbox.
type AcceptedEventPayload struct {
	TenantID          id.UUID `json:"tenant_id"`
	CaseID            id.UUID `json:"case_id"`
	ConversationID    id.UUID `json:"conversation_id"`
	EventID           id.UUID `json:"event_id"`
	ConnectionID      id.UUID `json:"connection_id"`
	Provider          string  `json:"provider"`
	ProviderEventID   string  `json:"provider_event_id"`
	ProviderSessionID string  `json:"provider_session_id"`
	Channel           string  `json:"channel"`
	Modality          string  `json:"modality"`
	Direction         string  `json:"direction"`
	PayloadDigest     string  `json:"payload_digest"`
	NewCase           bool    `json:"new_case"`
	Replay            bool    `json:"replay"`
}

// ConnectorAckPayload is the safe canonical JSON of the connector-ack
// outbox record: the ack is recorded by the connector contract and its real
// delivery is honestly deferred (no workers, no adapters — AG-4).
type ConnectorAckPayload struct {
	TenantID        id.UUID `json:"tenant_id"`
	EventID         id.UUID `json:"event_id"`
	ConnectionID    id.UUID `json:"connection_id"`
	Provider        string  `json:"provider"`
	ProviderEventID string  `json:"provider_event_id"`
	AckContract     string  `json:"ack_contract"`
	Deferred        bool    `json:"deferred"`
}

// AckContractSynthetic is the connector contract marker of the synthetic
// telephony connector of the test contour.
const AckContractSynthetic = "staffy-synthetic-telephony/ack-v1"

// DedupKeys of the two outbox records of one accepted event.
func DedupKeyAccepted(eventID id.UUID) string {
	return "case.event_accepted:" + eventID.String()
}

func DedupKeyConnectorAck(eventID id.UUID) string {
	return "case.connector_ack:" + eventID.String()
}

// OutboxDelivery is one append-only delivery outcome: the drain command
// records outcomes without ever updating the outbox record (the history is
// not edited; a repeated drain finds the record delivered and does
// nothing).
type OutboxDelivery struct {
	TenantID   id.UUID
	DeliveryID id.UUID
	EventID    id.UUID
	Attempt    int64
	Outcome    string
	Details    string
	CreatedAt  time.Time
}

// AcceptedEventPayloadJSON renders the safe canonical JSON of the
// accepted-event outbox record.
func AcceptedEventPayloadJSON(payload AcceptedEventPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ConnectorAckPayloadJSON renders the safe canonical JSON of the
// connector-ack outbox record.
func ConnectorAckPayloadJSON(payload ConnectorAckPayload) ([]byte, error) {
	return json.Marshal(payload)
}
