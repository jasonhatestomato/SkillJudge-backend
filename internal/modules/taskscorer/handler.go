package taskscorer

import (
	"errors"
	"net/http"

	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

type inviteTaskScorerRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListMine(c *gin.Context) {
	actor := middleware.CurrentUser(c)

	result, err := h.service.ListMine(c.Request.Context(), actor)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerNotificationRole):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list task scorer notifications", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) List(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	result, err := h.service.List(c.Request.Context(), actor, taskID)
	if err != nil {
		switch {
		case errors.Is(err, task.ErrTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to list task scorers", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Provision(c *gin.Context) {
	actor := middleware.CurrentUser(c)

	var req inviteTaskScorerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.Provision(c.Request.Context(), actor, InviteTaskScorerInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerNameRequired),
			errors.Is(err, ErrTaskScorerEmailRequired),
			errors.Is(err, ErrTaskScorerEmailInvalid),
			errors.Is(err, ErrTaskScorerSchoolRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerMailUnavailable):
			response.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		case errors.Is(err, user.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerRoleInvalid),
			errors.Is(err, ErrTaskScorerAlreadyInOtherSchool),
			errors.Is(err, user.ErrUsernameTaken):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationSendFailed):
			response.Error(c, http.StatusBadGateway, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInactiveUser):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to provision task scorer", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) Invite(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	var req inviteTaskScorerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	result, err := h.service.Invite(c.Request.Context(), actor, taskID, InviteTaskScorerInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerNameRequired),
			errors.Is(err, ErrTaskScorerEmailRequired),
			errors.Is(err, ErrTaskScorerEmailInvalid),
			errors.Is(err, ErrTaskScorerSchoolRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerMailUnavailable):
			response.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, user.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerRoleInvalid),
			errors.Is(err, ErrTaskScorerAlreadyInOtherSchool),
			errors.Is(err, user.ErrUsernameTaken):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationSendFailed):
			response.Error(c, http.StatusBadGateway, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInactiveUser),
			errors.Is(err, task.ErrTaskProjectScope),
			errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to invite task scorer", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) Notify(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	scorerID, err := uuid.Parse(c.Param("scorerId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid scorer id", nil)
		return
	}

	result, err := h.service.Notify(c.Request.Context(), actor, taskID, scorerID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerSchoolRequired):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, user.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerRoleInvalid), errors.Is(err, ErrTaskScorerAlreadyInOtherSchool):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInactiveUser), errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to notify task scorer", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) Remove(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	scorerID, err := uuid.Parse(c.Param("scorerId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid scorer id", nil)
		return
	}

	result, err := h.service.Remove(c.Request.Context(), actor, taskID, scorerID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerRelationNotFound), errors.Is(err, task.ErrTaskNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to remove task scorer", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Resend(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id", nil)
		return
	}

	scorerID, err := uuid.Parse(c.Param("scorerId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid scorer id", nil)
		return
	}

	result, err := h.service.Resend(c.Request.Context(), actor, taskID, scorerID)
	if err != nil {
		switch {
		case errors.Is(err, task.ErrTaskNotFound), errors.Is(err, ErrTaskScorerRelationNotFound), errors.Is(err, user.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerMailUnavailable):
			response.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerEmailRequired), errors.Is(err, ErrTaskScorerRelationInactive):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationSendFailed):
			response.Error(c, http.StatusBadGateway, err.Error(), nil)
		case errors.Is(err, task.ErrTaskProjectScope), errors.Is(err, task.ErrTaskRoleNotAllowed):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to resend task scorer invitation", nil)
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

func (h *Handler) Accept(c *gin.Context) {
	result, err := h.service.Accept(c.Request.Context(), c.Param("token"))
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerInvitationInvalid), errors.Is(err, ErrTaskScorerRelationInactive):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationExpired):
			response.Error(c, http.StatusConflict, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerRelationNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to accept task scorer invitation", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) MarkMineRead(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	invitationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid invitation id", nil)
		return
	}

	if err := h.service.MarkMineRead(c.Request.Context(), actor, invitationID); err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerNotificationRole):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to mark task scorer notification as read", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AcceptMine(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	invitationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid invitation id", nil)
		return
	}

	result, err := h.service.AcceptMine(c.Request.Context(), actor, invitationID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskScorerNotificationRole):
			response.Error(c, http.StatusForbidden, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationNotFound), errors.Is(err, ErrTaskScorerRelationNotFound):
			response.Error(c, http.StatusNotFound, err.Error(), nil)
		case errors.Is(err, ErrTaskScorerInvitationExpired), errors.Is(err, ErrTaskScorerInvitationInvalid), errors.Is(err, ErrTaskScorerRelationInactive):
			response.Error(c, http.StatusBadRequest, err.Error(), nil)
		default:
			response.Error(c, http.StatusInternalServerError, "failed to accept task scorer notification", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, result)
}

func (h *Handler) AcceptPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, acceptInvitationPage(c.Param("token")))
}

func acceptInvitationPage(token string) string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>确认评分邀请</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", sans-serif; background: #f7f8fa; color: #111827; margin: 0; }
    .wrap { max-width: 520px; margin: 12vh auto; padding: 24px; }
    .card { background: #fff; border-radius: 16px; padding: 28px; box-shadow: 0 12px 32px rgba(15, 23, 42, 0.08); }
    h1 { font-size: 24px; margin: 0 0 12px; }
    p { line-height: 1.7; color: #374151; }
    button { width: 100%; border: 0; border-radius: 10px; background: #111827; color: #fff; padding: 14px 18px; font-size: 16px; cursor: pointer; }
    button[disabled] { opacity: 0.7; cursor: progress; }
    .status { margin-top: 16px; font-size: 14px; }
    .ok { color: #047857; }
    .err { color: #b91c1c; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>确认参与评分</h1>
      <p>点击下方按钮后，系统会确认你参与当前任务的评分工作。</p>
      <button id="accept-btn" type="button">确认参与</button>
      <div id="status" class="status"></div>
    </div>
  </div>
  <script>
    const token = ` + "`" + token + "`" + `;
    const button = document.getElementById('accept-btn');
    const status = document.getElementById('status');
    button.addEventListener('click', async () => {
      button.disabled = true;
      status.className = 'status';
      status.textContent = '正在提交确认...';
      try {
        const res = await fetch('/api/v1/task-scorer-invitations/' + encodeURIComponent(token) + '/accept', {
          method: 'POST',
          headers: { 'Accept': 'application/json' }
        });
        const data = await res.json().catch(() => null);
        if (!res.ok) {
          const msg = data && data.message ? data.message : '确认失败，请稍后重试。';
          throw new Error(msg);
        }
        status.className = 'status ok';
        status.textContent = '确认成功，你已经加入该任务的评分员列表。';
        button.textContent = '已确认';
      } catch (err) {
        status.className = 'status err';
        status.textContent = err && err.message ? err.message : '确认失败，请稍后重试。';
        button.disabled = false;
      }
    });
  </script>
</body>
</html>`
}
