package project

import (
	"context"
	"math"
	"mime/multipart"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type CreateRubricInput struct {
	Name         string             `json:"name"`
	Description  *string            `json:"description"`
	TotalScore   int                `json:"totalScore"`
	TemplateType *string            `json:"templateType"`
	IsTemplate   bool               `json:"isTemplate"`
	IsPublic     bool               `json:"isPublic"`
	Items        []model.RubricItem `json:"items"`
}

type UpdateRubricInput struct {
	Name         *string            `json:"name"`
	Description  *string            `json:"description"`
	TotalScore   *int               `json:"totalScore"`
	TemplateType *string            `json:"templateType"`
	IsTemplate   *bool              `json:"isTemplate"`
	IsPublic     *bool              `json:"isPublic"`
	Items        []model.RubricItem `json:"items"`
}

func (s *Service) CreateRubric(ctx context.Context, actor user.UserContext, input CreateRubricInput) (*RubricDTO, error) {
	if input.Name == "" {
		return nil, ErrRubricNameRequired
	}
	if len(input.Items) == 0 {
		return nil, ErrRubricItemsRequired
	}
	if !canManageRubric(actor.Role) {
		return nil, ErrRoleNotAllowed
	}

	var schoolID *uuid.UUID
	switch actor.Role {
	case "admin":
		schoolID = nil
	case "school_admin", "teacher":
		if actor.SchoolID == nil {
			return nil, ErrProjectSchoolRequired
		}
		schoolID = actor.SchoolID
	}

	totalScore := input.TotalScore
	if totalScore <= 0 {
		totalScore = 100
	}

	item := &model.ScoringRubric{
		Name:         input.Name,
		Description:  input.Description,
		TotalScore:   totalScore,
		TemplateType: input.TemplateType,
		SchoolID:     schoolID,
		CreatorID:    &actor.UserID,
		IsTemplate:   input.IsTemplate,
		IsPublic:     input.IsPublic,
		Items:        input.Items,
	}

	if err := s.repo.CreateRubric(ctx, item); err != nil {
		return nil, err
	}

	created, err := s.repo.FindRubricByID(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	return ToRubricDTO(created), nil
}

func (s *Service) GetRubricByID(ctx context.Context, actor user.UserContext, id uuid.UUID) (*RubricDTO, error) {
	item, err := s.repo.FindRubricByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrRubricNotFound
	}
	if err := ensureActorCanAccessRubric(actor, item); err != nil {
		return nil, err
	}

	return ToRubricDTO(item), nil
}

func (s *Service) ListRubrics(ctx context.Context, actor user.UserContext, params RubricListParams) (*ListRubricsResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	items, total, err := s.repo.ListRubrics(ctx, params, actor.Role, actor.UserID, actor.SchoolID)
	if err != nil {
		return nil, err
	}

	result := make([]RubricDTO, 0, len(items))
	for i := range items {
		result = append(result, *ToRubricDTO(&items[i]))
	}

	return &ListRubricsResult{
		Items: result,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) UpdateRubric(ctx context.Context, actor user.UserContext, id uuid.UUID, input UpdateRubricInput) (*RubricDTO, error) {
	item, err := s.repo.FindRubricByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrRubricNotFound
	}
	if err := ensureActorCanManageRubric(actor, item); err != nil {
		return nil, err
	}

	updates := map[string]any{"updated_at": time.Now()}
	hasChange := false

	if input.Name != nil {
		if *input.Name == "" {
			return nil, ErrRubricNameRequired
		}
		updates["name"] = *input.Name
		hasChange = true
	}
	if input.Description != nil {
		updates["description"] = *input.Description
		hasChange = true
	}
	if input.TotalScore != nil {
		updates["total_score"] = *input.TotalScore
		hasChange = true
	}
	if input.TemplateType != nil {
		updates["template_type"] = *input.TemplateType
		hasChange = true
	}
	if input.IsTemplate != nil {
		updates["is_template"] = *input.IsTemplate
		hasChange = true
	}
	if input.IsPublic != nil {
		updates["is_public"] = *input.IsPublic
		hasChange = true
	}
	if input.Items != nil {
		if len(input.Items) == 0 {
			return nil, ErrRubricItemsRequired
		}
		updates["items"] = input.Items
		hasChange = true
	}
	if !hasChange {
		return nil, ErrEmptyUpdatePayload
	}

	if err := s.repo.UpdateRubric(ctx, id, updates); err != nil {
		return nil, err
	}

	updated, err := s.repo.FindRubricByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return ToRubricDTO(updated), nil
}

func (s *Service) DeleteRubric(ctx context.Context, actor user.UserContext, id uuid.UUID) error {
	item, err := s.repo.FindRubricByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrRubricNotFound
	}
	if err := ensureActorCanManageRubric(actor, item); err != nil {
		return err
	}

	return s.repo.DeleteRubric(ctx, id)
}

func (s *Service) CreateRubricFromTemplate(ctx context.Context, actor user.UserContext, name string, description *string, file multipart.File) (*RubricDTO, error) {
	if name == "" {
		return nil, ErrRubricNameRequired
	}

	parsed, err := parseRubricTemplate(file)
	if err != nil {
		return nil, err
	}

	return s.CreateRubric(ctx, actor, CreateRubricInput{
		Name:        name,
		Description: description,
		TotalScore:  parsed.TotalScore,
		Items:       parsed.Items,
	})
}

func canManageRubric(role string) bool {
	switch role {
	case "admin", "school_admin", "teacher":
		return true
	default:
		return false
	}
}

func ensureActorCanAccessRubric(actor user.UserContext, item *model.ScoringRubric) error {
	if item.IsPublic {
		return nil
	}

	switch actor.Role {
	case "admin":
		return nil
	case "school_admin":
		if actor.SchoolID == nil || item.SchoolID == nil || *actor.SchoolID != *item.SchoolID {
			return ErrInvalidProjectScope
		}
		return nil
	case "teacher":
		if item.CreatorID != nil && *item.CreatorID == actor.UserID {
			return nil
		}
		if actor.SchoolID != nil && item.SchoolID != nil && *actor.SchoolID == *item.SchoolID {
			return nil
		}
		return ErrInvalidProjectScope
	default:
		return ErrRoleNotAllowed
	}
}

func ensureActorCanManageRubric(actor user.UserContext, item *model.ScoringRubric) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin":
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
