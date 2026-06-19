package automation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// omniChatMaxQuestionLen e o limite de caracteres da pergunta (contrato:
// 400 question_too_long acima disso).
const omniChatMaxQuestionLen = 2000

// omniChatMaxPersonaLen e o limite de caracteres do systemPrompt da persona do Omni
// Chat (contrato: 400 prompt_too_long acima disso).
const omniChatMaxPersonaLen = 20000

// omniChatPersonaView e a resposta do contrato de config do Omni Chat: o systemPrompt
// efetivo da account, se ele e o default embutido (true) ou um custom salvo (false), e
// a janela de memoria (interacoes que o n8n mantem no contexto).
type omniChatPersonaView struct {
	SystemPrompt  string `json:"systemPrompt"`
	IsDefault     bool   `json:"isDefault"`
	HistoryWindow int    `json:"historyWindow"`
}

// handleOmniChatAsk responde POST /v1/omni-chat/ask: chat interno do painel de
// Operacao ligado ao n8n. Auth e' RequireAuth (rota FORA do prefixo
// /v1/automation, de proposito — nao exige o modulo automation de quem usa
// Operacao). accountID vem do principal (X-Account-Id), nunca do body.
func handleOmniChatAsk(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		// Escopo completo para o context token (Fase 2). accountID e' o canonico
		// (X-Account-Id resolvido); tenant/store/user/role vem do principal
		// autenticado, NUNCA do body. Sem principal, segue so com o accountID.
		scope := ContextScope{AccountID: accountID}
		if principal, hasPrincipal := auth.PrincipalFromContext(r.Context()); hasPrincipal {
			scope.TenantID = principal.TenantID
			scope.StoreIDs = principal.StoreIDs
			scope.UserID = principal.UserID
			scope.Role = string(principal.Role)
		}

		var body struct {
			Question       string `json:"question"`
			Topic          string `json:"topic"`
			ConversationID string `json:"conversationId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}

		question := strings.TrimSpace(body.Question)
		switch {
		case question == "":
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_question", "A pergunta nao pode ficar vazia.")
			return
		case len(question) > omniChatMaxQuestionLen:
			httpapi.WriteError(w, r, http.StatusBadRequest, "question_too_long", "A pergunta e' longa demais (max 2000 caracteres).")
			return
		}

		// conversationId vem do front (1 por conversa) e escopa a memoria do n8n;
		// o service o combina com account+user (nunca confia no body p/ escopo).
		view, err := svc.OmniChatAsk(r.Context(), scope, question, body.Topic, body.ConversationID)
		if err != nil {
			writeOmniChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// handleOmniChatPersonaGet responde GET /v1/omni-chat/persona: devolve o systemPrompt
// EFETIVO da account (custom salvo no banco, ou o default embutido) + isDefault.
// RequireAuth; accountID vem do principal (X-Account-Id), nunca do body.
func handleOmniChatPersonaGet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		prompt, isDefault, historyWindow, err := svc.OmniChatConfig(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar a persona do Omni Chat.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, omniChatPersonaView{SystemPrompt: prompt, IsDefault: isDefault, HistoryWindow: historyWindow})
	}
}

// handleOmniChatPersonaPut responde PUT /v1/omni-chat/persona: salva o systemPrompt
// customizado da account e passa a valer (isDefault=false). RequireAuth; accountID do
// principal. Valida vazio (400 empty_prompt) e tamanho (400 prompt_too_long).
func handleOmniChatPersonaPut(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			SystemPrompt  string `json:"systemPrompt"`
			HistoryWindow int    `json:"historyWindow"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		trimmed := strings.TrimSpace(body.SystemPrompt)
		switch {
		case trimmed == "":
			httpapi.WriteError(w, r, http.StatusBadRequest, "empty_prompt", "O comportamento nao pode ficar vazio.")
			return
		case len(trimmed) > omniChatMaxPersonaLen:
			httpapi.WriteError(w, r, http.StatusBadRequest, "prompt_too_long", "O comportamento e' longo demais (max 20000 caracteres).")
			return
		}
		// historyWindow normalizado no service (0/ausente -> default; clamp 1..20).
		saved, savedWindow, err := svc.SetOmniChatConfig(r.Context(), accountID, trimmed, body.HistoryWindow)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar a persona do Omni Chat.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, omniChatPersonaView{SystemPrompt: saved, IsDefault: false, HistoryWindow: savedWindow})
	}
}

// writeOmniChatError mapeia os erros do Omni Chat para o contrato congelado.
func writeOmniChatError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrN8NNotConfigured):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "omnichat_not_configured",
			"O Omni Chat ainda nao foi configurado (AUTOMATION_N8N_INTERNAL_URL/AUTOMATION_RUNTIME_TOKEN).")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		httpapi.WriteError(w, r, http.StatusGatewayTimeout, "omnichat_timeout",
			"O assistente demorou demais para responder. Tente de novo.")
	case errors.Is(err, errN8NFailed):
		httpapi.WriteError(w, r, http.StatusBadGateway, "omnichat_error",
			"Nao foi possivel falar com o assistente agora.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Falha ao responder no Omni Chat.")
	}
}
