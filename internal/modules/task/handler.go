package task

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/project"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type createTaskRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description *string        `json:"description"`
	RubricID    string         `json:"rubricId" binding:"required"`
	StartDate   *time.Time     `json:"startDate"`
	Deadline    *time.Time     `json:"deadline"`
	EndDate     *time.Time     `json:"endDate"`
	Metadata    map[string]any `json:"metadata"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id", nil)
		return
	}

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	rubricID, err := uuid.Parse(req.RubricID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid rubricId", nil)
		return
	}

	result, err := h.service.Create(c.Request.Context(), actor, CreateTaskInput{
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		RubricID:    rubricID,
		StartDate:   req.StartDate,
		Deadline:    req.Deadline,
		EndDate:     req.EndDate,
		Metadata:    req.Metadata,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskNameRequired), errors.Is(err, ErrTaskRubricRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrTaskNameConflict):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, project.ErrProjectNotFound), errors.Is(err, project.ErrRubricNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskProjectScope), errors.Is(err, ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to create task", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) ListByProject(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	result, err := h.service.List(c.Request.Context(), actor, ListParams{
		Page:      page,
		PageSize:  pageSize,
		ProjectID: projectID,
		Status:    c.Query("status"),
		Keyword:   c.Query("keyword"),
	})
	if err != nil {
		switch {
		case errors.Is(err, project.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskProjectScope), errors.Is(err, ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list tasks", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), actor, taskID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskProjectScope), errors.Is(err, ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load task", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}
