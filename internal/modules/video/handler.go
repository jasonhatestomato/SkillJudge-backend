package video

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/project"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/platform/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type createUploadCredentialRequest struct {
	TaskID        string  `json:"taskId" binding:"required"`
	Filename      string  `json:"filename" binding:"required"`
	FileSize      int64   `json:"fileSize" binding:"required"`
	StudentID     *string `json:"studentId"`
	StudentName   string  `json:"studentName" binding:"required"`
	StudentNumber string  `json:"studentNumber" binding:"required"`
}

type confirmUploadRequest struct {
	UploadID string                 `json:"uploadId" binding:"required"`
	Parts    []storage.UploadedPart `json:"parts" binding:"required"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateUploadCredential(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	var req createUploadCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid taskId", nil)
		return
	}

	input := CreateUploadCredentialInput{
		TaskID:        taskID,
		Filename:      req.Filename,
		FileSize:      req.FileSize,
		StudentName:   req.StudentName,
		StudentNumber: req.StudentNumber,
	}
	if req.StudentID != nil && *req.StudentID != "" {
		studentID, err := uuid.Parse(*req.StudentID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid studentId", nil)
			return
		}
		input.StudentID = &studentID
	}

	result, err := h.service.CreateUploadCredential(c.Request.Context(), actor, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoTaskRequired), errors.Is(err, ErrVideoFilenameRequired), errors.Is(err, ErrVideoFileSizeInvalid), errors.Is(err, ErrVideoStudentNameRequired), errors.Is(err, ErrVideoStudentNumberRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, project.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed), errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			log.Printf("video.CreateUploadCredential failed: actor=%s task=%s filename=%q err=%v", actor.UserID, taskID, req.Filename, err)
			response.Error(c, http.StatusInternalServerError, "failed to create upload credential", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ConfirmUpload(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	var req confirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.ConfirmUpload(c.Request.Context(), actor, videoID, ConfirmUploadInput{
		UploadID: req.UploadID,
		Parts:    req.Parts,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoUploadIDRequired), errors.Is(err, ErrVideoPartsRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to confirm upload", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) List(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskIDRaw := c.Query("taskId")
	if taskIDRaw == "" {
		response.Error(c, http.StatusBadRequest, "taskId is required", nil)
		return
	}
	taskID, err := uuid.Parse(taskIDRaw)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid taskId", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := ListParams{
		TaskID:        &taskID,
		Page:          page,
		PageSize:      pageSize,
		Status:        c.Query("status"),
		StudentNumber: c.Query("studentNumber"),
		Keyword:       c.Query("keyword"),
	}
	if studentIDRaw := c.Query("studentId"); studentIDRaw != "" {
		studentID, err := uuid.Parse(studentIDRaw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid studentId", nil)
			return
		}
		params.StudentID = &studentID
	}

	result, err := h.service.List(c.Request.Context(), actor, params)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoTaskRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, project.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed), errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list videos", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), actor, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load video", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) ListMine(c *gin.Context) {
	actor := middleware.CurrentUser(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	result, err := h.service.ListMine(c.Request.Context(), actor, ListParams{
		Page:          page,
		PageSize:      pageSize,
		Status:        c.Query("status"),
		StudentNumber: c.Query("studentNumber"),
		Keyword:       c.Query("keyword"),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list my videos", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetMine(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	result, err := h.service.GetMineByID(c.Request.Context(), actor, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load my video", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Delete(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	if err := h.service.Delete(c.Request.Context(), actor, videoID); err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to delete video", nil)
		}
		return
	}

	response.NoContent(c)
}

func (h *Handler) GenerateAIReport(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	result, err := h.service.GenerateAIReport(c.Request.Context(), actor, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrVideoAIReportNotReady):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			log.Printf("video.GenerateAIReport failed: actor=%s video=%s err=%v", actor.UserID, videoID, err)
			response.Error(c, http.StatusInternalServerError, "failed to generate video ai report", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) GetAIReport(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	result, err := h.service.GetAIReport(c.Request.Context(), actor, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound), errors.Is(err, ErrVideoAIReportNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load video ai report", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) PreviewAIReport(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	html, err := h.service.RenderAIReportHTML(c.Request.Context(), actor, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrVideoAIReportNotReady):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to preview video ai report", nil)
		}
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func (h *Handler) DownloadAIReport(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	videoID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid video id", nil)
		return
	}

	pdfBytes, fileName, err := h.service.RenderAIReportPDF(c.Request.Context(), actor, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrVideoNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrVideoAIReportNotReady):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrInvalidVideoScope), errors.Is(err, ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to download video ai report", nil)
		}
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
