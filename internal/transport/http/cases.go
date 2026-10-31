package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// BindCases mounts the case contour surface: the machine ingress contract
// (no user principal — the scope is derived from the verified connection),
// the operator Cases/Timeline surfaces, the sealed payload read under
// review/audit, and the Story 2.2 work command surface (the machine JSON
// contract and the operator forms of the Work Control Card).
func (s *Server) BindCases(service casesAPI.Service) {
	s.cases = service
	s.mux.HandleFunc("POST /v1/ingress/channel-events", s.ingressChannelEventAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases", s.listCasesAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}", s.readCaseAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/events/{event_id}/payload", s.readCasePayloadAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases", s.casesPage)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases/{case_id}", s.caseDetailPage)
	// The work commands (Story 2.2) — the machine JSON contract:
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items", s.createWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/classify", s.classifyWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/transition", s.transitionWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/split", s.splitWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/merge", s.mergeWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/events", s.linkEventAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/dispute", s.disputeWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts", s.recordFactAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/confirmations", s.confirmFactAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/rescope", s.rescopeFactAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/facts", s.factsViewAPI)

	// The Story 2.3 surfaces: the verification commands (machine JSON +
	// operator forms, PRG).
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/verifications", s.startVerificationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/bindings", s.bindChannelAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/bindings/{binding_id}/unlink", s.unlinkChannelAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/duplicates/{candidate_id}/resolution", s.resolveDuplicateAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/verifications", s.startVerificationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/bindings", s.bindChannelForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/bindings/{binding_id}/unlink", s.unlinkChannelForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/duplicates/{candidate_id}/resolution", s.resolveDuplicateForm)
	// The Story 2.4 surfaces: the five ownership commands (machine JSON +
	// operator forms, PRG) and the reply drain sweep.
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/ownership/take", s.takeOwnershipAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/ownership/transfer-requests", s.requestTransferAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/ownership/transfer-requests/{request_id}/resolution", s.resolveTransferAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/ownership/reassign", s.reassignOwnershipAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/client-replies", s.sendClientReplyAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/client-replies/drain", s.drainClientRepliesAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/ownership/take", s.takeOwnershipForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/ownership/transfer-requests", s.requestTransferForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/ownership/transfer-requests/{request_id}/resolution", s.resolveTransferForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/ownership/reassign", s.reassignOwnershipForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/client-replies", s.sendClientReplyForm)
	// The Story 2.5 surfaces: the four mode commands (machine JSON +
	// operator forms, PRG) — the conversation mode ladder, the
	// supervised exit, the AI disclosure records and the delivery
	// events.
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/mode/proposals", s.proposeModeAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/mode/exit", s.exitModeAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/disclosures", s.recordDisclosureAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/disclosures/{disclosure_id}/delivery-events", s.recordDisclosureEventAPI)
	// The Story 2.6 surfaces: the two policy commands (machine JSON +
	// operator forms, PRG) — the typed AI proposal of the proposer port
	// and the deterministic evaluation with the four input anchors.
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/ai-proposals", s.submitAIProposalAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/policy/evaluations", s.evaluatePolicyAPI)
	// The Story 2.7 surfaces: the owning-system read command (machine JSON
	// + the operator PRG form of the «Материальные поля Owning System»
	// panel) — one material field of the authority matrix per read.
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/owning-system-reads", s.readOwningSystemAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/owning-system-reads", s.readOwningSystemForm)
	// The Story 2.8 surfaces: the objection record command (machine JSON
	// + the operator PRG form of the «Возражения и бюджет убеждения»
	// panel) — one client objection per record with the verbatim spans.
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/objections", s.recordObjectionAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/objections", s.recordObjectionForm)
	// The Story 2.9 surfaces: the three commitment commands (machine JSON
	// + the operator PRG forms of the Commitment Review page) and the
	// Commitment Review page itself (the operator surface of one Work
	// Item: the version history, the confirmations, the declines).
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments", s.createCommitmentAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments/{commitment_id}/confirm", s.confirmCommitmentAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments/{commitment_id}/decline", s.declineCommitmentAPI)
	// The Story 2.10 surfaces: the two continuation commands (machine JSON
	// + the operator forms — the issuance re-render with the one-time URL,
	// the revoke PRG) and the public protected page (the opaque token IS
	// the credential — no session, no principal). The narrow interface is
	// type-asserted of the cases service (the BindEmployee precedent: the
	// composite Service stays untouched); the tenant-name reader of the
	// public header asserts of the identity service the same way.
	if accommodations, ok := service.(casesAPI.AccommodationService); ok {
		// The Story 2.11 surfaces: the accommodation record command (machine
		// JSON + the operator PRG form of the «Предпочтения связи» panel — the
		// narrow interface is type-asserted of the cases service (the
		// ContinuationService precedent: the composite Service and the test
		// fakes stay untouched).
		s.accommodations = accommodations
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/accommodations", s.recordAccommodationAPI)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/accommodations", s.recordAccommodationForm)
	}
	if degradations, ok := service.(casesAPI.DegradationService); ok {
		// The Story 2.12 surfaces: the route degradation record command and
		// the delivery evidence event command (machine JSON + the operator
		// PRG forms — the narrow interface is type-asserted of the cases
		// service, the AccommodationService/ContinuationService precedent:
		// the composite Service and the test fakes stay untouched).
		s.degradations = degradations
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/route-degradations", s.recordRouteDegradationAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/replies/{reply_id}/delivery-evidence", s.recordDeliveryEvidenceAPI)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/route-degradations", s.recordRouteDegradationForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/delivery-evidence", s.recordDeliveryEvidenceForm)
	}
	if handoffs, ok := service.(casesAPI.HandoffService); ok {
		// The Story 2.13 surfaces: the six handoff lifecycle commands (the
		// unconditional human request, the assignment round, the assignee's
		// acknowledgement and decline, the client's cancellation and the
		// return to Staffy — machine JSON + the operator PRG forms, the
		// narrow interface is type-asserted of the cases service, the
		// DegradationService precedent: the composite Service and the test
		// fakes stay untouched).
		s.handoffs = handoffs
		if fitments, ok := service.(casesAPI.FitmentService); ok {
			// The Story 4.4 surfaces: the manager's fitment evidence
			// record command (the append-only proof of the catalog
			// research — machine JSON + the operator PRG form; the
			// narrow interface is type-asserted of the cases service,
			// the HandoffService precedent).
			s.fitments = fitments
			s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/fitment-evidences", s.recordFitmentEvidenceAPI)
			s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/fitment-evidences", s.recordFitmentEvidenceForm)
			if offers, ok := service.(casesAPI.OffersService); ok {
				// The Story 4.5 surfaces: the manager's supplier offer and
				// the client selection record commands — the machine JSON
				// routes and the operator PRG forms of the case page (the
				// same narrow interface type-assertion precedent).
				s.offers = offers
				s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/supplier-offers", s.recordSupplierOfferAPI)
				s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/supplier-offers", s.recordSupplierOfferForm)
				s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/selection-snapshots", s.recordClientSelectionAPI)
				s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/selection-snapshots", s.recordClientSelectionForm)
			}
		}
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/human-requests", s.requestHumanAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/human-requests/assignment", s.assignHumanHandoffAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/human-requests/acknowledgement", s.acknowledgeHumanHandoffAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/human-requests/decline", s.declineHumanHandoffAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/human-requests/cancellation", s.cancelHumanRequestAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/human-requests/return", s.returnToStaffyAPI)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/human-requests", s.requestHumanForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/human-requests/assignment", s.assignHumanHandoffForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/human-requests/acknowledgement", s.acknowledgeHumanHandoffForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/human-requests/decline", s.declineHumanHandoffForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/human-requests/cancellation", s.cancelHumanRequestForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/human-requests/return", s.returnToStaffyForm)
	}
	if decisionContext, ok := service.(casesAPI.DecisionContextService); ok {
		// The Story 2.14 surfaces: the decision context read (the machine
		// GET of the twelve-block view — the operator page renders the
		// same projection in the «Контекст передачи» section) and the AI
		// summary record of the producer path (the machine JSON POST —
		// the PRG forms do NOT exist: the operator never writes the
		// summary, the kind dictionary carries the single producer word).
		s.decisionContext = decisionContext
		s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/decision-context", s.decisionContextAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/summaries", s.recordCaseSummaryAPI)
	}
	if corrections, ok := service.(casesAPI.CorrectionService); ok {
		// The Story 2.15 surfaces: the operator correction record (the
		// machine JSON POST of the fact-scoped command — the outcome is
		// NEVER a request field, the strict decode refuses it; the
		// correction number is the server truth under the locks) and
		// the PRG form «Исправить» of the fact row (POST → 303 → GET
		// #decision-context — the section position survives the anchor,
		// the exact scroll position does not; the no-JS boundary of the
		// slice). The narrow interface is type-asserted of the cases
		// service (the DecisionContextService precedent: the composite
		// Service and the test fakes stay untouched).
		s.corrections = corrections
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/corrections", s.recordCorrectionAPI)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/corrections", s.recordCorrectionForm)
	}
	if waitingUpdates, ok := service.(casesAPI.UpdatePromiseService); ok {
		// The Story 2.16 surfaces: the operator update promise of the
		// handoff round (the machine JSON POST of the case-scoped
		// command — the deadline word is the ONLY request material, the
		// promised time and the promise number are NEVER request fields,
		// the strict decode refuses them; the round anchor is resolved
		// by the store under the locks) and the PRG form «Обещать
		// обновление» of the handoff section (POST → 303 → GET
		// ?update_promised=1#handoff — the section position survives the
		// anchor, the exact scroll position does not; the no-JS boundary
		// of the slice). The narrow interface is type-asserted of the
		// cases service (the CorrectionService precedent: the composite
		// Service and the test fakes stay untouched).
		s.waitingUpdates = waitingUpdates
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/update-promises", s.promiseUpdateAPI)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/update-promises", s.promiseUpdateForm)
	}
	if continuations, ok := service.(casesAPI.ContinuationService); ok {
		s.continuations = continuations
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/continuation-links", s.issueContinuationLinkAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/continuation-links/{link_id}/revoke", s.revokeContinuationLinkAPI)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/continuation-links", s.issueContinuationLinkForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/continuation-links/{link_id}/revoke", s.revokeContinuationLinkForm)
		s.mux.HandleFunc("GET /continue/{token}", s.continuationPage)
		s.mux.HandleFunc("POST /continue/{token}/correct", s.continuationCorrectForm)
		s.mux.HandleFunc("POST /continue/{token}/confirm-vehicle", s.continuationVehicleConfirmForm)
		s.mux.HandleFunc("POST /continue/{token}/human", s.continuationHumanForm)
		s.mux.HandleFunc("POST /continue/{token}/return", s.continuationReturnForm)
		if namer, ok := s.identity.(identityAPI.TenantNameService); ok {
			s.tenantNamer = namer
		}
	}
	if external, ok := service.(casesAPI.ExternalCommitmentService); ok {
		// The Story 3.3 surfaces (the first slice): the machine JSON of
		// the envelope lifecycle (the create, the exact-version approval
		// with step-up, the persist-before-emit enqueue, the operator
		// receipt relay and the bounded drain) and the operator
		// Commitment Review page with the PRG forms — the narrow
		// interface is type-asserted of the cases service (the
		// UpdatePromiseService precedent: the composite Service and
		// the test fakes stay untouched). The mutation paths are
		// Work-Item-anchored (the 2.9 commitment family); the view and
		// the page are case-wide (the sections read every Work Item);
		// the drain is the tenant-level operator command (the
		// client-replies drain precedent).
		s.externalCommitments = external
		s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/external-commitments", s.readExternalCommitmentsAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments", s.createExternalCommitmentAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/approve", s.approveExternalCommitmentAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/enqueue", s.enqueueExternalCommitmentAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/dispatch-receipts", s.relayExternalReceiptAPI)
		s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/external-commitments/drain", s.drainExternalCommitmentsAPI)
		s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases/{case_id}/external-commitments", s.externalCommitmentsPage)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/external-commitments", s.createExternalCommitmentForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/approve", s.approveExternalCommitmentForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/enqueue", s.enqueueExternalCommitmentForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/dispatch-receipts", s.relayExternalReceiptForm)
		s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/external-commitments/drain", s.drainExternalCommitmentsForm)
	}
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments", s.createCommitmentForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments/{commitment_id}/confirm", s.confirmCommitmentForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments/{commitment_id}/decline", s.declineCommitmentForm)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments", s.commitmentReviewPage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/ai-proposals", s.submitAIProposalForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/policy/evaluations", s.evaluatePolicyForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/mode/proposals", s.proposeModeForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/mode/exit", s.exitModeForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/disclosures", s.recordDisclosureForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/disclosures/{disclosure_id}/delivery-events", s.recordDisclosureEventForm)
	// The operator forms of the Work Control Card (the PRG pattern):
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items", s.createWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/classify", s.classifyWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/transition", s.transitionWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/split", s.splitWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/merge", s.mergeWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/events", s.linkEventForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/dispute", s.disputeWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts", s.recordFactForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/confirmations", s.confirmFactForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/rescope", s.rescopeFactForm)
}

// ingressEnvelopeInput is the versioned canonical machine contract
// (schema v1): strict decoding, dictionary-bounded values, the raw
// provider payload as one JSON value. The provider DTO never becomes the
// domain model — this contract is the only translator.
type ingressEnvelopeInput struct {
	SchemaVersion     *int             `json:"schema_version"`
	TenantID          *string          `json:"tenant_id"`
	ConnectionID      *string          `json:"connection_id"`
	Provider          *string          `json:"provider"`
	ProviderEventID   *string          `json:"provider_event_id"`
	ProviderSessionID *string          `json:"provider_session_id"`
	Direction         *string          `json:"direction"`
	Modality          *string          `json:"modality"`
	Channel           *string          `json:"channel"`
	ProviderTimestamp *string          `json:"provider_timestamp"`
	Payload           *json.RawMessage `json:"payload"`
}

// ingressBodyLimit is the hard transport DoS cap: four times the fixed
// envelope dictionary limit (1 MiB over a 256 KiB bound). The admission
// ladder bounds the honest sizes with counters (envelope_exceeded); the
// transport only refuses what it cannot safely read.
const ingressBodyLimit = 4 * 256 * 1024 // 4 × MaxEnvelopeBytes of the fixed domain dictionary

func (s *Server) ingressChannelEventAPI(w http.ResponseWriter, r *http.Request) {
	if s.cases == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		s.problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Используйте Content-Type application/json.", false, nil)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 128 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Заголовок Idempotency-Key обязателен (до 128 символов).", false, nil)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, ingressBodyLimit))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Тело запроса не читается (превышен предел контура).", false, nil)
		return
	}
	decoder := json.NewDecoder(bytesReader(body))
	decoder.DisallowUnknownFields()
	var input ingressEnvelopeInput
	if err := decoder.Decode(&input); err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Проверьте поля envelope: строгий разбор (неизвестные поля запрещены).", false, nil)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Тело запроса должно содержать один JSON-объект.", false, nil)
		return
	}
	command, ok := s.ingressCommand(w, r, input, key, len(body))
	if !ok {
		return
	}
	outcome, err := s.cases.IngestEvent(r.Context(), command)
	if err != nil {
		correlationID, _ := correlationFromRequest(r)
		s.logger.ErrorContext(r.Context(), "ingress failed", "code", "ingress_failed", "correlation_id", correlationID.String(), "error", err.Error())
		casesProblem(w, r, s, err)
		return
	}
	if outcome.Accepted != nil {
		accepted := outcome.Accepted
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/v1/tenants/"+command.Envelope.TenantClaim.String()+"/cases/"+accepted.CaseID.String())
		if accepted.Replay {
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(accepted)
		return
	}
	if outcome.Quarantined != nil {
		quarantined := outcome.Quarantined
		s.problem(w, r, http.StatusConflict, quarantined.Kind,
			"Доставка карантинится и видна оператору с correlation ID; принятый материал не переписывается.",
			false, map[string]string{"exception_id": quarantined.ExceptionID.String(), "event_id": quarantined.EventID.String()})
		return
	}
	if outcome.Refused != nil {
		refused := outcome.Refused
		status := refusalStatus(refused.Kind)
		fields := map[string]string(nil)
		if refused.Counter != 0 {
			fields = map[string]string{"counter": jsonInt(refused.Counter)}
		}
		s.problem(w, r, status, refused.Kind, refused.Message, refused.Bounded, fields)
		return
	}
	s.problem(w, r, http.StatusInternalServerError, "internal_error", "Ingress вернул пустой исход — это ошибка контура.", true, nil)
}

func refusalStatus(kind string) int {
	switch kind {
	case casesDomain.KindMalformed, casesDomain.KindChannelMismatch:
		return http.StatusBadRequest
	case casesDomain.KindUnknownConnection:
		return http.StatusNotFound
	case casesDomain.KindEntitlementBlocked, casesDomain.KindEnvelopeExceeded:
		return http.StatusTooManyRequests
	default:
		return http.StatusForbidden
	}
}

// ingressCommand assembles the app command from the decoded input: the
// canonical payload bytes and the measured envelope size.
func (s *Server) ingressCommand(w http.ResponseWriter, r *http.Request, input ingressEnvelopeInput, key string, envelopeBytes int) (casesAPI.IngestCommand, bool) {
	tenancyClaim, err := id.Parse(stringOrEmpty(input.TenantID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле tenant_id должно быть UUID.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	connectionID, err := id.Parse(stringOrEmpty(input.ConnectionID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле connection_id должно быть UUID.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	providerTimestamp, err := time.Parse(time.RFC3339, stringOrEmpty(input.ProviderTimestamp))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле provider_timestamp должно быть RFC3339.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	if input.Payload == nil || len(*input.Payload) == 0 {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле payload обязательно.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	payload, err := casesDomain.CanonicalPayload(*input.Payload)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле payload должно быть одним JSON-значением.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	correlationID, _ := correlationFromRequest(r)
	return casesAPI.IngestCommand{
		Envelope: casesDomain.Envelope{
			SchemaVersion:     intOrZero(input.SchemaVersion),
			TenantClaim:       tenancyClaim,
			ConnectionID:      connectionID,
			Provider:          stringOrEmpty(input.Provider),
			ProviderEventID:   stringOrEmpty(input.ProviderEventID),
			ProviderSessionID: stringOrEmpty(input.ProviderSessionID),
			Direction:         stringOrEmpty(input.Direction),
			Modality:          stringOrEmpty(input.Modality),
			Channel:           stringOrEmpty(input.Channel),
			ProviderTimestamp: providerTimestamp,
			Payload:           payload,
			PayloadDigest:     casesDomain.PayloadDigest(payload),
			EnvelopeBytes:     envelopeBytes,
		},
		IdempotencyKey: key,
		CorrelationID:  correlationID,
		Now:            s.clock.Now().UTC(),
	}, true
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func jsonInt(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// casesProblem maps the service errors of the case surface onto the
// problem contract: a foreign tenant is a plain 404 (the tenant existence
// is never disclosed), a member without the scope a 403. Internal
// failures are logged with the correlation id — the honest text never
// leaks internals, the log carries them for the operator.
func casesProblem(w http.ResponseWriter, r *http.Request, s *Server, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, casesDomain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Пространство или обращение не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		s.problem(w, r, http.StatusForbidden, "cases_forbidden", "Недостаточно полномочий для контура обращений.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		s.logger.ErrorContext(r.Context(), "cases surface failed", "code", "cases_surface_failed", "correlation_id", correlationID.String(), "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить запрос. Повторите позднее.", true, nil)
	}
}

func (s *Server) listCasesAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CasesView(r.Context(), actor, tenant)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) readCaseAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CaseView(r.Context(), actor, tenant, caseID)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) readCasePayloadAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	eventID, err := id.Parse(r.PathValue("event_id"))
	if err != nil {
		s.problem(w, r, http.StatusNotFound, "not_found", "Событие не найдено.", false, nil)
		return
	}
	view, err := s.cases.ReadPayload(r.Context(), actor, tenant, caseID, eventID)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		EventID       id.UUID         `json:"event_id"`
		PayloadDigest string          `json:"payload_digest"`
		SealOK        bool            `json:"seal_ok"`
		Payload       json.RawMessage `json:"payload"`
	}{EventID: view.EventID, PayloadDigest: view.PayloadDigest, SealOK: view.SealOK, Payload: json.RawMessage(view.Payload)})
}

func (s *Server) casesPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CasesView(r.Context(), actor, tenant)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	s.render(w, "cases", map[string]any{
		"TenantID": tenant,
		"View":     prepareCasesView(view),
	})
}

func (s *Server) caseDetailPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CaseView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The facts of the Work Control Card follow the review/audit boundary:
	// a configuration-scoped member sees the metadata surface without the
	// values (the honest note instead of a failed page).
	facts, err := s.cases.FactsView(r.Context(), actor, tenant, caseID)
	if err != nil && !errors.Is(err, domain.ErrForbidden) {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 2.14 decision context of the «Контекст передачи» section:
	// the read gate is review or configuration — an audit-only member
	// sees the page without the section (the honest degradation, never a
	// failed page); the section renders only when the material exists.
	contextPanel, contextErr := s.decisionContextOf(r.Context(), actor, tenant, caseID)
	if contextErr != nil {
		s.casesPageProblem(w, r, contextErr)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	page := prepareCaseView(r, view, facts, flash)
	page.DecisionContext = contextPanel
	// The Story 4.6 order panel: the honest degradation of the
	// decision-context precedent (an unwired action surface or a
	// scope-forbidden member renders the zero panel — the page keeps the
	// rest; every other failure fails the page honestly).
	page.Order, err = s.orderPanelOf(r.Context(), actor, tenant, caseID, view)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 4.7 verification panel: the same honest degradation (an
	// unwired surface or a scope-forbidden member renders the zero panel —
	// the page keeps the rest; every other failure fails the page).
	page.OrderVerification, err = s.orderVerificationPanelOf(r.Context(), actor, tenant, caseID, view)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 4.8 correction panel: the same honest degradation (an
	// unwired surface or a scope-forbidden member renders the zero panel —
	// the page keeps the rest; every other failure fails the page honestly).
	page.OrderCorrection, err = s.orderCorrectionPanelOf(r.Context(), actor, tenant, caseID, view)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 4.9 status panel: the same honest degradation (an
	// unwired surface or a scope-forbidden member renders the zero panel —
	// the page keeps the rest; every other failure fails the page
	// honestly). The projection never fails on a case without an order.
	page.OrderStatus, err = s.orderStatusPanelOf(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 4.10 servicing panel: the same honest degradation (an
	// unwired surface or a scope-forbidden member renders the zero panel —
	// the page keeps the rest; every other failure fails the page
	// honestly). The panel never fails on a case without recorded
	// requests — the feed speaks the honest absence.
	page.OrderServicing, err = s.orderServicingPanelOf(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 4.11 outcome panel: the same honest degradation (an
	// unwired surface, the ports-missing word of the app gate or a
	// scope-forbidden member renders the zero panel — the page keeps
	// the rest; every other failure fails the page honestly). The
	// derivation never fails on a case without order material — the
	// funnel speaks the honest not_reached ladder.
	page.PilotOutcome, err = s.pilotOutcomePanelOf(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	s.render(w, "case", map[string]any{
		"TenantID": tenant,
		"View":     page,
	})
}

// decisionContextOf reads the decision context panel of the case page
// (the honest degradation of the unreadable or unauthorized read: the
// absent service renders the zero view — no section; the forbidden read
// of the audit-only member renders the zero view — the page keeps the
// rest of the surface; every other failure fails the page honestly).
func (s *Server) decisionContextOf(ctx context.Context, actor domain.Principal, tenant, caseID id.UUID) (decisionContextPanelView, error) {
	if s.decisionContext == nil {
		return decisionContextPanelView{}, nil
	}
	view, err := s.decisionContext.DecisionContext(ctx, actor, tenant, caseID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return decisionContextPanelView{}, nil
		}
		return decisionContextPanelView{}, err
	}
	return decisionContextPanelOf(view), nil
}

func (s *Server) casesPageProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, casesDomain.ErrNotFound):
		s.notFound(w, r, domain.Principal{})
	case errors.Is(err, domain.ErrForbidden):
		s.renderStatus(w, http.StatusForbidden, "cases", map[string]any{
			"View": casesPageView{ScopeText: casesScopeText(false), ForbiddenText: "Просмотр обращений требует полномочия review, configuration или audit."},
		})
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось открыть обращения. Повторите позднее.", true, nil)
	}
}

func (s *Server) casesActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, bool) {
	principal, ok := s.principal(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), false
	}
	tenantID, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.notFound(w, r, principal)
		return domain.Principal{}, id.UUID(""), false
	}
	return principal, tenantID, true
}

func (s *Server) casesActorAndCase(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, bool) {
	actor, tenantID, ok := s.casesActor(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), id.UUID(""), false
	}
	caseID, err := id.Parse(r.PathValue("case_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return domain.Principal{}, id.UUID(""), id.UUID(""), false
	}
	return actor, tenantID, caseID, true
}

// writeJSON renders one JSON answer of the case surface.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
