package taskscorer

import (
	"time"

	"github.com/google/uuid"
)

type LastInvitationDTO struct {
	InvitationID uuid.UUID  `json:"invitationId"`
	Status       string     `json:"status"`
	SentAt       time.Time  `json:"sentAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	RespondedAt  *time.Time `json:"respondedAt,omitempty"`
	DeliveryMode string     `json:"deliveryMode,omitempty"`
}

type TaskScorerDTO struct {
	TaskScorerID   uuid.UUID          `json:"taskScorerId"`
	ScorerID       uuid.UUID          `json:"scorerId"`
	Username       string             `json:"username"`
	RealName       *string            `json:"realName,omitempty"`
	Email          *string            `json:"email,omitempty"`
	Status         string             `json:"status"`
	InvitedAt      *time.Time         `json:"invitedAt,omitempty"`
	AcceptedAt     *time.Time         `json:"acceptedAt,omitempty"`
	RemovedAt      *time.Time         `json:"removedAt,omitempty"`
	LastInvitation *LastInvitationDTO `json:"lastInvitation,omitempty"`
}

type ListTaskScorersResult struct {
	Items []TaskScorerDTO `json:"items"`
}

type InviteTaskScorerInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ProvisionTaskScorerResult struct {
	ScorerID    uuid.UUID `json:"scorerId"`
	Username    string    `json:"username"`
	RealName    *string   `json:"realName,omitempty"`
	Email       *string   `json:"email,omitempty"`
	UserCreated bool      `json:"userCreated"`
	EmailSent   bool      `json:"emailSent"`
}

type InviteTaskScorerResult struct {
	ScorerID            uuid.UUID `json:"scorerId"`
	UserCreated         bool      `json:"userCreated"`
	TaskRelationCreated bool      `json:"taskRelationCreated"`
	TaskRelationStatus  string    `json:"taskRelationStatus"`
	InvitationID        uuid.UUID `json:"invitationId"`
	InvitationStatus    string    `json:"invitationStatus"`
	DeliveryMode        string    `json:"deliveryMode"`
}

type RemoveTaskScorerResult struct {
	TaskID   uuid.UUID `json:"taskId"`
	ScorerID uuid.UUID `json:"scorerId"`
	Status   string    `json:"status"`
}

type ResendTaskScorerInvitationResult struct {
	InvitationID     uuid.UUID `json:"invitationId"`
	InvitationStatus string    `json:"invitationStatus"`
	DeliveryMode     string    `json:"deliveryMode"`
}

type NotifyTaskScorerResult struct {
	ScorerID            uuid.UUID `json:"scorerId"`
	TaskRelationCreated bool      `json:"taskRelationCreated"`
	TaskRelationStatus  string    `json:"taskRelationStatus"`
	InvitationID        uuid.UUID `json:"invitationId"`
	InvitationStatus    string    `json:"invitationStatus"`
	DeliveryMode        string    `json:"deliveryMode"`
}

type AcceptTaskScorerInvitationResult struct {
	TaskID             uuid.UUID  `json:"taskId"`
	ScorerID           uuid.UUID  `json:"scorerId"`
	TaskRelationStatus string     `json:"taskRelationStatus"`
	AcceptedAt         *time.Time `json:"acceptedAt,omitempty"`
}

type MyTaskScorerNotificationDTO struct {
	InvitationID uuid.UUID  `json:"invitationId"`
	TaskID       uuid.UUID  `json:"taskId"`
	TaskName     string     `json:"taskName"`
	ProjectID    uuid.UUID  `json:"projectId"`
	ProjectName  string     `json:"projectName"`
	SchoolName   *string    `json:"schoolName,omitempty"`
	Status       string     `json:"status"`
	IsRead       bool       `json:"isRead"`
	ReadAt       *time.Time `json:"readAt,omitempty"`
	SentAt       time.Time  `json:"sentAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	RespondedAt  *time.Time `json:"respondedAt,omitempty"`
	DeliveryMode string     `json:"deliveryMode"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type ListMyTaskScorerNotificationsResult struct {
	Items       []MyTaskScorerNotificationDTO `json:"items"`
	UnreadCount int                           `json:"unreadCount"`
}
