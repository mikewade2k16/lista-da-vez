package metaads

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxActionHTTPBodyBytes = 64 << 10

func registerActionRoutes(
	mux *http.ServeMux,
	svc *Service,
	wrap func(string, http.HandlerFunc) http.Handler,
) {
	actions := NewActionService(svc, defaultActionExecutor(svc))
	mux.Handle("GET /v1/meta-ads/action-proposals", wrap("meta_ads.view", handleActionProposalList(actions)))
	mux.Handle("POST /v1/meta-ads/action-proposals", wrap("meta_ads.manage", handleActionProposalCreate(actions)))
	mux.Handle("GET /v1/meta-ads/action-proposals/{id}", wrap("meta_ads.view", handleActionProposalGet(actions)))
	mux.Handle("POST /v1/meta-ads/action-proposals/{id}/confirm", wrap("meta_ads.manage", handleActionProposalConfirm(actions)))
	mux.Handle("POST /v1/meta-ads/action-proposals/{id}/cancel", wrap("meta_ads.manage", handleActionProposalCancel(actions)))
	mux.Handle("POST /v1/meta-ads/action-proposals/{id}/reconcile", wrap("meta_ads.manage", handleActionProposalReconcile(actions)))
	mux.Handle("GET /v1/meta-ads/ad-accounts/{id}/action-policy", wrap("meta_ads.view", handleActionPolicyGet(actions)))
	mux.Handle("PUT /v1/meta-ads/ad-accounts/{id}/action-policy", wrap("meta_ads.manage", handleActionPolicyPut(actions)))
}

func handleActionProposalCancel(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := actionPrincipal(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		cancellationKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if !validActionIdempotencyKey(cancellationKey) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Informe uma chave idempotente valida.")
			return
		}
		proposal, err := service.CancelProposal(
			r.Context(), accountID, principal.UserID, r.PathValue("id"), cancellationKey,
		)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, proposal)
	}
}

func handleActionProposalList(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_limit", "Limit deve estar entre 1 e 100.")
				return
			}
			limit = parsed
		}
		proposals, err := service.ListProposals(r.Context(), accountID, limit)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"proposals": proposals})
	}
}

func handleActionProposalCreate(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := actionPrincipal(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if !validActionIdempotencyKey(idempotencyKey) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Informe uma chave idempotente valida.")
			return
		}
		var input ActionProposalInput
		if err := decodeActionHTTPBody(w, r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		proposal, created, err := service.CreateProposalFromUser(
			r.Context(), accountID, principal.UserID, idempotencyKey, input,
		)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		httpapi.WriteJSON(w, status, proposal)
	}
}

func handleActionProposalGet(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		proposal, err := service.GetProposal(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, proposal)
	}
}

func handleActionProposalConfirm(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := actionPrincipal(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		confirmationKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if !validActionIdempotencyKey(confirmationKey) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Informe uma chave idempotente valida.")
			return
		}
		var input ActionConfirmationInput
		if err := decodeOptionalActionHTTPBody(w, r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		proposal, err := service.ConfirmProposal(
			r.Context(), accountID, principal.UserID, r.PathValue("id"), confirmationKey,
			input.AcknowledgeSpend,
		)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, proposal)
	}
}

func handleActionProposalReconcile(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := actionPrincipal(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		proposal, err := service.ReconcileProposal(
			r.Context(), accountID, principal.UserID, r.PathValue("id"),
		)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, proposal)
	}
}

func handleActionPolicyGet(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		policy, err := service.GetPolicy(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, policy)
	}
}

func handleActionPolicyPut(service *ActionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := actionPrincipal(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var input ActionPolicyInput
		if err := decodeActionHTTPBody(w, r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		policy, err := service.PutPolicy(
			r.Context(), accountID, principal.UserID, r.PathValue("id"), input,
		)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, policy)
	}
}

func actionPrincipal(r *http.Request) (string, auth.Principal, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || strings.TrimSpace(principal.AccountID) == "" || strings.TrimSpace(principal.UserID) == "" {
		return "", auth.Principal{}, false
	}
	return strings.TrimSpace(principal.AccountID), principal, true
}

func decodeActionHTTPBody(w http.ResponseWriter, r *http.Request, destination any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxActionHTTPBodyBytes))
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		return ErrActionValidation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrActionValidation
	}
	return nil
}

func decodeOptionalActionHTTPBody(w http.ResponseWriter, r *http.Request, destination any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxActionHTTPBodyBytes))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrActionValidation
	}
	return nil
}

func writeActionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrActionValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_action_proposal", "Proposta Meta Ads invalida.")
	case errors.Is(err, ErrActionPolicyRequired):
		httpapi.WriteError(w, r, http.StatusConflict, "action_policy_required", "Configure os limites financeiros desta conta de anuncio antes de confirmar a acao.")
	case errors.Is(err, ErrActionPolicyDenied):
		httpapi.WriteError(w, r, http.StatusConflict, "action_not_allowed", "A politica financeira bloqueia esta acao.")
	case errors.Is(err, ErrActionBudgetCapExceeded):
		httpapi.WriteError(w, r, http.StatusConflict, "budget_cap_exceeded", "O orcamento excede o teto configurado para esta conta de anuncio.")
	case errors.Is(err, ErrActionBudgetUnavailable):
		httpapi.WriteError(w, r, http.StatusConflict, "budget_state_unavailable", "O orcamento atual nao esta disponivel para validar a ativacao com seguranca.")
	case errors.Is(err, ErrActionReinforcedConfirm):
		httpapi.WriteError(w, r, http.StatusConflict, "reinforced_confirmation_required", "Confirme explicitamente que deseja ativar a campanha ou alterar seu orcamento.")
	case errors.Is(err, ErrActionSourceUnbound):
		httpapi.WriteError(w, r, http.StatusConflict, "assistant_source_unbound", "O card do assistente nao existe mais ou ainda nao foi vinculado.")
	case errors.Is(err, ErrActionProposalStale):
		httpapi.WriteError(w, r, http.StatusConflict, "action_proposal_stale", "A conexao, o vinculo, a politica ou a campanha mudou. Prepare uma nova proposta.")
	case errors.Is(err, ErrActionCurrencyUnsupported):
		httpapi.WriteError(w, r, http.StatusConflict, "unsupported_budget_currency", "Alteracoes de orcamento estao disponiveis apenas para contas em BRL nesta fase.")
	case errors.Is(err, ErrActionNotCancellable):
		httpapi.WriteError(w, r, http.StatusConflict, "action_not_cancellable", "A acao ja foi iniciada ou concluida e nao pode mais ser cancelada.")
	case errors.Is(err, ErrActionExpired):
		httpapi.WriteError(w, r, http.StatusConflict, "action_expired", "A proposta expirou. Prepare uma nova acao pelo assistente.")
	case errors.Is(err, ErrActionIdempotencyConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "idempotency_conflict", "A chave idempotente ja foi usada com outro pedido.")
	case errors.Is(err, ErrMetaWritesDisabled):
		httpapi.WriteError(w, r, http.StatusConflict, "meta_writes_disabled", "As escritas Meta Ads estao desabilitadas no servidor.")
	case errors.Is(err, ErrMetaActionUnavailable):
		httpapi.WriteError(w, r, http.StatusConflict, "action_unavailable", "Esta acao ainda nao possui executor Graph seguro.")
	case errors.Is(err, pgx.ErrNoRows):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrNotConnected):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_connected", "Conecte uma conta Meta primeiro.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a proposta Meta Ads.")
	}
}
