package tasks

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func (handler *HTTPHandler) uploadTaskVideo(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	upload, err := readTaskVideoUpload(r, "video")
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	video, err := handler.service.UploadTaskVideo(r.Context(), ctx.Access, r.PathValue("taskId"), upload)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"video": video})
}

func readTaskVideoUpload(r *http.Request, fieldName string) (*TaskVideoUpload, error) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, ErrInvalidVideo
	}
	// #nosec G120 -- limite controlado por maxTaskVideoMultipartMemory acima
	if err := r.ParseMultipartForm(maxTaskVideoMultipartMemory); err != nil {
		return nil, ErrInvalidVideo
	}

	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, ErrInvalidVideo
		}
		return nil, ErrInvalidVideo
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxTaskVideoBytes+1))
	if err != nil {
		return nil, ErrInvalidVideo
	}
	if len(content) == 0 || len(content) > maxTaskVideoBytes {
		return nil, ErrInvalidVideo
	}

	return &TaskVideoUpload{
		FileName:    strings.TrimSpace(header.Filename),
		ContentType: strings.TrimSpace(header.Header.Get("Content-Type")),
		Content:     content,
	}, nil
}