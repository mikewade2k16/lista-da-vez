package users

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Users []UserView `json:"users"`
}

type userResponse struct {
	User       UserView                    `json:"user"`
	Invitation *auth.InvitationIssueResult `json:"invitation,omitempty"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

type resetPasswordResponse struct {
	User              UserView `json:"user"`
	TemporaryPassword string   `json:"temporaryPassword"`
}

type createRequest struct {
	DisplayName  string    `json:"displayName"`
	Email        string    `json:"email"`
	EmployeeCode string    `json:"employeeCode"`
	Password     string    `json:"password"`
	Role         auth.Role `json:"role"`
	TenantID     string    `json:"tenantId"`
	StoreIDs     []string  `json:"storeIds"`
	Active       *bool     `json:"active"`
}

type updateRequest struct {
	DisplayName  *string    `json:"displayName"`
	Email        *string    `json:"email"`
	EmployeeCode *string    `json:"employeeCode"`
	Password     *string    `json:"password"`
	Role         *auth.Role `json:"role"`
	TenantID     *string    `json:"tenantId"`
	StoreIDs     *[]string  `json:"storeIds"`
	Active       *bool      `json:"active"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/users", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var active *bool
		if rawActive := strings.TrimSpace(r.URL.Query().Get("active")); rawActive != "" {
			value := rawActive == "true" || rawActive == "1"
			active = &value
		}

		users, err := service.ListAccessible(r.Context(), principal, ListInput{
			TenantID: strings.TrimSpace(r.URL.Query().Get("tenantId")),
			StoreID:  strings.TrimSpace(r.URL.Query().Get("storeId")),
			Role:     auth.Role(strings.TrimSpace(r.URL.Query().Get("role"))),
			Active:   active,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{Users: users})
	})))

	mux.Handle("POST /v1/users", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request createRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		user, err := service.Create(r.Context(), principal, CreateInput{
			DisplayName:  request.DisplayName,
			Email:        request.Email,
			EmployeeCode: request.EmployeeCode,
			Password:     request.Password,
			Role:         request.Role,
			TenantID:     request.TenantID,
			StoreIDs:     request.StoreIDs,
			Active:       request.Active,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, userResponse{
			User:       user.User,
			Invitation: user.Invitation,
		})
	})))

	mux.Handle("PATCH /v1/users/", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		userID, action := splitUserSubroute(r.URL.Path)
		if userID == "" || action != "" {
			http.NotFound(w, r)
			return
		}

		var request updateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		user, err := service.Update(r.Context(), principal, UpdateInput{
			ID:           userID,
			DisplayName:  request.DisplayName,
			Email:        request.Email,
			EmployeeCode: request.EmployeeCode,
			Password:     request.Password,
			Role:         request.Role,
			TenantID:     request.TenantID,
			StoreIDs:     request.StoreIDs,
			Active:       request.Active,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, userResponse{User: user})
	})))

	mux.Handle("POST /v1/users/", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		userID, action := splitUserSubroute(r.URL.Path)
		if userID == "" || action == "" {
			http.NotFound(w, r)
			return
		}

		switch action {
		case "invite":
			result, err := service.Invite(r.Context(), principal, userID)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}

			httpapi.WriteJSON(w, http.StatusOK, userResponse{
				User:       result.User,
				Invitation: &result.Invitation,
			})
		case "reset-password":
			var request resetPasswordRequest
			if err := httpapi.ReadJSON(r, &request); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}

			result, err := service.ResetPassword(r.Context(), principal, userID, request.Password)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}

			httpapi.WriteJSON(w, http.StatusOK, resetPasswordResponse{
				User:              result.User,
				TemporaryPassword: result.TemporaryPassword,
			})
		case "archive":
			user, err := service.Archive(r.Context(), principal, userID)
			if err != nil {
				writeServiceError(w, r, err)
				return
			}

			httpapi.WriteJSON(w, http.StatusOK, userResponse{User: user})
		default:
			http.NotFound(w, r)
		}
	})))
}

func splitUserSubroute(path string) (string, string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/users/"), "/")
	if trimmed == "" {
		return "", ""
	}

	segments := strings.Split(trimmed, "/")
	userID := strings.TrimSpace(segments[0])
	if userID == "" {
		return "", ""
	}

	if len(segments) == 1 {
		return userID, ""
	}

	return userID, strings.TrimSpace(segments[1])
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para gerenciar usuarios.")
	case errors.Is(err, ErrPasswordForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "password_management_forbidden", "Somente o admin da plataforma pode definir ou resetar senhas pelo painel.")
	case errors.Is(err, ErrConsultantManaged):
		httpapi.WriteError(w, r, http.StatusConflict, "consultant_managed_by_roster", "Este acesso de consultor deve ser gerenciado na aba Consultores.")
	case errors.Is(err, ErrValidation), errors.Is(err, ErrTenantRequired), errors.Is(err, ErrStoreRequired), errors.Is(err, ErrInvalidStoreScope):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados do usuario.")
	case errors.Is(err, ErrConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "user_conflict", "Ja existe usuario com este email.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "user_not_found", "Usuario nao encontrado.")
	case errors.Is(err, ErrSelfArchive):
		httpapi.WriteError(w, r, http.StatusBadRequest, "self_archive_forbidden", "Nao e permitido inativar a propria conta.")
	case errors.Is(err, ErrInviteNotAllowed), errors.Is(err, auth.ErrInvitationAccepted), errors.Is(err, auth.ErrUserInactive):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invite_not_allowed", "Este usuario nao pode receber convite neste estado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar o usuario.")
	}
}
