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
	ActorID         *uuid.UUID
	Action          string
	ResourceType    *string
	ResourceID      *uuid.UUID
	RequestMethod   *string
	RequestPath     *string
	ResponseStatus  int
	ErrorMessage    *string
	RequestBody     map[string]any
}

type UserDTO struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	Email       *string    `json:"email,omitempty"`
	Phone       *string    `json:"phone,omitempty"`
	RealName    *string    `json:"realName,omitempty"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	Permissions []string   `json:"permissions,omitempty"`
	School      *SchoolDTO `json:"school,omitempty"`
}

type SchoolDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func ToUserDTO(user *model.User, permissions []string) *UserDTO {
	dto := &UserDTO{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Phone:       user.Phone,
		RealName:    user.RealName,
		Role:        user.Role,
		Status:      user.Status,
		Permissions: permissions,
	}

	if user.School != nil {
		dto.School = &SchoolDTO{
			ID:   user.School.ID,
			Name: user.School.Name,
		}
	}

	return dto
}
