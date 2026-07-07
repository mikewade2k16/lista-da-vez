package calendar

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
)

// Controle de acesso do chat com memoria (WAVE 4, contrato D2). TUDO resolvido
// server-side pela permissao (nunca do body): quem ve quais conversas e quais clientes
// entram no contexto da IA. Fonte da verdade:
//   - IsAgency          -> IsAgencyOfAccount (chat_store.go), espelho de auth/account_checker.
//   - VisibleClientIDs  -> a MESMA lista de /v1/tenants (tenants.Service.ListAccessible /
//     scope_queries.go), reusada via clientScopeLister — sem duplicar a query de escopo.

// Escopo de uma conversa (coluna scope_mode). 'client' = um cliente especifico;
// 'all' = todos os clientes visiveis (agencia). "Organizacao" fica p/ depois (D2).
const (
	chatScopeClient = "client"
	chatScopeAll    = "all"
)

// clientScopeLister resolve os clientes visiveis ao principal reusando a MESMA fonte de
// /v1/tenants (tenants.Service.ListAccessible -> scope_queries.go: platform_admin ve
// todas as contas; owner/director/marketing pelas suas core.roles; demais pelo escopo de
// loja). Injetado no Build (WithClientScope) para nao duplicar a query org-aware. nil =>
// nenhum cliente visivel (fecha o select).
type clientScopeLister interface {
	ListAccessible(ctx context.Context, principal auth.Principal, input tenants.ListInput) ([]tenants.TenantView, error)
}

// WithClientScope injeta o resolvedor de clientes visiveis (contrato D2). Encadeavel no
// Build, no mesmo estilo de WithAI/WithChat/WithTasks. nil = mantem sem clientes visiveis.
func (s *Service) WithClientScope(l clientScopeLister) *Service {
	if l != nil {
		s.clientScope = l
	}
	return s
}

// ChatAccess e o veredito de acesso do usuario ao chat de UMA account (contrato D2).
// IsAgency = ve TODAS as conversas da conta (senao so as suas). VisibleClientIDs = os
// clientes que ele PODE ver (o contexto da IA nunca sai desse conjunto).
type ChatAccess struct {
	IsAgency         bool
	VisibleClientIDs []string
}

// canSelectScope: o front so mostra o SELECT de escopo quando o usuario e agencia OU
// enxerga mais de um cliente. Com 1 cliente (cliente-side), a IA fica travada nele.
func (a ChatAccess) canSelectScope() bool {
	return a.IsAgency || len(a.VisibleClientIDs) > 1
}

// lockedClientID e o unico cliente visivel quando o select esta escondido (cliente-side);
// vazio quando ha select (agencia / multi-cliente) ou nenhum cliente.
func (a ChatAccess) lockedClientID() string {
	if a.canSelectScope() {
		return ""
	}
	if len(a.VisibleClientIDs) == 1 {
		return a.VisibleClientIDs[0]
	}
	return ""
}

// canSeeClient informa se o cliente esta no conjunto visivel (nunca da contexto de
// cliente que o usuario nao pode ver).
func (a ChatAccess) canSeeClient(clientID string) bool {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return false
	}
	for _, id := range a.VisibleClientIDs {
		if id == clientID {
			return true
		}
	}
	return false
}

// validateScope normaliza (scopeMode, scopeClientID) contra o acesso (contrato D2), sem
// confiar no body:
//   - cliente-side (sem select): ignora o body e trava no lockedClientID (unico visivel);
//     sem cliente visivel => ErrInvalidClient.
//   - modo 'all': so quando canSelectScope; devolve o par (all, vazio).
//   - modo 'client' (default): exige scopeClientID em VisibleClientIDs, senao ErrInvalidClient.
//
// Devolve o par normalizado (scope_mode, scope_client_id) para gravar na conversa.
func (a ChatAccess) validateScope(scopeMode, scopeClientID string) (string, string, error) {
	clientID := normalizeUUID(scopeClientID)
	if !a.canSelectScope() {
		locked := a.lockedClientID()
		if locked == "" {
			return "", "", ErrInvalidClient
		}
		return chatScopeClient, locked, nil
	}
	if strings.ToLower(strings.TrimSpace(scopeMode)) == chatScopeAll {
		return chatScopeAll, "", nil
	}
	if !a.canSeeClient(clientID) {
		return "", "", ErrInvalidClient
	}
	return chatScopeClient, clientID, nil
}

// canAccessSavedScope decide se o usuario pode REABRIR uma conversa com o escopo JA SALVO,
// contra o acesso ATUAL (WAVE 4, defesa contra perda de acesso pos-criacao). Diferente de
// validateScope (que NORMALIZA o escopo de um ask novo, forcando o cliente-side no locked),
// aqui NAO reescreve: 'all' exige o select (agencia ou >1 cliente); 'client' exige ver AQUELE
// cliente. Se falhar, o caller nega (404) — evita replay do historico de um cliente que o
// usuario nao pode mais ver (e evita expor o contexto/mensagens de escopo fora do acesso).
func (a ChatAccess) canAccessSavedScope(scopeMode, scopeClientID string) bool {
	if strings.ToLower(strings.TrimSpace(scopeMode)) == chatScopeAll {
		return a.canSelectScope()
	}
	return a.canSeeClient(scopeClientID)
}

// resolveChatAccess resolve o acesso do principal ao chat da account (contrato D2),
// 100% server-side. accountID vem do Principal/middleware (nunca do body). Descarta os
// NOMES dos clientes (o ask/create so precisa dos ids); o ChatScope usa resolveChatContext
// para os nomes que alimentam o SELECT.
func (s *Service) resolveChatAccess(ctx context.Context, principal auth.Principal, accountID string) (ChatAccess, error) {
	access, _, err := s.resolveChatContext(ctx, principal, accountID)
	return access, err
}

// resolveChatContext resolve, numa UNICA ida ao tenants scope (evita 2x ListAccessible),
// o acesso (IsAgency + VisibleClientIDs) E a lista NOMEADA (id+name) dos clientes visiveis
// que alimenta o SELECT do front (contrato D3). accountID vem do Principal (nunca do body).
func (s *Service) resolveChatContext(ctx context.Context, principal auth.Principal, accountID string) (ChatAccess, []ChatScopeClient, error) {
	account := strings.TrimSpace(accountID)
	isAgency, err := s.store.IsAgencyOfAccount(ctx, account, principal.UserID)
	if err != nil {
		return ChatAccess{}, nil, err
	}
	clients, err := s.visibleClients(ctx, principal)
	if err != nil {
		return ChatAccess{}, nil, err
	}
	ids := make([]string, 0, len(clients))
	for _, c := range clients {
		ids = append(ids, c.ID)
	}
	return ChatAccess{IsAgency: isAgency, VisibleClientIDs: ids}, clients, nil
}

// visibleClients reusa a lista permission-scoped de /v1/tenants (MESMA fonte do select) e
// projeta id+name (UUID normalizado, nome trim; dedup implicito pela fonte). clientScope
// nil (nao injetado) => lista vazia (fecha o select). Nao confia na lista do front.
func (s *Service) visibleClients(ctx context.Context, principal auth.Principal) ([]ChatScopeClient, error) {
	if s.clientScope == nil {
		return []ChatScopeClient{}, nil
	}
	clients, err := s.clientScope.ListAccessible(ctx, principal, tenants.ListInput{})
	if err != nil {
		return nil, err
	}
	out := make([]ChatScopeClient, 0, len(clients))
	for _, c := range clients {
		if id := normalizeUUID(c.ID); id != "" {
			out = append(out, ChatScopeClient{ID: id, Name: strings.TrimSpace(c.Name)})
		}
	}
	return out, nil
}

// authorizeConversation le a conversa e aplica a regra de acesso (contrato D2/D3): o
// dono SEMPRE ve a sua; a agencia ve qualquer uma da conta. Fora disso => ErrNotFound
// (404, nao 403 — nao vaza existencia da conversa de outro usuario). accountID amarra o
// escopo (conversa de outra conta ja volta ErrNoRows do store).
func (s *Service) authorizeConversation(ctx context.Context, access ChatAccess, accountID, conversationID, requesterUserID string) (ChatConversation, error) {
	conv, err := s.store.GetConversation(ctx, strings.TrimSpace(conversationID), strings.TrimSpace(accountID))
	if err != nil {
		return ChatConversation{}, mapNotFound(err)
	}
	if !access.IsAgency && conv.CreatedByUserID != strings.TrimSpace(requesterUserID) {
		return ChatConversation{}, ErrNotFound
	}
	return conv, nil
}
