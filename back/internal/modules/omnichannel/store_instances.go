package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
	return s.CreateRestrictedInstanceWithManager(ctx, accountID, w, createdByUserID)
}

// UpdateInstance aplica o PATCH das colunas gravaveis (full-replace do formulario, menos
// is_default e a credencial, que tem semantica propria no service). O merge do provider_config
// preserva as demais chaves (ex.: baseURL) e so sobrescreve userScopePolicy.
func (s *Store) UpdateInstance(ctx context.Context, accountID, id string, w instanceWrite) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var previousName string
	if err := tx.QueryRow(ctx, `select instance_name from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid for update`, accountID, id).Scan(&previousName); err != nil {
		return err
	}
	// 0200 permitia conversation.instance_id NULL e usava instance_scope_key como chave real.
	// Repare o vínculo pela chave ANTIGA antes do rename para o cutoff não poder ressuscitar.
	if _, err := tx.Exec(ctx, `update messaging.conversations
		set instance_id=$2::uuid, updated_at=now()
		where account_id=$1::uuid and channel='WHATSAPP' and instance_id is null
		  and instance_scope_key=$3`, accountID, id, previousName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update messaging.whatsapp_instances
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
		deref(w.ResponsibleUserID), w.IsActive, w.UserScopePolicy); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetInstanceNotDefault desmarca a instancia como default (o oposto de PromoteDefault).
// Usado quando o PATCH chega com isDefault=false.
func (s *Store) SetInstanceNotDefault(ctx context.Context, accountID, id string) error {
	_, err := s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set is_default = false, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, id)
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
	err := s.pool.QueryRow(ctx, `select count(*)
		from messaging.conversations conversation
		join messaging.whatsapp_instances history_instance
		  on history_instance.account_id=conversation.account_id and history_instance.id=$2::uuid
		where conversation.account_id=$1::uuid and conversation.channel='WHATSAPP'
		  and (conversation.instance_id=history_instance.id
		    or (conversation.instance_id is null
		      and conversation.instance_scope_key=history_instance.instance_name))`,
		accountID, instanceID).Scan(&n)
	return n, err
}

// DeleteInstance remove a instancia da conta. Account-scoped — instancia de outra conta nunca
// casa (isolamento). O guard de conversas atreladas fica no service (DeleteInstance).
func (s *Store) DeleteInstance(ctx context.Context, accountID, instanceID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var instanceName string
	if err := tx.QueryRow(ctx, `select instance_name from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid for update`, accountID, instanceID).
		Scan(&instanceName); err != nil {
		return err
	}
	var count int64
	if err := tx.QueryRow(ctx, `select count(*) from messaging.conversations
		where account_id=$1::uuid and channel='WHATSAPP'
		  and (instance_id=$2::uuid or (instance_id is null and instance_scope_key=$3))`,
		accountID, instanceID, instanceName).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrInstanceHasConversations
	}
	if _, err := tx.Exec(ctx, `delete from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid`, accountID, instanceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// O reset de historico e estritamente por instancia e logico. Nao existe mais operacao de
// limpeza tenant-wide nem DELETE de mensagens/conversas/auditoria neste caminho.
type historyResetWrite struct {
	AccountID        string
	InstanceID       string
	ActorUserID      string
	Confirmation     string
	Reason           string
	ExpectedRevision int64
}

type historyResetResult struct {
	InstanceID string
	Cutoff     time.Time
	Revision   int64
}

// ResetInstanceHistory avanca o cutoff de UMA instancia e invalida somente efeitos operacionais
// antigos. Mensagens, conversas, handoffs, auditoria e analises permanecem fisicamente intactos.
func (s *Store) ResetInstanceHistory(ctx context.Context, in historyResetWrite) (historyResetResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return historyResetResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var instanceName string
	var previousCutoff *time.Time
	var currentRevision int64
	err = tx.QueryRow(ctx, `select instance_name, history_visible_from, history_reset_revision
		from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid
		for update`, in.AccountID, in.InstanceID).Scan(&instanceName, &previousCutoff, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return historyResetResult{}, ErrNotFound
	}
	if err != nil {
		return historyResetResult{}, err
	}
	if strings.TrimSpace(in.Confirmation) != strings.TrimSpace(instanceName) {
		return historyResetResult{}, ErrHistoryResetConfirmationMismatch
	}
	if currentRevision != in.ExpectedRevision {
		return historyResetResult{}, ErrHistoryResetRevisionConflict
	}

	result := historyResetResult{InstanceID: in.InstanceID}
	err = tx.QueryRow(ctx, `update messaging.whatsapp_instances
		set history_visible_from=statement_timestamp(),
			history_reset_revision=history_reset_revision+1,
			updated_at=now()
		where account_id=$1::uuid and id=$2::uuid
		returning history_visible_from, history_reset_revision`, in.AccountID, in.InstanceID).
		Scan(&result.Cutoff, &result.Revision)
	if err != nil {
		return historyResetResult{}, err
	}
	result.Cutoff = result.Cutoff.UTC()

	auditPayload := map[string]any{
		"actorUserId": in.ActorUserID, "accountId": in.AccountID,
		"instanceId": in.InstanceID, "instanceName": instanceName,
		"previousCutoff": previousCutoff, "newCutoff": result.Cutoff,
		"previousRevision": currentRevision, "newRevision": result.Revision,
	}
	if in.Reason != "" {
		auditPayload["reason"] = in.Reason
	}
	rawAudit, err := json.Marshal(auditPayload)
	if err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events
		(account_id,actor_user_id,event_type,payload_json)
		values ($1::uuid,$2::uuid,'WHATSAPP_INSTANCE_HISTORY_RESET',$3::jsonb)`,
		in.AccountID, in.ActorUserID, string(rawAudit)); err != nil {
		return historyResetResult{}, err
	}

	if _, err := tx.Exec(ctx, `update messaging.conversations
		set instance_id=coalesce(instance_id,$2::uuid), ai_generation=ai_generation+1,
		    extracted_fields='{}'::jsonb, updated_at=now()
		where account_id=$1::uuid and channel='WHATSAPP'
		  and (instance_id=$2::uuid or (instance_id is null and instance_scope_key=$3))`,
		in.AccountID, in.InstanceID, instanceName); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_dispatches dispatch
		set status='cancelled', last_error='history_reset', locked_at=null, updated_at=now()
		from messaging.conversations conversation
		where dispatch.account_id=$1::uuid
		  and conversation.account_id=dispatch.account_id
		  and conversation.id=dispatch.conversation_id
		  and conversation.channel='WHATSAPP'
		  and (conversation.instance_id=$2::uuid
		    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
		  and dispatch.generation < conversation.ai_generation
		  and dispatch.status in ('buffering','queued','processing')`,
		in.AccountID, in.InstanceID, instanceName); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_reply_drafts draft
		set status='expired',decision_reason='history_reset',decided_at=now(),updated_at=now()
		from messaging.conversations conversation
		where draft.account_id=$1::uuid and draft.account_id=conversation.account_id
		  and draft.conversation_id=conversation.id and draft.status='pending'
		  and conversation.channel='WHATSAPP'
		  and (conversation.instance_id=$2::uuid
		    or (conversation.instance_id is null and conversation.instance_scope_key=$3))`,
		in.AccountID, in.InstanceID, instanceName); err != nil {
		return historyResetResult{}, err
	}

	// Jobs de dispatch usam dispatchId; outbound e media usam messageId. As linhas sao
	// preservadas e apenas levadas ao estado terminal ja suportado.
	if _, err := tx.Exec(ctx, `update messaging.outbox job
		set status='dead', last_error='history_reset', locked_at=null, locked_by='', updated_at=now()
		where job.account_id=$1::uuid and job.status='pending' and (
			exists (select 1 from messaging.ai_dispatches dispatch
				join messaging.conversations conversation
				  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
				where dispatch.account_id=job.account_id and conversation.channel='WHATSAPP'
				  and (conversation.instance_id=$2::uuid
				    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
				  and dispatch.status='cancelled' and dispatch.last_error='history_reset'
				  and job.payload->>'dispatchId'=dispatch.id::text)
			or exists (select 1 from messaging.messages message
				join messaging.conversations conversation
				  on conversation.account_id=message.account_id and conversation.id=message.conversation_id
				where message.account_id=job.account_id and conversation.channel='WHATSAPP'
				  and (conversation.instance_id=$2::uuid
				    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
				  and (message.created_at <= $4 or (message.origin='ai' and message.status='PENDING'
				    and coalesce(case when message.metadata_json->>'aiGeneration' ~ '^[0-9]+$'
				      then (message.metadata_json->>'aiGeneration')::bigint end,-1) < conversation.ai_generation))
				  and job.payload->>'messageId'=message.id::text)
		)`, in.AccountID, in.InstanceID, instanceName, result.Cutoff); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.intelligence_outbox job
		set status='dead', last_error='history_reset', locked_at=null, locked_by='', updated_at=now()
		where job.account_id=$1::uuid and job.status='pending'
		  and exists (select 1 from messaging.ai_dispatches dispatch
			join messaging.conversations conversation
			  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
			where dispatch.account_id=job.account_id
			  and conversation.channel='WHATSAPP'
			  and (conversation.instance_id=$2::uuid
			    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
			  and dispatch.generation < conversation.ai_generation
			  and job.payload->>'dispatchId'=dispatch.id::text)`,
		in.AccountID, in.InstanceID, instanceName); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.customer_data_outbox job
		set status='dead', last_error='history_reset', locked_at=null, locked_by='', updated_at=now()
		where job.account_id=$1::uuid and job.status='pending'
		  and exists (select 1 from messaging.messages message
			join messaging.conversations conversation
			  on conversation.account_id=message.account_id and conversation.id=message.conversation_id
			where message.account_id=job.account_id
			  and conversation.channel='WHATSAPP'
			  and (conversation.instance_id=$2::uuid
			    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
			  and message.created_at <= $4
			  and job.payload->>'messageId'=message.id::text)`,
		in.AccountID, in.InstanceID, instanceName, result.Cutoff); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_tool_approvals approval
		set status='expired', reason='history_reset', decided_at=now(), decided_by=null
		from messaging.ai_tool_runs tool_run
		join messaging.ai_dispatches dispatch
		  on dispatch.account_id=tool_run.account_id and dispatch.id=tool_run.dispatch_id
		join messaging.conversations conversation
		  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
		where approval.account_id=$1::uuid and approval.account_id=tool_run.account_id
		  and approval.tool_run_id=tool_run.id and approval.status='pending'
		  and conversation.channel='WHATSAPP'
		  and (conversation.instance_id=$2::uuid
		    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
		  and dispatch.generation < conversation.ai_generation`,
		in.AccountID, in.InstanceID, instanceName); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_tool_runs tool_run
		set status='failed', error='history_reset', completed_at=now()
		from messaging.ai_dispatches dispatch
		join messaging.conversations conversation
		  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
		where tool_run.account_id=$1::uuid and tool_run.account_id=dispatch.account_id
		  and tool_run.dispatch_id=dispatch.id
		  and tool_run.status in ('requested','approved','running')
		  and conversation.channel='WHATSAPP'
		  and (conversation.instance_id=$2::uuid
		    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
		  and dispatch.generation < conversation.ai_generation`,
		in.AccountID, in.InstanceID, instanceName); err != nil {
		return historyResetResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.messages message
		set status='FAILED', provider_error_code='history_reset', updated_at=now()
		from messaging.conversations conversation
		where message.account_id=$1::uuid
		  and conversation.account_id=message.account_id and conversation.id=message.conversation_id
		  and conversation.channel='WHATSAPP'
		  and (conversation.instance_id=$2::uuid
		    or (conversation.instance_id is null and conversation.instance_scope_key=$3))
		  and (message.created_at <= $4 or (message.origin='ai'
		    and coalesce(case when message.metadata_json->>'aiGeneration' ~ '^[0-9]+$'
		      then (message.metadata_json->>'aiGeneration')::bigint end,-1) < conversation.ai_generation))
		  and message.direction='OUTBOUND' and message.status='PENDING'`,
		in.AccountID, in.InstanceID, instanceName, result.Cutoff); err != nil {
		return historyResetResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return historyResetResult{}, err
	}
	return result, nil
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

// noRows normaliza pgx.ErrNoRows para o service (validate/clear resolvem instancia por id/nome).
func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
