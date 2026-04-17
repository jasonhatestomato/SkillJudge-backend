package task

import "errors"

var (
	ErrTaskNotFound               = errors.New("task not found")
	ErrTaskNameRequired           = errors.New("task name is required")
	ErrTaskRubricRequired         = errors.New("task rubricId is required")
	ErrTaskNameConflict           = errors.New("task name already exists in this project")
	ErrTaskProjectScope           = errors.New("task is outside of allowed scope")
	ErrTaskRoleNotAllowed         = errors.New("role is not allowed to access task")
	ErrTaskAnalysisNotReady       = errors.New("task analysis is only available after all videos are completed")
	ErrTaskAnalysisReportNotFound = errors.New("task analysis report not found")
)
