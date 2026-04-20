package video

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/ai"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

const videoAIReportMetadataKey = "aiReport"

func (s *Service) GetAIReport(ctx context.Context, actor user.UserContext, videoID uuid.UUID) (*VideoAIReportDTO, error) {
	item, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrVideoNotFound
	}
	if err := ensureActorCanAccessVideo(actor, item); err != nil {
		return nil, err
	}

	meta := readVideoAIReportMetadata(item)
	if meta == nil {
		return nil, ErrVideoAIReportNotFound
	}
	return toVideoAIReportDTO(meta), nil
}

func (s *Service) GenerateAIReport(ctx context.Context, actor user.UserContext, videoID uuid.UUID) (*VideoAIReportDTO, error) {
	item, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrVideoNotFound
	}
	if err := ensureActorCanManageVideo(actor, item); err != nil {
		return nil, err
	}

	reportContext, err := s.buildVideoReportContext(ctx, item)
	if err != nil {
		failMeta := buildFailedVideoAIReportMetadata(err.Error())
		if updateErr := s.repo.Update(ctx, item.ID, map[string]any{
			"metadata":   mergeVideoMetadata(item.Metadata, failMeta),
			"updated_at": time.Now(),
		}); updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}

	current := readVideoAIReportMetadata(item)
	currentEvalID := strings.TrimSpace(reportContext.AIEvaluation.EvaluationID.String())
	if current != nil &&
		current.ReportStatus == videoAIReportStatusReady &&
		current.EvaluationID != nil &&
		strings.TrimSpace(*current.EvaluationID) == currentEvalID {
		return toVideoAIReportDTO(current), nil
	}

	htmlDocument, err := buildVideoAIReportHTML(reportContext)
	if err != nil {
		return nil, err
	}
	pdfBytes, err := buildVideoAIReportPDF(ctx, htmlDocument)
	if err != nil {
		return nil, err
	}

	fileBaseName := buildVideoAIReportFileBase(item)
	htmlFileName := sanitizeFilename(fileBaseName + ".html")
	pdfFileName := sanitizeFilename(fileBaseName + ".pdf")
	objectPrefix := buildObjectPrefix(reportContext.ProjectID, reportContext.TaskID, item.ID) + "ai-reports/"

	htmlPut, err := s.storage.PutObject(ctx, storage.PutObjectInput{
		ObjectKey:   objectPrefix + htmlFileName,
		ContentType: "text/html; charset=utf-8",
		Body:        []byte(htmlDocument),
	})
	if err != nil {
		return nil, fmt.Errorf("upload video ai report html: %w", err)
	}
	pdfPut, err := s.storage.PutObject(ctx, storage.PutObjectInput{
		ObjectKey:   objectPrefix + pdfFileName,
		ContentType: "application/pdf",
		Body:        pdfBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("upload video ai report pdf: %w", err)
	}

	now := time.Now()
	meta := &videoAIReportMetadata{
		ReportStatus:    videoAIReportStatusReady,
		ReportType:      videoAIReportFormatPDF,
		FileName:        stringPtr(pdfFileName),
		HTMLFileName:    stringPtr(htmlFileName),
		PDFStoragePath:  &pdfPut.StoragePath,
		PDFPublicURL:    &pdfPut.StorageURL,
		HTMLStoragePath: &htmlPut.StoragePath,
		HTMLPublicURL:   &htmlPut.StorageURL,
		TemplateVersion: stringPtr(videoAIReportTemplateVersion),
		GeneratedAt:     &now,
		EvaluationID:    stringPtr(currentEvalID),
	}
	if err := s.repo.Update(ctx, item.ID, map[string]any{
		"metadata":   mergeVideoMetadata(item.Metadata, meta),
		"updated_at": now,
	}); err != nil {
		return nil, err
	}
	return toVideoAIReportDTO(meta), nil
}

func (s *Service) RenderAIReportHTML(ctx context.Context, actor user.UserContext, videoID uuid.UUID) (string, error) {
	item, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", ErrVideoNotFound
	}
	if err := ensureActorCanAccessVideo(actor, item); err != nil {
		return "", err
	}
	reportContext, err := s.buildVideoReportContext(ctx, item)
	if err != nil {
		return "", err
	}
	currentEvalID := strings.TrimSpace(reportContext.AIEvaluation.EvaluationID.String())
	if htmlDocument, ok, err := s.loadStoredAIReportHTML(ctx, item, currentEvalID); err == nil && ok {
		return htmlDocument, nil
	} else if err != nil {
		return "", err
	}

	meta, err := s.GenerateAIReport(ctx, actor, videoID)
	if err != nil {
		return "", err
	}
	if htmlDocument, err := s.readStoredAIReportHTML(ctx, meta); err == nil {
		return htmlDocument, nil
	}
	return buildVideoAIReportHTML(reportContext)
}

func (s *Service) RenderAIReportPDF(ctx context.Context, actor user.UserContext, videoID uuid.UUID) ([]byte, string, error) {
	item, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return nil, "", err
	}
	if item == nil {
		return nil, "", ErrVideoNotFound
	}
	if err := ensureActorCanAccessVideo(actor, item); err != nil {
		return nil, "", err
	}
	reportContext, err := s.buildVideoReportContext(ctx, item)
	if err != nil {
		return nil, "", err
	}
	currentEvalID := strings.TrimSpace(reportContext.AIEvaluation.EvaluationID.String())
	if pdfBytes, fileName, ok, err := s.loadStoredAIReportPDF(ctx, item, currentEvalID); err == nil && ok {
		return pdfBytes, fileName, nil
	} else if err != nil {
		return nil, "", err
	}

	meta, err := s.GenerateAIReport(ctx, actor, videoID)
	if err != nil {
		return nil, "", err
	}
	if pdfBytes, fileName, err := s.readStoredAIReportPDF(ctx, meta, sanitizeFilename(buildVideoAIReportFileBase(item)+".pdf")); err == nil {
		return pdfBytes, fileName, nil
	}

	htmlDocument, err := buildVideoAIReportHTML(reportContext)
	if err != nil {
		return nil, "", err
	}
	pdfBytes, err := buildVideoAIReportPDF(ctx, htmlDocument)
	if err != nil {
		return nil, "", err
	}
	return pdfBytes, sanitizeFilename(buildVideoAIReportFileBase(item) + ".pdf"), nil
}

func (s *Service) loadStoredAIReportHTML(ctx context.Context, item *model.Video, evaluationID string) (string, bool, error) {
	meta := readVideoAIReportMetadata(item)
	if !canReuseStoredVideoAIReport(meta, evaluationID) {
		return "", false, nil
	}
	htmlDocument, err := s.readStoredAIReportHTML(ctx, toVideoAIReportDTO(meta))
	if err != nil {
		return "", false, nil
	}
	return htmlDocument, true, nil
}

func (s *Service) loadStoredAIReportPDF(ctx context.Context, item *model.Video, evaluationID string) ([]byte, string, bool, error) {
	meta := readVideoAIReportMetadata(item)
	if !canReuseStoredVideoAIReport(meta, evaluationID) {
		return nil, "", false, nil
	}
	defaultFileName := sanitizeFilename(buildVideoAIReportFileBase(item) + ".pdf")
	pdfBytes, fileName, err := s.readStoredAIReportPDF(ctx, toVideoAIReportDTO(meta), defaultFileName)
	if err != nil {
		return nil, "", false, nil
	}
	return pdfBytes, fileName, true, nil
}

func (s *Service) readStoredAIReportHTML(ctx context.Context, meta *VideoAIReportDTO) (string, error) {
	if meta == nil || meta.HTMLStoragePath == nil || strings.TrimSpace(*meta.HTMLStoragePath) == "" {
		return "", ErrVideoAIReportNotFound
	}
	body, err := s.storage.GetObject(ctx, strings.TrimSpace(*meta.HTMLStoragePath))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *Service) readStoredAIReportPDF(ctx context.Context, meta *VideoAIReportDTO, fallbackFileName string) ([]byte, string, error) {
	if meta == nil || meta.PDFStoragePath == nil || strings.TrimSpace(*meta.PDFStoragePath) == "" {
		return nil, "", ErrVideoAIReportNotFound
	}
	body, err := s.storage.GetObject(ctx, strings.TrimSpace(*meta.PDFStoragePath))
	if err != nil {
		return nil, "", err
	}
	fileName := fallbackFileName
	if meta.FileName != nil && strings.TrimSpace(*meta.FileName) != "" {
		fileName = strings.TrimSpace(*meta.FileName)
	}
	return body, fileName, nil
}

func canReuseStoredVideoAIReport(meta *videoAIReportMetadata, evaluationID string) bool {
	return meta != nil &&
		meta.ReportStatus == videoAIReportStatusReady &&
		meta.EvaluationID != nil &&
		strings.TrimSpace(*meta.EvaluationID) != "" &&
		strings.TrimSpace(*meta.EvaluationID) == strings.TrimSpace(evaluationID)
}

func (s *Service) buildVideoReportContext(ctx context.Context, item *model.Video) (*videoReportContext, error) {
	if item.TaskID == nil || *item.TaskID == uuid.Nil || item.Task == nil {
		return nil, ErrVideoAIReportNotReady
	}
	aiEvaluation, err := s.latestAIEvaluation(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	if aiEvaluation == nil || aiEvaluation.Status != ai.EvaluationStatusCompleted {
		return nil, ErrVideoAIReportNotReady
	}
	return &videoReportContext{
		ProjectID:    item.ProjectID,
		TaskID:       *item.TaskID,
		AIEvaluation: aiEvaluation,
		RawVideo: &modelVideoBridge{
			TaskName:        strings.TrimSpace(item.Task.Name),
			RubricName:      videoRubricName(item),
			StudentName:     strings.TrimSpace(item.StudentName),
			StudentNumber:   strings.TrimSpace(item.StudentNumber),
			Filename:        strings.TrimSpace(derefStringOr(item.OriginalFilename, item.Filename)),
			DurationSeconds: item.Duration,
		},
	}, nil
}

func videoRubricName(item *model.Video) string {
	if item.Task != nil && item.Task.Rubric != nil {
		return strings.TrimSpace(item.Task.Rubric.Name)
	}
	return "--"
}

func buildVideoAIReportFileBase(item *model.Video) string {
	base := strings.TrimSpace(item.StudentName)
	if base == "" {
		base = strings.TrimSpace(derefStringOr(item.OriginalFilename, item.Filename))
	}
	if base == "" {
		base = "video-ai-report"
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSpace(base)
	if base == "" {
		base = "video-ai-report"
	}
	return buildSafeReportName(base) + "-ai-report"
}

func buildSafeReportName(value string) string {
	return strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	).Replace(value)
}

func mergeVideoMetadata(existing map[string]any, report *videoAIReportMetadata) map[string]any {
	next := make(map[string]any, len(existing)+1)
	for key, value := range existing {
		next[key] = value
	}
	next[videoAIReportMetadataKey] = map[string]any{
		"reportStatus":    report.ReportStatus,
		"reportType":      report.ReportType,
		"fileName":        report.FileName,
		"htmlFileName":    report.HTMLFileName,
		"pdfStoragePath":  report.PDFStoragePath,
		"pdfPublicUrl":    report.PDFPublicURL,
		"htmlStoragePath": report.HTMLStoragePath,
		"htmlPublicUrl":   report.HTMLPublicURL,
		"templateVersion": report.TemplateVersion,
		"generatedAt":     report.GeneratedAt,
		"errorMessage":    report.ErrorMessage,
		"evaluationId":    report.EvaluationID,
	}
	return next
}

func readVideoAIReportMetadata(item *model.Video) *videoAIReportMetadata {
	if item == nil || len(item.Metadata) == 0 {
		return nil
	}
	raw, ok := item.Metadata[videoAIReportMetadataKey]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	meta := &videoAIReportMetadata{
		ReportStatus:    readStringFromMap(m, "reportStatus"),
		ReportType:      readStringFromMap(m, "reportType"),
		FileName:        readOptionalStringFromMap(m, "fileName"),
		HTMLFileName:    readOptionalStringFromMap(m, "htmlFileName"),
		PDFStoragePath:  readOptionalStringFromMap(m, "pdfStoragePath"),
		PDFPublicURL:    readOptionalStringFromMap(m, "pdfPublicUrl"),
		HTMLStoragePath: readOptionalStringFromMap(m, "htmlStoragePath"),
		HTMLPublicURL:   readOptionalStringFromMap(m, "htmlPublicUrl"),
		TemplateVersion: readOptionalStringFromMap(m, "templateVersion"),
		ErrorMessage:    readOptionalStringFromMap(m, "errorMessage"),
		EvaluationID:    readOptionalStringFromMap(m, "evaluationId"),
	}
	if value, ok := m["generatedAt"].(time.Time); ok {
		meta.GeneratedAt = &value
	}
	return meta
}

func toVideoAIReportDTO(meta *videoAIReportMetadata) *VideoAIReportDTO {
	if meta == nil {
		return nil
	}
	return &VideoAIReportDTO{
		ReportStatus:    meta.ReportStatus,
		ReportType:      meta.ReportType,
		FileName:        meta.FileName,
		HTMLFileName:    meta.HTMLFileName,
		PDFStoragePath:  meta.PDFStoragePath,
		PDFPublicURL:    meta.PDFPublicURL,
		HTMLStoragePath: meta.HTMLStoragePath,
		HTMLPublicURL:   meta.HTMLPublicURL,
		TemplateVersion: meta.TemplateVersion,
		GeneratedAt:     meta.GeneratedAt,
		ErrorMessage:    meta.ErrorMessage,
	}
}

func buildFailedVideoAIReportMetadata(message string) *videoAIReportMetadata {
	now := time.Now()
	return &videoAIReportMetadata{
		ReportStatus:    videoAIReportStatusFailed,
		ReportType:      videoAIReportFormatPDF,
		TemplateVersion: stringPtr(videoAIReportTemplateVersion),
		GeneratedAt:     &now,
		ErrorMessage:    stringPtr(strings.TrimSpace(message)),
	}
}

func readStringFromMap(m map[string]any, key string) string {
	if value, ok := m[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func readOptionalStringFromMap(m map[string]any, key string) *string {
	value := readStringFromMap(m, key)
	if value == "" {
		return nil
	}
	return &value
}

func derefStringOr(value *string, fallback string) string {
	if value != nil && strings.TrimSpace(*value) != "" {
		return strings.TrimSpace(*value)
	}
	return strings.TrimSpace(fallback)
}
