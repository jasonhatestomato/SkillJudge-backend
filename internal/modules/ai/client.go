package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"skilljudge/backend/internal/config"
)

type Client struct {
	httpClient   *http.Client
	baseURL      string
	analysisPath string
	apiToken     string
}

type CreateJobInput struct {
	EvaluationID string
	VideoURL     string
	StoragePath  *string
	TaskID       string
	VideoID      string
	RubricID     string
	RubricName   string
	RubricJSON   any
}

type CreateJobResult struct {
	JobID        string     `json:"jobId"`
	Status       string     `json:"status"`
	AcceptedAt   *time.Time `json:"acceptedAt"`
	EvaluationID *string    `json:"evaluationId,omitempty"`
}

type JobStatusResult struct {
	JobID        string  `json:"jobId"`
	Status       string  `json:"status"`
	Progress     *int    `json:"progress,omitempty"`
	Message      *string `json:"message,omitempty"`
	ResultSlices any     `json:"result_slices,omitempty"`
}

type JobResult struct {
	JobID        string           `json:"jobId"`
	ModelVersion *string          `json:"modelVersion,omitempty"`
	Summary      *SummaryDTO      `json:"summary,omitempty"`
	Details      []DetailGroupDTO `json:"details,omitempty"`
	VideoStages  []VideoStageDTO  `json:"videoStages,omitempty"`
	VideoPoints  []VideoPointDTO  `json:"videoPoints,omitempty"`
	Artifacts    *ArtifactsDTO    `json:"artifacts,omitempty"`
}

type createJobRequest struct {
	Video struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"video"`
	Rubric struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"rubric"`
	RubricData any            `json:"rubricData,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

func NewClient(cfg config.AIConfig) *Client {
	return &Client{
		httpClient:   config.NewHTTPClient(cfg.RequestTimeout),
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		analysisPath: cfg.AnalysisPath,
		apiToken:     strings.TrimSpace(cfg.APIToken),
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

func (c *Client) CreateJob(ctx context.Context, input CreateJobInput) (*CreateJobResult, error) {
	if !c.Configured() {
		return nil, ErrProviderNotConfigured
	}

	rubricBytes, err := json.Marshal(input.RubricJSON)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal rubric", ErrProviderRequestFailed)
	}

	requestBody := createJobRequest{
		RubricData: input.RubricJSON,
		Metadata: map[string]any{
			"evaluationId": input.EvaluationID,
			"taskId":       input.TaskID,
			"videoId":      input.VideoID,
			"rubricId":     input.RubricID,
		},
		Options: map[string]any{
			"needStageSegmentation": true,
			"needErrorPoints":       true,
			"needReport":            true,
			"reportFormat":          "html",
		},
	}
	requestBody.Video.Type = "url"
	requestBody.Video.Value = input.VideoURL
	requestBody.Rubric.Type = "content"
	requestBody.Rubric.Value = string(rubricBytes)
	if input.StoragePath != nil && strings.TrimSpace(*input.StoragePath) != "" {
		requestBody.Metadata["storagePath"] = *input.StoragePath
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request", ErrProviderRequestFailed)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+normalizePath(c.analysisPath), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrProviderRequestFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProviderRequestFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result CreateJobResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: decode create response", ErrProviderResponseInvalid)
	}
	if strings.TrimSpace(result.JobID) == "" {
		return nil, fmt.Errorf("%w: missing jobId", ErrProviderResponseInvalid)
	}

	return &result, nil
}

func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*JobStatusResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+normalizePath(c.analysisPath)+"/"+jobID, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build status request", ErrProviderRequestFailed)
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProviderRequestFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result JobStatusResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: decode status response", ErrProviderResponseInvalid)
	}
	if strings.TrimSpace(result.JobID) == "" || strings.TrimSpace(result.Status) == "" {
		return nil, fmt.Errorf("%w: missing jobId or status", ErrProviderResponseInvalid)
	}

	return &result, nil
}

func (c *Client) GetJobResult(ctx context.Context, jobID string) (*JobResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+normalizePath(c.analysisPath)+"/"+jobID+"/result", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build result request", ErrProviderRequestFailed)
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProviderRequestFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result JobResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: decode result response", ErrProviderResponseInvalid)
	}
	if strings.TrimSpace(result.JobID) == "" {
		return nil, fmt.Errorf("%w: missing jobId", ErrProviderResponseInvalid)
	}

	return &result, nil
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/api/v1/analysis-jobs"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}
