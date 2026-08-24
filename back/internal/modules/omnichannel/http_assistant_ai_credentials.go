package omnichannel

import (
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// registerAssistantAICredentialRoutes expoe o cofre global pela superficie
// neutra do Assistente 360. O path fica fora do gate do modulo Omnichannel, mas
// todas as rotas exigem membership da account e a politica RBAC transversal.
func registerAssistantAICredentialRoutes(mux *http.ServeMux, svc *AIService, middleware *auth.Middleware) {
	wrap := func(handler http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(handler)
	}
	mux.Handle("GET /v1/assistant/ai-credentials", wrap(handleListAssistantAICredentials(svc)))
	mux.Handle("POST /v1/assistant/ai-credentials", wrap(handleCreateAssistantAICredential(svc)))
	mux.Handle("PATCH /v1/assistant/ai-credentials/{credentialId}", wrap(handleUpdateAssistantAICredential(svc)))
	mux.Handle("DELETE /v1/assistant/ai-credentials/{credentialId}", wrap(handleDeleteAssistantAICredential(svc)))
	mux.Handle("POST /v1/assistant/ai-credentials/import-legacy", wrap(handleImportAssistantLegacyAICredentials(svc)))
	mux.Handle("GET /v1/assistant/ai-credentials/{credentialId}/models", wrap(handleListAssistantCredentialModels(svc)))
}

func handleListAssistantAICredentials(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAssistantAICredentials(r.Context(), principal.AccountID, principal)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateAssistantAICredential(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := domainScope(w, r)
		if !ok {
			return
		}
		var input AICredentialInput
		if decodeJSONBody(w, r, &input) != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateAssistantAICredential(r.Context(), principal.AccountID, principal, input)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateAssistantAICredential(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := domainScope(w, r)
		if !ok {
			return
		}
		var input AICredentialPatch
		if decodeJSONBody(w, r, &input) != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateAssistantAICredential(
			r.Context(), principal.AccountID, principal,
			r.PathValue("credentialId"), input,
		)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteAssistantAICredential(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.DeleteAssistantAICredential(
			r.Context(), principal.AccountID, principal, r.PathValue("credentialId"),
		)
		writeAIResult(w, r, http.StatusNoContent, nil, err)
	}
}

func handleImportAssistantLegacyAICredentials(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ImportAssistantLegacyAICredentials(r.Context(), principal.AccountID, principal)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListAssistantCredentialModels(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAssistantCredentialModels(
			r.Context(), principal.AccountID, principal,
			r.PathValue("credentialId"), strings.TrimSpace(r.URL.Query().Get("capability")),
		)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}
