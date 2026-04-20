package scoring

import (
	"context"
	"errors"
	"strings"
	"time"

	"skilljudge/backend/internal/domain/evaluation"
	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

const videoSelectableColumns = "videos.*"

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindVideosByTaskAndIDs(ctx context.Context, taskID uuid.UUID, videoIDs []uuid.UUID) ([]model.Video, error) {
	var items []model.Video
	if err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Preload("Scorer").
		Where("task_id = ?", taskID).
		Where("id IN ?", videoIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) FindAssignableScorers(ctx context.Context, schoolID *uuid.UUID, scorerIDs []uuid.UUID) ([]model.User, error) {
	query := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("DISTINCT users.*").
		Joins("join user_roles on user_roles.user_id = users.id and user_roles.status = ?", "active").
		Joins("join roles on roles.id = user_roles.role_id and roles.status = ?", "active").
		Where("users.id IN ?", scorerIDs).
		Where("users.status = ?", "active").
		Where("roles.code = ?", "scorer")

	if schoolID != nil {
		query = query.Where("users.school_id = ?", *schoolID)
	}

	var users []model.User
	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (r *Repository) ListAssignableScorers(ctx context.Context, schoolID *uuid.UUID) ([]model.User, error) {
	query := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("DISTINCT users.*").
		Joins("join user_roles on user_roles.user_id = users.id and user_roles.status = ?", "active").
		Joins("join roles on roles.id = user_roles.role_id and roles.status = ?", "active").
		Where("users.status = ?", "active").
		Where("roles.code = ?", "scorer")

	if schoolID != nil {
		query = query.Where("users.school_id = ?", *schoolID)
	}

	var users []model.User
	if err := query.Order("users.real_name ASC NULLS LAST, users.username ASC").Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (r *Repository) AssignScorers(ctx context.Context, videos []model.Video, assignments map[uuid.UUID]uuid.UUID, assignedAt time.Time) error {
	videoByID := make(map[uuid.UUID]model.Video, len(videos))
	for _, item := range videos {
		videoByID[item.ID] = item
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for videoID, scorerID := range assignments {
			videoItem, ok := videoByID[videoID]
			if !ok {
				return ErrAssignmentVideoNotFound
			}

			overallStatus := evaluation.ResolveOverallStatus(evaluation.ManualStatusPending, videoItem.AIStatus)
			updates := map[string]any{
				"scorer_id":         scorerID,
				"assigned_at":       assignedAt,
				"evaluation_status": overallStatus,
				"manual_status":     evaluation.ManualStatusPending,
				"completed_at":      nil,
				"manual_score":      nil,
				"updated_at":        assignedAt,
			}
			if err := tx.Model(&model.Video{}).Where("id = ?", videoID).Updates(updates).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) FindVideoByID(ctx context.Context, id uuid.UUID) (*model.Video, error) {
	var item model.Video
	if err := r.db.WithContext(ctx).
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Rubric").
		Preload("Scorer").
		Where("id = ?", id).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindAssignedVideoDetail(ctx context.Context, videoID, scorerID uuid.UUID) (*model.Video, error) {
	var item model.Video
	err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Select(videoSelectableColumns).
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Project.School").
		Preload("Task.Rubric").
		Preload("Scorer").
		Joins("JOIN task_scorers ON task_scorers.task_id = videos.task_id AND task_scorers.scorer_id = videos.scorer_id AND task_scorers.status = ?", "accepted").
		Where("videos.id = ?", videoID).
		Where("videos.scorer_id = ?", scorerID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *Repository) FindLatestManualEvaluation(ctx context.Context, videoID, scorerID uuid.UUID) (*model.ManualEvaluation, error) {
	var item model.ManualEvaluation
	err := r.db.WithContext(ctx).
		Model(&model.ManualEvaluation{}).
		Where("video_id = ?", videoID).
		Where("scorer_id = ?", scorerID).
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

func (r *Repository) SubmitManualEvaluation(ctx context.Context, video *model.Video, input SubmitTaskInput, submittedAt time.Time) (*model.ManualEvaluation, error) {
	var manualEval model.ManualEvaluation

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Model(&model.ManualEvaluation{}).
			Where("video_id = ?", video.ID).
			Where("scorer_id = ?", *video.ScorerID).
			Order("submitted_at DESC NULLS LAST, updated_at DESC").
			First(&manualEval).Error

		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			manualEval = model.ManualEvaluation{
				TaskID:       *video.TaskID,
				VideoID:      video.ID,
				ScorerID:     *video.ScorerID,
				RubricID:     rubricIDFromVideo(video),
				TotalScore:   &input.TotalScore,
				ScoreDetails: input.ScoreDetails,
				Comments:     input.Comments,
				Status:       ManualEvaluationStatusSubmitted,
				StartedAt:    startedAtFromVideo(video, submittedAt),
				SubmittedAt:  &submittedAt,
			}
			if err := tx.Create(&manualEval).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			startedAt := manualEval.StartedAt
			if startedAt == nil {
				startedAt = startedAtFromVideo(video, submittedAt)
			}
			updates := map[string]any{
				"rubric_id":     rubricIDFromVideo(video),
				"total_score":   input.TotalScore,
				"score_details": input.ScoreDetails,
				"comments":      input.Comments,
				"status":        ManualEvaluationStatusSubmitted,
				"started_at":    startedAt,
				"submitted_at":  submittedAt,
				"updated_at":    submittedAt,
			}
			if err := tx.Model(&model.ManualEvaluation{}).Where("id = ?", manualEval.ID).Updates(updates).Error; err != nil {
				return err
			}
			manualEval.TotalScore = &input.TotalScore
			manualEval.ScoreDetails = input.ScoreDetails
			manualEval.Comments = input.Comments
			manualEval.Status = ManualEvaluationStatusSubmitted
			manualEval.StartedAt = startedAt
			manualEval.SubmittedAt = &submittedAt
			manualEval.UpdatedAt = submittedAt
		}

		overallStatus := evaluation.ResolveOverallStatus(evaluation.ManualStatusSubmitted, video.AIStatus)
		videoUpdates := map[string]any{
			"manual_score":      input.TotalScore,
			"manual_status":     evaluation.ManualStatusSubmitted,
			"evaluation_status": overallStatus,
			"completed_at":      evaluation.ResolveCompletedAt(overallStatus, submittedAt),
			"updated_at":        submittedAt,
		}
		if err := tx.Model(&model.Video{}).Where("id = ?", video.ID).Updates(videoUpdates).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &manualEval, nil
}

func rubricIDFromVideo(video *model.Video) *uuid.UUID {
	if video.Task != nil {
		return &video.Task.RubricID
	}
	return nil
}

func startedAtFromVideo(video *model.Video, fallback time.Time) *time.Time {
	if video.AssignedAt != nil {
		value := *video.AssignedAt
		return &value
	}
	value := fallback
	return &value
}

func (r *Repository) ListAssignedVideos(ctx context.Context, scorerID uuid.UUID, params MyTasksListParams) ([]model.Video, int64, error) {
	baseQuery := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Project.School").
		Preload("Task.Rubric").
		Preload("Scorer").
		Joins("JOIN task_scorers ON task_scorers.task_id = videos.task_id AND task_scorers.scorer_id = videos.scorer_id AND task_scorers.status = ?", "accepted").
		Where("videos.scorer_id = ?", scorerID)

	if params.ProjectID != nil && *params.ProjectID != uuid.Nil {
		baseQuery = baseQuery.Joins("JOIN tasks ON tasks.id = videos.task_id").
			Where("tasks.project_id = ?", *params.ProjectID)
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		switch status {
		case evaluation.OverallStatusPending:
			baseQuery = baseQuery.Where("videos.manual_status = ?", evaluation.ManualStatusPending)
		case evaluation.OverallStatusInProgress:
			baseQuery = baseQuery.Where("videos.manual_status = ?", evaluation.ManualStatusInProgress)
		case evaluation.OverallStatusCompleted:
			baseQuery = baseQuery.Where("videos.manual_status = ?", evaluation.ManualStatusSubmitted)
		case "skipped":
			baseQuery = baseQuery.Where("1 = 0")
		}
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var items []model.Video
	if err := baseQuery.
		Select(videoSelectableColumns).
		Order("videos.assigned_at DESC NULLS LAST, videos.created_at DESC").
		Offset(offset).
		Limit(params.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
