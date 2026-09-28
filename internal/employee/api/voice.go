package api

import (
	"context"
	"github.com/artemiosu/staffy/internal/employee/domain"
	identity "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type VoiceCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	ExpectedVersion                     int64
	Config                              domain.VoiceConfig
	ChangeSummary                       string
}

type VoicePreviewRequest struct {
	Actor                  identity.Principal
	TenantID, EmployeeID   id.UUID
	VoiceVersion           int64
	Text                   string
	Slow                   bool
	SyntheticDataConfirmed bool
}

type VoicePreview struct {
	Text                 string  `json:"text"`
	SynthesisText        string  `json:"synthesis_text"`
	PreviewID            id.UUID `json:"preview_id,omitempty"`
	VoiceVersion         int64   `json:"voice_version"`
	DictionaryVersion    int64   `json:"dictionary_version"`
	DraftVersion         int64   `json:"draft_version"`
	Route                string  `json:"route"`
	Capability           string  `json:"capability"`
	Audio                []byte  `json:"audio,omitempty"`
	AudioType            string  `json:"audio_type,omitempty"`
	PhoneQualityApproved bool    `json:"phone_quality_approved"`
	Stale                bool    `json:"stale"`
}

type VoiceAssessmentCommand struct {
	Actor                                          identity.Principal
	TenantID, EmployeeID, PreviewID, CorrelationID id.UUID
	VoiceVersion                                   int64
	Verdict                                        string
}

type VoiceService interface {
	VoiceCatalog(context.Context, identity.Principal, id.UUID) ([]domain.VoiceProfile, error)
	ReadVoice(context.Context, identity.Principal, id.UUID, id.UUID) ([]domain.VoiceVersion, error)
	SaveVoice(context.Context, VoiceCommand) (domain.VoiceVersion, error)
	PreviewVoice(context.Context, VoicePreviewRequest) (VoicePreview, error)
	ReadVoiceAssessments(context.Context, identity.Principal, id.UUID, id.UUID) ([]domain.VoiceAssessment, error)
	AssessVoice(context.Context, VoiceAssessmentCommand) (domain.VoiceAssessment, error)
}
