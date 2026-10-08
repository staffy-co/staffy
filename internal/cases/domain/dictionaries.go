package domain

import "time"

// ---------------------------------------------------------------------------
// Fixed dictionaries of the ingress slice
// ---------------------------------------------------------------------------

// AllowedChannels is the profile channel dictionary (the Brief dictionary
// of Epic 1): nine channels, voice phone plus eight text channels.
var AllowedChannels = map[string]bool{
	"phone": true, "web_chat": true, "telegram": true, "whatsapp": true,
	"email": true, "max": true, "jivo": true, "avito": true, "mobile_app_chat": true,
}

// ChannelOrder is the canonical display order of the dictionary.
var ChannelOrder = []string{"phone", "web_chat", "telegram", "whatsapp", "email", "max", "jivo", "avito", "mobile_app_chat"}

// ModalityOfChannel derives the honest modality of a channel — the same
// rule the profile matrix enforces (a channel never claims the other
// modality).
func ModalityOfChannel(channel string) string {
	if channel == "phone" {
		return ModalityVoice
	}
	return ModalityText
}

// Connector type dictionary of the catalog: the ingress ladder maps every
// channel to the connector type that may legally serve it. phone is served
// by SIP/telephony connectors; the eight text channels by chat/inbox ones.
const (
	ConnectorTypeSIP  = "SIP/telephony"
	ConnectorTypeChat = "chat/inbox"
)

// RequiredConnectorType maps a channel to its only legal connector type.
// One admitted combination never admits the others.
func RequiredConnectorType(channel string) string {
	if channel == "phone" {
		return ConnectorTypeSIP
	}
	return ConnectorTypeChat
}

// Capture policy dictionary of the pre-case session record: the slice
// records phone sessions as metadata only (media edge does not exist,
// AC-7); text sessions carry sealed payload references. The dictionary is
// fixed — an undeclared capture policy is impossible by construction.
const (
	CapturePolicyMetadataOnly  = "metadata_only"
	CapturePolicySealedPayload = "sealed_payload"
)

// CapturePolicyOfModality declares the capture policy of a session by the
// modality of its events.
func CapturePolicyOfModality(modality string) string {
	if modality == ModalityVoice {
		return CapturePolicyMetadataOnly
	}
	return CapturePolicySealedPayload
}

// ---------------------------------------------------------------------------
// Load envelope: the fixed conservative dictionary
// ---------------------------------------------------------------------------

// LoadEnvelope is the fixed conservative admission envelope of the domain.
// No per-tenant configuration exists: until AG-1/AG-4-proven values exist
// the dictionary stays hardcoded, and an overrun is an honest bounded
// refusal with a counter — never a silent loss.
type LoadEnvelope struct {
	MaxEnvelopeBytes         int
	MaxPayloadBytes          int
	MaxInflightPerConnection int
	InflightWindow           time.Duration
}

// FixedLoadEnvelope is the only envelope of this build.
var FixedLoadEnvelope = LoadEnvelope{
	MaxEnvelopeBytes:         256 * 1024,
	MaxPayloadBytes:          128 * 1024,
	MaxInflightPerConnection: 1000,
	InflightWindow:           time.Hour,
}

// The bound of pre-verification rejection rows: an unauthenticated machine
// endpoint may honestly record refusals of unverifiable deliveries, but
// never unboundedly — a fixed dictionary cap per (claimed tenant,
// connection, kind) inside the window keeps the durable noise bounded
// until a tenant-neutral area exists (deferred-work).
const (
	MaxClaimedRejectionsPerKey = 100
	RejectionBoundWindow       = time.Hour
)

// ---------------------------------------------------------------------------
// Refusal kinds (the vocabulary of every honest refusal)
// ---------------------------------------------------------------------------

// Refusal kinds: the fixed dictionary of the ingress ladder. Every refusal
// carries one of these kinds, a safe text and the correlation ID; the
// payload never travels with a refusal.
const (
	KindMalformed           = "malformed"
	KindUnknownConnection   = "unknown_connection"
	KindRevokedConnection   = "revoked_connection"
	KindProductionForbidden = "production_forbidden"
	KindChannelMismatch     = "channel_mismatch"
	KindRouteNotAdmitted    = "route_not_admitted"
	KindNoActiveRelease     = "no_active_release"
	KindAmbiguousEmployee   = "ambiguous_employee"
	KindNoResponseOwner     = "no_response_owner"
	KindEntitlementBlocked  = "entitlement_blocked"
	KindEnvelopeExceeded    = "envelope_exceeded"
)

// RefusalKinds is the full dictionary (order is the ladder order).
var RefusalKinds = []string{
	KindMalformed, KindUnknownConnection, KindRevokedConnection, KindProductionForbidden,
	KindChannelMismatch, KindRouteNotAdmitted, KindNoActiveRelease, KindAmbiguousEmployee,
	KindNoResponseOwner, KindEntitlementBlocked, KindEnvelopeExceeded,
}

// RejectionKinds are the kinds an ingress_rejections row may carry: every
// dictionary kind except ambiguous_employee — ambiguity quarantines the
// event and raises an operational exception for the human instead.
var RejectionKinds = []string{
	KindMalformed, KindUnknownConnection, KindRevokedConnection, KindProductionForbidden,
	KindChannelMismatch, KindRouteNotAdmitted, KindNoActiveRelease,
	KindNoResponseOwner, KindEntitlementBlocked, KindEnvelopeExceeded,
}

// BoundedKinds are the bounded outcomes: honest capacity refusals with a
// counter that never silently drop an event.
var BoundedKinds = []string{KindEntitlementBlocked, KindEnvelopeExceeded}

// IsBoundedKind reports whether a refusal kind is a bounded outcome.
func IsBoundedKind(kind string) bool {
	return kind == KindEntitlementBlocked || kind == KindEnvelopeExceeded
}

// ---------------------------------------------------------------------------
// Case contour state dictionaries (first slice)
// ---------------------------------------------------------------------------

// The first slice fixes the honest single values: a case opens, a
// conversation is active, the single work item state is untriaged, the only
// writable surface is the inbox and the owner epoch is 1. Later stories
// grow these dictionaries; this slice makes any other value impossible.
const (
	CaseStateOpen           = "open"
	ConversationStateActive = "active"
	WorkItemStateUntriaged  = "untriaged"
	WritableSurfaceInbox    = "inbox"
	ResponseOwnerEpochFirst = 1
)

// Operational exception kinds (the human queue of the contour).
const (
	ExceptionDigestConflict      = "digest_conflict"
	ExceptionAmbiguousRouting    = "ambiguous_employee"
	ExceptionIdempotencyConflict = "idempotency_key_conflict"
)

// Outbox event types of the slice (each with its schema version).
const (
	OutboxEventAccepted       = "case.event_accepted"
	OutboxEventAcceptedV1     = 1
	OutboxEventConnectorAck   = "case.connector_ack"
	OutboxEventConnectorAckV1 = 1
)

// Delivery outcomes of the outbox drain: the slice has no background
// workers and no provider adapters, so the honest outcome defers the
// delivery instead of faking it.
const (
	DeliveryOutcomeDeferred = "deferred"
	DeliveryDeferReason     = "Нет фоновых воркеров и провайдерских адаптеров (AG-4): доставка отложена; исход записи append-only."
)
