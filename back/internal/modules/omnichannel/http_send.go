package omnichannel

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Rotas de ENVIO e MIDIA (spec F6). Costuradas fora do module.go (a F6 nao edita module.go —
// ver AGENT.md §Wiring pendente): o orquestrador chama RegisterSendRoutes no RegisterRoutes do
// handle, passando os services construidos no Build.
//
// RequireAuthWithAccount (igual as rotas de leitura): valida MEMBERSHIP na conta do X-Account-Id
// e injeta o AccountID no Principal. account_id vem SEMPRE do Principal, nunca do body/header cru.

// maxSendBody limita o corpo do POST de mensagem. Cobre o teto de midia (60 MB) inflado pelo
// base64 (x4/3 ~= 80 MB) + folga. O teto DECODIFICADO exato por conta e aplicado no service
// (413). MaxBytesReader antes do decode fecha o G120 (corpo ilimitado).
const maxSendBody = 90 << 20

// RegisterSendRoutes monta POST /conversations/{id}/messages e GET
// /conversations/{cid}/messages/{mid}/media. middleware = deps.AuthMiddleware.
func RegisterSendRoutes(mux *http.ServeMux, send *SendService, media *MediaService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}
	mux.Handle("POST /v1/omnichannel/conversations/{id}/messages", wrap(handleSendMessage(send)))
	mux.Handle("GET /v1/omnichannel/conversations/{id}/ai-reply-draft", wrap(handleGetAIReplyDraft(send)))
	mux.Handle("POST /v1/omnichannel/conversations/{id}/ai-reply-drafts/{draftId}/dismiss", wrap(handleDismissAIReplyDraft(send)))
	mux.Handle("GET /v1/omnichannel/conversations/{cid}/messages/{mid}/media", wrap(handleGetMedia(media)))
	mux.Handle("GET /v1/omnichannel/conversations/{cid}/messages/{mid}/media/analyses", wrap(handleListMediaAnalyses(media)))
	mux.Handle("POST /v1/omnichannel/conversations/{cid}/messages/{mid}/media/retry", wrap(handleRetryMedia(media)))
}

func handleGetAIReplyDraft(svc *SendService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := sendPrincipalScope(w, r)
		if !ok {
			return
		}
		view, err := svc.GetPendingAIReplyDraft(r.Context(), accountID, principal, r.PathValue("id"))
		if err != nil {
			writeSendError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

type dismissAIReplyDraftInput struct {
	Reason string `json:"reason"`
}

func handleDismissAIReplyDraft(svc *SendService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := sendPrincipalScope(w, r)
		if !ok {
			return
		}
		var in dismissAIReplyDraftInput
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&in); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
				return
			}
		}
		if err := svc.DismissAIReplyDraft(r.Context(), accountID, principal,
			r.PathValue("id"), r.PathValue("draftId"), in.Reason); err != nil {
			writeSendError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListMediaAnalyses(svc *MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := sendPrincipalScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListAnalyses(r.Context(), accountID, principal, r.PathValue("cid"), r.PathValue("mid"))
		if err != nil {
			writeSendError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}

func handleRetryMedia(svc *MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := sendPrincipalScope(w, r)
		if !ok {
			return
		}
		view, err := svc.RetryMedia(r.Context(), accountID, principal,
			r.PathValue("cid"), r.PathValue("mid"))
		if err != nil {
			writeSendError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, view)
	}
}

// handleSendMessage recebe o body, resolve escopo + permissao de reply e devolve a mensagem
// somente depois que takeover, mensagem e outbox commitam atomicamente.
func handleSendMessage(svc *SendService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := sendPrincipalScope(w, r)
		if !ok {
			return
		}
		var in SendMessageInput
		if err := decodeSendBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, _, err := svc.SendMessage(r.Context(), accountID, principal, r.PathValue("id"), in)
		if err != nil {
			writeSendError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// sendPrincipalScope devolve o Principal autenticado cuja membership na conta ja foi
// validada por RequireAuthWithAccount. O service usa esse Principal para consultar o RBAC
// canonico; papel legado nao autoriza envio nem retry de midia.
func sendPrincipalScope(w http.ResponseWriter, r *http.Request) (string, auth.Principal, bool) {
	accountID, ok := accountScope(r)
	if !ok {
		writeNoAccount(w, r)
		return "", auth.Principal{}, false
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeNoAccount(w, r)
		return "", auth.Principal{}, false
	}
	return accountID, principal, true
}

// handleGetMedia faz stream do arquivo com Range (http.ServeContent). Content-Type explicito
// (do mime salvo, allowlist), Cache-Control private/no-store, nosniff. Disposition inline|attachment.
func handleGetMedia(svc *MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		err := svc.ServeMedia(r.Context(), accountID, caller, r.PathValue("cid"), r.PathValue("mid"), func(opened openedMedia) {
			w.Header().Set("Content-Type", opened.MimeType)
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Disposition", mediaDisposition(r, opened.FileName))
			// Range/If-Range/206 sem carregar o arquivo inteiro em memoria.
			http.ServeContent(w, r, opened.FileName, opened.ModTime, opened.File)
		})
		if err != nil {
			writeSendError(w, r, err)
			return
		}
	}
}

// decodeSendBody envolve o corpo num MaxBytesReader (teto de midia) antes do decode.
func decodeSendBody(w http.ResponseWriter, r *http.Request, dst *SendMessageInput) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSendBody)).Decode(dst)
}

// callerCanReply permanece apenas para as rotas legadas de stickers, cujo pacote nao faz
// parte desta correcao. Envio de mensagem e retry de midia nao usam este gate por papel.
func callerCanReply(r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return false
	}
	return legacyRole(principal.Role) != legacyRoleViewer
}

// mediaDisposition monta o Content-Disposition a partir da query (disposition=inline|attachment,
// download). Default inline; qualquer download/attachment => attachment com o nome do arquivo.
func mediaDisposition(r *http.Request, fileName string) string {
	disp := "inline"
	q := r.URL.Query()
	if q.Get("disposition") == "attachment" || q.Has("download") {
		disp = "attachment"
	}
	name := sanitizeHeaderValue(fileName)
	if name == "" {
		return disp
	}
	return disp + `; filename="` + name + `"`
}

// sanitizeHeaderValue remove aspas/quebras de linha do nome antes de por no header.
func sanitizeHeaderValue(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\"", "", "\r", "", "\n", "").Replace(value))
}

// writeSendError mapeia os erros do envio/midia para HTTP. 413/415/422 com {message, code,
// details}; fora de escopo => 404 (nunca 403); reply sem permissao => 403.
func writeSendError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrMediaTooLarge):
		httpapi.WriteErrorWithDetails(w, r, http.StatusRequestEntityTooLarge, "media_too_large",
			"Arquivo excede o limite de upload da conta.", map[string]string{"field": "mediaUrl"})
	case errors.Is(err, ErrMediaUnsupported):
		httpapi.WriteErrorWithDetails(w, r, http.StatusUnsupportedMediaType, "media_unsupported",
			"Tipo de arquivo nao suportado.", map[string]string{"field": "mediaUrl"})
	case errors.Is(err, ErrSSRFBadScheme):
		httpapi.WriteErrorWithDetails(w, r, http.StatusUnprocessableEntity, "invalid_media_url",
			"URL de midia com protocolo nao permitido.", map[string]string{"field": "mediaUrl"})
	case errors.Is(err, ErrSSRFBlockedHost):
		httpapi.WriteError(w, r, http.StatusForbidden, "blocked_media_url",
			"Destino de midia nao permitido.")
	case errors.Is(err, ErrMediaInvalid), errors.Is(err, ErrInvalidBody):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden",
			"Seu perfil nao pode responder conversas nesta conta.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Falha ao processar a requisicao.")
	}
}
