package omnichannel

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func RegisterKnowledgeRoutes(mux *http.ServeMux, svc *AIService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }

	mux.Handle("GET /v1/omnichannel/knowledge-bases", wrap(handleListKnowledgeBases(svc)))
	mux.Handle("POST /v1/omnichannel/knowledge-bases", wrap(handleCreateKnowledgeBase(svc)))
	mux.Handle("PATCH /v1/omnichannel/knowledge-bases/{id}", wrap(handleUpdateKnowledgeBase(svc)))
	mux.Handle("GET /v1/omnichannel/knowledge-bases/{baseId}/documents", wrap(handleListKnowledgeDocuments(svc)))
	mux.Handle("POST /v1/omnichannel/knowledge-bases/{baseId}/documents", wrap(handleCreateKnowledgeDocument(svc)))
	mux.Handle("PATCH /v1/omnichannel/knowledge-bases/{baseId}/documents/{docId}", wrap(handleUpdateKnowledgeDocument(svc)))
	mux.Handle("POST /v1/omnichannel/knowledge-bases/{baseId}/documents/{docId}/chunks", wrap(handleReplaceKnowledgeChunks(svc)))

	mux.Handle("GET /v1/omnichannel/agents/{id}/knowledge-bindings", wrap(handleListAIKnowledgeBindings(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/knowledge-bindings", wrap(handleCreateAIKnowledgeBinding(svc)))
	mux.Handle("PATCH /v1/omnichannel/agents/{id}/knowledge-bindings/{bindingId}", wrap(handleUpdateAIKnowledgeBinding(svc)))
	mux.Handle("DELETE /v1/omnichannel/agents/{id}/knowledge-bindings/{bindingId}", wrap(handleDeleteAIKnowledgeBinding(svc)))
	mux.Handle("POST /v1/omnichannel/agents/{id}/knowledge-search", wrap(handleSearchKnowledge(svc)))
}

func handleListKnowledgeBases(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListKnowledgeBases(r.Context(), p.AccountID, p)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateKnowledgeBase(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in KnowledgeBaseInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateKnowledgeBase(r.Context(), p.AccountID, p, in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateKnowledgeBase(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch KnowledgeBasePatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateKnowledgeBase(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleListKnowledgeDocuments(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListKnowledgeDocuments(r.Context(), p.AccountID, p, r.PathValue("baseId"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateKnowledgeDocument(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in KnowledgeDocumentInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateKnowledgeDocument(r.Context(), p.AccountID, p, r.PathValue("baseId"), in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateKnowledgeDocument(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch KnowledgeDocumentPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateKnowledgeDocument(r.Context(), p.AccountID, p, r.PathValue("baseId"), r.PathValue("docId"), patch)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleReplaceKnowledgeChunks(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in KnowledgeChunksInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		if err := svc.ReplaceKnowledgeChunks(r.Context(), p.AccountID, p, r.PathValue("baseId"), r.PathValue("docId"), in); err != nil {
			writeAIError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListAIKnowledgeBindings(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAIKnowledgeBindings(r.Context(), p.AccountID, p, r.PathValue("id"))
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateAIKnowledgeBinding(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in AIKnowledgeBindingInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateAIKnowledgeBinding(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateAIKnowledgeBinding(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch AIKnowledgeBindingPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateAIKnowledgeBinding(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("bindingId"), patch)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}

func handleDeleteAIKnowledgeBinding(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		if err := svc.DeleteAIKnowledgeBinding(r.Context(), p.AccountID, p, r.PathValue("id"), r.PathValue("bindingId")); err != nil {
			writeAIError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSearchKnowledge(svc *AIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in KnowledgeSearchInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.SearchKnowledge(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeAIResult(w, r, http.StatusOK, out, err)
	}
}
