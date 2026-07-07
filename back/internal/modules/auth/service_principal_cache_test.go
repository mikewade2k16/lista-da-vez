package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// White-box (package auth) para montar User direto e observar os round-trips ao
// banco. Importa httpapi so no teste — sem ciclo, pois httpapi nao importa auth.

type fakeAuthUserRepo struct {
	UserRepository
	user      User
	loadCalls int
}

func (f *fakeAuthUserRepo) LoadUserForAuth(_ context.Context, _ string) (User, error) {
	f.loadCalls++
	return f.user, nil
}

type fakeSessionRepo struct {
	SessionRepository
	revoked      map[string]bool
	revokedCalls int
}

func (f *fakeSessionRepo) IsRevoked(_ context.Context, sessionID string) (bool, error) {
	f.revokedCalls++
	return f.revoked[sessionID], nil
}

func (f *fakeSessionRepo) Revoke(_ context.Context, sessionID string) error {
	f.revoked[sessionID] = true
	return nil
}

func newCacheTestUser() User {
	return User{
		ID:          "user-1",
		DisplayName: "Diretor Teste",
		Email:       "diretor@omni.test",
		Role:        RoleDirector,
		TenantID:    "tenant-1",
		Active:      true,
	}
}

func newCacheTestService(repo UserRepository, sessions SessionRepository) (*Service, TokenManager) {
	tokens := NewHMACTokenManager("test-secret", time.Hour)
	service := NewService(repo, nil, tokens, nil, nil, nil, nil)
	if sessions != nil {
		service.SetSessionRepository(sessions)
	}
	return service, tokens
}

func issueTestToken(t *testing.T, tokens TokenManager, sessionID string, user User) string {
	t.Helper()
	session, err := tokens.Issue(sessionID, user)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return session.AccessToken
}

func TestAuthenticateToken_SecondCallHitsCache(t *testing.T) {
	repo := &fakeAuthUserRepo{user: newCacheTestUser()}
	sessions := &fakeSessionRepo{revoked: map[string]bool{}}
	service, tokens := newCacheTestService(repo, sessions)
	service.SetPrincipalCache(httpapi.NewPrincipalCache[Principal](time.Minute))

	token := issueTestToken(t, tokens, "sess-1", repo.user)

	first, err := service.AuthenticateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("primeira autenticacao: %v", err)
	}
	second, err := service.AuthenticateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("segunda autenticacao: %v", err)
	}

	if repo.loadCalls != 1 {
		t.Fatalf("esperado 1 LoadUserForAuth (segunda foi hit), veio %d", repo.loadCalls)
	}
	if sessions.revokedCalls != 1 {
		t.Fatalf("esperado 1 IsRevoked (segunda foi hit), veio %d", sessions.revokedCalls)
	}
	if first.UserID != second.UserID || first.Role != second.Role {
		t.Fatalf("principal divergente entre as chamadas: %+v vs %+v", first, second)
	}
}

func TestLogout_InvalidatesCachedSession(t *testing.T) {
	repo := &fakeAuthUserRepo{user: newCacheTestUser()}
	sessions := &fakeSessionRepo{revoked: map[string]bool{}}
	service, tokens := newCacheTestService(repo, sessions)
	service.SetPrincipalCache(httpapi.NewPrincipalCache[Principal](time.Minute))

	token := issueTestToken(t, tokens, "sess-1", repo.user)

	principal, err := service.AuthenticateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("autenticacao inicial: %v", err)
	}

	revokedBefore := sessions.revokedCalls
	if err := service.Logout(context.Background(), principal); err != nil {
		t.Fatalf("logout: %v", err)
	}

	_, err = service.AuthenticateToken(context.Background(), token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("apos logout esperava ErrUnauthorized imediato, veio %v", err)
	}
	if sessions.revokedCalls <= revokedBefore {
		t.Fatalf("miss forcado deveria ir ao banco (IsRevoked), calls antes=%d depois=%d", revokedBefore, sessions.revokedCalls)
	}
}

func TestAuthenticateToken_LegacyTokenSkipsCache(t *testing.T) {
	repo := &fakeAuthUserRepo{user: newCacheTestUser()}
	sessions := &fakeSessionRepo{revoked: map[string]bool{}}
	service, tokens := newCacheTestService(repo, sessions)
	service.SetPrincipalCache(httpapi.NewPrincipalCache[Principal](time.Minute))

	// Token legado: emitido sem sessionID (sid ausente no JWT).
	token := issueTestToken(t, tokens, "", repo.user)

	if _, err := service.AuthenticateToken(context.Background(), token); err != nil {
		t.Fatalf("primeira autenticacao legada: %v", err)
	}
	if _, err := service.AuthenticateToken(context.Background(), token); err != nil {
		t.Fatalf("segunda autenticacao legada: %v", err)
	}

	if repo.loadCalls != 2 {
		t.Fatalf("token legado nunca usa cache; esperado 2 LoadUserForAuth, veio %d", repo.loadCalls)
	}
}

func TestAuthenticateToken_CacheDisabledKeepsCurrentBehavior(t *testing.T) {
	repo := &fakeAuthUserRepo{user: newCacheTestUser()}
	sessions := &fakeSessionRepo{revoked: map[string]bool{}}
	// Sem SetPrincipalCache: comportamento legado (sempre vai ao banco).
	service, tokens := newCacheTestService(repo, sessions)

	token := issueTestToken(t, tokens, "sess-1", repo.user)

	if _, err := service.AuthenticateToken(context.Background(), token); err != nil {
		t.Fatalf("primeira autenticacao: %v", err)
	}
	if _, err := service.AuthenticateToken(context.Background(), token); err != nil {
		t.Fatalf("segunda autenticacao: %v", err)
	}

	if repo.loadCalls != 2 {
		t.Fatalf("cache desligado deve ir ao banco sempre; esperado 2 LoadUserForAuth, veio %d", repo.loadCalls)
	}
}
