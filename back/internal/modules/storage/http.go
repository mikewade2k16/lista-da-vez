package storage

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func RegisterRoutes(mux *http.ServeMux, service *Service, authMiddleware *auth.Middleware) {
	mux.Handle("GET /v1/storage/status", authMiddleware.RequireRoles(
		handleStatus(service),
		auth.RolePlatformAdmin,
	))
	mux.Handle("POST /v1/storage/connection-check", authMiddleware.RequireRoles(
		handleConnectionCheck(service),
		auth.RolePlatformAdmin,
	))
	mux.Handle("GET /v1/storage/settings", authMiddleware.RequireRoles(
		handleSettings(service),
		auth.RolePlatformAdmin,
	))
	mux.Handle("PUT /v1/storage/settings", authMiddleware.RequireRoles(
		handleUpdateSettings(service),
		auth.RolePlatformAdmin,
	))
	mux.Handle("POST /v1/storage/test-upload", authMiddleware.RequireAuthWithAccount(
		requirePlatformAdmin(handleTestUpload(service)),
	))
	mux.Handle("GET /v1/storage/objects/{objectID}/content", authMiddleware.RequireAuthWithAccount(
		requirePlatformAdmin(handleObjectContent(service)),
	))
}

func requirePlatformAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.Role != auth.RolePlatformAdmin {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Apenas administradores da plataforma.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleSettings(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := service.Settings(r.Context())
		if err != nil {
			writeSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "data": settings})
	})
}

func handleUpdateSettings(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input UpdateSettingsInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Informe limites validos.")
			return
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		settings, err := service.UpdateSettings(r.Context(), input, principal.UserID)
		if err != nil {
			writeSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "data": settings})
	})
}

func handleStatus(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := service.Status(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "data": status})
	})
}

func handleConnectionCheck(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := service.CheckConnection(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "data": status})
	})
}

func handleTestUpload(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.AccountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_account_id", "Selecione uma conta para testar o upload.")
			return
		}
		settings, err := service.Settings(r.Context())
		if err != nil {
			writeSettingsError(w, r, err)
			return
		}
		if err := clearUploadDeadlines(w); err != nil {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "upload_unavailable", "O servidor nao conseguiu preparar o upload.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, settings.MaxObjectBytes+(1<<20))
		if err := r.ParseMultipartForm(8 << 20); err != nil { //nolint:gosec // body limitado acima
			writeUploadReadError(w, r, err)
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "Arquivo ausente.")
			return
		}
		defer func() { _ = file.Close() }()
		content, err := io.ReadAll(io.LimitReader(file, settings.MaxObjectBytes+1))
		if err != nil {
			writeUploadReadError(w, r, err)
			return
		}
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		object, err := service.Upload(r.Context(), UploadInput{
			AccountID:      principal.AccountID,
			SourceModule:   "storage_test",
			IdempotencyKey: idempotencyKey,
			FileName:       header.Filename,
			ContentType:    header.Header.Get("Content-Type"),
			Content:        content,
			CreatedBy:      principal.UserID,
		})
		if err != nil {
			writeError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"status": "success", "data": object})
	})
}

func handleObjectContent(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.AccountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_account_id", "Selecione uma conta para visualizar o arquivo.")
			return
		}
		object, content, err := service.Download(r.Context(), principal.AccountID, r.PathValue("objectID"))
		if err != nil {
			writeError(w, r, err)
			return
		}
		defer func() { _ = content.Close() }()
		w.Header().Set("Content-Type", object.ContentType)
		w.Header().Set("Content-Length", formatInt(object.SizeBytes))
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": object.FileName}))
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, content)
	})
}

func clearUploadDeadlines(w http.ResponseWriter) error {
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	return controller.SetWriteDeadline(time.Time{})
}

func writeUploadReadError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Arquivo acima do limite configurado.")
		return
	}
	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "O arquivo nao chegou completo ou o upload esta invalido.")
}

func formatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrDisabled):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "storage_disabled", "O storage R2 esta desativado.")
	case errors.Is(err, ErrUploadsDisabled):
		httpapi.WriteError(w, r, http.StatusConflict, "r2_uploads_disabled", "Os uploads para o R2 estao desativados; os modulos continuam usando o storage local.")
	case errors.Is(err, ErrAnalyticsUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "r2_metrics_unavailable", "As metricas account-wide da Cloudflare estao indisponiveis; o upload R2 foi bloqueado por seguranca.")
	case errors.Is(err, ErrNotInitialized):
		httpapi.WriteError(w, r, http.StatusConflict, "storage_not_initialized", "Valide a conexao R2 antes do primeiro uso.")
	case errors.Is(err, ErrProviderMismatch):
		httpapi.WriteError(w, r, http.StatusConflict, "storage_provider_mismatch", "A configuracao R2 difere do provider inicializado no banco.")
	case errors.Is(err, ErrBucketNotEmpty):
		httpapi.WriteError(w, r, http.StatusConflict, "storage_bucket_not_empty", "O primeiro vinculo exige um bucket R2 dedicado e vazio.")
	case errors.Is(err, ErrClassAQuotaExceeded), errors.Is(err, ErrClassBQuotaExceeded), errors.Is(err, ErrStorageQuotaExceeded):
		httpapi.WriteError(w, r, http.StatusTooManyRequests, "storage_quota_exceeded", "O limite de seguranca do R2 foi atingido.")
	case errors.Is(err, ErrFileTypeLimit):
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "file_type_limit", "O arquivo supera o limite configurado para este tipo.")
	case errors.Is(err, ErrUnsupportedFileType):
		httpapi.WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_file_type", "Tipo de arquivo nao permitido pelo storage.")
	case errors.Is(err, ErrInvalidUpload):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "O arquivo ou a chave de idempotencia e invalido.")
	case errors.Is(err, ErrObjectNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "object_not_found", "Arquivo nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusBadGateway, "storage_provider_error", "Nao foi possivel validar o Cloudflare R2.")
	}
}

func writeSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrAnalyticsUnavailable) {
		httpapi.WriteError(w, r, http.StatusConflict, "r2_metrics_unavailable", "Configure o token de Analytics da Cloudflare antes de ativar uploads R2.")
		return
	}
	if errors.Is(err, ErrInvalidSettings) {
		httpapi.WriteError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_storage_settings",
			"Os limites devem ser positivos e permanecer dentro da franquia gratuita e do teto seguro por arquivo.",
		)
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao salvar os limites do storage.")
}
