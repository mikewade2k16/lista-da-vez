package operations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		if !canMutateOperations(access) {
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
		if !canMutateOperations(access) {
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
		if !canMutateOperations(access) {
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
		if !canMutateOperations(access) {
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
		if !canMutateOperations(access) {
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

	mux.Handle("POST /v1/operations/services/parallel", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		access := AccessContextFromPrincipal(principal)
		if !canMutateOperations(access) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input StartParallelCommandInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		ack, err := service.StartParallel(r.Context(), access, input)
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
		if !canMutateOperations(access) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", readOnlyOperationsMessage)
			return
		}

		var input FinishCommandInput
		if err := readJSONLenient(r, &input); err != nil {
			httpapi.WriteErrorWithDetails(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.", map[string]string{"cause": err.Error()})
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

func readJSONLenient(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}

	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read body failed: %w (content-type: %s)", err, r.Header.Get("Content-Type"))
	}

	if len(bodyBytes) == 0 {
		return fmt.Errorf("empty body (content-type: %s, content-length: %s)", r.Header.Get("Content-Type"), r.Header.Get("Content-Length"))
	}

	preview := string(bodyBytes)
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}

	if err := json.Unmarshal(bodyBytes, dst); err != nil {
		return fmt.Errorf("json decode failed: %w (content-type: %s, body: %q)", err, r.Header.Get("Content-Type"), preview)
	}

	return nil
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
