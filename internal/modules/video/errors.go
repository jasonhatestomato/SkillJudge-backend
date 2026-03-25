package video

import "errors"

var (
	ErrVideoNotFound              = errors.New("video not found")
	ErrVideoProjectRequired       = errors.New("projectId is required")
	ErrVideoTaskRequired          = errors.New("taskId is required")
	ErrVideoFilenameRequired      = errors.New("filename is required")
	ErrVideoFileSizeInvalid       = errors.New("fileSize must be greater than 0")
	ErrVideoStudentNameRequired   = errors.New("studentName is required")
	ErrVideoStudentNumberRequired = errors.New("studentNumber is required")
	ErrVideoUploadIDRequired      = errors.New("uploadId is required")
	ErrVideoPartsRequired         = errors.New("parts are required")
	ErrInvalidVideoScope          = errors.New("forbidden video scope")
	ErrRoleNotAllowed             = errors.New("role not allowed")
)
