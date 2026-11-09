package finance

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func decodeFiscal(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		httpapi.WriteError(w, r, 400, "invalid_input", "Dados fiscais inválidos ou muito extensos.")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		httpapi.WriteError(w, r, 400, "invalid_input", "Envie somente um objeto JSON.")
		return false
	}
	return true
}
func fiscalResult(w http.ResponseWriter, r *http.Request, data any, err error) {
	switch {
	case err == nil:
		httpapi.WriteJSON(w, 200, map[string]any{"status": "success", "data": data})
	case errors.Is(err, ErrFiscalInvalid):
		httpapi.WriteError(w, r, 400, "invalid_fiscal_input", strings.TrimPrefix(err.Error(), ErrFiscalInvalid.Error()+": "))
	case errors.Is(err, ErrFiscalNotFound):
		httpapi.WriteError(w, r, 404, "fiscal_not_found", "Registro fiscal não encontrado.")
	case errors.Is(err, ErrFiscalConflict):
		httpapi.WriteError(w, r, 409, "fiscal_conflict", "O cadastro mudou. Recarregue os dados antes de salvar novamente.")
	case errors.Is(err, ErrFiscalVaultUnavailable):
		httpapi.WriteError(w, r, 503, "fiscal_vault_unavailable", "Cofre indisponível. Tente novamente após configurar a chave de segredos.")
	default:
		httpapi.WriteError(w, r, 500, "fiscal_error", "Não foi possível concluir a operação fiscal.")
	}
}

// O perfil e seus templates são pessoais: o usuário sempre vem da sessão, nunca do body/URL.
func RegisterFiscalRoutes(mux *http.ServeMux, svc *FiscalService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireFinancePermission("finance.sheets.view", h))
	}
	mux.Handle("GET /v1/finance/fiscal/profile", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.GetProfile(r.Context(), p.AccountID, p.UserID)
		fiscalResult(w, r, data, err)
	}))
	mux.Handle("PUT /v1/finance/fiscal/profile", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in FiscalProfileInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.SaveProfile(r.Context(), p.AccountID, p.UserID, in)
		fiscalResult(w, r, data, err)
	}))
	mux.Handle("PUT /v1/finance/fiscal/credentials", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in FiscalCredentialInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.SaveCredentials(r.Context(), p.AccountID, p.UserID, in)
		fiscalResult(w, r, data, err)
	}))
	mux.Handle("DELETE /v1/finance/fiscal/credentials", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
		}
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.DeleteCredentials(r.Context(), p.AccountID, p.UserID, in.ExpectedUpdatedAt)
		fiscalResult(w, r, data, err)
	}))
	mux.Handle("GET /v1/finance/fiscal/portal-check", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.PortalStatus(r.Context(), p.AccountID, p.UserID)
		fiscalResult(w, r, out, err)
	}))
	mux.Handle("POST /v1/finance/fiscal/portal-check", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
		}
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		out, err := svc.TestPortalAccess(r.Context(), p.AccountID, p.UserID, in.ExpectedUpdatedAt)
		fiscalResult(w, r, out, err)
	}))
	mux.Handle("GET /v1/finance/fiscal/templates", wrap(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.ListTemplates(r.Context(), p.AccountID, p.UserID)
		fiscalResult(w, r, data, err)
	}))
	save := func(w http.ResponseWriter, r *http.Request) {
		var in FiscalTemplateInput
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		data, err := svc.SaveTemplate(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in)
		fiscalResult(w, r, data, err)
	}
	mux.Handle("POST /v1/finance/fiscal/templates", wrap(save))
	mux.Handle("PUT /v1/finance/fiscal/templates/{id}", wrap(save))
	mux.Handle("DELETE /v1/finance/fiscal/templates/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
		}
		if !decodeFiscal(w, r, &in) {
			return
		}
		p, _ := auth.PrincipalFromContext(r.Context())
		err := svc.DeleteTemplate(r.Context(), p.AccountID, p.UserID, r.PathValue("id"), in.ExpectedUpdatedAt)
		fiscalResult(w, r, nil, err)
	}))
}
