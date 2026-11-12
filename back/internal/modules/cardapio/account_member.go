package cardapio

import (
	"context"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// accountMemberChecker confirma vinculo ativo do usuario com a account. Satisfeito
// por *auth.PostgresAccountMemberChecker (core.account_users + account ativa).
type accountMemberChecker interface {
	IsMember(ctx context.Context, accountID, userID string) (bool, error)
}

type memberAccountKey struct{}

// crossAccountKey guarda a account pedida em ?accountId= que NAO e a do usuario,
// ja confirmada no banco (membro ativo + cardapio ligado). Usuario de varios
// clientes abre o estabelecimento de outro cliente por aqui.
type crossAccountKey struct{}

// WithMembers injeta o checker de vinculo usado por withMemberAccount.
func (s *Service) WithMembers(checker accountMemberChecker) *Service {
	s.members = checker
	return s
}

// withMemberAccount cobre o usuario que entra SEM conta padrao no login (papel
// so do cardapio, sem papel-coarse de fila): o Principal chega sem AccountID e
// todo endpoint do painel daria 404. Quando o header X-Account-Id aponta para
// uma account da qual ele e membro ativo, essa account vira o escopo da
// requisicao. Nao muda nada para platform_admin nem para quem ja tem conta no
// Principal; a permissao fina (cardapio.view/manage) continua no gate por handler.
func (s *Service) withMemberAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		if account := s.memberAccountFor(ctx, principal, r.Header.Get("X-Account-Id")); account != "" {
			ctx = context.WithValue(ctx, memberAccountKey{}, account)
		}
		own := principalOwnAccount(r.WithContext(ctx), principal)
		if cross := s.crossAccountFor(ctx, principal, own, r.URL.Query().Get("accountId")); cross != "" {
			ctx = context.WithValue(ctx, crossAccountKey{}, cross)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// memberAccountFor decide a account de membro: so para nao-admin sem conta no
// Principal, com header informado e vinculo ativo confirmado no banco.
func (s *Service) memberAccountFor(ctx context.Context, principal auth.Principal, header string) string {
	if s.members == nil || principal.Role == auth.RolePlatformAdmin ||
		strings.TrimSpace(principal.AccountID) != "" || strings.TrimSpace(principal.TenantID) != "" {
		return ""
	}
	requested := strings.TrimSpace(header)
	if requested == "" {
		return ""
	}
	member, err := s.members.IsMember(ctx, requested, principal.UserID)
	if err != nil || !member {
		return ""
	}
	return requested
}

// crossAccountFor valida a account de OUTRO cliente pedida em ?accountId=: so
// para nao-admin, e so quando o usuario e membro ativo dela com o cardapio ligado.
func (s *Service) crossAccountFor(ctx context.Context, principal auth.Principal, own, requested string) string {
	requested = strings.TrimSpace(requested)
	if s.store == nil || principal.Role == auth.RolePlatformAdmin || requested == "" || requested == own {
		return ""
	}
	member, err := s.store.IsCardapioMember(ctx, requested, principal.UserID)
	if err != nil || !member {
		return ""
	}
	return requested
}

// verifiedCrossAccount devolve a account de outro cliente ja validada.
func verifiedCrossAccount(r *http.Request) string {
	value, _ := r.Context().Value(crossAccountKey{}).(string)
	return value
}

// panelAuth e o wrapper das rotas do painel: autentica e resolve a account de
// membro (withMemberAccount) antes do handler.
func panelAuth(svc *Service, middleware *auth.Middleware, h http.HandlerFunc) http.Handler {
	return middleware.RequireAuth(svc.withMemberAccount(h))
}

// principalOwnAccount devolve a account propria do Principal ou, na falta, a
// account de membro validada por withMemberAccount.
func principalOwnAccount(r *http.Request, principal auth.Principal) string {
	own := strings.TrimSpace(principal.AccountID)
	if own == "" {
		own = strings.TrimSpace(principal.TenantID)
	}
	if own == "" {
		if member, ok := r.Context().Value(memberAccountKey{}).(string); ok {
			own = member
		}
	}
	return own
}
