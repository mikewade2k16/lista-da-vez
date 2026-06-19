package automation

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRoutes monta os endpoints do painel de automacao. O gating por modulo
// (account_modules) e aplicado globalmente via RequireModuleByPath no Chain;
// aqui so exigimos autenticacao. AccountID vem do principal (X-Account-Id).
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/automation", wrap(handleOverview(svc)))
	mux.Handle("POST /v1/automation/whatsapp/connect", wrap(handleConnect(svc)))
	mux.Handle("POST /v1/automation/whatsapp/disconnect", wrap(handleDisconnect(svc)))
	mux.Handle("PUT /v1/automation/settings", wrap(handleSetEnabled(svc)))
	mux.Handle("GET /v1/automation/persona", wrap(handlePersonaGet(svc)))
	mux.Handle("PUT /v1/automation/persona", wrap(handlePersonaPut(svc)))
	mux.Handle("GET /v1/automation/context-preview", wrap(handleContextPreview(svc)))
	mux.Handle("GET /v1/automation/knowledge-docs", wrap(handleKnowledgeDocsList(svc)))
	mux.Handle("POST /v1/automation/knowledge-docs", wrap(handleKnowledgeDocsCreate(svc)))
	mux.Handle("PATCH /v1/automation/knowledge-docs/{id}", wrap(handleKnowledgeDocsUpdate(svc)))
	mux.Handle("DELETE /v1/automation/knowledge-docs/{id}", wrap(handleKnowledgeDocsDelete(svc)))

	registerModelRoutes(mux, svc, wrap)
	registerSourcesRoutes(mux, svc, wrap)

	// Omni Chat (M0) — chat interno do painel de Operacao. Path FORA do prefixo
	// /v1/automation de proposito: nao casa nenhuma regra de moduleGatingRules
	// (RequireModuleByPath usa limite de segmento), entao quem usa Operacao nao
	// precisa do modulo `automation` habilitado. So RequireAuth.
	mux.Handle("POST /v1/omni-chat/ask", wrap(handleOmniChatAsk(svc)))
}

func handleOverview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Overview(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar a automacao.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleConnect(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Connect(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "waha_error", "Nao foi possivel falar com o WhatsApp (WAHA).")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDisconnect(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		if err := svc.Disconnect(r.Context(), accountID); err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "waha_error", "Nao foi possivel desconectar.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "STOPPED"})
	}
}

func handleSetEnabled(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.SetEnabled(r.Context(), accountID, body.Enabled)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// RegisterRuntimeRoutes monta a rota consumida pelo n8n. Fora do prefixo
// /v1/automation (que e gateado por modulo + X-Account-Id); auth aqui e por
// TOKEN DE SERVICO (AUTOMATION_RUNTIME_TOKEN), nao por JWT de usuario. ctxMgr
// valida o context token (Fase 2) das tools de dados do Omni Chat.
func RegisterRuntimeRoutes(mux *http.ServeMux, svc *Service, token string, ctxMgr *ContextTokenManager) {
	mux.Handle("GET /v1/runtime/automation/config", handleRuntimeConfig(svc, token))
	mux.Handle("GET /v1/runtime/automation/memory", handleRuntimeMemoryGet(svc, token))
	mux.Handle("PUT /v1/runtime/automation/memory", handleRuntimeMemoryPut(svc, token))
	registerConversationRuntimeRoutes(mux, svc, token)
	registerCatalogToolRoute(mux, svc, token)
	registerOmniChatToolRoutes(mux, svc, token, ctxMgr)
}

func handleRuntimeConfig(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "runtime_not_configured",
				"AUTOMATION_RUNTIME_TOKEN nao configurado.")
			return
		}
		if !bearerEquals(r.Header.Get("Authorization"), token) {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de servico invalido.")
			return
		}
		session := strings.TrimSpace(r.URL.Query().Get("session"))
		if session == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_session", "Parametro session e obrigatorio.")
			return
		}
		view, err := svc.RuntimeConfig(r.Context(), session)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Sessao nao encontrada.")
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao montar a configuracao.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func bearerEquals(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(header[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func handlePersonaGet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Persona(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar a persona.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePersonaPut(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Name         string `json:"name"`
			SystemPrompt string `json:"systemPrompt"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if strings.TrimSpace(body.SystemPrompt) == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "empty_prompt", "O comportamento nao pode ficar vazio.")
			return
		}
		view, err := svc.UpdatePersona(r.Context(), accountID, strings.TrimSpace(body.Name), body.SystemPrompt)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleContextPreview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.ContextPreview(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao montar a previa.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleKnowledgeDocsList(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		views, err := svc.KnowledgeDocs(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar os documentos.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, views)
	}
}

func handleKnowledgeDocsCreate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateKnowledgeDoc(r.Context(), accountID, strings.TrimSpace(body.Title), body.Body)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao criar documento.")
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleKnowledgeDocsUpdate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		docID := strings.TrimSpace(r.PathValue("id"))
		var body struct {
			Title     string `json:"title"`
			Body      string `json:"body"`
			SortOrder int    `json:"sortOrder"`
			Enabled   bool   `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateKnowledgeDoc(r.Context(), accountID, docID, strings.TrimSpace(body.Title), body.Body, body.SortOrder, body.Enabled)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Documento nao encontrado.")
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleKnowledgeDocsDelete(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		docID := strings.TrimSpace(r.PathValue("id"))
		if err := svc.DeleteKnowledgeDoc(r.Context(), accountID, docID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Documento nao encontrado.")
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao apagar.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleRuntimeMemoryGet(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "runtime_not_configured", "AUTOMATION_RUNTIME_TOKEN nao configurado.")
			return
		}
		if !bearerEquals(r.Header.Get("Authorization"), token) {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de servico invalido.")
			return
		}
		session := strings.TrimSpace(r.URL.Query().Get("session"))
		chatID := strings.TrimSpace(r.URL.Query().Get("chatId"))
		if session == "" || chatID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_params", "Parametros session e chatId sao obrigatorios.")
			return
		}
		view, err := svc.GetMemory(r.Context(), session, chatID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Sessao nao encontrada.")
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao ler a memoria.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleRuntimeMemoryPut(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "runtime_not_configured", "AUTOMATION_RUNTIME_TOKEN nao configurado.")
			return
		}
		if !bearerEquals(r.Header.Get("Authorization"), token) {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de servico invalido.")
			return
		}
		session := strings.TrimSpace(r.URL.Query().Get("session"))
		chatID := strings.TrimSpace(r.URL.Query().Get("chatId"))
		if session == "" || chatID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_params", "Parametros session e chatId sao obrigatorios.")
			return
		}
		var body struct {
			Seg     int    `json:"seg"`
			LastMsg string `json:"lastMsg"`
			Ts      int64  `json:"ts"`
			LongMem string `json:"longMem"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := svc.SaveMemory(r.Context(), session, chatID, body.Seg, body.LastMsg, body.Ts, body.LongMem); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Sessao nao encontrada.")
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar a memoria.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeNoAccount(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
}

func accountIDFromContext(r *http.Request) (string, bool) {
	if accountID := strings.TrimSpace(r.Header.Get("X-Account-Id")); accountID != "" {
		return accountID, true
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	return principal.TenantID, principal.TenantID != ""
}
