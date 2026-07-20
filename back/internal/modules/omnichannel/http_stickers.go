package omnichannel

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Rotas de STICKERS (spec F12 C1). Costuradas fora do module.go (mesma convencao da F6/F7): o
// orquestrador chama RegisterStickerRoutes no RegisterRoutes do handle, passando o service
// construido no Build.
//
// RequireAuthWithAccount valida MEMBERSHIP na conta do X-Account-Id e injeta o AccountID no
// Principal. account_id vem SEMPRE do Principal, nunca do body/query. Ver = parte de ver o
// inbox (qualquer membro da conta); salvar/apagar = parte de compor resposta (nao-viewer),
// gateado por callerCanReply — o MESMO padrao do envio (F6): o enforcement por key ainda nao
// existe no Go (module.go §GAP), o papel legado e a autoridade.

// stickerMaxBody limita o corpo do POST. Cobre o teto decodificado (~1MB) inflado pelo base64
// (x4/3) + o overhead do JSON, com folga para o 413 vir do tamanho DECODIFICADO (medido no
// service via copyCapped), nao do corte do body. MaxBytesReader fecha o G120 (corpo ilimitado).
const stickerMaxBody = 2 << 20

// RegisterStickerRoutes monta GET|POST /v1/omnichannel/stickers e DELETE .../stickers/{id}.
// middleware = deps.AuthMiddleware (o mesmo das demais rotas do modulo).
func RegisterStickerRoutes(mux *http.ServeMux, svc *StickerService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}
	mux.Handle("GET /v1/omnichannel/stickers", wrap(handleListStickers(svc)))
	mux.Handle("POST /v1/omnichannel/stickers", wrap(handleCreateSticker(svc)))
	mux.Handle("DELETE /v1/omnichannel/stickers/{id}", wrap(handleDeleteSticker(svc)))
}

// handleListStickers devolve o ARRAY DIRETO (nao envelopado) de stickers, mais novos primeiro.
// Cache-Control private: sticker e dado de conta (nunca public/CDN). limit = 1..200, default 36.
func handleListStickers(svc *StickerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		views, err := svc.List(r.Context(), accountID, parseLimit(r.URL.Query().Get("limit")))
		if err != nil {
			writeStickerError(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		httpapi.WriteJSON(w, http.StatusOK, views)
	}
}

// handleCreateSticker valida + grava (via service) e devolve 201 com o objeto criado.
// callerCanReply gateia a escrita (viewer => 403 forbidden).
func handleCreateSticker(svc *StickerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		if !callerCanReply(r) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden",
				"Seu perfil nao pode salvar figurinhas nesta conta.")
			return
		}
		var in StickerInput
		if err := decodeStickerBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Create(r.Context(), accountID, caller.UserID, in)
		if err != nil {
			writeStickerError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

// handleDeleteSticker apaga a figurinha (linha + arquivo). 204 no sucesso; sticker de outra
// conta => 404 (nunca 403 = enumeration). viewer => 403.
func handleDeleteSticker(svc *StickerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		if !callerCanReply(r) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden",
				"Seu perfil nao pode apagar figurinhas nesta conta.")
			return
		}
		if err := svc.Delete(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeStickerError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// decodeStickerBody envolve o corpo num MaxBytesReader (teto do sticker) antes do decode.
func decodeStickerBody(w http.ResponseWriter, r *http.Request, dst *StickerInput) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, stickerMaxBody)).Decode(dst)
}

// writeStickerError mapeia os erros do dominio para HTTP com {message, code, details}.
// 413/415 na validacao; 404 fora de escopo (nunca 403); 400 body/data URL invalido.
func writeStickerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrMediaTooLarge):
		httpapi.WriteErrorWithDetails(w, r, http.StatusRequestEntityTooLarge, "sticker_too_large",
			"Figurinha excede o limite de 1 MB.", map[string]string{"field": "dataUrl"})
	case errors.Is(err, ErrMediaUnsupported):
		httpapi.WriteErrorWithDetails(w, r, http.StatusUnsupportedMediaType, "sticker_unsupported",
			"Formato de figurinha nao suportado (use webp, png, jpeg ou gif).",
			map[string]string{"field": "dataUrl"})
	case errors.Is(err, ErrMediaInvalid), errors.Is(err, ErrInvalidBody):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Figurinha nao encontrada.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Falha ao processar a requisicao.")
	}
}
