package project

import (
	"context"
	"errors"
	"strings"
	"time"

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
	Status    string
	SchoolID  *uuid.UUID
	CreatorID *uuid.UUID
	Keyword   string
	StartDate *time.Time
	EndDate   *time.Time
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, item *model.Project) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	var item model.Project
	err := r.db.WithContext(ctx).
		Preload("School").
		Preload("Creator").
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

func (r *Repository) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.Project{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Project{}, "id = ?", id).Error
}

func (r *Repository) List(ctx context.Context, params ListParams, actorRole string, actorUserID uuid.UUID, actorSchoolID *uuid.UUID) ([]model.Project, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Project{}).
		Preload("School").
		Preload("Creator")

	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	if params.Keyword != "" {
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", keyword, keyword)
	}

	if params.StartDate != nil {
		query = query.Where("created_at >= ?", *params.StartDate)
	}
	if params.EndDate != nil {
		query = query.Where("created_at <= ?", *params.EndDate)
	}

	switch actorRole {
	case "admin":
		if params.SchoolID != nil {
			query = query.Where("school_id = ?", *params.SchoolID)
		}
		if params.CreatorID != nil {
			query = query.Where("creator_id = ?", *params.CreatorID)
		}
	case "school_admin", "school_leader":
		if actorSchoolID != nil {
			query = query.Where("school_id = ?", *actorSchoolID)
		}
		if params.CreatorID != nil {
			query = query.Where("creator_id = ?", *params.CreatorID)
		}
	case "teacher":
		query = query.Where("creator_id = ?", actorUserID)
	default:
		query = query.Where("1 = 0")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var items []model.Project
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
