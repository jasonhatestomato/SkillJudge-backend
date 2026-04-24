package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type ListParams struct {
	Page     int
	PageSize int
	Role     string
	SchoolID *uuid.UUID
	Keyword  string
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.userBaseQuery(ctx).
		Where("users.username = ?", username).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	normalized := normalizeEmail(email)
	if normalized == "" {
		return nil, nil
	}

	var user model.User
	err := r.userBaseQuery(ctx).
		Where("LOWER(BTRIM(users.email)) = ?", normalized).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.userBaseQuery(ctx).
		Where("users.id = ?", id).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *Repository) FindRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &role, nil
}

func (r *Repository) FindPermissionsByUserID(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var permissions []string
	err := r.db.WithContext(ctx).
		Table("permissions").
		Distinct("permissions.code").
		Joins("join role_permissions on role_permissions.permission_id = permissions.id").
		Joins("join roles on roles.id = role_permissions.role_id").
		Joins("join user_roles on user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Where("user_roles.status = ?", "active").
		Where("roles.status = ?", "active").
		Where("permissions.status = ?", "active").
		Order("permissions.code ASC").
		Pluck("permissions.code", &permissions).Error
	if err != nil {
		return nil, err
	}

	return permissions, nil
}

func (r *Repository) Create(ctx context.Context, user *model.User, roleID uuid.UUID, grantedBy *uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		userRole := model.UserRole{
			UserID:    user.ID,
			RoleID:    roleID,
			Status:    "active",
			GrantedBy: grantedBy,
		}

		return tx.Create(&userRole).Error
	})
}

func (r *Repository) UpdateProfile(ctx context.Context, userID uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *Repository) UpdateManagedUser(ctx context.Context, userID uuid.UUID, updates map[string]any, roleID *uuid.UUID, grantedBy *uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
				return err
			}
		}

		if roleID != nil {
			now := time.Now()
			if err := tx.Model(&model.UserRole{}).
				Where("user_id = ? AND status = ?", userID, "active").
				Updates(map[string]any{
					"status":     "disabled",
					"updated_at": now,
				}).Error; err != nil {
				return err
			}

			userRole := model.UserRole{
				UserID:    userID,
				RoleID:    *roleID,
				Status:    "active",
				GrantedBy: grantedBy,
			}
			if err := tx.Create(&userRole).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) Delete(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, "id = ?", userID).Error
}

func (r *Repository) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	return r.db.WithContext(ctx).Create(log).Error
}

func (r *Repository) List(ctx context.Context, params ListParams, actorRole string, actorSchoolID *uuid.UUID) ([]model.User, int64, error) {
	query := r.userBaseQuery(ctx).Model(&model.User{})

	if params.Role != "" {
		query = query.Where("COALESCE(primary_roles.code, users.role) = ?", params.Role)
	}

	if params.Keyword != "" {
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		query = query.Where("users.username ILIKE ? OR users.real_name ILIKE ? OR users.internal_number ILIKE ?", keyword, keyword, keyword)
	}

	if (actorRole == "school_admin" || actorRole == "school_leader") && actorSchoolID != nil {
		query = query.Where("users.school_id = ?", *actorSchoolID)
	} else if params.SchoolID != nil {
		query = query.Where("users.school_id = ?", *params.SchoolID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (params.Page - 1) * params.PageSize
	var users []model.User
	if err := query.Order("users.created_at DESC").Offset(offset).Limit(params.PageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *Repository) userBaseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Preload("School").
		Select("users.*, COALESCE(primary_roles.code, users.role) AS role").
		Joins(`
			LEFT JOIN user_roles AS primary_user_roles
				ON primary_user_roles.user_id = users.id
				AND primary_user_roles.status = 'active'
		`).
		Joins(`
			LEFT JOIN roles AS primary_roles
				ON primary_roles.id = primary_user_roles.role_id
				AND primary_roles.status = 'active'
		`)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
