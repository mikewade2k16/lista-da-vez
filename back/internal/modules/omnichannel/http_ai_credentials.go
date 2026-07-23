package omnichannel

import (
	"net/http"
	"strings"
)

func registerAICredentialRoutes(mux *http.ServeMux, svc *AIService, wrap func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /v1/omnichannel/settings/ai-credentials", wrap(handleListAICredentials(svc)))
	mux.Handle("POST /v1/omnichannel/settings/ai-credentials", wrap(handleCreateAICredential(svc)))
	mux.Handle("PATCH /v1/omnichannel/settings/ai-credentials/{credentialId}", wrap(handleUpdateAICredential(svc)))
	mux.Handle("DELETE /v1/omnichannel/settings/ai-credentials/{credentialId}", wrap(handleDeleteAICredential(svc)))
	mux.Handle("POST /v1/omnichannel/settings/ai-credentials/import-legacy", wrap(handleImportLegacyAICredentials(svc)))
	mux.Handle("GET /v1/omnichannel/settings/ai-credentials/{credentialId}/models", wrap(handleListAICredentialModels(svc)))
}

func handleListAICredentials(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAICredentials(r.Context(), p.AccountID, p)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateAICredential(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AICredentialInput
		if decodeJSONBody(w, r, &in) != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateAICredential(r.Context(), p.AccountID, p, in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateAICredential(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AICredentialPatch
		if decodeJSONBody(w, r, &in) != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateAICredential(r.Context(), p.AccountID, p, r.PathValue("credentialId"), in)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteAICredential(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		err := svc.DeleteAICredential(r.Context(), p.AccountID, p, r.PathValue("credentialId"))
		writeAIResult(w, r, http.StatusNoContent, nil, err)
	}
}

func handleImportLegacyAICredentials(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ImportLegacyAICredentials(r.Context(), p.AccountID, p)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListAICredentialModels(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListCredentialModels(r.Context(), p.AccountID, p,
			r.PathValue("credentialId"), strings.TrimSpace(r.URL.Query().Get("capability")))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}
