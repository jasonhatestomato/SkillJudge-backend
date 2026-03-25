package system

import (
	"context"
	"net/http"
	"time"

	"skilljudge/backend/internal/http/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Handler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHandler(db *gorm.DB, redisClient *redis.Client) *Handler {
	return &Handler{
		db:    db,
		redis: redisClient,
	}
}

func (h *Handler) Health(c *gin.Context) {
	response.Success(c, http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (h *Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := gin.H{}
	ready := true

	sqlDB, err := h.db.DB()
	if err != nil {
		ready = false
		checks["postgres"] = gin.H{
			"status": "down",
			"error":  err.Error(),
		}
	} else if err := sqlDB.PingContext(ctx); err != nil {
		ready = false
		checks["postgres"] = gin.H{
			"status": "down",
			"error":  err.Error(),
		}
	} else {
		checks["postgres"] = gin.H{
			"status": "up",
		}
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		ready = false
		checks["redis"] = gin.H{
			"status": "down",
			"error":  err.Error(),
		}
	} else {
		checks["redis"] = gin.H{
			"status": "up",
		}
	}

	payload := gin.H{
		"status": "ready",
		"checks": checks,
	}

	if !ready {
		payload["status"] = "not_ready"
		response.Error(c, http.StatusServiceUnavailable, "service dependencies not ready", payload)
		return
	}

	response.Success(c, http.StatusOK, payload)
}
