package task

import (
	"context"
	"math"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/project"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type Service struct {
	repo        *Repository
	projectRepo *project.Repository
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

func NewService(repo *Repository, projectRepo *project.Repository) *Service {
	return &Service{repo: repo, projectRepo: projectRepo}
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

	result := make([]TaskDTO, 0, len(items))
	for i := range items {
		result = append(result, *ToTaskDTO(&items[i]))
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

	return ToTaskDTO(resolved.Item), nil
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

func canManageTask(role string) bool {
	switch role {
	case "admin", "school_admin", "teacher":
		return true
	default:
		return false
	}
}

func ensureActorCanManageProject(actor user.UserContext, item *model.Project) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin":
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
	case "school_admin", "scorer":
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
