package calendar

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxJSONBody limita o corpo dos POST/PUT (notas podem ter HTML do editor).
const maxJSONBody = 1 << 20 // 1 MiB

// RegisterRoutes monta os endpoints do painel (/v1/calendar*). O gating por
// modulo (account_modules) e aplicado globalmente via RequireModuleByPath no
// Chain; aqui validamos membership e calendar.view/manage. O accountID vem do
// Principal resolvido pelo middleware, NUNCA do body.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrapView := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireCalendarPermission("calendar.view", h))
	}
	wrapManage := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireCalendarPermission("calendar.manage", h))
	}

	mux.Handle("GET /v1/calendar/scope", wrapView(handleGetCalendarScope(svc)))
	mux.Handle("GET /v1/calendar/events", wrapView(handleListEvents(svc)))
	mux.Handle("POST /v1/calendar/events", wrapManage(handleCreateEvent(svc)))
	mux.Handle("GET /v1/calendar/events/{id}", wrapView(handleGetEvent(svc)))
	mux.Handle("PUT /v1/calendar/events/{id}", wrapManage(handleUpdateEvent(svc)))
	mux.Handle("DELETE /v1/calendar/events/{id}", wrapManage(handleDeleteEvent(svc)))
	mux.Handle("POST /v1/calendar/events/{id}/task", wrapManage(handleCreateEventTask(svc)))
	mux.Handle("GET /v1/calendar/notes/{month}", wrapView(handleGetNotes(svc)))
	mux.Handle("PUT /v1/calendar/notes/{month}", wrapManage(handlePutNotes(svc)))
	mux.Handle("GET /v1/calendar/config", wrapView(handleGetConfig(svc)))
	mux.Handle("PUT /v1/calendar/config", wrapManage(handlePutConfig(svc)))
	mux.Handle("GET /v1/calendar/members", wrapView(handleListMembers(svc)))
	mux.Handle("GET /v1/calendar/responsibles", wrapView(handleListResponsibles(svc)))
	mux.Handle("GET /v1/calendar/holidays", wrapView(handleListHolidays(svc)))
}

func requireCalendarPermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		if principal.Role != auth.RolePlatformAdmin && principal.Role != auth.RoleOwner &&
			principal.PermissionsResolved && !containsPermission(principal.Permissions, permission) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar o calendario.")
			return
		}
		next(w, r)
	}
}

func containsPermission(permissions []string, wanted string) bool {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == wanted {
			return true
		}
	}
	return false
}

func handleGetCalendarScope(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		scope, err := svc.GetCalendarScope(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, scope)
	}
}

func handleListEvents(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		q := r.URL.Query()
		f := EventFilter{
			From:     strings.TrimSpace(q.Get("from")),
			To:       strings.TrimSpace(q.Get("to")),
			ClientID: strings.TrimSpace(q.Get("clientId")),
		}
		events, err := svc.ListScopedEvents(r.Context(), accountID, f)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"events": events})
	}
}

func handleCreateEvent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var in EventInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateScopedEvent(r.Context(), accountID, in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleGetEvent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.GetScopedEvent(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleUpdateEvent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var in EventInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		// If-Match opcional (contrato C12): sem header = comportamento atual (compat).
		// Com header, o valor e a version que o front carregou; divergencia => 409.
		expectedVersion, ok := parseIfMatch(w, r)
		if !ok {
			return
		}
		view, err := svc.UpdateScopedEvent(r.Context(), accountID, r.PathValue("id"), in, expectedVersion)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// parseIfMatch le o cabecalho If-Match (contrato C12). Ausente/vazio => (nil, true)
// (sem optimistic locking, comportamento legado). Presente e numerico => (&version,
// true). Presente e invalido => escreve 400 e devolve (nil, false). Aceita ETag entre
// aspas e o prefixo weak "W/" por robustez, mas o contrato e um inteiro simples.
func parseIfMatch(w http.ResponseWriter, r *http.Request) (*int, bool) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return nil, true
	}
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "W/"))
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	version, err := strconv.Atoi(raw)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_if_match",
			"Cabecalho If-Match invalido (use o numero de versao do evento).")
		return nil, false
	}
	return &version, true
}

func handleDeleteEvent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		// archiveTask=true (WAVE 6): a politica "excluir os dois" do modal — arquiva a task
		// vinculada junto. Ausente/false = so remove a relation (task fica).
		archiveTask := r.URL.Query().Get("archiveTask") == "true"
		if err := svc.DeleteScopedEvent(r.Context(), accountID, r.PathValue("id"), archiveTask); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleCreateEventTask cria (e vincula) uma task para um evento SEM task (WAVE 6): o botao
// "Criar task" do badge "evento sem task". Idempotente (se ja tem, devolve o taskId existente).
func handleCreateEventTask(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		taskID, err := svc.CreateTaskForScopedEvent(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"taskId": taskID})
	}
}

func handleGetNotes(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		note, err := svc.GetNotes(r.Context(), accountID, r.PathValue("month"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, note)
	}
}

func handlePutNotes(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body NoteInput
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		note, err := svc.PutNotes(r.Context(), accountID, r.PathValue("month"), body.Content, principalLabel(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, note)
	}
}

func handleGetConfig(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		cfg, err := svc.GetConfig(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, cfg)
	}
}

func handlePutConfig(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var cfg CalendarConfig
		if err := decodeJSONBody(w, r, &cfg); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		saved, err := svc.PutConfig(r.Context(), accountID, cfg)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, saved)
	}
}

func handleListMembers(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		members, err := svc.ListMembers(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"members": members})
	}
}

func handleListResponsibles(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		responsibles, err := svc.ListResponsibles(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"responsibles": responsibles})
	}
}

func handleListHolidays(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		q := r.URL.Query()
		holidays, err := svc.ListHolidays(r.Context(), accountID,
			strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"holidays": holidays})
	}
}

// ============================================================================
// Helpers
// ============================================================================

// accountScope resolve a account ATIVA ja validada pelo RequireAuthWithAccount.
// O header bruto so permanece como fallback para rotas legadas ainda em migracao;
// handlers principais nunca o usam sem a validacao de membership do middleware.
func accountScope(r *http.Request) (string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	accountID := strings.TrimSpace(principal.AccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(r.Header.Get("X-Account-Id"))
	}
	if accountID == "" {
		accountID = strings.TrimSpace(principal.TenantID)
	}
	if accountID == "" {
		return "", false
	}
	return accountID, true
}

// principalLabel devolve um rotulo do autor (nome ou userID) para updated_by.
func principalLabel(r *http.Request) string {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return ""
	}
	if strings.TrimSpace(principal.DisplayName) != "" {
		return principal.DisplayName
	}
	return principal.UserID
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody)).Decode(dst)
}

func writeNoAccount(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrInvalidDate):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_date", "Data invalida.")
	case errors.Is(err, ErrInvalidTitle):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_title", "Informe um titulo.")
	case errors.Is(err, ErrInvalidMedia):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Anexo invalido (tipo nao suportado).")
	case errors.Is(err, ErrMediaTooLarge):
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "media_too_large", "Arquivo acima do limite permitido.")
	case errors.Is(err, ErrInvalidClient):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_client", "Informe um cliente valido (clientId UUID).")
	case errors.Is(err, ErrAINotConfigured):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "ai_not_configured",
			"IA do calendario nao configurada. Defina CALENDAR_AI_WEBHOOK_URL e configure as credenciais no n8n.")
	case errors.Is(err, ErrPlanConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "plan_conflict", "Plano em estado que nao permite esta operacao.")
	case errors.Is(err, ErrInvalidStatus):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_status", "Status invalido (use done|error).")
	case errors.Is(err, ErrTasksNotConfigured):
		httpapi.WriteError(w, r, http.StatusBadRequest, "tasks_not_configured",
			"Integracao com tasks nao configurada. Selecione um board na aba Integracoes da config do calendario.")
	case errors.Is(err, ErrVersionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "version_conflict",
			"Este evento foi alterado por outra pessoa. Recarregue o item e tente novamente.")
	case errors.Is(err, ErrInvalidProvider):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_provider",
			"Provider invalido (use gemini, glm ou openai).")
	case errors.Is(err, ErrModelsUnavailable):
		httpapi.WriteError(w, r, http.StatusBadGateway, "models_unavailable",
			"Nao foi possivel listar os modelos deste provedor (a API falhou ou a chave e invalida). Verifique a chave e tente novamente.")
	case errors.Is(err, ErrAIDisabled):
		writeAIDisabled(w, r)
	case errors.Is(err, ErrAIKeyMissing):
		writeAIKeyMissing(w, r)
	case errors.Is(err, ErrForbidden):
		writeNoAccount(w, r)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a requisicao.")
	}
}

// writeAIDisabled responde 409 ai_disabled: a IA do calendario esta com o kill switch
// desligado (config ai.enabled=false). Mensagem acionavel citando a aba IA (SPEC-B2).
func writeAIDisabled(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusConflict, "ai_disabled",
		"IA do calendario desligada. Ligue a IA na aba IA da configuracao do calendario.")
}

// writeAIKeyMissing responde 409 ai_key_missing: o provider ativo nao tem chave
// gravada (nem na conta nem global). Mensagem acionavel citando a aba IA (SPEC-B2).
func writeAIKeyMissing(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusConflict, "ai_key_missing",
		"Chave de IA nao configurada. Defina a chave do provider na aba IA (ou peca ao admin as chaves globais).")
}
