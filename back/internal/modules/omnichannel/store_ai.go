package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// F9 — Persistencia dos agentes de IA (management): CRUD + publish/rollback + runs
// ============================================================================
//
// REGRA DA CASA, sem excecao: TODA query filtra por account_id, inclusive as que recebem um
// id ja validado no service (defesa em profundidade, principio 2). IDs sao string + cast no
// SQL ($1::uuid). A chave do provider (provider_key_ciphertext) NUNCA sai numa view/log.

const agentCols = `id::text, slug, name, enabled, active_version_id::text,
	provider_key_ciphertext, provider_key_last4, created_by, created_at, updated_at`

func scanAgent(row rowScanner) (agentRow, error) {
	var a agentRow
	err := row.Scan(&a.ID, &a.Slug, &a.Name, &a.Enabled, &a.ActiveVersionID,
		&a.ProviderKeyCipher, &a.ProviderKeyLast4, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

const versionCols = `id::text, agent_id::text, version, status, provider, model,
	temperature::float8, layers, output_schema, schema_version, published_at, published_by, created_at`

func scanVersion(row rowScanner) (versionRow, error) {
	var v versionRow
	err := row.Scan(&v.ID, &v.AgentID, &v.Version, &v.Status, &v.Provider, &v.Model,
		&v.Temperature, &v.Layers, &v.OutputSchema, &v.SchemaVersion,
		&v.PublishedAt, &v.PublishedBy, &v.CreatedAt)
	return v, err
}

// ============================================================================
// Agentes
// ============================================================================

// CreateAgent insere um agente. Slug repetido na conta => 23505 -> isUniqueViolation ->
// ErrConflict no service (409). Nasce sempre enabled=false, sem active_version_id.
func (s *Store) CreateAgent(ctx context.Context, accountID, slug, name string, enabled bool, createdBy string) (agentRow, error) {
	query := `insert into messaging.ai_agents (account_id, slug, name, enabled, created_by)
		values ($1::uuid, $2, $3, $4, $5)
		returning ` + agentCols
	return scanAgent(s.pool.QueryRow(ctx, query, accountID, slug, name, enabled, createdBy))
}

// ListAgents devolve os agentes da conta (ordem estavel por created_at).
func (s *Store) ListAgents(ctx context.Context, accountID string) ([]agentRow, error) {
	rows, err := s.pool.Query(ctx, `select `+agentCols+`
		from messaging.ai_agents where account_id = $1::uuid
		order by created_at, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]agentRow, 0)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAgent devolve um agente da conta. Agente de outra conta => pgx.ErrNoRows -> 404.
func (s *Store) GetAgent(ctx context.Context, accountID, id string) (agentRow, error) {
	return scanAgent(s.pool.QueryRow(ctx, `select `+agentCols+`
		from messaging.ai_agents where account_id = $1::uuid and id = $2::uuid`, accountID, id))
}

// agentPatch e o conjunto de campos gravaveis do PATCH /agents/{id}. nil = nao muda.
type agentPatch struct {
	Name             *string
	Enabled          *bool
	ProviderKeyCiph  *string // ciphertext ja produzido pelo secretbox (ou "" para limpar)
	ProviderKeyLast4 *string
}

// UpdateAgent aplica o patch (COALESCE por campo) e devolve o agente atualizado. Filtra por
// account (defesa em profundidade). Fora de escopo => pgx.ErrNoRows -> 404.
func (s *Store) UpdateAgent(ctx context.Context, accountID, id string, p agentPatch) (agentRow, error) {
	query := `update messaging.ai_agents set
		name = coalesce($3, name),
		enabled = coalesce($4, enabled),
		provider_key_ciphertext = coalesce($5, provider_key_ciphertext),
		provider_key_last4 = coalesce($6, provider_key_last4),
		updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		returning ` + agentCols
	return scanAgent(s.pool.QueryRow(ctx, query, accountID, id,
		p.Name, p.Enabled, p.ProviderKeyCiph, p.ProviderKeyLast4))
}

// ============================================================================
// Versoes (publish/rollback)
// ============================================================================

// CreateVersion cria um DRAFT com version = max(version)+1 do agente. Requer o agente da
// conta (subquery amarrada a account); agente fora de escopo => 0 linhas -> pgx.ErrNoRows.
func (s *Store) CreateVersion(ctx context.Context, accountID, agentID string, in AIVersionInput, schema, layers json.RawMessage) (versionRow, error) {
	query := `insert into messaging.ai_agent_versions
		(account_id, agent_id, version, status, provider, model, temperature,
		 layers, output_schema, schema_version)
		select $1::uuid, a.id, coalesce(max(v.version), 0) + 1, 'draft', $3, $4, $5,
		       $6::jsonb, $7::jsonb, $8
		from messaging.ai_agents a
		left join messaging.ai_agent_versions v on v.agent_id = a.id
		where a.account_id = $1::uuid and a.id = $2::uuid
		group by a.id
		returning ` + versionCols
	return scanVersion(s.pool.QueryRow(ctx, query, accountID, agentID,
		in.Provider, in.Model, in.Temperature, layers, schema, in.SchemaVersion))
}

// ListVersions devolve as versoes do agente, mais nova primeiro.
func (s *Store) ListVersions(ctx context.Context, accountID, agentID string) ([]versionRow, error) {
	rows, err := s.pool.Query(ctx, `select `+versionCols+`
		from messaging.ai_agent_versions
		where account_id = $1::uuid and agent_id = $2::uuid
		order by version desc`, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]versionRow, 0)
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersionByNumber resolve uma version pelo numero (path {v}). Amarrada a account+agent.
func (s *Store) GetVersionByNumber(ctx context.Context, accountID, agentID string, version int) (versionRow, error) {
	return scanVersion(s.pool.QueryRow(ctx, `select `+versionCols+`
		from messaging.ai_agent_versions
		where account_id = $1::uuid and agent_id = $2::uuid and version = $3`,
		accountID, agentID, version))
}

// GetVersionByID resolve uma version pelo id (rollback/simulate). Amarrada a account+agent.
func (s *Store) GetVersionByID(ctx context.Context, accountID, agentID, versionID string) (versionRow, error) {
	return scanVersion(s.pool.QueryRow(ctx, `select `+versionCols+`
		from messaging.ai_agent_versions
		where account_id = $1::uuid and agent_id = $2::uuid and id = $3::uuid`,
		accountID, agentID, versionID))
}

// PublishVersion publica a version {version} do agente e repointa active_version_id — numa
// UNICA transacao. Publicar NAO reescreve o prompt: marca status=published e aponta o ativo.
// Version/agente fora de escopo => pgx.ErrNoRows -> 404.
func (s *Store) PublishVersion(ctx context.Context, accountID, agentID string, version int, publishedBy string) (agentRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return agentRow{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var versionID string
	err = tx.QueryRow(ctx, `update messaging.ai_agent_versions set
		status = 'published',
		published_at = coalesce(published_at, now()),
		published_by = case when published_by = '' then $4 else published_by end
		where account_id = $1::uuid and agent_id = $2::uuid and version = $3
		returning id::text`, accountID, agentID, version, publishedBy).Scan(&versionID)
	if err != nil {
		return agentRow{}, err
	}

	row, err := s.repointActiveVersion(ctx, tx, accountID, agentID, versionID)
	if err != nil {
		return agentRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return agentRow{}, err
	}
	return row, nil
}

// RollbackAgent repointa active_version_id para uma version ja existente (imutabilidade: nao
// reescreve nada). A version precisa ser publicada e do agente/conta => senao 404/validacao.
func (s *Store) RollbackAgent(ctx context.Context, accountID, agentID, versionID string) (agentRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return agentRow{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var status string
	err = tx.QueryRow(ctx, `select status from messaging.ai_agent_versions
		where account_id = $1::uuid and agent_id = $2::uuid and id = $3::uuid`,
		accountID, agentID, versionID).Scan(&status)
	if err != nil {
		return agentRow{}, err
	}
	if status != versionPublished {
		return agentRow{}, ErrValidation
	}
	row, err := s.repointActiveVersion(ctx, tx, accountID, agentID, versionID)
	if err != nil {
		return agentRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return agentRow{}, err
	}
	return row, nil
}

// repointActiveVersion aponta ai_agents.active_version_id para versionID (dentro de uma tx).
func (s *Store) repointActiveVersion(ctx context.Context, tx pgx.Tx, accountID, agentID, versionID string) (agentRow, error) {
	return scanAgent(tx.QueryRow(ctx, `update messaging.ai_agents set
		active_version_id = $3::uuid, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		returning `+agentCols, accountID, agentID, versionID))
}

// ============================================================================
// Campos a coletar (collect_field_defs)
// ============================================================================

const collectFieldCols = `id::text, agent_id::text, key, label, field_type,
	enum_options, required, sort_order`

func scanCollectField(row rowScanner) (CollectFieldView, error) {
	var c CollectFieldView
	err := row.Scan(&c.ID, &c.AgentID, &c.Key, &c.Label, &c.FieldType,
		&c.EnumOptions, &c.Required, &c.SortOrder)
	return c, err
}

// CreateCollectField insere um campo. key repetida no agente => 23505 -> ErrConflict.
func (s *Store) CreateCollectField(ctx context.Context, accountID, agentID string, in CollectFieldInput, enum json.RawMessage) (CollectFieldView, error) {
	query := `insert into messaging.collect_field_defs
		(account_id, agent_id, key, label, field_type, enum_options, required, sort_order)
		select $1::uuid, a.id, $3, $4, $5, $6::jsonb, $7, $8
		from messaging.ai_agents a where a.account_id = $1::uuid and a.id = $2::uuid
		returning ` + collectFieldCols
	return scanCollectField(s.pool.QueryRow(ctx, query, accountID, agentID,
		in.Key, in.Label, in.FieldType, enum, in.Required, in.SortOrder))
}

// ListCollectFields devolve os campos do agente, na ordem de exibicao (sort_order, key).
func (s *Store) ListCollectFields(ctx context.Context, accountID, agentID string) ([]CollectFieldView, error) {
	rows, err := s.pool.Query(ctx, `select `+collectFieldCols+`
		from messaging.collect_field_defs
		where account_id = $1::uuid and agent_id = $2::uuid
		order by sort_order, key`, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CollectFieldView, 0)
	for rows.Next() {
		c, err := scanCollectField(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateCollectField aplica o patch (COALESCE por campo). Fora de escopo => pgx.ErrNoRows.
func (s *Store) UpdateCollectField(ctx context.Context, accountID, agentID, fieldID string, p CollectFieldPatch) (CollectFieldView, error) {
	var enum []byte
	if p.EnumOptions != nil {
		enum = []byte(*p.EnumOptions)
	}
	query := `update messaging.collect_field_defs set
		label = coalesce($4, label),
		field_type = coalesce($5, field_type),
		enum_options = coalesce($6::jsonb, enum_options),
		required = coalesce($7, required),
		sort_order = coalesce($8, sort_order)
		where account_id = $1::uuid and agent_id = $2::uuid and id = $3::uuid
		returning ` + collectFieldCols
	return scanCollectField(s.pool.QueryRow(ctx, query, accountID, agentID, fieldID,
		p.Label, p.FieldType, enum, p.Required, p.SortOrder))
}

// DeleteCollectField remove um campo do agente. Fora de escopo => 0 linhas -> ErrNotFound.
func (s *Store) DeleteCollectField(ctx context.Context, accountID, agentID, fieldID string) error {
	tag, err := s.pool.Exec(ctx, `delete from messaging.collect_field_defs
		where account_id = $1::uuid and agent_id = $2::uuid and id = $3::uuid`,
		accountID, agentID, fieldID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Trilha de runs (GET /agents/{id}/runs) — paginacao limit + beforeId (padrao F2)
// ============================================================================

const aiRunCols = `id::text, conversation_id::text, agent_id::text, agent_version_id::text,
	message_id::text, status, provider, model, schema_version, input, output,
	prompt_tokens, completion_tokens, total_tokens, cost_usd::float8, latency_ms, error, created_at`

func scanAIRun(row rowScanner) (AIRunView, error) {
	var r AIRunView
	err := row.Scan(&r.ID, &r.ConversationID, &r.AgentID, &r.AgentVersionID, &r.MessageID,
		&r.Status, &r.Provider, &r.Model, &r.SchemaVersion, &r.Input, &r.Output,
		&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.CostUSD, &r.LatencyMs,
		&r.Error, &r.CreatedAt)
	return r, err
}

// ListRuns devolve os runs do agente, mais recente primeiro, com paginacao limit + beforeId
// (mesmo padrao do historico da F2: resolve beforeId -> created_at e filtra por data).
func (s *Store) ListRuns(ctx context.Context, accountID, agentID string, limit int, beforeID string) ([]AIRunView, error) {
	query := `select ` + aiRunCols + ` from messaging.ai_runs
		where account_id = $1::uuid and agent_id = $2::uuid`
	args := []any{accountID, agentID}

	if strings.TrimSpace(beforeID) != "" {
		before, err := s.runCreatedAt(ctx, accountID, agentID, beforeID)
		if err != nil {
			return nil, err
		}
		if before != nil {
			args = append(args, *before)
			query += " and created_at < $" + strconv.Itoa(len(args)) + "::timestamptz"
		}
	}
	args = append(args, limit)
	query += " order by created_at desc, id desc limit $" + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AIRunView, 0, limit)
	for rows.Next() {
		r, err := scanAIRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// runCreatedAt traduz beforeId -> created_at (paginacao por data). Amarrado a account+agente:
// beforeId de outro escopo nao resolve e a pagina volta como se nao houvesse cursor.
func (s *Store) runCreatedAt(ctx context.Context, accountID, agentID, beforeID string) (*time.Time, error) {
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, `select created_at from messaging.ai_runs
		where account_id = $1::uuid and agent_id = $2::uuid and id = $3::uuid`,
		accountID, agentID, beforeID).Scan(&createdAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return &createdAt, nil
	}
}
