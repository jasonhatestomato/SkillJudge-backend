package ai

import (
	"context"
	"errors"
	"time"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*model.AIEvaluation, error) {
	var item model.AIEvaluation
	if err := r.db.WithContext(ctx).
		Model(&model.AIEvaluation{}).
		Where("id = ?", id).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindLatestByVideoID(ctx context.Context, videoID uuid.UUID) (*model.AIEvaluation, error) {
	var item model.AIEvaluation
	if err := r.db.WithContext(ctx).
		Model(&model.AIEvaluation{}).
		Where("video_id = ?", videoID).
		Order("created_at DESC").
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindVideoByID(ctx context.Context, id uuid.UUID) (*model.Video, error) {
	var item model.Video
	if err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Project.School").
		Preload("Task.Rubric").
		Where("videos.id = ?", id).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindVideosByTaskAndIDs(ctx context.Context, taskID uuid.UUID, videoIDs []uuid.UUID) ([]model.Video, error) {
	var items []model.Video
	if err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Project.School").
		Preload("Task.Rubric").
		Where("task_id = ?", taskID).
		Where("id IN ?", videoIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) CreateAndMarkVideoProcessing(ctx context.Context, evaluation *model.AIEvaluation, videoID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(evaluation).Error; err != nil {
			return err
		}
		return tx.Model(&model.Video{}).
			Where("id = ?", videoID).
			Updates(map[string]any{
				"ai_status":  EvaluationStatusProcessing,
				"updated_at": now,
			}).Error
	})
}

func (r *Repository) UpdateEvaluationAndVideo(ctx context.Context, evaluationID uuid.UUID, updates map[string]any, videoUpdates map[string]any) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AIEvaluation{}).
			Where("id = ?", evaluationID).
			Updates(updates).Error; err != nil {
			return err
		}
		if len(videoUpdates) > 0 {
			if err := tx.Model(&model.Video{}).
				Where("id = (SELECT video_id FROM ai_evaluations WHERE id = ?)", evaluationID).
				Updates(videoUpdates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SetJobID(ctx context.Context, evaluationID uuid.UUID, jobID string) error {
	return r.db.WithContext(ctx).
		Model(&model.AIEvaluation{}).
		Where("id = ?", evaluationID).
		Updates(map[string]any{
			"job_id":     jobID,
			"updated_at": time.Now(),
		}).Error
}

func (r *Repository) ListPollingCandidates(ctx context.Context, limit int) ([]model.AIEvaluation, error) {
	if limit <= 0 {
		limit = 20
	}

	var items []model.AIEvaluation
	if err := r.db.WithContext(ctx).
		Model(&model.AIEvaluation{}).
		Where("status = ?", EvaluationStatusProcessing).
		Where("job_id IS NOT NULL").
		Where("job_id <> ''").
		Order("updated_at ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}
