package project

import (
	"errors"
	"net/http"
	"strconv"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createRubricRequest struct {
	Name         string             `json:"name" binding:"required"`
	Description  *string            `json:"description"`
	TotalScore   int                `json:"totalScore"`
	TemplateType *string            `json:"templateType"`
	IsTemplate   bool               `json:"isTemplate"`
	IsPublic     bool               `json:"isPublic"`
	Items        []model.RubricItem `json:"items" binding:"required"`
}

type updateRubricRequest struct {
	Name         *string            `json:"name"`
	Description  *string            `json:"description"`
	TotalScore   *int               `json:"totalScore"`
	TemplateType *string            `json:"templateType"`
	IsTemplate   *bool              `json:"isTemplate"`
	IsPublic     *bool              `json:"isPublic"`
	Items        []model.RubricItem `json:"items"`
}

type uploadRubricTemplateRequest struct {
	Name        string  `form:"name" binding:"required"`
	Description *string `form:"description"`
}

func (h *Handler) CreateRubric(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	var req createRubricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.CreateRubric(c.Request.Context(), actor, CreateRubricInput{
		Name:         req.Name,
		Description:  req.Description,
		TotalScore:   req.TotalScore,
		TemplateType: req.TemplateType,
		IsTemplate:   req.IsTemplate,
		IsPublic:     req.IsPublic,
		Items:        req.Items,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrRubricNameRequired), errors.Is(err, ErrRubricItemsRequired), errors.Is(err, ErrProjectSchoolRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrRoleNotAllowed), errors.Is(err, ErrInvalidProjectScope):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to create rubric", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) UploadRubricTemplate(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	var req uploadRubricTemplateRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "file is required", nil)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to open uploaded file", nil)
		return
	}
	defer func() { _ = file.Close() }()

	result, err := h.service.CreateRubricFromTemplate(c.Request.Context(), actor, req.Name, req.Description, file)
	if err != nil {
		switch {
		case errors.Is(err, ErrRubricNameRequired), errors.Is(err, ErrRubricItemsRequired), errors.Is(err, ErrProjectSchoolRequired), errors.Is(err, ErrRubricTemplateInvalid):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrRoleNotAllowed), errors.Is(err, ErrInvalidProjectScope):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to upload rubric template", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) ListRubrics(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := RubricListParams{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
	}

	if raw := c.Query("isTemplate"); raw != "" {
		value := raw == "true"
		params.IsTemplate = &value
	}

	result, err := h.service.ListRubrics(c.Request.Context(), actor, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list rubrics", nil)
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetRubric(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	rubricID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid rubric id", nil)
		return
	}

	result, err := h.service.GetRubricByID(c.Request.Context(), actor, rubricID)
	if err != nil {
		switch {
		case errors.Is(err, ErrRubricNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidProjectScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load rubric", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) UpdateRubric(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	rubricID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid rubric id", nil)
		return
	}

	var req updateRubricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.UpdateRubric(c.Request.Context(), actor, rubricID, UpdateRubricInput{
		Name:         req.Name,
		Description:  req.Description,
		TotalScore:   req.TotalScore,
		TemplateType: req.TemplateType,
		IsTemplate:   req.IsTemplate,
		IsPublic:     req.IsPublic,
		Items:        req.Items,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrRubricNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrRubricNameRequired), errors.Is(err, ErrRubricItemsRequired), errors.Is(err, ErrEmptyUpdatePayload):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrInvalidProjectScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to update rubric", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) DeleteRubric(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	rubricID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid rubric id", nil)
		return
	}

	if err := h.service.DeleteRubric(c.Request.Context(), actor, rubricID); err != nil {
		switch {
		case errors.Is(err, ErrRubricNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidProjectScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to delete rubric", nil)
		}
		return
	}

	response.NoContent(c)
}
