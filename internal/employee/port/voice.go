package port

import (
	"context"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"time"
)

type VoiceSaveParams struct {
	Scope                                       tenancy.Context
	EmployeeID, CorrelationID, EventID, AuditID id.UUID
	ExpectedVersion                             int64
	Config                                      domain.VoiceConfig
	Summary                                     string
	Now                                         time.Time
}
type VoiceRepository interface {
	ReadVoice(context.Context, tenancy.Context, id.UUID) ([]domain.VoiceVersion, error)
	SaveVoice(context.Context, VoiceSaveParams) (domain.VoiceVersion, error)
	SaveVoicePreview(context.Context, tenancy.Context, domain.VoicePreviewRecord) error
	ReadVoiceAssessments(context.Context, tenancy.Context, id.UUID) ([]domain.VoiceAssessment, error)
	SaveVoiceAssessment(context.Context, VoiceAssessmentSaveParams) (domain.VoiceAssessment, error)
}

type VoiceAssessmentSaveParams struct {
	Scope                                                                tenancy.Context
	EmployeeID, PreviewID, AssessmentID, EventID, AuditID, CorrelationID id.UUID
	VoiceVersion                                                         int64
	Verdict                                                              string
	Now                                                                  time.Time
}

// TestTTS is deliberately narrow: an adapter may only synthesize fixture text offline.
type TestTTS interface {
	Capability(context.Context, domain.VoiceConfig) error
	Synthesize(context.Context, domain.VoiceConfig, string, bool) ([]byte, string, error)
}
