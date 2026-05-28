package realtime

import "net/http"

func RegisterRoutes(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("GET /v1/realtime/operations", service.HandleOperationSocket)
	mux.HandleFunc("GET /v1/realtime/context", service.HandleContextSocket)
	mux.HandleFunc("GET /v1/realtime/tasks", service.HandleTasksSocket)
	mux.HandleFunc("GET /v1/realtime/presence", service.HandlePresenceSocket)
	mux.HandleFunc("GET /v1/realtime/notifications", service.HandleNotificationsSocket)
}
