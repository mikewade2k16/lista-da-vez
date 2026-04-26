package settings

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// As rotas continuam aceitando os campos legados storeId no payload e na query
// string para nao quebrar clientes intermediarios, mas o servico ignora esses
// valores e resolve o tenant pelo principal autenticado. A configuracao agora
// e tenant-wide: nao existe escopo por loja para esses recursos.
func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/settings", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		bundle, err := service.GetBundle(r.Context(), principal)
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

		ack, err := service.SaveModalSection(r.Context(), principal, input)
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

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.SaveOptionItem(r.Context(), principal, optionGroup, input.Item)
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

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.SaveOptionItem(r.Context(), principal, optionGroup, OptionItem{
			ID:    strings.TrimSpace(r.PathValue("itemId")),
			Label: input.Label,
		})
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

		optionGroup, err := normalizeOptionGroupPath(r.PathValue("group"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		ack, err := service.SaveOptionSection(r.Context(), principal, optionGroup, input.Items)
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

		ack, err := service.SaveProductItem(r.Context(), principal, ProductItemInput{
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

		ack, err := service.SaveProductSection(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, ack)
	})))
}

func normalizeOptionGroupPath(rawGroup string) (string, error) {
	switch strings.TrimSpace(rawGroup) {
	case "visit-reasons":
		return optionKindVisitReason, nil
	case "customer-sources":
		return optionKindCustomerSource, nil
	case "pause-reasons":
		return optionKindPauseReason, nil
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
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrTenantRequired), errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados de configuracao.")
	case errors.Is(err, ErrTenantNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "tenant_not_found", "Tenant nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar as configuracoes.")
	}
}
