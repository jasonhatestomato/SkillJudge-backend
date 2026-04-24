package user

import (
	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
)

type UserContext struct {
	UserID   uuid.UUID
	Role     string
	SchoolID *uuid.UUID
}

type AuditInput struct {
	ActorID        *uuid.UUID
	Action         string
	ResourceType   *string
	ResourceID     *uuid.UUID
	RequestMethod  *string
	RequestPath    *string
	ResponseStatus int
	ErrorMessage   *string
	RequestBody    map[string]any
}

type UserDTO struct {
	ID             uuid.UUID  `json:"id"`
	Username       string     `json:"username"`
	Email          *string    `json:"email,omitempty"`
	Phone          *string    `json:"phone,omitempty"`
	RealName       *string    `json:"realName,omitempty"`
	InternalNumber *string    `json:"internalNumber,omitempty"`
	Role           string     `json:"role"`
	Status         string     `json:"status"`
	Permissions    []string   `json:"permissions,omitempty"`
	School         *SchoolDTO `json:"school,omitempty"`
}

type SchoolDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type BatchCreateUserError struct {
	Row      *int   `json:"row,omitempty"`
	Username string `json:"username,omitempty"`
	Error    string `json:"error"`
}

type BatchCreateUsersResult struct {
	Total   int                    `json:"total"`
	Success int                    `json:"success"`
	Failed  int                    `json:"failed"`
	Errors  []BatchCreateUserError `json:"errors,omitempty"`
}

func ToUserDTO(user *model.User, permissions []string) *UserDTO {
	dto := &UserDTO{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		Phone:          user.Phone,
		RealName:       user.RealName,
		InternalNumber: user.InternalNumber,
		Role:           user.Role,
		Status:         user.Status,
		Permissions:    permissions,
	}

	if user.School != nil {
		dto.School = &SchoolDTO{
			ID:   user.School.ID,
			Name: user.School.Name,
		}
	}

	return dto
}
