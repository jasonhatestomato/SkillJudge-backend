package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid token")
	ErrSessionNotFound    = errors.New("session not found")
	ErrForbidden          = errors.New("forbidden")
)
