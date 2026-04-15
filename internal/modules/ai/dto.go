package ai

import (
	"time"

	"github.com/google/uuid"
)

const (
	EvaluationStatusProcessing = "processing"
	EvaluationStatusCompleted  = "completed"
	EvaluationStatusFailed     = "failed"
)

type SummaryDTO struct {
	OverallDescription *string  `json:"overallDescription,omitempty"`
	Score              *float64 `json:"score,omitempty"`
	MaxScore           *float64 `json:"maxScore,omitempty"`
}

type DetailItemDTO struct {
	Subtitle  string             `json:"subtitle"`
	FullScore *float64           `json:"fullScore,omitempty"`
	AIScore   *float64           `json:"aiScore,omitempty"`
	Status    *string            `json:"status,omitempty"`
	Feedback  *string            `json:"feedback,omitempty"`
	Evidence  *DetailEvidenceDTO `json:"evidence,omitempty"`
}

type DetailEvidenceDTO struct {
	Times       []string `json:"times,omitempty"`
	Screenshots []string `json:"screenshots,omitempty"`
}

type DetailGroupDTO struct {
	Title     string          `json:"title"`
	FullScore *float64        `json:"fullScore,omitempty"`
	AIScore   *float64        `json:"aiScore,omitempty"`
	Items     []DetailItemDTO `json:"items,omitempty"`
}

type EvidenceDTO struct {
	EvidenceID *string  `json:"evidenceId,omitempty"`
	Kind       *string  `json:"kind,omitempty"`
	TimeSec    *float64 `json:"timeSec,omitempty"`
	URL        *string  `json:"url,omitempty"`
}

type VideoPointDTO struct {
	PointID   *string       `json:"pointId,omitempty"`
	Name      string        `json:"name"`
	Type      *string       `json:"type,omitempty"`
	Severity  *string       `json:"severity,omitempty"`
	StartSec  *float64      `json:"startSec,omitempty"`
	EndSec    *float64      `json:"endSec,omitempty"`
	StartTime *string       `json:"startTime,omitempty"`
	EndTime   *string       `json:"endTime,omitempty"`
	Feedback  *string       `json:"feedback,omitempty"`
	Evidences []EvidenceDTO `json:"evidences,omitempty"`
}

type VideoStageDTO struct {
	StageID   *string  `json:"stageId,omitempty"`
	Name      string   `json:"name"`
	StageType *string  `json:"stageType,omitempty"`
	StartSec  *float64 `json:"startSec,omitempty"`
	EndSec    *float64 `json:"endSec,omitempty"`
	StartTime *string  `json:"startTime,omitempty"`
	EndTime   *string  `json:"endTime,omitempty"`
}

type ArtifactsDTO struct {
	ReportHTMLURL        *string `json:"reportHtmlUrl,omitempty"`
	AnalysisJSONURL      *string `json:"analysisJsonUrl,omitempty"`
	EvidenceIndexJSONURL *string `json:"evidenceIndexJsonUrl,omitempty"`
}

type EmbeddedEvaluationDTO struct {
	EvaluationID uuid.UUID        `json:"evaluationId"`
	Status       string           `json:"status"`
	Score        *float64         `json:"score,omitempty"`
	ModelVersion *string          `json:"modelVersion,omitempty"`
	ErrorMessage *string          `json:"errorMessage,omitempty"`
	StartedAt    *time.Time       `json:"startedAt,omitempty"`
	CompletedAt  *time.Time       `json:"completedAt,omitempty"`
	Summary      *SummaryDTO      `json:"summary,omitempty"`
	Details      []DetailGroupDTO `json:"details,omitempty"`
	VideoStages  []VideoStageDTO  `json:"videoStages,omitempty"`
	VideoPoints  []VideoPointDTO  `json:"videoPoints,omitempty"`
	Artifacts    *ArtifactsDTO    `json:"artifacts,omitempty"`
}

type EvaluationDTO struct {
	ID           uuid.UUID  `json:"id"`
	VideoID      uuid.UUID  `json:"videoId"`
	TaskID       uuid.UUID  `json:"taskId"`
	Status       string     `json:"status"`
	ModelVersion *string    `json:"modelVersion,omitempty"`
	TotalScore   *float64   `json:"totalScore,omitempty"`
	ErrorMessage *string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

type EvaluationResultDTO struct {
	ID           uuid.UUID        `json:"id"`
	VideoID      uuid.UUID        `json:"videoId"`
	TaskID       uuid.UUID        `json:"taskId"`
	Status       string           `json:"status"`
	ModelVersion *string          `json:"modelVersion,omitempty"`
	Summary      *SummaryDTO      `json:"summary,omitempty"`
	Details      []DetailGroupDTO `json:"details,omitempty"`
	VideoStages  []VideoStageDTO  `json:"videoStages,omitempty"`
	VideoPoints  []VideoPointDTO  `json:"videoPoints,omitempty"`
	Artifacts    *ArtifactsDTO    `json:"artifacts,omitempty"`
	CompletedAt  *time.Time       `json:"completedAt,omitempty"`
}

type BatchCreateError struct {
	VideoID uuid.UUID `json:"videoId"`
	Error   string    `json:"error"`
}

type BatchCreateResult struct {
	Total      int                `json:"total"`
	Processing int                `json:"processing"`
	Failed     int                `json:"failed"`
	Errors     []BatchCreateError `json:"errors,omitempty"`
}

type storedResult struct {
	Summary     *SummaryDTO      `json:"summary,omitempty"`
	Details     []DetailGroupDTO `json:"details,omitempty"`
	VideoStages []VideoStageDTO  `json:"videoStages,omitempty"`
	VideoPoints []VideoPointDTO  `json:"videoPoints,omitempty"`
	Artifacts   *ArtifactsDTO    `json:"artifacts,omitempty"`
}
