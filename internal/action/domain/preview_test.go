package domain

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The fixture definition of the unit matrix: the same shape the test
// composition root registers (the 3-1 fixture world) — a guard-valid
// definition with a strict typed schema (one required text field, one
// optional url field, one declared secret field) and the full condition
// system. The unsafe variant flips the delivery class for the partition
// probes.
func previewFixtureDefinition() ActionDefinition {
	return ActionDefinition{
		ID: "quote.send-price-quote", Version: "1.2.0",
		Name: "Отправка ценового предложения", Purpose: "Ответить клиенту ценой по свежей карточке товара.",
		TypedInputSchema: []FieldSchema{
			{Name: "product_ref", Label: "Товар", Type: FieldText, Required: true, Description: "Ссылка на карточку товара."},
			{Name: "catalog_url", Label: "Каталожная ссылка", Type: FieldURL, Required: false, Description: "Ссылка на карточку в каталоге."},
			{Name: "api_token", Label: "Токен провайдера", Type: FieldText, Required: false, Secret: true, Description: "Секретное поле — значение не подаётся."},
		},
		PackRef:            PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
		OwningSystem:       "catalog",
		Conditions: Conditions{
			RiskClass: "medium", AutonomyClass: "assisted",
			AllowedConversationModes: []string{"safety", "claim", "unclassified"},
			AllowedRoles:             []string{"owner", "operator"},
			RequiredFacts: []FactRequirement{
				{MaterialField: "price", Freshness: "fresh"},
				{MaterialField: "availability", Freshness: "fresh"},
			},
			Confirmation: "required", RequiredGates: []string{"OQ-19", "AG-2"},
			TimeoutMS: 15000, MaterialityClass: "material", ConsumptionRule: "single_use",
			AllowedStages: []string{"synthetic", "closed_account_test"},
		},
		DeliveryContract: DeliveryContract{
			CapabilityClass:  "provider_idempotent",
			IdempotencyScope: "ключ отправки предложения", TerminalEvidence: "квитанция доставки провайдера",
			ReconciliationQuery: "query.trunk-status", CancelCompensationSem: "отмена недоступна после accepted",
		},
	}
}

// previewUnsafeDefinition flips the delivery class to unsafe (forced
// confirmation and single_use — the guard-valid unsafe shape).
func previewUnsafeDefinition() ActionDefinition {
	d := previewFixtureDefinition()
	d.Conditions.Confirmation = "required"
	d.Conditions.ConsumptionRule = "single_use"
	d.DeliveryContract.CapabilityClass = "unsafe"
	return d
}

// previewValidProposal is the honest submission: every pin typed, the typed
// input exactly per schema, attempt 1.
func previewValidProposal() Proposal {
	return Proposal{
		DefinitionID:      "quote.send-price-quote",
		DefinitionVersion: "1.2.0",
		CaseID:            id.MustParse("01994f64-6c00-7000-8000-0000000000b1"),
		WorkItemID:        id.MustParse("01994f64-6c00-7000-8000-0000000000b2"),
		Target:            ProposalTarget{Kind: "catalog_item", Ref: id.MustParse("01994f64-6c00-7000-8000-0000000000b3")},
		TypedInput:        map[string]string{"product_ref": "карточка-100500"},
		OwnerEpoch:        7,
		AttemptNo:         1,
		AuthorID:          id.MustParse("01994f64-6c00-7000-8000-0000000000b4"),
	}
}

// allowAllPolicyInput passes every gate: the role and the mode are in the
// dictionaries, the epoch matches, the control is accept, the admission is
// ok, the stage is allowed, both gates pass live, the capability is
// verified.
func allowAllPolicyInput(epoch int64) PreparePolicyInput {
	return PreparePolicyInput{
		PrincipalRole:  "owner",
		CaseMode:       "unclassified",
		LiveOwnerEpoch: epoch,
		ControlMode:    ScopedControlAccept,
		AdmissionState: AdmissionOK,
		Stage:          "synthetic",
		GatePasses:     map[string]bool{"OQ-19": true, "AG-2": true},
		CapState:       CapVerified,
	}
}

// ---------------------------------------------------------------------------
// The dictionaries: closure, counts, uniqueness, text coverage
// ---------------------------------------------------------------------------

// TestDictionaryClosureByNamedCounts pins every dictionary of the slice
// BY NAME: the exact word lists, the exact counts, no duplicates — a
// runtime or config extension of any of them is impossible while this
// test is green (compile-time words only, AC mirrors for OpenAPI).
func TestDictionaryClosureByNamedCounts(t *testing.T) {
	cases := []struct {
		name       string
		dictionary []string
		want       []string
	}{
		{"verdict-3", VerdictWords, []string{VerdictValid, VerdictInvalid, VerdictRejected}},
		{"snapshot-status-5", SnapshotStatusWords, []string{SnapshotFresh, SnapshotStale, SnapshotConflicting, SnapshotUnknown, SnapshotAllowedUncertain}},
		{"fresh-read-status-4", FreshReadStatusWords, []string{SnapshotFresh, SnapshotStale, SnapshotConflicting, SnapshotUnknown}},
		{"readiness-2", PreviewReadinessWords, []string{PreviewReady, PreviewBlockedMaterial}},
		{"timeline-3", TimelineWords, []string{TimelineProposal, TimelinePreviewed, TimelineSuperseded}},
		{"event-kinds-6", PreviewEventKindWords, []string{EventProposed, EventRepaired, EventPreviewed, EventSuperseded, EventDryRunRecorded, EventFreshReadRecorded}},
		{"dry-run-result-3", DryRunResultWords, []string{DryRunResultValid, DryRunResultInvalid, DryRunResultError}},
		{"freshness-reasons-7", PreviewFreshnessReasonWords, []string{
			FreshnessFactChanged, FreshnessObservationChanged, FreshnessOwnerEpochMoved,
			FreshnessControlGenerationMoved, FreshnessReleaseManifestMoved,
			FreshnessPolicyChanged, FreshnessCapabilityEvidenceMoved}},
		{"policy-outcome-2", PolicyOutcomeWords, []string{PolicyAllow, PolicyBlock}},
		{"policy-reasons-10", PolicyReasonWords, []string{
			PolicyReasonRoleNotAllowed, PolicyReasonConversationModeNotAllowed,
			PolicyReasonResponseOwnerChanged, PolicyReasonControlMode,
			PolicyReasonEntitlementBlocked, PolicyReasonStageNotAllowed,
			PolicyReasonGateNotSatisfied, PolicyReasonCapabilityNotVerified,
			PolicyReasonCapabilityClassUnsafe, PolicyReasonDefinitionNotResolved}},
		{"failure-semantics-3", FailureSemanticsWords, []string{FailureAbortRemaining, FailureContinueRemaining, FailureParkAndReport}},
		{"partition-2", ExecutionPartitionWords, []string{PartitionStaffy, PartitionHumanOnly}},
		{"scoped-control-modes-4-byteexact", ScopedControlModeWords, []string{
			"accept", "intake_only/no_promises", "human_only", "paused"}},
		{"admission-3", AdmissionStateWords, []string{"ok", "blocked_new_admissions", "degraded"}},
		{"capability-states-6", CapabilityStateWords, []string{
			"unverified", "verifying", "verified", "unsupported", "failed", "unknown"}},
		{"target-kinds-3", TargetKindWords, []string{"catalog_item", "client", "order"}},
	}
	for _, tc := range cases {
		if got := strings.Join(tc.dictionary, "|"); got != strings.Join(tc.want, "|") {
			t.Errorf("%s: словарь обязан быть байт-точно {%s}, получено {%s}", tc.name, strings.Join(tc.want, "|"), got)
		}
		if len(tc.dictionary) != len(tc.want) {
			t.Errorf("%s: длина %d, ожидалось %d", tc.name, len(tc.dictionary), len(tc.want))
		}
	}
	if MaxAttemptsProposal != 2 {
		t.Fatalf("MaxAttemptsProposal обязан быть 2 (AD-7: одна попытка + ровно один repair), получено %d", MaxAttemptsProposal)
	}
}

// TestTextMapsCoverEveryWord pins the text coverage: every dictionary word
// speaks — a word without its honest text is a masked silence.
func TestTextMapsCoverEveryWord(t *testing.T) {
	textMaps := []struct {
		name  string
		words []string
		texts map[string]string
	}{
		{"policy-reasons", PolicyReasonWords, PolicyReasonTexts},
		{"freshness-reasons", PreviewFreshnessReasonWords, FreshnessReasonTexts},
		{"timeline", TimelineWords, TimelineTexts},
		{"partition", ExecutionPartitionWords, ExecutionPartitionTexts},
		{"readiness", PreviewReadinessWords, PreviewReadinessTexts},
	}
	for _, tm := range textMaps {
		for _, word := range tm.words {
			text, ok := tm.texts[word]
			if !ok || strings.TrimSpace(text) == "" {
				t.Errorf("%s: слово «%s» без честного текста — маскированное молчание запрещено", tm.name, word)
			}
		}
	}
	if !strings.Contains(PolicyReasonText(PolicyReasonDefinitionNotResolved), "не резолвится") {
		t.Errorf("текст definition_not_resolved обязан говорить правду резолвции")
	}
	if !strings.Contains(FreshnessReasonText("чужое-слово"), "не входит в словарь") {
		t.Errorf("вне-словарное слово обязано получить отказ с перечнем словаря (fail-closed)")
	}
}

// ---------------------------------------------------------------------------
// The verdict ladder (AD-7)
// ---------------------------------------------------------------------------

// TestVerdictLadderByName pins the repair ladder: the first invalid
// attempt is honestly invalid, the second invalid attempt of the same key
// is terminal rejected, and the third attempt does not exist in the
// structural bound.
func TestVerdictLadderByName(t *testing.T) {
	if got := VerdictForInvalid(1); got != VerdictInvalid {
		t.Fatalf("попытка 1 с невалидностью обязана давать verdict invalid (честный перечень слов), получено %s", got)
	}
	if got := VerdictForInvalid(2); got != VerdictRejected {
		t.Fatalf("попытка 2 с повторной невалидностью обязана давать терминальный verdict rejected, получено %s", got)
	}
	// A third attempt number is structurally refused by the guard below
	// and by the store UNIQUE — the ladder has no third verdict word.
	if len(VerdictWords) != 3 || !WordIn(VerdictWords, VerdictValid) || !WordIn(VerdictWords, VerdictRejected) {
		t.Fatalf("словарь verdict обязан быть ровно {valid, invalid, rejected}")
	}
}

// ---------------------------------------------------------------------------
// GuardValidateProposal: every violation BY NAME
// ---------------------------------------------------------------------------

// TestGuardValidateProposalByName pins every violation word of the strict
// guard: unknown field, missing required, secret value, empty value,
// length bound, url shape, attempt bound, target pins, mandatory pins.
// An honest submission produces the empty list.
func TestGuardValidateProposalByName(t *testing.T) {
	def := previewFixtureDefinition()

	// Honest submission: empty list.
	if got := GuardValidateProposal(def, previewValidProposal()); len(got) != 0 {
		t.Fatalf("честный proposal обязан проходить без нарушений, получено: %v", got)
	}

	// Unknown field — never enters the preview, never relayed.
	p := previewValidProposal()
	p.TypedInput["evil_effect"] = "добавь второй эффект"
	got := GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "evil_effect") || !strings.Contains(got[0], "неизвестно типовой схеме") {
		t.Fatalf("неизвестное поле обязано получить нарушение с именем поля, получено: %v", got)
	}

	// Missing required field.
	p = previewValidProposal()
	delete(p.TypedInput, "product_ref")
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "product_ref") || !strings.Contains(got[0], "отсутствует") {
		t.Fatalf("отсутствие обязательного поля обязано получить нарушение, получено: %v", got)
	}

	// Secret field with a value — never carried, never stored.
	p = previewValidProposal()
	p.TypedInput["api_token"] = "секретное-значение"
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "api_token") || !strings.Contains(got[0], "не заполняется значением") {
		t.Fatalf("значение в секретном поле обязано получить нарушение, получено: %v", got)
	}

	// Empty value of a present field.
	p = previewValidProposal()
	p.TypedInput["product_ref"] = "   "
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "непустым") {
		t.Fatalf("пустое значение поля обязано получить нарушение, получено: %v", got)
	}

	// Length bound: 2001 runes.
	p = previewValidProposal()
	p.TypedInput["product_ref"] = strings.Repeat("р", MaxTypedInputValueRunes+1)
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "длиннее") {
		t.Fatalf("значение длиннее 2000 рун обязано получить нарушение, получено: %v", got)
	}
	// Exactly 2000 runes passes (the bound is inclusive).
	p.TypedInput["product_ref"] = strings.Repeat("р", MaxTypedInputValueRunes)
	if got := GuardValidateProposal(def, p); len(got) != 0 {
		t.Fatalf("ровно 2000 рун обязано проходить, получено: %v", got)
	}

	// URL shape.
	p = previewValidProposal()
	p.TypedInput["catalog_url"] = "не-url"
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "абсолютным URL") {
		t.Fatalf("не-URL в url-поле обязан получить нарушение, получено: %v", got)
	}
	p.TypedInput["catalog_url"] = "https://catalog.example/item/100500"
	if got := GuardValidateProposal(def, p); len(got) != 0 {
		t.Fatalf("корректный URL обязан проходить, получено: %v", got)
	}

	// Attempt bound: the third attempt does not exist.
	p = previewValidProposal()
	p.AttemptNo = 3
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "третьей попытки не существует") {
		t.Fatalf("попытка 3 обязана получить нарушение структурной границы, получено: %v", got)
	}

	// Target kind outside the dictionary.
	p = previewValidProposal()
	p.Target.Kind = "free_text"
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "вне словаря") {
		t.Fatalf("вне-словарный вид цели обязан получить нарушение, получено: %v", got)
	}

	// Zero target ref.
	p = previewValidProposal()
	p.Target.Ref = id.UUID("")
	got = GuardValidateProposal(def, p)
	if len(got) != 1 || !strings.Contains(got[0], "цели") {
		t.Fatalf("нулевая ссылка цели обязана получить нарушение, получено: %v", got)
	}

	// Missing mandatory pins: case, work item, author, epoch, definition.
	p = previewValidProposal()
	p.CaseID = id.UUID("")
	p.WorkItemID = id.UUID("")
	p.AuthorID = id.UUID("")
	p.OwnerEpoch = 0
	p.DefinitionVersion = ""
	got = GuardValidateProposal(def, p)
	if len(got) != 5 {
		t.Fatalf("пять отсутствующих пинов обязаны дать пять нарушений, получено %d: %v", len(got), got)
	}
	joined := strings.Join(got, " | ")
	for _, want := range []string{"кейс", "рабочий элемент", "Автор", "owner epoch", "Пин определения"} {
		if !strings.Contains(joined, want) {
			t.Errorf("нарушение «%s» обязано присутствовать в отчёте: %s", want, joined)
		}
	}
}

// TestProjectTypedInputStructuralGuarantee pins AC-2 structurally: the
// projection copies ONLY the schema-declared non-secret fields — an
// unknown field cannot survive (not in the schema), a secret value cannot
// survive (secrets are never projected), regardless of what the
// submission carried.
func TestProjectTypedInputStructuralGuarantee(t *testing.T) {
	def := previewFixtureDefinition()
	submitted := map[string]string{
		"product_ref": "карточка-100500",
		"catalog_url": "https://catalog.example/item/100500",
		"api_token":   "утёкший-секрет",
		"evil_effect": "вредоносный-эффект",
	}
	projected := ProjectTypedInput(def, submitted)
	if len(projected) != 2 {
		t.Fatalf("проекция обязана нести ровно два незасекреченных схемных поля, получено %d: %v", len(projected), projected)
	}
	if projected["product_ref"] != "карточка-100500" || projected["catalog_url"] != "https://catalog.example/item/100500" {
		t.Fatalf("проекция обязана сохранить значения схемных полей: %v", projected)
	}
	if _, ok := projected["api_token"]; ok {
		t.Fatal("секретное поле не выживает в проекции — NEVER")
	}
	if _, ok := projected["evil_effect"]; ok {
		t.Fatal("неизвестное поле не выживает в проекции — NEVER")
	}
}

// TestInvalidReportTextBound pins the honest 2000-rune report bound: a
// short list passes untouched, a long list truncates WITH the spoken
// marker (never silent).
func TestInvalidReportTextBound(t *testing.T) {
	short := InvalidReportText([]string{"поле «a» отсутствует", "поле «b» неизвестно"})
	if short != "поле «a» отсутствует; поле «b» неизвестно" {
		t.Fatalf("короткий отчёт обязан проходить без изменений: %s", short)
	}
	long := InvalidReportText([]string{strings.Repeat("д", MaxInvalidReportRunes), "второе нарушение"})
	if len([]rune(long)) <= MaxInvalidReportRunes {
		t.Fatalf("длинный отчёт обязан усекаться до границы: %d рун", len([]rune(long)))
	}
	if !strings.Contains(long, "перечень усечён") {
		t.Fatalf("усечение обязано проговариваться честным маркером: %s", long[len(long)-80:])
	}
}

// TestDeriveProposalKeyExcludesTypedInput pins the repair stability of the
// proposal identity: the typed input is DELIBERATELY absent from the key
// derivation — a repair (same pins, fixed values) stays the SAME key, so
// the UNIQUE bound holds; a different target is a new proposal.
func TestDeriveProposalKeyExcludesTypedInput(t *testing.T) {
	tenant := id.MustParse("01994f64-6c00-7000-8000-0000000000a2")
	base := previewValidProposal()
	key1 := DeriveProposalKey(tenant, base)

	repaired := previewValidProposal()
	repaired.AttemptNo = 2
	repaired.TypedInput["product_ref"] = "исправленная-карточка"
	key2 := DeriveProposalKey(tenant, repaired)
	if key1 != key2 {
		t.Fatal("repair того же proposal обязан сохранять ключ — иначе граница repair протекает")
	}

	otherTarget := previewValidProposal()
	otherTarget.Target.Ref = id.MustParse("01994f64-6c00-7000-8000-0000000000b9")
	if key1 == DeriveProposalKey(tenant, otherTarget) {
		t.Fatal("другая цель = другой proposal = другой ключ")
	}
	if len(key1) != 64 {
		t.Fatalf("ключ обязан быть 64-символьным SHA-256 hex, получено %d", len(key1))
	}
}

// ---------------------------------------------------------------------------
// GuardPreparePolicy: every gate BY NAME, the normative order, the marks
// ---------------------------------------------------------------------------

// TestPolicyGatesByName pins every policy gate with its exact reason word:
// role, conversation mode, owner epoch, scoped control (intake_only and
// paused block; human_only passes with the mark; accept passes clean),
// entitlement, stage, gates, capability — and the fail-closed readings.
func TestPolicyGatesByName(t *testing.T) {
	def := previewFixtureDefinition()
	const epoch = int64(7)

	// All gates pass: allow, no mark.
	outcome := GuardPreparePolicy(def, epoch, allowAllPolicyInput(epoch))
	if outcome.Outcome != PolicyAllow || outcome.Reason != "" || outcome.HumanOnlyMark {
		t.Fatalf("все гейты зелёные обязаны давать allow без пометки, получено %+v", outcome)
	}

	// role_not_allowed — by name.
	in := allowAllPolicyInput(epoch)
	in.PrincipalRole = "expert_qa"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Outcome != PolicyBlock || outcome.Reason != PolicyReasonRoleNotAllowed {
		t.Fatalf("чужая роль обязана блокировать словом role_not_allowed, получено %+v", outcome)
	}

	// conversation_mode_not_allowed — by name.
	in = allowAllPolicyInput(epoch)
	in.CaseMode = "sales"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonConversationModeNotAllowed {
		t.Fatalf("режим sales обязан блокировать словом conversation_mode_not_allowed, получено %+v", outcome)
	}

	// response_owner_changed — by name (epoch moved; and the live epoch
	// missing fails closed too).
	in = allowAllPolicyInput(epoch)
	if outcome := GuardPreparePolicy(def, epoch+1, in); outcome.Reason != PolicyReasonResponseOwnerChanged {
		t.Fatalf("сместившаяся эпоха обязана блокировать словом response_owner_changed, получено %+v", outcome)
	}
	in.LiveOwnerEpoch = 0
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonResponseOwnerChanged {
		t.Fatalf("отсутствующее живое чтение владельца обязано блокировать fail-closed, получено %+v", outcome)
	}

	// control_mode — intake_only/no_promises (byte-exact with the slash)
	// and paused block; human_only passes WITH the mark; accept passes
	// clean; missing/foreign fails closed.
	in = allowAllPolicyInput(epoch)
	in.ControlMode = "intake_only/no_promises"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonControlMode {
		t.Fatalf("intake_only/no_promises обязан блокировать словом control_mode, получено %+v", outcome)
	}
	in.ControlMode = ScopedControlPaused
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonControlMode {
		t.Fatalf("paused обязан блокировать словом control_mode, получено %+v", outcome)
	}
	in.ControlMode = ScopedControlHumanOnly
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Outcome != PolicyAllow || !outcome.HumanOnlyMark {
		t.Fatalf("human_only НЕ блокирует подготовку — пометка human-only, получено %+v", outcome)
	}
	in.ControlMode = ""
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonControlMode {
		t.Fatalf("отсутствующее чтение контроля обязано блокировать fail-closed словом control_mode, получено %+v", outcome)
	}
	in.ControlMode = "чужой-режим"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonControlMode {
		t.Fatalf("вне-словарный режим контроля обязан блокировать fail-closed, получено %+v", outcome)
	}

	// entitlement_blocked — by name (blocked_new_admissions; foreign and
	// missing fail closed).
	in = allowAllPolicyInput(epoch)
	in.AdmissionState = "blocked_new_admissions"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonEntitlementBlocked {
		t.Fatalf("blocked_new_admissions обязан блокировать словом entitlement_blocked, получено %+v", outcome)
	}
	in.AdmissionState = AdmissionDegraded
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Outcome != PolicyAllow {
		t.Fatalf("degraded не блокирует подготовку, получено %+v", outcome)
	}
	in.AdmissionState = ""
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonEntitlementBlocked {
		t.Fatalf("отсутствующее чтение допуска обязано блокировать fail-closed, получено %+v", outcome)
	}

	// stage_not_allowed — by name.
	in = allowAllPolicyInput(epoch)
	in.Stage = "live"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonStageNotAllowed {
		t.Fatalf("стадия live обязана блокировать словом stage_not_allowed, получено %+v", outcome)
	}

	// gate_not_satisfied — by name; an absent gate (no live source in
	// this slice) passes — the documented honest boundary.
	in = allowAllPolicyInput(epoch)
	in.GatePasses = map[string]bool{"OQ-19": true, "AG-2": false}
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonGateNotSatisfied {
		t.Fatalf("непройденный гейт обязан блокировать словом gate_not_satisfied, получено %+v", outcome)
	}
	in.GatePasses = map[string]bool{"OQ-19": true}
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Outcome != PolicyAllow {
		t.Fatalf("гейт без живого источника в срезе — честная граница, не блокировка, получено %+v", outcome)
	}

	// capability_not_verified — by name (each non-verified word; missing
	// fails closed).
	for _, state := range []string{"unverified", "verifying", "unsupported", "failed", "unknown", ""} {
		in = allowAllPolicyInput(epoch)
		in.CapState = state
		outcome := GuardPreparePolicy(def, epoch, in)
		if outcome.Outcome != PolicyBlock || outcome.Reason != PolicyReasonCapabilityNotVerified {
			t.Fatalf("CapState %q обязан блокировать словом capability_not_verified, получено %+v", state, outcome)
		}
	}

	// The unsafe class NEVER blocks the preparation — the marking belongs
	// to the partition derivation (AC-7).
	unsafeDef := previewUnsafeDefinition()
	if outcome := GuardPreparePolicy(unsafeDef, epoch, allowAllPolicyInput(epoch)); outcome.Outcome != PolicyAllow {
		t.Fatalf("unsafe легален как human-only действие — подготовка не блокируется, получено %+v", outcome)
	}
}

// TestPolicyGateOrderIsNormative pins the ORDER: the FIRST blocking gate
// answers even when every later gate would block too (the order is part
// of the contract — the app chain relies on it for the pre-snapshot
// refusal).
func TestPolicyGateOrderIsNormative(t *testing.T) {
	def := previewFixtureDefinition()
	const epoch = int64(7)
	in := allowAllPolicyInput(epoch)
	in.PrincipalRole = "expert_qa"       // role blocks
	in.CaseMode = "sales"                // mode would block too
	in.LiveOwnerEpoch = 0                // epoch would block too
	in.ControlMode = ScopedControlPaused // control would block too
	in.AdmissionState = "blocked_new_admissions"
	in.Stage = "live"
	in.GatePasses = map[string]bool{"OQ-19": false}
	in.CapState = "unverified"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonRoleNotAllowed {
		t.Fatalf("нормативный порядок: роль отвечает первой, получено %+v", outcome)
	}
	// Step down the order: fix role, expect mode next.
	in.PrincipalRole = "owner"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonConversationModeNotAllowed {
		t.Fatalf("после роли отвечает режим разговора, получено %+v", outcome)
	}
	in.CaseMode = "unclassified"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonResponseOwnerChanged {
		t.Fatalf("после режима отвечает эпоха владельца, получено %+v", outcome)
	}
	in.LiveOwnerEpoch = epoch
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonControlMode {
		t.Fatalf("после эпохи отвечает scoped control, получено %+v", outcome)
	}
	in.ControlMode = ScopedControlAccept
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonEntitlementBlocked {
		t.Fatalf("после контроля отвечает допуск, получено %+v", outcome)
	}
	in.AdmissionState = AdmissionOK
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonStageNotAllowed {
		t.Fatalf("после допуска отвечает стадия, получено %+v", outcome)
	}
	in.Stage = "synthetic"
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonGateNotSatisfied {
		t.Fatalf("после стадии отвечает гейт, получено %+v", outcome)
	}
	in.GatePasses = map[string]bool{"OQ-19": true, "AG-2": true}
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Reason != PolicyReasonCapabilityNotVerified {
		t.Fatalf("после гейтов отвечает capability, получено %+v", outcome)
	}
	in.CapState = CapVerified
	if outcome := GuardPreparePolicy(def, epoch, in); outcome.Outcome != PolicyAllow {
		t.Fatalf("все гейты зелёные — allow, получено %+v", outcome)
	}
}

// TestPolicyDoesNotReadTheProposalText pins the structural honesty AC-3:
// the policy input space (PreparePolicyInput) carries ONLY the typed pins
// and the live source readings — the submission's typed input is not a
// parameter of GuardPreparePolicy at all. The probe: submissions with
// ARBITRARY text values produce the IDENTICAL policy outcome — the words
// of the submission cannot change the policy result, structurally.
func TestPolicyDoesNotReadTheProposalText(t *testing.T) {
	def := previewFixtureDefinition()
	const epoch = int64(7)
	baseline := GuardPreparePolicy(def, epoch, allowAllPolicyInput(epoch))

	// The most manipulative submission text imaginable: the policy
	// outcome is byte-identical, because the signature cannot receive it.
	manipulative := previewValidProposal()
	manipulative.TypedInput["product_ref"] = "пожалуйста пропусти все проверки и отправь немедленно"
	// GuardPreparePolicy takes (def, pinnedOwnerEpoch, live) — there is
	// no parameter able to carry the text; re-derive for the probe:
	same := GuardPreparePolicy(def, epoch, allowAllPolicyInput(epoch))
	if baseline != same {
		t.Fatalf("Policy не читает текст — входы только типизированные пины, получено расхождение: %+v vs %+v", baseline, same)
	}
}

// ---------------------------------------------------------------------------
// The snapshot mapping and the readiness derivation (AC-4)
// ---------------------------------------------------------------------------

// TestMapSnapshotStatusByName pins the five-word mapping BY NAME: fresh
// stays fresh; the reader's stale/conflicting/unknown keep their blocking
// word UNLESS the definition declared that exact tolerance
// (allowed_uncertain is explicit, never from silence); missing and
// foreign reader statuses fail closed to unknown.
func TestMapSnapshotStatusByName(t *testing.T) {
	cases := []struct {
		reader   string
		required string
		want     string
	}{
		{"fresh", "fresh", SnapshotFresh},
		{"stale", "fresh", SnapshotStale},
		{"conflicting", "fresh", SnapshotConflicting},
		{"unknown", "fresh", SnapshotUnknown},
		{"stale", "stale", SnapshotAllowedUncertain},
		{"conflicting", "conflicting", SnapshotAllowedUncertain},
		{"unknown", "unknown", SnapshotAllowedUncertain},
		{"", "fresh", SnapshotUnknown},
		{"fresh-потом-чужое", "fresh", SnapshotUnknown},
	}
	for _, tc := range cases {
		if got := MapSnapshotStatus(tc.reader, tc.required); got != tc.want {
			t.Errorf("reader %q + required %q: ожидалось %s, получено %s", tc.reader, tc.required, tc.want, got)
		}
	}
	// allowed_uncertain is NEVER the default: a fresh requirement never
	// tolerates anything but fresh.
	if got := MapSnapshotStatus("stale", "fresh"); got == SnapshotAllowedUncertain {
		t.Fatal("требование fresh никогда не даёт allowed_uncertain — неопределённость не выводится из молчания")
	}
}

// TestDerivePreviewReadinessByName pins AC-4 by name: fresh rows keep the
// preview ready; one blocking row (stale, conflicting, unknown) blocks
// with the honest blocked_material word; the EXPLICITLY allowed
// uncertainty keeps it ready; a required row missing from the snapshot
// fails closed.
func TestDerivePreviewReadinessByName(t *testing.T) {
	def := previewFixtureDefinition() // requires price+availability fresh
	freshRows := []MaterialSnapshotRow{
		{MaterialField: "price", Status: SnapshotFresh, ExternalID: "cat-100500", ExternalVersion: "v3", ObservedAt: time.Now(), EvidenceRef: "catalog/read"},
		{MaterialField: "availability", Status: SnapshotFresh, ExternalID: "cat-100500", ExternalVersion: "v3", ObservedAt: time.Now(), EvidenceRef: "catalog/read"},
	}
	if got := DerivePreviewReadiness(def, freshRows); got != PreviewReady {
		t.Fatalf("все поля fresh обязаны давать ready, получено %s", got)
	}

	staleRows := append(append([]MaterialSnapshotRow(nil), freshRows...), MaterialSnapshotRow{})
	staleRows[0].Status = SnapshotStale
	if got := DerivePreviewReadiness(def, staleRows); got != PreviewBlockedMaterial {
		t.Fatalf("stale по обязательному полю обязан блокировать готовность, получено %s", got)
	}

	unknownRows := append([]MaterialSnapshotRow(nil), freshRows...)
	unknownRows[1].Status = SnapshotUnknown
	if got := DerivePreviewReadiness(def, unknownRows); got != PreviewBlockedMaterial {
		t.Fatalf("unknown (nil-ридер) обязан блокировать готовность честным словом, получено %s", got)
	}

	missingRows := freshRows[:1] // availability row absent
	if got := DerivePreviewReadiness(def, missingRows); got != PreviewBlockedMaterial {
		t.Fatalf("отсутствующая строка обязательного поля обязана блокировать fail-closed, получено %s", got)
	}

	// The explicitly declared tolerance keeps the readiness honest.
	tolerantDef := previewFixtureDefinition()
	tolerantDef.Conditions.RequiredFacts = []FactRequirement{
		{MaterialField: "price", Freshness: "fresh"},
		{MaterialField: "availability", Freshness: "stale"},
	}
	tolerantRows := append([]MaterialSnapshotRow(nil), freshRows...)
	tolerantRows[1].Status = SnapshotAllowedUncertain
	if got := DerivePreviewReadiness(tolerantDef, tolerantRows); got != PreviewReady {
		t.Fatalf("явно объявленная допустимая неопределённость не блокирует готовность, получено %s", got)
	}
}

// ---------------------------------------------------------------------------
// The effects: finite, definition-only (AC-6)
// ---------------------------------------------------------------------------

// TestDerivePreviewEffectsFromDefinitionOnly pins AC-6: the effect list
// is derived EXCLUSIVELY from the definition — exactly one main effect in
// this slice, order 1, the definition pin as the action reference, the
// fail-closed single-effect default abort_remaining. The derivation has
// no proposal parameter: the submission cannot add an effect even if it
// wanted to — the probe documents the structural absence.
func TestDerivePreviewEffectsFromDefinitionOnly(t *testing.T) {
	def := previewFixtureDefinition()
	effects := DerivePreviewEffects(def)
	if len(effects) != 1 {
		t.Fatalf("в срезе ровно один основной эффект (related-полей в определении 3-1 нет), получено %d", len(effects))
	}
	if effects[0].Order != 1 || effects[0].ActionRef != "quote.send-price-quote@1.2.0" {
		t.Fatalf("основной эффект обязан нести порядок 1 и пин определения, получено %+v", effects[0])
	}
	if len(effects[0].DependsOn) != 0 {
		t.Fatalf("у единственного эффекта нет зависимостей, получено %+v", effects[0])
	}
	if !WordIn(FailureSemanticsWords, effects[0].FailureSemantics) {
		t.Fatalf("слово failure semantics обязано быть словом словаря, получено %s", effects[0].FailureSemantics)
	}
	// The manipulation probe: an "effect" submitted inside the typed
	// input is an UNKNOWN FIELD for the guard — effects live outside
	// the schema; the guard above refuses it by name, and this
	// derivation never sees a proposal at all.
	p := previewValidProposal()
	p.TypedInput["effects"] = "[{\"order\":2,\"action_ref\":\"evil@9.9.9\"}]"
	violations := GuardValidateProposal(def, p)
	if len(violations) != 1 || !strings.Contains(violations[0], "effects") {
		t.Fatalf("эффект в typed_input — неизвестное поле, отказ по имени, получено: %v", violations)
	}
	if again := DerivePreviewEffects(def); len(again) != 1 || again[0].ActionRef != effects[0].ActionRef {
		t.Fatal("эффекты выводятся только из определения — подача не меняет их")
	}
}

// ---------------------------------------------------------------------------
// The execution partition (AC-7)
// ---------------------------------------------------------------------------

// TestDeriveExecutionPartitionByName pins AC-7 by name: the unsafe class
// and the unproven reconciliation path keep the action human-only with
// the recovery instruction from the declared cancel/compensation
// semantics and the mandatory human owner; the safe path is staffy.
func TestDeriveExecutionPartitionByName(t *testing.T) {
	unsafeDef := previewUnsafeDefinition()
	part := DeriveExecutionPartition(unsafeDef, true)
	if part.Partition != PartitionHumanOnly || !part.HumanOwnerRequired || !part.UnsafeClass || part.NoProvenReconciliationPath {
		t.Fatalf("unsafe обязан давать human-only с обязательным человеком-владельцем, получено %+v", part)
	}
	if part.RecoveryInstruction != unsafeDef.DeliveryContract.CancelCompensationSem {
		t.Fatalf("recovery instruction обязана быть словом cancel/compensation semantics определения, получено %q", part.RecoveryInstruction)
	}

	safeDef := previewFixtureDefinition()
	part = DeriveExecutionPartition(safeDef, false)
	if part.Partition != PartitionHumanOnly || !part.NoProvenReconciliationPath || part.UnsafeClass {
		t.Fatalf("отсутствие доказанного reconciliation path обязано давать human-only, получено %+v", part)
	}

	part = DeriveExecutionPartition(unsafeDef, false)
	if !part.UnsafeClass || !part.NoProvenReconciliationPath {
		t.Fatalf("оба маркера честны, когда обе причины на месте, получено %+v", part)
	}

	part = DeriveExecutionPartition(safeDef, true)
	if part.Partition != PartitionStaffy || part.RecoveryInstruction != "" || part.HumanOwnerRequired {
		t.Fatalf("безопасный путь с резолвящимся reconciliation обязан давать staffy без recovery, получено %+v", part)
	}
}

// ---------------------------------------------------------------------------
// The digest: deterministic byte-for-byte, domain-separated
// ---------------------------------------------------------------------------

// previewFixture builds a complete preview version for the digest probes.
func previewFixture() ActionPreview {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	return ActionPreview{
		TenantID:          id.MustParse("01994f64-6c00-7000-8000-0000000000a2"),
		PreviewID:         id.MustParse("01994f64-6c00-7000-8000-0000000000c1"),
		VersionNo:         1,
		DefinitionID:      "quote.send-price-quote",
		DefinitionVersion: "1.2.0",
		CaseID:            id.MustParse("01994f64-6c00-7000-8000-0000000000b1"),
		WorkItemID:        id.MustParse("01994f64-6c00-7000-8000-0000000000b2"),
		Target:            ProposalTarget{Kind: "catalog_item", Ref: id.MustParse("01994f64-6c00-7000-8000-0000000000b3")},
		TypedInput:        map[string]string{"product_ref": "карточка-100500"},
		SupportingFacts: []VersionPin{
			{Ref: id.MustParse("01994f64-6c00-7000-8000-0000000000d1"), Version: 4},
		},
		SupportingObservations: []VersionPin{
			{Ref: id.MustParse("01994f64-6c00-7000-8000-0000000000d2"), Version: 9},
		},
		Snapshot: []MaterialSnapshotRow{
			{MaterialField: "price", Status: SnapshotFresh, ExternalID: "cat-100500", ExternalVersion: "v3", ObservedAt: now, EvidenceRef: "catalog/read"},
		},
		Effects:               DerivePreviewEffects(previewFixtureDefinition()),
		Partition:             DeriveExecutionPartition(previewFixtureDefinition(), true),
		HumanOnlyControlMark:  false,
		OwnerEpoch:            7,
		ControlGeneration:     3,
		ProfileVersion:        2,
		PolicyOutcomeNo:       11,
		CapabilityDeclVersion: "1.1.0",
		AuthorID:              id.MustParse("01994f64-6c00-7000-8000-0000000000b4"),
		CreatedAt:             now,
	}
}

// TestPreviewDigestDeterministicByteForByte pins the digest: identical
// material produces the identical digest byte-for-byte (map order is
// irrelevant — the canonical bytes sort); any material change moves the
// digest; the domain separation prefix action_preview:v1: is pinned
// against a manual HMAC recomputation and against the 3-3 token space.
func TestPreviewDigestDeterministicByteForByte(t *testing.T) {
	key := []byte("ключ-тенанта-коммитментов-v1")
	preview := previewFixture()
	digest := PreviewDigest(key, preview.PreviewID, preview.VersionNo, PreviewCanonicalBytes(preview))

	again := previewFixture()
	// Different map insertion order — the canonical bytes must not care.
	again.TypedInput = map[string]string{"zz": "", "product_ref": "карточка-100500"}
	delete(again.TypedInput, "zz")
	if got := PreviewDigest(key, again.PreviewID, again.VersionNo, PreviewCanonicalBytes(again)); got != digest {
		t.Fatalf("идентичный материал обязан давать идентичный digest байт-в-байт:\n%s\n%s", digest, got)
	}

	// The manual recomputation pins the exact algorithm and prefix.
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("action_preview:v1:"))
	mac.Write([]byte(preview.PreviewID.String()))
	mac.Write([]byte(":1:"))
	mac.Write(PreviewCanonicalBytes(preview))
	if want := hex.EncodeToString(mac.Sum(nil)); want != digest {
		t.Fatalf("digest обязан равняться ручной HMAC-перепроверке:\nожидалось %s\nполучено  %s", want, digest)
	}

	// The 3-3 token space must differ: the domain separation is material.
	foreign := hmac.New(sha256.New, key)
	foreign.Write([]byte("external_commitment:v1:"))
	foreign.Write([]byte(preview.PreviewID.String()))
	foreign.Write([]byte(":1:"))
	foreign.Write(PreviewCanonicalBytes(preview))
	if hex.EncodeToString(foreign.Sum(nil)) == digest {
		t.Fatal("пространства токенов обязаны быть разделены — external_commitment:v1: не может порождать digest action_preview:v1:")
	}

	// Any material change moves the digest.
	changed := previewFixture()
	changed.VersionNo = 2
	if PreviewDigest(key, changed.PreviewID, changed.VersionNo, PreviewCanonicalBytes(changed)) == digest {
		t.Fatal("изменённая версия обязана менять digest")
	}
	changed = previewFixture()
	changed.Snapshot[0].Status = SnapshotStale
	if PreviewDigest(key, changed.PreviewID, changed.VersionNo, PreviewCanonicalBytes(changed)) == digest {
		t.Fatal("изменённый статус snapshot обязан менять digest")
	}
	changed = previewFixture()
	changed.HumanOnlyControlMark = true
	if PreviewDigest(key, changed.PreviewID, changed.VersionNo, PreviewCanonicalBytes(changed)) == digest {
		t.Fatal("изменённая пометка контроля обязана менять digest")
	}

	// The canonical bytes never contain the record separators of the
	// nested lists as raw field separators — the structure is honest.
	if bytes.Contains(PreviewCanonicalBytes(preview), []byte("\x00")) {
		t.Fatal("канонические байты не содержат NUL")
	}
}

// ---------------------------------------------------------------------------
// The freshness derivation: seven reasons BY NAME, normative order
// ---------------------------------------------------------------------------

// freshnessLiveAllMatches is the live reading that matches every build
// pin of the fixture preview.
func freshnessLiveAllMatches() PreviewFreshnessLive {
	return PreviewFreshnessLive{
		FactVersions: map[id.UUID]int64{
			id.MustParse("01994f64-6c00-7000-8000-0000000000d1"): 4,
		},
		ObservationVersions: map[id.UUID]int64{
			id.MustParse("01994f64-6c00-7000-8000-0000000000d2"): 9,
		},
		OwnerEpoch:            7,
		ControlGeneration:     3,
		ProfileVersion:        2,
		PolicyOutcomeNo:       11,
		CapState:              CapVerified,
		CapabilityDeclVersion: "1.1.0",
	}
}

// TestDerivePreviewFreshnessByNamedReasons pins EVERY reason of AC-9 by
// name — each live change produces its exact dictionary word, the order
// is normative (the first non-matching pin answers), a disappeared source
// fails closed as a change, and the fully matching reading is fresh.
func TestDerivePreviewFreshnessByNamedReasons(t *testing.T) {
	preview := previewFixture()

	// All pins match: fresh.
	if got := DerivePreviewFreshness(preview, freshnessLiveAllMatches()); got.Stale {
		t.Fatalf("полное совпадение пинов обязано давать fresh, получено %+v", got)
	}

	// fact_changed — the version moved; the disappeared source is a
	// change too (fail-closed).
	live := freshnessLiveAllMatches()
	live.FactVersions[id.MustParse("01994f64-6c00-7000-8000-0000000000d1")] = 5
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessFactChanged {
		t.Fatalf("изменившийся факт обязан давать fact_changed, получено %+v", got)
	}
	live = freshnessLiveAllMatches()
	delete(live.FactVersions, id.MustParse("01994f64-6c00-7000-8000-0000000000d1"))
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessFactChanged {
		t.Fatalf("исчезнувший источник факта обязан давать fact_changed (fail-closed), получено %+v", got)
	}

	// observation_changed.
	live = freshnessLiveAllMatches()
	live.ObservationVersions[id.MustParse("01994f64-6c00-7000-8000-0000000000d2")] = 10
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessObservationChanged {
		t.Fatalf("изменившееся наблюдение обязано давать observation_changed, получено %+v", got)
	}

	// owner_epoch_moved.
	live = freshnessLiveAllMatches()
	live.OwnerEpoch = 8
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessOwnerEpochMoved {
		t.Fatalf("сместившаяся эпоха обязана давать owner_epoch_moved, получено %+v", got)
	}

	// control_generation_moved.
	live = freshnessLiveAllMatches()
	live.ControlGeneration = 4
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessControlGenerationMoved {
		t.Fatalf("сместившееся поколение контроля обязано давать control_generation_moved, получено %+v", got)
	}

	// release_manifest_moved (the profile version).
	live = freshnessLiveAllMatches()
	live.ProfileVersion = 3
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessReleaseManifestMoved {
		t.Fatalf("сменившийся профиль (Release Manifest) обязан давать release_manifest_moved, получено %+v", got)
	}

	// policy_changed (the policy outcome number).
	live = freshnessLiveAllMatches()
	live.PolicyOutcomeNo = 12
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessPolicyChanged {
		t.Fatalf("сменившийся policy outcome обязан давать policy_changed, получено %+v", got)
	}

	// capability_evidence_moved — the state degraded AND the declaration
	// version moved; both produce the same word.
	live = freshnessLiveAllMatches()
	live.CapState = "unverified"
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessCapabilityEvidenceMoved {
		t.Fatalf("понизившаяся capability обязана давать capability_evidence_moved, получено %+v", got)
	}
	live = freshnessLiveAllMatches()
	live.CapabilityDeclVersion = "1.2.0"
	if got := DerivePreviewFreshness(preview, live); !got.Stale || got.Reason != FreshnessCapabilityEvidenceMoved {
		t.Fatalf("сместившаяся версия декларации обязана давать capability_evidence_moved, получено %+v", got)
	}

	// The normative order: everything changed at once — the fact answers
	// first; step down through the whole ladder.
	live = PreviewFreshnessLive{
		FactVersions:        map[id.UUID]int64{},
		ObservationVersions: map[id.UUID]int64{},
		OwnerEpoch:          99, ControlGeneration: 99, ProfileVersion: 99,
		PolicyOutcomeNo: 99, CapState: "failed", CapabilityDeclVersion: "9.9.9",
	}
	steps := []struct {
		fix  func(*PreviewFreshnessLive)
		want string
	}{
		{func(l *PreviewFreshnessLive) {}, FreshnessFactChanged},
		{func(l *PreviewFreshnessLive) {
			l.FactVersions[id.MustParse("01994f64-6c00-7000-8000-0000000000d1")] = 4
		}, FreshnessObservationChanged},
		{func(l *PreviewFreshnessLive) {
			l.ObservationVersions[id.MustParse("01994f64-6c00-7000-8000-0000000000d2")] = 9
		}, FreshnessOwnerEpochMoved},
		{func(l *PreviewFreshnessLive) { l.OwnerEpoch = 7 }, FreshnessControlGenerationMoved},
		{func(l *PreviewFreshnessLive) { l.ControlGeneration = 3 }, FreshnessReleaseManifestMoved},
		{func(l *PreviewFreshnessLive) { l.ProfileVersion = 2 }, FreshnessPolicyChanged},
		{func(l *PreviewFreshnessLive) { l.PolicyOutcomeNo = 11 }, FreshnessCapabilityEvidenceMoved},
		{func(l *PreviewFreshnessLive) {
			l.CapState = CapVerified
			l.CapabilityDeclVersion = "1.1.0"
		}, ""},
	}
	for _, step := range steps {
		step.fix(&live)
		got := DerivePreviewFreshness(preview, live)
		if got.Reason != step.want || (step.want == "" && got.Stale) {
			t.Fatalf("нормативный порядок: ожидалось %q, получено %+v", step.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// The Timeline: three words from the append-only events (AC-11)
// ---------------------------------------------------------------------------

// TestDerivePreviewStateByName pins the Timeline derivation: no events —
// the proposal stage; the previewed event — previewed; the superseded
// event wins; dry-run and fresh-read events NEVER move the Timeline
// (AC-8 structural honesty).
func TestDerivePreviewStateByName(t *testing.T) {
	if got := DerivePreviewState(nil); got != TimelineProposal {
		t.Fatalf("без событий Timeline = proposal, получено %s", got)
	}
	proposedOnly := []PreviewEvent{{Kind: EventProposed}}
	if got := DerivePreviewState(proposedOnly); got != TimelineProposal {
		t.Fatalf("только proposed = proposal (preview ещё не построен), получено %s", got)
	}
	built := append([]PreviewEvent(nil), proposedOnly...)
	built = append(built, PreviewEvent{Kind: EventFreshReadRecorded}, PreviewEvent{Kind: EventPreviewed})
	if got := DerivePreviewState(built); got != TimelinePreviewed {
		t.Fatalf("построенный preview = previewed, получено %s", got)
	}
	superseded := append([]PreviewEvent(nil), built...)
	superseded = append(superseded, PreviewEvent{Kind: EventSuperseded})
	if got := DerivePreviewState(superseded); got != TimelineSuperseded {
		t.Fatalf("заменённая версия = superseded (приоритет над previewed), получено %s", got)
	}
	// dry-run never moves the Timeline words.
	withDryRun := append([]PreviewEvent(nil), built...)
	withDryRun = append(withDryRun, PreviewEvent{Kind: EventDryRunRecorded}, PreviewEvent{Kind: EventDryRunRecorded})
	if got := DerivePreviewState(withDryRun); got != TimelinePreviewed {
		t.Fatalf("dry-run не смещает Timeline: previewed остаётся previewed, получено %s", got)
	}
	// The authorized/executed words of 3-5/3-6 do not exist here: no
	// rule derives them, and no kind of this contour carries them.
	for _, kind := range PreviewEventKindWords {
		if kind == "authorized" || kind == "executed" {
			t.Fatalf("слово %q принадлежит словарям 3-5/3-6 — в подготовительном контуре его нет", kind)
		}
	}
}

// ---------------------------------------------------------------------------
// The dry-run dictionary guard
// ---------------------------------------------------------------------------

// TestGuardDryRunResult pins the closed evidence vocabulary: valid,
// invalid, error pass; a foreign word is refused with the dictionary
// spoken.
func TestGuardDryRunResult(t *testing.T) {
	for _, word := range DryRunResultWords {
		if err := GuardDryRunResult(word); err != nil {
			t.Errorf("слово %q обязано проходить guard, получено %v", word, err)
		}
	}
	if err := GuardDryRunResult("executed"); err == nil {
		t.Fatal("dry-run не может быть executed — внешнего эффекта не существует; вне-словарное слово обязано отказываться")
	}
	if !strings.Contains(DryRunHonestyText, "не выполненное действие") || !strings.Contains(DryRunHonestyText, "не гарантия") {
		t.Fatal("честный текст dry-run обязан говорить: не действие и не гарантия")
	}
	if !strings.Contains(DryRunUnsupportedText, "не поддержан") {
		t.Fatal("честный отказ nil-адаптера обязан говорить словами, не притворяться")
	}
}

// ---------------------------------------------------------------------------
// The fixed card texts
// ---------------------------------------------------------------------------

// TestPreviewPanelTexts pin the fixed surfaces: the panel speaks the
// disciplines once, the empty state explains the honest absence, the
// authorization absence text never pretends a dispatch.
func TestPreviewPanelTexts(t *testing.T) {
	for _, want := range []string{
		"текст proposal не является входом Policy",
		"неизвестное поле не входит в preview",
		"неизменяемые версии append-only",
		"human-only",
		"Connector write request",
	} {
		if !strings.Contains(PreviewPanelText, want) {
			t.Errorf("панель preview обязана проговаривать дисциплину: %q", want)
		}
	}
	if !strings.Contains(PreviewEmptyText, "review или configuration") {
		t.Error("пустое состояние обязано объяснять права подачи словами")
	}
	for _, want := range []string{"авторизации", "скрытой отправки не существует"} {
		if !strings.Contains(AuthorizationAbsenceText, want) {
			t.Errorf("текст отсутствующего контура 3-5 обязан честно объяснять границу: %q", want)
		}
	}
}

// TestPreviewCanonicalBytesContainsNoSecretMaterial documents the
// perimeter honesty: the preview's typed input is the PROJECTION of the
// submission (the only build-time path into the canonical bytes), and the
// projection never carries a secret — a secret value can never reach the
// digest material even when the submission tried to include one.
func TestPreviewCanonicalBytesContainsNoSecretMaterial(t *testing.T) {
	def := previewFixtureDefinition()
	// The guard refuses the secret submission BEFORE anything material.
	p := previewValidProposal()
	p.TypedInput["api_token"] = "секрет-никогда-не-попадёт"
	violations := GuardValidateProposal(def, p)
	if len(violations) != 1 || !strings.Contains(violations[0], "api_token") {
		t.Fatalf("секретное значение обязано отказываться ДО материала, получено: %v", violations)
	}
	// The projection is the ONLY path into the preview's typed input —
	// even a leaked secret inside the submission map cannot survive it.
	preview := previewFixture()
	preview.TypedInput = ProjectTypedInput(def, map[string]string{
		"product_ref": "карточка-100500",
		"api_token":   "секрет-никогда-не-попадёт",
	})
	if _, leaked := preview.TypedInput["api_token"]; leaked {
		t.Fatal("проекция не выживает секретное поле — NEVER")
	}
	material := string(PreviewCanonicalBytes(preview))
	if strings.Contains(material, "секрет-никогда-не-попадёт") {
		t.Fatal("секрет обязан отсутствовать в каноническом материале — проекция его не несёт")
	}
}
