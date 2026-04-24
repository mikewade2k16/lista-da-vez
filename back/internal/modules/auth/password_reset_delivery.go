package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTPTLSMode string

const (
	SMTPTLSModeNone     SMTPTLSMode = "none"
	SMTPTLSModeStartTLS SMTPTLSMode = "starttls"
	SMTPTLSModeTLS      SMTPTLSMode = "tls"
)

type SMTPPasswordResetDeliveryConfig struct {
	AppName            string
	Host               string
	Port               int
	Username           string
	Password           string
	FromEmail          string
	FromName           string
	TLSMode            SMTPTLSMode
	InsecureSkipVerify bool
	Timeout            time.Duration
}

type SMTPPasswordResetDelivery struct {
	config SMTPPasswordResetDeliveryConfig
	logger *slog.Logger
}

type LogPasswordResetDelivery struct {
	logger *slog.Logger
}

func BuildPasswordResetDelivery(config SMTPPasswordResetDeliveryConfig, logger *slog.Logger) (PasswordResetDelivery, error) {
	if !config.Enabled() {
		return NewLogPasswordResetDelivery(logger), nil
	}

	return NewSMTPPasswordResetDelivery(config, logger)
}

func NewLogPasswordResetDelivery(logger *slog.Logger) *LogPasswordResetDelivery {
	if logger == nil {
		logger = slog.Default()
	}

	return &LogPasswordResetDelivery{logger: logger}
}

func (delivery *LogPasswordResetDelivery) DeliverPasswordResetCode(_ context.Context, user User, code string, expiresAt time.Time) error {
	delivery.logger.Info(
		"auth_password_reset_code_issued",
		"user_id", user.ID,
		"email", user.Email,
		"code", code,
		"expires_at", expiresAt,
	)

	return nil
}

func NewSMTPPasswordResetDelivery(config SMTPPasswordResetDeliveryConfig, logger *slog.Logger) (*SMTPPasswordResetDelivery, error) {
	if logger == nil {
		logger = slog.Default()
	}

	normalized := config.normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	return &SMTPPasswordResetDelivery{
		config: normalized,
		logger: logger,
	}, nil
}

func (delivery *SMTPPasswordResetDelivery) DeliverPasswordResetCode(ctx context.Context, user User, code string, expiresAt time.Time) error {
	recipient := strings.TrimSpace(user.Email)
	if recipient == "" {
		return fmt.Errorf("auth: missing recipient email for password reset")
	}

	message := delivery.buildMessage(recipient, code, expiresAt)
	if err := delivery.send(ctx, recipient, []byte(message)); err != nil {
		return err
	}

	delivery.logger.Info(
		"auth_password_reset_code_sent",
		"user_id", user.ID,
		"email", recipient,
		"smtp_host", delivery.config.Host,
		"smtp_port", delivery.config.Port,
		"expires_at", expiresAt,
	)

	return nil
}

func (config SMTPPasswordResetDeliveryConfig) Enabled() bool {
	return strings.TrimSpace(config.Host) != ""
}

func (config SMTPPasswordResetDeliveryConfig) normalized() SMTPPasswordResetDeliveryConfig {
	normalized := config
	normalized.AppName = strings.TrimSpace(normalized.AppName)
	normalized.Host = strings.TrimSpace(normalized.Host)
	normalized.Username = strings.TrimSpace(normalized.Username)
	normalized.Password = strings.TrimSpace(normalized.Password)
	normalized.FromEmail = strings.TrimSpace(normalized.FromEmail)
	normalized.FromName = strings.TrimSpace(normalized.FromName)
	normalized.TLSMode = normalizeSMTPTLSMode(normalized.TLSMode)

	if normalized.Port <= 0 {
		normalized.Port = 587
	}

	if normalized.Timeout <= 0 {
		normalized.Timeout = 10 * time.Second
	}

	if normalized.AppName == "" {
		normalized.AppName = "Lista da Vez"
	}

	return normalized
}

func (config SMTPPasswordResetDeliveryConfig) Validate() error {
	if !config.Enabled() {
		return nil
	}

	if config.FromEmail == "" {
		return fmt.Errorf("auth: SMTP_FROM_EMAIL is required when SMTP_HOST is configured")
	}

	if config.Port <= 0 {
		return fmt.Errorf("auth: SMTP_PORT must be greater than zero")
	}

	if (config.Username == "") != (config.Password == "") {
		return fmt.Errorf("auth: SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}

	switch config.TLSMode {
	case SMTPTLSModeNone, SMTPTLSModeStartTLS, SMTPTLSModeTLS:
		return nil
	default:
		return fmt.Errorf("auth: invalid SMTP_TLS_MODE %q", config.TLSMode)
	}
}

func normalizeSMTPTLSMode(value SMTPTLSMode) SMTPTLSMode {
	switch strings.ToLower(strings.TrimSpace(string(value))) {
	case "", string(SMTPTLSModeStartTLS):
		return SMTPTLSModeStartTLS
	case string(SMTPTLSModeTLS):
		return SMTPTLSModeTLS
	case string(SMTPTLSModeNone):
		return SMTPTLSModeNone
	default:
		return value
	}
}

func (delivery *SMTPPasswordResetDelivery) buildMessage(recipient string, code string, expiresAt time.Time) string {
	fromHeader := formatSMTPAddress(delivery.config.FromName, delivery.config.FromEmail)
	toHeader := formatSMTPAddress("", recipient)
	subject := "Codigo de recuperacao de senha"
	body := delivery.buildBody(code, expiresAt)

	lines := []string{
		fmt.Sprintf("From: %s", fromHeader),
		fmt.Sprintf("To: %s", toHeader),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
		"",
	}

	return strings.Join(lines, "\r\n")
}

func (delivery *SMTPPasswordResetDelivery) buildBody(code string, expiresAt time.Time) string {
	appName := delivery.config.AppName
	minutesRemaining := int(time.Until(expiresAt).Round(time.Minute).Minutes())
	if minutesRemaining < 1 {
		minutesRemaining = 1
	}

	return strings.Join([]string{
		fmt.Sprintf("Voce solicitou a recuperacao de senha no %s.", appName),
		"",
		fmt.Sprintf("Seu codigo de verificacao e: %s", code),
		fmt.Sprintf("Este codigo expira em aproximadamente %d minutos.", minutesRemaining),
		"",
		"Se voce nao solicitou esta troca, ignore este email.",
	}, "\r\n")
}

func (delivery *SMTPPasswordResetDelivery) send(ctx context.Context, recipient string, message []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	address := net.JoinHostPort(delivery.config.Host, strconv.Itoa(delivery.config.Port))
	tlsConfig := &tls.Config{
		ServerName:         delivery.config.Host,
		InsecureSkipVerify: delivery.config.InsecureSkipVerify,
	}

	if delivery.config.TLSMode == SMTPTLSModeTLS {
		return delivery.sendWithTLS(address, tlsConfig, recipient, message)
	}

	dialer := &net.Dialer{Timeout: delivery.config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("auth: failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, delivery.config.Host)
	if err != nil {
		return fmt.Errorf("auth: failed to create SMTP client: %w", err)
	}
	defer client.Close()

	if delivery.config.TLSMode == SMTPTLSModeStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("auth: SMTP server does not support STARTTLS")
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("auth: failed to negotiate STARTTLS: %w", err)
		}
	}

	if err := delivery.authenticate(client); err != nil {
		return err
	}

	return delivery.writeMessage(client, recipient, message)
}

func (delivery *SMTPPasswordResetDelivery) sendWithTLS(address string, tlsConfig *tls.Config, recipient string, message []byte) error {
	dialer := &net.Dialer{Timeout: delivery.config.Timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		return fmt.Errorf("auth: failed to connect to SMTPS server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, delivery.config.Host)
	if err != nil {
		return fmt.Errorf("auth: failed to create SMTPS client: %w", err)
	}
	defer client.Close()

	if err := delivery.authenticate(client); err != nil {
		return err
	}

	return delivery.writeMessage(client, recipient, message)
}

func (delivery *SMTPPasswordResetDelivery) authenticate(client *smtp.Client) error {
	if delivery.config.Username == "" {
		return nil
	}

	if ok, _ := client.Extension("AUTH"); !ok {
		return fmt.Errorf("auth: SMTP server does not support AUTH")
	}

	auth := smtp.PlainAuth("", delivery.config.Username, delivery.config.Password, delivery.config.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("auth: SMTP authentication failed: %w", err)
	}

	return nil
}

func (delivery *SMTPPasswordResetDelivery) writeMessage(client *smtp.Client, recipient string, message []byte) error {
	if err := client.Mail(delivery.config.FromEmail); err != nil {
		return fmt.Errorf("auth: SMTP MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("auth: SMTP RCPT TO failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("auth: SMTP DATA failed: %w", err)
	}

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("auth: failed to write SMTP message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("auth: failed to finalize SMTP message: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("auth: failed to close SMTP session: %w", err)
	}

	return nil
}

func formatSMTPAddress(name string, address string) string {
	formatted := mail.Address{
		Name:    strings.TrimSpace(name),
		Address: strings.TrimSpace(address),
	}

	return formatted.String()
}
