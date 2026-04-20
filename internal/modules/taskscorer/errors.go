package taskscorer

import "errors"

var (
	ErrTaskScorerNameRequired         = errors.New("task scorer name is required")
	ErrTaskScorerEmailRequired        = errors.New("task scorer email is required")
	ErrTaskScorerEmailInvalid         = errors.New("task scorer email is invalid")
	ErrTaskScorerSchoolRequired       = errors.New("task scorer school is required")
	ErrTaskScorerMailUnavailable      = errors.New("task scorer mail service is unavailable")
	ErrTaskScorerInvitationSendFailed = errors.New("task scorer invitation email send failed")
	ErrTaskScorerRoleInvalid          = errors.New("task scorer role is invalid")
	ErrTaskScorerNotificationRole     = errors.New("task scorer notification role is not allowed")
	ErrTaskScorerAlreadyInOtherSchool = errors.New("task scorer already belongs to another school")
	ErrTaskScorerInactiveUser         = errors.New("task scorer user is not active")
	ErrTaskScorerRelationNotFound     = errors.New("task scorer relation not found")
	ErrTaskScorerInvitationInvalid    = errors.New("task scorer invitation is invalid")
	ErrTaskScorerInvitationNotFound   = errors.New("task scorer invitation is not found")
	ErrTaskScorerInvitationExpired    = errors.New("task scorer invitation is expired")
	ErrTaskScorerRelationInactive     = errors.New("task scorer relation is inactive")
)
