package calendar

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// transcribeBodyBudget e o teto do CORPO do multipart: o audio (15 MiB) + folga
// para os headers do form (mesmo padrao do upload de midia). O tamanho do AUDIO em
// si ainda e checado em maxTranscribeBytes exatos (413 acima disso). maxMemory do
// parse = este teto => tudo em RAM, sem spill para arquivo temporario.
const transcribeBodyBudget = maxTranscribeBytes + (1 << 20)

// transcribeMimes e a whitelist de tipos de audio aceitos no /chat/transcribe
// (contrato C8). Parametros do mime (ex.: "audio/webm;codecs=opus") sao removidos
// antes da comparacao.
var transcribeMimes = map[string]bool{
	"audio/webm": true,
	"audio/ogg":  true,
	"audio/mp4":  true,
	"audio/mpeg": true,
	"audio/wav":  true,
}

// RegisterChatRoutes mantem o contrato legado do Calendar e registra o contrato
// canonico do assistente. Ambos usam o mesmo service/historico; /v1/assistant fica
// fora do gate de um modulo especifico para poder operar capacidades autorizadas
// de Calendar, Tasks, Meta Ads e Core sem exigir que Calendar esteja contratado.
func RegisterChatRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	registerChatRoutesAt(mux, svc, middleware, "/v1/calendar/chat", "", false, func(h http.HandlerFunc) http.HandlerFunc {
		return requireCalendarPermission("calendar.view", h)
	})
	registerChatRoutesAt(mux, svc, middleware, "/v1/assistant/chat", AssistantSurfaceGlobal, true, requireAssistantAccess)
}

func registerChatRoutesAt(
	mux *http.ServeMux,
	svc *Service,
	middleware *auth.Middleware,
	prefix string,
	defaultSurface string,
	assistantRoute bool,
	authorize func(http.HandlerFunc) http.HandlerFunc,
) {
	// RequireAuthWithAccount valida membership e hidrata a RBAC efetiva da account.
	// Sem esse gate, uma conta poderia consumir credencial/contexto de outra por header forjado.
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(authorize(h))
	}
	mux.Handle("GET "+prefix+"/status", wrap(handleChatStatus(svc, defaultSurface)))
	mux.Handle("POST "+prefix+"/ask", wrap(handleChatAsk(svc, defaultSurface, assistantRoute)))
	mux.Handle("POST "+prefix+"/transcribe", wrap(handleChatTranscribe(svc, defaultSurface, assistantRoute)))
	// Conversas persistidas + escopo (WAVE 4, contrato D3). Todas RequireAuthWithAccount
	// (membership); acesso a cada conversa/cliente resolvido server-side pela permissao.
	mux.Handle("GET "+prefix+"/conversations", wrap(handleListChatConversations(svc)))
	mux.Handle("POST "+prefix+"/conversations", wrap(handleCreateChatConversation(
		svc, firstNonEmpty(defaultSurface, AssistantSurfaceCalendar), assistantRoute,
	)))
	mux.Handle("GET "+prefix+"/conversations/{id}", wrap(handleGetChatConversation(svc)))
	mux.Handle("DELETE "+prefix+"/conversations/{id}", wrap(handleDeleteChatConversation(svc)))
	mux.Handle("POST "+prefix+"/conversations/{id}/messages/{messageId}/proposals/{proposalId}/confirm", wrap(handleConfirmChatProposal(svc)))
	mux.Handle("PATCH "+prefix+"/conversations/{id}/messages/{messageId}/proposals/{proposalId}/status", wrap(handleUpdateChatProposal(svc)))
	mux.Handle("GET "+prefix+"/scope", wrap(handleChatScope(svc)))
}

// requireAssistantAccess e o teto comum do chat 360. Ele nao concede nenhuma
// capacidade: apenas exige que a RBAC da account contenha ao menos uma permissao
// de leitura de um dominio suportado. A intersecao fina por modulo/operacao e
// recalculada pelo service antes de montar contexto ou executar propostas.
func requireAssistantAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || strings.TrimSpace(principal.AccountID) == "" {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}
		if principal.Role != auth.RolePlatformAdmin && principal.Role != auth.RoleOwner {
			if !principal.PermissionsResolved {
				httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Permissoes da conta indisponiveis.")
				return
			}
			allowed := []string{
				"calendar.view", "calendar.manage",
				"tasks.boards.view", "tasks.tasks.view", "tasks.tasks.create", "tasks.tasks.edit",
				"meta_ads.view", "meta_ads.manage",
				"core.users.view", "core.users.manage", "workspace.usuarios.view", "workspace.usuarios.edit",
			}
			hasAccess := false
			for _, permission := range allowed {
				if containsPermission(principal.Permissions, permission) {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para usar o assistente nesta conta.")
				return
			}
		}
		next(w, r)
	}
}

// handleChatStatus faz o preflight sem tokens usado ao abrir o painel e antes de cada
// envio. Nao persiste conversa/mensagem e nao dispara o workflow de IA.
func handleChatStatus(svc *Service, defaultSurface string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		surface := strings.TrimSpace(r.URL.Query().Get("surface"))
		if surface == "" {
			surface = defaultSurface
		}
		if surface == "" {
			if err := svc.ChatStatus(r.Context(), accountID, principal,
				r.URL.Query().Get("scopeMode"), r.URL.Query().Get("scopeClientId")); err != nil {
				writeChatError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"available": true})
			return
		}
		status, err := svc.AssistantChatStatus(r.Context(), accountID, principal, surface,
			r.URL.Query().Get("scopeMode"), r.URL.Query().Get("scopeClientId"))
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}

// chatAuth resolve a account (X-Account-Id / TenantID) + o Principal para as rotas do
// chat; escreve 403 no_account e devolve ok=false quando falta contexto. Centraliza o
// boilerplate dos handlers de conversa (todos precisam da account E do Principal).
func chatAuth(w http.ResponseWriter, r *http.Request) (string, auth.Principal, bool) {
	accountID, ok := accountScope(r)
	if !ok {
		writeNoAccount(w, r)
		return "", auth.Principal{}, false
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeNoAccount(w, r)
		return "", auth.Principal{}, false
	}
	return accountID, principal, true
}

// handleChatAsk recebe a pergunta do painel, persiste na conversa com memoria/escopo
// (contrato D4) e devolve { answer, conversationId, title }.
func handleChatAsk(svc *Service, defaultSurface string, assistantRoute bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		var req ChatAskRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		req.AssistantRuntime = assistantRoute
		req.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if strings.TrimSpace(req.Surface) == "" && strings.TrimSpace(req.ConversationID) == "" {
			req.Surface = defaultSurface
		}
		res, err := svc.ChatAsk(r.Context(), accountID, principal, req)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"answer":         res.Answer,
			"conversationId": res.ConversationID,
			"title":          res.Title,
			"surface":        res.Surface,
			"capabilities":   res.Capabilities,
			// message carrega as propostas (multi-tarefa, WAVE 5.1) em message.proposals.
			"message": res.Message,
			// aiError (WAVE 5): true quando o LLM nao respondeu — o front mostra "IA off".
			"aiError": res.AIError,
		})
	}
}

func handleUpdateChatProposal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		var req UpdateChatProposalRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		message, err := svc.UpdateChatProposal(r.Context(), accountID, r.PathValue("id"), r.PathValue("messageId"), r.PathValue("proposalId"), principal, req)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, message)
	}
}

// handleConfirmChatProposal e a unica porta de execucao para cards locais do
// assistente. A chave obrigatoria identifica a tentativa inclusive quando a
// resposta HTTP se perde; o store persiste o efeito e o receipt na mesma tx.
func handleConfirmChatProposal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		var req ConfirmChatProposalRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		result, err := svc.ConfirmChatProposal(
			r.Context(), accountID, r.PathValue("id"), r.PathValue("messageId"),
			r.PathValue("proposalId"), strings.TrimSpace(r.Header.Get("Idempotency-Key")),
			principal, req,
		)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

// handleListChatConversations lista as conversas visiveis (contrato D3): agencia todas,
// cliente-side so as suas. Resposta { conversations: [...] }.
func handleListChatConversations(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		items, err := svc.ListChatConversations(r.Context(), accountID, principal)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"conversations": items})
	}
}

// handleGetChatConversation devolve a conversa + mensagens (contrato D3). Dono ou agencia;
// fora disso 404.
func handleGetChatConversation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		res, err := svc.GetChatConversation(r.Context(), accountID, r.PathValue("id"), principal)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, res)
	}
}

// handleCreateChatConversation cria uma conversa vazia (contrato D3). Escopo normalizado
// server-side; devolve 201 com o resumo da conversa criada.
func handleCreateChatConversation(svc *Service, defaultSurface string, assistantRoute bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		var req CreateChatConversationRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		if !assistantRoute && strings.TrimSpace(req.Surface) != "" && strings.TrimSpace(req.Surface) != AssistantSurfaceCalendar {
			writeChatError(w, r, ErrAssistantSurfaceMismatch)
			return
		}
		if !assistantRoute {
			req.Surface = AssistantSurfaceCalendar
		} else if strings.TrimSpace(req.Surface) == "" {
			req.Surface = defaultSurface
		}
		res, err := svc.CreateChatConversation(r.Context(), accountID, principal, req)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, res)
	}
}

// handleDeleteChatConversation faz soft-delete da conversa (contrato D3). Dono ou agencia;
// fora disso 404. 204 No Content no sucesso.
func handleDeleteChatConversation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		if err := svc.DeleteChatConversation(r.Context(), accountID, r.PathValue("id"), principal); err != nil {
			writeChatError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleChatScope alimenta o SELECT de escopo do front (contrato D3): { canSelect,
// lockedClientId, clients }.
func handleChatScope(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		res, err := svc.ChatScope(r.Context(), accountID, principal)
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, res)
	}
}

// handleChatTranscribe recebe o audio (multipart campo file, max 15 MiB), valida o
// mime (whitelist C8), repassa ao webhook calendar-transcribe e devolve { text }.
// NADA e gravado em disco: o corpo e limitado por MaxBytesReader e mantido em RAM
// (maxMemory do parse = teto do audio, sem spill para arquivo temporario).
func handleChatTranscribe(svc *Service, defaultSurface string, assistantRoute bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, principal, ok := chatAuth(w, r)
		if !ok {
			return
		}
		surface, err := resolveTranscribeSurface(defaultSurface, assistantRoute, r.URL.Query().Get("surface"))
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		if !svc.transcribeConfigured() {
			writeChatError(w, r, ErrTranscribeNotConfigured)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, transcribeBodyBudget)
		if err := r.ParseMultipartForm(transcribeBodyBudget); err != nil { //nolint:gosec // G120: corpo ja limitado pelo MaxBytesReader acima
			writeTranscribeParseError(w, r, err)
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()
		file, header, err := r.FormFile("file")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Audio ausente (campo file).")
			return
		}
		defer func() { _ = file.Close() }()

		contentType := header.Header.Get("Content-Type")
		if !transcribeMimeAllowed(contentType) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media",
				"Formato de audio nao suportado (use webm, ogg, mp4, mpeg ou wav).")
			return
		}
		content, err := io.ReadAll(io.LimitReader(file, maxTranscribeBytes+1))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Falha ao ler o audio.")
			return
		}
		if int64(len(content)) > maxTranscribeBytes {
			httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "media_too_large",
				"Audio acima do limite de 15 MiB.")
			return
		}
		var text string
		if assistantRoute {
			text, err = svc.AssistantChatTranscribe(
				r.Context(), accountID, principal, surface, header.Filename, contentType, content,
			)
		} else {
			text, err = svc.ChatTranscribe(r.Context(), accountID, header.Filename, contentType, content)
		}
		if err != nil {
			writeChatError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"text": text})
	}
}

func resolveTranscribeSurface(defaultSurface string, assistantRoute bool, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if !assistantRoute {
		if requested != "" && requested != AssistantSurfaceCalendar {
			return "", ErrAssistantSurfaceMismatch
		}
		return AssistantSurfaceCalendar, nil
	}
	return normalizeAssistantSurface(firstNonEmpty(requested, defaultSurface))
}

// transcribeMimeAllowed compara o Content-Type do audio com a whitelist C8,
// ignorando parametros ("audio/webm;codecs=opus" -> "audio/webm").
func transcribeMimeAllowed(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return transcribeMimes[ct]
}

// writeTranscribeParseError distingue corpo acima do teto (413) de multipart
// malformado (400) no parse do upload. Cobre tanto o *http.MaxBytesError (Go 1.19+)
// quanto o texto legado "request body too large" por seguranca.
func writeTranscribeParseError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "media_too_large",
			"Audio acima do limite de 15 MiB.")
		return
	}
	httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_media", "Upload de audio invalido.")
}

// writeChatError mapeia os erros do chat/transcricao (contratos C7/C8) para HTTP.
// Erros compartilhados (data/mes invalido etc.) caem no writeServiceError padrao.
func writeChatError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrChatNotConfigured):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "chat_not_configured",
			"Chat do calendario nao configurado. Defina CALENDAR_CHAT_WEBHOOK_URL e importe o workflow calendar-chat no n8n.")
	case errors.Is(err, ErrTranscribeNotConfigured):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "transcribe_not_configured",
			"Transcricao de voz nao configurada. Defina CALENDAR_TRANSCRIBE_WEBHOOK_URL e importe o workflow calendar-transcribe no n8n.")
	case errors.Is(err, ErrInvalidQuestion):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_question",
			"Informe uma pergunta (ate 4000 caracteres).")
	case errors.Is(err, ErrIdempotencyKeyRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "idempotency_key_required",
			"Envie um Idempotency-Key ASCII de 8 a 200 caracteres.")
	case errors.Is(err, ErrIdempotencyConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "idempotency_conflict",
			"A chave de idempotencia ja foi usada com outro conteudo.")
	case errors.Is(err, ErrIdempotencyInProgress):
		httpapi.WriteError(w, r, http.StatusConflict, "idempotency_in_progress",
			"A solicitacao anterior ainda esta em andamento ou ficou incerta; ela nao sera repetida automaticamente.")
	case errors.Is(err, ErrProposalExecutionUnavailable):
		httpapi.WriteError(w, r, http.StatusConflict, "proposal_execution_unavailable",
			"Este card nao possui um executor transacional seguro e nao pode ser confirmado.")
	case errors.Is(err, ErrProposalSnapshotMissing):
		httpapi.WriteError(w, r, http.StatusConflict, "proposal_snapshot_missing",
			"O estado original deste card nao pode ser comprovado; gere uma nova proposta.")
	case errors.Is(err, ErrProposalCrossModuleEffect):
		httpapi.WriteError(w, r, http.StatusConflict, "proposal_cross_module_effect_unavailable",
			"Este evento esta vinculado a Tasks e nao pode ser alterado sem uma transacao entre os modulos.")
	case errors.Is(err, ErrProposalExecutionUnknown):
		httpapi.WriteError(w, r, http.StatusConflict, "proposal_execution_unknown",
			"O resultado da execucao ficou incerto. Consulte o mesmo card; nao tente executar novamente com outra chave.")
	case errors.Is(err, ErrInvalidProposalStatus):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_proposal_status",
			"Status da proposta invalido.")
	case errors.Is(err, ErrMetaActionNotSucceeded):
		httpapi.WriteError(w, r, http.StatusConflict, "meta_action_not_succeeded",
			"A acao Meta Ads precisa terminar com sucesso antes de o cartao ser marcado como aplicado.")
	case errors.Is(err, ErrInvalidAssistantSurface):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_surface",
			"Surface invalida; use calendar, meta_ads ou global.")
	case errors.Is(err, ErrAssistantSurfaceMismatch):
		httpapi.WriteError(w, r, http.StatusConflict, "surface_mismatch",
			"A conversa pertence a outra surface.")
	case errors.Is(err, ErrAssistantNoCapability):
		httpapi.WriteError(w, r, http.StatusForbidden, "assistant_forbidden",
			"Nenhum modulo autorizado para esta surface.")
	case errors.Is(err, ErrAssistantDisabled):
		httpapi.WriteError(w, r, http.StatusConflict, "assistant_disabled",
			"O assistente esta desativado nesta conta.")
	case errors.Is(err, ErrAssistantCredentialUnavailable):
		httpapi.WriteError(w, r, http.StatusConflict, "assistant_credential_unavailable",
			"Configure uma credencial valida para o assistente.")
	case errors.Is(err, ErrAssistantRuntimeUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "assistant_runtime_unavailable",
			"A configuracao compartilhada do assistente esta indisponivel.")
	case errors.Is(err, ErrInvalidClient):
		// Escopo/cliente fora do visivel do usuario (contrato D2): 404, nao 400/403 —
		// nao vaza QUAIS clientes existem (enumeration). Difere do writeServiceError
		// (onde invalid_client e 400 no contexto de events/plans).
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		httpapi.WriteError(w, r, http.StatusGatewayTimeout, "upstream_timeout",
			"A IA demorou para responder. Tente novamente.")
	case errors.Is(err, errChatUpstream):
		httpapi.WriteError(w, r, http.StatusBadGateway, "upstream_error",
			"Falha ao falar com o servico de IA (n8n). Verifique o workflow importado e ativo.")
	case errors.Is(err, ErrAIDisabled):
		writeAIDisabled(w, r)
	case errors.Is(err, ErrAIKeyMissing):
		writeAIKeyMissing(w, r)
	default:
		writeServiceError(w, r, err)
	}
}
