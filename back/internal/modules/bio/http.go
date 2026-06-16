package bio

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxJSONBody limita o corpo dos POST/PATCH com jsonb (drafts podem ser grandes).
const maxJSONBody = 2 << 20 // 2 MiB

// RegisterRoutes monta os endpoints do painel (/v1/bio*). O gating por modulo
// (account_modules) e aplicado globalmente via RequireModuleByPath no Chain;
// aqui so exigimos autenticacao. accountID vem do Principal/X-Account-Id, NUNCA
// do body (exceto o filtro accountId, validado contra o Principal no service).
func RegisterRoutes(mux *http.ServeMux, svc *Service, storage *MediaStorage, middleware *auth.Middleware, broker *sseBroker) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/bio/bios", wrap(handleList(svc)))
	mux.Handle("POST /v1/bio/bios", wrap(handleCreate(svc)))
	mux.Handle("POST /v1/bio/bios/{id}/duplicate", wrap(handleDuplicate(svc)))
	mux.Handle("GET /v1/bio/bios/{id}", wrap(handleGet(svc)))
	mux.Handle("PATCH /v1/bio/bios/{id}", wrap(handlePatch(svc)))
	mux.Handle("DELETE /v1/bio/bios/{id}", wrap(handleDelete(svc)))
	mux.Handle("POST /v1/bio/bios/{id}/publish", wrap(handlePublish(svc, broker)))
	mux.Handle("POST /v1/bio/bios/{id}/unpublish", wrap(handleUnpublish(svc, broker)))
	mux.Handle("GET /v1/bio/bios/{id}/preview", wrap(handlePreview(svc)))
	mux.Handle("POST /v1/bio/bios/{id}/media", wrap(handleMediaUpload(svc, storage)))
	mux.Handle("GET /v1/bio/defaults", wrap(handleDefaultsGet(svc)))
	mux.Handle("PUT /v1/bio/defaults", wrap(handleDefaultsPut(svc)))
}

func handleList(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		q := r.URL.Query()
		filter := ListFilter{
			AccountID: strings.TrimSpace(q.Get("accountId")),
			Status:    strings.TrimSpace(q.Get("status")),
			Q:         strings.TrimSpace(q.Get("q")),
		}
		summaries, err := svc.List(r.Context(), isAdmin, accountID, filter)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"bios": summaries})
	}
}

func handleCreate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var req CreateRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Create(r.Context(), isAdmin, accountID, req)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleDuplicate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		// Body opcional ({accountId?}); ausente/vazio = mesma account da origem.
		var req DuplicateRequest
		if r.ContentLength != 0 {
			if err := decodeJSONBody(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
				return
			}
		}
		view, err := svc.Duplicate(r.Context(), isAdmin, accountID, r.PathValue("id"), req)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleGet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Get(r.Context(), isAdmin, accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePatch(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var req PatchRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.Patch(r.Context(), isAdmin, accountID, r.PathValue("id"), req)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDelete(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		if err := svc.Delete(r.Context(), isAdmin, accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handlePublish(svc *Service, broker *sseBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Publish(r.Context(), isAdmin, accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		// Push para as bios publicas abertas: o front recebe `updated` e refetcha.
		broker.notify(view.Slug)
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleUnpublish(svc *Service, broker *sseBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Unpublish(r.Context(), isAdmin, accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		broker.notify(view.Slug)
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePreview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		merged, err := svc.Preview(r.Context(), isAdmin, accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		// Resposta de API em application/json (nao HTML); o JSON vem do banco da
		// propria account. Sem superficie de XSS.
		_, _ = w.Write(merged) //nolint:gosec // G705: resposta JSON da API, nao HTML
	}
}

func handleMediaUpload(svc *Service, storage *MediaStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAdmin, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		bioID := r.PathValue("id")
		// Valida escopo da bio antes de aceitar o upload (404 fora do escopo).
		view, err := svc.Get(r.Context(), isAdmin, accountID, bioID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		// Corpo limitado por MaxBytesReader antes do parse — sem exaustao de memoria.
		// Teto = maior limite de midia configurado + folga (storage.MaxUploadBytes).
		maxUpload := storage.MaxUploadBytes()
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(maxUpload); err != nil { //nolint:gosec // G120: body limitado por MaxBytesReader acima
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "Upload invalido ou acima do limite.")
			return
		}
		kind := strings.TrimSpace(r.FormValue("kind"))
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_file", "Arquivo ausente.")
			return
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_upload", "Falha ao ler o arquivo.")
			return
		}

		stored, err := storage.Save(view.AccountID, kind, header.Filename, header.Header.Get("Content-Type"), content)
		if err != nil {
			writeStorageError(w, r, err)
			return
		}
		if err := svc.RegisterMedia(r.Context(), view.AccountID, view.ID, kind, stored.Path, stored.ContentType, stored.SizeBytes); err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao registrar a midia.")
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, MediaView{URL: stored.Path})
	}
}

func handleDefaultsGet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isPlatformAdmin(r) {
			writeForbidden(w, r)
			return
		}
		defaults, err := svc.Defaults(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar os defaults.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, defaults)
	}
}

func handleDefaultsPut(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isPlatformAdmin(r) {
			writeForbidden(w, r)
			return
		}
		var body struct {
			Data json.RawMessage `json:"data"`
		}
		if err := decodeJSONBody(w, r, &body); err != nil || len(body.Data) == 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		defaults, err := svc.SaveDefaults(r.Context(), body.Data)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, defaults)
	}
}

// ============================================================================
// Helpers
// ============================================================================

// principalScope resolve (isAdmin, accountID, ok) do Principal. accountID vem do
// header X-Account-Id ou, na ausencia, do TenantID do JWT.
func principalScope(r *http.Request) (bool, string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return false, "", false
	}
	isAdmin := principal.Role == auth.RolePlatformAdmin
	accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
	if accountID == "" {
		accountID = strings.TrimSpace(principal.TenantID)
	}
	// Admin pode operar sem account no contexto (lista global); nao-admin precisa.
	if !isAdmin && accountID == "" {
		return false, "", false
	}
	return isAdmin, accountID, true
}

func isPlatformAdmin(r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	return ok && principal.Role == auth.RolePlatformAdmin
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody)).Decode(dst)
}

func writeNoAccount(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
}

func writeForbidden(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrInvalidSlug):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_slug", "Slug invalido. Use apenas letras minusculas, numeros e hifen.")
	case errors.Is(err, ErrSlugTaken):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_taken", "Ja existe uma bio com este slug.")
	case errors.Is(err, ErrInvalidName):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_name", "Informe um nome valido.")
	case errors.Is(err, ErrPublishEmpty):
		httpapi.WriteError(w, r, http.StatusBadRequest, "publish_incomplete", "Preencha o logo e um fundo (video OU imagem) antes de publicar.")
	case errors.Is(err, ErrForbidden):
		writeForbidden(w, r)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a requisicao.")
	}
}

func writeStorageError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidKind):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_kind", "Tipo de midia invalido.")
	case errors.Is(err, ErrMediaTooBig):
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "media_too_big", "Arquivo acima do limite permitido.")
	case errors.Is(err, ErrInvalidMedia):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo invalido ou formato nao suportado.")
	case errors.Is(err, ErrStorageUnset):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "storage_unset", "UPLOADS_DIR nao configurado no servidor.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar a midia.")
	}
}
