package mail

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	stdmail "net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"skilljudge/backend/internal/config"
)

type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	timeout  time.Duration
	from     Address
}

func NewSMTPSender(cfg config.MailConfig) (*SMTPSender, error) {
	from, err := NormalizeAddress(Address{
		Name:  cfg.FromName,
		Email: cfg.FromAddress,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return nil, fmt.Errorf("mail smtp host is required")
	}
	if cfg.SMTPPort <= 0 {
		return nil, fmt.Errorf("mail smtp port is invalid")
	}
	if strings.TrimSpace(cfg.SMTPUsername) == "" {
		return nil, fmt.Errorf("mail smtp username is required")
	}
	if strings.TrimSpace(cfg.SMTPPassword) == "" {
		return nil, fmt.Errorf("mail smtp password is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	return &SMTPSender{
		host:     strings.TrimSpace(cfg.SMTPHost),
		port:     cfg.SMTPPort,
		username: strings.TrimSpace(cfg.SMTPUsername),
		password: cfg.SMTPPassword,
		timeout:  cfg.Timeout,
		from:     from,
	}, nil
}

func (s *SMTPSender) Enabled() bool {
	return true
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(msg.Subject) == "" {
		return fmt.Errorf("mail subject is required")
	}
	if strings.TrimSpace(msg.TextBody) == "" {
		return fmt.Errorf("mail body is required")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail recipients are required")
	}

	from := s.from
	if strings.TrimSpace(msg.From.Email) != "" {
		normalized, err := NormalizeAddress(msg.From)
		if err != nil {
			return err
		}
		from = normalized
	}

	recipients := make([]Address, 0, len(msg.To))
	recipientEmails := make([]string, 0, len(msg.To))
	for i := range msg.To {
		normalized, err := NormalizeAddress(msg.To[i])
		if err != nil {
			return err
		}
		recipients = append(recipients, normalized)
		recipientEmails = append(recipientEmails, normalized.Email)
	}

	payload := buildPlainTextMessage(from, recipients, msg.Subject, msg.TextBody)
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))

	dialer := &net.Dialer{Timeout: s.timeout}
	var conn net.Conn
	var err error
	if s.port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client failed: %w", err)
	}
	defer client.Close()

	if s.port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{
				ServerName: s.host,
				MinVersion: tls.VersionTLS12,
			}); err != nil {
				return fmt.Errorf("smtp starttls failed: %w", err)
			}
		}
	}

	if ok, _ := client.Extension("AUTH"); ok {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	if err := client.Mail(from.Email); err != nil {
		return fmt.Errorf("smtp mail from failed: %w", err)
	}
	for _, email := range recipientEmails {
		if err := client.Rcpt(email); err != nil {
			return fmt.Errorf("smtp recipient failed: %w", err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}
	if _, err := writer.Write([]byte(payload)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp finalize failed: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit failed: %w", err)
	}

	return nil
}

func buildPlainTextMessage(from Address, to []Address, subject, body string) string {
	headers := []string{
		"From: " + formatAddress(from),
		"To: " + formatAddressList(to),
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
	}

	encodedBody := wrapBase64(base64.StdEncoding.EncodeToString([]byte(body)))
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + encodedBody
}

func formatAddress(addr Address) string {
	if addr.Name == "" {
		return addr.Email
	}
	return (&stdmail.Address{Name: mime.QEncoding.Encode("utf-8", addr.Name), Address: addr.Email}).String()
}

func formatAddressList(items []Address) string {
	formatted := make([]string, 0, len(items))
	for i := range items {
		formatted = append(formatted, formatAddress(items[i]))
	}
	return strings.Join(formatted, ", ")
}

func wrapBase64(value string) string {
	if value == "" {
		return ""
	}

	const lineLength = 76
	lines := make([]string, 0, (len(value)/lineLength)+1)
	for start := 0; start < len(value); start += lineLength {
		end := start + lineLength
		if end > len(value) {
			end = len(value)
		}
		lines = append(lines, value[start:end])
	}

	return strings.Join(lines, "\r\n")
}
