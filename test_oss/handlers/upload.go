package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	obs "github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"github.com/google/uuid"
	"test_oss/models"
	"test_oss/obsstore"
)

// --- Request / Response types ---

type InitUploadReq struct {
	FileName    string `json:"filename"     binding:"required"`
	FileSize    int64  `json:"filesize"     binding:"required"`
	TotalChunks int    `json:"total_chunks" binding:"required"`
}

type CompleteUploadReq struct {
	FileID string `json:"file_id" binding:"required"`
}

// --- Helpers ---

func objectKey(fileID, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return fmt.Sprintf("uploads/%s/%s%s", time.Now().Format("2006/01/02"), fileID, ext)
}

// --- Handlers ---

// POST /api/upload/init
// Returns file_id and OBS upload_id so the client can start sending chunks.
func InitUpload(c *gin.Context) {
	var req InitUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fileID := uuid.NewString()
	key := objectKey(fileID, req.FileName)

	obsUploadID, err := obsstore.InitiateMultipartUpload(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OBS initiate failed: " + err.Error()})
		return
	}

	session := models.UploadSession{
		FileID:      fileID,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		TotalChunks: req.TotalChunks,
		ObjectKey:   key,
		OBSUploadID: obsUploadID,
		Status:      "uploading",
	}
	session.SetParts(nil)

	if err := models.DB.Create(&session).Error; err != nil {
		_ = obsstore.AbortMultipartUpload(key, obsUploadID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id":      fileID,
		"upload_id":    obsUploadID,
		"object_key":   key,
		"chunk_size":   5 * 1024 * 1024, // advise 5 MB
	})
}

// POST /api/upload/chunk  (multipart/form-data)
// Fields: file_id, chunk_index (1-based), chunk (file)
func UploadChunk(c *gin.Context) {
	fileID := c.PostForm("file_id")
	chunkIndexStr := c.PostForm("chunk_index")
	if fileID == "" || chunkIndexStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id and chunk_index required"})
		return
	}

	var chunkIndex int
	if _, err := fmt.Sscanf(chunkIndexStr, "%d", &chunkIndex); err != nil || chunkIndex < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk_index must be a positive integer"})
		return
	}

	var session models.UploadSession
	if err := models.DB.Where("file_id = ?", fileID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "upload session not found"})
		return
	}
	if session.Status != "uploading" {
		c.JSON(http.StatusConflict, gin.H{"error": "upload session is not in uploading state"})
		return
	}

	file, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk file required: " + err.Error()})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open chunk"})
		return
	}
	defer f.Close()

	etag, err := obsstore.UploadPart(session.ObjectKey, session.OBSUploadID, chunkIndex, f, file.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Append the part info and increment counter.
	parts := session.GetParts()
	parts = append(parts, models.Part{PartNumber: chunkIndex, ETag: etag})
	session.SetParts(parts)
	session.UploadedChunks++
	if err := models.DB.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db save error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"chunk_index":     chunkIndex,
		"etag":            etag,
		"uploaded_chunks": session.UploadedChunks,
		"total_chunks":    session.TotalChunks,
	})
}

// POST /api/upload/complete
func CompleteUpload(c *gin.Context) {
	var req CompleteUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var session models.UploadSession
	if err := models.DB.Where("file_id = ?", req.FileID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "upload session not found"})
		return
	}
	if session.Status != "uploading" {
		c.JSON(http.StatusConflict, gin.H{"error": "already " + session.Status})
		return
	}

	// Build OBS parts slice
	modelParts := session.GetParts()
	obsParts := make([]obs.Part, len(modelParts))
	for i, p := range modelParts {
		obsParts[i] = obs.Part{PartNumber: p.PartNumber, ETag: p.ETag}
	}

	url, err := obsstore.CompleteMultipartUpload(session.ObjectKey, session.OBSUploadID, obsParts)
	if err != nil {
		session.Status = "failed"
		models.DB.Save(&session)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	session.Status = "completed"
	session.URL = url
	models.DB.Save(&session)

	c.JSON(http.StatusOK, gin.H{
		"file_id":    session.FileID,
		"filename":   session.FileName,
		"url":        url,
		"object_key": session.ObjectKey,
		"status":     "completed",
	})
}

// GET /api/upload/status/:fileId
func GetUploadStatus(c *gin.Context) {
	fileID := c.Param("fileId")
	var session models.UploadSession
	if err := models.DB.Where("file_id = ?", fileID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"file_id":         session.FileID,
		"filename":        session.FileName,
		"filesize":        session.FileSize,
		"status":          session.Status,
		"uploaded_chunks": session.UploadedChunks,
		"total_chunks":    session.TotalChunks,
		"url":             session.URL,
	})
}
