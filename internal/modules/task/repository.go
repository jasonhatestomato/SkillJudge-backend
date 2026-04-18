package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type ListParams struct {
	Page      int
	PageSize  int
	ProjectID uuid.UUID
	Status    string
	Keyword   string
}

type ScoreboardSummary struct {
	TotalStudents      int64    `json:"totalStudents"`
	CompletedStudents  int64    `json:"completedStudents"`
	AverageAIScore     *float64 `json:"averageAIScore,omitempty"`
	AverageManualScore *float64 `json:"averageManualScore,omitempty"`
}

type CreateAnalysisReportInput struct {
	TaskID          uuid.UUID
	Status          string
	ReportFormat    string
	FileName        *string
	StoragePath     *string
	PublicURL       *string
	TemplateVersion *string
	SnapshotData    map[string]any
	RequestedBy     *uuid.UUID
	StartedAt       *time.Time
	GeneratedAt     *time.Time
	ErrorMessage    *string
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, item *model.Task) error {
	err := r.db.WithContext(ctx).Create(item).Error
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrTaskNameConflict
	}

	return err
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	var item model.Task
	err := r.baseQuery(ctx).Where("tasks.id = ?", id).First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) List(ctx context.Context, params ListParams) ([]model.Task, int64, error) {
	query := r.baseQuery(ctx).Where("tasks.project_id = ?", params.ProjectID)
	if params.Status != "" {
		query = query.Where("tasks.status = ?", params.Status)
	}
	if params.Keyword != "" {
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		query = query.Where("tasks.name ILIKE ? OR tasks.description ILIKE ?", keyword, keyword)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var items []model.Task
	if err := query.Order("tasks.created_at DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) ListScoreboard(ctx context.Context, params ScoreboardParams) ([]model.Video, int64, *ScoreboardSummary, error) {
	query := r.scoreboardBaseQuery(ctx, params)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, nil, err
	}

	type summaryRow struct {
		TotalStudents      int64
		CompletedStudents  int64
		AverageAIScore     sql.NullFloat64
		AverageManualScore sql.NullFloat64
	}

	var row summaryRow
	if err := r.scoreboardBaseQuery(ctx, params).
		Select(
			"COUNT(*) AS total_students, " +
				"COALESCE(SUM(CASE WHEN evaluation_status = 'completed' THEN 1 ELSE 0 END), 0) AS completed_students, " +
				"AVG(ai_score) AS average_ai_score, " +
				"AVG(manual_score) AS average_manual_score",
		).
		Scan(&row).Error; err != nil {
		return nil, 0, nil, err
	}

	summary := &ScoreboardSummary{
		TotalStudents:     row.TotalStudents,
		CompletedStudents: row.CompletedStudents,
	}
	if row.AverageAIScore.Valid {
		value := row.AverageAIScore.Float64
		summary.AverageAIScore = &value
	}
	if row.AverageManualScore.Valid {
		value := row.AverageManualScore.Float64
		summary.AverageManualScore = &value
	}

	ordered := r.applyScoreboardOrder(query, params.SortBy, params.SortOrder)

	var items []model.Video
	if params.Scope == "all" {
		if err := ordered.Find(&items).Error; err != nil {
			return nil, 0, nil, err
		}
		return items, total, summary, nil
	}

	offset := (params.Page - 1) * params.PageSize
	if err := ordered.Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, 0, nil, err
	}

	return items, total, summary, nil
}

func (r *Repository) ListAnalysisVideos(ctx context.Context, taskID uuid.UUID) ([]model.Video, error) {
	var items []model.Video
	err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Where("task_id = ?", taskID).
		Where("status = ?", "ready").
		Order("student_number ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) FindLatestAnalysisReportByTaskID(ctx context.Context, taskID uuid.UUID) (*model.TaskAnalysisReport, error) {
	var item model.TaskAnalysisReport
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at DESC").
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) CreateAnalysisReport(ctx context.Context, input CreateAnalysisReportInput) (*model.TaskAnalysisReport, error) {
	item := &model.TaskAnalysisReport{
		TaskID:          input.TaskID,
		Status:          input.Status,
		ReportFormat:    input.ReportFormat,
		FileName:        input.FileName,
		StoragePath:     input.StoragePath,
		PublicURL:       input.PublicURL,
		TemplateVersion: input.TemplateVersion,
		SnapshotData:    input.SnapshotData,
		RequestedBy:     input.RequestedBy,
		StartedAt:       input.StartedAt,
		GeneratedAt:     input.GeneratedAt,
		ErrorMessage:    input.ErrorMessage,
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) EnsureQueuedAnalysisReport(ctx context.Context, taskID uuid.UUID, requestedBy *uuid.UUID, templateVersion *string) (*model.TaskAnalysisReport, bool, error) {
	var (
		result  *model.TaskAnalysisReport
		created bool
	)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var taskItem model.Task
		if err := tx.Model(&model.Task{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", taskID).
			First(&taskItem).Error; err != nil {
			return err
		}

		var existing model.TaskAnalysisReport
		err := tx.Where("task_id = ?", taskID).Order("created_at DESC").First(&existing).Error
		switch {
		case err == nil:
			if existing.Status == taskAnalysisReportStatusQueued || existing.Status == taskAnalysisReportStatusProcessing || existing.Status == taskAnalysisReportStatusReady {
				result = &existing
				return nil
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		item := &model.TaskAnalysisReport{
			TaskID:          taskID,
			Status:          taskAnalysisReportStatusQueued,
			ReportFormat:    taskAnalysisReportFormatPDF,
			TemplateVersion: templateVersion,
			RequestedBy:     requestedBy,
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		result = item
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return result, created, nil
}

func (r *Repository) UpdateAnalysisReport(ctx context.Context, reportID uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&model.TaskAnalysisReport{}).
		Where("id = ?", reportID).
		Updates(updates).Error
}

func (r *Repository) ClaimAnalysisReport(ctx context.Context, reportID uuid.UUID, startedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.TaskAnalysisReport{}).
		Where("id = ?", reportID).
		Where("status = ?", taskAnalysisReportStatusQueued).
		Updates(map[string]any{
			"status":     taskAnalysisReportStatusProcessing,
			"started_at": startedAt,
			"updated_at": startedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *Repository) ListQueuedAnalysisReports(ctx context.Context, limit int) ([]model.TaskAnalysisReport, error) {
	if limit <= 0 {
		limit = 10
	}

	var items []model.TaskAnalysisReport
	if err := r.db.WithContext(ctx).
		Where("status = ?", taskAnalysisReportStatusQueued).
		Order("created_at ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) RefreshVideoStats(ctx context.Context, taskID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var taskItem model.Task
		if err := tx.Model(&model.Task{}).
			Select("id", "project_id").
			Where("id = ?", taskID).
			First(&taskItem).Error; err != nil {
			return err
		}

		taskTotal, taskCompleted, err := countTaskVideoStats(tx, taskID)
		if err != nil {
			return err
		}

		now := time.Now()
		taskUpdates := map[string]any{
			"total_videos":     taskTotal,
			"completed_videos": taskCompleted,
			"updated_at":       now,
		}
		if err := tx.Model(&model.Task{}).Where("id = ?", taskID).Updates(taskUpdates).Error; err != nil {
			return err
		}

		projectTotal, projectCompleted, err := countProjectVideoStats(tx, taskItem.ProjectID)
		if err != nil {
			return err
		}

		projectUpdates := map[string]any{
			"total_videos":     projectTotal,
			"completed_videos": projectCompleted,
			"updated_at":       now,
		}
		if err := tx.Model(&model.Project{}).Where("id = ?", taskItem.ProjectID).Updates(projectUpdates).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.Task{}).
		Preload("Project").
		Preload("Project.School").
		Preload("Rubric").
		Preload("Creator")
}

func (r *Repository) scoreboardBaseQuery(ctx context.Context, params ScoreboardParams) *gorm.DB {
	query := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Where("task_id = ?", params.TaskID).
		Where("status = ?", "ready")

	if params.Keyword != "" {
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		query = query.Where("student_name ILIKE ? OR student_number ILIKE ?", keyword, keyword)
	}

	if params.EvaluationStatus != "" {
		query = query.Where("evaluation_status = ?", params.EvaluationStatus)
	}

	return query
}

func (r *Repository) applyScoreboardOrder(query *gorm.DB, sortBy, sortOrder string) *gorm.DB {
	order := "ASC"
	if strings.EqualFold(sortOrder, "desc") {
		order = "DESC"
	}

	column := "student_number"
	switch sortBy {
	case "studentName":
		column = "student_name"
	case "aiScore":
		column = "ai_score"
	case "manualScore":
		column = "manual_score"
	case "completedAt":
		column = "completed_at"
	case "studentNumber", "":
		column = "student_number"
	}

	if column == "ai_score" || column == "manual_score" || column == "completed_at" {
		return query.Order(fmt.Sprintf("%s %s NULLS LAST", column, order)).Order("student_number ASC")
	}

	return query.Order(fmt.Sprintf("%s %s", column, order))
}

func countTaskVideoStats(db *gorm.DB, taskID uuid.UUID) (int64, int64, error) {
	var total int64
	if err := db.Model(&model.Video{}).
		Where("task_id = ?", taskID).
		Where("status = ?", "ready").
		Count(&total).Error; err != nil {
		return 0, 0, err
	}

	var completed int64
	if err := db.Model(&model.Video{}).
		Where("task_id = ?", taskID).
		Where("status = ?", "ready").
		Where("evaluation_status = ?", "completed").
		Count(&completed).Error; err != nil {
		return 0, 0, err
	}

	return total, completed, nil
}

func countProjectVideoStats(db *gorm.DB, projectID uuid.UUID) (int64, int64, error) {
	projectVideos := db.Model(&model.Video{}).
		Joins("JOIN tasks ON tasks.id = videos.task_id").
		Where("tasks.project_id = ?", projectID).
		Where("videos.status = ?", "ready")

	var total int64
	if err := projectVideos.Count(&total).Error; err != nil {
		return 0, 0, err
	}

	var completed int64
	if err := db.Model(&model.Video{}).
		Joins("JOIN tasks ON tasks.id = videos.task_id").
		Where("tasks.project_id = ?", projectID).
		Where("videos.status = ?", "ready").
		Where("videos.evaluation_status = ?", "completed").
		Count(&completed).Error; err != nil {
		return 0, 0, err
	}

	return total, completed, nil
}

func (r *Repository) CountManualSubmittedByTaskIDs(ctx context.Context, taskIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	result := make(map[uuid.UUID]int64, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}

	type row struct {
		TaskID uuid.UUID
		Count  int64
	}

	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Select("task_id, COUNT(*) AS count").
		Where("task_id IN ?", taskIDs).
		Where("status = ?", "ready").
		Where("manual_status = ?", "submitted").
		Group("task_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, item := range rows {
		result[item.TaskID] = item.Count
	}

	return result, nil
}
