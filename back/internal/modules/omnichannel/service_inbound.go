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
	// domain owns the FSM used by provider-device takeover. enqueueAutomation
	// means the durable worker path is available; nil domain in narrow unit
	// tests keeps persistence and realtime independent from automation.
	domain            *Service
	enqueueAutomation bool
	logger            *slog.Logger
}

// NewInboundService monta o service do webhook. publisher nil vira no-op (realtime e F5). O
// qrCache e COMPARTILHADO com o SessionService (module.go): o QR que a Evolution empurra por
// webhook (QRCODE_UPDATED) vai para o mesmo cache que o endpoint /qrcode le. nil = sem cache
// de QR por webhook (o evento vira ignored).
func NewInboundService(store *Store, registry *channel.Registry, box *secretbox.Box, publisher Publisher, qr *qrCache, domain *Service, logger *slog.Logger) *InboundService {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &InboundService{
		store: store, registry: registry, secretBox: box, publisher: publisher, qr: qr,
		domain: domain, enqueueAutomation: domain != nil, logger: logger,
	}
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
		EnqueueAutomation: ev.Message != nil && !ev.Message.FromMe &&
			s.enqueueAutomation,
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
	if write.Message != nil && s.domain != nil {
		event := EventMsgOutboundHuman
		hasActiveAgent := false
		if !write.Message.FromMe {
			event = EventMsgInbound
			if s.store.AIDispatchV2Enabled() &&
				!isWhatsAppGroupExternalID(write.Message.ContactExternalID) {
				_, hasActiveAgent, err = s.store.ActiveAgentForInstance(
					ctx, accountID, write.InstanceID,
				)
				if err != nil {
					// Agent resolution is an optional automation gate. A lookup
					// failure must not lose the customer message: fail open to
					// deterministic human routing in the same transaction.
					s.logger.Error(
						"omnichannel_inbound_agent_lookup_failed",
						"account_id", accountID,
						"instance_id", write.InstanceID,
					)
					hasActiveAgent = false
				}
			}
		}
		res, err = s.store.PersistInboundWithTransition(
			ctx,
			write,
			func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
				if event == EventMsgInbound {
					return s.domain.decideTransitionWithContext(
						ctx,
						accountID,
						event,
						TransitionPayload{},
						snap,
						TransitionContext{
							HasActiveAgent: hasActiveAgent,
							HasQueue:       snap.QueueID != nil,
						},
					)
				}
				return s.domain.decideTransition(
					ctx, accountID, event, TransitionPayload{}, snap,
				)
			},
		)
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
	// O intento F5->F9->F8 ja foi commitado com a mensagem. O worker pode
	// reexecuta-lo de forma idempotente sem depender do contexto do webhook.
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
