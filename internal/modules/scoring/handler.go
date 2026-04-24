package scoring

import (
	"errors"
	"net/http"
	"strconv"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/task"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type assignScorersRequest struct {
	VideoIDs            []string `json:"videoIds" binding:"required"`
	ScorerIDs           []string `json:"scorerIds" binding:"required"`
	AssignmentStrategy  string   `json:"assignmentStrategy" binding:"required"`
	SpecificAssignments []struct {
		VideoID  string `json:"videoId"`
		ScorerID string `json:"scorerId"`
	} `json:"specificAssignments"`
}

type reassignPendingVideosRequest struct {
	VideoIDs            []string `json:"videoIds" binding:"required"`
	ScorerIDs           []string `json:"scorerIds" binding:"required"`
	ReassignmentMode    string   `json:"reassignmentMode" binding:"required"`
	QuantityAssignments []struct {
		ScorerID string `json:"scorerId"`
		Count    int    `json:"count"`
	} `json:"quantityAssignments"`
}

type submitTaskRequest struct {
	ScoreDetails []map[string]any `json:"scoreDetails" binding:"required"`
	TotalScore   float64          `json:"totalScore"`
	Comments     *string          `json:"comments"`
}

type saveTaskDraftRequest struct {
	ScoreDetails []map[string]any `json:"scoreDetails"`
	TotalScore   float64          `json:"totalScore"`
	Comments     *string          `json:"comments"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetTaskDetail(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	result, err := h.service.GetTaskDetail(c.Request.Context(), actor, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrScoringTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load scoring task", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) SubmitTask(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	var req submitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.SubmitTask(c.Request.Context(), actor, id, SubmitTaskInput{
		ScoreDetails: req.ScoreDetails,
		TotalScore:   req.TotalScore,
		Comments:     req.Comments,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrSubmitScoreDetailsRequired), errors.Is(err, ErrSubmitTotalScoreInvalid):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrScoringTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrScoringTaskCompleted):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to submit scoring task", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) SaveTaskDraft(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	var req saveTaskDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.SaveTaskDraft(c.Request.Context(), actor, id, SubmitTaskInput{
		ScoreDetails: req.ScoreDetails,
		TotalScore:   req.TotalScore,
		Comments:     req.Comments,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrSubmitTotalScoreInvalid):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrScoringTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrScoringTaskCompleted):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to save scoring draft", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) SubmitSavedTask(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	result, err := h.service.SubmitSavedTask(c.Request.Context(), actor, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrScoringTaskNoSavedDrafts):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to submit saved scoring drafts", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ListMyTasks(c *gin.Context) {
	actor := middleware.CurrentUser(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := MyTasksListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
	}
	if projectIDRaw := c.Query("projectId"); projectIDRaw != "" {
		projectID, err := uuid.Parse(projectIDRaw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid projectId", nil)
			return
		}
		params.ProjectID = &projectID
	}

	result, err := h.service.ListMyTasks(c.Request.Context(), actor, params)
	if err != nil {
		switch {
		case errors.Is(err, ErrMyTasksStatusInvalid):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list my tasks", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ListAssignableScorers(c *gin.Context) {
	actor := middleware.CurrentUser(c)

	result, err := h.service.ListAssignableScorers(c.Request.Context(), actor)
	if err != nil {
		switch {
		case errors.Is(err, ErrAssignmentRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list assignable scorers", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) AssignScorers(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	var req assignScorersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	videoIDs, err := parseUUIDList(req.VideoIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid videoIds", nil)
		return
	}
	scorerIDs, err := parseUUIDList(req.ScorerIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid scorerIds", nil)
		return
	}

	specificAssignments := make([]SpecificAssignment, 0, len(req.SpecificAssignments))
	for _, item := range req.SpecificAssignments {
		videoID, err := uuid.Parse(item.VideoID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid specificAssignments.videoId", nil)
			return
		}
		scorerID, err := uuid.Parse(item.ScorerID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid specificAssignments.scorerId", nil)
			return
		}
		specificAssignments = append(specificAssignments, SpecificAssignment{
			VideoID:  videoID,
			ScorerID: scorerID,
		})
	}

	result, err := h.service.AssignScorers(c.Request.Context(), actor, AssignScorersInput{
		TaskID:              taskID,
		VideoIDs:            videoIDs,
		ScorerIDs:           scorerIDs,
		AssignmentStrategy:  AssignmentStrategy(req.AssignmentStrategy),
		SpecificAssignments: specificAssignments,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrAssignmentStrategyRequired),
			errors.Is(err, ErrAssignmentStrategyInvalid),
			errors.Is(err, ErrAssignmentVideoIDsRequired),
			errors.Is(err, ErrAssignmentScorerIDsRequired),
			errors.Is(err, ErrAssignmentSpecificRequired),
			errors.Is(err, ErrAssignmentSpecificInvalid),
			errors.Is(err, ErrAssignmentVideoNotReady),
			errors.Is(err, ErrAssignmentVideoCompleted):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, ErrAssignmentVideoNotFound), errors.Is(err, ErrAssignmentScorerNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed), errors.Is(err, ErrAssignmentRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to assign scorers", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) ListPendingAssignments(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	result, err := h.service.ListPendingAssignments(c.Request.Context(), actor, taskID)
	if err != nil {
		switch {
		case errors.Is(err, task.ErrTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed), errors.Is(err, ErrAssignmentRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list pending assignments", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ReassignPendingVideos(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	var req reassignPendingVideosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	videoIDs, err := parseUUIDList(req.VideoIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid videoIds", nil)
		return
	}
	scorerIDs, err := parseUUIDList(req.ScorerIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid scorerIds", nil)
		return
	}
	quantityAssignments := make([]QuantityAssignment, 0, len(req.QuantityAssignments))
	for _, item := range req.QuantityAssignments {
		scorerID, parseErr := uuid.Parse(item.ScorerID)
		if parseErr != nil {
			response.Error(c, http.StatusBadRequest, "invalid quantityAssignments.scorerId", nil)
			return
		}
		quantityAssignments = append(quantityAssignments, QuantityAssignment{
			ScorerID: scorerID,
			Count:    item.Count,
		})
	}

	result, err := h.service.ReassignPendingVideos(c.Request.Context(), actor, ReassignPendingVideosInput{
		TaskID:              taskID,
		VideoIDs:            videoIDs,
		ScorerIDs:           scorerIDs,
		ReassignmentMode:    ReassignmentMode(req.ReassignmentMode),
		QuantityAssignments: quantityAssignments,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrAssignmentVideoIDsRequired),
			errors.Is(err, ErrAssignmentScorerIDsRequired),
			errors.Is(err, ErrAssignmentVideoNotReady),
			errors.Is(err, ErrAssignmentVideoCompleted),
			errors.Is(err, ErrReassignmentModeRequired),
			errors.Is(err, ErrReassignmentModeInvalid),
			errors.Is(err, ErrReassignmentCountRequired),
			errors.Is(err, ErrReassignmentCountInvalid),
			errors.Is(err, ErrReassignmentCountMismatch):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, ErrAssignmentVideoNotFound), errors.Is(err, ErrAssignmentScorerNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed), errors.Is(err, ErrAssignmentRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to reassign pending videos", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func parseUUIDList(items []string) ([]uuid.UUID, error) {
	result := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		id, err := uuid.Parse(item)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}
