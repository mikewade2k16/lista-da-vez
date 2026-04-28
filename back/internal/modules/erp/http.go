package erp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	handleRawRecords := func(dataType string) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
				return
			}

			query, err := parseRawRecordsQuery(r)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}
			if strings.TrimSpace(dataType) != "" {
				query.DataType = dataType
			}

			response, err := service.Records(r.Context(), principal, query)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}

			httpapi.WriteJSON(w, http.StatusOK, response)
		}))
	}

	mux.Handle("GET /v1/erp/status", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		status, err := service.Status(
			r.Context(),
			principal,
			strings.TrimSpace(r.URL.Query().Get("tenantId")),
			strings.TrimSpace(r.URL.Query().Get("storeCode")),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, status)
	})))

	mux.Handle("GET /v1/erp/products", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		query, err := parseProductQuery(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		response, err := service.Products(r.Context(), principal, query)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("GET /v1/erp/records", handleRawRecords(""))
	mux.Handle("GET /v1/erp/customers", handleRawRecords(DataTypeCustomer))
	mux.Handle("GET /v1/erp/employees", handleRawRecords(DataTypeEmployee))
	mux.Handle("GET /v1/erp/orders", handleRawRecords(DataTypeOrder))
	mux.Handle("GET /v1/erp/orders/canceled", handleRawRecords(DataTypeOrderCanceled))

	mux.Handle("POST /v1/erp/bootstrap/items", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input ItemBootstrapInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.BootstrapItems(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, result)
	})))

	mux.Handle("POST /v1/erp/bootstrap", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input BootstrapInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.Bootstrap(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, result)
	})))
}

func parseProductQuery(r *http.Request) (ProductQuery, error) {
	query := r.URL.Query()

	page, err := parseOptionalInt(query.Get("page"))
	if err != nil {
		return ProductQuery{}, ErrValidation
	}
	pageSize, err := parseOptionalInt(query.Get("pageSize"))
	if err != nil {
		return ProductQuery{}, ErrValidation
	}

	return ProductQuery{
		TenantID:         strings.TrimSpace(query.Get("tenantId")),
		StoreCode:        strings.TrimSpace(query.Get("storeCode")),
		IdentifierPrefix: strings.TrimSpace(query.Get("identifierPrefix")),
		Search:           strings.TrimSpace(query.Get("search")),
		Page:             page,
		PageSize:         pageSize,
	}, nil
}

func parseRawRecordsQuery(r *http.Request) (RawRecordsQuery, error) {
	query := r.URL.Query()

	page, err := parseOptionalInt(query.Get("page"))
	if err != nil {
		return RawRecordsQuery{}, ErrValidation
	}
	pageSize, err := parseOptionalInt(query.Get("pageSize"))
	if err != nil {
		return RawRecordsQuery{}, ErrValidation
	}

	return RawRecordsQuery{
		TenantID:       strings.TrimSpace(query.Get("tenantId")),
		StoreCode:      strings.TrimSpace(query.Get("storeCode")),
		DataType:       strings.TrimSpace(query.Get("dataType")),
		Search:         strings.TrimSpace(query.Get("search")),
		SpecificSearch: strings.TrimSpace(firstNonEmpty(query.Get("specificSearch"), query.Get("keySearch"))),
		Page:           page,
		PageSize:       pageSize,
	}, nil
}

func parseOptionalInt(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return strconv.Atoi(trimmed)
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrManualSyncDisabled):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrStoreRequired), errors.Is(err, ErrTenantRequired), errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os parametros enviados.")
	case errors.Is(err, ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, ErrSourceNotConfigured), errors.Is(err, ErrSourcePathOutsideRoot):
		httpapi.WriteError(w, r, http.StatusBadRequest, "source_error", "Origem do bootstrap ERP invalida ou nao configurada.")
	default:
		slog.Error("erp_request_failed", slog.String("path", r.URL.Path), slog.Any("error", err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar o ERP.")
	}
}
