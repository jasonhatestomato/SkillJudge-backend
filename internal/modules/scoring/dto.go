package scoring

import (
	"time"

	"skilljudge/backend/internal/domain/evaluation"
	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
)

type AssignmentStrategy string

const (
	AssignmentStrategyAverage  AssignmentStrategy = "average"
	AssignmentStrategySpecific AssignmentStrategy = "specific"
)

type ReassignmentMode string

const (
	ReassignmentModeAverage  ReassignmentMode = "average"
	ReassignmentModeQuantity ReassignmentMode = "quantity"
)

type SpecificAssignment struct {
	VideoID  uuid.UUID `json:"videoId"`
	ScorerID uuid.UUID `json:"scorerId"`
}

type AssignScorersInput struct {
	TaskID              uuid.UUID
	VideoIDs            []uuid.UUID
	ScorerIDs           []uuid.UUID
	AssignmentStrategy  AssignmentStrategy
	SpecificAssignments []SpecificAssignment
}

type QuantityAssignment struct {
	ScorerID uuid.UUID `json:"scorerId"`
	Count    int       `json:"count"`
}

type PendingAssignmentVideoDTO struct {
	ID               uuid.UUID  `json:"id"`
	StudentName      string     `json:"studentName"`
	StudentNumber    string     `json:"studentNumber"`
	Filename         string     `json:"filename"`
	ScorerID         *uuid.UUID `json:"scorerId,omitempty"`
	ScorerName       string     `json:"scorerName"`
	ScorerStatus     string     `json:"scorerStatus"`
	ManualStatus     string     `json:"manualStatus"`
	AssignedAt       *time.Time `json:"assignedAt,omitempty"`
	IsReassignable   bool       `json:"isReassignable"`
	CurrentTaskState string     `json:"currentTaskState"`
}

type PendingAssignmentsResult struct {
	Items []PendingAssignmentVideoDTO `json:"items"`
	Total int                         `json:"total"`
}

type ReassignPendingVideosInput struct {
	TaskID              uuid.UUID
	VideoIDs            []uuid.UUID
	ScorerIDs           []uuid.UUID
	ReassignmentMode    ReassignmentMode
	QuantityAssignments []QuantityAssignment
}

type ReassignPendingVideosResult struct {
	Total                  int               `json:"total"`
	Created                int               `json:"created"`
	Tasks                  []AssignedTaskDTO `json:"tasks"`
	NotifiedScorerIDs      []uuid.UUID       `json:"notifiedScorerIds,omitempty"`
	ExpiredSourceScorerIDs []uuid.UUID       `json:"expiredSourceScorerIds,omitempty"`
}

type AssignmentTaskVideoDTO struct {
	ID            uuid.UUID `json:"id"`
	StudentName   string    `json:"studentName"`
	StudentNumber string    `json:"studentNumber"`
	Filename      string    `json:"filename"`
}

type AssignmentTaskScorerDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	RealName *string   `json:"realName,omitempty"`
}

type AssignedTaskDTO struct {
	Video      AssignmentTaskVideoDTO  `json:"video"`
	Scorer     AssignmentTaskScorerDTO `json:"scorer"`
	Status     string                  `json:"status"`
	AssignedAt *time.Time              `json:"assignedAt,omitempty"`
}

type AssignScorersResult struct {
	Total   int               `json:"total"`
	Created int               `json:"created"`
	Tasks   []AssignedTaskDTO `json:"tasks"`
}

type AssignableScorerDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	RealName *string   `json:"realName,omitempty"`
}

type AssignableScorersResult struct {
	Items []AssignableScorerDTO `json:"items"`
}

type MyTasksListParams struct {
	Page      int
	PageSize  int
	Status    string
	ProjectID *uuid.UUID
}

type MyTaskProjectDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type MyTaskTaskDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type MyTaskVideoDTO struct {
	ID               uuid.UUID `json:"id"`
	Filename         string    `json:"filename"`
	OriginalFilename *string   `json:"originalFilename,omitempty"`
	StudentName      string    `json:"studentName"`
	StudentNumber    string    `json:"studentNumber"`
	Duration         *int      `json:"duration,omitempty"`
	Status           string    `json:"status"`
	PlayURL          *string   `json:"playUrl,omitempty"`
}

type MyTaskRubricDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	TotalScore int       `json:"totalScore"`
}

type MyTaskListItemDTO struct {
	ID               uuid.UUID         `json:"id"`
	Project          *MyTaskProjectDTO `json:"project,omitempty"`
	Task             *MyTaskTaskDTO    `json:"task,omitempty"`
	Video            MyTaskVideoDTO    `json:"video"`
	Rubric           *MyTaskRubricDTO  `json:"rubric,omitempty"`
	Status           string            `json:"status"`
	ManualStatus     string            `json:"manualStatus"`
	AIStatus         string            `json:"aiStatus"`
	EvaluationStatus string            `json:"evaluationStatus"`
	Deadline         *time.Time        `json:"deadline,omitempty"`
	AssignedAt       *time.Time        `json:"assignedAt,omitempty"`
}

type MyTasksPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type MyTasksResult struct {
	Items      []MyTaskListItemDTO `json:"items"`
	Pagination MyTasksPagination   `json:"pagination"`
}

type ScoringTaskDetailProjectDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ScoringTaskDetailVideoDTO struct {
	ID               uuid.UUID `json:"id"`
	PlayURL          *string   `json:"playUrl,omitempty"`
	Duration         *int      `json:"duration,omitempty"`
	StudentName      string    `json:"studentName"`
	StudentNumber    string    `json:"studentNumber"`
	Filename         string    `json:"filename"`
	OriginalFilename *string   `json:"originalFilename,omitempty"`
	Status           string    `json:"status"`
}

type ScoringTaskDetailRubricDTO struct {
	ID         uuid.UUID          `json:"id"`
	Name       string             `json:"name"`
	TotalScore int                `json:"totalScore"`
	Items      []model.RubricItem `json:"items,omitempty"`
}

type ScoringTaskManualEvaluationDTO struct {
	Score        *float64         `json:"score,omitempty"`
	ScoreDetails []map[string]any `json:"scoreDetails,omitempty"`
	Comments     *string          `json:"comments,omitempty"`
	Status       string           `json:"status"`
	StartedAt    *time.Time       `json:"startedAt,omitempty"`
	SubmittedAt  *time.Time       `json:"submittedAt,omitempty"`
}

type ScoringTaskDetailDTO struct {
	ID               uuid.UUID                       `json:"id"`
	Project          *ScoringTaskDetailProjectDTO    `json:"project,omitempty"`
	Task             *MyTaskTaskDTO                  `json:"task,omitempty"`
	Video            ScoringTaskDetailVideoDTO       `json:"video"`
	Rubric           *ScoringTaskDetailRubricDTO     `json:"rubric,omitempty"`
	AIEvaluation     any                             `json:"aiEvaluation,omitempty"`
	ManualEvaluation *ScoringTaskManualEvaluationDTO `json:"manualEvaluation,omitempty"`
	Status           string                          `json:"status"`
	ManualStatus     string                          `json:"manualStatus"`
	AIStatus         string                          `json:"aiStatus"`
	EvaluationStatus string                          `json:"evaluationStatus"`
	Deadline         *time.Time                      `json:"deadline,omitempty"`
	AssignedAt       *time.Time                      `json:"assignedAt,omitempty"`
}

type SubmitTaskInput struct {
	ScoreDetails []map[string]any `json:"scoreDetails"`
	TotalScore   float64          `json:"totalScore"`
	Comments     *string          `json:"comments"`
}

type SubmitTaskComparisonDTO struct {
	AIScore     *float64 `json:"aiScore,omitempty"`
	ManualScore *float64 `json:"manualScore,omitempty"`
	Difference  *float64 `json:"difference,omitempty"`
}

type SubmitTaskResult struct {
	ID               uuid.UUID                       `json:"id"`
	Status           string                          `json:"status"`
	ManualStatus     string                          `json:"manualStatus"`
	AIStatus         string                          `json:"aiStatus"`
	EvaluationStatus string                          `json:"evaluationStatus"`
	ManualEvaluation *ScoringTaskManualEvaluationDTO `json:"manualEvaluation,omitempty"`
	Comparison       *SubmitTaskComparisonDTO        `json:"comparison,omitempty"`
}

type SubmitSavedTaskResult struct {
	TaskID    uuid.UUID `json:"taskId"`
	Submitted int       `json:"submitted"`
}

func toAssignedTaskDTO(video *model.Video, scorer *model.User) AssignedTaskDTO {
	dto := AssignedTaskDTO{
		Video: AssignmentTaskVideoDTO{
			ID:            video.ID,
			StudentName:   video.StudentName,
			StudentNumber: video.StudentNumber,
			Filename:      video.Filename,
		},
		Scorer: AssignmentTaskScorerDTO{
			ID:       scorer.ID,
			Username: scorer.Username,
			RealName: scorer.RealName,
		},
		Status:     video.EvaluationStatus,
		AssignedAt: video.AssignedAt,
	}

	return dto
}

func toMyTaskListItemDTO(video *model.Video, playURL *string) MyTaskListItemDTO {
	item := MyTaskListItemDTO{
		ID: video.ID,
		Video: MyTaskVideoDTO{
			ID:               video.ID,
			Filename:         video.Filename,
			OriginalFilename: video.OriginalFilename,
			StudentName:      video.StudentName,
			StudentNumber:    video.StudentNumber,
			Duration:         video.Duration,
			Status:           video.Status,
			PlayURL:          playURL,
		},
		Status:           evaluation.ResolveScorerTaskStatus(video.ManualStatus),
		ManualStatus:     video.ManualStatus,
		AIStatus:         video.AIStatus,
		EvaluationStatus: video.EvaluationStatus,
		AssignedAt:       video.AssignedAt,
	}

	project := video.Project
	if project == nil && video.Task != nil {
		project = video.Task.Project
	}
	if project != nil {
		item.Project = &MyTaskProjectDTO{
			ID:   project.ID,
			Name: project.Name,
		}
	}
	if video.Task != nil {
		item.Task = &MyTaskTaskDTO{
			ID:   video.Task.ID,
			Name: video.Task.Name,
		}
		item.Deadline = video.Task.Deadline
		if video.Task.Rubric != nil {
			item.Rubric = &MyTaskRubricDTO{
				ID:         video.Task.Rubric.ID,
				Name:       video.Task.Rubric.Name,
				TotalScore: video.Task.Rubric.TotalScore,
			}
		}
	}

	return item
}

func toScoringTaskDetailDTO(video *model.Video, manual *model.ManualEvaluation, aiEvaluation any, playURL *string) *ScoringTaskDetailDTO {
	item := &ScoringTaskDetailDTO{
		ID: video.ID,
		Video: ScoringTaskDetailVideoDTO{
			ID:               video.ID,
			PlayURL:          playURL,
			Duration:         video.Duration,
			StudentName:      video.StudentName,
			StudentNumber:    video.StudentNumber,
			Filename:         video.Filename,
			OriginalFilename: video.OriginalFilename,
			Status:           video.Status,
		},
		AIEvaluation:     aiEvaluation,
		Status:           evaluation.ResolveScorerTaskStatus(video.ManualStatus),
		ManualStatus:     video.ManualStatus,
		AIStatus:         video.AIStatus,
		EvaluationStatus: video.EvaluationStatus,
		Deadline:         nil,
		AssignedAt:       video.AssignedAt,
	}

	project := video.Project
	if project == nil && video.Task != nil {
		project = video.Task.Project
	}
	if project != nil {
		item.Project = &ScoringTaskDetailProjectDTO{
			ID:   project.ID,
			Name: project.Name,
		}
	}
	if video.Task != nil {
		item.Task = &MyTaskTaskDTO{
			ID:   video.Task.ID,
			Name: video.Task.Name,
		}
		item.Deadline = video.Task.Deadline
		if video.Task.Rubric != nil {
			item.Rubric = &ScoringTaskDetailRubricDTO{
				ID:         video.Task.Rubric.ID,
				Name:       video.Task.Rubric.Name,
				TotalScore: video.Task.Rubric.TotalScore,
				Items:      video.Task.Rubric.Items,
			}
		}
	}
	if manual != nil {
		item.ManualEvaluation = &ScoringTaskManualEvaluationDTO{
			Score:        manual.TotalScore,
			ScoreDetails: manual.ScoreDetails,
			Comments:     manual.Comments,
			Status:       manual.Status,
			StartedAt:    manual.StartedAt,
			SubmittedAt:  manual.SubmittedAt,
		}
	}

	return item
}

func toSubmitTaskResult(video *model.Video, manual *model.ManualEvaluation) *SubmitTaskResult {
	result := &SubmitTaskResult{
		ID:               video.ID,
		Status:           evaluation.ResolveScorerTaskStatus(video.ManualStatus),
		ManualStatus:     video.ManualStatus,
		AIStatus:         video.AIStatus,
		EvaluationStatus: video.EvaluationStatus,
	}
	if manual != nil {
		result.ManualEvaluation = &ScoringTaskManualEvaluationDTO{
			Score:        manual.TotalScore,
			ScoreDetails: manual.ScoreDetails,
			Comments:     manual.Comments,
			Status:       manual.Status,
			StartedAt:    manual.StartedAt,
			SubmittedAt:  manual.SubmittedAt,
		}
	}
	if video.AIScore != nil || video.ManualScore != nil {
		comparison := &SubmitTaskComparisonDTO{
			AIScore:     video.AIScore,
			ManualScore: video.ManualScore,
		}
		if video.AIScore != nil && video.ManualScore != nil {
			diff := *video.ManualScore - *video.AIScore
			comparison.Difference = &diff
		}
		result.Comparison = comparison
	}

	return result
}
