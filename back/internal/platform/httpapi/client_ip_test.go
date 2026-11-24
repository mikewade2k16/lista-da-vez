package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newIPRequest(remote string, headers map[string]string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.RemoteAddr = remote
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		headers map[string]string
		want    string
	}{
		{"origem publica ignora XFF", "203.0.113.9:80", map[string]string{"X-Forwarded-For": "1.2.3.4"}, "203.0.113.9"},
		{"origem publica ignora X-Real-IP", "203.0.113.9:80", map[string]string{"X-Real-IP": "1.2.3.4"}, "203.0.113.9"},
		{"proxy confiavel usa ultimo hop nao confiavel", "172.19.0.2:80", map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.1"}, "198.51.100.1"},
		{"pula proxies confiaveis encadeados", "10.0.0.2:80", map[string]string{"X-Forwarded-For": "198.51.100.1, 10.0.0.9"}, "198.51.100.1"},
		{"cai para X-Real-IP", "172.19.0.2:80", map[string]string{"X-Real-IP": "198.51.100.2"}, "198.51.100.2"},
		{"XFF invalido cai para remote", "172.19.0.2:80", map[string]string{"X-Forwarded-For": "lixo"}, "172.19.0.2"},
		{"sem headers", "172.19.0.2:80", nil, "172.19.0.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClientIP(newIPRequest(tc.remote, tc.headers)); got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSetTrustedProxies_EmptyMeansNoneTrusted(t *testing.T) {
	previous := *trustedProxyNets.Load()
	t.Cleanup(func() { SetTrustedProxies(previous) })

	SetTrustedProxies(nil)
	got := ClientIP(newIPRequest("172.19.0.2:80", map[string]string{"X-Forwarded-For": "198.51.100.1"}))
	if got != "172.19.0.2" {
		t.Errorf("sem proxies confiaveis deve usar RemoteAddr; got %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	nets, err := ParseTrustedProxies([]string{" 10.0.0.0/8 ", "", "192.0.2.1"})
	if err != nil || len(nets) != 2 {
		t.Fatalf("parse: nets=%d err=%v", len(nets), err)
	}
	if _, err := ParseTrustedProxies([]string{"nao-e-ip"}); err == nil {
		t.Error("entrada invalida deveria falhar")
	}
}
