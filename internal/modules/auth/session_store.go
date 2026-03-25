package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"skilljudge/backend/internal/platform/jwt"

	"github.com/redis/go-redis/v9"
)

type SessionStore struct {
	client *redis.Client
}

type Session struct {
	UserID           string    `json:"userId"`
	Role             string    `json:"role"`
	SchoolID         *string   `json:"schoolId,omitempty"`
	RefreshTokenHash string    `json:"refreshTokenHash"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

func (s *SessionStore) Save(ctx context.Context, sessionID string, payload Session, ttl time.Duration) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, sessionKey(payload.UserID, sessionID), body, ttl).Err()
}

func (s *SessionStore) Get(ctx context.Context, userID, sessionID string) (*Session, error) {
	value, err := s.client.Get(ctx, sessionKey(userID, sessionID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	var session Session
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *SessionStore) Delete(ctx context.Context, userID, sessionID string) error {
	return s.client.Del(ctx, sessionKey(userID, sessionID)).Err()
}

func sessionKey(userID, sessionID string) string {
	return fmt.Sprintf("session:%s:%s", userID, sessionID)
}

func NewSessionPayload(userID, role string, schoolID *string, refreshToken string, expiresAt time.Time) Session {
	return Session{
		UserID:           userID,
		Role:             role,
		SchoolID:         schoolID,
		RefreshTokenHash: jwt.HashToken(refreshToken),
		ExpiresAt:        expiresAt,
	}
}
