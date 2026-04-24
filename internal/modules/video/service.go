package video

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/ai"
	"skilljudge/backend/internal/modules/project"
	"skilljudge/backend/internal/modules/scoring"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

type Service struct {
	repo        *Repository
	aiService   *ai.Service
	projectRepo *project.Repository
	taskService *task.Service
	users       *user.Repository
	storage     storage.Provider
}

type CreateUploadCredentialInput struct {
	TaskID        uuid.UUID  `json:"taskId"`
	Filename      string     `json:"filename"`
	FileSize      int64      `json:"fileSize"`
	StudentID     *uuid.UUID `json:"studentId"`
	StudentName   string     `json:"studentName"`
	StudentNumber string     `json:"studentNumber"`
}

type ConfirmUploadInput struct {
	UploadID string                 `json:"uploadId"`
	Parts    []storage.UploadedPart `json:"parts"`
}

var filenameStudentIDPattern = regexp.MustCompile(`^(.*)_([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`)

func NewService(repo *Repository, aiService *ai.Service, projectRepo *project.Repository, taskService *task.Service, users *user.Repository, provider storage.Provider) *Service {
	return &Service{
		repo:        repo,
		aiService:   aiService,
		projectRepo: projectRepo,
		taskService: taskService,
		users:       users,
		storage:     provider,
	}
}

func (s *Service) CreateUploadCredential(ctx context.Context, actor user.UserContext, input CreateUploadCredentialInput) (*UploadCredentialDTO, error) {
	if err := validateCreateUploadInput(input); err != nil {
		return nil, err
	}
	if !canCreateVideo(actor.Role) {
		return nil, ErrRoleNotAllowed
	}

	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, input.TaskID)
	if err != nil {
		return nil, err
	}

	resolvedStudentID, resolvedStudentName, resolvedStudentNumber := resolveStudentIdentity(input)

	videoID := uuid.New()
	// Video storage now follows the project -> task -> video hierarchy so later
	// scoring and cleanup logic can recover the full business context from task_id.
	objectKey := buildObjectKey(resolvedTask.ProjectID, resolvedTask.TaskID, videoID, input.Filename)
	objectPrefix := buildObjectPrefix(resolvedTask.ProjectID, resolvedTask.TaskID, videoID)
	session, err := s.storage.CreateMultipartUpload(ctx, storage.CreateMultipartUploadInput{
		ObjectKey:    objectKey,
		ObjectPrefix: objectPrefix,
		Filename:     input.Filename,
		FileSize:     input.FileSize,
	})
	if err != nil {
		return nil, fmt.Errorf("create multipart upload: %w", err)
	}

	item := &model.Video{
		ID:               videoID,
		ProjectID:        resolvedTask.ProjectID,
		TaskID:           &resolvedTask.TaskID,
		SchoolID:         resolvedTask.SchoolID,
		StudentID:        resolvedStudentID,
		StudentName:      resolvedStudentName,
		StudentNumber:    resolvedStudentNumber,
		Filename:         sanitizeFilename(input.Filename),
		OriginalFilename: stringPtr(strings.TrimSpace(input.Filename)),
		FileSize:         input.FileSize,
		Format:           detectFormat(input.Filename),
		StoragePath:      stringPtr(session.ObjectKey),
		UploadID:         stringPtr(session.UploadID),
		Status:           "uploading",
		UploadProgress:   0,
		// Scoring summary fields live on videos so task lists can render current
		// status without joining the full manual evaluation payload every time.
		EvaluationStatus: scoring.VideoEvaluationStatusPending,
		AIStatus:         "pending",
		ManualStatus:     scoring.VideoManualStatusPending,
		CreatorID:        &actor.UserID,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}

	return &UploadCredentialDTO{
		VideoID:    item.ID,
		UploadURL:  session.UploadURL,
		UploadID:   session.UploadID,
		Credential: session.Credential,
	}, nil
}

func (s *Service) ConfirmUpload(ctx context.Context, actor user.UserContext, videoID uuid.UUID, input ConfirmUploadInput) (*VideoDetailDTO, error) {
	if strings.TrimSpace(input.UploadID) == "" {
		return nil, ErrVideoUploadIDRequired
	}
	if len(input.Parts) == 0 {
		return nil, ErrVideoPartsRequired
	}

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
	if item.StoragePath == nil {
		return nil, ErrVideoNotFound
	}

	result, err := s.storage.CompleteMultipartUpload(ctx, storage.CompleteMultipartUploadInput{
		ObjectKey: *item.StoragePath,
		UploadID:  input.UploadID,
		Parts:     input.Parts,
	})
	if err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	now := time.Now()
	updates := map[string]any{
		"storage_url":     result.StorageURL,
		"storage_path":    result.StoragePath,
		"upload_id":       input.UploadID,
		"uploaded_at":     now,
		"upload_progress": 100,
		"status":          "ready",
		"updated_at":      now,
	}
	// Phase 1 stops at a successfully uploaded "ready" asset. Transcoding and
	// playback optimization are deliberately deferred to later phases.
	if err := s.repo.Update(ctx, item.ID, updates); err != nil {
		return nil, err
	}
	if item.TaskID != nil {
		if err := s.taskService.RefreshVideoStats(ctx, *item.TaskID); err != nil {
			return nil, fmt.Errorf("refresh task video stats: %w", err)
		}
	}

	updated, err := s.repo.FindByID(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	if s.aiService != nil && s.aiService.Configured() {
		if _, err := s.aiService.CreateForVideo(ctx, actor, updated.ID, false); err != nil &&
			!errors.Is(err, ai.ErrEvaluationAlreadyProcessing) &&
			!errors.Is(err, ai.ErrEvaluationForceRequired) {
			log.Printf("video.ConfirmUpload auto ai trigger failed: actor=%s video=%s err=%v", actor.UserID, updated.ID, err)
		}
		refreshed, refreshErr := s.repo.FindByID(ctx, item.ID)
		if refreshErr != nil {
			return nil, refreshErr
		}
		if refreshed != nil {
			updated = refreshed
		}
	}

	playURL, err := s.playURL(ctx, updated)
	if err != nil {
		return nil, err
	}

	manual, err := s.repo.FindLatestManualEvaluation(ctx, updated.ID)
	if err != nil {
		return nil, err
	}

	aiEvaluation, err := s.latestAIEvaluation(ctx, updated.ID)
	if err != nil {
		return nil, err
	}

	return ToVideoDetailDTO(updated, playURL, manual, aiEvaluation), nil
}

func (s *Service) List(ctx context.Context, actor user.UserContext, params ListParams) (*ListVideosResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.TaskID == nil || *params.TaskID == uuid.Nil {
		return nil, ErrVideoTaskRequired
	}

	if _, err := s.taskService.ResolveReadable(ctx, actor, *params.TaskID); err != nil {
		return nil, err
	}

	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	result := make([]VideoListItemDTO, 0, len(items))
	for i := range items {
		result = append(result, ToVideoListItemDTO(&items[i]))
	}

	return &ListVideosResult{
		Items: result,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) ListMine(ctx context.Context, actor user.UserContext, params ListParams) (*ListVideosResult, error) {
	if actor.Role != "student" {
		return nil, ErrRoleNotAllowed
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	currentUser, err := s.users.FindByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if currentUser == nil {
		return nil, user.ErrUserNotFound
	}

	params.StudentOwnerID = &actor.UserID
	if currentUser.InternalNumber != nil && strings.TrimSpace(*currentUser.InternalNumber) != "" {
		params.StudentOwnerNumber = strings.TrimSpace(*currentUser.InternalNumber)
	}
	params.SchoolID = actor.SchoolID
	params.TaskID = nil
	params.ProjectID = uuid.Nil

	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	result := make([]VideoListItemDTO, 0, len(items))
	for i := range items {
		result = append(result, ToVideoListItemDTO(&items[i]))
	}

	return &ListVideosResult{
		Items: result,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) GetByID(ctx context.Context, actor user.UserContext, videoID uuid.UUID) (*VideoDetailDTO, error) {
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

	playURL, err := s.playURL(ctx, item)
	if err != nil {
		return nil, err
	}

	manual, err := s.repo.FindLatestManualEvaluation(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	aiEvaluation, err := s.latestAIEvaluation(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	return ToVideoDetailDTO(item, playURL, manual, aiEvaluation), nil
}

func (s *Service) GetMineByID(ctx context.Context, actor user.UserContext, videoID uuid.UUID) (*VideoDetailDTO, error) {
	if actor.Role != "student" {
		return nil, ErrRoleNotAllowed
	}

	item, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrVideoNotFound
	}
	currentUser, err := s.users.FindByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if currentUser == nil {
		return nil, user.ErrUserNotFound
	}
	if !studentCanAccessVideo(actor, currentUser, item) {
		return nil, ErrInvalidVideoScope
	}

	playURL, err := s.playURL(ctx, item)
	if err != nil {
		return nil, err
	}

	manual, err := s.repo.FindLatestManualEvaluation(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	aiEvaluation, err := s.latestAIEvaluation(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	return ToVideoDetailDTO(item, playURL, manual, aiEvaluation), nil
}

func studentCanAccessVideo(actor user.UserContext, currentUser *model.User, item *model.Video) bool {
	if item.StudentID != nil && *item.StudentID == actor.UserID {
		return true
	}
	if currentUser == nil || currentUser.InternalNumber == nil {
		return false
	}
	internalNumber := strings.TrimSpace(*currentUser.InternalNumber)
	if internalNumber == "" || strings.TrimSpace(item.StudentNumber) != internalNumber {
		return false
	}
	videoSchool := videoSchoolID(item)
	return actor.SchoolID != nil && videoSchool != nil && *actor.SchoolID == *videoSchool
}

func (s *Service) Delete(ctx context.Context, actor user.UserContext, videoID uuid.UUID) error {
	item, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrVideoNotFound
	}
	if err := ensureActorCanManageVideo(actor, item); err != nil {
		return err
	}

	if item.StoragePath != nil && *item.StoragePath != "" {
		if err := s.storage.DeleteObject(ctx, *item.StoragePath); err != nil {
			return fmt.Errorf("delete storage object: %w", err)
		}
	}

	return s.repo.Delete(ctx, item.ID)
}

func (s *Service) playURL(ctx context.Context, item *model.Video) (*string, error) {
	if item.StoragePath == nil || *item.StoragePath == "" {
		return nil, nil
	}

	playURL, err := s.storage.GeneratePlayURL(ctx, *item.StoragePath, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("generate play url: %w", err)
	}
	return &playURL, nil
}

func validateCreateUploadInput(input CreateUploadCredentialInput) error {
	if input.TaskID == uuid.Nil {
		return ErrVideoTaskRequired
	}
	if strings.TrimSpace(input.Filename) == "" {
		return ErrVideoFilenameRequired
	}
	if input.FileSize <= 0 {
		return ErrVideoFileSizeInvalid
	}
	if strings.TrimSpace(input.StudentName) == "" {
		return ErrVideoStudentNameRequired
	}
	if strings.TrimSpace(input.StudentNumber) == "" {
		return ErrVideoStudentNumberRequired
	}
	return nil
}

func buildObjectKey(projectID, taskID, videoID uuid.UUID, filename string) string {
	return buildObjectPrefix(projectID, taskID, videoID) + sanitizeFilename(filename)
}

func buildObjectPrefix(projectID, taskID, videoID uuid.UUID) string {
	return fmt.Sprintf("projects/%s/tasks/%s/videos/%s/", projectID.String(), taskID.String(), videoID.String())
}

func sanitizeFilename(filename string) string {
	cleanName := strings.TrimSpace(filename)
	cleanName = strings.ReplaceAll(cleanName, " ", "_")
	return filepath.Base(cleanName)
}

func detectFormat(filename string) *string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(filename))), ".")
	if ext == "" {
		return nil
	}
	return &ext
}

func canCreateVideo(role string) bool {
	switch role {
	case "admin", "school_admin", "school_leader", "teacher":
		return true
	default:
		return false
	}
}

func resolveStudentIdentity(input CreateUploadCredentialInput) (*uuid.UUID, string, string) {
	studentID := input.StudentID
	studentName := strings.TrimSpace(input.StudentName)
	studentNumber := strings.TrimSpace(input.StudentNumber)

	filenameTitle := strings.TrimSpace(strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename)))
	if studentID == nil {
		if parsedName, parsedID, ok := parseStudentIdentityFromTitle(filenameTitle); ok {
			studentID = &parsedID
			if studentName == "" || studentName == filenameTitle {
				studentName = parsedName
			}
		}
	}
	if parsedName, parsedNumber, ok := parseStudentNameNumberFromTitle(filenameTitle); ok {
		if studentName == "" || studentName == filenameTitle {
			studentName = parsedName
		}
		if studentNumber == "" || studentNumber == filenameTitle || studentNumber == studentName || isTemporaryStudentNumber(studentNumber) {
			studentNumber = parsedNumber
		}
	}

	if studentName == "" {
		studentName = filenameTitle
	}
	if studentNumber == "" {
		studentNumber = studentName
	}

	return studentID, studentName, studentNumber
}

func parseStudentIdentityFromTitle(title string) (string, uuid.UUID, bool) {
	matches := filenameStudentIDPattern.FindStringSubmatch(strings.TrimSpace(title))
	if len(matches) != 3 {
		return "", uuid.Nil, false
	}

	studentID, err := uuid.Parse(matches[2])
	if err != nil {
		return "", uuid.Nil, false
	}

	studentName := strings.TrimSpace(matches[1])
	if studentName == "" {
		return "", uuid.Nil, false
	}

	return studentName, studentID, true
}

func parseStudentNameNumberFromTitle(title string) (string, string, bool) {
	trimmed := strings.TrimSpace(title)
	matches := filenameStudentIDPattern.FindStringSubmatch(trimmed)
	if len(matches) == 3 {
		return "", "", false
	}

	separatorIndex := strings.LastIndex(trimmed, "_")
	if separatorIndex <= 0 || separatorIndex >= len(trimmed)-1 {
		return "", "", false
	}

	studentName := strings.TrimSpace(trimmed[:separatorIndex])
	studentNumber := strings.TrimSpace(trimmed[separatorIndex+1:])
	if studentName == "" || studentNumber == "" {
		return "", "", false
	}

	return studentName, studentNumber, true
}

func isTemporaryStudentNumber(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "tmp-")
}

func (s *Service) latestAIEvaluation(ctx context.Context, videoID uuid.UUID) (*ai.EmbeddedEvaluationDTO, error) {
	if s.aiService == nil {
		return nil, nil
	}
	return s.aiService.GetLatestForVideo(ctx, videoID)
}

func ensureActorCanManageProject(actor user.UserContext, item *model.Project) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader":
		if actor.SchoolID == nil || item.SchoolID == nil || *actor.SchoolID != *item.SchoolID {
			return ErrInvalidVideoScope
		}
		return nil
	case "teacher":
		if item.CreatorID == nil || *item.CreatorID != actor.UserID {
			return ErrInvalidVideoScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func ensureActorCanAccessProject(actor user.UserContext, item *model.Project) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader", "scorer":
		if actor.SchoolID == nil || item.SchoolID == nil || *actor.SchoolID != *item.SchoolID {
			return ErrInvalidVideoScope
		}
		return nil
	case "teacher":
		if item.CreatorID == nil || *item.CreatorID != actor.UserID {
			return ErrInvalidVideoScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func ensureActorCanAccessVideo(actor user.UserContext, item *model.Video) error {
	derivedSchoolID := videoSchoolID(item)
	derivedCreatorID := videoCreatorID(item)
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader", "scorer":
		if actor.SchoolID == nil || derivedSchoolID == nil || *actor.SchoolID != *derivedSchoolID {
			return ErrInvalidVideoScope
		}
		return nil
	case "teacher":
		if derivedCreatorID == nil || *derivedCreatorID != actor.UserID {
			return ErrInvalidVideoScope
		}
		return nil
	case "student":
		if item.StudentID == nil || *item.StudentID != actor.UserID {
			return ErrInvalidVideoScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func ensureActorCanManageVideo(actor user.UserContext, item *model.Video) error {
	derivedSchoolID := videoSchoolID(item)
	derivedCreatorID := videoCreatorID(item)
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader":
		if actor.SchoolID == nil || derivedSchoolID == nil || *actor.SchoolID != *derivedSchoolID {
			return ErrInvalidVideoScope
		}
		return nil
	case "teacher":
		if derivedCreatorID == nil || *derivedCreatorID != actor.UserID {
			return ErrInvalidVideoScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func stringPtr(value string) *string {
	return &value
}

func videoSchoolID(item *model.Video) *uuid.UUID {
	if item.SchoolID != nil {
		return item.SchoolID
	}
	if item.Task != nil && item.Task.Project != nil {
		return item.Task.Project.SchoolID
	}
	if item.Project != nil {
		return item.Project.SchoolID
	}
	return nil
}

func videoCreatorID(item *model.Video) *uuid.UUID {
	if item.CreatorID != nil {
		return item.CreatorID
	}
	if item.Task != nil && item.Task.CreatorID != nil {
		return item.Task.CreatorID
	}
	if item.Project != nil && item.Project.CreatorID != nil {
		return item.Project.CreatorID
	}
	return nil
}
