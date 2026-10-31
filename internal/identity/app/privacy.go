package app

import (
	"context"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/platform/tenancy"
	privacyDomain "github.com/staffy-co/staffy/internal/privacy/domain"
	privacyPort "github.com/staffy-co/staffy/internal/privacy/port"
)

// identityPurgeRepository is the narrow store capability the identity privacy
// executor needs. The identity module owns its tables: only it may delete
// them, through the SECURITY DEFINER purge function of migration 00003.
type identityPurgeRepository interface {
	PurgeExpiredInvitations(ctx context.Context, scope tenancy.Context, cutoff time.Time) (int64, error)
	IdentityCensus(ctx context.Context, scope tenancy.Context) (activeMemberships, pendingInvitations int64, err error)
	EarliestPendingExpiry(ctx context.Context, scope tenancy.Context) (*time.Time, error)
}

// PrivacyDeletionExecutor exposes the identity module as the owning-module
// executor of the privacy orchestrator.
func (s *Service) PrivacyDeletionExecutor() privacyPort.ClassDeletionExecutor {
	return identityPrivacyExecutor{service: s}
}

type identityPrivacyExecutor struct{ service *Service }

// ExecuteClassDeletion handles the member-identity class: expired invitations
// are deleted honestly; active memberships keep the offboarding/DSAR block
// (a future slice); still-pending invitations schedule the deletion.
func (e identityPrivacyExecutor) ExecuteClassDeletion(ctx context.Context, scope tenancy.Context, class privacyDomain.ArtifactClass, now time.Time) (privacyPort.ExecutorResult, error) {
	if class != privacyDomain.ClassMemberIdentity {
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnknown,
			Note:    "Класс не принадлежит модулю identity: исполнитель не подключён для этого класса.",
		}, nil
	}
	repo, ok := e.service.repository.(identityPurgeRepository)
	if !ok {
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnknown,
			Note:    "Хранилище identity не поддерживает очистку в этой сборке: статус неопределён, повтор вслепую исключён.",
		}, nil
	}
	deleted, err := repo.PurgeExpiredInvitations(ctx, scope, now)
	if err != nil {
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnknown,
			Note:    "Очистка приглашений завершилась неопределённым сбоем (" + err.Error() + "): локальный эффект неизвестен, повтор вслепую исключён; требуется reconciliation.",
		}, nil
	}
	active, pending, err := repo.IdentityCensus(ctx, scope)
	if err != nil {
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnknown,
			Note:    "Не удалось пересчитать активные участники после очистки: статус неопределён, повтор вслепую исключён; требуется reconciliation.",
		}, nil
	}
	if active > 0 {
		block := privacyDomain.OffboardingBlock
		return privacyPort.ExecutorResult{
			Outcome: privacyDomain.OutcomeUnsupported,
			AffectedCount: deleted,
			Note: fmt.Sprintf("Истёкшие приглашения удалены: %d. Активные участники сохранены: %d — их удаление требует offboarding/DSAR-потока (будущий срез), ответственный: %s.",
				deleted, active, block.Owner),
			Block: &block,
		}, nil
	}
	if pending > 0 {
		due := now
		if expiry, err := repo.EarliestPendingExpiry(ctx, scope); err == nil && expiry != nil {
			due = *expiry
		}
		return privacyPort.ExecutorResult{
			Outcome:       privacyDomain.OutcomeScheduled,
			AffectedCount: deleted,
			DueAt:         &due,
			Note:          fmt.Sprintf("Истёкшие приглашения удалены: %d. Ожидающие приглашения (%d) не удалены: удаление возможно после их истечения.", deleted, pending),
		}, nil
	}
	return privacyPort.ExecutorResult{
		Outcome:       privacyDomain.OutcomeDeleted,
		AffectedCount: deleted,
		Note:          fmt.Sprintf("Удалено истёкших приглашений: %d; активных участников и ожидающих приглашений нет.", deleted),
	}, nil
}
