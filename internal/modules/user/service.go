package user

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)

type CreateUserInput struct {
	Username string
	Password string
	Email    *string
	RealName *string
	Phone    *string
	Role     string
	SchoolID *uuid.UUID
}

type UpdateProfileInput struct {
	Email    *string `json:"email"`
	RealName *string `json:"realName"`
	Phone    *string `json:"phone"`
}

type UpdateManagedUserInput struct {
	Role   *string `json:"role"`
	Status *string `json:"status"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type ListUsersResult struct {
	Items      []UserDTO  `json:"items"`
	Pagination Pagination `json:"pagination"`
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, actor UserContext, input CreateUserInput) (*UserDTO, error) {
	if err := validateCreateUserInput(input); err != nil {
		return nil, err
	}
	if err := validateManageRole(actor, input.Role, input.SchoolID); err != nil {
		return nil, err
	}

	role, err := s.repo.FindRoleByCode(ctx, input.Role)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, ErrRoleNotFound
	}

	existing, err := s.repo.FindByUsername(ctx, input.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Username:     input.Username,
		PasswordHash: string(hash),
		Email:        input.Email,
		Phone:        input.Phone,
		RealName:     input.RealName,
		Role:         input.Role,
		Status:       "active",
		SchoolID:     input.SchoolID,
	}

	if actor.Role == "school_admin" || actor.Role == "school_leader" {
		user.SchoolID = actor.SchoolID
	}

	if err := s.repo.Create(ctx, user, role.ID, &actor.UserID); err != nil {
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	permissions, err := s.repo.FindPermissionsByUserID(ctx, created.ID)
	if err != nil {
		return nil, err
	}

	return ToUserDTO(created, permissions), nil
}

func (s *Service) GetByID(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (*UserDTO, error) {
	user, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	permissions, err := s.repo.FindPermissionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return ToUserDTO(user, permissions), nil
}

func (s *Service) UpdateMe(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*UserDTO, error) {
	if input.Email == nil && input.RealName == nil && input.Phone == nil {
		return nil, ErrEmptyUpdatePayload
	}
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if input.Email != nil {
		updates["email"] = *input.Email
	}
	if input.RealName != nil {
		updates["real_name"] = *input.RealName
	}
	if input.Phone != nil {
		updates["phone"] = *input.Phone
	}

	if err := s.repo.UpdateProfile(ctx, userID, updates); err != nil {
		return nil, err
	}

	return s.GetMe(ctx, userID)
}

func (s *Service) List(ctx context.Context, actor UserContext, params ListParams) (*ListUsersResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	users, total, err := s.repo.List(ctx, params, actor.Role, actor.SchoolID)
	if err != nil {
		return nil, err
	}

	items := make([]UserDTO, 0, len(users))
	for _, item := range users {
		items = append(items, *ToUserDTO(&item, nil))
	}

	return &ListUsersResult{
		Items: items,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(params.PageSize))),
		},
	}, nil
}

func (s *Service) UpdateManagedUser(ctx context.Context, actor UserContext, targetUserID uuid.UUID, input UpdateManagedUserInput) (*UserDTO, error) {
	if input.Role == nil && input.Status == nil {
		return nil, ErrEmptyUpdatePayload
	}
	target, err := s.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	if err := ensureActorCanManageTarget(actor, target); err != nil {
		return nil, err
	}

	updates := map[string]any{
		"updated_at": time.Now(),
	}
	var roleID *uuid.UUID
	var permissions []string

	if input.Status != nil {
		if !isValidStatus(*input.Status) {
			return nil, ErrInvalidStatus
		}
		updates["status"] = *input.Status
	}

	if input.Role != nil {
		if !isValidRole(*input.Role) {
			return nil, ErrInvalidRole
		}
		if err := validateManageRole(actor, *input.Role, target.SchoolID); err != nil {
			return nil, err
		}
		role, err := s.repo.FindRoleByCode(ctx, *input.Role)
		if err != nil {
			return nil, err
		}
		if role == nil {
			return nil, ErrRoleNotFound
		}

		updates["role"] = *input.Role
		roleID = &role.ID
	}

	if err := s.repo.UpdateManagedUser(ctx, targetUserID, updates, roleID, &actor.UserID); err != nil {
		return nil, err
	}

	updated, err := s.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	permissions, err = s.repo.FindPermissionsByUserID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	return ToUserDTO(updated, permissions), nil
}

func (s *Service) Delete(ctx context.Context, actor UserContext, targetUserID uuid.UUID) error {
	target, err := s.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}

	if err := ensureActorCanManageTarget(actor, target); err != nil {
		return err
	}

	return s.repo.Delete(ctx, targetUserID)
}

func (s *Service) CreateAuditLog(ctx context.Context, input AuditInput) error {
	log := &model.AuditLog{
		UserID:         input.ActorID,
		Action:         input.Action,
		ResourceType:   input.ResourceType,
		ResourceID:     input.ResourceID,
		RequestMethod:  input.RequestMethod,
		RequestPath:    input.RequestPath,
		ResponseStatus: &input.ResponseStatus,
		ErrorMessage:   input.ErrorMessage,
		RequestBody:    input.RequestBody,
	}

	return s.repo.CreateAuditLog(ctx, log)
}

func validateManageRole(actor UserContext, role string, schoolID *uuid.UUID) error {
	if !isValidRole(role) {
		return ErrInvalidRole
	}
	switch actor.Role {
	case "admin":
		if role != "admin" && schoolID == nil {
			return ErrSchoolIDRequired
		}
		return nil
	case "school_admin", "school_leader":
		if role == "admin" || role == "school_admin" || role == "school_leader" {
			return ErrRoleNotAllowed
		}
		if actor.SchoolID == nil {
			return ErrForbiddenSchoolScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func ensureActorCanManageTarget(actor UserContext, target *model.User) error {
	switch actor.Role {
	case "admin":
		return nil
	case "school_admin", "school_leader":
		if target.Role == "admin" || target.Role == "school_admin" || target.Role == "school_leader" {
			return ErrRoleNotAllowed
		}
		if actor.SchoolID == nil || target.SchoolID == nil || *actor.SchoolID != *target.SchoolID {
			return ErrForbiddenSchoolScope
		}
		return nil
	default:
		return ErrRoleNotAllowed
	}
}

func validateCreateUserInput(input CreateUserInput) error {
	if !usernamePattern.MatchString(input.Username) {
		return ErrInvalidUsername
	}
	if len(input.Password) < 8 {
		return ErrInvalidPassword
	}
	if !isValidRole(input.Role) {
		return ErrInvalidRole
	}

	return nil
}

func isValidRole(role string) bool {
	switch role {
	case "admin", "school_admin", "school_leader", "teacher", "scorer", "student":
		return true
	default:
		return false
	}
}

func isValidStatus(status string) bool {
	switch status {
	case "active", "inactive", "banned":
		return true
	default:
		return false
	}
}
