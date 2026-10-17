package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.14 command and view: the producer record of the AI summary
// and the decision context of the assigned operator
// ---------------------------------------------------------------------------

// decisionContextScopeText is the honest scope refusal of the summary
// record and the context read (the shared «review или configuration»
// vocabulary of the 2.5..2.13 rights gates): the decision context is the
// operator working surface — an audit-only member reads the audit
// contours, never the decision assembly.
const decisionContextScopeText = "Контекст передачи и запись сводки требуют полномочия review или configuration: участники только с audit не работают с контекстом передачи обращения."

// casesStoreTerminalSummaryText mirrors the store refusal of the terminal
// case (the honest boundary of the write command — the v1 backstop word
// of the degradation and handoff precedents).
const casesStoreTerminalSummaryText = "Терминальное обращение не принимает записей сводки: обслуживание завершено — материал передан истории."

// The compile-time proof of the narrow interface registration: the
// composite Service carries the summary record command and the context
// read (the transport type-asserts casesAPI.DecisionContextService — the
// HandoffService precedent; the test fakes of the other stories stay
// untouched).
var _ casesAPI.DecisionContextService = (*Service)(nil)

// RecordCaseSummary writes ONE append-only AI summary row (AC-5): the
// rights gate FIRST (review or configuration — an audit-only member
// never writes the producer record), the domain guard of the material
// (the kind word, the rune bounds, the claim discipline with the
// byte-exact spans — the honest 400 before SQL), the replay lookup with
// the whole-material hash (the claims INCLUDED — the same key with
// another material is the honest key conflict), the case anchor (the
// case must exist in the tenant scope and stay open — the terminal
// refusal), the conversation of the lock plan and the single
// transactional write. The store re-validates every guard under the
// case + conversation advisory locks: the ownership gate (the summary is
// the act of the owner-Staffy — a human owner gets the honest NotOwner
// words), the domain guard, the ref sentinels (every linked claim
// anchors the material of THIS case), the SERVER-truth watermark and
// the NEXT monotonic summary number. The outbox envelope carries the
// identifiers, the kind word and the claim counters ONLY — the summary
// text and the claims NEVER enter.
func (s *Service) RecordCaseSummary(ctx context.Context, command casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, decisionContextScopeText)
	}
	// The domain guard of the producer material (the honest 400 with the
	// dictionary words in the text — the domain truth before SQL; the table
	// CHECK words are the backstops).
	if err := rule("record_case_summary", casesDomain.GuardRecordCaseSummary(command.Kind, command.Text, command.Claims)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material INCLUDING the
	// claims with the refs and the disputed marks (the replay of the exact
	// repeat returns the stored outcome; the same key with another material
	// is the honest key conflict).
	hash := requestHash(struct {
		Case   string                     `json:"case"`
		Kind   string                     `json:"kind"`
		Text   string                     `json:"text"`
		Claims []casesDomain.SummaryClaim `json:"claims"`
	}{command.CaseID.String(), command.Kind, command.Text, command.Claims})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_case_summary", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist in the tenant scope and stay
	// open (the store re-validates under the locks).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("record_case_summary", fmt.Errorf("%s", casesStoreTerminalSummaryText))
	}
	// The conversation of the lock plan: the case's own conversation (the
	// SAME sorted plan of the mode/policy/handoff commands — the summary
	// write serializes with the ownership, mode and commitment writers).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The identifiers of the summary envelope (the summary number and the
	// watermark are the SERVER truth — the store derives them under the
	// locks; the request never carries them).
	ids, err := s.handoffIDs(command.Now, 1)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	// The stored outcome carries the request-material words only: the kind
	// word of the single-word dictionary and the claim counter (the summary
	// text and the claims NEVER enter the stored outcome; the number is the
	// server truth the reader learns from the context view).
	outcome := casesAPI.WorkOutcome{
		Command: "record_case_summary", SummaryKind: command.Kind, SummaryClaims: len(command.Claims),
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_case_summary", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		Kind: command.Kind, SummaryText: command.Text, SummaryClaims: command.Claims,
		SummaryOutboxEventID: ids[0], CorrelationID: correlation, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// DecisionContext reads the ONE consistent decision context snapshot of
// the case and compiles the twelve blocks of the operator section (the
// open handoff, the active work items, the mode, the assurance, the
// owner, the three fact groups, the observations, the deliveries, the
// commitments, the preferences, the recommendation projection and the
// last summary with its staleness). The read gate is review or
// configuration (the audit-only member gets the honest 403 of the scope
// words); the advisory locks are NOT taken (the consistent snapshot of
// one transaction is the honest boundary of the GET); the derivations
// are the read-time domain truth, never stored statuses. The section
// appears only when the material exists (the open handoff or the
// recorded summary) — the honest absence of both renders nothing.
func (s *Service) DecisionContext(ctx context.Context, actor domain.Principal, tenantID, caseID id.UUID) (casesAPI.DecisionContextView, error) {
	// The read gate: review or configuration (the decision context is the
	// operator working surface, not the audit material).
	scope, rights, err := s.ownershipScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.DecisionContextView{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.DecisionContextView{}, fmt.Errorf("%w: %s", domain.ErrForbidden, decisionContextScopeText)
	}
	now := s.clock.Now()
	// The Story 2.15 extension: the ONE consistent snapshot carries the
	// correction rows of the case TOGETHER with the context blocks (the
	// same read transaction — the fact rows derive the correction
	// markers over the consistent material; the 2.14 derivation and the
	// DTO mapping stay untouched).
	material, err := s.repository.ReadDecisionContextWithCorrections(ctx, scope, caseID, now)
	if err != nil {
		return casesAPI.DecisionContextView{}, err
	}
	input := material.Input
	// The release lineage word of the confirmation validity: the routing
	// facts live outside the cases schema — the app resolves the word for
	// the derivation (the store snapshot leaves it honestly empty). An
	// unreadable route degrades to the empty word — the confirmations
	// derive the validity with the honest unknown lineage, the read never
	// fails on the routing facts.
	input.CurrentLineage = s.currentLineageOf(ctx, scope, caseID)
	view := casesDomain.DeriveDecisionContext(input)
	out := decisionContextViewOf(view)
	attachContextCorrections(&out, material.Corrections)
	return out, nil
}

// currentLineageOf resolves the release lineage word of the case's own
// conversation (the routing facts of the current outbound route — the
// same resolution the commitment view uses; the failure degrades
// honestly to the empty word).
func (s *Service) currentLineageOf(ctx context.Context, scope tenancy.Context, caseID id.UUID) string {
	conversation, err := s.caseConnection(ctx, scope, caseID)
	if err != nil {
		return ""
	}
	employee, decisionNo, profileVersion, err := s.lineageWord(ctx, scope, conversation)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("employee/%s/%d/%d", employee, decisionNo, profileVersion)
}

// decisionContextViewOf maps the domain view onto the API DTO with the
// honest render texts (the texts never enter the database or the
// outbox — they are the render material of the section and the JSON
// view).
func decisionContextViewOf(view casesDomain.DecisionContextView) casesAPI.DecisionContextView {
	out := casesAPI.DecisionContextView{
		Present: view.Handoff.Present || view.Summary.Present,
		PanelText: "Контекст передачи — производное чтение собранного материала, не новая истина: блоки читаются существующими деривациями историй 2-2..2-13, слова — словари среза. " +
			"Сводка AI — производное представление: сверяйтесь с источниками рядом. Рекомендация — проекция слов плана, не команда и не полномочие.",
		Handoff: casesAPI.ContextHandoffView{
			Present: view.Handoff.Present,
			Reason:  view.Handoff.Reason, ReasonText: casesDomain.HandoffReasonText(view.Handoff.Reason),
			State: view.Handoff.State, StateText: casesDomain.HandoffStateText(view.Handoff.State),
			TurnRef:     view.Handoff.TurnRef.String(),
			AckDeadline: view.Handoff.AckDeadline, AckDeadlineText: casesDomain.HandoffAckDeadlineText(view.Handoff.AckDeadline),
			DeadlineAt: view.Handoff.DeadlineAt,
		},
		WorkItems: make([]casesAPI.ContextWorkItemRow, 0, len(view.WorkItems)),
		Mode: casesAPI.ContextModeView{
			Present: view.Mode.Present, Mode: view.Mode.Mode, ModeVersion: view.Mode.ModeVersion,
		},
		Assurance: casesAPI.ContextAssuranceView{
			IdentityLevel: view.Assurance.IdentityLevel, IdentityMethod: view.Assurance.IdentityMethod,
			IdentityExpiresAt: view.Assurance.IdentityExpiresAt,
			AuthorityLevel:    view.Assurance.AuthorityLevel, AuthorityMethod: view.Assurance.AuthorityMethod,
			AuthorityScope: view.Assurance.AuthorityScope, AuthorityExpiresAt: view.Assurance.AuthorityExpiresAt,
		},
		Owner: casesAPI.ContextOwnerView{
			Present: view.Owner.Present, OwnerKind: view.Owner.OwnerKind,
			OwnerRef: view.Owner.OwnerRef.String(), Epoch: view.Owner.Epoch,
		},
		Facts: casesAPI.ContextFactsView{
			Confirmed:   make([]casesAPI.ContextFactRow, 0, len(view.Facts.Confirmed)),
			Unconfirmed: make([]casesAPI.ContextFactRow, 0, len(view.Facts.Unconfirmed)),
			Conflicting: make([]casesAPI.ContextFactRow, 0, len(view.Facts.Conflicting)),
		},
		Observations: make([]casesAPI.ContextObservationRow, 0, len(view.Observations)),
		Deliveries:   make([]casesAPI.ContextDeliveryRow, 0, len(view.Deliveries)),
		Commitments:  make([]casesAPI.ContextCommitmentRow, 0, len(view.Commitments)),
		Preferences: casesAPI.ContextPreferencesView{
			AccommodationKeys: append([]string{}, view.Preferences.AccommodationKeys...),
			AlternativePaths:  append([]string{}, view.Preferences.AlternativePaths...),
			UsualModalities:   make([]casesAPI.ContextModalityRow, 0, len(view.Preferences.UsualModalities)),
		},
		Recommendation: casesAPI.ContextRecommendationView{
			Present:     view.Recommendation.Present,
			Steps:       append([]string{}, view.Recommendation.Steps...),
			Reason:      view.Recommendation.Reason,
			Checks:      make([]casesAPI.ContextCheckRow, 0, len(view.Recommendation.Checks)),
			AckDeadline: view.Recommendation.AckDeadline, AckDeadlineText: casesDomain.HandoffAckDeadlineText(view.Recommendation.AckDeadline),
			DeadlineAt: view.Recommendation.DeadlineAt,
			Mode:       view.Recommendation.Mode, ModeVersion: view.Recommendation.ModeVersion,
			ProjectionText: casesDomain.RecommendationProjectionText,
		},
		Summary: casesAPI.SummaryView{
			Present: view.Summary.Present, SummaryNo: view.Summary.SummaryNo, Kind: view.Summary.Kind,
			MarkingText: casesDomain.AIMarkingText, Text: view.Summary.Text,
			Claims:                make([]casesAPI.ClaimView, 0, len(view.Summary.Claims)),
			Staleness:             view.Summary.Staleness.Word,
			StalenessText:         casesDomain.ContextStaleText,
			StalenessCountersText: view.Summary.Staleness.StalenessCountersText(),
		},
	}
	for _, item := range view.WorkItems {
		out.WorkItems = append(out.WorkItems, casesAPI.ContextWorkItemRow{
			WorkItemID: item.WorkItemID.String(), Title: item.Title, Type: item.Type, State: item.State,
		})
	}
	appendFactRows := func(rows []casesDomain.FactContextRow, target *[]casesAPI.ContextFactRow) {
		for _, row := range rows {
			*target = append(*target, casesAPI.ContextFactRow{
				FactID: row.FactID.String(), MaterialKey: row.MaterialKey,
				Status: row.Status, StatusText: casesDomain.ContextFactStatusText(row.Status),
				VersionNo: row.VersionNo, SourceType: row.SourceType, SourceID: row.SourceID,
				Channel: row.Channel, ObservedAt: row.ObservedAt,
			})
		}
	}
	appendFactRows(view.Facts.Confirmed, &out.Facts.Confirmed)
	appendFactRows(view.Facts.Unconfirmed, &out.Facts.Unconfirmed)
	appendFactRows(view.Facts.Conflicting, &out.Facts.Conflicting)
	for _, row := range view.Observations {
		out.Observations = append(out.Observations, casesAPI.ContextObservationRow{
			WorkItemID: row.WorkItemID.String(), MaterialKey: row.MaterialKey, OwningSystem: row.OwningSystem,
			ObservedAt: row.ObservedAt, FreshnessWord: row.FreshnessWord, ConflictRule: row.ConflictRule,
			EvidenceStatus: row.EvidenceStatus, Projection: row.Projection,
			ExternalID: row.ExternalID, ExternalVersion: row.ExternalVersion,
		})
	}
	for _, row := range view.Deliveries {
		out.Deliveries = append(out.Deliveries, casesAPI.ContextDeliveryRow{
			ReplyID: row.ReplyID.String(), State: row.State, LastKind: row.LastKind, LastEventAt: row.LastEventAt,
		})
	}
	for _, row := range view.Commitments {
		out.Commitments = append(out.Commitments, casesAPI.ContextCommitmentRow{
			CommitmentID: row.CommitmentID.String(), VersionNo: row.VersionNo, Status: row.Status,
			ValidUntil: row.ValidUntil, ConfirmationStatus: row.ConfirmationStatus,
			TermsKnown: append([]string{}, row.TermsKnown...), TermsUnknown: append([]string{}, row.TermsUnknown...),
			ReceiptText: row.ReceiptText,
		})
	}
	for _, row := range view.Preferences.UsualModalities {
		out.Preferences.UsualModalities = append(out.Preferences.UsualModalities, casesAPI.ContextModalityRow{
			MaterialKey: row.MaterialKey, Word: row.Word,
		})
	}
	for _, row := range view.Recommendation.Checks {
		out.Recommendation.Checks = append(out.Recommendation.Checks, casesAPI.ContextCheckRow{
			MaterialKey: row.MaterialKey, State: row.State,
		})
	}
	for _, claim := range view.Summary.Claims {
		out.Summary.Claims = append(out.Summary.Claims, casesAPI.ClaimView{
			RefType: claim.RefType, RefID: claim.RefID.String(), Disputed: claim.Disputed, Text: claim.Text,
		})
	}
	return out
}
