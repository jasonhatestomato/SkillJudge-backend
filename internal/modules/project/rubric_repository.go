package project

import (
	"context"
	"errors"
	"strings"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RubricListParams struct {
	Page       int
	PageSize   int
	IsTemplate *bool
	Keyword    string
}

func (r *Repository) CreateRubric(ctx context.Context, item *model.ScoringRubric) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) FindRubricByID(ctx context.Context, id uuid.UUID) (*model.ScoringRubric, error) {
	var item model.ScoringRubric
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

func (r *Repository) UpdateRubric(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.ScoringRubric{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) DeleteRubric(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.ScoringRubric{}, "id = ?", id).Error
}

func (r *Repository) ListRubrics(ctx context.Context, params RubricListParams, actorRole string, actorUserID uuid.UUID, actorSchoolID *uuid.UUID) ([]model.ScoringRubric, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ScoringRubric{}).Preload("School").Preload("Creator")

	if params.IsTemplate != nil {
		query = query.Where("is_template = ?", *params.IsTemplate)
	}
	if params.Keyword != "" {
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", keyword, keyword)
	}

	switch actorRole {
	case "admin":
	case "school_admin", "school_leader":
		if actorSchoolID != nil {
			query = query.Where("(school_id = ? OR is_public = true)", *actorSchoolID)
		} else {
			query = query.Where("is_public = true")
		}
	case "teacher":
		if actorSchoolID != nil {
			query = query.Where("(creator_id = ? OR school_id = ? OR is_public = true)", actorUserID, *actorSchoolID)
		} else {
			query = query.Where("(creator_id = ? OR is_public = true)", actorUserID)
		}
	default:
		query = query.Where("is_public = true")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var items []model.ScoringRubric
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
