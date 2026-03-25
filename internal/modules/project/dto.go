package project

import (
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type ProjectDTO struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	Status          string          `json:"status"`
	Deadline        *time.Time      `json:"deadline,omitempty"`
	StartDate       *time.Time      `json:"startDate,omitempty"`
	EndDate         *time.Time      `json:"endDate,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	ExperimentType  *string         `json:"experimentType,omitempty"`
	GradeLevel      *string         `json:"gradeLevel,omitempty"`
	Subject         *string         `json:"subject,omitempty"`
	TotalVideos     int             `json:"totalVideos"`
	CompletedVideos int             `json:"completedVideos"`
	School          *user.SchoolDTO `json:"school,omitempty"`
	Creator         *ProjectUserDTO `json:"creator,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type ProjectUserDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	RealName *string   `json:"realName,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type ListProjectsResult struct {
	Items      []ProjectDTO `json:"items"`
	Pagination Pagination   `json:"pagination"`
}

func ToProjectDTO(item *model.Project) *ProjectDTO {
	dto := &ProjectDTO{
		ID:              item.ID,
		Name:            item.Name,
		Description:     item.Description,
		Status:          item.Status,
		Deadline:        item.Deadline,
		StartDate:       item.StartDate,
		EndDate:         item.EndDate,
		Tags:            item.Tags,
		ExperimentType:  item.ExperimentType,
		GradeLevel:      item.GradeLevel,
		Subject:         item.Subject,
		TotalVideos:     item.TotalVideos,
		CompletedVideos: item.CompletedVideos,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}

	if item.School != nil {
		dto.School = &user.SchoolDTO{
			ID:   item.School.ID,
			Name: item.School.Name,
		}
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
