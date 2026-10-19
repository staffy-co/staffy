package main

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/artemiosu/staffy/internal/adapters/externaldispatcher"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	controlDomain "github.com/artemiosu/staffy/internal/control/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The Story 3-3 fixture: the external commitment dispatch surface over
// the REAL domain derivations of the slice (GuardCreateExternalCommitment,
// DeriveExternalReadiness, GuardEnqueueExternalCommitment,
// DeriveExternalDeliveryState, RelayObservationDispatchKind,
// ExternalCommitmentContentToken, ExternalCommitmentIdempotencyKey) and
// the honest fixture-world semantics: the append-only version ladder, the
// approval UNIQUE of the exact version, the queue evidence of the dedup
// key, the append-only history with the monotonic event numbers, the
// drain fences IN ORDER with the conditional gasp (the words provably
// never left) and the retry policy by the RECORDED guarantee. The
// synthetic dispatcher of the test contour serves the sends (the fixture
// wires it — the fixture world IS the test contour, production stays
// AG-4 honest); the control fence reads the LIVE controls fixture world
// (the z31 journey seeds the control points through the REAL Set path of
// the controls fixture and the drain observes the enforcement point).
//
// The identity-scope gates of the real app ladder (the review-or-
// configuration rights, the step-up of the approval) stay the APP's unit
// truth — the fixture serves the transport contract over the two fixture
// principals (the honest simplification of the fixture family: the
// journeys walk the machine/page surfaces, the gates are not the
// fixture's truth); the step-up sentinel DOES answer the fixture resolver
// switch (the fixture relay of the REAL identity sentinel).

// externalWorld is the Story 3.3 material of the fixture world: the
// envelope versions per case, the approvals, the queue evidence (the
// dedup keys of the queued versions), the append-only dispatch histories,
// the fixture HMAC key of the content tokens, the synthetic dispatcher
// (the test contour — nil keeps the honest AG-4 refusal) and the control
// fence reader (nil gases fail-closed, the honest emergency brake).
type externalWorld struct {
	versions  map[id.UUID][]casesDomain.ExternalCommitment
	approvals map[id.UUID][]casesDomain.ExternalApproval
	queued    map[id.UUID]map[string]bool
	events    map[id.UUID][]casesDomain.ExternalDispatchEvent
	key       []byte

	dispatcher casesPort.ExternalDispatcher
	control    casesPort.ControlEffectiveReader
}

func newExternalWorld() *externalWorld {
	return &externalWorld{
		versions:  map[id.UUID][]casesDomain.ExternalCommitment{},
		approvals: map[id.UUID][]casesDomain.ExternalApproval{},
		queued:    map[id.UUID]map[string]bool{},
		events:    map[id.UUID][]casesDomain.ExternalDispatchEvent{},
		key:       []byte("fixture-external-commitment-hmac-key"),
	}
}

// The compile-time proof the fixture serves the narrow transport surface.
var _ casesAPI.ExternalCommitmentService = (*casesFixture)(nil)

// externalCase gates the tenant and the case material (the fixture
// family precedent: the foreign tenant is the honest identity 404, the
// unknown case the module 404 — the surfaces never disclose more).
func (f *casesFixture) externalCase(tenant, caseID id.UUID) error {
	if tenant != f.tenant {
		return domain.ErrNotFound
	}
	if f.material[caseID] == nil {
		return casesDomain.ErrNotFound
	}
	return nil
}

// externalEnvelopes reads the version ladder of one envelope inside the
// case (nil slices stay honest empties).
func (f *casesFixture) externalEnvelopes(caseID, extID id.UUID) []casesDomain.ExternalCommitment {
	versions := f.external.versions[caseID]
	envelope := []casesDomain.ExternalCommitment{}
	for _, version := range versions {
		if version.ExtCommitmentID == extID {
			envelope = append(envelope, version)
		}
	}
	return envelope
}

// externalPinReadiness resolves the LIVE readiness word of the pinned 2.9
// version against the fixture commitment material (tenant-wide by the
// stable identifier — the same discipline the app reads): the STORED
// token match and the DeriveCommitmentReadiness of the exact version; the
// absent row or a mismatched token degrades to the fail-closed empty word.
func (f *casesFixture) externalPinReadiness(pin casesDomain.ExternalCommitmentPin) string {
	pinned := []casesDomain.CommitmentVersion{}
	for _, versions := range f.commitments {
		for _, version := range versions {
			if version.CommitmentID == pin.CommitmentID {
				pinned = append(pinned, version)
			}
		}
	}
	var exact *casesDomain.CommitmentVersion
	maxVersionNo := int64(0)
	for i := range pinned {
		if pinned[i].VersionNo == pin.VersionNo {
			exact = &pinned[i]
		}
		if pinned[i].VersionNo > maxVersionNo {
			maxVersionNo = pinned[i].VersionNo
		}
	}
	if exact == nil || exact.ContentToken != pin.ContentToken {
		return ""
	}
	return casesDomain.DeriveCommitmentReadiness(*exact, maxVersionNo)
}

// CreateExternalCommitment mirrors the app command over the fixture
// material: the guard with the SERVER-side owner snapshot (the current
// ownership term of the fixture case), the honest lineage zeros (the
// fixture world carries no decision ladder — 0 is the honest absence the
// guard accepts), the live pin gate of the fixture 2.9 rows and the
// content token of the fixture key.
func (f *casesFixture) CreateExternalCommitment(_ context.Context, command casesAPI.CreateExternalCommitmentCommand) (casesAPI.ExternalCommitmentOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := f.externalCase(command.TenantID, command.CaseID); err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	extCommitmentID := command.ExtCommitmentID
	if !extCommitmentID.Valid() {
		minted, err := id.NewV7(journeyClock.Now())
		if err != nil {
			return casesAPI.ExternalCommitmentOutcome{}, err
		}
		extCommitmentID = minted
	}
	versionNo := int64(0)
	for _, version := range f.externalEnvelopes(command.CaseID, extCommitmentID) {
		if version.VersionNo > versionNo {
			versionNo = version.VersionNo
		}
	}
	versionNo++
	owner := casesDomain.CurrentOwnership(f.ownershipTerms[command.CaseID])
	if owner == nil {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.ErrNotFound
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.WorkRuleError{Rule: "create_external_commitment", Reason: casesDomain.TerminalWorkItemCommitmentText}
	}
	if err := casesDomain.GuardCreateExternalCommitment(casesDomain.CreateExternalCommitmentCommand{
		TenantID: command.TenantID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: extCommitmentID, VersionNo: versionNo, Purpose: command.Purpose,
		RecipientKind: command.RecipientKind, Channel: command.Channel,
		ContentText: command.ContentText, ValidUntil: command.ValidUntil,
		ExpectedNextStep: command.ExpectedNextStep, AckRequirement: command.AckRequirement,
		Pin: command.Pin, OwnerKind: owner.OwnerKind, OwnerRef: owner.OwnerRef, OwnerEpoch: owner.Epoch,
	}, command.Now); err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.WorkRuleError{Rule: "create_external_commitment", Reason: err.Error()}
	}
	if command.Pin.Present() && f.externalPinReadiness(command.Pin) == "" {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.WorkRuleError{Rule: "create_external_commitment",
			Reason: "Пин обязательства не совпал с живой строкой 2.9 (нет версии с таким номером либо токен иной): сверьте пин точной версии и повторите."}
	}
	version := casesDomain.ExternalCommitment{
		TenantID: command.TenantID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: extCommitmentID, VersionNo: versionNo,
		Purpose: command.Purpose, RecipientKind: command.RecipientKind, Channel: command.Channel,
		ContentText: command.ContentText, ValidUntil: command.ValidUntil,
		ExpectedNextStep: command.ExpectedNextStep, AckRequirement: command.AckRequirement,
		Pin: command.Pin, OwnerKind: owner.OwnerKind, OwnerRef: owner.OwnerRef, OwnerEpoch: owner.Epoch,
		ActorID: command.Actor.ID, CreatedAt: command.Now.UTC().Truncate(1000),
	}
	version.ContentToken = casesDomain.ExternalCommitmentContentToken(f.external.key, version)
	f.external.versions[command.CaseID] = append(f.external.versions[command.CaseID], version)
	return casesAPI.ExternalCommitmentOutcome{
		Command: "create_external_commitment", ExtCommitmentID: extCommitmentID,
		VersionNo: versionNo, ContentToken: version.ContentToken,
	}, nil
}

// ApproveExternalCommitment mirrors the app approval: the step-up sentinel
// of the REAL identity resolver, the current-human-owner gate and the
// append-only row with the silent UNIQUE idempotency of the same
// approver of the same version.
func (f *casesFixture) ApproveExternalCommitment(_ context.Context, command casesAPI.ApproveExternalCommitmentCommand) (casesAPI.ExternalApprovalOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := f.externalCase(command.TenantID, command.CaseID); err != nil {
		return casesAPI.ExternalApprovalOutcome{}, err
	}
	if !command.Actor.StepUpVerified {
		return casesAPI.ExternalApprovalOutcome{}, domain.ErrStepUpRequired
	}
	owner := casesDomain.CurrentOwnership(f.ownershipTerms[command.CaseID])
	if owner == nil || owner.OwnerKind != casesDomain.OwnerKindHuman || owner.OwnerRef != command.Actor.ID {
		return casesAPI.ExternalApprovalOutcome{}, casesDomain.WorkRuleError{Rule: "approve_external_commitment",
			Reason: "Утверждать версию конверта может только текущий человек-владелец кейса (response owner): подтвердите передачу владения или создайте версию под текущим владельцем."}
	}
	if casesDomain.ApproverOf(f.external.approvals[command.CaseID], command.ExtCommitmentID, command.VersionNo) != nil {
		// The silent idempotency of the UNIQUE: the repeated approval of
		// the same approver writes nothing and answers the honest false.
		return casesAPI.ExternalApprovalOutcome{ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo, ApprovalRecorded: false}, nil
	}
	approvalID, err := id.NewV7(command.Now)
	if err != nil {
		return casesAPI.ExternalApprovalOutcome{}, err
	}
	f.external.approvals[command.CaseID] = append(f.external.approvals[command.CaseID], casesDomain.ExternalApproval{
		TenantID: command.TenantID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo,
		ApprovalID: approvalID, ApproverID: command.Actor.ID, StepUpVerified: true,
		CreatedAt: command.Now.UTC().Truncate(1000),
	})
	return casesAPI.ExternalApprovalOutcome{ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo, ApprovalRecorded: true}, nil
}

// EnqueueExternalCommitment mirrors the app enqueue: the approval gate of
// the EXACT version by the LIVE owner, the readiness fences IN ORDER and
// the queue evidence write (the dedup key of the version pin — the
// repeated enqueue of the same version refuses honestly).
func (f *casesFixture) EnqueueExternalCommitment(_ context.Context, command casesAPI.EnqueueExternalCommitmentCommand) (casesAPI.ExternalEnqueueOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := f.externalCase(command.TenantID, command.CaseID); err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	envelope := f.externalEnvelopes(command.CaseID, command.ExtCommitmentID)
	var version *casesDomain.ExternalCommitment
	maxVersionNo := int64(0)
	for i := range envelope {
		if envelope[i].VersionNo > maxVersionNo {
			maxVersionNo = envelope[i].VersionNo
		}
		if envelope[i].VersionNo == command.VersionNo {
			version = &envelope[i]
		}
	}
	if version == nil {
		return casesAPI.ExternalEnqueueOutcome{}, casesDomain.ErrNotFound
	}
	owner := casesDomain.CurrentOwnership(f.ownershipTerms[command.CaseID])
	approved := false
	if owner != nil && owner.OwnerKind == casesDomain.OwnerKindHuman && owner.Epoch == version.OwnerEpoch {
		if approval := casesDomain.ApproverOf(f.external.approvals[command.CaseID], command.ExtCommitmentID, command.VersionNo); approval != nil &&
			approval.ApproverID == owner.OwnerRef && approval.StepUpVerified {
			approved = true
		}
	}
	if !approved {
		return casesAPI.ExternalEnqueueOutcome{}, casesDomain.WorkRuleError{Rule: "enqueue_external_commitment",
			Reason: casesDomain.ExternalApprovalRequiredText(command.VersionNo)}
	}
	ownerEpoch := int64(0)
	if owner != nil {
		ownerEpoch = owner.Epoch
	}
	pinReadiness := casesDomain.CommitmentReadinessConfirmable
	if version.Purpose == casesDomain.ExternalPurposeOffer {
		pinReadiness = f.externalPinReadiness(version.Pin)
	}
	readiness := casesDomain.DeriveExternalReadiness(*version, casesDomain.ExternalReadinessInput{
		Now: journeyClock.Now(), MaxVersionNo: maxVersionNo, OwnerEpochCurrent: ownerEpoch, PinReadiness: pinReadiness,
	})
	if err := casesDomain.GuardEnqueueExternalCommitment(readiness); err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, casesDomain.WorkRuleError{Rule: "enqueue_external_commitment", Reason: err.Error()}
	}
	dedup := casesDomain.DedupKeyExternalCommitmentQueued(command.ExtCommitmentID, command.VersionNo)
	if f.external.queued[command.CaseID] != nil && f.external.queued[command.CaseID][dedup] {
		return casesAPI.ExternalEnqueueOutcome{}, casesDomain.WorkRuleError{Rule: "enqueue_external_commitment",
			Reason: "Точная версия уже стоит в очереди: повторная постановка не создаёт второго намерения (dedup-ключ версии)."}
	}
	if f.external.queued[command.CaseID] == nil {
		f.external.queued[command.CaseID] = map[string]bool{}
	}
	f.external.queued[command.CaseID][dedup] = true
	return casesAPI.ExternalEnqueueOutcome{ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo, Queued: true}, nil
}

// RelayExternalReceipt mirrors the app relay: the observation dictionary
// of the domain guard, the live queue evidence and the append-only
// receipt event with the next monotonic number.
func (f *casesFixture) RelayExternalReceipt(_ context.Context, command casesAPI.RelayExternalReceiptCommand) (casesAPI.ExternalRelayOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := f.externalCase(command.TenantID, command.CaseID); err != nil {
		return casesAPI.ExternalRelayOutcome{}, err
	}
	if err := casesDomain.GuardRelayObservation(command.Observation); err != nil {
		return casesAPI.ExternalRelayOutcome{}, casesDomain.WorkRuleError{Rule: "relay_external_receipt", Reason: err.Error()}
	}
	envelope := f.externalEnvelopes(command.CaseID, command.ExtCommitmentID)
	var version *casesDomain.ExternalCommitment
	for i := range envelope {
		if envelope[i].VersionNo == command.VersionNo {
			version = &envelope[i]
		}
	}
	if version == nil {
		return casesAPI.ExternalRelayOutcome{}, casesDomain.ErrNotFound
	}
	dedup := casesDomain.DedupKeyExternalCommitmentQueued(command.ExtCommitmentID, command.VersionNo)
	if f.external.queued[command.CaseID] == nil || !f.external.queued[command.CaseID][dedup] {
		return casesAPI.ExternalRelayOutcome{}, casesDomain.WorkRuleError{Rule: "relay_external_receipt",
			Reason: "Наблюдение передаётся только для версии в очереди: поставьте точную версию в очередь прежде записи наблюдения."}
	}
	kind := casesDomain.RelayObservationDispatchKind(command.Observation)
	f.externalAppendEvent(command.CaseID, command.ExtCommitmentID, command.VersionNo, kind, "", 0, "", false, "наблюдение оператора канала", command.Actor.ID)
	return casesAPI.ExternalRelayOutcome{ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo, Kind: kind}, nil
}

// externalAppendEvent appends one history event under the lock of the
// caller: the monotonic event number of the version's own ladder and the
// bounded safe details of the machine boundary (never the content).
func (f *casesFixture) externalAppendEvent(caseID, extID id.UUID, versionNo int64, kind, reason string, attemptNo int64, correlation string, guarantee bool, details string, actor id.UUID) casesDomain.ExternalDispatchEvent {
	number := int64(0)
	for _, event := range f.external.events[caseID] {
		if event.ExtCommitmentID == extID && event.VersionNo == versionNo && event.EventNo > number {
			number = event.EventNo
		}
	}
	eventID, err := id.NewV7(journeyClock.Now())
	if err != nil {
		eventID = extID
	}
	event := casesDomain.ExternalDispatchEvent{
		TenantID: f.tenant, CaseID: caseID, ExtCommitmentID: extID, VersionNo: versionNo,
		EventID: eventID, EventNo: number + 1, Kind: kind, Reason: reason, AttemptNo: attemptNo,
		ExternalCorrelationID: correlation, IdempotentGuarantee: guarantee, Details: details,
		ActorID: actor, CreatedAt: journeyClock.Now().UTC().Truncate(1000),
	}
	f.external.events[caseID] = append(f.external.events[caseID], event)
	return event
}

// externalVersionEvents reads the append-only history of one version in
// the event-number order.
func (f *casesFixture) externalVersionEvents(caseID, extID id.UUID, versionNo int64) []casesDomain.ExternalDispatchEvent {
	events := []casesDomain.ExternalDispatchEvent{}
	for _, event := range f.external.events[caseID] {
		if event.ExtCommitmentID == extID && event.VersionNo == versionNo {
			events = append(events, event)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].EventNo < events[j].EventNo })
	return events
}

// externalQueuedPins reads the queue evidence of one case (the dedup keys
// ARE the version pins — the honest mirror of the store payload).
func (f *casesFixture) externalQueuedPins(caseID id.UUID) map[casesPort.ExternalQueuedVersion]bool {
	pins := map[casesPort.ExternalQueuedVersion]bool{}
	for dedup := range f.external.queued[caseID] {
		parts := splitDedupKey(dedup)
		if parts == nil {
			continue
		}
		pins[casesPort.ExternalQueuedVersion{ExtCommitmentID: parts[0].(id.UUID), VersionNo: parts[1].(int64)}] = true
	}
	return pins
}

// splitDedupKey parses "external_commitment.queued:<ext>:<version>".
func splitDedupKey(key string) []any {
	if len(key) < len("external_commitment.queued:") {
		return nil
	}
	rest := key[len("external_commitment.queued:"):]
	cut := lastColon(rest)
	if cut < 0 {
		return nil
	}
	extID, err := id.Parse(rest[:cut])
	if err != nil {
		return nil
	}
	var versionNo int64
	if _, err := fmt.Sscanf(rest[cut+1:], "%d", &versionNo); err != nil {
		return nil
	}
	return []any{extID, versionNo}
}

// lastColon finds the last colon of the dedup tail (the UUID carries no
// colon — the last one separates the version number).
func lastColon(text string) int {
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == ':' {
			return i
		}
	}
	return -1
}

// DrainExternalCommitments mirrors the app drain over the fixture
// material: the AG-4 refusal without a dispatcher, the pending selection
// (the decided, the bound-exhausted and the guarantee-free unknowns are
// structurally unselectable), the token re-check alarm FIRST, the stale
// fences IN ORDER with the conditional gasp (the words provably never
// left), the LIVE control fence (nil or an unresolvable control gases
// fail-closed), the retry policy by the RECORDED guarantee and the
// bounded attempts.
func (f *casesFixture) DrainExternalCommitments(context.Context, domain.Principal, id.UUID) (casesAPI.ExternalDrainResult, error) {
	f.lock()
	// The selection and the fence reads happen under the one lock (the
	// fixture world is single-threaded per command — the raced-twin honesty
	// stays the app/store truth, the journeys never race the fixture).
	if f.external.dispatcher == nil {
		f.unlock()
		return casesAPI.ExternalDrainResult{}, casesDomain.WorkRuleError{Rule: "drain_external_commitments",
			Reason: "Диспетчер внешних отправок не сконфигурирован: записи остаются в очереди (производственный контур не подключает провайдеров — AG-4); слова не уходят."}
	}
	type candidate struct {
		version casesDomain.ExternalCommitment
		events  []casesDomain.ExternalDispatchEvent
	}
	pending := []candidate{}
	for caseID, queued := range f.external.queued {
		for dedup := range queued {
			parts := splitDedupKey(dedup)
			if parts == nil {
				continue
			}
			extID, versionNo := parts[0].(id.UUID), parts[1].(int64)
			var version *casesDomain.ExternalCommitment
			for i := range f.external.versions[caseID] {
				if f.external.versions[caseID][i].ExtCommitmentID == extID && f.external.versions[caseID][i].VersionNo == versionNo {
					version = &f.external.versions[caseID][i]
				}
			}
			if version == nil {
				continue
			}
			events := f.externalVersionEvents(caseID, extID, versionNo)
			// The pending selection (the SQL mirror): the submitted, the
			// gassed, the bound-exhausted and the guarantee-free unknowns
			// stay structurally unselectable.
			selectable := true
			attemptUsed := int64(0)
			var last *casesDomain.ExternalDispatchEvent
			for i := range events {
				event := &events[i]
				switch event.Kind {
				case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchCancelledStale:
					selectable = false
				case casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
					if event.AttemptNo > attemptUsed {
						attemptUsed = event.AttemptNo
					}
				}
				if last == nil || event.EventNo >= last.EventNo {
					last = event
				}
			}
			if attemptUsed >= casesDomain.MaxDispatchAttempts {
				selectable = false
			}
			if last != nil && last.Kind == casesDomain.ExternalDispatchAttemptUnknown && !last.IdempotentGuarantee {
				selectable = false
			}
			if selectable {
				pending = append(pending, candidate{version: *version, events: events})
			}
		}
	}
	// The deterministic order of the batch (the fixture world — the
	// store orders by the outbox occurred_at; the fixture has the one
	// clock, the ids order the material).
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].version.CreatedAt.Equal(pending[j].version.CreatedAt) {
			return pending[i].version.ExtCommitmentID.String() < pending[j].version.ExtCommitmentID.String()
		}
		return pending[i].version.CreatedAt.Before(pending[j].version.CreatedAt)
	})
	if len(pending) > casesDomain.ExternalCommitmentDrainLimit {
		pending = pending[:casesDomain.ExternalCommitmentDrainLimit]
	}
	result := casesAPI.ExternalDrainResult{Considered: len(pending), Reports: []casesAPI.ExternalDrainReport{}}
	report := func(extID id.UUID, versionNo int64, text string) {
		result.Reports = append(result.Reports, casesAPI.ExternalDrainReport{ExtCommitmentID: extID, VersionNo: versionNo, Text: text})
	}
	now := journeyClock.Now()
	for _, one := range pending {
		version, events := one.version, one.events
		// The integrity probe FIRST (the alarm, never a gasp, never a send).
		if casesDomain.ExternalCommitmentContentToken(f.external.key, version) != version.ContentToken {
			result.Skipped++
			report(version.ExtCommitmentID, version.VersionNo, "Токен версии не перепровернулся: запись оставлена в очереди, отправки нет; передайте в reconciliation (3-7).")
			continue
		}
		// The stale fences IN ORDER; the gasp is conditional — the words
		// provably never left (the history is empty or attempt_failed only).
		reason := ""
		switch {
		case f.externalNewerExists(version):
			reason = casesDomain.ExternalStaleNewerVersion
		case !version.ValidUntil.After(now):
			reason = casesDomain.ExternalStaleExpired
		case casesDomain.CurrentOwnership(f.ownershipTerms[version.CaseID]) == nil ||
			casesDomain.CurrentOwnership(f.ownershipTerms[version.CaseID]).Epoch != version.OwnerEpoch:
			reason = casesDomain.ExternalStaleOwnerEpoch
		}
		if reason != "" && f.externalWordsNeverLeft(events) {
			f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
				casesDomain.ExternalDispatchCancelledStale, reason, 0, "", false, casesDomain.ExternalStaleReasonText(reason), f.owner)
			result.Gassed++
			continue
		}
		// The LIVE control fence (nil or an unresolvable control = the
		// fail-closed gasp — the emergency brake); the conditional gasp
		// covers the control fence the same honest way (the words never
		// left when the fence fires).
		if f.external.control == nil {
			if f.externalWordsNeverLeft(events) {
				f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
					casesDomain.ExternalDispatchCancelledStale, casesDomain.ExternalStaleControlMode, 0, "", false,
					casesDomain.ExternalStaleReasonText(casesDomain.ExternalStaleControlMode), f.owner)
				result.Gassed++
			} else {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, "Запись уже обработана параллельным прогоном: второго намерения нет.")
			}
			continue
		}
		control, controlErr := f.external.control.EffectiveFor(context.Background(), f.tenant, version.Channel)
		if controlErr != nil || control.Mode != casesPort.ExternalControlAccept {
			if f.externalWordsNeverLeft(events) {
				f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
					casesDomain.ExternalDispatchCancelledStale, casesDomain.ExternalStaleControlMode, 0, "", false,
					casesDomain.ExternalStaleReasonText(casesDomain.ExternalStaleControlMode), f.owner)
				result.Gassed++
			} else {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, "Запись уже обработана параллельным прогоном: второго намерения нет.")
			}
			continue
		}
		// The retry policy by the derived state and the RECORDED guarantee.
		state := casesDomain.DeriveExternalDeliveryState(true, events)
		attemptUsed := int64(0)
		var lastEvent *casesDomain.ExternalDispatchEvent
		for i := range events {
			event := &events[i]
			switch event.Kind {
			case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
				if event.AttemptNo > attemptUsed {
					attemptUsed = event.AttemptNo
				}
			}
			if lastEvent == nil || event.EventNo >= lastEvent.EventNo {
				lastEvent = event
			}
		}
		attemptNo := attemptUsed + 1
		switch state.State {
		case casesDomain.ExternalDeliveryFailed:
			if attemptUsed >= casesDomain.MaxDispatchAttempts {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, "Попытки отправки исчерпаны (не более трёх): запись припаркована — создайте новую версию либо передайте в reconciliation (3-7).")
				continue
			}
		case casesDomain.ExternalDeliveryUnknown:
			if lastEvent == nil || !lastEvent.IdempotentGuarantee {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, "Итог неопределён (timeout без идемпотентной гарантии): слепого повтора нет — передаётся в reconciliation (3-7).")
				continue
			}
			if attemptUsed >= casesDomain.MaxDispatchAttempts {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, "Попытки отправки исчерпаны (не более трёх): запись припаркована — создайте новую версию либо передайте в reconciliation (3-7).")
				continue
			}
		}
		f.unlock()
		idempotencyKey := casesDomain.ExternalCommitmentIdempotencyKey(f.tenant, version.ExtCommitmentID, version.VersionNo)
		dispatched, dispatchErr := f.external.dispatcher.Dispatch(context.Background(), tenancy.Context{}, casesDomain.BuildOutboundExternalPayload(version, idempotencyKey))
		f.lock()
		outcome := casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: false}
		switch {
		case dispatchErr != nil:
			outcome = casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: false, Details: "ошибка вызова диспетчера"}
		case casesDomain.GuardDispatchOutcome(dispatched.Outcome) != nil:
			outcome = casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: false,
				Details: casesDomain.TruncateExternalDetails("исход вне словаря: " + dispatched.Outcome)}
		default:
			outcome = dispatched
		}
		details := casesDomain.ExternalDispatchText(outcome.Outcome, outcome.CapabilityID, outcome.ExternalCorrelationID)
		if outcome.Details != "" {
			details = casesDomain.TruncateExternalDetails(outcome.Details)
		}
		switch outcome.Outcome {
		case casesDomain.DispatchOutcomeAccepted:
			f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
				casesDomain.ExternalDispatchAttemptSubmitted, "", attemptNo, outcome.ExternalCorrelationID, outcome.IdempotentGuarantee, details, f.owner)
			result.Submitted++
		case casesDomain.DispatchOutcomeDelivered:
			f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
				casesDomain.ExternalDispatchAttemptSubmitted, "", attemptNo, outcome.ExternalCorrelationID, outcome.IdempotentGuarantee, details, f.owner)
			f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
				casesDomain.ExternalDispatchReceiptDelivered, "", 0, "", false, "квитанция канала: доставка", f.owner)
			result.Delivered++
		case casesDomain.DispatchOutcomeFailed:
			f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
				casesDomain.ExternalDispatchAttemptFailed, "", attemptNo, "", outcome.IdempotentGuarantee, details, f.owner)
			result.FailedRetry++
		default:
			f.externalAppendEvent(version.CaseID, version.ExtCommitmentID, version.VersionNo,
				casesDomain.ExternalDispatchAttemptUnknown, "", attemptNo, outcome.ExternalCorrelationID, outcome.IdempotentGuarantee, details, f.owner)
			result.UnknownParked++
			report(version.ExtCommitmentID, version.VersionNo, "Итог неопределён (timeout без идемпотентной гарантии): слепого повтора нет — передаётся в reconciliation (3-7).")
		}
	}
	f.unlock()
	return result, nil
}

// externalNewerExists resolves the newer_version fence: a bigger version
// number of the same envelope exists in the case.
func (f *casesFixture) externalNewerExists(version casesDomain.ExternalCommitment) bool {
	for _, existing := range f.external.versions[version.CaseID] {
		if existing.ExtCommitmentID == version.ExtCommitmentID && existing.VersionNo > version.VersionNo {
			return true
		}
	}
	return false
}

// externalWordsNeverLeft answers whether the words provably never left:
// the history is empty or carries the pre-send refusals only.
func (f *casesFixture) externalWordsNeverLeft(events []casesDomain.ExternalDispatchEvent) bool {
	for _, event := range events {
		if event.Kind != casesDomain.ExternalDispatchAttemptFailed {
			return false
		}
	}
	return true
}

// ReadExternalCommitmentView mirrors the app read assembly over the
// fixture material: the sections per Work Item of the case, the queue
// evidence, the seven-state derivation, the readiness fences with the
// LIVE owner epoch and the fixture 2.9 pin readiness, the approval
// material and the current flags.
func (f *casesFixture) ReadExternalCommitmentView(_ context.Context, actor domain.Principal, tenant, caseID id.UUID) (casesAPI.ExternalCommitmentView, error) {
	f.lock()
	defer f.unlock()
	if err := f.externalCase(tenant, caseID); err != nil {
		return casesAPI.ExternalCommitmentView{}, err
	}
	_ = actor
	versions := f.external.versions[caseID]
	ownerEpoch := int64(0)
	if owner := casesDomain.CurrentOwnership(f.ownershipTerms[caseID]); owner != nil {
		ownerEpoch = owner.Epoch
	}
	now := journeyClock.Now()
	queuedSet := f.externalQueuedPins(caseID)
	maxVersionNo := map[id.UUID]int64{}
	for _, version := range versions {
		if version.VersionNo > maxVersionNo[version.ExtCommitmentID] {
			maxVersionNo[version.ExtCommitmentID] = version.VersionNo
		}
	}
	pinReadiness := map[id.UUID]string{}
	for _, version := range versions {
		if version.Purpose != casesDomain.ExternalPurposeOffer || !version.Pin.Present() {
			continue
		}
		if _, resolved := pinReadiness[version.Pin.CommitmentID]; !resolved {
			pinReadiness[version.Pin.CommitmentID] = f.externalPinReadiness(version.Pin)
		}
	}
	view := casesAPI.ExternalCommitmentView{
		PurposeWords:   append([]string(nil), casesDomain.ExternalPurposeWords...),
		RecipientWords: append([]string(nil), casesDomain.ExternalRecipientKindWords...),
		AckWords:       append([]string(nil), casesDomain.ExternalAckRequirementWords...),
		RelayWords:     append([]string(nil), casesDomain.RelayObservationWords...),
		ChannelWords:   append([]string(nil), casesDomain.ChannelOrder...),
		StateWords:     append([]string(nil), casesDomain.ExternalDeliveryStateWords...),
		PanelText:      casesDomain.ExternalCommitmentPanelText,
		EmptyText:      casesDomain.ExternalCommitmentEmptyText,
		Items:          []casesAPI.ExternalCommitmentWorkItemView{},
	}
	for _, anchor := range f.items[caseID] {
		section := casesAPI.ExternalCommitmentWorkItemView{
			WorkItemID: anchor.WorkItemID,
			Versions:   []casesAPI.ExternalCommitmentVersionCard{},
		}
		for _, version := range versions {
			if version.WorkItemID != anchor.WorkItemID {
				continue
			}
			events := f.externalVersionEvents(caseID, version.ExtCommitmentID, version.VersionNo)
			enqueued := queuedSet[casesPort.ExternalQueuedVersion{ExtCommitmentID: version.ExtCommitmentID, VersionNo: version.VersionNo}]
			delivery := casesDomain.DeriveExternalDeliveryState(enqueued, events)
			pinWord := casesDomain.CommitmentReadinessConfirmable
			if version.Purpose == casesDomain.ExternalPurposeOffer && version.Pin.Present() {
				pinWord = pinReadiness[version.Pin.CommitmentID]
			}
			readiness := casesDomain.DeriveExternalReadiness(version, casesDomain.ExternalReadinessInput{
				Now: now, MaxVersionNo: maxVersionNo[version.ExtCommitmentID],
				OwnerEpochCurrent: ownerEpoch, PinReadiness: pinWord,
			})
			card := casesAPI.ExternalCommitmentVersionCard{
				Version:       casesAPI.FromExternalCommitment(version),
				Enqueued:      enqueued,
				Delivery:      casesAPI.FromExternalDeliveryState(delivery),
				Readiness:     readiness,
				ReadinessText: casesDomain.ExternalReadinessRefusalTexts[readiness],
				Current:       version.VersionNo == maxVersionNo[version.ExtCommitmentID],
			}
			if approval := casesDomain.ApproverOf(f.external.approvals[caseID], version.ExtCommitmentID, version.VersionNo); approval != nil {
				card.Approved = true
				card.ApproverID = approval.ApproverID
			}
			for _, event := range events {
				card.Events = append(card.Events, casesAPI.FromExternalDispatchEvent(event))
			}
			section.Versions = append(section.Versions, card)
		}
		view.Items = append(view.Items, section)
	}
	return view, nil
}

// fixtureControlReader is the LIVE control fence of the fixture drain:
// the read walks the REAL control app service of the controls fixture
// (the owner principal holds every scope of the fixture membership) with
// the route keys of the delivery — the same adapter discipline as the
// production composition root (the fixture world owns its own reader —
// the cases module stays DAG-clean).
type fixtureControlReader struct {
	controls *controlsFixture
}

func (r fixtureControlReader) EffectiveFor(ctx context.Context, tenantID id.UUID, channel string) (casesPort.ExternalControlMode, error) {
	effective, err := r.controls.service.EffectiveFor(ctx, domain.Principal{ID: principalID, StepUpVerified: true}, tenantID,
		[]controlDomain.ScopeKey{
			{Kind: controlDomain.ScopeKindTenant, Ref: tenantID.String()},
			{Kind: controlDomain.ScopeKindChannel, Ref: channel},
		})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			// The unreadable control of the fixture world (the membership
			// surface of the control module): the fail-closed gasp — the
			// words never leave on an unreadable control.
			return casesPort.ExternalControlMode{Mode: ""}, nil
		}
		return casesPort.ExternalControlMode{}, err
	}
	mode := casesPort.ExternalControlMode{Mode: effective.Mode, Reason: effective.Reason}
	for _, control := range effective.Influencing {
		if controlDomain.SafeReadMode(control.Mode) == effective.Mode && control.Generation > mode.Generation {
			mode.Generation = control.Generation
		}
	}
	return mode, nil
}

// newFixtureControlReader wires the reader over the controls fixture.
func newFixtureControlReader(controls *controlsFixture) fixtureControlReader {
	return fixtureControlReader{controls: controls}
}

// The fixture server wiring (the main of the journeys binary): the
// synthetic dispatcher and the LIVE control reader of the fixture world —
// production never wires the dispatcher (AG-4), the fixture world IS the
// test contour.
func (f *casesFixture) wireExternalDispatchWorld(controls *controlsFixture) *externaldispatcher.Synthetic {
	dispatcher := externaldispatcher.New()
	f.external.dispatcher = dispatcher
	f.external.control = newFixtureControlReader(controls)
	return dispatcher
}
