package finance

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func retentionResult(w http.ResponseWriter, r *http.Request, data any, err error) {
	switch {
	case errors.Is(err, ErrRetentionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "retention_conflict", "A política de retenção mudou. Recarregue antes de salvar.")
	case errors.Is(err, ErrOriginalPurged):
		httpapi.WriteError(w, r, http.StatusGone, "original_purged", "O arquivo original já foi apagado pela política de retenção. Os dados extraídos continuam salvos.")
	case errors.Is(err, ErrOriginalUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "original_unavailable", "O arquivo original não está disponível no armazenamento.")
	default:
		fiscalResult(w, r, data, err)
	}
}

// Rotas de comprovantes, originais e retenção. Comprovantes e originais são do
// usuário da sessão; a política é da conta e exige finance.config.manage para mudar.
func RegisterRetentionRoutes(mux *http.ServeMux, svc *DocumentService, middleware *auth.Middleware) {
	wrap := func(permission string, h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission(permission, h))
	}
	mux.Handle("GET /v1/finance/receipts", wrap("finance.sheets.view", func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 30
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.ListReceipts(r.Context(), p.AccountID, p.UserID, page, limit)
		retentionResult(w, r, data, err)
	}))
	mux.Handle("GET /v1/finance/documents/{id}/original", wrap("finance.sheets.view", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		o, body, err := svc.OpenOriginal(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		if err != nil {
			retentionResult(w, r, nil, err)
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", o.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(o.SizeBytes, 10))
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": o.FileName}))
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = io.Copy(w, io.LimitReader(body, o.SizeBytes))
	}))
	mux.Handle("DELETE /v1/finance/documents/{id}/original", wrap("finance.sheets.manage", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.RemoveOriginal(r.Context(), p.AccountID, p.UserID, r.PathValue("id"))
		retentionResult(w, r, data, err)
	}))
	mux.Handle("GET /v1/finance/retention", wrap("finance.config.view", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.GetRetention(r.Context(), p.AccountID, p.UserID)
		retentionResult(w, r, data, err)
	}))
	mux.Handle("GET /v1/finance/retention/history", wrap("finance.config.view", func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.RetentionHistory(r.Context(), p.AccountID)
		retentionResult(w, r, data, err)
	}))
	mux.Handle("PUT /v1/finance/retention", wrap("finance.config.manage", func(w http.ResponseWriter, r *http.Request) {
		var in RetentionSettingsInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.UpdateRetention(r.Context(), p.AccountID, p.UserID, in)
		retentionResult(w, r, data, err)
	}))
}
