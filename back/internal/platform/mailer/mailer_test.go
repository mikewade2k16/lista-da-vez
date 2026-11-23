package mailer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConfigured(t *testing.T) {
	cases := map[string]struct {
		config Config
		want   bool
	}{
		"sem host":          {Config{FromEmail: "a@b.com"}, false},
		"sem remetente":     {Config{Host: "smtp"}, false},
		"ok":                {Config{Host: "smtp", FromEmail: "a@b.com"}, true},
		"usuario sem senha": {Config{Host: "smtp", FromEmail: "a@b.com", Username: "u"}, false},
		"tls invalido":      {Config{Host: "smtp", FromEmail: "a@b.com", TLSMode: "xyz"}, false},
	}
	for name, tc := range cases {
		if got := New(tc.config).Configured(); got != tc.want {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
	var nilMailer *Mailer
	if nilMailer.Configured() {
		t.Error("nil mailer deve ser nao configurado")
	}
}

func TestSendNotConfigured(t *testing.T) {
	err := New(Config{}).Send(context.Background(), []string{"a@b.com"}, "x", "y")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("esperava ErrNotConfigured, veio %v", err)
	}
}

func TestBuildMessageBlocksHeaderInjection(t *testing.T) {
	m := New(Config{Host: "smtp", FromEmail: "from@x.com", FromName: "Omni"})
	msg := m.buildMessage([]string{"to@x.com"}, "Alerta\r\nBcc: evil@x.com", "linha1\nlinha2")
	head, body, _ := strings.Cut(msg, "\r\n\r\n")
	if strings.Contains(head, "\r\nBcc:") {
		t.Fatalf("injecao de cabecalho: %q", head)
	}
	if !strings.Contains(body, "linha1\r\nlinha2") {
		t.Fatalf("corpo sem CRLF: %q", body)
	}
}

func TestCleanRecipients(t *testing.T) {
	got := cleanRecipients([]string{" a@b.com ", "invalido", "Nome <c@d.com>"})
	if len(got) != 2 || got[0] != "a@b.com" || got[1] != "c@d.com" {
		t.Fatalf("got %v", got)
	}
}
