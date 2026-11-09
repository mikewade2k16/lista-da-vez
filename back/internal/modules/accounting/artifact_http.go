package accounting

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
	"strconv"
)

func RegisterArtifactRoute(mux *http.ServeMux, s *Service, m *auth.Middleware) {
	mux.Handle("GET /v1/accounting/issuers/{issuer}/executions/{id}/artifacts/{artifact}", m.RequireAuthWithAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		artifact, e := s.DownloadArtifact(r.Context(), scope(r), r.PathValue("id"), r.PathValue("artifact"))
		if e != nil {
			result(w, r, 0, nil, e)
			return
		}
		mime, name := "application/octet-stream", "artifact.bin"
		switch artifact.Kind {
		case "xml":
			mime, name = "application/xml", "document.xml"
		case "pdf":
			mime, name = "application/pdf", "document.pdf"
		case "protocol":
			mime, name = "application/json", "technical-receipt.json"
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Content)))
		_, _ = w.Write(artifact.Content)
	})))
}
