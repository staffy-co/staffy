package app

import (
	"context"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/platform/tenancy"
	privacyPort "github.com/staffy-co/staffy/internal/privacy/port"
	privacyDomain "github.com/staffy-co/staffy/internal/privacy/domain"
)

// purgeRepository is the narrow store capability the privacy executor needs.
// The employee module owns its tables: only it may delete them, through the
// SECURITY DEFINER purge functions of migration 00009.
type purgeRepository interface {
	PurgeKnowledgePayload(ctx context.Context, scope tenancy.Context) (sources, dependent int64, err error)
	PurgeVoicePreviewMedia(ctx context.Context, scope tenancy.Context) (previews, assessments int64, err error)
	PurgeVoiceAssessments(ctx context.Context, scope tenancy.Context) (assessments int64, err error)
	PurgeEmployeeDrafts(ctx context.Context, scope tenancy.Context) (drafts, dependent int64, err error)
}

// PrivacyDeletionExecutor exposes the employee module as the owning-module
// executor of the privacy orchestrator.
func (s *Service) PrivacyDeletionExecutor() privacyPort.ClassDeletionExecutor {
	return privacyExecutor{service: s}
}

type privacyExecutor struct{ service *Service }

// ExecuteClassDeletion deletes the local copies of one employee-owned class
// and returns the honest outcome. Refusals and ambiguity are outcomes, not
// panics; a repository that cannot purge reports unknown.
func (e privacyExecutor) ExecuteClassDeletion(ctx context.Context, scope tenancy.Context, class privacyDomain.ArtifactClass, now time.Time) (privacyPort.ExecutorResult, error) {
	repo, ok := e.service.repository.(purgeRepository)
	if !ok {
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnknown,
			Note:    "Хранилище employee не поддерживает очистку в этой сборке: статус неопределён, повтор вслепую исключён.",
		}, nil
	}
	switch class {
	case privacyDomain.ClassKnowledgeSourcePayload:
		sources, dependent, err := repo.PurgeKnowledgePayload(ctx, scope)
		if err != nil {
			return privacyPort.ExecutorResult{Outcome: privacyDomain.OutcomeUnknown, Note: purgeFailureNote(err)}, nil
		}
		return privacyPort.ExecutorResult{
			Outcome:       privacyDomain.OutcomeDeleted,
			AffectedCount: sources,
			Note:          fmt.Sprintf("Удалено источников знаний: %d (включая зависимые строки версий, чанков и журнала: %d). Содержимое в квитанцию не входит.", sources, dependent),
		}, nil
	case privacyDomain.ClassEmployeeDraft:
		drafts, dependent, err := repo.PurgeEmployeeDrafts(ctx, scope)
		if err != nil {
			return privacyPort.ExecutorResult{Outcome: privacyDomain.OutcomeUnknown, Note: purgeFailureNote(err)}, nil
		}
		return privacyPort.ExecutorResult{
			Outcome:       privacyDomain.OutcomeDeleted,
			AffectedCount: drafts,
			Note:          fmt.Sprintf("Удалено черновиков ИИ-сотрудников: %d. Внешние ключи связывают данные сотрудника в один агрегат: вместе удалено зависимых строк (знания, голос, поведение, аудит, события, идемпотентные записи): %d.", drafts, dependent),
		}, nil
	case privacyDomain.ClassVoicePreviewMedia:
		previews, assessments, err := repo.PurgeVoicePreviewMedia(ctx, scope)
		if err != nil {
			return privacyPort.ExecutorResult{Outcome: privacyDomain.OutcomeUnknown, Note: purgeFailureNote(err)}, nil
		}
		return privacyPort.ExecutorResult{
			Outcome:       privacyDomain.OutcomeDeleted,
			AffectedCount: previews,
			Note:          fmt.Sprintf("Удалено предпрослушиваний: %d (включая связанные оценки, удаляемые по целостности: %d). Содержимое предпрослушиваний в квитанцию не входит.", previews, assessments),
		}, nil
	case privacyDomain.ClassVoiceAssessment:
		assessments, err := repo.PurgeVoiceAssessments(ctx, scope)
		if err != nil {
			return privacyPort.ExecutorResult{Outcome: privacyDomain.OutcomeUnknown, Note: purgeFailureNote(err)}, nil
		}
		return privacyPort.ExecutorResult{
			Outcome:       privacyDomain.OutcomeDeleted,
			AffectedCount: assessments,
			Note:          fmt.Sprintf("Удалено оценок предпрослушиваний: %d.", assessments),
		}, nil
	default:
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnknown,
			Note:    "Класс не принадлежит модулю employee: исполнитель не подключён для этого класса.",
		}, nil
	}
}

func purgeFailureNote(err error) string {
	return "Очистка завершилась неопределённым сбоем (" + err.Error() + "): локальный эффект неизвестен, повтор вслепую исключён; требуется reconciliation."
}
