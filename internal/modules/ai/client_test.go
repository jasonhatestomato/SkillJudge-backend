package ai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"skilljudge/backend/internal/config"
)

func TestClientGetJobResultSupportsSnakeCaseResponse(t *testing.T) {
	client := NewClient(config.AIConfig{
		BaseURL:        "https://provider.test",
		AnalysisPath:   "/api/v1/analysis-jobs",
		RequestTimeout: time.Second,
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.String() != "https://provider.test/api/v1/analysis-jobs/job_20260401_abcd/result" {
			t.Fatalf("unexpected url: %s", r.URL.String())
		}

		return jsonResponse(http.StatusOK, `{
			"job_id": "job_20260401_abcd",
			"video_stages": [
				{
					"stage_id": "stage_1",
					"name": "作业准备",
					"stage_type": "preparation",
					"start_sec": 0,
					"end_sec": 30,
					"start_time": "00:00:00",
					"end_time": "00:00:30"
				}
			],
			"video_points": [
				{
					"point_id": "point_1",
					"name": "未锁止工具车及台架",
					"type": "general_error",
					"severity": "general",
					"start_sec": 0,
					"end_sec": 15,
					"start_time": "00:00:00",
					"end_time": "00:00:15",
					"feedback": "视频中未见锁止工具车和台架的动作。",
					"evidences": [
						{
							"evidence_id": "ev_001",
							"kind": "image",
							"time_sec": 8.2,
							"url": "https://example.com/evidences/ev_001.jpg"
						}
					]
				}
			],
			"summary": {
				"overall_description": "该生操作流程熟练度尚可，但在标准作业流程的执行上存在重大缺失。",
				"score": 69,
				"max_score": 100
			},
			"details": [
				{
					"title": "一、作业准备",
					"full_score": 10,
					"ai_score": 2,
					"items": [
						{
							"subtitle": "个人防护",
							"full_score": 2,
							"ai_score": 2,
							"status": "correct",
							"feedback": "穿戴了工作服和手套，符合要求。"
						}
					]
				}
			],
			"artifacts": {
				"report_html_url": "https://example.com/reports/job_20260401_abcd.html",
				"analysis_json_url": "https://example.com/results/job_20260401_abcd/analysis.json",
				"evidence_index_json_url": "https://example.com/results/job_20260401_abcd/evidence_index.json"
			}
		}`)
	})}

	result, err := client.GetJobResult(context.Background(), "job_20260401_abcd")
	if err != nil {
		t.Fatalf("GetJobResult returned error: %v", err)
	}
	if result.JobID != "job_20260401_abcd" {
		t.Fatalf("unexpected job id: %s", result.JobID)
	}
	if result.Summary == nil || result.Summary.OverallDescription == nil || *result.Summary.OverallDescription == "" {
		t.Fatalf("summary.overallDescription was not decoded")
	}
	if result.Summary.MaxScore == nil || *result.Summary.MaxScore != 100 {
		t.Fatalf("unexpected max score: %+v", result.Summary.MaxScore)
	}
	if len(result.VideoStages) != 1 || result.VideoStages[0].StageID == nil || *result.VideoStages[0].StageID != "stage_1" {
		t.Fatalf("video stages were not decoded: %+v", result.VideoStages)
	}
	if result.VideoStages[0].StageType == nil || *result.VideoStages[0].StageType != "preparation" {
		t.Fatalf("unexpected stage type: %+v", result.VideoStages[0].StageType)
	}
	if len(result.VideoPoints) != 1 || result.VideoPoints[0].PointID == nil || *result.VideoPoints[0].PointID != "point_1" {
		t.Fatalf("video points were not decoded: %+v", result.VideoPoints)
	}
	if len(result.VideoPoints[0].Evidences) != 1 || result.VideoPoints[0].Evidences[0].EvidenceID == nil || *result.VideoPoints[0].Evidences[0].EvidenceID != "ev_001" {
		t.Fatalf("evidences were not decoded: %+v", result.VideoPoints[0].Evidences)
	}
	if result.Artifacts == nil || result.Artifacts.ReportHTMLURL == nil || *result.Artifacts.ReportHTMLURL == "" {
		t.Fatalf("artifacts.reportHtmlUrl was not decoded")
	}
}

func TestClientCreateAndStatusSupportSnakeCaseJobID(t *testing.T) {
	client := NewClient(config.AIConfig{
		BaseURL:        "https://provider.test",
		AnalysisPath:   "/api/v1/analysis-jobs",
		RequestTimeout: time.Second,
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.String() == "https://provider.test/api/v1/analysis-jobs":
			return jsonResponse(http.StatusOK, `{
				"job_id": "job_20260401_abcd",
				"status": "queued",
				"accepted_at": "2026-04-01T08:00:00Z",
				"evaluation_id": "eval_001"
			}`)
		case r.Method == http.MethodGet && r.URL.String() == "https://provider.test/api/v1/analysis-jobs/job_20260401_abcd":
			return jsonResponse(http.StatusOK, `{
				"job_id": "job_20260401_abcd",
				"status": "processing",
				"progress": 60,
				"message": "running",
				"result_slices": {"chunk": 1}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})}

	createResult, err := client.CreateJob(context.Background(), CreateJobInput{
		EvaluationID: "eval_001",
		VideoURL:     "https://example.com/video.mp4",
		TaskID:       "task_001",
		VideoID:      "video_001",
		RubricID:     "rubric_001",
		RubricName:   "rubric",
		RubricJSON:   map[string]any{"id": "rubric_001"},
	})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if createResult.JobID != "job_20260401_abcd" {
		t.Fatalf("unexpected created job id: %s", createResult.JobID)
	}
	if createResult.AcceptedAt == nil {
		t.Fatalf("acceptedAt was not decoded")
	}

	statusResult, err := client.GetJobStatus(context.Background(), "job_20260401_abcd")
	if err != nil {
		t.Fatalf("GetJobStatus returned error: %v", err)
	}
	if statusResult.JobID != "job_20260401_abcd" {
		t.Fatalf("unexpected status job id: %s", statusResult.JobID)
	}
	if statusResult.Progress == nil || *statusResult.Progress != 60 {
		t.Fatalf("unexpected progress: %+v", statusResult.Progress)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
