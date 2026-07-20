package omnichannel

import (
	"context"
	"encoding/json"
)

// ============================================================================
// F8 — Persistencia de membros de fila (gate de dado) e regras de roteamento
// ============================================================================

// AddQueueMember (re)ativa o vinculo atendente<->fila. Idempotente: se ja existe, reativa
// (on conflict). O usuario e validado contra core.account_users no service (fora da conta
// => 404). Devolve a view com nome/email (join core.users) para a tela nao refazer fetch.
func (s *Store) AddQueueMember(ctx context.Context, accountID, queueID, userID string) (QueueMemberView, error) {
	_, err := s.pool.Exec(ctx, `insert into messaging.queue_members
		(account_id, queue_id, user_id, is_active) values ($1::uuid, $2::uuid, $3::uuid, true)
		on conflict (queue_id, user_id) do update set is_active = true`,
		accountID, queueID, userID)
	if err != nil {
		return QueueMemberView{}, err
	}
	return s.getQueueMember(ctx, accountID, queueID, userID)
}

func (s *Store) getQueueMember(ctx context.Context, accountID, queueID, userID string) (QueueMemberView, error) {
	var m QueueMemberView
	err := s.pool.QueryRow(ctx, `select qm.id::text, qm.queue_id::text, qm.user_id::text,
		coalesce(u.display_name, u.email), u.email, qm.is_active, qm.created_at
		from messaging.queue_members qm
		join core.users u on u.id = qm.user_id
		where qm.account_id = $1::uuid and qm.queue_id = $2::uuid and qm.user_id = $3::uuid`,
		accountID, queueID, userID).Scan(&m.ID, &m.QueueID, &m.UserID, &m.UserName,
		&m.UserEmail, &m.IsActive, &m.CreatedAt)
	return m, err
}

// ListQueueMembers devolve os membros ATIVOS da fila (o gate de dado). Fila de outra conta
// => lista vazia (o filtro de account esconde).
func (s *Store) ListQueueMembers(ctx context.Context, accountID, queueID string) ([]QueueMemberView, error) {
	rows, err := s.pool.Query(ctx, `select qm.id::text, qm.queue_id::text, qm.user_id::text,
		coalesce(u.display_name, u.email), u.email, qm.is_active, qm.created_at
		from messaging.queue_members qm
		join core.users u on u.id = qm.user_id
		where qm.account_id = $1::uuid and qm.queue_id = $2::uuid and qm.is_active
		order by coalesce(u.display_name, u.email)`, accountID, queueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]QueueMemberView, 0)
	for rows.Next() {
		var m QueueMemberView
		if err := rows.Scan(&m.ID, &m.QueueID, &m.UserID, &m.UserName, &m.UserEmail,
			&m.IsActive, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RemoveQueueMember desativa o vinculo (is_active=false). Idempotente. Nao apaga: o
// historico do gate de dado fica. Vinculo inexistente => ErrNotFound (404).
func (s *Store) RemoveQueueMember(ctx context.Context, accountID, queueID, userID string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.queue_members set is_active = false
		where account_id = $1::uuid and queue_id = $2::uuid and user_id = $3::uuid and is_active`,
		accountID, queueID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// userInAccount valida que o usuario e membro ATIVO da conta (core.account_users). Usuario
// de fora da conta => 404 (spec Contrato 6: membro da fila so pode ser membro da conta).
func (s *Store) userInAccount(ctx context.Context, accountID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from core.account_users
		where account_id = $1::uuid and user_id = $2::uuid and is_active)`,
		accountID, userID).Scan(&ok)
	return ok, err
}

// ============================================================================
// Regras de roteamento
// ============================================================================

const routingRuleCols = `id::text, name, priority, is_active, conditions, target_queue_id::text, created_at, updated_at`

func scanRoutingRule(row rowScanner) (RoutingRuleView, error) {
	var r RoutingRuleView
	var conditions []byte
	if err := row.Scan(&r.ID, &r.Name, &r.Priority, &r.IsActive, &conditions,
		&r.TargetQueueID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return RoutingRuleView{}, err
	}
	// conditions e jsonb: desserializa para o array tipado. Vazio/null => slice vazio (nunca
	// nil, para o front receber [] e nao null).
	r.Conditions = make([]Condition, 0)
	if len(conditions) > 0 && string(conditions) != "null" {
		if err := json.Unmarshal(conditions, &r.Conditions); err != nil {
			return RoutingRuleView{}, err
		}
	}
	return r, nil
}

// CreateRoutingRule insere a regra. target_queue_id ja validado (ativo, da conta) no service.
func (s *Store) CreateRoutingRule(ctx context.Context, accountID string, in RoutingRuleInput, isActive bool) (RoutingRuleView, error) {
	conditions, err := json.Marshal(normalizeConditions(in.Conditions))
	if err != nil {
		return RoutingRuleView{}, err
	}
	return scanRoutingRule(s.pool.QueryRow(ctx, `insert into messaging.routing_rules
		(account_id, name, priority, is_active, conditions, target_queue_id)
		values ($1::uuid, $2, $3, $4, $5::jsonb, $6::uuid)
		returning `+routingRuleCols, accountID, in.Name, in.Priority, isActive, conditions, in.TargetQueueID))
}

// ListRoutingRules devolve as regras da conta na ordem de avaliacao (priority asc, id asc)
// — a MESMA que o motor usa, para a tela mostrar o que o roteamento fara.
func (s *Store) ListRoutingRules(ctx context.Context, accountID string) ([]RoutingRuleView, error) {
	rows, err := s.pool.Query(ctx, `select `+routingRuleCols+`
		from messaging.routing_rules where account_id = $1::uuid order by priority, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RoutingRuleView, 0)
	for rows.Next() {
		r, err := scanRoutingRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRoutingRule aplica o patch parcial. conditions substitui o array inteiro quando
// presente (nao e merge — o front manda a lista completa).
func (s *Store) UpdateRoutingRule(ctx context.Context, accountID, id string, patch RoutingRulePatch) (RoutingRuleView, error) {
	var conditions any
	if patch.Conditions != nil {
		raw, err := json.Marshal(normalizeConditions(*patch.Conditions))
		if err != nil {
			return RoutingRuleView{}, err
		}
		conditions = raw
	}
	return scanRoutingRule(s.pool.QueryRow(ctx, `update messaging.routing_rules set
		name = coalesce($3, name),
		priority = coalesce($4, priority),
		is_active = coalesce($5, is_active),
		conditions = coalesce($6::jsonb, conditions),
		target_queue_id = coalesce($7::uuid, target_queue_id),
		updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		returning `+routingRuleCols, accountID, id, patch.Name, patch.Priority,
		patch.IsActive, conditions, patch.TargetQueueID))
}

// SoftDeleteRoutingRule desativa a regra (is_active=false); some da avaliacao do motor.
func (s *Store) SoftDeleteRoutingRule(ctx context.Context, accountID, id string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.routing_rules set is_active = false, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderRoutingRules reescreve priority na ordem de ruleIDs, em UMA transacao (tudo ou
// nada). Se algum id nao for da conta, aborta e a ordem NAO muda (spec Contrato 6): conta
// os ids validos e compara com o tamanho pedido antes de gravar.
func (s *Store) ReorderRoutingRules(ctx context.Context, accountID string, ruleIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var found int
	if err := tx.QueryRow(ctx, `select count(*) from messaging.routing_rules
		where account_id = $1::uuid and id = any($2::uuid[])`, accountID, ruleIDs).Scan(&found); err != nil {
		return err
	}
	if found != len(ruleIDs) {
		return ErrNotFound // algum id nao e da conta => 404 e nada muda
	}
	for i, id := range ruleIDs {
		if _, err := tx.Exec(ctx, `update messaging.routing_rules set priority = $3, updated_at = now()
			where account_id = $1::uuid and id = $2::uuid`, accountID, id, (i+1)*10); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// normalizeConditions garante um slice nao-nil (jsonb '[]' e nao 'null') e preserva a ordem.
func normalizeConditions(in []Condition) []Condition {
	if in == nil {
		return []Condition{}
	}
	return in
}
