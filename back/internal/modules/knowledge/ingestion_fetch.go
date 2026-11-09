package knowledge

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ingestionDeniedPrefixes = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("fec0::/10")}

func publicIngestionIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range ingestionDeniedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func validateIngestionSource(s IngestionSource) error {
	u, e := url.Parse(s.URL)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.Host == "" || strings.HasSuffix(u.Hostname(), ".") || net.ParseIP(u.Hostname()) != nil || s.ID == "" || len(s.ID) > 100 || s.Revision == "" || len(s.URL) > 1000 || len(s.Title) > 500 || s.Title == "" || (s.ContentType != "text/plain" && s.ContentType != "application/pdf") {
		return fail("source_not_allowed", 422)
	}
	return nil
}
func ingestionDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil || port != "443" {
		return nil, fail("source_not_allowed", 422)
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(ips) == 0 {
		return nil, fail("source_dns_unavailable", 503)
	}
	for _, ip := range ips {
		if !publicIngestionIP(ip) {
			return nil, fail("source_not_allowed", 422)
		}
	}
	var dialer net.Dialer
	dialer.Timeout = 5 * time.Second
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}
func newIngestionHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fail("redirect_not_allowed", 422) }, Transport: &http.Transport{Proxy: nil, DialContext: ingestionDial, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ResponseHeaderTimeout: 10 * time.Second, DisableCompression: true, MaxConnsPerHost: 2, IdleConnTimeout: time.Second}}
}
func fetchIngestionSource(ctx context.Context, source IngestionSource) ([]byte, error) {
	client := newIngestionHTTPClient()
	defer client.CloseIdleConnections()
	return fetchIngestionHTTP(ctx, client, source)
}
func fetchIngestionHTTP(ctx context.Context, client *http.Client, source IngestionSource) ([]byte, error) {
	if e := validateIngestionSource(source); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if e != nil {
		return nil, fail("source_not_allowed", 422)
	}
	req.Header.Set("Accept", source.ContentType)
	req.Header.Set("Accept-Encoding", "identity")
	res, e := client.Do(req)
	if e != nil {
		var blocked *Error
		if errors.As(e, &blocked) {
			return nil, blocked
		}
		return nil, fail("source_fetch_unavailable", 503)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != 200 {
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			return nil, fail("source_fetch_unavailable", 503)
		}
		return nil, fail("source_http_rejected", 422)
	}
	kind, _, e := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if e != nil || kind != source.ContentType || res.Header.Get("Content-Encoding") != "" || res.ContentLength > MaxBytes {
		return nil, fail("source_type_or_size_rejected", 422)
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, MaxBytes+1))
	if e != nil {
		return nil, fail("source_fetch_unavailable", 503)
	}
	if len(raw) == 0 || len(raw) > MaxBytes || (res.ContentLength >= 0 && int64(len(raw)) != res.ContentLength) {
		return nil, fail("source_type_or_size_rejected", 422)
	}
	return raw, nil
}
