package knowledge

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func publicCurationResult(w http.ResponseWriter, r *http.Request, data any, err error) {
	var domain *Error
	switch {
	case err == nil:
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "data": data})
	case errors.As(err, &domain):
		httpapi.WriteError(w, r, domain.Status, domain.Code, publicCurationMessage(domain.Code))
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "public_curation_error", "Não foi possível concluir a operação na base pública.")
	}
}

func publicCurationMessage(code string) string {
	switch code {
	case "owner_approval_required":
		return "Registre a sua aprovação (dono) antes de publicar."
	case "collection_empty":
		return "Todos os trechos foram excluídos; não há o que publicar."
	case "collection_not_publishable", "collection_not_discardable":
		return "Esta coleta já foi publicada, substituída ou descartada."
	case "invalid_review":
		return "Informe quem revisou, o tipo (dono, IA ou contador) e o resultado."
	case "not_found":
		return "Registro não encontrado."
	}
	return "Pedido inválido para a base pública."
}

func publicCurationDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || !errors.Is(d.Decode(new(any)), io.EOF) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Corpo inválido.")
		return false
	}
	return true
}

// RegisterPublicCurationRoutes expõe a curadoria do acervo público somente a
// platform_admin: o acervo é global e alimenta o chat de todas as contas.
func RegisterPublicCurationRoutes(mux *http.ServeMux, s *Service, middleware *auth.Middleware) {
	admin := func(h http.HandlerFunc) http.Handler { return middleware.RequireRoles(h, auth.RolePlatformAdmin) }
	actor := func(r *http.Request) string {
		p, _ := auth.PrincipalFromContext(r.Context())
		return p.UserID
	}
	const prefix = "/v1/knowledge/public"
	mux.Handle("GET "+prefix+"/catalog", admin(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.PublicCatalogStatus(r.Context())
		publicCurationResult(w, r, data, err)
	}))
	mux.Handle("POST "+prefix+"/catalog/{key}/collect", admin(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.CollectPublic(r.Context(), actor(r), r.PathValue("key"))
		publicCurationResult(w, r, data, err)
	}))
	mux.Handle("GET "+prefix+"/collections/{id}/chunks", admin(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		data, err := s.PublicChunks(r.Context(), r.PathValue("id"), page, 50)
		publicCurationResult(w, r, data, err)
	}))
	mux.Handle("DELETE "+prefix+"/collections/{id}/chunks/{chunk}", admin(func(w http.ResponseWriter, r *http.Request) {
		err := s.ExcludePublicChunk(r.Context(), r.PathValue("id"), r.PathValue("chunk"))
		publicCurationResult(w, r, map[string]bool{"excluded": err == nil}, err)
	}))
	mux.Handle("GET "+prefix+"/collections/{id}/reviews", admin(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.PublicReviews(r.Context(), r.PathValue("id"))
		publicCurationResult(w, r, data, err)
	}))
	mux.Handle("POST "+prefix+"/collections/{id}/reviews", admin(func(w http.ResponseWriter, r *http.Request) {
		var in PublicReviewInput
		if !publicCurationDecode(w, r, &in) {
			return
		}
		data, err := s.ReviewPublicCollection(r.Context(), actor(r), r.PathValue("id"), in)
		publicCurationResult(w, r, data, err)
	}))
	mux.Handle("POST "+prefix+"/collections/{id}/publish", admin(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.PublishPublicCollection(r.Context(), actor(r), r.PathValue("id"))
		publicCurationResult(w, r, data, err)
	}))
	mux.Handle("POST "+prefix+"/collections/{id}/discard", admin(func(w http.ResponseWriter, r *http.Request) {
		err := s.DiscardPublicCollection(r.Context(), r.PathValue("id"))
		publicCurationResult(w, r, map[string]bool{"discarded": err == nil}, err)
	}))
	mux.Handle("GET "+prefix+"/collections/{id}/export", admin(func(w http.ResponseWriter, r *http.Request) {
		text, err := s.ExportPublicCollection(r.Context(), r.PathValue("id"))
		if err != nil {
			publicCurationResult(w, r, nil, err)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="coleta-`+r.PathValue("id")+`.md"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, text)
	}))
}
