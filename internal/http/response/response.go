package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Errors    any    `json:"errors,omitempty"`
	Timestamp string `json:"timestamp"`
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{
		Code:      status,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func Error(c *gin.Context, status int, message string, details any) {
	c.JSON(status, envelope{
		Code:      status,
		Message:   message,
		Errors:    details,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
