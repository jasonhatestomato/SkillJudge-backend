package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"skilljudge/backend/internal/config"
	"skilljudge/backend/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type roleSeed struct {
	Name         string
	Code         string
	Description  string
	ScopeType    string
	IsBuiltin    bool
	IsSuperAdmin bool
	Permissions  []string
}

var defaultRoleSeeds = []roleSeed{
	{
		Name:         "系统管理员",
		Code:         "admin",
		Description:  "系统最高管理员",
		ScopeType:    "platform",
		IsBuiltin:    true,
		IsSuperAdmin: true,
		Permissions: []string{
			"user:create", "user:read", "user:update", "user:delete",
			"project:create", "project:read", "project:update", "project:delete",
			"rubric:create", "rubric:read", "rubric:update", "rubric:delete",
			"video:create", "video:read", "video:update", "video:delete",
			"task:create", "task:read", "task:update", "task:submit",
			"ai:evaluate", "ai:read",
		},
	},
	{
		Name:        "学校管理员",
		Code:        "school_admin",
		Description: "学校范围内的管理员",
		ScopeType:   "school",
		IsBuiltin:   true,
		Permissions: []string{
			"user:create", "user:read", "user:update", "user:delete",
			"project:create", "project:read", "project:update", "project:delete",
			"rubric:create", "rubric:read", "rubric:update", "rubric:delete",
			"video:create", "video:read", "video:delete", "task:read", "task:update",
		},
	},
	{
		Name:        "校领导",
		Code:        "school_leader",
		Description: "学校负责人，当前权限与学校管理员一致",
		ScopeType:   "school",
		IsBuiltin:   true,
		Permissions: []string{
			"user:create", "user:read", "user:update", "user:delete",
			"project:create", "project:read", "project:update", "project:delete",
			"rubric:create", "rubric:read", "rubric:update", "rubric:delete",
			"video:create", "video:read", "video:delete", "task:read", "task:update",
		},
	},
	{
		Name:        "教师",
		Code:        "teacher",
		Description: "项目与评分细则管理者",
		ScopeType:   "school",
		IsBuiltin:   true,
		Permissions: []string{
			"project:create", "project:read", "project:update", "project:delete",
			"rubric:create", "rubric:read", "rubric:update", "rubric:delete",
			"video:create", "video:read", "video:delete",
			"task:create", "task:read", "task:update",
			"ai:evaluate", "ai:read",
		},
	},
	{
		Name:        "评分员",
		Code:        "scorer",
		Description: "执行人工评分",
		ScopeType:   "school",
		IsBuiltin:   true,
		Permissions: []string{
			"task:read", "task:update", "task:submit", "video:read", "ai:read",
		},
	},
	{
		Name:        "学生",
		Code:        "student",
		Description: "学生角色",
		ScopeType:   "school",
		IsBuiltin:   true,
		Permissions: []string{
			"project:read", "video:read",
		},
	},
}

func Run(ctx context.Context, db *gorm.DB, cfg config.BootstrapConfig) error {
	if err := seedRBAC(ctx, db); err != nil {
		return err
	}
	if err := ensureDefaultAdmin(ctx, db, cfg); err != nil {
		return err
	}

	return nil
}

func seedRBAC(ctx context.Context, db *gorm.DB) error {
	permissionsByCode, err := seedPermissions(ctx, db)
	if err != nil {
		return err
	}

	for _, item := range defaultRoleSeeds {
		var role model.Role
		err := db.WithContext(ctx).Where("code = ?", item.Code).First(&role).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			description := item.Description
			role = model.Role{
				Name:         item.Name,
				Code:         item.Code,
				Description:  &description,
				Permissions:  item.Permissions,
				ScopeType:    item.ScopeType,
				IsBuiltin:    item.IsBuiltin,
				IsSuperAdmin: item.IsSuperAdmin,
				Status:       "active",
			}
			if err := db.WithContext(ctx).Create(&role).Error; err != nil {
				return fmt.Errorf("create role %s: %w", item.Code, err)
			}
		case err != nil:
			return fmt.Errorf("find role %s: %w", item.Code, err)
		default:
			legacyPermissions, err := json.Marshal(item.Permissions)
			if err != nil {
				return fmt.Errorf("marshal legacy permissions for %s: %w", item.Code, err)
			}
			updates := map[string]any{
				"name": item.Name,
				// Keep the legacy JSON field in sync for compatibility, even though
				// runtime permission checks now read from role_permissions.
				"permissions":    string(legacyPermissions),
				"scope_type":     item.ScopeType,
				"is_builtin":     item.IsBuiltin,
				"is_super_admin": item.IsSuperAdmin,
				"status":         "active",
			}
			description := item.Description
			updates["description"] = &description
			if err := db.WithContext(ctx).Model(&model.Role{}).Where("id = ?", role.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update role %s: %w", item.Code, err)
			}
		}

		if err := syncRolePermissions(ctx, db, role.ID, item.Permissions, permissionsByCode); err != nil {
			return fmt.Errorf("sync role permissions for %s: %w", item.Code, err)
		}
	}

	return nil
}

func seedPermissions(ctx context.Context, db *gorm.DB) (map[string]model.Permission, error) {
	codes := make(map[string]struct{})
	for _, role := range defaultRoleSeeds {
		for _, code := range role.Permissions {
			codes[code] = struct{}{}
		}
	}

	for code := range codes {
		resourceCode, actionCode := splitPermissionCode(code)
		module := resourceCode
		description := code

		var permission model.Permission
		err := db.WithContext(ctx).Where("code = ?", code).First(&permission).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			permission = model.Permission{
				Code:         code,
				ResourceCode: resourceCode,
				ActionCode:   actionCode,
				Name:         code,
				Description:  &description,
				Module:       &module,
				IsBuiltin:    true,
				Status:       "active",
			}
			if err := db.WithContext(ctx).Create(&permission).Error; err != nil {
				return nil, fmt.Errorf("create permission %s: %w", code, err)
			}
		case err != nil:
			return nil, fmt.Errorf("find permission %s: %w", code, err)
		default:
			if err := db.WithContext(ctx).Model(&model.Permission{}).
				Where("id = ?", permission.ID).
				Updates(map[string]any{
					"resource_code": resourceCode,
					"action_code":   actionCode,
					"name":          code,
					"description":   &description,
					"module":        &module,
					"is_builtin":    true,
					"status":        "active",
				}).Error; err != nil {
				return nil, fmt.Errorf("update permission %s: %w", code, err)
			}
		}
	}

	var permissions []model.Permission
	if err := db.WithContext(ctx).Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("load permissions: %w", err)
	}

	result := make(map[string]model.Permission, len(permissions))
	for _, permission := range permissions {
		result[permission.Code] = permission
	}

	return result, nil
}

func syncRolePermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, codes []string, permissionsByCode map[string]model.Permission) error {
	// Phase 1 seeds from a fixed builtin definition, so a replace-all sync keeps
	// the role_permissions table deterministic across environments.
	if err := db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		return err
	}

	if len(codes) == 0 {
		return nil
	}

	links := make([]model.RolePermission, 0, len(codes))
	for _, code := range codes {
		permission, ok := permissionsByCode[code]
		if !ok {
			return fmt.Errorf("permission not found: %s", code)
		}
		links = append(links, model.RolePermission{
			RoleID:       roleID,
			PermissionID: permission.ID,
		})
	}

	return db.WithContext(ctx).Create(&links).Error
}

func splitPermissionCode(code string) (string, string) {
	left, right, ok := strings.Cut(code, ":")
	if !ok {
		return code, code
	}
	return left, right
}

func ensureDefaultAdmin(ctx context.Context, db *gorm.DB, cfg config.BootstrapConfig) error {
	var count int64
	if err := db.WithContext(ctx).
		Table("users").
		Joins("join user_roles on user_roles.user_id = users.id and user_roles.status = ?", "active").
		Joins("join roles on roles.id = user_roles.role_id and roles.status = ?", "active").
		Where("roles.code = ?", "admin").
		Count(&count).Error; err != nil {
		return fmt.Errorf("count admin users: %w", err)
	}
	if count > 0 {
		return nil
	}

	var adminRole model.Role
	if err := db.WithContext(ctx).Where("code = ?", "admin").First(&adminRole).Error; err != nil {
		return fmt.Errorf("load admin role: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.InitAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash init admin password: %w", err)
	}

	realName := cfg.InitAdminRealName
	admin := model.User{
		ID:           uuid.New(),
		Username:     cfg.InitAdminUsername,
		PasswordHash: string(hashed),
		Role:         "admin",
		Status:       "active",
		RealName:     &realName,
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return fmt.Errorf("create init admin: %w", err)
		}

		userRole := model.UserRole{
			UserID: admin.ID,
			RoleID: adminRole.ID,
			Status: "active",
		}
		if err := tx.Create(&userRole).Error; err != nil {
			return fmt.Errorf("bind init admin role: %w", err)
		}

		return nil
	})
}
