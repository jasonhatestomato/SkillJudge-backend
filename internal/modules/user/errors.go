package user

import "errors"

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrRoleNotFound             = errors.New("role not found")
	ErrRoleNotAllowed           = errors.New("role not allowed")
	ErrBatchCreateItemsRequired = errors.New("items is required")
	ErrSchoolIDRequired         = errors.New("schoolId is required for this role")
	ErrForbiddenSchoolScope     = errors.New("school scope is required")
	ErrUsernameTaken            = errors.New("username already exists")
	ErrInvalidUsername          = errors.New("username must be 3-50 characters and contain only letters, numbers, underscore or hyphen")
	ErrInvalidPassword          = errors.New("password must be at least 8 characters")
	ErrInvalidRole              = errors.New("invalid role")
	ErrInvalidStatus            = errors.New("invalid status")
	ErrEmptyUpdatePayload       = errors.New("no fields provided for update")
)
