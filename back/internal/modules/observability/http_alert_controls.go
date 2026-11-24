package observability

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

var alertIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,17}$`)

// parseAlertID valida o {id} da rota (inteiro positivo de até 18 dígitos).
func parseAlertID(raw string) (int64, bool) {
	if !alertIDPattern.MatchString(raw) {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil && value > 0
}

type silenceInput struct {
	Hours int `json:"hours"`
}

func (h *httpHandler) alertActor(r *http.Request) alertActor {
	principal, _ := auth.PrincipalFromContext(r.Context())
	return alertActor{UserID: principal.UserID, Name: principal.DisplayName}
}

// alertControlError traduz o resultado de ack/silence/unsilence em 204 ou erro.
func alertControlError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Alerta não encontrado.")
	case errors.Is(err, ErrAlertResolved):
		httpapi.WriteError(w, r, http.StatusConflict, "alert_resolved", "O alerta já foi resolvido.")
	case errors.Is(err, ErrInvalidSilenceHours):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_hours", "Informe hours entre 1 e 168.")
	default:
		respond(w, r, nil, err)
	}
}

func (h *httpHandler) alertID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := parseAlertID(r.PathValue("id"))
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_alert_id", "Alerta inválido.")
	}
	return id, ok
}

func (h *httpHandler) ackAlert(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r)
	if !ok {
		return
	}
	alertControlError(w, r, h.service.AcknowledgeAlert(r.Context(), id, h.alertActor(r)))
}

func (h *httpHandler) silenceAlert(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r)
	if !ok {
		return
	}
	var input silenceInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Corpo inválido.")
		return
	}
	alertControlError(w, r, h.service.SilenceAlert(r.Context(), id, input.Hours, h.alertActor(r)))
}

func (h *httpHandler) unsilenceAlert(w http.ResponseWriter, r *http.Request) {
	id, ok := h.alertID(w, r)
	if !ok {
		return
	}
	alertControlError(w, r, h.service.UnsilenceAlert(r.Context(), id))
}

func (h *httpHandler) integrations(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Integrations(r.Context(), days(r))
	respond(w, r, data, err)
}
