package video

import (
	"context"
	"errors"
	"strings"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type ListParams struct {
	ProjectID          uuid.UUID
	TaskID             *uuid.UUID
	Page               int
	PageSize           int
	Status             string
	StudentID          *uuid.UUID
	StudentOwnerID     *uuid.UUID
	StudentOwnerNumber string
	SchoolID           *uuid.UUID
	ScorerID           *uuid.UUID
	StudentNumber      string
	Keyword            string
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, item *model.Video) error {
	return r.db.WithContext(ctx).
		Omit("ProjectID", "SchoolID", "StorageURL", "UploadID", "CreatorID").
		Create(item).Error
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*model.Video, error) {
	var item model.Video
	err := r.db.WithContext(ctx).
		Preload("Project").
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Project.School").
		Preload("Task.Rubric").
		Preload("Task.Creator").
		Preload("Scorer").
		Preload("Creator").
		Preload("School").
		Where("id = ?", id).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindLatestManualEvaluation(ctx context.Context, videoID uuid.UUID) (*model.ManualEvaluation, error) {
	var item model.ManualEvaluation
	err := r.db.WithContext(ctx).
		Model(&model.ManualEvaluation{}).
		Where("video_id = ?", videoID).
		Order("submitted_at DESC NULLS LAST, updated_at DESC").
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	delete(updates, "storage_url")
	delete(updates, "upload_id")
	return r.db.WithContext(ctx).Model(&model.Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Video{}, "id = ?", id).Error
}

func (r *Repository) List(ctx context.Context, params ListParams) ([]model.Video, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Preload("Project").
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Project.School").
		Preload("Scorer").
		Preload("Creator")

	if params.TaskID != nil {
		query = query.Where("task_id = ?", *params.TaskID)
	} else if params.ProjectID != uuid.Nil {
		query = query.Joins("JOIN tasks ON tasks.id = videos.task_id").
			Where("tasks.project_id = ?", params.ProjectID)
	}

	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.StudentID != nil {
		query = query.Where("student_id = ?", *params.StudentID)
	}
	if params.StudentOwnerID != nil || strings.TrimSpace(params.StudentOwnerNumber) != "" {
		ownerQuery := r.db.WithContext(ctx).Where("1 = 0")
		if params.StudentOwnerID != nil {
			ownerQuery = ownerQuery.Or("videos.student_id = ?", *params.StudentOwnerID)
		}
		if strings.TrimSpace(params.StudentOwnerNumber) != "" {
			ownerQuery = ownerQuery.Or("videos.student_number = ?", strings.TrimSpace(params.StudentOwnerNumber))
		}
		query = query.Where(ownerQuery)
	}
	if params.SchoolID != nil {
		query = query.
			Joins("JOIN tasks AS school_scope_tasks ON school_scope_tasks.id = videos.task_id").
			Joins("JOIN projects AS school_scope_projects ON school_scope_projects.id = school_scope_tasks.project_id").
			Where("school_scope_projects.school_id = ?", *params.SchoolID)
	}
	if params.ScorerID != nil {
		query = query.Where("scorer_id = ?", *params.ScorerID)
	}
	if params.StudentNumber != "" {
		query = query.Where("student_number = ?", params.StudentNumber)
	}
	if params.Keyword != "" {
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		query = query.Where("filename ILIKE ? OR student_name ILIKE ? OR student_number ILIKE ?", keyword, keyword, keyword)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var items []model.Video
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
