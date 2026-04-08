package scoring

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/ai"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

type Service struct {
	repo        *Repository
	aiService   *ai.Service
	taskService *task.Service
	storage     storage.Provider
}

func NewService(repo *Repository, aiService *ai.Service, taskService *task.Service, storageProvider storage.Provider) *Service {
	return &Service{repo: repo, aiService: aiService, taskService: taskService, storage: storageProvider}
}

func (s *Service) ListMyTasks(ctx context.Context, actor user.UserContext, params MyTasksListParams) (*MyTasksResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if !isValidMyTaskStatus(params.Status) {
		return nil, ErrMyTasksStatusInvalid
	}

	items, total, err := s.repo.ListAssignedVideos(ctx, actor.UserID, params)
	if err != nil {
		return nil, err
	}

	// Phase 1 does not expose a standalone scoring task table. The scorer's
	// "task list" is a projection of videos already assigned to that scorer.
	result := make([]MyTaskListItemDTO, 0, len(items))
	for i := range items {
		result = append(result, toMyTaskListItemDTO(&items[i], nil))
	}

	return &MyTasksResult{
		Items: result,
		Pagination: MyTasksPagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) ListAssignableScorers(ctx context.Context, actor user.UserContext) (*AssignableScorersResult, error) {
	if !canAssignScorers(actor.Role) {
		return nil, ErrAssignmentRoleNotAllowed
	}

	scorers, err := s.repo.ListAssignableScorers(ctx, actor.SchoolID)
	if err != nil {
		return nil, err
	}

	items := make([]AssignableScorerDTO, 0, len(scorers))
	for _, scorer := range scorers {
		items = append(items, AssignableScorerDTO{
			ID:       scorer.ID,
			Username: scorer.Username,
			RealName: scorer.RealName,
		})
	}

	return &AssignableScorersResult{Items: items}, nil
}

func (s *Service) GetTaskDetail(ctx context.Context, actor user.UserContext, id uuid.UUID) (*ScoringTaskDetailDTO, error) {
	// The API path keeps task semantics from api.md, but the concrete identifier
	// here is the assigned video ID that the scorer is expected to review.
	item, err := s.repo.FindAssignedVideoDetail(ctx, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrScoringTaskNotFound
	}

	manualEvaluation, err := s.repo.FindLatestManualEvaluation(ctx, item.ID, actor.UserID)
	if err != nil {
		return nil, err
	}
	aiEvaluation, err := s.latestAIEvaluation(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	var playURL *string
	if s.storage != nil && item.StoragePath != nil && *item.StoragePath != "" {
		generated, err := s.storage.GeneratePlayURL(ctx, *item.StoragePath, time.Hour)
		if err != nil {
			return nil, err
		}
		playURL = &generated
	}

	return toScoringTaskDetailDTO(item, manualEvaluation, aiEvaluation, playURL), nil
}

func (s *Service) SubmitTask(ctx context.Context, actor user.UserContext, id uuid.UUID, input SubmitTaskInput) (*SubmitTaskResult, error) {
	if len(input.ScoreDetails) == 0 {
		return nil, ErrSubmitScoreDetailsRequired
	}
	if input.TotalScore < 0 {
		return nil, ErrSubmitTotalScoreInvalid
	}

	item, err := s.repo.FindAssignedVideoDetail(ctx, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if item == nil || item.ScorerID == nil || item.TaskID == nil {
		return nil, ErrScoringTaskNotFound
	}
	if item.EvaluationStatus == VideoEvaluationStatusCompleted || item.ManualStatus == VideoManualStatusSubmitted {
		return nil, ErrScoringTaskCompleted
	}

	now := time.Now()
	if item.EvaluationStatus == VideoEvaluationStatusPending {
		item.EvaluationStatus = VideoEvaluationStatusInProgress
	}
	if item.ManualStatus == VideoManualStatusPending {
		item.ManualStatus = VideoManualStatusInProgress
	}

	// videos keeps the latest summary state, while manual_evaluations stores the
	// durable scoring detail payload and timestamps.
	manual, err := s.repo.SubmitManualEvaluation(ctx, item, input, now)
	if err != nil {
		return nil, err
	}
	if err := s.taskService.RefreshVideoStats(ctx, *item.TaskID); err != nil {
		return nil, err
	}

	item.ManualScore = &input.TotalScore
	item.ManualStatus = VideoManualStatusSubmitted
	item.EvaluationStatus = VideoEvaluationStatusCompleted
	item.CompletedAt = &now

	return toSubmitTaskResult(item, manual), nil
}

func (s *Service) AssignScorers(ctx context.Context, actor user.UserContext, input AssignScorersInput) (*AssignScorersResult, error) {
	if err := validateAssignScorersInput(input); err != nil {
		return nil, err
	}
	if !canAssignScorers(actor.Role) {
		return nil, ErrAssignmentRoleNotAllowed
	}

	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, input.TaskID)
	if err != nil {
		return nil, err
	}

	videos, err := s.repo.FindVideosByTaskAndIDs(ctx, input.TaskID, input.VideoIDs)
	if err != nil {
		return nil, err
	}
	if len(videos) != len(uniqueUUIDs(input.VideoIDs)) {
		return nil, ErrAssignmentVideoNotFound
	}
	if err := validateAssignableVideos(videos); err != nil {
		return nil, err
	}

	scorers, err := s.repo.FindAssignableScorers(ctx, resolvedTask.SchoolID, input.ScorerIDs)
	if err != nil {
		return nil, err
	}
	if len(scorers) != len(uniqueUUIDs(input.ScorerIDs)) {
		return nil, ErrAssignmentScorerNotFound
	}

	assignments, err := buildAssignments(input, videos, scorers)
	if err != nil {
		return nil, err
	}

	assignedAt := time.Now()
	// Phase 1 assignment is single-review only: one video maps to one scorer at
	// any moment, and status immediately becomes pending manual evaluation.
	if err := s.repo.AssignScorers(ctx, assignments, assignedAt); err != nil {
		return nil, err
	}

	assigned := make([]AssignedTaskDTO, 0, len(assignments))
	for _, videoID := range input.VideoIDs {
		scorerID, ok := assignments[videoID]
		if !ok {
			continue
		}
		videoItem := findVideoByID(videos, videoID)
		scorerItem := findScorerByID(scorers, scorerID)
		if videoItem == nil || scorerItem == nil {
			continue
		}

		videoItem.ScorerID = &scorerID
		videoItem.Scorer = scorerItem
		videoItem.EvaluationStatus = VideoEvaluationStatusPending
		videoItem.ManualStatus = VideoManualStatusPending
		videoItem.AssignedAt = &assignedAt
		assigned = append(assigned, toAssignedTaskDTO(videoItem, scorerItem))
	}

	return &AssignScorersResult{
		Total:   len(input.VideoIDs),
		Created: len(assigned),
		Tasks:   assigned,
	}, nil
}

func (s *Service) latestAIEvaluation(ctx context.Context, videoID uuid.UUID) (*ai.EmbeddedEvaluationDTO, error) {
	if s.aiService == nil {
		return nil, nil
	}
	return s.aiService.GetLatestForVideo(ctx, videoID)
}

func validateAssignScorersInput(input AssignScorersInput) error {
	if input.TaskID == uuid.Nil {
		return task.ErrTaskNotFound
	}
	if len(input.VideoIDs) == 0 {
		return ErrAssignmentVideoIDsRequired
	}
	if len(input.ScorerIDs) == 0 {
		return ErrAssignmentScorerIDsRequired
	}

	switch input.AssignmentStrategy {
	case AssignmentStrategyAverage:
		return nil
	case AssignmentStrategySpecific:
		if len(input.SpecificAssignments) == 0 {
			return ErrAssignmentSpecificRequired
		}
		return nil
	default:
		if strings.TrimSpace(string(input.AssignmentStrategy)) == "" {
			return ErrAssignmentStrategyRequired
		}
		return ErrAssignmentStrategyInvalid
	}
}

func canAssignScorers(role string) bool {
	switch role {
	case "admin", "school_admin", "school_leader", "teacher":
		return true
	default:
		return false
	}
}

func isValidMyTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "", VideoEvaluationStatusPending, VideoEvaluationStatusInProgress, VideoEvaluationStatusCompleted, "skipped":
		return true
	default:
		return false
	}
}

func validateAssignableVideos(videos []model.Video) error {
	for _, item := range videos {
		if item.Status != "ready" {
			return ErrAssignmentVideoNotReady
		}
		if item.EvaluationStatus == VideoEvaluationStatusCompleted || item.ManualStatus == VideoManualStatusSubmitted {
			return ErrAssignmentVideoCompleted
		}
	}

	return nil
}

func buildAssignments(input AssignScorersInput, videos []model.Video, scorers []model.User) (map[uuid.UUID]uuid.UUID, error) {
	videoByID := make(map[uuid.UUID]model.Video, len(videos))
	for _, item := range videos {
		videoByID[item.ID] = item
	}

	scorerIDs := make([]uuid.UUID, 0, len(scorers))
	for _, scorer := range scorers {
		scorerIDs = append(scorerIDs, scorer.ID)
	}

	assignments := make(map[uuid.UUID]uuid.UUID, len(input.VideoIDs))

	switch input.AssignmentStrategy {
	case AssignmentStrategyAverage:
		for idx, videoID := range input.VideoIDs {
			if _, ok := videoByID[videoID]; !ok {
				return nil, ErrAssignmentVideoNotFound
			}
			assignments[videoID] = scorerIDs[idx%len(scorerIDs)]
		}
	case AssignmentStrategySpecific:
		for _, item := range input.SpecificAssignments {
			if _, ok := videoByID[item.VideoID]; !ok {
				return nil, ErrAssignmentSpecificInvalid
			}
			if !slices.Contains(scorerIDs, item.ScorerID) {
				return nil, ErrAssignmentSpecificInvalid
			}
			assignments[item.VideoID] = item.ScorerID
		}
		for _, videoID := range input.VideoIDs {
			if _, ok := assignments[videoID]; !ok {
				return nil, ErrAssignmentSpecificInvalid
			}
		}
	}

	return assignments, nil
}

func uniqueUUIDs(items []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(items))
	result := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item == uuid.Nil {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func findVideoByID(items []model.Video, id uuid.UUID) *model.Video {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func findScorerByID(items []model.User, id uuid.UUID) *model.User {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
