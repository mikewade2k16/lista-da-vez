package omnichannel

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Rotas de GESTAO de instancia do painel (/v1/omnichannel/tenant/whatsapp/instances*,
// validate-endpoints, conversations/clear). Sob RequireAuthWithAccount (valida membership)
// + gate de modulo (estao sob /v1/omnichannel). account_id vem do Principal, nunca do body.
//
// A LEITURA (GET .../instances[/access]) fica no read Service (http.go). A ESCRITA vive aqui,
// no SessionService — ele ja tem o secretBox (cifra a evolutionApiKey), o limits (teto de
// canais) e o registry (provider). Os paths seguem o que o front verbatim chama (D-B).
//
// Sem conflito de rota com http.go/http_session.go: os GET .../instances e o
// PUT .../instances/{instanceName}/credentials tem metodo/segmento final distintos.

func registerInstanceRoutes(mux *http.ServeMux, svc *SessionService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/instances", wrap(handleCreateInstance(svc)))
	mux.Handle("PATCH /v1/omnichannel/tenant/whatsapp/instances/{id}", wrap(handleUpdateInstance(svc)))
	mux.Handle("DELETE /v1/omnichannel/tenant/whatsapp/instances/{id}", wrap(handleDeleteInstance(svc)))
	mux.Handle("PUT /v1/omnichannel/tenant/whatsapp/limits", wrap(handleUpdateChannelLimit(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/instances/{id}/capabilities", wrap(handleInstanceCapabilities(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/instances/{id}/users", wrap(handleGetInstanceAccess(svc)))
	mux.Handle("PUT /v1/omnichannel/tenant/whatsapp/instances/{id}/users", wrap(handlePutInstanceAccess(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/validate-endpoints", wrap(handleValidateEndpoints(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/instances/{id}/history/reset", wrap(handleResetInstanceHistory(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/conversations/clear", wrap(handleClearConversations(svc)))
}

func handleUpdateChannelLimit(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in ChannelLimitInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateChannelLimit(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleCreateInstance(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in InstanceWriteInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateInstance(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleUpdateInstance(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in InstanceWriteInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateInstance(r.Context(), accountID, caller, r.PathValue("id"), in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteInstance(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		if err := svc.DeleteInstance(r.Context(), accountID, caller, r.PathValue("id")); err != nil {
			writeSessionError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleInstanceCapabilities(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		caps, err := svc.InstanceCapabilities(r.Context(), accountID, caller, r.PathValue("id"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, caps)
	}
}

func handleGetInstanceAccess(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.GetInstanceAccess(r.Context(), accountID, r.PathValue("id"), caller)
		if err != nil {
			writeInstanceAccessError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePutInstanceAccess(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in InstanceAccessRequest
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.PutInstanceAccess(r.Context(), accountID, r.PathValue("id"), caller, in)
		if err != nil {
			writeInstanceAccessError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func writeInstanceAccessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInstanceAccessRevisionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "instance_access_revision_conflict",
			"Os acessos desta conexao foram alterados. Recarregue antes de salvar novamente.")
	case errors.Is(err, ErrLastInstanceManager):
		httpapi.WriteError(w, r, http.StatusConflict, "last_instance_manager",
			"Transfira a gestao antes de remover o ultimo usuario com acesso manage.")
	default:
		writeSessionError(w, r, err)
	}
}

func handleValidateEndpoints(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in EndpointValidationInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.ValidateEndpoints(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleClearConversations(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Compatibilidade deliberadamente inerte: nenhum body, inclusive o antigo tenant-wide,
		// consegue alcançar persistencia destrutiva.
		writeHistoryResetError(w, r, ErrHistoryResetMoved)
	}
}

func handleResetInstanceHistory(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in InstanceHistoryResetInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.ResetInstanceHistory(r.Context(), accountID, r.PathValue("id"), caller, in)
		if err != nil {
			writeHistoryResetError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func writeHistoryResetError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrHistoryResetMoved):
		httpapi.WriteError(w, r, http.StatusConflict, "history_reset_moved",
			"Use POST /v1/omnichannel/tenant/whatsapp/instances/{id}/history/reset com confirmacao e revisao.")
	case errors.Is(err, ErrHistoryResetRevisionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "history_reset_revision_conflict",
			"A instancia foi alterada. Atualize os dados antes de tentar novamente.")
	case errors.Is(err, ErrHistoryResetConfirmationMismatch):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "history_reset_confirmation_mismatch",
			"A confirmacao nao corresponde ao instanceName atual.")
	default:
		writeSessionError(w, r, err)
	}
}
