//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/adapters/capabilityresolver"
	"github.com/staffy-co/staffy/internal/adapters/packresolver"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// actionScopeWords mirrors the app wrap of the honest 403 (the words of
// the two scopes that DO open the surface — the single source lives in
// the app layer; the mirror pins the text against silent drift, the
// transport-test precedent of step-03).
const actionScopeWords = "Чтение каталога действий требует полномочия integrations или configuration: словарь действий — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

// actionMatrixSMS builds the guard-valid SMS fixture of the slice
// vertical (the transport step pinned the same shape): the full
// condition system and the delivery contract over the LIVE synthetic
// telephony declaration — command.send-sms carries Idempotent=true, so
// the provider_idempotent class passes the delivery-word cross-check of
// AC-3 against the real connector catalog. The 1.0.0 and 1.1.0 pairs
// differ MATERIALLY (risk low→medium, timeout 15000→30000) so the pin
// rows prove the old version is served byte-exact, never the newest
// projection (AC-8).
func actionMatrixSMS(defID, version string) actionDomain.ActionDefinition {
	risk, timeout := "medium", 30000
	if version == "1.0.0" {
		risk, timeout = "low", 15000
	}
	return actionDomain.ActionDefinition{
		ID:      defID,
		Version: version,
		Name:    "Отправка SMS клиенту",
		Purpose: "Уведомить клиента о готовности заказа",
		TypedInputSchema: []actionDomain.FieldSchema{
			{Name: "phone", Label: "Телефон получателя", Type: actionDomain.FieldText, Required: true, Description: "Номер в формате E.164"},
			{Name: "template_url", Label: "Ссылка на шаблон", Type: actionDomain.FieldURL, Secret: true, Description: "Шаблон сообщения"},
		},
		TypedOutputSchema: []actionDomain.FieldSchema{
			{Name: "message_id", Label: "Идентификатор сообщения", Type: actionDomain.FieldText},
		},
		PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
		OwningSystem:       "erp",
		Conditions: actionDomain.Conditions{
			RiskClass:                risk,
			AutonomyClass:            "assisted",
			AllowedConversationModes: []string{"claim", "cancellation"},
			AllowedRoles:             []string{"owner", "configurator"},
			RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: "fresh"}},
			Confirmation:             "required",
			RequiredGates:            []string{"OQ-7A", "AG-1"},
			TimeoutMS:                timeout,
			MaterialityClass:         "material",
			ConsumptionRule:          "single_use",
			AllowedStages:            []string{"closed_account_test", "controlled_real_customer"},
		},
		DeliveryContract: actionDomain.DeliveryContract{
			CapabilityClass:       "provider_idempotent",
			IdempotencyScope:      "connector-ack",
			TerminalEvidence:      "delivery-receipt",
			ReconciliationQuery:   "query.trunk-status",
			CancelCompensationSem: "не компенсируется — отказ до подтверждения",
		},
	}
}

// actionMatrixTrunkStatus builds the query-class fixture of the
// vertical: query_reconcilable with ConfirmationRequirement=not_required
// (the ONLY survival of the pair — confirm-before-mutate forces required
// on every mutating class), leaning on the LIVE trunk-status declaration
// (Idempotent=false with the declared reconciliation semantics — the
// query cross-check of AC-3 against the real connector catalog).
func actionMatrixTrunkStatus() actionDomain.ActionDefinition {
	return actionDomain.ActionDefinition{
		ID:      "query.trunk-status",
		Version: "1.0.0",
		Name:    "Статус телефонной линии",
		Purpose: "Узнать текущий статус телефонной линии перед звонком",
		TypedOutputSchema: []actionDomain.FieldSchema{
			{Name: "status", Label: "Статус линии", Type: actionDomain.FieldText, Description: "Слово статуса транка"},
		},
		PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "query.trunk-status"},
		OwningSystem:       "catalog",
		Conditions: actionDomain.Conditions{
			RiskClass:                "low",
			AutonomyClass:            "auto",
			AllowedConversationModes: []string{"safety", "claim"},
			AllowedRoles:             []string{"operator", "expert_qa"},
			RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "availability", Freshness: "fresh"}},
			Confirmation:             "not_required",
			RequiredGates:            []string{"OQ-7B"},
			TimeoutMS:                5000,
			MaterialityClass:         "immaterial",
			ConsumptionRule:          "unlimited",
			AllowedStages:            []string{"synthetic", "closed_account_test", "controlled_real_customer"},
		},
		DeliveryContract: actionDomain.DeliveryContract{
			CapabilityClass:       "query_reconcilable",
			IdempotencyScope:      "не требуется (read-only)",
			TerminalEvidence:      "query_result",
			ReconciliationQuery:   "query.trunk-status",
			CancelCompensationSem: "не применимо (read-only)",
		},
	}
}

// TestActionCatalogMatrixRowsBySpec is the Story 3.1 step-04 matrix: the
// honest rows of the action catalog read surface walked by name over the
// REAL app service with the REAL composition-root resolvers and the LIVE
// identity/PG membership (the step-03 transport test pinned the transport
// mapping over the fake; this matrix walks the live gate order, the pin
// semantics and the admission truth of the fixture composition root).
// The identity ids live in the free a2xx suffix range — no other test
// world derives from them (the membership collision lesson).
func TestActionCatalogMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	identity, owner := newService("00000000a201", "00000000a202", "00000000a203")
	tenant := createTenant(t, identity, owner, "Каталог действий", "action-matrix-create-1", "00000000a204").Tenant.ID
	// The membership-clock rebuild of the membership-test precedent: the
	// invitation UPDATE compares expires_at against the LIVE database
	// clock_timestamp(), so the service clock must sit at (near) the real
	// now — the frozen newService clock of 2026-09-16 would make every
	// invitation look expired at the acceptance UPDATE.
	identity = identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC().Truncate(time.Microsecond)}, &sequenceIDs{})
	owner.StepUpVerified = true
	invite := func(email, inviteSuffix, principalSuffix string, roles []identityDomain.Role, scopes []identityDomain.Scope) identityDomain.Principal {
		t.Helper()
		corr := id.MustParse("01994f64-6c00-7000-8000-" + inviteSuffix)
		_, token, err := identity.InviteMember(ctx, identityAPI.MembershipCommand{
			Actor: owner, TenantID: tenant, Email: email, Roles: roles, Scopes: scopes, CorrelationID: corr,
		})
		if err != nil {
			t.Fatalf("invite %s: %v", email, err)
		}
		member := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-" + principalSuffix), VerifiedEmail: email}
		if _, err := identity.AcceptInvitation(ctx, member, token, corr); err != nil {
			t.Fatalf("accept %s: %v", email, err)
		}
		return member
	}
	integrator := invite("action-integration@example.org", "00000000a205", "00000000a206", []identityDomain.Role{identityDomain.IntegrationAdmin}, []identityDomain.Scope{identityDomain.Integrations})
	configurer := invite("action-configuration@example.org", "00000000a207", "00000000a208", []identityDomain.Role{identityDomain.Configurator}, []identityDomain.Scope{identityDomain.Configuration})
	worker := invite("action-operator@example.org", "00000000a209", "00000000a20a", []identityDomain.Role{identityDomain.Operator}, []identityDomain.Scope{identityDomain.Operations})
	auditor := invite("action-audit@example.org", "00000000a20b", "00000000a20c", []identityDomain.Role{identityDomain.ExpertQA}, []identityDomain.Scope{identityDomain.Review, identityDomain.Audit})
	// The foreign world: the owner of ANOTHER tenant reading this tenant
	// (the cross-tenant 404 row) and a principal with no membership at
	// all (the stranger row).
	foreignIdentity, foreignOwner := newService("00000000a211", "00000000a212", "00000000a213")
	createTenant(t, foreignIdentity, foreignOwner, "Чужой каталог", "action-matrix-foreign-1", "00000000a214")
	stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000a215")}

	packs := packresolver.New()
	caps := capabilityresolver.New()
	fixtureDefs := []actionDomain.ActionDefinition{
		actionMatrixSMS("notify.order-ready", "1.1.0"),
		actionMatrixSMS("notify.order-ready", "1.0.0"),
		actionMatrixTrunkStatus(),
	}
	live := actionApp.New(identity, packs, caps, fixtureDefs)
	// The production-shaped service: the binary dictionary of the product
	// (honestly empty today, AG-4) over the same live identity and
	// resolvers — the control branch of the matrix walks the REAL binary.
	production := actionApp.New(identity, packs, caps, actionCatalog.All())

	// Row 1 — the production binary carries its FIRST product entry
	// (story 4.6, step-01): the owner reads the product dictionary and
	// receives exactly one entry — create_abcp_customer_order@1.0.0. The
	// catalog stopped being honestly empty because the 4-1 admission
	// registry finally holds a capability-bearing declaration the entry
	// resolves against (abcp-parts@1.1.0/command.prepare-client-order).
	t.Run("row-1 production binary: the first product entry for the owner", func(t *testing.T) {
		defs, err := production.List(ctx, owner, tenant)
		if err != nil {
			t.Fatalf("the first product entry is a state, not an error, got %v", err)
		}
		if len(defs) != 1 {
			t.Fatalf("the production binary carries exactly one product entry (4-6), got %d", len(defs))
		}
		if defs[0].ID != "create_abcp_customer_order" || defs[0].Version != "1.0.0" {
			t.Fatalf("the first product entry is create_abcp_customer_order@1.0.0, got %s@%s", defs[0].ID, defs[0].Version)
		}
		if defs[0].PackRef.PackID != "auto-parts" || defs[0].PackRef.PackVersion != "6.0.0" {
			t.Fatalf("the first product entry pins auto-parts@6.0.0, got %s@%s", defs[0].PackRef.PackID, defs[0].PackRef.PackVersion)
		}
		if defs[0].RequiredCapability.ConnectorID != "abcp-parts" || defs[0].RequiredCapability.CapabilityID != "command.prepare-client-order" {
			t.Fatalf("the first product entry pins abcp-parts@1.1.0/command.prepare-client-order")
		}
	})

	// Row 2 — the admission pass over the production binary walks the
	// FIRST product entry for real (no more vacuous green): the guard
	// integrity plus the LIVE reference resolution — auto-parts@6.0.0 is
	// published (packresolver) and abcp-parts@1.1.0/command.prepare-client-order
	// resolves Complete with Idempotent=true (capabilityresolver), so the
	// provider_idempotent class passes the delivery-word cross-check of
	// AC-3 against the real connector catalog. The OQ-3 fence probe: no
	// supplier-send field exists in either typed schema.
	t.Run("row-2 production binary: the first entry passes the live admission", func(t *testing.T) {
		if err := production.ValidateBinaryCatalog(); err != nil {
			t.Fatalf("the first product entry passes the full admission pass against the live catalogs, got %v", err)
		}
		facts, ok := caps.Capability("abcp-parts", "1.1.0", "command.prepare-client-order")
		if !ok {
			t.Fatal("abcp-parts@1.1.0/command.prepare-client-order resolves against the live connector catalog")
		}
		if !facts.Idempotent {
			t.Fatal("the declaration carries Idempotent=true — the provider_idempotent class is admitted")
		}
		if strings.TrimSpace(facts.Reconciliation) == "" {
			t.Fatal("the declaration carries the reconciliation word (OQ-3)")
		}
		if !packs.Get("auto-parts", "6.0.0") {
			t.Fatal("auto-parts@6.0.0 is published in the live pack catalog")
		}
		def, found := actionCatalog.Get("create_abcp_customer_order", "1.0.0")
		if !found {
			t.Fatal("the first product entry pin resolves")
		}
		for _, schema := range []struct {
			label  string
			fields []actionDomain.FieldSchema
		}{
			{"typed_input_schema", def.TypedInputSchema},
			{"typed_output_schema", def.TypedOutputSchema},
		} {
			for _, f := range schema.fields {
				joined := strings.ToLower(f.Name + " " + f.Label + " " + f.Description)
				for _, word := range []string{"supplier_form", "supplier_send", "send_to_supplier", "delivery_to_supplier", "ship_to_supplier"} {
					if strings.Contains(joined, word) {
						t.Fatalf("schema %s carries the supplier-send field %s — the OQ-3 fence is violated", schema.label, f.Name)
					}
				}
			}
		}
	})

	// Row 3 — the fixture dictionary serves the NEWEST version of every
	// ID only (the 1-8 convention): two ids in the list, the pinned old
	// 1.0.0 of notify.order-ready is ABSENT from the list (the newest is
	// proposed, the past stays pinned).
	t.Run("row-3 fixture list: the newest of every id, the old pin absent", func(t *testing.T) {
		defs, err := live.List(ctx, owner, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if len(defs) != 2 {
			t.Fatalf("two ids of the fixture dictionary, got %d", len(defs))
		}
		if defs[0].ID != "notify.order-ready" || defs[0].Version != "1.1.0" {
			t.Fatalf("the newest of the first id first, got %s@%s", defs[0].ID, defs[0].Version)
		}
		if defs[1].ID != "query.trunk-status" || defs[1].Version != "1.0.0" {
			t.Fatalf("the second id in dictionary order, got %s@%s", defs[1].ID, defs[1].Version)
		}
		for _, def := range defs {
			if def.ID == "notify.order-ready" && def.Version == "1.0.0" {
				t.Fatal("the list proposes the newest only — the old pin lives behind Get")
			}
		}
	})

	// Row 4 — the pin read of the OLD version serves the materially older
	// bytes (risk low, timeout 15000 — the newest is medium/30000): the
	// pinned past stays immutable and resolvable (AC-8 — the material for
	// the future Manifest/Operation/Receipt references).
	t.Run("row-4 pin read: the old 1.0.0 served byte-exact, AC-8", func(t *testing.T) {
		def, err := live.Get(ctx, owner, tenant, "notify.order-ready", "1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		if def.Version != "1.0.0" || def.Conditions.RiskClass != "low" || def.Conditions.TimeoutMS != 15000 {
			t.Fatalf("the pin serves the OLD material (low/15000), got %s risk=%s timeout=%d",
				def.Version, def.Conditions.RiskClass, def.Conditions.TimeoutMS)
		}
		if def.Name != "Отправка SMS клиенту" || def.OwningSystem != "erp" {
			t.Fatalf("the full old definition material, got %s/%s", def.Name, def.OwningSystem)
		}
	})

	// Row 5 — the unknown version of a known id is the honest 404: the
	// invented future does not exist.
	t.Run("row-5 unknown version pin: the honest 404", func(t *testing.T) {
		if _, err := live.Get(ctx, owner, tenant, "notify.order-ready", "9.9.9"); !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the invented version pin is the honest 404, got %v", err)
		}
	})

	// Row 6 — the unknown definition id is the honest 404 too (the pin
	// surface never invents material).
	t.Run("row-6 unknown definition pin: the honest 404", func(t *testing.T) {
		if _, err := live.Get(ctx, owner, tenant, "notify.unknown", "1.0.0"); !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the unknown id pin is the honest 404, got %v", err)
		}
	})

	// Row 7 — the zero actor is the honest 404 BEFORE any membership
	// roundtrip: no disclosure that the tenant has an action catalog.
	t.Run("row-7 zero actor: the honest 404 without a membership call", func(t *testing.T) {
		if _, err := live.List(ctx, identityDomain.Principal{}, tenant); !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the unparseable actor is the honest 404, got %v", err)
		}
	})

	// Row 8 — the stranger (no membership anywhere) is the honest 404:
	// membership FIRST, no catalog material leaves the service.
	t.Run("row-8 stranger: membership first, the honest 404", func(t *testing.T) {
		_, err := live.List(ctx, stranger, tenant)
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404, got %v", err)
		}
		if strings.Contains(err.Error(), "notify.order-ready") || strings.Contains(err.Error(), "Отправка SMS") {
			t.Fatalf("the 404 never carries catalog material: %s", err.Error())
		}
	})

	// Row 9 — the foreign member (the owner of another tenant) is the
	// honest 404 on THIS tenant: the cross-tenant boundary is closed by
	// the live membership truth.
	t.Run("row-9 foreign member: the cross-tenant 404", func(t *testing.T) {
		if _, err := live.List(ctx, foreignOwner, tenant); !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the foreign member reads nothing of this tenant, got %v", err)
		}
	})

	// Row 10 — the operator (operations only) is the 403 WITH the scope
	// words BEFORE any material: the words name the two scopes that DO
	// open the surface; the catalog material never travels with the
	// refusal.
	t.Run("row-10 operator: the 403 words before material", func(t *testing.T) {
		_, err := live.List(ctx, worker, tenant)
		if !errors.Is(err, actionDomain.ErrForbidden) {
			t.Fatalf("the operator without the scopes is the honest 403, got %v", err)
		}
		if !strings.Contains(err.Error(), "integrations") || !strings.Contains(err.Error(), "configuration") {
			t.Fatalf("the 403 speaks the words of the two admitting scopes, got %s", err.Error())
		}
		if strings.Contains(err.Error(), "notify.order-ready") || strings.Contains(err.Error(), "Отправка SMS") {
			t.Fatalf("the 403 never carries catalog material: %s", err.Error())
		}
	})

	// Row 11 — the audit member (review+audit) gets the SAME honest 403:
	// the setup material is not audit material (the decision-context
	// 2-14 precedent of the words-first refusal).
	t.Run("row-11 audit member: the same honest 403 words", func(t *testing.T) {
		_, err := live.List(ctx, auditor, tenant)
		if !errors.Is(err, actionDomain.ErrForbidden) {
			t.Fatalf("the audit member does not open the setup material, got %v", err)
		}
		if !strings.Contains(err.Error(), "integrations") || !strings.Contains(err.Error(), "configuration") {
			t.Fatalf("the audit refusal speaks the same scope words, got %s", err.Error())
		}
	})

	// Row 12 — the integration admin opens the list (the FIRST admitting
	// scope): the full newest-only material over the live membership.
	t.Run("row-12 integration admin: the list green", func(t *testing.T) {
		defs, err := live.List(ctx, integrator, tenant)
		if err != nil {
			t.Fatalf("integrations opens the catalog read, got %v", err)
		}
		if len(defs) != 2 || defs[0].Version != "1.1.0" {
			t.Fatalf("the admitting member reads the same newest-only list, got %d", len(defs))
		}
	})

	// Row 13 — the configurator opens the list too (the SECOND honest way
	// in — the app tries integrations first, configuration second).
	t.Run("row-13 configurator: the second honest way in", func(t *testing.T) {
		defs, err := live.List(ctx, configurer, tenant)
		if err != nil {
			t.Fatalf("configuration opens the catalog read, got %v", err)
		}
		if len(defs) != 2 {
			t.Fatalf("the configurator reads the same dictionary, got %d", len(defs))
		}
	})

	// Row 14 — the pin read is gated by the SAME scopes: the
	// configurator pins the old version green (the gate covers both
	// surfaces, never only the list).
	t.Run("row-14 configurator pin read: green under the same gate", func(t *testing.T) {
		def, err := live.Get(ctx, configurer, tenant, "notify.order-ready", "1.0.0")
		if err != nil {
			t.Fatalf("the pin read is open to the admitting member, got %v", err)
		}
		if def.Conditions.RiskClass != "low" {
			t.Fatalf("the pinned old material is served, got %+v", def.Conditions)
		}
	})

	// Row 15 — the 403 wrap identity: the module sentinel IS the error
	// (the transport maps it) and the message is the exact app wrap —
	// the single source of the words pinned by the mirror.
	t.Run("row-15 the 403 wrap: the sentinel and the exact words", func(t *testing.T) {
		_, err := live.List(ctx, worker, tenant)
		if err == nil {
			t.Fatal("the operator refusal must carry the wrap")
		}
		if err.Error() != actionDomain.ErrForbidden.Error()+": "+actionScopeWords {
			t.Fatalf("the wrap is the sentinel plus the exact scope words, got %s", err.Error())
		}
	})

	// Row 16 — the secret discipline of the read: the pinned schema
	// DECLARES the secret field (Secret=true — the marking travels), and
	// the declaration carries no value material (FieldSchema has no value
	// slot — the type is the guarantee; the blind probe asserts the
	// marking and the declared shape only).
	t.Run("row-16 secret: the declaration marks, the value never exists", func(t *testing.T) {
		def, err := live.Get(ctx, owner, tenant, "notify.order-ready", "1.1.0")
		if err != nil {
			t.Fatal(err)
		}
		if len(def.TypedInputSchema) != 2 || !def.TypedInputSchema[1].Secret {
			t.Fatalf("the template_url field is declared secret, got %+v", def.TypedInputSchema)
		}
		if def.TypedInputSchema[1].Name != "template_url" || def.TypedInputSchema[1].Description != "Шаблон сообщения" {
			t.Fatalf("the declaration travels whole, got %+v", def.TypedInputSchema[1])
		}
	})

	// Row 17 — the defensive copies: the caller may mutate the returned
	// slice and the returned definition, never the dictionary (the next
	// read serves the original bytes).
	t.Run("row-17 defensive copies: the mutation never reaches the dictionary", func(t *testing.T) {
		defs, err := live.List(ctx, owner, tenant)
		if err != nil {
			t.Fatal(err)
		}
		defs[0].Name = "подделка"
		defs[0].Conditions.RiskClass = "critical"
		pinned, err := live.Get(ctx, owner, tenant, "notify.order-ready", "1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		pinned.Name = "подделка"
		fresh, err := live.List(ctx, owner, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if fresh[0].Name != "Отправка SMS клиенту" || fresh[0].Conditions.RiskClass != "medium" {
			t.Fatalf("the dictionary is immutable to the caller mutations, got %s/%s", fresh[0].Name, fresh[0].Conditions.RiskClass)
		}
		again, err := live.Get(ctx, owner, tenant, "notify.order-ready", "1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		if again.Name != "Отправка SMS клиенту" {
			t.Fatalf("the pinned read returns a copy, got %s", again.Name)
		}
	})

	// Row 18 — the dictionary is member-independent: the integrator and
	// the configurator receive the SAME material (the compile-time
	// dictionary never varies per member — availability computation is a
	// future slice, never this surface).
	t.Run("row-18 the same dictionary for every admitting member", func(t *testing.T) {
		byIntegrator, err := live.List(ctx, integrator, tenant)
		if err != nil {
			t.Fatal(err)
		}
		byConfigurer, err := live.List(ctx, configurer, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%+v", byIntegrator) != fmt.Sprintf("%+v", byConfigurer) {
			t.Fatal("the served dictionary is identical for every admitting member")
		}
	})

	// Row 19 — the fixture dictionary passes the FULL admission pass over
	// the LIVE composition-root resolvers: the guard integrity plus the
	// reference resolution with the delivery-word cross-check of AC-3
	// (send-sms declares Idempotent=true ↔ provider_idempotent;
	// trunk-status declares the reconciliation ↔ query_reconcilable).
	t.Run("row-19 fixture admission: green over the live resolvers", func(t *testing.T) {
		if err := actionDomain.CatalogIntegrity(fixtureDefs); err != nil {
			t.Fatalf("the fixture dictionary is integral, got %v", err)
		}
		for _, def := range fixtureDefs {
			if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
				t.Fatalf("%s@%s must resolve over the live catalogs, got %v", def.ID, def.Version, err)
			}
		}
	})

	// Row 20 — the dishonest delivery cross-check refused LIVE: a
	// provider_idempotent definition leaning on query.trunk-status (the
	// connector declares Idempotent=false) fails the reference walk — the
	// classification is never self-consistent, it is CHECKED against the
	// provider contract (AC-3).
	t.Run("row-20 dishonest idempotency: the live cross-check refuses", func(t *testing.T) {
		dishonest := actionMatrixSMS("notify.dishonest", "1.0.0")
		dishonest.RequiredCapability.CapabilityID = "query.trunk-status"
		if err := actionDomain.GuardValidateActionDefinition(dishonest); err != nil {
			t.Fatalf("the structure stays guard-valid (the cross-check is the refs truth), got %v", err)
		}
		if err := actionDomain.ValidateDefinitionRefs(dishonest, packs, caps); err == nil {
			t.Fatal("provider_idempotent over the Idempotent=false declaration must be refused")
		}
	})

	// Row 21 — the unresolvable pack reference refused fail-closed: the
	// unknown pack version never resolves, the definition is not
	// admissible — an honest cause, never a guess.
	t.Run("row-21 orphan pack ref: fail-closed refusal", func(t *testing.T) {
		orphan := actionMatrixSMS("notify.orphan-pack", "1.0.0")
		orphan.PackRef.PackVersion = "9.9.9"
		if err := actionDomain.ValidateDefinitionRefs(orphan, packs, caps); err == nil {
			t.Fatal("the unresolvable pack reference must be refused fail-closed")
		}
	})
}
