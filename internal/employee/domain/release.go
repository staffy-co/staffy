package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Errors of the release surface. Authorization failures of members surface
// as ErrForbidden (an honest refusal); unknown employees/tenants as
// ErrNotFound; optimistic-concurrency losses as ReleaseConflictError, which
// carries the current state for an honest comparison.
var (
	ErrReleaseForbidden = errors.New("principal is not allowed on the release surface")
	ErrManifestExists   = errors.New("a manifest is already sealed for this configuration version")
)

// ReleaseRuleError is a named validation rejection with an honest reason.
type ReleaseRuleError struct {
	Rule   string
	Reason string
}

func (e ReleaseRuleError) Error() string { return e.Rule + ": " + e.Reason }
func (e ReleaseRuleError) Unwrap() error { return ErrInvalid }

// ReleaseConflictError is the honest optimistic-concurrency loss: it carries
// the expected and the current draft revision plus the current refs so the
// caller can compare instead of blindly retrying.
type ReleaseConflictError struct {
	Expected int64
	Current  int64
	Refs     ConfigRefs
}

func (e ReleaseConflictError) Error() string {
	return fmt.Sprintf("release draft revision conflict: expected %d, current %d", e.Expected, e.Current)
}
func (e ReleaseConflictError) Unwrap() error { return ErrConflict }

// Manifest statuses. `candidate` is derived on a complete component set;
// any gap — a missing component, an unproven capability — makes the sealed
// manifest honestly `incomplete`.
const (
	ManifestStatusCandidate  = "candidate"
	ManifestStatusIncomplete = "incomplete"
)

// Component reference statuses. A missing component is a recorded absence
// with a reason, never an empty value that silently passes for presence.
const (
	RefPresent       = "present"
	RefAbsent        = "absent"
	RefNotApplicable = "not_applicable"
)

// ManifestSchemaCurrent is the schema version written by this build.
const ManifestSchemaCurrent = 1

// manifestSchemaReadable lists the versions this build reads compatibly: the
// current one and, once a real schema change lands, the immediately previous
// one. Anything else is an explicit quarantine, never a silent drop.
var manifestSchemaReadable = map[int]bool{ManifestSchemaCurrent: true}

// Quarantine kinds of a manifest read.
const (
	QuarantineNone           = ""
	QuarantineSchemaVersion  = "schema_version"
	QuarantineUnknownField   = "unknown_field"
	QuarantineDigestMismatch = "digest_mismatch"
)

// ---------------------------------------------------------------------------
// Configuration component references
// ---------------------------------------------------------------------------

// ConfigRefs is the honest snapshot of the exact component versions a
// configuration version pins. Every component carries a status: present,
// absent (with a reason) or not applicable (with a reason).
type ConfigRefs struct {
	Brief     BriefRef     `json:"brief"`
	Pack      PackRef      `json:"domain_pack"`
	Behavior  BehaviorRef  `json:"behavior"`
	Knowledge KnowledgeRef `json:"knowledge"`
	Voice     VoiceRef     `json:"voice"`
}

type BriefRef struct {
	Status       string `json:"status"`
	DraftVersion int64  `json:"draft_version,omitempty"`
	Note         string `json:"note,omitempty"`
}

type PackRef struct {
	Status   string         `json:"status"`
	ID       string         `json:"id,omitempty"`
	Version  string         `json:"version,omitempty"`
	Variants map[string]any `json:"variants,omitempty"`
	Note     string         `json:"note,omitempty"`
}

type BehaviorRef struct {
	Status  string `json:"status"`
	Version int64  `json:"version,omitempty"`
	Note    string `json:"note,omitempty"`
}

type KnowledgeRef struct {
	Status  string               `json:"status"`
	Sources []KnowledgeSourceRef `json:"sources"`
	Note    string               `json:"note,omitempty"`
}

// KnowledgeSourceRef pins one knowledge source to its exact content version.
type KnowledgeSourceRef struct {
	SourceID id.UUID `json:"source_id"`
	Version  int64   `json:"version"`
}

type VoiceRef struct {
	Status  string `json:"status"`
	Version int64  `json:"version,omitempty"`
	Note    string `json:"note,omitempty"`
}

// ConfigFacts are the current employee facts the refs are assembled from.
type ConfigFacts struct {
	Brief        Brief
	DraftVersion int64
	Pack         *PackPin
	Behavior     []BehaviorVersion
	Knowledge    []KnowledgeSource
	Voice        []VoiceVersion
}

// BriefHasPhone reports whether the brief declares the phone channel: voice
// is a required component only in that case.
func BriefHasPhone(b Brief) bool {
	for _, channel := range b.DesiredChannels {
		if channel == "phone" {
			return true
		}
	}
	return false
}

// AssembleConfigRefs derives the honest reference snapshot from facts. The
// knowledge list only carries non-retired sources; the voice rule follows
// the brief channels.
func AssembleConfigRefs(facts ConfigFacts) ConfigRefs {
	refs := ConfigRefs{}
	refs.Brief = BriefRef{Status: RefPresent, DraftVersion: facts.DraftVersion}
	if facts.DraftVersion <= 0 {
		refs.Brief = BriefRef{Status: RefAbsent, Note: "Brief не сохранён: черновик сотрудника не существует."}
	}
	if facts.Pack != nil {
		refs.Pack = PackRef{Status: RefPresent, ID: facts.Pack.ID, Version: facts.Pack.Version, Variants: facts.Pack.Variants}
	} else {
		refs.Pack = PackRef{Status: RefAbsent, Note: "Domain Pack не закреплён: выберите пакет и Tenant-варианты."}
	}
	if version := latestBehaviorVersion(facts.Behavior); version > 0 {
		refs.Behavior = BehaviorRef{Status: RefPresent, Version: version}
	} else {
		refs.Behavior = BehaviorRef{Status: RefAbsent, Note: "Поведение не настроено: безопасная конфигурация поведения отсутствует."}
	}
	sources := []KnowledgeSourceRef{}
	for _, source := range facts.Knowledge {
		if source.Status == "retired" {
			continue
		}
		sources = append(sources, KnowledgeSourceRef{SourceID: source.SourceID, Version: source.CurrentVersion})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].SourceID.String() < sources[j].SourceID.String() })
	if len(sources) > 0 {
		refs.Knowledge = KnowledgeRef{Status: RefPresent, Sources: sources}
	} else {
		refs.Knowledge = KnowledgeRef{Status: RefAbsent, Sources: []KnowledgeSourceRef{}, Note: "Knowledge Sources не подключены: хотя бы один источник знаний требуется для полной конфигурации."}
	}
	switch {
	case !BriefHasPhone(facts.Brief):
		refs.Voice = VoiceRef{Status: RefNotApplicable, Note: "Канал phone не входит в Brief: голосовой профиль неприменим и не делает манифест неполным."}
	case latestVoiceVersion(facts.Voice) > 0:
		refs.Voice = VoiceRef{Status: RefPresent, Version: latestVoiceVersion(facts.Voice)}
	default:
		refs.Voice = VoiceRef{Status: RefAbsent, Note: "Голос не настроен, хотя Brief включает канал phone: настройте Voice Profile."}
	}
	return refs
}

func latestBehaviorVersion(versions []BehaviorVersion) int64 {
	var latest int64
	for _, v := range versions {
		if v.Version > latest {
			latest = v.Version
		}
	}
	return latest
}

func latestVoiceVersion(versions []VoiceVersion) int64 {
	var latest int64
	for _, v := range versions {
		if v.Version > latest {
			latest = v.Version
		}
	}
	return latest
}

// ---------------------------------------------------------------------------
// Draft, config version and manifest records
// ---------------------------------------------------------------------------

// ConfigVersionLink points at the configuration version a restored draft
// came from. The past version itself never changes.
type ConfigVersionLink struct {
	Version   int64     `json:"version"`
	VersionID id.UUID   `json:"version_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ReleaseDraft is the only mutable record of the release surface: a revision
// for optimistic concurrency plus the current refs. Everything else —
// versions, manifests — is append-only.
type ReleaseDraft struct {
	TenantID     id.UUID            `json:"tenant_id"`
	EmployeeID   id.UUID            `json:"employee_id"`
	Revision     int64              `json:"revision"`
	Refs         ConfigRefs         `json:"refs"`
	RestoredFrom *ConfigVersionLink `json:"restored_from,omitempty"`
	ActorID      id.UUID            `json:"actor_id"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// ConfigVersion is the immutable, append-only configuration snapshot.
type ConfigVersion struct {
	TenantID    id.UUID    `json:"tenant_id"`
	EmployeeID  id.UUID    `json:"employee_id"`
	VersionID   id.UUID    `json:"version_id"`
	Version     int64      `json:"version"`
	Description string     `json:"description"`
	Refs        ConfigRefs `json:"refs"`
	ActorID     id.UUID    `json:"actor_id"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ReleaseManifest is the sealed, immutable manifest record. The digest pins
// the canonical content; the status is derived before sealing and never
// rewritten afterwards.
type ReleaseManifest struct {
	TenantID      id.UUID         `json:"tenant_id"`
	EmployeeID    id.UUID         `json:"employee_id"`
	ManifestID    id.UUID         `json:"manifest_id"`
	ConfigVersion int64           `json:"config_version"`
	SchemaVersion int             `json:"schema_version"`
	Status        string          `json:"status"`
	Digest        string          `json:"digest"`
	ActorID       id.UUID         `json:"actor_id"`
	SealedAt      time.Time       `json:"sealed_at"`
	Content       ManifestContent `json:"content"`
}

// ---------------------------------------------------------------------------
// Runtime component dictionary
// ---------------------------------------------------------------------------

// RuntimeComponentSpec names one runtime component of the manifest. The
// dictionary is fixed in the domain so future surfaces fill it without a
// schema change.
type RuntimeComponentSpec struct {
	Component   string `json:"component"`
	Title       string `json:"title"`
	Owner       string `json:"owner"`
	Responsible string `json:"responsible"`
}

// RuntimeDictionary is the fixed order of runtime components.
var runtimeDictionary = []RuntimeComponentSpec{
	{Component: "model_route", Title: "Маршрут модели (model route)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "media_route", Title: "Маршрут медиа (media route)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "prompt_assembly", Title: "Сборка промптов (prompt assembly)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "tool_schemas", Title: "Схемы инструментов (tool schemas)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "policy_bundle", Title: "Пакет политик (policy bundle)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "eval_set", Title: "Набор eval (eval set)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "runtime_build", Title: "Сборка runtime (runtime build)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
	{Component: "autonomy_envelope", Title: "Огибающая автономности (autonomy envelope)", Owner: "AI Runtime (Epic 2)", Responsible: "Technical Lead"},
}

// RuntimeDictionary returns the fixed runtime component dictionary in order.
func RuntimeDictionary() []RuntimeComponentSpec {
	out := make([]RuntimeComponentSpec, len(runtimeDictionary))
	copy(out, runtimeDictionary)
	return out
}

// ToolBindingFact is the honest status of one tool contract bound by the
// pinned pack (Story 7-2): the contract id, the exact version and the
// adapter status resolved from the tool registry at sealing time. The
// facts are the INPUT of the derived tool_schemas state — the domain never
// touches the registry itself.
type ToolBindingFact struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	AdapterStatus string `json:"adapter_status"`
}

// RuntimeComponentState is the presence record of one runtime component in
// a manifest. In this slice every component is honestly absent.
type RuntimeComponentState struct {
	Component   string `json:"component"`
	Present     bool   `json:"present"`
	Ref         string `json:"ref,omitempty"`
	Owner       string `json:"owner"`
	Responsible string `json:"responsible"`
	Note        string `json:"note"`
}

// BuildRuntimeState lists every dictionary component honestly with the
// DERIVED tool_schemas state as the input (Story 7-2): the sealing caller
// computes the state from the pack pin's tool binding and passes it in
// explicitly — the static note is never the default path of a new seal.
// Every other component still carries the honest static gap note (its
// surface arrives with the AI Runtime epic).
func BuildRuntimeState(toolSchemas RuntimeComponentState) []RuntimeComponentState {
	out := make([]RuntimeComponentState, 0, len(runtimeDictionary))
	for _, spec := range runtimeDictionary {
		if spec.Component == "tool_schemas" {
			out = append(out, toolSchemas)
			continue
		}
		out = append(out, RuntimeComponentState{
			Component: spec.Component, Present: false,
			Owner: spec.Owner, Responsible: spec.Responsible,
			Note: "Поверхность компонента ещё не существует в системе: зафиксирован как явный gap; ответственный — " + spec.Responsible + ".",
		})
	}
	return out
}

// DeriveToolSchemasState is the pure derivation of the tool_schemas
// component state from the pinned pack's tool binding (Story 7-2): the
// state is DERIVED from the binding, never the static honest gap of the
// pre-registry era. Four honest branches: a pack not pinned (no binding
// possible), a pinned pack without a binding (nil), a binding whose
// adapters are all not_implemented (visible but not callable — Present
// stays false), and at least one implemented adapter (Present=true with
// the pack-pinned ref). Sealed history is never recomputed (AD-10): the
// derivation runs at new seals only.
func DeriveToolSchemasState(packRef PackRef, bindings []ToolBindingFact) RuntimeComponentState {
	spec := RuntimeComponentSpec{}
	for _, candidate := range runtimeDictionary {
		if candidate.Component == "tool_schemas" {
			spec = candidate
			break
		}
	}
	state := RuntimeComponentState{Component: spec.Component, Present: false, Owner: spec.Owner, Responsible: spec.Responsible}
	switch {
	case packRef.Status != RefPresent:
		state.Note = "Domain Pack не закреплён — tool-контракты не привязаны."
		return state
	case len(bindings) == 0:
		state.Note = "Domain Pack " + packRef.ID + "@" + packRef.Version + " не привязывает tool-контракты."
		return state
	}
	implemented := 0
	for _, binding := range bindings {
		if binding.AdapterStatus == "implemented" {
			implemented++
		}
	}
	if implemented == 0 {
		state.Note = fmt.Sprintf("Пак %s@%s привязывает %d tool-контракт(ов), адаптеры не реализованы — вызов невозможен.", packRef.ID, packRef.Version, len(bindings))
		return state
	}
	state.Present = true
	state.Ref = fmt.Sprintf("%s@%s: %d tool-контракт(ов)", packRef.ID, packRef.Version, len(bindings))
	state.Note = fmt.Sprintf("Пак %s@%s привязывает %d tool-контракт(ов), реализовано адаптеров: %d.", packRef.ID, packRef.Version, len(bindings), implemented)
	return state
}

// ---------------------------------------------------------------------------
// Manifest content, canonical digest and status
// ---------------------------------------------------------------------------

// ManifestContent is the canonical manifest payload: the configuration
// snapshot, the connection facts with capability evidence and gaps, and the
// runtime components. It never contains secrets, secret hints, raw PII,
// transcripts or protected media — only identifiers, versions, states and
// timestamps.
type ManifestContent struct {
	SchemaVersion int                     `json:"schema_version"`
	Config        ConfigSnapshot          `json:"configuration"`
	Connections   []ConnectionFact        `json:"connections"`
	Runtime       []RuntimeComponentState `json:"runtime_components"`
}

// ConfigSnapshot embeds the configuration version into the manifest.
type ConfigSnapshot struct {
	EmployeeID  id.UUID    `json:"employee_id"`
	VersionID   id.UUID    `json:"version_id"`
	Version     int64      `json:"version"`
	Description string     `json:"description"`
	ActorID     id.UUID    `json:"actor_id"`
	CreatedAt   time.Time  `json:"created_at"`
	Refs        ConfigRefs `json:"refs"`
}

// ConnectionFact is one connection as sealed into the manifest: identifiers,
// contract and adapter versions, environment and status — never secrets.
type ConnectionFact struct {
	ConnectionID    id.UUID          `json:"connection_id"`
	ConnectorID     string           `json:"connector_id"`
	ContractVersion string           `json:"contract_version"`
	AdapterBuild    string           `json:"adapter_build"`
	Environment     string           `json:"environment"`
	Status          string           `json:"status"`
	Capabilities    []CapabilityFact `json:"capabilities"`
}

// CapabilityFact is the derived capability state of one connection frozen
// at sealing time. A capability without applicable evidence stays an
// explicit gap: it is never replaced by another account's or provider's
// possibility.
type CapabilityFact struct {
	Capability string        `json:"capability"`
	State      string        `json:"state"`
	Reason     string        `json:"state_reason"`
	Evidence   *EvidenceFact `json:"evidence,omitempty"`
	Gap        bool          `json:"gap"`
	Owner      string        `json:"reconciliation_owner"`
}

// EvidenceFact is the safe evidence reference: identifiers, outcome and
// times only.
type EvidenceFact struct {
	EvidenceID id.UUID   `json:"evidence_id"`
	Outcome    string    `json:"outcome"`
	CheckedAt  time.Time `json:"checked_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// BuildManifestContent composes the canonical content from a configuration
// version and the connection facts gathered at sealing time. Connections are
// sorted by connection id, capabilities by capability id, knowledge refs by
// source id — the canonical order the digest depends on.
func BuildManifestContent(config ConfigVersion, connections []ConnectionFact, runtime []RuntimeComponentState) ManifestContent {
	sorted := append([]ConnectionFact(nil), connections...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ConnectionID.String() < sorted[j].ConnectionID.String() })
	for i := range sorted {
		caps := append([]CapabilityFact(nil), sorted[i].Capabilities...)
		sort.Slice(caps, func(a, b int) bool { return caps[a].Capability < caps[b].Capability })
		sorted[i].Capabilities = caps
	}
	state := runtime
	if state == nil {
		// Не-seal читатели и прямые тесты получают честный словарь с
		// производной запиской tool_schemas без привязки (production
		// seal передаёт вычисленное состояние явно — nil здесь не путь
		// по умолчанию нового seal).
		state = BuildRuntimeState(DeriveToolSchemasState(config.Refs.Pack, nil))
	}
	return ManifestContent{
		SchemaVersion: ManifestSchemaCurrent,
		Config: ConfigSnapshot{
			EmployeeID: config.EmployeeID, VersionID: config.VersionID, Version: config.Version,
			Description: config.Description, ActorID: config.ActorID, CreatedAt: config.CreatedAt,
			Refs: config.Refs,
		},
		Connections: sorted,
		Runtime:     state,
	}
}

// DeriveManifestStatus is the pure status function: `candidate` only when
// every applicable component is present, every capability of every
// connection is proven, and every runtime component is present. Any gap —
// a recorded absence, an unverified or stale capability, a missing runtime
// component — makes the manifest honestly `incomplete`.
func DeriveManifestStatus(content ManifestContent) string {
	refs := content.Config.Refs
	switch {
	case refs.Brief.Status != RefPresent:
		return ManifestStatusIncomplete
	case refs.Pack.Status != RefPresent:
		return ManifestStatusIncomplete
	case refs.Behavior.Status != RefPresent:
		return ManifestStatusIncomplete
	case refs.Knowledge.Status != RefPresent || len(refs.Knowledge.Sources) == 0:
		return ManifestStatusIncomplete
	case refs.Voice.Status == RefAbsent:
		return ManifestStatusIncomplete
	}
	for _, conn := range content.Connections {
		for _, cap := range conn.Capabilities {
			if cap.Gap || cap.State != "verified" {
				return ManifestStatusIncomplete
			}
		}
	}
	for _, component := range content.Runtime {
		if !component.Present {
			return ManifestStatusIncomplete
		}
	}
	return ManifestStatusCandidate
}

// CanonicalManifestJSON serializes the content deterministically: struct
// field order is fixed by the encoder, arrays are pre-sorted by the builder
// and map keys are sorted by encoding/json.
func CanonicalManifestJSON(content ManifestContent) ([]byte, error) {
	return json.Marshal(content)
}

// ManifestDigest computes the sha256 hex digest of the canonical bytes.
func ManifestDigest(content ManifestContent) (string, error) {
	raw, err := CanonicalManifestJSON(content)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ---------------------------------------------------------------------------
// Reading a manifest by schema version
// ---------------------------------------------------------------------------

// ManifestRead is the outcome of reading a stored manifest payload. When
// Quarantine is empty the content is trusted after a digest re-check; any
// other state names the concrete reason and keeps the raw payload for
// display — nothing is silently discarded.
type ManifestRead struct {
	Content       ManifestContent
	Status        string // quarantine kind or ""
	Reason        string
	Raw           []byte
	Digest        string
	SchemaVersion int
}

// ReadManifestPayload decodes one stored payload honestly. The schema
// version is checked first, then a strict decode (unknown fields are a
// quarantine, not a silent drop), then the digest is recomputed from the
// canonical serialization of the decoded content.
func ReadManifestPayload(payload []byte, storedDigest string) ManifestRead {
	read := ManifestRead{Raw: payload, Digest: storedDigest}
	var probe struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil || probe.SchemaVersion == nil {
		read.Status = QuarantineUnknownField
		read.SchemaVersion = 0
		read.Reason = "Payload манифеста не читается: " + honestDecodeReason(err)
		return read
	}
	read.SchemaVersion = *probe.SchemaVersion
	if !manifestSchemaReadable[*probe.SchemaVersion] {
		read.Status = QuarantineSchemaVersion
		read.Reason = fmt.Sprintf("Версия схемы манифеста %d не поддерживается этим билдом: payload сохранён и отправлен в явный quarantine; миграция на месте запрещена.", *probe.SchemaVersion)
		return read
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var content ManifestContent
	if err := decoder.Decode(&content); err != nil {
		read.Status = QuarantineUnknownField
		read.Reason = "Payload манифеста содержит неизвестное поле или нечитаемое значение: " + honestDecodeReason(err) + ". Поле не отбрасывается молча."
		return read
	}
	recomputed, err := ManifestDigest(content)
	if err != nil || recomputed != storedDigest {
		read.Status = QuarantineDigestMismatch
		read.Reason = "Digest манифеста не совпадает с пересчитанным значением: целостность снимка нарушена. Молчаливое доверие исключено."
		return read
	}
	read.Content = content
	return read
}

func honestDecodeReason(err error) string {
	if err == nil {
		return "пустой или некорректный JSON"
	}
	return err.Error()
}

// ---------------------------------------------------------------------------
// Restore compatibility report
// ---------------------------------------------------------------------------

// CompatibilityIssue is one honest incompatibility of a restored version
// against the current facts.
type CompatibilityIssue struct {
	Area    string `json:"area"`
	Subject string `json:"subject"`
	Problem string `json:"problem"`
}

// RestoreFacts are the current facts the restored refs are compared with.
type RestoreFacts struct {
	DraftVersion    int64
	PackAvailable   bool
	BehaviorVersion int64
	VoiceVersion    int64
	Knowledge       map[id.UUID]int64
	Connections     []ConnectionFacts
	Sealed          *ManifestContent
}

// ConnectionFacts are the current facts of one connection for the restore
// compatibility report: identifier, state and derived capabilities. The app
// layer adapts them from the connector module — the release domain never
// re-derives capability states itself (one truth: connector).
type ConnectionFacts struct {
	ConnectionID id.UUID
	Status       string
	Capabilities []CapabilityFact
}

// RestoreCompatibility reports every incompatibility of the selected
// version's refs against the current facts: the pack catalog availability,
// current connection states and applicable evidence, current knowledge
// versions, behavior, voice and the brief. Restore itself is never blocked,
// but nothing is hidden either.
func RestoreCompatibility(refs ConfigRefs, facts RestoreFacts) []CompatibilityIssue {
	issues := []CompatibilityIssue{}
	if refs.Brief.Status == RefPresent && facts.DraftVersion != refs.Brief.DraftVersion {
		issues = append(issues, CompatibilityIssue{Area: "brief", Subject: "Employee Brief",
			Problem: fmt.Sprintf("Brief обновлялся после версии: в версии черновик v%d, текущий v%d. Ссылки версии остаются валидными, но текущее состояние иное.", refs.Brief.DraftVersion, facts.DraftVersion)})
	}
	if refs.Pack.Status == RefPresent && !facts.PackAvailable {
		issues = append(issues, CompatibilityIssue{Area: "domain_pack", Subject: refs.Pack.ID + " " + refs.Pack.Version,
			Problem: "Закреплённая версия Pack недоступна в текущем каталоге: выберите доступную версию перед публикацией."})
	}
	for _, source := range refs.Knowledge.Sources {
		current, known := facts.Knowledge[source.SourceID]
		switch {
		case !known:
			issues = append(issues, CompatibilityIssue{Area: "knowledge", Subject: source.SourceID.String(),
				Problem: "Источник знаний из версии отсутствует в текущей конфигурации: он был удалён или выведен из оборота."})
		case current != source.Version:
			issues = append(issues, CompatibilityIssue{Area: "knowledge", Subject: source.SourceID.String(),
				Problem: fmt.Sprintf("Версия знания сместилась: в версии v%d, текущая v%d.", source.Version, current)})
		}
	}
	if refs.Behavior.Status == RefPresent && facts.BehaviorVersion != refs.Behavior.Version {
		issues = append(issues, CompatibilityIssue{Area: "behavior", Subject: "Поведение",
			Problem: fmt.Sprintf("Конфигурация поведения изменилась: в версии v%d, текущая v%d.", refs.Behavior.Version, facts.BehaviorVersion)})
	}
	if refs.Behavior.Status != RefPresent && facts.BehaviorVersion > 0 {
		issues = append(issues, CompatibilityIssue{Area: "behavior", Subject: "Поведение",
			Problem: fmt.Sprintf("В версии поведение отсутствовало, а сейчас настроено (v%d): восстановленный черновик не включает его до пересборки.", facts.BehaviorVersion)})
	}
	if refs.Voice.Status == RefPresent && facts.VoiceVersion != refs.Voice.Version {
		issues = append(issues, CompatibilityIssue{Area: "voice", Subject: "Голос",
			Problem: fmt.Sprintf("Конфигурация голоса изменилась: в версии v%d, текущая v%d.", refs.Voice.Version, facts.VoiceVersion)})
	}
	if refs.Voice.Status != RefPresent && refs.Voice.Status == RefAbsent && facts.VoiceVersion > 0 {
		issues = append(issues, CompatibilityIssue{Area: "voice", Subject: "Голос",
			Problem: fmt.Sprintf("В версии голос отсутствовал, а сейчас настроен (v%d).", facts.VoiceVersion)})
	}
	if facts.Sealed != nil {
		issues = append(issues, connectionCompatibility(facts.Sealed.Connections, facts.Connections)...)
	}
	return issues
}

// connectionCompatibility compares the sealed connection facts with the
// current ones: removed connections, changed states and capabilities whose
// evidence no longer applies.
func connectionCompatibility(sealed []ConnectionFact, current []ConnectionFacts) []CompatibilityIssue {
	issues := []CompatibilityIssue{}
	currentByID := map[id.UUID]ConnectionFacts{}
	for _, conn := range current {
		currentByID[conn.ConnectionID] = conn
	}
	for _, was := range sealed {
		now, known := currentByID[was.ConnectionID]
		if !known {
			issues = append(issues, CompatibilityIssue{Area: "connection", Subject: was.ConnectionID.String(),
				Problem: "Подключение из манифеста отсутствует в текущих фактах: оно было удалено."})
			continue
		}
		if now.Status != was.Status {
			issues = append(issues, CompatibilityIssue{Area: "connection", Subject: was.ConnectionID.String(),
				Problem: fmt.Sprintf("Состояние подключения изменилось: было %s, стало %s.", was.Status, now.Status)})
		}
		currentCaps := map[string]CapabilityFact{}
		for _, cap := range now.Capabilities {
			currentCaps[cap.Capability] = cap
		}
		for _, cap := range was.Capabilities {
			if nowCap, ok := currentCaps[cap.Capability]; ok && nowCap.State != cap.State {
				issues = append(issues, CompatibilityIssue{Area: "capability", Subject: was.ConnectionID.String() + " · " + cap.Capability,
					Problem: fmt.Sprintf("Состояние capability изменилось: было %s, стало %s. Evidence версии могло потерять применимость.", cap.State, nowCap.State)})
			}
		}
	}
	return issues
}

// ValidateReleaseDescription checks the change description of a new
// configuration version.
func ValidateReleaseDescription(value string) error {
	note := len([]rune(value))
	if note < 1 || note > 300 {
		return ReleaseRuleError{Rule: "description", Reason: "Описание изменения: от 1 до 300 символов."}
	}
	return nil
}
