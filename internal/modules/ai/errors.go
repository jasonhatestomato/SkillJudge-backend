package ai

import "errors"

var (
	ErrProviderNotConfigured       = errors.New("ai provider is not configured")
	ErrProviderRequestFailed       = errors.New("ai provider request failed")
	ErrProviderResponseInvalid     = errors.New("ai provider response invalid")
	ErrEvaluationVideoNotFound     = errors.New("video not found")
	ErrEvaluationNotFound          = errors.New("ai evaluation not found")
	ErrEvaluationResultNotReady    = errors.New("ai evaluation result not ready")
	ErrEvaluationAlreadyProcessing = errors.New("ai evaluation is already processing")
	ErrEvaluationForceRequired     = errors.New("ai evaluation already exists, use force=true to create a new run")
	ErrEvaluationVideoNotReady     = errors.New("video is not ready for ai evaluation")
	ErrEvaluationTaskRequired      = errors.New("video task is required")
	ErrBatchVideoIDsRequired       = errors.New("videoIds are required")
)
