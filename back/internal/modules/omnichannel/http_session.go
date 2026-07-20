package omnichannel

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Rotas do ciclo de sessao/QR do painel (/v1/omnichannel/tenant/whatsapp/*). Sob
// RequireAuthWithAccount (valida membership) + gate de modulo (estao sob /v1/omnichannel).
// account_id vem do Principal, nunca do body.
//
// Os paths seguem o que o FRONT verbatim chama (tenant/whatsapp/*) — regra D-B: o Go se
// adapta ao front, nao o contrario. Ver docs/omnichannel/ESTADO.md (mapa do remap).

func registerSessionRoutes(mux *http.ServeMux, svc *SessionService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/bootstrap", wrap(handleSessionBootstrap(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/connect", wrap(handleSessionConnect(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/status", wrap(handleSessionStatus(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/qrcode", wrap(handleSessionQRCode(svc)))
	mux.Handle("POST /v1/omnichannel/tenant/whatsapp/logout", wrap(handleSessionLogout(svc)))
	mux.Handle("PUT /v1/omnichannel/tenant/whatsapp/instances/{instanceName}/credentials", wrap(handleSetCredentials(svc)))
}

func handleSessionBootstrap(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in SessionBootstrapInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Bootstrap(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleSessionConnect(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in SessionInstanceInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Connect(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleSessionStatus(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.Status(r.Context(), accountID, caller,
			r.URL.Query().Get("instanceId"), r.URL.Query().Get("instanceName"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleSessionQRCode(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.QRCode(r.Context(), accountID, caller,
			r.URL.Query().Get("instanceId"), r.URL.Query().Get("instanceName"))
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleSessionLogout(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in SessionInstanceInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Logout(r.Context(), accountID, caller, in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleSetCredentials(svc *SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in CredentialInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		status, err := svc.SetCredentials(r.Context(), accountID, caller, r.PathValue("instanceName"), in)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// writeSessionError mapeia os erros de sessao. NumberInUseError vira 409 acionavel nomeando
// a instancia; o resto delega ao writeServiceError comum.
func writeSessionError(w http.ResponseWriter, r *http.Request, err error) {
	var numErr *NumberInUseError
	switch {
	case errors.As(err, &numErr):
		httpapi.WriteError(w, r, http.StatusConflict, "number_in_use",
			"Este numero ja esta em uso pela instancia \""+strings.TrimSpace(numErr.InstanceName)+
				"\" nesta conta. Use outro numero ou desconecte a instancia existente.")
	case errors.Is(err, ErrInstanceNameConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "instance_name_conflict",
			"Ja existe uma instancia com esse nome nesta conta. Escolha outro nome.")
	case errors.Is(err, ErrChannelLimit):
		httpapi.WriteError(w, r, http.StatusConflict, "channel_limit",
			"Limite de numeros de WhatsApp da conta atingido. Remova um numero ou fale com o suporte.")
	case errors.Is(err, ErrInstanceHasConversations):
		httpapi.WriteError(w, r, http.StatusConflict, "instance_has_conversations",
			"Esta instancia tem conversas atreladas. Desative-a (em vez de excluir) ou limpe as conversas antes.")
	case errors.Is(err, ErrProviderUnsupported):
		httpapi.WriteError(w, r, http.StatusBadRequest, "provider_unsupported",
			"Provedor de WhatsApp invalido ou sem suporte a este fluxo.")
	case errors.Is(err, ErrSessionUnavailable):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Instancia nao encontrada.")
	default:
		writeServiceError(w, r, err)
	}
}
