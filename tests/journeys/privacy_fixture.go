package main

import (
	"context"
	"sync"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	privacyAPI "github.com/artemiosu/staffy/internal/privacy/api"
	privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
)

// privacyFixture is the in-memory data perimeter for the journey server: the
// full lifecycle (map entry → policy → deletion → receipt) works honestly so
// the browser journeys exercise the real page flow.
type privacyFixture struct {
	mu       sync.Mutex
	entries  map[privacyDomain.ArtifactClass][]privacyDomain.DataMapEntry
	policies map[privacyDomain.ArtifactClass][]privacyDomain.RetentionPolicy
	holds    []privacyDomain.LegalHoldEvent
	requests map[string]privacyDomain.Receipt
}

var fixtureNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func (f *privacyFixture) DataView(_ context.Context, _ domain.Principal, _ id.UUID) (privacyAPI.DataViewResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	classes := make([]privacyAPI.ClassView, 0, len(privacyDomain.ClassSpecs()))
	now := fixtureNow
	holdStates := privacyDomain.DeriveHolds(f.holds, now)
	for _, spec := range privacyDomain.ClassSpecs() {
		view := privacyAPI.ClassView{Spec: spec, Status: "undocumented", Hold: privacyDomain.HoldState{Status: privacyDomain.HoldNone}}
		if versions := f.entries[spec.ID]; len(versions) > 0 {
			entry := versions[len(versions)-1]
			view.Entry = &entry
			view.Status = entry.Status()
			view.Missing = entry.MissingAttributes()
		}
		if versions := f.policies[spec.ID]; len(versions) > 0 {
			policy := versions[len(versions)-1]
			view.Policy = &policy
			anchor := policy.CreatedAt
			due := policy.DeletionDueAt(anchor)
			mandatory := policy.MandatoryDueAt(anchor)
			view.ReferenceTime = &anchor
			view.DeletionDueAt = &due
			view.MandatoryDueAt = &mandatory
		}
		if hold, ok := holdStates[spec.ID]; ok {
			view.Hold = hold
		}
		classes = append(classes, view)
	}
	receipts := make([]privacyDomain.Receipt, 0, len(f.requests))
	for _, receipt := range f.requests {
		receipts = append(receipts, receipt)
	}
	return privacyAPI.DataViewResult{Classes: classes, Receipts: receipts, HoldEvents: f.holds}, nil
}

func (f *privacyFixture) ReadReceipt(_ context.Context, _ domain.Principal, _ id.UUID, requestID id.UUID) (privacyAPI.DeletionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, receipt := range f.requests {
		if receipt.RequestID == requestID.String() {
			return privacyAPI.DeletionResult{Receipt: receipt}, nil
		}
	}
	return privacyAPI.DeletionResult{}, privacyDomain.ErrNotFound
}

func (f *privacyFixture) PublishMapEntry(_ context.Context, c privacyAPI.MapEntryCommand) (privacyDomain.DataMapEntry, error) {
	if _, err := privacyDomain.LookupClass(c.Class); err != nil {
		return privacyDomain.DataMapEntry{}, privacyDomain.RuleError{Rule: "class", Reason: "Класс артефактов не найден в системном словаре."}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.entries == nil {
		f.entries = map[privacyDomain.ArtifactClass][]privacyDomain.DataMapEntry{}
	}
	entry := privacyDomain.DataMapEntry{
		Class: c.Class, Version: int64(len(f.entries[c.Class]) + 1),
		Purpose: c.Purpose, LegalBasis: c.LegalBasis, Controller: c.Controller, Processor: c.Processor,
		Recipients: c.Recipients, SubProcessors: c.SubProcessors, Locations: c.Locations,
		AccessibleRoles: c.AccessibleRoles, KnownCopies: c.KnownCopies,
		RetentionPolicy: c.PolicyRef, DeletionMethod: c.DeletionMethod, ExportMethod: c.ExportMethod,
		ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: fixtureNow,
	}
	f.entries[c.Class] = append(f.entries[c.Class], entry)
	return entry, nil
}

func (f *privacyFixture) PublishPolicy(_ context.Context, c privacyAPI.PolicyCommand) (privacyDomain.RetentionPolicy, error) {
	if c.Class == "" {
		return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "class", Reason: "Политика хранения привязывается строго к одному классу артефактов: blanket-политика без класса отклоняется."}
	}
	if _, err := privacyDomain.LookupClass(c.Class); err != nil {
		return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "class", Reason: "Класс артефактов не найден в системном словаре."}
	}
	policy := privacyDomain.RetentionPolicy{
		Class: c.Class, Version: int64(len(f.policies[c.Class]) + 1),
		Purpose: c.Purpose, LegalBasis: c.LegalBasis, Source: c.Source, TerminalEvent: c.TerminalEvent,
		ActiveUseDays: c.ActiveUseDays, ArchiveDays: c.ArchiveDays, DeletionDays: c.DeletionDays,
		BackupExpiryDays: c.BackupExpiryDays, LegalHoldException: c.LegalHoldException, OwnerNote: c.OwnerNote,
		ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: fixtureNow,
	}
	if !policy.HonorsVoicePreviewInvariant() {
		return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "active_use_days", Reason: "Политика предпрослушиваний не может обещать срок дольше 24-часового security-инварианта очистки."}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.policies == nil {
		f.policies = map[privacyDomain.ArtifactClass][]privacyDomain.RetentionPolicy{}
	}
	f.policies[c.Class] = append(f.policies[c.Class], policy)
	return policy, nil
}

func (f *privacyFixture) ActivateHold(_ context.Context, c privacyAPI.HoldCommand) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.holds = append(f.holds, privacyDomain.LegalHoldEvent{
		EventID: "01994f64-6c00-7000-8000-000000000003", Event: "activated", Classes: c.Classes,
		Basis: c.Basis, Approver: c.Approver, ReviewDeadline: c.ReviewDeadline,
		ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: fixtureNow,
	})
	return nil
}

func (f *privacyFixture) ReleaseHold(_ context.Context, c privacyAPI.HoldCommand) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.holds = append(f.holds, privacyDomain.LegalHoldEvent{
		EventID: "01994f64-6c00-7000-8000-000000000003", Event: "released", Classes: c.Classes,
		Basis: c.Basis, Approver: c.Approver, ReviewDeadline: fixtureNow,
		ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: fixtureNow,
	})
	return nil
}

func (f *privacyFixture) RunDeletion(_ context.Context, c privacyAPI.DeletionCommand) (privacyAPI.DeletionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(c.Classes) == 0 {
		return privacyAPI.DeletionResult{}, privacyDomain.RuleError{Rule: "classes", Reason: "Укажите хотя бы один класс артефактов для удаления."}
	}
	classes := map[privacyDomain.ArtifactClass]bool{}
	for _, class := range c.Classes {
		classes[class] = true
	}
	// Fail-closed: complete map entry and current policy are required.
	for class := range classes {
		versions := f.entries[class]
		if len(versions) == 0 {
			return privacyAPI.DeletionResult{}, privacyDomain.RuleError{Rule: "map", Reason: "Класс " + string(class) + " не описан в карте данных: заполните карту перед удалением."}
		}
		if versions[len(versions)-1].Status() != privacyDomain.MapStatusComplete {
			return privacyAPI.DeletionResult{}, privacyDomain.RuleError{Rule: "map", Reason: "Запись класса " + string(class) + " в карте данных неполная: заполните обязательные атрибуты."}
		}
		if len(f.policies[class]) == 0 {
			return privacyAPI.DeletionResult{}, privacyDomain.RuleError{Rule: "policy", Reason: "Класс " + string(class) + " не имеет действующей retention-политики: опубликуйте политику перед удалением."}
		}
	}
	holds := privacyDomain.DeriveHolds(f.holds, fixtureNow)
	outcomes := []privacyDomain.DeletionOutcome{}
	pins := []privacyDomain.PolicyPin{}
	ordered := privacyDomain.AllClasses()
	for _, class := range ordered {
		if !classes[class] {
			continue
		}
		spec, _ := privacyDomain.LookupClass(class)
		policy := f.policies[class][len(f.policies[class])-1]
		pins = append(pins, privacyDomain.PolicyPin{Class: class, Version: policy.Version, TerminalEvent: policy.TerminalEvent})
		switch {
		case holds[class].Status == privacyDomain.HoldActive || holds[class].Status == privacyDomain.HoldRequiresDecision:
			outcomes = append(outcomes, privacyDomain.DeletionOutcome{Class: class, Outcome: privacyDomain.OutcomeRetainedByLaw, Note: "Юридическое удержание: " + holds[class].Basis + "."})
		case spec.Block != nil:
			outcomes = append(outcomes, privacyDomain.DeletionOutcome{Class: class, Outcome: privacyDomain.OutcomeUnsupported, Note: "Удаление заблокировано: " + spec.Block.Required + ". Ответственный: " + spec.Block.Owner + ".", Block: spec.Block})
		case spec.Executor != "":
			outcomes = append(outcomes, privacyDomain.DeletionOutcome{Class: class, Outcome: privacyDomain.OutcomeDeleted, AffectedCount: 2, Note: "Удалено локальных копий: 2. Содержимое в квитанцию не входит."})
		default:
			outcomes = append(outcomes, privacyDomain.DeletionOutcome{Class: class, Outcome: privacyDomain.OutcomeUnknown, Note: "Результат неопределён: копия класса не имеет локального исполнителя в текущем срезе."})
		}
	}
	receipt := privacyDomain.Receipt{
		TenantID: "01994f64-6c00-7000-8000-000000000002", ReceiptID: "01994f64-6c00-7000-8000-000000000004",
		RequestID: "01994f64-6c00-7000-8000-000000000005", ReceiptHash: "fixture-receipt-hash-0000000000000000000000",
		Authority: "data_governance", PolicyVersions: pins, Outcomes: outcomes,
		CompletedAt: fixtureNow, ActorID: "01994f64-6c00-7000-8000-000000000001",
	}
	if f.requests == nil {
		f.requests = map[string]privacyDomain.Receipt{}
	}
	f.requests[c.IdempotencyKey] = receipt
	return privacyAPI.DeletionResult{Receipt: receipt}, nil
}
