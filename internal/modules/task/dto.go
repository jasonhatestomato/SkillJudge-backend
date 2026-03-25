package task

import (
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"

	"github.com/google/uuid"
)

type TaskDTO struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	Status          string          `json:"status"`
	StartDate       *time.Time      `json:"startDate,omitempty"`
	Deadline        *time.Time      `json:"deadline,omitempty"`
	EndDate         *time.Time      `json:"endDate,omitempty"`
	TotalVideos     int             `json:"totalVideos"`
	CompletedVideos int             `json:"completedVideos"`
	Project         *TaskProjectDTO `json:"project,omitempty"`
	Rubric          *TaskRubricDTO  `json:"rubric,omitempty"`
	Creator         *TaskCreatorDTO `json:"creator,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
	School          *user.SchoolDTO `json:"school,omitempty"`
}

type TaskProjectDTO struct {
	ID     uuid.UUID       `json:"id"`
	Name   string          `json:"name"`
	School *user.SchoolDTO `json:"school,omitempty"`
}

type TaskRubricDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	TotalScore int       `json:"totalScore"`
}

type TaskCreatorDTO struct {
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

type ListTasksResult struct {
	Items      []TaskDTO  `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Context struct {
	TaskID      uuid.UUID
	TaskName    string
	TaskStatus  string
	ProjectID   uuid.UUID
	ProjectName string
	SchoolID    *uuid.UUID
	SchoolName  *string
	RubricID    uuid.UUID
	RubricName  string
	CreatorID   *uuid.UUID
	CreatorName *string
	CreatorRole *string
	Item        *model.Task
}

func ToTaskDTO(item *model.Task) *TaskDTO {
	dto := &TaskDTO{
		ID:              item.ID,
		Name:            item.Name,
		Description:     item.Description,
		Status:          item.Status,
		StartDate:       item.StartDate,
		Deadline:        item.Deadline,
		EndDate:         item.EndDate,
		TotalVideos:     item.TotalVideos,
		CompletedVideos: item.CompletedVideos,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		Metadata:        item.Metadata,
	}

	if item.Project != nil {
		dto.Project = &TaskProjectDTO{
			ID:   item.Project.ID,
			Name: item.Project.Name,
		}
		if item.Project.School != nil {
			school := &user.SchoolDTO{
				ID:   item.Project.School.ID,
				Name: item.Project.School.Name,
			}
			dto.School = school
			dto.Project.School = school
		}
	}
	if item.Rubric != nil {
		dto.Rubric = &TaskRubricDTO{
			ID:         item.Rubric.ID,
			Name:       item.Rubric.Name,
			TotalScore: item.Rubric.TotalScore,
		}
	}
	if item.Creator != nil {
		dto.Creator = &TaskCreatorDTO{
			ID:       item.Creator.ID,
			Username: item.Creator.Username,
			RealName: item.Creator.RealName,
		}
	}

	return dto
}

func ToContext(item *model.Task) *Context {
	ctx := &Context{
		TaskID:     item.ID,
		TaskName:   item.Name,
		TaskStatus: item.Status,
		RubricID:   item.RubricID,
		CreatorID:  item.CreatorID,
		Item:       item,
	}

	if item.Project != nil {
		ctx.ProjectID = item.Project.ID
		ctx.ProjectName = item.Project.Name
		ctx.SchoolID = item.Project.SchoolID
		if item.Project.School != nil {
			ctx.SchoolName = &item.Project.School.Name
		}
	}
	if item.Rubric != nil {
		ctx.RubricName = item.Rubric.Name
	}
	if item.Creator != nil {
		ctx.CreatorName = &item.Creator.Username
		ctx.CreatorRole = &item.Creator.Role
	}

	return ctx
}
