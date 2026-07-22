package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// ============================================================================
// F7 — Acoes do inbox (spec OMNI-F7). Conversas (status/assign/open) + o nucleo
// compartilhado (escopo, realtime, auditoria). As acoes de MENSAGEM estao em
// service_actions_messages.go.
// ============================================================================
//
// PRINCIPIO CENTRAL (risco 4 do canonico): status/assign NUNCA escrevem state/status na mao —
// mapeiam a requisicao para um EVENTO e chamam Service.Transition (F8), que roda sob lock. O
// escopo de cada rota soma DOIS gates com AND (spec C3): instancia (F7 corrigido,
// AccessibleScopeKeys) + fila (F8, GetVisibleConversation). Fora de qualquer um => 404.

var (
	// ErrActionUnsupported: o numero/provider nao suporta a acao (Capabilities) ou a conversa
	// nao tem instancia/provider resolvido. 409 acionavel — o botao nao pode mentir.
	ErrActionUnsupported = errors.New("omnichannel: action not supported by number")
	// ErrMessageNotSent: mensagem sem external_message_id (nunca saiu pelo provider). 409
	// (precedente do legado: reagir/apagar exige o id do provedor).
	ErrMessageNotSent = errors.New("omnichannel: message has no external id")
	// ErrProviderActionUnavailable: a operacao SINCRONA (group/sync/import) existe na spec mas o
	// channel.Provider da F4 ainda nao expoe o metodo (o adapter nao implementa). 409 acionavel
	// ("indisponivel para este provider no momento"), nunca sucesso fingido nem 500. reaction e
	// delete-for-all JA ligaram no provider (SendReaction/DeleteForAll); as demais sync ops
	// seguem aqui ate a F4 estender a interface (ver AGENT.md §Wiring pendente F7).
	ErrProviderActionUnavailable = errors.New("omnichannel: provider action not available")
	// ErrProviderUnavailable: a acao SINCRONA foi ao provider e ele FALHOU (transporte ou HTTP
	// non-2xx). Mapeia 502 (o legado responde 502 na falha da reaction). O erro NUNCA carrega o
	// corpo/chave do provider (canonico §10) — so sinaliza a falha ao caller HTTP.
	ErrProviderUnavailable = errors.New("omnichannel: provider call failed")
)

// capability enumera as capacidades checadas por numero antes de uma acao sincrona.
type capability int

const capReaction capability = iota

// ActionsService orquestra as acoes do inbox (F7). Reusa o Service de leitura/FSM (svc:
// Transition, requirePermission, resolveVisibility) e o SendService (send: o forward reusa o
// outbox da F6). O registry serve o gate de Capabilities por numero. account_id vem SEMPRE do
// Principal.
type ActionsService struct {
	store     *Store
	svc       *Service
	send      *SendService
	registry  *channel.Registry
	publisher Publisher
	logger    *slog.Logger
	// secretBox decifra a credencial por instancia nas acoes SINCRONAS (reaction/delete-for-all).
	// Opcional (dependencia injetada pos-construcao via WithActionsSecretBox para nao quebrar os
	// call-sites existentes): nil => a credencial cai no provider_config + fallback de ambiente do
	// adapter (ver resolveActionCredentials em service_actions_messages.go).
	secretBox *secretbox.Box
}

// ActionsOption configura dependencias OPCIONAIS do ActionsService sem alterar a assinatura
// posicional de NewActionsService (mantem os call-sites existentes compilando — o wiring novo
// entra por aqui).
type ActionsOption func(*ActionsService)

// WithActionsSecretBox injeta o secretbox usado para decifrar a credencial por instancia nas
// acoes sincronas de mensagem. Sem ele, reaction/delete-for-all usam provider_config + o
// fallback de ambiente (EVOLUTION_*) do adapter.
func WithActionsSecretBox(box *secretbox.Box) ActionsOption {
	return func(a *ActionsService) { a.secretBox = box }
}

// NewActionsService monta o service. publisher nil => no-op; registry obrigatorio (gate de
// capability). O svc/send sao os mesmos ja construidos no Build (reuso, nao recriacao). opts
// injeta dependencias opcionais (ex.: WithActionsSecretBox).
func NewActionsService(store *Store, svc *Service, send *SendService, registry *channel.Registry, publisher Publisher, logger *slog.Logger, opts ...ActionsOption) *ActionsService {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	a := &ActionsService{store: store, svc: svc, send: send, registry: registry, publisher: publisher, logger: logger}
	for _, opt := range opts {
		if opt != nil {
			opt(a)
		}
	}
	return a
}

// ============================================================================
// Status (PATCH .../conversations/{id}/status) — via maquina de estados
// ============================================================================

// SetStatus mapeia o status do front (OPEN|PENDING|CLOSED) para o evento da FSM e transiciona
// (Contrato 3 da F8). OPEN em conversa ja aberta = no-op 200; PENDING = human.pending (D-E);
// CLOSED = conv.close. Emite conversation.updated COM instanceName e audita se mudou de fato.
func (a *ActionsService) SetStatus(ctx context.Context, accountID string, p auth.Principal, convID, status string) (ConversationView, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.close"); err != nil {
		return ConversationView{}, err
	}
	row, err := a.resolveConversation(ctx, accountID, p, convID)
	if err != nil {
		return ConversationView{}, err
	}
	before, err := conversationView(row)
	if err != nil {
		return ConversationView{}, err
	}
	ev, noOp, err := statusEvent(status, before.Status)
	if err != nil {
		return ConversationView{}, err
	}
	after := before
	if !noOp {
		after, err = a.svc.Transition(ctx, p, convID, ev, TransitionPayload{})
		if err != nil {
			return ConversationView{}, err
		}
	}
	if before.Status != after.Status {
		a.auditConversation(ctx, accountID, p.UserID, convID, "CONVERSATION_STATUS_CHANGED",
			string(before.Status), string(after.Status))
	}
	a.publishConversationUpdated(ctx, accountID, after)
	return after, nil
}

// statusEvent projeta o status do front no evento da FSM (F8 Contrato 3). Status desconhecido
// => ErrInvalidBody (400). OPEN so vira conv.reopen quando a conversa esta CLOSED; nos demais
// (ja aberta/pendente) e no-op — nunca lemos `state` para decidir destino (usamos a projecao).
func statusEvent(status string, current ConversationStatus) (Event, bool, error) {
	switch ConversationStatus(strings.ToUpper(strings.TrimSpace(status))) {
	case StatusClosed:
		return EventConvClose, false, nil
	case StatusPending:
		return EventHumanPending, false, nil
	case StatusOpen:
		if current == StatusClosed {
			return EventConvReopen, false, nil
		}
		return "", true, nil // ja aberta/pendente: no-op 200 (regra de ouro do port)
	default:
		return "", false, ErrInvalidBody
	}
}

// ============================================================================
// Assign (PATCH .../conversations/{id}/assign) — via maquina de estados
// ============================================================================

// Assign atribui/desatribui via FSM: assignedToId com valor => human.assign (=> human_active
// => hard-block da IA); null => human.unassign. O destino e validado pela guarda da nota 8 na
// F8 (fora da conta/nao-atribuivel => 404). Reconcilia assigned_to_id (coluna que o front le)
// com assigned_user_id (autoritativo da FSM), emite conversation.updated e audita se mudou.
func (a *ActionsService) Assign(ctx context.Context, accountID string, p auth.Principal, convID string, assignedToID *string) (ConversationView, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.assign"); err != nil {
		return ConversationView{}, err
	}
	row, err := a.resolveConversation(ctx, accountID, p, convID)
	if err != nil {
		return ConversationView{}, err
	}
	before, err := conversationView(row)
	if err != nil {
		return ConversationView{}, err
	}
	target := ""
	if assignedToID != nil {
		target = strings.TrimSpace(*assignedToID)
	}
	ev, payload := EventHumanUnassign, TransitionPayload{}
	if target != "" {
		ev, payload = EventHumanAssign, TransitionPayload{TargetUserID: target}
	}
	if _, err := a.svc.Transition(ctx, p, convID, ev, payload); err != nil {
		return ConversationView{}, err
	}
	// Reconcilia a coluna legada de exibicao (migration 0200: "quem reconcilia e a F7/F8").
	if err := a.store.SyncAssignedToID(ctx, accountID, convID); err != nil {
		return ConversationView{}, err
	}
	finalRow, err := a.store.GetConversation(ctx, accountID, convID)
	if err != nil {
		return ConversationView{}, translate(err)
	}
	after, err := conversationView(finalRow)
	if err != nil {
		return ConversationView{}, err
	}
	if !sameAssignee(before.AssignedToID, after.AssignedToID) {
		a.auditConversation(ctx, accountID, p.UserID, convID, "CONVERSATION_ASSIGNED",
			before.AssignedToID, after.AssignedToID)
	}
	a.publishConversationUpdated(ctx, accountID, after)
	return after, nil
}

// TakeConversation é a operação explícita de handoff do E5. Diferente do PATCH
// de atribuição (que permite reatribuir), take aceita somente a primeira pessoa
// concorrente; o Store mantém o lock da conversa até cancelar a IA e gravar o
// aceite do handoff.
func (a *ActionsService) TakeConversation(ctx context.Context, accountID string, p auth.Principal, convID, idempotencyKey string) (ConversationView, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.assign"); err != nil {
		return ConversationView{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len([]rune(idempotencyKey)) > 128 {
		return ConversationView{}, ErrInvalidBody
	}
	if _, err := a.resolveConversation(ctx, accountID, p, convID); err != nil {
		return ConversationView{}, err
	}
	allowUnscoped, err := a.svc.hasPermission(ctx, accountID, p, "omnichannel.settings.manage")
	if err != nil {
		return ConversationView{}, err
	}
	row, err := a.store.TakeConversation(ctx, accountID, convID, p.UserID, allowUnscoped)
	if err != nil {
		return ConversationView{}, translate(err)
	}
	if err := a.store.SyncAssignedToID(ctx, accountID, convID); err != nil {
		return ConversationView{}, err
	}
	view, err := conversationView(row)
	if err != nil {
		return ConversationView{}, err
	}
	a.publishConversationUpdated(ctx, accountID, view)
	return view, nil
}

func (a *ActionsService) RequestHandoff(ctx context.Context, accountID string, p auth.Principal, convID string, in HandoffRequest) (HandoffView, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.assign"); err != nil {
		return HandoffView{}, err
	}
	if err := normalizeHandoffRequest(&in); err != nil {
		return HandoffView{}, err
	}
	if _, err := a.resolveConversation(ctx, accountID, p, convID); err != nil {
		return HandoffView{}, err
	}
	if in.TargetQueueID != nil {
		if err := a.svc.assertActiveQueue(ctx, accountID, *in.TargetQueueID); err != nil {
			return HandoffView{}, translate(err)
		}
	}
	return a.store.CreateHandoff(ctx, accountID, convID, p.UserID, in)
}

func (a *ActionsService) ReleaseConversation(ctx context.Context, accountID string, p auth.Principal, convID string) (ConversationView, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.assign"); err != nil {
		return ConversationView{}, err
	}
	return a.Assign(ctx, accountID, p, convID, nil)
}

// ============================================================================
// Contatos: abrir conversa (POST .../contacts/{id}/open-conversation)
// ============================================================================

// OpenContactConversation devolve a conversa do contato, criando uma se ainda nao existir
// (find-or-create pela chave natural). Contato de outra conta => 404. Requer contacts.manage.
func (a *ActionsService) OpenContactConversation(ctx context.Context, accountID string, p auth.Principal, contactID string) (ConversationView, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.contacts.manage"); err != nil {
		return ConversationView{}, err
	}
	contact, err := a.store.GetContact(ctx, accountID, contactID)
	if err != nil {
		return ConversationView{}, translate(err)
	}
	if convID, found, err := a.store.FindContactConversationID(ctx, accountID, contactID); err != nil {
		return ConversationView{}, err
	} else if found {
		row, gerr := a.store.GetConversation(ctx, accountID, convID)
		if gerr != nil {
			return ConversationView{}, translate(gerr)
		}
		return conversationView(row)
	}
	row, err := a.store.CreateContactConversation(ctx, accountID, contactID, contact.Phone, contact.Name, contact.AvatarURL)
	if err != nil {
		return ConversationView{}, err
	}
	view, err := conversationView(row)
	if err != nil {
		return ConversationView{}, err
	}
	a.publishConversationUpdated(ctx, accountID, view)
	return view, nil
}

// ============================================================================
// Nucleo compartilhado: escopo, capability, realtime, auditoria
// ============================================================================

// resolveConversation soma os DOIS gates com AND (spec C3): fila (F8, GetVisibleConversation)
// E instancia (F7 corrigido, AccessibleScopeKeys). Fora de qualquer um => ErrNotFound (404,
// nunca 403 — enumeration). O mais restritivo vence; unir com OR reabriria o furo.
func (a *ActionsService) resolveConversation(ctx context.Context, accountID string, p auth.Principal, convID string) (conversationRow, error) {
	vis, err := a.svc.resolveVisibility(ctx, accountID, p)
	if err != nil {
		return conversationRow{}, err
	}
	row, err := a.store.GetVisibleConversation(ctx, accountID, vis, convID)
	if err != nil {
		return conversationRow{}, translate(err)
	}
	keys, unrestricted, err := a.store.AccessibleScopeKeys(ctx, accountID, p.UserID, isAdminPrincipal(p))
	if err != nil {
		return conversationRow{}, err
	}
	if !unrestricted && !containsString(keys, row.InstanceScopeKey) {
		return conversationRow{}, ErrNotFound
	}
	return row, nil
}

// assertProviderCapability gateia uma acao sincrona por NUMERO (spec C4): sem provider
// resolvido ou capability ausente => ErrActionUnsupported (409). A UI degrada por numero em
// vez de mentir que todo numero faz tudo.
func (a *ActionsService) assertProviderCapability(providerKey string, cap capability) error {
	prov, err := a.resolveProvider(providerKey)
	if err != nil {
		return err
	}
	caps := prov.Capabilities()
	if cap == capReaction && !caps.SupportsReaction {
		return ErrActionUnsupported
	}
	return nil
}

// resolveProvider resolve o adapter do provider da conversa. Provider vazio (sem instancia) ou
// sem adapter registrado => ErrActionUnsupported (409 acionavel).
func (a *ActionsService) resolveProvider(providerKey string) (channel.Provider, error) {
	if strings.TrimSpace(providerKey) == "" {
		return nil, ErrActionUnsupported
	}
	prov, err := a.registry.Get(providerKey)
	if err != nil {
		return nil, ErrActionUnsupported
	}
	return prov, nil
}

// publishConversationUpdated emite conversation.updated no canal da conta, COM instanceName
// (shape de mapConversation — status/assign/contacts trazem a instancia, spec F5). mediaUrl do
// preview com data: vira null (nunca base64 no WS).
func (a *ActionsService) publishConversationUpdated(ctx context.Context, accountID string, view ConversationView) {
	a.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
		Type:       RealtimeEventConversationUpdated,
		AccountID:  accountID,
		ResourceID: view.ID,
		Payload:    conversationUpdatedPayload(view),
	})
}

// conversationUpdatedPayload serializa a ConversationView (inclui instanceName) e sanitiza o
// mediaUrl aninhado em lastMessage.
func conversationUpdatedPayload(view ConversationView) map[string]any {
	raw, _ := json.Marshal(view)
	payload := map[string]any{}
	_ = json.Unmarshal(raw, &payload)
	if lm, ok := payload["lastMessage"].(map[string]any); ok {
		if mu, ok := lm["mediaUrl"].(string); ok {
			if clean := sanitizeMediaURLForRealtime(mu); clean == "" {
				lm["mediaUrl"] = nil
			} else {
				lm["mediaUrl"] = clean
			}
		}
	}
	return payload
}

// auditConversation grava CONVERSATION_STATUS_CHANGED|ASSIGNED com {before, after, changedBy}
// (igual ao legado). Best-effort: falha de auditoria nao derruba a acao. before/after aceitam
// *string (assign) ou string (status) — nil serializa como null no payload.
func (a *ActionsService) auditConversation(ctx context.Context, accountID, actorUserID, convID, eventType string, before, after any) {
	payload, _ := json.Marshal(map[string]any{"before": before, "after": after, "changedBy": actorUserID})
	if err := a.store.InsertAudit(ctx, accountID, actorUserID, convID, "", eventType, payload); err != nil {
		a.logger.Error("omnichannel_action_audit", "account_id", accountID, "event", eventType, "error", err.Error())
	}
}

// isAdminPrincipal responde se o Principal e admin da conta (papeis administrativos do Omni),
// o que torna o escopo de instancia irrestrito (spec C3).
func isAdminPrincipal(p auth.Principal) bool {
	return legacyRole(p.Role) == legacyRoleAdmin
}

// sameAssignee compara dois responsaveis (*string): ambos nil = igual; um nil = mudou.
func sameAssignee(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// containsString responde se v esta em list.
func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
