package app

import (
        "context"
        "crypto/sha256"
        "encoding/hex"
        "fmt"
        "sort"
        "strings"
        "time"

        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        "github.com/artemiosu/staffy/internal/privacy/api"
        privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
        "github.com/artemiosu/staffy/internal/privacy/port"
)

// unknownReconciliationWindow bounds how long an unknown outcome may stay
// unresolved before a human must reconcile it.
const unknownReconciliationWindow = 72 * time.Hour

// Service implements the privacy application logic. Viewing the data map
// requires the data_governance or audit scope; every mutation requires
// data_governance. Authorization failures surface as ErrNotFound so tenant
// existence is never disclosed across tenants.
type Service struct {
        repository port.Repository
        identity   identityAPI.MembershipService
        executors  map[string]port.ClassDeletionExecutor
        clock      api.Clock
        ids        api.IDGenerator
}

// New wires the privacy orchestrator with the owning-module executors.
func New(repository port.Repository, identity identityAPI.MembershipService, executors map[string]port.ClassDeletionExecutor, clock api.Clock, ids api.IDGenerator) *Service {
        return &Service{repository: repository, identity: identity, executors: executors, clock: clock, ids: ids}
}

func (s *Service) tenancy(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        if !actor.ID.Valid() || !tenantID.Valid() {
                return tenancy.Context{}, privacyDomain.ErrNotFound
        }
        return tenancy.New(actor.ID, tenantID)
}

// viewScope authorizes read access: data_governance or audit. The second
// return reports whether the actor may also manage the perimeter.
func (s *Service) viewScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, bool, error) {
        scope, err := s.tenancy(ctx, actor, tenantID)
        if err != nil {
                return tenancy.Context{}, false, err
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.DataGovernance); err == nil {
                return scope, true, nil
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Audit); err == nil {
                return scope, false, nil
        }
        return tenancy.Context{}, false, privacyDomain.ErrNotFound
}

func (s *Service) manageScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        scope, err := s.tenancy(ctx, actor, tenantID)
        if err != nil {
                return tenancy.Context{}, err
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.DataGovernance); err != nil {
                return tenancy.Context{}, privacyDomain.ErrNotFound
        }
        return scope, nil
}

// DataView composes the derived per-class projection of the whole perimeter:
// current map entries with honest statuses, current policies, hold states,
// due positions and the latest receipts.
func (s *Service) DataView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (api.DataViewResult, error) {
        scope, canManage, err := s.viewScope(ctx, actor, tenantID)
        if err != nil {
                return api.DataViewResult{}, err
        }
        entries, err := s.repository.ListMapEntries(ctx, scope)
        if err != nil {
                return api.DataViewResult{}, err
        }
        policies, err := s.repository.ListPolicies(ctx, scope)
        if err != nil {
                return api.DataViewResult{}, err
        }
        events, err := s.repository.ListHoldEvents(ctx, scope)
        if err != nil {
                return api.DataViewResult{}, err
        }
        receipts, err := s.repository.ListReceipts(ctx, scope)
        if err != nil {
                return api.DataViewResult{}, err
        }
        now := s.clock.Now().UTC()
        currentEntries := latestMapEntries(entries)
        currentPolicies := latestPolicies(policies)
        holds := privacyDomain.DeriveHolds(events, now)

        classes := make([]api.ClassView, 0, len(privacyDomain.ClassSpecs()))
        for _, spec := range privacyDomain.ClassSpecs() {
                view := api.ClassView{Spec: spec, Status: "undocumented", Hold: privacyDomain.HoldState{Status: privacyDomain.HoldNone}}
                if entry, ok := currentEntries[spec.ID]; ok {
                        view.Entry = &entry
                        view.Status = entry.Status()
                        view.Missing = entry.MissingAttributes()
                }
                if policy, ok := currentPolicies[spec.ID]; ok {
                        view.Policy = &policy
                        anchor := policy.CreatedAt
                        if reference, err := s.repository.ClassReferenceTime(ctx, scope, spec.ID); err == nil && reference != nil {
                                view.ReferenceTime = reference
                                anchor = *reference
                        }
                        due := policy.DeletionDueAt(anchor)
                        mandatory := policy.MandatoryDueAt(anchor)
                        view.DeletionDueAt = &due
                        view.MandatoryDueAt = &mandatory
                }
                if hold, ok := holds[spec.ID]; ok {
                        view.Hold = hold
                }
                classes = append(classes, view)
        }
        _ = canManage
        return api.DataViewResult{Classes: classes, Receipts: receipts, HoldEvents: events}, nil
}

func latestMapEntries(entries []privacyDomain.DataMapEntry) map[privacyDomain.ArtifactClass]privacyDomain.DataMapEntry {
        out := map[privacyDomain.ArtifactClass]privacyDomain.DataMapEntry{}
        for _, entry := range entries {
                current, seen := out[entry.Class]
                if !seen || entry.Version > current.Version {
                        out[entry.Class] = entry
                }
        }
        return out
}

func latestPolicies(policies []privacyDomain.RetentionPolicy) map[privacyDomain.ArtifactClass]privacyDomain.RetentionPolicy {
        out := map[privacyDomain.ArtifactClass]privacyDomain.RetentionPolicy{}
        for _, policy := range policies {
                current, seen := out[policy.Class]
                if !seen || policy.Version > current.Version {
                        out[policy.Class] = policy
                }
        }
        return out
}

// PublishMapEntry appends the next honest version of one class entry.
// Unknown mandatory attributes are stored empty: the entry then derives as
// incomplete and blocks the class route instead of being guessed.
func (s *Service) PublishMapEntry(ctx context.Context, c api.MapEntryCommand) (privacyDomain.DataMapEntry, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return privacyDomain.DataMapEntry{}, err
        }
        if _, err := privacyDomain.LookupClass(c.Class); err != nil {
                return privacyDomain.DataMapEntry{}, privacyDomain.RuleError{Rule: "class", Reason: "Класс артефактов не найден в системном словаре."}
        }
        entry := privacyDomain.DataMapEntry{
                Class:           c.Class,
                Purpose:         strings.TrimSpace(c.Purpose),
                LegalBasis:      strings.TrimSpace(c.LegalBasis),
                Controller:      strings.TrimSpace(c.Controller),
                Processor:       strings.TrimSpace(c.Processor),
                Recipients:      cleanList(c.Recipients),
                SubProcessors:   cleanList(c.SubProcessors),
                Locations:       cleanList(c.Locations),
                AccessibleRoles: cleanList(c.AccessibleRoles),
                KnownCopies:     cleanList(c.KnownCopies),
                RetentionPolicy: strings.TrimSpace(c.PolicyRef),
                DeletionMethod:  strings.TrimSpace(c.DeletionMethod),
                ExportMethod:    strings.TrimSpace(c.ExportMethod),
        }
        for _, check := range []struct {
                field string
                value string
                max   int
        }{
                {"purpose", entry.Purpose, 600},
                {"legal_basis", entry.LegalBasis, 300},
                {"controller", entry.Controller, 300},
                {"processor", entry.Processor, 300},
                {"retention_policy", entry.RetentionPolicy, 300},
                {"deletion_method", entry.DeletionMethod, 600},
                {"export_method", entry.ExportMethod, 600},
        } {
                if len([]rune(check.value)) > check.max {
                        return privacyDomain.DataMapEntry{}, privacyDomain.RuleError{Rule: check.field, Reason: "Поле превышает допустимую длину."}
                }
        }
        for name, list := range map[string][]string{
                "recipients": entry.Recipients, "sub_processors": entry.SubProcessors,
                "locations": entry.Locations, "accessible_roles": entry.AccessibleRoles, "known_copies": entry.KnownCopies,
        } {
                if err := validateList(list, name); err != nil {
                        return privacyDomain.DataMapEntry{}, err
                }
        }
        return s.repository.SaveMapEntry(ctx, port.MapEntryParams{Scope: scope, Class: c.Class, Entry: entry, Now: s.clock.Now().UTC()})
}

func cleanList(values []string) []string {
        out := []string{}
        for _, value := range values {
                trimmed := strings.TrimSpace(value)
                if trimmed != "" {
                        out = append(out, trimmed)
                }
        }
        return out
}

func validateList(list []string, name string) error {
        if len(list) > 50 {
                return privacyDomain.RuleError{Rule: name, Reason: "Список слишком длинный (до 50 элементов)."}
        }
        for _, value := range list {
                if len([]rune(value)) > 300 {
                        return privacyDomain.RuleError{Rule: name, Reason: "Элемент списка превышает 300 символов."}
                }
        }
        return nil
}

// PublishPolicy appends the next retention policy version. A policy binds to
// exactly one artifact class: a blanket policy without a class is rejected
// with an honest cause.
func (s *Service) PublishPolicy(ctx context.Context, c api.PolicyCommand) (privacyDomain.RetentionPolicy, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return privacyDomain.RetentionPolicy{}, err
        }
        if c.Class == "" {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "class", Reason: "Политика хранения привязывается строго к одному классу артефактов: blanket-политика без класса отклоняется."}
        }
        if _, err := privacyDomain.LookupClass(c.Class); err != nil {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "class", Reason: "Класс артефактов не найден в системном словаре."}
        }
        policy := privacyDomain.RetentionPolicy{
                Class:              c.Class,
                Purpose:            strings.TrimSpace(c.Purpose),
                LegalBasis:         strings.TrimSpace(c.LegalBasis),
                Source:             strings.TrimSpace(c.Source),
                TerminalEvent:      strings.TrimSpace(c.TerminalEvent),
                ActiveUseDays:      c.ActiveUseDays,
                ArchiveDays:        c.ArchiveDays,
                DeletionDays:       c.DeletionDays,
                BackupExpiryDays:   c.BackupExpiryDays,
                LegalHoldException: strings.TrimSpace(c.LegalHoldException),
                OwnerNote:          strings.TrimSpace(c.OwnerNote),
        }
        if policy.Purpose == "" || len([]rune(policy.Purpose)) > 300 {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "purpose", Reason: "Укажите назначение политики (до 300 символов)."}
        }
        if policy.LegalBasis == "" || len([]rune(policy.LegalBasis)) > 300 {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "legal_basis", Reason: "Укажите правовое основание (до 300 символов)."}
        }
        if !privacyDomain.RetentionSources[policy.Source] {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "source", Reason: "Источник политики неизвестен."}
        }
        if !privacyDomain.TerminalEvents[policy.TerminalEvent] {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "terminal_event", Reason: "Терминальное событие не входит в словарь."}
        }
        for field, value := range map[string]int{
                "active_use_days": policy.ActiveUseDays, "archive_days": policy.ArchiveDays,
                "deletion_days": policy.DeletionDays, "backup_expiry_days": policy.BackupExpiryDays,
        } {
                if value < 0 || value > 3650 {
                        return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: field, Reason: "Срок задан в днях от 0 до 3650."}
                }
        }
        if policy.DeletionDays < policy.ActiveUseDays+policy.ArchiveDays {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "deletion_days", Reason: "Максимальный срок хранения не может быть короче суммы active use и archive."}
        }
        if !policy.HonorsVoicePreviewInvariant() {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "active_use_days", Reason: "Политика предпрослушиваний не может обещать срок дольше 24-часового security-инварианта очистки."}
        }
        if policy.LegalHoldException == "" || len([]rune(policy.LegalHoldException)) > 300 {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "legal_hold_exception", Reason: "Опишите исключение legal hold (до 300 символов)."}
        }
        if policy.OwnerNote == "" || len([]rune(policy.OwnerNote)) > 300 {
                return privacyDomain.RetentionPolicy{}, privacyDomain.RuleError{Rule: "owner_note", Reason: "Укажите владельца политики (до 300 символов)."}
        }
        return s.repository.SavePolicy(ctx, port.PolicyParams{Scope: scope, Class: c.Class, Policy: policy, Now: s.clock.Now().UTC()})
}

// ActivateHold appends a hold activation event over a set of classes.
func (s *Service) ActivateHold(ctx context.Context, c api.HoldCommand) error {
        return s.holdEvent(ctx, c, "activated")
}

// ReleaseHold appends a hold release event over a set of classes.
func (s *Service) ReleaseHold(ctx context.Context, c api.HoldCommand) error {
        return s.holdEvent(ctx, c, "released")
}

func (s *Service) holdEvent(ctx context.Context, c api.HoldCommand, kind string) error {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return err
        }
        if len(c.Classes) == 0 {
                return privacyDomain.RuleError{Rule: "classes", Reason: "Укажите хотя бы один класс артефактов."}
        }
        for _, class := range c.Classes {
                if _, err := privacyDomain.LookupClass(class); err != nil {
                        return privacyDomain.RuleError{Rule: "classes", Reason: "Класс артефактов не найден в системном словаре."}
                }
        }
        basis := strings.TrimSpace(c.Basis)
        approver := strings.TrimSpace(c.Approver)
        if basis == "" || len([]rune(basis)) > 600 {
                return privacyDomain.RuleError{Rule: "basis", Reason: "Укажите основание удержания (до 600 символов)."}
        }
        if approver == "" || len([]rune(approver)) > 300 {
                return privacyDomain.RuleError{Rule: "approver", Reason: "Укажите утвердившего (до 300 символов)."}
        }
        now := s.clock.Now().UTC()
        deadline := c.ReviewDeadline
        if kind == "activated" {
                if !deadline.After(now) {
                        return privacyDomain.RuleError{Rule: "review_deadline", Reason: "Срок пересмотра удержания должен быть в будущем."}
                }
        } else {
                deadline = now
        }
        eventID, err := s.ids.New(now)
        if err != nil {
                return err
        }
        return s.repository.SaveHoldEvent(ctx, port.HoldEventParams{
                Scope: scope,
                Event: privacyDomain.LegalHoldEvent{EventID: eventID.String(), Event: kind, Classes: c.Classes, Basis: basis, Approver: approver, ReviewDeadline: deadline},
                Now:   now,
        })
}

// RunDeletion executes the idempotent data-governance deletion workflow.
// Every class in the scope must have a complete map entry and a current
// retention policy (fail-closed), then the honest outcome dictionary decides:
// hold → retained_by_law, blocked class → unsupported, unexpired term →
// scheduled, executor class → executor result (or unknown on ambiguity),
// class without a local executor → unknown with a reconciliation owner.
func (s *Service) RunDeletion(ctx context.Context, c api.DeletionCommand) (api.DeletionResult, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return api.DeletionResult{}, err
        }
        key := strings.TrimSpace(c.IdempotencyKey)
        if runes := len([]rune(key)); runes < 8 || runes > 200 || !c.CorrelationID.Valid() {
                return api.DeletionResult{}, privacyDomain.ErrInvalidIdempotency
        }
        if len(c.Classes) == 0 {
                return api.DeletionResult{}, privacyDomain.RuleError{Rule: "classes", Reason: "Укажите хотя бы один класс артефактов для удаления."}
        }
        classes := dedupeClasses(c.Classes)
        for _, class := range classes {
                if _, err := privacyDomain.LookupClass(class); err != nil {
                        return api.DeletionResult{}, privacyDomain.RuleError{Rule: "classes", Reason: "Класс артефактов не найден в системном словаре."}
                }
        }
        hash := deletionRequestHash(scope.TenantID(), classes)
        // One read answers the idempotency question: a stored key with the
        // same hash replays the stored receipt; a different hash conflicts.
        priorRequest, priorOutcomes, priorReceipt, found, err := s.repository.LookupDeletion(ctx, scope, scope.PrincipalID(), key, hash)
        if err != nil {
                return api.DeletionResult{}, err
        }
        if found {
                priorReceipt.Outcomes = priorOutcomes
                _ = priorRequest
                return api.DeletionResult{Receipt: priorReceipt, Replayed: true}, nil
        }
        // Fail-closed gate: every requested class needs a complete map entry and
        // a current retention policy before anything is deleted.
        entries, err := s.repository.ListMapEntries(ctx, scope)
        if err != nil {
                return api.DeletionResult{}, err
        }
        policies, err := s.repository.ListPolicies(ctx, scope)
        if err != nil {
                return api.DeletionResult{}, err
        }
        currentEntries := latestMapEntries(entries)
        currentPolicies := latestPolicies(policies)
        for _, class := range classes {
                entry, ok := currentEntries[class]
                if !ok {
                        return api.DeletionResult{}, privacyDomain.RuleError{Rule: "map", Reason: "Класс " + string(class) + " не описан в карте данных: заполните карту перед удалением."}
                }
                if entry.Status() != privacyDomain.MapStatusComplete {
                        return api.DeletionResult{}, privacyDomain.RuleError{Rule: "map", Reason: "Запись класса " + string(class) + " в карте данных неполная (" + strings.Join(entry.MissingAttributes(), ", ") + "): заполните обязательные атрибуты."}
                }
                if _, ok := currentPolicies[class]; !ok {
                        return api.DeletionResult{}, privacyDomain.RuleError{Rule: "policy", Reason: "Класс " + string(class) + " не имеет действующей retention-политики: опубликуйте политику перед удалением."}
                }
        }
        events, err := s.repository.ListHoldEvents(ctx, scope)
        if err != nil {
                return api.DeletionResult{}, err
        }
        now := s.clock.Now().UTC()
        holds := privacyDomain.DeriveHolds(events, now)

        outcomes := make([]privacyDomain.DeletionOutcome, 0, len(classes))
        pins := make([]privacyDomain.PolicyPin, 0, len(classes))
        for _, class := range classes {
                spec, _ := privacyDomain.LookupClass(class)
                policy := currentPolicies[class]
                pins = append(pins, privacyDomain.PolicyPin{Class: class, Version: policy.Version, TerminalEvent: policy.TerminalEvent})
                outcomes = append(outcomes, s.classOutcome(ctx, scope, spec, policy, holds[class], now))
        }
        requestID, err := s.ids.New(now)
        if err != nil {
                return api.DeletionResult{}, err
        }
        receiptID, err := s.ids.New(now)
        if err != nil {
                return api.DeletionResult{}, err
        }
        receipt := privacyDomain.Receipt{
                TenantID:       scope.TenantID().String(),
                ReceiptID:      receiptID.String(),
                RequestID:      requestID.String(),
                ReceiptHash:    receiptHash(requestID, scope.TenantID(), classes, pins, outcomes),
                Authority:      "data_governance",
                PolicyVersions: pins,
                Outcomes:       outcomes,
                CompletedAt:    now,
                ActorID:        scope.PrincipalID().String(),
        }
        _, storedOutcomes, storedReceipt, replayed, err := s.repository.SaveDeletion(ctx, port.DeletionRecordParams{
                Scope: scope,
                Request: port.DeletionRequestRecord{
                        RequestID: requestID, ActorID: scope.PrincipalID(),
                        IdempotencyKey: key, RequestHash: hash, Scope: classes, RequestedAt: now,
                },
                Outcomes: outcomes,
                Receipt:  receipt,
        })
        if err != nil {
                return api.DeletionResult{}, err
        }
        if replayed {
                storedReceipt.Outcomes = storedOutcomes
                return api.DeletionResult{Receipt: storedReceipt, Replayed: true}, nil
        }
        return api.DeletionResult{Receipt: receipt, Replayed: false}, nil
}

// classOutcome computes the honest outcome of one class from facts only.
func (s *Service) classOutcome(ctx context.Context, scope tenancy.Context, spec privacyDomain.ClassSpec, policy privacyDomain.RetentionPolicy, hold privacyDomain.HoldState, now time.Time) privacyDomain.DeletionOutcome {
        // 1. Legal hold keeps the minimal protected set with its basis.
        if hold.Status == privacyDomain.HoldActive || hold.Status == privacyDomain.HoldRequiresDecision {
                note := "Юридическое удержание: " + hold.Basis + " (утвердил: " + hold.Approver + "; срок пересмотра: " + hold.ReviewDeadline.Format("2006-01-02") + ")."
                if hold.Status == privacyDomain.HoldRequiresDecision {
                        note = "Юридическое удержание действует, но срок пересмотра истёк — требуется решение владельца. Основание: " + hold.Basis + " (утвердил: " + hold.Approver + ")."
                }
                return privacyDomain.DeletionOutcome{Class: spec.ID, Outcome: privacyDomain.OutcomeRetainedByLaw, Note: note}
        }
        // 2. Deliberately blocked routes carry the managed block.
        if spec.Block != nil {
                block := *spec.Block
                return privacyDomain.DeletionOutcome{
                        Class: spec.ID, Outcome: privacyDomain.OutcomeUnsupported,
                        Note:  "Удаление заблокировано: " + block.Required + ". Ответственный: " + block.Owner + ".",
                        Block: &block,
                }
        }
        // 3. The audit class keeps its journal until the policy minimal term.
        if spec.AuditMinTerm {
                anchor, err := s.repository.ClassReferenceTime(ctx, scope, spec.ID)
                if err == nil && anchor == nil {
                        return privacyDomain.DeletionOutcome{Class: spec.ID, Outcome: privacyDomain.OutcomeDeleted, Note: "Строк журнала аудита нет: удалять нечего."}
                }
                if err != nil {
                        return unknownOutcome(spec, now, "не удалось определить точку отсчёта журнала аудита; статус неопределён, повтор вслепую исключён")
                }
                minDue := policy.MandatoryDueAt(*anchor)
                if now.Before(minDue) {
                        return privacyDomain.DeletionOutcome{
                                Class: spec.ID, Outcome: privacyDomain.OutcomeRetainedByLaw, DueAt: &minDue,
                                Note: "Минимальный срок хранения журнала аудита по политике класса не истёк (до " + minDue.Format("2006-01-02") + "): ранее удалять нельзя.",
                        }
                }
                block := privacyDomain.AuditMaintenanceBlock
                return privacyDomain.DeletionOutcome{
                        Class: spec.ID, Outcome: privacyDomain.OutcomeUnsupported,
                        Note:  "Минимальный срок хранения истёк, но удаление журнала аудита выполняется выделенной maintenance-процедурой; маршрут заблокирован. Ответственный: " + block.Owner + ".",
                        Block: &block,
                }
        }
        // 4. Term check: an unexpired policy schedules the deletion honestly.
        // A failing anchor query is an honest unknown (the same shape as the
        // audit path): the term decision never silently falls back to the
        // policy publication time.
        anchor := policy.CreatedAt
        reference, err := s.repository.ClassReferenceTime(ctx, scope, spec.ID)
        if err != nil {
                return unknownOutcome(spec, now, "не удалось определить точку отсчёта данных класса; статус неопределён, повтор вслепую исключён")
        }
        if reference != nil {
                anchor = *reference
        }
        due := policy.DeletionDueAt(anchor)
        if now.Before(due) {
                return privacyDomain.DeletionOutcome{
                        Class: spec.ID, Outcome: privacyDomain.OutcomeScheduled, DueAt: &due,
                        Note: "Срок политики класса не истёк: удаление запланировано на " + due.Format("2006-01-02") + " (точка отсчёта: " + anchor.Format("2006-01-02") + ", активное использование " + fmt.Sprint(policy.ActiveUseDays) + " дн. + архив " + fmt.Sprint(policy.ArchiveDays) + " дн.).",
                }
        }
        // 5. Owning-module executor decides the local deletion result.
        if spec.Executor != "" {
                executor, ok := s.executors[spec.Executor]
                if !ok {
                        return unknownOutcome(spec, now, "модуль-исполнитель не подключён в этой сборке")
                }
                result, err := executor.ExecuteClassDeletion(ctx, scope, spec.ID, now)
                if err != nil {
                        return unknownOutcome(spec, now, "исполнитель вернул неопределённый сбой: локальный эффект неизвестен, повтор вслепую исключён")
                }
                outcome := privacyDomain.DeletionOutcome{
                        Class: spec.ID, Outcome: result.Outcome, AffectedCount: result.AffectedCount,
                        Note: result.Note, Block: result.Block,
                }
                if result.DueAt != nil {
                        outcome.DueAt = result.DueAt
                }
                if result.Outcome == privacyDomain.OutcomeUnknown {
                        deadline := now.Add(unknownReconciliationWindow)
                        outcome.ReconciliationOwner = orDefault(result.ReconciliationOwner, "Technical Lead")
                        outcome.ReconciliationDeadline = &deadline
                }
                return outcome
        }
        // 6. No local executor in this slice: honest unknown with reconciliation.
        return unknownOutcome(spec, now, "копия класса не имеет локального исполнителя в текущем срезе: требуется ручная сверка")
}

func unknownOutcome(spec privacyDomain.ClassSpec, now time.Time, reason string) privacyDomain.DeletionOutcome {
        deadline := now.Add(unknownReconciliationWindow)
        return privacyDomain.DeletionOutcome{
                Class: spec.ID, Outcome: privacyDomain.OutcomeUnknown,
                Note: "Результат неопределён: " + reason + ". Владелец reconciliation: Technical Lead; срок: " + deadline.Format("2006-01-02") + ".",
                ReconciliationOwner:    "Technical Lead",
                ReconciliationDeadline: &deadline,
        }
}

func orDefault(value, fallback string) string {
        if value == "" {
                return fallback
        }
        return value
}

// ReadReceipt returns one request bundle for viewing.
func (s *Service) ReadReceipt(ctx context.Context, actor identityDomain.Principal, tenantID, requestID id.UUID) (api.DeletionResult, error) {
        scope, _, err := s.viewScope(ctx, actor, tenantID)
        if err != nil {
                return api.DeletionResult{}, err
        }
        request, outcomes, receipt, err := s.repository.ReadReceipt(ctx, scope, requestID)
        if err != nil {
                return api.DeletionResult{}, err
        }
        receipt.Outcomes = outcomes
        _ = request
        return api.DeletionResult{Receipt: receipt}, nil
}

func dedupeClasses(classes []privacyDomain.ArtifactClass) []privacyDomain.ArtifactClass {
        seen := map[privacyDomain.ArtifactClass]bool{}
        out := make([]privacyDomain.ArtifactClass, 0, len(classes))
        for _, class := range classes {
                if !seen[class] {
                        seen[class] = true
                        out = append(out, class)
                }
        }
        sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
        return out
}

// deletionRequestHash canonicalizes the command for replay/conflict detection.
func deletionRequestHash(tenant interface{ String() string }, classes []privacyDomain.ArtifactClass) string {
        h := sha256.New()
        _, _ = h.Write([]byte("privacy.deletion.v1"))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(tenant.String()))
        for _, class := range classes {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(class))
        }
        return hex.EncodeToString(h.Sum(nil))
}

// receiptHash anchors the receipt evidence: request identity, scope, applied
// policy versions and outcome lines. Deleted content never enters the hash.
func receiptHash(requestID interface{ String() string }, tenant interface{ String() string }, classes []privacyDomain.ArtifactClass, pins []privacyDomain.PolicyPin, outcomes []privacyDomain.DeletionOutcome) string {
        h := sha256.New()
        _, _ = h.Write([]byte("privacy.receipt.v1"))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(requestID.String()))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(tenant.String()))
        for _, class := range classes {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(class))
        }
        for _, pin := range pins {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(pin.Class))
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(fmt.Sprint(pin.Version)))
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(pin.TerminalEvent))
        }
        for _, outcome := range outcomes {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(outcome.Class))
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(outcome.Outcome))
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(fmt.Sprint(outcome.AffectedCount)))
                if outcome.DueAt != nil {
                        _, _ = h.Write([]byte{0})
                        _, _ = h.Write([]byte(outcome.DueAt.UTC().Format(time.RFC3339)))
                }
        }
        return hex.EncodeToString(h.Sum(nil))
}
