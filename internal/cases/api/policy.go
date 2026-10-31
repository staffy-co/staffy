package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.6: the two policy commands and the PolicyView projection
// ---------------------------------------------------------------------------

// SubmitAIProposalCommand records one typed AI proposal (the proposer
// port of the slice — the synthetic port or the operator; the future AI
// Runtime passes through the same command): the Work Item scope, the
// accepted turn it answers, the intent claim (a bounded string — the
// dictionary is a POLICY matter, never a submission matter), the bounded
// candidate answer (possibly empty), the missing-fact claims with their
// reason words and the extracted-fact claims with their source spans
// (the AUDIT claims — the facts are never written from them).
type SubmitAIProposalCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	WorkItemID      id.UUID
	TurnRef         id.UUID
	Intent          string
	CandidateAnswer string
	MissingFacts    []casesDomain.MissingFact
	ExtractedFacts  []casesDomain.ExtractedFact
	Proposer        string
	IdempotencyKey  string
	Now             time.Time
}

// EvaluatePolicyCommand runs the deterministic policy over the anchored
// input versions: the proposal reference and the four anchors (the mode
// version, the owner epoch, the confirmed facts digest of the case and
// the Work Item version). The anchors close the TOCTOU of the fact reads
// — a moved anchor is the honest stale conflict with the current words.
type EvaluatePolicyCommand struct {
	Actor                   domain.Principal
	TenantID                id.UUID
	CaseID                  id.UUID
	ProposalID              id.UUID
	ExpectedModeVersion     int64
	ExpectedOwnerEpoch      int64
	ExpectedFactsDigest     string
	ExpectedWorkItemVersion int64
	IdempotencyKey          string
	Now                     time.Time
}

// PolicyWorkItemView is one Work Item section of the policy view: the
// proposals of the item (the claims — the intent, the proposer word, the
// candidate answer the operator sees), the CURRENT outcome (the maximal
// number; nil when no outcome exists) and the full outcome history.
type PolicyWorkItemView struct {
	WorkItemID id.UUID                     `json:"work_item_id"`
	Proposals  []casesDomain.AIProposal    `json:"proposals"`
	Outcomes   []casesDomain.PolicyOutcome `json:"outcomes"`
	Current    *casesDomain.PolicyOutcome  `json:"current"`
}

// PolicyView is the policy section of the case view: the proposals and
// the outcomes per Work Item, the dictionary hint words of the panel and
// the fixed rule words (the intent dictionary, the commercial subset and
// the class dictionary — the operator panel renders them textually).
type PolicyView struct {
	Items       []PolicyWorkItemView `json:"items"`
	IntentWords []string             `json:"intent_words"`
	Commercial  []string             `json:"commercial_intents"`
	ClassWords  []string             `json:"class_words"`
	RuleVersion string               `json:"rule_version"`
	PanelText   string               `json:"panel_text"`
	// The current anchor words of the evaluation form (the server truth
	// — the panel renders them read-only and the form prefills them):
	// the confirmed facts digest, the owner epoch and the mode version.
	FactsDigest string `json:"facts_digest"`
	OwnerEpoch  int64  `json:"owner_epoch"`
	ModeVersion int64  `json:"mode_version"`
}

// PolicyService is the command surface of Story 2.6 (both commands are
// idempotent by the tenant-scoped key; the evaluation carries the four
// anchors of the input versions — the request hash includes them, the
// m-r2-1 lesson; the outcomes are append-only with the monotonic number;
// the sales reply gate is the load-bearing extension of send_client_reply).
type PolicyService interface {
	SubmitAIProposal(context.Context, SubmitAIProposalCommand) (WorkOutcome, error)
	EvaluatePolicy(context.Context, EvaluatePolicyCommand) (WorkOutcome, error)
}
