package scoring

import (
	"context"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"time"

	"skilljudge/backend/internal/domain/evaluation"
	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/ai"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

type taskScorerCoordinator interface {
	EnsureNotificationIfNeeded(ctx context.Context, actor user.UserContext, taskID, scorerID uuid.UUID) (bool, error)
	ExpirePendingRelation(ctx context.Context, actor user.UserContext, taskID, scorerID uuid.UUID) error
}

type Service struct {
	repo        *Repository
	aiService   *ai.Service
	taskService *task.Service
	storage     storage.Provider
	taskScorers taskScorerCoordinator
}

func NewService(repo *Repository, aiService *ai.Service, taskService *task.Service, storageProvider storage.Provider, taskScorers taskScorerCoordinator) *Service {
	return &Service{
		repo:        repo,
		aiService:   aiService,
		taskService: taskService,
		storage:     storageProvider,
		taskScorers: taskScorers,
	}
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

func (s *Service) SaveTaskDraft(ctx context.Context, actor user.UserContext, id uuid.UUID, input SubmitTaskInput) (*SubmitTaskResult, error) {
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
	manual, err := s.repo.SaveManualEvaluationDraft(ctx, item, input, now)
	if err != nil {
		return nil, err
	}

	item.ManualScore = &input.TotalScore
	item.ManualStatus = evaluation.ManualStatusInProgress
	item.EvaluationStatus = evaluation.ResolveOverallStatus(item.ManualStatus, item.AIStatus)
	item.CompletedAt = nil

	return toSubmitTaskResult(item, manual), nil
}

func (s *Service) SubmitSavedTask(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*SubmitSavedTaskResult, error) {
	drafts, err := s.repo.ListSavedDraftVideosByTask(ctx, taskID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if len(drafts) == 0 {
		return nil, ErrScoringTaskNoSavedDrafts
	}

	now := time.Now()
	submittedCount, err := s.repo.SubmitSavedDrafts(ctx, drafts, now)
	if err != nil {
		return nil, err
	}
	if submittedCount == 0 {
		return nil, ErrScoringTaskNoSavedDrafts
	}

	if err := s.taskService.RefreshVideoStats(ctx, taskID); err != nil {
		return nil, err
	}

	return &SubmitSavedTaskResult{
		TaskID:    taskID,
		Submitted: submittedCount,
	}, nil
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

	now := time.Now()
	if item.ManualStatus == VideoManualStatusPending {
		item.ManualStatus = evaluation.ManualStatusInProgress
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
	item.ManualStatus = evaluation.ManualStatusSubmitted
	item.EvaluationStatus = evaluation.ResolveOverallStatus(item.ManualStatus, item.AIStatus)
	item.CompletedAt = evaluation.ResolveCompletedAt(item.EvaluationStatus, now)

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
	if err := s.repo.AssignScorers(ctx, videos, assignments, assignedAt); err != nil {
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
		videoItem.ManualStatus = evaluation.ManualStatusPending
		videoItem.EvaluationStatus = evaluation.ResolveOverallStatus(videoItem.ManualStatus, videoItem.AIStatus)
		videoItem.AssignedAt = &assignedAt
		assigned = append(assigned, toAssignedTaskDTO(videoItem, scorerItem))
	}

	return &AssignScorersResult{
		Total:   len(input.VideoIDs),
		Created: len(assigned),
		Tasks:   assigned,
	}, nil
}

func (s *Service) ListPendingAssignments(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*PendingAssignmentsResult, error) {
	if !canAssignScorers(actor.Role) {
		return nil, ErrAssignmentRoleNotAllowed
	}
	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, taskID)
	if err != nil {
		return nil, err
	}

	videos, err := s.repo.ListPendingVideosByTask(ctx, resolvedTask.TaskID)
	if err != nil {
		return nil, err
	}

	taskScorerStatuses, err := s.repo.ListTaskScorerStatuses(ctx, resolvedTask.TaskID)
	if err != nil {
		return nil, err
	}

	items := make([]PendingAssignmentVideoDTO, 0, len(videos))
	for i := range videos {
		videoItem := videos[i]
		scorerName := "未分配"
		scorerStatus := "unassigned"
		currentTaskState := "待分配"
		if videoItem.Scorer != nil {
			scorerName = strings.TrimSpace(displayNameForScorer(videoItem.Scorer))
		}
		if videoItem.ScorerID != nil {
			if status, ok := taskScorerStatuses[*videoItem.ScorerID]; ok {
				scorerStatus = status
				if status == "accepted" {
					currentTaskState = "已确认分配"
				} else if status == "pending" {
					currentTaskState = "待确认预分配"
				}
			} else {
				scorerStatus = "unknown"
			}
		}

		items = append(items, PendingAssignmentVideoDTO{
			ID:               videoItem.ID,
			StudentName:      videoItem.StudentName,
			StudentNumber:    videoItem.StudentNumber,
			Filename:         videoItem.Filename,
			ScorerID:         videoItem.ScorerID,
			ScorerName:       scorerName,
			ScorerStatus:     scorerStatus,
			ManualStatus:     videoItem.ManualStatus,
			AssignedAt:       videoItem.AssignedAt,
			IsReassignable:   videoItem.ManualStatus == evaluation.ManualStatusPending,
			CurrentTaskState: currentTaskState,
		})
	}

	return &PendingAssignmentsResult{
		Items: items,
		Total: len(items),
	}, nil
}

func (s *Service) ReassignPendingVideos(ctx context.Context, actor user.UserContext, input ReassignPendingVideosInput) (*ReassignPendingVideosResult, error) {
	if err := validateReassignPendingVideosInput(input); err != nil {
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
	if err := validateReassignablePendingVideos(videos); err != nil {
		return nil, err
	}

	scorers, err := s.repo.FindAssignableScorers(ctx, resolvedTask.SchoolID, input.ScorerIDs)
	if err != nil {
		return nil, err
	}
	if len(scorers) != len(uniqueUUIDs(input.ScorerIDs)) {
		return nil, ErrAssignmentScorerNotFound
	}

	assignments, err := buildReassignments(input, videos, scorers)
	if err != nil {
		return nil, err
	}

	sourceScorers := collectSourceScorerIDs(videos)
	assignedAt := time.Now()
	if err := s.repo.AssignScorers(ctx, videos, assignments, assignedAt); err != nil {
		return nil, err
	}

	notifiedScorerIDs := make([]uuid.UUID, 0, len(input.ScorerIDs))
	if s.taskScorers != nil {
		for _, scorerID := range uniqueUUIDs(input.ScorerIDs) {
			notified, notifyErr := s.taskScorers.EnsureNotificationIfNeeded(ctx, actor, resolvedTask.TaskID, scorerID)
			if notifyErr != nil {
				return nil, notifyErr
			}
			if notified {
				notifiedScorerIDs = append(notifiedScorerIDs, scorerID)
			}
		}
	}

	expiredSourceScorerIDs := make([]uuid.UUID, 0, len(sourceScorers))
	for _, scorerID := range sourceScorers {
		if scorerID == uuid.Nil || slices.Contains(input.ScorerIDs, scorerID) {
			continue
		}
		remaining, countErr := s.repo.CountVideosByTaskAndScorer(ctx, resolvedTask.TaskID, scorerID)
		if countErr != nil {
			return nil, countErr
		}
		if remaining > 0 || s.taskScorers == nil {
			continue
		}
		if expireErr := s.taskScorers.ExpirePendingRelation(ctx, actor, resolvedTask.TaskID, scorerID); expireErr != nil {
			return nil, expireErr
		}
		expiredSourceScorerIDs = append(expiredSourceScorerIDs, scorerID)
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
		videoItem.ManualStatus = evaluation.ManualStatusPending
		videoItem.EvaluationStatus = evaluation.ResolveOverallStatus(videoItem.ManualStatus, videoItem.AIStatus)
		videoItem.AssignedAt = &assignedAt
		assigned = append(assigned, toAssignedTaskDTO(videoItem, scorerItem))
	}

	sort.Slice(notifiedScorerIDs, func(i, j int) bool {
		return notifiedScorerIDs[i].String() < notifiedScorerIDs[j].String()
	})
	sort.Slice(expiredSourceScorerIDs, func(i, j int) bool {
		return expiredSourceScorerIDs[i].String() < expiredSourceScorerIDs[j].String()
	})

	return &ReassignPendingVideosResult{
		Total:                  len(input.VideoIDs),
		Created:                len(assigned),
		Tasks:                  assigned,
		NotifiedScorerIDs:      notifiedScorerIDs,
		ExpiredSourceScorerIDs: expiredSourceScorerIDs,
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

func validateReassignablePendingVideos(videos []model.Video) error {
	if err := validateAssignableVideos(videos); err != nil {
		return err
	}
	for _, item := range videos {
		if item.ManualStatus != evaluation.ManualStatusPending {
			return ErrAssignmentVideoCompleted
		}
	}
	return nil
}

func validateReassignPendingVideosInput(input ReassignPendingVideosInput) error {
	if input.TaskID == uuid.Nil {
		return task.ErrTaskNotFound
	}
	if len(input.VideoIDs) == 0 {
		return ErrAssignmentVideoIDsRequired
	}
	if len(input.ScorerIDs) == 0 {
		return ErrAssignmentScorerIDsRequired
	}
	switch input.ReassignmentMode {
	case ReassignmentModeAverage:
		return nil
	case ReassignmentModeQuantity:
		if len(input.QuantityAssignments) == 0 {
			return ErrReassignmentCountRequired
		}
		total := 0
		seen := make(map[uuid.UUID]struct{}, len(input.QuantityAssignments))
		for _, item := range input.QuantityAssignments {
			if item.ScorerID == uuid.Nil || item.Count <= 0 {
				return ErrReassignmentCountInvalid
			}
			if _, ok := seen[item.ScorerID]; ok {
				return ErrReassignmentCountInvalid
			}
			seen[item.ScorerID] = struct{}{}
			total += item.Count
		}
		if total != len(uniqueUUIDs(input.VideoIDs)) {
			return ErrReassignmentCountMismatch
		}
		return nil
	default:
		if strings.TrimSpace(string(input.ReassignmentMode)) == "" {
			return ErrReassignmentModeRequired
		}
		return ErrReassignmentModeInvalid
	}
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

func buildReassignments(input ReassignPendingVideosInput, videos []model.Video, scorers []model.User) (map[uuid.UUID]uuid.UUID, error) {
	videoByID := make(map[uuid.UUID]model.Video, len(videos))
	videoIDs := make([]uuid.UUID, 0, len(videos))
	for _, item := range videos {
		videoByID[item.ID] = item
		videoIDs = append(videoIDs, item.ID)
	}
	scorerIDs := make([]uuid.UUID, 0, len(scorers))
	for _, scorer := range scorers {
		scorerIDs = append(scorerIDs, scorer.ID)
	}

	shuffleUUIDs(videoIDs)
	assignments := make(map[uuid.UUID]uuid.UUID, len(videoIDs))

	switch input.ReassignmentMode {
	case ReassignmentModeAverage:
		for idx, videoID := range videoIDs {
			assignments[videoID] = scorerIDs[idx%len(scorerIDs)]
		}
	case ReassignmentModeQuantity:
		position := 0
		for _, item := range input.QuantityAssignments {
			if !slices.Contains(scorerIDs, item.ScorerID) {
				return nil, ErrReassignmentCountInvalid
			}
			for i := 0; i < item.Count; i++ {
				if position >= len(videoIDs) {
					return nil, ErrReassignmentCountMismatch
				}
				assignments[videoIDs[position]] = item.ScorerID
				position++
			}
		}
		if position != len(videoIDs) {
			return nil, ErrReassignmentCountMismatch
		}
	}

	for _, videoID := range input.VideoIDs {
		if _, ok := videoByID[videoID]; !ok {
			return nil, ErrAssignmentVideoNotFound
		}
		if _, ok := assignments[videoID]; !ok {
			return nil, ErrReassignmentCountMismatch
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

func collectSourceScorerIDs(videos []model.Video) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(videos))
	result := make([]uuid.UUID, 0, len(videos))
	for _, item := range videos {
		if item.ScorerID == nil || *item.ScorerID == uuid.Nil {
			continue
		}
		if _, ok := seen[*item.ScorerID]; ok {
			continue
		}
		seen[*item.ScorerID] = struct{}{}
		result = append(result, *item.ScorerID)
	}
	return result
}

func shuffleUUIDs(items []uuid.UUID) {
	if len(items) <= 1 {
		return
	}
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	rnd.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
}

func displayNameForScorer(scorer *model.User) string {
	if scorer == nil {
		return "评分员"
	}
	if scorer.RealName != nil && strings.TrimSpace(*scorer.RealName) != "" {
		return strings.TrimSpace(*scorer.RealName)
	}
	if strings.TrimSpace(scorer.Username) != "" {
		return strings.TrimSpace(scorer.Username)
	}
	if scorer.Email != nil && strings.TrimSpace(*scorer.Email) != "" {
		return strings.TrimSpace(*scorer.Email)
	}
	return "评分员"
}
