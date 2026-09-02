package omnichannel

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterAutomationRoutes(mux *http.ServeMux, svc *AutomationService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("GET /v1/omnichannel/automation/profiles", wrap(handleListAutomationProfiles(svc)))
	mux.Handle("GET /v1/omnichannel/automation/profiles/{clientId}", wrap(handleGetAutomationProfile(svc)))
	mux.Handle("PUT /v1/omnichannel/automation/profiles/{clientId}", wrap(handlePutAutomationProfile(svc)))
	mux.Handle("GET /v1/omnichannel/automation/interventions", wrap(handleListAutomationInterventions(svc)))
	mux.Handle("GET /v1/omnichannel/automation/attendances", wrap(handleListAutomationAttendances(svc)))
	mux.Handle("POST /v1/omnichannel/automation/conversations/{conversationId}/pause-ai", wrap(handlePauseAutomationAI(svc)))
	mux.Handle("POST /v1/omnichannel/automation/conversations/{conversationId}/reply-with-ai", wrap(handleReplyAutomationWithAI(svc)))
}

func handleListAutomationAttendances(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAttendances(r.Context(), p.AccountID, p,
			r.URL.Query().Get("clientId"), parseLimit(r.URL.Query().Get("limit")))
		writeAutomationResult(w, r, http.StatusOK, out, err)
	}
}

func handlePauseAutomationAI(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AutomationActionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.PauseAI(r.Context(), p.AccountID, p, r.PathValue("conversationId"), in)
		writeAutomationResult(w, r, http.StatusOK, out, err)
	}
}

func handleReplyAutomationWithAI(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AutomationActionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.ReplyWithAI(r.Context(), p.AccountID, p, r.PathValue("conversationId"), in)
		writeAutomationResult(w, r, http.StatusAccepted, out, err)
	}
}

func handleListAutomationInterventions(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListInterventions(r.Context(), p.AccountID, p,
			r.URL.Query().Get("clientId"), parseLimit(r.URL.Query().Get("limit")))
		writeAutomationResult(w, r, http.StatusOK, out, err)
	}
}

func handleListAutomationProfiles(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListProfiles(r.Context(), p.AccountID, p)
		writeAutomationResult(w, r, http.StatusOK, out, err)
	}
}

func handleGetAutomationProfile(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.GetProfile(r.Context(), p.AccountID, p, r.PathValue("clientId"))
		writeAutomationResult(w, r, http.StatusOK, out, err)
	}
}

func handlePutAutomationProfile(svc *AutomationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AutomationProfileInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.PutProfile(r.Context(), p.AccountID, p, r.PathValue("clientId"), in)
		writeAutomationResult(w, r, http.StatusOK, out, err)
	}
}

func writeAutomationResult(w http.ResponseWriter, r *http.Request, status int, payload any, err error) {
	if err != nil {
		if errors.Is(err, ErrAutomationNoUnansweredInput) {
			httpapi.WriteError(w, r, http.StatusConflict, "no_unanswered_message",
				"Nao existe mensagem pendente para a IA responder.")
			return
		}
		if errors.Is(err, ErrAutomationNotReady) {
			httpapi.WriteError(w, r, http.StatusConflict, "automation_not_ready",
				"Ative o numero e publique um agente com provedor, modelo e chave antes de ligar a automacao.")
			return
		}
		if errors.Is(err, ErrAutomationBindingMismatch) {
			httpapi.WriteError(w, r, http.StatusConflict, "automation_binding_mismatch",
				"O numero precisa ter um vinculo ativo com o mesmo cliente do perfil de automacao.")
			return
		}
		writeDomainError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, payload)
}
