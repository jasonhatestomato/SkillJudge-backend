package project

import "errors"

var (
	ErrProjectNotFound      = errors.New("project not found")
	ErrProjectNameRequired  = errors.New("project name is required")
	ErrProjectSchoolRequired = errors.New("schoolId is required")
	ErrInvalidProjectStatus = errors.New("invalid project status")
	ErrInvalidProjectScope  = errors.New("forbidden project scope")
	ErrRoleNotAllowed       = errors.New("role not allowed")
	ErrEmptyUpdatePayload   = errors.New("no fields provided for update")
	ErrRubricNotFound       = errors.New("rubric not found")
	ErrRubricNameRequired   = errors.New("rubric name is required")
	ErrRubricItemsRequired  = errors.New("rubric items are required")
	ErrRubricTemplateInvalid = errors.New("invalid rubric template")
)
