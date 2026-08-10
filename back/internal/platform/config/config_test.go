package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadHTTPRateLimitDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_RATE_LIMIT_REQUESTS", "")
	t.Setenv("HTTP_RATE_LIMIT_WINDOW", "")

	cfg := Load()

	if cfg.HTTPRateLimitRequests != 1200 {
		t.Fatalf("expected default HTTPRateLimitRequests=1200, got %d", cfg.HTTPRateLimitRequests)
	}
	if cfg.HTTPRateLimitWindow != time.Minute {
		t.Fatalf("expected default HTTPRateLimitWindow=%s, got %s", time.Minute, cfg.HTTPRateLimitWindow)
	}
}

func TestLoadHTTPRateLimitProductionDefault(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_RATE_LIMIT_REQUESTS", "")
	t.Setenv("HTTP_RATE_LIMIT_WINDOW", "")

	cfg := Load()

	if cfg.HTTPRateLimitRequests != 300 {
		t.Fatalf("expected production default HTTPRateLimitRequests=300, got %d", cfg.HTTPRateLimitRequests)
	}
	if cfg.HTTPRateLimitWindow != time.Minute {
		t.Fatalf("expected production default HTTPRateLimitWindow=%s, got %s", time.Minute, cfg.HTTPRateLimitWindow)
	}
}

func TestLoadHTTPRateLimitDockerDefault(t *testing.T) {
	t.Setenv("APP_ENV", "docker")
	t.Setenv("HTTP_RATE_LIMIT_REQUESTS", "")
	t.Setenv("HTTP_RATE_LIMIT_WINDOW", "")

	cfg := Load()

	if cfg.HTTPRateLimitRequests != 1200 {
		t.Fatalf("expected docker default HTTPRateLimitRequests=1200, got %d", cfg.HTTPRateLimitRequests)
	}
	if cfg.HTTPRateLimitWindow != time.Minute {
		t.Fatalf("expected docker default HTTPRateLimitWindow=%s, got %s", time.Minute, cfg.HTTPRateLimitWindow)
	}
}

func TestLoadHTTPRateLimitOverrides(t *testing.T) {
	t.Setenv("HTTP_RATE_LIMIT_REQUESTS", "180")
	t.Setenv("HTTP_RATE_LIMIT_WINDOW", "30s")

	cfg := Load()

	if cfg.HTTPRateLimitRequests != 180 {
		t.Fatalf("expected HTTPRateLimitRequests=180, got %d", cfg.HTTPRateLimitRequests)
	}
	if cfg.HTTPRateLimitWindow != 30*time.Second {
		t.Fatalf("expected HTTPRateLimitWindow=%s, got %s", 30*time.Second, cfg.HTTPRateLimitWindow)
	}
}

func TestLoadR2NonEmptyBucketInitializationDefaultsToDisabled(t *testing.T) {
	t.Setenv("R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION", "")

	if cfg := Load(); cfg.R2AllowNonEmptyBucketInitialization {
		t.Fatal("expected non-empty R2 bucket initialization to default to false")
	}
}

func TestLoadR2NonEmptyBucketInitializationExplicitlyEnabled(t *testing.T) {
	t.Setenv("R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION", "true")

	if cfg := Load(); !cfg.R2AllowNonEmptyBucketInitialization {
		t.Fatal("expected explicit non-empty R2 bucket initialization to be enabled")
	}
}

func TestValidateNoOpInDevelopment(t *testing.T) {
	cfg := Config{Env: "development", AuthTokenSecret: devTokenSecretDefault, BcryptCost: 4}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error in development, got %v", err)
	}
}

func TestValidateRejectsProductionWithDevSecret(t *testing.T) {
	cfg := Config{Env: "production", AuthTokenSecret: devTokenSecretDefault, BcryptCost: productionMinBcrypt}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when production uses default secret, got nil")
	}
	if !strings.Contains(err.Error(), "AUTH_TOKEN_SECRET") {
		t.Fatalf("expected error to mention AUTH_TOKEN_SECRET, got %q", err.Error())
	}
}

func TestValidateRejectsProductionWithEmptySecret(t *testing.T) {
	cfg := Config{Env: "production", AuthTokenSecret: "", BcryptCost: productionMinBcrypt}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when production has empty secret, got nil")
	}
	if !strings.Contains(err.Error(), "AUTH_TOKEN_SECRET") {
		t.Fatalf("expected error to mention AUTH_TOKEN_SECRET, got %q", err.Error())
	}
}

func TestValidateRejectsProductionWithLowBcryptCost(t *testing.T) {
	cfg := Config{Env: "production", AuthTokenSecret: "secret-real-aqui", BcryptCost: 4}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when bcrypt cost < 10 in production, got nil")
	}
	if !strings.Contains(err.Error(), "AUTH_BCRYPT_COST") {
		t.Fatalf("expected error to mention AUTH_BCRYPT_COST, got %q", err.Error())
	}
}

func TestValidateAcceptsProductionWithSecureValues(t *testing.T) {
	cfg := Config{
		Env:             "production",
		AuthTokenSecret: "secret-real-aqui",
		BcryptCost:      productionMinBcrypt,
		DatabaseAppURL:  "postgres://omni_app:secure-password@postgres:5432/omni",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error with secure production config, got %v", err)
	}
}

func TestValidateRejectsR2WithoutCoreV2InProduction(t *testing.T) {
	cfg := Config{
		Env:             "production",
		AuthTokenSecret: "secret-real-aqui",
		BcryptCost:      productionMinBcrypt,
		DatabaseAppURL:  "postgres://omni_app:secure-password@postgres:5432/omni",
		R2Enabled:       true,
		CoreV2Enabled:   false,
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "CORE_V2_ENABLED") {
		t.Fatalf("expected R2/CoreV2 validation error, got %v", err)
	}
}
