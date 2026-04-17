package evaluation

import (
	"strings"
	"time"
)

const (
	OverallStatusPending    = "pending"
	OverallStatusInProgress = "in_progress"
	OverallStatusCompleted  = "completed"
	OverallStatusFailed     = "failed"
)

const (
	ManualStatusPending    = "pending"
	ManualStatusInProgress = "in_progress"
	ManualStatusSubmitted  = "submitted"
)

const (
	AIStatusPending    = "pending"
	AIStatusProcessing = "processing"
	AIStatusCompleted  = "completed"
	AIStatusFailed     = "failed"
)

func ResolveOverallStatus(manualStatus, aiStatus string) string {
	manual := normalize(manualStatus)
	ai := normalize(aiStatus)

	switch {
	case ai == AIStatusFailed:
		return OverallStatusFailed
	case manual == ManualStatusSubmitted && ai == AIStatusCompleted:
		return OverallStatusCompleted
	case manual == ManualStatusInProgress,
		manual == ManualStatusSubmitted,
		ai == AIStatusProcessing,
		ai == AIStatusCompleted:
		return OverallStatusInProgress
	default:
		return OverallStatusPending
	}
}

func ResolveScorerTaskStatus(manualStatus string) string {
	switch normalize(manualStatus) {
	case ManualStatusSubmitted:
		return OverallStatusCompleted
	case ManualStatusInProgress:
		return OverallStatusInProgress
	default:
		return OverallStatusPending
	}
}

func ResolveCompletedAt(overallStatus string, now time.Time) *time.Time {
	if normalize(overallStatus) != OverallStatusCompleted {
		return nil
	}
	value := now
	return &value
}

func normalize(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
