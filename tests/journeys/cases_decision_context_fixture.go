package main

import (
	"context"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.14 fixture: the producer summary record command over the
// REAL domain guard (GuardRecordCaseSummary — the kind word of the
// single-word dictionary, the rune bounds, the linked-or-disputed
// discipline with the byte-exact spans) and the decision context read
// over the REAL read-time assembly (DeriveDecisionContext +
// DeriveContextStaleness over the fixture material of the 2.2..2.13
// surfaces). The fixture world stores the append-only summary rows; the
// staleness is never stored — every read derives it anew against the
// CURRENT watermark counters (the mirror of the store). The summary
// number and the watermark are the fixture-owned monotonic truths (the
// requests never carry them). The request hashes cover the WHOLE
// command materials INCLUDING the claims: the exact repeats answer the
// stored outcomes (the replays), the same keys with other materials
// are the honest conflicts.

// fixtureSummaryID generates the next deterministic summary material id
// (the counter space of the Story 2.14 surface — the ids never collide
// with the other fixture surfaces).
func (f *casesFixture) fixtureSummaryID() id.UUID {
	f.summarySeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(70000+f.summarySeq)))
}

// fixtureSummaryTerminalText mirrors the store refusal of the terminal
// case (the honest boundary of the write command).
const fixtureSummaryTerminalText = "Терминальное обращение не принимает записей сводки: обслуживание завершено — материал передан истории."

// fixtureSummaryNotOwnerText mirrors the store ownership gate refusal
// (the summary is the act of the owner-Staffy of the moment).
const fixtureSummaryNotOwnerText = "Сводку пишет Staffy, пока владеет обращением: текущий владелец — человек; дождитесь возврата владения (return_to_staffy) — новая сводка будет новой записью."

// fixtureSummaryRefForeignText mirrors the store ref guard refusal (a
// foreign or missing ref is a fabrication the store refuses).
const fixtureSummaryRefForeignText = "Утверждение сводки ссылается на материал другого обращения или несуществующий материал: ссылки утверждают факты и наблюдения этого обращения."

// fixtureWatermark derives the CURRENT watermark counters of the case
// material (the staleness comparison truth — the accepted-event count,
// the fact-version count, the owner epoch and the mode version).
func (f *casesFixture) fixtureWatermark(caseID id.UUID) casesDomain.SummaryWatermark {
	watermark := casesDomain.SummaryWatermark{}
	if material := f.material[caseID]; material != nil {
		for _, event := range material.timeline {
			if event.State == "accepted" {
				watermark.EventCount++
			}
		}
	}
	for _, versions := range f.factVersions {
		for _, version := range versions {
			if version.CaseID == caseID {
				watermark.FactVersionCount++
			}
		}
	}
	if state := f.fixtureState(caseID, timeNow()); state.Current != nil {
		watermark.OwnerEpoch = state.Current.Epoch
	}
	watermark.ModeVersion = casesDomain.ModeTermMaxVersion(f.modeTerms[caseID])
	return watermark
}

// fixtureFactOfCase reports whether the fact id belongs to the case (the
// ref guard of the linked claims).
func (f *casesFixture) fixtureFactOfCase(caseID, factID id.UUID) bool {
	prefix := caseID.String() + "|"
	for key, anchor := range f.factAnchors {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix && anchor == factID {
			return true
		}
	}
	return false
}

// fixtureObservationOfCase reports whether the observation id belongs to
// the case (the ref guard of the linked claims).
func (f *casesFixture) fixtureObservationOfCase(caseID, observationID id.UUID) bool {
	for _, observation := range f.observations[caseID] {
		if observation.ObservationID == observationID {
			return true
		}
	}
	return false
}

// RecordCaseSummary writes ONE append-only AI summary row (AC-5): the
// REAL domain guard of the producer material (the kind word, the rune
// bounds, the claim discipline with the byte-exact spans), the
// idempotent command flow with the hash of the WHOLE material INCLUDING
// the claims, the honest refusal of an unknown or a terminal case, the
// ownership gate BEFORE the material sentinels (the summary is the act
// of the owner-Staffy of the moment — a human owner gets the honest
// refusal words), the ref guards (every linked claim anchors a fact or
// an observation of THIS case) and the single write: the append-only
// row with the NEXT monotonic summary number and the SERVER-truth
// watermark (the request never carries them).
func (f *casesFixture) RecordCaseSummary(_ context.Context, command casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The REAL domain guard before any write (the machine checks the
	// structure, never the truth).
	if err := fixtureWorkRule("record_case_summary", casesDomain.GuardRecordCaseSummary(command.Kind, command.Text, command.Claims)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE material INCLUDING the claims.
	outcome, fresh, err := f.workCommandFlow("record_case_summary", command.IdempotencyKey, struct {
		Case   string                     `json:"case"`
		Kind   string                     `json:"kind"`
		Text   string                     `json:"text"`
		Claims []casesDomain.SummaryClaim `json:"claims"`
	}{command.CaseID.String(), command.Kind, command.Text, command.Claims})
	if err != nil || !fresh {
		return outcome, err
	}
	material := f.material[command.CaseID]
	if material == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if material.cases.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_case_summary", fmt.Errorf("%s", fixtureSummaryTerminalText))
	}
	// The ownership gate BEFORE the material sentinels: the summary is
	// the act of the owner-Staffy of the moment.
	state := f.fixtureState(command.CaseID, command.Now)
	if state.Current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if state.Current.OwnerKind != casesDomain.OwnerKindStaffy {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_case_summary", fmt.Errorf("%s", fixtureSummaryNotOwnerText))
	}
	// The ref guards: every linked claim anchors the material of THIS
	// case (a foreign ref is a fabrication the fixture refuses).
	for _, claim := range command.Claims {
		if !casesDomain.ClaimRefTypeInDictionary(claim.RefType) || !claim.RefID.Valid() {
			continue
		}
		linked := false
		switch claim.RefType {
		case casesDomain.ClaimRefFactVersion:
			linked = f.fixtureFactOfCase(command.CaseID, claim.RefID)
		case casesDomain.ClaimRefObservation:
			linked = f.fixtureObservationOfCase(command.CaseID, claim.RefID)
		}
		if !linked {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("record_case_summary", fmt.Errorf("%s", fixtureSummaryRefForeignText))
		}
	}
	// The append-only row: the NEXT monotonic summary number and the
	// SERVER-truth watermark (the fixture-owned truths, the request
	// never carries them).
	summaryNo := int64(0)
	for _, summary := range f.summaries[command.CaseID] {
		if summary.SummaryNo > summaryNo {
			summaryNo = summary.SummaryNo
		}
	}
	summaryNo++
	f.summaries[command.CaseID] = append(f.summaries[command.CaseID], casesDomain.RecordedSummary{
		TenantID: f.tenant, CaseID: command.CaseID, SummaryNo: summaryNo,
		Kind: command.Kind, ActorID: command.Actor.ID,
		Text:      command.Text,
		Claims:    append([]casesDomain.SummaryClaim(nil), command.Claims...),
		Watermark: f.fixtureWatermark(command.CaseID), CreatedAt: command.Now,
	})
	return casesAPI.WorkOutcome{Command: "record_case_summary", SummaryKind: command.Kind, SummaryClaims: len(command.Claims)}, nil
}

// DecisionContext reads the twelve-block context view of the case over
// the REAL read-time assembly: the fixture material of the 2.2..2.13
// surfaces feeds the domain DecisionContextInput (the open handoff, the
// work items, the mode ladder, the assurance, the ownership, the facts
// with the confirmations, the observations, the reply deliveries, the
// commitments with the confirmations, the accommodations, the policy
// outcomes, the LAST summary and the CURRENT watermark counters) and
// DeriveDecisionContext derives every word read-time (the staleness
// against the current counters — never a stored status).
func (f *casesFixture) DecisionContext(_ context.Context, actor domain.Principal, tenantID, caseID id.UUID) (casesAPI.DecisionContextView, error) {
	_ = actor
	if tenantID != f.tenant {
		return casesAPI.DecisionContextView{}, casesDomain.ErrNotFound
	}
	f.lock()
	defer f.unlock()
	if f.material[caseID] == nil {
		return casesAPI.DecisionContextView{}, casesDomain.ErrNotFound
	}
	input := f.fixtureDecisionContextInput(caseID, timeNow())
	view := casesDomain.DeriveDecisionContext(input)
	return fixtureDecisionContextViewOf(view), nil
}

// fixtureDecisionContextInput gathers the material snapshot of the case
// (the mirror of the store read: ONE consistent read of every block).
func (f *casesFixture) fixtureDecisionContextInput(caseID id.UUID, now time.Time) casesDomain.DecisionContextInput {
	input := casesDomain.DecisionContextInput{Now: now}
	// The open handoff with the whole lifecycle history.
	input.HandoffEvents = f.handoffEvents[caseID]
	if open := f.fixtureHandoffOpen(caseID, now); open != nil {
		request := open.Request
		input.HandoffRequest = &request
	}
	// The work items with the current ladder views.
	for _, anchor := range f.items[caseID] {
		if current, known := f.workCurrentOf(anchor.WorkItemID); known {
			current.Terminal = casesDomain.TerminalWorkItemStates[current.State]
			input.WorkItems = append(input.WorkItems, current)
		}
	}
	input.ModeTerms = f.modeTerms[caseID]
	input.Verifications = f.verificationsOf(caseID)
	ownershipTerms := f.ownershipTerms[caseID]
	input.OwnershipTerms = ownershipTerms
	if state := f.fixtureState(caseID, now); state.Current != nil {
		input.CurrentOwnerEpoch = state.Current.Epoch
	}
	// The facts with the confirmations of THIS case.
	for _, versions := range f.factVersions {
		for _, version := range versions {
			if version.CaseID == caseID {
				input.FactVersions = append(input.FactVersions, version)
				if confirmations, known := f.factConfirms[version.FactID]; known {
					for _, confirmation := range confirmations {
						if confirmation.CaseID == caseID {
							input.FactConfirmations = append(input.FactConfirmations, confirmation)
						}
					}
				}
			}
		}
	}
	input.Observations = f.observations[caseID]
	// The replies with the delivery events.
	for _, reply := range f.replies[caseID] {
		material := casesDomain.ReplyDeliveryMaterial{ReplyID: reply.ReplyID, Dispatched: true}
		for _, event := range f.replyDeliveryEvents[caseID] {
			if event.ReplyID == reply.ReplyID {
				material.Events = append(material.Events, event)
			}
		}
		input.Replies = append(input.Replies, material)
	}
	input.CommitmentVersions = f.commitments[caseID]
	input.Confirmations = f.confirmations[caseID]
	input.Accommodations = f.accommodations[caseID]
	input.Outcomes = f.policyOutcomes[caseID]
	// The LAST summary with the CURRENT watermark counters.
	if summaries := f.summaries[caseID]; len(summaries) > 0 {
		last := summaries[len(summaries)-1]
		input.Summary = &last
	}
	input.Watermark = f.fixtureWatermark(caseID)
	return input
}

// fixtureDecisionContextViewOf maps the domain view onto the API DTO
// with the honest render texts (the app decisionContextViewOf mirror —
// the texts never enter the fixture state).
func fixtureDecisionContextViewOf(view casesDomain.DecisionContextView) casesAPI.DecisionContextView {
	out := casesAPI.DecisionContextView{
		Present: view.Handoff.Present || view.Summary.Present,
		PanelText: "Контекст передачи — производное чтение собранного материала, не новая истина: блоки читаются существующими деривациями историй 2-2..2-13, слова — словари среза. " +
			"Сводка AI — производное представление: сверяйтесь с источниками рядом. Рекомендация — проекция слов плана, не команда и не полномочие.",
		Handoff: casesAPI.ContextHandoffView{
			Present:         view.Handoff.Present,
			Reason:          view.Handoff.Reason,
			ReasonText:      casesDomain.HandoffReasonText(view.Handoff.Reason),
			State:           view.Handoff.State,
			StateText:       casesDomain.HandoffStateText(view.Handoff.State),
			TurnRef:         view.Handoff.TurnRef.String(),
			AckDeadline:     view.Handoff.AckDeadline,
			AckDeadlineText: casesDomain.HandoffAckDeadlineText(view.Handoff.AckDeadline),
			DeadlineAt:      view.Handoff.DeadlineAt,
		},
		WorkItems: []casesAPI.ContextWorkItemRow{},
		Mode: casesAPI.ContextModeView{
			Present: view.Mode.Present, Mode: view.Mode.Mode, ModeVersion: view.Mode.ModeVersion,
		},
		Assurance: casesAPI.ContextAssuranceView{
			IdentityLevel:      view.Assurance.IdentityLevel,
			IdentityMethod:     view.Assurance.IdentityMethod,
			IdentityExpiresAt:  view.Assurance.IdentityExpiresAt,
			AuthorityLevel:     view.Assurance.AuthorityLevel,
			AuthorityMethod:    view.Assurance.AuthorityMethod,
			AuthorityScope:     view.Assurance.AuthorityScope,
			AuthorityExpiresAt: view.Assurance.AuthorityExpiresAt,
		},
		Owner: casesAPI.ContextOwnerView{
			Present: view.Owner.Present, OwnerKind: view.Owner.OwnerKind,
			OwnerRef: view.Owner.OwnerRef.String(), Epoch: view.Owner.Epoch,
		},
		Facts: casesAPI.ContextFactsView{
			Confirmed:   []casesAPI.ContextFactRow{},
			Unconfirmed: []casesAPI.ContextFactRow{},
			Conflicting: []casesAPI.ContextFactRow{},
		},
		Observations: []casesAPI.ContextObservationRow{},
		Deliveries:   []casesAPI.ContextDeliveryRow{},
		Commitments:  []casesAPI.ContextCommitmentRow{},
		Preferences: casesAPI.ContextPreferencesView{
			AccommodationKeys: append([]string{}, view.Preferences.AccommodationKeys...),
			AlternativePaths:  append([]string{}, view.Preferences.AlternativePaths...),
			UsualModalities:   []casesAPI.ContextModalityRow{},
		},
		Recommendation: casesAPI.ContextRecommendationView{
			Present:         view.Recommendation.Present,
			Steps:           append([]string{}, view.Recommendation.Steps...),
			Reason:          view.Recommendation.Reason,
			Checks:          []casesAPI.ContextCheckRow{},
			AckDeadline:     view.Recommendation.AckDeadline,
			AckDeadlineText: casesDomain.HandoffAckDeadlineText(view.Recommendation.AckDeadline),
			DeadlineAt:      view.Recommendation.DeadlineAt,
			Mode:            view.Recommendation.Mode,
			ModeVersion:     view.Recommendation.ModeVersion,
			ProjectionText:  casesDomain.RecommendationProjectionText,
		},
		Summary: casesAPI.SummaryView{
			Present: view.Summary.Present, SummaryNo: view.Summary.SummaryNo, Kind: view.Summary.Kind,
			MarkingText: casesDomain.AIMarkingText, Text: view.Summary.Text,
			Claims:                []casesAPI.ClaimView{},
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
