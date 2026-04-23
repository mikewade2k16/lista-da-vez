package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                   string
	Env                       string
	HTTPAddr                  string
	WebAppURL                 string
	UploadsDir                string
	DatabaseURL               string
	DatabaseMinConns          int
	DatabaseMaxConns          int
	CORSAllowedOrigins        []string
	AuthTokenSecret           string
	AuthTokenTTL              time.Duration
	AuthInviteTTL             time.Duration
	AuthPasswordResetTTL      time.Duration
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string
	SMTPFromEmail             string
	SMTPFromName              string
	SMTPTLSMode               string
	SMTPInsecureSkipVerify    bool
	SMTPTimeout               time.Duration
	BcryptCost                int
	ConsultantEmailDomain     string
	ConsultantDefaultPassword string
}

func Load() Config {
	return Config{
		AppName:          getEnv("APP_NAME", "lista-da-vez-api"),
		Env:              getEnv("APP_ENV", "development"),
		HTTPAddr:         getEnv("APP_ADDR", ":8080"),
		WebAppURL:        getEnv("WEB_APP_URL", "http://localhost:3003"),
		UploadsDir:       getEnv("UPLOADS_DIR", "data/uploads"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		DatabaseMinConns: getEnvInt("DATABASE_MIN_CONNS", 0),
		DatabaseMaxConns: getEnvInt("DATABASE_MAX_CONNS", 10),
		CORSAllowedOrigins: getEnvCSV(
			"CORS_ALLOWED_ORIGINS",
			[]string{
				"http://localhost:*",
				"http://127.0.0.1:*",
				"http://[::1]:*",
			},
		),
		AuthTokenSecret:           getEnv("AUTH_TOKEN_SECRET", "dev-secret-change-me"),
		AuthTokenTTL:              getEnvDuration("AUTH_TOKEN_TTL", 12*time.Hour),
		AuthInviteTTL:             getEnvDuration("AUTH_INVITE_TTL", 7*24*time.Hour),
		AuthPasswordResetTTL:      getEnvDuration("AUTH_PASSWORD_RESET_TTL", 30*time.Minute),
		SMTPHost:                  getEnv("SMTP_HOST", ""),
		SMTPPort:                  getEnvInt("SMTP_PORT", 587),
		SMTPUsername:              getEnv("SMTP_USERNAME", ""),
		SMTPPassword:              getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail:             getEnv("SMTP_FROM_EMAIL", ""),
		SMTPFromName:              getEnv("SMTP_FROM_NAME", "Lista da Vez"),
		SMTPTLSMode:               getEnv("SMTP_TLS_MODE", "starttls"),
		SMTPInsecureSkipVerify:    getEnvBool("SMTP_INSECURE_SKIP_VERIFY", false),
		SMTPTimeout:               getEnvDuration("SMTP_TIMEOUT", 10*time.Second),
		BcryptCost:                getEnvInt("AUTH_BCRYPT_COST", 10),
		ConsultantEmailDomain:     getEnv("AUTH_CONSULTANT_EMAIL_DOMAIN", "acesso.omni.local"),
		ConsultantDefaultPassword: getEnv("AUTH_CONSULTANT_DEFAULT_PASSWORD", "Omni@123"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}

	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getEnvCSV(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return append([]string{}, fallback...)
	}

	items := strings.Split(raw, ",")
	values := make([]string, 0, len(items))

	for _, item := range items {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}

		values = append(values, normalized)
	}

	if len(values) == 0 {
		return append([]string{}, fallback...)
	}

	return values
}
