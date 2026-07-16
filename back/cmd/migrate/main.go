package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout())
	defer cancel()

	pool, err := database.OpenPool(ctx, cfg)
	if err != nil {
		logger.Error("database_connect_failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "up":
		if err := database.ApplyMigrationsWithOptions(ctx, pool, database.MigrationOptions{
			SkipDataSeeds: strings.EqualFold(cfg.Env, "production"),
		}); err != nil {
			logger.Error("migration_up_failed", slog.Any("error", err))
			os.Exit(1)
		}

		logger.Info("migration_up_ok")

		ensured, err := database.EnsureAppRole(ctx, pool, cfg.DatabaseAppURL)
		if err != nil {
			logger.Error("app_role_ensure_failed", slog.Any("error", err))
			os.Exit(1)
		}
		switch {
		case ensured.SkipReason == "":
			logger.Info("app_role_ensure_ok", slog.Bool("created", ensured.Created))
		case strings.EqualFold(cfg.Env, "production") &&
			(ensured.SkipReason == "empty_password" || ensured.SkipReason == "empty_url"):
			// Fail-fast: subir a api sem role utilizável = crash-loop 28P01
			// (incidente 2026-07-03). Melhor derrubar o migrate com motivo claro.
			logger.Error("app_role_ensure_failed", slog.String("reason", ensured.SkipReason))
			os.Exit(1)
		default:
			logger.Info("app_role_ensure_skipped", slog.String("reason", ensured.SkipReason))
		}

		granted, err := database.SyncAppRoleGrants(ctx, pool, cfg.DatabaseAppURL)
		if err != nil {
			logger.Error("app_role_grants_failed", slog.Any("error", err))
			os.Exit(1)
		}
		if granted {
			logger.Info("app_role_grants_ok")
		} else {
			logger.Info("app_role_grants_skipped") // sem DATABASE_APP_URL, mesma role, ou role ausente
		}
	case "bootstrap-owner":
		password := strings.TrimSpace(os.Getenv("BOOTSTRAP_OWNER_PASSWORD"))
		if password == "" {
			logger.Error("bootstrap_owner_failed", slog.String("error", "BOOTSTRAP_OWNER_PASSWORD is required"))
			os.Exit(1)
		}

		hasher := auth.NewBcryptHasher(cfg.BcryptCost)
		passwordHash, err := hasher.Hash(password)
		if err != nil {
			logger.Error("bootstrap_owner_failed", slog.Any("error", err))
			os.Exit(1)
		}

		if err := database.BootstrapInitialOwner(ctx, pool, database.InitialOwnerBootstrapInput{
			TenantSlug:        os.Getenv("BOOTSTRAP_TENANT_SLUG"),
			TenantName:        os.Getenv("BOOTSTRAP_TENANT_NAME"),
			StoreCode:         os.Getenv("BOOTSTRAP_STORE_CODE"),
			StoreName:         os.Getenv("BOOTSTRAP_STORE_NAME"),
			StoreCity:         os.Getenv("BOOTSTRAP_STORE_CITY"),
			OwnerName:         os.Getenv("BOOTSTRAP_OWNER_NAME"),
			OwnerEmail:        os.Getenv("BOOTSTRAP_OWNER_EMAIL"),
			OwnerPasswordHash: passwordHash,
		}); err != nil {
			logger.Error("bootstrap_owner_failed", slog.Any("error", err))
			os.Exit(1)
		}

		logger.Info("bootstrap_owner_ok")
	case "bootstrap-erp-store":
		result, err := database.BootstrapERPStore(ctx, pool, database.ERPStoreBootstrapInput{
			TenantID:   os.Getenv("ERP_BOOTSTRAP_TENANT_ID"),
			TenantSlug: os.Getenv("ERP_BOOTSTRAP_TENANT_SLUG"),
			StoreCode:  os.Getenv("ERP_BOOTSTRAP_STORE_CODE"),
			StoreName:  os.Getenv("ERP_BOOTSTRAP_STORE_NAME"),
			StoreCity:  os.Getenv("ERP_BOOTSTRAP_STORE_CITY"),
		})
		if err != nil {
			logger.Error("erp_store_bootstrap_failed", slog.Any("error", err))
			os.Exit(1)
		}
		if !result.Bootstrapped {
			logger.Info(
				"erp_store_bootstrap_skipped",
				slog.String("reason", result.Reason),
				slog.String("storeCode", result.StoreCode),
			)
			return
		}

		logger.Info(
			"erp_store_bootstrap_ok",
			slog.String("tenantId", result.TenantID),
			slog.String("storeId", result.StoreID),
			slog.String("storeCode", result.StoreCode),
		)
	case "status":
		applied, err := database.ListAppliedMigrations(ctx, pool)
		if err != nil {
			logger.Error("migration_status_failed", slog.Any("error", err))
			os.Exit(1)
		}

		for _, item := range applied {
			fmt.Printf("%s %s %s\n", item.Version, item.AppliedAt.Format(time.RFC3339), item.Name)
		}
	default:
		logger.Error("unknown_command", slog.String("command", command))
		os.Exit(1)
	}
}

func migrationTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("MIGRATION_TIMEOUT"))
	if raw == "" {
		return 5 * time.Minute
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 5 * time.Minute
	}

	return value
}
