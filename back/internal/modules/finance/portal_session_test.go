package finance

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

type portalTestTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (r portalTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *req.URL
	u.Scheme = r.target.Scheme
	u.Host = r.target.Host
	clone.URL = &u
	return r.base.RoundTrip(clone)
}
func TestPortalSessionPersistenceReuseAndInvalidation(t *testing.T) {
	pool := financeTestPool(t)
	ctx := context.Background()
	account, other := financeTestAccount(t, pool), financeTestAccount(t, pool)
	var user string
	if err := pool.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `delete from core.users where id=$1::uuid`, user) })
	var logins atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/EmissorNacional/Login" && r.Method == http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: "csrf", Value: "qa-csrf", Path: "/", Secure: true, HttpOnly: true})
			_, _ = w.Write([]byte(`<input name="__RequestVerificationToken" value="qa-token"><input name="Senha">`))
		case r.URL.Path == "/EmissorNacional/Login" && r.Method == http.MethodPost:
			logins.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("Senha") != "qa-secret" || r.Form.Get("Inscricao") != "qa-user" || r.Form.Get("__RequestVerificationToken") != "qa-token" {
				t.Error("incorrect credential form")
			}
			if _, err := r.Cookie("csrf"); err != nil {
				t.Error("CSRF cookie not sent")
			}
			http.SetCookie(w, &http.Cookie{Name: "auth", Value: "qa-auth-cookie", Path: "/EmissorNacional", Secure: true, HttpOnly: true, MaxAge: 3600})
			http.Redirect(w, r, "/EmissorNacional/", http.StatusFound)
		case r.URL.Path == "/EmissorNacional/" && r.Method == http.MethodGet:
			if c, err := r.Cookie("auth"); err == nil && c.Value == "qa-auth-cookie" {
				_, _ = w.Write([]byte(`<script src="captcha.js"></script><a href="/EmissorNacional/Login/Sair">Sair</a>`))
				return
			}
			_, _ = w.Write([]byte(`<input name="Senha">`))
		default:
			t.Error("unexpected remote operation")
			http.Error(w, "unexpected", 400)
		}
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewPostgresFiscalStore(pool)
	makeService := func() *FiscalService {
		s := NewFiscalService(store, box)
		s.portalHTTPClient = func(jar http.CookieJar) *http.Client {
			c := newPortalHTTPClient(jar)
			c.Transport = portalTestTransport{target: target, base: server.Client().Transport}
			return c
		}
		return s
	}
	svc := makeService()
	p, err := svc.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: FiscalProfile{PortalURL: nationalPortalLogin, PortalLoginMethod: "portal"}})
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.SaveCredentials(ctx, account, user, FiscalCredentialInput{Username: "qa-user", Password: "qa-secret", ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	connected, err := svc.ConnectPortal(ctx, account, user, p.UpdatedAt)
	if err != nil || connected.Status != "authenticated" || connected.Reused || logins.Load() != 1 {
		t.Fatal("initial connection failed", err)
	}
	saved, err := store.readPortalSession(ctx, account, user)
	if err != nil || strings.Contains(saved.Ciphertext, "qa-auth") || !strings.HasPrefix(saved.Ciphertext, "v1:") {
		t.Fatal("session not protected", err)
	}
	raw, _ := json.Marshal(connected)
	if strings.Contains(string(raw), "qa-auth") || strings.Contains(string(raw), "qa-secret") {
		t.Fatal("session secret exposed")
	}
	// A new service instance restores the protected cookie and performs GET, without another login POST.
	reused, err := makeService().ConnectPortal(ctx, account, user, p.UpdatedAt)
	if err != nil || !reused.Reused || reused.Status != "authenticated" || logins.Load() != 1 {
		t.Fatal("persisted session not reused", err)
	}
	hidden, err := svc.PortalSessionStatus(ctx, other, user)
	if err != nil || hidden.Status != "disconnected" {
		t.Fatal("session leaked")
	}
	p.Profile.LegalName = "Edited fiscal data"
	p, err = svc.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p.Profile, ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	reused, err = svc.ConnectPortal(ctx, account, user, p.UpdatedAt)
	if err != nil || !reused.Reused || logins.Load() != 1 {
		t.Fatal("fiscal edit discarded unchanged access", err)
	}
	if _, err = pool.Exec(ctx, `update finance.portal_sessions set expires_at=clock_timestamp()-interval '1 minute' where account_id=$1::uuid and user_id=$2::uuid`, account, user); err != nil {
		t.Fatal(err)
	}
	renewed, err := svc.ConnectPortal(ctx, account, user, p.UpdatedAt)
	if err != nil || renewed.Reused || logins.Load() != 2 {
		t.Fatal("expired session not renewed", err)
	}
	old, err := store.readPortalSession(ctx, account, user)
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.SaveCredentials(ctx, account, user, FiscalCredentialInput{Username: "qa-user", Password: "replacement", ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.readPortalSession(ctx, account, user); !errors.Is(err, ErrFiscalNotFound) {
		t.Fatal("old session not removed")
	}
	if _, err = store.writePortalSession(ctx, account, user, old); !errors.Is(err, ErrFiscalConflict) {
		t.Fatal("stale session resurrected")
	}
}

func TestPortalCookieDeletionDoesNotResurrectHostCookie(t *testing.T) {
	jar, err := newPortalCookieJar()
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(nationalPortalHome)
	jar.SetCookies(target, []*http.Cookie{{Name: "auth", Value: "qa-value", Path: "/"}})
	jar.SetCookies(target, []*http.Cookie{{Name: "auth", Domain: "www.nfse.gov.br", Path: "/", MaxAge: -1}})
	if len(jar.snapshot()) != 0 {
		t.Fatal("deleted cookie retained for persistence")
	}
}
