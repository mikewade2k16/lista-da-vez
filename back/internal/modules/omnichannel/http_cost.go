package omnichannel

import (
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// ============================================================================
// F13 — Rota do custo de LLM por conta (C7)
// ============================================================================
//
// GET /v1/omnichannel/ai/usage?from=&to= — custo AGREGADO por conta e periodo. Permissao
// PRETENDIDA: omnichannel.audit.view. Como nao ha middleware de permissao por key no Go (gap
// honesto declarado no module.go), a rota fica sob RequireAuthWithAccount + gate de modulo +
// escopo de conta, IGUAL a rota de auditoria de roteamento (routing-decisions, F8). O
// enforcement por key vira load-bearing junto com as demais rotas de escrita/auditoria.
//
// AGREGA, nao lista: GET /agents/{id}/runs (F9) ja lista run a run. accountID vem do Principal.

// RegisterCostRoutes registra a rota de custo. Chamada pelo orquestrador (module.go) — ver
// AGENT.md "Wiring pendente"; este arquivo nao edita o module.go.
func RegisterCostRoutes(mux *http.ServeMux, svc *CostService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}
	mux.Handle("GET /v1/omnichannel/ai/usage", wrap(handleAIUsage(svc)))
}

func handleAIUsage(svc *CostService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		from, toExclusive := parseUsageWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
		report, err := svc.Usage(r.Context(), accountID, from, toExclusive)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, report)
	}
}

// parseUsageWindow resolve o intervalo [from, toExclusive). Default = mes corrente (C7): from =
// primeiro dia do mes; toExclusive = agora. `to` informado e tratado como INCLUSIVO do dia
// (soma 24h). Datas ilegiveis caem no default — a tela nunca quebra por query ruim.
func parseUsageWindow(fromRaw, toRaw string, now time.Time) (time.Time, time.Time) {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	from := monthStart
	if d, ok := parseDay(fromRaw); ok {
		from = d
	}
	toExclusive := now
	if d, ok := parseDay(toRaw); ok {
		toExclusive = d.AddDate(0, 0, 1)
	}
	if !toExclusive.After(from) {
		toExclusive = from.AddDate(0, 0, 1) // janela minima de 1 dia (nunca vazia/invertida)
	}
	return from, toExclusive
}

// parseDay le uma data YYYY-MM-DD (UTC). Vazio/invalido => ok=false.
func parseDay(raw string) (time.Time, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation("2006-01-02", v, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}
