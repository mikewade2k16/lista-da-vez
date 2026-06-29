package cardapio

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Handlers do analytics do painel (Fase 10 / F2). Base
// /v1/cardapio/restaurants/{id}/analytics/*. Mesmo auth/escopo dos demais GETs do
// painel: RequireAuth + gating de modulo (cardapio) no Chain + scopedAccountID
// (accountId da query tem precedencia sobre X-Account-Id). O service valida o
// pertencimento do restaurante (404 uniforme fora do escopo). Todos respondem com
// Cache-Control: private, max-age=60 e excluem bots por padrao.

// RegisterAnalyticsRoutes monta os 9 endpoints GET de analytics sob /v1/cardapio. O
// gating de modulo ja cobre o prefixo; aqui exigimos apenas autenticacao (como os
// demais GETs do painel, que reusam a permissao cardapio.view do mesmo mecanismo).
func RegisterAnalyticsRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}
	const base = "/v1/cardapio/restaurants/{id}/analytics/"
	mux.Handle("GET "+base+"overview", wrap(handleAnalyticsOverview(svc)))
	mux.Handle("GET "+base+"timeseries", wrap(handleAnalyticsTimeseries(svc)))
	mux.Handle("GET "+base+"funnel", wrap(handleAnalyticsFunnel(svc)))
	mux.Handle("GET "+base+"top-products", wrap(handleAnalyticsTopProducts(svc)))
	mux.Handle("GET "+base+"sources", wrap(handleAnalyticsSources(svc)))
	mux.Handle("GET "+base+"devices", wrap(handleAnalyticsDevices(svc)))
	mux.Handle("GET "+base+"pages", wrap(handleAnalyticsPages(svc)))
	mux.Handle("GET "+base+"dwell", wrap(handleAnalyticsDwell(svc)))
	mux.Handle("GET "+base+"clicks", wrap(handleAnalyticsClicks(svc)))
}

// analyticsContext resolve o escopo (accountId validado contra o Principal) e o range
// comum. Em erro de escopo/range, escreve a resposta HTTP e retorna ok=false. O
// restaurantId vem do path; a validacao de pertencimento e feita no service.
func analyticsContext(w http.ResponseWriter, r *http.Request, svc *Service) (accountID, restaurantID string, rg analyticsRange, ok bool) {
	accountID, _, err := scopedAccountID(r)
	if err != nil {
		writeServiceError(w, r, err)
		return "", "", analyticsRange{}, false
	}
	// Analytics e leitura: exige cardapio.view (curto-circuito de platform_admin/
	// agency_owner). Gate unico para os 9 GETs; falha => 404 uniforme.
	if err := requireCardapioPerm(svc, r, accountID, permView); err != nil {
		writeServiceError(w, r, err)
		return "", "", analyticsRange{}, false
	}
	rg, err = resolveRange(AnalyticsParams{
		From: r.URL.Query().Get("from"),
		To:   r.URL.Query().Get("to"),
		TZ:   r.URL.Query().Get("tz"),
	}, time.Now())
	if err != nil {
		writeServiceError(w, r, err)
		return "", "", analyticsRange{}, false
	}
	return accountID, r.PathValue("id"), rg, true
}

// writeAnalytics serializa o payload com o Cache-Control de analytics.
func writeAnalytics(w http.ResponseWriter, payload any) {
	w.Header().Set("Cache-Control", "private, max-age=60")
	httpapi.WriteJSON(w, http.StatusOK, payload)
}

// parseLimit le ?limit (default no service via clampLimit; aqui so converte).
func parseLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	return limit
}

func handleAnalyticsOverview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		out, err := svc.Overview(r.Context(), accountID, restaurantID, rg)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsTimeseries(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		granularity := strings.TrimSpace(r.URL.Query().Get("granularity"))
		if granularity == "" {
			granularity = "day"
		}
		if _, valid := timeseriesGranularities[granularity]; !valid {
			writeServiceError(w, r, ErrValidation)
			return
		}
		out, err := svc.Timeseries(r.Context(), accountID, restaurantID, granularity, rg)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsFunnel(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		out, err := svc.Funnel(r.Context(), accountID, restaurantID, rg)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsTopProducts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		metric := strings.TrimSpace(r.URL.Query().Get("metric"))
		if metric == "" {
			metric = "viewed"
		}
		if _, valid := topProductMetrics[metric]; !valid {
			writeServiceError(w, r, ErrValidation)
			return
		}
		out, err := svc.TopProducts(r.Context(), accountID, restaurantID, metric, rg, parseLimit(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsSources(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		dimension := strings.TrimSpace(r.URL.Query().Get("dimension"))
		if dimension == "" {
			dimension = "utm_source"
		}
		if _, valid := sourceDimensions[dimension]; !valid {
			writeServiceError(w, r, ErrValidation)
			return
		}
		out, err := svc.Sources(r.Context(), accountID, restaurantID, dimension, rg, parseLimit(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsDevices(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		out, err := svc.Devices(r.Context(), accountID, restaurantID, rg)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsPages(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		out, err := svc.Pages(r.Context(), accountID, restaurantID, rg, parseLimit(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsDwell(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		dimension := strings.TrimSpace(r.URL.Query().Get("dimension"))
		if dimension == "" {
			dimension = "page"
		}
		if _, valid := dwellDimensions[dimension]; !valid {
			writeServiceError(w, r, ErrValidation)
			return
		}
		out, err := svc.Dwell(r.Context(), accountID, restaurantID, dimension, rg, parseLimit(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}

func handleAnalyticsClicks(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, restaurantID, rg, ok := analyticsContext(w, r, svc)
		if !ok {
			return
		}
		out, err := svc.Clicks(r.Context(), accountID, restaurantID, rg, parseLimit(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeAnalytics(w, out)
	}
}
