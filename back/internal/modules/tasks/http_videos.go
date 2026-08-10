package tasks

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func (handler *HTTPHandler) uploadTaskVideo(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	maxBytes := handler.service.TaskVideoMaxBytes(r.Context())
	if err := clearTaskVideoUploadDeadlines(w); err != nil {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "upload_unavailable", "O servidor nao conseguiu preparar o upload.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(1<<20))
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(maxTaskVideoMultipartMemory); err != nil { //nolint:gosec // corpo limitado dinamicamente pelo teto ativo via MaxBytesReader acima
		writeServiceError(w, r, ErrInvalidVideo)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		writeServiceError(w, r, ErrInvalidVideo)
		return
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxBytes {
		writeServiceError(w, r, ErrVideoTooLarge)
		return
	}
	video, err := handler.service.UploadTaskVideoStream(r.Context(), ctx.Access, r.PathValue("taskId"), strings.TrimSpace(r.Header.Get("Idempotency-Key")), header.Filename, header.Header.Get("Content-Type"), strings.TrimSpace(r.FormValue("checklistItemId")), header.Size, file)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"video": video})
}

func (handler *HTTPHandler) taskVideoLimit(w http.ResponseWriter, r *http.Request, _ taskHTTPContext) {
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"maxBytes": handler.service.TaskVideoMaxBytes(r.Context()),
	})
}

func clearTaskVideoUploadDeadlines(w http.ResponseWriter) error {
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	return controller.SetWriteDeadline(time.Time{})
}

func (handler *HTTPHandler) taskVideoContent(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("accountID"))
	objectID := strings.TrimSpace(r.PathValue("objectID"))
	requestedName := strings.TrimSpace(r.PathValue("fileName"))
	var content TaskVideoContent
	var err error
	if r.Method == http.MethodHead {
		content, err = handler.service.StatTaskVideo(r.Context(), accountID, objectID)
	} else {
		byteRange := r.Header.Get("Range")
		if byteRange != "" && r.Header.Get("If-Range") != "" {
			metadata, metadataErr := handler.service.StatTaskVideo(r.Context(), accountID, objectID)
			if metadataErr != nil {
				err = metadataErr
			} else if r.Header.Get("If-Range") != quotedTaskVideoETag(metadata.ETag) {
				byteRange = ""
			}
		}
		if err == nil {
			content, err = handler.service.OpenTaskVideo(r.Context(), accountID, objectID, byteRange)
		}
	}
	if err != nil || content.FileName != requestedName {
		if content.Body != nil {
			_ = content.Body.Close()
		}
		if errors.Is(err, ErrInvalidVideoRange) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if errors.Is(err, ErrVideoUnavailable) {
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
	if etag := quotedTaskVideoETag(content.ETag); etag != "" {
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

func quotedTaskVideoETag(etag string) string {
	etag = strings.Trim(strings.TrimSpace(etag), "\"")
	if etag == "" {
		return ""
	}
	return strconv.Quote(etag)
}
