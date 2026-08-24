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
// clientes que ele PODE ver (o contexto da IA nunca sai desse conjunto). VisibleClients =
// os MESMOS clientes com NOME (fonte do select de escopo): usado para nomear os clientes
// no contexto da IA mesmo quando eles ainda nao tem evento/perfil (loadAccountNames so
// nomeia cliente ja referenciado; sem isto um cliente visivel viajava sem nome e a IA nao
// conseguia cita-lo).
type ChatAccess struct {
	IsAgency         bool
	StorageAccountID string
	VisibleClientIDs []string
	VisibleClients   []ChatScopeClient
}

func (a ChatAccess) calendarAccountID(activeAccountID string) string {
	if accountID := strings.TrimSpace(a.StorageAccountID); accountID != "" {
		return accountID
	}
	return strings.TrimSpace(activeAccountID)
}

// clientNameByID indexa os clientes visiveis por id (nome ja trim). Alimenta o preenchimento
// de nome no contexto da IA sem reconsultar o banco (a lista ja veio permission-scoped).
func (a ChatAccess) clientNameByID() map[string]string {
	out := make(map[string]string, len(a.VisibleClients))
	for _, c := range a.VisibleClients {
		if id := strings.TrimSpace(c.ID); id != "" {
			out[id] = strings.TrimSpace(c.Name)
		}
	}
	return out
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
// 100% server-side. accountID vem do Principal/middleware (nunca do body). Mantem os NOMES
// dos clientes visiveis (access.VisibleClients) para o contexto da IA poder nomear todo
// cliente visivel, nao so os que ja tem evento/perfil.
func (s *Service) resolveChatAccess(ctx context.Context, principal auth.Principal, accountID string) (ChatAccess, error) {
	access, clients, err := s.resolveChatContext(ctx, principal, accountID)
	if err != nil {
		return ChatAccess{}, err
	}
	access.VisibleClients = clients
	return access, nil
}

// resolveChatContext resolve, numa UNICA ida ao tenants scope (evita 2x ListAccessible),
// o acesso (IsAgency + VisibleClientIDs) E a lista NOMEADA (id+name) dos clientes visiveis
// que alimenta o SELECT do front (contrato D3). accountID vem do Principal (nunca do body).
func (s *Service) resolveChatContext(ctx context.Context, principal auth.Principal, accountID string) (ChatAccess, []ChatScopeClient, error) {
	account := strings.TrimSpace(accountID)
	calendarScope, err := s.GetCalendarScope(ctx, account)
	if err != nil {
		return ChatAccess{}, nil, err
	}
	isAgency, err := s.store.IsAgencyOfAccount(ctx, account, principal.UserID)
	if err != nil {
		return ChatAccess{}, nil, err
	}
	rbacClients, err := s.visibleClients(ctx, principal)
	if err != nil {
		return ChatAccess{}, nil, err
	}
	clients := intersectChatScopeClients(calendarScope.Clients, rbacClients)
	ids := make([]string, 0, len(clients))
	for _, c := range clients {
		ids = append(ids, c.ID)
	}
	return ChatAccess{
		IsAgency:         isAgency && calendarScope.CanSelect,
		StorageAccountID: strings.TrimSpace(calendarScope.StorageAccountID),
		VisibleClientIDs: ids,
	}, clients, nil
}

// intersectChatScopeClients combina duas fontes autoritativas e independentes:
// GetCalendarScope limita a organization/storage da account ativa; ListAccessible
// aplica a RBAC do principal. Nomes e ordenacao sempre vem do escopo do Calendar,
// portanto uma listagem ampla de tenants nunca injeta cliente de outra organizacao.
func intersectChatScopeClients(calendarClients []CalendarScopeClient, rbacClients []ChatScopeClient) []ChatScopeClient {
	allowed := make(map[string]bool, len(rbacClients))
	for _, client := range rbacClients {
		if id := normalizeUUID(client.ID); id != "" {
			allowed[id] = true
		}
	}
	out := make([]ChatScopeClient, 0, len(calendarClients))
	seen := make(map[string]bool, len(calendarClients))
	for _, client := range calendarClients {
		id := normalizeUUID(client.ID)
		if id == "" || !allowed[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, ChatScopeClient{ID: id, Name: strings.TrimSpace(client.Name)})
	}
	return out
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
