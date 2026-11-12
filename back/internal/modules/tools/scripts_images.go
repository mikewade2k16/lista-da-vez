package tools

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxScriptImageBytes = 10 << 20

var errScriptStorageUnavailable = errors.New("script storage unavailable")

type scriptImage struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	MIME string `json:"-"`
	Size int64  `json:"-"`
	// Account owning the file (differs from the caller for images of a shared script).
	Account string `json:"-"`
}

func scriptImagePath(root, account, id string) (string, error) {
	if root == "" {
		return "", errScriptStorageUnavailable
	}
	if !isUUID(account) || !isUUID(id) {
		return "", errScriptNotFound
	}
	return filepath.Join(root, "scripts", account, id), nil
}
func (s *scriptService) uploadImage(ctx context.Context, account string, reader io.Reader) (scriptImage, error) {
	if !isUUID(account) {
		return scriptImage{}, errInvalidScript
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxScriptImageBytes+1))
	if err != nil {
		return scriptImage{}, err
	}
	if len(data) == 0 || len(data) > maxScriptImageBytes {
		return scriptImage{}, errInvalidScript
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return scriptImage{}, errInvalidScript
	}
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return scriptImage{}, err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:])
	path, err := scriptImagePath(s.imageRoot, account, id)
	if err != nil {
		return scriptImage{}, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return scriptImage{}, err
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		return scriptImage{}, err
	}
	image := scriptImage{ID: id, URL: "/v1/tools/scripts/images/" + id, MIME: mime, Size: int64(len(data))}
	if err = s.store.addImage(ctx, account, image); err != nil {
		_ = os.Remove(path)
		return scriptImage{}, err
	}
	return image, nil
}
func (s *scriptService) openImage(ctx context.Context, account, id string) (*os.File, scriptImage, error) {
	if _, err := scriptImagePath(s.imageRoot, account, id); err != nil {
		return nil, scriptImage{}, err
	}
	image, err := s.store.getImage(ctx, account, id)
	if err != nil {
		return nil, image, err
	}
	path, err := scriptImagePath(s.imageRoot, image.Account, image.ID)
	if err != nil {
		return nil, image, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, image, errScriptNotFound
	}
	return file, image, err
}
func registerScriptImageRoutes(mux *http.ServeMux, svc *scriptService, wrap func(string, func(http.ResponseWriter, *http.Request, string)) http.Handler) {
	mux.Handle("POST /v1/tools/scripts/images", wrap("tools.scripts.manage", func(w http.ResponseWriter, r *http.Request, account string) {
		r.Body = http.MaxBytesReader(w, r.Body, maxScriptImageBytes+(1<<20))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			scriptError(w, r, errInvalidScript)
			return
		} //nolint:gosec // G120: corpo limitado acima por MaxBytesReader.
		if r.MultipartForm != nil {
			defer func() { _ = r.MultipartForm.RemoveAll() }()
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			scriptError(w, r, errInvalidScript)
			return
		}
		defer func() { _ = file.Close() }()
		image, err := svc.uploadImage(r.Context(), account, file)
		if err != nil {
			scriptError(w, r, err)
			return
		}
		writeSuccess(w, http.StatusCreated, image)
	}))
	mux.Handle("GET /v1/tools/scripts/images/{id}", wrap("tools.scripts.view", func(w http.ResponseWriter, r *http.Request, account string) {
		file, image, err := svc.openImage(r.Context(), account, r.PathValue("id"))
		if err != nil {
			scriptError(w, r, err)
			return
		}
		defer func() { _ = file.Close() }()
		stat, err := file.Stat()
		if err != nil {
			scriptError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", image.MIME)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, no-store")
		http.ServeContent(w, r, strings.TrimSpace(image.ID), stat.ModTime(), file)
	}))
}
