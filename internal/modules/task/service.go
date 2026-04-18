package task

import (
	"context"
	"math"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/project"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/platform/storage"

	"github.com/google/uuid"
)

type Service struct {
	repo        *Repository
	projectRepo *project.Repository
	storage     storage.Provider
}

type AccessLevel string

const (
	AccessRead   AccessLevel = "read"
	AccessManage AccessLevel = "manage"
)

type CreateTaskInput struct {
	ProjectID   uuid.UUID      `json:"projectId"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	RubricID    uuid.UUID      `json:"rubricId"`
	StartDate   *time.Time     `json:"startDate"`
	Deadline    *time.Time     `json:"deadline"`
	EndDate     *time.Time     `json:"endDate"`
	Metadata    map[string]any `json:"metadata"`
}

type ScoreboardParams struct {
	TaskID           uuid.UUID
	Page             int
	PageSize         int
	Keyword          string
	Scope            string
	EvaluationStatus string
	SortBy           string
	SortOrder        string
}

func NewService(repo *Repository, projectRepo *project.Repository, provider storage.Provider) *Service {
	return &Service{repo: repo, projectRepo: projectRepo, storage: provider}
}

func (s *Service) Create(ctx context.Context, actor user.UserContext, input CreateTaskInput) (*TaskDTO, error) {
	if input.ProjectID == uuid.Nil {
		return nil, project.ErrProjectNotFound
	}
	if input.Name == "" {
		return nil, ErrTaskNameRequired
	}
	if input.RubricID == uuid.Nil {
		return nil, ErrTaskRubricRequired
	}
	if !canManageTask(actor.Role) {
		return nil, ErrTaskRoleNotAllowed
	}

	projectItem, err := s.projectRepo.FindByID(ctx, input.ProjectID)
	if err != nil {
		return nil, err
	}
	if projectItem == nil {
		return nil, project.ErrProjectNotFound
	}
	if err := ensureActorCanManageProject(actor, projectItem); err != nil {
		return nil, err
	}

	rubric, err := s.projectRepo.FindRubricByID(ctx, input.RubricID)
	if err != nil {
		return nil, err
	}
	if rubric == nil {
		return nil, project.ErrRubricNotFound
	}

	item := &model.Task{
		ProjectID:   input.ProjectID,
		Name:        input.Name,
		Description: input.Description,
		RubricID:    input.RubricID,
		CreatorID:   &actor.UserID,
		Status:      "draft",
		StartDate:   input.StartDate,
		Deadline:    input.Deadline,
		EndDate:     input.EndDate,
		Metadata:    input.Metadata,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	return ToTaskDTO(created), nil
}

func (s *Service) List(ctx context.Context, actor user.UserContext, params ListParams) (*ListTasksResult, error) {
	if params.ProjectID == uuid.Nil {
		return nil, project.ErrProjectNotFound
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	projectItem, err := s.projectRepo.FindByID(ctx, params.ProjectID)
	if err != nil {
		return nil, err
	}
	if projectItem == nil {
		return nil, project.ErrProjectNotFound
	}
	if err := ensureActorCanAccessProject(actor, projectItem); err != nil {
		return nil, err
	}

	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	taskIDs := make([]uuid.UUID, 0, len(items))
	for i := range items {
		taskIDs = append(taskIDs, items[i].ID)
	}
	manualCompletedMap, err := s.repo.CountManualSubmittedByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}

	result := make([]TaskDTO, 0, len(items))
	for i := range items {
		dto := ToTaskDTO(&items[i])
		applyManualProgress(dto, items[i].TotalVideos, int(manualCompletedMap[items[i].ID]))
		result = append(result, *dto)
	}

	return &ListTasksResult{
		Items: result,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) GetByID(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*TaskDTO, error) {
	resolved, err := s.Resolve(ctx, actor, taskID, AccessRead)
	if err != nil {
		return nil, err
	}

	dto := ToTaskDTO(resolved.Item)
	manualCompletedMap, err := s.repo.CountManualSubmittedByTaskIDs(ctx, []uuid.UUID{resolved.Item.ID})
	if err != nil {
		return nil, err
	}
	applyManualProgress(dto, resolved.Item.TotalVideos, int(manualCompletedMap[resolved.Item.ID]))

	return dto, nil
}

func (s *Service) GetScoreboard(ctx context.Context, actor user.UserContext, params ScoreboardParams) (*ScoreboardResult, error) {
	resolved, err := s.Resolve(ctx, actor, params.TaskID, AccessRead)
	if err != nil {
		return nil, err
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.Scope == "" {
		params.Scope = "page"
	}
	if params.SortOrder == "" {
		params.SortOrder = "asc"
	}

	items, total, summary, err := s.repo.ListScoreboard(ctx, params)
	if err != nil {
		return nil, err
	}

	resultItems := make([]ScoreboardItemDTO, 0, len(items))
	for i := range items {
		resultItems = append(resultItems, ToScoreboardItemDTO(&items[i]))
	}

	page := params.Page
	pageSize := params.PageSize
	totalPages := 0
	if params.Scope == "all" {
		page = 1
		pageSize = int(total)
		if pageSize == 0 {
			pageSize = len(resultItems)
		}
		if pageSize == 0 {
			pageSize = 1
		}
		totalPages = 1
	} else {
		totalPages = int(math.Ceil(float64(total) / float64(params.PageSize)))
		if totalPages == 0 {
			totalPages = 1
		}
	}

	return &ScoreboardResult{
		Task: ToScoreboardTaskDTO(resolved.Item),
		Summary: ScoreboardSummaryDTO{
			TotalStudents:      summary.TotalStudents,
			CompletedStudents:  summary.CompletedStudents,
			AverageAIScore:     summary.AverageAIScore,
			AverageManualScore: summary.AverageManualScore,
		},
		Items: resultItems,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) Resolve(ctx context.Context, actor user.UserContext, taskID uuid.UUID, access AccessLevel) (*Context, error) {
	item, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Project == nil || item.Rubric == nil {
		return nil, ErrTaskNotFound
	}

	// Task is the stable bridge between project-level ownership and all
	// downstream video/scoring operations, so access checks are normalized here.
	switch access {
	case AccessManage:
		if err := ensureActorCanManageProject(actor, item.Project); err != nil {
			return nil, err
		}
	default:
		if err := ensureActorCanAccessProject(actor, item.Project); err != nil {
			return nil, err
		}
	}

	return ToContext(item), nil
}

func (s *Service) ResolveReadable(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*Context, error) {
	return s.Resolve(ctx, actor, taskID, AccessRead)
}

func (s *Service) ResolveManageable(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*Context, error) {
	return s.Resolve(ctx, actor, taskID, AccessManage)
}

func (s *Service) RefreshVideoStats(ctx context.Context, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return ErrTaskNotFound
	}

	return s.repo.RefreshVideoStats(ctx, taskID)
}

func canManageTask(role string) bool {
	switch role {
	case "admin", "school_admin", "school_leader", "teacher":
		return true
	default:
		return false
	}
}

func ensureActorCanManageProject(actor user.UserContext, item *model.Project) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader":
		if actor.SchoolID == nil || item.SchoolID == nil || *actor.SchoolID != *item.SchoolID {
			return ErrTaskProjectScope
		}
		return nil
	case "teacher":
		if item.CreatorID == nil || *item.CreatorID != actor.UserID {
			return ErrTaskProjectScope
		}
		return nil
	default:
		return ErrTaskRoleNotAllowed
	}
}

func ensureActorCanAccessProject(actor user.UserContext, item *model.Project) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader", "scorer":
		if actor.SchoolID == nil || item.SchoolID == nil || *actor.SchoolID != *item.SchoolID {
			return ErrTaskProjectScope
		}
		return nil
	case "teacher":
		if item.CreatorID == nil || *item.CreatorID != actor.UserID {
			return ErrTaskProjectScope
		}
		return nil
	default:
		return ErrTaskRoleNotAllowed
	}
}
