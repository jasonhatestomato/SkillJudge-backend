package task

import "errors"

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskNameRequired   = errors.New("task name is required")
	ErrTaskRubricRequired = errors.New("task rubricId is required")
	ErrTaskProjectScope   = errors.New("task is outside of allowed scope")
	ErrTaskRoleNotAllowed = errors.New("role is not allowed to access task")
)
