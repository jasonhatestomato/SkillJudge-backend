package project

import (
	"context"
	"math"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

type CreateProjectInput struct {
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	SchoolID       *uuid.UUID
	Deadline       *time.Time     `json:"deadline"`
	Tags           []string       `json:"tags"`
	ExperimentType *string        `json:"experimentType"`
	GradeLevel     *string        `json:"gradeLevel"`
	Subject        *string        `json:"subject"`
	Metadata       map[string]any `json:"metadata"`
}

type UpdateProjectInput struct {
	Name           *string        `json:"name"`
	Description    *string        `json:"description"`
	Status         *string        `json:"status"`
	Deadline       *time.Time     `json:"deadline"`
	Tags           []string       `json:"tags"`
	ExperimentType *string        `json:"experimentType"`
	GradeLevel     *string        `json:"gradeLevel"`
	Subject        *string        `json:"subject"`
	Metadata       map[string]any `json:"metadata"`
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, actor user.UserContext, input CreateProjectInput) (*ProjectDTO, error) {
	if input.Name == "" {
		return nil, ErrProjectNameRequired
	}
	if !canCreateProject(actor.Role) {
		return nil, ErrRoleNotAllowed
	}

	projectSchoolID := input.SchoolID
	switch actor.Role {
	case "teacher", "school_admin", "school_leader":
		if actor.SchoolID == nil {
			return nil, ErrProjectSchoolRequired
		}
		projectSchoolID = actor.SchoolID
	case "admin":
		if projectSchoolID == nil {
			return nil, ErrProjectSchoolRequired
		}
	}

	item := &model.Project{
		Name:           input.Name,
		Description:    input.Description,
		SchoolID:       projectSchoolID,
		CreatorID:      &actor.UserID,
		Status:         "draft",
		Deadline:       input.Deadline,
		Tags:           model.StringArray(input.Tags),
		ExperimentType: input.ExperimentType,
		GradeLevel:     input.GradeLevel,
		Subject:        input.Subject,
		Metadata:       input.Metadata,
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	return ToProjectDTO(created), nil
}

func (s *Service) GetByID(ctx context.Context, actor user.UserContext, id uuid.UUID) (*ProjectDTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrProjectNotFound
	}
	if err := ensureActorCanAccessProject(actor, item); err != nil {
		return nil, err
	}

	return ToProjectDTO(item), nil
}

func (s *Service) List(ctx context.Context, actor user.UserContext, params ListParams) (*ListProjectsResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	items, total, err := s.repo.List(ctx, params, actor.Role, actor.UserID, actor.SchoolID)
	if err != nil {
		return nil, err
	}

	result := make([]ProjectDTO, 0, len(items))
	for i := range items {
		result = append(result, *ToProjectDTO(&items[i]))
	}

	return &ListProjectsResult{
		Items: result,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) Update(ctx context.Context, actor user.UserContext, id uuid.UUID, input UpdateProjectInput) (*ProjectDTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrProjectNotFound
	}
	if err := ensureActorCanManageProject(actor, item); err != nil {
		return nil, err
	}

	updates := map[string]any{
		"updated_at": time.Now(),
	}
	hasChange := false

	if input.Name != nil {
		if *input.Name == "" {
			return nil, ErrProjectNameRequired
		}
		updates["name"] = *input.Name
		hasChange = true
	}
	if input.Description != nil {
		updates["description"] = *input.Description
		hasChange = true
	}
	if input.Status != nil {
		if !isValidStatus(*input.Status) {
			return nil, ErrInvalidProjectStatus
		}
		updates["status"] = *input.Status
		hasChange = true
	}
	if input.Deadline != nil {
		updates["deadline"] = *input.Deadline
		hasChange = true
	}
	if input.Tags != nil {
		updates["tags"] = model.StringArray(input.Tags)
		hasChange = true
	}
	if input.ExperimentType != nil {
		updates["experiment_type"] = *input.ExperimentType
		hasChange = true
	}
	if input.GradeLevel != nil {
		updates["grade_level"] = *input.GradeLevel
		hasChange = true
	}
	if input.Subject != nil {
		updates["subject"] = *input.Subject
		hasChange = true
	}
	if input.Metadata != nil {
		updates["metadata"] = input.Metadata
		hasChange = true
	}

	if !hasChange {
		return nil, ErrEmptyUpdatePayload
	}

	if err := s.repo.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return ToProjectDTO(updated), nil
}

func (s *Service) Delete(ctx context.Context, actor user.UserContext, id uuid.UUID) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrProjectNotFound
	}
	if err := ensureActorCanManageProject(actor, item); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id)
}

func canCreateProject(role string) bool {
	switch role {
	case "admin", "school_admin", "school_leader", "teacher":
		return true
	default:
		return false
	}
}

func isValidStatus(status string) bool {
	switch status {
	case "draft", "in_progress", "completed", "archived":
		return true
	default:
		return false
	}
}

func ensureActorCanAccessProject(actor user.UserContext, item *model.Project) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader":
		if actor.SchoolID == nil || item.SchoolID == nil || *actor.SchoolID != *item.SchoolID {
			return ErrInvalidProjectScope
		}
		return nil
	case "teacher":
		if item.CreatorID == nil || *item.CreatorID != actor.UserID {
			return ErrInvalidProjectScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func ensureActorCanManageProject(actor user.UserContext, item *model.Project) error {
	return ensureActorCanAccessProject(actor, item)
}
