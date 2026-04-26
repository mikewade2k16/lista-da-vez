package auth

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	User    UserView        `json:"user"`
	Session loginSessionDTO `json:"session"`
}

type loginSessionDTO struct {
	AccessToken     string    `json:"accessToken"`
	TokenType       string    `json:"tokenType"`
	ExpiresAt       time.Time `json:"expiresAt"`
	ExpiresInSecond int64     `json:"expiresInSeconds"`
}

type invitationResponse struct {
	Invitation InvitationView `json:"invitation"`
}

type acceptInvitationRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type passwordResetRequestPayload struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

type roleCatalogResponse struct {
	TenantModel string           `json:"tenantModel"`
	Roles       []RoleDefinition `json:"roles"`
}

type meResponse struct {
	User      UserView     `json:"user"`
	Principal principalDTO `json:"principal"`
}

type updateProfileRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type updatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type principalDTO struct {
	UserID              string    `json:"userId"`
	Role                Role      `json:"role"`
	TenantID            string    `json:"tenantId,omitempty"`
	StoreIDs            []string  `json:"storeIds,omitempty"`
	Permissions         []string  `json:"permissions"`
	PermissionsResolved bool      `json:"permissionsResolved"`
	ExpiresAt           time.Time `json:"expiresAt"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, invitations *InvitationService, passwordResets *PasswordResetService, middleware *Middleware) {
	mux.HandleFunc("GET /v1/auth/roles", func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, roleCatalogResponse{
			TenantModel: "tenant-owner-is-client",
			Roles:       RoleCatalog(),
		})
	})

	mux.HandleFunc("POST /v1/auth/password-reset/request", func(w http.ResponseWriter, r *http.Request) {
		var request passwordResetRequestPayload
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		if passwordResets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Recuperacao de senha indisponivel.")
			return
		}

		if err := passwordResets.Request(r.Context(), PasswordResetRequestInput{Email: request.Email}); err != nil {
			writePasswordResetError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	})

	mux.HandleFunc("POST /v1/auth/password-reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		var request passwordResetConfirmRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		if passwordResets == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Recuperacao de senha indisponivel.")
			return
		}

		user, err := passwordResets.Confirm(r.Context(), PasswordResetConfirmInput{
			Email:    request.Email,
			Code:     request.Code,
			Password: request.Password,
		})
		if err != nil {
			writePasswordResetError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"user": user,
		})
	})

	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var request loginRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.Login(r.Context(), LoginInput{
			Email:    request.Email,
			Password: request.Password,
		})
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidCredentials):
				httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_credentials", "Email ou senha invalidos.")
			case errors.Is(err, ErrUserInactive):
				httpapi.WriteError(w, r, http.StatusForbidden, "user_inactive", "Usuario inativo.")
			case errors.Is(err, ErrOnboardingRequired):
				httpapi.WriteError(w, r, http.StatusForbidden, "onboarding_required", "Conta criada, mas ainda falta definir a primeira senha pelo convite.")
			default:
				httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao autenticar.")
			}
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, loginResponse{
			User: result.User,
			Session: loginSessionDTO{
				AccessToken:     result.Session.AccessToken,
				TokenType:       result.Session.TokenType,
				ExpiresAt:       result.Session.ExpiresAt,
				ExpiresInSecond: int64(time.Until(result.Session.ExpiresAt).Seconds()),
			},
		})
	})

	mux.Handle("GET /v1/auth/me", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		user, err := service.CurrentUser(r.Context(), principal)
		if err != nil {
			switch {
			case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrUserInactive):
				httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Sessao invalida.")
			default:
				httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar o usuario autenticado.")
			}
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, meResponse{
			User: user,
			Principal: principalDTO{
				UserID:              principal.UserID,
				Role:                principal.Role,
				TenantID:            principal.TenantID,
				StoreIDs:            append([]string{}, principal.StoreIDs...),
				Permissions:         append([]string{}, principal.Permissions...),
				PermissionsResolved: principal.PermissionsResolved,
				ExpiresAt:           principal.ExpiresAt,
			},
		})
	})))

	mux.Handle("PATCH /v1/auth/me/profile", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request updateProfileRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		user, err := service.UpdateProfile(r.Context(), principal, UpdateProfileInput{
			DisplayName: request.DisplayName,
			Email:       request.Email,
		})
		if err != nil {
			writeSelfServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"user": user,
		})
	})))

	mux.Handle("PATCH /v1/auth/me/password", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request updatePasswordRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		user, err := service.ChangePassword(r.Context(), principal, ChangePasswordInput{
			CurrentPassword: request.CurrentPassword,
			NewPassword:     request.NewPassword,
		})
		if err != nil {
			writeSelfServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"user": user,
		})
	})))

	mux.Handle("POST /v1/auth/me/avatar", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_avatar", "Nao foi possivel ler a foto enviada.")
			return
		}

		file, header, err := r.FormFile("avatar")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_avatar", "Envie uma foto valida.")
			return
		}
		defer file.Close()

		content, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_avatar", "Nao foi possivel processar a foto.")
			return
		}

		if len(content) > maxAvatarBytes {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_avatar", "Envie uma imagem JPG, PNG ou WebP com ate 2 MB.")
			return
		}

		user, err := service.UpdateAvatar(r.Context(), principal, UpdateAvatarInput{
			FileName:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
			Content:     content,
		})
		if err != nil {
			writeSelfServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"user": user,
		})
	})))

	mux.HandleFunc("GET /v1/auth/invitations/{token}", func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("token")
		result, err := invitations.Inspect(r.Context(), token)
		if err != nil {
			writeInvitationError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, invitationResponse{
			Invitation: result.Invitation,
		})
	})

	mux.HandleFunc("POST /v1/auth/invitations/accept", func(w http.ResponseWriter, r *http.Request) {
		var request acceptInvitationRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := invitations.Accept(r.Context(), InvitationAcceptInput{
			Token:    request.Token,
			Password: request.Password,
		})
		if err != nil {
			writeInvitationError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, loginResponse{
			User: result.User,
			Session: loginSessionDTO{
				AccessToken:     result.Session.AccessToken,
				TokenType:       result.Session.TokenType,
				ExpiresAt:       result.Session.ExpiresAt,
				ExpiresInSecond: int64(time.Until(result.Session.ExpiresAt).Seconds()),
			},
		})
	})

	mux.Handle("GET /v1/dev/ping", middleware.RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "Area interna da plataforma liberada para platform_admin.",
		})
	}), RolePlatformAdmin))
}

func writeSelfServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "user_conflict", "Ja existe usuario com este email.")
	case errors.Is(err, ErrInvalidAvatar):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_avatar", "Envie uma imagem JPG, PNG ou WebP com ate 2 MB.")
	case errors.Is(err, ErrInvalidCredentials):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados enviados.")
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrUserInactive):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Sessao invalida.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao atualizar o perfil.")
	}
}

func writeInvitationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvitationNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "invitation_not_found", "Convite nao encontrado.")
	case errors.Is(err, ErrInvitationExpired):
		httpapi.WriteError(w, r, http.StatusGone, "invitation_expired", "Este convite expirou. Gere um novo convite para continuar.")
	case errors.Is(err, ErrInvitationAccepted):
		httpapi.WriteError(w, r, http.StatusConflict, "invitation_accepted", "Este convite ja foi utilizado.")
	case errors.Is(err, ErrInvitationRevoked):
		httpapi.WriteError(w, r, http.StatusGone, "invitation_revoked", "Este convite foi revogado.")
	case errors.Is(err, ErrInvalidCredentials):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Defina uma senha valida para concluir o convite.")
	case errors.Is(err, ErrUserInactive):
		httpapi.WriteError(w, r, http.StatusForbidden, "user_inactive", "Usuario inativo.")
	default:
		slog.Error("auth_invitation_error", "request_id", httpapi.RequestIDFromContext(r.Context()), "error", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar o convite.")
	}
}

func writePasswordResetError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Informe email, codigo e uma senha com pelo menos 8 caracteres.")
	case errors.Is(err, ErrPasswordResetNotFound), errors.Is(err, ErrPasswordResetConsumed):
		httpapi.WriteError(w, r, http.StatusBadRequest, "password_reset_invalid", "Codigo invalido ou ja utilizado.")
	case errors.Is(err, ErrPasswordResetExpired):
		httpapi.WriteError(w, r, http.StatusGone, "password_reset_expired", "Este codigo expirou. Solicite um novo para continuar.")
	case errors.Is(err, ErrUserInactive):
		httpapi.WriteError(w, r, http.StatusForbidden, "user_inactive", "Usuario inativo.")
	case errors.Is(err, ErrOnboardingRequired):
		httpapi.WriteError(w, r, http.StatusConflict, "onboarding_required", "Use o convite do primeiro acesso para definir a senha inicial.")
	default:
		slog.Error("auth_password_reset_error", "request_id", httpapi.RequestIDFromContext(r.Context()), "error", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar a recuperacao de senha.")
	}
}
