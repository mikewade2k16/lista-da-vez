package observability

import (
	"errors"
	"io"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxSettingsBody = 64 * 1024

// registerSettingsRoutes: configurações editáveis (2.1 a 2.3). Fica separado de
// RegisterRoutes para o bloco de rotas não colidir com outras frentes.
func registerSettingsRoutes(mux *http.ServeMux, handler *httpHandler, admin func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET "+adminRoot+"/settings", admin(handler.settings))
	mux.Handle("PUT "+adminRoot+"/settings/{key}", admin(handler.putSetting))
	mux.Handle("POST "+adminRoot+"/settings/email/preview", admin(handler.previewEmail))
	mux.Handle("GET "+adminRoot+"/reports/weekly/preview", admin(handler.weeklyPreview))
	mux.Handle("POST "+adminRoot+"/reports/weekly/send", admin(handler.weeklySend))
}

func (h *httpHandler) settings(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Settings(r.Context())
	settingsRespond(w, r, data, err)
}

// settingsRespond acrescenta o 503 de "configurações indisponíveis" ao respond padrão.
func settingsRespond(w http.ResponseWriter, r *http.Request, data any, err error) {
	if errors.Is(err, ErrSettingsUnavailable) {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "settings_unavailable", "As configurações de alertas não estão disponíveis neste ambiente.")
		return
	}
	respond(w, r, data, err)
}

func readSettingsBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSettingsBody))
	if err != nil || len(raw) == 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Corpo inválido.")
		return nil, false
	}
	return raw, true
}

func writeFieldErrors(w http.ResponseWriter, r *http.Request, errs fieldErrors) {
	httpapi.WriteErrorWithDetails(w, r, http.StatusBadRequest, settingsFieldsErrorCode,
		"Corrija os campos destacados.", map[string]any{"fields": errs})
}

func (h *httpHandler) putSetting(w http.ResponseWriter, r *http.Request) {
	raw, ok := readSettingsBody(w, r)
	if !ok {
		return
	}
	actor := h.alertActor(r)
	name := actor.Name
	if name == "" {
		name = actor.UserID
	}
	errs, err := h.service.PutSetting(r.Context(), r.PathValue("key"), raw, name)
	switch {
	case errors.Is(err, ErrUnknownSetting):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Configuração desconhecida.")
	case len(errs) > 0:
		writeFieldErrors(w, r, errs)
	case err != nil:
		settingsRespond(w, r, nil, err)
	default:
		data, getErr := h.service.Settings(r.Context())
		settingsRespond(w, r, data, getErr)
	}
}

func (h *httpHandler) previewEmail(w http.ResponseWriter, r *http.Request) {
	raw, ok := readSettingsBody(w, r)
	if !ok {
		return
	}
	preview, errs, err := h.service.PreviewEmail(raw)
	switch {
	case len(errs) > 0:
		writeFieldErrors(w, r, errs)
	case err != nil:
		settingsRespond(w, r, nil, err)
	default:
		settingsRespond(w, r, map[string]string{"subject": preview.Subject, "text": preview.Body}, nil)
	}
}

func (h *httpHandler) weeklyPreview(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.WeeklyPreview(r.Context())
	settingsRespond(w, r, data, err)
}

func (h *httpHandler) weeklySend(w http.ResponseWriter, r *http.Request) {
	err := h.service.WeeklySend(r.Context())
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrAlertsNotConfigured):
		httpapi.WriteError(w, r, http.StatusConflict, "alerts_not_configured", "E-mail não configurado: defina o SMTP e ao menos um destinatário.")
	case errors.Is(err, ErrSettingsUnavailable):
		settingsRespond(w, r, nil, err)
	default:
		httpapi.WriteError(w, r, http.StatusBadGateway, "report_send_failed", "Não foi possível enviar o relatório semanal.")
	}
}
