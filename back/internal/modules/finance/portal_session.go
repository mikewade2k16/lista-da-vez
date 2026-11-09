package finance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type PortalSessionView struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	ExpiresAt  string `json:"expiresAt"`
	VerifiedAt string `json:"verifiedAt"`
	Reused     bool   `json:"reused"`
}
type portalSessionPayload struct {
	Account, User, CredentialsVersion string
	Cookies                           []portalCookie
}

func portalSessionView(s storedPortalSession, status string, reused bool) PortalSessionView {
	out := PortalSessionView{Status: status, Reused: reused, Message: "Sessão não conectada. Conecte o acesso salvo."}
	if !s.VerifiedAt.IsZero() {
		out.VerifiedAt = s.VerifiedAt.UTC().Format(time.RFC3339Nano)
	}
	if !s.ExpiresAt.IsZero() {
		out.ExpiresAt = s.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	switch status {
	case "saved":
		out.Message = "Sessão protegida salva. Ela será verificada antes do uso."
	case "authenticated":
		out.Message = "Sessão autenticada e protegida no Finance. Nenhuma nota foi emitida."
	case "expired":
		out.Message = "A sessão expirou. Conecte novamente com o acesso já salvo."
	}
	return out
}
func (s *FiscalService) PortalSessionStatus(ctx context.Context, account, user string) (PortalSessionView, error) {
	store, ok := s.store.(portalSessionStore)
	if !ok {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	p, err := s.GetProfile(ctx, account, user)
	if err != nil {
		return PortalSessionView{}, err
	}
	saved, err := store.readPortalSession(ctx, account, user)
	if errors.Is(err, ErrFiscalNotFound) {
		return portalSessionView(storedPortalSession{}, "disconnected", false), nil
	}
	if err != nil {
		return PortalSessionView{}, err
	}
	if !p.CredentialsSet || saved.CredentialsVersion != p.CredentialsUpdatedAt {
		return portalSessionView(storedPortalSession{}, "disconnected", false), nil
	}
	status := "saved"
	if !saved.ExpiresAt.After(time.Now()) {
		status = "expired"
	}
	return portalSessionView(saved, status, false), nil
}
func (s *FiscalService) ConnectPortal(ctx context.Context, account, user, expected string) (PortalSessionView, error) {
	store, ok := s.store.(portalSessionStore)
	if !ok || s.box == nil {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	p, err := s.GetProfile(ctx, account, user)
	if err != nil {
		return PortalSessionView{}, err
	}
	if p.UpdatedAt == "" || p.UpdatedAt != expected {
		return PortalSessionView{}, ErrFiscalConflict
	}
	if p.Profile.PortalLoginMethod == "govbr" {
		return PortalSessionView{Status: "manual_required", Message: portalCheck("manual_required").Message}, nil
	}
	if p.Profile.PortalURL != nationalPortalLogin && p.Profile.PortalURL != "https://www.nfse.gov.br/EmissorNacional" {
		return PortalSessionView{Status: "unsupported", Message: portalCheck("unsupported").Message}, nil
	}
	if !p.CredentialsSet {
		return PortalSessionView{}, invalidFiscal("Cadastre o acesso ao portal antes de conectar.")
	}
	select {
	case s.portalSlots <- struct{}{}:
		defer func() { <-s.portalSlots }()
	default:
		return PortalSessionView{}, invalidFiscal("Conexão ocupada. Aguarde e tente novamente.")
	}
	jar, err := newPortalCookieJar()
	if err != nil {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	client := s.portalHTTPClient(jar)
	saved, err := store.readPortalSession(ctx, account, user)
	if err != nil && !errors.Is(err, ErrFiscalNotFound) {
		return PortalSessionView{}, err
	}
	if err == nil && saved.CredentialsVersion == p.CredentialsUpdatedAt && saved.ExpiresAt.After(time.Now()) {
		raw, e := s.box.Decrypt(saved.Ciphertext)
		if e != nil {
			return PortalSessionView{}, ErrFiscalVaultUnavailable
		}
		var payload portalSessionPayload
		if json.Unmarshal([]byte(raw), &payload) != nil || payload.Account != account || payload.User != user || payload.CredentialsVersion != saved.CredentialsVersion {
			return PortalSessionView{}, ErrFiscalVaultUnavailable
		}
		jar.restore(payload.Cookies)
		body, response, e := portalRequest(ctx, client, http.MethodGet, nationalPortalHome, nil) //nolint:bodyclose // portalRequest reads and closes the body.
		if e != nil {
			return PortalSessionView{Status: "unavailable", Message: portalCheck("unavailable").Message}, nil
		}
		result := classifyPortalResponse(body, response)
		if result.Status == "authenticated" {
			return s.persistPortalSession(ctx, store, account, user, p.CredentialsUpdatedAt, jar, saved.ExpiresAt, true)
		}
		if result.Status == "manual_required" || result.Status == "unavailable" {
			return PortalSessionView{Status: result.Status, Message: result.Message}, nil
		}
		if e = store.dropPortalSession(ctx, account, user, p.CredentialsUpdatedAt); e != nil {
			return PortalSessionView{}, e
		}
	}
	// Only the saved portal credential is used for renewal; cookies never leave this service.
	access, ok := s.store.(portalAccessStore)
	if !ok {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	cipher, err := access.portalCiphertext(ctx, account, user)
	if err != nil {
		return PortalSessionView{}, err
	}
	raw, err := s.box.Decrypt(cipher)
	if err != nil {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	var credentials portalCredentials
	if json.Unmarshal([]byte(raw), &credentials) != nil {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	jar, err = newPortalCookieJar()
	if err != nil {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	client = s.portalHTTPClient(jar)
	result := probePortalWithClient(ctx, client, nationalPortalLogin, credentials)
	result.ProfileVersion = p.UpdatedAt
	if _, err = access.savePortalCheck(ctx, account, user, result); err != nil {
		return PortalSessionView{}, err
	}
	if result.Status != "authenticated" {
		return PortalSessionView{Status: result.Status, Message: result.Message}, nil
	}
	return s.persistPortalSession(ctx, store, account, user, p.CredentialsUpdatedAt, jar, time.Now().Add(portalSessionTTL), false)
}
func (s *FiscalService) persistPortalSession(ctx context.Context, store portalSessionStore, account, user, version string, jar *portalCookieJar, expires time.Time, reused bool) (PortalSessionView, error) {
	cookies := jar.snapshot()
	if len(cookies) == 0 || len(cookies) > 40 {
		return PortalSessionView{}, invalidFiscal("O portal não forneceu uma sessão que possa ser preservada.")
	}
	payload := portalSessionPayload{Account: account, User: user, CredentialsVersion: version, Cookies: cookies}
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > 65536 {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	cipher, err := s.box.Encrypt(string(raw))
	if err != nil {
		return PortalSessionView{}, ErrFiscalVaultUnavailable
	}
	saved, err := store.writePortalSession(ctx, account, user, storedPortalSession{CredentialsVersion: version, Ciphertext: cipher, ExpiresAt: expires})
	if err != nil {
		return PortalSessionView{}, err
	}
	return portalSessionView(saved, "authenticated", reused), nil
}
