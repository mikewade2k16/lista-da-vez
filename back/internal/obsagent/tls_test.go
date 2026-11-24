package obsagent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestParseTLSHosts(t *testing.T) {
	got := ParseTLSHosts(" a.com, b.com:8443,,a.com ")
	if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com:8443" {
		t.Fatalf("got %v", got)
	}
	if len(ParseTLSHosts("")) != 0 {
		t.Fatal("vazio deve ficar vazio")
	}
}

func tlsTestServer(t *testing.T) (*httptest.Server, *tls.Config, string, *x509.Certificate) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	u, _ := url.Parse(srv.URL)
	return srv, &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, u.Host, srv.Certificate()
}

func TestCheckTLSDaysLeft(t *testing.T) {
	_, cfg, hostPort, cert := tlsTestServer(t)
	// O certificado do httptest usa 127.0.0.1 nos SANs; ServerName vira "127.0.0.1".
	now := cert.NotAfter.Add(-10*24*time.Hour - time.Hour)
	res := checkTLS(context.Background(), hostPort, cfg, now)
	if res.Error != "" {
		t.Fatalf("erro inesperado: %s", res.Error)
	}
	if res.DaysLeft != 10 {
		t.Fatalf("daysLeft = %d, want 10", res.DaysLeft)
	}
	if res.ExpiresAt != cert.NotAfter.UTC().Format(time.RFC3339) {
		t.Fatalf("expiresAt = %s", res.ExpiresAt)
	}
	expired := checkTLS(context.Background(), hostPort, cfg, cert.NotAfter.Add(48*time.Hour))
	if expired.DaysLeft >= 0 {
		t.Fatalf("expirado deveria ser negativo: %d", expired.DaysLeft)
	}
}

func TestCheckTLSErrors(t *testing.T) {
	srv, _, hostPort, _ := tlsTestServer(t)
	// Sem a CA de teste o certificado nao e confiavel: vira erro, nao panic.
	res := checkTLS(context.Background(), hostPort, nil, time.Now())
	if res.Error == "" {
		t.Fatal("esperava erro de certificado nao confiavel")
	}
	srv.Close()
	res = checkTLS(context.Background(), hostPort, nil, time.Now())
	if res.Error == "" {
		t.Fatal("esperava erro de conexao")
	}
}

func TestTLSCheckerCache(t *testing.T) {
	_, cfg, hostPort, _ := tlsTestServer(t)
	clock := time.Now()
	c := newTLSChecker([]string{hostPort})
	c.config = cfg
	c.now = func() time.Time { return clock }
	first := c.Results(context.Background())
	if len(first) != 1 || first[0].Error != "" {
		t.Fatalf("first = %+v", first)
	}
	c.hosts = []string{"invalido.invalid"} // se refizesse a checagem, mudaria
	clock = clock.Add(time.Hour)
	if got := c.Results(context.Background()); got[0].Host != hostPort {
		t.Fatalf("deveria usar cache: %+v", got)
	}
	clock = clock.Add(6 * time.Hour)
	if got := c.Results(context.Background()); got[0].Host != "invalido.invalid" {
		t.Fatalf("deveria refazer apos 6h: %+v", got)
	}
	if (&tlsChecker{}).Results(context.Background()) != nil {
		t.Fatal("sem hosts = nil")
	}
}
