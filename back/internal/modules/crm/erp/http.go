package erp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	mux.Handle("GET /v1/erp/overview", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		overview, err := service.Overview(
			r.Context(),
			principal,
			strings.TrimSpace(r.URL.Query().Get("tenantId")),
			strings.TrimSpace(r.URL.Query().Get("storeCode")),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, overview)
	})))

	mux.Handle("GET /v1/erp/crm", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		query, err := parseCRMOverviewQuery(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		overview, err := service.CRMOverview(r.Context(), principal, query)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, overview)
	})))

	mux.Handle("GET /v1/erp/consultant-links", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		response, err := service.ConsultantERPLinks(
			r.Context(),
			principal,
			strings.TrimSpace(r.URL.Query().Get("tenantId")),
			strings.TrimSpace(r.URL.Query().Get("storeCode")),
			parseConsultantLinkEmployeeIDs(r),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("PUT /v1/erp/consultant-links", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input ConsultantERPLinkUpsertInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		response, err := service.UpsertConsultantERPLink(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("POST /v1/erp/consultant-links/auto", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		response, err := service.AutoLinkConsultantERP(
			r.Context(),
			principal,
			strings.TrimSpace(r.URL.Query().Get("tenantId")),
			strings.TrimSpace(r.URL.Query().Get("storeCode")),
			parseConsultantLinkEmployeeIDs(r),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("DELETE /v1/erp/consultant-links/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		response, err := service.DeleteConsultantERPLink(r.Context(), principal, ConsultantERPLinkDeleteInput{
			TenantID:    strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreCode:   strings.TrimSpace(r.URL.Query().Get("storeCode")),
			EmployeeIDs: parseConsultantLinkEmployeeIDs(r),
			LinkID:      strings.TrimSpace(r.PathValue("id")),
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	})))

	mux.Handle("GET /v1/erp/runs", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		query, err := parseRunsQuery(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		response, err := service.Runs(r.Context(), principal, query)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
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

	mux.Handle("GET /v1/erp/stats", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		query := RecordsStatsQuery{
			TenantID:       strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreCode:      strings.TrimSpace(r.URL.Query().Get("storeCode")),
			DataType:       strings.TrimSpace(r.URL.Query().Get("dataType")),
			Search:         strings.TrimSpace(r.URL.Query().Get("search")),
			SpecificSearch: strings.TrimSpace(firstNonEmpty(r.URL.Query().Get("specificSearch"), r.URL.Query().Get("keySearch"))),
			DateFrom:       strings.TrimSpace(r.URL.Query().Get("dateFrom")),
			DateTo:         strings.TrimSpace(r.URL.Query().Get("dateTo")),
			DateField:      normalizeDateField(r.URL.Query().Get("dateField")),
			MinValueCents:  parseOptionalCents(r.URL.Query().Get("minValueCents")),
			StoreFilter:    strings.TrimSpace(r.URL.Query().Get("storeFilter")),
			EmployeeFilter: strings.TrimSpace(r.URL.Query().Get("employeeFilter")),
		}
		result, err := service.RecordsStats(r.Context(), principal, query)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	})))

	mux.Handle("GET /v1/erp/records/facets", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		query := RecordsFacetsQuery{
			TenantID:    strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreCode:   strings.TrimSpace(r.URL.Query().Get("storeCode")),
			DataType:    strings.TrimSpace(r.URL.Query().Get("dataType")),
			DateFrom:    strings.TrimSpace(r.URL.Query().Get("dateFrom")),
			DateTo:      strings.TrimSpace(r.URL.Query().Get("dateTo")),
			DateField:   normalizeDateField(r.URL.Query().Get("dateField")),
			StoreFilter: strings.TrimSpace(r.URL.Query().Get("storeFilter")),
		}
		result, err := service.RecordsFacets(r.Context(), principal, query)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
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

	mux.Handle("POST /v1/erp/sync", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input IngestInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		if strings.TrimSpace(input.TriggeredBy) == "" {
			input.TriggeredBy = SyncTriggeredByManual
		}

		result, err := service.IngestStore(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, result)
	})))

	mux.Handle("POST /v1/erp/backfill", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var input IngestInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
		input.TriggeredBy = SyncTriggeredByBackfill

		result, err := service.IngestStore(r.Context(), principal, input)
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
		SortBy:           strings.TrimSpace(query.Get("sortBy")),
		SortDir:          strings.TrimSpace(query.Get("sortDir")),
		DateFrom:         strings.TrimSpace(query.Get("dateFrom")),
		DateTo:           strings.TrimSpace(query.Get("dateTo")),
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
		Dedup:          strings.TrimSpace(query.Get("dedup")) == "true",
		SortBy:         strings.TrimSpace(query.Get("sortBy")),
		SortDir:        strings.TrimSpace(query.Get("sortDir")),
		DateFrom:       strings.TrimSpace(query.Get("dateFrom")),
		DateTo:         strings.TrimSpace(query.Get("dateTo")),
		DateField:      normalizeDateField(query.Get("dateField")),
		MinValueCents:  parseOptionalCents(query.Get("minValueCents")),
		StoreFilter:    strings.TrimSpace(query.Get("storeFilter")),
		EmployeeFilter: strings.TrimSpace(query.Get("employeeFilter")),
	}, nil
}

// normalizeDateField restringe o filtro de periodo de pedidos a um valor seguro:
// "batch_date" (data do lote) ou "order_date" (PADRAO, data real da compra).
// Qualquer outro valor cai em order_date. Vai para SQL como enum controlado.
func normalizeDateField(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "batch_date") {
		return "batch_date"
	}
	return "order_date"
}

// parseOptionalCents le um valor em centavos da query; vazio/invalido/negativo = 0
// (sem filtro). Tolerante por design: filtro ausente nao deve quebrar a requisicao.
func parseOptionalCents(raw string) int64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func parseRunsQuery(r *http.Request) (RunsQuery, error) {
	query := r.URL.Query()

	page, err := parseOptionalInt(query.Get("page"))
	if err != nil {
		return RunsQuery{}, ErrValidation
	}
	pageSize, err := parseOptionalInt(query.Get("pageSize"))
	if err != nil {
		return RunsQuery{}, ErrValidation
	}

	return RunsQuery{
		TenantID:  strings.TrimSpace(query.Get("tenantId")),
		StoreCode: strings.TrimSpace(query.Get("storeCode")),
		DataType:  strings.TrimSpace(query.Get("dataType")),
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

func parseConsultantLinkEmployeeIDs(r *http.Request) []string {
	query := r.URL.Query()
	values := make([]string, 0, len(query["employeeId"])+1)
	for _, value := range query["employeeId"] {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	for _, value := range strings.Split(strings.TrimSpace(query.Get("employeeIds")), ",") {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func parseCRMOverviewQuery(r *http.Request) (CRMOverviewQuery, error) {
	query := r.URL.Query()

	dateFrom, dateFromHasTime, err := parseOptionalDateTime(query.Get("dateFrom"))
	if err != nil {
		return CRMOverviewQuery{}, ErrValidation
	}
	dateTo, dateToHasTime, err := parseOptionalDateTime(query.Get("dateTo"))
	if err != nil {
		return CRMOverviewQuery{}, ErrValidation
	}

	return CRMOverviewQuery{
		TenantID:        strings.TrimSpace(query.Get("tenantId")),
		StoreCode:       strings.TrimSpace(query.Get("storeCode")),
		DateFrom:        dateFrom,
		DateTo:          dateTo,
		DateFromHasTime: dateFromHasTime,
		DateToHasTime:   dateToHasTime,
	}, nil
}

func parseOptionalInt(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return strconv.Atoi(trimmed)
}

func parseOptionalDateTime(raw string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false, nil
	}

	layouts := []struct {
		layout  string
		hasTime bool
	}{
		{layout: time.RFC3339, hasTime: true},
		{layout: "2006-01-02T15:04:05", hasTime: true},
		{layout: "2006-01-02T15:04", hasTime: true},
		{layout: "2006-01-02 15:04:05", hasTime: true},
		{layout: "2006-01-02 15:04", hasTime: true},
		{layout: "2006-01-02", hasTime: false},
	}

	var lastErr error
	for _, item := range layouts {
		parsed, err := time.Parse(item.layout, trimmed)
		if err == nil {
			return parsed.UTC(), item.hasTime, nil
		}
		lastErr = err
	}

	return time.Time{}, false, lastErr
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrManualSyncDisabled):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrSyncAlreadyRunning):
		httpapi.WriteError(w, r, http.StatusConflict, "sync_already_running", "Ja existe uma sincronizacao ERP em andamento.")
	case errors.Is(err, ErrSyncRateLimited):
		httpapi.WriteError(w, r, http.StatusTooManyRequests, "sync_rate_limited", "Aguarde antes de disparar outra sincronizacao ERP.")
	case errors.Is(err, ErrStoreRequired), errors.Is(err, ErrTenantRequired), errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os parametros enviados.")
	case errors.Is(err, ErrUnsupportedDataType):
		httpapi.WriteError(w, r, http.StatusBadRequest, "unsupported_data_type", "Tipo de dado ERP nao suportado.")
	case errors.Is(err, ErrStoreNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
	case errors.Is(err, ErrSourceNotConfigured), errors.Is(err, ErrSourcePathOutsideRoot), errors.Is(err, ErrSourceHostKeyRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "source_error", "Origem do bootstrap ERP invalida ou nao configurada.")
	case errors.As(err, new(*ErrCSVTooLarge)):
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "csv_too_large", err.Error())
	case errors.As(err, new(*ErrCSVColumnCountMismatch)), errors.As(err, new(*ErrCSVHeaderMismatch)), errors.As(err, new(*ErrCSVRowParse)), errors.As(err, new(*ErrCSVEncoding)), errors.As(err, new(*ErrCSVFilenameInvalid)):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "csv_error", err.Error())
	default:
		slog.Error("erp_request_failed", slog.String("path", r.URL.Path), slog.Any("error", err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar o ERP.")
	}
}
