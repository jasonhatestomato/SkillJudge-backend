package task

import (
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type TaskDTO struct {
	ID                       uuid.UUID       `json:"id"`
	Name                     string          `json:"name"`
	Description              *string         `json:"description,omitempty"`
	Status                   string          `json:"status"`
	StartDate                *time.Time      `json:"startDate,omitempty"`
	Deadline                 *time.Time      `json:"deadline,omitempty"`
	EndDate                  *time.Time      `json:"endDate,omitempty"`
	TotalVideos              int             `json:"totalVideos"`
	CompletedVideos          int             `json:"completedVideos"`
	AllVideosCompleted       bool            `json:"allVideosCompleted"`
	CompletionRate           float64         `json:"completionRate"`
	ManualCompletedVideos    int             `json:"manualCompletedVideos"`
	AllManualVideosCompleted bool            `json:"allManualVideosCompleted"`
	ManualCompletionRate     float64         `json:"manualCompletionRate"`
	Project                  *TaskProjectDTO `json:"project,omitempty"`
	Rubric                   *TaskRubricDTO  `json:"rubric,omitempty"`
	Creator                  *TaskCreatorDTO `json:"creator,omitempty"`
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
	Metadata                 map[string]any  `json:"metadata,omitempty"`
	School                   *user.SchoolDTO `json:"school,omitempty"`
}

type TaskProjectDTO struct {
	ID     uuid.UUID       `json:"id"`
	Name   string          `json:"name"`
	School *user.SchoolDTO `json:"school,omitempty"`
}

type TaskRubricDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	TotalScore int       `json:"totalScore"`
}

type TaskCreatorDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	RealName *string   `json:"realName,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type ListTasksResult struct {
	Items      []TaskDTO  `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type ScoreboardTaskDTO struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Status             string    `json:"status"`
	TotalVideos        int       `json:"totalVideos"`
	CompletedVideos    int       `json:"completedVideos"`
	AllVideosCompleted bool      `json:"allVideosCompleted"`
	CompletionRate     float64   `json:"completionRate"`
}

type ScoreboardSummaryDTO struct {
	TotalStudents      int64    `json:"totalStudents"`
	CompletedStudents  int64    `json:"completedStudents"`
	AverageAIScore     *float64 `json:"averageAIScore,omitempty"`
	AverageManualScore *float64 `json:"averageManualScore,omitempty"`
}

type ScoreboardItemDTO struct {
	VideoID          uuid.UUID  `json:"videoId"`
	StudentName      string     `json:"studentName"`
	StudentNumber    string     `json:"studentNumber"`
	AIScore          *float64   `json:"aiScore,omitempty"`
	ManualScore      *float64   `json:"manualScore,omitempty"`
	AIStatus         string     `json:"aiStatus"`
	ManualStatus     string     `json:"manualStatus"`
	EvaluationStatus string     `json:"evaluationStatus"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
	DisplayStatus    string     `json:"displayStatus"`
}

type ScoreboardResult struct {
	Task       ScoreboardTaskDTO    `json:"task"`
	Summary    ScoreboardSummaryDTO `json:"summary"`
	Items      []ScoreboardItemDTO  `json:"items"`
	Pagination Pagination           `json:"pagination"`
}

type AnalysisTaskDTO struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	RubricTotalScore   int       `json:"rubricTotalScore"`
	TotalVideos        int       `json:"totalVideos"`
	CompletedVideos    int       `json:"completedVideos"`
	AllVideosCompleted bool      `json:"allVideosCompleted"`
}

type AnalysisScoreSummaryDTO struct {
	AverageAIScore     *float64 `json:"averageAIScore,omitempty"`
	AverageManualScore *float64 `json:"averageManualScore,omitempty"`
	HighestAIScore     *float64 `json:"highestAIScore,omitempty"`
	LowestAIScore      *float64 `json:"lowestAIScore,omitempty"`
	HighestManualScore *float64 `json:"highestManualScore,omitempty"`
	LowestManualScore  *float64 `json:"lowestManualScore,omitempty"`
}

type AnalysisDistributionBucketDTO struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type AnalysisResult struct {
	Task                    AnalysisTaskDTO                 `json:"task"`
	ScoreSummary            AnalysisScoreSummaryDTO         `json:"scoreSummary"`
	ManualScoreDistribution []AnalysisDistributionBucketDTO `json:"manualScoreDistribution"`
	AIScoreDistribution     []AnalysisDistributionBucketDTO `json:"aiScoreDistribution"`
	ScoreGapDistribution    []AnalysisDistributionBucketDTO `json:"scoreGapDistribution"`
}

type AnalysisReportDTO struct {
	ID              uuid.UUID  `json:"reportId"`
	TaskID          uuid.UUID  `json:"taskId"`
	ReportStatus    string     `json:"reportStatus"`
	ReportFormat    string     `json:"reportFormat"`
	FileName        *string    `json:"fileName,omitempty"`
	URL             *string    `json:"url,omitempty"`
	TemplateVersion *string    `json:"templateVersion,omitempty"`
	GeneratedAt     *time.Time `json:"generatedAt,omitempty"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type Context struct {
	TaskID      uuid.UUID
	TaskName    string
	TaskStatus  string
	ProjectID   uuid.UUID
	ProjectName string
	SchoolID    *uuid.UUID
	SchoolName  *string
	RubricID    uuid.UUID
	RubricName  string
	CreatorID   *uuid.UUID
	CreatorName *string
	CreatorRole *string
	Item        *model.Task
}

func ToTaskDTO(item *model.Task) *TaskDTO {
	dto := &TaskDTO{
		ID:                 item.ID,
		Name:               item.Name,
		Description:        item.Description,
		Status:             item.Status,
		StartDate:          item.StartDate,
		Deadline:           item.Deadline,
		EndDate:            item.EndDate,
		TotalVideos:        item.TotalVideos,
		CompletedVideos:    item.CompletedVideos,
		AllVideosCompleted: item.TotalVideos > 0 && item.TotalVideos == item.CompletedVideos,
		CompletionRate:     completionRate(item.TotalVideos, item.CompletedVideos),
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		Metadata:           item.Metadata,
	}

	if item.Project != nil {
		dto.Project = &TaskProjectDTO{
			ID:   item.Project.ID,
			Name: item.Project.Name,
		}
		if item.Project.School != nil {
			school := &user.SchoolDTO{
				ID:   item.Project.School.ID,
				Name: item.Project.School.Name,
			}
			dto.School = school
			dto.Project.School = school
		}
	}
	if item.Rubric != nil {
		dto.Rubric = &TaskRubricDTO{
			ID:         item.Rubric.ID,
			Name:       item.Rubric.Name,
			TotalScore: item.Rubric.TotalScore,
		}
	}
	if item.Creator != nil {
		dto.Creator = &TaskCreatorDTO{
			ID:       item.Creator.ID,
			Username: item.Creator.Username,
			RealName: item.Creator.RealName,
		}
	}

	return dto
}

func applyManualProgress(dto *TaskDTO, totalVideos int, manualCompletedVideos int) {
	if dto == nil {
		return
	}
	dto.ManualCompletedVideos = manualCompletedVideos
	dto.AllManualVideosCompleted = totalVideos > 0 && totalVideos == manualCompletedVideos
	dto.ManualCompletionRate = completionRate(totalVideos, manualCompletedVideos)
}

func ToContext(item *model.Task) *Context {
	ctx := &Context{
		TaskID:     item.ID,
		TaskName:   item.Name,
		TaskStatus: item.Status,
		RubricID:   item.RubricID,
		CreatorID:  item.CreatorID,
		Item:       item,
	}

	if item.Project != nil {
		ctx.ProjectID = item.Project.ID
		ctx.ProjectName = item.Project.Name
		ctx.SchoolID = item.Project.SchoolID
		if item.Project.School != nil {
			ctx.SchoolName = &item.Project.School.Name
		}
	}
	if item.Rubric != nil {
		ctx.RubricName = item.Rubric.Name
	}
	if item.Creator != nil {
		ctx.CreatorName = &item.Creator.Username
		ctx.CreatorRole = &item.Creator.Role
	}

	return ctx
}

func ToScoreboardTaskDTO(item *model.Task) ScoreboardTaskDTO {
	return ScoreboardTaskDTO{
		ID:                 item.ID,
		Name:               item.Name,
		Status:             item.Status,
		TotalVideos:        item.TotalVideos,
		CompletedVideos:    item.CompletedVideos,
		AllVideosCompleted: item.TotalVideos > 0 && item.TotalVideos == item.CompletedVideos,
		CompletionRate:     completionRate(item.TotalVideos, item.CompletedVideos),
	}
}

func ToScoreboardItemDTO(item *model.Video) ScoreboardItemDTO {
	return ScoreboardItemDTO{
		VideoID:          item.ID,
		StudentName:      item.StudentName,
		StudentNumber:    item.StudentNumber,
		AIScore:          item.AIScore,
		ManualScore:      item.ManualScore,
		AIStatus:         item.AIStatus,
		ManualStatus:     item.ManualStatus,
		EvaluationStatus: item.EvaluationStatus,
		CompletedAt:      item.CompletedAt,
		DisplayStatus:    scoreboardDisplayStatus(item.EvaluationStatus),
	}
}

func ToAnalysisTaskDTO(item *model.Task) AnalysisTaskDTO {
	rubricTotalScore := 0
	if item.Rubric != nil {
		rubricTotalScore = item.Rubric.TotalScore
	}

	return AnalysisTaskDTO{
		ID:                 item.ID,
		Name:               item.Name,
		RubricTotalScore:   rubricTotalScore,
		TotalVideos:        item.TotalVideos,
		CompletedVideos:    item.CompletedVideos,
		AllVideosCompleted: item.TotalVideos > 0 && item.TotalVideos == item.CompletedVideos,
	}
}

func ToAnalysisReportDTO(item *model.TaskAnalysisReport) *AnalysisReportDTO {
	if item == nil {
		return nil
	}

	return &AnalysisReportDTO{
		ID:              item.ID,
		TaskID:          item.TaskID,
		ReportStatus:    item.Status,
		ReportFormat:    item.ReportFormat,
		FileName:        item.FileName,
		URL:             item.PublicURL,
		TemplateVersion: item.TemplateVersion,
		GeneratedAt:     item.GeneratedAt,
		ErrorMessage:    item.ErrorMessage,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func scoreboardDisplayStatus(evaluationStatus string) string {
	if evaluationStatus == "completed" {
		return "已完成"
	}
	return "未完成"
}

func completionRate(total, completed int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(completed) * 100 / float64(total)
}
