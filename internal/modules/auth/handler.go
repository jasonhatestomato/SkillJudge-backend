package auth

import (
	"errors"
	"net/http"

	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/user"
	platformjwt "skilljudge/backend/internal/platform/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service     *Service
	userService *user.Service
}

func NewHandler(service *Service, userService *user.Service) *Handler {
	return &Handler{service: service, userService: userService}
}

func (h *Handler) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		message := "login failed"
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			status = http.StatusUnauthorized
			message = err.Error()
		case errors.Is(err, ErrForbidden):
			status = http.StatusForbidden
			message = "user is not active"
		}
		h.audit(c, nil, "user.login", status, message, map[string]any{"username": input.Username})
		response.Error(c, status, message, nil)
		return
	}

	h.audit(c, &result.User.ID, "user.login", http.StatusOK, "", map[string]any{"username": input.Username})
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Refresh(c *gin.Context) {
	var input RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), input.RefreshToken)
	if err != nil {
		status := http.StatusInternalServerError
		message := "refresh failed"
		switch {
		case errors.Is(err, ErrInvalidToken):
			status = http.StatusUnauthorized
			message = err.Error()
		}
		response.Error(c, status, message, nil)
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Logout(c *gin.Context) {
	claims, ok := c.Get("authClaims")
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	if err := h.service.Logout(c.Request.Context(), claims.(*platformjwt.Claims)); err != nil {
		response.Error(c, http.StatusInternalServerError, "logout failed", nil)
		return
	}

	parsedUserID := parseOptionalUserID(claims.(*platformjwt.Claims).UserID)
	h.audit(c, parsedUserID, "user.logout", http.StatusOK, "", nil)
	response.Success(c, http.StatusOK, gin.H{"message": "Logout successful"})
}

func (h *Handler) audit(c *gin.Context, actorID *uuid.UUID, action string, status int, errorMessage string, body map[string]any) {
	if h.userService == nil {
		return
	}

	var message *string
	if errorMessage != "" {
		message = &errorMessage
	}

	method := c.Request.Method
	path := c.FullPath()
	_ = h.userService.CreateAuditLog(c.Request.Context(), user.AuditInput{
		ActorID:        actorID,
		Action:         action,
		RequestMethod:  &method,
		RequestPath:    &path,
		ResponseStatus: status,
		ErrorMessage:   message,
		RequestBody:    body,
	})
}

func parseOptionalUserID(raw string) *uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}

	return &parsed
}
