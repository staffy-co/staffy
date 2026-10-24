package app

import (
        "context"
        "errors"
        "fmt"

        casesPort "github.com/artemiosu/staffy/internal/cases/port"
        "github.com/artemiosu/staffy/internal/employee/api"
        "github.com/artemiosu/staffy/internal/employee/domain"
        "github.com/artemiosu/staffy/internal/employee/packs"
        employeePort "github.com/artemiosu/staffy/internal/employee/port"
        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// EntitlementAdmissionFacts is the narrow caller-authority entitlement
// admission fact port (the ConnectionsFacts/MembersFacts precedent): the
// consumer (the RouteFacts facade for the ingress of Story 2-1) verified
// its own scope from the trusted connection metadata, so the admission
// read does not re-require a user principal. The concrete entitlement
// service satisfies it structurally.
type EntitlementAdmissionFacts interface {
        AdmissionFacts(ctx context.Context, scope tenancy.Context) (entitlementDomain.AdmissionState, error)
}

// WithEntitlementAdmissionFacts wires the scope-based entitlement
// admission fact port (the ingress relay). Without it the RouteFacts
// facade honestly refuses instead of assuming an "ok" perimeter.
func (s *Service) WithEntitlementAdmissionFacts(entitlement EntitlementAdmissionFacts) *Service {
        s.entitlementFacts = entitlement
        return s
}

// routeFactsRepository returns the RouteFacts persistence when the
// repository supports it (the postgres store does).
func (s *Service) routeFactsRepository() employeePort.RouteFactsRepository {
        if repository, ok := s.repository.(employeePort.RouteFactsRepository); ok {
                return repository
        }
        return nil
}

// RouteFacts is the safe cross-module projection for the ingress ladder of
// Story 2.1: the acting release decisions whose sealed manifests contain
// the connection, each with the per-route results and the human coverage
// of the exact pinned profile version, plus the relayed entitlement
// admission word. This is a caller-authority read: the scope comes from
// the verified connection (trusted metadata), never from a user
// principal; RLS still isolates the tenant. Zero, one or many candidates
// decide the ladder step — the policy (refusal, admission or the
// ambiguity quarantine) belongs to the consumer, not to this facade.
func (s *Service) RouteFacts(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (api.RouteFacts, error) {
        if !scope.Valid() || !connectionID.Valid() {
                return api.RouteFacts{}, domain.ErrNotFound
        }
        repository := s.routeFactsRepository()
        if repository == nil {
                return api.RouteFacts{}, domain.ProfileRuleError{Section: "route_facts", Field: "repository", Reason: "Хранилище фактов маршрутизации не сконфигурировано: ingress честно отклоняется, состояние «ok» не предполагается."}
        }
        admissions, err := repository.ListTenantAdmissions(ctx, scope)
        if err != nil {
                return api.RouteFacts{}, err
        }
        facts := api.RouteFacts{ConnectionID: connectionID, Candidates: []api.RouteCandidate{}}
        for _, admission := range actingDecisions(admissions) {
                candidate, sealed, err := s.routeCandidate(ctx, repository, scope, admission, connectionID)
                if err != nil {
                        return api.RouteFacts{}, err
                }
                if sealed {
                        facts.Candidates = append(facts.Candidates, candidate)
                }
        }
        word, err := s.entitlementAdmissionWord(ctx, scope)
        if err != nil {
                return api.RouteFacts{}, err
        }
        facts.Entitlement = word
        return facts, nil
}

// actingDecisions derives the acting decision per employee (the highest
// decision number — DeriveActiveRelease per aggregate, the budget-current
// pattern of 1-11).
func actingDecisions(admissions []domain.ReleaseAdmission) []domain.ReleaseAdmission {
        acting := map[id.UUID]*domain.ReleaseAdmission{}
        order := []id.UUID{}
        for index := range admissions {
                employeeID := admissions[index].EmployeeID
                current, known := acting[employeeID]
                if !known {
                        acting[employeeID] = &admissions[index]
                        order = append(order, employeeID)
                        continue
                }
                if admissions[index].DecisionNo > current.DecisionNo {
                        *current = admissions[index]
                }
        }
        out := make([]domain.ReleaseAdmission, 0, len(order))
        for _, employeeID := range order {
                out = append(out, *acting[employeeID])
        }
        return out
}

// routeCandidate resolves one acting decision against the connection: the
// decision only counts when its sealed manifest content contains the
// connection fact. A manifest that cannot be read honestly (schema drift)
// fails the whole read — a candidate is never silently dropped.
func (s *Service) routeCandidate(ctx context.Context, repository employeePort.RouteFactsRepository, scope tenancy.Context, admission domain.ReleaseAdmission, connectionID id.UUID) (api.RouteCandidate, bool, error) {
        content, err := repository.ReadManifestContent(ctx, scope, admission.EmployeeID, admission.Manifest)
        if err != nil {
                return api.RouteCandidate{}, false, err
        }
        sealed := false
        for _, connection := range content.Connections {
                if connection.ConnectionID == connectionID {
                        sealed = true
                        break
                }
        }
        if !sealed {
                return api.RouteCandidate{}, false, nil
        }
        profile, err := repository.ReadApprovedProfileVersion(ctx, scope, admission.EmployeeID, admission.ProfileVersion)
        if err != nil {
                return api.RouteCandidate{}, false, err
        }
        if profile.Version != admission.ProfileVersion {
                return api.RouteCandidate{}, false, fmt.Errorf("запечатленная версия профиля %d сотрудника %s не читается: найдена версия %d", admission.ProfileVersion, admission.EmployeeID, profile.Version)
        }
        // The Domain Pack of the sealed configuration version: the taxonomy
        // source the case contour pins onto the case row at ingest. An honest
        // absent pack ref stays empty — the case contour refuses classification
        // with the honest "taxonomy unknown" text.
        packID, packVersion := "", ""
        if content.Config.Refs.Pack.Status == domain.RefPresent {
                packID, packVersion = content.Config.Refs.Pack.ID, content.Config.Refs.Pack.Version
        }
        return api.RouteCandidate{
                EmployeeID:     admission.EmployeeID,
                DecisionNo:     admission.DecisionNo,
                Stage:          admission.Stage,
                ProfileVersion: admission.ProfileVersion,
                PackID:         packID,
                PackVersion:    packVersion,
                Routes:         admission.Routes,
                Coverage:       profile.Content.Coverage,
        }, true, nil
}

// PackJourneys resolves the journeys dictionary of one Domain Pack version
// (the Work Item type taxonomy of the case contour). The manifest catalog
// is immutable in the binary: the same (id, version) always resolves the
// same dictionary — there is no TOCTOU between the pin and the read. An
// unknown pack version is an honest error, never an assumed empty
// dictionary.
func (s *Service) PackJourneys(packID, packVersion string) ([]string, error) {
        if packID == "" || packVersion == "" {
                return nil, fmt.Errorf("пак не закреплён: taxonomy неизвестна")
        }
        manifest, ok := packs.Get(packID, packVersion)
        if !ok {
                return nil, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
        }
        return append([]string(nil), manifest.Journeys...), nil
}

// PackCriticalEntities resolves the material key dictionary of one Domain
// Pack version (the fact form hints of the Work Control Card). The same
// immutability guarantees as PackJourneys; an unknown pack version is an
// honest error, never an assumed hint list.
func (s *Service) PackCriticalEntities(packID, packVersion string) ([]string, error) {
        if packID == "" || packVersion == "" {
                return nil, fmt.Errorf("пак не закреплён: словарь материальных ключей неизвестен")
        }
        manifest, ok := packs.Get(packID, packVersion)
        if !ok {
                return nil, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
        }
        return append([]string(nil), manifest.CriticalEntities...), nil
}

// PackIdentificationSchema resolves the vehicle identification schema of
// one Domain Pack version (Story 4.2 — the material keys with the kinds
// and the alternative sufficient sets). The same immutability guarantees
// as PackJourneys; a pack without a schema is the honest absent answer
// (the caller degrades the panel honestly, never a hardcoded list); an
// unknown pack version is an honest error.
func (s *Service) PackIdentificationSchema(packID, packVersion string) (casesPort.IdentificationSchema, error) {
        if packID == "" || packVersion == "" {
                return casesPort.IdentificationSchema{}, fmt.Errorf("пак не закреплён: схема идентификации неизвестна")
        }
        manifest, ok := packs.Get(packID, packVersion)
        if !ok {
                return casesPort.IdentificationSchema{}, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
        }
        schema := casesPort.IdentificationSchema{MaterialKeys: []casesPort.MaterialKeySpec{}, AlternativeSets: [][]string{}}
        if manifest.VehicleIdentification == nil {
                return schema, nil
        }
        for _, key := range manifest.VehicleIdentification.MaterialKeys {
                schema.MaterialKeys = append(schema.MaterialKeys, casesPort.MaterialKeySpec{
                        Key: key.Key, Label: key.Label, Kind: key.Kind, Min: key.Min, Max: key.Max, Optional: key.Optional,
                })
        }
        for _, set := range manifest.VehicleIdentification.AlternativeSets {
                schema.AlternativeSets = append(schema.AlternativeSets, append([]string(nil), set...))
        }
        return schema, nil
}

// PackPartRequestSchema resolves the part request schema of one Domain
// Pack version (Story 4.3 — the material keys with the roles, the mode
// dictionary with the priorities and the mandatory sets, and the safety
// markers). The same immutability guarantees as PackJourneys; a pack
// without a schema is the honest absent answer (the caller degrades the
// panel honestly, never a hardcoded list); an unknown pack version is an
// honest error.
func (s *Service) PackPartRequestSchema(packID, packVersion string) (casesPort.PartRequestSchema, error) {
        if packID == "" || packVersion == "" {
                return casesPort.PartRequestSchema{}, fmt.Errorf("пак не закреплён: схема запроса детали неизвестна")
        }
        manifest, ok := packs.Get(packID, packVersion)
        if !ok {
                return casesPort.PartRequestSchema{}, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
        }
        schema := casesPort.PartRequestSchema{MaterialKeys: []casesPort.PartMaterialKeySpec{}, Modes: []casesPort.PartModeSpec{}, SafetyMarkers: []string{}}
        if manifest.PartRequests == nil {
                return schema, nil
        }
        for _, key := range manifest.PartRequests.MaterialKeys {
                schema.MaterialKeys = append(schema.MaterialKeys, casesPort.PartMaterialKeySpec{
                        Key: key.Key, Label: key.Label, Kind: key.Kind, Values: append([]string(nil), key.Values...),
                        Min: key.Min, Max: key.Max, Optional: key.Optional, Role: key.Role,
                })
        }
        for _, mode := range manifest.PartRequests.Modes {
                schema.Modes = append(schema.Modes, casesPort.PartModeSpec{
                        Mode: mode.Mode, Label: mode.Label, Priority: mode.Priority, Requires: append([]string(nil), mode.Requires...),
                })
        }
        schema.SafetyMarkers = append([]string(nil), manifest.PartRequests.SafetyMarkers...)
        schema.PartSearchJourney = manifest.PartRequests.PartSearchJourney
        return schema, nil
}

// PackFitmentSchema resolves the fitment evidence policy of one Domain
// Pack version (Story 4.4 — the link types, the maximal cross chain
// depth, the allowed catalog sources and the maximal evidence age). The
// same immutability guarantees as PackJourneys; a pack without a policy
// is the honest absent answer (the caller degrades the panel honestly,
// never a hardcoded policy); an unknown pack version is an honest error.
func (s *Service) PackFitmentSchema(packID, packVersion string) (casesPort.FitmentPolicySpec, error) {
        if packID == "" || packVersion == "" {
                return casesPort.FitmentPolicySpec{}, fmt.Errorf("пак не закреплён: политика применимости неизвестна")
        }
        manifest, ok := packs.Get(packID, packVersion)
        if !ok {
                return casesPort.FitmentPolicySpec{}, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
        }
        policy := casesPort.FitmentPolicySpec{LinkTypes: []string{}, AllowedSources: []string{}}
        if manifest.Fitment == nil {
                return policy, nil
        }
        policy.LinkTypes = append([]string(nil), manifest.Fitment.LinkTypes...)
        policy.MaxCrossDepth = manifest.Fitment.MaxCrossDepth
        policy.AllowedSources = append([]string(nil), manifest.Fitment.AllowedSources...)
        policy.MaxAgeSeconds = manifest.Fitment.MaxAgeSeconds
        return policy, nil
}

// PackComparisonSchema resolves the offer comparison policy of one Domain
// Pack version (Story 4.5 — the allowed currencies, the maximal offer age,
// the ranking criteria with the labels, the deterministic tie breakers and
// the maximal visible variant count). The same immutability guarantees as
// PackJourneys; a pack without a policy is the honest absent answer (the
// caller degrades the panel honestly, never a hardcoded policy); an
// unknown pack version is an honest error.
func (s *Service) PackComparisonSchema(packID, packVersion string) (casesPort.ComparisonPolicySpec, error) {
        if packID == "" || packVersion == "" {
                return casesPort.ComparisonPolicySpec{}, fmt.Errorf("пак не закреплён: политика сравнения предложений неизвестна")
        }
        manifest, ok := packs.Get(packID, packVersion)
        if !ok {
                return casesPort.ComparisonPolicySpec{}, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
        }
        policy := casesPort.ComparisonPolicySpec{Currencies: []string{}, RankCriteria: []string{}, RankCriteriaLabels: map[string]string{}, TieBreakers: []string{}}
        if manifest.Comparison == nil {
                return policy, nil
        }
        policy.Currencies = append([]string(nil), manifest.Comparison.Currencies...)
        policy.MaxOfferAgeSeconds = manifest.Comparison.MaxOfferAgeSeconds
        policy.RankCriteria = append([]string(nil), manifest.Comparison.RankCriteria...)
        for criterion, label := range manifest.Comparison.RankCriteriaLabels {
                policy.RankCriteriaLabels[criterion] = label
        }
        policy.TieBreakers = append([]string(nil), manifest.Comparison.TieBreakers...)
        policy.MaxVisibleVariants = manifest.Comparison.MaxVisibleVariants
        return policy, nil
}

// entitlementAdmissionWord relays the scope-based entitlement admission
// state. Without the port the facade honestly refuses.
func (s *Service) entitlementAdmissionWord(ctx context.Context, scope tenancy.Context) (api.RouteEntitlementWord, error) {
        if s.entitlementFacts == nil {
                return api.RouteEntitlementWord{}, domain.ProfileRuleError{Section: "route_facts", Field: "entitlement_fact", Reason: "Факт-порт entitlement (скоуп) не сконфигурирован: ingress честно отклоняется, состояние «ok» не предполагается."}
        }
        state, err := s.entitlementFacts.AdmissionFacts(ctx, scope)
        if err != nil {
                if errors.Is(err, entitlementDomain.ErrForbidden) {
                        return api.RouteEntitlementWord{}, domain.ErrReadinessForbidden
                }
                if errors.Is(err, entitlementDomain.ErrNotFound) {
                        return api.RouteEntitlementWord{}, domain.ErrNotFound
                }
                return api.RouteEntitlementWord{}, err
        }
        // The relay normalizes the word into the ingress dictionary: "ok" or
        // "blocked" — the exact commercial states (blocked_new_admissions,
        // degraded) both bound NEW conversations; the reason text carries the
        // honest original detail.
        status := "ok"
        if state.Status != entitlementDomain.AdmissionOK {
                status = "blocked"
        }
        return api.RouteEntitlementWord{Status: status, Reason: state.Reason}, nil
}
