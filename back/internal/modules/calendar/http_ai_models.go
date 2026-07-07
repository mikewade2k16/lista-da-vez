package calendar

import (
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAIModelsRoutes monta a listagem de modelos por provedor (Opcao C do painel).
// Sob RequireAuthWithAccount (membership na account do X-Account-Id): a rota resolve a
// API key da conta server-side, entao e tao sensivel quanto os secrets — um usuario da
// conta A NAO pode forjar X-Account-Id: B e usar a chave de B para listar. Fica sob
// /v1/calendar (gate de modulo).
func RegisterAIModelsRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrapAcct := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("GET /v1/calendar/ai/models", wrapAcct(handleListAIModels(svc)))
}

// handleListAIModels devolve `{models:[...]}` (IDs de chat) do provedor da query. O
// provider vem de `?provider=`; a chave e o endpoint resolvem server-side. Erros:
// 400 invalid_provider (fora de gemini|glm|openai), 409 ai_key_missing (sem chave),
// 502 models_unavailable (provedor falhou/chave invalida).
func handleListAIModels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		provider := strings.TrimSpace(r.URL.Query().Get("provider"))
		models, err := svc.ListAIModels(r.Context(), accountID, provider)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"models": models})
	}
}
