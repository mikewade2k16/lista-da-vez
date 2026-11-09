package finance

import (
	"errors"
	"io"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterDocumentRoutes(mux *http.ServeMux, svc *DocumentService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", h))
	}
	mux.Handle("GET /v1/finance/documents", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.List(r.Context(), p.AccountID, p.UserID)
		fiscalResult(w, r, out, err)
	}))
	mux.Handle("POST /v1/finance/documents/extract", wrap(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxDocumentBytes+65536)
		reader, err := r.MultipartReader()
		if err != nil {
			httpapi.WriteError(w, r, 400, "invalid_image", "Envie o documento como arquivo multipart.")
			return
		}
		part, err := reader.NextPart()
		if err != nil || (part.FormName() != "file" && part.FormName() != "image") {
			httpapi.WriteError(w, r, 400, "invalid_image", "Selecione PDF, JPEG, PNG, TXT, CSV ou OFX.")
			return
		}
		defer part.Close()
		raw, err := io.ReadAll(io.LimitReader(part, maxDocumentBytes+1))
		if err != nil || len(raw) > maxDocumentBytes {
			httpapi.WriteError(w, r, 413, "image_too_large", "O documento deve ter até 8 MB.")
			return
		}
		if _, err = reader.NextPart(); !errors.Is(err, io.EOF) {
			httpapi.WriteError(w, r, 400, "invalid_image", "Envie somente um documento por vez.")
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Extract(r.Context(), p.AccountID, p.UserID, part.FileName(), raw)
		if errors.Is(err, ErrDocumentOCR) {
			httpapi.WriteError(w, r, 503, "ocr_unavailable", "Leitura indisponível ou ocupada. Tente novamente; em ambiente nativo instale Poppler e Tesseract com idioma português.")
			return
		}
		fiscalResult(w, r, out, err)
	}))
	mux.Handle("POST /v1/finance/documents/{id}/parties", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
		}
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.ReidentifyParties(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in.ExpectedUpdatedAt)
		fiscalResult(w, r, out, err)
	}))
	mux.Handle("PUT /v1/finance/documents/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in DocumentDraftInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.Save(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in)
		fiscalResult(w, r, out, err)
	}))
}
