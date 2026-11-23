// Package mailer envia e-mails de texto simples por SMTP (STARTTLS, TLS implícito
// ou sem TLS). É um transporte genérico: quem monta assunto e corpo é o chamador.
package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type TLSMode string

const (
	TLSModeNone     TLSMode = "none"
	TLSModeStartTLS TLSMode = "starttls"
	TLSModeTLS      TLSMode = "tls"
)

var ErrNotConfigured = errors.New("mailer: smtp nao configurado")

type Config struct {
	Host               string
	Port               int
	Username           string
	Password           string
	FromEmail          string
	FromName           string
	TLSMode            TLSMode
	InsecureSkipVerify bool
	Timeout            time.Duration
}

// Mailer é seguro para uso concorrente (não guarda conexão). Um *Mailer nil conta
// como não configurado.
type Mailer struct {
	config Config
}

func New(config Config) *Mailer {
	config.Host = strings.TrimSpace(config.Host)
	config.Username = strings.TrimSpace(config.Username)
	config.Password = strings.TrimSpace(config.Password)
	config.FromEmail = strings.TrimSpace(config.FromEmail)
	config.FromName = strings.TrimSpace(config.FromName)
	config.TLSMode = normalizeTLSMode(config.TLSMode)
	if config.Port <= 0 {
		config.Port = 587
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	return &Mailer{config: config}
}

// Configured indica que há servidor e remetente válidos para enviar.
func (m *Mailer) Configured() bool {
	return m != nil && m.config.Host != "" && m.config.FromEmail != "" && m.validate() == nil
}

func (m *Mailer) validate() error {
	if (m.config.Username == "") != (m.config.Password == "") {
		return errors.New("mailer: usuario e senha SMTP devem ser configurados juntos")
	}
	switch m.config.TLSMode {
	case TLSModeNone, TLSModeStartTLS, TLSModeTLS:
		return nil
	default:
		return fmt.Errorf("mailer: modo TLS invalido %q", m.config.TLSMode)
	}
}

func normalizeTLSMode(value TLSMode) TLSMode {
	switch strings.ToLower(strings.TrimSpace(string(value))) {
	case "", string(TLSModeStartTLS):
		return TLSModeStartTLS
	case string(TLSModeTLS):
		return TLSModeTLS
	case string(TLSModeNone):
		return TLSModeNone
	default:
		return value
	}
}

// Send envia uma mensagem de texto simples (UTF-8) a todos os destinatários.
func (m *Mailer) Send(ctx context.Context, to []string, subject, textBody string) error {
	if !m.Configured() {
		return ErrNotConfigured
	}
	recipients := cleanRecipients(to)
	if len(recipients) == 0 {
		return errors.New("mailer: sem destinatarios")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	message := m.buildMessage(recipients, subject, textBody)
	return m.deliver(ctx, recipients, []byte(message))
}

func cleanRecipients(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		address, err := mail.ParseAddress(strings.TrimSpace(value))
		if err == nil {
			out = append(out, address.Address)
		}
	}
	return out
}

func (m *Mailer) buildMessage(recipients []string, subject, body string) string {
	toHeaders := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		toHeaders = append(toHeaders, formatAddress("", recipient))
	}
	lines := []string{
		"From: " + formatAddress(m.config.FromName, m.config.FromEmail),
		"To: " + strings.Join(toHeaders, ", "),
		"Subject: " + mimeHeader(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"),
		"",
	}
	return strings.Join(lines, "\r\n")
}

// mimeHeader codifica o assunto (RFC 2047) e remove quebras de linha para evitar
// injeção de cabeçalho.
func mimeHeader(value string) string {
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	return mime.QEncoding.Encode("utf-8", value)
}

func formatAddress(name, address string) string {
	return (&mail.Address{Name: strings.TrimSpace(name), Address: strings.TrimSpace(address)}).String()
}

func (m *Mailer) deliver(ctx context.Context, recipients []string, message []byte) error {
	address := net.JoinHostPort(m.config.Host, strconv.Itoa(m.config.Port))
	// InsecureSkipVerify so com opt-in explicito (SMTP_INSECURE_SKIP_VERIFY), como no auth.
	tlsConfig := &tls.Config{ServerName: m.config.Host, InsecureSkipVerify: m.config.InsecureSkipVerify} //nolint:gosec // opt-in explicito por env
	dialer := &net.Dialer{Timeout: m.config.Timeout}

	var conn net.Conn
	var err error
	if m.config.TLSMode == TLSModeTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("mailer: falha ao conectar no SMTP: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(m.config.Timeout * 3))

	client, err := smtp.NewClient(conn, m.config.Host)
	if err != nil {
		return fmt.Errorf("mailer: falha ao criar cliente SMTP: %w", err)
	}
	defer client.Close()

	if m.config.TLSMode == TLSModeStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("mailer: servidor SMTP sem suporte a STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("mailer: falha no STARTTLS: %w", err)
		}
	}
	if m.config.Username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("mailer: servidor SMTP sem suporte a AUTH")
		}
		if err := client.Auth(smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)); err != nil {
			return fmt.Errorf("mailer: autenticacao SMTP falhou: %w", err)
		}
	}
	return writeMessage(client, m.config.FromEmail, recipients, message)
}

func writeMessage(client *smtp.Client, from string, recipients []string, message []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mailer: MAIL FROM falhou: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("mailer: RCPT TO falhou: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA falhou: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("mailer: falha ao escrever a mensagem: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mailer: falha ao finalizar a mensagem: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("mailer: falha ao encerrar a sessao: %w", err)
	}
	return nil
}
