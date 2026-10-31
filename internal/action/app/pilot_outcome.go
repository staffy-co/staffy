package app

import (
        "context"
        "errors"
        "fmt"
        "time"

        "github.com/staffy-co/staffy/internal/action/api"
        "github.com/staffy-co/staffy/internal/action/domain"
        casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
        identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
        "github.com/staffy-co/staffy/internal/platform/id"
        "github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.11 — the pilot outcome application contour: the READ-ONLY
// derivation of the ten-stage inquiry funnel of one case (the case
// outcome panel) and the aggregate pilot report of one period (the
// pilot report page). The contour writes NOTHING (0 writes, 0
// migrations — the computed representation never becomes a new source
// of truth, AC-13): the panel/report material assembles through the NEW
// narrow port PilotCaseSource (the cases module — the composition-root
// adapter by the CaseMaterialSource precedent) plus the EXISTING action
// stores (the operations, the receipts, the verifications, the
// corrections, the facts, the recovery cases — the OrderStatusView read
// pattern of 4.9). The honest degradation: a NIL pilot port (or any nil
// store of the assembly) is the spoken refusal word — the transport
// renders the zero panel/page and the rest of the surface stays alive
// (the 4.9/4.10 precedent, never a nil panic). The report re-reads the
// case listing per period case by case (the pilot volume is small — the
// AG-1 envelope; batch reads are a future optimization, spoken
// honestly); a repeated read of the same world answers the same.

// pilotOutcomeScopeText is the honest scope refusal of the contour: the
// words BEFORE any outcome material (the decision-context 2-14
// precedent — the review scope is the operator surface family).
const pilotOutcomeScopeText = "Чтение исхода обращения и отчёта пилота требуют полномочия review: участник без полномочия review не читает панель исхода и отчёт пилота."

// pilotOutcomePortsMissingText is the honest refusal of the unwired
// contour (the preview precedent — never a nil panic).
const pilotOutcomePortsMissingText = "контур исхода обращения не подключён: сверьте конфигурацию поверхности"

// pilotOutcomeWholePeriodWord is the period word of the report when the
// period is absent (AC-11 — the word rides every number of the render).
const pilotOutcomeWholePeriodWord = "весь период наблюдения"

// pilotOutcomePeriodWordPrefix is the period word prefix of the report
// when the period IS given (well-formed or malformed — the malformed
// word passes through as the period word, no filtering, nothing is
// guessed; AC-11 — the word rides every number of the render).
const pilotOutcomePeriodWordPrefix = "период: "

// The compile-time proof of the narrow interface registration (the
// transport type-asserts api.PilotOutcomeService — the OrderStatusService
// precedent).
var _ api.PilotOutcomeService = (*Service)(nil)

// requirePilotOutcomePorts refuses honestly when the contour is not
// wired: the pilot case source, the operation reads, the receipt reads,
// the verification reads, the correction reads, the supplier fact reads
// and the recovery case reads are mandatory (the same stores the
// assembly needs — the OrderStatusView pattern plus the recovery
// repository of 3-8).
func (s *Service) requirePilotOutcomePorts() error {
        if s.pilotCases == nil || s.operations == nil || s.dispatches == nil || s.verifications == nil || s.corrections == nil || s.orderFacts == nil || s.recoveries == nil {
                return fmt.Errorf("%s", pilotOutcomePortsMissingText)
        }
        return nil
}

// authorizePilotOutcome is the read gate of the contour: the membership
// FIRST (a non-member is the honest 404), the review scope 403 BEFORE
// any outcome material (the words-first discipline — the exact
// authorizeOrderStatus pattern of the operator surface family).
func (s *Service) authorizePilotOutcome(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        if !actor.ID.Valid() || !tenantID.Valid() {
                return tenancy.Context{}, domain.ErrNotFound
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Review); err != nil {
                if errors.Is(err, identityDomain.ErrNotFound) {
                        return tenancy.Context{}, domain.ErrNotFound
                }
                if errors.Is(err, identityDomain.ErrForbidden) {
                        return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, pilotOutcomeScopeText)
                }
                return tenancy.Context{}, err
        }
        return tenancy.New(actor.ID, tenantID)
}

// assemblePilotCaseMaterial reads the WHOLE outcome material of one case
// by the LIVE read-time assembly (ZERO writes — the pure reads of the
// existing surfaces): the case anchor with the accepted inbound timeline
// events (the attribution material), the vehicle identification panel of
// 4-2, the part request panel of 4-3, the fitment ladder of 4-4, the
// work items and the handoff requests of 2-13 — all through the NEW
// PilotCaseSource port; then EVERY order operation of the case through
// the EXISTING action stores (the committed receipt with the external
// ABCP Order ID, the verification tasks with their events, the supplier
// facts, the corrections count and the recovery case count — the
// OrderStatusView read pattern). A foreign case is the honest error of
// the cases module semantics (the 404 rides the CaseView read).
func (s *Service) assemblePilotCaseMaterial(ctx context.Context, scope tenancy.Context, actor identityDomain.Principal, caseID id.UUID) (domain.PilotCaseMaterial, error) {
        material := domain.PilotCaseMaterial{}
        caseView, err := s.pilotCases.CaseView(ctx, actor, scope.TenantID(), caseID)
        if err != nil {
                return material, err
        }
        material.CaseFound = true
        material.CaseID = caseView.Case.CaseID
        material.CaseNo = caseView.Case.CaseNo
        material.CreatedAt = caseView.Case.CreatedAt
        // The attribution material: ONLY the accepted inbound channel events
        // of the timeline (the first one wins the attribution inside the pure
        // derivation — the honest fold over any arrival order).
        for _, event := range caseView.Timeline {
                if event.Direction == casesDomain.DirectionInbound && event.State == casesDomain.EventStateAccepted {
                        material.InboundEvents = append(material.InboundEvents, domain.PilotInboundEvent{
                                Provider:        event.Provider,
                                Channel:         event.Channel,
                                ProviderEventID: event.ProviderEventID,
                                ReceivedAt:      event.ReceivedAt,
                        })
                }
        }
        // The vehicle identification material of 4-2: the panel's honest
        // schema flag, the completeness answer of the alternative sets and
        // the confirmed attribute keys BY NAME with the last confirmation
        // time (the honest primitives the fold reads).
        material.Vehicle = domain.PilotVehicleMaterial{
                SchemaDefined:           caseView.Vehicle.SchemaDefined,
                IdentificationSatisfied: len(caseView.Vehicle.Profile.Completeness.SatisfiedBy) > 0,
        }
        for _, attribute := range caseView.Vehicle.Profile.Attributes {
                if attribute.Confirmation == nil {
                        continue
                }
                material.Vehicle.ConfirmedKeys = append(material.Vehicle.ConfirmedKeys, attribute.MaterialKey)
                if attribute.Confirmation.At.After(material.Vehicle.LastConfirmedAt) {
                        material.Vehicle.LastConfirmedAt = attribute.Confirmation.At
                }
        }
        // The part request material of 4-3: the honest confirmed signal is
        // the PRESENCE OF A CONFIRMED ATTRIBUTE (a manager's confirmation of
        // the exact version exists) — the PartRequestDigest is ALWAYS
        // non-empty (an empty confirmed set digests the honest empty object)
        // and therefore never a confirmed signal; the same
        // confirmation-presence pattern as the vehicle panel.
        material.PartRequestConfirmed = caseView.PartRequest.SchemaDefined
        for _, attribute := range caseView.PartRequest.Part.Attributes {
                if attribute.Confirmation != nil {
                        material.PartRequestConfirmed = true
                        break
                }
        }
        // The fitment material of 4-4: every position state BY NAME; the
        // proven position carries the LATEST evidence receive time (the
        // moment the ladder reached the proof — honest over the append-only
        // records).
        for _, position := range caseView.Fitment.Fitment.Positions {
                row := domain.PilotFitmentPositionMaterial{
                        PositionKey: position.PositionKey,
                        Status:      position.Status,
                }
                if position.Status == casesDomain.FitmentStatusProven {
                        for _, evidence := range position.Evidences {
                                if evidence.ReceivedAt.After(row.ProvenAt) {
                                        row.ProvenAt = evidence.ReceivedAt
                                }
                        }
                }
                material.Fitment.Positions = append(material.Fitment.Positions, row)
        }
        material.WorkItemsCount = len(caseView.WorkItems)
        material.HandoffRequests = caseView.Handoff.RequestsCount
        // The order machine material of 4.6..4.9 — EVERY order operation of
        // the case (the honest aggregate: the case is anchored by ITS OWN
        // operations, never by an external ID — the double-count structural
        // fence of AC-3/AC-17).
        operations, err := s.operations.ReadCaseOperations(ctx, scope, caseID)
        if err != nil {
                return material, err
        }
        for _, op := range operations {
                if op.DefinitionID != domain.OrderActionID {
                        continue
                }
                order := domain.PilotOrderMaterial{
                        Operation: domain.OrderStatusOperationMaterial{
                                Found:       true,
                                OperationID: op.OperationID,
                                WorkItemID:  op.WorkItemID,
                        },
                }
                // The committed receipt with the external ABCP Order ID (the
                // FIRST committed receipt with a non-empty external ID — the
                // evidence of the client_order_created stage).
                receipts, err := s.dispatches.ReadActionReceipts(ctx, scope, op.OperationID)
                if err != nil {
                        return material, err
                }
                for _, receipt := range receipts {
                        if receipt.Outcome == domain.ReceiptOutcomeCommitted && receipt.References.ExternalObjectID != "" {
                                order.Operation.Receipt = &domain.OrderStatusReceiptPin{
                                        ReceiptVersionNo: receipt.ReceiptVersionNo,
                                        ExternalOrderID:  receipt.References.ExternalObjectID,
                                        CreatedAt:        receipt.CreatedAt,
                                }
                                break
                        }
                }
                // The verification tasks of the operation with their events (the
                // 4-7 material of the fold — the exact OrderStatusView pattern).
                tasks, err := s.verifications.ReadOperationVerifications(ctx, scope, op.OperationID)
                if err != nil {
                        return material, err
                }
                for _, task := range tasks {
                        events, err := s.verifications.ReadVerificationEvents(ctx, scope, task.CaseID, task.VerificationNo)
                        if err != nil {
                                return material, err
                        }
                        order.Verifications = append(order.Verifications, domain.OrderStatusVerificationMaterial{
                                Task:   task,
                                Events: events,
                        })
                }
                // The supplier facts of the operation (the human receipts of
                // 4-9 — the sent fact proves the supplier_sent stage).
                facts, err := s.orderFacts.ReadOperationSupplierFacts(ctx, scope, op.OperationID)
                if err != nil {
                        return material, err
                }
                order.Facts = facts
                material.Orders = append(material.Orders, order)
                // The corrections of the operation (the 4-8 counter — the honest
                // count of the recorded rows, never a hidden miss).
                corrections, err := s.corrections.ReadOperationCorrections(ctx, scope, op.OperationID)
                if err != nil {
                        return material, err
                }
                material.Corrections += len(corrections)
                // The recovery case of the operation (the 3-8 counter — the
                // honest degradation of the recovery read: an error means NO
                // recovery case (ErrRecoveryNotFound), the exact
                // recoveryOperationCardExtension pattern).
                if _, err := s.recoveries.ReadRecoveryCaseByOperation(ctx, scope, op.OperationID); err == nil {
                        material.RecoveryCases++
                }
        }
        return material, nil
}

// ReadPilotCaseOutcome derives the outcome of ONE case over the recorded
// material (the panel view of the case page): the ports gate, the review
// gate, the LIVE material assembly and the PURE domain derivation
// (DeriveCaseOutcome — zero writes, idempotent). The now parameter stays
// in the signature for the surface symmetry of the read family; the pure
// derivation reads the recorded times only (the same world answers the
// same — AC-13).
func (s *Service) ReadPilotCaseOutcome(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (domain.PilotCaseOutcome, error) {
        if err := s.requirePilotOutcomePorts(); err != nil {
                return domain.PilotCaseOutcome{}, err
        }
        scope, err := s.authorizePilotOutcome(ctx, actor, tenantID)
        if err != nil {
                return domain.PilotCaseOutcome{}, err
        }
        material, err := s.assemblePilotCaseMaterial(ctx, scope, actor, caseID)
        if err != nil {
                return domain.PilotCaseOutcome{}, err
        }
        return domain.DeriveCaseOutcome(material), nil
}

// ReadPilotReport derives the aggregate pilot report of one period (the
// report page): the listing of the cases of the tenant is re-read
// through the pilot port, the period filter keeps ONLY the cases created
// in the month of a WELL-FORMED YYYY-MM period (a malformed period word
// passes through as the period word with NO filtering — the word rides
// the report, the render shows it, nothing is guessed; an absent period
// is the whole observation period), every case of the sample derives by
// the SAME per-case fold and the aggregate carries the sample size, the
// period word, the stage counts, the conversions, the counters, the
// unknown share and the Measurement Dictionary v1. ZERO writes.
func (s *Service) ReadPilotReport(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, period string, now time.Time) (domain.PilotReport, error) {
        if err := s.requirePilotOutcomePorts(); err != nil {
                return domain.PilotReport{}, err
        }
        scope, err := s.authorizePilotOutcome(ctx, actor, tenantID)
        if err != nil {
                return domain.PilotReport{}, err
        }
        listing, err := s.pilotCases.CasesView(ctx, actor, scope.TenantID())
        if err != nil {
                return domain.PilotReport{}, err
        }
        // The honest period rule: ONLY a well-formed YYYY-MM period filters
        // the sample; anything else is the whole observation period with the
        // period word riding the report as given.
        var month *time.Time
        if parsed, parseErr := time.Parse("2006-01", period); parseErr == nil {
                month = &parsed
        }
        outcomes := []domain.PilotCaseOutcome{}
        for _, row := range listing.Cases {
                if month != nil {
                        created := row.CreatedAt
                        if created.Year() != month.Year() || created.Month() != month.Month() {
                                continue
                        }
                }
                material, err := s.assemblePilotCaseMaterial(ctx, scope, actor, row.CaseID)
                if err != nil {
                        return domain.PilotReport{}, err
                }
                outcomes = append(outcomes, domain.DeriveCaseOutcome(material))
        }
        periodWord := pilotOutcomePeriodWordPrefix + period
        if period == "" {
                periodWord = pilotOutcomeWholePeriodWord
        }
        return domain.DerivePilotReport(outcomes, periodWord), nil
}
