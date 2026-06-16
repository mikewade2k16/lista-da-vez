package automation

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// registerConversationRuntimeRoutes monta os endpoints de persistencia de conversa
// (A7) consumidos pelo n8n. Mesmo padrao de auth dos endpoints de memory: token de
// servico, fora do prefixo /v1/automation (sem gating de modulo / X-Account-Id).
func registerConversationRuntimeRoutes(mux *http.ServeMux, svc *Service, token string) {
	mux.Handle("POST /v1/runtime/automation/messages", handleRuntimeMessagePost(svc, token))
	mux.Handle("GET /v1/runtime/automation/lead-state", handleRuntimeLeadStateGet(svc, token))
	mux.Handle("PUT /v1/runtime/automation/lead-state", handleRuntimeLeadStatePut(svc, token))
	mux.Handle("POST /v1/runtime/automation/handover", handleRuntimeHandover(svc, token))
}

// runtimeAuth valida o token de servico e devolve a sessao. Em falha, ja escreve
// a resposta de erro e retorna ok=false.
func runtimeAuth(w http.ResponseWriter, r *http.Request, token string) (session string, ok bool) {
	if token == "" {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "runtime_not_configured", "AUTOMATION_RUNTIME_TOKEN nao configurado.")
		return "", false
	}
	if !bearerEquals(r.Header.Get("Authorization"), token) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de servico invalido.")
		return "", false
	}
	session = strings.TrimSpace(r.URL.Query().Get("session"))
	if session == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "missing_session", "Parametro session e obrigatorio.")
		return "", false
	}
	return session, true
}

func writeRuntimeErr(w http.ResponseWriter, r *http.Request, err error, msg string) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Sessao nao encontrada.")
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", msg)
}

func handleRuntimeMessagePost(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := runtimeAuth(w, r, token)
		if !ok {
			return
		}
		var body struct {
			ContactID string `json:"contactId"`
			Direction string `json:"direction"`
			Type      string `json:"type"`
			Content   string `json:"content"`
			MediaURL  string `json:"mediaUrl"`
			Segment   string `json:"segment"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if strings.TrimSpace(body.ContactID) == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_contact", "contactId e obrigatorio.")
			return
		}
		view, err := svc.SaveMessage(r.Context(), session, strings.TrimSpace(body.ContactID),
			body.Direction, body.Type, body.Content, body.MediaURL, body.Segment)
		if err != nil {
			writeRuntimeErr(w, r, err, "Falha ao gravar a mensagem.")
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleRuntimeLeadStateGet(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := runtimeAuth(w, r, token)
		if !ok {
			return
		}
		contactID := strings.TrimSpace(r.URL.Query().Get("contactId"))
		if contactID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_contact", "Parametro contactId e obrigatorio.")
			return
		}
		view, err := svc.LeadState(r.Context(), session, contactID)
		if err != nil {
			writeRuntimeErr(w, r, err, "Falha ao ler o estado do lead.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handleRuntimeHandover (M4) pausa/retoma o bot para um contato (handover humano).
// POST /v1/runtime/automation/handover?session=&contactId= body
// { "pausedMinutes": 30 } => silencia por N min; { "resume": true } (ou
// pausedMinutes<=0) => retoma. Responde com a memoria atualizada (paused/pausedUntil).
func handleRuntimeHandover(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := runtimeAuth(w, r, token)
		if !ok {
			return
		}
		contactID := strings.TrimSpace(r.URL.Query().Get("contactId"))
		if contactID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_contact", "Parametro contactId e obrigatorio.")
			return
		}
		var body struct {
			PausedMinutes int  `json:"pausedMinutes"`
			Resume        bool `json:"resume"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.SetHandover(r.Context(), session, contactID, body.PausedMinutes, body.Resume)
		if err != nil {
			writeRuntimeErr(w, r, err, "Falha ao aplicar o handover.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleRuntimeLeadStatePut(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := runtimeAuth(w, r, token)
		if !ok {
			return
		}
		contactID := strings.TrimSpace(r.URL.Query().Get("contactId"))
		if contactID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_contact", "Parametro contactId e obrigatorio.")
			return
		}
		var body struct {
			Status        string `json:"status"`
			FollowUpCount int    `json:"followUpCount"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.SetLeadState(r.Context(), session, contactID, strings.TrimSpace(body.Status), body.FollowUpCount)
		if err != nil {
			writeRuntimeErr(w, r, err, "Falha ao salvar o estado do lead.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}
