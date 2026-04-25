package scoring

const (
	VideoEvaluationStatusPending    = "pending"
	VideoEvaluationStatusInProgress = "in_progress"
	VideoEvaluationStatusCompleted  = "completed"
)

const (
	VideoManualStatusPending    = "pending"
	VideoManualStatusInProgress = "in_progress"
	VideoManualStatusSubmitted  = "submitted"
)

const (
	ManualEvaluationStatusInProgress = "in_progress"
	ManualEvaluationStatusSubmitted  = "submitted"
)

const (
	ReviewAssignmentStatusPending    = "pending"
	ReviewAssignmentStatusInProgress = "in_progress"
	ReviewAssignmentStatusSubmitted  = "submitted"
	ReviewAssignmentStatusCancelled  = "cancelled"
)

const (
	ReviewAssignmentTypeNormal = "normal"
	SingleReviewNo             = 1
	ScoreDecisionTypeSingle    = "single"
)
