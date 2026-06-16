package automation

import (
	"context"
	"encoding/json"
)

// ListCatalog retorna o catalogo global de modelos habilitados, ordenado por
// kind e sort_order. Provider-agnostico (openai|anthropic|...).
func (s *Store) ListCatalog(ctx context.Context) ([]CatalogModel, error) {
	const q = `select id, provider, kind, label, requires_responses_api,
			accepts_temperature, vision_ok, enabled, sort_order
		from automation.model_catalog
		where enabled
		order by kind, sort_order, label`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []CatalogModel
	for rows.Next() {
		var m CatalogModel
		if err := rows.Scan(&m.ID, &m.Provider, &m.Kind, &m.Label, &m.RequiresResponsesAPI,
			&m.AcceptsTemperature, &m.VisionOK, &m.Enabled, &m.SortOrder); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, rows.Err()
}

// GetCatalogModel busca uma entrada especifica do catalogo (provider+id+kind).
// Retorna pgx.ErrNoRows se nao existir/estiver desabilitada.
func (s *Store) GetCatalogModel(ctx context.Context, provider, modelID, kind string) (CatalogModel, error) {
	const q = `select id, provider, kind, label, requires_responses_api,
			accepts_temperature, vision_ok, enabled, sort_order
		from automation.model_catalog
		where provider = $1 and id = $2 and kind = $3 and enabled`
	var m CatalogModel
	err := s.pool.QueryRow(ctx, q, provider, modelID, kind).Scan(
		&m.ID, &m.Provider, &m.Kind, &m.Label, &m.RequiresResponsesAPI,
		&m.AcceptsTemperature, &m.VisionOK, &m.Enabled, &m.SortOrder)
	return m, err
}

// ListAutomationModels retorna a selecao de modelos de uma automacao por funcao.
func (s *Store) ListAutomationModels(ctx context.Context, automationID string) ([]AutomationModel, error) {
	const q = `select automation_id, account_id, role, provider, model_id, params
		from automation.automation_models
		where automation_id = $1`
	rows, err := s.pool.Query(ctx, q, automationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var selection []AutomationModel
	for rows.Next() {
		var m AutomationModel
		var params []byte
		if err := rows.Scan(&m.AutomationID, &m.AccountID, &m.Role, &m.Provider, &m.ModelID, &params); err != nil {
			return nil, err
		}
		m.Params = json.RawMessage(params)
		selection = append(selection, m)
	}
	return selection, rows.Err()
}

// UpsertAutomationModel grava (insere/atualiza) a escolha de modelo para uma
// funcao da automacao. Filtra por automation_id na PK; account_id vem do caller
// (resolvido a partir do Principal), nunca do body.
func (s *Store) UpsertAutomationModel(ctx context.Context, automationID, accountID, role, provider, modelID string, params json.RawMessage) (AutomationModel, error) {
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	const q = `insert into automation.automation_models
			(automation_id, account_id, role, provider, model_id, params, updated_at)
		values ($1, $2, $3, $4, $5, $6::jsonb, now())
		on conflict (automation_id, role) do update
		set provider = excluded.provider,
		    model_id = excluded.model_id,
		    params = excluded.params,
		    updated_at = now()
		returning automation_id, account_id, role, provider, model_id, params`
	var m AutomationModel
	var out []byte
	err := s.pool.QueryRow(ctx, q, automationID, accountID, role, provider, modelID, string(params)).Scan(
		&m.AutomationID, &m.AccountID, &m.Role, &m.Provider, &m.ModelID, &out)
	m.Params = json.RawMessage(out)
	return m, err
}
