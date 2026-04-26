package app

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/analytics"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/consultants"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/realtime"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/reports"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/settings"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/users"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func BuildHTTPHandler(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) (http.Handler, error) {
	hasher := auth.NewBcryptHasher(cfg.BcryptCost)
	userStore := auth.NewPostgresUserStore(pool)
	tokenManager := auth.NewHMACTokenManager(cfg.AuthTokenSecret, cfg.AuthTokenTTL)
	avatarStorage := auth.NewDiskAvatarStorage(cfg.UploadsDir)
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
	invitationService := auth.NewInvitationService(userStore, hasher, tokenManager, cfg.WebAppURL, cfg.AuthInviteTTL)
	passwordResetService := auth.NewPasswordResetService(userStore, userStore, hasher, passwordResetDelivery, cfg.AuthPasswordResetTTL)
	authMiddleware := auth.NewMiddleware(authService)
	tenantRepository := tenants.NewPostgresRepository(pool)
	tenantService := tenants.NewService(tenantRepository)
	realtimeHub := realtime.NewHub()
	realtimeService := realtime.NewService(authService, nil, tenantService, cfg.CORSAllowedOrigins, realtimeHub)
	authService.SetContextPublisher(realtimeService)
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
	operationsRepository := operations.NewPostgresRepository(pool)
	operationsService := operations.NewService(operationsRepository, realtimeService, newOperationsStoreScopeAdapter(storeService))
	reportsRepository := reports.NewPostgresRepository(pool)
	reportsService := reports.NewService(reportsRepository, storeService)
	analyticsRepository := analytics.NewPostgresRepository(pool)
	analyticsService := analytics.NewService(analyticsRepository, storeService)
	usersService := users.NewService(usersRepository, hasher, invitationService, realtimeService, consultantProfileSync)

	mux := http.NewServeMux()
	if strings.TrimSpace(cfg.UploadsDir) != "" {
		fileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir)))
		mux.Handle("GET /uploads/", fileServer)
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"service": cfg.AppName,
			"status":  "ok",
			"modules": []string{
				"auth",
				"tenants",
				"stores",
				"consultants",
				"settings",
				"operations",
				"realtime",
				"reports",
				"analytics",
				"access",
				"users",
			},
			"tenantMode": "owner-is-client",
		})
	})

	auth.RegisterRoutes(mux, authService, invitationService, passwordResetService, authMiddleware)
	registerContextRoutes(mux, authService, authMiddleware, tenantService, storeService)
	tenants.RegisterRoutes(mux, tenantService, authMiddleware)
	stores.RegisterRoutes(mux, storeService, authMiddleware)
	consultants.RegisterRoutes(mux, consultantService, authMiddleware)
	settings.RegisterRoutes(mux, settingsService, authMiddleware)
	operations.RegisterRoutes(mux, operationsService, authMiddleware)
	realtime.RegisterRoutes(mux, realtimeService)
	reports.RegisterRoutes(mux, reportsService, authMiddleware)
	analytics.RegisterRoutes(mux, analyticsService, authMiddleware)
	access.RegisterRoutes(mux, accessService, authMiddleware)
	users.RegisterRoutes(mux, usersService, authMiddleware)

	return httpapi.Chain(
		mux,
		httpapi.CORS(cfg.CORSAllowedOrigins),
		httpapi.RequestID,
		httpapi.Logging(logger),
		httpapi.Recover(logger),
	), nil
}
