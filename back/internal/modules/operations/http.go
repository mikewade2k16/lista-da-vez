package operations

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const readOnlyOperationsMessage = "Este perfil pode apenas acompanhar a operacao."

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/operations/overview", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)

		overview, err := service.Overview(r.Context(), access)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, overview)
	})))

	mux.Handle("GET /v1/operations/snapshot", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)

		snapshot, err := service.Snapshot(r.Context(), access, strings.TrimSpace(r.URL.Query().Get("storeId")))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, snapshot)
	})))

	mux.Handle("POST /v1/operations/queue", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !CanMutateOperationsRole(access.Role) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input QueueCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.AddToQueue(r.Context(), access, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/operations/pause", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !CanMutateOperationsRole(access.Role) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input PauseCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.Pause(r.Context(), access, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/operations/assign-task", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !CanMutateOperationsRole(access.Role) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input AssignTaskCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.AssignTask(r.Context(), access, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/operations/resume", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !CanMutateOperationsRole(access.Role) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input QueueCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.Resume(r.Context(), access, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/operations/start", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !CanMutateOperationsRole(access.Role) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input StartCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.Start(r.Context(), access, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/operations/finish", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !CanMutateOperationsRole(access.Role) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input FinishCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.Finish(r.Context(), access, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrStoreRequired), errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados da operacao.")
	case errors.Is(err, ErrConsultantBusy):
		httpapi.WriteError(w, r, http.StatusConflict, "consultant_busy", "O consultor ja esta em atendimento e nao pode ser deslocado agora.")
	case errors.Is(err, ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, ErrConsultantNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "consultant_not_found", "Consultor nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar a operacao.")
	}
}
