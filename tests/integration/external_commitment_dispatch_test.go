//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/adapters/connectorrefresolver"
	"github.com/artemiosu/staffy/internal/adapters/controleffectiveresolver"
	"github.com/artemiosu/staffy/internal/adapters/externaldispatcher"
	"github.com/artemiosu/staffy/internal/adapters/ingresssyn"
	"github.com/artemiosu/staffy/internal/adapters/observationsyn"
	"github.com/artemiosu/staffy/internal/adapters/verificationsyn"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesApp "github.com/artemiosu/staffy/internal/cases/app"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesStore "github.com/artemiosu/staffy/internal/cases/store"
	controlAPI "github.com/artemiosu/staffy/internal/control/api"
	controlApp "github.com/artemiosu/staffy/internal/control/app"
	controlDomain "github.com/artemiosu/staffy/internal/control/domain"
	controlStore "github.com/artemiosu/staffy/internal/control/store"
	identityApp "github.com/artemiosu/staffy/internal/identity/app"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	identityStore "github.com/artemiosu/staffy/internal/identity/store"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// TestExternalCommitmentDispatchMatrixRowsBySpec is the Story 3-3 step-04
// matrix: the honest rows of the external commitment dispatch walked by name
// over the REAL app service with the REAL store, the REAL synthetic
// dispatcher of the test contour, the REAL composition control adapter over
// the LIVE 3.2 control store and the LIVE identity/PG membership. The rows
// pin the live gate order (the rights gate, the step-up, the exact-version
// approval of the current human owner, the readiness BEFORE the
// transaction), the persist-before-emit atomicity (the dedup-twin probe and
// the concurrent enqueue), the drain policy by name (the four outcome words,
// the RECORDED guarantee of the unknown retry, the bounded attempts, the
// raced skip, the integrity alarm), the execute-time stale fences IN ORDER
// with the LIVE control paused/accept walk, the relay observations, the
// idempotency key stability of the version pin and the database discipline
// (RLS + FORCE, the append-only grants, the CHECK backstops of 00017).
//
// The slot registry (the retro-2-3 discipline): the world seed is 332 (the
// free slot between 330 and the 9xx zone), the identity ids live in the
// free b5xx suffix range, the control ids reuse the controlMatrixIDs
// generator (distinct tenants — no PK collision), the envelope ids own the
// 019a3f33 range (the 2.9 world owns 019a2f4c), the service id pool is the
// 3320 range with the 512-value headroom (one pool serves the two ingests,
// the commands and the drain events of the whole matrix).
func TestExternalCommitmentDispatchMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	w, svc, dispatcher := externalDispatchWorld(t, 332)
	caseID, workItemID := w.acceptedCase(t, "evt-ext-dispatch-m1", "sess-ext-dispatch-m1", `{"call":"inbound-call","from":"+79000000331"}`)
	// The invited operations-only member: the no-scope actor of row 3 and
	// the LIVE control operator of row 20 (the membership-clock rebuild of
	// the 3-2 precedent — the invitation acceptance reads the live
	// database clock, the frozen world clock would look expired).
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC().Truncate(time.Microsecond)}, newPrivacyIDs(13330))
	operator := inviteMember(t, identityLive, w.owner, w.tenant, "ext-dispatch-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000b501", "00000000b502")
	operator.StepUpVerified = true // the control mutations demand the confirmed identity (the 3-2 precedent)
	auditor := inviteMember(t, identityLive, w.owner, w.tenant, "ext-dispatch-audit@example.org", identityDomain.ExpertQA, []identityDomain.Scope{identityDomain.Audit}, "00000000b505", "00000000b506")

	// Row 1 — the stranger (no membership anywhere) is the honest 404:
	// membership FIRST, no envelope material leaves the service.
	t.Run("row-1 stranger: membership first, the honest 404", func(t *testing.T) {
		stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000b516")}
		_, err := svc.CreateExternalCommitment(ctx, extCreateCommand(w, stranger, caseID, workItemID, extID(102), casesDomain.ExternalPurposePromise, extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{}))
		if !errors.Is(err, identityDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404, got %v", err)
		}
		if strings.Contains(err.Error(), "конверт") || strings.Contains(err.Error(), "+79000000331") {
			t.Fatalf("the 404 never carries envelope material: %s", err.Error())
		}
	})

	// Row 2 — the foreign member (the owner of another tenant) is the honest
	// 404 on THIS tenant: the cross-tenant boundary is closed by the live
	// membership truth (the RLS mirror of the 404 gate).
	t.Run("row-2 foreign member: the cross-tenant 404", func(t *testing.T) {
		foreignIdentity, foreignOwner := newService("00000000b511", "00000000b512", "00000000b513")
		foreignTenant := createTenant(t, foreignIdentity, foreignOwner, "Чужие внешние обязательства", "ext-matrix-foreign-1", "00000000b514").Tenant.ID
		if foreignTenant == w.tenant {
			t.Fatal("the foreign tenant must differ")
		}
		_, err := svc.CreateExternalCommitment(ctx, extCreateCommand(w, foreignOwner, caseID, workItemID, extID(102), casesDomain.ExternalPurposePromise, extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{}))
		if !errors.Is(err, identityDomain.ErrNotFound) {
			t.Fatalf("the foreign member reads nothing of this tenant, got %v", err)
		}
	})

	// Row 3 — the honest 403 rows of the rights gate: the member without
	// ANY cases-family scope (operations-only) refuses at the scope layer
	// with the raw sentinel; the audit-only member passes the scope layer
	// and hears the create's own words — the rights gate (review or
	// configuration) fires BEFORE every material guard, the operator of
	// the control surface never creates, approves, enqueues or relays.
	t.Run("row-3 the scope layers: the operations-only member and the audit-only member hear their honest refusals", func(t *testing.T) {
		_, err := svc.CreateExternalCommitment(ctx, extCreateCommand(w, operator, caseID, workItemID, extID(102), casesDomain.ExternalPurposePromise, extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{}))
		if !errors.Is(err, identityDomain.ErrForbidden) {
			t.Fatalf("the member without any cases scope is the honest 403 of the scope layer, got %v", err)
		}
		_, err = svc.CreateExternalCommitment(ctx, extCreateCommand(w, auditor, caseID, workItemID, extID(102), casesDomain.ExternalPurposePromise, extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{}))
		if !errors.Is(err, identityDomain.ErrForbidden) {
			t.Fatalf("the audit-only member is the honest 403, got %v", err)
		}
		if !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the 403 carries the scope words, got %s", err.Error())
		}
	})

	// Row 4 — the create lands the version №1 append-only with the owner
	// snapshot: the content token of the exact bytes, the persisted state
	// (the words have NOT left), NO queue evidence and NO approval.
	t.Run("row-4 create: version №1 lands persisted with the token, no queue evidence", func(t *testing.T) {
		outcome := extCreate(t, svc, w, caseID, workItemID, extID(101), casesDomain.ExternalPurposePromise, "Обещаем отгрузку запчастей до пятницы", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		if outcome.VersionNo != 1 || outcome.ContentToken == "" {
			t.Fatalf("expected the version №1 with the content token, got %+v", outcome)
		}
		card := extCardOf(t, svc, w, caseID, extID(101), 1)
		if card.Delivery.State != casesDomain.ExternalDeliveryPersisted || card.Enqueued || card.Approved || !card.Current {
			t.Fatalf("the fresh version is persisted, current, not enqueued, not approved, got %+v", card)
		}
		if card.Version.ContentToken != outcome.ContentToken || card.Version.OwnerKind != casesDomain.OwnerKindHuman || card.Version.OwnerEpoch != 1 {
			t.Fatalf("the row carries the owner snapshot and the exact token, got %+v", card.Version)
		}
		if outboxQueuedCount(t, w, extID(101), 1) != 0 {
			t.Fatal("the create never writes the queue evidence")
		}
	})

	// Row 5 — the ladder: the changed text is the NEW version №2 of the same
	// envelope; №1 stays byte-identical (the append-only discipline — the
	// token of the exact bytes pins the immutability), №2 is the current.
	t.Run("row-5 ladder: №2 appends, №1 stays byte-identical", func(t *testing.T) {
		first := extCardOf(t, svc, w, caseID, extID(101), 1)
		second := extCreate(t, svc, w, caseID, workItemID, extID(101), casesDomain.ExternalPurposePromise, "Обещаем отгрузку запчастей до субботы (изменение срока)", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		if second.VersionNo != 2 {
			t.Fatalf("the changed text is the NEW version №2, got %+v", second)
		}
		original := extCardOf(t, svc, w, caseID, extID(101), 1)
		if original.Version.ContentToken != first.Version.ContentToken || original.Version.ContentText != first.Version.ContentText {
			t.Fatalf("№1 stays byte-identical: %+v vs %+v", original.Version, first.Version)
		}
		fresh := extCardOf(t, svc, w, caseID, extID(101), 2)
		if !fresh.Current || fresh.Delivery.State != casesDomain.ExternalDeliveryPersisted {
			t.Fatalf("№2 is the current persisted version, got %+v", fresh)
		}
		if original.Current {
			t.Fatal("№1 is honestly superseded — the current flag moves to №2")
		}
	})

	// Row 6 — the dictionary guards refuse with the honest words: the
	// foreign purpose word, the offer without the 2.9 pin and the partial
	// all-or-nothing pin never open a transaction.
	t.Run("row-6 dictionary guards: the foreign word, the offer without the pin, the partial pin", func(t *testing.T) {
		_, err := svc.CreateExternalCommitment(ctx, extCreateCommand(w, w.ownershipActor(), caseID, workItemID, extID(102), "pledge", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{}))
		assertRuleError(t, err, "не входит в словарь")
		_, err = svc.CreateExternalCommitment(ctx, extCreateCommand(w, w.ownershipActor(), caseID, workItemID, extID(102), casesDomain.ExternalPurposeOffer, extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{}))
		assertRuleError(t, err, "требует пин точной версии")
		partial := casesDomain.ExternalCommitmentPin{CommitmentID: extID(101), VersionNo: 1}
		_, err = svc.CreateExternalCommitment(ctx, extCreateCommand(w, w.ownershipActor(), caseID, workItemID, extID(102), casesDomain.ExternalPurposePromise, extNow.Add(48*time.Hour), partial))
		assertRuleError(t, err, "требует content token версии")
	})

	// Row 7 — the LIVE 2.9 pin of the offer: the pinned version must exist
	// tenant-wide and its STORED content token must equal the pin token; the
	// mismatched token refuses with the honest words, the honest pin lands
	// the offer and the full offer lifecycle (approve → enqueue → submit)
	// passes the confirmable readiness of the LIVE 2.9 row.
	t.Run("row-7 offer with the LIVE 2.9 pin: the mismatch refuses, the honest pin passes end-to-end", func(t *testing.T) {
		w.modeTurnEvent(t, caseID, "sess-ext-dispatch-m1", "evt-ext-dispatch-m1-turn-1", `{"say":"интересует замена масла"}`)
		w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "ext-matrix-obs-1"))
		pinned := id.MustParse("019a2f4c-5000-7000-8000-000000003301")
		if _, err := svc.CreateCommitment(ctx, w.commitmentCreateCommand(caseID, workItemID, pinned, casesDomain.CommitmentStatusOffered, "ext-matrix-29-create-1")); err != nil {
			t.Fatal(err)
		}
		section := commitmentSectionOf(t, w.commitmentViewOf(t, caseID), workItemID)
		if len(section.Versions) != 1 || section.Versions[0].Version.VersionNo != 1 {
			t.Fatalf("the pinned 2.9 version №1 with the price observation, got %+v", section.Versions)
		}
		pin := casesDomain.ExternalCommitmentPin{CommitmentID: pinned, VersionNo: 1, ContentToken: section.Versions[0].Version.ContentToken}
		badPin := pin
		badPin.ContentToken = strings.Repeat("a", 64)
		_, err := svc.CreateExternalCommitment(ctx, extCreateCommand(w, w.ownershipActor(), caseID, workItemID, extID(103), casesDomain.ExternalPurposeOffer, extNow.Add(48*time.Hour), badPin))
		assertRuleError(t, err, "Пин обязательства не совпал с живой строкой 2.9")
		offer := extCreate(t, svc, w, caseID, workItemID, extID(103), casesDomain.ExternalPurposeOffer, "Предлагаем условия обязательства по точной версии", extNow.Add(48*time.Hour), pin)
		if offer.VersionNo != 1 {
			t.Fatalf("the offer with the honest pin lands, got %+v", offer)
		}
		extApprove(t, svc, w, caseID, workItemID, extID(103), 1)
		queued := extEnqueue(t, svc, w, caseID, workItemID, extID(103), 1)
		if !queued.Queued {
			t.Fatalf("the confirmable LIVE 2.9 readiness opens the offer queue, got %+v", queued)
		}
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		if result := extDrain(t, svc, w); result.Submitted != 1 {
			t.Fatalf("the offer version submits through the queue, got %+v", result)
		}
	})

	// Row 8 — the approval gates: the non-owner member refuses with the
	// owner words, the unverified session is the honest step-up refusal,
	// the owner approves the exact version and the repeated approval of the
	// same approver is the silent idempotency of the UNIQUE.
	t.Run("row-8 approval gates: the non-owner, the step-up, the owner, the silent repeat", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(104), casesDomain.ExternalPurposePromise, "Обещаем подготовить документы к утру", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		_, err := svc.ApproveExternalCommitment(ctx, extApproveCommand(w, w.backupActor(), caseID, workItemID, extID(104), 1))
		assertRuleError(t, err, "только текущий человек-владелец")
		unverified := w.ownershipActor()
		unverified.StepUpVerified = false
		if _, err = svc.ApproveExternalCommitment(ctx, extApproveCommand(w, unverified, caseID, workItemID, extID(104), 1)); !errors.Is(err, identityDomain.ErrStepUpRequired) {
			t.Fatalf("the unverified session never approves, got %v", err)
		}
		if outcome := extApprove(t, svc, w, caseID, workItemID, extID(104), 1); !outcome.ApprovalRecorded {
			t.Fatalf("the owner approves the exact version, got %+v", outcome)
		}
		if outcome := extApprove(t, svc, w, caseID, workItemID, extID(104), 1); outcome.ApprovalRecorded {
			t.Fatalf("the repeated approval of the same approver is the silent idempotency, got %+v", outcome)
		}
		if card := extCardOf(t, svc, w, caseID, extID(104), 1); !card.Approved || card.ApproverID != w.owner.ID {
			t.Fatalf("the card carries the approval material of the exact version, got %+v", card)
		}
	})

	// Row 9 — the enqueue approval gate: the unapproved version refuses
	// with the exact-version words BEFORE the transaction — no queue
	// evidence ever exists for the refused path.
	t.Run("row-9 enqueue approval gate: the unapproved version refuses BEFORE the transaction", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(105), casesDomain.ExternalPurposePromise, "Обещаем обратный звонок в течение часа", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		_, err := svc.EnqueueExternalCommitment(ctx, extEnqueueCommand(w, caseID, workItemID, extID(105), 1))
		assertRuleError(t, err, "Требуется утверждение текущим человеком-владельцем")
		if outboxQueuedCount(t, w, extID(105), 1) != 0 {
			t.Fatal("the refused enqueue never writes the queue evidence")
		}
	})

	// Row 10 — the persist-before-emit enqueue: the ONE outbox record with
	// the SAFE payload only (the identifiers, the version pin, the
	// dictionary words, the idempotency key and the content digest — the
	// content NEVER enters, AC-10), the queued state of the view and the
	// empty dispatch history.
	t.Run("row-10 enqueue: the ONE outbox record, the safe payload, the queued state", func(t *testing.T) {
		extApprove(t, svc, w, caseID, workItemID, extID(105), 1)
		queued := extEnqueue(t, svc, w, caseID, workItemID, extID(105), 1)
		if !queued.Queued {
			t.Fatalf("the approved version enters the queue, got %+v", queued)
		}
		if outboxQueuedCount(t, w, extID(105), 1) != 1 {
			t.Fatal("the enqueue writes exactly ONE queue record")
		}
		payload := extQueuedPayload(t, w, extID(105), 1)
		key := casesDomain.ExternalCommitmentIdempotencyKey(w.tenant, extID(105), 1)
		if payload["idempotency_key"] != key {
			t.Fatalf("the queue record carries the idempotency key of the version pin, got %v", payload["idempotency_key"])
		}
		if _, ok := payload["content_digest"]; !ok {
			t.Fatalf("the queue record carries the content digest, got %+v", payload)
		}
		if _, ok := payload["content_text"]; ok {
			t.Fatal("the content NEVER enters the queue payload (AC-10)")
		}
		if payload["purpose"] != casesDomain.ExternalPurposePromise {
			t.Fatalf("the queue record carries the dictionary words, got %+v", payload)
		}
		card := extCardOf(t, svc, w, caseID, extID(105), 1)
		if !card.Enqueued || card.Delivery.State != casesDomain.ExternalDeliveryQueued {
			t.Fatalf("the card shows the queued state with the queue evidence, got %+v", card)
		}
		if events := extEvents(t, w, extID(105), 1); len(events) != 0 {
			t.Fatalf("the enqueue never writes dispatch events, got %+v", events)
		}
		// The first drain submits the queued record (the words leave ONLY
		// through the dispatcher — the row also closes its own record so the
		// later rows consider their own material only).
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		if result := extDrain(t, svc, w); result.Submitted != 1 {
			t.Fatalf("the queued record submits on the drain, got %+v", result)
		}
	})

	// Row 11 — the enqueue atomicity probe: the pre-committed dedup twin of
	// a crashed run makes the transaction fail on the outbox conflict —
	// NOTHING of the failed transaction leaks (the twin's record is the
	// only one, the dispatch history stays empty, the approval row of the
	// EARLIER command is untouched).
	t.Run("row-11 enqueue atomicity: the dedup twin conflict leaves NOTHING new", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(106), casesDomain.ExternalPurposePromise, "Обещаем счёт до конца дня", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(106), 1)
		externalSeedQueueTwin(t, w, caseID, extID(106), 1)
		_, err := svc.EnqueueExternalCommitment(ctx, extEnqueueCommand(w, caseID, workItemID, extID(106), 1))
		assertRuleError(t, err, "Версия уже стоит в очереди")
		if outboxQueuedCount(t, w, extID(106), 1) != 1 {
			t.Fatal("the twin's record is the only queue record — the failed transaction leaks nothing")
		}
		if events := extEvents(t, w, extID(106), 1); len(events) != 0 {
			t.Fatalf("the failed transaction never writes dispatch events, got %+v", events)
		}
		// The crashed twin's record is REAL queue material (persist-before-
		// emit: the committed record survives the crash): the operator drain
		// dispatches it honestly — the row closes its own phantom so the
		// later rows consider their own material only.
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		if result := extDrain(t, svc, w); result.Submitted != 1 {
			t.Fatalf("the twin's committed record drains honestly, got %+v", result)
		}
	})

	// Row 12 — the concurrent enqueue of the same approved version: the
	// advisory anchors serialise the twins, the dedup UNIQUE is the second
	// line — exactly ONE queue record exists, one twin answers the honest
	// raced-conflict words.
	t.Run("row-12 concurrent enqueue: exactly ONE queue record", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(107), casesDomain.ExternalPurposePromise, "Обещаем обновление статуса заказа", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(107), 1)
		twin := externalService(w, fixedClock{extNow}, dispatcher, externalMatrixIDs(3325, 64), nil)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var landed, raced int
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := twin.EnqueueExternalCommitment(ctx, extEnqueueCommand(w, caseID, workItemID, extID(107), 1))
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					landed++
					return
				}
				if strings.Contains(err.Error(), "Версия уже стоит в очереди") {
					raced++
					return
				}
				t.Errorf("the racing twin answers the honest conflict words, got %v", err)
			}()
		}
		wg.Wait()
		if landed != 1 || raced != 1 {
			t.Fatalf("exactly one enqueue lands, one races honestly: landed=%d raced=%d", landed, raced)
		}
		if outboxQueuedCount(t, w, extID(107), 1) != 1 {
			t.Fatal("exactly ONE queue record exists after the race")
		}
		if events := extEvents(t, w, extID(107), 1); len(events) != 0 {
			t.Fatalf("the race never writes dispatch events, got %+v", events)
		}
		// The winner's record decides on the drain (the row closes its own
		// material for the later rows).
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		if result := extDrain(t, svc, w); result.Submitted != 1 {
			t.Fatalf("the winner's record submits on the drain, got %+v", result)
		}
	})

	// Row 13 — the accepted outcome: the drain submits the queued record
	// with the external correlation of the AC-8 evidence and the LIVE
	// idempotent guarantee of the served capability declaration; the SECOND
	// drain considers nothing — the submitted record is structurally
	// unselectable, the second Staffy intent never exists.
	t.Run("row-13 drain accepted: submitted with the correlation and the LIVE guarantee; the re-drain considers nothing", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(108), casesDomain.ExternalPurposePromise, "Обещаем отправку подтверждения после проверки данных", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(108), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(108), 1)
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		result := extDrain(t, svc, w)
		if result.Submitted != 1 || result.Considered != 1 {
			t.Fatalf("the accepted outcome submits the queued record, got %+v", result)
		}
		events := extEvents(t, w, extID(108), 1)
		if len(events) != 1 || events[0].Kind != casesDomain.ExternalDispatchAttemptSubmitted || events[0].AttemptNo != 1 {
			t.Fatalf("one attempt_submitted event of the attempt №1, got %+v", events)
		}
		key := casesDomain.ExternalCommitmentIdempotencyKey(w.tenant, extID(108), 1)
		if events[0].Correlation != "syn-"+key[:16] {
			t.Fatalf("the correlation derives from the idempotency key of the version pin, got %q", events[0].Correlation)
		}
		if !events[0].Guarantee {
			t.Fatal("the LIVE declaration of command.send-sms carries the idempotent guarantee")
		}
		card := extCardOf(t, svc, w, caseID, extID(108), 1)
		if card.Delivery.State != casesDomain.ExternalDeliverySubmitted || card.Delivery.LastExternalCorrelationID != events[0].Correlation {
			t.Fatalf("submitted is never a delivery — the card carries the derivation and the evidence, got %+v", card.Delivery)
		}
		if again := extDrain(t, svc, w); again.Considered != 0 {
			t.Fatalf("the submitted record never re-selects — the second intent does not exist, got %+v", again)
		}
	})

	// Row 14 — the delivered outcome is the two-event truth of ONE call (the
	// words left + the channel receipt); the operator relay of the
	// acknowledged observation appends the honest third event; the relay
	// refusals are the queue evidence and the observation dictionary.
	t.Run("row-14 drain delivered: TWO events of ONE call; the relay acknowledged; the honest relay refusals", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(109), casesDomain.ExternalPurposePromise, "Обещаем доставку подготовленных материалов к вечеру", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(109), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(109), 1)
		dispatcher.Script(casesDomain.DispatchOutcomeDelivered, "", nil)
		if result := extDrain(t, svc, w); result.Delivered != 1 {
			t.Fatalf("the delivered outcome answers the two-event truth, got %+v", result)
		}
		events := extEvents(t, w, extID(109), 1)
		if len(events) != 2 || events[0].Kind != casesDomain.ExternalDispatchAttemptSubmitted || events[1].Kind != casesDomain.ExternalDispatchReceiptDelivered || events[1].AttemptNo != 0 {
			t.Fatalf("the attempt evidence and the channel receipt of ONE call, got %+v", events)
		}
		if card := extCardOf(t, svc, w, caseID, extID(109), 1); card.Delivery.State != casesDomain.ExternalDeliveryDelivered {
			t.Fatalf("delivered is never an acceptance — the fixed words differ materially, got %+v", card.Delivery)
		}
		outcome, err := svc.RelayExternalReceipt(ctx, extRelayCommand(w, caseID, workItemID, extID(109), 1, casesDomain.RelayObservationAcknowledged))
		if err != nil || outcome.Kind != casesDomain.ExternalDispatchReceiptAcknowledged {
			t.Fatalf("the operator relay of the acknowledged observation appends the receipt, got %+v %v", outcome, err)
		}
		if events = extEvents(t, w, extID(109), 1); len(events) != 3 {
			t.Fatalf("the relay observation appends the third event, got %+v", events)
		}
		if card := extCardOf(t, svc, w, caseID, extID(109), 1); card.Delivery.State != casesDomain.ExternalDeliveryAcknowledged {
			t.Fatalf("the acceptance is the distinct word of the receipt, got %+v", card.Delivery)
		}
		_, err = svc.RelayExternalReceipt(ctx, extRelayCommand(w, caseID, workItemID, extID(104), 1, casesDomain.RelayObservationDelivered))
		assertRuleError(t, err, "не состоит в очереди")
		_, err = svc.RelayExternalReceipt(ctx, extRelayCommand(w, caseID, workItemID, extID(109), 1, "передано"))
		assertRuleError(t, err, "не входит в словарь")
	})

	// Row 15 — the failed outcome retries within the bound (the same
	// idempotency key, the honest attempt ladder 1..3); the exhausted bound
	// parks the record structurally — the pending selection excludes the
	// attempt-maxed version, the blind loop never exists.
	t.Run("row-15 drain failed: the bounded attempts, the structural park", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(110), casesDomain.ExternalPurposePromise, "Обещаем обновление статуса после согласования", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(110), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(110), 1)
		dispatcher.Script(casesDomain.DispatchOutcomeFailed, "", nil)
		for attempt := 1; attempt <= 3; attempt++ {
			result := extDrain(t, svc, w)
			if result.FailedRetry != 1 || result.Considered != 1 {
				t.Fatalf("the pre-send refusal retries within the bound (attempt %d), got %+v", attempt, result)
			}
			events := extEvents(t, w, extID(110), 1)
			if len(events) != attempt || events[attempt-1].Kind != casesDomain.ExternalDispatchAttemptFailed || events[attempt-1].AttemptNo != int64(attempt) {
				t.Fatalf("the honest attempt ladder 1..%d, got %+v", attempt, events)
			}
		}
		if parked := extDrain(t, svc, w); parked.Considered != 0 {
			t.Fatalf("the exhausted bound parks the record structurally, got %+v", parked)
		}
		if card := extCardOf(t, svc, w, caseID, extID(110), 1); card.Delivery.State != casesDomain.ExternalDeliveryFailed {
			t.Fatalf("the honest failure state, got %+v", card.Delivery)
		}
	})

	// Row 16 — the timeout is the possible-send ambiguity: the retry of the
	// unknown is decided by the RECORDED capability guarantee of the last
	// attempt (a guaranteed timeout retries, a guarantee-free one parks —
	// the blind retry is structurally unselectable); the repeated attempts
	// of the same version carry the SAME idempotency key — the provider
	// correlation stays byte-stable.
	t.Run("row-16 drain timeout: the RECORDED guarantee decides the retry, the key stays stable", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(111), casesDomain.ExternalPurposePromise, "Обещаем выгрузку документов во внешнюю систему", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(111), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(111), 1)
		key := casesDomain.ExternalCommitmentIdempotencyKey(w.tenant, extID(111), 1)
		wantCorrelation := "syn-" + key[:16]
		for attempt := 1; attempt <= 3; attempt++ {
			dispatcher.Script(casesDomain.DispatchOutcomeTimeout, "", nil)
			dispatcher.ScriptGuarantee(attempt < 3)
			result := extDrain(t, svc, w)
			if result.UnknownParked != 1 {
				t.Fatalf("the timeout parks honestly (attempt %d), got %+v", attempt, result)
			}
			events := extEvents(t, w, extID(111), 1)
			if len(events) != attempt {
				t.Fatalf("the append-only ladder of the unknown attempts, got %+v", events)
			}
			event := events[attempt-1]
			if event.Kind != casesDomain.ExternalDispatchAttemptUnknown || event.Correlation != wantCorrelation {
				t.Fatalf("the same version — the same key and correlation (attempt %d), got %+v", attempt, event)
			}
			if (attempt < 3) != event.Guarantee {
				t.Fatalf("the RECORDED guarantee word travels with the attempt %d, got %+v", attempt, event)
			}
		}
		if parked := extDrain(t, svc, w); parked.Considered != 0 {
			t.Fatalf("the guarantee-free unknown never blindly retries, got %+v", parked)
		}
	})

	// Row 17 — the execute-time stale fence newer_version: the superseded
	// queued version gases with the honest reason BEFORE any send (the
	// accepted script stays untouched — the words provably never left), the
	// derivation returns the record to persisted with the reason material
	// in the history.
	t.Run("row-17 stale fence newer_version: the gasp before the send, the state returns to persisted", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(112), casesDomain.ExternalPurposePromise, "Обещаем резервирование деталей на складе", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(112), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(112), 1)
		extCreate(t, svc, w, caseID, workItemID, extID(112), casesDomain.ExternalPurposePromise, "Обещаем резервирование деталей на складе (срок изменён)", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		result := extDrain(t, svc, w)
		if result.Gassed != 1 || result.Submitted != 0 {
			t.Fatalf("the superseded version gases before the send, got %+v", result)
		}
		events := extEvents(t, w, extID(112), 1)
		if len(events) != 1 || events[0].Kind != casesDomain.ExternalDispatchCancelledStale || events[0].Reason != casesDomain.ExternalStaleNewerVersion {
			t.Fatalf("one cancelled_stale event with the newer_version reason, got %+v", events)
		}
		card := extCardOf(t, svc, w, caseID, extID(112), 1)
		if card.Delivery.State != casesDomain.ExternalDeliveryPersisted || card.Delivery.LastKind != casesDomain.ExternalDispatchCancelledStale {
			t.Fatalf("the words never left — the state returns to persisted with the reason material, got %+v", card.Delivery)
		}
	})

	// Row 18 — the execute-time stale fence expired: the window closes
	// between the enqueue and the drain — the later-clock drain reads the
	// honest expired fence (the 2.3 threshold-window precedent of the
	// later-clock service instance).
	t.Run("row-18 stale fence expired: the later-clock drain gases honestly", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(113), casesDomain.ExternalPurposePromise, "Обещаем обратный звонок в течение получаса", extNow.Add(30*time.Minute), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(113), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(113), 1)
		later := externalService(w, fixedClock{extNow.Add(time.Hour)}, dispatcher, externalMatrixIDs(3330, 64), nil)
		if result := extDrain(t, later, w); result.Gassed != 1 {
			t.Fatalf("the closed window gases the queued record, got %+v", result)
		}
		events := extEvents(t, w, extID(113), 1)
		if len(events) != 1 || events[0].Reason != casesDomain.ExternalStaleExpired {
			t.Fatalf("one cancelled_stale event with the expired reason, got %+v", events)
		}
	})

	// Row 19 — the execute-time stale fence owner_epoch: the resolved
	// ownership transfer moves the epoch under the queue — the drain gases
	// the queued version of the departed epoch with the honest reason (the
	// words never leave on a stale owner material).
	t.Run("row-19 stale fence owner_epoch: the moved ownership gases the queued version", func(t *testing.T) {
		case2, work2 := w.acceptedCase(t, "evt-ext-dispatch-m2", "sess-ext-dispatch-m2", `{"call":"inbound-call","from":"+79000000332"}`)
		extCreate(t, svc, w, case2, work2, extID(114), casesDomain.ExternalPurposePromise, "Обещаем продолжение работ после смены владельца", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, case2, work2, extID(114), 1)
		extEnqueue(t, svc, w, case2, work2, extID(114), 1)
		request, err := svc.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: case2, ExpectedEpoch: 1,
			ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
			WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "передача владения на время отпуска",
			IdempotencyKey: "ext-matrix-tr-1", Now: extNow,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = svc.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: case2, RequestID: request.TransferRequestID,
			Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение кейсом",
			IdempotencyKey: "ext-matrix-tr-2", Now: extNow.Add(time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		result := extDrain(t, svc, w)
		if result.Gassed != 1 || result.Considered != 1 {
			t.Fatalf("the moved epoch gases the queued version, got %+v", result)
		}
		events := extEvents(t, w, extID(114), 1)
		if len(events) != 1 || events[0].Reason != casesDomain.ExternalStaleOwnerEpoch {
			t.Fatalf("one cancelled_stale event with the owner_epoch_moved reason, got %+v", events)
		}
	})

	// Row 20 — the LIVE 3.2 enforcement point: the real control Set of the
	// channel pause gases the queued record with the control_mode reason;
	// the resumed control opens the honest recovery path — the gassed
	// record never re-selects, the NEW version of the same envelope passes
	// the resumed accept through the whole queue discipline.
	t.Run("row-20 the LIVE control fence: paused gases, the resumed accept opens the NEW version path", func(t *testing.T) {
		controls := controlApp.New(controlStore.NewPostgres(runtimePool), identityLive, connectorrefresolver.New(), fixedClock{extNow}, &sequenceIDs{values: controlMatrixIDs(16)})
		point, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindChannel, Ref: "phone", Mode: controlDomain.ControlModePaused,
			Reason:             "инцидент телефонии: внешние отправки остановлены",
			ExpectedGeneration: 0, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b503"),
		})
		if err != nil {
			t.Fatal(err)
		}
		extCreate(t, svc, w, caseID, workItemID, extID(115), casesDomain.ExternalPurposePromise, "Обещаем отправку данных поставщику", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(115), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(115), 1)
		result := extDrain(t, svc, w)
		if result.Gassed != 1 || result.Submitted != 0 {
			t.Fatalf("the LIVE paused control gases the queued record, got %+v", result)
		}
		events := extEvents(t, w, extID(115), 1)
		if len(events) != 1 || events[0].Reason != casesDomain.ExternalStaleControlMode {
			t.Fatalf("one cancelled_stale event with the control_mode reason — the promised enforcement point, got %+v", events)
		}
		if _, err = controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindChannel, Ref: "phone", Reason: "телефония восстановлена",
			ExpectedGeneration: point.Generation, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b504"),
		}); err != nil {
			t.Fatal(err)
		}
		extCreate(t, svc, w, caseID, workItemID, extID(115), casesDomain.ExternalPurposePromise, "Повторное обещание после восстановления телефонии", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(115), 2)
		extEnqueue(t, svc, w, caseID, workItemID, extID(115), 2)
		dispatcher.Script(casesDomain.DispatchOutcomeAccepted, "", nil)
		if resumed := extDrain(t, svc, w); resumed.Submitted != 1 || resumed.Gassed != 0 {
			t.Fatalf("the resumed control lets the NEW version's words through, got %+v", resumed)
		}
	})

	// Row 21 — the integrity alarm: the corrupted content token (the admin
	// probe bypasses the guards — the corruption the drain must catch) is
	// NEVER a gasp and NEVER a send: the record stays queued with the alarm
	// report, the history stays empty, the repeatable alarm names the
	// reconciliation path.
	t.Run("row-21 integrity alarm: the corrupted token skips, never gases, never sends", func(t *testing.T) {
		extCreate(t, svc, w, caseID, workItemID, extID(116), casesDomain.ExternalPurposePromise, "Обещаем согласование с бухгалтерией поставщика", extNow.Add(48*time.Hour), casesDomain.ExternalCommitmentPin{})
		extApprove(t, svc, w, caseID, workItemID, extID(116), 1)
		extEnqueue(t, svc, w, caseID, workItemID, extID(116), 1)
		if _, err := adminPool.Exec(ctx, `update cases.external_commitments set content_token=$3 where tenant_id=$1 and ext_commitment_id=$2 and version_no=1`, w.tenant, extID(116), strings.Repeat("f", 64)); err != nil {
			t.Fatal(err)
		}
		result := extDrain(t, svc, w)
		if result.Skipped != 1 || result.Gassed != 0 || result.Submitted != 0 {
			t.Fatalf("the integrity alarm skips the corrupted record, got %+v", result)
		}
		if len(result.Reports) != 1 || result.Reports[0].ExtCommitmentID != extID(116) || !strings.Contains(result.Reports[0].Text, "Токен версии не перепровернулся") {
			t.Fatalf("the alarm report names the reconciliation path, got %+v", result.Reports)
		}
		if events := extEvents(t, w, extID(116), 1); len(events) != 0 {
			t.Fatalf("the alarm never writes the history, got %+v", events)
		}
		if again := extDrain(t, svc, w); again.Skipped != 1 || again.Gassed != 0 {
			t.Fatalf("the record stays queued for the human — the alarm is repeatable, got %+v", again)
		}
	})

	// Row 22 — the database discipline of 00017: RLS + FORCE on the three
	// tables, the append-only grants of the runtime role (INSERT/SELECT
	// only), the blind cross-tenant read of the runtime role without the
	// tenant setting (the RLS mirror of the 404 gate) and the CHECK
	// backstops of the frozen vocabulary — the dishonest knobs refuse at
	// the database boundary, the honest template lands.
	t.Run("row-22 database discipline: RLS+FORCE, the append-only grants, the blind read, the CHECK backstops", func(t *testing.T) {
		rows, err := runtimePool.Query(ctx, `
                        select relname, relrowsecurity, relforcerowsecurity from pg_class
                        where relnamespace = 'cases'::regnamespace
                          and relname in ('external_commitments','external_commitment_approvals','external_dispatch_events')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		seen := map[string][2]bool{}
		for rows.Next() {
			var name string
			var rls, force bool
			if err = rows.Scan(&name, &rls, &force); err != nil {
				t.Fatal(err)
			}
			seen[name] = [2]bool{rls, force}
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"external_commitments", "external_commitment_approvals", "external_dispatch_events"} {
			if state, ok := seen[table]; !ok || !state[0] || !state[1] {
				t.Fatalf("%s must carry RLS + FORCE (the append-only discipline), got %+v", table, state)
			}
			for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
				var allowed bool
				if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases."+table, grant).Scan(&allowed); err != nil {
					t.Fatal(err)
				}
				if (grant == "INSERT" || grant == "SELECT") != allowed {
					t.Fatalf("%s on cases.%s: allowed=%v — the append-only grants", grant, table, allowed)
				}
			}
		}
		var blind int
		if err := runtimePool.QueryRow(ctx, `select count(*) from cases.external_commitments where tenant_id=$1`, w.tenant).Scan(&blind); err != nil {
			t.Fatal(err)
		}
		if blind != 0 {
			t.Fatal("the runtime role without the tenant setting sees NOTHING — the RLS mirror of the 404 gate")
		}
		var visible int
		if err := adminPool.QueryRow(ctx, `select count(*) from cases.external_commitments where tenant_id=$1`, w.tenant).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible == 0 {
			t.Fatal("the admin probe sees the honest rows of the tenant")
		}
		envelope := int64(901)
		insertEnvelope := func(purpose, channel, content string, pinCommitment, pinVersion, pinToken any) error {
			envelope++
			_, err := adminPool.Exec(ctx, `
                                insert into cases.external_commitments (tenant_id, case_id, work_item_id, ext_commitment_id, version_no,
                                        purpose, recipient_kind, channel, content_text, valid_until, expected_next_step, ack_requirement,
                                        pin_commitment_id, pin_version_no, pin_content_token,
                                        owner_kind, owner_ref, owner_epoch, decision_no, profile_version, policy_outcome_no,
                                        content_token, actor_id, created_at)
                                values ($1,$2,$3,$4,1,$5,'client',$6,$7,$8,'проверка доменных слов',$9,$10,$11,$12,'human',$13,1,0,0,0,$14,$13,$8)`,
				w.tenant, caseID, workItemID, extID(envelope), purpose, channel, content, extNow.Add(24*time.Hour),
				"not_required", pinCommitment, pinVersion, pinToken, w.owner.ID, strings.Repeat("0", 64))
			return err
		}
		if err := insertEnvelope(casesDomain.ExternalPurposePromise, "phone", "честная строка проверки", nil, nil, nil); err != nil {
			t.Fatalf("the honest template lands — the knobs below fail on their own dishonesty: %v", err)
		}
		if err := insertEnvelope("pledge", "phone", "честная строка проверки", nil, nil, nil); err == nil {
			t.Fatal("the foreign purpose word refuses at the CHECK backstop")
		}
		if err := insertEnvelope(casesDomain.ExternalPurposePromise, "sms", "честная строка проверки", nil, nil, nil); err == nil {
			t.Fatal("the foreign channel word refuses at the CHECK backstop")
		}
		if err := insertEnvelope(casesDomain.ExternalPurposePromise, "phone", "", nil, nil, nil); err == nil {
			t.Fatal("the empty content refuses at the CHECK backstop")
		}
		if err := insertEnvelope(casesDomain.ExternalPurposeOffer, "phone", "честная строка проверки", nil, nil, nil); err == nil {
			t.Fatal("the offer without the pin refuses at the CHECK backstop")
		}
		if err := insertEnvelope(casesDomain.ExternalPurposePromise, "phone", "честная строка проверки", extID(901), nil, strings.Repeat("a", 64)); err == nil {
			t.Fatal("the partial pin refuses — the all-or-nothing backstop")
		}
		event := int64(950)
		eventNo := int64(0)
		insertEvent := func(kind string, reason any, attemptNo int64) error {
			event++
			eventNo++
			_, err := adminPool.Exec(ctx, `
                                insert into cases.external_dispatch_events (tenant_id, case_id, ext_commitment_id, version_no,
                                        event_id, event_no, kind, reason, attempt_no, external_correlation_id, idempotent_guarantee, details, actor_id, created_at)
                                values ($1,$2,$3,1,$4,$5,$6,$7,$8,'',false,'',$9,$10)`,
				w.tenant, caseID, extID(101), extID(event), eventNo, kind, reason, attemptNo, w.owner.ID, extNow)
			return err
		}
		if err := insertEvent(casesDomain.ExternalDispatchReceiptFailed, nil, 0); err != nil {
			t.Fatalf("the honest receipt event lands: %v", err)
		}
		if err := insertEvent("queued", nil, 0); err == nil {
			t.Fatal("the queued word is NOT an event kind — the vocabulary backstop")
		}
		if err := insertEvent(casesDomain.ExternalDispatchAttemptSubmitted, nil, 0); err == nil {
			t.Fatal("the attempt shape backstop refuses the zero attempt of an attempt kind")
		}
		if err := insertEvent(casesDomain.ExternalDispatchAttemptSubmitted, "почина", 1); err == nil {
			t.Fatal("the reason shape backstop refuses the reason on the non-gasp kind")
		}
		if err := insertEvent(casesDomain.ExternalDispatchCancelledStale, nil, 0); err == nil {
			t.Fatal("the reason shape backstop refuses the gasp without the reason")
		}
	})
}

// extNow is the command clock of the Story 3-3 matrix: one hour after the
// ownership world — the pinned 2.9 version of the offer row stays inside
// its validity window (the 2.3 threshold precedent of the later-clock
// legs).
var extNow = ownershipNow.Add(1 * time.Hour)

// extEventRow is the adminPool probe row of the append-only history.
type extEventRow struct {
	EventNo     int64
	Kind        string
	Reason      string
	AttemptNo   int64
	Correlation string
	Guarantee   bool
}

// extID mints the deterministic envelope identifier of the matrix (the
// 019a3f33 range of 3-3; the 2.9 world owns 019a2f4c).
func extID(row int64) id.UUID {
	return id.MustParse(fmt.Sprintf("019a3f33-7000-7000-8000-%012d", row))
}

// externalMatrixIDs mints the id pool of the matrix world (the 512-value
// headroom over the 64 of the suite helper — the two ingests, the
// commands and the drain events of one matrix share ONE pool).
func externalMatrixIDs(prefix, count int) *sequenceIDs {
	values := make([]id.UUID, 0, count)
	for i := 0; i < count; i++ {
		values = append(values, id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", prefix*1000+i)))
	}
	return &sequenceIDs{values: values}
}

// externalService wires the REAL cases service of the Story 3-3 contour:
// the live store, the live identity, the synthetic dispatch port of the
// test contour (AG-4: production wires none) and the REAL composition
// control adapter of the promised 3.2 enforcement point.
func externalService(w *casesWorld, clock casesAPI.Clock, dispatcher *externaldispatcher.Synthetic, ids *sequenceIDs, scenarios map[string]observationsyn.Scenario) *casesApp.Service {
	return casesApp.New(casesStore.NewPostgres(runtimePool).WithIDs(ids), w.identity, ingresssyn.New(w.connector), clock, ids).
		WithRouteFacts(w.employee).
		WithConnectionFacts(w.connector).
		WithMembers(w.identity).
		WithPackTaxonomy(w.employee).
		WithIdentityVerifier(verificationsyn.New()).
		WithOwningSystemReader(observationsyn.New(scenarios)).
		WithExternalDispatcher(dispatcher).
		WithControlEffective(controleffectiveresolver.New(controlStore.NewPostgres(runtimePool), clock))
}

// externalDispatchWorld builds the matrix world: the cases world whose
// service is replaced by the Story 3-3 contour (the commitmentWorld
// precedent — every helper of the suite walks the same service), with the
// price observation scenario of the LIVE 2.9 pin rows.
func externalDispatchWorld(t *testing.T, seed int) (*casesWorld, *casesApp.Service, *externaldispatcher.Synthetic) {
	t.Helper()
	w := newCasesWorld(t, seed)
	dispatcher := externaldispatcher.New()
	w.cases = externalService(w, fixedClock{extNow}, dispatcher, externalMatrixIDs(seed*10+300, 512), map[string]observationsyn.Scenario{
		casesDomain.MaterialFieldPrice: {Word: observationsyn.ScenarioOK, Value: commitmentPriceValue(t)},
	})
	return w, w.cases, dispatcher
}

// extCreateCommand builds the honest create command of the matrix rows (the
// raw form for the refusal legs — the fatal form lives in extCreate).
func extCreateCommand(w *casesWorld, actor identityDomain.Principal, caseID, workItemID, envelope id.UUID, purpose string, validUntil time.Time, pin casesDomain.ExternalCommitmentPin) casesAPI.CreateExternalCommitmentCommand {
	return casesAPI.CreateExternalCommitmentCommand{
		Actor: actor, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		ExtCommitmentID: envelope, Purpose: purpose, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещаем отправку подготовленных материалов внешней системе",
		ValidUntil: validUntil, ExpectedNextStep: "ожидание диспетчеризации внешнего обязательства",
		AckRequirement: casesDomain.ExternalAckNotRequired, Pin: pin, Now: extNow,
	}
}

// extCreate is the honest create shorthand (the fatal form).
func extCreate(t *testing.T, svc *casesApp.Service, w *casesWorld, caseID, workItemID, envelope id.UUID, purpose, content string, validUntil time.Time, pin casesDomain.ExternalCommitmentPin) casesAPI.ExternalCommitmentOutcome {
	t.Helper()
	command := extCreateCommand(w, w.ownershipActor(), caseID, workItemID, envelope, purpose, validUntil, pin)
	command.ContentText = content
	outcome, err := svc.CreateExternalCommitment(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// extApproveCommand builds the approval command of the exact version.
func extApproveCommand(w *casesWorld, actor identityDomain.Principal, caseID, workItemID, envelope id.UUID, versionNo int64) casesAPI.ApproveExternalCommitmentCommand {
	return casesAPI.ApproveExternalCommitmentCommand{
		Actor: actor, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		ExtCommitmentID: envelope, VersionNo: versionNo, Now: extNow,
	}
}

// extApprove approves the exact version by the acting owner (the fatal
// form).
func extApprove(t *testing.T, svc *casesApp.Service, w *casesWorld, caseID, workItemID, envelope id.UUID, versionNo int64) casesAPI.ExternalApprovalOutcome {
	t.Helper()
	outcome, err := svc.ApproveExternalCommitment(context.Background(), extApproveCommand(w, w.ownershipActor(), caseID, workItemID, envelope, versionNo))
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// extEnqueueCommand builds the queue command of the exact version.
func extEnqueueCommand(w *casesWorld, caseID, workItemID, envelope id.UUID, versionNo int64) casesAPI.EnqueueExternalCommitmentCommand {
	return casesAPI.EnqueueExternalCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		ExtCommitmentID: envelope, VersionNo: versionNo, Now: extNow,
	}
}

// extEnqueue puts the exact version into the queue (the fatal form).
func extEnqueue(t *testing.T, svc *casesApp.Service, w *casesWorld, caseID, workItemID, envelope id.UUID, versionNo int64) casesAPI.ExternalEnqueueOutcome {
	t.Helper()
	outcome, err := svc.EnqueueExternalCommitment(context.Background(), extEnqueueCommand(w, caseID, workItemID, envelope, versionNo))
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// extRelayCommand builds the operator relay command.
func extRelayCommand(w *casesWorld, caseID, workItemID, envelope id.UUID, versionNo int64, observation string) casesAPI.RelayExternalReceiptCommand {
	return casesAPI.RelayExternalReceiptCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		ExtCommitmentID: envelope, VersionNo: versionNo, Observation: observation, Now: extNow,
	}
}

// extDrain is the operator drain of the matrix (the fatal form).
func extDrain(t *testing.T, svc *casesApp.Service, w *casesWorld) casesAPI.ExternalDrainResult {
	t.Helper()
	result, err := svc.DrainExternalCommitments(context.Background(), w.ownershipActor(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// extCardOf resolves one version card of the Commitment Review view.
func extCardOf(t *testing.T, svc *casesApp.Service, w *casesWorld, caseID, envelope id.UUID, versionNo int64) casesAPI.ExternalCommitmentVersionCard {
	t.Helper()
	view, err := svc.ReadExternalCommitmentView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range view.Items {
		for _, card := range section.Versions {
			if card.Version.ExtCommitmentID == envelope && card.Version.VersionNo == versionNo {
				return card
			}
		}
	}
	t.Fatalf("the version card %s/%d not found", envelope, versionNo)
	return casesAPI.ExternalCommitmentVersionCard{}
}

// extEvents reads the append-only history of one version over the admin
// pool (the superuser bypasses RLS — the probe reads the honest rows).
func extEvents(t *testing.T, w *casesWorld, envelope id.UUID, versionNo int64) []extEventRow {
	t.Helper()
	rows, err := adminPool.Query(context.Background(), `
                select event_no, kind, coalesce(reason, '') as reason, attempt_no, external_correlation_id, idempotent_guarantee
                from cases.external_dispatch_events
                where tenant_id=$1 and ext_commitment_id=$2 and version_no=$3
                order by event_no`, w.tenant, envelope, versionNo)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	events := []extEventRow{}
	for rows.Next() {
		var row extEventRow
		if err := rows.Scan(&row.EventNo, &row.Kind, &row.Reason, &row.AttemptNo, &row.Correlation, &row.Guarantee); err != nil {
			t.Fatal(err)
		}
		events = append(events, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

// outboxQueuedCount counts the queue evidence of one version (the admin
// pool probe of the persist-before-emit record).
func outboxQueuedCount(t *testing.T, w *casesWorld, envelope id.UUID, versionNo int64) int {
	t.Helper()
	var n int
	if err := adminPool.QueryRow(context.Background(), `
                select count(*) from cases.outbox_events
                where tenant_id=$1 and event_type='external_commitment.queued' and dedup_key=$2`,
		w.tenant, casesDomain.DedupKeyExternalCommitmentQueued(envelope, versionNo)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// extQueuedPayload reads the safe payload JSON of the queue record (the
// AC-10 probe material).
func extQueuedPayload(t *testing.T, w *casesWorld, envelope id.UUID, versionNo int64) map[string]any {
	t.Helper()
	var payload []byte
	if err := adminPool.QueryRow(context.Background(), `
                select payload from cases.outbox_events
                where tenant_id=$1 and event_type='external_commitment.queued' and dedup_key=$2`,
		w.tenant, casesDomain.DedupKeyExternalCommitmentQueued(envelope, versionNo)).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var material map[string]any
	if err := json.Unmarshal(payload, &material); err != nil {
		t.Fatal(err)
	}
	return material
}

// externalSeedQueueTwin pre-commits the dedup twin of a crashed enqueue run
// (the admin pool simulates the crashed writer's committed record — the
// probe input of the atomicity row; the payload carries the version pin of
// the victim, the way the honest writer would).
func externalSeedQueueTwin(t *testing.T, w *casesWorld, caseID, envelope id.UUID, versionNo int64) {
	t.Helper()
	var sourceConnection id.UUID
	if err := adminPool.QueryRow(context.Background(), `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		w.tenant, caseID).Scan(&sourceConnection); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"tenant_id": w.tenant.String(), "case_id": caseID.String(),
		"ext_commitment_id": envelope.String(), "version_no": versionNo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(context.Background(), `
                insert into cases.outbox_events (tenant_id, event_id, event_type, schema_version, aggregate_id, source_connection_id, correlation_id, occurred_at, payload, dedup_key, created_at)
                values ($1,$2,'external_commitment.queued',1,$3,$4,$5,$6,$7,$8,$6)`,
		w.tenant, extID(890), caseID, sourceConnection, extID(891), extNow, string(payload),
		casesDomain.DedupKeyExternalCommitmentQueued(envelope, versionNo)); err != nil {
		t.Fatal(err)
	}
}
