package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	devTokenSecretDefault = "dev-secret-change-me"
	productionMinBcrypt   = 10
)

type Config struct {
	AppName                       string
	Env                           string
	HTTPAddr                      string
	HTTPRateLimitRequests         int
	HTTPRateLimitWindow           time.Duration
	WebAppURL                     string
	UploadsDir                    string
	ERPSourceKind                 string
	ERPSourceRecursive            bool
	ERPSourceDir                  string
	ERPLocalSourceDir             string
	ERPStorageDir                 string
	ERPBootstrapItemFile          string
	ERPBootstrapCustomerFile      string
	ERPBootstrapEmployeeFile      string
	ERPBootstrapOrderFile         string
	ERPBootstrapOrderCanceledFile string
	ERPAllowManualSync            bool
	ERPFTPHost                    string
	ERPFTPPort                    int
	ERPFTPUser                    string
	ERPFTPPassword                string
	ERPFTPKeyPath                 string
	ERPFTPRemoteDir               string
	ERPFTPHostKey                 string
	ERPRootStoreCode              string
	ERPSyncInterval               time.Duration
	ERPSyncHourUTC                int
	ERPSyncAutomaticEnabled       bool
	ERPSyncDryRunDefault          bool
	ERPCSVMaxBytes                int64
	ERPManualSyncMaxFiles         int
	ERPBackfillMaxFiles           int
	ERPManualSyncMinInterval      time.Duration
	PerolaBICompanyKey            string
	PerolaBILogin                 string
	PerolaBIPass                  string
	PerolaBIStaticToken           string
	PerolaBICNPJEmpresa           string
	PerolaBITokenTTL              time.Duration
	PerolaBIRequestTimeout        time.Duration
	PerolaBIPageLimit             int
	PerolaBIMaxPages              int
	DatabaseURL                   string
	DatabaseMinConns              int
	DatabaseMaxConns              int
	CORSAllowedOrigins            []string
	AuthTokenSecret               string
	AuthTokenTTL                  time.Duration
	AuthInviteTTL                 time.Duration
	AuthPasswordResetTTL          time.Duration
	SMTPHost                      string
	SMTPPort                      int
	SMTPUsername                  string
	SMTPPassword                  string
	SMTPFromEmail                 string
	SMTPFromName                  string
	SMTPTLSMode                   string
	SMTPInsecureSkipVerify        bool
	SMTPTimeout                   time.Duration
	BcryptCost                    int
	ConsultantEmailDomain         string
	ConsultantDefaultPassword     string
	CoreV2Enabled                 bool
}

func Load() Config {
	env := getEnv("APP_ENV", "development")
	return Config{
		AppName:               getEnv("APP_NAME", "omni-api"),
		Env:                   env,
		HTTPAddr:              getEnv("APP_ADDR", ":8080"),
		HTTPRateLimitRequests: getEnvInt("HTTP_RATE_LIMIT_REQUESTS", defaultHTTPRateLimitRequests(env)),
		HTTPRateLimitWindow:   getEnvDuration("HTTP_RATE_LIMIT_WINDOW", time.Minute),
		WebAppURL:             getEnv("WEB_APP_URL", "http://localhost:3003"),
		UploadsDir:            getEnv("UPLOADS_DIR", "data/uploads"),
		ERPSourceKind:         getEnv("ERP_SOURCE_KIND", "local"),
		ERPSourceRecursive:    getEnvBool("ERP_SOURCE_RECURSIVE", false),
		ERPSourceDir:          getEnv("ERP_SOURCE_DIR", ""),
		ERPLocalSourceDir: getEnv(
			"ERP_LOCAL_SOURCE_DIR",
			getEnv("ERP_SOURCE_DIR", ""),
		),
		ERPStorageDir: getEnv("ERP_STORAGE_DIR", "data/erp"),
		ERPBootstrapItemFile: getEnv(
			"ERP_BOOTSTRAP_ITEM_FILE",
			"consolidados/184/item_184_consolidado.md",
		),
		ERPBootstrapCustomerFile: getEnv(
			"ERP_BOOTSTRAP_CUSTOMER_FILE",
			"consolidados/184/customer_184_consolidado.md",
		),
		ERPBootstrapEmployeeFile: getEnv(
			"ERP_BOOTSTRAP_EMPLOYEE_FILE",
			"consolidados/184/employee_184_consolidado.md",
		),
		ERPBootstrapOrderFile: getEnv(
			"ERP_BOOTSTRAP_ORDER_FILE",
			"consolidados/184/order_184_consolidado.md",
		),
		ERPBootstrapOrderCanceledFile: getEnv(
			"ERP_BOOTSTRAP_ORDER_CANCELED_FILE",
			"consolidados/184/ordercanceled_184_consolidado.md",
		),
		ERPAllowManualSync:      getEnvBool("ERP_ALLOW_MANUAL_SYNC", false),
		ERPFTPHost:              getEnv("ERP_FTP_HOST", ""),
		ERPFTPPort:              getEnvInt("ERP_FTP_PORT", 0),
		ERPFTPUser:              getEnv("ERP_FTP_USER", ""),
		ERPFTPPassword:          getEnv("ERP_FTP_PASSWORD", ""),
		ERPFTPKeyPath:           getEnv("ERP_FTP_KEY_PATH", ""),
		ERPFTPRemoteDir:         getEnv("ERP_FTP_REMOTE_DIR", ""),
		ERPFTPHostKey:           getEnv("ERP_FTP_HOST_KEY", ""),
		ERPRootStoreCode:        getEnv("ERP_ROOT_STORE_CODE", getEnv("ERP_BOOTSTRAP_STORE_CODE", "")),
		ERPSyncInterval:         getEnvDuration("ERP_SYNC_INTERVAL", 24*time.Hour),
		ERPSyncHourUTC:          getEnvInt("ERP_SYNC_HOUR_UTC", 4),
		ERPSyncAutomaticEnabled: getEnvBool("ERP_SYNC_AUTOMATIC_ENABLED", false),
		ERPSyncDryRunDefault:    getEnvBool("ERP_SYNC_DRY_RUN_DEFAULT", false),
		ERPCSVMaxBytes:          getEnvInt64("ERP_CSV_MAX_BYTES", 128*1024*1024),
		ERPManualSyncMaxFiles:   getEnvInt("ERP_MANUAL_SYNC_MAX_FILES", 100),
		ERPBackfillMaxFiles:     getEnvInt("ERP_BACKFILL_MAX_FILES", 1000),
		ERPManualSyncMinInterval: getEnvDuration(
			"ERP_MANUAL_SYNC_MIN_INTERVAL",
			5*time.Minute,
		),
		PerolaBICompanyKey:  getEnv("PEROLA_BI_COMPANY_KEY", ""),
		PerolaBILogin:       getEnv("PEROLA_BI_LOGIN", ""),
		PerolaBIPass:        getEnv("PEROLA_BI_PASS", ""),
		PerolaBIStaticToken: getEnv("PEROLA_BI_TOKEN", ""),
		PerolaBICNPJEmpresa: getEnv("PEROLA_BI_CNPJ_EMPRESA", ""),
		PerolaBITokenTTL:    getEnvDuration("PEROLA_BI_TOKEN_TTL", 50*time.Minute),
		PerolaBIRequestTimeout: getEnvDuration(
			"PEROLA_BI_REQUEST_TIMEOUT",
			12*time.Second,
		),
		PerolaBIPageLimit: getEnvInt("PEROLA_BI_PAGE_LIMIT", 100),
		PerolaBIMaxPages:  getEnvInt("PEROLA_BI_MAX_PAGES", 50),
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		DatabaseMinConns:  getEnvInt("DATABASE_MIN_CONNS", 0),
		DatabaseMaxConns:  getEnvInt("DATABASE_MAX_CONNS", 10),
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
		SMTPFromName:              getEnv("SMTP_FROM_NAME", "Omni"),
		SMTPTLSMode:               getEnv("SMTP_TLS_MODE", "starttls"),
		SMTPInsecureSkipVerify:    getEnvBool("SMTP_INSECURE_SKIP_VERIFY", false),
		SMTPTimeout:               getEnvDuration("SMTP_TIMEOUT", 10*time.Second),
		BcryptCost:                getEnvInt("AUTH_BCRYPT_COST", 10),
		ConsultantEmailDomain:     getEnv("AUTH_CONSULTANT_EMAIL_DOMAIN", "acesso.omni.local"),
		ConsultantDefaultPassword: getEnv("AUTH_CONSULTANT_DEFAULT_PASSWORD", "Omni@123"),
		CoreV2Enabled:             getEnvBool("CORE_V2_ENABLED", false),
	}
}

// Validate runs production-only sanity checks on the loaded configuration.
// Em dev/docker o método é no-op para não atrapalhar onboarding local.
// Em production aborta o boot se algum default inseguro escapou para a VPS.
func (cfg Config) Validate() error {
	if !strings.EqualFold(strings.TrimSpace(cfg.Env), "production") {
		return nil
	}

	var problems []string

	if cfg.AuthTokenSecret == "" || cfg.AuthTokenSecret == devTokenSecretDefault {
		problems = append(problems, "AUTH_TOKEN_SECRET ainda usa o default de dev")
	}
	if cfg.BcryptCost < productionMinBcrypt {
		problems = append(problems, fmt.Sprintf("AUTH_BCRYPT_COST=%d é menor que o mínimo recomendado para produção (%d)", cfg.BcryptCost, productionMinBcrypt))
	}

	if len(problems) > 0 {
		return fmt.Errorf("configuração inválida para ambiente production: %s", strings.Join(problems, "; "))
	}

	return nil
}

func defaultHTTPRateLimitRequests(env string) int {
	normalizedEnv := strings.TrimSpace(env)
	if strings.EqualFold(normalizedEnv, "development") || strings.EqualFold(normalizedEnv, "docker") {
		return 1200
	}
	return 300
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

func getEnvInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseInt(raw, 10, 64)
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
