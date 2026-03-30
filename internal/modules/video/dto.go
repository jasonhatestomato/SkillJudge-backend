package video

import (
	"time"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
)

type UploadCredentialDTO struct {
	VideoID    uuid.UUID `json:"videoId"`
	UploadURL  string    `json:"uploadUrl"`
	UploadID   string    `json:"uploadId"`
	Credential any       `json:"credential"`
}

type VideoTaskDTO struct {
	ID          *uuid.UUID `json:"id,omitempty"`
	Name        *string    `json:"name,omitempty"`
	Status      *string    `json:"status,omitempty"`
	AIScore     *float64   `json:"aiScore,omitempty"`
	ManualScore *float64   `json:"manualScore,omitempty"`
}

type VideoListItemDTO struct {
	ID               uuid.UUID      `json:"id"`
	Filename         string         `json:"filename"`
	OriginalFilename *string        `json:"originalFilename,omitempty"`
	StudentName      string         `json:"studentName"`
	StudentNumber    string         `json:"studentNumber"`
	Duration         *int           `json:"duration,omitempty"`
	FileSize         int64          `json:"fileSize"`
	Status           string         `json:"status"`
	UploadProgress   int            `json:"uploadProgress"`
	Resolution       *string        `json:"resolution,omitempty"`
	Format           *string        `json:"format,omitempty"`
	TranscodeStatus  *string        `json:"transcodeStatus,omitempty"`
	ThumbnailURL     *string        `json:"thumbnailUrl,omitempty"`
	UploadedAt       *time.Time     `json:"uploadedAt,omitempty"`
	AssignedAt       *time.Time     `json:"assignedAt,omitempty"`
	CompletedAt      *time.Time     `json:"completedAt,omitempty"`
	EvaluationStatus string         `json:"evaluationStatus"`
	AIStatus         string         `json:"aiStatus"`
	ManualStatus     string         `json:"manualStatus"`
	AIScore          *float64       `json:"aiScore,omitempty"`
	ManualScore      *float64       `json:"manualScore,omitempty"`
	Task             *VideoTaskDTO  `json:"task,omitempty"`
	Scorer           *VideoOwnerDTO `json:"scorer,omitempty"`
}

type VideoProjectDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type VideoOwnerDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	RealName *string   `json:"realName,omitempty"`
}

type VideoManualEvaluationDTO struct {
	Score        *float64         `json:"score,omitempty"`
	ScoreDetails []map[string]any `json:"scoreDetails,omitempty"`
	Comments     *string          `json:"comments,omitempty"`
	Status       string           `json:"status"`
	StartedAt    *time.Time       `json:"startedAt,omitempty"`
	SubmittedAt  *time.Time       `json:"submittedAt,omitempty"`
}

type VideoDetailDTO struct {
	ID               uuid.UUID        `json:"id"`
	Filename         string           `json:"filename"`
	OriginalFilename *string          `json:"originalFilename,omitempty"`
	StudentName      string           `json:"studentName"`
	StudentNumber    string           `json:"studentNumber"`
	Duration         *int             `json:"duration,omitempty"`
	Status           string           `json:"status"`
	UploadProgress   int              `json:"uploadProgress"`
	Resolution       *string          `json:"resolution,omitempty"`
	Format           *string          `json:"format,omitempty"`
	TranscodeStatus  *string          `json:"transcodeStatus,omitempty"`
	PlayURL          *string          `json:"playUrl,omitempty"`
	StorageURL       *string          `json:"storageUrl,omitempty"`
	ThumbnailURL     *string          `json:"thumbnailUrl,omitempty"`
	FileSize         int64            `json:"fileSize"`
	UploadedAt       *time.Time       `json:"uploadedAt,omitempty"`
	AssignedAt       *time.Time       `json:"assignedAt,omitempty"`
	CompletedAt      *time.Time       `json:"completedAt,omitempty"`
	EvaluationStatus string           `json:"evaluationStatus"`
	AIStatus         string           `json:"aiStatus"`
	ManualStatus     string           `json:"manualStatus"`
	AIScore          *float64         `json:"aiScore,omitempty"`
	ManualScore      *float64         `json:"manualScore,omitempty"`
	Project          *VideoProjectDTO `json:"project,omitempty"`
	Creator          *VideoOwnerDTO   `json:"creator,omitempty"`
	Scorer           *VideoOwnerDTO   `json:"scorer,omitempty"`
	Task             *VideoTaskDTO    `json:"task,omitempty"`
	AIEvaluation     any              `json:"aiEvaluation,omitempty"`
	ManualEvaluation *VideoManualEvaluationDTO `json:"manualEvaluation,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type ListVideosResult struct {
	Items      []VideoListItemDTO `json:"items"`
	Pagination Pagination         `json:"pagination"`
}

func ToVideoListItemDTO(item *model.Video) VideoListItemDTO {
	dto := VideoListItemDTO{
		ID:               item.ID,
		Filename:         item.Filename,
		OriginalFilename: item.OriginalFilename,
		StudentName:      item.StudentName,
		StudentNumber:    item.StudentNumber,
		Duration:         item.Duration,
		FileSize:         item.FileSize,
		Status:           item.Status,
		UploadProgress:   item.UploadProgress,
		Resolution:       item.Resolution,
		Format:           item.Format,
		TranscodeStatus:  item.TranscodeStatus,
		ThumbnailURL:     item.ThumbnailURL,
		UploadedAt:       item.UploadedAt,
		AssignedAt:       item.AssignedAt,
		CompletedAt:      item.CompletedAt,
		EvaluationStatus: item.EvaluationStatus,
		AIStatus:         item.AIStatus,
		ManualStatus:     item.ManualStatus,
		AIScore:          item.AIScore,
		ManualScore:      item.ManualScore,
	}

	if item.Task != nil {
		dto.Task = &VideoTaskDTO{
			ID:          &item.Task.ID,
			Name:        &item.Task.Name,
			Status:      &item.Task.Status,
			AIScore:     item.AIScore,
			ManualScore: item.ManualScore,
		}
	}
	if item.Scorer != nil {
		dto.Scorer = &VideoOwnerDTO{
			ID:       item.Scorer.ID,
			Username: item.Scorer.Username,
			RealName: item.Scorer.RealName,
		}
	}

	return dto
}

func ToVideoDetailDTO(item *model.Video, playURL *string, manual *model.ManualEvaluation) *VideoDetailDTO {
	dto := &VideoDetailDTO{
		ID:               item.ID,
		Filename:         item.Filename,
		OriginalFilename: item.OriginalFilename,
		StudentName:      item.StudentName,
		StudentNumber:    item.StudentNumber,
		Duration:         item.Duration,
		Status:           item.Status,
		UploadProgress:   item.UploadProgress,
		Resolution:       item.Resolution,
		Format:           item.Format,
		TranscodeStatus:  item.TranscodeStatus,
		PlayURL:          playURL,
		StorageURL:       item.StorageURL,
		ThumbnailURL:     item.ThumbnailURL,
		FileSize:         item.FileSize,
		UploadedAt:       item.UploadedAt,
		AssignedAt:       item.AssignedAt,
		CompletedAt:      item.CompletedAt,
		EvaluationStatus: item.EvaluationStatus,
		AIStatus:         item.AIStatus,
		ManualStatus:     item.ManualStatus,
		AIScore:          item.AIScore,
		ManualScore:      item.ManualScore,
		AIEvaluation:     nil,
	}

	if item.Task != nil {
		dto.Task = &VideoTaskDTO{
			ID:          &item.Task.ID,
			Name:        &item.Task.Name,
			Status:      &item.Task.Status,
			AIScore:     item.AIScore,
			ManualScore: item.ManualScore,
		}
	}
	project := item.Project
	if project == nil && item.Task != nil {
		project = item.Task.Project
	}
	if project != nil {
		dto.Project = &VideoProjectDTO{
			ID:   project.ID,
			Name: project.Name,
		}
	}
	if item.Creator != nil {
		dto.Creator = &VideoOwnerDTO{
			ID:       item.Creator.ID,
			Username: item.Creator.Username,
			RealName: item.Creator.RealName,
		}
	}
	if item.Scorer != nil {
		dto.Scorer = &VideoOwnerDTO{
			ID:       item.Scorer.ID,
			Username: item.Scorer.Username,
			RealName: item.Scorer.RealName,
		}
	}
	if manual != nil {
		dto.ManualEvaluation = &VideoManualEvaluationDTO{
			Score:        manual.TotalScore,
			ScoreDetails: manual.ScoreDetails,
			Comments:     manual.Comments,
			Status:       manual.Status,
			StartedAt:    manual.StartedAt,
			SubmittedAt:  manual.SubmittedAt,
		}
	}

	return dto
}
