package omnichannel

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// ============================================================================
// F8 — Superficie HTTP do dominio de atendimento (Contrato 6 da spec OMNI-F8.md)
// ============================================================================
//
// RegisterDomainRoutes NAO e chamada aqui nem em module.go/http.go: o orquestrador costura
// as rotas da F4 e da F8 juntas (needsWiring). Todas sob /v1/omnichannel, com
// RequireAuthWithAccount (injeta AccountID validado no Principal a partir do X-Account-Id).
// A permissao por feature e checada no service (403); escopo/gate de dado => 404.
//
// A F10 CONSOME estas rotas (nao recria). Os paths /settings/* sao os definitivos.
func RegisterDomainRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}

	mux.Handle("GET /v1/omnichannel/settings/departments", wrap(handleListDepartments(svc)))
	mux.Handle("POST /v1/omnichannel/settings/departments", wrap(handleCreateDepartment(svc)))
	mux.Handle("PATCH /v1/omnichannel/settings/departments/{id}", wrap(handleUpdateDepartment(svc)))
	mux.Handle("DELETE /v1/omnichannel/settings/departments/{id}", wrap(handleDeleteDepartment(svc)))

	mux.Handle("GET /v1/omnichannel/settings/queues", wrap(handleListQueues(svc)))
	mux.Handle("POST /v1/omnichannel/settings/queues", wrap(handleCreateQueue(svc)))
	mux.Handle("PATCH /v1/omnichannel/settings/queues/{id}", wrap(handleUpdateQueue(svc)))
	mux.Handle("DELETE /v1/omnichannel/settings/queues/{id}", wrap(handleDeleteQueue(svc)))

	mux.Handle("GET /v1/omnichannel/settings/queues/{id}/members", wrap(handleListQueueMembers(svc)))
	mux.Handle("POST /v1/omnichannel/settings/queues/{id}/members", wrap(handleAddQueueMember(svc)))
	mux.Handle("DELETE /v1/omnichannel/settings/queues/{id}/members/{userId}", wrap(handleRemoveQueueMember(svc)))

	mux.Handle("GET /v1/omnichannel/settings/routing-rules", wrap(handleListRoutingRules(svc)))
	mux.Handle("POST /v1/omnichannel/settings/routing-rules", wrap(handleCreateRoutingRule(svc)))
	mux.Handle("PUT /v1/omnichannel/settings/routing-rules/order", wrap(handleReorderRoutingRules(svc)))
	mux.Handle("PATCH /v1/omnichannel/settings/routing-rules/{id}", wrap(handleUpdateRoutingRule(svc)))
	mux.Handle("DELETE /v1/omnichannel/settings/routing-rules/{id}", wrap(handleDeleteRoutingRule(svc)))

	mux.Handle("PATCH /v1/omnichannel/conversations/{id}/queue", wrap(handleTransferQueue(svc)))
	mux.Handle("GET /v1/omnichannel/conversations/{id}/routing-decisions", wrap(handleListRoutingDecisions(svc)))
}

// domainScope resolve o Principal com AccountID ja validado pelo middleware. Sem conta =>
// 403 no_account (padrao do modulo).
func domainScope(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || strings.TrimSpace(principal.AccountID) == "" {
		writeNoAccount(w, r)
		return auth.Principal{}, false
	}
	return principal, true
}

// ============================================================================
// Setores
// ============================================================================

func handleListDepartments(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListDepartments(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateDepartment(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in DepartmentInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateDepartment(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateDepartment(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch DepartmentPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateDepartment(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteDepartment(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.DeleteDepartment(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeDomainNoContent(w, r, err)
	}
}

// ============================================================================
// Filas
// ============================================================================

func handleListQueues(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListQueues(r.Context(), p.AccountID, p, r.URL.Query().Get("departmentId"))
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateQueue(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in QueueInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateQueue(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateQueue(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch QueuePatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateQueue(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteQueue(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.DeleteQueue(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeDomainNoContent(w, r, err)
	}
}

// ============================================================================
// Membros da fila
// ============================================================================

func handleListQueueMembers(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListQueueMembers(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleAddQueueMember(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in QueueMemberInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.AddQueueMember(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleRemoveQueueMember(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.RemoveQueueMember(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("userId"))
		writeDomainNoContent(w, r, err)
	}
}

// ============================================================================
// Regras de roteamento
// ============================================================================

func handleListRoutingRules(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListRoutingRules(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateRoutingRule(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in RoutingRuleInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateRoutingRule(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateRoutingRule(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch RoutingRulePatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateRoutingRule(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteRoutingRule(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.DeleteRoutingRule(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeDomainNoContent(w, r, err)
	}
}

func handleReorderRoutingRules(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in RoutingRuleOrder
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		err := svc.ReorderRoutingRules(r.Context(), p.AccountID, p, in.RuleIDs)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		out, err := svc.ListRoutingRules(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

// ============================================================================
// Conversas — transferencia de fila e auditoria de roteamento
// ============================================================================

func handleTransferQueue(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var body struct {
			QueueID string `json:"queueId"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.TransferQueue(r.Context(), p, r.PathValue("id"), body.QueueID)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListRoutingDecisions(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListRoutingDecisions(r.Context(), p, r.PathValue("id"))
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

// ============================================================================
// Helpers de resposta
// ============================================================================

func writeDomainResult(w http.ResponseWriter, r *http.Request, status int, payload any, err error) {
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, payload)
}

func writeDomainNoContent(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeInvalidBody(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
}

// writeDomainError mapeia os erros do dominio da F8 para HTTP. Escopo => 404 (nunca 403,
// enumeration); feature sem permissao => 403; transicao invalida => 409 com mensagem
// acionavel (princípio 5).
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidTransition):
		httpapi.WriteError(w, r, http.StatusConflict, "invalid_transition",
			"Transicao de estado invalida para esta conversa. Reabra ou ajuste antes de tentar de novo.")
	case errors.Is(err, ErrConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "conflict",
			"Ja existe um registro com este identificador nesta conta.")
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidBody):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation",
			"Dados invalidos: confira os campos obrigatorios.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden",
			"Voce nao tem permissao para esta operacao nesta conta.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Falha ao processar a requisicao.")
	}
}
