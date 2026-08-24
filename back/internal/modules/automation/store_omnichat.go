package automation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// OmniChatConfig e a fonte autoritativa por account. Nao carrega segredo: a
// credencial e resolvida server-side no cofre global somente durante a execucao.
type OmniChatConfig struct {
	AccountID         string
	SourceAccountID   string
	SourceAccountName string
	Inherited         bool
	Enabled           bool
	SystemPrompt      string
	CredentialID      string
	Provider          string
	Model             string
	Temperature       float64
	HistoryWindow     int
	SurfaceModules    map[string]map[string]string
	UpdatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

const omniChatDirectConfigQuery = `select account_id::text, enabled, system_prompt,
		coalesce(credential_id::text, ''), provider, model,
		temperature::float8, history_window, surface_modules,
		coalesce(updated_by::text, ''), created_at, updated_at
	from automation.omni_chat_configs
	where account_id = $1::uuid`

const omniChatInheritedConfigQuery = `select consumer.id::text, config.enabled,
		config.system_prompt,coalesce(config.credential_id::text, ''),
		config.provider,config.model,config.temperature::float8,
		config.history_window,config.surface_modules,
		coalesce(config.updated_by::text, ''),config.created_at,config.updated_at,
		agency.id::text,agency.name
	from core.accounts consumer
	join lateral (
		select candidate.id,candidate.name
		from core.accounts candidate
		where candidate.organization_id=consumer.organization_id
		  and candidate.is_agency=true
		  and candidate.is_active
		order by candidate.created_at,candidate.id
		limit 1
	) agency on true
	join automation.omni_chat_configs config on config.account_id=agency.id
	where consumer.id=$1::uuid
	  and consumer.is_active
	  and consumer.is_agency=false`

func defaultOmniChatConfig(accountID string) OmniChatConfig {
	return OmniChatConfig{
		AccountID: accountID, SourceAccountID: accountID, Enabled: true, Provider: "openai",
		Model: "gpt-4.1-mini", Temperature: 0.2, HistoryWindow: defaultHistoryWindow,
		SurfaceModules: defaultAssistantSurfaceModules(),
	}
}

func (s *Store) GetOmniChatConfig(ctx context.Context, accountID string) (OmniChatConfig, error) {
	return getOmniChatConfig(ctx, s.pool, accountID, false)
}

// GetOmniChatConfigTx resolve a configuracao direta/herdada/default usando o
// snapshot da transacao chamadora. O lock de leitura evita que uma configuracao
// existente mude enquanto uma confirmacao de card ainda aplica seu efeito.
func (s *Store) GetOmniChatConfigTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID string,
) (OmniChatConfig, error) {
	return getOmniChatConfig(ctx, tx, accountID, true)
}

type omniChatConfigQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getOmniChatConfig(
	ctx context.Context,
	querier omniChatConfigQuerier,
	accountID string,
	lock bool,
) (OmniChatConfig, error) {
	directQuery := omniChatDirectConfigQuery
	inheritedQuery := omniChatInheritedConfigQuery
	if lock {
		directQuery += ` for share`
		inheritedQuery += ` for share of consumer, config`
	}
	config, err := scanOmniChatConfig(querier.QueryRow(ctx, directQuery, accountID), false)
	if errors.Is(err, pgx.ErrNoRows) {
		config, err = scanOmniChatConfig(querier.QueryRow(ctx, inheritedQuery, accountID), true)
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultOmniChatConfig(accountID), nil
		}
	}
	if err != nil {
		return config, err
	}
	return config, nil
}

func scanOmniChatConfig(row rowScanner, inherited bool) (OmniChatConfig, error) {
	var config OmniChatConfig
	var surfaceModulesRaw []byte
	dest := []any{
		&config.AccountID, &config.Enabled, &config.SystemPrompt,
		&config.CredentialID, &config.Provider, &config.Model,
		&config.Temperature, &config.HistoryWindow, &surfaceModulesRaw,
		&config.UpdatedBy, &config.CreatedAt, &config.UpdatedAt,
	}
	if inherited {
		dest = append(dest, &config.SourceAccountID, &config.SourceAccountName)
	}
	err := row.Scan(dest...)
	if err != nil {
		return config, err
	}
	if !inherited {
		config.SourceAccountID = config.AccountID
	}
	config.Inherited = inherited
	if json.Unmarshal(surfaceModulesRaw, &config.SurfaceModules) != nil {
		config.SurfaceModules = defaultAssistantSurfaceModules()
	}
	return config, nil
}

func (s *Store) SaveOmniChatConfig(ctx context.Context, config OmniChatConfig) (OmniChatConfig, error) {
	surfaceModulesRaw, err := json.Marshal(normalizeAssistantSurfaceModules(config.SurfaceModules))
	if err != nil {
		return OmniChatConfig{}, err
	}
	const q = `insert into automation.omni_chat_configs (
			account_id, enabled, system_prompt, credential_id, provider, model,
			temperature, history_window, surface_modules, updated_by
		) values (
			$1::uuid, $2, $3, nullif($4, '')::uuid, $5, $6, $7, $8,
			$9::jsonb, nullif($10, '')::uuid
		)
		on conflict (account_id) do update set
			enabled = excluded.enabled,
			system_prompt = excluded.system_prompt,
			credential_id = excluded.credential_id,
			provider = excluded.provider,
			model = excluded.model,
			temperature = excluded.temperature,
			history_window = excluded.history_window,
			surface_modules = excluded.surface_modules,
			updated_by = excluded.updated_by,
			updated_at = now()
		returning account_id::text, enabled, system_prompt,
			coalesce(credential_id::text, ''), provider, model,
			temperature::float8, history_window, surface_modules,
			coalesce(updated_by::text, ''),
			created_at, updated_at`
	saved, err := scanOmniChatConfig(s.pool.QueryRow(ctx, q,
		config.AccountID, config.Enabled, config.SystemPrompt, config.CredentialID,
		config.Provider, config.Model, config.Temperature, config.HistoryWindow,
		surfaceModulesRaw, config.UpdatedBy,
	), false)
	return saved, err
}
