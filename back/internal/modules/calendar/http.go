package calendar

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxJSONBody limita o corpo dos POST/PUT (notas podem ter HTML do editor).
const maxJSONBody = 1 << 20 // 1 MiB

// RegisterRoutes monta os endpoints do painel (/v1/calendar*). O gating por
// modulo (account_modules) e aplicado globalmente via RequireModuleByPath no
// Chain; aqui so exigimos autenticacao. O accountID vem do Principal/X-Account-Id,
// NUNCA do body.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/calendar/events", wrap(handleListEvents(svc)))
	mux.Handle("POST /v1/calendar/events", wrap(handleCreateEvent(svc)))
	mux.Handle("GET /v1/calendar/events/{id}", wrap(handleGetEvent(svc)))
	mux.Handle("PUT /v1/calendar/events/{id}", wrap(handleUpdateEvent(svc)))
	mux.Handle("DELETE /v1/calendar/events/{id}", wrap(handleDeleteEvent(svc)))
	mux.Handle("GET /v1/calendar/notes/{month}", wrap(handleGetNotes(svc)))
	mux.Handle("PUT /v1/calendar/notes/{month}", wrap(handlePutNotes(svc)))
	mux.Handle("GET /v1/calendar/config", wrap(handleGetConfig(svc)))
	mux.Handle("PUT /v1/calendar/config", wrap(handlePutConfig(svc)))
	mux.Handle("GET /v1/calendar/members", wrap(handleListMembers(svc)))
	mux.Handle("GET /v1/calendar/responsibles", wrap(handleListResponsibles(svc)))
	mux.Handle("GET /v1/calendar/holidays", wrap(handleListHolidays(svc)))
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
		events, err := svc.ListEvents(r.Context(), accountID, f)
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
		view, err := svc.CreateEvent(r.Context(), accountID, in)
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
		view, err := svc.GetEvent(r.Context(), accountID, r.PathValue("id"))
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
		view, err := svc.UpdateEvent(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteEvent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		if err := svc.DeleteEvent(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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

// accountScope resolve o accountID do calendario a partir do Principal. Vem do
// header X-Account-Id ou, na ausencia, do TenantID do JWT. O calendario e sempre
// escopado por account (inclusive para admin, que opera na account do switcher).
func accountScope(r *http.Request) (string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
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
	case errors.Is(err, ErrForbidden):
		writeNoAccount(w, r)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a requisicao.")
	}
}
