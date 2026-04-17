package ai

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"skilljudge/backend/internal/config"
	"skilljudge/backend/internal/domain/evaluation"
	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

type Service struct {
	repo        *Repository
	client      *Client
	storage     storage.Provider
	taskService *task.Service
	config      config.AIConfig
}

func NewService(repo *Repository, taskService *task.Service, storageProvider storage.Provider, cfg config.AIConfig) *Service {
	return &Service{
		repo:        repo,
		client:      NewClient(cfg),
		storage:     storageProvider,
		taskService: taskService,
		config:      cfg,
	}
}

func (s *Service) Configured() bool {
	return s != nil && s.client != nil && s.client.Configured()
}

func (s *Service) CreateForVideo(ctx context.Context, actor user.UserContext, videoID uuid.UUID, force bool) (*EvaluationDTO, error) {
	item, err := s.repo.FindVideoByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrEvaluationVideoNotFound
	}
	if item.TaskID == nil || *item.TaskID == uuid.Nil {
		return nil, ErrEvaluationTaskRequired
	}
	if _, err := s.taskService.ResolveManageable(ctx, actor, *item.TaskID); err != nil {
		return nil, err
	}

	return s.createForResolvedVideo(ctx, item, force)
}

func (s *Service) BatchCreateForTask(ctx context.Context, actor user.UserContext, taskID uuid.UUID, videoIDs []uuid.UUID, force bool) (*BatchCreateResult, error) {
	uniqueVideoIDs := uniqueUUIDs(videoIDs)
	if len(uniqueVideoIDs) == 0 {
		return nil, ErrBatchVideoIDsRequired
	}
	if _, err := s.taskService.ResolveManageable(ctx, actor, taskID); err != nil {
		return nil, err
	}

	items, err := s.repo.FindVideosByTaskAndIDs(ctx, taskID, uniqueVideoIDs)
	if err != nil {
		return nil, err
	}

	result := &BatchCreateResult{Total: len(uniqueVideoIDs)}
	videoByID := make(map[uuid.UUID]*model.Video, len(items))
	for i := range items {
		item := items[i]
		videoByID[item.ID] = &item
	}

	type batchCreateOutcome struct {
		processing bool
		err        string
	}

	outcomes := make([]batchCreateOutcome, len(uniqueVideoIDs))
	concurrency := s.createConcurrency()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for idx, videoID := range uniqueVideoIDs {
		item, ok := videoByID[videoID]
		if !ok {
			outcomes[idx] = batchCreateOutcome{err: ErrEvaluationVideoNotFound.Error()}
			continue
		}

		wg.Add(1)
		go func(idx int, item *model.Video) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				outcomes[idx] = batchCreateOutcome{err: ctx.Err().Error()}
				return
			}
			defer func() { <-semaphore }()

			if _, err := s.createForResolvedVideo(ctx, item, force); err != nil {
				outcomes[idx] = batchCreateOutcome{err: err.Error()}
				return
			}
			outcomes[idx] = batchCreateOutcome{processing: true}
		}(idx, item)
	}

	wg.Wait()

	for idx, videoID := range uniqueVideoIDs {
		if outcomes[idx].processing {
			result.Processing++
			continue
		}
		if outcomes[idx].err != "" {
			result.Failed++
			result.Errors = append(result.Errors, BatchCreateError{VideoID: videoID, Error: outcomes[idx].err})
		}
	}

	return result, nil
}

func (s *Service) Get(ctx context.Context, actor user.UserContext, evaluationID uuid.UUID) (*EvaluationDTO, error) {
	item, err := s.repo.FindByID(ctx, evaluationID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrEvaluationNotFound
	}
	if _, err := s.taskService.ResolveReadable(ctx, actor, item.TaskID); err != nil {
		return nil, err
	}

	return toEvaluationDTO(item), nil
}

func (s *Service) GetResult(ctx context.Context, actor user.UserContext, evaluationID uuid.UUID) (*EvaluationResultDTO, error) {
	item, err := s.repo.FindByID(ctx, evaluationID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrEvaluationNotFound
	}
	if _, err := s.taskService.ResolveReadable(ctx, actor, item.TaskID); err != nil {
		return nil, err
	}
	if item.Status != EvaluationStatusCompleted || len(item.ResultData) == 0 {
		return nil, ErrEvaluationResultNotReady
	}

	return toEvaluationResultDTO(item)
}

func (s *Service) GetLatestForVideo(ctx context.Context, videoID uuid.UUID) (*EmbeddedEvaluationDTO, error) {
	item, err := s.repo.FindLatestByVideoID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	return toEmbeddedEvaluationDTO(item)
}

func (s *Service) createForResolvedVideo(ctx context.Context, item *model.Video, force bool) (*EvaluationDTO, error) {
	if item.TaskID == nil || *item.TaskID == uuid.Nil {
		return nil, ErrEvaluationTaskRequired
	}
	if item.Status != "ready" {
		return nil, ErrEvaluationVideoNotReady
	}

	latest, err := s.repo.FindLatestByVideoID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		switch latest.Status {
		case EvaluationStatusProcessing:
			return nil, ErrEvaluationAlreadyProcessing
		case EvaluationStatusCompleted, EvaluationStatusFailed:
			if !force {
				return nil, ErrEvaluationForceRequired
			}
		}
	}

	now := time.Now()
	evalRecord := &model.AIEvaluation{
		TaskID:    *item.TaskID,
		VideoID:   item.ID,
		Status:    EvaluationStatusProcessing,
		StartedAt: &now,
	}
	if err := s.repo.CreateAndMarkVideoProcessing(ctx, evalRecord, item.ID, s.buildVideoStatusUpdates(item.ManualStatus, evaluation.AIStatusProcessing, now)); err != nil {
		return nil, err
	}
	if err := s.refreshTaskVideoStats(ctx, evalRecord.TaskID); err != nil {
		return nil, err
	}
	if err := s.dispatchCreateJob(ctx, evalRecord, item); err != nil {
		failedAt := time.Now()
		failureMessage := err.Error()
		overallStatus := evaluation.ResolveOverallStatus(item.ManualStatus, evaluation.AIStatusFailed)
		_ = s.repo.UpdateEvaluationAndVideo(ctx, evalRecord.ID, map[string]any{
			"status":        EvaluationStatusFailed,
			"error_message": failureMessage,
			"completed_at":  failedAt,
			"updated_at":    failedAt,
		}, map[string]any{
			"ai_status":         evaluation.AIStatusFailed,
			"evaluation_status": overallStatus,
			"completed_at":      evaluation.ResolveCompletedAt(overallStatus, failedAt),
			"updated_at":        failedAt,
		})
		_ = s.refreshTaskVideoStats(ctx, evalRecord.TaskID)
		return nil, err
	}

	return toEvaluationDTO(evalRecord), nil
}

func (s *Service) dispatchCreateJob(ctx context.Context, evaluation *model.AIEvaluation, item *model.Video) error {
	if s.client == nil || !s.client.Configured() {
		return ErrProviderNotConfigured
	}
	if item.Task == nil || item.Task.Rubric == nil {
		return ErrEvaluationTaskRequired
	}

	videoURL, err := s.resolveVideoURL(ctx, item)
	if err != nil {
		return err
	}

	result, err := s.client.CreateJob(ctx, CreateJobInput{
		EvaluationID: evaluation.ID.String(),
		VideoURL:     videoURL,
		StoragePath:  item.StoragePath,
		TaskID:       evaluation.TaskID.String(),
		VideoID:      evaluation.VideoID.String(),
		RubricID:     item.Task.Rubric.ID.String(),
		RubricName:   item.Task.Rubric.Name,
		RubricJSON: map[string]any{
			"id":         item.Task.Rubric.ID.String(),
			"name":       item.Task.Rubric.Name,
			"totalScore": item.Task.Rubric.TotalScore,
			"items":      item.Task.Rubric.Items,
		},
	})
	if err != nil {
		return err
	}

	jobID := strings.TrimSpace(result.JobID)
	if jobID == "" {
		return ErrProviderResponseInvalid
	}
	log.Printf(
		"ai dispatch: evaluation=%s video=%s provider_job=%s accepted_status=%s",
		evaluation.ID,
		evaluation.VideoID,
		jobID,
		result.Status,
	)
	evaluation.JobID = &jobID
	return s.repo.SetJobID(ctx, evaluation.ID, jobID)
}

func (s *Service) resolveVideoURL(ctx context.Context, item *model.Video) (string, error) {
	if item.StorageURL != nil && strings.TrimSpace(*item.StorageURL) != "" {
		return *item.StorageURL, nil
	}
	if s.storage != nil && item.StoragePath != nil && strings.TrimSpace(*item.StoragePath) != "" {
		playURL, err := s.storage.GeneratePlayURL(ctx, *item.StoragePath, time.Hour)
		if err != nil {
			return "", err
		}
		return playURL, nil
	}
	return "", ErrEvaluationVideoNotReady
}

func (s *Service) RunPoller(ctx context.Context) {
	if s.client == nil || !s.client.Configured() {
		return
	}

	interval := s.config.PollInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	log.Printf(
		"ai poller configured: base_url=%s analysis_path=%s poll_interval=%s request_timeout=%s job_not_found_grace=%s",
		s.config.BaseURL,
		s.config.AnalysisPath,
		interval,
		s.config.RequestTimeout,
		s.config.JobNotFoundGrace,
	)

	s.pollOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollOnce(ctx)
		}
	}
}

func (s *Service) pollOnce(ctx context.Context) {
	items, err := s.repo.ListPollingCandidates(ctx, s.config.PollBatchSize)
	if err != nil {
		log.Printf("ai poller: load candidates failed: %v", err)
		return
	}

	concurrency := s.pollConcurrency()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for i := range items {
		item := items[i]
		wg.Add(1)
		go func(item model.AIEvaluation) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			pollCtx := ctx
			cancel := func() {}
			if timeout := s.pollTaskTimeout(); timeout > 0 {
				pollCtx, cancel = context.WithTimeout(ctx, timeout)
			}
			defer cancel()

			if err := s.pollEvaluation(pollCtx, &item); err != nil {
				log.Printf("ai poller: evaluation=%s failed: %v", item.ID, err)
			}
		}(item)
	}

	wg.Wait()
}

func (s *Service) pollEvaluation(ctx context.Context, item *model.AIEvaluation) error {
	if item.JobID == nil || strings.TrimSpace(*item.JobID) == "" {
		return nil
	}

	log.Printf("ai poller: polling evaluation=%s provider_job=%s", item.ID, strings.TrimSpace(*item.JobID))
	statusResult, err := s.client.GetJobStatus(ctx, *item.JobID)
	if err != nil {
		if isProviderJobNotFound(err) {
			inGrace, age, grace := s.jobNotFoundGraceDecision(item)
			if inGrace {
				log.Printf(
					"ai poller: evaluation=%s job=%s provider returned not found within grace period, will retry age=%s grace=%s",
					item.ID,
					strings.TrimSpace(*item.JobID),
					age,
					grace,
				)
				return s.markEvaluationProcessing(ctx, item)
			}
			log.Printf(
				"ai poller: evaluation=%s job=%s provider returned not found and grace period elapsed age=%s grace=%s",
				item.ID,
				strings.TrimSpace(*item.JobID),
				age,
				grace,
			)
			return s.failEvaluation(ctx, item, "ai job not found on provider")
		}
		return err
	}

	switch normalizeProviderStatus(statusResult.Status) {
	case EvaluationStatusProcessing:
		return s.markEvaluationProcessing(ctx, item)
	case EvaluationStatusCompleted:
		return s.completeEvaluationFromProvider(ctx, item, *item.JobID)
	case EvaluationStatusFailed:
		return s.failEvaluation(ctx, item, providerFailureMessage(statusResult.Status, statusResult.Message))
	default:
		return ErrProviderResponseInvalid
	}
}

func (s *Service) markEvaluationProcessing(ctx context.Context, item *model.AIEvaluation) error {
	now := time.Now()
	manualStatus, err := s.resolveManualStatus(ctx, item.VideoID)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"updated_at": now,
	}
	if item.StartedAt == nil {
		updates["started_at"] = now
	}

	if err := s.repo.UpdateEvaluationAndVideo(ctx, item.ID, updates, s.buildVideoStatusUpdates(manualStatus, evaluation.AIStatusProcessing, now)); err != nil {
		return err
	}
	return s.refreshTaskVideoStats(ctx, item.TaskID)
}

func (s *Service) completeEvaluationFromProvider(ctx context.Context, item *model.AIEvaluation, jobID string) error {
	result, err := s.client.GetJobResult(ctx, jobID)
	if err != nil {
		if isProviderJobNotFound(err) {
			return s.failEvaluation(ctx, item, "ai job not found on provider")
		}
		return err
	}

	now := time.Now()
	manualStatus, err := s.resolveManualStatus(ctx, item.VideoID)
	if err != nil {
		return err
	}
	resultData, err := providerResultData(result)
	if err != nil {
		return err
	}
	totalScore := extractTotalScore(result)
	overallStatus := evaluation.ResolveOverallStatus(manualStatus, evaluation.AIStatusCompleted)

	if err := s.repo.UpdateEvaluationAndVideo(ctx, item.ID, map[string]any{
		"status":        EvaluationStatusCompleted,
		"model_version": result.ModelVersion,
		"total_score":   totalScore,
		"result_data":   resultData,
		"completed_at":  now,
		"updated_at":    now,
	}, map[string]any{
		"ai_status":         evaluation.AIStatusCompleted,
		"ai_score":          totalScore,
		"evaluation_status": overallStatus,
		"completed_at":      evaluation.ResolveCompletedAt(overallStatus, now),
		"updated_at":        now,
	}); err != nil {
		return err
	}
	return s.refreshTaskVideoStats(ctx, item.TaskID)
}

func (s *Service) failEvaluation(ctx context.Context, item *model.AIEvaluation, message string) error {
	now := time.Now()
	manualStatus, err := s.resolveManualStatus(ctx, item.VideoID)
	if err != nil {
		return err
	}
	overallStatus := evaluation.ResolveOverallStatus(manualStatus, evaluation.AIStatusFailed)
	if err := s.repo.UpdateEvaluationAndVideo(ctx, item.ID, map[string]any{
		"status":        EvaluationStatusFailed,
		"error_message": message,
		"completed_at":  now,
		"updated_at":    now,
	}, map[string]any{
		"ai_status":         evaluation.AIStatusFailed,
		"evaluation_status": overallStatus,
		"completed_at":      evaluation.ResolveCompletedAt(overallStatus, now),
		"updated_at":        now,
	}); err != nil {
		return err
	}
	return s.refreshTaskVideoStats(ctx, item.TaskID)
}

func (s *Service) createConcurrency() int {
	if s.config.CreateConcurrency > 0 {
		return s.config.CreateConcurrency
	}
	return 5
}

func (s *Service) pollConcurrency() int {
	if s.config.PollConcurrency > 0 {
		return s.config.PollConcurrency
	}
	return 5
}

func (s *Service) pollTaskTimeout() time.Duration {
	timeout := s.config.RequestTimeout
	if timeout <= 0 {
		return 30 * time.Second
	}
	return timeout * 2
}

func (s *Service) withinJobNotFoundGracePeriod(item *model.AIEvaluation) bool {
	inGrace, _, _ := s.jobNotFoundGraceDecision(item)
	return inGrace
}

func (s *Service) jobNotFoundGraceDecision(item *model.AIEvaluation) (bool, time.Duration, time.Duration) {
	grace := s.config.JobNotFoundGrace
	if grace <= 0 {
		grace = 10 * time.Minute
	}

	reference := item.CreatedAt
	if item.StartedAt != nil && !item.StartedAt.IsZero() {
		reference = *item.StartedAt
	}
	age := time.Since(reference)
	return age < grace, age, grace
}

func isProviderJobNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "status=404") && strings.Contains(message, "job not found")
}

func toEvaluationDTO(item *model.AIEvaluation) *EvaluationDTO {
	return &EvaluationDTO{
		ID:           item.ID,
		VideoID:      item.VideoID,
		TaskID:       item.TaskID,
		Status:       item.Status,
		ModelVersion: item.ModelVersion,
		TotalScore:   item.TotalScore,
		ErrorMessage: item.ErrorMessage,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		StartedAt:    item.StartedAt,
		CompletedAt:  item.CompletedAt,
	}
}

func toEvaluationResultDTO(item *model.AIEvaluation) (*EvaluationResultDTO, error) {
	result, err := decodeStoredResult(item.ResultData)
	if err != nil {
		return nil, err
	}
	return &EvaluationResultDTO{
		ID:           item.ID,
		VideoID:      item.VideoID,
		TaskID:       item.TaskID,
		Status:       item.Status,
		ModelVersion: item.ModelVersion,
		Summary:      result.Summary,
		Details:      result.Details,
		VideoStages:  result.VideoStages,
		VideoPoints:  result.VideoPoints,
		Artifacts:    result.Artifacts,
		CompletedAt:  item.CompletedAt,
	}, nil
}

func toEmbeddedEvaluationDTO(item *model.AIEvaluation) (*EmbeddedEvaluationDTO, error) {
	dto := &EmbeddedEvaluationDTO{
		EvaluationID: item.ID,
		Status:       item.Status,
		Score:        item.TotalScore,
		ModelVersion: item.ModelVersion,
		ErrorMessage: item.ErrorMessage,
		StartedAt:    item.StartedAt,
		CompletedAt:  item.CompletedAt,
	}
	if len(item.ResultData) == 0 {
		return dto, nil
	}
	result, err := decodeStoredResult(item.ResultData)
	if err != nil {
		return nil, err
	}
	dto.Summary = result.Summary
	dto.Details = result.Details
	dto.VideoStages = result.VideoStages
	dto.VideoPoints = result.VideoPoints
	dto.Artifacts = result.Artifacts
	return dto, nil
}

func decodeStoredResult(raw map[string]any) (*storedResult, error) {
	if len(raw) == 0 {
		return &storedResult{}, nil
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var result storedResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
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

func normalizeProviderStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "processing":
		return EvaluationStatusProcessing
	case "completed":
		return EvaluationStatusCompleted
	case "failed", "cancelled":
		return EvaluationStatusFailed
	default:
		return ""
	}
}

func (s *Service) resolveManualStatus(ctx context.Context, videoID uuid.UUID) (string, error) {
	state, err := s.repo.FindVideoProgressState(ctx, videoID)
	if err != nil {
		return "", err
	}
	if state == nil || strings.TrimSpace(state.ManualStatus) == "" {
		return evaluation.ManualStatusPending, nil
	}
	return state.ManualStatus, nil
}

func (s *Service) buildVideoStatusUpdates(manualStatus, aiStatus string, now time.Time) map[string]any {
	overallStatus := evaluation.ResolveOverallStatus(manualStatus, aiStatus)
	return map[string]any{
		"ai_status":         aiStatus,
		"evaluation_status": overallStatus,
		"completed_at":      evaluation.ResolveCompletedAt(overallStatus, now),
		"updated_at":        now,
	}
}

func (s *Service) refreshTaskVideoStats(ctx context.Context, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return nil
	}
	return s.taskService.RefreshVideoStats(ctx, taskID)
}

func providerFailureMessage(status string, message *string) string {
	if message != nil && strings.TrimSpace(*message) != "" {
		return strings.TrimSpace(*message)
	}
	if strings.EqualFold(strings.TrimSpace(status), "cancelled") {
		return "ai provider job cancelled"
	}
	return "ai provider job failed"
}

func providerResultData(result *JobResult) (map[string]any, error) {
	payload, err := json.Marshal(storedResult{
		Summary:     result.Summary,
		Details:     result.Details,
		VideoStages: result.VideoStages,
		VideoPoints: result.VideoPoints,
		Artifacts:   result.Artifacts,
	})
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func extractTotalScore(result *JobResult) *float64 {
	if result == nil || result.Summary == nil || result.Summary.Score == nil {
		return nil
	}
	return result.Summary.Score
}
