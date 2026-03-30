package task

import (
	"context"
	"errors"
	"strings"
	"time"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
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
