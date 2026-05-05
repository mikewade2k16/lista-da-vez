package settings

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// As rotas continuam aceitando storeId legado, mas a configuracao agora e
// tenant-wide. Para usuarios globais, a UI deve enviar tenantId do contexto
// ativo na query ou no payload; o servico valida o acesso antes de ler/gravar.
func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware, env string) {
	mux.Handle("GET /v1/settings", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		if handleDebugSettingsFailure(w, r, env) {
			return
		}

		bundle, err := service.GetBundle(r.Context(), principal, requestTenantID(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, bundle)
	})))

	mux.Handle("PUT /v1/settings", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var bundle Bundle
		if err := httpapi.ReadJSON(r, &bundle); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		bundle.TenantID = firstNonEmpty(bundle.TenantID, requestTenantID(r))

		ack, err := service.SaveBundle(r.Context(), principal, bundle)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("PATCH /v1/settings/operation", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input OperationSectionInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		ack, err := service.SaveOperationSection(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("PATCH /v1/settings/modal", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input ModalSectionInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		ack, err := service.SaveModalSection(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/settings/templates/{templateId}/apply", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		ack, err := service.ApplyOperationTemplate(r.Context(), principal, OperationTemplateApplyInput{
			TenantID:   requestTenantID(r),
			TemplateID: strings.TrimSpace(r.PathValue("templateId")),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/settings/options/{group}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input OptionItemInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.SaveOptionItem(r.Context(), principal, optionGroup, input.Item, input.TenantID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, ack)
	})))

	mux.Handle("PATCH /v1/settings/options/{group}/{itemId}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input OptionItemPatchInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.SaveOptionItem(r.Context(), principal, optionGroup, OptionItem{
			ID:    strings.TrimSpace(r.PathValue("itemId")),
			Label: input.Label,
		}, input.TenantID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("DELETE /v1/settings/options/{group}/{itemId}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.DeleteOptionItem(
			r.Context(),
			principal,
			optionGroup,
			r.PathValue("itemId"),
			requestTenantID(r),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("PUT /v1/settings/options/{group}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input OptionSectionInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.SaveOptionSection(r.Context(), principal, optionGroup, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("POST /v1/settings/products", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input ProductItemInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		ack, err := service.SaveProductItem(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, ack)
	})))

	mux.Handle("PATCH /v1/settings/products/{itemId}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input ProductItemPatchInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		ack, err := service.SaveProductItem(r.Context(), principal, ProductItemInput{
			TenantID: input.TenantID,
			Item: ProductItem{
				ID:        strings.TrimSpace(r.PathValue("itemId")),
				Name:      input.Name,
				Code:      input.Code,
				Category:  input.Category,
				BasePrice: input.BasePrice,
			},
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("DELETE /v1/settings/products/{itemId}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		ack, err := service.DeleteProductItem(
			r.Context(),
			principal,
			r.PathValue("itemId"),
			requestTenantID(r),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))

	mux.Handle("PUT /v1/settings/products", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input ProductSectionInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TenantID = firstNonEmpty(input.TenantID, requestTenantID(r))

		ack, err := service.SaveProductSection(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))
}

func requestTenantID(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("tenantId"))
}

func handleDebugSettingsFailure(w http.ResponseWriter, r *http.Request, env string) bool {
	if strings.EqualFold(strings.TrimSpace(env), "production") {
		return false
	}

	mode := strings.TrimSpace(r.URL.Query().Get("__debugSettingsFailure"))
	if mode == "" {
		cookie, err := r.Cookie("ldv_debug_settings_failure")
		if err == nil {
			mode = strings.TrimSpace(cookie.Value)
		}
	}

	switch strings.ToLower(mode) {
	case "":
		return false
	case "500":
		httpapi.WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"debug_settings_failure",
			"Falha simulada de settings para smoke local.",
		)
		return true
	case "slow-500":
		time.Sleep(12 * time.Second)
		httpapi.WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"debug_settings_failure",
			"Falha lenta simulada de settings para smoke local.",
		)
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}

	return ""
}

func normalizeOptionGroupPath(rawGroup string) (string, error) {
	switch strings.TrimSpace(rawGroup) {
	case "visit-reasons":
		return optionKindVisitReason, nil
	case "customer-sources":
		return optionKindCustomerSource, nil
	case "pause-reasons":
		return optionKindPauseReason, nil
	case "cancel-reasons":
		return optionKindCancelReason, nil
	case "stop-reasons":
		return optionKindStopReason, nil
	case "queue-jump-reasons":
		return optionKindQueueJump, nil
	case "loss-reasons":
		return optionKindLossReason, nil
	case "professions":
		return optionKindProfession, nil
	default:
		return "", ErrValidation
	}
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Warn("settings_http_error",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String("tenantQuery", r.URL.Query().Get("tenantId")),
		slog.String("requestId", httpapi.RequestIDFromContext(r.Context())),
		slog.Any("error", err))

	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrTenantRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "tenant_required", "Tenant ativo nao identificado para a sessao. Informe tenantId.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados de configuracao.")
	case errors.Is(err, ErrTenantNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "tenant_not_found", "Tenant nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar as configuracoes.")
	}
}
