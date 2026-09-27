package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/ingest"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	httptransport "github.com/artemiosu/staffy/internal/transport/http"
)

var (
	principalID = id.MustParse("01994f64-6c00-7000-8000-000000000001")
	tenantID    = id.MustParse("01994f64-6c00-7000-8000-000000000002")
)

type identityFixture struct{}

func (identityFixture) CreateTenant(context.Context, api.CreateTenantCommand) (api.CreateTenantResult, error) {
	return api.CreateTenantResult{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "Owner"}, nil
}
func (identityFixture) Workspace(context.Context, domain.Principal, id.UUID) (domain.Workspace, error) {
	return domain.Workspace{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "owner"}, nil
}
func (identityFixture) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return []domain.Membership{{TenantID: tenantID, PrincipalID: &principalID, Roles: []domain.Role{domain.Owner}, Scopes: domain.AllScopes(), Status: "active", Version: 1}}, nil
}
func (identityFixture) InviteMember(context.Context, api.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrStepUpRequired
}
func (identityFixture) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}
func (identityFixture) ChangeMember(context.Context, api.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrStepUpRequired
}
func (identityFixture) RevokeMember(context.Context, api.MembershipCommand) error {
	return domain.ErrStepUpRequired
}
func (identityFixture) Authorize(context.Context, domain.Principal, id.UUID, domain.Scope) error {
	return nil
}

type resolverFixture struct{}

func (resolverFixture) Resolve(context.Context) (domain.Principal, error) {
	return domain.Principal{ID: principalID, CanCreateTenant: true}, nil
}

type clockFixture struct{}

func (clockFixture) Now() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }

type idsFixture struct{}

func (idsFixture) New(time.Time) (id.UUID, error) {
	return id.MustParse("01994f64-6c00-7000-8000-000000000003"), nil
}

type employeeFixture struct {
	mu        sync.Mutex
	draft     *employeeDomain.Draft
	knowledge *employeeDomain.KnowledgeSource
	versions  []employeeDomain.KnowledgeVersion
}

var fixtureEmployeeID = id.MustParse("01994f64-6c00-7000-8000-000000000004")
var fixtureSourceID = id.MustParse("01994f64-6c00-7000-8000-000000000005")

func (f *employeeFixture) Create(_ context.Context, _ employeeAPI.CreateCommand) (employeeAPI.CreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil {
		f.draft = &employeeDomain.Draft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, OwnerID: principalID, Status: "draft", Readiness: "incomplete", Version: 1, CurrentStep: "brief", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	}
	return employeeAPI.CreateResult{Draft: *f.draft}, nil
}
func (f *employeeFixture) List(_ context.Context, _ domain.Principal, _ id.UUID) ([]employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil {
		return []employeeDomain.Draft{}, nil
	}
	return []employeeDomain.Draft{*f.draft}, nil
}
func (f *employeeFixture) Read(_ context.Context, _ domain.Principal, _ id.UUID, employeeID id.UUID) (employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil || employeeID != fixtureEmployeeID {
		return employeeDomain.Draft{}, employeeDomain.ErrNotFound
	}
	return *f.draft, nil
}
func (f *employeeFixture) Save(_ context.Context, c employeeAPI.SaveCommand) (employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil || c.EmployeeID != fixtureEmployeeID {
		return employeeDomain.Draft{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedVersion != f.draft.Version {
		return employeeDomain.Draft{}, employeeDomain.ErrConflict
	}
	if !c.Brief.Valid() {
		return employeeDomain.Draft{}, employeeDomain.ErrInvalid
	}
	f.draft.Version++
	f.draft.Brief = c.Brief
	f.draft.CurrentStep = "launch_plan"
	f.draft.UpdatedAt = time.Now().UTC()
	return *f.draft, nil
}
func (f *employeeFixture) Catalog(context.Context, domain.Principal, id.UUID) ([]employeeDomain.PackManifest, error) {
	return packs.List(), nil
}
func (f *employeeFixture) Preview(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.PackPreview, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := packs.Get(c.PackID, c.PackVersion)
	if !ok || f.draft == nil || f.draft.Pack == nil {
		return employeeDomain.PackPreview{}, employeeDomain.ErrInvalid
	}
	impact, allowed := packs.Impact(*f.draft.Pack, m)
	if !allowed {
		return employeeDomain.PackPreview{}, employeeDomain.ErrInvalid
	}
	return employeeDomain.PackPreview{Current: *f.draft.Pack, Target: m, Impact: impact}, nil
}
func (f *employeeFixture) Pin(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil {
		return employeeDomain.Draft{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedVersion != f.draft.Version {
		return employeeDomain.Draft{}, employeeDomain.ErrConflict
	}
	m, ok := packs.Get(c.PackID, c.PackVersion)
	if !ok || len(employeeDomain.ValidateVariants(m, c.Variants)) > 0 {
		return employeeDomain.Draft{}, employeeDomain.ErrInvalid
	}
	if f.draft.Pack != nil && (f.draft.Pack.ID != c.PackID || f.draft.Pack.Version != c.PackVersion) {
		impact, ok := packs.Impact(*f.draft.Pack, m)
		if !ok || !c.Confirmed {
			return employeeDomain.Draft{}, employeeDomain.ErrInvalid
		}
		f.draft.ReviewSlots = impact.ReviewSlots
	}
	f.draft.Pack = &employeeDomain.PackPin{ID: c.PackID, Version: c.PackVersion, Variants: c.Variants}
	f.draft.Version++
	return *f.draft, nil
}

func (f *employeeFixture) CreateKnowledge(_ context.Context, c employeeAPI.KnowledgeCreateCommand) (employeeDomain.KnowledgeSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil || f.draft.Pack == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID {
		return employeeDomain.KnowledgeSource{}, employeeDomain.ErrNotFound
	}
	if !c.Source.Valid() {
		return employeeDomain.KnowledgeSource{}, employeeDomain.ErrInvalid
	}
	s := c.Source
	s.TenantID = tenantID
	s.EmployeeID = fixtureEmployeeID
	s.SourceID = fixtureSourceID
	s.Status = "processing"
	s.Version = 1
	s.CreatedAt = time.Now().UTC()
	s.UpdatedAt = s.CreatedAt
	f.knowledge = &s
	f.versions = nil
	return s, nil
}
func (f *employeeFixture) IngestKnowledge(_ context.Context, c employeeAPI.KnowledgeIngestCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil || f.draft.Pack == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID || !c.Fixture || c.Source.Type != "document" || c.URL != "" || len(c.File) != 0 {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, employeeDomain.ErrInvalid
	}
	s := c.Source
	s.TenantID, s.EmployeeID, s.SourceID = tenantID, fixtureEmployeeID, fixtureSourceID
	s.Origin, s.Status, s.Version = ingest.FixtureOrigin, "processing", 1
	s.CreatedAt = time.Now().UTC()
	s.UpdatedAt = s.CreatedAt
	f.knowledge = &s
	f.versions = nil
	return s, employeeDomain.KnowledgeVersion{}, nil
}
func (f *employeeFixture) ReingestSyntheticKnowledge(_ context.Context, c employeeAPI.KnowledgeFixtureCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.knowledge == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID || c.SourceID != fixtureSourceID || f.knowledge.Origin != ingest.FixtureOrigin || f.knowledge.Status == "retired" {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrNotFound
	}
	text, err := ingest.SyntheticText(c.FixtureID)
	if err != nil {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrInvalid
	}
	if len(f.versions) > 0 && f.versions[0].Text == text {
		return *f.knowledge, f.versions[0], true, nil
	}
	if c.ExpectedVersion != f.knowledge.Version {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrConflict
	}
	now := time.Now().UTC()
	key := []byte("fixture-secret-for-knowledge-checksums")
	v := employeeDomain.KnowledgeVersion{SourceID: fixtureSourceID, Version: f.knowledge.CurrentVersion + 1, Checksum: employeeDomain.KnowledgeChecksum(key, text), Text: text, Origin: ingest.FixtureOrigin, CreatedAt: now, Chunks: employeeDomain.KnowledgeChunks(key, text)}
	for i := range v.Chunks {
		v.Chunks[i].SourceID, v.Chunks[i].Version = fixtureSourceID, v.Version
		v.Chunks[i].Origin, v.Chunks[i].CapturedAt = v.Origin, now
	}
	f.versions = append([]employeeDomain.KnowledgeVersion{v}, f.versions...)
	f.knowledge.CurrentVersion = v.Version
	f.knowledge.Version++
	f.knowledge.Status = "preview"
	return *f.knowledge, v, false, nil
}
func (f *employeeFixture) ListKnowledge(_ context.Context, _ domain.Principal, tenant, employee id.UUID) ([]employeeDomain.KnowledgeSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tenant != tenantID || employee != fixtureEmployeeID {
		return nil, employeeDomain.ErrNotFound
	}
	if f.knowledge == nil {
		return []employeeDomain.KnowledgeSource{}, nil
	}
	return []employeeDomain.KnowledgeSource{*f.knowledge}, nil
}
func (f *employeeFixture) ReadKnowledge(_ context.Context, _ domain.Principal, tenant, employee, source id.UUID) (employeeDomain.KnowledgeSource, []employeeDomain.KnowledgeVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tenant != tenantID || employee != fixtureEmployeeID || source != fixtureSourceID || f.knowledge == nil {
		return employeeDomain.KnowledgeSource{}, nil, employeeDomain.ErrNotFound
	}
	return *f.knowledge, append([]employeeDomain.KnowledgeVersion(nil), f.versions...), nil
}
func (f *employeeFixture) ProcessKnowledge(_ context.Context, c employeeAPI.KnowledgeProcessCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.knowledge == nil || c.SourceID != fixtureSourceID {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrNotFound
	}
	if !employeeDomain.ValidKnowledgeText(c.Text) {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrInvalid
	}
	if f.knowledge.Status == "retired" {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrInvalid
	}
	key := []byte("fixture-secret-for-knowledge-checksums")
	checksum := employeeDomain.KnowledgeChecksum(key, c.Text)
	if len(f.versions) > 0 && f.versions[0].Checksum == checksum {
		return *f.knowledge, f.versions[0], true, nil
	}
	if c.ExpectedVersion != f.knowledge.Version {
		return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrConflict
	}
	v := employeeDomain.KnowledgeVersion{SourceID: fixtureSourceID, Version: f.knowledge.CurrentVersion + 1, Checksum: checksum, Text: c.Text, Chunks: employeeDomain.KnowledgeChunks(key, c.Text), CreatedAt: time.Now().UTC()}
	for i := range v.Chunks {
		v.Chunks[i].SourceID = fixtureSourceID
		v.Chunks[i].Version = v.Version
	}
	if len(f.versions) > 0 {
		v.RemovedFromPrevious, v.AddedFromPrevious = employeeDomain.KnowledgeDiff(f.versions[0].Text, v.Text)
	}
	f.versions = append([]employeeDomain.KnowledgeVersion{v}, f.versions...)
	f.knowledge.CurrentVersion = v.Version
	f.knowledge.Version++
	f.knowledge.Status = "ready"
	return *f.knowledge, v, false, nil
}
func (f *employeeFixture) ConflictKnowledge(_ context.Context, c employeeAPI.KnowledgeConflictCommand) (employeeDomain.KnowledgeSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.knowledge == nil || c.SourceID != fixtureSourceID {
		return employeeDomain.KnowledgeSource{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedVersion != f.knowledge.Version {
		return employeeDomain.KnowledgeSource{}, employeeDomain.ErrConflict
	}
	f.knowledge.ConflictNote = c.Note
	f.knowledge.ConflictProvenance = c.Provenance
	f.knowledge.Version++
	return *f.knowledge, nil
}
func (f *employeeFixture) RetireKnowledge(_ context.Context, c employeeAPI.KnowledgeRetireCommand) (employeeDomain.KnowledgeSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.knowledge == nil || c.SourceID != fixtureSourceID {
		return employeeDomain.KnowledgeSource{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedVersion != f.knowledge.Version {
		return employeeDomain.KnowledgeSource{}, employeeDomain.ErrConflict
	}
	f.knowledge.Status = "retired"
	f.knowledge.Version++
	return *f.knowledge, nil
}

func main() {
	server, err := httptransport.New(identityFixture{}, resolverFixture{}, idsFixture{}, clockFixture{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	server.BindEmployee(&employeeFixture{})
	if err := http.ListenAndServe("127.0.0.1:18081", server.Handler()); err != nil {
		panic(err)
	}
}
