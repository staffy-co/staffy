package domain

import "fmt"

// ArtifactClass is one entry of the fixed system dictionary of data classes.
// The dictionary is synchronized with the physical tables created by the
// migrations: the perimeter integrity test fails when a table exists without
// a claiming class or a class references a table that does not exist.
// Unregistered data is impossible by construction.
type ArtifactClass string

const (
	ClassTenantConfiguration    ArtifactClass = "tenant-configuration"
	ClassMemberIdentity         ArtifactClass = "member-identity"
	ClassEmployeeDraft          ArtifactClass = "employee-draft"
	ClassDomainPackPin          ArtifactClass = "domain-pack-pin"
	ClassKnowledgeSourcePayload ArtifactClass = "knowledge-source-payload"
	ClassBehaviorConfiguration  ArtifactClass = "behavior-configuration"
	ClassVoiceConfiguration     ArtifactClass = "voice-configuration"
	ClassVoicePreviewMedia      ArtifactClass = "voice-preview-media"
	ClassVoiceAssessment        ArtifactClass = "voice-assessment"
	ClassReleaseSnapshot        ArtifactClass = "release-snapshot"
	ClassConnectionConfig       ArtifactClass = "connection-configuration"
	ClassConnectionSecretSeal   ArtifactClass = "connection-secret-seal"
	ClassCapabilityEvidence     ArtifactClass = "capability-evidence"
	ClassIdempotencyRecords     ArtifactClass = "idempotency-records"
	ClassAuditTrail             ArtifactClass = "audit-trail"
	ClassOutboxEvents           ArtifactClass = "outbox-events"
)

// Executor names the owning module that executes local deletion for a class.
// Empty means: no local executor exists in this slice.
const (
	ExecutorEmployee = "employee"
	ExecutorIdentity = "identity"
)

// ManagedBlock is the honest, managed block of a deletion route: it names the
// required step, the gate and the responsible owner instead of silently
// hiding the missing function (the same shape as the connector 1.9 block).
type ManagedBlock struct {
	Required string `json:"required"`
	Gate     string `json:"gate"`
	Owner    string `json:"owner"`
}

// AG4SecretStoreBlock is the single managed block of the sealed-secret class:
// physical destruction and an external SecretStore are architectural steps
// behind gate AG-4 and are deliberately not touched by this slice.
var AG4SecretStoreBlock = ManagedBlock{
	Required: "Внешний SecretStore и физическое уничтожение запечатанных секретов",
	Gate:     "AG-4",
	Owner:    "Technical Lead + Security",
}

// ImmutableEvidenceBlock blocks deletion of append-only evidence rows.
var ImmutableEvidenceBlock = ManagedBlock{
	Required: "Immutable integrity: доказательства подключений не перезаписываются и не удаляются точечно",
	Gate:     "-",
	Owner:    "Technical Lead",
}

// AuditMaintenanceBlock names the dedicated procedure that audit deletion
// requires once the minimal retention term has passed.
var AuditMaintenanceBlock = ManagedBlock{
	Required: "Выделенная maintenance-процедура удаления журнала аудита",
	Gate:     "-",
	Owner:    "Technical Lead",
}

// OffboardingBlock names the member lifecycle flow that active membership
// deletion requires; it is a future slice, so the class route stays blocked.
var OffboardingBlock = ManagedBlock{
	Required: "Offboarding/DSAR-поток для участников (Epic 2+)",
	Gate:     "-",
	Owner:    "Product Owner + Technical Lead",
}

// ClassSpec describes one class of the dictionary: its title, the physical
// tables it claims and its deletion execution profile for this slice.
type ClassSpec struct {
	ID      ArtifactClass
	Title   string
	Tables  []string
	// Executor is the owning-module port that may delete local copies.
	// Empty string means there is no local executor in this slice.
	Executor string
	// Block is set for classes whose deletion route is deliberately blocked
	// with a managed reason (seal, evidence).
	Block *ManagedBlock
	// AuditMinTerm marks the audit class: deletion before the policy minimal
	// term yields retained_by_law; after the term the route is blocked on the
	// dedicated maintenance procedure.
	AuditMinTerm bool
}

var classRegistry = []ClassSpec{
	{ID: ClassTenantConfiguration, Title: "Настройка пространства (tenant)", Tables: []string{"identity.tenants"}},
	{ID: ClassMemberIdentity, Title: "Личности участников и приглашения", Tables: []string{"identity.principals", "identity.memberships", "identity.invitations"}, Executor: ExecutorIdentity},
	{ID: ClassEmployeeDraft, Title: "Черновики ИИ-сотрудников", Tables: []string{"employee.drafts"}, Executor: ExecutorEmployee},
	{ID: ClassDomainPackPin, Title: "Закреплённые domain-pack", Tables: []string{"employee.drafts"}},
	{ID: ClassKnowledgeSourcePayload, Title: "Источники знаний и их содержимое", Tables: []string{"employee.knowledge_sources", "employee.knowledge_versions", "employee.knowledge_chunks"}, Executor: ExecutorEmployee},
	{ID: ClassBehaviorConfiguration, Title: "Конфигурации поведения", Tables: []string{"employee.behavior_versions"}},
	{ID: ClassVoiceConfiguration, Title: "Конфигурации голоса", Tables: []string{"employee.voice_versions"}},
	{ID: ClassVoicePreviewMedia, Title: "Предпрослушивания голоса (24-часовой инвариант)", Tables: []string{"employee.voice_previews"}, Executor: ExecutorEmployee},
	{ID: ClassVoiceAssessment, Title: "Оценки предпрослушиваний", Tables: []string{"employee.voice_assessments"}, Executor: ExecutorEmployee},
	{ID: ClassReleaseSnapshot, Title: "Версии конфигурации и Release Manifest", Tables: []string{"employee.release_drafts", "employee.config_versions", "employee.release_manifests"}},
	{ID: ClassConnectionConfig, Title: "Конфигурации подключений", Tables: []string{"connector.connections", "connector.connection_versions"}},
	{ID: ClassConnectionSecretSeal, Title: "Запечатанные секреты подключений", Tables: []string{"connector.secret_seals"}, Block: &AG4SecretStoreBlock},
	{ID: ClassCapabilityEvidence, Title: "Доказательства capabilities", Tables: []string{"connector.capability_evidence"}, Block: &ImmutableEvidenceBlock},
	{ID: ClassIdempotencyRecords, Title: "Идемпотентные записи команд", Tables: []string{"identity.idempotency_records", "employee.idempotency_records", "connector.idempotency_records"}},
	{ID: ClassAuditTrail, Title: "Журналы аудита", Tables: []string{"identity.membership_audit", "employee.audit", "employee.knowledge_audit", "connector.audit"}, AuditMinTerm: true},
	{ID: ClassOutboxEvents, Title: "События outbox", Tables: []string{"identity.outbox_events", "employee.outbox_events", "employee.knowledge_outbox", "connector.outbox_events"}},
}

var classIndex = func() map[ArtifactClass]ClassSpec {
	index := make(map[ArtifactClass]ClassSpec, len(classRegistry))
	for _, spec := range classRegistry {
		index[spec.ID] = spec
	}
	return index
}()

// AllClasses returns the ordered system dictionary.
func AllClasses() []ArtifactClass {
	out := make([]ArtifactClass, 0, len(classRegistry))
	for _, spec := range classRegistry {
		out = append(out, spec.ID)
	}
	return out
}

// ClassSpecs returns the ordered registry itself.
func ClassSpecs() []ClassSpec {
	return append([]ClassSpec(nil), classRegistry...)
}

// LookupClass resolves one dictionary entry.
func LookupClass(class ArtifactClass) (ClassSpec, error) {
	spec, ok := classIndex[class]
	if !ok {
		return ClassSpec{}, fmt.Errorf("%w: %s", ErrUnknownClass, class)
	}
	return spec, nil
}

// TableOwners maps every physical table claimed by the dictionary to its
// claiming classes. A table may be shared by several classes (co-located
// data), but every migration table must be claimed by at least one class.
func TableOwners() map[string][]ArtifactClass {
	out := map[string][]ArtifactClass{}
	for _, spec := range classRegistry {
		for _, table := range spec.Tables {
			out[table] = append(out[table], spec.ID)
		}
	}
	return out
}
