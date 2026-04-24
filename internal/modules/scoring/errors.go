package scoring

import "errors"

var (
	ErrAssignmentStrategyRequired  = errors.New("assignmentStrategy is required")
	ErrAssignmentStrategyInvalid   = errors.New("assignmentStrategy must be one of: average, specific")
	ErrAssignmentVideoIDsRequired  = errors.New("videoIds are required")
	ErrAssignmentScorerIDsRequired = errors.New("scorerIds are required")
	ErrAssignmentScorerNotFound    = errors.New("one or more scorers are invalid")
	ErrAssignmentVideoNotFound     = errors.New("one or more videos are invalid for the task")
	ErrAssignmentSpecificRequired  = errors.New("specificAssignments are required when assignmentStrategy is specific")
	ErrAssignmentSpecificInvalid   = errors.New("specificAssignments are invalid")
	ErrAssignmentVideoNotReady     = errors.New("one or more videos are not ready for assignment")
	ErrAssignmentVideoCompleted    = errors.New("one or more videos have already completed evaluation")
	ErrAssignmentRoleNotAllowed    = errors.New("role is not allowed to assign scorers")
	ErrReassignmentModeRequired    = errors.New("reassignmentMode is required")
	ErrReassignmentModeInvalid     = errors.New("reassignmentMode must be one of: average, quantity")
	ErrReassignmentCountRequired   = errors.New("quantityAssignments are required when reassignmentMode is quantity")
	ErrReassignmentCountInvalid    = errors.New("quantityAssignments are invalid")
	ErrReassignmentCountMismatch   = errors.New("quantityAssignments total must equal the selected pending video count")
	ErrMyTasksStatusInvalid        = errors.New("status must be one of: pending, in_progress, completed, skipped")
	ErrScoringTaskNotFound         = errors.New("scoring task not found")
	ErrScoringTaskNoSavedDrafts    = errors.New("no saved scoring drafts found for this task")
	ErrSubmitScoreDetailsRequired  = errors.New("scoreDetails are required")
	ErrSubmitTotalScoreInvalid     = errors.New("totalScore must be greater than or equal to 0")
	ErrScoringTaskCompleted        = errors.New("scoring task already completed")
)
