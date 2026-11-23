package observability

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// parseHours valida ?hours= (1..72, padrão 6). Vazio usa o padrão; fora da faixa é erro.
func parseHours(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 6, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 72 {
		return 0, false
	}
	return value, true
}

// parseAlertStatus valida ?status= (open|resolved|all, padrão open).
func parseAlertStatus(raw string) (string, bool) {
	switch raw = strings.TrimSpace(raw); raw {
	case "":
		return "open", true
	case "open", "resolved", "all":
		return raw, true
	}
	return "", false
}

func (h *httpHandler) health(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Health(r.Context())
	respond(w, r, data, err)
}

func (h *httpHandler) security(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Security(r.Context())
	respond(w, r, data, err)
}

func (h *httpHandler) system(w http.ResponseWriter, r *http.Request) {
	hours, ok := parseHours(r.URL.Query().Get("hours"))
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_hours", "Informe hours entre 1 e 72.")
		return
	}
	data, err := h.service.System(r.Context(), hours)
	respond(w, r, data, err)
}

func (h *httpHandler) alerts(w http.ResponseWriter, r *http.Request) {
	status, ok := parseAlertStatus(r.URL.Query().Get("status"))
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_status", "Informe status open, resolved ou all.")
		return
	}
	data, err := h.service.Alerts(r.Context(), status)
	respond(w, r, data, err)
}

func (h *httpHandler) testAlert(w http.ResponseWriter, r *http.Request) {
	err := h.service.SendTestAlert(r.Context())
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrAlertsNotConfigured):
		httpapi.WriteError(w, r, http.StatusConflict, "alerts_not_configured", "Configure o SMTP e OBS_ALERT_EMAILS para enviar alertas.")
	default:
		httpapi.WriteError(w, r, http.StatusBadGateway, "alerts_send_failed", "Não foi possível enviar o e-mail de teste.")
	}
}

// parseDeployLimit valida ?limit= (1..100, padrão 30).
func parseDeployLimit(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 30, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 100 {
		return 0, false
	}
	return value, true
}

func (h *httpHandler) deploys(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseDeployLimit(r.URL.Query().Get("limit"))
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_limit", "Informe limit entre 1 e 100.")
		return
	}
	data, err := h.service.Deploys(r.Context(), limit)
	respond(w, r, data, err)
}
