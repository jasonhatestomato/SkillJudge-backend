package user

import (
	"errors"
	"net/http"
	"strconv"

	"skilljudge/backend/internal/http/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type createUserRequest struct {
	Username       string  `json:"username" binding:"required"`
	Password       string  `json:"password" binding:"required"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
	RealName       *string `json:"realName"`
	InternalNumber *string `json:"internalNumber"`
	Role           string  `json:"role" binding:"required"`
	SchoolID       *string `json:"schoolId"`
}

type batchCreateUserItemRequest struct {
	Row            *int    `json:"row"`
	Username       string  `json:"username" binding:"required"`
	Password       string  `json:"password" binding:"required"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
	RealName       *string `json:"realName"`
	InternalNumber *string `json:"internalNumber"`
	Role           string  `json:"role" binding:"required"`
	SchoolID       *string `json:"schoolId"`
}

type batchCreateUsersRequest struct {
	Items []batchCreateUserItemRequest `json:"items" binding:"required"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetMe(c *gin.Context) {
	actor := currentUser(c)
	result, err := h.service.GetMe(c.Request.Context(), actor.UserID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to load current user", nil)
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) UpdateMe(c *gin.Context) {
	actor := currentUser(c)
	var input UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.UpdateMe(c.Request.Context(), actor.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyUpdatePayload):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to update current user", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Create(c *gin.Context) {
	actor := currentUser(c)
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	input := CreateUserInput{
		Username:       req.Username,
		Password:       req.Password,
		Email:          req.Email,
		Phone:          req.Phone,
		RealName:       req.RealName,
		InternalNumber: req.InternalNumber,
		Role:           req.Role,
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
		status := http.StatusInternalServerError
		message := "failed to create user"
		switch {
		case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidPassword), errors.Is(err, ErrInvalidRole):
			status = http.StatusBadRequest
			message = err.Error()
		case errors.Is(err, ErrRoleNotAllowed), errors.Is(err, ErrSchoolIDRequired), errors.Is(err, ErrForbiddenSchoolScope):
			status = http.StatusForbidden
			message = err.Error()
		case errors.Is(err, ErrRoleNotFound):
			status = http.StatusNotFound
			message = err.Error()
		case errors.Is(err, ErrUsernameTaken):
			status = http.StatusConflict
			message = err.Error()
		}
		h.audit(c, actor.UserID, "user.create", stringPtr("user"), nil, status, message, buildCreateUserAuditBody(req))
		response.Error(c, status, message, nil)
		return
	}

	h.audit(c, actor.UserID, "user.create", stringPtr("user"), &result.ID, http.StatusCreated, "", buildCreateUserAuditBody(req))
	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) BatchCreate(c *gin.Context) {
	actor := currentUser(c)
	var req batchCreateUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	inputs := make([]BatchCreateUserInput, 0, len(req.Items))
	for _, item := range req.Items {
		inputs = append(inputs, BatchCreateUserInput{
			Row:            item.Row,
			Username:       item.Username,
			Password:       item.Password,
			Email:          item.Email,
			Phone:          item.Phone,
			RealName:       item.RealName,
			InternalNumber: item.InternalNumber,
			Role:           item.Role,
			SchoolID:       item.SchoolID,
		})
	}

	result, err := h.service.BatchCreate(c.Request.Context(), actor, inputs)
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to batch create users"
		switch {
		case errors.Is(err, ErrBatchCreateItemsRequired):
			status = http.StatusBadRequest
			message = err.Error()
		}
		h.audit(c, actor.UserID, "user.batch_create", stringPtr("user"), nil, status, message, buildBatchCreateAuditBody(req, nil))
		response.Error(c, status, message, nil)
		return
	}

	h.audit(c, actor.UserID, "user.batch_create", stringPtr("user"), nil, http.StatusOK, "", buildBatchCreateAuditBody(req, result))
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) List(c *gin.Context) {
	actor := currentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := ListParams{
		Page:     page,
		PageSize: pageSize,
		Role:     c.Query("role"),
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

	result, err := h.service.List(c.Request.Context(), actor, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list users", nil)
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Update(c *gin.Context) {
	actor := currentUser(c)
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user id", nil)
		return
	}

	var input UpdateManagedUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.UpdateManagedUser(c.Request.Context(), actor, userID, input)
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to update user"
		switch {
		case errors.Is(err, ErrUserNotFound), errors.Is(err, ErrRoleNotFound):
			status = http.StatusNotFound
			message = err.Error()
		case errors.Is(err, ErrRoleNotAllowed), errors.Is(err, ErrForbiddenSchoolScope):
			status = http.StatusForbidden
			message = err.Error()
		case errors.Is(err, ErrInvalidRole), errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrEmptyUpdatePayload):
			status = http.StatusBadRequest
			message = err.Error()
		}
		h.audit(c, actor.UserID, "user.update", stringPtr("user"), &userID, status, message, buildManagedUserAuditBody(input))
		response.Error(c, status, message, nil)
		return
	}

	h.audit(c, actor.UserID, "user.update", stringPtr("user"), &userID, http.StatusOK, "", buildManagedUserAuditBody(input))
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Delete(c *gin.Context) {
	actor := currentUser(c)
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user id", nil)
		return
	}

	if err := h.service.Delete(c.Request.Context(), actor, userID); err != nil {
		status := http.StatusInternalServerError
		message := "failed to delete user"
		switch {
		case errors.Is(err, ErrUserNotFound):
			status = http.StatusNotFound
			message = err.Error()
		case errors.Is(err, ErrRoleNotAllowed), errors.Is(err, ErrForbiddenSchoolScope):
			status = http.StatusForbidden
			message = err.Error()
		}
		h.audit(c, actor.UserID, "user.delete", stringPtr("user"), &userID, status, message, nil)
		response.Error(c, status, message, nil)
		return
	}

	h.audit(c, actor.UserID, "user.delete", stringPtr("user"), &userID, http.StatusNoContent, "", nil)
	response.NoContent(c)
}

func currentUser(c *gin.Context) UserContext {
	value, _ := c.Get("authUser")
	return value.(UserContext)
}

func (h *Handler) audit(c *gin.Context, actorID uuid.UUID, action string, resourceType *string, resourceID *uuid.UUID, status int, errorMessage string, body map[string]any) {
	var message *string
	if errorMessage != "" {
		message = &errorMessage
	}

	method := c.Request.Method
	path := c.FullPath()
	_ = h.service.CreateAuditLog(c.Request.Context(), AuditInput{
		ActorID:        &actorID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		RequestMethod:  &method,
		RequestPath:    &path,
		ResponseStatus: status,
		ErrorMessage:   message,
		RequestBody:    body,
	})
}

func stringPtr(value string) *string {
	return &value
}

func buildCreateUserAuditBody(req createUserRequest) map[string]any {
	body := map[string]any{
		"username": req.Username,
		"role":     req.Role,
	}
	if req.SchoolID != nil {
		body["schoolId"] = *req.SchoolID
	}

	return body
}

func buildManagedUserAuditBody(input UpdateManagedUserInput) map[string]any {
	body := map[string]any{}
	if input.Role != nil {
		body["role"] = *input.Role
	}
	if input.Status != nil {
		body["status"] = *input.Status
	}

	return body
}

func buildBatchCreateAuditBody(req batchCreateUsersRequest, result *BatchCreateUsersResult) map[string]any {
	body := map[string]any{
		"total": len(req.Items),
	}

	items := make([]map[string]any, 0, minInt(len(req.Items), 10))
	for index, item := range req.Items {
		if index >= 10 {
			body["truncated"] = true
			break
		}

		row := map[string]any{
			"username": item.Username,
			"role":     item.Role,
		}
		if item.Row != nil {
			row["row"] = *item.Row
		}
		if item.SchoolID != nil {
			row["schoolId"] = *item.SchoolID
		}
		items = append(items, row)
	}
	if len(items) > 0 {
		body["items"] = items
	}
	if result != nil {
		body["success"] = result.Success
		body["failed"] = result.Failed
	}

	return body
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
