package tasks

import (
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func (handler *HTTPHandler) registerTrackingRoutes(mux *http.ServeMux, middleware *auth.Middleware) {
	mux.Handle("GET /v1/tasks/tracking/active", middleware.RequireAuth(handler.withPermission(PermTrackingUse, handler.listActiveTracking)))
	mux.Handle("GET /v1/tasks/tracking/metrics", middleware.RequireAuth(handler.withPermission(PermTrackingViewAll, handler.trackingMetrics)))
	mux.Handle("POST /v1/tasks/{taskId}/tracking/start", middleware.RequireAuth(handler.withPermission(PermTrackingUse, handler.startTracking)))
	mux.Handle("POST /v1/tasks/{taskId}/tracking/pause", middleware.RequireAuth(handler.withPermission(PermTrackingUse, handler.pauseTracking)))
	mux.Handle("POST /v1/tasks/{taskId}/tracking/resume", middleware.RequireAuth(handler.withPermission(PermTrackingUse, handler.resumeTracking)))
	mux.Handle("POST /v1/tasks/{taskId}/tracking/stop", middleware.RequireAuth(handler.withPermission(PermTrackingUse, handler.stopTracking)))
}

func (handler *HTTPHandler) listActiveTracking(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	entries, err := handler.service.ListActiveTimeEntries(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (handler *HTTPHandler) startTracking(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	entry, err := handler.service.StartTracking(r.Context(), ctx.Access, r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (handler *HTTPHandler) pauseTracking(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	entry, err := handler.service.PauseTracking(r.Context(), ctx.Access, r.PathValue("taskId"), parseIfMatch(r.Header.Get("If-Match")))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (handler *HTTPHandler) resumeTracking(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	entry, err := handler.service.ResumeTracking(r.Context(), ctx.Access, r.PathValue("taskId"), parseIfMatch(r.Header.Get("If-Match")))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (handler *HTTPHandler) stopTracking(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	entry, err := handler.service.StopTracking(r.Context(), ctx.Access, r.PathValue("taskId"), parseIfMatch(r.Header.Get("If-Match")))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (handler *HTTPHandler) trackingMetrics(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil {
		writeServiceError(w, r, ErrValidation)
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil {
		writeServiceError(w, r, ErrValidation)
		return
	}
	metrics, err := handler.service.TrackingMetrics(r.Context(), ctx.Access, TrackingMetricsInput{
		UserID:          strings.TrimSpace(r.URL.Query().Get("userId")),
		ClientAccountID: strings.TrimSpace(r.URL.Query().Get("clientAccountId")),
		From:            from,
		To:              to,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"metrics": metrics})
}
