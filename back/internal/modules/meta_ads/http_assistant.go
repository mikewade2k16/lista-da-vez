package metaads

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// registerAssistantRoutes monta os endpoints do assistente MCP (chat da pagina
// /meta-ads). Chamado por RegisterRoutes (http.go) com o mesmo wrap de auth.
func registerAssistantRoutes(mux *http.ServeMux, svc *Service, wrap func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /v1/meta-ads/assistant/messages", wrap(handleAssistantHistory(svc)))
	mux.Handle("POST /v1/meta-ads/assistant/messages", wrap(handleAssistantSend(svc)))
	mux.Handle("DELETE /v1/meta-ads/assistant/messages", wrap(handleAssistantClear(svc)))
	mux.Handle("GET /v1/meta-ads/assistant/health", wrap(handleAssistantHealth(svc)))
	mux.Handle("POST /v1/meta-ads/assistant/auth/start", wrap(handleAssistantAuthStart(svc)))
	mux.Handle("POST /v1/meta-ads/assistant/auth/complete", wrap(handleAssistantAuthComplete(svc)))
	mux.Handle("GET /v1/meta-ads/assistant/settings", wrap(handleAssistantSettings(svc)))
	mux.Handle("PUT /v1/meta-ads/assistant/settings", wrap(handleAssistantSettingsSave(svc)))
}

func handleAssistantSettings(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.AssistantSettings(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao carregar as configuracoes do assistente.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleAssistantSettingsSave(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Model        string `json:"model"`
			SystemPrompt string `json:"systemPrompt"`
		}
		// system_prompt pode ser longo: limite maior (1MB) que o do chat.
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := svc.SaveAssistantSettings(r.Context(), accountID, body.Model, body.SystemPrompt); err != nil {
			writeServiceError(w, r, err, "Falha ao salvar as configuracoes.")
			return
		}
		view, err := svc.AssistantSettings(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err, "Salvo, mas falha ao reler as configuracoes.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleAssistantHistory(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		// limit invalido/ausente vira 0 -> default do service.
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		views, err := svc.AssistantHistory(r.Context(), accountID, limit)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao carregar o historico do assistente.")
			return
		}
		if views == nil {
			views = []AssistantMessageView{}
		}
		httpapi.WriteJSON(w, http.StatusOK, views)
	}
}

func handleAssistantSend(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			Message     string `json:"message"`
			AdAccountID string `json:"adAccountId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		result, err := svc.AssistantSend(r.Context(), accountID, body.Message, body.AdAccountID)
		if err != nil {
			writeAssistantError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

func handleAssistantClear(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		if err := svc.AssistantClear(r.Context(), accountID); err != nil {
			writeServiceError(w, r, err, "Falha ao limpar a conversa.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAssistantHealth(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := accountIDFromContext(r); !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.AssistantHealth(r.Context())
		if err != nil {
			view = AssistantHealthView{OK: false, Detail: "internal_error"}
		}
		// 200 sempre: o card de conexoes le OK/Detail, nao o status HTTP.
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handleAssistantAuthStart inicia o login do MCP da Meta (devolve a URL). O
// usuario abre a URL, autoriza, e depois cola a URL de callback em /auth/complete.
func handleAssistantAuthStart(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		url, err := svc.AssistantAuthStart(r.Context(), accountID)
		if err != nil {
			writeAssistantError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
	}
}

// handleAssistantAuthComplete conclui o login com a URL de callback colada.
func handleAssistantAuthComplete(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			CallbackURL string `json:"callbackUrl"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		connected, detail, err := svc.AssistantAuthComplete(r.Context(), accountID, body.CallbackURL)
		if err != nil {
			writeAssistantError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"ok": connected, "detail": detail})
	}
}

// writeAssistantError mapeia os erros do fluxo do assistente:
//   - mensagem vazia/longa demais -> 400
//   - runner nao configurado      -> 503 assistant_not_configured
//   - falha do runner             -> 502 assistant_error
//   - resto                       -> writeServiceError (404/500)
func writeAssistantError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrAssistantMessageEmpty):
		httpapi.WriteError(w, r, http.StatusBadRequest, "missing_message", "Digite uma mensagem.")
	case errors.Is(err, ErrAssistantMessageTooLong):
		httpapi.WriteError(w, r, http.StatusBadRequest, "message_too_long", "Mensagem longa demais (max 4000 caracteres).")
	case errors.Is(err, errAuthSessionGone):
		httpapi.WriteError(w, r, http.StatusConflict, "auth_session_gone", "A sessao de login expirou. Gere o link novamente.")
	case errors.Is(err, ErrRunnerNotConfigured):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "assistant_not_configured", "O assistente nao esta configurado no servidor.")
	case errors.Is(err, errRunnerFailed):
		httpapi.WriteError(w, r, http.StatusBadGateway, "assistant_error", "O assistente nao conseguiu responder. Verifique o runner.")
	default:
		writeServiceError(w, r, err, "Falha ao processar a mensagem do assistente.")
	}
}
