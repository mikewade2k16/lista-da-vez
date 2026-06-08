package auth

import (
	"context"
	"errors"
	"strings"

	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type authContextKey string

const principalContextKey authContextKey = "auth.principal"

type Middleware struct {
	service        *Service
	accountChecker AccountMemberChecker
}

func NewMiddleware(service *Service) *Middleware {
	return &Middleware{service: service}
}

// SetAccountChecker habilita RequireAuthWithAccount. Deve ser chamado antes
// de registrar rotas multi-tenant.
func (middleware *Middleware) SetAccountChecker(checker AccountMemberChecker) {
	middleware.accountChecker = checker
}

func (middleware *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := middleware.service.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			switch {
			case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrUserInactive):
				httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			default:
				httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar a sessao.")
			}
			return
		}

		ctx := context.WithValue(r.Context(), principalContextKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (middleware *Middleware) RequireRoles(next http.Handler, roles ...Role) http.Handler {
	return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		if !roleAllowed(principal.Role, roles...) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
			return
		}

		next.ServeHTTP(w, r)
	}))
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

// RequireAuthWithAccount autentica o usuario E valida que ele e membro ativo
// da account informada em X-Account-Id. Injeta AccountID no Principal do
// contexto para que handlers downstream leiam via PrincipalFromContext.
//
// Erros retornados:
//   - 401 unauthorized: token ausente ou invalido.
//   - 400 missing_account_id: header X-Account-Id ausente.
//   - 403 account_not_member: user nao e membro ativo da account.
//   - 500 internal_error: falha de banco ou checker nao configurado.
func (middleware *Middleware) RequireAuthWithAccount(next http.Handler) http.Handler {
	return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		if middleware.accountChecker == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Account checker nao configurado.")
			return
		}

		accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
		if accountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_account_id", "Header X-Account-Id e obrigatorio.")
			return
		}

		member, err := middleware.accountChecker.IsMember(r.Context(), accountID, principal.UserID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar membership.")
			return
		}
		if !member {
			httpapi.WriteError(w, r, http.StatusForbidden, "account_not_member", "Sem acesso a esta account.")
			return
		}

		principal.AccountID = accountID
		ctx := context.WithValue(r.Context(), principalContextKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

func ExtractBearerToken(header string) (string, error) {
	rawHeader := strings.TrimSpace(header)
	if rawHeader == "" {
		return "", ErrUnauthorized
	}

	parts := strings.SplitN(rawHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrUnauthorized
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrUnauthorized
	}

	return token, nil
}

func roleAllowed(current Role, allowedRoles ...Role) bool {
	if current == RolePlatformAdmin {
		return true
	}

	for _, allowed := range allowedRoles {
		if current == allowed {
			return true
		}
	}

	return false
}
