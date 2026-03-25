package project

import (
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type RubricDTO struct {
	ID           uuid.UUID          `json:"id"`
	Name         string             `json:"name"`
	Description  *string            `json:"description,omitempty"`
	TotalScore   int                `json:"totalScore"`
	TemplateType *string            `json:"templateType,omitempty"`
	IsTemplate   bool               `json:"isTemplate"`
	IsPublic     bool               `json:"isPublic"`
	Items        []model.RubricItem `json:"items"`
	School       *user.SchoolDTO    `json:"school,omitempty"`
	Creator      *ProjectUserDTO    `json:"creator,omitempty"`
	CreatedAt    time.Time          `json:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt"`
}

type ListRubricsResult struct {
	Items      []RubricDTO `json:"items"`
	Pagination Pagination  `json:"pagination"`
}

func ToRubricDTO(item *model.ScoringRubric) *RubricDTO {
	dto := &RubricDTO{
		ID:           item.ID,
		Name:         item.Name,
		Description:  item.Description,
		TotalScore:   item.TotalScore,
		TemplateType: item.TemplateType,
		IsTemplate:   item.IsTemplate,
		IsPublic:     item.IsPublic,
		Items:        item.Items,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
	if item.School != nil {
		dto.School = &user.SchoolDTO{ID: item.School.ID, Name: item.School.Name}
	}
	if item.Creator != nil {
		dto.Creator = &ProjectUserDTO{
			ID:       item.Creator.ID,
			Username: item.Creator.Username,
			RealName: item.Creator.RealName,
		}
	}

	return dto
}
