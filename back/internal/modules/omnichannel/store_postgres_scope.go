package omnichannel

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// F7 — Escopo de instancia CORRIGIDO (spec OMNI-F7 C3) + reconciliacao do
// assigned_to_id (a coluna que o front verbatim le).
// ============================================================================
//
// O BUG do legado (whatsapp-instances.ts:681-683): o ternario
// `isTenantAdmin || active.length <= 1 ? active : active` devolve o MESMO valor nos dois
// ramos — o filtro por responsible_user_id NUNCA rodou e todo usuario ve todas as
// instancias. O indice (tenantId, responsibleUserId) existe e nao serve a ninguem.
//
// Aqui o filtro roda de verdade, no REPOSITORIO (defesa em profundidade, principio 2), com a
// intencao legivel reconstruida do proprio codigo morto (o guard `<= 1` evita trancar todos
// fora quando so ha um numero). DIVERGE DE PROPOSITO do legado — registrado em docs/LEGADO.md
// e no AGENT.md. O gate de fila (F8, queue_members) se soma a este com AND no service.

// instanceScopeRow e a tupla lida por AccessibleScopeKeys: o nome (= instance_scope_key) e
// se o usuario e o responsavel pela instancia.
// AccessibleScopeKeys resolve os instance_scope_key que o ator alcanca, conforme a tabela do
// C3 corrigido. Devolve (keys, unrestricted): unrestricted=true => sem filtro (admin ve todas
// as ativas); keys vazio com unrestricted=false => o ator nao ve NADA (fail-close — nunca cai
// em "ve tudo"). Uma unica query (sem N+1).
//
//	| Ator                                   | Instancias visiveis                     |
//	| admin da conta / platform_admin        | todas as ativas (unrestricted)          |
//	| qualquer usuario, conta com <= 1 ativa | a unica ativa                           |
//	| demais                                 | ativas com responsible_user_id = ator   |
func (s *Store) AccessibleScopeKeys(ctx context.Context, accountID, userID string, isAdmin bool) (keys []string, unrestricted bool, err error) {
	_ = isAdmin
	scope, err := s.LoadConversationAccessScope(ctx, accountID, userID)
	if err != nil || !scope.Eligible || !scope.allowsPermission("omnichannel.conversations.view") {
		return nil, false, err
	}
	return scope.conversationVisibility(InstanceGrantView).InstanceScopeKeys, false, nil
	// Guard `<= 1`: conta com no maximo uma instancia ativa nao tranca ninguem fora — todos
	// veem a unica ativa (0 ou 1 nome). Reconstroi a intencao do codigo morto do legado.
}

func appendConversationVisibility(query string, args []any, alias string, scope VisibilityScope) (string, []any) {
	args = append(args, scope.UserID, scope.InstanceScopeKeys, scope.ManageInstanceScopeKeys)
	userPos := strconv.Itoa(len(args) - 2)
	visiblePos := strconv.Itoa(len(args) - 1)
	managePos := strconv.Itoa(len(args))
	queueOrAssignment := `(` + alias + `.assigned_user_id=$` + userPos + `::uuid or (` +
		alias + `.queue_id is not null and exists (select 1 from messaging.queue_members scope_member
		where scope_member.account_id=` + alias + `.account_id and scope_member.queue_id=` + alias +
		`.queue_id and scope_member.user_id=$` + userPos + `::uuid and scope_member.is_active)))`
	query += ` and ((` + alias + `.channel='WHATSAPP'
		and ` + alias + `.instance_scope_key=any($` + visiblePos + `::text[])
		and (` + alias + `.instance_scope_key=any($` + managePos + `::text[]) or ` + queueOrAssignment + `))
		or (` + alias + `.channel<>'WHATSAPP' and ` + queueOrAssignment + `))`
	return query, args
}

func (s *Store) ListVisibleMessages(ctx context.Context, accountID, conversationID string, scope VisibilityScope, f MessagePageFilter) ([]MessageView, error) {
	query := `select ` + s.messageCols() + ` from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id=$1::uuid and m.conversation_id=$2::uuid`
	args := []any{accountID, conversationID}
	query, args = appendConversationVisibility(query, args, "c", scope)
	query += ` and not exists (select 1 from messaging.hidden_messages hidden
		where hidden.message_id=m.id and hidden.user_id=$3::uuid)` + s.historyVisibleMessagePredicate("m", "c")

	var before *messageCursor
	if strings.TrimSpace(f.BeforeCursor) != "" {
		decoded, err := decodeMessageCursor(f.BeforeCursor)
		if err != nil {
			return nil, err
		}
		before = &decoded
	} else if strings.TrimSpace(f.BeforeID) != "" {
		resolved, err := s.resolveBeforeMessage(ctx, accountID, conversationID, f.BeforeID)
		if err != nil {
			return nil, err
		}
		before = resolved
	}
	if before != nil {
		args = append(args, before.CreatedAt, before.ID)
		query += " and (m.created_at,m.id)<($" + strconv.Itoa(len(args)-1) + ",$" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, f.Limit)
	query += " order by m.created_at desc,m.id desc limit $" + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	desc := make([]MessageView, 0, f.Limit)
	for rows.Next() {
		message, scanErr := scanMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		desc = append(desc, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]MessageView, 0, len(desc))
	for index := len(desc) - 1; index >= 0; index-- {
		out = append(out, desc[index])
	}
	return out, nil
}

func (s *Store) HasOlderVisibleMessage(ctx context.Context, accountID, conversationID string, scope VisibilityScope, oldest time.Time, oldestID string) (bool, error) {
	query := `select exists(select 1 from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id=$1::uuid and m.conversation_id=$2::uuid`
	args := []any{accountID, conversationID}
	query, args = appendConversationVisibility(query, args, "c", scope)
	args = append(args, oldest, oldestID)
	query += " and (m.created_at,m.id)<($" + strconv.Itoa(len(args)-1) + ",$" + strconv.Itoa(len(args)) + `::uuid)
		and not exists (select 1 from messaging.hidden_messages hidden
			where hidden.message_id=m.id and hidden.user_id=$3::uuid)` +
		s.historyVisibleMessagePredicate("m", "c") + `)`
	var exists bool
	err := s.pool.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func (s *Store) GetVisibleMessage(ctx context.Context, accountID, conversationID, messageID string, scope VisibilityScope) (MessageView, error) {
	query := `select ` + s.messageCols() + ` from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id=$1::uuid and m.conversation_id=$2::uuid`
	args := []any{accountID, conversationID}
	query, args = appendConversationVisibility(query, args, "c", scope)
	args = append(args, messageID)
	query += " and m.id=$" + strconv.Itoa(len(args)) + `::uuid
		and not exists (select 1 from messaging.hidden_messages hidden
			where hidden.message_id=m.id and hidden.user_id=$3::uuid)` + s.historyVisibleMessagePredicate("m", "c")
	return scanMessage(s.pool.QueryRow(ctx, query, args...))
}

func (s *Store) RequireVisibleConversationForCompose(ctx context.Context, accountID, conversationID string, scope VisibilityScope) error {
	query := `select c.id::text from messaging.conversations c
		where c.account_id=$1::uuid and c.id=$2::uuid` + visibleConversationFilter
	args := []any{accountID, conversationID}
	query, args = appendConversationVisibility(query, args, "c", scope)
	var id string
	return s.pool.QueryRow(ctx, query, args...).Scan(&id)
}

// SyncAssignedToID reconcilia a coluna LEGADA assigned_to_id (texto, servida ao front
// verbatim como `assignedToId`) com a coluna AUTORITATIVA da maquina de estados
// assigned_user_id (uuid). A migration 0200 deixou explicito que "quem reconcilia e a
// F7/F8 via maquina de estados": a F8 (ApplyTransition) grava so assigned_user_id, entao a
// F7 espelha o valor no assigned_to_id apos assign/unassign para o inbox mostrar o dono.
//
// NAO e escrita de state/status (risco 4): assigned_to_id e projecao de exibicao, nao o
// ciclo de vida. assigned_user_id null => assigned_to_id null (desatribuicao limpa os dois).
func (s *Store) SyncAssignedToID(ctx context.Context, accountID, convID string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set assigned_to_id = assigned_user_id::text, updated_at = now()
		where id = $1::uuid and account_id = $2::uuid`, convID, accountID)
	return err
}
