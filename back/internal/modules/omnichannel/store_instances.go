package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// AccountSlug resolve o slug da conta (para montar a URL do webhook que o provider chama
// de volta: /v1/webhooks/omnichannel/{provider}/{slug}). Leitura de plataforma (core.accounts),
// por contrato explicito — o modulo continua independente dos outros modulos satelite.
func (s *Store) AccountSlug(ctx context.Context, accountID string) (string, error) {
	var slug string
	err := s.pool.QueryRow(ctx, `select slug from core.accounts where id = $1::uuid`, accountID).Scan(&slug)
	return slug, err
}

// Persistencia da GESTAO de instancia (criar/atualizar/atribuir usuarios) + resolvedores
// para as operacoes (validate-endpoints, conversations/clear) e a limpeza de conversas.
// Mesma regra da casa: TODA query filtra por account_id.
//
// userScopePolicy/assignedUserIds vivem em provider_config (jsonb) — chaves proprias, ao
// lado do que o adapter usa (ex.: baseURL). NAO ha tabela nova (armadilha A3): reusa a
// coluna existente. O merge (`|| jsonb_build_object(...)`) preserva as demais chaves.

// instanceWrite carrega as colunas gravaveis de uma instancia (menos is_default, tratado a
// parte via PromoteDefault/SetInstanceNotDefault). Os *string nullable ja chegam resolvidos
// do service (nil = grava NULL; nao-nil = valor ja aparado).
type instanceWrite struct {
	InstanceName      string
	DisplayName       *string
	PhoneNumber       *string
	QueueLabel        *string
	ResponsibleUserID *string
	IsActive          bool
	UserScopePolicy   string
	Provider          string
}

// InsertInstance cria uma instancia gerenciada e devolve o id. O par (account_id,
// instance_name) tem indice unico (0200) e (account_id, phone_number) tem indice parcial
// unico (0201) — quem chama trata a violacao (nome vs numero) via mapInstanceWriteError.
func (s *Store) InsertInstance(ctx context.Context, accountID string, w instanceWrite, createdByUserID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `insert into messaging.whatsapp_instances
		(account_id, instance_name, display_name, phone_number, queue_label,
		 responsible_user_id, is_active, provider, provider_config, created_by_user_id)
		values ($1::uuid, $2, $3, $4, $5, nullif($6,'')::uuid, $7, $8,
			jsonb_build_object('userScopePolicy', $9::text), nullif($10,'')::uuid)
		returning id::text`,
		accountID, w.InstanceName, w.DisplayName, w.PhoneNumber, w.QueueLabel,
		deref(w.ResponsibleUserID), w.IsActive, w.Provider, w.UserScopePolicy, createdByUserID).Scan(&id)
	return id, err
}

// UpdateInstance aplica o PATCH das colunas gravaveis (full-replace do formulario, menos
// is_default e a credencial, que tem semantica propria no service). O merge do provider_config
// preserva as demais chaves (ex.: baseURL) e so sobrescreve userScopePolicy.
func (s *Store) UpdateInstance(ctx context.Context, accountID, id string, w instanceWrite) error {
	_, err := s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set instance_name = $3,
			display_name = $4,
			phone_number = $5,
			queue_label = $6,
			responsible_user_id = nullif($7,'')::uuid,
			is_active = $8,
			provider_config = provider_config || jsonb_build_object('userScopePolicy', $9::text),
			updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, id, w.InstanceName, w.DisplayName, w.PhoneNumber, w.QueueLabel,
		deref(w.ResponsibleUserID), w.IsActive, w.UserScopePolicy)
	return err
}

// SetInstanceNotDefault desmarca a instancia como default (o oposto de PromoteDefault).
// Usado quando o PATCH chega com isDefault=false.
func (s *Store) SetInstanceNotDefault(ctx context.Context, accountID, id string) error {
	_, err := s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set is_default = false, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, id)
	return err
}

// SetInstanceAssignedUsers grava os usuarios atribuidos em provider_config.assignedUserIds
// (PUT .../users). userIDs ja chega filtrado (so membros da conta) e desduplicado no service.
// Merge preserva as demais chaves do jsonb.
func (s *Store) SetInstanceAssignedUsers(ctx context.Context, accountID, id string, userIDs []string) error {
	if userIDs == nil {
		userIDs = []string{}
	}
	idsJSON, err := json.Marshal(userIDs)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set provider_config = provider_config || jsonb_build_object('assignedUserIds', $3::jsonb),
			updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, id, string(idsJSON))
	return err
}

// GetInstanceView le UMA instancia gerenciada da conta (a resposta de POST/PATCH/PUT users).
// Instancia de outra conta cai no filtro e volta pgx.ErrNoRows -> o service traduz 404.
func (s *Store) GetInstanceView(ctx context.Context, accountID, id string) (InstanceView, error) {
	query := `select ` + instanceCols + `
		from messaging.whatsapp_instances wi
		left join core.users ru on ru.id = wi.responsible_user_id
		where wi.account_id = $1::uuid and wi.id = $2::uuid`
	return scanInstance(s.pool.QueryRow(ctx, query, accountID, id))
}

// instanceOpsRow e o subconjunto que as operacoes (validate-endpoints, clear) precisam:
// nome, provider, config nao-secreto e se ha credencial.
type instanceOpsRow struct {
	ID             string
	InstanceName   string
	Provider       string
	Config         map[string]string
	HasCredentials bool
}

// ResolveInstanceForOps resolve a instancia por id, senao por nome, senao a default/1a
// ativa da conta. Serve validate-endpoints (instanceId OU instanceName no body) e o escopo
// por instancia do clear. Nenhuma correspondencia -> pgx.ErrNoRows -> 404.
func (s *Store) ResolveInstanceForOps(ctx context.Context, accountID, instanceID, instanceName string) (instanceOpsRow, error) {
	var r instanceOpsRow
	var config []byte
	err := s.pool.QueryRow(ctx, `select id::text, instance_name, provider,
			provider_config, (credentials_ciphertext is not null)
		from messaging.whatsapp_instances
		where account_id = $1::uuid
			and ($2::text = '' or id = $2::uuid)
			and ($3::text = '' or instance_name = $3)
		order by is_default desc, is_active desc, instance_name
		limit 1`,
		accountID, strings.TrimSpace(instanceID), strings.TrimSpace(instanceName)).
		Scan(&r.ID, &r.InstanceName, &r.Provider, &config, &r.HasCredentials)
	if err != nil {
		return instanceOpsRow{}, err
	}
	r.Config = decodeStringMap(config)
	return r, nil
}

// CountInstanceConversations conta conversas atreladas a uma instancia (guard do delete duro).
// Account-scoped: instancia/conversa de outra conta nunca entra na contagem.
func (s *Store) CountInstanceConversations(ctx context.Context, accountID, instanceID string) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `select count(*) from messaging.conversations
		where account_id = $1::uuid and instance_id = $2::uuid`, accountID, instanceID).Scan(&n)
	return n, err
}

// DeleteInstance remove a instancia da conta. Account-scoped — instancia de outra conta nunca
// casa (isolamento). O guard de conversas atreladas fica no service (DeleteInstance).
func (s *Store) DeleteInstance(ctx context.Context, accountID, instanceID string) error {
	_, err := s.pool.Exec(ctx, `delete from messaging.whatsapp_instances
		where account_id = $1::uuid and id = $2::uuid`, accountID, instanceID)
	return err
}

// IsAccountMember diz se o usuario e membro ATIVO da conta (isolamento: responsavel/
// atribuido de outra conta nao entra). Vazio ja e tratado como "sem responsavel" no service.
func (s *Store) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from core.account_users
		where account_id = $1::uuid and user_id = $2::uuid and is_active = true)`,
		accountID, userID).Scan(&ok)
	return ok, err
}

// FilterAccountMemberIDs devolve, na ordem da entrada e sem repetir, os ids que sao membros
// ATIVOS da conta. Ids de fora da conta sao descartados (isolamento: nunca persistir um
// usuario de outro tenant em assignedUserIds).
func (s *Store) FilterAccountMemberIDs(ctx context.Context, accountID string, ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `select user_id::text from core.account_users
		where account_id = $1::uuid and is_active = true and user_id = any($2::uuid[])`,
		accountID, dedupeStrings(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	valid := map[string]bool{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		valid[uid] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reconstroi na ordem da entrada, sem repetir, so os validos.
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || !valid[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// ClearConversations apaga o historico de conversas da conta (escopo tenant) ou de UMA
// instancia (escopo instancia, instanceID nao vazio), numa transacao. Devolve as contagens.
//
// Ordem deliberada: audit_events -> messages -> conversations. Apagar as folhas ANTES da
// conversa da contagem exata (o DELETE da conversa cascatearia messages e anularia
// audit_events, escondendo os numeros). hidden_messages/routing_decisions cascateiam.
// O predicado usa ($2::uuid is null or ...) — tenant quando instanceID e nil.
func (s *Store) ClearConversations(ctx context.Context, accountID string, instanceID *string) (deletedAudit, deletedMessages, deletedConversations int64, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	convScope := `($2::uuid is null or conversation_id in (
		select c.id from messaging.conversations c
		where c.account_id = $1::uuid and c.instance_id = $2::uuid))`

	auditTag, err := tx.Exec(ctx, `delete from messaging.audit_events
		where account_id = $1::uuid and `+convScope, accountID, instanceID)
	if err != nil {
		return 0, 0, 0, err
	}
	msgTag, err := tx.Exec(ctx, `delete from messaging.messages
		where account_id = $1::uuid and `+convScope, accountID, instanceID)
	if err != nil {
		return 0, 0, 0, err
	}
	convTag, err := tx.Exec(ctx, `delete from messaging.conversations
		where account_id = $1::uuid and ($2::uuid is null or instance_id = $2::uuid)`,
		accountID, instanceID)
	if err != nil {
		return 0, 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, 0, err
	}
	return auditTag.RowsAffected(), msgTag.RowsAffected(), convTag.RowsAffected(), nil
}

// ============================================================================
// Helpers
// ============================================================================

// parseInstanceScope extrai userScopePolicy/assignedUserIds do provider_config. Ausentes ou
// jsonb invalido caem nos defaults do legado (MULTI_INSTANCE, []). Nunca devolve slice nil
// (o front tipa assignedUserIds como array obrigatorio).
func parseInstanceScope(raw []byte) (policy string, ids []string) {
	policy = userScopePolicyMultiInstance
	ids = []string{}
	if len(raw) == 0 {
		return policy, ids
	}
	var cfg struct {
		UserScopePolicy string   `json:"userScopePolicy"`
		AssignedUserIDs []string `json:"assignedUserIds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return policy, ids
	}
	if strings.TrimSpace(cfg.UserScopePolicy) != "" {
		policy = cfg.UserScopePolicy
	}
	if cfg.AssignedUserIDs != nil {
		ids = cfg.AssignedUserIDs
	}
	return policy, ids
}

// mapInstanceWriteError traduz a violacao de indice unico do insert/update: o indice de
// telefone vira NumberInUseError (409 acionavel, mesmo path do number_guard); o de nome vira
// ErrInstanceNameConflict. Qualquer outro erro passa cru.
func mapInstanceWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	if strings.Contains(pgErr.ConstraintName, "phone") {
		return &NumberInUseError{InstanceName: "outra instancia"}
	}
	return ErrInstanceNameConflict
}

// dedupeStrings remove repetidos e vazios preservando a ordem (para o any($2::uuid[])).
func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// noRows normaliza pgx.ErrNoRows para o service (validate/clear resolvem instancia por id/nome).
func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
