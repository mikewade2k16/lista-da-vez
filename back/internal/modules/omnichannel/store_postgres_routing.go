package omnichannel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// F8 — Persistencia do runtime: transicao sob lock, decisoes, visibilidade
// ============================================================================

// convSnapshot e o estado da conversa lido SOB LOCK no inicio da transicao (risco 5 do
// canonico). O service computa o estado destino a partir dele.
type convSnapshot struct {
	State            State
	QueueID          *string
	DepartmentID     *string
	AssignedUserID   *string
	ExtractedFields  json.RawMessage
	ContactPhone     *string
	InstanceScopeKey string
	ExternalID       string
}

// stateUpdate e o estado destino COMPLETO que o service manda gravar. O service e o dono
// de computar a tupla final (state + as tres FKs), nao o store — assim o store nao conhece
// as regras das notas. NoChange pula o UPDATE (no-op da matriz). BumpLastMessage atualiza
// last_message_at (celulas `self`).
type stateUpdate struct {
	State           State
	QueueID         *string
	DepartmentID    *string
	AssignedUserID  *string
	BumpLastMessage bool
	NoChange        bool
}

// decisionRecord e a linha de auditoria a inserir (nil = transicao sem decisao de roteamento).
type decisionRecord struct {
	RuleID             *string
	Outcome            string
	Reason             string
	Input              map[string]any
	TargetDepartmentID *string
	TargetQueueID      *string
}

// ApplyTransition roda a transicao numa UNICA transacao com `select ... for update`
// (Contrato 2, concorrencia). Le o snapshot sob lock, chama `decide` (que roda a maquina +
// motor), grava o estado destino e, se houver, a decisao — tudo atomico. Devolve a view
// atualizada. Conversa de outra conta => ErrNotFound (404).
func (s *Store) ApplyTransition(ctx context.Context, accountID, convID string,
	decide func(convSnapshot) (stateUpdate, *decisionRecord, error)) (conversationRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return conversationRow{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var snap convSnapshot
	err = tx.QueryRow(ctx, `select state, queue_id::text, department_id::text,
		assigned_user_id::text, extracted_fields, contact_phone, instance_scope_key, external_id
		from messaging.conversations
		where id = $1::uuid and account_id = $2::uuid
		for update`, convID, accountID).Scan(&snap.State, &snap.QueueID, &snap.DepartmentID,
		&snap.AssignedUserID, &snap.ExtractedFields, &snap.ContactPhone,
		&snap.InstanceScopeKey, &snap.ExternalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return conversationRow{}, ErrNotFound
		}
		return conversationRow{}, err
	}

	upd, dec, err := decide(snap)
	if err != nil {
		return conversationRow{}, err
	}

	if !upd.NoChange {
		if _, err := tx.Exec(ctx, `update messaging.conversations set
			state = $3,
			queue_id = $4::uuid,
			department_id = $5::uuid,
			assigned_user_id = $6::uuid,
			last_message_at = case when $7 then now() else last_message_at end,
			updated_at = now()
			where id = $1::uuid and account_id = $2::uuid`,
			convID, accountID, string(upd.State), upd.QueueID, upd.DepartmentID,
			upd.AssignedUserID, upd.BumpLastMessage); err != nil {
			return conversationRow{}, err
		}
	}

	if dec != nil {
		input, mErr := json.Marshal(dec.Input)
		if mErr != nil {
			return conversationRow{}, mErr
		}
		if _, err := tx.Exec(ctx, `insert into messaging.routing_decisions
			(account_id, conversation_id, rule_id, outcome, reason, input,
			 target_department_id, target_queue_id)
			values ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6::jsonb, $7::uuid, $8::uuid)`,
			accountID, convID, dec.RuleID, dec.Outcome, dec.Reason, input,
			dec.TargetDepartmentID, dec.TargetQueueID); err != nil {
			return conversationRow{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return conversationRow{}, err
	}
	return s.GetConversation(ctx, accountID, convID)
}

// ============================================================================
// Fonte do motor de roteamento (routingRuleSource)
// ============================================================================

// ActiveRoutingRules carrega as regras ativas prontas para avaliar, na ordem determinista
// `priority asc, id asc`. Junta a fila (target) para trazer o department_id e so inclui
// regras cuja fila destino esteja ativa (regra apontando para fila morta nao roteia).
func (s *Store) ActiveRoutingRules(ctx context.Context, accountID string) ([]compiledRule, error) {
	rows, err := s.pool.Query(ctx, `select r.id::text, r.name, r.target_queue_id::text,
		q.department_id::text, r.conditions
		from messaging.routing_rules r
		join messaging.queues q on q.id = r.target_queue_id and q.account_id = r.account_id
		where r.account_id = $1::uuid and r.is_active and q.is_active
		order by r.priority, r.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]compiledRule, 0)
	for rows.Next() {
		var r compiledRule
		var conditions []byte
		if err := rows.Scan(&r.RuleID, &r.Name, &r.QueueID, &r.DepartmentID, &conditions); err != nil {
			return nil, err
		}
		r.Conditions = make([]Condition, 0)
		if len(conditions) > 0 && string(conditions) != "null" {
			if err := json.Unmarshal(conditions, &r.Conditions); err != nil {
				return nil, err
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DefaultTarget resolve a fila default do setor default (ambos ativos). Sem default
// configurado => (_, false, nil): o motor cai em `unrouted` (Contrato 4, nota 7).
func (s *Store) DefaultTarget(ctx context.Context, accountID string) (routingTarget, bool, error) {
	var t routingTarget
	err := s.pool.QueryRow(ctx, `select q.id::text, d.id::text
		from messaging.departments d
		join messaging.queues q on q.department_id = d.id and q.account_id = d.account_id
		where d.account_id = $1::uuid and d.is_default and d.is_active
		  and q.is_default and q.is_active
		limit 1`, accountID).Scan(&t.QueueID, &t.DepartmentID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return routingTarget{}, false, nil
	case err != nil:
		return routingTarget{}, false, err
	default:
		return t, true, nil
	}
}

// ============================================================================
// Auditoria de decisoes
// ============================================================================

// ListRoutingDecisions devolve as decisoes de uma conversa, mais recente primeiro. O escopo
// (visibilidade) e checado ANTES no service; aqui o filtro de account e defesa em profundidade.
func (s *Store) ListRoutingDecisions(ctx context.Context, accountID, convID string) ([]RoutingDecisionView, error) {
	rows, err := s.pool.Query(ctx, `select id::text, conversation_id::text, rule_id::text,
		outcome, reason, target_department_id::text, target_queue_id::text, decided_at
		from messaging.routing_decisions
		where account_id = $1::uuid and conversation_id = $2::uuid
		order by decided_at desc`, accountID, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RoutingDecisionView, 0)
	for rows.Next() {
		var d RoutingDecisionView
		if err := rows.Scan(&d.ID, &d.ConversationID, &d.RuleID, &d.Outcome, &d.Reason,
			&d.TargetDepartmentID, &d.TargetQueueID, &d.DecidedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ============================================================================
// Gate de dado (visibilidade) e permissao efetiva
// ============================================================================

// visibilityWhere e o predicado do Contrato 5, aplicado no REPOSITORIO (defesa em
// profundidade, nao so no service/front). $1=account, $2=isBroad, $3=userID. Escopo amplo
// (platform_admin ou settings.manage) ve inclusive conversa unrouted (queue_id null).
const visibilityWhere = `c.account_id = $1::uuid and (
		$2
		or c.assigned_user_id = $3::uuid
		or (c.queue_id is not null and exists (
			select 1 from messaging.queue_members qm
			where qm.queue_id = c.queue_id and qm.user_id = $3::uuid and qm.is_active))
	)`

// GetVisibleConversation devolve a conversa SE o escopo a alcanca (Contrato 5). Fora do
// gate de dado => ErrNoRows -> ErrNotFound (404, nunca 403). Reutiliza conversationCols/
// lastMessageCol (F2) — mesma view do inbox.
func (s *Store) GetVisibleConversation(ctx context.Context, accountID string, scope VisibilityScope, convID string) (conversationRow, error) {
	query := `select ` + conversationCols + lastMessageCol + `
		from messaging.conversations c
		left join messaging.whatsapp_instances i
			on i.id = c.instance_id and i.account_id = c.account_id
		where ` + visibilityWhere + ` and c.id = $4::uuid`
	return scanConversation(s.pool.QueryRow(ctx, query, accountID, scope.IsBroad, scope.UserID, convID))
}

// HasSettingsManage responde a permissao efetiva `omnichannel.settings.manage` NA CONTA
// (padrao de service_calendar.go: role_permissions + overrides allow/deny). Decide o escopo
// amplo da visibilidade e a guarda de atribuicao (nota 8).
func (s *Store) HasSettingsManage(ctx context.Context, accountID, userID string) (bool, error) {
	return s.hasEffectivePermission(ctx, accountID, userID, "omnichannel.settings.manage")
}

func (s *Store) hasEffectivePermission(ctx context.Context, accountID, userID, key string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (
		select 1 from (
			select rp.permission_key
			from core.user_role_assignments ura
			join core.role_permissions rp on rp.role_id = ura.role_id
			join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
			where ura.account_id = $1::uuid and ura.user_id = $2::uuid
			union
			select permission_key from core.user_permission_overrides
			where account_id = $1::uuid and user_id = $2::uuid and effect = 'allow' and is_active = true
			except
			select permission_key from core.user_permission_overrides
			where account_id = $1::uuid and user_id = $2::uuid and effect = 'deny' and is_active = true
		) eff
		where eff.permission_key = $3)`, accountID, userID, key).Scan(&ok)
	return ok, err
}

// IsActiveQueueMember responde se o usuario e membro ativo da fila (guarda da nota 8).
func (s *Store) IsActiveQueueMember(ctx context.Context, accountID, queueID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from messaging.queue_members
		where account_id = $1::uuid and queue_id = $2::uuid and user_id = $3::uuid and is_active)`,
		accountID, queueID, userID).Scan(&ok)
	return ok, err
}

// queueDepartment resolve o department_id de uma fila ATIVA da conta (usado no
// queue.transfer). Fila inativa/de outra conta => ErrNoRows -> o service traduz para 404.
func (s *Store) queueDepartment(ctx context.Context, accountID, queueID string) (string, error) {
	var deptID string
	err := s.pool.QueryRow(ctx, `select department_id::text from messaging.queues
		where account_id = $1::uuid and id = $2::uuid and is_active`, accountID, queueID).Scan(&deptID)
	return deptID, err
}

// LatestMessageText devolve o conteudo da mensagem mais recente da conversa (o texto que o
// motor avalia como `message.text`). Sem mensagem => string vazia (nao e erro). Filtra por
// account (defesa em profundidade).
func (s *Store) LatestMessageText(ctx context.Context, accountID, convID string) (string, error) {
	var text string
	err := s.pool.QueryRow(ctx, `select content from messaging.messages
		where account_id = $1::uuid and conversation_id = $2::uuid
		order by created_at desc, id desc limit 1`, accountID, convID).Scan(&text)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return text, err
}
