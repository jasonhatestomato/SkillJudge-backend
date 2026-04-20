package video

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skilljudge/backend/internal/modules/ai"

	"github.com/google/uuid"
)

const (
	defaultVideoAIReportRenderTimeout = 20 * time.Second
	videoAIReportTemplateVersion      = "video-ai-report-v1"
	videoAIReportFormatPDF            = "pdf"
	videoAIReportStatusReady          = "ready"
	videoAIReportStatusFailed         = "failed"
)

var chromiumBinaryCandidates = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/usr/bin/google-chrome",
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/snap/bin/chromium",
}

//go:embed ai_report_template.html
var videoAIReportTemplate string

type videoAIReportView struct {
	GeneratedAt         string
	TaskName            string
	RubricName          string
	StudentName         string
	StudentNumber       string
	VideoFileName       string
	VideoDuration       string
	TotalScoreDisplay   string
	TotalPercentDisplay string
	DetailGroups        []videoAIReportGroupView
}

type videoAIReportGroupView struct {
	Title        string
	ScoreDisplay string
	Items        []videoAIReportItemView
}

type videoAIReportItemView struct {
	Subtitle     string
	Feedback     string
	StatusLabel  string
	ScoreDisplay string
	Times        []string
	Screenshots  []string
}

type videoAIReportMetadata struct {
	ReportStatus    string     `json:"reportStatus"`
	ReportType      string     `json:"reportType"`
	FileName        *string    `json:"fileName,omitempty"`
	HTMLFileName    *string    `json:"htmlFileName,omitempty"`
	PDFStoragePath  *string    `json:"pdfStoragePath,omitempty"`
	PDFPublicURL    *string    `json:"pdfPublicUrl,omitempty"`
	HTMLStoragePath *string    `json:"htmlStoragePath,omitempty"`
	HTMLPublicURL   *string    `json:"htmlPublicUrl,omitempty"`
	TemplateVersion *string    `json:"templateVersion,omitempty"`
	GeneratedAt     *time.Time `json:"generatedAt,omitempty"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	EvaluationID    *string    `json:"evaluationId,omitempty"`
}

func buildVideoAIReportHTML(item *videoReportContext) (string, error) {
	view := toVideoAIReportView(item)
	tmpl, err := template.New("video-ai-report").Parse(videoAIReportTemplate)
	if err != nil {
		return "", fmt.Errorf("parse video ai report template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		return "", fmt.Errorf("execute video ai report template: %w", err)
	}
	return buf.String(), nil
}

func buildVideoAIReportPDF(ctx context.Context, htmlDocument string) ([]byte, error) {
	renderCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		renderCtx, cancel = context.WithTimeout(ctx, defaultVideoAIReportRenderTimeout)
		defer cancel()
	}

	chromiumBin, err := resolveChromiumBinary()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "skilljudge-video-ai-report-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "report.html")
	pdfPath := filepath.Join(tmpDir, "report.pdf")
	profileDir := filepath.Join(tmpDir, "profile")
	if err := os.WriteFile(htmlPath, []byte(htmlDocument), 0o600); err != nil {
		return nil, fmt.Errorf("write report html: %w", err)
	}

	args := []string{
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--allow-file-access-from-files",
		"--disable-background-networking",
		"--disable-default-apps",
		"--disable-extensions",
		"--metrics-recording-only",
		"--no-first-run",
		"--no-default-browser-check",
		"--mute-audio",
		"--hide-scrollbars",
		"--user-data-dir=" + profileDir,
		"--no-pdf-header-footer",
		"--print-to-pdf=" + pdfPath,
		"file://" + htmlPath,
	}

	cmd := exec.CommandContext(renderCtx, chromiumBin, args...)
	output, runErr := cmd.CombinedOutput()

	pdfBytes, readErr := os.ReadFile(pdfPath)
	if len(pdfBytes) > 0 && bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		return pdfBytes, nil
	}
	if readErr != nil {
		if runErr != nil {
			return nil, fmt.Errorf("run chrome pdf render: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		return nil, fmt.Errorf("read rendered pdf: %w", readErr)
	}
	if runErr != nil {
		return nil, fmt.Errorf("run chrome pdf render: %w: %s", runErr, strings.TrimSpace(string(output)))
	}
	return nil, fmt.Errorf("chrome finished without producing a valid pdf")
}

func resolveChromiumBinary() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CHROMIUM_BIN")); value != "" {
		if _, err := os.Stat(value); err != nil {
			return "", fmt.Errorf("CHROMIUM_BIN is set but not usable: %w", err)
		}
		return value, nil
	}

	for _, candidate := range chromiumBinaryCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	for _, name := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("chromium binary not found; set CHROMIUM_BIN to a local Chrome/Chromium executable")
}

type videoReportContext struct {
	ProjectID    uuid.UUID
	TaskID       uuid.UUID
	RawVideo     *modelVideoBridge
	AIEvaluation *ai.EmbeddedEvaluationDTO
}

type modelVideoBridge struct {
	TaskName        string
	RubricName      string
	StudentName     string
	StudentNumber   string
	Filename        string
	DurationSeconds *int
}

func toVideoAIReportView(item *videoReportContext) videoAIReportView {
	totalScore := 0.0
	totalMaxScore := 0.0
	if item.AIEvaluation.Summary != nil {
		if item.AIEvaluation.Summary.Score != nil {
			totalScore = *item.AIEvaluation.Summary.Score
		}
		if item.AIEvaluation.Summary.MaxScore != nil {
			totalMaxScore = *item.AIEvaluation.Summary.MaxScore
		}
	}
	if totalMaxScore <= 0 {
		for _, group := range item.AIEvaluation.Details {
			if group.FullScore != nil {
				totalMaxScore += *group.FullScore
			}
			if group.AIScore != nil {
				totalScore += *group.AIScore
			}
		}
	}
	percent := 0.0
	if totalMaxScore > 0 {
		percent = totalScore / totalMaxScore * 100
	}

	groups := make([]videoAIReportGroupView, 0, len(item.AIEvaluation.Details))
	for _, group := range item.AIEvaluation.Details {
		groupView := videoAIReportGroupView{
			Title:        strings.TrimSpace(group.Title),
			ScoreDisplay: formatAIScore(group.AIScore, group.FullScore),
			Items:        make([]videoAIReportItemView, 0, len(group.Items)),
		}
		for _, detail := range group.Items {
			screenshots := make([]string, 0)
			if detail.Evidence != nil {
				for _, raw := range detail.Evidence.Screenshots {
					if source := resolveScreenshotSource(raw); source != "" {
						screenshots = append(screenshots, source)
					}
				}
			}
			itemView := videoAIReportItemView{
				Subtitle:     strings.TrimSpace(detail.Subtitle),
				Feedback:     derefString(detail.Feedback),
				StatusLabel:  normalizeReportStatus(detail.Status),
				ScoreDisplay: formatAIScore(detail.AIScore, detail.FullScore),
				Times:        nil,
				Screenshots:  screenshots,
			}
			if detail.Evidence != nil {
				itemView.Times = append(itemView.Times, detail.Evidence.Times...)
			}
			groupView.Items = append(groupView.Items, itemView)
		}
		groups = append(groups, groupView)
	}

	return videoAIReportView{
		GeneratedAt:         time.Now().Format("2006-01-02 15:04:05"),
		TaskName:            item.RawVideo.TaskName,
		RubricName:          item.RawVideo.RubricName,
		StudentName:         item.RawVideo.StudentName,
		StudentNumber:       item.RawVideo.StudentNumber,
		VideoFileName:       item.RawVideo.Filename,
		VideoDuration:       formatDuration(item.RawVideo.DurationSeconds),
		TotalScoreDisplay:   formatAIScore(floatPtr(totalScore), floatPtr(totalMaxScore)),
		TotalPercentDisplay: fmt.Sprintf("%.1f%%", percent),
		DetailGroups:        groups,
	}
}

func formatAIScore(score *float64, maxScore *float64) string {
	if score == nil && maxScore == nil {
		return "--"
	}
	left := "--"
	right := "--"
	if score != nil {
		left = formatFloat(*score)
	}
	if maxScore != nil {
		right = formatFloat(*maxScore)
	}
	return fmt.Sprintf("%s/%s", left, right)
}

func formatFloat(value float64) string {
	if float64(int(value)) == value {
		return fmt.Sprintf("%d", int(value))
	}
	return fmt.Sprintf("%.2f", value)
}

func formatDuration(seconds *int) string {
	if seconds == nil || *seconds <= 0 {
		return "--"
	}
	total := *seconds
	minutes := total / 60
	remain := total % 60
	return fmt.Sprintf("%02d:%02d", minutes, remain)
}

func normalizeReportStatus(status *string) string {
	if status == nil || strings.TrimSpace(*status) == "" {
		return "待确认"
	}
	value := strings.TrimSpace(*status)
	switch strings.ToLower(value) {
	case "ok", "correct", "passed", "completed":
		return "完成"
	case "error", "incorrect", "failed":
		return "未完成"
	default:
		return value
	}
}

func resolveScreenshotSource(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return value
	}
	if strings.HasPrefix(lower, "file://") {
		value = strings.TrimPrefix(value, "file://")
	}
	data, mimeType, err := readScreenshotBytes(value)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

func readScreenshotBytes(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return data, mimeType, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func floatPtr(value float64) *float64 {
	return &value
}
