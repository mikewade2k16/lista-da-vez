package cardapio

import (
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// registerPhotoRoutes monta os endpoints de foto de produto (aba Produtos).
func registerPhotoRoutes(mux *http.ServeMux, wrap func(http.HandlerFunc) http.Handler, svc *Service) {
	mux.Handle("POST /v1/cardapio/restaurants/{id}/product-photos", wrap(handleUploadProductPhoto(svc)))
	mux.Handle("GET /v1/cardapio/restaurants/{id}/product-photos/large", wrap(handleListLargeProductPhotos(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/product-photos/{photoId}/reviewed",
		wrap(handleMarkProductPhotoReviewed(svc)))
}

// maxProductPhotoMultipartBytes cobre a foto (20MB) + headers do form.
const maxProductPhotoMultipartBytes = maxProductPhotoBytes + 256*1024

func handleUploadProductPhoto(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requireCardapioPerm(svc, r, accountID, permManage); err != nil {
			writeServiceError(w, r, err)
			return
		}
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Envie multipart/form-data.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxProductPhotoMultipartBytes)
		if err := r.ParseMultipartForm(maxProductPhotoMultipartBytes); err != nil { //nolint:gosec // corpo limitado acima
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Foto acima de 20MB ou upload invalido.")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo ausente.")
			return
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, maxProductPhotoBytes+1))
		if err != nil || len(content) == 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo invalido.")
			return
		}
		photo, err := svc.SaveProductPhoto(r.Context(), accountID, r.PathValue("id"),
			strings.TrimSpace(header.Filename), strings.TrimSpace(header.Header.Get("Content-Type")), content)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, photo)
	}
}

// requirePhotoAdmin limita o lembrete de originais grandes ao platform_admin
// (404 uniforme para os demais).
func requirePhotoAdmin(r *http.Request) error {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.Role != auth.RolePlatformAdmin {
		return ErrForbidden
	}
	return nil
}

func handleListLargeProductPhotos(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requirePhotoAdmin(r); err != nil {
			writeServiceError(w, r, err)
			return
		}
		items, err := svc.ListLargeProductPhotos(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"photos":   items,
			"minBytes": largeProductPhotoBytes,
		})
	}
}

func handleMarkProductPhotoReviewed(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := requirePhotoAdmin(r); err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := svc.MarkProductPhotoReviewed(r.Context(), accountID, r.PathValue("id"), r.PathValue("photoId")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
