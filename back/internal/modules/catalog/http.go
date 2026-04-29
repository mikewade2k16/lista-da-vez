package catalog

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/catalog/products/search", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		input, err := parseSearchProductsInput(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		response, err := service.SearchProducts(r.Context(), accessContextFromPrincipal(principal), input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))
}

func parseSearchProductsInput(r *http.Request) (SearchProductsInput, error) {
	query := r.URL.Query()

	limit, err := parseOptionalInt(query.Get("limit"))
	if err != nil {
		return SearchProductsInput{}, err
	}

	return SearchProductsInput{
		StoreID:   strings.TrimSpace(query.Get("storeId")),
		SourceKey: ProductSourceKey(strings.TrimSpace(query.Get("sourceKey"))),
		Term:      strings.TrimSpace(firstNonEmpty(query.Get("term"), query.Get("search"))),
		Limit:     limit,
	}, nil
}

func parseOptionalInt(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return strconv.Atoi(trimmed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func accessContextFromPrincipal(principal auth.Principal) AccessContext {
	return AccessContext{
		UserID:   strings.TrimSpace(principal.UserID),
		TenantID: strings.TrimSpace(principal.TenantID),
		Role:     strings.TrimSpace(string(principal.Role)),
		StoreIDs: append([]string{}, principal.StoreIDs...),
	}
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrStoreRequired), errors.Is(err, ErrSearchTermTooShort):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os parametros da busca de produtos.")
	case errors.Is(err, ErrUnsupportedProductSource):
		httpapi.WriteError(w, r, http.StatusBadRequest, "unsupported_source", "Fonte de catalogo nao suportada.")
	case errors.Is(err, stores.ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, stores.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar a busca de catalogo.")
	}
}
