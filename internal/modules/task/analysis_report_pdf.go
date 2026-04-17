package task

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultTaskAnalysisRenderTimeout = 20 * time.Second

var chromiumBinaryCandidates = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/usr/bin/google-chrome",
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/snap/bin/chromium",
}

type analysisReportView struct {
	GeneratedAt             string
	TaskName                string
	RubricTotalScore        int
	TotalVideos             int
	CompletedVideos         int
	AverageManualScore      string
	AverageAIScore          string
	HighestManualScore      string
	LowestManualScore       string
	HighestAIScore          string
	LowestAIScore           string
	ManualScoreDistribution []analysisDistributionView
	AIScoreDistribution     []analysisDistributionView
	ScoreGapDistribution    []analysisDistributionView
}

type analysisDistributionView struct {
	Label string
	Count int
	Width int
}

func buildTaskAnalysisPDF(ctx context.Context, result AnalysisResult) ([]byte, error) {
	renderCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		renderCtx, cancel = context.WithTimeout(ctx, defaultTaskAnalysisRenderTimeout)
		defer cancel()
	}

	htmlDocument, err := buildTaskAnalysisHTML(result)
	if err != nil {
		return nil, fmt.Errorf("render analysis html: %w", err)
	}

	chromiumBin, err := resolveChromiumBinary()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "skilljudge-task-analysis-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "report.html")
	pdfPath := filepath.Join(tmpDir, "report.pdf")
	profileDir := filepath.Join(tmpDir, "profile")
	if err := os.WriteFile(htmlPath, []byte(htmlDocument), 0o600); err != nil {
		return nil, fmt.Errorf("write analysis html: %w", err)
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

func buildTaskAnalysisHTML(result AnalysisResult) (string, error) {
	view := toAnalysisReportView(result)

	const tpl = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>任务分析报告</title>
  <style>
    @page { size: A4; margin: 16mm 14mm; }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      color: #1b2430;
      font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
      background: #ffffff;
      font-size: 14px;
      line-height: 1.5;
    }
    .report {
      padding: 8px 0 0;
    }
    .header {
      border-bottom: 3px solid #1f4b99;
      padding-bottom: 12px;
      margin-bottom: 18px;
    }
    .title {
      margin: 0;
      font-size: 28px;
      font-weight: 700;
      letter-spacing: 1px;
      color: #102544;
    }
    .meta {
      margin-top: 8px;
      color: #546377;
      font-size: 13px;
    }
    .section {
      margin-top: 18px;
      page-break-inside: avoid;
    }
    .section h2 {
      margin: 0 0 10px;
      font-size: 18px;
      color: #17315c;
    }
    .overview-grid {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 10px;
    }
    .metric-card {
      background: #f4f7fb;
      border: 1px solid #dbe5f2;
      border-radius: 10px;
      padding: 12px;
      min-height: 92px;
    }
    .metric-label {
      font-size: 12px;
      color: #627286;
      margin-bottom: 6px;
    }
    .metric-value {
      font-size: 24px;
      font-weight: 700;
      color: #102544;
    }
    .summary-table {
      width: 100%;
      border-collapse: collapse;
      background: #fff;
      border: 1px solid #dbe5f2;
    }
    .summary-table th, .summary-table td {
      padding: 10px 12px;
      border-bottom: 1px solid #e6edf7;
      text-align: left;
    }
    .summary-table th {
      width: 22%;
      background: #f6f9fd;
      color: #526174;
      font-weight: 600;
    }
    .chart-grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 16px;
    }
    .chart-card {
      border: 1px solid #dbe5f2;
      border-radius: 10px;
      padding: 14px 16px;
      background: #fff;
      page-break-inside: avoid;
    }
    .chart-title {
      margin: 0 0 12px;
      font-size: 15px;
      font-weight: 700;
      color: #17315c;
    }
    .bucket {
      display: grid;
      grid-template-columns: 72px 1fr 48px;
      gap: 10px;
      align-items: center;
      margin-bottom: 10px;
    }
    .bucket-label, .bucket-count {
      color: #556579;
      font-size: 13px;
    }
    .bar-track {
      width: 100%;
      height: 16px;
      border-radius: 999px;
      background: #e8eef7;
      overflow: hidden;
    }
    .bar-fill {
      height: 100%;
      border-radius: 999px;
      background: linear-gradient(90deg, #1f6feb, #4aa3ff);
    }
    .footer {
      margin-top: 18px;
      padding-top: 10px;
      border-top: 1px solid #dbe5f2;
      color: #6a7685;
      font-size: 12px;
    }
  </style>
</head>
<body>
  <main class="report">
    <header class="header">
      <h1 class="title">任务分析报告</h1>
      <div class="meta">任务名称：{{ .TaskName }} ｜ 生成时间：{{ .GeneratedAt }}</div>
    </header>

    <section class="section">
      <h2>任务概览</h2>
      <div class="overview-grid">
        <div class="metric-card">
          <div class="metric-label">任务满分</div>
          <div class="metric-value">{{ .RubricTotalScore }}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">视频总数</div>
          <div class="metric-value">{{ .TotalVideos }}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">已完成视频数</div>
          <div class="metric-value">{{ .CompletedVideos }}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">交付状态</div>
          <div class="metric-value">已完成</div>
        </div>
      </div>
    </section>

    <section class="section">
      <h2>成绩概览</h2>
      <table class="summary-table">
        <tbody>
          <tr><th>人工平均分</th><td>{{ .AverageManualScore }}</td><th>AI 平均分</th><td>{{ .AverageAIScore }}</td></tr>
          <tr><th>人工最高分</th><td>{{ .HighestManualScore }}</td><th>AI 最高分</th><td>{{ .HighestAIScore }}</td></tr>
          <tr><th>人工最低分</th><td>{{ .LowestManualScore }}</td><th>AI 最低分</th><td>{{ .LowestAIScore }}</td></tr>
        </tbody>
      </table>
    </section>

    <section class="section">
      <h2>图表区</h2>
      <div class="chart-grid">
        <section class="chart-card">
          <h3 class="chart-title">人工成绩分布（按满分归一化）</h3>
          {{ range .ManualScoreDistribution }}
          <div class="bucket">
            <div class="bucket-label">{{ .Label }}</div>
            <div class="bar-track"><div class="bar-fill" style="width: {{ .Width }}%;"></div></div>
            <div class="bucket-count">{{ .Count }}</div>
          </div>
          {{ end }}
        </section>

        <section class="chart-card">
          <h3 class="chart-title">AI 成绩分布（按满分归一化）</h3>
          {{ range .AIScoreDistribution }}
          <div class="bucket">
            <div class="bucket-label">{{ .Label }}</div>
            <div class="bar-track"><div class="bar-fill" style="width: {{ .Width }}%;"></div></div>
            <div class="bucket-count">{{ .Count }}</div>
          </div>
          {{ end }}
        </section>

        <section class="chart-card">
          <h3 class="chart-title">AI 与人工分差分布（按满分归一化）</h3>
          {{ range .ScoreGapDistribution }}
          <div class="bucket">
            <div class="bucket-label">{{ .Label }}</div>
            <div class="bar-track"><div class="bar-fill" style="width: {{ .Width }}%;"></div></div>
            <div class="bucket-count">{{ .Count }}</div>
          </div>
          {{ end }}
        </section>
      </div>
    </section>

    <footer class="footer">
      当前报告基于任务最终完成后的 AI 与人工评分聚合结果生成。
    </footer>
  </main>
</body>
</html>`

	tmpl, err := template.New("task-analysis-report").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("parse report template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		return "", fmt.Errorf("execute report template: %w", err)
	}
	return buf.String(), nil
}

func toAnalysisReportView(result AnalysisResult) analysisReportView {
	return analysisReportView{
		GeneratedAt:             time.Now().Format("2006-01-02 15:04:05"),
		TaskName:                result.Task.Name,
		RubricTotalScore:        result.Task.RubricTotalScore,
		TotalVideos:             result.Task.TotalVideos,
		CompletedVideos:         result.Task.CompletedVideos,
		AverageManualScore:      formatScoreDisplay(result.ScoreSummary.AverageManualScore, result.Task.RubricTotalScore),
		AverageAIScore:          formatScoreDisplay(result.ScoreSummary.AverageAIScore, result.Task.RubricTotalScore),
		HighestManualScore:      formatScoreDisplay(result.ScoreSummary.HighestManualScore, result.Task.RubricTotalScore),
		LowestManualScore:       formatScoreDisplay(result.ScoreSummary.LowestManualScore, result.Task.RubricTotalScore),
		HighestAIScore:          formatScoreDisplay(result.ScoreSummary.HighestAIScore, result.Task.RubricTotalScore),
		LowestAIScore:           formatScoreDisplay(result.ScoreSummary.LowestAIScore, result.Task.RubricTotalScore),
		ManualScoreDistribution: toDistributionView(result.ManualScoreDistribution),
		AIScoreDistribution:     toDistributionView(result.AIScoreDistribution),
		ScoreGapDistribution:    toDistributionView(result.ScoreGapDistribution),
	}
}

func toDistributionView(items []AnalysisDistributionBucketDTO) []analysisDistributionView {
	maxCount := 0
	for _, item := range items {
		if item.Count > maxCount {
			maxCount = item.Count
		}
	}

	result := make([]analysisDistributionView, 0, len(items))
	for _, item := range items {
		width := 0
		if maxCount > 0 {
			width = int(float64(item.Count) / float64(maxCount) * 100)
			if item.Count > 0 && width == 0 {
				width = 1
			}
		}
		result = append(result, analysisDistributionView{
			Label: item.Label,
			Count: item.Count,
			Width: width,
		})
	}
	return result
}

func formatScoreDisplay(value *float64, totalScore int) string {
	if value == nil {
		return "-"
	}
	if float64(int(*value)) == *value {
		return fmt.Sprintf("%d / %d", int(*value), totalScore)
	}
	return fmt.Sprintf("%.2f / %d", *value, totalScore)
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
