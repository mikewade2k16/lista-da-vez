package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// InboundService orquestra o webhook inbound PROVIDER-AGNOSTICO: resolve a conta pelo slug,
// autentica via provider.VerifyWebhook, traduz o payload em eventos canonicos
// (provider.ParseWebhook), deduplica pelo banco e persiste a mensagem. NAO conhece HTTP
// (o handler cuida das protecoes de transporte); NAO conhece o provider concreto (so a
// interface channel.Provider).
type InboundService struct {
	store     *Store
	registry  *channel.Registry
	secretBox *secretbox.Box
	publisher Publisher
	qr        *qrCache
	// ai (F9) e domain (F8) alimentam o auto-disparo da triagem pos-persistencia. nil (ex.:
	// testes) => auto-triagem desligada; a persistencia/realtime seguem intactas.
	ai                *AIService
	domain            *Service
	send              *SendService
	logger            *slog.Logger
	durableAIDispatch bool
}

// NewInboundService monta o service do webhook. publisher nil vira no-op (realtime e F5). O
// qrCache e COMPARTILHADO com o SessionService (module.go): o QR que a Evolution empurra por
// webhook (QRCODE_UPDATED) vai para o mesmo cache que o endpoint /qrcode le. nil = sem cache
// de QR por webhook (o evento vira ignored).
func NewInboundService(store *Store, registry *channel.Registry, box *secretbox.Box, publisher Publisher, qr *qrCache, ai *AIService, domain *Service, send *SendService, logger *slog.Logger) *InboundService {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &InboundService{store: store, registry: registry, secretBox: box, publisher: publisher, qr: qr, ai: ai, domain: domain, send: send, logger: logger}
}

// SetDurableAIDispatch enables the E2 PostgreSQL-backed dispatcher after the boot probe.
// It is deliberately opt-in so an older database keeps the legacy rollback path.
func (s *InboundService) SetDurableAIDispatch(enabled bool) {
	s.durableAIDispatch = enabled
}

// InboundStatus e o resultado agregado do processamento, mapeado a HTTP no handler.
type InboundStatus string

const (
	inboundAccepted  InboundStatus = "accepted"
	inboundDuplicate InboundStatus = "duplicate"
	inboundIgnored   InboundStatus = "ignored"
)

// ResolveAccount resolve o accountID do slug publico. slug inexistente / conta inativa /
// modulo desabilitado => ErrNotFound (404). Provider sem adapter => ErrNotFound tambem
// (nao revelar que o endpoint existe para um provider que nao servimos).
func (s *InboundService) ResolveAccount(ctx context.Context, providerKey, slug string) (string, error) {
	if !s.registry.Has(providerKey) {
		return "", ErrNotFound
	}
	accountID, err := s.store.ResolveWebhookAccount(ctx, strings.TrimSpace(slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return accountID, nil
}

// Verify autentica a requisicao via provider.VerifyWebhook, com a credencial resolvida por
// conta+provider (decifrada pelo secretbox). Falha => ErrUnauthorized (401). O erro do
// provider NUNCA carrega o body (canonico §10).
//
// ORDEM (deviacao documentada da C3): resolver a conta precede a verificacao, porque a
// credencial e POR INSTANCIA (D-A) — sem a conta nao ha o que comparar. E o mesmo padrao
// do site/http_ingest.go (resolve a source, depois valida a assinatura). O token global de
// env do legado permitia verificar antes; o modelo multi-provider por conta nao.
func (s *InboundService) Verify(ctx context.Context, accountID, providerKey string, hdr http.Header, body []byte) error {
	provider, err := s.registry.Get(providerKey)
	if err != nil {
		return ErrNotFound
	}
	cred, err := s.resolveCredentials(ctx, accountID, providerKey, body)
	if err != nil {
		return err
	}
	if err := provider.VerifyWebhook(hdr, body, cred); err != nil {
		// Nao propaga o erro do provider (pode conter detalhe de header): resposta generica.
		return ErrUnauthorized
	}
	return nil
}

// VerifyChallenge handles provider subscription handshakes without exposing
// credentials or letting a query parameter select a tenant. Providers that do
// not implement the optional challenge contract are treated as out of scope.
func (s *InboundService) VerifyChallenge(ctx context.Context, accountID, providerKey string, query map[string]string) (string, error) {
	provider, err := s.registry.Get(providerKey)
	if err != nil {
		return "", ErrNotFound
	}
	verifier, ok := provider.(channel.WebhookChallengeVerifier)
	if !ok {
		return "", ErrNotFound
	}
	cred, err := s.resolveCredentials(ctx, accountID, providerKey, nil)
	if err != nil {
		return "", err
	}
	challenge, err := verifier.VerifyWebhookChallenge(query, cred)
	if err != nil {
		return "", ErrUnauthorized
	}
	return challenge, nil
}

// Ingest traduz o payload e persiste os eventos. Assume conta ja resolvida e requisicao ja
// autenticada. Retorna o status agregado: se QUALQUER evento novo foi aceito => accepted;
// se todos ja existiam => duplicate; se nada era de dominio => ignored.
func (s *InboundService) Ingest(ctx context.Context, accountID, providerKey string, hdr http.Header, body []byte) (InboundStatus, error) {
	provider, err := s.registry.Get(providerKey)
	if err != nil {
		return inboundIgnored, ErrNotFound
	}
	events, err := provider.ParseWebhook(ctx, hdr, body)
	if err != nil {
		// ParseWebhook nunca embute o body no erro; ainda assim nao o repassamos ao cliente.
		return inboundIgnored, ErrInvalidBody
	}

	anyAccepted, anyDuplicate := false, false
	for i := range events {
		status, err := s.ingestOne(ctx, accountID, providerKey, body, events[i])
		if err != nil {
			return inboundIgnored, err
		}
		switch status {
		case inboundAccepted:
			anyAccepted = true
		case inboundDuplicate:
			anyDuplicate = true
		}
	}

	switch {
	case anyAccepted:
		return inboundAccepted, nil
	case anyDuplicate:
		return inboundDuplicate, nil
	default:
		return inboundIgnored, nil
	}
}

// ingestOne processa um evento canonico. Eventos sem instancia conhecida ou nao-de-dominio
// sao IGNORADOS (nunca auto-criam instancia — armadilha 1).
func (s *InboundService) ingestOne(ctx context.Context, accountID, providerKey string, body []byte, ev channel.Event) (InboundStatus, error) {
	if ev.Kind == channel.EventIgnored || ev.ExternalEventID == "" {
		return inboundIgnored, nil
	}

	// Eventos de SESSAO (QR/conexao) chegam por webhook async e NAO viram dominio: alimentam
	// o qrCache que o painel le em /qrcode. Sem isto, o QR que a Evolution empurra
	// (QRCODE_UPDATED) se perde e o pareamento nunca aparece. Cache e escopado por
	// (accountID ja validado, instanceName) — a mesma chave do SessionService.
	switch ev.Kind {
	case channel.EventQRUpdated:
		if s.qr != nil && ev.Session != nil && strings.TrimSpace(ev.Session.QRCode) != "" {
			s.qr.set(accountID, ev.InstanceName, ev.Session.QRCode)
			return inboundAccepted, nil
		}
		return inboundIgnored, nil
	case channel.EventSessionStatus:
		// Conectou => limpa o QR pendente (o mesmo que o SessionService faz ao parear).
		if s.qr != nil && ev.Session != nil && ev.Session.Connected {
			s.qr.set(accountID, ev.InstanceName, "")
		}
		return inboundIgnored, nil
	}

	if ev.Kind != channel.EventMessageReceived && ev.Kind != channel.EventMessageStatus {
		return inboundIgnored, nil
	}
	if ev.Kind == channel.EventMessageReceived && ev.Message == nil {
		return inboundIgnored, nil
	}
	if ev.Kind == channel.EventMessageStatus && ev.Status == nil {
		return inboundIgnored, nil
	}

	instanceID, found, err := s.store.FindInstanceIDByName(ctx, accountID, providerKey, ev.InstanceName)
	if err != nil {
		return inboundIgnored, err
	}
	if !found {
		// Instancia desconhecida: 202 ignored + audit. NAO auto-criar (input nao-confiavel).
		s.logger.Warn("omnichannel_webhook_unknown_instance",
			"account_id", accountID, "provider", providerKey, "instance", ev.InstanceName)
		return inboundIgnored, nil
	}

	if ev.Message != nil && isWhatsAppGroupExternalID(ev.Message.ContactExternalID) {
		if groupProvider, ok := providerForGroupMetadata(s.registry, providerKey); ok {
			metadataCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			cred, credErr := s.resolveCredentials(metadataCtx, accountID, providerKey, body)
			if credErr == nil {
				metadata, metadataErr := groupProvider.FetchGroupMetadata(
					metadataCtx, cred, ev.InstanceName, ev.Message.ContactExternalID,
				)
				if metadataErr == nil && strings.TrimSpace(metadata.Name) != "" {
					ev.Message.ContactName = strings.TrimSpace(metadata.Name)
				} else if metadataErr != nil {
					s.logger.Debug("omnichannel_group_metadata_unavailable", "account_id", accountID, "provider", providerKey, "instance", ev.InstanceName)
				}
			}
			cancel()
		}
	}

	write := inboundWrite{
		AccountID:       accountID,
		Provider:        providerKey,
		ExternalEventID: ev.ExternalEventID,
		EventKind:       string(ev.Kind),
		InstanceName:    ev.InstanceName,
		InstanceID:      instanceID,
		PayloadMasked:   maskEvent(providerKey, ev),
	}
	if ev.Message != nil {
		write.Message = &inboundMessageWrite{
			ExternalMessageID:    ev.Message.ExternalMessageID,
			Channel:              normalizeChannel(ev.Message.Channel),
			ContactExternalID:    firstNonEmpty(ev.Message.ContactExternalID, ev.Message.ContactPhone),
			ContactPhone:         ev.Message.ContactPhone,
			ContactName:          ev.Message.ContactName,
			ContactAvatarURL:     ev.Message.ContactAvatarURL,
			MessageType:          normalizeMessageType(ev.Message.MessageType),
			Content:              ev.Message.Content,
			MediaURL:             ev.Message.MediaURL,
			MediaMimeType:        ev.Message.MediaMimeType,
			MediaFileName:        ev.Message.MediaFileName,
			MediaCaption:         ev.Message.MediaCaption,
			OccurredAt:           ev.OccurredAt,
			FromMe:               ev.Message.FromMe,
			Reply:                ev.Message.Reply,
			SocialEventKind:      ev.Message.SocialEventKind,
			SocialContentID:      ev.Message.SocialContentID,
			SocialMediaID:        ev.Message.SocialMediaID,
			SocialParentID:       ev.Message.SocialParentID,
			SocialIsLive:         ev.Message.SocialIsLive,
			SocialReplyExpiresAt: ev.Message.SocialReplyExpiresAt,
		}
	}
	if ev.Status != nil {
		write.Status = &inboundStatusWrite{
			ExternalMessageID: ev.Status.ExternalMessageID,
			Status:            ev.Status.Status,
			ErrorCode:         ev.Status.ErrorCode,
			OccurredAt:        ev.OccurredAt,
		}
	}

	var res inboundResult
	if write.Message != nil && write.Message.FromMe && s.domain != nil {
		res, err = s.store.PersistInboundWithTransition(ctx, write, func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
			return s.domain.decideTransition(ctx, accountID, EventMsgOutboundHuman, TransitionPayload{}, snap)
		})
	} else {
		res, err = s.store.PersistInbound(ctx, write)
	}
	if err != nil {
		return inboundIgnored, err
	}
	if res.Duplicate {
		return inboundDuplicate, nil
	}
	if write.Status != nil {
		if res.StatusChanged && res.MessageID != "" {
			payload := minimalUpdatePayload(res.MessageID, res.ProviderStatus, "", res.MessageID)
			payload["updatedAt"] = res.ProviderStatusAt.UTC().Format(time.RFC3339)
			if res.ProviderErrorCode != "" {
				payload["providerErrorCode"] = res.ProviderErrorCode
			}
			s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
				Type:       RealtimeEventMessageUpdated,
				AccountID:  accountID,
				ResourceID: res.MessageID,
				Payload:    payload,
			})
		}
		return inboundAccepted, nil
	}
	if !res.MessageCreated && res.StatusChanged && res.MessageID != "" {
		payload := minimalUpdatePayload(res.MessageID, res.ProviderStatus, "", res.MessageID)
		payload["updatedAt"] = res.ProviderStatusAt.UTC().Format(time.RFC3339)
		if res.ProviderErrorCode != "" {
			payload["providerErrorCode"] = res.ProviderErrorCode
		}
		s.publisher.PublishOmnichannelEvent(ctx, RealtimeEvent{
			Type: RealtimeEventMessageUpdated, AccountID: accountID,
			ResourceID: res.MessageID, Payload: payload,
		})
	}
	// fromMe deduplicado devolve MessageID vazio (a plataforma ja registrou esse envio, ou
	// re-entrega): nao republica nem dispara IA.
	if !res.MessageCreated {
		return inboundDuplicate, nil
	}
	// Realtime (F5): fora da transacao (persiste -> commita -> publica). O call-site monta o
	// subconjunto `message.created` do webhook com o id INTERNO persistido; a midia data: e
	// sanitizada (nunca base64 no WS). Publisher no-op quando o canal esta desligado. Vale tambem
	// para o fromMe (OUTBOUND): o painel mostra ao vivo a mensagem enviada pelo aparelho.
	s.publishInboundMessage(ctx, accountID, res, write.Message)
	// Auto-disparo da IA (F5->F9->F8): a triagem roda SOZINHA sobre a mensagem recem-persistida.
	// FIRE-AND-FORGET — a mensagem ja esta commitada e o realtime ja saiu; a IA e best-effort por
	// cima e NUNCA falha o webhook (goroutine com contexto proprio, erro engolido). SO para
	// mensagem do CLIENTE (inbound): fromMe = nosso proprio envio, nunca dispara triagem.
	if !ev.Message.FromMe {
		s.maybeAutoTriage(accountID, res.ConversationID, res.MessageID)
	}
	return inboundAccepted, nil
}

func providerForGroupMetadata(registry *channel.Registry, providerKey string) (channel.GroupMetadataProvider, bool) {
	provider, err := registry.Get(providerKey)
	if err != nil {
		return nil, false
	}
	groupProvider, ok := provider.(channel.GroupMetadataProvider)
	return groupProvider, ok
}

// autoTriageTimeout limita a triagem em background: o LLM pode demorar, mas nao pode segurar
// recursos indefinidamente. A mensagem JA foi persistida/emitida — isto e best-effort por cima.
const autoTriageTimeout = 90 * time.Second

// maybeAutoTriage agenda a triagem da IA (F9) + roteamento (F8) da mensagem inbound recem-
// persistida em background. Sem ai/domain injetados (testes) ou sem ids => no-op. A goroutine
// tem contexto PROPRIO: o ctx do webhook morre ao responder, e a IA nao pode segurar o handler.
func (s *InboundService) maybeAutoTriage(accountID, convID, messageID string) {
	if s.ai == nil || s.domain == nil ||
		strings.TrimSpace(convID) == "" || strings.TrimSpace(messageID) == "" {
		return
	}
	if s.durableAIDispatch && s.store != nil && s.store.AIDispatchV2Enabled() {
		go s.scheduleAIDispatch(accountID, convID, messageID)
		return
	}
	go s.runAutoTriage(accountID, convID, messageID)
}

func (s *InboundService) scheduleAIDispatch(accountID, convID, messageID string) {
	defer func() {
		if recover() != nil {
			s.logger.Error("omnichannel_ai_dispatch_schedule_panic", "account_id", accountID, "conversation_id", convID)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), autoTriageTimeout)
	defer cancel()
	conv, err := s.store.ConvTriageContext(ctx, accountID, convID)
	if err != nil || !conv.Found {
		if err != nil {
			s.logger.Error("omnichannel_ai_dispatch_context_failed", "account_id", accountID, "conversation_id", convID)
		}
		return
	}

	state, err := s.domain.SystemTransition(ctx, accountID, convID, EventMsgInbound, TransitionPayload{})
	if err != nil {
		s.logger.Error("omnichannel_ai_dispatch_transition_failed", "account_id", accountID, "conversation_id", convID)
		return
	}
	if state != StateAIActive {
		if state == StateRouting {
			if _, err := s.domain.SystemRoute(ctx, accountID, convID); err != nil {
				s.logger.Error("omnichannel_ai_dispatch_route_failed", "account_id", accountID, "conversation_id", convID)
			}
		}
		return
	}
	if isWhatsAppGroupExternalID(conv.ExternalID) {
		if _, err := s.domain.SystemTransition(ctx, accountID, convID, EventAITriageFailed, TransitionPayload{}); err == nil {
			_, _ = s.domain.SystemRoute(ctx, accountID, convID)
		}
		return
	}
	agent, ok, err := s.store.ActiveAgentForInstance(ctx, accountID, deref(conv.InstanceID))
	if err != nil {
		s.logger.Error("omnichannel_ai_dispatch_agent_failed", "account_id", accountID)
		return
	}
	if !ok || agent.ActiveVersionID == nil || strings.TrimSpace(*agent.ActiveVersionID) == "" {
		if _, err := s.domain.SystemTransition(ctx, accountID, convID, EventAITriageFailed, TransitionPayload{}); err == nil {
			_, _ = s.domain.SystemRoute(ctx, accountID, convID)
		}
		return
	}
	debounce := 2500
	if cfg, cfgErr := s.store.AIDispatchConfig(ctx, accountID, *agent.ActiveVersionID); cfgErr == nil && cfg.DebounceMS > 0 {
		debounce = cfg.DebounceMS
	}
	runAfter := time.Now().UTC().Add(time.Duration(debounce) * time.Millisecond)
	if _, err := s.store.UpsertAIDispatch(ctx, accountID, convID, *agent.ActiveVersionID, messageID, runAfter); err != nil {
		if !errors.Is(err, ErrAILeaseInvalid) && !errors.Is(err, ErrNotFound) {
			s.logger.Error("omnichannel_ai_dispatch_enqueue_failed", "account_id", accountID, "conversation_id", convID)
		}
	}
}

// runAutoTriage conduz o ciclo de vida canonico (F8) convidando a IA (F9):
//
//	msg.inbound  -> new/closed vira ai_active (ha agente ativo) OU routing (sem agente); demais
//	                estados ficam onde estao (self: human_active/pending/queued nao re-disparam).
//	ai_active    -> Dispatch (a IA SUGERE, funde extracted_fields) -> ai.triage.done|failed -> routing.
//	routing      -> SystemRoute (o motor DETERMINISTICO decide a fila lendo routing_rules) -> queued.
//
// Todo erro e ENGOLIDO (so log, sem PII): a persistencia/realtime ja aconteceram e a conversa
// nunca trava por falha da IA/infra. O recover blinda o processo de um panic no caminho de TODA
// mensagem recebida. A IA nunca escreve queue_id — quem roteia e o motor.
func (s *InboundService) runAutoTriage(accountID, convID, messageID string) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("omnichannel_auto_triage_panic", "account_id", accountID, "conversation_id", convID)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), autoTriageTimeout)
	defer cancel()

	// 1. msg.inbound avanca a maquina (transitionContextFor resolve HasActiveAgent do banco).
	state, err := s.domain.SystemTransition(ctx, accountID, convID, EventMsgInbound, TransitionPayload{})
	if err != nil {
		s.logger.Error("omnichannel_auto_triage_inbound_failed", "account_id", accountID, "conversation_id", convID)
		return
	}

	// 2. So `ai_active` convida a IA. Qualquer desfecho degradado sai de ai_active FAIL-OPEN para
	//    routing — a conversa nunca fica presa esperando a IA (canonico: fail-open, motor decide).
	if state == StateAIActive {
		outcome := dispatchProviderError
		var dispatch DispatchResult
		res, derr := s.ai.Dispatch(ctx, TriageInput{AccountID: accountID, ConversationID: convID, MessageID: messageID})
		if derr != nil {
			s.logger.Error("omnichannel_auto_triage_dispatch_failed", "account_id", accountID, "conversation_id", convID)
		} else {
			dispatch = res
			outcome = res.Outcome
		}
		s.logger.Info("omnichannel_auto_triage_ran", "account_id", accountID, "conversation_id", convID, "outcome", string(outcome))

		// Saida multi-turno: a IA responde PELO OUTBOX DO GO e permanece em ai_active para
		// ouvir a proxima mensagem. needs_human encerra a etapa da IA e segue ao roteamento;
		// falha de envio tambem faz fail-open para um humano, nunca deixa o cliente no limbo.
		if outcome == dispatchBlocked {
			return
		}
		if outcome == dispatchNoReply {
			return
		}
		if outcome == dispatchTriaged && strings.TrimSpace(dispatch.Output.ReplyDraft) != "" && s.send != nil {
			moderated, moderationErr := s.store.IsInstagramModeratedMessage(ctx, accountID, messageID)
			if moderationErr != nil && !errors.Is(moderationErr, ErrNotFound) {
				s.logger.Error("omnichannel_instagram_moderation_lookup_failed", "account_id", accountID)
			}
			if moderated {
				// Instagram comments/mentions are suggestions only. The human
				// moderation action is the sole path that can publish a reply.
				if err := s.store.SaveInstagramAIDraft(ctx, accountID, messageID, dispatch.Output.ReplyDraft); err != nil {
					s.logger.Error("omnichannel_instagram_draft_save_failed", "account_id", accountID)
				}
			} else if sendErr := s.send.SendAIMessage(ctx, accountID, convID, dispatch.Output.ReplyDraft, dispatch.RunID, messageID, dispatch.AIGeneration); sendErr != nil {
				if errors.Is(sendErr, ErrAILeaseInvalid) {
					return
				}
				outcome = dispatchProviderError
				s.logger.Error("omnichannel_auto_reply_enqueue_failed", "account_id", accountID, "conversation_id", convID)
			} else if !dispatch.Output.NeedsHuman {
				s.logger.Info("omnichannel_auto_reply_queued", "account_id", accountID, "conversation_id", convID)
				return
			}
		}
		state, err = s.domain.SystemTransition(ctx, accountID, convID, triageEventFor(outcome), TransitionPayload{})
		if err != nil {
			s.logger.Error("omnichannel_auto_triage_done_failed", "account_id", accountID, "conversation_id", convID)
			return
		}
	}

	// 3. So `routing` chama o motor (route.* fora de routing => 409). Resultado: queued.
	if state == StateRouting {
		if _, err := s.domain.SystemRoute(ctx, accountID, convID); err != nil {
			s.logger.Error("omnichannel_auto_triage_route_failed", "account_id", accountID, "conversation_id", convID)
		}
	}
}

// triageEventFor mapeia o desfecho do Dispatch para o evento que TIRA a conversa de ai_active.
// So `triaged` (LLM rodou, saida valida) e sucesso; todo desfecho degradado (no_agent/
// provider_error/schema_invalid/limit_exceeded/blocked) falha OPEN para routing — a IA nunca
// aprisiona a conversa: o motor deterministico da F8 ainda decide a fila.
func triageEventFor(outcome DispatchOutcome) Event {
	if outcome == dispatchTriaged {
		return EventAITriageDone
	}
	return EventAITriageFailed
}

// resolveCredentials monta as Credentials de (conta, provider): pega a instancia da conta
// com credentials_ciphertext preenchido e decifra pelo secretbox. Sem ciphertext (ex.:
// mock) => Credentials vazio. A chave crua NUNCA vai a log nem volta ao front.
func (s *InboundService) resolveCredentials(ctx context.Context, accountID, providerKey string, body []byte) (channel.Credentials, error) {
	instanceKey := ""
	if provider, err := s.registry.Get(providerKey); err == nil {
		if resolver, ok := provider.(channel.WebhookInstanceResolver); ok && len(body) > 0 {
			instanceKey = resolver.WebhookInstanceKey(body)
		}
	}
	cipherText, config, found, err := s.store.FindProviderCredentialForKey(ctx, accountID, providerKey, instanceKey)
	if err != nil {
		return channel.Credentials{}, err
	}
	cred := channel.Credentials{Config: config}
	if !found || strings.TrimSpace(cipherText) == "" || s.secretBox == nil {
		return cred, nil
	}
	token, err := s.secretBox.Decrypt(cipherText)
	if err != nil {
		// Falha ao decifrar e erro operacional (chave errada/dado adulterado). NUNCA
		// vaza o ciphertext nem a chave; loga so o identificador da conta.
		s.logger.Error("omnichannel_credential_decrypt_failed", "account_id", accountID, "provider", providerKey)
		return channel.Credentials{}, err
	}
	cred.Token = token
	return cred, nil
}

// maskEvent produz o payload_masked (triagem): telefone -> ultimos 4, corpo OMITIDO. Nunca
// o body cru (canonico §10).
func maskEvent(providerKey string, ev channel.Event) json.RawMessage {
	masked := map[string]any{
		"provider": providerKey,
		"instance": ev.InstanceName,
		"kind":     string(ev.Kind),
	}
	if ev.Message != nil {
		masked["externalMessageId"] = ev.Message.ExternalMessageID
		masked["messageType"] = ev.Message.MessageType
		masked["contactPhoneLast4"] = last4(ev.Message.ContactPhone)
	}
	raw, err := json.Marshal(masked)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func last4(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 4 {
		return phone
	}
	return phone[len(phone)-4:]
}

func normalizeChannel(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if c == "INSTAGRAM" {
		return "INSTAGRAM"
	}
	return "WHATSAPP"
}

func normalizeMessageType(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "IMAGE", "AUDIO", "VIDEO", "DOCUMENT":
		return strings.ToUpper(strings.TrimSpace(t))
	default:
		return "TEXT"
	}
}
