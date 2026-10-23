package api

import (
        "context"
        "time"

        casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
        casesPort "github.com/artemiosu/staffy/internal/cases/port"
        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// Case contour scope levels (the readiness precedent): viewing the cases
// surface requires review, configuration or audit; the sealed payload
// requires review or audit — a member without any of them gets an honest
// refusal, a non-member a plain 404 so tenant existence is never
// disclosed.
const (
        CaseScopeView    = "review-or-configuration-or-audit"
        CaseScopePayload = "review-or-audit"
)

// IngestCommand is the decoded, structurally valid ingress delivery. The
// ingress is not a user principal: the tenant scope is derived from the
// verified connection, never from the envelope claim alone.
type IngestCommand struct {
        Envelope       casesDomain.Envelope
        IdempotencyKey string
        CorrelationID  id.UUID
        Now            time.Time
}

// IngestOutcome is the material result of one ingress delivery.
type IngestOutcome struct {
        // Accepted is set when the event (or its identical redelivery) is
        // material.
        Accepted *IngestResult `json:"accepted,omitempty"`
        // Refused is the honest refusal: the ladder kind, the safe text and
        // the bounded counter.
        Refused *IngestRefusal `json:"refused,omitempty"`
        // Quarantined names the quarantine kind and the operational exception
        // raised for the human.
        Quarantined *IngestQuarantine `json:"quarantined,omitempty"`
}

type IngestResult struct {
        EventID         id.UUID `json:"event_id"`
        CaseID          id.UUID `json:"case_id"`
        ConversationID  id.UUID `json:"conversation_id"`
        CaseNo          int64   `json:"case_no"`
        ConversationNo  int64   `json:"conversation_no"`
        ResponseOwnerID id.UUID `json:"response_owner_id"`
        EmployeeID      id.UUID `json:"employee_id"`
        NewCase         bool    `json:"new_case"`
        Replay          bool    `json:"replay"`
}

type IngestRefusal struct {
        Kind    string `json:"kind"`
        Message string `json:"message"`
        Counter int    `json:"counter,omitempty"`
        Bounded bool   `json:"bounded"`
}

type IngestQuarantine struct {
        Kind        string  `json:"kind"`
        ExceptionID id.UUID `json:"exception_id"`
        EventID     id.UUID `json:"event_id"`
}

// CasesView is the page/API projection of the cases list.
type CasesView struct {
        TenantID   id.UUID                            `json:"tenant_id"`
        Cases      []casesDomain.CaseSummary          `json:"cases"`
        Exceptions []casesDomain.OperationalException `json:"operational_exceptions"`
        CanView    bool                               `json:"can_view"`
        ViewScope  string                             `json:"view_scope"`
}

// CaseView is the full page/API projection of one case: the conversations
// with owners and surfaces, the timeline, the DERIVED work items (the Work
// Control Card material) and the related exceptions. The honest statements
// of the slice are part of the view contract; the fact VALUES follow the
// same review/audit boundary as the payload (CanViewFacts).
type CaseView struct {
        TenantID        id.UUID                            `json:"tenant_id"`
        Case            casesDomain.Case                   `json:"case"`
        Conversations   []casesDomain.Conversation         `json:"conversations"`
        Timeline        []casesDomain.TimelineEvent        `json:"timeline"`
        WorkItems       []casesDomain.WorkItemCurrent      `json:"work_items"`
        Exceptions      []casesDomain.OperationalException `json:"operational_exceptions"`
        UntriagedText   string                             `json:"untriaged_note"`
        NoAutoReplyText string                             `json:"no_auto_reply_note"`
        CanViewPayload  bool                               `json:"can_view_payload"`
        CanViewFacts    bool                               `json:"can_view_facts"`
        CanManageWork   bool                               `json:"can_manage_work"`
        ViewScope       string                             `json:"view_scope"`
        // TaxonomyTypes is the journeys dictionary of the pack pinned on the
        // case (the Work Item type dictionary of the Work Control Card forms);
        // empty when the pin has no readable pack (the honest pre-migration
        // state — the classification forms fall back to the text input).
        TaxonomyTypes []string `json:"taxonomy_types"`
        // CriticalEntities is the material key dictionary of the pinned pack:
        // the UI hint of the fact forms (the frozen spec: the hints come from
        // the pack, never a hardcoded list). Empty mirrors TaxonomyTypes — the
        // form degrades to the generic placeholder, honestly.
        CriticalEntities []string `json:"critical_entities"`
        // The Story 2.3 sections: the derived assurance state with the records
        // and the allowed methods, the channel bindings (with the reversals)
        // and the duplicate candidate cards (the masked minimal material).
        Assurance  AssuranceView             `json:"assurance"`
        Bindings   []casesPort.BindingCard   `json:"bindings"`
        Duplicates []casesPort.DuplicateCard `json:"duplicates"`
        // The Story 2.4 section: the derived ownership state with the
        // history, the outgoing replies and the member options of the
        // transfer forms.
        Ownership OwnershipView `json:"ownership"`
        // The Story 2.5 section: the conversation mode derivation with the
        // ladder, the term history, the Work Item contexts and the AI
        // disclosure records with their delivery statuses.
        Mode ModeView `json:"mode"`
        // The Story 2.6 section: the progressive intake proposals and the
        // deterministic Policy Outcomes per Work Item (the CURRENT outcome is
        // the maximal number — the sales reply gate reads it).
        Policy PolicyView `json:"policy"`
        // The Story 2.7 section: the owning-system field observations per
        // Work Item (the CURRENT observation per matrix field with the
        // freshness derivation and the conflict badge against the confirmed
        // fact of the case).
        Observations ObservationView `json:"observations"`
        // The Story 2.8 section: the objection records per Work Item with the
        // CURRENT objection, the lifetime persuasion budget and the stopped
        // word with the allowed endings.
        Objections ObjectionView `json:"objections"`
        // The Story 2.9 section: the commitment versions per Work Item with
        // the readiness derivations, the exact-version confirmations with
        // the validity derivations and the honest declines.
        Commitments CommitmentView `json:"commitments"`
        // The Story 2.10 section: the continuation links of the case with
        // the read-time state derivations, the delivered binding of the
        // outgoing replies, the use accounting against the limit and the
        // event feed (the FR-16 statuses — never stored words).
        Continuations ContinuationView `json:"continuations"`
        // The Story 4.2 section: the vehicle identification panel — the
        // read-time profile of the case under the pack identification schema
        // (the attributes with the versions, the statuses and the
        // confirmations, the completeness by the alternative sets, the
        // ProfileDigest) with the honest degradation when the pinned pack
        // carries no schema (the panel is absent, never a hardcoded set).
        Vehicle VehiclePanelView `json:"vehicle"`
        // The Story 2.11 section: the recorded communication preferences
        // of the case — the CURRENT set (the read-time derivation), the
        // append-only revision history, the key dictionary of the record
        // form and the channel dictionary of the preferred_channel value
        // (an OPERATIONAL setting, never a fact).
        Accommodations AccommodationView `json:"accommodations"`
        // The Story 2.12 section: the degraded route slots of the case —
        // the CURRENT set (the read-time derivation with the honest cause
        // texts), the append-only revision history including the recovery
        // rows and the dictionaries of the record form (an OPERATIONAL
        // record, never a fact).
        RouteDegradations RouteDegradationView `json:"route_degradations"`
        // The Story 2.13 section: the human handoff lifecycle — every
        // append-only request row with the derived lifecycle state and
        // the event history, the honest client status of the LAST request
        // (назначение ≠ принятие — two words, never one) and the
        // dictionaries of the forms (a LIFECYCLE RECORD, never a fact;
        // the overdue state is the read-time derivation).
        Handoff HandoffView `json:"handoff"`
        // The Story 2.16 section: the truthful waiting updates — the client
        // waiting status of the last round in the THREE distinctions of
        // AC-1 and the state of the last update promise (the read-time
        // derivations over ONE consistent snapshot; the block appears only
        // with the promise material — the control branch renders the page
        // byte-for-byte unchanged).
        Waiting WaitingUpdateView `json:"waiting"`
}

// PayloadView is the sealed payload read under review/audit: the digest,
// the seal check result and the payload itself.
type PayloadView struct {
        EventID       id.UUID `json:"event_id"`
        PayloadDigest string  `json:"payload_digest"`
        SealOK        bool    `json:"seal_ok"`
        Payload       []byte  `json:"-"`
}

// DrainResult is the outcome of the idempotent outbox drain command.
type DrainResult struct {
        Considered int `json:"considered"`
        Deferred   int `json:"deferred"`
}

// Service is the application surface consumed by the transport layer.
// IngestEvent is the machine ingress; the reads are the operator surface;
// DrainOutbox is the idempotent delivery command; the Work* commands and
// the facts view are the Story 2.2 surface (idempotent, optimistic).
type Service interface {
        IngestEvent(context.Context, IngestCommand) (IngestOutcome, error)
        CasesView(context.Context, domain.Principal, id.UUID) (CasesView, error)
        CaseView(context.Context, domain.Principal, id.UUID, id.UUID) (CaseView, error)
        ReadPayload(context.Context, domain.Principal, id.UUID, id.UUID, id.UUID) (PayloadView, error)
        DrainOutbox(context.Context, domain.Principal, id.UUID) (DrainResult, error)
        WorkService
        VerificationService
        OwnershipService
        ModeService
        PolicyService
        ObservationService
        ObjectionService
        CommitmentService
}

// Clock and IDs follow the module conventions.
type Clock interface{ Now() time.Time }

type IDGenerator interface {
        New(time.Time) (id.UUID, error)
}

// ---------------------------------------------------------------------------
// Story 2.2: the ten work commands, the facts view and the WCC projections
// ---------------------------------------------------------------------------

// WorkCommandInput is the shared material of one operator work command: the
// actor, the tenant, the case and the work item anchor, the optimistic
// concurrency expectation, the honest reason and the idempotency key.
type WorkCommandInput struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        WorkItemID      id.UUID
        ExpectedVersion int64
        Reason          string
        IdempotencyKey  string
        Now             time.Time
}

// CreateWorkItemCommand creates one typed and titled work item for a new
// intention of an EXISTING conversation (the automatic link does not exist
// as a class — the operator binds events explicitly).
type CreateWorkItemCommand struct {
        Actor          domain.Principal
        TenantID       id.UUID
        CaseID         id.UUID
        ConversationID id.UUID
        Type           string
        Title          string
        Reason         string
        IdempotencyKey string
        Now            time.Time
}

// ClassifyCommand classifies (or re-classifies) one work item: the type
// comes from the taxonomy of the pack pinned on the case, the title and the
// reason are the operator material. The command writes a new version — the
// history is never rewritten.
type ClassifyCommand struct {
        WorkCommandInput
        Type  string
        Title string
}

// TransitionCommand moves one work item along the state ladder; the
// completion of one work item never closes the conversation or the case.
type TransitionCommand struct {
        WorkCommandInput
        ToState string
}

// SplitCommand splits one work item into at least two typed and titled
// targets: the source goes terminal `split`, the targets are born in_work
// with the backward reference.
type SplitCommand struct {
        WorkCommandInput
        Targets []casesDomain.SplitTarget
}

// MergeSourceInput is one merge source with its own optimistic concurrency
// expectation.
type MergeSourceInput struct {
        WorkItemID      id.UUID
        ExpectedVersion int64
}

// MergeCommand merges the sources into one target: the sources go terminal
// `merged` with merged_into, the fact scopes move by NEW versions (no fact
// rows are duplicated), the event links are appended onto the target.
type MergeCommand struct {
        WorkCommandInput
        Sources []MergeSourceInput
}

// LinkEventCommand binds one accepted event of the case to one active work
// item — the explicit operator action (there is no automatic link).
type LinkEventCommand struct {
        WorkCommandInput
        EventID id.UUID
}

// DisputeCommand disputes one split/merge (or any other work command
// outcome): the dispute is an append-only operational exception for the
// human, never a rewrite.
type DisputeCommand struct {
        WorkCommandInput
        Note string
}

// RecordFactCommand records one fact: the material key, the canonical value
// with the full provenance. A channel_event source cites the event of this
// case; an operator source carries the operator's own words.
type RecordFactCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        WorkItemID      id.UUID
        MaterialKey     string
        Value           []byte
        SourceType      string
        SourceID        string
        EventChannel    string
        EventObservedAt time.Time
        EventReceivedAt time.Time
        Reason          string
        IdempotencyKey  string
        Now             time.Time
}

// ConfirmFactVersionCommand confirms the EXACT version of one fact: the
// confirmer of this slice is operator_relay, the evidence is an accepted
// event of the case or a note.
type ConfirmFactVersionCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        FactID          id.UUID
        VersionNo       int64
        ConfirmerType   string
        EvidenceEventID id.UUID
        Note            string
        IdempotencyKey  string
        Now             time.Time
}

// RescopeFactCommand moves the scope of one fact to another active work
// item of the same case by writing a NEW version (the fact anchor and the
// rows are never duplicated).
type RescopeFactCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        FactID          id.UUID
        TargetWorkItem  id.UUID
        ExpectedVersion int64
        Reason          string
        IdempotencyKey  string
        Now             time.Time
}

// WorkOutcome is the stored and replayed material result of one work
// command: the command name, the material identifiers it wrote and the
// replay marker of an idempotent redelivery.
type WorkOutcome struct {
        Command     string    `json:"command"`
        Replay      bool      `json:"replay"`
        WorkItemID  id.UUID   `json:"work_item_id,omitempty"`
        Version     int64     `json:"version,omitempty"`
        State       string    `json:"state,omitempty"`
        Type        string    `json:"type,omitempty"`
        Title       string    `json:"title,omitempty"`
        EventID     id.UUID   `json:"event_id,omitempty"`
        TargetIDs   []id.UUID `json:"target_ids,omitempty"`
        SourceIDs   []id.UUID `json:"source_ids,omitempty"`
        MergedInto  id.UUID   `json:"merged_into,omitempty"`
        FactID      id.UUID   `json:"fact_id,omitempty"`
        FactVersion int64     `json:"fact_version,omitempty"`
        FactStatus  string    `json:"fact_status,omitempty"`
        ExceptionID id.UUID   `json:"exception_id,omitempty"`
        // The Story 2.3 material: the verification, binding and duplicate
        // identifiers with the canonical outcome, the assurance level, the
        // resolution decision and the client confirmation text.
        VerificationID       id.UUID `json:"verification_id,omitempty"`
        VerificationKind     string  `json:"verification_kind,omitempty"`
        VerificationOutcome  string  `json:"verification_outcome,omitempty"`
        AssuranceLevel       string  `json:"assurance_level,omitempty"`
        BindingID            id.UUID `json:"binding_id,omitempty"`
        DuplicateCandidateID id.UUID `json:"duplicate_candidate_id,omitempty"`
        DuplicateDecision    string  `json:"duplicate_decision,omitempty"`
        ConfirmationText     string  `json:"confirmation_text,omitempty"`
        // The Story 2.4 material: the epoch of the ownership transition,
        // the transfer request, the reply and its outcome, and the honest
        // gate refusal of the client reply (the current owner, the epoch
        // and the permitted path — the UI shows it directly).
        Epoch             int64   `json:"epoch,omitempty"`
        TransferRequestID id.UUID `json:"transfer_request_id,omitempty"`
        TransferDecision  string  `json:"transfer_decision,omitempty"`
        ReplyID           id.UUID `json:"reply_id,omitempty"`
        ReplyState        string  `json:"reply_state,omitempty"`
        GateOwnerKind     string  `json:"gate_owner_kind,omitempty"`
        GateOwnerName     string  `json:"gate_owner_name,omitempty"`
        GateCurrentEpoch  int64   `json:"gate_current_epoch,omitempty"`
        GatePermittedPath string  `json:"gate_permitted_path,omitempty"`
        // The Story 2.5 material: the mode term of the transition, the
        // disclosure record with its number, the delivery event kind and the
        // reply class the producer marked.
        ModeTermID   id.UUID `json:"mode_term_id,omitempty"`
        ModeVersion  int64   `json:"mode_version,omitempty"`
        ModeScope    string  `json:"mode_scope,omitempty"`
        DisclosureID id.UUID `json:"disclosure_id,omitempty"`
        DisclosureNo int64   `json:"disclosure_no,omitempty"`
        DeliveryKind string  `json:"delivery_kind,omitempty"`
        ReplyClass   string  `json:"reply_class,omitempty"`
        // The Story 2.6 material: the proposal of the submission, the
        // outcome of the evaluation (the class word) and the Policy
        // Outcome the sales reply passed through.
        ProposalID      id.UUID `json:"proposal_id,omitempty"`
        OutcomeID       id.UUID `json:"outcome_id,omitempty"`
        OutcomeNo       int64   `json:"outcome_no,omitempty"`
        OutcomeClass    string  `json:"outcome_class,omitempty"`
        PolicyOutcomeID id.UUID `json:"policy_outcome_id,omitempty"`
        // The Story 2.7 material: the observation of the owning-system read
        // (the id, the status word with the reason and the carrier words of
        // the value — the projection value itself never travels in the
        // outcome).
        ObservationID         id.UUID `json:"observation_id,omitempty"`
        ObservationStatus     string  `json:"observation_status,omitempty"`
        ObservationReason     string  `json:"observation_reason,omitempty"`
        ObservationExternalID string  `json:"observation_external_id,omitempty"`
        ObservationVersion    string  `json:"observation_external_version,omitempty"`
        // The Story 2.8 material: the objection record of the command and the
        // step word the sales reply passed through.
        ObjectionID     id.UUID `json:"objection_id,omitempty"`
        ObjectionNo     int64   `json:"objection_no,omitempty"`
        ObjectionType   string  `json:"objection_type,omitempty"`
        ObjectionSignal string  `json:"objection_signal,omitempty"`
        PlaybookStep    string  `json:"playbook_step,omitempty"`
        // The Story 2.9 material: the commitment version of the command (the
        // id, the monotonic number, the status word), the confirmation with
        // its subject kind and the decline with its outcome word (the
        // readiness and the validity are the read-time derivations — they
        // never travel in the stored outcome).
        CommitmentID      id.UUID `json:"commitment_id,omitempty"`
        CommitmentVersion int64   `json:"commitment_version,omitempty"`
        CommitmentStatus  string  `json:"commitment_status,omitempty"`
        ConfirmationID    id.UUID `json:"confirmation_id,omitempty"`
        SubjectKind       string  `json:"subject_kind,omitempty"`
        DeclineID         id.UUID `json:"decline_id,omitempty"`
        DeclineOutcome    string  `json:"decline_outcome,omitempty"` // The Story 2.10 material: the continuation link of the issued or
        // revoked command (the token NEVER enters the stored outcome — the
        // replay shows the honest «выдан» mask and the empty URL), the
        // issuance flag with the expiry moment.
        LinkID              id.UUID   `json:"link_id,omitempty"`
        ContinuationIssued  bool      `json:"continuation_issued,omitempty"`
        ContinuationExpires time.Time `json:"continuation_expires_at,omitempty"`
        // The Story 2.11 material: the accommodation record of the command
        // (the record id, the key word and the clearing mark — the revision
        // is the server-side truth of the store, the stored outcome never
        // carries it) and the accuracy qualifier word the sales reply
        // passed through (the row of the reply carries it — the honest
        // limitation word of the outcome's active set).
        AccommodationRecordID id.UUID `json:"accommodation_record_id,omitempty"`
        AccommodationKey      string  `json:"accommodation_key,omitempty"`
        AccommodationCleared  bool    `json:"accommodation_cleared,omitempty"`
        AccuracyQualifier     string  `json:"accuracy_qualifier,omitempty"`
        // The Story 2.12 material: the degradation record of the command
        // (the record id with the cause word and the clearing mark — the
        // revision is the server-side truth of the store, the stored outcome
        // never carries it), the delivery evidence event (the event id with
        // the kind word — the event number is the server-side truth too) and
        // the update promise word the reply passed through (the row of the
        // reply carries it — the honest timing word of the approved policy).
        DegradationRecordID id.UUID `json:"degradation_record_id,omitempty"`
        DegradationCause    string  `json:"degradation_cause,omitempty"`
        DegradationCleared  bool    `json:"degradation_cleared,omitempty"`
        DeliveryEvidenceID  id.UUID `json:"delivery_evidence_id,omitempty"`
        DeliveryKindWord    string  `json:"delivery_evidence_kind,omitempty"`
        UpdatePromise       string  `json:"update_promise,omitempty"`
        // The Story 2.13 material: the handoff lifecycle words of the six
        // commands (the anchor turn of the unconditional ask and the
        // cancellation, the assignee and the backup of the assignment round
        // with its deadline word, the result word of the return — the
        // request and event numbers are the server-side truth of the store,
        // the stored outcome never carries them; the mode raise reuses the
        // 2.5 fields ModeTermID/ModeVersion, the ownership transition reuses
        // the 2.4 fields Epoch/TransferRequestID).
        HandoffTurnRef     id.UUID `json:"handoff_turn_ref,omitempty"`
        HandoffAssignee    id.UUID `json:"handoff_assignee,omitempty"`
        HandoffBackup      id.UUID `json:"handoff_backup,omitempty"`
        HandoffAckDeadline string  `json:"handoff_ack_deadline,omitempty"`
        HandoffResult      string  `json:"handoff_result,omitempty"`
        // The Story 2.14 material: the request-material words of the summary
        // record (the kind word of the single-word dictionary with the claim
        // counter — the summary TEXT and the claims NEVER enter the stored
        // outcome; the summary number and the watermark are the server-side
        // truth of the store, the stored outcome never carries them).
        SummaryKind   string `json:"summary_kind,omitempty"`
        SummaryClaims int    `json:"summary_claims,omitempty"`
        // The Story 2.15 material: the correction record of the command (the
        // correction id with the OUTCOME word of the domain matrix gate and
        // the span counter — the proposed value and the spans NEVER enter the
        // stored outcome; the correction number is the server-side truth of
        // the store, the stored outcome never carries it; the outcome word is
        // the SERVER decision of the material key — the request never
        // carries it).
        CorrectionID      id.UUID `json:"correction_id,omitempty"`
        CorrectionOutcome string  `json:"correction_outcome,omitempty"`
        CorrectionSpans   int     `json:"correction_spans,omitempty"`
        // The Story 2.16 material: the promise id with the deadline word of
        // the fixed dictionary (the ONLY request-carried material — the
        // honest unknown word included). The promised time and the promise
        // number are the SERVER truths of the locks — the stored outcome
        // NEVER carries them (the reader learns the number from the waiting
        // view of the case page); the client texts NEVER enter the stored
        // outcome (the texts are the read-time render of the dictionary).
        PromiseID           id.UUID `json:"promise_id,omitempty"`
        PromiseDeadlineWord string  `json:"promise_deadline_word,omitempty"`
}

// FactView is one fact of the facts view: the anchor, the derived status,
// the versions with the full provenance (both values of a conflict are
// always present) and the confirmations.
type FactView struct {
        FactID        id.UUID                        `json:"fact_id"`
        MaterialKey   string                         `json:"material_key"`
        Status        string                         `json:"status"`
        NextAction    string                         `json:"next_action"`
        Versions      []casesDomain.FactVersion      `json:"versions"`
        Confirmations []casesDomain.FactConfirmation `json:"confirmations"`
}

// FactsView is the read of the facts of one case: the review/audit boundary
// applies (the values are case content).
type FactsView struct {
        TenantID  id.UUID    `json:"tenant_id"`
        CaseID    id.UUID    `json:"case_id"`
        Facts     []FactView `json:"facts"`
        CanView   bool       `json:"can_view"`
        ViewScope string     `json:"view_scope"`
}

// The command surface of Story 2.2 (all ten commands are idempotent by the
// tenant-scoped key; every anchor mutation carries expected_version).
type WorkService interface {
        CreateWorkItem(context.Context, CreateWorkItemCommand) (WorkOutcome, error)
        ClassifyWorkItem(context.Context, ClassifyCommand) (WorkOutcome, error)
        TransitionWorkItem(context.Context, TransitionCommand) (WorkOutcome, error)
        SplitWorkItem(context.Context, SplitCommand) (WorkOutcome, error)
        MergeWorkItems(context.Context, MergeCommand) (WorkOutcome, error)
        LinkEvent(context.Context, LinkEventCommand) (WorkOutcome, error)
        DisputeWorkItem(context.Context, DisputeCommand) (WorkOutcome, error)
        RecordFact(context.Context, RecordFactCommand) (WorkOutcome, error)
        ConfirmFactVersion(context.Context, ConfirmFactVersionCommand) (WorkOutcome, error)
        RescopeFact(context.Context, RescopeFactCommand) (WorkOutcome, error)
        FactsView(context.Context, domain.Principal, id.UUID, id.UUID) (FactsView, error)
}

// ---------------------------------------------------------------------------
// Story 2.3: the four verification commands and the surface projections
// ---------------------------------------------------------------------------

// StartVerificationCommand records one verification of the case participant:
// the kind, the method, the provider-observed code (the domain maps it into
// the canonical outcome — the provider spelling never lands in a record),
// the declared subject handle, the evidence (an accepted event of the case
// or a note — never a bare code) and the authority scope (the authority
// kind only). There is NO field for a one-time code, a password or a
// secret: the command surface itself refuses to carry one.
type StartVerificationCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        ConversationID  id.UUID
        Kind            string
        Method          string
        ProviderCode    string
        SubjectHandle   string
        AuthorityScope  string
        EvidenceEventID id.UUID
        Note            string
        IdempotencyKey  string
        Now             time.Time
}

// BindChannelCommand links one conversation (the new channel session, born
// with its own case) into an existing conversation and case: the consent
// handoff requires the verified identity (and the authority with its scope
// when the binding is on behalf), the purpose and the evidence of the
// client's agreement.
type BindChannelCommand struct {
        Actor               domain.Principal
        TenantID            id.UUID
        CaseID              id.UUID
        ConversationID      id.UUID
        BoundCaseID         id.UUID
        BoundConversationID id.UUID
        Method              string
        Purpose             string
        OnBehalf            bool
        SubjectHandle       string
        EvidenceEventID     id.UUID
        Note                string
        IdempotencyKey      string
        Now                 time.Time
}

// UnlinkChannelCommand reverses one binding: the append-only reversal keeps
// the whole history (the events, the access records), the derived views
// recompute for the subsequent operations.
type UnlinkChannelCommand struct {
        Actor          domain.Principal
        TenantID       id.UUID
        CaseID         id.UUID
        BindingID      id.UUID
        Reason         string
        IdempotencyKey string
        Now            time.Time
}

// ResolveDuplicateCommand decides one duplicate candidate: the same-client
// decision creates the duplicate-review binding atomically (the resolution
// and the link are one transaction); the separate decision keeps the two
// requests apart. The note is the reviewer's recorded basis.
type ResolveDuplicateCommand struct {
        Actor               domain.Principal
        TenantID            id.UUID
        CaseID              id.UUID
        CandidateID         id.UUID
        Decision            string
        Note                string
        ConversationID      id.UUID
        BoundConversationID id.UUID
        SubjectHandle       string
        IdempotencyKey      string
        Now                 time.Time
}

// AssuranceView is the assurance section of the case view: the derived
// state (identity/authority separately, the step-up flag), the records with
// the canonical outcomes, the allowed methods of the conversation modality
// (the honest step-up list of the refusal) and the fixed fallback options
// dictionary. The absence of a record is the honest none — it never becomes
// a confirmed identity.
type AssuranceView struct {
        State           casesDomain.AssuranceState       `json:"state"`
        Records         []casesDomain.VerificationRecord `json:"records"`
        AllowedMethods  []string                         `json:"allowed_methods"`
        FallbackOptions []string                         `json:"fallback_options"`
        StepUpText      string                           `json:"step_up_text"`
}

// VerificationService is the command surface of Story 2.3 (all four
// commands are idempotent by the tenant-scoped key; the binding gate and
// the step-up threshold are the honest protected steps of the slice).
type VerificationService interface {
        StartVerification(context.Context, StartVerificationCommand) (WorkOutcome, error)
        BindChannel(context.Context, BindChannelCommand) (WorkOutcome, error)
        UnlinkChannel(context.Context, UnlinkChannelCommand) (WorkOutcome, error)
        ResolveDuplicate(context.Context, ResolveDuplicateCommand) (WorkOutcome, error)
}

// ---------------------------------------------------------------------------
// Story 3.3: the external commitment dispatch commands (the first slice)
// ---------------------------------------------------------------------------

// CreateExternalCommitmentCommand is the material of one envelope version
// creation: the dictionary words of the epic (the purpose, the recipient
// kind, the channel of the route, the ack requirement), the content, the
// validity window, the expected next step and the optional 2.9 exact-version
// pin (MANDATORY for the offer purpose). The owner snapshot, the lineage
// numbers and the policy outcome number are the SERVER-side truth — the app
// resolves them from the live case material, never from the command.
type CreateExternalCommitmentCommand struct {
        Actor            domain.Principal
        TenantID         id.UUID
        CaseID           id.UUID
        WorkItemID       id.UUID
        ExtCommitmentID  id.UUID
        Purpose          string
        RecipientKind    string
        Channel          string
        ContentText      string
        ValidUntil       time.Time
        ExpectedNextStep string
        AckRequirement   string
        Pin              casesDomain.ExternalCommitmentPin
        Now              time.Time
}

// ExternalCommitmentOutcome is the material result of one envelope command:
// the envelope identifier with the exact version number and the content
// binding token of the version (the page renders the full material from the
// version row; the outcome carries the anchors only).
type ExternalCommitmentOutcome struct {
        Command         string  `json:"command"`
        ExtCommitmentID id.UUID `json:"ext_commitment_id"`
        VersionNo       int64   `json:"version_no"`
        ContentToken    string  `json:"content_token,omitempty"`
}

// ApproveExternalCommitmentCommand is the material of the approval gate:
// the EXACT version the CURRENT human owner approves with the step-up
// evidence. The approver is the acting principal (the app gate verifies the
// actor IS the current owner — an approval of a departed owner or of an old
// version structurally never authorises the send, AC-9).
type ApproveExternalCommitmentCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        WorkItemID      id.UUID
        ExtCommitmentID id.UUID
        VersionNo       int64
        Now             time.Time
}

// ExternalApprovalOutcome is the material result of the approval: the
// version it approves and whether THIS call wrote the row (false — the
// repeated approval of the same approver, the silent idempotency of the
// UNIQUE).
type ExternalApprovalOutcome struct {
        ExtCommitmentID  id.UUID `json:"ext_commitment_id"`
        VersionNo        int64   `json:"version_no"`
        ApprovalRecorded bool    `json:"approval_recorded"`
}

// EnqueueExternalCommitmentCommand is the material of the queue command:
// the EXACT version the enqueue addresses. The approval of the CURRENT
// human owner and the readiness of the version are the SERVER-side gates —
// both refuse BEFORE the transaction with the honest words; the store
// re-validates both under the advisory locks (the persist-before-emit
// transaction carries no adapter — the network call is physically
// impossible before the commit, AC-3).
type EnqueueExternalCommitmentCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        WorkItemID      id.UUID
        ExtCommitmentID id.UUID
        VersionNo       int64
        Now             time.Time
}

// ExternalEnqueueOutcome is the material result of the enqueue: the version
// that entered the queue (the safe outbox record is the queue evidence —
// the words have NOT left).
type ExternalEnqueueOutcome struct {
        ExtCommitmentID id.UUID `json:"ext_commitment_id"`
        VersionNo       int64   `json:"version_no"`
        Queued          bool    `json:"queued"`
}

// RelayExternalReceiptCommand is the material of the operator relay: the
// observation word of the channel world (the 2.12 ConfirmerOperatorRelay
// precedent — the human passes the machine observation, the machine never
// invents it). The version must exist and be enqueued; the append-only
// history accepts the observation whatever came before.
type RelayExternalReceiptCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        CaseID          id.UUID
        WorkItemID      id.UUID
        ExtCommitmentID id.UUID
        VersionNo       int64
        Observation     string
        Now             time.Time
}

// ExternalRelayOutcome is the material result of the relay: the version and
// the receipt kind that landed in the history (the read-time derivation
// takes the LAST event — the state word follows from the history, never
// from this answer).
type ExternalRelayOutcome struct {
        ExtCommitmentID id.UUID `json:"ext_commitment_id"`
        VersionNo       int64   `json:"version_no"`
        Kind            string  `json:"kind"`
}

// ExternalDrainReport is the honest per-record refusal of the drain: the
// version pin and the safe text (the bounded-parking, the integrity alarm
// or the raced skip) — the operator surface of the words that stayed.
type ExternalDrainReport struct {
        ExtCommitmentID id.UUID `json:"ext_commitment_id"`
        VersionNo       int64   `json:"version_no"`
        Text            string  `json:"text"`
}

// ExternalDrainResult is the material result of the drain command: the
// counted outcomes of the batch (the gassed fences, the submitted, the
// delivered, the retryable failures, the parked unknowns, the skipped
// races) and the per-record reports. The command is idempotent — the
// repeated run considers the still-pending records only.
type ExternalDrainResult struct {
        Considered    int                   `json:"considered"`
        Gassed        int                   `json:"gassed"`
        Submitted     int                   `json:"submitted"`
        Delivered     int                   `json:"delivered"`
        FailedRetry   int                   `json:"failed_retry"`
        UnknownParked int                   `json:"unknown_parked"`
        Skipped       int                   `json:"skipped"`
        Reports       []ExternalDrainReport `json:"reports"`
}
