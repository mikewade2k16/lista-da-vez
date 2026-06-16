package automation

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
)

// ErrInvalidModel sinaliza selecao invalida (modelo fora do catalogo ou funcao
// desconhecida). O handler traduz para 400.
var ErrInvalidModel = errors.New("modelo invalido para esta funcao")

// Models retorna o catalogo de opcoes + a selecao atual da automacao por funcao.
// Funcoes sem escolha explicita vem com o default (o mesmo que o n8n usa hoje),
// com as flags do catalogo embutidas para o painel aplicar as regras do MODELOS.md.
func (s *Service) Models(ctx context.Context, accountID string) (ModelsView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return ModelsView{}, err
	}
	catalog, err := s.store.ListCatalog(ctx)
	if err != nil {
		return ModelsView{}, err
	}
	selection, err := s.resolveSelection(ctx, a.ID, catalog)
	if err != nil {
		return ModelsView{}, err
	}
	catViews := make([]CatalogModelView, len(catalog))
	for i, m := range catalog {
		catViews[i] = toCatalogModelView(m)
	}
	return ModelsView{Catalog: catViews, Selection: selection}, nil
}

// SetModel grava a escolha de modelo para uma funcao da automacao, aplicando as
// regras do MODELOS.md no servidor (defesa em profundidade): valida o modelo
// contra o catalogo, recusa modelo de visao sem vision_ok e remove `temperature`
// de params quando o modelo nao aceita.
func (s *Service) SetModel(ctx context.Context, accountID, role, provider, modelID string, params json.RawMessage) (AutomationModelView, error) {
	if !slices.Contains(modelRoles, role) {
		return AutomationModelView{}, ErrInvalidModel
	}
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return AutomationModelView{}, err
	}
	cat, err := s.store.GetCatalogModel(ctx, provider, modelID, role)
	if err != nil {
		return AutomationModelView{}, ErrInvalidModel
	}
	if role == roleVision && !cat.VisionOK {
		return AutomationModelView{}, ErrInvalidModel
	}
	cleanParams := sanitizeParams(params, cat)
	saved, err := s.store.UpsertAutomationModel(ctx, a.ID, a.AccountID, role, provider, modelID, cleanParams)
	if err != nil {
		return AutomationModelView{}, err
	}
	return toAutomationModelView(saved, cat), nil
}

// resolveSelection junta a selecao persistida com os defaults: toda funcao em
// modelRoles aparece (com o modelo escolhido ou o default), cada uma enriquecida
// com as flags do catalogo.
func (s *Service) resolveSelection(ctx context.Context, automationID string, catalog []CatalogModel) ([]AutomationModelView, error) {
	saved, err := s.store.ListAutomationModels(ctx, automationID)
	if err != nil {
		return nil, err
	}
	byRole := make(map[string]AutomationModel, len(saved))
	for _, m := range saved {
		byRole[m.Role] = m
	}
	views := make([]AutomationModelView, 0, len(modelRoles))
	for _, role := range modelRoles {
		m, ok := byRole[role]
		if !ok {
			provider, modelID := defaultModelForRole(role)
			m = AutomationModel{Role: role, Provider: provider, ModelID: modelID, Params: json.RawMessage("{}")}
		}
		cat := findCatalog(catalog, m.Provider, m.ModelID, role)
		views = append(views, toAutomationModelView(m, cat))
	}
	return views, nil
}

func findCatalog(catalog []CatalogModel, provider, modelID, kind string) CatalogModel {
	for _, c := range catalog {
		if c.Provider == provider && c.ID == modelID && c.Kind == kind {
			return c
		}
	}
	// Fallback: modelo escolhido nao esta mais no catalogo (label vazio, flags neutras).
	return CatalogModel{ID: modelID, Provider: provider, Kind: kind, AcceptsTemperature: true}
}

func toAutomationModelView(m AutomationModel, cat CatalogModel) AutomationModelView {
	params := m.Params
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	return AutomationModelView{
		Role:                 m.Role,
		Provider:             m.Provider,
		ModelID:              m.ModelID,
		Label:                cat.Label,
		RequiresResponsesAPI: cat.RequiresResponsesAPI,
		AcceptsTemperature:   cat.AcceptsTemperature,
		VisionOK:             cat.VisionOK,
		Params:               params,
	}
}

// sanitizeParams remove `temperature` quando o modelo nao aceita (regra do
// MODELOS.md) e devolve sempre um JSON object valido.
func sanitizeParams(raw json.RawMessage, cat CatalogModel) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return json.RawMessage("{}")
	}
	if !cat.AcceptsTemperature {
		delete(obj, "temperature")
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}
