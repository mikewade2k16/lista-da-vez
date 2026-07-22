package omnichannel

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// ============================================================================
// F9 — Superficie HTTP do agente de IA (spec OMNI-F9.md, Contrato C9.5)
// ============================================================================
//
// RegisterAIRoutes NAO e chamada em module.go/http.go: a F5 edita esses arquivos em paralelo,
// entao o orquestrador costura estas rotas (needsWiring). Todas sob /v1/omnichannel, com
// RequireAuthWithAccount (injeta AccountID validado no Principal a partir do X-Account-Id).
// A permissao por feature (agents.manage / audit.view) e checada no service (403); escopo/
// recurso de outra conta => 404 (nunca 403, enumeration).
//
// A F10 CONSOME estas rotas (nao recria). Os paths /agents/* sao os definitivos (NAO existe
// /ai-agents/*). Publish e POST /agents/{id}/versions/{v}/publish (versao no path).
func RegisterAIRoutes(mux *http.ServeMux, svc *AIService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}

	mux.Handle("GET /v1/omnichannel/agents", wrap(handleListAgents(svc)))
	mux.Handle("POST /v1/omnichannel/agents", wrap(handleCreateAgent(svc)))
	mux.Handle("GET /v1/omnichannel/agents/{id}", wrap(handleGetAgent(svc)))
	mux.Handle("PATCH /v1/omnichannel/agents/{id}", wrap(handleUpdateAgent(svc)))
	mux.Handle("GET /v1/omnichannel/agents/{id}/models", wrap(handleListAgentModels(svc)))
	mux.Handle("GET /v1/omnichannel/agents/{id}/provider-keys", wrap(handleListProviderKeys(svc)))
	mux.Handle("PUT /v1/omnichannel/agents/{id}/provider-keys/{provider}", wrap(handlePutProviderKey(svc)))
	mux.Handle("DELETE /v1/omnichannel/agents/{id}/provider-keys/{provider}", wrap(handleDeleteProviderKey(svc)))

	mux.Handle("GET /v1/omnichannel/agents/{id}/versions", wrap(handleListVersions(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/versions", wrap(handleCreateVersion(svc)))
	mux.Handle("PUT /v1/omnichannel/agents/{id}/configuration", wrap(handleSaveConfiguration(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/versions/{v}/publish", wrap(handlePublishVersion(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/rollback", wrap(handleRollback(svc)))

	mux.Handle("GET /v1/omnichannel/agents/{id}/collect-fields", wrap(handleListCollectFields(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/collect-fields", wrap(handleCreateCollectField(svc)))
	mux.Handle("PATCH /v1/omnichannel/agents/{id}/collect-fields/{fieldId}", wrap(handleUpdateCollectField(svc)))
	mux.Handle("DELETE /v1/omnichannel/agents/{id}/collect-fields/{fieldId}", wrap(handleDeleteCollectField(svc)))

	mux.Handle("GET /v1/omnichannel/agents/{id}/tool-bindings", wrap(handleListAIToolBindings(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/tool-bindings", wrap(handleCreateAIToolBinding(svc)))
	mux.Handle("PATCH /v1/omnichannel/agents/{id}/tool-bindings/{bindingId}", wrap(handleUpdateAIToolBinding(svc)))
	mux.Handle("DELETE /v1/omnichannel/agents/{id}/tool-bindings/{bindingId}", wrap(handleDeleteAIToolBinding(svc)))

	mux.Handle("POST /v1/omnichannel/agents/{id}/simulate", wrap(handleSimulate(svc)))
	mux.Handle("GET /v1/omnichannel/agents/{id}/runs", wrap(handleListRuns(svc)))
	mux.Handle("GET /v1/omnichannel/agents/{id}/tool-runs", wrap(handleListAIToolRuns(svc)))
	mux.Handle("GET /v1/omnichannel/agents/{id}/tool-approvals", wrap(handleListAIToolApprovals(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/tool-approvals/{approvalId}/approve", wrap(handleApproveAIToolApproval(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/tool-approvals/{approvalId}/reject", wrap(handleRejectAIToolApproval(svc)))
}

func handleListProviderKeys(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListProviderKeys(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handlePutProviderKey(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AIProviderKeyInput
		if err := decodeJSONBody(w, r, &in); err != nil || strings.TrimSpace(in.APIKey) == "" {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.PutProviderKey(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("provider"), in.APIKey)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteProviderKey(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.PutProviderKey(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("provider"), "")
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListAgentModels(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		models, err := svc.ListAgentModels(
			r.Context(),
			p.AccountID,
			p,
			r.PathValue("id"),
			r.URL.Query().Get("provider"),
		)
		writeAIResult(w, r, http.StatusOK, map[string]any{"models": models}, err)
	}
}

// ============================================================================
// Agentes
// ============================================================================

func handleListAgents(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAgents(r.Context(), p.AccountID, p)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateAgent(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AIAgentInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateAgent(r.Context(), p.AccountID, p, in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleGetAgent(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.GetAgent(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleUpdateAgent(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch AIAgentPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateAgent(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

// ============================================================================
// Versoes
// ============================================================================

func handleListVersions(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListVersions(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateVersion(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AIVersionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateVersion(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleSaveConfiguration(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AIVersionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.SaveConfiguration(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handlePublishVersion(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		version, err := strconv.Atoi(strings.TrimSpace(r.PathValue("v")))
		if err != nil || version <= 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_version", "Versao invalida no path.")
			return
		}
		out, pErr := svc.PublishVersion(r.Context(), p.AccountID, p, r.PathValue("id"), version)
		writeAIResult(w, r, http.StatusOK, out, pErr)
	}
}

func handleRollback(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in RollbackInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.Rollback(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

// ============================================================================
// Campos a coletar
// ============================================================================

func handleListCollectFields(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListCollectFields(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateCollectField(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in CollectFieldInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateCollectField(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateCollectField(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch CollectFieldPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateCollectField(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("fieldId"), patch)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteCollectField(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.DeleteCollectField(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("fieldId"))
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Vinculos de tools autorizadas
// ============================================================================

func handleListAIToolBindings(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAIToolBindings(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateAIToolBinding(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AIToolBindingInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateAIToolBinding(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateAIToolBinding(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch AIToolBindingPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateAIToolBinding(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("bindingId"), patch)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteAIToolBinding(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		if err := svc.DeleteAIToolBinding(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("bindingId")); err != nil {
			writeAIError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Simulate e runs
// ============================================================================

func handleSimulate(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in SimulateInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.Simulate(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListRuns(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := svc.ListRuns(r.Context(), p.AccountID, p, r.PathValue("id"),
			parseLimit(q.Get("limit")), strings.TrimSpace(q.Get("beforeId")))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListAIToolRuns(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := svc.ListAIToolRuns(r.Context(), p.AccountID, p, r.PathValue("id"),
			q.Get("status"), strings.TrimSpace(q.Get("beforeId")), parseLimit(q.Get("limit")))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListAIToolApprovals(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := svc.ListAIToolApprovals(r.Context(), p.AccountID, p, r.PathValue("id"),
			strings.TrimSpace(q.Get("beforeId")), parseLimit(q.Get("limit")))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleApproveAIToolApproval(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.DecideAIToolApproval(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("approvalId"), true, "")
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleRejectAIToolApproval(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.DecideAIToolApproval(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("approvalId"), false, "")
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

// ============================================================================
// Mapeamento de erro
// ============================================================================

// writeAIResult escreve o payload ou mapeia o erro (com os codigos especificos de IA).
func writeAIResult(w http.ResponseWriter, r *http.Request, status int, payload any, err error) {
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, payload)
}

// writeAIError mapeia os erros especificos da F9 e delega o resto ao writeDomainError (F8).
// provider ausente e limite estourado sao 409 ACIONAVEIS (principio 5); escopo => 404.
func writeAIError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrAIProviderNotConfigured):
		httpapi.WriteError(w, r, http.StatusConflict, "ai_provider_not_configured",
			"Configure o provider, o modelo e a chave do agente antes de simular.")
	case errors.Is(err, ErrAIProviderKeyMissing):
		httpapi.WriteError(w, r, http.StatusConflict, "ai_key_missing",
			"Salve a chave da API deste agente para listar os modelos disponíveis.")
	case errors.Is(err, ErrAIProviderUnsupported):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_provider",
			"Selecione um provedor de IA válido.")
	case errors.Is(err, ErrAIModelsUnavailable):
		httpapi.WriteError(w, r, http.StatusBadGateway, "models_unavailable",
			"Não foi possível listar os modelos. Verifique a chave e tente novamente.")
	case errors.Is(err, ErrVersionImmutable):
		httpapi.WriteError(w, r, http.StatusConflict, "version_immutable",
			"Versao publicada e imutavel. Crie uma nova versao para editar.")
	case modules.IsLimitExceeded(err):
		httpapi.WriteError(w, r, http.StatusConflict, "ai_limit_exceeded",
			"Limite mensal de execucoes de IA atingido para esta conta.")
	default:
		writeDomainError(w, r, err)
	}
}
