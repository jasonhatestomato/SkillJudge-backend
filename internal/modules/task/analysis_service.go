package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

const (
	taskAnalysisReportStatusQueued     = "queued"
	taskAnalysisReportStatusProcessing = "processing"
	taskAnalysisReportStatusReady      = "ready"
	taskAnalysisReportStatusFailed     = "failed"
	taskAnalysisReportFormatPDF        = "pdf"
	taskAnalysisTemplateVersion        = "task-analysis-report-v1"
	taskAnalysisReportPollInterval     = 3 * time.Second
	taskAnalysisReportPollBatchSize    = 5
	taskAnalysisReportPollConcurrency  = 2
)

type distributionBucket struct {
	Label string
	Min   float64
	Max   float64
}

var (
	normalizedScoreBuckets = []distributionBucket{
		{Label: "0-20", Min: 0, Max: 20},
		{Label: "21-40", Min: 20, Max: 40},
		{Label: "41-60", Min: 40, Max: 60},
		{Label: "61-80", Min: 60, Max: 80},
		{Label: "81-100", Min: 80, Max: 100},
	}
	normalizedGapBuckets = []distributionBucket{
		{Label: "0-5", Min: 0, Max: 5},
		{Label: "6-10", Min: 5, Max: 10},
		{Label: "11-20", Min: 10, Max: 20},
		{Label: "21+", Min: 20, Max: math.MaxFloat64},
	}
)

func (s *Service) GetAnalysis(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*AnalysisResult, error) {
	resolved, err := s.Resolve(ctx, actor, taskID, AccessRead)
	if err != nil {
		return nil, err
	}
	if !isTaskAnalysisReady(resolved.Item) {
		return nil, ErrTaskAnalysisNotReady
	}

	return s.buildAnalysisResult(ctx, resolved.Item)
}

func (s *Service) GetAnalysisReport(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*AnalysisReportDTO, error) {
	resolved, err := s.Resolve(ctx, actor, taskID, AccessRead)
	if err != nil {
		return nil, err
	}
	if !isTaskAnalysisReady(resolved.Item) {
		return nil, ErrTaskAnalysisNotReady
	}

	item, err := s.repo.FindLatestAnalysisReportByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrTaskAnalysisReportNotFound
	}

	return ToAnalysisReportDTO(item), nil
}

func (s *Service) GenerateAnalysisReport(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*AnalysisReportDTO, error) {
	resolved, err := s.Resolve(ctx, actor, taskID, AccessManage)
	if err != nil {
		return nil, err
	}
	if !isTaskAnalysisReady(resolved.Item) {
		return nil, ErrTaskAnalysisNotReady
	}
	templateVersion := taskAnalysisTemplateVersion
	report, _, err := s.repo.EnsureQueuedAnalysisReport(ctx, taskID, &actor.UserID, &templateVersion)
	if err != nil {
		return nil, err
	}
	return ToAnalysisReportDTO(report), nil
}

func (s *Service) RunReportWorker(ctx context.Context) {
	log.Printf("task analysis report worker starting")
	ticker := time.NewTicker(taskAnalysisReportPollInterval)
	defer ticker.Stop()

	for {
		s.processQueuedReports(ctx)

		select {
		case <-ctx.Done():
			log.Printf("task analysis report worker stopping: %v", ctx.Err())
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) processQueuedReports(ctx context.Context) {
	items, err := s.repo.ListQueuedAnalysisReports(ctx, taskAnalysisReportPollBatchSize)
	if err != nil {
		log.Printf("task analysis report worker: load queued reports failed: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, taskAnalysisReportPollConcurrency)
	for i := range items {
		item := items[i]
		wg.Add(1)
		go func(report model.TaskAnalysisReport) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()
			if err := s.processQueuedReport(ctx, report); err != nil {
				log.Printf("task analysis report worker: report=%s failed: %v", report.ID, err)
			}
		}(item)
	}
	wg.Wait()
}

func (s *Service) processQueuedReport(ctx context.Context, report model.TaskAnalysisReport) error {
	startedAt := time.Now()
	claimed, err := s.repo.ClaimAnalysisReport(ctx, report.ID, startedAt)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	latest, err := s.repo.FindLatestAnalysisReportByTaskID(ctx, report.TaskID)
	if err != nil {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("load latest analysis report: %w", err))
	}
	if latest == nil || latest.ID != report.ID {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("analysis report superseded by a newer record"))
	}

	taskItem, err := s.repo.FindByID(ctx, report.TaskID)
	if err != nil {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("load task: %w", err))
	}
	if taskItem == nil {
		return s.failAnalysisReport(ctx, report.ID, ErrTaskNotFound)
	}
	if !isTaskAnalysisReady(taskItem) {
		return s.failAnalysisReport(ctx, report.ID, ErrTaskAnalysisNotReady)
	}

	analysis, err := s.buildAnalysisResult(ctx, taskItem)
	if err != nil {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("build analysis result: %w", err))
	}

	snapshot, err := toMap(analysis)
	if err != nil {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("marshal analysis snapshot: %w", err))
	}

	fileName := buildTaskAnalysisReportFileName(taskItem.Name)
	pdf, err := buildTaskAnalysisPDF(ctx, *analysis)
	if err != nil {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("build analysis report pdf: %w", err))
	}

	objectKey := buildTaskAnalysisReportObjectKey(taskItem.ProjectID, report.TaskID, report.ID, fileName)
	putResult, err := s.storage.PutObject(ctx, storage.PutObjectInput{
		ObjectKey:   objectKey,
		ContentType: "application/pdf",
		Body:        pdf,
	})
	if err != nil {
		return s.failAnalysisReport(ctx, report.ID, fmt.Errorf("upload analysis report: %w", err))
	}

	generatedAt := time.Now()
	if err := s.repo.UpdateAnalysisReport(ctx, report.ID, map[string]any{
		"status":           taskAnalysisReportStatusReady,
		"file_name":        fileName,
		"storage_path":     putResult.StoragePath,
		"public_url":       putResult.StorageURL,
		"snapshot_data":    snapshot,
		"generated_at":     generatedAt,
		"error_message":    nil,
		"template_version": taskAnalysisTemplateVersion,
		"updated_at":       generatedAt,
	}); err != nil {
		return err
	}

	return nil
}

func (s *Service) failAnalysisReport(ctx context.Context, reportID uuid.UUID, cause error) error {
	message := cause.Error()
	now := time.Now()
	if err := s.repo.UpdateAnalysisReport(ctx, reportID, map[string]any{
		"status":        taskAnalysisReportStatusFailed,
		"error_message": message,
		"updated_at":    now,
	}); err != nil {
		return err
	}
	return cause
}

func (s *Service) buildAnalysisResult(ctx context.Context, taskItem *model.Task) (*AnalysisResult, error) {
	videos, err := s.repo.ListAnalysisVideos(ctx, taskItem.ID)
	if err != nil {
		return nil, err
	}

	rubricTotalScore := 0
	if taskItem.Rubric != nil {
		rubricTotalScore = taskItem.Rubric.TotalScore
	}

	manualScores := collectScorePointers(videos, func(video model.Video) *float64 { return video.ManualScore })
	aiScores := collectScorePointers(videos, func(video model.Video) *float64 { return video.AIScore })
	manualDistribution := buildNormalizedDistribution(videos, rubricTotalScore, normalizedScoreBuckets, func(video model.Video) *float64 {
		return video.ManualScore
	})
	aiDistribution := buildNormalizedDistribution(videos, rubricTotalScore, normalizedScoreBuckets, func(video model.Video) *float64 {
		return video.AIScore
	})
	gapDistribution := buildGapDistribution(videos, rubricTotalScore, normalizedGapBuckets)

	return &AnalysisResult{
		Task: ToAnalysisTaskDTO(taskItem),
		ScoreSummary: AnalysisScoreSummaryDTO{
			AverageAIScore:     averageFloat64Pointers(aiScores),
			AverageManualScore: averageFloat64Pointers(manualScores),
			HighestAIScore:     maxFloat64Pointers(aiScores),
			LowestAIScore:      minFloat64Pointers(aiScores),
			HighestManualScore: maxFloat64Pointers(manualScores),
			LowestManualScore:  minFloat64Pointers(manualScores),
		},
		ManualScoreDistribution: manualDistribution,
		AIScoreDistribution:     aiDistribution,
		ScoreGapDistribution:    gapDistribution,
	}, nil
}

func isTaskAnalysisReady(taskItem *model.Task) bool {
	return taskItem != nil && taskItem.TotalVideos > 0 && taskItem.TotalVideos == taskItem.CompletedVideos
}

func collectScorePointers(items []model.Video, selector func(model.Video) *float64) []*float64 {
	result := make([]*float64, 0, len(items))
	for _, item := range items {
		if score := selector(item); score != nil {
			result = append(result, score)
		}
	}
	return result
}

func averageFloat64Pointers(values []*float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	var sum float64
	for _, value := range values {
		sum += *value
	}
	result := sum / float64(len(values))
	return &result
}

func maxFloat64Pointers(values []*float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	current := *values[0]
	for _, value := range values[1:] {
		if *value > current {
			current = *value
		}
	}
	return &current
}

func minFloat64Pointers(values []*float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	current := *values[0]
	for _, value := range values[1:] {
		if *value < current {
			current = *value
		}
	}
	return &current
}

func buildNormalizedDistribution(items []model.Video, rubricTotalScore int, buckets []distributionBucket, selector func(model.Video) *float64) []AnalysisDistributionBucketDTO {
	result := make([]AnalysisDistributionBucketDTO, len(buckets))
	for i, bucket := range buckets {
		result[i] = AnalysisDistributionBucketDTO{Label: bucket.Label}
	}

	if rubricTotalScore <= 0 {
		return result
	}

	for _, item := range items {
		score := selector(item)
		if score == nil {
			continue
		}
		normalized := (*score / float64(rubricTotalScore)) * 100
		index := matchBucket(normalized, buckets)
		result[index].Count++
	}

	return result
}

func buildGapDistribution(items []model.Video, rubricTotalScore int, buckets []distributionBucket) []AnalysisDistributionBucketDTO {
	result := make([]AnalysisDistributionBucketDTO, len(buckets))
	for i, bucket := range buckets {
		result[i] = AnalysisDistributionBucketDTO{Label: bucket.Label}
	}

	if rubricTotalScore <= 0 {
		return result
	}

	for _, item := range items {
		if item.ManualScore == nil || item.AIScore == nil {
			continue
		}
		gap := math.Abs(*item.ManualScore-*item.AIScore) / float64(rubricTotalScore) * 100
		index := matchBucket(gap, buckets)
		result[index].Count++
	}

	return result
}

func matchBucket(value float64, buckets []distributionBucket) int {
	for index, bucket := range buckets {
		if index == 0 {
			if value >= bucket.Min && value <= bucket.Max {
				return index
			}
			continue
		}
		if value > bucket.Min && value <= bucket.Max {
			return index
		}
	}
	return len(buckets) - 1
}

func toMap(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func buildTaskAnalysisReportFileName(taskName string) string {
	name := sanitizeReportFileName(taskName)
	if name == "" {
		name = "task-analysis"
	}
	return fmt.Sprintf("%s-analysis-report.pdf", name)
}

func sanitizeReportFileName(value string) string {
	if value == "" {
		return ""
	}
	result := make([]rune, 0, len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			result = append(result, r)
		case r >= 'A' && r <= 'Z':
			result = append(result, r)
		case r >= '0' && r <= '9':
			result = append(result, r)
		case r == '-' || r == '_':
			result = append(result, r)
		case r == ' ':
			result = append(result, '-')
		}
	}
	return string(result)
}

func buildTaskAnalysisReportObjectKey(projectID, taskID, reportID uuid.UUID, fileName string) string {
	return fmt.Sprintf("projects/%s/tasks/%s/analysis-reports/%s/%s", projectID.String(), taskID.String(), reportID.String(), fileName)
}
