package app

import (
	"context"

	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// MontagioAdmission (spec 5-1, Epic 5) composes the tenant-level read-time
// derivation of the Montagio admission registry: the sealed «Услуги» Domain
// Pack pin, the DERIVED capability states through the narrow connector
// facts port (never secrets) and the full gate instance history of the
// tenant's employees — every input read live through the REUSED ports of
// the readiness/release surfaces, nothing cached, nothing written. The
// function performs no mutation: the admission is a derivation, not a
// record (a repeated read of an unchanged world is the same answer).
func (s *Service) MontagioAdmission(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (api.MontagioAdmissionView, error) {
	// The rights of the test & release view scope reused (precedent 1-14/4-1):
	// configuration manages, review/audit view, members without a scope get
	// the honest 403, non-members the plain 404 — the tenant existence is
	// never disclosed before the material.
	scope, canManage, err := s.readinessViewScope(ctx, actor, tenantID)
	if err != nil {
		return api.MontagioAdmissionView{}, err
	}
	release := s.releaseRepository()
	readiness := s.readinessRepository()
	if release == nil || readiness == nil {
		return api.MontagioAdmissionView{}, domain.ErrNotFound
	}
	// (1) The pack pin: the sealed manifests of the tenant's employees read
	// live; the derivation consumes the highest pinned version of the
	// required «Услуги» pack. An absent pin is the honest fail-closed start
	// of every route — never an assumed "ok".
	packID, packVersion, err := s.montagioPackPin(ctx, scope, release)
	if err != nil {
		return api.MontagioAdmissionView{}, err
	}
	// (2) The capability facts: the derived states of the tenant's current
	// connections through the narrow caller-authority port (the same reused
	// composition the Oleg ladder reads).
	capabilities, err := s.pilotCapabilityFacts(ctx, scope)
	if err != nil {
		return api.MontagioAdmissionView{}, err
	}
	// (3) The gate instances: the full append-only history of the tenant's
	// employees (the Montagio gates are tenant-level definitions — a PASS
	// recorded by any employee aggregate of the tenant is a tenant fact).
	instances, err := s.pilotGateInstances(ctx, scope, readiness)
	if err != nil {
		return api.MontagioAdmissionView{}, err
	}
	report := domain.DeriveMontagioAdmission(domain.MontagioFacts{
		PackID: packID, PackVersion: packVersion,
		Capabilities: capabilities,
		Instances:    instances,
	})
	view := api.MontagioAdmissionView{TenantID: tenantID, Report: report, CanManage: canManage, ViewScope: api.ReadinessScopeView}
	if canManage {
		view.ViewScope = api.ReadinessScopeManage
	}
	return view, nil
}

// montagioPackPin reads the sealed manifests of every employee of the tenant
// and resolves the highest pinned version of the required «Услуги» pack —
// the Montagio twin of pilotPackPin (the pack words differ, the honest
// payload read discipline is identical: schema version, strict decode,
// digest re-computation; a quarantined or unreadable manifest never pins
// anything).
func (s *Service) montagioPackPin(ctx context.Context, scope tenancy.Context, release port.ReleaseRepository) (string, string, error) {
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
			read := domain.ReadManifestPayload(record.Payload, record.Manifest.Digest)
			if read.Status != "" {
				continue
			}
			ref := read.Content.Config.Refs.Pack
			if ref.Status != domain.RefPresent || ref.ID != domain.MontagioRequiredPackID {
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
