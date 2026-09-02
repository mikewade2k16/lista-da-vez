package omnichannel

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/evolution"
	instagram "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/instagram"
	meta_whatsapp "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/meta_whatsapp"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/mock"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// Module e o adaptador do modulo `omnichannel` para o Module Registry.
//
// Atendimento WhatsApp (inbox humano + setores/filas + triagem IA), schema messaging.*.
// Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md. Spec desta fase (F2 — schema +
// leitura): docs/omnichannel/specs/OMNI-F2.md.
//
// O modulo e INDEPENDENTE por construcao (canonico §4): nao le, nao escreve e nao depende
// do schema, da API nem do runtime de nenhum outro modulo satelite. Todo dado dele nasce
// e vive em messaging.*. As unicas tabelas de fora que ele le sao as da PLATAFORMA
// (core.accounts/core.users/core.account_modules) — e por contrato explicito da spec C4.
type Module struct {
	handle *handle
	// secretBox cifra/decifra as credenciais das instancias (F3.5, injetado no app.go via
	// WithSecretBox). nil = credenciais indisponiveis (o mock nao precisa; providers reais
	// sim). O app.go faz fail-fast do OMNI_SECRETS_KEY no boot antes de injetar.
	secretBox *secretbox.Box
	// publisher e o seam de realtime (F5). nil = no-op (F4 nao emite realtime).
	publisher Publisher
	// toolRegistry recebe adaptadores corporativos explícitos. Sem injeção, tools não são
	// descobertas nem executadas por fallback: chamadas ficam negadas e auditadas.
	toolRegistry *AIToolRegistry
	// clientCatalog reutiliza a lista permission-scoped de clientes da plataforma.
	// businessContext projeta o perfil estrategico do Calendario por interface explicita.
	clientCatalog        AutomationClientCatalog
	businessContext      AutomationBusinessContextProvider
	customerIntelligence CustomerIntelligenceBridge
	customerDataIngest   CustomerDataInboundBridge
}

// Option configura o Module na composicao (padrao calendar.New).
type Option func(*Module)

// WithSecretBox injeta o Box de cifragem das credenciais (F4 e a 1a consumidora real).
func WithSecretBox(box *secretbox.Box) Option {
	return func(m *Module) { m.secretBox = box }
}

// WithPublisher injeta o transporte de realtime (F5). nil = canal desligado.
func WithPublisher(p Publisher) Option {
	return func(m *Module) { m.publisher = p }
}

// WithAIToolRegistry injeta o catálogo de execução aprovado pelo compositor. O módulo não
// cria handlers para endpoints/SQL de terceiros por conta própria.
func WithAIToolRegistry(registry *AIToolRegistry) Option {
	return func(m *Module) { m.toolRegistry = registry }
}

func WithAutomationClientCatalog(catalog AutomationClientCatalog) Option {
	return func(m *Module) { m.clientCatalog = catalog }
}

func WithAutomationBusinessContext(provider AutomationBusinessContextProvider) Option {
	return func(m *Module) { m.businessContext = provider }
}

// WithCustomerIntelligence injects the optional headless intelligence module.
// The default is nil/off and preserves the complete legacy chat runtime.
func WithCustomerIntelligence(bridge CustomerIntelligenceBridge) Option {
	return func(m *Module) { m.customerIntelligence = bridge }
}

// WithCustomerDataIngest injects the independent deterministic relationship
// resolver. It consumes the integration outbox even when all AI modes are off.
func WithCustomerDataIngest(bridge CustomerDataInboundBridge) Option {
	return func(m *Module) { m.customerDataIngest = bridge }
}

// New cria um Module pronto para registrar no Registry.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Module) ID() string { return "omnichannel" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "messaging",
		Label:           "Omnichannel",
		Description:     "Atendimento WhatsApp: inbox, setores/filas e triagem por IA.",
		IsCore:          false,
		OptionalModules: []string{"customer_data", "customer_intelligence"},
		SortOrder:       47,
	}
}

// Permissions declara as 10 keys do canonico §5.2.
//
// GAP HONESTO, registrado na spec (§Seguranca): nao existe middleware de permissao por
// key no Go — o disco so tem RequireAuth/RequireAuthWithAccount/RequireRoles e o gate de
// modulo por path. Hoje estas keys gateiam o FRONT; a F2 as DECLARA (para o gating da F1
// funcionar) e protege as rotas com RequireAuth + gate de modulo + escopo de conta. O
// enforcement por key vira load-bearing na ESCRITA (F6/F7), onde se decide entre
// middleware novo ou checagem no service. Nao fingir que a key protege a rota agora.
func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "omnichannel.conversations.view", Label: "Ver o inbox", Scope: "account"},
		{Key: "omnichannel.conversations.reply", Label: "Responder conversas", Scope: "account"},
		{Key: "omnichannel.conversations.assign", Label: "Atribuir/transferir conversas", Scope: "account"},
		{Key: "omnichannel.conversations.close", Label: "Encerrar conversas", Scope: "account"},
		{Key: "omnichannel.contacts.manage", Label: "Gerenciar contatos", Scope: "account"},
		{Key: "omnichannel.instances.manage", Label: "Gerenciar numeros/instancias/providers", Scope: "account"},
		{Key: "omnichannel.settings.manage", Label: "Gerenciar setores, filas e regras de roteamento", Scope: "account"},
		{Key: "omnichannel.agents.manage", Label: "Editar agente de IA (publish/rollback)", Scope: "account"},
		{Key: "omnichannel.audit.view", Label: "Ver a trilha de auditoria", Scope: "account"},
		{Key: conversationPrivacyManagePermission, Label: "Gerenciar contatos ocultos", Scope: "account"},
	}
}

// RoleTemplates declara os 3 papeis do canonico §5.2: attendant, supervisor, manager.
//
// Regra central do modulo: permissao gateia FEATURE; fila gateia DADO. conversations.view
// nao e "ve tudo" — o atendente ve so as conversas das filas onde e queue_member + as
// atribuidas a ele. Esse filtro e no REPOSITORIO e chega na F8 (na F2 o que existe e o
// escopo por instancia/responsible_user_id — ver A2 no service).
func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "omnichannel.attendant",
			Label:       "Atendente",
			Description: "Atende o inbox: ve e responde as conversas do seu escopo.",
			SortOrder:   100,
			Permissions: []string{
				"omnichannel.conversations.view",
				"omnichannel.conversations.reply",
				"omnichannel.contacts.manage",
			},
		},
		{
			ID:          "omnichannel.supervisor",
			Label:       "Supervisor de Atendimento",
			Description: "Atende, atribui/transfere, encerra conversas e ve a auditoria.",
			SortOrder:   110,
			Permissions: []string{
				"omnichannel.conversations.view",
				"omnichannel.conversations.reply",
				"omnichannel.conversations.assign",
				"omnichannel.conversations.close",
				"omnichannel.contacts.manage",
				"omnichannel.audit.view",
			},
		},
		{
			ID:          "omnichannel.manager",
			Label:       "Gestor de Atendimento",
			Description: "Tudo do supervisor + numeros/providers, setores/filas/regras e agente de IA.",
			SortOrder:   120,
			Permissions: []string{
				"omnichannel.conversations.view",
				"omnichannel.conversations.reply",
				"omnichannel.conversations.assign",
				"omnichannel.conversations.close",
				"omnichannel.contacts.manage",
				"omnichannel.instances.manage",
				"omnichannel.settings.manage",
				"omnichannel.agents.manage",
				"omnichannel.audit.view",
			},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	store := NewStore(deps.Pool)
	store.SetHistoryCutoffEnforced(historyCutoffEnforcedFromEnv())
	if !store.HistoryCutoffEnforced() && deps.Logger != nil {
		deps.Logger.Warn("omnichannel_history_instance_cutoff_disabled", "contact_privacy", "enforced")
	}
	toolRegistry := m.toolRegistry
	if toolRegistry == nil {
		toolRegistry = NewAIToolRegistry()
		registerBuiltinAITools(toolRegistry, store)
	}

	// Registry de providers (spec C2). O `mock` roda sem rede; o `evolution` e o 1o adapter
	// REAL (D-A). A credencial e POR INSTANCIA (provider_config + credentials_ciphertext);
	// EVOLUTION_BASE_URL/EVOLUTION_API_KEY sao apenas FALLBACK de ambiente (canonico §13-item 3).
	registry := channel.NewRegistry(
		mock.New(),
		// evolution: F4 adapter — registrado pela MESMA interface; env so como fallback.
		// deps.Logger p/ o adapter logar falha de setWebhook (sem vazar token).
		evolution.New(os.Getenv("EVOLUTION_BASE_URL"), os.Getenv("EVOLUTION_API_KEY"), deps.Logger),
		meta_whatsapp.New(os.Getenv("META_GRAPH_BASE_URL")),
		instagram.New(os.Getenv("META_GRAPH_BASE_URL")),
	)
	cloudService := NewWhatsAppCloudService(store, registry, m.secretBox)
	instagramService := NewInstagramService(store, registry, m.secretBox)

	limitReader := modules.NewLimitReader(deps.Pool)

	// qrCache COMPARTILHADO: o SessionService escreve o QR que vem sincrono no connect e le no
	// /qrcode; o InboundService escreve o QR que a Evolution empurra por webhook (QRCODE_UPDATED).
	// Precisa ser a MESMA instancia senao o QR do webhook nunca chega ao endpoint que o painel le.
	qr := newSharedQRCache(deps.Pool)

	// Triagem IA (F9): provider/modelo/chave continuam vindo do painel/banco. O executor
	// pode ser nativo (rollback) ou n8n (cerebro configuravel); em ambos os casos o Go
	// valida o schema e continua sendo o unico dono de estado, roteamento e envio.
	llmClient := llm.New(deps.Logger)
	var brainExec brainExecutor
	var brainGateway *BrainGateway
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OMNI_AI_EXECUTOR")), "n8n") {
		webhookURL := strings.TrimSpace(os.Getenv("OMNI_N8N_BRAIN_WEBHOOK_URL"))
		internalToken := strings.TrimSpace(os.Getenv("OMNI_N8N_INTERNAL_TOKEN"))
		if m.secretBox != nil && webhookURL != "" && internalToken != "" {
			brainExec = newN8NBrainExecutor(webhookURL, internalToken, m.secretBox, deps.Logger)
			brainGateway = newBrainGateway(m.secretBox, llmClient, store)
		} else if deps.Logger != nil {
			deps.Logger.Warn("omnichannel_n8n_executor_blocked_until_brain_v2_gateway_configured")
		}
	}
	var aiOpts []AIServiceOption
	if brainExec != nil {
		aiOpts = append(aiOpts, WithBrainExecutor(brainExec))
	}
	if m.businessContext != nil {
		aiOpts = append(aiOpts, WithAIBusinessContext(m.businessContext))
	}
	aiSvc := NewAIService(store, llmClient, m.secretBox, limitReader, deps.Logger, aiOpts...)

	// Envio (F6): outbox durável (platform/jobs, F3) + mídia em disco. O worker DESPACHA o envio
	// pelo provider — sem ele a mensagem enfileira e nunca sai. FIFO por conversa (ordering_key).
	media := NewDiskMediaStorage(MediaDirFromEnv())
	mediaAnalysisGateway := newMediaAnalysisGateway(m.secretBox, store, media)
	outboxStore, err := jobs.NewPostgresStore(deps.Pool, jobs.DefaultTable)
	if err != nil {
		return nil, err
	}
	worker := jobs.New(outboxStore, jobs.Config{WorkerID: "omnichannel", Logger: deps.Logger})
	worker.Register(OutboundJobKind, NewOutboundHandler(store, registry, m.secretBox, m.publisher, deps.Logger))
	worker.Register(InstagramActionJobKind, NewInstagramActionHandler(store, registry, m.secretBox))
	// Poda LGPD (F13): o mesmo worker despacha o purge. Registrar ANTES do Start — kind sem
	// handler vira ErrNoHandler → dead-letter. Roda em batches com teto de tempo (não segura envio).
	retStore := NewRetentionStore(deps.Pool)
	purgeHandler := NewPurgeHandler(NewRetentionResolver(deps.Pool), retStore, outboxStore, MediaDirFromEnv(), deps.Logger)
	worker.Register(PurgeAccountJobKind, purgeHandler)
	worker.Register(PurgeMediaOrphanJobKind, purgeHandler)
	// service e send em locais: o ActionsService (F7) depende dos dois (forward reusa o send;
	// status/assign passam pela FSM via o Service).
	svc := NewService(store)
	svc.publisher = m.publisher
	svc.setGroupMetadataResolver(newGroupMetadataResolver(store, registry, m.secretBox, deps.Logger))
	automationSvc := NewAutomationService(store, svc, m.clientCatalog, m.businessContext, svc)
	channelBindingSvc := NewChannelClientBindingService(store, svc, m.clientCatalog)
	sendSvc := NewSendService(store, media, m.publisher, deps.Logger)
	durableAIDispatch, probeErr := store.AIDispatchSchemaAvailable(context.Background())
	if probeErr != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("omnichannel_ai_dispatch_schema_probe_failed", "error", probeErr.Error())
		}
		durableAIDispatch = false
	}
	store.SetAIDispatchV2Enabled(durableAIDispatch)
	if deps.Logger != nil {
		if !durableAIDispatch {
			deps.Logger.Error(
				"omnichannel_ai_dispatch_disabled",
				"fallback", "human_routing",
			)
		}
		requestedExecutor := strings.TrimSpace(os.Getenv("OMNI_AI_EXECUTOR"))
		if strings.EqualFold(requestedExecutor, "n8n") && brainExec == nil {
			requestedExecutor = "native (n8n aguardando gateway brain.v2)"
		}
		deps.Logger.Info("omnichannel_ai_dispatch_runtime", "durable_dispatch", durableAIDispatch, "executor", requestedExecutor)
	}
	worker.Register(AIDispatchJobKind, newAIDispatchHandler(
		store, aiSvc, svc, sendSvc, m.customerIntelligence, deps.Logger,
	))
	worker.Register(AIInboundJobKind, newAIInboundHandler(store, svc, deps.Logger))
	intelligenceOutboxStore, err := jobs.NewPostgresStore(
		deps.Pool, "messaging.intelligence_outbox",
	)
	if err != nil {
		return nil, err
	}
	intelligenceWorker := jobs.New(
		intelligenceOutboxStore,
		jobs.Config{WorkerID: "omnichannel-intelligence", Batch: 10, Logger: deps.Logger},
	)
	intelligenceWorker.Register(
		intelligenceAcceptedJobKind,
		intelligenceAcceptedHandler{
			bridge: m.customerIntelligence, acceptanceLease: store.WithAIIntelligenceAcceptanceLease,
		},
	)
	customerDataOutboxStore, err := jobs.NewPostgresStore(
		deps.Pool, "messaging.customer_data_outbox",
	)
	if err != nil {
		return nil, err
	}
	customerDataWorker := jobs.New(
		customerDataOutboxStore,
		jobs.Config{WorkerID: "omnichannel-customer-data", Batch: 20, Logger: deps.Logger},
	)
	customerDataWorker.Register(
		customerDataRelationshipJobKind,
		customerDataInboundHandler{bridge: m.customerDataIngest, historyLease: store.WithMessageExternalEffectLease},
	)
	mediaBaseURL := strings.TrimSpace(os.Getenv("OMNI_INTERNAL_API_BASE_URL"))
	if mediaBaseURL == "" {
		mediaBaseURL = "http://api:8080"
	}
	mediaAnalyzer := newN8NMediaAnalyzer(store, m.secretBox, aiSvc,
		os.Getenv("OMNI_N8N_MEDIA_WEBHOOK_URL"), mediaBaseURL, os.Getenv("OMNI_N8N_INTERNAL_TOKEN"))
	if mediaAnalyzer != nil {
		worker.Register(MediaFetchJobKind, NewMediaFetchHandler(store, media, registry, m.secretBox, m.publisher, deps.Logger, mediaAnalyzer))
	} else {
		worker.Register(MediaFetchJobKind, NewMediaFetchHandler(store, media, registry, m.secretBox, m.publisher, deps.Logger))
		if deps.Logger != nil {
			deps.Logger.Warn("omnichannel_n8n_media_analyzer_not_configured")
		}
	}
	// Contexto de vida dos workers: criado somente depois das inicializações
	// falíveis e cancelado no Handle.Close (shutdown limpo).
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	worker.Start(workerCtx)
	intelligenceWorker.Start(workerCtx)
	if m.customerDataIngest != nil {
		customerDataWorker.Start(workerCtx)
	} else if deps.Logger != nil {
		deps.Logger.Warn("omnichannel_customer_data_ingest_bridge_unavailable")
	}
	StartRetentionScheduler(workerCtx, retStore, outboxStore, deps.Logger)
	StartSLAScheduler(workerCtx, store, deps.Logger)
	inbound := NewInboundService(store, registry, m.secretBox, m.publisher, qr, svc, deps.Logger)

	m.handle = &handle{
		service:    svc,
		automation: automationSvc,
		bindings:   channelBindingSvc,
		operational: NewOperationalService(store,
			strings.TrimSpace(os.Getenv("OMNI_N8N_BRAIN_WEBHOOK_URL")) != ""),
		rollout: NewRolloutService(store),
		// F5->F9->F8: o inbound grava um intento no outbox na mesma transacao da
		// mensagem; o worker revalida a FSM e cria o dispatch duravel.
		inbound: inbound,
		session: NewSessionService(store, registry, m.secretBox, limitReader, qr, deps.Logger, m.publisher),
		ai:      aiSvc,
		send:    sendSvc,
		media:   NewMediaService(store, media, registry, m.secretBox, m.publisher, deps.Logger),
		// F12: GIF (search/media via Tenor) + chave do Tenor cifrada (secretbox). Stickers: CRUD +
		// disco (media_storage da F6) + poda FIFO. Reusam store/media/secretBox ja construidos.
		gif:      NewGifService(store, m.secretBox),
		stickers: NewStickerService(store, media, deps.Logger),
		// Acoes do inbox (F7): reaction/forward/delete/status/assign. status/assign passam pela FSM (F8).
		actions: NewActionsService(store, svc, sendSvc, registry, m.publisher, deps.Logger, WithActionsSecretBox(m.secretBox)),
		// Custo de LLM por conta (F13): agrega ai_runs (usage/cost) por conta+periodo.
		cost:               NewCostService(deps.Pool, limitReader),
		worker:             worker,
		intelligenceWorker: intelligenceWorker,
		customerDataWorker: customerDataWorker,
		cancelWorker:       cancelWorker,
		webhookLimiter:     newSharedRateLimiter(deps.Pool),
		authMiddleware:     deps.AuthMiddleware,
		brainGateway:       brainGateway,
		toolGateway:        newAIToolCallGateway(m.secretBox, store, toolRegistry),
		mediaGateway:       mediaAnalysisGateway,
		cloud:              cloudService,
		instagram:          instagramService,
	}
	return m.handle, nil
}

// historyCutoffEnforcedFromEnv e fail-closed: ausente, vazio ou invalido mantem o cutoff
// de instancia ligado. O valor false desliga somente essa camada; contato continua privado.
func historyCutoffEnforcedFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OMNICHANNEL_HISTORY_CUTOFF_ENFORCED"))) {
	case "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

// Service devolve o Service construido no Build (nil antes do Build). Ponto de injecao
// LAZY para as fases seguintes (F4 webhook, F5 realtime, F6 envio).
func (m *Module) Service() *Service {
	if m.handle == nil {
		return nil
	}
	return m.handle.service
}

// AIService exposes the account-scoped AI vault/runtime facade to adapters
// wired by platform/app. Satellite modules must not import the Store or query
// messaging.* directly.
func (m *Module) AIService() *AIService {
	if m.handle == nil {
		return nil
	}
	return m.handle.ai
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service            *Service
	automation         *AutomationService
	bindings           *ChannelClientBindingService
	operational        *OperationalService
	rollout            *RolloutService
	inbound            *InboundService
	session            *SessionService
	ai                 *AIService
	send               *SendService
	media              *MediaService
	gif                *GifService
	stickers           *StickerService
	actions            *ActionsService
	cost               *CostService
	worker             *jobs.Worker
	intelligenceWorker *jobs.Worker
	customerDataWorker *jobs.Worker
	cancelWorker       context.CancelFunc
	webhookLimiter     *rateLimiter
	authMiddleware     *auth.Middleware
	brainGateway       *BrainGateway
	toolGateway        *aiToolCallGateway
	mediaGateway       *MediaAnalysisGateway
	cloud              *WhatsAppCloudService
	instagram          *InstagramService
}

func (h *handle) ID() string { return "omnichannel" }

func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	registerBrainGatewayRoutes(mux, h.brainGateway)
	registerAIToolCallRoutes(mux, h.toolGateway)
	registerMediaAnalysisRoutes(mux, h.mediaGateway)
	// Rotas de leitura do inbox e do ciclo de sessao /v1/omnichannel/* (RequireAuthWithAccount
	// + gate de modulo no Chain via moduleGatingRules).
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterContactPrivacyRoutes(mux, h.service, h.authMiddleware)
	RegisterCRMContactRoutes(mux, h.service, h.authMiddleware)
	registerSessionRoutes(mux, h.session, h.authMiddleware)
	// Gestao de instancia (escrita): criar/atualizar/atribuir usuarios + validate-endpoints +
	// conversations/clear. No SessionService (secretBox/limits/registry). Ver http_instances.go.
	registerInstanceRoutes(mux, h.session, h.authMiddleware)
	registerWhatsAppCloudRoutes(mux, h.cloud, h.authMiddleware)
	registerInstagramRoutes(mux, h.instagram, h.authMiddleware)
	// Dominio de atendimento (F8): setores/filas/membros/regras + PATCH .../queue +
	// GET .../routing-decisions, sob /v1/omnichannel/* (RequireAuthWithAccount + gate).
	// Costurado aqui (a F8 rodou em paralelo e nao pode editar este arquivo).
	RegisterDomainRoutes(mux, h.service, h.authMiddleware)
	// Triagem IA (F9): CRUD de ai_agents + publish/rollback de versao + collect-fields + runs +
	// simulate, sob /v1/omnichannel/* (RequireAuthWithAccount + gate). Costurado aqui (a F9 rodou
	// em paralelo e nao pode editar este arquivo).
	RegisterAIRoutes(mux, h.ai, h.authMiddleware)
	registerAssistantAICredentialRoutes(mux, h.ai, h.authMiddleware)
	RegisterKnowledgeRoutes(mux, h.ai, h.authMiddleware)
	// MVP de automacao: cliente -> numero -> agente + policy configuravel de encerramento.
	RegisterAutomationRoutes(mux, h.automation, h.authMiddleware)
	// Ownership operacional canal -> cliente independe da IA. Conversas e
	// touchpoints guardam snapshot; reatribuicao nunca move historico.
	RegisterChannelClientBindingRoutes(mux, h.bindings, h.authMiddleware)
	RegisterOperationalRoutes(mux, h.operational, h.authMiddleware)
	RegisterRolloutRoutes(mux, h.rollout, h.authMiddleware)
	// Envio + mídia (F6): POST conversations/{id}/messages (outbox) + GET .../media (stream+Range).
	RegisterSendRoutes(mux, h.send, h.media, h.authMiddleware)
	// Ações do inbox (F7): reaction/forward/delete/status/assign + group/sync/contatos.
	RegisterActionRoutes(mux, h.actions, h.authMiddleware)
	// Custo de LLM por conta (F13): GET /v1/omnichannel/ai/usage.
	RegisterCostRoutes(mux, h.cost, h.authMiddleware)
	// F12: GIF (search/media, conversations.reply) + chave do Tenor cifrada (settings so
	// platform_admin) + stickers (CRUD + disco), sob /v1/omnichannel/* (RequireAuthWithAccount + gate).
	RegisterGifRoutes(mux, h.gif, h.authMiddleware)
	RegisterStickerRoutes(mux, h.stickers, h.authMiddleware)
	// Webhook inbound PUBLICO /v1/webhooks/omnichannel/{provider}/{accountSlug}: FORA do gate
	// de modulo por design (nao esta sob /v1/omnichannel; precedente /v1/public/*, /s/{slug}).
	registerWebhookRoutes(mux, h.inbound, h.webhookLimiter)
	registerLeadCaptureRoutes(mux, h.service.store, h.webhookLimiter)
	// Avatar PUBLICO (F12 C4): GET /v1/public/omnichannel/avatar — o front poe a URL no <img src>
	// (sem token), entao vai FORA do gate, com allowlist dos 4 hosts do WhatsApp + rate-limit por
	// IP + anti-SSRF da F6. Reusa o limiter do webhook. Ver http_avatar.go.
	registerAvatarRoutes(mux, h.webhookLimiter)
}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

// Close para o worker do outbox (F6): cancela o contexto de vida e drena o worker.
func (h *handle) Close() error {
	if h.cancelWorker != nil {
		h.cancelWorker()
	}
	var firstErr error
	if h.worker != nil {
		firstErr = h.worker.Close()
	}
	if h.intelligenceWorker != nil {
		if err := h.intelligenceWorker.Close(); firstErr == nil {
			firstErr = err
		}
	}
	if h.customerDataWorker != nil {
		if err := h.customerDataWorker.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
