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
	service *Service
}

func NewMiddleware(service *Service) *Middleware {
	return &Middleware{service: service}
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
