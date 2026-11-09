package knowledge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fixtureSourceTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t fixtureSourceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	u := *r.URL
	u.Scheme = t.target.Scheme
	u.Host = t.target.Host
	copy.URL = &u
	return t.base.RoundTrip(copy)
}
func TestIngestionPublicAddressAndURLPolicy(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "192.168.1.1", "198.18.0.1", "::1", "fc00::1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "2001:db8::1"} {
		if publicIngestionIP(netip.MustParseAddr(ip)) {
			t.Fatal("SSRF address allowed", ip)
		}
	}
	if !publicIngestionIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public IPv4 rejected")
	}
	base := IngestionSource{ID: "test", Revision: "v1", Title: "Synthetic", URL: "https://source.example.test/doc", ContentType: "text/plain"}
	for _, v := range []string{"http://source.example.test/doc", "https://127.0.0.1/doc", "https://user:pass@source.example.test/doc", "https://source.example.test:444/doc", "https://source.example.test/doc?url=private", "https://source.example.test/doc#private"} {
		base.URL = v
		if validateIngestionSource(base) == nil {
			t.Fatal(v)
		}
	}
}
func TestIngestionHTTPBoundsRedirectPartialAndTimeout(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		switch r.URL.Path {
		case "/ok":
			_, _ = io.WriteString(w, "Synthetic source payload")
		case "/redirect":
			w.Header().Set("Location", "http://169.254.169.254/latest/meta-data")
			w.WriteHeader(302)
		case "/large":
			_, _ = io.WriteString(w, strings.Repeat("x", MaxBytes+1))
		case "/partial":
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "incomplete")
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html>not supported</html>")
		case "/retry":
			w.WriteHeader(503)
		case "/timeout":
			<-r.Context().Done()
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	client := newIngestionHTTPClient()
	original := client.Transport
	client.Transport = fixtureSourceTransport{target: target, base: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	defer original.(*http.Transport).CloseIdleConnections()
	source := IngestionSource{ID: "synthetic", Revision: "v1", Title: "Synthetic", ContentType: "text/plain"}
	for _, path := range []string{"/ok", "/redirect", "/large", "/partial", "/html", "/retry", "/timeout"} {
		source.URL = "https://source.example.test" + path
		if path == "/timeout" {
			client.Timeout = 20 * time.Millisecond
		}
		raw, e := fetchIngestionHTTP(context.Background(), client, source)
		if path == "/ok" {
			if e != nil || string(raw) != "Synthetic source payload" {
				t.Fatal(e)
			}
		} else if e == nil {
			t.Fatal("unsafe response accepted", path)
		}
	}
	if hits.Load() != 7 {
		t.Fatal("redirect made another request", hits.Load())
	}
}
