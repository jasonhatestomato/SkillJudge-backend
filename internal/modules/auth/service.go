package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"skilljudge/backend/internal/config"
	"skilljudge/backend/internal/model"
	"skilljudge/backend/internal/modules/user"
	platformjwt "skilljudge/backend/internal/platform/jwt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	users       *user.Repository
	userService *user.Service
	sessions    *SessionStore
	jwtManager  *platformjwt.Manager
	jwtConfig   config.JWTConfig
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshInput struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string        `json:"accessToken"`
	RefreshToken string        `json:"refreshToken"`
	ExpiresIn    int64         `json:"expiresIn"`
	User         *user.UserDTO `json:"user"`
}

func NewService(userRepo *user.Repository, userService *user.Service, sessions *SessionStore, jwtManager *platformjwt.Manager, jwtConfig config.JWTConfig) *Service {
	return &Service{
		users:       userRepo,
		userService: userService,
		sessions:    sessions,
		jwtManager:  jwtManager,
		jwtConfig:   jwtConfig,
	}
}

func (s *Service) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	if len(input.Username) < 3 || len(input.Password) < 1 {
		return nil, ErrInvalidCredentials
	}
	found, err := s.users.FindByUsername(ctx, input.Username)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, ErrInvalidCredentials
	}
	if found.Status != "active" {
		return nil, ErrForbidden
	}

	if err := bcrypt.CompareHashAndPassword([]byte(found.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	if err := s.users.UpdateProfile(ctx, found.ID, map[string]any{
		"last_login_at": now,
		"updated_at":    now,
	}); err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, found)
}

func (s *Service) Refresh(ctx context.Context, token string) (*AuthResponse, error) {
	claims, err := s.jwtManager.Parse(token)
	if err != nil || claims.Type != "refresh" {
		return nil, ErrInvalidToken
	}

	session, err := s.sessions.Get(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	if session.RefreshTokenHash != platformjwt.HashToken(token) {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if found == nil || found.Status != "active" {
		return nil, ErrInvalidToken
	}

	if err := s.sessions.Delete(ctx, claims.UserID, claims.SessionID); err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, found)
}

func (s *Service) Logout(ctx context.Context, claims *platformjwt.Claims) error {
	return s.sessions.Delete(ctx, claims.UserID, claims.SessionID)
}

func (s *Service) ValidateAccessToken(ctx context.Context, token string) (*platformjwt.Claims, error) {
	claims, err := s.jwtManager.Parse(token)
	if err != nil || claims.Type != "access" {
		return nil, ErrInvalidToken
	}

	if _, err := s.sessions.Get(ctx, claims.UserID, claims.SessionID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	return claims, nil
}

func (s *Service) issueTokens(ctx context.Context, found *model.User) (*AuthResponse, error) {
	var schoolID *string
	if found.SchoolID != nil {
		value := found.SchoolID.String()
		schoolID = &value
	}

	tokens, err := s.jwtManager.GenerateTokenPair(
		found.ID.String(),
		found.Role,
		schoolID,
		s.jwtConfig.AccessTokenTTL,
		s.jwtConfig.RefreshTokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	if err := s.sessions.Save(
		ctx,
		tokens.SessionID,
		NewSessionPayload(found.ID.String(), found.Role, schoolID, tokens.RefreshToken, time.Now().Add(s.jwtConfig.RefreshTokenTTL)),
		s.jwtConfig.RefreshTokenTTL,
	); err != nil {
		return nil, err
	}

	dto, err := s.userService.GetMe(ctx, found.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         dto,
	}, nil
}
