package ai

import (
	"errors"
	"net/http"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/task"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type createEvaluationRequest struct {
	Force bool `json:"force"`
}

type batchCreateEvaluationsRequest struct {
	VideoIDs []string `json:"videoIds" binding:"required"`
	Force    bool     `json:"force"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateForVideo(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	var req createEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.CreateForVideo(c.Request.Context(), actor, videoID, req.Force)
	if err != nil {
		h.handleServiceError(c, err, "failed to create ai evaluation")
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) BatchCreateForTask(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	var req batchCreateEvaluationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	videoIDs := make([]uuid.UUID, 0, len(req.VideoIDs))
	for _, raw := range req.VideoIDs {
		videoID, err := uuid.Parse(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid videoIds", nil)
			return
		}
		videoIDs = append(videoIDs, videoID)
	}

	result, err := h.service.BatchCreateForTask(c.Request.Context(), actor, taskID, videoIDs, req.Force)
	if err != nil {
		h.handleServiceError(c, err, "failed to batch create ai evaluations")
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) Get(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	evaluationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid ai evaluation id", nil)
		return
	}

	result, err := h.service.Get(c.Request.Context(), actor, evaluationID)
	if err != nil {
		h.handleServiceError(c, err, "failed to load ai evaluation")
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetResult(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	evaluationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid ai evaluation id", nil)
		return
	}

	result, err := h.service.GetResult(c.Request.Context(), actor, evaluationID)
	if err != nil {
		h.handleServiceError(c, err, "failed to load ai evaluation result")
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) handleServiceError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, ErrEvaluationVideoNotFound), errors.Is(err, task.ErrTaskNotFound), errors.Is(err, ErrEvaluationNotFound):
		response.Error(c, http.StatusNotFound, err.Error(), nil)
	case errors.Is(err, ErrEvaluationVideoNotReady),
		errors.Is(err, ErrEvaluationTaskRequired),
		errors.Is(err, ErrBatchVideoIDsRequired),
		errors.Is(err, ErrEvaluationForceRequired):
		response.Error(c, http.StatusBadRequest, err.Error(), nil)
	case errors.Is(err, ErrEvaluationResultNotReady), errors.Is(err, ErrEvaluationAlreadyProcessing):
		response.Error(c, http.StatusConflict, err.Error(), nil)
	case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
		response.Error(c, http.StatusForbidden, err.Error(), nil)
	case errors.Is(err, ErrProviderRequestFailed), errors.Is(err, ErrProviderResponseInvalid):
		response.Error(c, http.StatusBadGateway, err.Error(), nil)
	default:
		response.Error(c, http.StatusInternalServerError, fallback, nil)
	}
}
