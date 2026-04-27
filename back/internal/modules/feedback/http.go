package feedback

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Feedbacks []FeedbackView `json:"feedbacks"`
}

type feedbackResponse struct {
	Feedback FeedbackView `json:"feedback"`
}

type createRequest struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type updateRequest struct {
	Status    *string `json:"status"`
	AdminNote *string `json:"admin_note"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("POST /v1/feedback", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request createRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.Create(r.Context(), principal, CreateInput{
			Kind:    request.Kind,
			Subject: request.Subject,
			Body:    request.Body,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, feedbackResponse{
			Feedback: *result,
		})
	})))

	mux.Handle("GET /v1/feedback", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		feedbacks, err := service.List(r.Context(), principal, ListInput{
			Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
			Status: strings.TrimSpace(r.URL.Query().Get("status")),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{
			Feedbacks: feedbacks,
		})
	})))

	mux.Handle("PATCH /v1/feedback/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_id", "ID invalido.")
			return
		}

		var request updateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.Update(r.Context(), principal, id, UpdateInput{
			Status:    request.Status,
			AdminNote: request.AdminNote,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, feedbackResponse{
			Feedback: *result,
		})
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "feedback_not_found", "Feedback nao encontrado.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar feedback.")
	}
}
