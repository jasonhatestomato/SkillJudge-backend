package task

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"skilljudge/backend/internal/model"
)

func floatPtr(value float64) *float64 {
	return &value
}

func TestBuildNormalizedDistribution(t *testing.T) {
	items := []model.Video{
		{ManualScore: floatPtr(10)},
		{ManualScore: floatPtr(25)},
		{ManualScore: floatPtr(40)},
		{ManualScore: floatPtr(50)},
	}

	result := buildNormalizedDistribution(items, 50, normalizedScoreBuckets, func(video model.Video) *float64 {
		return video.ManualScore
	})

	if len(result) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(result))
	}
	if result[0].Count != 1 || result[2].Count != 1 || result[3].Count != 1 || result[4].Count != 1 {
		t.Fatalf("unexpected distribution: %+v", result)
	}
}

func TestBuildGapDistribution(t *testing.T) {
	items := []model.Video{
		{ManualScore: floatPtr(45), AIScore: floatPtr(44)},
		{ManualScore: floatPtr(45), AIScore: floatPtr(40)},
		{ManualScore: floatPtr(45), AIScore: floatPtr(30)},
	}

	result := buildGapDistribution(items, 50, normalizedGapBuckets)

	if len(result) != 4 {
		t.Fatalf("expected 4 buckets, got %d", len(result))
	}
	if result[0].Count != 1 || result[1].Count != 1 || result[3].Count != 1 {
		t.Fatalf("unexpected gap distribution: %+v", result)
	}
}

func TestBuildTaskAnalysisHTML(t *testing.T) {
	html, err := buildTaskAnalysisHTML(AnalysisResult{
		Task: AnalysisTaskDTO{
			Name:               "任务A",
			RubricTotalScore:   50,
			TotalVideos:        30,
			CompletedVideos:    30,
			AllVideosCompleted: true,
		},
		ScoreSummary: AnalysisScoreSummaryDTO{
			AverageAIScore:     floatPtr(41.2),
			AverageManualScore: floatPtr(40.1),
		},
		ManualScoreDistribution: []AnalysisDistributionBucketDTO{{Label: "0-20", Count: 1}},
		AIScoreDistribution:     []AnalysisDistributionBucketDTO{{Label: "0-20", Count: 2}},
		ScoreGapDistribution:    []AnalysisDistributionBucketDTO{{Label: "0-5", Count: 3}},
	})
	if err != nil {
		t.Fatalf("buildTaskAnalysisHTML returned error: %v", err)
	}
	if !strings.Contains(html, "任务分析报告") {
		t.Fatalf("expected report title in html, got %q", html)
	}
	if !strings.Contains(html, "人工成绩分布（按满分归一化）") {
		t.Fatalf("expected distribution section in html, got %q", html)
	}
}

func TestBuildTaskAnalysisPDF(t *testing.T) {
	if os.Getenv("RUN_CHROME_RENDER_TEST") != "1" {
		t.Skip("set RUN_CHROME_RENDER_TEST=1 to run chrome-backed pdf rendering test")
	}
	if _, err := resolveChromiumBinary(); err != nil {
		t.Skipf("skipping chrome-backed pdf test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	report, err := buildTaskAnalysisPDF(ctx, AnalysisResult{
		Task: AnalysisTaskDTO{
			Name:               "任务A",
			RubricTotalScore:   50,
			TotalVideos:        30,
			CompletedVideos:    30,
			AllVideosCompleted: true,
		},
		ScoreSummary: AnalysisScoreSummaryDTO{
			AverageAIScore:     floatPtr(41.2),
			AverageManualScore: floatPtr(40.1),
		},
		ManualScoreDistribution: []AnalysisDistributionBucketDTO{{Label: "0-20", Count: 1}},
		AIScoreDistribution:     []AnalysisDistributionBucketDTO{{Label: "0-20", Count: 2}},
		ScoreGapDistribution:    []AnalysisDistributionBucketDTO{{Label: "0-5", Count: 3}},
	})
	if err != nil {
		t.Fatalf("buildTaskAnalysisPDF returned error: %v", err)
	}
	if len(report) == 0 {
		t.Fatal("expected non-empty pdf bytes")
	}
	if !bytes.HasPrefix(report, []byte("%PDF-")) {
		t.Fatalf("expected pdf header, got %q", report[:8])
	}
}
