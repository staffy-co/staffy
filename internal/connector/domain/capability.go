package domain

import (
        "fmt"
        "time"

        "github.com/artemiosu/staffy/internal/platform/id"
)

// Capability direction: which side initiates the operation.
type CapabilityDirection string

const (
        DirectionOutbound CapabilityDirection = "outbound"
        DirectionInbound  CapabilityDirection = "inbound"
)

// CapabilityOperation is the declared operation kind.
// Only query capabilities may be probe-verified in this slice: commands mutate
// external state and stay blocked until the Action Gateway exists (AG-4).
type CapabilityOperation string

const (
        OperationQuery   CapabilityOperation = "query"
        OperationCommand CapabilityOperation = "command"
        OperationEvent   CapabilityOperation = "event"
)

// DeliveryLevel separates API acceptance from committed business results.
type DeliveryLevel string

const (
        DeliveryAccepted  DeliveryLevel = "accepted"
        DeliveryDelivered DeliveryLevel = "delivered"
        DeliveryCommitted DeliveryLevel = "committed"
        DeliveryUnknown   DeliveryLevel = "unknown"
)

// Derived capability states. They are never stored: the state is a pure
// function of facts (declared contract + evidence + connection facts + now).
const (
        CapStateUnverified  = "unverified"
        CapStateVerifying   = "verifying"
        CapStateVerified    = "verified"
        CapStateUnsupported = "unsupported"
        CapStateFailed      = "failed"
        CapStateUnknown     = "unknown"
        CapStateStale       = "stale"
)

// Immutable evidence outcomes (persisted rows).
const (
        EvidenceVerified = "verified"
        EvidenceFailed   = "failed"
        EvidenceUnknown  = "unknown"
)

// EvidenceKind marks the safe read-only probe that produced the evidence.
const EvidenceProbeKind = "read_only_probe"

// CapabilityBlock is the honest, managed block of a mutating capability:
// it names the required architectural capability, the gate and the owner
// instead of silently hiding the missing function.
type CapabilityBlock struct {
        Required string `json:"required"` // e.g. "Action Gateway (Epic 3)"
        Gate     string `json:"gate"`     // e.g. "AG-4"
        Owner    string `json:"owner"`    // e.g. "Technical Lead + Security"
}

// ActionGatewayBlock is the single managed block used by mutating capabilities.
var ActionGatewayBlock = CapabilityBlock{
        Required: "Action Gateway (Epic 3)",
        Gate:     "AG-4",
        Owner:    "Technical Lead + Security",
}

// CapabilityDeclaration is the operational semantics contract of one adapter
// capability. Every field is mandatory: a declaration missing semantics is
// rendered `unsupported`, never inferred from another provider's behavior.
type CapabilityDeclaration struct {
        ID                  string              `json:"id"`
        Title               string              `json:"title"`
        Description         string              `json:"description"`
        Direction           CapabilityDirection `json:"direction"`
        Operation           CapabilityOperation `json:"operation"`
        TimeoutMS           int                 `json:"timeout_ms"`
        Ordering            string              `json:"ordering"`
        RateLimit           string              `json:"rate_limit"`
        SizeLimit           string              `json:"size_limit"`
        Idempotent          bool                `json:"idempotent"`
        Receipt             string              `json:"receipt"`
        ReceiptLevel        DeliveryLevel       `json:"receipt_level"`
        CancelCompensation  string              `json:"cancel_compensation"`
        Reconciliation      string              `json:"reconciliation"`
        ReconciliationOwner string              `json:"reconciliation_owner"`
        ProbeAllowed        bool                `json:"probe_allowed"`
}

// Complete reports whether every mandatory semantic is declared. Missing
// semantics means the capability is shown as `unsupported` (fail-closed).
func (d CapabilityDeclaration) Complete() bool {
        switch {
        case d.ID == "" || d.Title == "":
                return false
        case d.Direction != DirectionInbound && d.Direction != DirectionOutbound:
                return false
        case d.Operation != OperationQuery && d.Operation != OperationCommand && d.Operation != OperationEvent:
                return false
        case d.TimeoutMS <= 0:
                return false
        case d.Ordering == "" || d.RateLimit == "" || d.SizeLimit == "":
                return false
        case d.Receipt == "":
                return false
        case d.ReceiptLevel != DeliveryAccepted && d.ReceiptLevel != DeliveryDelivered && d.ReceiptLevel != DeliveryCommitted:
                return false
        case d.CancelCompensation == "" || d.Reconciliation == "" || d.ReconciliationOwner == "":
                return false
        }
        return true
}

// Mutating reports whether exercising this capability would effect external
// state: such capabilities are never probe-verified (Action Gateway, AG-4).
func (d CapabilityDeclaration) Mutating() bool {
        return d.Operation == OperationCommand
}

// ProbeVerifiable reports whether this declared capability may be exercised by
// the safe read-only synthetic probe in this slice.
func (d CapabilityDeclaration) ProbeVerifiable() bool {
        return d.Complete() && d.Operation == OperationQuery && d.Direction == DirectionOutbound && d.ProbeAllowed
}

// CapabilityEvidence is one immutable (append-only) proof row: it anchors a
// read-only probe result to a specific connection, secret, environment,
// contract version and adapter build. It never stores secret material or raw
// provider responses: only fixed redacted summaries.
type CapabilityEvidence struct {
        TenantID         id.UUID     `json:"tenant_id"`
        EvidenceID       id.UUID     `json:"evidence_id"`
        ConnectionID     id.UUID     `json:"connection_id"`
        SecretID         id.UUID     `json:"secret_id"`
        Environment      Environment `json:"environment"`
        ContractVersion  string      `json:"contract_version"`
        AdapterBuild     string      `json:"adapter_build"`
        Capability       string      `json:"capability"`
        ProbeKind        string      `json:"probe_kind"`
        ActorID          id.UUID     `json:"actor_id"`
        RequestedAt      time.Time   `json:"requested_at"`
        DeadlineAt       time.Time   `json:"deadline_at"`
        CompletedAt      time.Time   `json:"completed_at"`
        Outcome          string      `json:"outcome"` // verified / failed / unknown
        DeliveryLevel    string      `json:"delivery_level"`
        RedactedRequest  string      `json:"redacted_request"`
        RedactedResult   string      `json:"redacted_result"`
        ExpiresAt        time.Time   `json:"expires_at"`
        IdempotencyKey   string      `json:"-"`
        RequestHash      string      `json:"-"`
}

// CapabilityEvidenceSummary is the safe, published projection of evidence for
// the UI/API progressive disclosure.
type CapabilityEvidenceSummary struct {
        EvidenceID      id.UUID   `json:"evidence_id"`
        Outcome         string    `json:"outcome"`
        DeliveryLevel   string    `json:"delivery_level"`
        ProbeKind       string    `json:"probe_kind"`
        CheckedAt       time.Time `json:"checked_at"`
        ExpiresAt       time.Time `json:"expires_at"`
        RedactedRequest string    `json:"redacted_request"`
        RedactedResult  string    `json:"redacted_result"`
}

// Summary projects the evidence into its published (safe) shape.
func (e CapabilityEvidence) Summary() CapabilityEvidenceSummary {
        return CapabilityEvidenceSummary{
                EvidenceID:      e.EvidenceID,
                Outcome:         e.Outcome,
                DeliveryLevel:   e.DeliveryLevel,
                ProbeKind:       e.ProbeKind,
                CheckedAt:       e.CompletedAt,
                ExpiresAt:       e.ExpiresAt,
                RedactedRequest: e.RedactedRequest,
                RedactedResult:  e.RedactedResult,
        }
}

// Applicable reports whether the evidence still proves anything about the
// connection as it stands right now. All anchors must match: connection,
// active secret, environment, pinned contract version, adapter build, the
// connection's last change time, and the evidence TTL.
func (e CapabilityEvidence) Applicable(conn Connection, adapterBuild string, now time.Time) bool {
        switch {
        case e.ConnectionID != conn.ConnectionID:
                return false
        case !conn.SecretActive || e.SecretID != conn.SecretID:
                return false
        case e.Environment != conn.Environment:
                return false
        case e.ContractVersion != conn.ConnectorVer:
                return false
        case e.AdapterBuild != adapterBuild:
                return false
        case conn.UpdatedAt.After(e.RequestedAt):
                // The connection changed (config save, rotate, revoke, re-check) after
                // this evidence was produced: it no longer describes current facts.
                return false
        case !e.ExpiresAt.After(now):
                return false
        }
        return true
}

// CapabilityReport is the derived state of one declared capability for one
// connection: what the adapter claims vs what is actually proven for the
// account, with the reason text and the evidence for disclosure.
type CapabilityReport struct {
        CapabilityDeclaration
        State       string                        `json:"state"`
        StateReason string                        `json:"state_reason"`
        CheckedAt   *time.Time                    `json:"checked_at,omitempty"`
        ExpiresAt   *time.Time                    `json:"expires_at,omitempty"`
        Evidence    *CapabilityEvidenceSummary    `json:"evidence,omitempty"`
        Block       *CapabilityBlock              `json:"block,omitempty"`
        CanVerify   bool                          `json:"can_verify"`
}

// DeriveCapabilityStates computes the honest state of every declared
// capability from facts only. There is no second writer of states, so a
// desync is impossible by construction. Evidence that no longer applies is
// reported as `stale` with the concrete reason, never as success.
func DeriveCapabilityStates(def ConnectorDefinition, conn Connection, evidence []CapabilityEvidence, adapterBuild string, now time.Time) []CapabilityReport {
        reports := make([]CapabilityReport, 0, len(def.Capabilities))
        for _, dec := range def.Capabilities {
                report := CapabilityReport{CapabilityDeclaration: dec, State: CapStateUnverified, StateReason: "Проверка ещё не выполнялась."}
                switch {
                case !dec.Complete():
                        report.State = CapStateUnsupported
                        report.StateReason = "Контракт не декларирует обязательную операционную семантику этой capability; состояние показывается как unsupported и не выводится из поведения других провайдеров."
                case dec.Mutating():
                        report.State = CapStateUnverified
                        block := ActionGatewayBlock
                        report.Block = &block
                        report.StateReason = "Мутирующая capability: тестовое выполнение запрещено архитектурой до Action Gateway. Блокировка не мешает независимо доказанным read-only возможностям этого подключения."
                case dec.Operation == OperationEvent:
                        report.State = CapStateUnverified
                        report.StateReason = "Входящие события: путь проверки не реализован в текущем срезе — честное состояние без ложного успеха."
                case conn.Status != StatusConnectedUnverified:
                        report.State = CapStateUnverified
                        report.StateReason = connectionGuardReason(conn)
                default:
                        latest := latestEvidence(evidence, dec.ID)
                        if latest == nil {
                                report.CanVerify = dec.ProbeVerifiable()
                        } else {
                                checked := latest.CompletedAt
                                expires := latest.ExpiresAt
                                report.CheckedAt = &checked
                                report.ExpiresAt = &expires
                                summary := latest.Summary()
                                report.Evidence = &summary
                                if latest.Applicable(conn, adapterBuild, now) {
                                        report.State = latest.Outcome
                                        report.StateReason = outcomeReason(dec, *latest)
                                        report.CanVerify = dec.ProbeVerifiable()
                                } else {
                                        report.State = CapStateStale
                                        report.StateReason = staleReason(*latest, conn, adapterBuild, now)
                                        report.CanVerify = dec.ProbeVerifiable()
                                }
                        }
                }
                reports = append(reports, report)
        }
        return reports
}

// latestEvidence picks the most recent evidence row for one capability.
func latestEvidence(evidence []CapabilityEvidence, capability string) *CapabilityEvidence {
        var latest *CapabilityEvidence
        for i := range evidence {
                e := &evidence[i]
                if e.Capability != capability {
                        continue
                }
                if latest == nil || e.CompletedAt.After(latest.CompletedAt) || (e.CompletedAt.Equal(latest.CompletedAt) && e.EvidenceID.String() > latest.EvidenceID.String()) {
                        latest = e
                }
        }
        return latest
}

// connectionGuardReason explains why capabilities cannot be claimed at all
// while the connection itself is not in the connected/unverified state.
func connectionGuardReason(conn Connection) string {
        switch conn.Status {
        case StatusDraft:
                return "Подключение в черновике: capabilities не заявлены до повторной базовой проверки (семантика Story 1-8)."
        case StatusFailed:
                return "Базовая проверка не пройдена: capabilities не заявлены; повторите подключение секрета."
        case StatusRevoked:
                return "Подключение отозвано: capabilities неприменимы; история доказательств сохранена."
        }
        return "Состояние подключения не допускает проверку capabilities."
}

// outcomeReason maps an evidence outcome to its honest explanation. The
// reconciliation owner comes from the capability declaration so the unknown
// case always names a responsible human and a safe next step.
func outcomeReason(dec CapabilityDeclaration, e CapabilityEvidence) string {
        owner := dec.ReconciliationOwner
        if owner == "" {
                owner = "Technical Lead"
        }
        switch e.Outcome {
        case EvidenceVerified:
                return "Доказано синтетическим read-only probe для этого аккаунта: уровень доставки " + e.DeliveryLevel + " (принятие запроса не является бизнес-результатом)."
        case EvidenceFailed:
                return "Read-only probe не прошёл для этого аккаунта: возможность не подтверждена."
        case EvidenceUnknown:
                return "Probe не завершился до deadline: результат неопределён (возможен эффект). Повтор вслепую исключён; владелец reconciliation: " + owner + ". Безопасный следующий шаг: проверьте статус вручную и повторите проверку осознанно."
        }
        return e.Outcome
}

// staleReason names the concrete fact that invalidated the evidence.
func staleReason(e CapabilityEvidence, conn Connection, adapterBuild string, now time.Time) string {
        switch {
        case conn.Status == StatusRevoked:
                return "Доказательство устарело: подключение отозвано."
        case !conn.SecretActive || e.SecretID != conn.SecretID:
                return "Доказательство устарело: секрет ротирован или отозван — доказательство не переносится на новый секрет."
        case conn.UpdatedAt.After(e.RequestedAt):
                return "Доказательство устарело: конфигурация или состояние подключения изменились после проверки."
        case e.ContractVersion != conn.ConnectorVer:
                return "Доказательство устарело: версия контракта подключения изменилась."
        case e.AdapterBuild != adapterBuild:
                return "Доказательство устарело: сборка адаптера изменилась."
        case !e.ExpiresAt.After(now):
                return fmt.Sprintf("Доказательство устарело: срок действия истёк (%s); требуется повторная проверка.", e.ExpiresAt.Format("2006-01-02 15:04 UTC"))
        }
        return "Доказательство не применимо к текущим фактам подключения."
}
