package project

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type createProjectRequest struct {
	Name           string         `json:"name" binding:"required"`
	Description    *string        `json:"description"`
	SchoolID       *string        `json:"schoolId"`
	RubricID       *string        `json:"rubricId"`
	Deadline       *time.Time     `json:"deadline"`
	Tags           []string       `json:"tags"`
	ExperimentType *string        `json:"experimentType"`
	GradeLevel     *string        `json:"gradeLevel"`
	Subject        *string        `json:"subject"`
	Metadata       map[string]any `json:"metadata"`
}

type updateProjectRequest struct {
	Name           *string        `json:"name"`
	Description    *string        `json:"description"`
	Status         *string        `json:"status"`
	RubricID       *string        `json:"rubricId"`
	Deadline       *time.Time     `json:"deadline"`
	Tags           []string       `json:"tags"`
	ExperimentType *string        `json:"experimentType"`
	GradeLevel     *string        `json:"gradeLevel"`
	Subject        *string        `json:"subject"`
	Metadata       map[string]any `json:"metadata"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	input := CreateProjectInput{
		Name:           req.Name,
		Description:    req.Description,
		Deadline:       req.Deadline,
		Tags:           req.Tags,
		ExperimentType: req.ExperimentType,
		GradeLevel:     req.GradeLevel,
		Subject:        req.Subject,
		Metadata:       req.Metadata,
	}

	if req.SchoolID != nil && *req.SchoolID != "" {
		schoolID, err := uuid.Parse(*req.SchoolID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid schoolId", nil)
			return
		}
		input.SchoolID = &schoolID
	}

	result, err := h.service.Create(c.Request.Context(), actor, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNameRequired), errors.Is(err, ErrProjectSchoolRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrRoleNotAllowed), errors.Is(err, ErrInvalidProjectScope):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to create project", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) List(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := ListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
	}

	if schoolIDRaw := c.Query("schoolId"); schoolIDRaw != "" {
		schoolID, err := uuid.Parse(schoolIDRaw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid schoolId", nil)
			return
		}
		params.SchoolID = &schoolID
	}
	if creatorIDRaw := c.Query("creatorId"); creatorIDRaw != "" {
		creatorID, err := uuid.Parse(creatorIDRaw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid creatorId", nil)
			return
		}
		params.CreatorID = &creatorID
	}
	if startRaw := c.Query("startDate"); startRaw != "" {
		startDate, err := time.Parse("2006-01-02", startRaw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid startDate", nil)
			return
		}
		params.StartDate = &startDate
	}
	if endRaw := c.Query("endDate"); endRaw != "" {
		endDate, err := time.Parse("2006-01-02", endRaw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid endDate", nil)
			return
		}
		params.EndDate = &endDate
	}

	result, err := h.service.List(c.Request.Context(), actor, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list projects", nil)
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id", nil)
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), actor, projectID)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidProjectScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load project", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Update(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id", nil)
		return
	}

	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	input := UpdateProjectInput{
		Name:           req.Name,
		Description:    req.Description,
		Status:         req.Status,
		Deadline:       req.Deadline,
		Tags:           req.Tags,
		ExperimentType: req.ExperimentType,
		GradeLevel:     req.GradeLevel,
		Subject:        req.Subject,
		Metadata:       req.Metadata,
	}
	result, err := h.service.Update(c.Request.Context(), actor, projectID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrProjectNameRequired), errors.Is(err, ErrInvalidProjectStatus), errors.Is(err, ErrEmptyUpdatePayload):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrInvalidProjectScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to update project", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Delete(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id", nil)
		return
	}

	if err := h.service.Delete(c.Request.Context(), actor, projectID); err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidProjectScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to delete project", nil)
		}
		return
	}

	response.NoContent(c)
}
