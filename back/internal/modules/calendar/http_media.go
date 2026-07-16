package calendar

import (
	"io"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// multipartMemory e o buffer em memoria do parse; o excedente vai para arquivos
// temporarios (nao segura o video inteiro na RAM ao parsear).
const multipartMemory = 8 << 20 // 8 MiB

// RegisterMediaRoutes monta os endpoints de anexos (Fase 3). Upload/leitura por
// account (Principal); os tetos (media-limits) sao GLOBAIS: leitura por qualquer
// autenticado, escrita so por platform_admin.
func RegisterMediaRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(h) }

	// WAVE 13: a midia pertence a um ITEM (evento). Upload continua (POST /media devolve o
	// MediaItem); a persistencia e via PUT /events/{id} (campo media). As rotas de day-media
	// (GET/PUT /day-media) sairam junto com o conceito de "anexo do dia".
	mux.Handle("POST /v1/calendar/media", wrap(handleUploadMedia(svc)))
	mux.Handle("GET /v1/calendar/media-limits", wrap(handleGetMediaLimits(svc)))
	mux.Handle("PUT /v1/calendar/media-limits", wrap(handlePutMediaLimits(svc)))
}

func handleUploadMedia(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		// Teto do corpo = maior anexo permitido (video) + folga para headers do form.
		limits, err := svc.GetMediaLimits(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limits.VideoMaxBytes+(1<<20))
		if err := r.ParseMultipartForm(multipartMemory); err != nil { //nolint:gosec // G120: corpo ja limitado pelo MaxBytesReader acima
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Upload invalido.")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo ausente.")
			return
		}
		defer file.Close()

		content, err := io.ReadAll(io.LimitReader(file, limits.VideoMaxBytes+1))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Falha ao ler o arquivo.")
			return
		}
		item, err := svc.SaveMedia(r.Context(), accountID, header.Filename, header.Header.Get("Content-Type"), content)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, item)
	}
}

func handleGetMediaLimits(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limits, err := svc.GetMediaLimits(r.Context())
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, limits)
	}
}

func handlePutMediaLimits(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.Role != auth.RolePlatformAdmin {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Apenas platform_admin.")
			return
		}
		var body MediaLimits
		if err := decodeJSONBody(w, r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		saved, err := svc.SaveMediaLimits(r.Context(), body, principal.UserID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, saved)
	}
}
