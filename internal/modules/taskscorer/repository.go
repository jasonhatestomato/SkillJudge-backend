package taskscorer

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

type scorerNotificationRow struct {
	model.TaskScorerInvitation
	TaskName    string
	ProjectID   uuid.UUID
	ProjectName string
	SchoolName  *string
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListTaskScorers(ctx context.Context, taskID uuid.UUID) ([]model.TaskScorer, error) {
	var items []model.TaskScorer
	err := r.db.WithContext(ctx).
		Model(&model.TaskScorer{}).
		Preload("Scorer").
		Joins("LEFT JOIN users AS scorers ON scorers.id = task_scorers.scorer_id").
		Where("task_scorers.task_id = ?", taskID).
		Order("scorers.real_name ASC NULLS LAST, scorers.username ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) FindTaskScorer(ctx context.Context, taskID, scorerID uuid.UUID) (*model.TaskScorer, error) {
	var item model.TaskScorer
	err := r.db.WithContext(ctx).
		Model(&model.TaskScorer{}).
		Preload("Scorer").
		Where("task_id = ?", taskID).
		Where("scorer_id = ?", scorerID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) CreateTaskScorer(ctx context.Context, item *model.TaskScorer) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) UpdateTaskScorer(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&model.TaskScorer{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *Repository) DeactivateTaskScorer(ctx context.Context, taskID, scorerID uuid.UUID, removedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.TaskScorer{}).
		Where("task_id = ?", taskID).
		Where("scorer_id = ?", scorerID).
		Updates(map[string]any{
			"status":     "inactive",
			"removed_at": removedAt,
			"updated_at": removedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskScorerRelationNotFound
	}

	return nil
}

func (r *Repository) CreateInvitation(ctx context.Context, item *model.TaskScorerInvitation) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) FindInvitationByTokenHash(ctx context.Context, tokenHash string) (*model.TaskScorerInvitation, error) {
	var item model.TaskScorerInvitation
	err := r.db.WithContext(ctx).
		Model(&model.TaskScorerInvitation{}).
		Where("token_hash = ?", tokenHash).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindInvitationByIDAndScorer(ctx context.Context, invitationID, scorerID uuid.UUID) (*model.TaskScorerInvitation, error) {
	var item model.TaskScorerInvitation
	err := r.db.WithContext(ctx).
		Model(&model.TaskScorerInvitation{}).
		Where("id = ?", invitationID).
		Where("scorer_id = ?", scorerID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) UpdateInvitation(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&model.TaskScorerInvitation{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *Repository) CancelSentInvitations(ctx context.Context, taskID, scorerID uuid.UUID, updatedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.TaskScorerInvitation{}).
		Where("task_id = ?", taskID).
		Where("scorer_id = ?", scorerID).
		Where("status = ?", "sent").
		Updates(map[string]any{
			"status":     "cancelled",
			"updated_at": updatedAt,
		}).Error
}

func (r *Repository) FindLatestInvitationsByTaskAndScorerIDs(ctx context.Context, taskID uuid.UUID, scorerIDs []uuid.UUID) (map[uuid.UUID]*model.TaskScorerInvitation, error) {
	result := make(map[uuid.UUID]*model.TaskScorerInvitation)
	if len(scorerIDs) == 0 {
		return result, nil
	}

	var items []model.TaskScorerInvitation
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (scorer_id) *
		FROM task_scorer_invitations
		WHERE task_id = ? AND scorer_id IN ?
		ORDER BY scorer_id, created_at DESC
	`, taskID, scorerIDs).Scan(&items).Error
	if err != nil {
		return nil, err
	}

	for i := range items {
		item := items[i]
		result[item.ScorerID] = &item
	}

	return result, nil
}

func (r *Repository) ListLatestInvitationsByScorer(ctx context.Context, scorerID uuid.UUID) ([]scorerNotificationRow, error) {
	var items []scorerNotificationRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (tsi.task_id)
			tsi.*,
			t.name AS task_name,
			p.id AS project_id,
			p.name AS project_name,
			s.name AS school_name
		FROM task_scorer_invitations tsi
		JOIN tasks t ON t.id = tsi.task_id
		JOIN projects p ON p.id = t.project_id
		LEFT JOIN schools s ON s.id = p.school_id
		JOIN task_scorers ts ON ts.task_id = tsi.task_id AND ts.scorer_id = tsi.scorer_id
		WHERE tsi.scorer_id = ?
		  AND ts.status <> 'inactive'
		ORDER BY tsi.task_id, tsi.created_at DESC
	`, scorerID).Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}
