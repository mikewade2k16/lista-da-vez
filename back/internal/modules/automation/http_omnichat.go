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
			Question string `json:"question"`
			Topic    string `json:"topic"`
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

		view, err := svc.OmniChatAsk(r.Context(), scope, question, body.Topic)
		if err != nil {
			writeOmniChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
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
