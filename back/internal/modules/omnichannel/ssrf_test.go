package omnichannel

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},       // loopback
		{"10.0.0.1", true},        // privado
		{"192.168.1.10", true},    // privado
		{"172.16.5.5", true},      // privado
		{"169.254.169.254", true}, // metadata / link-local
		{"100.64.0.1", true},      // CGNAT
		{"0.0.0.0", true},         // unspecified
		{"8.8.8.8", false},        // publico
		{"1.1.1.1", false},        // publico
	}
	for _, c := range cases {
		got := isBlockedIP(net.ParseIP(c.ip))
		if got != c.blocked {
			t.Errorf("isBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
	if !isBlockedIP(nil) {
		t.Error("isBlockedIP(nil) deveria bloquear (fail-close)")
	}
}

func TestValidatePublicURL(t *testing.T) {
	ctx := context.Background()
	// Protocolo != http/https => 422.
	for _, raw := range []string{"ftp://example.com/x", "file:///etc/passwd", "gopher://x", ""} {
		if err := validatePublicURL(ctx, raw); !errors.Is(err, ErrSSRFBadScheme) {
			t.Errorf("validatePublicURL(%q) = %v, want ErrSSRFBadScheme", raw, err)
		}
	}
	// Host interno (IP literal, sem DNS) => 403.
	for _, raw := range []string{"http://127.0.0.1/x", "https://10.0.0.5/y", "http://169.254.169.254/latest"} {
		if err := validatePublicURL(ctx, raw); !errors.Is(err, ErrSSRFBlockedHost) {
			t.Errorf("validatePublicURL(%q) = %v, want ErrSSRFBlockedHost", raw, err)
		}
	}
	// IP publico literal => ok (LookupIPAddr devolve o proprio IP, sem rede).
	if err := validatePublicURL(ctx, "http://8.8.8.8/ok"); err != nil {
		t.Errorf("validatePublicURL(publico) = %v, want nil", err)
	}
}
