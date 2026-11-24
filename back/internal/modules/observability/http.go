package observability

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	platformobs "github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
)

const adminRoot = "/v1/admin/observability"

var (
	runIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	modulePattern = regexp.MustCompile(`^[a-z0-9_-]{0,64}$`)
	onlyPattern   = regexp.MustCompile(`^[a-z,]{0,64}$`)
)

// RegisterRoutes monta o painel (somente platform_admin: a telemetria é global,
// atravessa tenants) e o coletor do front (qualquer usuário logado).
func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	admin := func(h http.HandlerFunc) http.Handler { return middleware.RequireRoles(h, auth.RolePlatformAdmin) }
	handler := &httpHandler{service: service}

	mux.Handle("GET "+adminRoot+"/overview", admin(handler.overview))
	mux.Handle("GET "+adminRoot+"/endpoints", admin(handler.endpoints))
	mux.Handle("GET "+adminRoot+"/db", admin(handler.database))
	mux.Handle("GET "+adminRoot+"/pages", admin(handler.pages))
	mux.Handle("GET "+adminRoot+"/errors", admin(handler.errors))
	mux.Handle("GET "+adminRoot+"/trace/{correlationId}", admin(handler.trace))
	mux.Handle("GET "+adminRoot+"/runs", admin(handler.runs))
	mux.Handle("GET "+adminRoot+"/runs/compare", admin(handler.compare))
	mux.Handle("GET "+adminRoot+"/runs/{id}", admin(handler.run))
	mux.Handle("GET "+adminRoot+"/run-requests", admin(handler.runRequests))
	mux.Handle("POST "+adminRoot+"/run-requests", admin(handler.requestRun))
	mux.Handle("GET "+adminRoot+"/health", admin(handler.health))
	mux.Handle("GET "+adminRoot+"/security", admin(handler.security))
	mux.Handle("PATCH "+adminRoot+"/findings/{id}", admin(handler.updateFinding))
	mux.Handle("GET "+adminRoot+"/system", admin(handler.system))
	mux.Handle("GET "+adminRoot+"/alerts", admin(handler.alerts))
	mux.Handle("GET "+adminRoot+"/deploys", admin(handler.deploys))
	mux.Handle("POST "+adminRoot+"/alerts/test", admin(handler.testAlert))
	mux.Handle("POST "+adminRoot+"/alerts/{id}/ack", admin(handler.ackAlert))
	mux.Handle("POST "+adminRoot+"/alerts/{id}/silence", admin(handler.silenceAlert))
	mux.Handle("POST "+adminRoot+"/alerts/{id}/unsilence", admin(handler.unsilenceAlert))
	mux.Handle("GET "+adminRoot+"/integrations", admin(handler.integrations))
	registerSettingsRoutes(mux, handler, admin) // 2.1 a 2.3: settings e relatório semanal
	mux.Handle("POST /v1/obs/collect", middleware.RequireAuth(http.HandlerFunc(handler.collect)))
}

type httpHandler struct {
	service *Service
}

func days(r *http.Request) int {
	value, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || value < 1 {
		return 7
	}
	return min(value, 90)
}

func respond(w http.ResponseWriter, r *http.Request, data any, err error) {
	switch {
	case err == nil:
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": data})
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Registro não encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "observability_error", "Não foi possível ler os dados de observabilidade. Confira se as migrations 0329 e 0330 foram aplicadas.")
	}
}

func (h *httpHandler) overview(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Overview(r.Context(), days(r))
	respond(w, r, data, err)
}

func (h *httpHandler) endpoints(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	module := strings.TrimSpace(query.Get("module"))
	if !modulePattern.MatchString(module) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_module", "Módulo inválido.")
		return
	}
	data, err := h.service.Endpoints(r.Context(), days(r), module, query.Get("sort"))
	respond(w, r, data, err)
}

func (h *httpHandler) database(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Database(r.Context(), days(r))
	respond(w, r, data, err)
}

func (h *httpHandler) pages(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Pages(r.Context(), days(r))
	respond(w, r, data, err)
}

func (h *httpHandler) errors(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Errors(r.Context(), days(r))
	respond(w, r, data, err)
}

func (h *httpHandler) trace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("correlationId")
	if !platformobs.ValidCorrelationID(id) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_correlation_id", "Correlation ID inválido.")
		return
	}
	data, err := h.service.Trace(r.Context(), id)
	respond(w, r, data, err)
}

func (h *httpHandler) runs(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = 30
	}
	data, err := h.service.Runs(r.Context(), min(limit, 200))
	respond(w, r, data, err)
}

func (h *httpHandler) run(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !runIDPattern.MatchString(id) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_run_id", "Execução inválida.")
		return
	}
	data, err := h.service.Run(r.Context(), id)
	respond(w, r, data, err)
}

func (h *httpHandler) compare(w http.ResponseWriter, r *http.Request) {
	a, b := r.URL.Query().Get("a"), r.URL.Query().Get("b")
	if !runIDPattern.MatchString(a) || !runIDPattern.MatchString(b) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_run_id", "Informe duas execuções (a e b).")
		return
	}
	data, err := h.service.Compare(r.Context(), a, b)
	respond(w, r, data, err)
}

func (h *httpHandler) runRequests(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.RunRequests(r.Context())
	respond(w, r, data, err)
}

type runRequestInput struct {
	Module string `json:"module"`
	Only   string `json:"only"`
}

func (h *httpHandler) requestRun(w http.ResponseWriter, r *http.Request) {
	var input runRequestInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Corpo inválido.")
		return
	}
	input.Module, input.Only = strings.TrimSpace(input.Module), strings.TrimSpace(input.Only)
	if !modulePattern.MatchString(input.Module) || !onlyPattern.MatchString(input.Only) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Módulo ou filtro inválido.")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	id, err := h.service.RequestRun(r.Context(), principal.UserID, input.Module, input.Only)
	switch {
	case errors.Is(err, ErrRunnerDisabled):
		httpapi.WriteError(w, r, http.StatusForbidden, "runner_disabled_in_production", "O runner não pode ser disparado em produção.")
	case errors.Is(err, ErrQueueBusy):
		httpapi.WriteError(w, r, http.StatusTooManyRequests, "run_queue_busy", "Já existem execuções na fila. Aguarde terminarem.")
	case err != nil:
		respond(w, r, nil, err)
	default:
		httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": id, "status": "queued"}})
	}
}

// collect aceita o lote do front (64 KB). Campos desconhecidos são tolerados para
// não perder telemetria em versões diferentes do front.
func (h *httpHandler) collect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var batch platformobs.FrontendBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Lote inválido.")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	accepted, err := h.service.Collect(batch, principal.UserID, r.UserAgent())
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Lote inválido.")
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"accepted": accepted}})
}
