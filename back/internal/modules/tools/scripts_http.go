package tools

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func registerScriptRoutes(mux *http.ServeMux, svc *scriptService, middleware *auth.Middleware) {
	wrap := func(permission string, fn func(http.ResponseWriter, *http.Request, string)) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				httpapi.WriteError(w, r, 401, "unauthorized", "Autenticacao obrigatoria.")
				return
			}
			allowed := principal.Role == auth.RolePlatformAdmin
			for _, p := range principal.Permissions {
				if p == permission || (permission == "tools.scripts.view" && p == "tools.scripts.manage") {
					allowed = true
					break
				}
			}
			if !allowed {
				httpapi.WriteError(w, r, 403, "forbidden", "Sem permissao para roteiros.")
				return
			}
			account := principal.AccountID
			if principal.Role == auth.RolePlatformAdmin {
				account = r.Header.Get("X-Account-Id")
			}
			if !isUUID(account) {
				httpapi.WriteError(w, r, 400, "account_required", "Selecione uma conta valida.")
				return
			}
			fn(w, r, account)
		})
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, _ := auth.PrincipalFromContext(r.Context())
			if principal.Role == auth.RolePlatformAdmin {
				handler.ServeHTTP(w, r)
				return
			}
			middleware.RequireAuthWithAccount(handler).ServeHTTP(w, r)
		}))
	}
	mux.Handle("GET /v1/tools/scripts", wrap("tools.scripts.view", func(w http.ResponseWriter, r *http.Request, account string) {
		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if r.URL.Query().Get("offset") != "" && (err != nil || offset < 0 || offset > 1000000) {
			httpapi.WriteError(w, r, 400, "invalid_offset", "Paginacao invalida.")
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		query := r.URL.Query()
		items, err := svc.list(r.Context(), account, principal.UserID,
			scriptListFilter{Search: query.Get("q"), Client: query.Get("client"), Offset: offset})
		if err != nil {
			scriptError(w, r, err)
			return
		}
		hasMore := len(items) > 50
		if hasMore {
			items = items[:50]
		}
		httpapi.WriteJSON(w, 200, map[string]any{"data": items, "hasMore": hasMore})
	}))
	mux.Handle("PUT /v1/tools/scripts/order", wrap("tools.scripts.view", func(w http.ResponseWriter, r *http.Request, account string) {
		var input struct {
			IDs []string `json:"ids"`
		}
		if err := httpapi.ReadJSON(r, &input); err != nil {
			scriptError(w, r, errInvalidScript)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		if err := svc.reorder(r.Context(), account, principal.UserID, input.IDs); err != nil {
			scriptError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	// Clientes que podem ser marcados num roteiro (só conta-agência recebe opções).
	mux.Handle("GET /v1/tools/scripts/clients", wrap("tools.scripts.manage", func(w http.ResponseWriter, r *http.Request, account string) {
		items, err := svc.clients(r.Context(), account)
		if err != nil {
			scriptError(w, r, err)
			return
		}
		writeSuccess(w, 200, items)
	}))
	registerScriptImageRoutes(mux, svc, wrap)

	mux.Handle("GET /v1/tools/scripts/{id}", wrap("tools.scripts.view", func(w http.ResponseWriter, r *http.Request, account string) {
		item, err := svc.get(r.Context(), account, r.PathValue("id"))
		if err != nil {
			scriptError(w, r, err)
			return
		}
		writeSuccess(w, 200, item)
	}))
	save := func(w http.ResponseWriter, r *http.Request, account string) {
		var in scriptInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, 400, "invalid_body", "Dados invalidos.")
			return
		}
		item, err := svc.save(r.Context(), account, r.PathValue("id"), in)
		if err != nil {
			scriptError(w, r, err)
			return
		}
		writeSuccess(w, 200, item)
	}
	mux.Handle("POST /v1/tools/scripts", wrap("tools.scripts.manage", save))
	mux.Handle("PUT /v1/tools/scripts/{id}", wrap("tools.scripts.manage", save))
	mux.Handle("DELETE /v1/tools/scripts/{id}", wrap("tools.scripts.manage", func(w http.ResponseWriter, r *http.Request, account string) {
		if err := svc.delete(r.Context(), account, r.PathValue("id")); err != nil {
			scriptError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
func scriptError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errScriptStorageUnavailable):
		httpapi.WriteError(w, r, 503, "storage_unavailable", "Armazenamento privado indisponivel. Configure PRIVATE_FILES_DIR.")
	case errors.Is(err, errScriptNotFound):
		httpapi.WriteError(w, r, 404, "not_found", "Roteiro nao encontrado.")
	case errors.Is(err, errScriptConflict):
		httpapi.WriteError(w, r, 409, "script_conflict", "O roteiro mudou em outra sessao. Reabra o documento antes de salvar.")
	case isForeignKeyViolation(err):
		httpapi.WriteError(w, r, 400, "invalid_account", "Conta invalida.")
	case errors.Is(err, errInvalidScriptClient):
		httpapi.WriteError(w, r, 400, "invalid_client", "Cliente invalido para esta conta.")
	case errors.Is(err, errInvalidScript):
		httpapi.WriteError(w, r, 400, "invalid_script", "Informe titulo e conteudo validos.")
	default:
		httpapi.WriteError(w, r, 500, "internal_error", "Nao foi possivel acessar os roteiros.")
	}
}
