package realtime

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("POST /v1/ws/ticket", middleware.RequireAuth(http.HandlerFunc(service.HandleTicket)))
	mux.HandleFunc("GET /v1/realtime/operations", service.HandleOperationSocket)
	mux.HandleFunc("GET /v1/realtime/context", service.HandleContextSocket)
	mux.HandleFunc("GET /v1/realtime/tasks", service.HandleTasksSocket)
	mux.HandleFunc("GET /v1/realtime/presence", service.HandlePresenceSocket)
	mux.HandleFunc("GET /v1/realtime/notifications", service.HandleNotificationsSocket)
}
