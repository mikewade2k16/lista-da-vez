package performancefeedback

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type managerRequest struct {
	StoreID          string            `json:"storeId"`
	ConsultantID     string            `json:"consultantId"`
	Month            string            `json:"month"`
	Week             int               `json:"week"`
	FeedbackSections []FeedbackSection `json:"feedbackSections"`
	Status           string            `json:"status"`
	ExpectedVersion  int               `json:"expectedVersion"`
	MetricsSnapshot  *Metrics          `json:"metricsSnapshot"`
}

type settingsRequest struct {
	StoreID         string            `json:"storeId"`
	Cadence         string            `json:"cadence"`
	DefaultSections []FeedbackSection `json:"defaultSections"`
	ExpectedVersion int               `json:"expectedVersion"`
}

type consultantRequest struct {
	ConsultantNotesHTML string `json:"consultantNotesHtml"`
	ExpectedVersion     int    `json:"expectedVersion"`
}

type reviewResponse struct {
	Review Review `json:"review"`
}

type settingsResponse struct {
	Settings Settings `json:"settings"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/performance-feedback/context", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		week := 0
		if rawWeek := strings.TrimSpace(r.URL.Query().Get("week")); rawWeek != "" {
			parsedWeek, err := strconv.Atoi(rawWeek)
			if err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Periodo invalido.")
				return
			}
			week = parsedWeek
		}

		view, err := service.Context(r.Context(), principal, ContextInput{
			StoreID:      strings.TrimSpace(r.URL.Query().Get("storeId")),
			ConsultantID: strings.TrimSpace(r.URL.Query().Get("consultantId")),
			Month:        strings.TrimSpace(r.URL.Query().Get("month")),
			Week:         week,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, view)
	})))

	mux.Handle("PUT /v1/performance-feedback/manager", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request managerRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		review, err := service.SaveManager(r.Context(), principal, ManagerInput(request))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, reviewResponse{Review: review})
	})))

	mux.Handle("PUT /v1/performance-feedback/settings", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request settingsRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		settings, err := service.SaveSettings(r.Context(), principal, SettingsInput(request))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, settingsResponse{Settings: settings})
	})))

	mux.Handle("PUT /v1/performance-feedback/{id}/consultant", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request consultantRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		review, err := service.SaveConsultant(r.Context(), principal, ConsultantInput{
			ReviewID:            strings.TrimSpace(r.PathValue("id")),
			ConsultantNotesHTML: request.ConsultantNotesHTML,
			ExpectedVersion:     request.ExpectedVersion,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, reviewResponse{Review: review})
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar feedbacks de desempenho.")
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrConsultantNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Feedback ou consultor nao encontrado.")
	case errors.Is(err, ErrStoreRequired), errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique a loja, o periodo e os campos informados.")
	case errors.Is(err, ErrConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "version_conflict", "Este feedback foi atualizado em outra sessao. Recarregue antes de salvar.")
	default:
		slog.ErrorContext(r.Context(), "performance_feedback_internal_error",
			slog.String("request_id", httpapi.RequestIDFromContext(r.Context())),
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Nao foi possivel processar o feedback de desempenho.")
	}
}
