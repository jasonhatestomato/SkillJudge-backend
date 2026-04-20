package mail

import (
	"context"
	"errors"
	"fmt"
	stdmail "net/mail"
	"strings"

	"skilljudge/backend/internal/config"
)

var (
	ErrDisabled          = errors.New("mail sender is disabled")
	ErrUnsupportedSender = errors.New("mail provider is unsupported")
)

type Address struct {
	Name  string
	Email string
}

type Message struct {
	From     Address
	To       []Address
	Subject  string
	TextBody string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
	Enabled() bool
}

type DisabledSender struct{}

func NewSender(cfg config.MailConfig) (Sender, error) {
	if !cfg.Enabled {
		return DisabledSender{}, nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "smtp":
		return NewSMTPSender(cfg)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSender, cfg.Provider)
	}
}

func (DisabledSender) Send(context.Context, Message) error {
	return ErrDisabled
}

func (DisabledSender) Enabled() bool {
	return false
}

func NormalizeAddress(addr Address) (Address, error) {
	normalized := Address{
		Name:  strings.TrimSpace(addr.Name),
		Email: strings.ToLower(strings.TrimSpace(addr.Email)),
	}
	if normalized.Email == "" {
		return Address{}, fmt.Errorf("mail address is required")
	}
	parsed, err := stdmail.ParseAddress(normalized.Email)
	if err != nil {
		return Address{}, fmt.Errorf("mail address is invalid: %w", err)
	}
	normalized.Email = strings.ToLower(strings.TrimSpace(parsed.Address))
	return normalized, nil
}
