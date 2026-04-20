package taskscorer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/user"
	platformmail "skilljudge/backend/internal/platform/mail"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const invitationTTL = 7 * 24 * time.Hour

const (
	deliveryModeEmailAndInApp = "email_and_in_app"
	deliveryModeInApp         = "in_app"
)

type Service struct {
	repo          *Repository
	taskService   *task.Service
	users         *user.Repository
	mailer        platformmail.Sender
	inviteBaseURL string
}

func NewService(repo *Repository, taskService *task.Service, users *user.Repository, mailer platformmail.Sender, inviteBaseURL string) *Service {
	return &Service{
		repo:          repo,
		taskService:   taskService,
		users:         users,
		mailer:        mailer,
		inviteBaseURL: strings.TrimRight(strings.TrimSpace(inviteBaseURL), "/"),
	}
}

func (s *Service) List(ctx context.Context, actor user.UserContext, taskID uuid.UUID) (*ListTaskScorersResult, error) {
	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, taskID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListTaskScorers(ctx, resolvedTask.TaskID)
	if err != nil {
		return nil, err
	}

	scorerIDs := make([]uuid.UUID, 0, len(items))
	for i := range items {
		scorerIDs = append(scorerIDs, items[i].ScorerID)
	}
	latestInvitations, err := s.repo.FindLatestInvitationsByTaskAndScorerIDs(ctx, resolvedTask.TaskID, scorerIDs)
	if err != nil {
		return nil, err
	}

	result := make([]TaskScorerDTO, 0, len(items))
	for i := range items {
		item := items[i]
		dto := TaskScorerDTO{
			TaskScorerID: item.ID,
			ScorerID:     item.ScorerID,
			Status:       item.Status,
			InvitedAt:    item.InvitedAt,
			AcceptedAt:   item.AcceptedAt,
			RemovedAt:    item.RemovedAt,
		}
		if item.Scorer != nil {
			dto.Username = item.Scorer.Username
			dto.RealName = item.Scorer.RealName
			dto.Email = item.Scorer.Email
		}
		if latest := latestInvitations[item.ScorerID]; latest != nil {
			dto.LastInvitation = &LastInvitationDTO{
				InvitationID: latest.ID,
				Status:       latest.Status,
				SentAt:       latest.SentAt,
				ExpiresAt:    latest.ExpiresAt,
				RespondedAt:  latest.RespondedAt,
				DeliveryMode: invitationDeliveryMode(latest.Metadata),
			}
		}
		result = append(result, dto)
	}

	return &ListTaskScorersResult{Items: result}, nil
}

func (s *Service) ListMine(ctx context.Context, actor user.UserContext) (*ListMyTaskScorerNotificationsResult, error) {
	if actor.Role != "scorer" {
		return nil, ErrTaskScorerNotificationRole
	}

	rows, err := s.repo.ListLatestInvitationsByScorer(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}

	result := make([]MyTaskScorerNotificationDTO, 0, len(rows))
	unreadCount := 0
	for i := range rows {
		row := rows[i]
		isRead, readAt := invitationReadState(row.Metadata)
		if row.Status == "sent" && !isRead {
			unreadCount++
		}
		result = append(result, MyTaskScorerNotificationDTO{
			InvitationID: row.ID,
			TaskID:       row.TaskID,
			TaskName:     row.TaskName,
			ProjectID:    row.ProjectID,
			ProjectName:  row.ProjectName,
			SchoolName:   row.SchoolName,
			Status:       row.Status,
			IsRead:       isRead,
			ReadAt:       readAt,
			SentAt:       row.SentAt,
			ExpiresAt:    row.ExpiresAt,
			RespondedAt:  row.RespondedAt,
			DeliveryMode: invitationDeliveryMode(row.Metadata),
			CreatedAt:    row.CreatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Status == "sent" && result[j].Status != "sent" {
			return true
		}
		if result[i].Status != "sent" && result[j].Status == "sent" {
			return false
		}
		return result[i].SentAt.After(result[j].SentAt)
	})

	return &ListMyTaskScorerNotificationsResult{
		Items:       result,
		UnreadCount: unreadCount,
	}, nil
}

func (s *Service) MarkMineRead(ctx context.Context, actor user.UserContext, invitationID uuid.UUID) error {
	if actor.Role != "scorer" {
		return ErrTaskScorerNotificationRole
	}

	invitation, err := s.repo.FindInvitationByIDAndScorer(ctx, invitationID, actor.UserID)
	if err != nil {
		return err
	}
	if invitation == nil {
		return ErrTaskScorerInvitationNotFound
	}

	metadata := markInvitationReadMetadata(invitation.Metadata, time.Now())
	return s.repo.UpdateInvitation(ctx, invitation.ID, map[string]any{
		"metadata":   metadata,
		"updated_at": time.Now(),
	})
}

func (s *Service) AcceptMine(ctx context.Context, actor user.UserContext, invitationID uuid.UUID) (*AcceptTaskScorerInvitationResult, error) {
	if actor.Role != "scorer" {
		return nil, ErrTaskScorerNotificationRole
	}

	invitation, err := s.repo.FindInvitationByIDAndScorer(ctx, invitationID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if invitation == nil {
		return nil, ErrTaskScorerInvitationNotFound
	}

	return s.acceptInvitation(ctx, invitation)
}

func (s *Service) Provision(ctx context.Context, actor user.UserContext, input InviteTaskScorerInput) (*ProvisionTaskScorerResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTaskScorerNameRequired
	}

	email, err := normalizeInviteEmail(input.Email)
	if err != nil {
		return nil, err
	}
	if actor.SchoolID == nil {
		return nil, ErrTaskScorerSchoolRequired
	}

	existingScorer, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existingScorer == nil {
		if err := s.ensureMailAvailable(); err != nil {
			return nil, err
		}
	}

	scorerUser, userCreated, temporaryPassword, err := s.findOrCreateScorer(ctx, actor, *actor.SchoolID, name, email)
	if err != nil {
		return nil, err
	}

	emailSent := false
	if userCreated {
		if err := s.sendProvisionEmail(ctx, scorerUser, temporaryPassword); err != nil {
			return nil, err
		}
		emailSent = true
	}

	return &ProvisionTaskScorerResult{
		ScorerID:    scorerUser.ID,
		Username:    scorerUser.Username,
		RealName:    scorerUser.RealName,
		Email:       scorerUser.Email,
		UserCreated: userCreated,
		EmailSent:   emailSent,
	}, nil
}

func (s *Service) Invite(ctx context.Context, actor user.UserContext, taskID uuid.UUID, input InviteTaskScorerInput) (*InviteTaskScorerResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTaskScorerNameRequired
	}

	email, err := normalizeInviteEmail(input.Email)
	if err != nil {
		return nil, err
	}

	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, taskID)
	if err != nil {
		return nil, err
	}
	if resolvedTask.SchoolID == nil {
		return nil, ErrTaskScorerSchoolRequired
	}
	existingScorer, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existingScorer == nil {
		if err := s.ensureMailAvailable(); err != nil {
			return nil, err
		}
	}

	scorerUser, userCreated, temporaryPassword, err := s.findOrCreateScorer(ctx, actor, *resolvedTask.SchoolID, name, email)
	if err != nil {
		return nil, err
	}

	taskScorer, relationCreated, err := s.findOrCreateTaskScorer(ctx, actor, resolvedTask.TaskID, scorerUser.ID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	rawToken, tokenHash, err := newInvitationToken()
	if err != nil {
		return nil, err
	}
	deliveryMode := deliveryModeInApp
	if userCreated {
		deliveryMode = deliveryModeEmailAndInApp
	}

	invitation := &model.TaskScorerInvitation{
		TaskID:           resolvedTask.TaskID,
		ScorerID:         scorerUser.ID,
		EmailSnapshot:    email,
		RealNameSnapshot: stringPtr(name),
		TokenHash:        tokenHash,
		Status:           "sent",
		SentAt:           now,
		ExpiresAt:        now.Add(invitationTTL),
		CreatedBy:        &actor.UserID,
		Metadata: map[string]any{
			"mailPending":   userCreated,
			"deliveryMode":  deliveryMode,
			"inAppRead":     false,
			"inAppReadAt":   nil,
			"createdByFlow": "teacher_invite",
		},
	}
	if err := s.repo.CreateInvitation(ctx, invitation); err != nil {
		return nil, err
	}
	if userCreated {
		if err := s.sendInvitationEmail(ctx, resolvedTask, scorerUser, invitation, rawToken, temporaryPassword); err != nil {
			return nil, err
		}
	} else {
		if err := s.updateInvitationInAppMetadata(ctx, invitation); err != nil {
			return nil, err
		}
	}

	return &InviteTaskScorerResult{
		ScorerID:            scorerUser.ID,
		UserCreated:         userCreated,
		TaskRelationCreated: relationCreated,
		TaskRelationStatus:  taskScorer.Status,
		InvitationID:        invitation.ID,
		InvitationStatus:    invitation.Status,
		DeliveryMode:        deliveryMode,
	}, nil
}

func (s *Service) Notify(ctx context.Context, actor user.UserContext, taskID, scorerID uuid.UUID) (*NotifyTaskScorerResult, error) {
	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, taskID)
	if err != nil {
		return nil, err
	}
	if resolvedTask.SchoolID == nil {
		return nil, ErrTaskScorerSchoolRequired
	}

	scorerUser, err := s.findExistingScorerByIDForSchool(ctx, scorerID, *resolvedTask.SchoolID)
	if err != nil {
		return nil, err
	}

	taskScorer, relationCreated, err := s.findOrCreateTaskScorer(ctx, actor, resolvedTask.TaskID, scorerUser.ID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	_, tokenHash, err := newInvitationToken()
	if err != nil {
		return nil, err
	}
	emailSnapshot := ""
	if scorerUser.Email != nil {
		emailSnapshot = strings.ToLower(strings.TrimSpace(*scorerUser.Email))
	}
	invitation := &model.TaskScorerInvitation{
		TaskID:           resolvedTask.TaskID,
		ScorerID:         scorerUser.ID,
		EmailSnapshot:    emailSnapshot,
		RealNameSnapshot: scorerUser.RealName,
		TokenHash:        tokenHash,
		Status:           "sent",
		SentAt:           now,
		ExpiresAt:        now.Add(invitationTTL),
		CreatedBy:        &actor.UserID,
		Metadata: map[string]any{
			"mailPending":   false,
			"deliveryMode":  deliveryModeInApp,
			"inAppRead":     false,
			"inAppReadAt":   nil,
			"createdByFlow": "teacher_task_notify",
		},
	}
	if err := s.repo.CreateInvitation(ctx, invitation); err != nil {
		return nil, err
	}
	if err := s.updateInvitationInAppMetadata(ctx, invitation); err != nil {
		return nil, err
	}

	return &NotifyTaskScorerResult{
		ScorerID:            scorerUser.ID,
		TaskRelationCreated: relationCreated,
		TaskRelationStatus:  taskScorer.Status,
		InvitationID:        invitation.ID,
		InvitationStatus:    invitation.Status,
		DeliveryMode:        deliveryModeInApp,
	}, nil
}

func (s *Service) Resend(ctx context.Context, actor user.UserContext, taskID, scorerID uuid.UUID) (*ResendTaskScorerInvitationResult, error) {
	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, taskID)
	if err != nil {
		return nil, err
	}

	taskScorer, err := s.repo.FindTaskScorer(ctx, resolvedTask.TaskID, scorerID)
	if err != nil {
		return nil, err
	}
	if taskScorer == nil {
		return nil, ErrTaskScorerRelationNotFound
	}
	if taskScorer.Status == "inactive" {
		return nil, ErrTaskScorerRelationInactive
	}
	if taskScorer.Scorer == nil {
		found, err := s.users.FindByID(ctx, scorerID)
		if err != nil {
			return nil, err
		}
		if found == nil {
			return nil, user.ErrUserNotFound
		}
		taskScorer.Scorer = found
	}
	if taskScorer.Scorer.Email == nil || strings.TrimSpace(*taskScorer.Scorer.Email) == "" {
		return nil, ErrTaskScorerEmailRequired
	}

	now := time.Now()
	deliveryMode := deliveryModeInApp
	var temporaryPassword *string
	if userCredentialPendingDelivery(taskScorer.Scorer) {
		if err := s.ensureMailAvailable(); err != nil {
			return nil, err
		}
		temporaryPassword, err = s.rotateCredentialForPendingDelivery(ctx, taskScorer.Scorer)
		if err != nil {
			return nil, err
		}
		deliveryMode = deliveryModeEmailAndInApp
	}
	rawToken, tokenHash, err := newInvitationToken()
	if err != nil {
		return nil, err
	}
	invitation := &model.TaskScorerInvitation{
		TaskID:           resolvedTask.TaskID,
		ScorerID:         scorerID,
		EmailSnapshot:    strings.ToLower(strings.TrimSpace(*taskScorer.Scorer.Email)),
		RealNameSnapshot: taskScorer.Scorer.RealName,
		TokenHash:        tokenHash,
		Status:           "sent",
		SentAt:           now,
		ExpiresAt:        now.Add(invitationTTL),
		CreatedBy:        &actor.UserID,
		Metadata: map[string]any{
			"mailPending":   temporaryPassword != nil,
			"deliveryMode":  deliveryMode,
			"inAppRead":     false,
			"inAppReadAt":   nil,
			"resend":        true,
			"createdByFlow": "teacher_resend",
		},
	}
	if err := s.repo.CreateInvitation(ctx, invitation); err != nil {
		return nil, err
	}

	if taskScorer.Status != "accepted" {
		if err := s.repo.UpdateTaskScorer(ctx, taskScorer.ID, map[string]any{
			"status":      "pending",
			"invited_by":  actor.UserID,
			"invited_at":  now,
			"accepted_at": nil,
			"updated_at":  now,
		}); err != nil {
			return nil, err
		}
	}
	if temporaryPassword != nil {
		if err := s.sendInvitationEmail(ctx, resolvedTask, taskScorer.Scorer, invitation, rawToken, temporaryPassword); err != nil {
			return nil, err
		}
	} else {
		if err := s.updateInvitationInAppMetadata(ctx, invitation); err != nil {
			return nil, err
		}
	}

	return &ResendTaskScorerInvitationResult{
		InvitationID:     invitation.ID,
		InvitationStatus: invitation.Status,
		DeliveryMode:     deliveryMode,
	}, nil
}

func (s *Service) Remove(ctx context.Context, actor user.UserContext, taskID, scorerID uuid.UUID) (*RemoveTaskScorerResult, error) {
	resolvedTask, err := s.taskService.ResolveManageable(ctx, actor, taskID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.DeactivateTaskScorer(ctx, resolvedTask.TaskID, scorerID, time.Now()); err != nil {
		return nil, err
	}

	return &RemoveTaskScorerResult{
		TaskID:   resolvedTask.TaskID,
		ScorerID: scorerID,
		Status:   "inactive",
	}, nil
}

func (s *Service) Accept(ctx context.Context, token string) (*AcceptTaskScorerInvitationResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrTaskScorerInvitationInvalid
	}

	tokenHash := hashInvitationToken(token)
	invitation, err := s.repo.FindInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if invitation == nil {
		return nil, ErrTaskScorerInvitationInvalid
	}

	return s.acceptInvitation(ctx, invitation)
}

func (s *Service) findExistingScorerByIDForSchool(ctx context.Context, scorerID, schoolID uuid.UUID) (*model.User, error) {
	existing, err := s.users.FindByID(ctx, scorerID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, user.ErrUserNotFound
	}
	if existing.Status != "active" {
		return nil, ErrTaskScorerInactiveUser
	}
	if existing.Role != "scorer" {
		return nil, ErrTaskScorerRoleInvalid
	}
	if existing.SchoolID == nil || *existing.SchoolID != schoolID {
		return nil, ErrTaskScorerAlreadyInOtherSchool
	}
	return existing, nil
}

func (s *Service) findOrCreateScorer(ctx context.Context, actor user.UserContext, schoolID uuid.UUID, realName, email string) (*model.User, bool, *string, error) {
	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, false, nil, err
	}
	if existing != nil {
		if existing.Status != "active" {
			return nil, false, nil, ErrTaskScorerInactiveUser
		}
		if existing.Role != "scorer" {
			return nil, false, nil, ErrTaskScorerRoleInvalid
		}
		if existing.SchoolID == nil || *existing.SchoolID != schoolID {
			return nil, false, nil, ErrTaskScorerAlreadyInOtherSchool
		}
		return existing, false, nil, nil
	}

	role, err := s.users.FindRoleByCode(ctx, "scorer")
	if err != nil {
		return nil, false, nil, err
	}
	if role == nil {
		return nil, false, nil, user.ErrRoleNotFound
	}

	username, err := s.generateUniqueUsername(ctx, schoolID)
	if err != nil {
		return nil, false, nil, err
	}
	password, err := newRandomCredential(18)
	if err != nil {
		return nil, false, nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, nil, fmt.Errorf("hash password: %w", err)
	}

	scorerUser := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		Email:        stringPtr(email),
		RealName:     stringPtr(realName),
		Role:         "scorer",
		Status:       "active",
		SchoolID:     &schoolID,
		Metadata: map[string]any{
			"provisionedBy":   "taskscorer",
			"credentialReady": false,
		},
	}
	if err := s.users.Create(ctx, scorerUser, role.ID, &actor.UserID); err != nil {
		return nil, false, nil, err
	}

	created, err := s.users.FindByID(ctx, scorerUser.ID)
	if err != nil {
		return nil, false, nil, err
	}
	if created == nil {
		return nil, false, nil, user.ErrUserNotFound
	}

	return created, true, stringPtr(password), nil
}

func (s *Service) findOrCreateTaskScorer(ctx context.Context, actor user.UserContext, taskID, scorerID uuid.UUID) (*model.TaskScorer, bool, error) {
	now := time.Now()
	existing, err := s.repo.FindTaskScorer(ctx, taskID, scorerID)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		item := &model.TaskScorer{
			TaskID:    taskID,
			ScorerID:  scorerID,
			Status:    "pending",
			InvitedBy: &actor.UserID,
			InvitedAt: &now,
			Metadata: map[string]any{
				"mailPending": true,
			},
		}
		if err := s.repo.CreateTaskScorer(ctx, item); err != nil {
			return nil, false, err
		}
		return item, true, nil
	}

	if existing.Status != "accepted" {
		updates := map[string]any{
			"status":      "pending",
			"invited_by":  actor.UserID,
			"invited_at":  now,
			"accepted_at": nil,
			"removed_at":  nil,
			"updated_at":  now,
			"metadata": map[string]any{
				"mailPending": true,
			},
		}
		if err := s.repo.UpdateTaskScorer(ctx, existing.ID, updates); err != nil {
			return nil, false, err
		}
		existing.Status = "pending"
		existing.InvitedBy = &actor.UserID
		existing.InvitedAt = &now
		existing.AcceptedAt = nil
		existing.RemovedAt = nil
	}

	return existing, false, nil
}

func (s *Service) generateUniqueUsername(ctx context.Context, schoolID uuid.UUID) (string, error) {
	prefix := fmt.Sprintf("scorer_%s_", strings.ToLower(schoolID.String()[:8]))
	for i := 0; i < 10; i++ {
		suffix, err := newRandomCredential(6)
		if err != nil {
			return "", err
		}
		candidate := prefix + strings.ToLower(suffix)
		existing, err := s.users.FindByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("generate unique username failed")
}

func normalizeInviteEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrTaskScorerEmailRequired
	}

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", ErrTaskScorerEmailInvalid
	}

	normalized := strings.ToLower(strings.TrimSpace(addr.Address))
	if normalized == "" {
		return "", ErrTaskScorerEmailInvalid
	}

	return normalized, nil
}

func (s *Service) acceptInvitation(ctx context.Context, invitation *model.TaskScorerInvitation) (*AcceptTaskScorerInvitationResult, error) {
	if invitation == nil {
		return nil, ErrTaskScorerInvitationInvalid
	}
	if invitation.Status == "cancelled" {
		return nil, ErrTaskScorerInvitationInvalid
	}

	now := time.Now()
	if invitation.ExpiresAt.Before(now) {
		_ = s.repo.UpdateInvitation(ctx, invitation.ID, map[string]any{
			"status":     "expired",
			"updated_at": now,
		})
		return nil, ErrTaskScorerInvitationExpired
	}

	taskScorer, err := s.repo.FindTaskScorer(ctx, invitation.TaskID, invitation.ScorerID)
	if err != nil {
		return nil, err
	}
	if taskScorer == nil {
		return nil, ErrTaskScorerRelationNotFound
	}
	if taskScorer.Status == "inactive" {
		return nil, ErrTaskScorerRelationInactive
	}
	if invitation.Status == "accepted" || taskScorer.Status == "accepted" {
		acceptedAt := taskScorer.AcceptedAt
		if acceptedAt == nil {
			acceptedAt = invitation.RespondedAt
		}
		return &AcceptTaskScorerInvitationResult{
			TaskID:             invitation.TaskID,
			ScorerID:           invitation.ScorerID,
			TaskRelationStatus: "accepted",
			AcceptedAt:         acceptedAt,
		}, nil
	}

	metadata := markInvitationReadMetadata(invitation.Metadata, now)
	if err := s.repo.UpdateInvitation(ctx, invitation.ID, map[string]any{
		"status":       "accepted",
		"responded_at": now,
		"metadata":     metadata,
		"updated_at":   now,
	}); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTaskScorer(ctx, taskScorer.ID, map[string]any{
		"status":      "accepted",
		"accepted_at": now,
		"removed_at":  nil,
		"updated_at":  now,
	}); err != nil {
		return nil, err
	}

	return &AcceptTaskScorerInvitationResult{
		TaskID:             invitation.TaskID,
		ScorerID:           invitation.ScorerID,
		TaskRelationStatus: "accepted",
		AcceptedAt:         &now,
	}, nil
}

func (s *Service) ensureMailAvailable() error {
	if s.mailer == nil || !s.mailer.Enabled() || s.inviteBaseURL == "" {
		return ErrTaskScorerMailUnavailable
	}
	return nil
}

func (s *Service) updateInvitationInAppMetadata(ctx context.Context, invitation *model.TaskScorerInvitation) error {
	if invitation == nil {
		return nil
	}
	metadata := copyMetadata(invitation.Metadata)
	metadata["mailPending"] = false
	metadata["mailStatus"] = "not_applicable"
	metadata["inAppUpdatedAt"] = time.Now().UTC().Format(time.RFC3339)
	invitation.Metadata = metadata
	return s.repo.UpdateInvitation(ctx, invitation.ID, map[string]any{
		"metadata":   metadata,
		"updated_at": time.Now(),
	})
}

func (s *Service) sendProvisionEmail(ctx context.Context, scorerUser *model.User, temporaryPassword *string) error {
	if scorerUser == nil || scorerUser.Email == nil || strings.TrimSpace(*scorerUser.Email) == "" {
		return ErrTaskScorerEmailRequired
	}
	if temporaryPassword == nil {
		return nil
	}

	message := platformmail.Message{
		To: []platformmail.Address{{
			Name:  displayNameForUser(scorerUser),
			Email: *scorerUser.Email,
		}},
		Subject:  "【SkillJudge】评分员账号已开通",
		TextBody: buildProvisionBody(s.inviteBaseURL, scorerUser, *temporaryPassword),
	}
	if err := s.mailer.Send(ctx, message); err != nil {
		return fmt.Errorf("%w: %v", ErrTaskScorerInvitationSendFailed, err)
	}
	return s.markCredentialDelivered(ctx, scorerUser)
}

func (s *Service) sendInvitationEmail(ctx context.Context, resolvedTask *task.Context, scorerUser *model.User, invitation *model.TaskScorerInvitation, rawToken string, temporaryPassword *string) error {
	if scorerUser.Email == nil || strings.TrimSpace(*scorerUser.Email) == "" {
		return ErrTaskScorerEmailRequired
	}

	acceptURL, err := s.buildAcceptURL(rawToken)
	if err != nil {
		return err
	}
	message := platformmail.Message{
		To: []platformmail.Address{{
			Name:  displayNameForUser(scorerUser),
			Email: *scorerUser.Email,
		}},
		Subject:  fmt.Sprintf("【SkillJudge】评分邀请：%s", resolvedTask.TaskName),
		TextBody: buildInvitationBody(resolvedTask, scorerUser, invitation, acceptURL, temporaryPassword),
	}

	sendErr := s.mailer.Send(ctx, message)
	if updateErr := s.updateInvitationMailMetadata(ctx, invitation, acceptURL, sendErr); updateErr != nil && sendErr == nil {
		sendErr = updateErr
	}
	if sendErr != nil {
		return fmt.Errorf("%w: %v", ErrTaskScorerInvitationSendFailed, sendErr)
	}

	if temporaryPassword != nil {
		if err := s.markCredentialDelivered(ctx, scorerUser); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) rotateCredentialForPendingDelivery(ctx context.Context, scorerUser *model.User) (*string, error) {
	if scorerUser == nil || !userCredentialPendingDelivery(scorerUser) {
		return nil, nil
	}

	password, err := newRandomCredential(18)
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	metadata := copyMetadata(scorerUser.Metadata)
	metadata["credentialReady"] = false
	metadata["credentialRefreshedAt"] = time.Now().UTC().Format(time.RFC3339)
	if err := s.users.UpdateProfile(ctx, scorerUser.ID, map[string]any{
		"password_hash": string(hash),
		"metadata":      metadata,
		"updated_at":    time.Now(),
	}); err != nil {
		return nil, err
	}

	scorerUser.Metadata = metadata
	return stringPtr(password), nil
}

func (s *Service) markCredentialDelivered(ctx context.Context, scorerUser *model.User) error {
	if scorerUser == nil {
		return nil
	}
	metadata := copyMetadata(scorerUser.Metadata)
	metadata["credentialReady"] = true
	metadata["credentialDeliveredAt"] = time.Now().UTC().Format(time.RFC3339)
	return s.users.UpdateProfile(ctx, scorerUser.ID, map[string]any{
		"metadata":   metadata,
		"updated_at": time.Now(),
	})
}

func (s *Service) updateInvitationMailMetadata(ctx context.Context, invitation *model.TaskScorerInvitation, acceptURL string, sendErr error) error {
	if invitation == nil {
		return nil
	}

	metadata := copyMetadata(invitation.Metadata)
	metadata["mailPending"] = false
	metadata["acceptURL"] = acceptURL
	metadata["mailUpdatedAt"] = time.Now().UTC().Format(time.RFC3339)
	if sendErr != nil {
		metadata["mailStatus"] = "failed"
		metadata["mailError"] = sendErr.Error()
	} else {
		metadata["mailStatus"] = "sent"
		delete(metadata, "mailError")
	}
	invitation.Metadata = metadata

	return s.repo.UpdateInvitation(ctx, invitation.ID, map[string]any{
		"metadata":   metadata,
		"updated_at": time.Now(),
	})
}

func invitationDeliveryMode(metadata map[string]any) string {
	if metadata == nil {
		return deliveryModeInApp
	}
	value, ok := metadata["deliveryMode"]
	if !ok {
		return deliveryModeInApp
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return deliveryModeInApp
	}
	return strings.TrimSpace(text)
}

func invitationReadState(metadata map[string]any) (bool, *time.Time) {
	if metadata == nil {
		return false, nil
	}
	value, ok := metadata["inAppRead"]
	if !ok {
		return false, nil
	}
	isRead, ok := value.(bool)
	if !ok || !isRead {
		return false, nil
	}
	readAtValue, ok := metadata["inAppReadAt"]
	if !ok || readAtValue == nil {
		return true, nil
	}
	readAtText, ok := readAtValue.(string)
	if !ok || strings.TrimSpace(readAtText) == "" {
		return true, nil
	}
	parsed, err := time.Parse(time.RFC3339, readAtText)
	if err != nil {
		return true, nil
	}
	return true, &parsed
}

func markInvitationReadMetadata(metadata map[string]any, now time.Time) map[string]any {
	next := copyMetadata(metadata)
	next["inAppRead"] = true
	next["inAppReadAt"] = now.UTC().Format(time.RFC3339)
	return next
}

func (s *Service) buildAcceptURL(rawToken string) (string, error) {
	base, err := url.Parse(s.inviteBaseURL)
	if err != nil {
		return "", fmt.Errorf("invite base url is invalid: %w", err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/task-scorer-invitations/" + rawToken + "/accept"
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func buildInvitationBody(resolvedTask *task.Context, scorerUser *model.User, invitation *model.TaskScorerInvitation, acceptURL string, temporaryPassword *string) string {
	lines := []string{
		fmt.Sprintf("您好，%s：", displayNameForUser(scorerUser)),
		"",
		"您已被邀请参与 SkillJudge 的任务评分。",
	}
	if resolvedTask != nil {
		if resolvedTask.SchoolName != nil && strings.TrimSpace(*resolvedTask.SchoolName) != "" {
			lines = append(lines, "学校："+strings.TrimSpace(*resolvedTask.SchoolName))
		}
		if strings.TrimSpace(resolvedTask.ProjectName) != "" {
			lines = append(lines, "项目："+resolvedTask.ProjectName)
		}
		if strings.TrimSpace(resolvedTask.TaskName) != "" {
			lines = append(lines, "任务："+resolvedTask.TaskName)
		}
	}
	lines = append(lines, "", "请点击以下链接确认参与：", acceptURL)
	if invitation != nil && !invitation.ExpiresAt.IsZero() {
		lines = append(lines, "链接有效期至："+invitation.ExpiresAt.Local().Format("2006-01-02 15:04:05"))
	}
	lines = append(lines, "")
	if scorerUser != nil {
		lines = append(lines, "系统账号："+scorerUser.Username)
	}
	if temporaryPassword != nil {
		lines = append(lines,
			"临时密码："+*temporaryPassword,
			"这是系统首次为您创建账号，请在登录后尽快修改密码。",
		)
	} else {
		lines = append(lines, "本次邀请复用了您已有的评分员账号，不会重复创建账号。")
	}
	lines = append(lines, "", "此邮件由系统自动发送，请勿直接回复。")

	return strings.Join(lines, "\n")
}

func buildProvisionBody(portalURL string, scorerUser *model.User, temporaryPassword string) string {
	lines := []string{
		fmt.Sprintf("您好，%s：", displayNameForUser(scorerUser)),
		"",
		"SkillJudge 已为您开通评分员账号。",
	}
	if strings.TrimSpace(portalURL) != "" {
		lines = append(lines, "登录入口："+strings.TrimSpace(portalURL))
	}
	if scorerUser != nil {
		lines = append(lines, "系统账号："+scorerUser.Username)
	}
	lines = append(lines,
		"临时密码："+temporaryPassword,
		"首次登录后，请先在系统内确认相关评分任务，并尽快修改密码。",
		"",
		"此邮件由系统自动发送，请勿直接回复。",
	)
	return strings.Join(lines, "\n")
}

func displayNameForUser(item *model.User) string {
	if item == nil {
		return "评分员"
	}
	if item.RealName != nil && strings.TrimSpace(*item.RealName) != "" {
		return strings.TrimSpace(*item.RealName)
	}
	if strings.TrimSpace(item.Username) != "" {
		return item.Username
	}
	return "评分员"
}

func userCredentialPendingDelivery(item *model.User) bool {
	if item == nil || item.Metadata == nil {
		return false
	}
	value, ok := item.Metadata["credentialReady"]
	if !ok {
		return false
	}
	ready, ok := value.(bool)
	return ok && !ready
}

func copyMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func newInvitationToken() (string, string, error) {
	tokenBytes, err := newRandomBytes(32)
	if err != nil {
		return "", "", err
	}

	token := hex.EncodeToString(tokenBytes)
	return token, hashInvitationToken(token), nil
}

func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newRandomCredential(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if length <= 0 {
		length = 16
	}

	bytes, err := newRandomBytes(length)
	if err != nil {
		return "", err
	}

	out := make([]byte, length)
	for i := range bytes {
		out[i] = alphabet[int(bytes[i])%len(alphabet)]
	}

	return string(out), nil
}

func newRandomBytes(length int) ([]byte, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}

	return buf, nil
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
