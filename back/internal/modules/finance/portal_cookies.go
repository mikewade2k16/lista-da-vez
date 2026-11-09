package finance

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

const portalSessionTTL = 30 * time.Minute
const nationalPortalHome = "https://www.nfse.gov.br/EmissorNacional/"

type portalCookie struct {
	Origin string      `json:"origin"`
	Cookie http.Cookie `json:"cookie"`
}
type portalCookieJar struct {
	jar    *cookiejar.Jar
	mu     sync.Mutex
	stored map[string]portalCookie
}

func newPortalCookieJar() (*portalCookieJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &portalCookieJar{jar: jar, stored: map[string]portalCookie{}}, nil
}
func (j *portalCookieJar) Cookies(u *url.URL) []*http.Cookie { return j.jar.Cookies(u) }
func (j *portalCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.jar.SetCookies(u, cookies)
	if u.Scheme != "https" || u.Host != "www.nfse.gov.br" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, original := range cookies {
		c := *original
		if c.Domain != "" && strings.TrimPrefix(c.Domain, ".") != "www.nfse.gov.br" && strings.TrimPrefix(c.Domain, ".") != "nfse.gov.br" {
			continue
		}
		if c.Path == "" {
			i := strings.LastIndex(u.Path, "/")
			c.Path = "/"
			if i > 0 {
				c.Path = u.Path[:i]
			}
		}
		domain := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if domain == "" {
			domain = strings.ToLower(u.Hostname())
		}
		key := domain + "|" + c.Path + "|" + c.Name
		if c.MaxAge < 0 || !c.Expires.IsZero() && c.Expires.Before(time.Now()) {
			delete(j.stored, key)
			continue
		}
		if c.MaxAge > 0 {
			c.Expires = time.Now().Add(time.Duration(c.MaxAge) * time.Second)
		}
		j.stored[key] = portalCookie{Origin: nationalPortalHome, Cookie: c}
	}
}
func (j *portalCookieJar) snapshot() []portalCookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := []portalCookie{}
	for _, c := range j.stored {
		if c.Cookie.Expires.IsZero() || c.Cookie.Expires.After(time.Now()) {
			out = append(out, c)
		}
	}
	return out
}
func (j *portalCookieJar) restore(cookies []portalCookie) {
	for _, saved := range cookies {
		if saved.Origin != nationalPortalHome {
			continue
		}
		u, _ := url.Parse(saved.Origin)
		c := saved.Cookie
		if !c.Expires.IsZero() && c.Expires.Before(time.Now()) {
			continue
		}
		c.MaxAge = 0
		j.SetCookies(u, []*http.Cookie{&c})
	}
}
func nationalPortalRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.Host != "www.nfse.gov.br" {
		return http.ErrUseLastResponse
	}
	return nil
}
func newPortalHTTPClient(jar http.CookieJar) *http.Client {
	return &http.Client{Timeout: 20 * time.Second, Jar: jar, CheckRedirect: nationalPortalRedirect}
}
