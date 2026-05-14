package notifications

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type HTTPHandler struct {
	service *Service
}

type notificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	NextCursor    string         `json:"nextCursor,omitempty"`
}

type notificationResponse struct {
	Notification Notification `json:"notification"`
}

type preferencesResponse struct {
	Preferences []NotificationPreference `json:"preferences"`
}

type markAllReadResponse struct {
	Updated int64 `json:"updated"`
}

type muteResponse struct {
	Mute Mute `json:"mute"`
}

type preferencesRequest struct {
	Preferences []NotificationPreference `json:"preferences"`
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	NewHTTPHandler(service).RegisterRoutes(mux, middleware)
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux, middleware *auth.Middleware) {
	mux.Handle("GET /v1/notifications", middleware.RequireAuth(handler.withPermission(PermNotificationsRead, handler.listNotifications)))
	mux.Handle("POST /v1/notifications/{notificationId}/read", middleware.RequireAuth(handler.withPermission(PermNotificationsRead, handler.markRead)))
	mux.Handle("POST /v1/notifications/mark-all-read", middleware.RequireAuth(handler.withPermission(PermNotificationsRead, handler.markAllRead)))
	mux.Handle("GET /v1/notifications/preferences", middleware.RequireAuth(handler.withPermission(PermNotificationsPreferencesManage, handler.listPreferences)))
	mux.Handle("PUT /v1/notifications/preferences", middleware.RequireAuth(handler.withPermission(PermNotificationsPreferencesManage, handler.savePreferences)))
	mux.Handle("POST /v1/notifications/mute", middleware.RequireAuth(handler.withPermission(PermNotificationsPreferencesManage, handler.mute)))
}

type notificationsHTTPContext struct {
	Access AccessContext
}

func (handler *HTTPHandler) withPermission(permission string, next func(http.ResponseWriter, *http.Request, notificationsHTTPContext)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
		if accountID == "" {
			accountID = strings.TrimSpace(r.URL.Query().Get("accountId"))
		}
		if accountID == "" {
			accountID = strings.TrimSpace(principal.TenantID)
		}

		access, err := handler.service.ResolveAccessContext(r.Context(), principal, accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if !access.Has(permission) {
			writeServiceError(w, r, ErrForbidden)
			return
		}

		next(w, r, notificationsHTTPContext{Access: access})
	})
}

func (handler *HTTPHandler) listNotifications(w http.ResponseWriter, r *http.Request, ctx notificationsHTTPContext) {
	limit, err := parseOptionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		writeServiceError(w, r, ErrInvalid)
		return
	}

	page, err := handler.service.ListNotifications(r.Context(), ctx.Access, ListNotificationsInput{
		Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:  limit,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, notificationsResponse{
		Notifications: page.Notifications,
		NextCursor:    page.NextCursor,
	})
}

func (handler *HTTPHandler) markRead(w http.ResponseWriter, r *http.Request, ctx notificationsHTTPContext) {
	notification, err := handler.service.MarkRead(r.Context(), ctx.Access, r.PathValue("notificationId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, notificationResponse{Notification: notification})
}

func (handler *HTTPHandler) markAllRead(w http.ResponseWriter, r *http.Request, ctx notificationsHTTPContext) {
	updated, err := handler.service.MarkAllRead(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, markAllReadResponse{Updated: updated})
}

func (handler *HTTPHandler) listPreferences(w http.ResponseWriter, r *http.Request, ctx notificationsHTTPContext) {
	preferences, err := handler.service.ListPreferences(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, preferencesResponse{Preferences: preferences})
}

func (handler *HTTPHandler) savePreferences(w http.ResponseWriter, r *http.Request, ctx notificationsHTTPContext) {
	var request preferencesRequest
	if err := httpapi.ReadJSON(r, &request); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	preferences, err := handler.service.SavePreferences(r.Context(), ctx.Access, request.Preferences)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, preferencesResponse{Preferences: preferences})
}

func (handler *HTTPHandler) mute(w http.ResponseWriter, r *http.Request, ctx notificationsHTTPContext) {
	var input MuteInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	mute, err := handler.service.Mute(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, muteResponse{Mute: mute})
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrAccountRequired), errors.Is(err, ErrInvalid):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados enviados.")
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrNotificationNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar notifications.")
	}
}

func parseOptionalInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
