package cardapio

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRoutes monta os endpoints do painel de restaurantes/dominios/media. O
// gating por modulo (account_modules) e aplicado globalmente via
// RequireModuleByPath no Chain; aqui exigimos apenas autenticacao.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/cardapio/restaurants", wrap(handleListRestaurants(svc)))
	mux.Handle("POST /v1/cardapio/restaurants", wrap(handleCreateRestaurant(svc)))
	mux.Handle("GET /v1/cardapio/restaurants/{id}", wrap(handleGetRestaurant(svc)))
	mux.Handle("PATCH /v1/cardapio/restaurants/{id}", wrap(handleUpdateRestaurant(svc)))
	mux.Handle("DELETE /v1/cardapio/restaurants/{id}", wrap(handleDeleteRestaurant(svc)))

	mux.Handle("GET /v1/cardapio/restaurants/{id}/domains", wrap(handleListDomains(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/domains", wrap(handleCreateDomain(svc)))
	mux.Handle("DELETE /v1/cardapio/domains", wrap(handleDeleteDomain(svc)))

	mux.Handle("GET /v1/cardapio/restaurants/{id}/delivery-zones", wrap(handleListZones(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/delivery-zones", wrap(handleCreateZone(svc)))
	mux.Handle("PATCH /v1/cardapio/delivery-zones/{id}", wrap(handleUpdateZone(svc)))
	mux.Handle("DELETE /v1/cardapio/delivery-zones/{id}", wrap(handleDeleteZone(svc)))

	mux.Handle("POST /v1/cardapio/restaurants/{id}/media", wrap(handleUploadMedia(svc)))
}

// scopedAccountID resolve o accountId efetivo da requisicao validado contra o
// Principal. platform_admin pode informar qualquer accountId (ou "" para "todos"
// no contexto de listagem); demais papeis tem o accountId fixado na propria
// account e qualquer accountId divergente vira 404 (escopo uniforme). allowEmpty
// permite "" so para admin (listagem sem filtro).
func scopedAccountID(r *http.Request, allowEmpty bool) (string, bool, error) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false, errNoAccount
	}
	requested := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if requested == "" {
		requested = strings.TrimSpace(r.Header.Get("X-Account-Id"))
	}

	if principal.Role == auth.RolePlatformAdmin {
		if requested == "" && !allowEmpty {
			// admin sem filtro num endpoint que exige escopo => 404 uniforme.
			return "", true, errNoAccount
		}
		return requested, true, nil
	}

	own := strings.TrimSpace(principal.AccountID)
	if own == "" {
		own = strings.TrimSpace(principal.TenantID)
	}
	if own == "" {
		return "", false, errNoAccount
	}
	if requested != "" && requested != own {
		// escopo forjado => 404 uniforme (nao revelar existencia de outra account).
		return "", false, ErrNotFound
	}
	return own, false, nil
}

var errNoAccount = errors.New("cardapio: no account context")

// listScopeAccountID resolve o escopo da LISTAGEM. Para platform_admin o filtro
// vem SO do query accountId (vazio = todas as accounts, igual a bio) — o header
// X-Account-Id serve ao gating de modulo, nao restringe o que o admin enxerga.
// Nao-admin fica preso na propria account (accountId divergente => 404 uniforme).
func listScopeAccountID(r *http.Request) (string, error) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", errNoAccount
	}
	requested := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if principal.Role == auth.RolePlatformAdmin {
		return requested, nil
	}
	own := strings.TrimSpace(principal.AccountID)
	if own == "" {
		own = strings.TrimSpace(principal.TenantID)
	}
	if own == "" {
		return "", errNoAccount
	}
	if requested != "" && requested != own {
		return "", ErrNotFound
	}
	return own, nil
}

// writeServiceError traduz erros de dominio para HTTP no formato do contrato.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Dados invalidos.")
	case errors.Is(err, ErrSlugConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_conflict", "Ja existe um registro com este identificador.")
	case errors.Is(err, ErrInvalidMedia):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Imagem invalida (formato ou tamanho).")
	case errors.Is(err, ErrVersionConflict):
		httpapi.WriteError(w, r, http.StatusPreconditionFailed, "version_conflict", "O layout foi alterado em outra aba. Recarregue e tente de novo.")
	case errors.Is(err, errNoAccount):
		httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Contexto de account obrigatorio.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno.")
	}
}

// ============================================================================
// Restaurants
// ============================================================================

func handleListRestaurants(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := listScopeAccountID(r)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		items, err := svc.ListRestaurants(r.Context(), accountID, r.URL.Query().Get("q"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"restaurants": items})
	}
}

func handleCreateRestaurant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in CreateRestaurantInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		// Sobrescreve a query com o accountId do body (admin escolhe o cliente).
		if strings.TrimSpace(in.AccountID) != "" {
			q := r.URL.Query()
			q.Set("accountId", strings.TrimSpace(in.AccountID))
			r.URL.RawQuery = q.Encode()
		}
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		view, err := svc.CreateRestaurant(r.Context(), accountID, in.Slug, in.Name)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleGetRestaurant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		view, err := svc.GetRestaurant(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleUpdateRestaurant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, isAdmin, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		var in UpdateRestaurantInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		// Mover de conta e exclusivo de platform_admin: nao-admin nunca troca a
		// account do restaurante (zera o campo antes do service decidir).
		if !isAdmin {
			in.AccountID = nil
		}
		view, err := svc.UpdateRestaurant(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteRestaurant(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := svc.DeleteRestaurant(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Domains
// ============================================================================

func handleListDomains(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		items, err := svc.ListDomains(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"domains": items})
	}
}

func handleCreateDomain(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		var in DomainInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateDomain(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleDeleteDomain(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		host := strings.TrimSpace(r.URL.Query().Get("host"))
		if host == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "host obrigatorio.")
			return
		}
		if err := svc.DeleteDomain(r.Context(), accountID, host); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Media
// ============================================================================

func handleUploadMedia(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Envie multipart/form-data.")
			return
		}
		// Limita o corpo total antes de parsear (evita exaustao de memoria).
		r.Body = http.MaxBytesReader(w, r.Body, maxMediaMultipartBytes)
		if err := r.ParseMultipartForm(maxMediaMultipartBytes); err != nil { //nolint:gosec // corpo limitado por MaxBytesReader acima
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Upload invalido.")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo ausente.")
			return
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, maxMediaBytes+1))
		if err != nil || len(content) == 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Arquivo invalido.")
			return
		}
		url, err := svc.SaveMedia(r.Context(), accountID, r.PathValue("id"),
			strings.TrimSpace(header.Filename), strings.TrimSpace(header.Header.Get("Content-Type")), content)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, map[string]string{"url": url})
	}
}

// parsePage le page/perPage da query (1-based; perPage default no service).
func parsePage(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	return page, perPage
}
