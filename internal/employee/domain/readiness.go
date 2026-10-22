package domain

import (
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "fmt"
        "sort"
        "strings"
        "time"

        "github.com/artemiosu/staffy/internal/platform/id"
)

// Errors of the test & release surface. Authorization failures of members
// surface as ErrReadinessForbidden (an honest refusal); unknown employees or
// tenants as ErrNotFound; gate transitions and the publication without
// step-up as ErrReadinessStepUpRequired (the membership/profile precedent:
// no provider is wired, so the refusal is honest).
var (
        ErrReadinessForbidden      = errors.New("principal is not allowed on the test & release surface")
        ErrReadinessStepUpRequired = errors.New("gate transitions and release publication require step-up confirmation")
)

// Traffic stages of the accepted GATE-REGISTRY Execution order. The `scale`
// stage stays outside this slice (Epic 2+) — an honest boundary, never a
// silent absence.
const (
        StageSynthetic              = "synthetic"
        StageClosedAccountTest      = "closed_account_test"
        StageControlledRealCustomer = "controlled_real_customer"
        StageLive                   = "live"
)

// StageOrder is the fixed ladder: a closed stage N requires the acting
// decision of stage N-1 of the same candidate (the exact digests).
var StageOrder = []string{StageSynthetic, StageClosedAccountTest, StageControlledRealCustomer, StageLive}

// StageIndex resolves the position of a stage in the ladder.
func StageIndex(stage string) (int, bool) {
        for index, candidate := range StageOrder {
                if candidate == stage {
                        return index, true
                }
        }
        return 0, false
}

// GateInstance states — the fixed vocabulary of the registry
// (allowed_states). A PASS is permission only inside its exact scope; a
// missing, open, in_test, failed, expired or superseded instance never is.
const (
        GateStateOpen          = "open"
        GateStateInTest        = "in_test"
        GateStatePass          = "pass"
        GateStateFail          = "fail"
        GateStateExpired       = "expired"
        GateStateNotApplicable = "not_applicable"
)

// Approver role labels of the accepted registry. They are registry marks,
// not tenant membership roles: the system checks that the named participants
// exist as active members and that the role coverage is structurally
// complete — who actually holds the role in the organization is a human
// governance fact (the honest note of the spec).
const (
        GateRoleSecurityPrivacy    = "SecurityPrivacy"
        GateRoleTechnicalLead      = "TechnicalLead"
        GateRoleVoiceTechnicalLead = "VoiceTechnicalLead"
        GateRoleOperations         = "Operations"
        GateRoleProductOwner       = "ProductOwner"
        GateRoleMontagioOperations = "MontagioOperations"
        GateRoleOperationsQA       = "OperationsQA"
        GateRoleUXQA               = "UXQA"
        GateRoleSecurity           = "Security"
        // The pilot approver-role labels of spec 4-1 (the registry marks of the
        // Oleg pilot gates carried into the product surface).
        GateRoleOleg               = "Oleg"
        GateRoleIntegrationLead    = "IntegrationLead"
        GateRoleABCPIntegrator     = "ABCPIntegrator"
        GateRole1CIntegrator       = "1CIntegrator"
        GateRoleFitmentExpert      = "FitmentExpert"
        GateRoleArchitecture       = "Architecture"
        GateRoleOwningSystemOwners = "OwningSystemOwners"
)

// Readiness element states: a missing mandatory element is a recorded
// absence (the publication does not exist); blocked names the concrete
// blocking with a reason; linked_not_executed is the honest eval state.
const (
        ElementMissing           = "missing"
        ElementNotApplicable     = "not_applicable"
        ElementPass              = "pass"
        ElementBlocked           = "blocked"
        ElementLinkedNotExecuted = "linked_not_executed"
)

// Route admission states of the readiness derivation.
const (
        RouteAdmitted = "admitted"
        RouteBlocked  = "blocked"
)

// Overall candidate status for a requested stage: incomplete (a mandatory
// element is missing), blocked (elements filled but no admitted route) or
// ready (at least one route admitted). A technical success of a single
// component never becomes the readiness of the whole.
const (
        ReadinessIncomplete = "incomplete"
        ReadinessBlocked    = "blocked"
        ReadinessReady      = "ready"
)

// ---------------------------------------------------------------------------
// The fixed dictionary of platform gate definitions (GATE-REGISTRY, adopted)
// ---------------------------------------------------------------------------

// GateDefinition is one platform gate of the accepted registry: what must be
// proven, by which approver roles, with which evidence, and the safe state
// until PASS. The dictionary is fixed in the domain — it is never tenant
// configurable; extending it is a new spec version.
type GateDefinition struct {
        ID                    string   `json:"definition_id"`
        Criticality           string   `json:"criticality"`
        RequiredApproverRoles []string `json:"required_approver_roles"`
        EvidenceRequirement   string   `json:"evidence_requirement"`
        SafeState             string   `json:"safe_state"`
        AllowRoleCoincidence  bool     `json:"allow_role_coincidence"`
        // RouteScoped marks the definitions whose instances carry the route
        // scope dimension (OQ-11 — the data route). Every other definition is
        // tenant-level: its instances carry no route.
        RouteScoped bool `json:"route_scoped"`
}

// gateDictionary is the fixed order of the platform definitions that
// participate in the stage map, followed by the pilot definitions of spec
// 4-1 (Pilot Admission Registry): the Oleg pilot gates {OQ-1..OQ-5, OQ-19}
// are carried from the GATE-REGISTRY planning artifacts into the product
// surface by that spec — the dictionary extension the comment here
// anticipated ("extending it is a new spec version"). The AG-4 of the
// pilot set is the platform definition above (identical canon). The
// remaining tenant-specific gates (OQ-6..OQ-10, AG-3…) stay project-
// planning artifacts — deliberately not carried into the product surface.
var gateDictionary = []GateDefinition{
        {
                ID: "OQ-11", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleSecurityPrivacy, GateRoleTechnicalLead},
                EvidenceRequirement:   "Provider-route-artifact-purpose perimeter, basis, retention/deletion and fallback evidence",
                SafeState:             "synthetic/non-PII only",
                AllowRoleCoincidence:  false,
                RouteScoped:           true,
        },
        {
                ID: "OQ-12", Criticality: "high",
                RequiredApproverRoles: []string{GateRoleVoiceTechnicalLead},
                EvidenceRequirement:   "Measured route latency, interruption, reconnect, quality and cost",
                SafeState:             "no numeric immediacy promise",
                AllowRoleCoincidence:  true,
        },
        {
                ID: "OQ-13B", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleTechnicalLead, GateRoleOperations},
                EvidenceRequirement:   "Dated restore/rebind/reconcile/media replacement drill",
                SafeState:             "synthetic or closed-account test only",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-14", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleProductOwner, GateRoleOperationsQA},
                EvidenceRequirement:   "Versioned quantitative admission thresholds and evidence",
                SafeState:             "manual scoped promotion",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-7B", Criticality: "high",
                RequiredApproverRoles: []string{GateRoleProductOwner, GateRoleMontagioOperations},
                EvidenceRequirement:   "Baseline-derived route thresholds and sample sufficiency",
                SafeState:             "controlled learning only",
                AllowRoleCoincidence:  true,
        },
        {
                ID: "AG-1", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleTechnicalLead, GateRoleOperations},
                EvidenceRequirement:   "Pilot Load Envelope tested at approved safety factor",
                SafeState:             "simulation/load test only",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "AG-2", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleProductOwner, GateRoleUXQA},
                EvidenceRequirement:   "Automated and manual WCAG 2.2 AA evidence",
                SafeState:             "affected route does not publish",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "AG-4", Criticality: "high",
                RequiredApproverRoles: []string{GateRoleTechnicalLead, GateRoleSecurity},
                EvidenceRequirement:   "Concrete protected dependencies plus compatibility, license, SBOM, vulnerability and rollback evidence",
                SafeState:             "local synthetic implementation only",
                AllowRoleCoincidence:  false,
        },
        // The pilot gate definitions of spec 4-1 (the Oleg pilot gates of the
        // GATE-REGISTRY canon, verbatim fields; the criticality follows the
        // registry table — OQ-1 and OQ-2 are high there).
        {
                ID: "OQ-1", Criticality: "high",
                RequiredApproverRoles: []string{GateRoleOleg, GateRoleTechnicalLead},
                EvidenceRequirement:   "Account/contract inventory and recorded SIP/trunk, codec, identity, event, transfer, fallback and failure tests",
                SafeState:             "awaiting_verification/limited; no unsupported transfer/caller claim",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-2", Criticality: "high",
                RequiredApproverRoles: []string{GateRoleOleg, GateRoleIntegrationLead},
                EvidenceRequirement:   "Account tests for IDs, reply, assignment, acknowledgement, writer exclusion, receipts, timeout and explicit return",
                SafeState:             "production shared inbox is human-only; Staffy-only requires isolated/enforced writer exclusion and tested takeover",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-3", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleOleg, GateRoleABCPIntegrator},
                EvidenceRequirement:   "Actual product/version, API rights, prepared-status mapping, notification, reread and enforceable human supplier-send fence",
                SafeState:             "neutral pending_manager_verification; manual reread and human receipt",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-4", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleOleg, GateRole1CIntegrator},
                EvidenceRequirement:   "State/ID mapping, correlation, version/time, duplicate behavior, reread/reconcile and terminal evidence",
                SafeState:             "no terminal success; mismatch/unknown is manual reconciliation",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-5", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleOleg, GateRoleFitmentExpert, GateRoleIntegrationLead},
                EvidenceRequirement:   "Recorded workflow; ABCP/Laximo tariff/account inventory; API/support confirmation; representative VIN -> vehicle -> diagram/node -> OEM -> cross/offer tests; allowed item classes",
                SafeState:             "no inferred fitment/UI scraping; human task or licensed reusable catalog Connector",
                AllowRoleCoincidence:  false,
        },
        {
                ID: "OQ-19", Criticality: "critical",
                RequiredApproverRoles: []string{GateRoleArchitecture, GateRoleOwningSystemOwners},
                EvidenceRequirement:   "Per material field/entity: authority, external ID/version/time, projection, conflict rule, reconcile query, terminal evidence and contract tests",
                SafeState:             "field is unknown; no mutation or terminal success",
                AllowRoleCoincidence:  false,
        },
}

var gateDictionaryIndex = func() map[string]GateDefinition {
        index := make(map[string]GateDefinition, len(gateDictionary))
        for _, definition := range gateDictionary {
                index[definition.ID] = definition
        }
        return index
}()

// GateDictionary returns the fixed dictionary in order (a copy).
func GateDictionary() []GateDefinition {
        return append([]GateDefinition(nil), gateDictionary...)
}

// GateDefinitionByID resolves one dictionary entry.
func GateDefinitionByID(definitionID string) (GateDefinition, bool) {
        definition, ok := gateDictionaryIndex[definitionID]
        return definition, ok
}

// StageRequiresGates reports whether the stage demands gate PASS instances
// (synthetic and closed_account_test do not: fixtures and approved test
// accounts with synthetic or minimized data up to OQ-11 PASS).
func StageRequiresGates(stage string) bool {
        index, ok := StageIndex(stage)
        return ok && index >= 2
}

// StageRequiresEval reports whether the stage demands executed eval — from
// controlled_real_customer up the eval element blocks (no eval surfaces
// exist: Epic 6), an honest eternal block of this slice.
func StageRequiresEval(stage string) bool {
        index, ok := StageIndex(stage)
        return ok && index >= 2
}

// StageRequiresEntitlementAdmission reports whether the entitlement
// admission state blocks the publication: from closed_account_test up, a
// draft/paused/hard-limit state means "new admissions and publications are
// blocked" (the words of the 1-11 derivation). Synthetic is the documented
// exception: fixtures do not consume commercial surfaces.
func StageRequiresEntitlementAdmission(stage string) bool {
        index, ok := StageIndex(stage)
        return ok && index >= 1
}

// RequiredGateIDs lists the dictionary IDs a route must prove at the stage:
// the Execution order of the registry — controlled_real_customer requires
// OQ-11 (route-scoped), AG-1, AG-2 and AG-4, voice routes additionally
// OQ-12 and OQ-13B; live adds OQ-14 and OQ-7B.
func RequiredGateIDs(stage string, route Route) []string {
        if !StageRequiresGates(stage) {
                return nil
        }
        ids := []string{"OQ-11", "AG-1", "AG-2", "AG-4"}
        if route.IsVoice() {
                ids = append(ids, "OQ-12", "OQ-13B")
        }
        if stage == StageLive {
                ids = append(ids, "OQ-14", "OQ-7B")
        }
        return ids
}

// ---------------------------------------------------------------------------
// Gate instances (append-only lifecycle records)
// ---------------------------------------------------------------------------

// GateApproval is one named approver of a PASS: a participant of the tenant
// plus the registry role label they acted under. The participant must be an
// active member (checked by the app through MembersFacts); the org truth of
// "who really is the Technical Lead" stays a human governance fact.
type GateApproval struct {
        ParticipantID id.UUID `json:"participant_id"`
        Role          string  `json:"role"`
}

// GateInstance is one append-only lifecycle record against a dictionary
// definition. Scope dimensions are present only when exact — a nil route
// means the instance is not scoped by route (tenant level); an empty
// modality means it is not scoped by modality. There are no wildcards.
// Every state change inserts a new record linked by supersedes; a record no
// other record supersedes is the chain head (the current state).
type GateInstance struct {
        TenantID             id.UUID        `json:"tenant_id"`
        EmployeeID           id.UUID        `json:"employee_id"`
        InstanceID           id.UUID        `json:"instance_id"`
        DefinitionID         string         `json:"definition_id"`
        Route                *Route         `json:"route,omitempty"`
        Modality             string         `json:"modality,omitempty"`
        State                string         `json:"state"`
        ApprovedBy           []GateApproval `json:"approved_by"`
        EvidenceRefs         []string       `json:"evidence_refs"`
        EvidenceHash         string         `json:"evidence_hash,omitempty"`
        NotApplicableReason  string         `json:"not_applicable_reason,omitempty"`
        TestedAt             *time.Time     `json:"tested_at,omitempty"`
        ExpiryTrigger        string         `json:"expiry_trigger,omitempty"`
        SupersedesInstanceID *id.UUID       `json:"supersedes_instance_id,omitempty"`
        ActorID              id.UUID        `json:"actor_id"`
        RecordedAt           time.Time      `json:"recorded_at"`
}

// SameScope reports whether two records carry the exact same scope
// (definition + route + modality) — the chain identity.
func (i GateInstance) SameScope(other GateInstance) bool {
        if i.DefinitionID != other.DefinitionID || i.Modality != other.Modality {
                return false
        }
        switch {
        case i.Route == nil && other.Route == nil:
                return true
        case i.Route == nil || other.Route == nil:
                return false
        default:
                return *i.Route == *other.Route
        }
}

// SupersededSet derives which instance records are superseded: a record is
// superseded when another record's supersedes link points at it. A
// superseded PASS is never the current permission.
func SupersededSet(instances []GateInstance) map[id.UUID]bool {
        superseded := map[id.UUID]bool{}
        for _, instance := range instances {
                if instance.SupersedesInstanceID != nil {
                        superseded[*instance.SupersedesInstanceID] = true
                }
        }
        return superseded
}

// InstanceApplicableTo reports whether the record applies to the exact
// route: the route dimension is absent (tenant level) or equals the route
// exactly; the modality likewise. No wildcards exist.
func InstanceApplicableTo(instance GateInstance, route Route) bool {
        if instance.Route != nil && *instance.Route != route {
                return false
        }
        if instance.Modality != "" && instance.Modality != ModalityOfRoute(route) {
                return false
        }
        return true
}

// ValidPassForRoute resolves the acting PASS of a definition for one route:
// an applicable, unsuperseded record in the exact pass state. Missing,
// open, in_test, fail, expired, superseded and not_applicable never are.
func ValidPassForRoute(definitionID string, instances []GateInstance, route Route) *GateInstance {
        superseded := SupersededSet(instances)
        for index := range instances {
                instance := instances[index]
                if instance.DefinitionID != definitionID || instance.State != GateStatePass {
                        continue
                }
                if superseded[instance.InstanceID] {
                        continue
                }
                if !InstanceApplicableTo(instance, route) {
                        continue
                }
                return &instance
        }
        return nil
}

// InstanceStateForRoute names the honest current state of a definition for
// one route: the newest applicable unsuperseded record's state, or "missing"
// when no applicable record exists. The reason text names the concrete
// record, so a blocked route shows what it needs.
func InstanceStateForRoute(definitionID string, instances []GateInstance, route Route) (string, string) {
        superseded := SupersededSet(instances)
        var latest *GateInstance
        for index := range instances {
                instance := instances[index]
                if instance.DefinitionID != definitionID || superseded[instance.InstanceID] {
                        continue
                }
                if !InstanceApplicableTo(instance, route) {
                        continue
                }
                if latest == nil || instance.RecordedAt.After(latest.RecordedAt) || (instance.RecordedAt == latest.RecordedAt && instance.InstanceID.String() > latest.InstanceID.String()) {
                        latest = &instances[index]
                }
        }
        if latest == nil {
                return "missing", "применимого инстанса нет"
        }
        scope := "tenant"
        if latest.Route != nil {
                scope = "маршрут " + latest.Route.String()
        }
        return latest.State, fmt.Sprintf("инстанс %s (%s, записан %s)", latest.InstanceID, scope, latest.RecordedAt.Format("02.01.2006"))
}

// GateTransitionSpec is the validated shape of one RecordGateTransition
// command: the definition, the exact scope, the target state and the
// supersedes link of the lifecycle record to insert.
type GateTransitionSpec struct {
        DefinitionID        string
        Route               *Route
        Modality            string
        TargetState         string
        Supersedes          *id.UUID
        ApprovedBy          []GateApproval
        EvidenceRefs        []string
        EvidenceHash        string
        NotApplicableReason string
        ExpiryTrigger       string
        TestedAt            *time.Time
}

// GateConflictError is the honest loss of a gate race: the referenced record
// has already been transitioned (exactly one successor exists — the partial
// UNIQUE index on supersedes), so the command refuses instead of rewriting.
type GateConflictError struct {
        InstanceID id.UUID
        Reason     string
}

func (e GateConflictError) Error() string {
        return "gate transition conflict: " + e.Reason
}
func (e GateConflictError) Unwrap() error { return ErrConflict }

// gateSuccessors is the fixed state machine of the registry: open -> in_test
// -> pass|fail; any active state (open, in_test, pass) may become expired;
// an approved scoped determination (pass) may become not_applicable. The
// settled states (fail, expired, not_applicable) reopen only through a new
// chain: a fresh record linked by supersedes.
var gateSuccessors = map[string][]string{
        GateStateOpen:          {GateStateInTest, GateStateExpired},
        GateStateInTest:        {GateStatePass, GateStateFail, GateStateExpired},
        GateStatePass:          {GateStateExpired, GateStateNotApplicable},
        GateStateFail:          {},
        GateStateExpired:       {},
        GateStateNotApplicable: {},
}

func isTerminalGateState(state string) bool {
        return len(gateSuccessors[state]) == 0
}

func successorAllowed(from, to string) bool {
        for _, candidate := range gateSuccessors[from] {
                if candidate == to {
                        return true
                }
        }
        return false
}

// ValidateGateTransition validates one transition command against the full
// instance history of the employee and returns the normalized payload of
// the record to insert (the caller stamps tenant, employee, instance id,
// actor and time). The history is the append-only record list; the command
// never rewrites anything.
func ValidateGateTransition(spec GateTransitionSpec, history []GateInstance) (GateTransitionSpec, error) {
        invalid := func(field, reason string) error {
                return ProfileRuleError{Section: "gate_instance", Field: field, Reason: reason, Kind: ProfileProblemInvalid}
        }
        definition, known := GateDefinitionByID(spec.DefinitionID)
        if !known {
                return spec, invalid("definition_id", "Определение «"+spec.DefinitionID+"» вне фиксированного словаря реестра (14 определений: 8 платформенных + 6 пилотных спеки 4-1): запись против локальных определений запрещена.")
        }
        switch spec.TargetState {
        case GateStateOpen, GateStateInTest, GateStatePass, GateStateFail, GateStateExpired, GateStateNotApplicable:
        default:
                return spec, invalid("state", "Состояние инстанса: open, in_test, pass, fail, expired или not_applicable.")
        }
        // Scope discipline: route is exact or absent; OQ-11 requires the route
        // dimension, the tenant-level definitions reject it; modality is exact
        // or absent.
        if spec.Route != nil {
                if !spec.Route.valid() {
                        return spec, invalid("route", "Scope маршрута: канал из словаря Brief + направление inbound/outbound.")
                }
                if !definition.RouteScoped {
                        return spec, invalid("route", "Определение "+definition.ID+" — tenant-уровень: его инстансы не несут scope маршрута (отсутствующее измерение, не wildcard).")
                }
        } else if definition.RouteScoped {
                return spec, invalid("route", "Определение "+definition.ID+" требует точный scope маршрута (route-scoped data gate).")
        }
        if spec.Modality != "" && spec.Modality != ModalityVoice && spec.Modality != ModalityText {
                return spec, invalid("modality", "Scope modality: voice или text (или отсутствует).")
        }
        if len(spec.ExpiryTrigger) > 0 && !length(spec.ExpiryTrigger, 1, 300) {
                return spec, invalid("expiry_trigger", "Триггер экспирации: 1–300 символов (хранится и отображается; планировщиков в срезе нет).")
        }
        var predecessor *GateInstance
        if spec.Supersedes != nil {
                for index := range history {
                        if history[index].InstanceID == *spec.Supersedes {
                                predecessor = &history[index]
                                break
                        }
                }
                if predecessor == nil {
                        return spec, invalid("supersedes", "Инстанс для перехода не найден среди записей этого сотрудника.")
                }
                specScope := GateInstance{DefinitionID: spec.DefinitionID, Route: spec.Route, Modality: spec.Modality}
                if !predecessor.SameScope(specScope) {
                        return spec, invalid("supersedes", "Переход применяется к инстансу другого определения или scope: определение и scope должны совпадать точно.")
                }
                // Exactly one successor per record: a raced transition is an honest
                // conflict, never a rewrite.
                superseded := SupersededSet(history)
                if superseded[predecessor.InstanceID] {
                        return spec, GateConflictError{InstanceID: predecessor.InstanceID,
                                Reason: "инстанс " + predecessor.InstanceID.String() + " уже переведён другой командой: ровно один преемник, перечитайте текущее состояние."}
                }
                // Acyclicity: walking the supersedes chain from the predecessor must
                // terminate without revisiting any record (a corrupt or cyclic
                // history is refused at the write, never trusted).
                visited := map[id.UUID]bool{predecessor.InstanceID: true}
                cursor := predecessor
                for cursor.SupersedesInstanceID != nil {
                        if visited[*cursor.SupersedesInstanceID] {
                                return spec, invalid("supersedes", "Цикл в цепочке supersedes: запись отвергается, история не доверяется.")
                        }
                        visited[*cursor.SupersedesInstanceID] = true
                        next := (*GateInstance)(nil)
                        for index := range history {
                                if history[index].InstanceID == *cursor.SupersedesInstanceID {
                                        next = &history[index]
                                        break
                                }
                        }
                        if next == nil {
                                break
                        }
                        cursor = next
                }
                if isTerminalGateState(predecessor.State) {
                        // Reopening: a settled record never changes; a fresh chain
                        // starts at open/in_test and links to it.
                        if spec.TargetState != GateStateOpen && spec.TargetState != GateStateInTest {
                                return spec, invalid("state", "Инстанс в состоянии "+predecessor.State+" (settled): перезапись запрещена — reopening создаёт новый инстанс (open или in_test) со ссылкой supersedes.")
                        }
                } else if !successorAllowed(predecessor.State, spec.TargetState) {
                        return spec, invalid("state", "Переход "+predecessor.State+" → "+spec.TargetState+" вне state machine реестра (open → in_test → pass|fail; активный статус → expired; pass → not_applicable).")
                }
        } else if spec.TargetState != GateStateOpen && spec.TargetState != GateStateInTest {
                return spec, invalid("state", "Новый инстанс создаётся в состоянии open или in_test; терминальные состояния записываются переходом.")
        }
        // PASS discipline: full role coverage by distinct principals (except an
        // explicit allow_role_coincidence), non-empty evidence references and
        // the proof hash.
        if spec.TargetState == GateStatePass {
                if err := validateGatePass(spec, definition); err != nil {
                        return spec, err
                }
        }
        if spec.TargetState == GateStateNotApplicable {
                if !length(spec.NotApplicableReason, 1, 300) {
                        return spec, invalid("not_applicable_reason", "Scoped-решение not_applicable требует обоснование (1–300 символов); оно не расширяет разрешение на другие scope.")
                }
        }
        if len(spec.ApprovedBy) > 0 && spec.TargetState != GateStatePass {
                return spec, invalid("approved_by", "approved_by записывается только вместе с PASS (дисциплина подтверждения реестра).")
        }
        if len(spec.EvidenceRefs) > 0 && spec.TargetState != GateStatePass {
                return spec, invalid("evidence_refs", "evidence-ссылки записываются только вместе с PASS.")
        }
        if spec.EvidenceHash != "" && spec.TargetState != GateStatePass {
                return spec, invalid("evidence_hash", "hash доказательства записывается только вместе с PASS.")
        }
        return spec, nil
}

// validateGatePass enforces the PASS discipline of the registry: every
// required approver role is covered, the principals are distinct across
// roles unless the definition explicitly allows coincidence, the evidence
// references are non-empty (1–300 each) and the hash is a sha256 hex.
func validateGatePass(spec GateTransitionSpec, definition GateDefinition) error {
        invalid := func(field, reason string) error {
                return ProfileRuleError{Section: "gate_instance", Field: field, Reason: reason, Kind: ProfileProblemInvalid}
        }
        if len(spec.ApprovedBy) == 0 {
                return invalid("approved_by", "PASS требует полный список approver'ов: каждая требуемая роль определения должна быть покрыта участником.")
        }
        seen := map[string]bool{}
        byRole := map[string][]id.UUID{}
        for _, approval := range spec.ApprovedBy {
                if !approval.ParticipantID.Valid() {
                        return invalid("approved_by", "Каждый approver — идентификатор участника Tenant (UUID).")
                }
                if _, ok := gateDictionaryIndex[definition.ID]; !ok {
                        return invalid("approved_by", "Определение неизвестно.")
                }
                required := false
                for _, role := range definition.RequiredApproverRoles {
                        if approval.Role == role {
                                required = true
                                break
                        }
                }
                if !required {
                        return invalid("approved_by", "Роль «"+approval.Role+"» не входит в требуемые роли определения "+definition.ID+" ("+joinDictionary(definition.RequiredApproverRoles)+").")
                }
                key := approval.ParticipantID.String() + "\x00" + approval.Role
                if seen[key] {
                        return invalid("approved_by", "Запись approver (участник, роль) продублирована.")
                }
                seen[key] = true
                byRole[approval.Role] = append(byRole[approval.Role], approval.ParticipantID)
        }
        var missing []string
        for _, role := range definition.RequiredApproverRoles {
                if len(byRole[role]) == 0 {
                        missing = append(missing, role)
                }
        }
        if len(missing) > 0 {
                return invalid("approved_by", "Роли без покрытия: "+joinDictionary(missing)+". Каждая требуемая роль определения должна присутствовать в approved_by.")
        }
        if !definition.AllowRoleCoincidence {
                for first := range byRole {
                        for second := range byRole {
                                if first >= second {
                                        continue
                                }
                                for _, left := range byRole[first] {
                                        for _, right := range byRole[second] {
                                                if left == right {
                                                        return invalid("approved_by", "Принципалы ролей «"+first+"» и «"+second+"» совпадают: определение "+definition.ID+" требует различных принципалов (allow_role_coincidence не задан).")
                                                }
                                        }
                                }
                        }
                }
        }
        if len(spec.EvidenceRefs) == 0 {
                return invalid("evidence_refs", "PASS требует непустые evidence-ссылки (каждая 1–300 символов): вербальное заверение, mock или sandbox success никогда не меняют состояние.")
        }
        for _, reference := range spec.EvidenceRefs {
                if !length(reference, 1, 300) {
                        return invalid("evidence_refs", "Каждая evidence-ссылка: 1–300 символов.")
                }
        }
        if !validSHA256Hex(spec.EvidenceHash) {
                return invalid("evidence_hash", "Hash доказательства: 64 hex-символа sha256.")
        }
        return nil
}

// validSHA256Hex checks the 64 lowercase hexadecimal characters.
func validSHA256Hex(value string) bool {
        if len(value) != 64 {
                return false
        }
        for _, symbol := range value {
                if !strings.ContainsRune("0123456789abcdef", symbol) {
                        return false
                }
        }
        return true
}

// ---------------------------------------------------------------------------
// Readiness derivation (a pure function of live facts, never cached)
// ---------------------------------------------------------------------------

// AdmissionFact is the caller-authority entitlement admission projection:
// the derived answer of 1-11 (ok / blocked_new_admissions / degraded) with
// its honest reason. The employee surface reads it through the narrow fact
// port under its own verified scope.
type AdmissionFact struct {
        Status string `json:"status"`
        Reason string `json:"reason"`
}

// ReadinessFacts are the live facts the derivation reads: the approved
// operating profile, the sealed manifest it pins (with the honest payload
// read), the latest sealed manifest (for applicability), the current
// connection facts (drift detection against the sealed snapshot), the
// entitlement admission fact, the pack journeys of the manifest's pack
// reference and the full gate instance history. Readiness is derived on
// every read — a stored "computed" status does not exist.
type ReadinessFacts struct {
        Approved           *ProfileVersion
        ManifestRead       *ManifestRead
        LatestManifest     *ManifestLink
        CurrentConnections []ConnectionFacts
        Entitlement        AdmissionFact
        PackJourneys       []string
        Instances          []GateInstance
}

// ReadinessElement is one mandatory element of the release candidate with
// its honest state, reason and the source of the fact. A missing element is
// a recorded absence — the publication does not exist for the stage.
type ReadinessElement struct {
        Key    string `json:"key"`
        Title  string `json:"title"`
        State  string `json:"state"`
        Reason string `json:"reason"`
        Source string `json:"source"`
}

// RouteBlock is one honest blocking of a route: the cause, the reason, the
// unlock owner, the required proof and the recommended next step. A blocked
// route never hides behind an average or aggregate status.
type RouteBlock struct {
        Cause            string `json:"cause"`
        Reason           string `json:"reason"`
        UnlockOwner      string `json:"unlock_owner"`
        RequiredEvidence string `json:"required_evidence"`
        NextStep         string `json:"next_step"`
}

// RouteAdmission is the derived admission of one route for a stage:
// admitted or blocked with every concrete reason. Permission of one route
// never extends to another route, channel, modality or class.
type RouteAdmission struct {
        Route           Route        `json:"route"`
        Modality        string       `json:"modality"`
        AutonomyClasses []string     `json:"autonomy_classes"`
        State           string       `json:"state"`
        Reasons         []RouteBlock `json:"reasons,omitempty"`
}

// ReadinessReport is the whole derivation for one requested stage: the
// element checklist, the per-route admissions and the overall status
// (incomplete / blocked / ready). The technical success of a single
// component never becomes the readiness of the whole.
type ReadinessReport struct {
        Stage      string             `json:"stage"`
        Status     string             `json:"status"`
        StatusText string             `json:"status_text"`
        Elements   []ReadinessElement `json:"elements"`
        Routes     []RouteAdmission   `json:"routes"`
}

// readinessTitles are the fixed display titles of the element dictionary.
var readinessTitles = map[string]string{
        "operating_profile": "Утверждённый Operating Profile",
        "release_manifest":  "Release Manifest",
        "brief":             "Brief",
        "domain_pack":       "Domain Pack",
        "knowledge":         "Знания",
        "behavior":          "Поведение",
        "voice":             "Голос",
        "connections":       "Connections",
        "capabilities":      "Capabilities",
        "entitlement":       "Entitlement",
        "human_coverage":    "Человеческое покрытие",
        "consent":           "Privacy и consent (OQ-11)",
        "eval":              "Eval",
        "gates":             "Gates стадии",
}

// routeAutonomyClasses derives the autonomy classes of one route from the
// profile's coverage matrix cells of that route (declared, sorted,
// distinct) — the classes the admission decision carries per route.
func routeAutonomyClasses(content ProfileContent, route Route) []string {
        seen := map[string]bool{}
        for _, cell := range content.Matrix {
                if cell.Route == route {
                        seen[cell.AutonomyClass] = true
                }
        }
        out := make([]string, 0, len(seen))
        for class := range seen {
                out = append(out, class)
        }
        sort.Strings(out)
        return out
}

// DeriveReadiness is the pure derivation of the candidate state for one
// stage: every mandatory element with its source, every route with its
// honest blocks, the overall status as the conjunction. Nothing is cached:
// stale capability facts, a newer manifest or an expired instance reflect
// at the next read immediately.
func DeriveReadiness(facts ReadinessFacts, stage string) ReadinessReport {
        report := ReadinessReport{Stage: stage, Status: ReadinessBlocked, StatusText: "", Elements: []ReadinessElement{}, Routes: []RouteAdmission{}}
        element := func(key, state, reason, source string) {
                report.Elements = append(report.Elements, ReadinessElement{Key: key, Title: readinessTitles[key], State: state, Reason: reason, Source: source})
        }
        if _, known := StageIndex(stage); !known {
                report.Status = ReadinessIncomplete
                report.StatusText = "Неизвестная стадия: словарь стадий фиксирован (synthetic → closed_account_test → controlled_real_customer → live)."
                return report
        }
        // Prerequisite: the approved profile (admission consumes the immutable
        // approved version, never the mutable draft).
        if facts.Approved == nil {
                element("operating_profile", ElementMissing, "Утверждённой версии Operating Profile нет: черновик редактируется, допуск не затрагивается. Публикация невозможна.", "operating_profiles")
                report.Status = ReadinessIncomplete
                report.StatusText = "Обязательный элемент отсутствует: утверждённый Operating Profile."
                return report
        }
        applicability := DeriveProfileApplicability(facts.Approved, nil, facts.LatestManifest)
        profileSource := fmt.Sprintf("operating_profiles v%d (digest %s…)", facts.Approved.Version, firstDigestRunes(facts.Approved.Digest))
        switch applicability.State {
        case ProfileApplicabilityCurrent:
                element("operating_profile", ElementPass, "Утверждённая версия применима к указанной конфигурационной версии манифеста.", profileSource)
        case ProfileApplicabilityStale:
                element("operating_profile", ElementBlocked, applicability.Reason, profileSource)
        default:
                element("operating_profile", ElementBlocked, applicability.Reason, profileSource)
        }
        manifestSource := fmt.Sprintf("release_manifests config v%d", facts.Approved.Manifest.ConfigVersion)
        var refs ConfigRefs
        manifestOK := false
        if facts.ManifestRead == nil {
                element("release_manifest", ElementMissing, "Запечатанный манифест, на который ссылается утверждённый профиль, не найден: привязка только к точной запечатанной версии.", manifestSource)
        } else if facts.ManifestRead.Status != QuarantineNone {
                element("release_manifest", ElementBlocked, "Манифест в quarantine ("+facts.ManifestRead.Status+"): "+facts.ManifestRead.Reason+" Молчаливое доверие исключено.", manifestSource)
        } else {
                manifestOK = true
                refs = facts.ManifestRead.Content.Config.Refs
                element("release_manifest", ElementPass, fmt.Sprintf("Манифест читается, digest сверен (config v%d, digest %s…).", facts.ManifestRead.Content.Config.Version, firstDigestRunes(facts.ManifestRead.Digest)), manifestSource)
        }
        refState := func(present bool, title, note string) (string, string) {
                if !manifestOK {
                        return ElementBlocked, title + ": манифест не читается — состояние ссылки неизвестно."
                }
                if present {
                        return ElementPass, title + " запечатан в манифесте" + note + "."
                }
                return ElementMissing, title + " отсутствует в манифесте: обязательный элемент релиз-кандидата не заполнен."
        }
        state, reason := refState(refs.Brief.Status == RefPresent, "Brief", fmt.Sprintf(" (draft v%d)", refs.Brief.DraftVersion))
        element("brief", state, reason, manifestSource)
        state, reason = refState(refs.Pack.Status == RefPresent, "Domain Pack", fmt.Sprintf(" (%s@%s)", refs.Pack.ID, refs.Pack.Version))
        element("domain_pack", state, reason, manifestSource)
        state, reason = refState(refs.Knowledge.Status == RefPresent && len(refs.Knowledge.Sources) > 0, "Знания", fmt.Sprintf(" (%d источников)", len(refs.Knowledge.Sources)))
        element("knowledge", state, reason, manifestSource)
        state, reason = refState(refs.Behavior.Status == RefPresent, "Поведение", fmt.Sprintf(" (v%d)", refs.Behavior.Version))
        element("behavior", state, reason, manifestSource)
        hasVoiceRoute := false
        for _, route := range facts.Approved.Content.WorkScope.Routes {
                if route.IsVoice() {
                        hasVoiceRoute = true
                        break
                }
        }
        if !hasVoiceRoute {
                element("voice", ElementNotApplicable, "Голосовых маршрутов в профиле нет: голосовой элемент неприменим (разрешение не расширяется).", profileSource)
        } else {
                state, reason = refState(refs.Voice.Status == RefPresent, "Голос", fmt.Sprintf(" (v%d)", refs.Voice.Version))
                element("voice", state, reason, manifestSource)
        }
        sealedConnections := 0
        if manifestOK {
                sealedConnections = len(facts.ManifestRead.Content.Connections)
        }
        if sealedConnections == 0 {
                element("connections", ElementMissing, "В запечатанном манифесте нет ни одного подключения: обязательный элемент не заполнен.", manifestSource)
        } else {
                element("connections", ElementPass, fmt.Sprintf("Запечатано подключений: %d (идентификаторы, версии контрактов, состояния).", sealedConnections), manifestSource)
        }
        // Capabilities: the CURRENT facts (drift against the sealed snapshot
        // shows immediately — the derivation never trusts the sealed state
        // alone).
        if len(facts.CurrentConnections) == 0 {
                element("capabilities", ElementMissing, "Текущие факты подключений пусты: хотя бы одно действующее подключение с подтверждённой (verified) capability требуется.", "connector (текущие факты)")
        } else {
                // The acting capability: at least one current connection has a
                // capability proven by live evidence — "connected" alone never
                // suffices (the technical success of a single component is not
                // the readiness of the whole), and one proven connection never
                // hides the others: every connection without an acting
                // capability is listed honestly (revoked and failed are
                // terminal or unproven states of their own — the AG-4 gate of
                // the stages from controlled_real_customer up carries the full
                // dependency discipline, not this element).
                var unproven []string
                var notes []string
                acting := 0
                for _, connection := range facts.CurrentConnections {
                        connectionActing := false
                        for _, capability := range connection.Capabilities {
                                if !capability.Gap && capability.State == "verified" {
                                        connectionActing = true
                                        continue
                                }
                                notes = append(notes, fmt.Sprintf("подключение %s · %s: %s%s", connection.ConnectionID, capability.Capability, capability.State, capabilityReason(capability)))
                        }
                        if connectionActing {
                                acting++
                        } else {
                                unproven = append(unproven, fmt.Sprintf("подключение %s (%s): ни одной подтверждённой capability (connected ≠ доказано)", connection.ConnectionID, connection.Status))
                        }
                }
                if acting > 0 {
                        noteSuffix := ""
                        if len(notes) > 0 {
                                noteSuffix = " Непроверенные остатки честно видны: " + joinDictionary(notes[:minInt(len(notes), 4)]) + honestSuffix(len(notes)) + " — их дисциплина: gate AG-4 стадий от controlled_real_customer."
                        }
                        element("capabilities", ElementPass, fmt.Sprintf("Действующее подключение с подтверждённой (verified) capability есть (%d из %d)."+noteSuffix, acting, len(facts.CurrentConnections)), "connector (текущие факты)")
                } else {
                        element("capabilities", ElementBlocked, "Ни одно подключение не имеет действующей подтверждённой capability (текущие факты, не запечатанные): "+joinDictionary(unproven[:minInt(len(unproven), 4)])+honestSuffix(len(unproven)), "connector (текущие факты)")
                }
        }
        capabilitiesBlocked := len(facts.CurrentConnections) > 0 && !anyActingCapability(facts.CurrentConnections)
        // Entitlement: shown honestly for every stage; it blocks the
        // publication from closed_account_test up (the words of the 1-11
        // derivation), synthetic is the documented fixture exception.
        switch facts.Entitlement.Status {
        case "ok":
                element("entitlement", ElementPass, "Entitlement admission: ok — новые admission и публикации не блокируются коммерческим периметром.", "entitlement DeriveAdmission (факт-порт)")
        default:
                element("entitlement", ElementBlocked, "Entitlement admission: "+facts.Entitlement.Status+" — "+facts.Entitlement.Reason+" Блокирует публикации стадий от closed_account_test (synthetic — исключение: фикстуры).", "entitlement DeriveAdmission (факт-порт)")
        }
        entitlementBlocked := facts.Entitlement.Status != "ok"
        // Human coverage: the per-route states of the approved profile.
        routeStates := DeriveRouteStates(facts.Approved.Content)
        var blockedRoutes []string
        for _, routeState := range routeStates {
                if routeState.State == RouteStateBlocked {
                        blockedRoutes = append(blockedRoutes, routeState.Route.String())
                }
        }
        if len(blockedRoutes) == 0 {
                element("human_coverage", ElementPass, "Все разрешённые маршруты имеют обязательное человеческое покрытие.", profileSource)
        } else {
                element("human_coverage", ElementBlocked, "Маршруты без полного покрытия: "+joinDictionary(blockedRoutes)+" (профиль утверждён, маршруты честно заблокированы per-route).", profileSource)
        }
        // Consent: the OQ-11 states of the in-use privacy assignments.
        var consentOpen []string
        for _, assignment := range facts.Approved.Content.Privacy {
                if assignment.InUse && assignment.OQ11State == OQ11Open {
                        consentOpen = append(consentOpen, assignment.Route.String()+" · "+assignment.Purpose)
                }
        }
        if len(consentOpen) == 0 {
                element("consent", ElementPass, "Все используемые privacy-назначения в состоянии OQ-11 pass (purpose-specific consent подтверждён).", profileSource)
        } else {
                element("consent", ElementBlocked, "OQ-11 open: "+joinDictionary(consentOpen[:minInt(len(consentOpen), 4)])+honestSuffix(len(consentOpen)), profileSource)
        }
        // Eval: the honest linked-not-executed state of the coverage matrix
        // links (no eval surfaces exist: Epic 6).
        covered := 0
        for _, cell := range DeriveMatrixCoverage(facts.Approved.Content, facts.PackJourneys) {
                if cell.State == MatrixCellCovered {
                        covered++
                }
        }
        if covered > 0 {
                element("eval", ElementLinkedNotExecuted, fmt.Sprintf("Eval-ссылки матрицы связаны (%d покрытых ячеек), исполнений нет: поверхностей eval в системе нет (Epic 6) — стадии от controlled честно заблокированы.", covered), profileSource)
        } else {
                element("eval", ElementMissing, "Матрица покрытия не содержит допущенных ячеек с eval-ссылками: обязательный элемент не заполнен.", profileSource)
        }
        evalRequired := StageRequiresEval(stage)
        // Gates of the stage: the element derives from the profile's ACTUAL
        // routes — the union of the per-route requirements (voice routes add
        // OQ-12/OQ-13B, live adds OQ-14/OQ-7B) resolved per route, so the
        // aggregate never disagrees with the route table (a text-only profile
        // never lists voice gates; a route-scoped PASS never satisfies another
        // route's requirement).
        if !StageRequiresGates(stage) {
                element("gates", ElementNotApplicable, "Стадия "+stage+" не требует gate PASS (синтетика и закрытые тестовые аккаунты до OQ-11 PASS — словарь реестра).", "gate_instances + словарь домена")
        } else {
                required := map[string][]Route{}
                for _, routeState := range routeStates {
                        for _, gateID := range RequiredGateIDs(stage, routeState.Route) {
                                required[gateID] = append(required[gateID], routeState.Route)
                        }
                }
                gateIDs := make([]string, 0, len(required))
                for gateID := range required {
                        gateIDs = append(gateIDs, gateID)
                }
                sort.Strings(gateIDs)
                var gateProblems []string
                for _, gateID := range gateIDs {
                        definition, _ := GateDefinitionByID(gateID)
                        for _, route := range required[gateID] {
                                if ValidPassForRoute(gateID, facts.Instances, route) != nil {
                                        continue
                                }
                                state, reason := InstanceStateForRoute(gateID, facts.Instances, route)
                                gateProblems = append(gateProblems, fmt.Sprintf("%s · маршрут %s: %s (%s) — safe state: %s", gateID, route, state, reason, definition.SafeState))
                        }
                }
                if len(gateProblems) == 0 {
                        element("gates", ElementPass, "Все требования стадии по всем маршрутам профиля имеют действующие применимые unsuperseded PASS.", "gate_instances + словарь домена")
                } else {
                        element("gates", ElementBlocked, joinDictionary(gateProblems), "gate_instances + словарь домена")
                }
        }
        // Per-route admission: the conjunction of the profile route state, the
        // current capabilities, the entitlement (stage-dependent), the eval
        // element (stage-dependent) and every applicable gate of the stage.
        for _, routeState := range routeStates {
                route := routeState.Route
                admission := RouteAdmission{Route: route, Modality: ModalityOfRoute(route), AutonomyClasses: routeAutonomyClasses(facts.Approved.Content, route), State: RouteAdmitted, Reasons: []RouteBlock{}}
                if routeState.State == RouteStateBlocked {
                        admission.Reasons = append(admission.Reasons, RouteBlock{
                                Cause: "route_state", Reason: routeState.Reason,
                                UnlockOwner: routeState.UnlockOwner, RequiredEvidence: "заполненное human coverage (Response Owner, часы, fallback с capability evidence) и OQ-11 pass",
                                NextStep: "исправить профиль и утвердить новую версию (новый кандидат — новая лестница публикаций)",
                        })
                }
                if capabilitiesBlocked {
                        admission.Reasons = append(admission.Reasons, RouteBlock{
                                Cause: "capability", Reason: "Подключение без действующей подтверждённой (verified) capability: маршрут fail-closed — connected ≠ доказано (текущие факты).",
                                UnlockOwner: "Owner/Configurator", RequiredEvidence: "capability evidence с актуальной применимостью",
                                NextStep: "провести verification capability и запечатать новый манифест",
                        })
                }
                if StageRequiresEntitlementAdmission(stage) && entitlementBlocked {
                        admission.Reasons = append(admission.Reasons, RouteBlock{
                                Cause: "entitlement", Reason: facts.Entitlement.Reason,
                                UnlockOwner: "Owner", RequiredEvidence: "активный (не draft/paused/hard limit) коммерческий периметр",
                                NextStep: "активировать или возобновить Entitlement (compose terms / state transition / budget)",
                        })
                }
                if evalRequired {
                        admission.Reasons = append(admission.Reasons, RouteBlock{
                                Cause: "eval", Reason: "Eval linked, not executed: поверхностей исполнения eval в системе нет (Epic 6); числовых результатов не существует.",
                                UnlockOwner: "Product Owner + Operations/QA", RequiredEvidence: "исполненные eval-прогоны с результатами против порогов",
                                NextStep: "дождаться eval-поверхностей (Epic 6) — до этого стадии от controlled честно заблокированы",
                        })
                }
                for _, gateID := range RequiredGateIDs(stage, route) {
                        definition, _ := GateDefinitionByID(gateID)
                        if pass := ValidPassForRoute(gateID, facts.Instances, route); pass == nil {
                                state, reason := InstanceStateForRoute(gateID, facts.Instances, route)
                                admission.Reasons = append(admission.Reasons, RouteBlock{
                                        Cause: "gate:" + gateID, Reason: "Нет действующего применимого PASS: " + state + " — " + reason + ". Safe state: " + definition.SafeState + ".",
                                        UnlockOwner: " approver-роли: " + joinDictionary(definition.RequiredApproverRoles), RequiredEvidence: definition.EvidenceRequirement,
                                        NextStep: "записать PASS gate-инстанса с полным покрытием ролей, evidence-ссылками и hash",
                                })
                        }
                }
                if len(admission.Reasons) > 0 {
                        admission.State = RouteBlocked
                }
                report.Routes = append(report.Routes, admission)
        }
        // The overall status is the conjunction: a missing element makes the
        // whole candidate incomplete; a blocked element makes it blocked (the
        // one documented exception: the entitlement element shows its honest
        // draft/paused/blocked state at synthetic but never blocks the
        // synthetic publication — fixtures do not consume commercial surfaces);
        // otherwise at least one admitted route makes it ready, zero admitted
        // routes make it blocked. A technical success of a single component
        // never becomes the readiness of the whole, and no block hides behind
        // an aggregate.
        missing := 0
        var blockedTitles []string
        for _, item := range report.Elements {
                switch item.State {
                case ElementMissing:
                        missing++
                case ElementBlocked:
                        if item.Key == "entitlement" && stage == StageSynthetic {
                                continue
                        }
                        blockedTitles = append(blockedTitles, item.Title)
                }
        }
        admitted := 0
        for _, route := range report.Routes {
                if route.State == RouteAdmitted {
                        admitted++
                }
        }
        switch {
        case missing > 0:
                report.Status = ReadinessIncomplete
                report.StatusText = fmt.Sprintf("Обязательные элементы не заполнены (%d): публикация невозможна — технический успех компонента не превращается в готовность целого.", missing)
        case len(blockedTitles) > 0:
                report.Status = ReadinessBlocked
                report.StatusText = "Обязательные элементы заблокированы: " + joinDictionary(blockedTitles) + ". Блокировка элемента не скрывается за статусом маршрутов."
        case admitted == 0:
                report.Status = ReadinessBlocked
                report.StatusText = "Все маршруты заблокированы: пустых публикаций не существует (ноль допущенных маршрутов — честный отказ)."
        default:
                report.Status = ReadinessReady
                report.StatusText = fmt.Sprintf("Готов к публикации стадии %s: элементов заполнено %d, допущено маршрутов %d из %d.", stage, len(report.Elements), admitted, len(report.Routes))
        }
        return report
}

// capabilityReason formats the honest state reason of a capability.
func capabilityReason(capability CapabilityFact) string {
        if capability.Reason != "" {
                return " — " + capability.Reason
        }
        return ""
}

// anyActingCapability reports whether at least one current connection has a
// capability proven by live evidence (verified, no gap) — the acting
// capability of the spec: "at least one acting connection with a proven
// capability". The connections without any proven capability stay visible in
// the element reason (the aggregate never hides them), and a stale capability
// is listed even when the connection keeps another acting one.
func anyActingCapability(connections []ConnectionFacts) bool {
        for _, connection := range connections {
                for _, capability := range connection.Capabilities {
                        if !capability.Gap && capability.State == "verified" {
                                return true
                        }
                }
        }
        return false
}

// firstDigestRunes renders the first 12 runes of a digest for display.
func firstDigestRunes(digest string) string {
        runes := []rune(digest)
        if len(runes) > 12 {
                runes = runes[:12]
        }
        return string(runes)
}

func minInt(left, right int) int {
        if left < right {
                return left
        }
        return right
}

func honestSuffix(total int) string {
        if total > 4 {
                return fmt.Sprintf(" (и ещё %d)", total-4)
        }
        return ""
}

// ---------------------------------------------------------------------------
// Release admission decisions (append-only publications)
// ---------------------------------------------------------------------------

// AdmissionConflictError is the honest loss of a publication race: the
// acting decision of the same candidate and stage already exists — exactly
// one record wins, the competitor receives the current state.
type AdmissionConflictError struct {
        Stage   string
        Current *ReleaseAdmission
}

func (e AdmissionConflictError) Error() string {
        return "release admission conflict: the decision of stage " + e.Stage + " already acts for this candidate"
}
func (e AdmissionConflictError) Unwrap() error { return ErrConflict }

// ReleaseAdmission is the append-only record of the controlled publication
// decision: the stage, the exact manifest and profile digests of the
// candidate, the per-route results, the actor with the approver roles at
// decision time, the step-up fact and the canonical digest of the record.
// It never contains secrets, raw PII, transcripts or protected media —
// identifiers, states, reasons and digests only.
type ReleaseAdmission struct {
        TenantID       id.UUID          `json:"tenant_id"`
        EmployeeID     id.UUID          `json:"employee_id"`
        DecisionNo     int64            `json:"decision_no"`
        Stage          string           `json:"stage"`
        Manifest       ManifestLink     `json:"manifest"`
        ProfileVersion int64            `json:"profile_version"`
        ProfileDigest  string           `json:"profile_digest"`
        Routes         []RouteAdmission `json:"routes"`
        ActorID        id.UUID          `json:"actor_id"`
        ApproverRoles  []string         `json:"approver_roles"`
        StepUpVerified bool             `json:"step_up_verified"`
        DecidedAt      time.Time        `json:"decided_at"`
        Digest         string           `json:"digest"`
}

// CandidateKey identifies the release candidate: the exact manifest link
// plus the approved profile version (its digest follows from the
// append-only version). Stage progression compares the exact digests — a
// decision of another candidate is never the previous stage.
type CandidateKey struct {
        Manifest       ManifestLink
        ProfileVersion int64
        ProfileDigest  string
}

// CandidateOf derives the candidate key of an admission record.
func CandidateOf(admission ReleaseAdmission) CandidateKey {
        return CandidateKey{Manifest: admission.Manifest, ProfileVersion: admission.ProfileVersion, ProfileDigest: admission.ProfileDigest}
}

// SameCandidate compares the exact digests of two candidate keys.
func (k CandidateKey) SameCandidate(other CandidateKey) bool {
        return k.Manifest.Equal(other.Manifest) && k.ProfileVersion == other.ProfileVersion && k.ProfileDigest == other.ProfileDigest
}

// DeriveActiveRelease derives the acting release pointer: the record with
// the highest decision number (the budget-current pattern of 1-11; there is
// no rollback, the pointer grows monotonically).
func DeriveActiveRelease(admissions []ReleaseAdmission) *ReleaseAdmission {
        var active *ReleaseAdmission
        for index := range admissions {
                if active == nil || admissions[index].DecisionNo > active.DecisionNo {
                        active = &admissions[index]
                }
        }
        return active
}

// ValidateStageProgression checks the ladder rule for the requested stage
// against the candidate's OWN decision history (the exact manifest and
// profile digests): a closed stage N requires a recorded decision of stage
// N-1 of the same candidate — decisions are append-only and never rollback,
// so a recorded decision of the same candidate stays "acting" for the
// ladder forever. Interleaved candidates never interfere: the globally
// latest decision of another candidate is irrelevant to this one. A repeat
// of a stage the candidate already recorded is an honest conflict (exactly
// one record per candidate and stage); a request below the candidate's
// highest recorded stage is a disguised rollback (rollback is deferred); a
// skip above the ladder names the required previous stage and the
// candidate's acting decision.
func ValidateStageProgression(requested string, candidate CandidateKey, history []ReleaseAdmission) error {
        requestedIndex, known := StageIndex(requested)
        if !known {
                return ProfileRuleError{Section: "publication", Field: "stage", Reason: "Неизвестная стадия: synthetic, closed_account_test, controlled_real_customer или live.", Kind: ProfileProblemInvalid}
        }
        mine := make([]ReleaseAdmission, 0, len(history))
        for _, admission := range history {
                if CandidateOf(admission).SameCandidate(candidate) {
                        mine = append(mine, admission)
                }
        }
        // Exactly one record per candidate and stage: the repeat is a conflict
        // carrying the recorded decision for an honest comparison.
        for index := range mine {
                if mine[index].Stage == requested {
                        recorded := mine[index]
                        return AdmissionConflictError{Stage: recorded.Stage, Current: &recorded}
                }
        }
        highest := (*ReleaseAdmission)(nil)
        highestIndex := -1
        for index := range mine {
                stageIndex, _ := StageIndex(mine[index].Stage)
                if highest == nil || stageIndex > highestIndex {
                        highest = &mine[index]
                        highestIndex = stageIndex
                }
        }
        if highest == nil {
                if requested != StageSynthetic {
                        return ProfileRuleError{Section: "publication", Field: "stage", Reason: "Перескок стадии невозможен: synthetic — первая стадия, решений этого кандидата нет. Опубликуйте synthetic этого кандидата.", Kind: ProfileProblemInvalid}
                }
                return nil
        }
        switch {
        case requestedIndex == highestIndex:
                // Unreachable (the repeat conflict fired above); kept for defense.
                return AdmissionConflictError{Stage: highest.Stage, Current: highest}
        case requestedIndex == highestIndex+1:
                return nil
        case requestedIndex < highestIndex:
                return ProfileRuleError{Section: "publication", Field: "stage", Reason: fmt.Sprintf("Действующее решение этого кандидата — стадия %s (решение №%d): публикация нижней стадии выглядела бы откатом, а rollback отложен. Разрешение не расширяется автоматически.", highest.Stage, highest.DecisionNo), Kind: ProfileProblemInvalid}
        default:
                previous := StageOrder[requestedIndex-1]
                return ProfileRuleError{Section: "publication", Field: "stage", Reason: fmt.Sprintf("Перескок стадии невозможен: требуется действующее решение стадии %s того же кандидата (те же digest'ы манифеста и профиля); действующее решение этого кандидата — %s (№%d, стадия заполнена и действует).", previous, highest.Stage, highest.DecisionNo), Kind: ProfileProblemInvalid}
        }
}

// canonicalizeAdmission returns a copy with the routes in the canonical
// order the digest depends on (sorted by route string; the reasons keep the
// fixed derivation order, the autonomy classes are pre-sorted).
func canonicalizeAdmission(admission ReleaseAdmission) ReleaseAdmission {
        out := admission
        out.Routes = append([]RouteAdmission(nil), admission.Routes...)
        sort.SliceStable(out.Routes, func(i, j int) bool { return out.Routes[i].Route.String() < out.Routes[j].Route.String() })
        out.ApproverRoles = sortedCopy(admission.ApproverRoles)
        // The canonical bytes render the decision time in UTC: the digest is a
        // property of the record, never of the reading process's timezone.
        out.DecidedAt = out.DecidedAt.UTC()
        return out
}

// CanonicalAdmissionJSON serializes the decision deterministically: the
// struct field order is fixed by the encoder, the routes are pre-sorted and
// the digest field itself is excluded from the canonical bytes (it is the
// sha256 of exactly these bytes).
func CanonicalAdmissionJSON(admission ReleaseAdmission) ([]byte, error) {
        out := canonicalizeAdmission(admission)
        out.Digest = ""
        return json.Marshal(out)
}

// AdmissionDigest computes the sha256 hex digest of the canonical bytes.
func AdmissionDigest(admission ReleaseAdmission) (string, error) {
        raw, err := CanonicalAdmissionJSON(admission)
        if err != nil {
                return "", err
        }
        sum := sha256.Sum256(raw)
        return hex.EncodeToString(sum[:]), nil
}

// AdmittedRouteCount counts the admitted routes of a decision.
func AdmittedRouteCount(admission ReleaseAdmission) int {
        count := 0
        for _, route := range admission.Routes {
                if route.State == RouteAdmitted {
                        count++
                }
        }
        return count
}
