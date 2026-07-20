package omnichannel

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// maxJSONBody limita o corpo dos POST/PATCH desta fase (contatos e config — payloads
// pequenos; a midia so entra na F6).
const maxJSONBody = 1 << 20 // 1 MiB

// RegisterRoutes monta as rotas de leitura do inbox (/v1/omnichannel/*).
//
// O gating por modulo (account_modules) e aplicado globalmente via RequireModuleByPath
// no Chain (moduleGatingRules em app.go) — conta sem o modulo leva 403 module_disabled e
// platform_admin tem bypass.
//
// RequireAuthWithAccount (NAO RequireAuth) valida MEMBERSHIP na account do header
// X-Account-Id antes de qualquer handler rodar, e injeta o AccountID ja validado no
// Principal. Sem isso o gate de modulo nao fecha o furo: ele checa se a account do header
// contratou o modulo, nunca se o usuario pertence a ela — e um usuario autenticado de
// qualquer conta leria conversas, mensagens e contatos de OUTRA conta so trocando o
// header. Conversa de WhatsApp e dado pessoal de cliente final (LGPD). Mesmo gate dos
// secrets/chat do calendario; o checker cobre platform_admin e agency_owner
// (auth/account_checker.go:23).
//
// O accountID vem do Principal, NUNCA do body e nunca do header cru.
func RegisterRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}

	mux.Handle("GET /v1/omnichannel/conversations", wrap(handleListConversations(svc)))
	mux.Handle("GET /v1/omnichannel/conversations/{id}/messages", wrap(handleListMessages(svc)))
	mux.Handle("GET /v1/omnichannel/conversations/{cid}/messages/{mid}", wrap(handleGetMessage(svc)))

	mux.Handle("GET /v1/omnichannel/contacts", wrap(handleListContacts(svc)))
	mux.Handle("POST /v1/omnichannel/contacts", wrap(handleCreateContact(svc)))
	mux.Handle("PATCH /v1/omnichannel/contacts/{id}", wrap(handleUpdateContact(svc)))

	mux.Handle("GET /v1/omnichannel/account", wrap(handleGetAccount(svc)))
	mux.Handle("PATCH /v1/omnichannel/account", wrap(handleUpdateAccount(svc)))
	// /tenant = alias do /account (o front verbatim chama os dois; mesmo shape TenantSettings).
	// O inbox usa so o maxUploadMb; a config usa o objeto inteiro.
	mux.Handle("GET /v1/omnichannel/tenant", wrap(handleGetAccount(svc)))
	// /users = membros ativos da conta para o picker de atribuicao do inbox.
	mux.Handle("GET /v1/omnichannel/users", wrap(handleListTenantUsers(svc)))

	// tenant/whatsapp/* = o path que o front verbatim chama (D-B: o Go se adapta ao front).
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/instances", wrap(handleListInstances(svc)))
	mux.Handle("GET /v1/omnichannel/tenant/whatsapp/instances/access", wrap(handleListAccessibleInstances(svc)))
}

// ============================================================================
// Conversas e mensagens
// ============================================================================

func handleListConversations(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		instanceID := strings.TrimSpace(r.URL.Query().Get("instanceId"))
		conversations, err := svc.ListConversations(r.Context(), accountID, caller, instanceID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		// Array DIRETO (nao envelopado): e o que o front verbatim espera do legado.
		httpapi.WriteJSON(w, http.StatusOK, conversations)
	}
}

func handleListMessages(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		f := MessagePageFilter{
			Limit:    parseLimit(q.Get("limit")),
			BeforeID: strings.TrimSpace(q.Get("beforeId")),
		}
		page, err := svc.ListMessages(r.Context(), accountID, caller, r.PathValue("id"), f)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, page)
	}
}

// parseLimit le o query param. Invalido/ausente => 0, que o service normaliza para o
// default 100 (o zod do legado faz o mesmo: coerce com default).
func parseLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return limit
}

func handleGetMessage(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		message, err := svc.GetMessage(r.Context(), accountID, caller, r.PathValue("cid"), r.PathValue("mid"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, message)
	}
}

// ============================================================================
// Contatos
// ============================================================================

func handleListContacts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		contacts, err := svc.ListContacts(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, contacts)
	}
}

func handleCreateContact(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		var in ContactInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		contact, err := svc.CreateContact(r.Context(), accountID, in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, contact)
	}
}

func handleUpdateContact(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		var patch ContactPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		contact, err := svc.UpdateContact(r.Context(), accountID, r.PathValue("id"), patch)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, contact)
	}
}

// ============================================================================
// Conta e instancias
// ============================================================================

func handleListTenantUsers(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, ok := scope(w, r)
		if !ok {
			return
		}
		users, err := svc.ListTenantUsers(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, users)
	}
}

func handleGetAccount(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		settings, err := svc.GetAccountSettings(r.Context(), accountID, caller)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, settings)
	}
}

func handleUpdateAccount(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		var patch AccountSettingsPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		settings, err := svc.UpdateAccountSettings(r.Context(), accountID, caller, patch)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, settings)
	}
}

func handleListInstances(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.ListInstances(r.Context(), accountID, caller)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleListAccessibleInstances(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, caller, ok := scope(w, r)
		if !ok {
			return
		}
		view, err := svc.ListAccessibleInstances(r.Context(), accountID, caller)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// ============================================================================
// Helpers
// ============================================================================

// scope resolve accountID + Caller do Principal e ja escreve o 403 no_account quando
// nao ha conta no contexto. Padrao: todo handler comeca por aqui.
func scope(w http.ResponseWriter, r *http.Request) (string, Caller, bool) {
	accountID, ok := accountScope(r)
	if !ok {
		writeNoAccount(w, r)
		return "", Caller{}, false
	}
	return accountID, callerFrom(r), true
}

// accountScope resolve o accountID a partir do PRINCIPAL — nunca do body, nunca do header
// cru. O RequireAuthWithAccount (RegisterRoutes) ja leu o X-Account-Id, VALIDOU o
// membership em core.account_users e injetou o resultado em principal.AccountID: aqui so
// se le o que ele carimbou.
//
// NAO reintroduzir a leitura de r.Header.Get("X-Account-Id") nem o fallback para
// principal.TenantID. Ler o header direto pula a validacao de membership (o usuario
// escolheria a conta que quisesse) e o fallback para TenantID mascararia o header ausente
// em vez de deixar o middleware devolver 400 missing_account_id.
//
// O modulo e sempre escopado por account, inclusive para admin (que opera na account do
// switcher; o checker cobre platform_admin e agency_owner).
func accountScope(r *http.Request) (string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	accountID := strings.TrimSpace(principal.AccountID)
	if accountID == "" {
		return "", false
	}
	return accountID, true
}

// callerFrom monta o Caller do Principal. IsAdmin cobre os papeis administrativos do
// Omni; e o que decide o filtro de instancia (A2) e o canViewSensitive.
func callerFrom(r *http.Request) Caller {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return Caller{}
	}
	return Caller{
		UserID:  principal.UserID,
		IsAdmin: legacyRole(principal.Role) == legacyRoleAdmin,
	}
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody)).Decode(dst)
}

func writeNoAccount(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, http.StatusForbidden, "no_account", "Account context required.")
}

// writeServiceError mapeia o erro do dominio para HTTP.
//
// ErrNotFound cobre TAMBEM o recurso de outra conta: 404, nunca 403 — 403 confirmaria
// que o recurso existe (enumeration). Mensagens acionaveis, sem payload (principio 5 e
// canonico §10: nunca logar/ecoar payload bruto).
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrInvalidPhone):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_phone",
			"Telefone invalido. Informe o numero com DDD (so digitos).")
	case errors.Is(err, ErrPhoneConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "phone_conflict",
			"Ja existe um contato com este telefone nesta conta.")
	case errors.Is(err, ErrInvalidLimit):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_limit",
			"Limite invalido (use um numero entre 1 e 200).")
	case errors.Is(err, ErrInvalidBody):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden",
			"Voce nao tem acesso a esta operacao nesta conta.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Falha ao processar a requisicao.")
	}
}
