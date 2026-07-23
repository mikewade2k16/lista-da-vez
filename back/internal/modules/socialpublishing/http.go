package socialpublishing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxRequestBodyBytes = 64 << 10

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func RegisterRoutes(
	mux *http.ServeMux,
	service *Service,
	middleware *auth.Middleware,
	gate *permissionGate,
) {
	wrap := func(permission string, handler http.HandlerFunc) http.Handler {
		authorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok || principal.AccountID == "" {
				httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
				return
			}
			if err := gate.Authorize(r.Context(), principal, permission); err != nil {
				if errors.Is(err, ErrForbidden) {
					httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta acao.")
				} else {
					httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao validar permissao.")
				}
				return
			}
			handler.ServeHTTP(w, r)
		})
		return middleware.RequireAuthWithAccount(authorized)
	}

	mux.Handle("GET /v1/social-publishing/connection", wrap(PermissionView, handleConnectionGet(service)))
	mux.Handle("POST /v1/social-publishing/connection", wrap(PermissionConnect, handleConnectionPost(service)))
	mux.Handle("DELETE /v1/social-publishing/connection", wrap(PermissionConnect, handleConnectionDelete(service)))
	mux.Handle("GET /v1/social-publishing/posts", wrap(PermissionView, handlePostsGet(service)))
	mux.Handle("POST /v1/social-publishing/posts", wrap(PermissionManage, handlePostsPost(service)))
	mux.Handle("GET /v1/social-publishing/posts/{id}", wrap(PermissionView, handlePostGet(service)))
	mux.Handle("PATCH /v1/social-publishing/posts/{id}", wrap(PermissionManage, handlePostPatch(service)))
	mux.Handle("POST /v1/social-publishing/posts/{id}/schedule", wrap(PermissionManage, handlePostSchedule(service)))
	mux.Handle("POST /v1/social-publishing/posts/{id}/cancel", wrap(PermissionManage, handlePostCancel(service)))
	mux.Handle("POST /v1/social-publishing/posts/{id}/retry", wrap(PermissionManage, handlePostRetry(service)))
	mux.Handle("GET /v1/social-publishing/overview", wrap(PermissionAnalytics, handleOverview(service)))
	mux.Handle("GET /v1/social-publishing/analytics/posts", wrap(PermissionAnalytics, handleAnalyticsPosts(service)))
	mux.Handle("POST /v1/social-publishing/analytics/sync", wrap(PermissionAnalytics, handleAnalyticsSync(service)))
}

func handleConnectionGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		connection, err := service.Connection(r.Context(), principal.AccountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, connection)
	}
}

func handleConnectionPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccessToken string `json:"accessToken"`
		}
		if err := decodeBody(w, r, &body); err != nil {
			writeInvalidBody(w, r)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		connection, err := service.Connect(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			body.AccessToken,
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, connection)
	}
}

func handleConnectionDelete(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		if err := service.Disconnect(r.Context(), principal.AccountID); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handlePostsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		posts, err := service.ListPosts(r.Context(), principal.AccountID, ListPostsFilter{
			Status: PostStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, posts)
	}
}

func handlePostsPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Caption        string     `json:"caption"`
			MediaURL       string     `json:"mediaUrl"`
			MediaType      string     `json:"mediaType"`
			AltText        string     `json:"altText"`
			Status         PostStatus `json:"status"`
			ScheduledFor   *time.Time `json:"scheduledFor"`
			Timezone       string     `json:"timezone"`
			IdempotencyKey string     `json:"idempotencyKey"`
		}
		if err := decodeBody(w, r, &body); err != nil {
			writeInvalidBody(w, r)
			return
		}
		body.IdempotencyKey = strings.TrimSpace(body.IdempotencyKey)
		if !idempotencyKeyPattern.MatchString(body.IdempotencyKey) {
			httpapi.WriteError(
				w,
				r,
				http.StatusBadRequest,
				"invalid_idempotency_key",
				"idempotencyKey obrigatorio e deve ter de 1 a 128 caracteres seguros.",
			)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		result, err := service.CreatePost(r.Context(), principal.AccountID, principal.UserID, CreatePostInput{
			Caption:      body.Caption,
			MediaURL:     body.MediaURL,
			MediaType:    body.MediaType,
			AltText:      body.AltText,
			Status:       body.Status,
			ScheduledFor: body.ScheduledFor,
			Timezone:     body.Timezone,
			SourceType:   "manual",
			SourceRef:    body.IdempotencyKey,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		status := http.StatusCreated
		if !result.Created {
			status = http.StatusOK
		}
		httpapi.WriteJSON(w, status, result.Post)
	}
}

func handlePostGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		post, err := service.Post(r.Context(), principal.AccountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, post)
	}
}

func handlePostPatch(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body PatchPostInput
		if err := decodeBody(w, r, &body); err != nil {
			writeInvalidBody(w, r)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		post, err := service.PatchPost(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			r.PathValue("id"),
			body,
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, post)
	}
}

func handlePostSchedule(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body SchedulePostInput
		if err := decodeBody(w, r, &body); err != nil {
			writeInvalidBody(w, r)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		post, err := service.SchedulePost(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			r.PathValue("id"),
			body,
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, post)
	}
}

func handlePostCancel(service *Service) http.HandlerFunc {
	return versionAction(service.CancelPost)
}

func handlePostRetry(service *Service) http.HandlerFunc {
	return versionAction(service.RetryPost)
}

func versionAction(
	action func(context.Context, string, string, string, VersionInput) (Post, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body VersionInput
		if err := decodeBody(w, r, &body); err != nil {
			writeInvalidBody(w, r)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		post, err := action(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			r.PathValue("id"),
			body,
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, post)
	}
}

func handleOverview(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		overview, err := service.Overview(r.Context(), principal.AccountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, overview)
	}
}

func handleAnalyticsSync(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		queued, err := service.QueueAnalyticsSync(r.Context(), principal.AccountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, map[string]int{"queued": queued})
	}
}

func handleAnalyticsPosts(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		analytics, err := service.ListAnalytics(r.Context(), principal.AccountID, limit)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, analytics)
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalidInput
	}
	return nil
}

func writeInvalidBody(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrNotConnected):
		httpapi.WriteError(w, r, http.StatusConflict, "instagram_not_connected", "Conecte uma conta profissional do Instagram.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta acao.")
	case errors.Is(err, ErrConflict), errors.Is(err, ErrInvalidState):
		httpapi.WriteError(w, r, http.StatusConflict, "state_conflict", "O post foi alterado ou nao aceita esta operacao.")
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidMediaURL),
		errors.Is(err, ErrInvalidTimezone), errors.Is(err, ErrScheduleInPast),
		errors.Is(err, ErrInvalidToken):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "validation_error", "Revise os dados informados.")
	case errors.Is(err, ErrSecretsUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "secrets_unavailable", "Cofre de segredos indisponivel.")
	default:
		var providerErr *ProviderError
		if errors.As(err, &providerErr) || errors.Is(err, ErrProviderUnavailable) {
			httpapi.WriteError(w, r, http.StatusBadGateway, "instagram_error", "Instagram indisponivel ou recusou a requisicao.")
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a solicitacao.")
	}
}
