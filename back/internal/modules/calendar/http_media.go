package calendar

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	wrapView := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireCalendarPermission("calendar.view", h))
	}
	wrapManage := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(requireCalendarPermission("calendar.manage", h))
	}

	// WAVE 13: a midia pertence a um ITEM (evento). Upload continua (POST /media devolve o
	// MediaItem); a persistencia e via PUT /events/{id} (campo media). As rotas de day-media
	// (GET/PUT /day-media) sairam junto com o conceito de "anexo do dia".
	mux.Handle("POST /v1/calendar/media", wrapManage(handleUploadMedia(svc)))
	// URLs permanecem no contrato /uploads/calendar/... para nao alterar thumbs,
	// viewer e player. O shape com objectID e exclusivo dos objetos novos no R2;
	// arquivos legados (account/arquivo) continuam no file server global.
	mux.HandleFunc("GET /uploads/calendar/{accountID}/{objectID}/{fileName}", handleCalendarMediaContent(svc))
	mux.HandleFunc("HEAD /uploads/calendar/{accountID}/{objectID}/{fileName}", handleCalendarMediaContent(svc))
	mux.Handle("GET /v1/calendar/media-limits", wrapView(handleGetMediaLimits(svc)))
	mux.Handle("PUT /v1/calendar/media-limits", wrapManage(handlePutMediaLimits(svc)))
}

func handleUploadMedia(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		// Uploads grandes nao podem herdar os deadlines globais da API (15s de
		// leitura / 30s de escrita). O tamanho continua limitado por MaxBytesReader;
		// somente esta rota pode levar o tempo necessario da conexao do usuario.
		if err := clearMediaUploadDeadlines(w); err != nil {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "upload_unavailable",
				"O servidor nao conseguiu preparar este upload. Tente novamente.")
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
			writeMediaUploadReadError(w, r, err)
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo ausente.")
			return
		}
		defer func() { _ = file.Close() }()

		if header.Size <= 0 || header.Size > limits.VideoMaxBytes {
			writeServiceError(w, r, ErrMediaTooLarge)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := svc.SaveScopedMediaStream(r.Context(), accountID, principal.UserID,
			r.Header.Get("Idempotency-Key"), header.Filename, header.Header.Get("Content-Type"), header.Size, file)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, item)
	}
}

func handleCalendarMediaContent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := strings.TrimSpace(r.PathValue("accountID"))
		objectID := strings.TrimSpace(r.PathValue("objectID"))
		requestedName := strings.TrimSpace(r.PathValue("fileName"))
		var content MediaContent
		var err error
		if r.Method == http.MethodHead {
			content, err = svc.StatMedia(r.Context(), accountID, objectID)
		} else {
			byteRange := r.Header.Get("Range")
			if byteRange != "" && r.Header.Get("If-Range") != "" {
				metadata, metadataErr := svc.StatMedia(r.Context(), accountID, objectID)
				if metadataErr != nil {
					err = metadataErr
				} else if r.Header.Get("If-Range") != quotedMediaETag(metadata.ETag) {
					byteRange = ""
				}
			}
			if err == nil {
				content, err = svc.OpenMedia(r.Context(), accountID, objectID, byteRange)
			}
		}
		if err != nil || content.FileName != requestedName {
			if content.Body != nil {
				_ = content.Body.Close()
			}
			if errors.Is(err, ErrInvalidMediaRange) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if errors.Is(err, ErrMediaUnavailable) {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", content.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(content.FileName))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if etag := quotedMediaETag(content.ETag); etag != "" {
			w.Header().Set("ETag", etag)
		}
		if content.ContentLength > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(content.ContentLength, 10))
		}
		if content.ContentRange != "" {
			w.Header().Set("Content-Range", content.ContentRange)
			w.WriteHeader(http.StatusPartialContent)
		}
		if r.Method == http.MethodHead {
			return
		}
		defer func() { _ = content.Body.Close() }()
		_, _ = io.Copy(w, content.Body)
	}
}

func quotedMediaETag(etag string) string {
	etag = strings.Trim(strings.TrimSpace(etag), "\"")
	if etag == "" {
		return ""
	}
	return strconv.Quote(etag)
}

// clearMediaUploadDeadlines remove os prazos globais apenas do upload de midia.
// ReadHeaderTimeout continua protegendo o recebimento dos headers e MaxBytesReader
// limita o corpo; as demais rotas preservam os deadlines padrao do servidor.
func clearMediaUploadDeadlines(w http.ResponseWriter) error {
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	return controller.SetWriteDeadline(time.Time{})
}

// writeMediaUploadReadError traduz tamanho, timeout e multipart invalido em
// respostas distintas para o frontend explicar a causa real ao usuario.
func writeMediaUploadReadError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "media_too_large",
			"Arquivo acima do limite permitido pelo servidor.")
		return
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		httpapi.WriteError(w, r, http.StatusRequestTimeout, "upload_timeout",
			"A conexao demorou demais ou parou de enviar dados.")
		return
	}
	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media",
		"O arquivo nao chegou completo ou o upload esta invalido.")
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
