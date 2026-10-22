package app

import (
        "context"

        "github.com/artemiosu/staffy/internal/employee/api"
        "github.com/artemiosu/staffy/internal/employee/domain"
        "github.com/artemiosu/staffy/internal/employee/port"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// PilotAdmission (spec 4-1, Epic 4) composes the tenant-level read-time
// derivation of the pilot admission registry: the sealed Domain Pack pin,
// the DERIVED capability states through the narrow connector facts port
// (never secrets) and the full gate instance history of the tenant's
// employees — every input read live through the REUSED ports of the
// readiness/release surfaces, nothing cached, nothing written. The
// function performs no mutation: the pilot admission is a derivation, not
// a record (a repeated read of an unchanged world is the same answer).
func (s *Service) PilotAdmission(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (api.PilotAdmissionView, error) {
        // The rights of the test & release view scope reused (precedent 1-14):
        // configuration manages, review/audit view, members without a scope get
        // the honest 403, non-members the plain 404 — the tenant existence is
        // never disclosed before the material.
        scope, canManage, err := s.readinessViewScope(ctx, actor, tenantID)
        if err != nil {
                return api.PilotAdmissionView{}, err
        }
        release := s.releaseRepository()
        readiness := s.readinessRepository()
        if release == nil || readiness == nil {
                return api.PilotAdmissionView{}, domain.ErrNotFound
        }
        // (1) The pack pin: the sealed manifests of the tenant's employees read
        // live (the honest payload read of the reused release surface). A
        // manifest pins the pack in its config refs; the derivation consumes
        // the highest pinned version of the required pack. An absent pin is the
        // honest fail-closed start of every route — never an assumed "ok".
        packID, packVersion, err := s.pilotPackPin(ctx, scope, release)
        if err != nil {
                return api.PilotAdmissionView{}, err
        }
        // (2) The capability facts: the derived states of the tenant's current
        // connections through the narrow caller-authority port — identifiers,
        // states and evidence metadata, never secrets or config values.
        capabilities, err := s.pilotCapabilityFacts(ctx, scope)
        if err != nil {
                return api.PilotAdmissionView{}, err
        }
        // (3) The gate instances: the full append-only history of the tenant's
        // employees (the pilot gates are tenant-level definitions — a PASS
        // recorded by any employee aggregate of the tenant is a tenant fact of
        // the external account; the instance rows carry their employee
        // attribution visibly).
        instances, err := s.pilotGateInstances(ctx, scope, readiness)
        if err != nil {
                return api.PilotAdmissionView{}, err
        }
        report := domain.DerivePilotAdmission(domain.PilotFacts{
                PackID: packID, PackVersion: packVersion,
                Capabilities: capabilities,
                Instances:    instances,
        })
        view := api.PilotAdmissionView{TenantID: tenantID, Report: report, CanManage: canManage, ViewScope: api.ReadinessScopeView}
        if canManage {
                view.ViewScope = api.ReadinessScopeManage
        }
        return view, nil
}

// pilotPackPin reads the sealed manifests of every employee of the tenant
// and resolves the highest pinned version of the required pilot pack. The
// read is the reused release-surface honest payload read: a manifest that
// cannot be read honestly fails the whole view — a pin is never silently
// assumed (a sealed manifest that pins nothing stays an honest absence).
func (s *Service) pilotPackPin(ctx context.Context, scope tenancy.Context, release port.ReleaseRepository) (string, string, error) {
        drafts, err := s.repository.List(ctx, scope)
        if err != nil {
                return "", "", err
        }
        packID, packVersion := "", ""
        for _, draft := range drafts {
                manifests, err := release.ListManifests(ctx, scope, draft.EmployeeID)
                if err != nil {
                        return "", "", err
                }
                for _, record := range manifests {
                        // The honest payload read of the reused release surface: schema
                        // version, strict decode, digest re-computation. A quarantined or
                        // unreadable manifest never pins anything — an honest absence, not
                        // an assumed pin.
                        read := domain.ReadManifestPayload(record.Payload, record.Manifest.Digest)
                        if read.Status != "" {
                                continue
                        }
                        ref := read.Content.Config.Refs.Pack
                        if ref.Status != domain.RefPresent || ref.ID != domain.PilotRequiredPackID {
                                continue
                        }
                        // The highest pinned version wins (the display word carries it);
                        // the derivation itself re-checks the minimum.
                        if packVersion == "" || domain.PackVersionAtLeast(ref.Version, packVersion) {
                                packID, packVersion = ref.ID, ref.Version
                        }
                }
        }
        return packID, packVersion, nil
}

// pilotCapabilityFacts flattens the derived capability states of the
// tenant's current connections into the per-capability fact map the
// derivation reads: a capability proven by live evidence (verified, no
// gap) on ANY current connection is the live proof of the tenant; without
// such a proof the first fact stays visible with its honest state word
// (stale, failed, unverified — the word names the exact state).
func (s *Service) pilotCapabilityFacts(ctx context.Context, scope tenancy.Context) (map[string]domain.CapabilityFact, error) {
        connections, err := s.connectorFacts(ctx, scope)
        if err != nil {
                return nil, err
        }
        capabilities := make(map[string]domain.CapabilityFact)
        for _, connection := range connections {
                for _, fact := range connection.Capabilities {
                        if fact.Capability == "" {
                                continue
                        }
                        current, known := capabilities[fact.Capability]
                        if !known {
                                capabilities[fact.Capability] = fact
                                continue
                        }
                        // A verified live proof wins over any other state of another
                        // connection; a proven fact is never displaced by a worse one.
                        if current.State != "verified" && fact.State == "verified" {
                                capabilities[fact.Capability] = fact
                        }
                }
        }
        return capabilities, nil
}

// pilotGateInstances reads the full gate instance history of the tenant's
// employees (the reused readiness store read; the instances stay
// append-only — this path only ever reads them).
func (s *Service) pilotGateInstances(ctx context.Context, scope tenancy.Context, readiness port.ReadinessRepository) ([]domain.GateInstance, error) {
        drafts, err := s.repository.List(ctx, scope)
        if err != nil {
                return nil, err
        }
        instances := make([]domain.GateInstance, 0)
        for _, draft := range drafts {
                rows, err := readiness.ListGateInstances(ctx, scope, draft.EmployeeID)
                if err != nil {
                        return nil, err
                }
                instances = append(instances, rows...)
        }
        return instances, nil
}
