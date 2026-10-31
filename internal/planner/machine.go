package planner

// The planner machine (Story 8-1): the protocol machine of one turn —
// the AD-7 discipline as executable code. The flow is closed:
//
//      strict decode of the context (BEFORE any model round — a refused
//      context is not a turn: no round, no record)
//        → round 1 over the model port
//          → adapter error      = failed turn + approved fallback (no blind
//                                 retry, no repair round)
//          → strict decode + validation of the proposal
//            → pass             = validated turn
//            → named refusal    → EXACTLY ONE repair round (the model
//                                 receives the named cause and the same
//                                 context)
//                                  → adapter error  = failed + fallback
//                                  → pass           = repaired turn
//                                  → second refusal = failed turn, persisted
//                                    honestly, ended by the approved safe
//                                    fallback — NEVER the model output
//
// The model rounds are structurally ≤ 2: the machine has no third
// round to call. Every STARTED turn is journaled exactly once
// (validated | repaired | failed); the Build constant travels with
// every record.

import (
	"context"
	"errors"

	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Machine is the turn machine over the model port, the journal port
// and the clock.
type Machine struct {
	model  ModelPort
	ledger TurnRecorder
	clock  Clock
}

// NewMachine wires the machine. None of the ports may be nil — a
// machine without a journal cannot exist (an unjournaled turn would be
// a silent model round, the exact opposite of the AD-7 honesty).
func NewMachine(model ModelPort, ledger TurnRecorder, clock Clock) *Machine {
	return &Machine{model: model, ledger: ledger, clock: clock}
}

// RunResult is the honest outcome of one Run: either the context was
// refused BEFORE the turn (Started=false — no round, no record), or a
// turn ran to exactly one of the three outcome words. The validated
// proposal material is present only for validated/repaired turns — a
// failed turn NEVER carries model material out.
type RunResult struct {
	Started bool
	// Outcome is validated | repaired | failed (empty when !Started).
	Outcome string
	// Rounds is the number of model rounds executed (0 when !Started;
	// at most 2 — the machine has no third round).
	Rounds int
	// Refusal is the named cause: the context refusal when !Started;
	// the final validation refusal or the adapter cause of a failed
	// turn; the repaired cause of a repaired turn; nil for validated.
	Refusal *RuleError
	// Proposal is the validated proposal material (nil on failed turns
	// and on context refusals — the model output of a failed turn
	// never leaves the machine).
	Proposal *TurnProposal
	// Fallback is the approved safe form word that ended a failed turn
	// (empty otherwise).
	Fallback string
	// Record is the journal record of the turn (zero when !Started).
	Record TurnRecord
}

// Run executes one planner turn. The scope carries the tenant of the
// journal write (the RLS identity); the raw context must declare the
// SAME tenant — a mismatch is a named refusal before any round (the
// AD-11 guard; the RLS check of the insert is the belt-and-suspenders
// behind it).
func (m *Machine) Run(ctx context.Context, scope tenancy.Context, rawContext []byte) (RunResult, error) {
	context, refusal := DecodeTurnContext(rawContext)
	if refusal != nil {
		return RunResult{Started: false, Refusal: refusal}, nil
	}
	if !scope.Valid() {
		return RunResult{}, tenancy.ErrMissingContext
	}
	if scope.TenantID() != context.TenantID {
		return RunResult{Started: false, Refusal: rule("context.tenant_scope", "тенент контекста не совпадает с тенентом вызова — материал чужого тенента не входит в ход (AD-11)")}, nil
	}

	// Round 1.
	result, err := m.model.Round(ctx, RoundRequest{Context: context})
	if err != nil {
		return m.finish(ctx, scope, context, 1, &RuleError{Rule: "planner.adapter_round", Detail: "ошибка модельного адаптера первого раунда: " + err.Error()}, nil, nil)
	}
	proposal, firstRefusal := m.decodeAndValidate(result.Raw, context)
	if firstRefusal == nil {
		return m.finish(ctx, scope, context, 1, nil, nil, proposal)
	}

	// The repair round: exactly one, with the named cause and the same
	// context.
	result, err = m.model.Round(ctx, RoundRequest{Context: context, Repair: firstRefusal.Error()})
	if err != nil {
		return m.finish(ctx, scope, context, 2, &RuleError{Rule: "planner.adapter_round", Detail: "ошибка модельного адаптера repair-раунда: " + err.Error()}, nil, nil)
	}
	repaired, secondRefusal := m.decodeAndValidate(result.Raw, context)
	if secondRefusal == nil {
		return m.finish(ctx, scope, context, 2, nil, firstRefusal, repaired)
	}
	return m.finish(ctx, scope, context, 2, secondRefusal, nil, nil)
}

// decodeAndValidate runs the strict decode and the cross-field
// validation of one model round's raw material. Either the material is
// a validated proposal or there is a named refusal — there is no third
// outcome and no silent repair.
func (m *Machine) decodeAndValidate(raw []byte, context TurnContext) (*TurnProposal, *RuleError) {
	proposal, refusal := DecodeTurnProposal(raw)
	if refusal != nil {
		return nil, refusal
	}
	if refusal := ValidateTurnProposal(proposal, context); refusal != nil {
		return nil, refusal
	}
	return &proposal, nil
}

// finish journals the turn exactly once and shapes the honest result.
// The validatedProposal is present only for validated/repaired turns;
// a failed turn journals the named cause and ends with the approved
// fallback of the context's declared policy — never the model output.
// The repairedCause of a repaired turn is honest journal material: the
// reason the first round was refused.
func (m *Machine) finish(ctx context.Context, scope tenancy.Context, context TurnContext, rounds int, cause *RuleError, repairedCause *RuleError, validatedProposal *TurnProposal) (RunResult, error) {
	outcome := OutcomeValidated
	switch {
	case cause != nil:
		outcome = OutcomeFailed
	case rounds == 2:
		outcome = OutcomeRepaired
	}
	var fallback FallbackForm
	refusal := cause
	if outcome == OutcomeFailed {
		// The failed turn ends ONLY with the approved safe fallback of
		// the context policy — the model material is discarded here and
		// never leaves the machine.
		var err error
		fallback, err = ApprovedFallback(context.FallbackPolicy)
		if err != nil {
			return RunResult{}, err
		}
	} else if outcome == OutcomeRepaired {
		refusal = repairedCause
	}
	var proposalJSON []byte
	if validatedProposal != nil {
		material, err := compactJSON(*validatedProposal)
		if err != nil {
			return RunResult{}, err
		}
		proposalJSON = material
	}
	reason := ""
	if refusal != nil {
		reason = refusal.Error()
	}
	record := TurnRecord{
		TenantID:       scope.TenantID(),
		TurnID:         context.TurnID,
		CaseID:         context.CaseID,
		ConversationID: context.ConversationID,
		WorkItemID:     context.WorkItemID,
		EmployeeID:     context.EmployeeID,
		Outcome:        outcome,
		RefusalReason:  reason,
		FallbackForm:   fallback.Kind,
		SchemaVersion:  ProposalSchemaVersion,
		Build:          Build,
		Proposal:       proposalJSON,
		CreatedAt:      m.clock.Now(),
	}
	if err := m.ledger.Record(ctx, scope, record); err != nil {
		// The honest conflict of the uniqueness surfaces to the caller:
		// the second recording of the same turn does not happen. Other
		// ledger errors surface too — an unjournaled turn is never
		// reported as a success.
		return RunResult{Started: true, Outcome: outcome, Rounds: rounds, Refusal: refusal, Proposal: validatedProposal, Fallback: fallback.Kind, Record: record}, err
	}
	return RunResult{Started: true, Outcome: outcome, Rounds: rounds, Refusal: refusal, Proposal: validatedProposal, Fallback: fallback.Kind, Record: record}, nil
}

// IsTurnConflict reports whether err is the honest journal conflict of
// the turn uniqueness.
func IsTurnConflict(err error) bool { return errors.Is(err, ErrTurnConflict) }
