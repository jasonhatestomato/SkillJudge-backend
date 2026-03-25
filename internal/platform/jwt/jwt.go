package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Manager struct {
	issuer string
	secret []byte
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	SessionID    string `json:"-"`
}

type Claims struct {
	UserID    string `json:"uid"`
	Role      string `json:"role"`
	SchoolID  string `json:"schoolId,omitempty"`
	SessionID string `json:"sid"`
	Type      string `json:"typ"`
	jwtlib.RegisteredClaims
}

func NewManager(issuer, secret string) *Manager {
	return &Manager{
		issuer: issuer,
		secret: []byte(secret),
	}
}

func (m *Manager) GenerateTokenPair(userID, role string, schoolID *string, accessTTL, refreshTTL time.Duration) (TokenPair, error) {
	sessionID := uuid.NewString()

	accessToken, err := m.signToken(userID, role, schoolID, sessionID, "access", accessTTL)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := m.signToken(userID, role, schoolID, sessionID, "refresh", refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTTL.Seconds()),
		SessionID:    sessionID,
	}, nil
}

func (m *Manager) Parse(token string) (*Claims, error) {
	parsed, err := jwtlib.ParseWithClaims(token, &Claims{}, func(t *jwtlib.Token) (any, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}

		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) signToken(userID, role string, schoolID *string, sessionID, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Role:      role,
		SessionID: sessionID,
		Type:      tokenType,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
		},
	}

	if schoolID != nil {
		claims.SchoolID = *schoolID
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}
