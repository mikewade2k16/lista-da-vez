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
	mux.Handle("GET /v1/omnichannel/conversations/{cid}/messages/{mid}/media", wrap(handleGetMedia(media)))
}

// handleSendMessage recebe o body do legado, resolve escopo + permissao de reply e devolve a
// mensagem criada. 200 = enfileirado; 202 = falhou ao enfileirar (mensagem FAILED).
func handleSendMessage(svc *SendService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var in SendMessageInput
		if err := decodeSendBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, outcome, err := svc.SendMessage(r.Context(), accountID, caller, callerCanReply(r), r.PathValue("id"), in)
		if err != nil {
			writeSendError(w, r, err)
			return
		}
		status := http.StatusOK
		if outcome == outcomeFailedToQueue {
			status = http.StatusAccepted
		}
		httpapi.WriteJSON(w, status, view)
	}
}

// handleGetMedia faz stream do arquivo com Range (http.ServeContent). Content-Type explicito
// (do mime salvo, allowlist), Cache-Control private, nosniff. Disposition inline|attachment.
func handleGetMedia(svc *MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		opened, err := svc.OpenMedia(r.Context(), accountID, caller, r.PathValue("cid"), r.PathValue("mid"))
		if err != nil {
			writeSendError(w, r, err)
			return
		}
		defer func() { _ = opened.File.Close() }()

		w.Header().Set("Content-Type", opened.MimeType)
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", mediaDisposition(r, opened.FileName))
		// ServeContent resolve Range/If-Range/206/Content-Range/Accept-Ranges e NAO carrega o
		// arquivo em memoria (seek no *os.File). O legado fazia arrayBuffer() inteiro — D2 elimina.
		http.ServeContent(w, r, opened.FileName, opened.ModTime, opened.File)
	}
}

// decodeSendBody envolve o corpo num MaxBytesReader (teto de midia) antes do decode.
func decodeSendBody(w http.ResponseWriter, r *http.Request, dst *SendMessageInput) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSendBody)).Decode(dst)
}

// callerCanReply gateia a feature de RESPONDER: VIEWER nao pode (403 — permissao, nao escopo).
// O enforcement por key ainda nao existe no Go (module.go §GAP); o papel legado e a autoridade.
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
