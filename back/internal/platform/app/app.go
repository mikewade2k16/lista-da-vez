package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/automation"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/bi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/bio"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/cardapio"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/catalog"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/erp"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
	metaads "github.com/mikewade2k16/lista-da-vez/back/internal/modules/meta_ads"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/notifications"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operationgoals"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/alerts"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/analytics"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/consultants"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/feedback"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/reports"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/settings"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/realtime"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/roadmap"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/site"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/socialpublishing"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tools"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/users"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func BuildHTTPHandler(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) (http.Handler, error) {
	const operationsAlertMonitorInterval = 3 * time.Second
	const feedbackAttachmentCleanupInterval = 6 * time.Hour

	hasher := auth.NewBcryptHasher(cfg.BcryptCost)
	userStore := auth.NewPostgresUserStore(pool)
	tokenManager := auth.NewHMACTokenManager(cfg.AuthTokenSecret, cfg.AuthTokenTTL)
	avatarStorage := auth.NewDiskAvatarStorage(cfg.UploadsDir)
	feedbackImageStorage := feedback.NewDiskImageStorage(cfg.UploadsDir)
	taskVideoStorage := tasks.NewDiskVideoStorage(cfg.UploadsDir)
	calendarMediaStorage := calendar.NewDiskMediaStorage(cfg.UploadsDir)
	passwordResetDelivery, err := auth.BuildPasswordResetDelivery(auth.SMTPPasswordResetDeliveryConfig{
		AppName:            cfg.AppName,
		Host:               cfg.SMTPHost,
		Port:               cfg.SMTPPort,
		Username:           cfg.SMTPUsername,
		Password:           cfg.SMTPPassword,
		FromEmail:          cfg.SMTPFromEmail,
		FromName:           cfg.SMTPFromName,
		TLSMode:            auth.SMTPTLSMode(cfg.SMTPTLSMode),
		InsecureSkipVerify: cfg.SMTPInsecureSkipVerify,
		Timeout:            cfg.SMTPTimeout,
	}, logger)
	if err != nil {
		return nil, err
	}
	consultantRepository := consultants.NewPostgresRepository(pool)
	consultantProfileSync := consultants.NewProfileSync(consultantRepository)
	usersRepository := users.NewPostgresRepository(pool)
	accessRepository := access.NewPostgresRepository(pool)
	accessService := access.NewService(accessRepository, newAccessSubjectResolver(usersRepository))
	authService := auth.NewService(userStore, hasher, tokenManager, avatarStorage, accessService, nil, consultantProfileSync)
	// P0.2: sessoes reais ligadas. Login passa a emitir `sid` + linha em
	// core.user_sessions; Authenticate checa revoked_at; logout revoga. Sem isto
	// o logout era apenas client-side e o token continuava valido ate expirar.
	authService.SetSessionRepository(auth.NewPostgresSessionRepository(pool))
	invitationService := auth.NewInvitationService(userStore, hasher, tokenManager, cfg.WebAppURL, cfg.AuthInviteTTL)
	passwordResetService := auth.NewPasswordResetService(userStore, userStore, hasher, passwordResetDelivery, cfg.AuthPasswordResetTTL)
	authMiddleware := auth.NewMiddleware(authService)
	tenantRepository := tenants.NewPostgresRepository(pool)
	tenantService := tenants.NewService(tenantRepository)
	realtimeHub := realtime.NewHub()
	realtimeService := realtime.NewService(authService, nil, tenantService, cfg.CORSAllowedOrigins, realtimeHub, pool)
	authService.SetContextPublisher(realtimeService)
	accessService.SetContextPublisher(realtimeService)
	storeRepository := stores.NewPostgresRepository(pool)
	storeService := stores.NewService(storeRepository, realtimeService)
	realtimeService.SetStoreFinder(storeService)
	consultantService := consultants.NewService(
		consultantRepository,
		hasher,
		cfg.ConsultantEmailDomain,
		cfg.ConsultantDefaultPassword,
	)
	settingsRepository := settings.NewPostgresRepository(pool)
	settingsService := settings.NewService(settingsRepository, realtimeService)
	catalogRepository := catalog.NewPostgresRepository(pool)
	catalogService := catalog.NewService(catalogRepository, newCatalogStoreFinderAdapter(storeService))
	alertsRepository := alerts.NewPostgresRepository(pool)
	alertsService := alerts.NewService(alertsRepository)
	alertsService.SetContextPublisher(realtimeService)
	operationsRepository := operations.NewPostgresRepository(pool)
	operationsService := operations.NewService(operationsRepository, realtimeService, newOperationsStoreScopeAdapter(storeService))
	operationsService.SetAlertCoordinator(alertsService)
	alertsService.SetOperationsScanner(operationsService)
	go func() {
		ticker := time.NewTicker(operationsAlertMonitorInterval)
		defer ticker.Stop()

		for {
			if err := operationsService.ProcessTimedAlerts(context.Background()); err != nil {
				logger.Warn("operations_alert_monitor_failed", "error", err)
			}
			<-ticker.C
		}
	}()
	reportsRepository := reports.NewPostgresRepository(pool)
	reportsService := reports.NewService(reportsRepository, storeService)
	analyticsRepository := analytics.NewPostgresRepository(pool)
	analyticsService := analytics.NewService(analyticsRepository, storeService)
	feedbackRepository := feedback.NewPostgresRepository(pool)
	feedbackService := feedback.NewService(feedbackRepository, feedbackImageStorage)
	go func() {
		ticker := time.NewTicker(feedbackAttachmentCleanupInterval)
		defer ticker.Stop()

		for {
			deletedCount, err := feedbackService.CleanupExpiredAttachments(context.Background())
			if err != nil {
				logger.Warn("feedback_attachment_cleanup_failed", "error", err)
			} else if deletedCount > 0 {
				logger.Info("feedback_attachment_cleanup_completed", "deleted_count", deletedCount)
			}
			<-ticker.C
		}
	}()
	erpRepository := erp.NewPostgresRepository(pool)
	erpService := erp.NewService(erpRepository, erp.Options{
		Env:                        cfg.Env,
		SourceKind:                 cfg.ERPSourceKind,
		SourceRecursive:            cfg.ERPSourceRecursive,
		SourceDir:                  cfg.ERPLocalSourceDir,
		StorageDir:                 cfg.ERPStorageDir,
		BootstrapItemFile:          cfg.ERPBootstrapItemFile,
		BootstrapCustomerFile:      cfg.ERPBootstrapCustomerFile,
		BootstrapEmployeeFile:      cfg.ERPBootstrapEmployeeFile,
		BootstrapOrderFile:         cfg.ERPBootstrapOrderFile,
		BootstrapOrderCanceledFile: cfg.ERPBootstrapOrderCanceledFile,
		AllowManualSync:            cfg.ERPAllowManualSync,
		FTPHost:                    cfg.ERPFTPHost,
		FTPPort:                    cfg.ERPFTPPort,
		FTPUser:                    cfg.ERPFTPUser,
		FTPPassword:                cfg.ERPFTPPassword,
		FTPKeyPath:                 cfg.ERPFTPKeyPath,
		FTPRemoteDir:               cfg.ERPFTPRemoteDir,
		FTPHostKey:                 cfg.ERPFTPHostKey,
		RootStoreCode:              cfg.ERPRootStoreCode,
		SyncAutomaticEnabled:       cfg.ERPSyncAutomaticEnabled,
		SyncInterval:               cfg.ERPSyncInterval,
		SyncHourUTC:                cfg.ERPSyncHourUTC,
		SyncDryRunDefault:          cfg.ERPSyncDryRunDefault,
		CSVMaxBytes:                cfg.ERPCSVMaxBytes,
		ManualSyncMaxFiles:         cfg.ERPManualSyncMaxFiles,
		BackfillMaxFiles:           cfg.ERPBackfillMaxFiles,
		ManualSyncMinInterval:      cfg.ERPManualSyncMinInterval,
	})
	// Ponte de meta no snapshot: operations obtem o goalProgress canonico do CRM
	// via o service do erp (adapter na composition root => sem ciclo de import).
	// Injecao opcional/nil-safe: se falhar, o snapshot degrada com GoalStats=nil.
	operationsService.SetGoalProgressProvider(newOperationsGoalProgressAdapter(erpService))
	if recovered, err := erpRepository.RecoverOrphanedSyncRuns(context.Background(), 2*time.Hour); err != nil {
		logger.Warn("erp_orphan_recovery_failed", "error", err)
	} else if recovered > 0 {
		logger.Info("erp_orphaned_runs_recovered", "count", recovered)
	}

	if cfg.ERPSyncAutomaticEnabled {
		logger.Info("erp_sync_scheduler_started",
			"source_kind", cfg.ERPSourceKind,
			"interval", cfg.ERPSyncInterval.String(),
			"hour_utc", cfg.ERPSyncHourUTC,
			"dry_run", cfg.ERPSyncDryRunDefault,
		)
		go func() {
			if missedFor, ok := missedERPScheduledRun(time.Now().UTC(), cfg.ERPSyncInterval, cfg.ERPSyncHourUTC); ok {
				alreadyRan, err := erpRepository.HasAutomaticCSVSyncRunSince(context.Background(), missedFor)
				if err != nil {
					logger.Warn("erp_automatic_sync_catchup_check_failed",
						"scheduled_for", missedFor,
						"error", err,
					)
				} else if !alreadyRan {
					logger.Info("erp_automatic_sync_catchup_started",
						"scheduled_for", missedFor,
						"dry_run", cfg.ERPSyncDryRunDefault,
					)
					runERPAutomaticSync(context.Background(), logger, erpService, cfg.ERPSyncDryRunDefault, missedFor, "catchup")
				}
			}

			for {
				scheduledFor := nextERPScheduledRun(time.Now().UTC(), cfg.ERPSyncInterval, cfg.ERPSyncHourUTC)
				wait := time.Until(scheduledFor)
				if wait > 0 {
					timer := time.NewTimer(wait)
					<-timer.C
				}

				runERPAutomaticSync(context.Background(), logger, erpService, cfg.ERPSyncDryRunDefault, scheduledFor, "scheduled")
			}
		}()
	}
	usersService := users.NewService(usersRepository, hasher, invitationService, realtimeService, consultantProfileSync)
	// Atalho unificado: editar/criar consultor pela grade de Usuarios sincroniza o
	// roster (queue.consultants) e transmite operation.updated ao vivo por loja.
	usersService.SetOperationPublisher(realtimeService)
	// AC-01: cache de Principals com TTL curto + invalidacao sincrona. nil quando
	// AUTH_PRINCIPAL_CACHE_TTL=0s (comportamento legado preservado).
	principalCache := wirePrincipalCache(cfg, logger, authService, accessService, usersService)
	biService := bi.NewService(bi.Options{
		CompanyKey:         cfg.PerolaBICompanyKey,
		Login:              cfg.PerolaBILogin,
		Pass:               cfg.PerolaBIPass,
		StaticToken:        cfg.PerolaBIStaticToken,
		DefaultCNPJEmpresa: cfg.PerolaBICNPJEmpresa,
		TokenTTL:           cfg.PerolaBITokenTTL.String(),
		RequestTimeout:     cfg.PerolaBIRequestTimeout.String(),
		PageLimit:          cfg.PerolaBIPageLimit,
		MaxPages:           cfg.PerolaBIMaxPages,
	})

	mux := http.NewServeMux()
	if strings.TrimSpace(cfg.UploadsDir) != "" {
		fileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir)))
		mux.Handle("GET /uploads/", fileServer)
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		// AC-16: ping real no banco (o pool pgx e lazy — sem isso a api responde 200
		// mesmo com o Postgres morto). 503 = healthcheck do compose marca unhealthy e
		// o smoke do deploy falha alto. Erro NUNCA vai no corpo (endpoint publico).
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := http.StatusOK
		overall := "ok"
		dbStatus := "ok"
		if err := pool.Ping(pingCtx); err != nil {
			status = http.StatusServiceUnavailable
			overall = "degraded"
			dbStatus = "unreachable"
			logger.Warn("healthz_db_ping_failed", "error", err)
		}

		httpapi.WriteJSON(w, status, map[string]any{
			"service": cfg.AppName,
			"status":  overall,
			"db":      dbStatus,
			"modules": []string{
				"auth",
				"tenants",
				"stores",
				"consultants",
				"settings",
				"catalog",
				"operations",
				"realtime",
				"reports",
				"analytics",
				"alerts",
				"access",
				"feedback",
				"erp",
				"bi",
				"users",
			},
			"tenantMode":    "owner-is-client",
			"coreV2Enabled": cfg.CoreV2Enabled,
		})
	})

	if cfg.CoreV2Enabled {
		logger.Info("core_v2 feature flag enabled — multi-tenant refactor code paths active")
	}

	auth.RegisterRoutes(mux, authService, invitationService, passwordResetService, authMiddleware, auth.GatewayConfig{
		CookieDomain: cfg.AuthGatewayCookieDomain,
		LoginURL:     cfg.WebAppURL,
	})
	registerContextRoutes(mux, authService, authMiddleware, tenantService, storeService)
	tenants.RegisterRoutes(mux, tenantService, authMiddleware)
	stores.RegisterRoutes(mux, storeService, authMiddleware)
	consultants.RegisterRoutes(mux, consultantService, authMiddleware)
	settings.RegisterRoutes(mux, settingsService, authMiddleware, cfg.Env)
	catalog.RegisterRoutes(mux, catalogService, authMiddleware)
	operations.RegisterRoutes(mux, operationsService, authMiddleware)
	alerts.RegisterRoutes(mux, alertsService, authMiddleware)
	realtime.RegisterRoutes(mux, realtimeService, authMiddleware)
	reports.RegisterRoutes(mux, reportsService, authMiddleware)
	analytics.RegisterRoutes(mux, analyticsService, authMiddleware)
	access.RegisterRoutes(mux, accessService, authMiddleware)
	feedback.RegisterRoutes(mux, feedbackService, authMiddleware)
	erp.RegisterRoutes(mux, erpService, authMiddleware)
	bi.RegisterRoutes(mux, biService, authMiddleware)
	users.RegisterRoutes(mux, usersService, authMiddleware)

	// P0.5: modulos presentes no codigo mas ausentes do boot (front chamava as
	// rotas e recebia 404). site = admin de leads/produtos/tracking + ingest de
	// webhooks; operationgoals = metas (/v1/operations/goals); roadmap = /v1/roadmap/*.
	siteService := site.NewService(
		site.NewPostgresLeadRepository(pool),
		site.NewPostgresProductRepository(pool),
		site.NewPostgresWebhookSourceRepository(pool),
		site.NewPostgresTrackingRepository(pool),
	).WithProductSync(site.NewPostgresProductSourceRepository(pool), site.NewProductSourceClient()).
		WithImageCache(site.NewImageCache(cfg.UploadsDir)).
		WithProductErp(site.NewPostgresProductErpRepository(pool))
	site.RegisterAdminRoutes(mux, siteService, authMiddleware)
	site.RegisterIngestRoutes(mux, siteService)

	operationGoalsService := operationgoals.NewService(operationgoals.NewPostgresRepository(pool), storeService, realtimeService)
	operationgoals.RegisterRoutes(mux, operationGoalsService, authMiddleware)

	roadmapService := roadmap.NewService(roadmap.NewPostgresRepository(pool))
	roadmap.RegisterRoutes(mux, roadmapService, authMiddleware)

	// modulesGuard fica nil quando CoreV2 esta desligado (modo legado, sem
	// gating multi-tenant). Quando ligado, e instanciado no bloco abaixo e
	// aplicado no Chain via RequireModuleByPath.
	var modulesGuard *httpapi.AccountModulesGuard

	if cfg.CoreV2Enabled {
		ctx := context.Background()

		bus := events.NewInMemoryBus(logger)
		notificationService := notifications.NewService(
			notifications.NewPostgresRepository(pool),
			notifications.NewInAppAdapter(realtimeService),
			notifications.NewEmailAdapter(),
			notifications.NewWhatsAppAdapter(),
			notifications.NewPushAdapter(),
		)
		relationRegistry := modules.NewRelationRegistry(
			erp.NewRelationResolver(pool),
			erp.NewCRMRelationResolver(pool),
			operations.NewRelationResolver(pool),
			// calendar: resolve o vinculo evento<->task (contrato C10) para o expand de
			// relations do tasks (label "<date> - <title>", url /calendario?date=).
			calendar.NewRelationResolver(pool),
		)
		// WAVE 5 (E2): registry de sync INVERTIDO tasks->calendar. Fica vazio aqui; o handler
		// do calendar entra abaixo (LAZY), depois que o calendarModule existe. Injetado no
		// tasksModule (WithRelationSync) para o tasks avisar o calendar a cada mudanca de task.
		syncRegistry := modules.NewRelationSyncRegistry()

		registry := modules.NewRegistry(logger)
		registry.MustRegister(core.New())
		registry.MustRegister(notifications.New(notificationService))
		// tasksModule e retido para injetar seu Service no calendar (integracao C10) como
		// provider LAZY — a closure resolve o Service so no primeiro uso, imune a ordem de
		// Build no Registry.
		tasksModule := tasks.New(realtimeService, notificationService, relationRegistry, taskVideoStorage).
			WithRelationSync(syncRegistry) // WAVE 5 (E2): tasks avisa o calendar a cada mudanca de task
		registry.MustRegister(tasksModule)
		// queue e crm declaram catalogo (permissoes + role templates) para que
		// core.modules contenha "queue"/"crm". Sem isso, a seed 0124 vira no-op
		// e o RequireModuleByPath fail-close em todas as rotas de queue/crm.
		// Rotas continuam no wiring legado abaixo (Build retorna handle sem rotas).
		registry.MustRegister(queue.New())
		registry.MustRegister(crm.New())
		// automation: painel de WhatsApp/IA. Rotas montadas via handle.RegisterRoutes
		// (loop abaixo); gating em moduleGatingRules (/v1/automation -> automation).
		registry.MustRegister(automation.New())
		// meta_ads: painel de Meta/Facebook Ads. Rotas montadas via
		// handle.RegisterRoutes (loop abaixo); gating em moduleGatingRules
		// (/v1/meta-ads -> meta_ads).
		registry.MustRegister(metaads.New())
		// bio: paginas de link-in-bio servidas pelo front bio (repo separado).
		// Painel em /v1/bio (gating abaixo); rota publica /v1/public/bio/{slug}
		// fora do gate. Plano: docs/bio/PLANO_MODULO_BIO.md.
		registry.MustRegister(bio.New())
		// calendar: agenda de conteudo por cliente (eventos + notas por mes).
		// Painel em /v1/calendar (gating abaixo). Plano: docs/CALENDARIO_PLAN.md.
		// WithTasksService injeta o modulo tasks como provider LAZY (integracao C10:
		// createTask/vincular/desvincular + taskId no EventView). WithPublisher injeta o
		// realtimeService como transporte do canal calendar:account:{id} (contrato C11:
		// o Service publica create/update/delete evento, notas, day media, config e plano).
		calendarModule := calendar.New(
			calendarMediaStorage,
			calendar.WithTasksService(func() *tasks.Service { return tasksModule.Service() }),
			calendar.WithPublisher(realtimeService),
		)
		registry.MustRegister(calendarModule)
		// WAVE 5 (E2/E3): handler de sync INVERTIDO — quando uma task muda, o calendar mantem
		// o evento-espelho. Provider LAZY (resolve o Service so no 1o uso, imune a ordem de
		// Build). Registrado agora que o calendarModule existe.
		syncRegistry.Register(calendar.NewTaskSyncHandler(func() *calendar.Service { return calendarModule.Service() }))
		// cardapio: cardapios online servidos pelo front estatico no host do
		// cliente. Painel em /v1/cardapio (gating abaixo); rotas publicas
		// /v1/public/* fora do gate. Plano: docs/cardapio/PLANO_MODULO_CARDAPIO.md.
		registry.MustRegister(cardapio.New())
		// finance: planilhas financeiras mensais por cliente (substitui o mock BFF
		// Nitro — ADR 0002). Painel em /v1/finance (gating abaixo).
		// Plano: docs/finance/PLANO_MODULO_FINANCE.md.
		registry.MustRegister(finance.New())
		// tools: encurtador de link + gerador de QR Code (reconstroi os mocks do
		// projeto antigo como back real). Painel em /v1/tools (gating abaixo);
		// redirects publicos /s/{slug} e /q/{slug} fora do gate.
		// Plano: docs/tools/PLANO_MODULO_TOOLS.md.
		registry.MustRegister(tools.New())
		// omnichannel: atendimento WhatsApp (inbox + setores/filas + triagem IA),
		// schema messaging.*. Painel em /v1/omnichannel (gating abaixo); webhook inbound
		// publico /v1/webhooks/omnichannel/* FORA do gate. Plano: docs/omnichannel/PLANO_ATENDIMENTO.md.
		//
		// F3.5 (fail-fast): OMNI_SECRETS_KEY e OBRIGATORIA — a F4 e a 1a consumidora real
		// de credenciais cifradas. Sem a chave a api NAO sobe (erro nomeia a env). O Box e
		// injetado no modulo (WithSecretBox). WithPublisher liga o canal realtime da F5:
		// o realtimeService e o transporte de omnichannel:account:{id} (spec F5 — o webhook
		// inbound publica message.created; F6/F7 publicam message.updated/conversation.updated).
		omnichannelSecretBox, err := secretbox.FromEnv()
		if err != nil {
			return nil, err
		}
		registry.MustRegister(omnichannel.New(
			omnichannel.WithSecretBox(omnichannelSecretBox),
			omnichannel.WithPublisher(realtimeService),
			omnichannel.WithAutomationClientCatalog(omnichannelClientCatalogAdapter{service: tenantService}),
			omnichannel.WithAutomationBusinessContext(omnichannelCalendarContextAdapter{
				service: func() *calendar.Service { return calendarModule.Service() },
			}),
		))
		// social_publishing: agendamento e analytics de publicacoes organicas.
		// O piloto e isolado do Calendar/Crow Assistant; a credencial Instagram usa
		// o mesmo secretbox canonico da plataforma e nunca retorna ao frontend.
		registry.MustRegister(socialpublishing.New(
			socialpublishing.WithSecretBox(omnichannelSecretBox),
		))

		catalogRepo := modules.NewPostgresCatalogRepository(pool)
		if err := registry.SyncCatalog(ctx, catalogRepo); err != nil {
			return nil, err
		}

		// AccountModulesGuard ativo (C20): instancia unica, passada aos modulos
		// via Dependencies e aplicada no Chain via RequireModuleByPath. O core
		// e excecao (e o ponto de descoberta dos modulos habilitados).
		modulesGuard = httpapi.NewAccountModulesGuard(pool)

		// platform_admin gerencia TODAS as accounts e nao esta vinculado aos
		// modulos de uma account especifica — nunca e barrado pelo gating. O guard
		// roda no Chain (antes do RequireAuth por rota), entao o principal ainda
		// nao esta no contexto; autenticamos o token aqui so para checar o papel.
		modulesGuard.SetBypass(func(r *http.Request) bool {
			principal, err := authService.Authenticate(r.Context(), r.Header.Get("Authorization"))
			return err == nil && principal.Role == auth.RolePlatformAdmin
		})

		// Habilita RequireAuthWithAccount: valida membership do user na account
		// informada em X-Account-Id contra core.account_users.
		authMiddleware.SetAccountChecker(auth.NewPostgresAccountMemberChecker(pool))

		moduleHandles, err := registry.Build(modules.Dependencies{
			Pool:           pool,
			Logger:         logger,
			Bus:            bus,
			AuthMiddleware: authMiddleware,
			ModulesGuard:   modulesGuard,
			PasswordHasher: hasher,
			PrincipalCache: principalCache,
		})
		if err != nil {
			return nil, err
		}

		// Invalidacao reativa: ao habilitar/desabilitar modulos de uma account,
		// descarta o cache do guard na hora (403 module_disabled sem esperar o
		// TTL de 60s). AdminService publica este evento apos PUT .../modules.
		bus.Subscribe("account.modules.changed", func(_ context.Context, e events.Event) error {
			if e.AccountID != "" {
				modulesGuard.Invalidate(e.AccountID)
			}
			return nil
		})

		for _, h := range moduleHandles {
			h.RegisterRoutes(mux)
			h.RegisterEventHandlers(bus)
		}
	}

	middlewares := []httpapi.Middleware{
		// SecurityHeaders mais externo: headers de seguranca em TODA resposta,
		// inclusive erros/preflight. HSTS so em producao (HTTPS).
		httpapi.SecurityHeaders(strings.EqualFold(cfg.Env, "production")),
		httpapi.CORS(cfg.CORSAllowedOrigins),
		httpapi.RequestID,
		// RateLimit antes do Logging para que requisicoes bloqueadas tambem apareçam nos logs
		// com status 429. Identidade preferida = principal.UserID (via callback que evita import
		// cycle com o pacote auth); fallback para IP.
		httpapi.RateLimit(httpapi.RateLimitOptions{
			Limit:  cfg.HTTPRateLimitRequests,
			Window: cfg.HTTPRateLimitWindow,
			Resolver: func(r *http.Request) string {
				principal, ok := auth.PrincipalFromContext(r.Context())
				if !ok {
					return ""
				}
				return principal.UserID
			},
			// Cota por tenant: impede noisy-neighbor (muitos users de um tenant degradando vizinhos).
			// AccountResolver usa X-Account-Id (enviado pelo front em todas as rotas autenticadas).
			AccountResolver: func(r *http.Request) string {
				return strings.TrimSpace(r.Header.Get("X-Account-Id"))
			},
		}),
		httpapi.Logging(logger),
		httpapi.Recover(logger),
		// Gzip mais interno (depois do Logging para o status logado ser o real):
		// comprime o corpo das respostas; pula WebSocket/uploads/respostas vazias.
		httpapi.Gzip(),
	}

	// Gating multi-tenant (C20): aplicado por ultimo (mais interno) para que o
	// Logging capture o 403 module_disabled e o Recover proteja de panics. So
	// ativo quando CoreV2 esta ligado (guard instanciado no bloco acima).
	if modulesGuard != nil {
		middlewares = append(middlewares, modulesGuard.RequireModuleByPath(moduleGatingRules()))
	}

	return httpapi.Chain(mux, middlewares...), nil
}

// moduleGatingRules mapeia prefixos de path para o modulo satelite que os
// protege (gate-list do RequireModuleByPath). Espelha o MODULE_PATH_GUARDS do
// front (web/app/middleware/module-enabled.global.ts). Rotas nao listadas
// (auth, me, admin, users, notifications, access, tenants, webhooks, realtime,
// bi, roadmap, uploads, healthz) NAO sao gateadas.
func moduleGatingRules() []httpapi.ModulePathRule {
	return []httpapi.ModulePathRule{
		// queue (modulo central — sempre habilitado na seed 0124)
		{Prefix: "/v1/operations", ModuleID: "queue"},
		{Prefix: "/v1/alerts", ModuleID: "queue"},
		{Prefix: "/v1/reports", ModuleID: "queue"},
		{Prefix: "/v1/analytics", ModuleID: "queue"},
		{Prefix: "/v1/feedback", ModuleID: "queue"},
		{Prefix: "/v1/consultants", ModuleID: "queue"},
		{Prefix: "/v1/settings", ModuleID: "queue"},
		{Prefix: "/v1/stores", ModuleID: "queue"},
		// crm
		{Prefix: "/v1/erp", ModuleID: "crm"},
		{Prefix: "/v1/catalog", ModuleID: "crm"},
		// tasks
		{Prefix: "/v1/tasks", ModuleID: "tasks"},
		{Prefix: "/v1/task-boards", ModuleID: "tasks"},
		// automation (painel WhatsApp/IA). platform_admin tem bypass; contas sem o
		// modulo habilitado levam 403 module_disabled.
		{Prefix: "/v1/automation", ModuleID: "automation"},
		// meta_ads (painel Meta/Facebook Ads). Mesmo padrao: bypass admin; contas
		// sem o modulo habilitado levam 403 module_disabled.
		{Prefix: "/v1/meta-ads", ModuleID: "meta_ads"},
		// social_publishing (agendamento organico). A integracao futura com o
		// Calendar nao passa por este prefixo enquanto o piloto estiver isolado.
		{Prefix: "/v1/social-publishing", ModuleID: "social_publishing"},
		// bio (paginas de link-in-bio). Rota publica /v1/public/bio fica fora.
		{Prefix: "/v1/bio", ModuleID: "bio"},
		// calendar (agenda de conteudo). platform_admin tem bypass; contas sem o
		// modulo habilitado levam 403 module_disabled.
		{Prefix: "/v1/calendar", ModuleID: "calendar"},
		// cardapio (cardapio online). Rotas publicas /v1/public/* ficam fora.
		{Prefix: "/v1/cardapio", ModuleID: "cardapio"},
		// finance (planilhas financeiras). platform_admin tem bypass; contas sem o
		// modulo habilitado levam 403 module_disabled.
		{Prefix: "/v1/finance", ModuleID: "finance"},
		// tools (encurtador + QR). So o painel /v1/tools e gateado; os redirects
		// publicos /s/{slug} e /q/{slug} NAO estao aqui (sao abertos).
		{Prefix: "/v1/tools", ModuleID: "tools"},
		// omnichannel (atendimento WhatsApp). platform_admin tem bypass; contas sem o
		// modulo habilitado levam 403 module_disabled. So o painel /v1/omnichannel e
		// gateado: o webhook inbound (/v1/webhooks/*, F4) e o runtime NAO entram aqui —
		// mesmo precedente de /v1/public/* e /s/{slug}.
		{Prefix: "/v1/omnichannel", ModuleID: "omnichannel"},
	}
}

func nextERPScheduledRun(now time.Time, interval time.Duration, hourUTC int) time.Time {
	nowUTC := now.UTC()
	if interval > 0 && interval < 24*time.Hour {
		return nowUTC.Add(interval)
	}
	if hourUTC < 0 || hourUTC > 23 {
		hourUTC = 4
	}
	next := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), hourUTC, 0, 0, 0, time.UTC)
	if !next.After(nowUTC) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func missedERPScheduledRun(now time.Time, interval time.Duration, hourUTC int) (time.Time, bool) {
	nowUTC := now.UTC()
	if interval > 0 && interval < 24*time.Hour {
		return time.Time{}, false
	}
	if hourUTC < 0 || hourUTC > 23 {
		hourUTC = 4
	}
	scheduledToday := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), hourUTC, 0, 0, 0, time.UTC)
	if scheduledToday.After(nowUTC) {
		return time.Time{}, false
	}
	return scheduledToday, true
}

func runERPAutomaticSync(ctx context.Context, logger *slog.Logger, erpService *erp.Service, dryRun bool, scheduledFor time.Time, runKind string) {
	startedAt := time.Now().UTC()
	results, err := erpService.IngestAllStores(ctx, erp.IngestInput{
		DryRun:      dryRun,
		TriggeredBy: erp.SyncTriggeredByCron,
	})
	if err != nil {
		logger.Warn("erp_automatic_sync_failed",
			"scheduled_for", scheduledFor,
			"run_kind", runKind,
			"error", err,
		)
		return
	}

	storeCount, runCount, filesImported, fileFailures, rowsImported := summarizeERPAutomaticResults(results)
	logger.Info("erp_automatic_sync_completed",
		"scheduled_for", scheduledFor,
		"run_kind", runKind,
		"duration", time.Since(startedAt).String(),
		"stores", storeCount,
		"runs", runCount,
		"files_imported", filesImported,
		"file_failures", fileFailures,
		"rows_imported", rowsImported,
	)
}

func summarizeERPAutomaticResults(results []erp.IngestResult) (storeCount int, runCount int, filesImported int, fileFailures int, rowsImported int) {
	storeCount = len(results)
	for _, result := range results {
		runCount += len(result.RunIDs)
		filesImported += result.FilesImported
		fileFailures += len(result.FilesFailed)
		rowsImported += result.RowsImported
	}
	return storeCount, runCount, filesImported, fileFailures, rowsImported
}
