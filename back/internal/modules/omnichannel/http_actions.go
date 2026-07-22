package omnichannel

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// ============================================================================
// F7 — Rotas das acoes do inbox (spec OMNI-F7). Paths ANINHADOS em
// /conversations/{id}/... e /contacts/{id}/..., todos sob RequireAuthWithAccount.
// ============================================================================
//
// Costurado FORA do module.go (a F7 nao edita module.go — ver AGENT.md §Wiring pendente F7):
// o orquestrador constroi o ActionsService no Build (onde registry/store/send/publisher
// existem) e chama RegisterActionRoutes no RegisterRoutes do handle. account_id vem SEMPRE do
// Principal (RequireAuthWithAccount valida membership), nunca do body/header cru.

// maxForwardMessages e o teto do contrato do legado ({ messageIds: 1..100 }).
const maxForwardMessages = 100

// RegisterActionRoutes monta as 11 rotas de acao. middleware = deps.AuthMiddleware.
func RegisterActionRoutes(mux *http.ServeMux, actions *ActionsService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}

	// Conversas — via maquina de estados (F8).
	mux.Handle("PATCH /v1/omnichannel/conversations/{id}/status", wrap(handleSetStatus(actions)))
	mux.Handle("PATCH /v1/omnichannel/conversations/{id}/assign", wrap(handleAssign(actions)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/take", wrap(handleTake(actions)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/handoff", wrap(handleRequestHandoff(actions)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/release", wrap(handleRelease(actions)))

	// Mensagens.
	mux.Handle("POST /v1/omnichannel/conversations/{id}/messages/{mid}/reaction", wrap(handleReaction(actions)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/messages/forward", wrap(handleForward(actions)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/messages/delete-for-me", wrap(handleDeleteForMe(actions)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/messages/delete-for-all", wrap(handleDeleteForAll(actions)))

	// Contatos.
	mux.Handle("POST /v1/omnichannel/contacts/{id}/open-conversation", wrap(handleOpenConversation(actions)))

	// Sincronizacao com o provedor (group-participants/sync-open/sync-history/import-whatsapp):
	// dependem de metodos do channel.Provider que a F4 ainda nao expoe. Rotas registradas e
	// honestas — 409 acionavel ate a F4 estender a interface (ver AGENT.md §Wiring pendente F7).
	mux.Handle("GET /v1/omnichannel/conversations/{id}/group-participants", wrap(handleProviderSync(actions, "omnichannel.conversations.view")))
	mux.Handle("POST /v1/omnichannel/conversations/sync-open", wrap(handleProviderSync(actions, "omnichannel.conversations.view")))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/messages/sync-history", wrap(handleProviderSync(actions, "omnichannel.conversations.view")))
	mux.Handle("POST /v1/omnichannel/contacts/import-whatsapp", wrap(handleProviderSync(actions, "omnichannel.contacts.manage")))
}

// ============================================================================
// Bodies
// ============================================================================

type statusBody struct {
	Status string `json:"status"`
}

type assignBody struct {
	AssignedToID *string `json:"assignedToId"`
}

type reactionBody struct {
	Emoji *string `json:"emoji"`
}

type forwardBody struct {
	MessageIDs           []string `json:"messageIds"`
	TargetConversationID string   `json:"targetConversationId"`
}

type messageIDsBody struct {
	MessageIDs []string `json:"messageIds"`
}

// ============================================================================
// Handlers — conversas
// ============================================================================

func handleSetStatus(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		var body statusBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := a.SetStatus(r.Context(), accountID, p, r.PathValue("id"), body.Status)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleAssign(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		var body assignBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := a.Assign(r.Context(), accountID, p, r.PathValue("id"), body.AssignedToID)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleTake(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		var body TakeConversationRequest
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := a.TakeConversation(r.Context(), accountID, p, r.PathValue("id"), body.IdempotencyKey)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleRequestHandoff(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		var body HandoffRequest
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := a.RequestHandoff(r.Context(), accountID, p, r.PathValue("id"), body)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleRelease(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		view, err := a.ReleaseConversation(r.Context(), accountID, p, r.PathValue("id"))
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// ============================================================================
// Handlers — mensagens
// ============================================================================

func handleReaction(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		var body reactionBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if err := a.React(r.Context(), accountID, p, r.PathValue("id"), r.PathValue("mid"), body.Emoji); err != nil {
			writeActionError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleForward(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		var body forwardBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if !validMessageIDs(body.MessageIDs) || strings.TrimSpace(body.TargetConversationID) == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body",
				"Informe de 1 a 100 mensagens e a conversa de destino.")
			return
		}
		res, err := a.Forward(r.Context(), accountID, p, r.PathValue("id"), body.MessageIDs, strings.TrimSpace(body.TargetConversationID))
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, res)
	}
}

func handleDeleteForMe(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		body, ok := decodeMessageIDs(w, r)
		if !ok {
			return
		}
		res, err := a.DeleteForMe(r.Context(), accountID, p, r.PathValue("id"), body.MessageIDs)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, res)
	}
}

func handleDeleteForAll(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		body, ok := decodeMessageIDs(w, r)
		if !ok {
			return
		}
		res, err := a.DeleteForAll(r.Context(), accountID, p, r.PathValue("id"), body.MessageIDs)
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, res)
	}
}

// ============================================================================
// Handlers — contatos e sincronizacao
// ============================================================================

func handleOpenConversation(a *ActionsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		view, err := a.OpenContactConversation(r.Context(), accountID, p, r.PathValue("id"))
		if err != nil {
			writeActionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handleProviderSync serve as rotas de sincronizacao com o provedor que a F4 ainda nao
// sustenta. Valida a permissao (403 se faltar) e responde 409 acionavel — o botao nao mente
// que a operacao rodou.
func handleProviderSync(a *ActionsService, permKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, p, ok := actionScope(w, r)
		if !ok {
			return
		}
		if err := a.svc.requirePermission(r.Context(), accountID, p, permKey); err != nil {
			writeActionError(w, r, err)
			return
		}
		writeActionError(w, r, ErrProviderActionUnavailable)
	}
}

// ============================================================================
// Helpers
// ============================================================================

// actionScope resolve accountID + Principal (o Transition/permissao precisam do Principal, nao
// so do Caller). Escreve o 403 no_account quando nao ha conta no contexto.
func actionScope(w http.ResponseWriter, r *http.Request) (string, auth.Principal, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeNoAccount(w, r)
		return "", auth.Principal{}, false
	}
	accountID := strings.TrimSpace(p.AccountID)
	if accountID == "" {
		writeNoAccount(w, r)
		return "", auth.Principal{}, false
	}
	return accountID, p, true
}

// decodeMessageIDs decodifica e valida { messageIds: 1..100 }, escrevendo o 400 quando falha.
func decodeMessageIDs(w http.ResponseWriter, r *http.Request) (messageIDsBody, bool) {
	var body messageIDsBody
	if err := decodeJSONBody(w, r, &body); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
		return messageIDsBody{}, false
	}
	if !validMessageIDs(body.MessageIDs) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Informe de 1 a 100 mensagens.")
		return messageIDsBody{}, false
	}
	return body, true
}

// validMessageIDs aplica o contrato 1..100 (o zod do legado). Ids vazios nao contam.
func validMessageIDs(ids []string) bool {
	if len(ids) == 0 || len(ids) > maxForwardMessages {
		return false
	}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return false
		}
	}
	return true
}

// writeActionError mapeia os erros das acoes para HTTP. Os erros de escopo/permissao caem no
// writeServiceError (404/403); os especificos das acoes viram 409 acionavel (o botao nao pode
// mentir — principio 5).
func writeActionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidTransition):
		httpapi.WriteError(w, r, http.StatusConflict, "invalid_transition",
			"Transicao de status invalida para o estado atual da conversa.")
	case errors.Is(err, ErrActionUnsupported):
		httpapi.WriteError(w, r, http.StatusConflict, "action_unsupported",
			"Este numero/provedor nao suporta esta acao.")
	case errors.Is(err, ErrMessageNotSent):
		httpapi.WriteError(w, r, http.StatusConflict, "message_not_sent",
			"A mensagem ainda nao tem id do provedor (nao enviada).")
	case errors.Is(err, ErrProviderActionUnavailable):
		httpapi.WriteError(w, r, http.StatusConflict, "provider_action_unavailable",
			"Acao indisponivel para este provedor no momento.")
	case errors.Is(err, ErrProviderUnavailable):
		httpapi.WriteError(w, r, http.StatusBadGateway, "provider_unavailable",
			"Nao foi possivel concluir a acao no provedor (WhatsApp). Tente novamente.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Requisicao invalida.")
	default:
		writeServiceError(w, r, err)
	}
}
