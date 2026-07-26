package bi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	allowedRoles := []auth.Role{
		auth.RoleManager,
		auth.RoleMarketing,
		auth.RoleDirector,
		auth.RoleOwner,
		auth.RolePlatformAdmin,
	}

	mux.Handle("POST /v1/bi/perola/login", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input PerolaLoginInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		response, err := service.PerolaLogin(r.Context(), input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}), allowedRoles...))

	mux.Handle("POST /v1/bi/perola/find", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input PerolaFindInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		response, err := service.PerolaFind(r.Context(), input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}), allowedRoles...))

	mux.Handle("GET /v1/bi/perola/datasets", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, service.PerolaDatasetCatalog())
	}), allowedRoles...))

	mux.Handle("GET /v1/bi/perola/sales/recent", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := service.PerolaRecentSales(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}), allowedRoles...))

	mux.Handle("POST /v1/bi/perola/datasets/{dataset}/query", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input PerolaDatasetQueryInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		response, err := service.QueryPerolaDataset(r.Context(), r.PathValue("dataset"), input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}), allowedRoles...))

	mux.Handle("GET /v1/bi/perola/overview", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := service.PerolaOverview(r.Context(), PerolaOverviewInput{
			CompanyKey:       r.Header.Get("X-Perola-Company-Key"),
			CNPJEmpresa:      r.URL.Query().Get("cnpjEmpresa"),
			Token:            r.Header.Get("X-Perola-Token"),
			IncludeInventory: r.URL.Query().Get("includeInventory") == "1" || r.URL.Query().Get("includeInventory") == "true",
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}), allowedRoles...))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrConfiguration):
		httpapi.WriteError(w, r, http.StatusBadRequest, "missing_bi_configuration", "Configure as credenciais da Perola BI no backend.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados enviados para a Perola BI.")
	case errors.Is(err, ErrUnsupportedEndpoint):
		httpapi.WriteError(w, r, http.StatusBadRequest, "unsupported_endpoint", "Endpoint da Perola BI nao permitido.")
	case errors.Is(err, ErrUnsupportedDataset):
		httpapi.WriteError(w, r, http.StatusNotFound, "dataset_not_found", "Dataset da Perola BI nao encontrado.")
	case errors.Is(err, ErrFilterRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "filter_required", "A consulta exige um filtro seletivo.")
	case errors.Is(err, ErrSalesUnauthorized):
		httpapi.WriteError(w, r, http.StatusBadGateway, "sales_unauthorized", "A Datajoias ainda nao autorizou esta credencial a consultar Vendas.")
	case errors.Is(err, ErrUpstream):
		httpapi.WriteError(w, r, http.StatusBadGateway, "upstream_error", "Nao foi possivel conectar na Perola BI.")
	default:
		slog.Error("bi_request_failed", slog.String("path", r.URL.Path), slog.Any("error", err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar BI.")
	}
}
