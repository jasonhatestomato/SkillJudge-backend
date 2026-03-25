package task

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
	return r.db.WithContext(ctx).Create(item).Error
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

func (r *Repository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.Task{}).
		Preload("Project").
		Preload("Project.School").
		Preload("Rubric").
		Preload("Creator")
}
