package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const omnichannelOperationalRunbook = "docs/omnichannel/runbooks/OPERACAO_E9.md"

type OperationalComponentHealth struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type OperationalBacklogHealth struct {
	Pending              int64 `json:"pending"`
	Processing           int64 `json:"processing"`
	Dead                 int64 `json:"dead"`
	OldestPendingSeconds int64 `json:"oldestPendingSeconds"`
}

type OperationalAIHealth struct {
	Queued          int64 `json:"queued"`
	Processing      int64 `json:"processing"`
	StuckProcessing int64 `json:"stuckProcessing"`
	Failed24h       int64 `json:"failed24h"`
}

type OperationalProviderHealth struct {
	ActiveInstances       int64      `json:"activeInstances"`
	MissingCredentials    int64      `json:"missingCredentials"`
	WebhookEvents24h      int64      `json:"webhookEvents24h"`
	LastWebhookReceivedAt *time.Time `json:"lastWebhookReceivedAt"`
}

type OperationalRetentionHealth struct {
	LastFinishedAt *time.Time `json:"lastFinishedAt"`
	LastError      string     `json:"lastError"`
}

type OperationalBindingHealth struct {
	EnabledProfiles int64 `json:"enabledProfiles"`
	Mismatches      int64 `json:"mismatches"`
}

type OperationalAlert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Action   string `json:"action"`
	Owner    string `json:"owner"`
	Runbook  string `json:"runbook"`
}

type OperationalHealthView struct {
	Status      string                     `json:"status"`
	GeneratedAt time.Time                  `json:"generatedAt"`
	Process     OperationalComponentHealth `json:"process"`
	Database    OperationalComponentHealth `json:"database"`
	N8N         OperationalComponentHealth `json:"n8n"`
	Outbox      OperationalBacklogHealth   `json:"outbox"`
	AI          OperationalAIHealth        `json:"ai"`
	Provider    OperationalProviderHealth  `json:"provider"`
	Retention   OperationalRetentionHealth `json:"retention"`
	Bindings    OperationalBindingHealth   `json:"bindings"`
	Alerts      []OperationalAlert         `json:"alerts"`
}

type OperationalService struct {
	store         *Store
	n8nConfigured bool
	now           func() time.Time
}

func NewOperationalService(store *Store, n8nConfigured bool) *OperationalService {
	return &OperationalService{store: store, n8nConfigured: n8nConfigured, now: time.Now}
}

func (s *OperationalService) Health(ctx context.Context, accountID, userID string) (OperationalHealthView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, userID)
	if err != nil {
		return OperationalHealthView{}, err
	}
	if !scope.Eligible || (!scope.allowsPermission("omnichannel.audit.view") &&
		!scope.allowsPermission("omnichannel.settings.manage")) {
		return OperationalHealthView{}, ErrForbidden
	}

	view := OperationalHealthView{
		Status:      "ok",
		GeneratedAt: s.now().UTC(),
		Process:     OperationalComponentHealth{Status: "ok", Detail: "api_process_ready"},
		Database:    OperationalComponentHealth{Status: "ok", Detail: "postgres_ready"},
		Alerts:      make([]OperationalAlert, 0),
	}
	if s.n8nConfigured {
		view.N8N = OperationalComponentHealth{Status: "configured", Detail: "runtime_probe_not_persisted"}
	} else {
		view.N8N = OperationalComponentHealth{Status: "disabled", Detail: "native_executor_or_not_configured"}
	}
	if err := s.store.LoadOperationalHealth(ctx, accountID, &view); err != nil {
		return OperationalHealthView{}, err
	}
	view.Alerts = deriveOperationalAlerts(view, s.now().UTC())
	for _, alert := range view.Alerts {
		if alert.Severity == "critical" || alert.Severity == "warning" {
			view.Status = "degraded"
			break
		}
	}
	return view, nil
}

func (s *Store) LoadOperationalHealth(ctx context.Context, accountID string, view *OperationalHealthView) error {
	if err := s.pool.QueryRow(ctx, `select
		count(*) filter (where status='pending'),
		count(*) filter (where status='processing'),
		count(*) filter (where status='dead'),
		coalesce(extract(epoch from (now()-min(created_at) filter (where status='pending')))::bigint,0)
		from messaging.outbox where account_id=$1::uuid`, accountID).
		Scan(&view.Outbox.Pending, &view.Outbox.Processing, &view.Outbox.Dead,
			&view.Outbox.OldestPendingSeconds); err != nil {
		return err
	}
	if err := s.pool.QueryRow(ctx, `select
		count(*) filter (where status in ('buffering','queued')),
		count(*) filter (where status='processing'),
		count(*) filter (where status='processing' and updated_at < now()-interval '2 minutes'),
		count(*) filter (where status='failed' and updated_at >= now()-interval '24 hours')
		from messaging.ai_dispatches where account_id=$1::uuid`, accountID).
		Scan(&view.AI.Queued, &view.AI.Processing, &view.AI.StuckProcessing, &view.AI.Failed24h); err != nil {
		return err
	}
	if err := s.pool.QueryRow(ctx, `select
		(select count(*) from messaging.whatsapp_instances
		 where account_id=$1::uuid and is_active),
		(select count(*) from messaging.whatsapp_instances
		 where account_id=$1::uuid and is_active and coalesce(credentials_ciphertext,'')=''),
		count(*) filter (where received_at >= now()-interval '24 hours'), max(received_at)
		from messaging.webhook_events where account_id=$1::uuid`, accountID).
		Scan(&view.Provider.ActiveInstances, &view.Provider.MissingCredentials,
			&view.Provider.WebhookEvents24h, &view.Provider.LastWebhookReceivedAt); err != nil {
		return err
	}
	if err := s.pool.QueryRow(ctx, `select finished_at, error from messaging.purge_runs
		where account_id=$1::uuid order by started_at desc,id desc limit 1`, accountID).
		Scan(&view.Retention.LastFinishedAt, &view.Retention.LastError); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return s.pool.QueryRow(ctx, `select
		count(*) filter (where profile.enabled),
		count(*) filter (where profile.enabled and not exists (
			select 1 from messaging.channel_client_bindings binding
			where binding.account_id=profile.account_id
			  and binding.client_account_id=profile.client_account_id
			  and binding.whatsapp_instance_id=profile.whatsapp_instance_id
			  and binding.channel='WHATSAPP'
			  and binding.effective_from <= now()
			  and (binding.effective_to is null or binding.effective_to > now())
		))
		from messaging.automation_profiles profile where profile.account_id=$1::uuid`, accountID).
		Scan(&view.Bindings.EnabledProfiles, &view.Bindings.Mismatches)
}

func deriveOperationalAlerts(view OperationalHealthView, now time.Time) []OperationalAlert {
	alerts := make([]OperationalAlert, 0, 6)
	add := func(code, severity, message, action string) {
		alerts = append(alerts, OperationalAlert{
			Code: code, Severity: severity, Message: message, Action: action,
			Owner: "omnichannel", Runbook: omnichannelOperationalRunbook,
		})
	}
	if view.Outbox.Dead > 0 {
		add("outbox_dead", "critical", "Há jobs em dead-letter.", "Inspecionar por conta/kind e reprocessar somente itens idempotentes.")
	}
	if view.Outbox.OldestPendingSeconds > 120 {
		add("outbox_backlog", "warning", "O job pendente mais antigo excedeu 120 segundos.", "Verificar worker, provider e ordering key antes de qualquer retry.")
	}
	if view.AI.StuckProcessing > 0 {
		add("ai_dispatch_stuck", "critical", "Há dispatches de IA presos além de dois minutos.", "Acionar recuperação de lease e confirmar geração antes de reprocessar.")
	}
	if view.Provider.MissingCredentials > 0 {
		add("provider_credentials_missing", "warning", "Há instância ativa sem credencial configurada.", "Configurar a credencial ou desativar a instância até o onboarding.")
	}
	if view.Bindings.Mismatches > 0 {
		add("automation_binding_mismatch", "critical", "Há automação habilitada sem vínculo cliente×número ativo.", "Desabilitar a automação ou corrigir o vínculo antes de retomar a IA.")
	}
	if strings.TrimSpace(view.Retention.LastError) != "" {
		add("retention_failed", "critical", "A última execução de retenção terminou com erro.", "Executar dry-run tenant-scoped e corrigir a classe indicada.")
	} else if view.Retention.LastFinishedAt == nil || now.Sub(*view.Retention.LastFinishedAt) > 48*time.Hour {
		add("retention_stale", "warning", "Não há purge concluído nas últimas 48 horas.", "Verificar scheduler e executar primeiro em dry-run.")
	}
	return alerts
}
