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
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

const videoSelectableColumns = "videos.*"

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func reviewAssignmentManualStatus(status string) string {
	switch strings.TrimSpace(status) {
	case ReviewAssignmentStatusSubmitted:
		return evaluation.ManualStatusSubmitted
	case ReviewAssignmentStatusInProgress:
		return evaluation.ManualStatusInProgress
	default:
		return evaluation.ManualStatusPending
	}
}

func applyReviewAssignmentToVideo(video *model.Video, assignment *model.VideoReviewAssignment) {
	if video == nil || assignment == nil {
		return
	}
	scorerID := assignment.ScorerID
	video.ScorerID = &scorerID
	video.Scorer = assignment.Scorer
	video.AssignedAt = assignment.AssignedAt
	video.ManualStatus = reviewAssignmentManualStatus(assignment.Status)
	video.EvaluationStatus = evaluation.ResolveOverallStatus(video.ManualStatus, video.AIStatus)
}

func videosFromReviewAssignments(assignments []model.VideoReviewAssignment) []model.Video {
	items := make([]model.Video, 0, len(assignments))
	for i := range assignments {
		if assignments[i].Video == nil {
			continue
		}
		video := *assignments[i].Video
		applyReviewAssignmentToVideo(&video, &assignments[i])
		items = append(items, video)
	}
	return items
}

func (r *Repository) attachSingleReviewAssignments(ctx context.Context, videos []model.Video) error {
	videoIDs := make([]uuid.UUID, 0, len(videos))
	for i := range videos {
		videoIDs = append(videoIDs, videos[i].ID)
	}
	if len(videoIDs) == 0 {
		return nil
	}

	var assignments []model.VideoReviewAssignment
	if err := r.db.WithContext(ctx).
		Preload("Scorer").
		Where("video_id IN ?", videoIDs).
		Where("review_no = ?", SingleReviewNo).
		Where("review_type = ?", ReviewAssignmentTypeNormal).
		Where("status <> ?", ReviewAssignmentStatusCancelled).
		Find(&assignments).Error; err != nil {
		return err
	}

	assignmentByVideoID := make(map[uuid.UUID]*model.VideoReviewAssignment, len(assignments))
	for i := range assignments {
		assignmentByVideoID[assignments[i].VideoID] = &assignments[i]
	}
	for i := range videos {
		applyReviewAssignmentToVideo(&videos[i], assignmentByVideoID[videos[i].ID])
	}
	return nil
}

func findSingleReviewAssignmentTx(tx *gorm.DB, videoID, scorerID uuid.UUID) (*model.VideoReviewAssignment, error) {
	var assignment model.VideoReviewAssignment
	err := tx.
		Where("video_id = ?", videoID).
		Where("scorer_id = ?", scorerID).
		Where("review_no = ?", SingleReviewNo).
		Where("review_type = ?", ReviewAssignmentTypeNormal).
		Where("status <> ?", ReviewAssignmentStatusCancelled).
		First(&assignment).Error
	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) FindVideosByTaskAndIDs(ctx context.Context, taskID uuid.UUID, videoIDs []uuid.UUID) ([]model.Video, error) {
	var items []model.Video
	if err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Select(videoSelectableColumns).
		Where("task_id = ?", taskID).
		Where("id IN ?", videoIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	if err := r.attachSingleReviewAssignments(ctx, items); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) ListPendingVideosByTask(ctx context.Context, taskID uuid.UUID) ([]model.Video, error) {
	var items []model.Video
	err := r.db.WithContext(ctx).
		Model(&model.Video{}).
		Select(videoSelectableColumns).
		Where("task_id = ?", taskID).
		Where("manual_status = ?", evaluation.ManualStatusPending).
		Order("assigned_at DESC NULLS LAST, created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	if err := r.attachSingleReviewAssignments(ctx, items); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) ListTaskScorerStatuses(ctx context.Context, taskID uuid.UUID) (map[uuid.UUID]string, error) {
	type row struct {
		ScorerID uuid.UUID
		Status   string
	}

	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&model.TaskScorer{}).
		Select("scorer_id, status").
		Where("task_id = ?", taskID).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]string, len(rows))
	for _, item := range rows {
		result[item.ScorerID] = item.Status
	}
	return result, nil
}

func (r *Repository) CountVideosByTaskAndScorer(ctx context.Context, taskID, scorerID uuid.UUID) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&model.VideoReviewAssignment{}).
		Where("task_id = ?", taskID).
		Where("scorer_id = ?", scorerID).
		Where("review_no = ?", SingleReviewNo).
		Where("review_type = ?", ReviewAssignmentTypeNormal).
		Where("status <> ?", ReviewAssignmentStatusCancelled).
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *Repository) FindAssignableScorers(ctx context.Context, schoolID *uuid.UUID, scorerIDs []uuid.UUID) ([]model.User, error) {
	query := r.db.WithContext(ctx).
		Model(&model.User{}).
		Select("DISTINCT users.*").
		Joins("join user_roles on user_roles.user_id = users.id and user_roles.status = ?", "active").
		Joins("join roles on roles.id = user_roles.role_id and roles.status = ?", "active").
		Where("users.id IN ?", scorerIDs).
		Where("users.status = ?", "active").
		Where("roles.code = ?", "scorer").
		Where("NOT (COALESCE(users.metadata ->> 'provisionedBy', '') = ? AND COALESCE(users.metadata ->> 'credentialReady', 'true') = ?)", "taskscorer", "false")

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
		Where("roles.code = ?", "scorer").
		Where("NOT (COALESCE(users.metadata ->> 'provisionedBy', '') = ? AND COALESCE(users.metadata ->> 'credentialReady', 'true') = ?)", "taskscorer", "false")

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
			if videoItem.TaskID == nil {
				return ErrAssignmentVideoNotFound
			}

			overallStatus := evaluation.ResolveOverallStatus(evaluation.ManualStatusPending, videoItem.AIStatus)
			assignment := model.VideoReviewAssignment{
				TaskID:     *videoItem.TaskID,
				VideoID:    videoID,
				ScorerID:   scorerID,
				ReviewNo:   SingleReviewNo,
				ReviewType: ReviewAssignmentTypeNormal,
				Status:     ReviewAssignmentStatusPending,
				AssignedAt: &assignedAt,
				Metadata:   map[string]any{},
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "video_id"}, {Name: "review_no"}},
				DoUpdates: clause.Assignments(map[string]any{
					"task_id":      assignment.TaskID,
					"scorer_id":    assignment.ScorerID,
					"review_type":  assignment.ReviewType,
					"status":       assignment.Status,
					"assigned_at":  assignedAt,
					"started_at":   nil,
					"submitted_at": nil,
					"metadata":     assignment.Metadata,
					"updated_at":   assignedAt,
				}),
			}).Create(&assignment).Error; err != nil {
				return err
			}

			updates := map[string]any{
				"assigned_at":            assignedAt,
				"evaluation_status":      overallStatus,
				"manual_status":          evaluation.ManualStatusPending,
				"completed_at":           nil,
				"manual_score":           nil,
				"required_review_count":  SingleReviewNo,
				"submitted_review_count": 0,
				"score_decision_type":    ScoreDecisionTypeSingle,
				"updated_at":             assignedAt,
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
	var assignment model.VideoReviewAssignment
	err := r.db.WithContext(ctx).
		Model(&model.VideoReviewAssignment{}).
		Preload("Video.Task").
		Preload("Video.Task.Project").
		Preload("Video.Task.Project.School").
		Preload("Video.Task.Rubric").
		Preload("Scorer").
		Joins("JOIN task_scorers ON task_scorers.task_id = video_review_assignments.task_id AND task_scorers.scorer_id = video_review_assignments.scorer_id AND task_scorers.status = ?", "accepted").
		Where("video_review_assignments.video_id = ?", videoID).
		Where("video_review_assignments.scorer_id = ?", scorerID).
		Where("video_review_assignments.review_no = ?", SingleReviewNo).
		Where("video_review_assignments.review_type = ?", ReviewAssignmentTypeNormal).
		Where("video_review_assignments.status <> ?", ReviewAssignmentStatusCancelled).
		First(&assignment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if assignment.Video == nil {
		return nil, nil
	}

	item := *assignment.Video
	applyReviewAssignmentToVideo(&item, &assignment)
	return &item, nil
}

func (r *Repository) ListSavedDraftVideosByTask(ctx context.Context, taskID, scorerID uuid.UUID) ([]model.Video, error) {
	var assignments []model.VideoReviewAssignment
	err := r.db.WithContext(ctx).
		Model(&model.VideoReviewAssignment{}).
		Preload("Video.Task").
		Preload("Video.Task.Project").
		Preload("Video.Task.Project.School").
		Preload("Video.Task.Rubric").
		Preload("Scorer").
		Joins("JOIN task_scorers ON task_scorers.task_id = video_review_assignments.task_id AND task_scorers.scorer_id = video_review_assignments.scorer_id AND task_scorers.status = ?", "accepted").
		Joins("JOIN manual_evaluations ON manual_evaluations.assignment_id = video_review_assignments.id AND manual_evaluations.status = ?", ManualEvaluationStatusInProgress).
		Where("video_review_assignments.task_id = ?", taskID).
		Where("video_review_assignments.scorer_id = ?", scorerID).
		Where("video_review_assignments.review_no = ?", SingleReviewNo).
		Where("video_review_assignments.review_type = ?", ReviewAssignmentTypeNormal).
		Where("video_review_assignments.status = ?", ReviewAssignmentStatusInProgress).
		Order("video_review_assignments.assigned_at DESC NULLS LAST, video_review_assignments.created_at DESC").
		Find(&assignments).Error
	if err != nil {
		return nil, err
	}

	return videosFromReviewAssignments(assignments), nil
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

func (r *Repository) SaveManualEvaluationDraft(ctx context.Context, video *model.Video, input SubmitTaskInput, savedAt time.Time) (*model.ManualEvaluation, error) {
	var manualEval model.ManualEvaluation

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if video.TaskID == nil || video.ScorerID == nil {
			return ErrScoringTaskNotFound
		}
		assignment, err := findSingleReviewAssignmentTx(tx, video.ID, *video.ScorerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrScoringTaskNotFound
			}
			return err
		}

		err = tx.
			Model(&model.ManualEvaluation{}).
			Where("assignment_id = ?", assignment.ID).
			Order("submitted_at DESC NULLS LAST, updated_at DESC").
			First(&manualEval).Error

		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			startedAt := startedAtFromAssignment(assignment, video, savedAt)
			manualEval = model.ManualEvaluation{
				TaskID:       *video.TaskID,
				VideoID:      video.ID,
				ScorerID:     *video.ScorerID,
				RubricID:     rubricIDFromVideo(video),
				AssignmentID: &assignment.ID,
				TotalScore:   &input.TotalScore,
				ScoreDetails: input.ScoreDetails,
				Comments:     input.Comments,
				Status:       ManualEvaluationStatusInProgress,
				StartedAt:    startedAt,
			}
			if err := tx.Create(&manualEval).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			startedAt := manualEval.StartedAt
			if startedAt == nil {
				startedAt = startedAtFromAssignment(assignment, video, savedAt)
			}
			updates := map[string]any{
				"rubric_id":     rubricIDFromVideo(video),
				"assignment_id": assignment.ID,
				"total_score":   input.TotalScore,
				"score_details": input.ScoreDetails,
				"comments":      input.Comments,
				"status":        ManualEvaluationStatusInProgress,
				"started_at":    startedAt,
				"submitted_at":  nil,
				"updated_at":    savedAt,
			}
			if err := tx.Model(&model.ManualEvaluation{}).Where("id = ?", manualEval.ID).Updates(updates).Error; err != nil {
				return err
			}
			manualEval.TotalScore = &input.TotalScore
			manualEval.ScoreDetails = input.ScoreDetails
			manualEval.Comments = input.Comments
			manualEval.Status = ManualEvaluationStatusInProgress
			manualEval.StartedAt = startedAt
			manualEval.AssignmentID = &assignment.ID
			manualEval.SubmittedAt = nil
			manualEval.UpdatedAt = savedAt
		}

		overallStatus := evaluation.ResolveOverallStatus(evaluation.ManualStatusInProgress, video.AIStatus)
		startedAt := startedAtFromAssignment(assignment, video, savedAt)
		assignmentUpdates := map[string]any{
			"status":     ReviewAssignmentStatusInProgress,
			"started_at": startedAt,
			"updated_at": savedAt,
		}
		if err := tx.Model(&model.VideoReviewAssignment{}).Where("id = ?", assignment.ID).Updates(assignmentUpdates).Error; err != nil {
			return err
		}
		videoUpdates := map[string]any{
			"manual_score":           input.TotalScore,
			"manual_status":          evaluation.ManualStatusInProgress,
			"evaluation_status":      overallStatus,
			"completed_at":           nil,
			"required_review_count":  SingleReviewNo,
			"submitted_review_count": 0,
			"score_decision_type":    ScoreDecisionTypeSingle,
			"updated_at":             savedAt,
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

func (r *Repository) SubmitManualEvaluation(ctx context.Context, video *model.Video, input SubmitTaskInput, submittedAt time.Time) (*model.ManualEvaluation, error) {
	var manualEval model.ManualEvaluation

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if video.TaskID == nil || video.ScorerID == nil {
			return ErrScoringTaskNotFound
		}
		assignment, err := findSingleReviewAssignmentTx(tx, video.ID, *video.ScorerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrScoringTaskNotFound
			}
			return err
		}

		err = tx.
			Model(&model.ManualEvaluation{}).
			Where("assignment_id = ?", assignment.ID).
			Order("submitted_at DESC NULLS LAST, updated_at DESC").
			First(&manualEval).Error

		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			startedAt := startedAtFromAssignment(assignment, video, submittedAt)
			manualEval = model.ManualEvaluation{
				TaskID:       *video.TaskID,
				VideoID:      video.ID,
				ScorerID:     *video.ScorerID,
				RubricID:     rubricIDFromVideo(video),
				AssignmentID: &assignment.ID,
				TotalScore:   &input.TotalScore,
				ScoreDetails: input.ScoreDetails,
				Comments:     input.Comments,
				Status:       ManualEvaluationStatusSubmitted,
				StartedAt:    startedAt,
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
				startedAt = startedAtFromAssignment(assignment, video, submittedAt)
			}
			updates := map[string]any{
				"rubric_id":     rubricIDFromVideo(video),
				"assignment_id": assignment.ID,
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
			manualEval.AssignmentID = &assignment.ID
			manualEval.SubmittedAt = &submittedAt
			manualEval.UpdatedAt = submittedAt
		}

		overallStatus := evaluation.ResolveOverallStatus(evaluation.ManualStatusSubmitted, video.AIStatus)
		startedAt := startedAtFromAssignment(assignment, video, submittedAt)
		assignmentUpdates := map[string]any{
			"status":       ReviewAssignmentStatusSubmitted,
			"started_at":   startedAt,
			"submitted_at": submittedAt,
			"updated_at":   submittedAt,
		}
		if err := tx.Model(&model.VideoReviewAssignment{}).Where("id = ?", assignment.ID).Updates(assignmentUpdates).Error; err != nil {
			return err
		}
		videoUpdates := map[string]any{
			"manual_score":           input.TotalScore,
			"manual_status":          evaluation.ManualStatusSubmitted,
			"evaluation_status":      overallStatus,
			"completed_at":           evaluation.ResolveCompletedAt(overallStatus, submittedAt),
			"required_review_count":  SingleReviewNo,
			"submitted_review_count": 1,
			"score_decision_type":    ScoreDecisionTypeSingle,
			"updated_at":             submittedAt,
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

func (r *Repository) SubmitSavedDrafts(ctx context.Context, videos []model.Video, submittedAt time.Time) (int, error) {
	if len(videos) == 0 {
		return 0, nil
	}

	submittedCount := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, video := range videos {
			if video.ScorerID == nil {
				continue
			}
			assignment, err := findSingleReviewAssignmentTx(tx, video.ID, *video.ScorerID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			manualUpdates := map[string]any{
				"status":       ManualEvaluationStatusSubmitted,
				"submitted_at": submittedAt,
				"updated_at":   submittedAt,
			}
			result := tx.Model(&model.ManualEvaluation{}).
				Where("assignment_id = ?", assignment.ID).
				Where("status = ?", ManualEvaluationStatusInProgress).
				Updates(manualUpdates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}

			overallStatus := evaluation.ResolveOverallStatus(evaluation.ManualStatusSubmitted, video.AIStatus)
			assignmentUpdates := map[string]any{
				"status":       ReviewAssignmentStatusSubmitted,
				"submitted_at": submittedAt,
				"updated_at":   submittedAt,
			}
			if err := tx.Model(&model.VideoReviewAssignment{}).Where("id = ?", assignment.ID).Updates(assignmentUpdates).Error; err != nil {
				return err
			}
			videoUpdates := map[string]any{
				"manual_status":          evaluation.ManualStatusSubmitted,
				"evaluation_status":      overallStatus,
				"completed_at":           evaluation.ResolveCompletedAt(overallStatus, submittedAt),
				"required_review_count":  SingleReviewNo,
				"submitted_review_count": 1,
				"score_decision_type":    ScoreDecisionTypeSingle,
				"updated_at":             submittedAt,
			}
			if err := tx.Model(&model.Video{}).Where("id = ?", video.ID).Updates(videoUpdates).Error; err != nil {
				return err
			}

			submittedCount += 1
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return submittedCount, nil
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

func startedAtFromAssignment(assignment *model.VideoReviewAssignment, video *model.Video, fallback time.Time) *time.Time {
	if assignment != nil {
		if assignment.StartedAt != nil {
			value := *assignment.StartedAt
			return &value
		}
		if assignment.AssignedAt != nil {
			value := *assignment.AssignedAt
			return &value
		}
	}
	return startedAtFromVideo(video, fallback)
}

func (r *Repository) ListAssignedVideos(ctx context.Context, scorerID uuid.UUID, params MyTasksListParams) ([]model.Video, int64, error) {
	baseQuery := r.db.WithContext(ctx).
		Model(&model.VideoReviewAssignment{}).
		Preload("Video.Task").
		Preload("Video.Task.Project").
		Preload("Video.Task.Project.School").
		Preload("Video.Task.Rubric").
		Preload("Scorer").
		Joins("JOIN videos ON videos.id = video_review_assignments.video_id").
		Joins("JOIN task_scorers ON task_scorers.task_id = video_review_assignments.task_id AND task_scorers.scorer_id = video_review_assignments.scorer_id AND task_scorers.status = ?", "accepted").
		Where("video_review_assignments.scorer_id = ?", scorerID).
		Where("video_review_assignments.review_no = ?", SingleReviewNo).
		Where("video_review_assignments.review_type = ?", ReviewAssignmentTypeNormal).
		Where("video_review_assignments.status <> ?", ReviewAssignmentStatusCancelled)

	if params.ProjectID != nil && *params.ProjectID != uuid.Nil {
		baseQuery = baseQuery.Joins("JOIN tasks ON tasks.id = video_review_assignments.task_id").
			Where("tasks.project_id = ?", *params.ProjectID)
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		switch status {
		case evaluation.OverallStatusPending:
			baseQuery = baseQuery.Where("video_review_assignments.status = ?", ReviewAssignmentStatusPending)
		case evaluation.OverallStatusInProgress:
			baseQuery = baseQuery.Where("video_review_assignments.status = ?", ReviewAssignmentStatusInProgress)
		case evaluation.OverallStatusCompleted:
			baseQuery = baseQuery.Where("video_review_assignments.status = ?", ReviewAssignmentStatusSubmitted)
		case "skipped":
			baseQuery = baseQuery.Where("1 = 0")
		}
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var assignments []model.VideoReviewAssignment
	if err := baseQuery.
		Order("video_review_assignments.assigned_at DESC NULLS LAST, videos.created_at DESC").
		Offset(offset).
		Limit(params.PageSize).
		Find(&assignments).Error; err != nil {
		return nil, 0, err
	}

	return videosFromReviewAssignments(assignments), total, nil
}
