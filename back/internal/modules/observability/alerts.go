package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrAlertsNotConfigured: sem SMTP ou sem destinatários em OBS_ALERT_EMAILS.
var ErrAlertsNotConfigured = errors.New("alerts not configured")

const emailNotConfigured = "email nao configurado"

// Sender é o transporte de e-mail (implementado por platform/mailer).
type Sender interface {
	Configured() bool
	Send(ctx context.Context, to []string, subject, textBody string) error
}

type AlertConfig struct {
	Enabled    bool
	Recipients []string
	Renotify   time.Duration
	Interval   time.Duration
	LinkBase   string // URL pública do web, sem barra final
}

// AlertConfigFromEnv lê OBS_ALERT_EMAILS, OBS_ALERT_RENOTIFY_MINUTES (60),
// OBS_ALERT_INTERVAL (60s) e OBS_ALERTS_ENABLED (true).
func AlertConfigFromEnv(webAppURL string) AlertConfig {
	cfg := AlertConfig{
		Enabled:  true,
		Renotify: 60 * time.Minute,
		Interval: 60 * time.Second,
		LinkBase: strings.TrimRight(strings.TrimSpace(webAppURL), "/"),
	}
	if value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("OBS_ALERTS_ENABLED"))); err == nil {
		cfg.Enabled = value
	}
	if minutes, err := strconv.Atoi(strings.TrimSpace(os.Getenv("OBS_ALERT_RENOTIFY_MINUTES"))); err == nil && minutes > 0 {
		cfg.Renotify = time.Duration(minutes) * time.Minute
	}
	if interval, err := time.ParseDuration(strings.TrimSpace(os.Getenv("OBS_ALERT_INTERVAL"))); err == nil && interval >= time.Second {
		cfg.Interval = interval
	}
	for _, part := range strings.Split(os.Getenv("OBS_ALERT_EMAILS"), ",") {
		if part = strings.TrimSpace(part); part != "" {
			cfg.Recipients = append(cfg.Recipients, part)
		}
	}
	return cfg
}

func (c AlertConfig) panelLink() string {
	if c.LinkBase == "" {
		return ""
	}
	return c.LinkBase + "/manage/observabilidade"
}

// Alerts é o motor: avalia as regras, mantém obs.alerts e notifica por e-mail.
type Alerts struct {
	store  *Store
	sender Sender
	cfg    AlertConfig
	logger *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewAlerts(store *Store, sender Sender, cfg AlertConfig, logger *slog.Logger) *Alerts {
	if logger == nil {
		logger = slog.Default()
	}
	return &Alerts{store: store, sender: sender, cfg: cfg, logger: logger}
}

// Configured: canal de e-mail utilizável (SMTP + ao menos um destinatário).
func (a *Alerts) Configured() bool {
	return a != nil && a.sender != nil && a.sender.Configured() && len(a.cfg.Recipients) > 0
}

func (a *Alerts) RecipientCount() int {
	if a == nil {
		return 0
	}
	return len(a.cfg.Recipients)
}

// Start liga o laço em segundo plano. Sem efeito se desabilitado ou já iniciado.
func (a *Alerts) Start(parent context.Context) {
	if a == nil || !a.cfg.Enabled {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	a.cancel = cancel
	go a.loop(ctx)
}

func (a *Alerts) Stop() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}

func (a *Alerts) loop(ctx context.Context) {
	// Espera o coletor gravar a primeira amostra antes de avaliar collector_silent.
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}
	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()
	for {
		a.safeTick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Alerts) safeTick(ctx context.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			a.logger.Error("obs_alerts_panic", slog.Any("panic", recovered))
		}
	}()
	tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := a.Tick(tickCtx, time.Now().UTC()); err != nil && ctx.Err() == nil {
		a.logger.Warn("obs_alerts_tick_failed", slog.Any("error", err))
	}
}

// syncPlan é o resultado puro da comparação entre condições ativas e alertas abertos.
type syncPlan struct {
	Insert  []condition
	Update  []updateItem
	Resolve []alertRow
}

type updateItem struct {
	Row  alertRow
	Cond condition
}

func planSync(conds []condition, open []alertRow) syncPlan {
	byFingerprint := make(map[string]alertRow, len(open))
	for _, row := range open {
		byFingerprint[row.Fingerprint] = row
	}
	var plan syncPlan
	active := make(map[string]bool, len(conds))
	for _, c := range conds {
		fingerprint := c.Fingerprint()
		if active[fingerprint] {
			continue
		}
		active[fingerprint] = true
		if row, ok := byFingerprint[fingerprint]; ok {
			plan.Update = append(plan.Update, updateItem{Row: row, Cond: c})
		} else {
			plan.Insert = append(plan.Insert, c)
		}
	}
	for _, row := range open {
		if !active[row.Fingerprint] {
			plan.Resolve = append(plan.Resolve, row)
		}
	}
	return plan
}

func withMeasured(c condition) condition {
	detail := make(map[string]any, len(c.Detail)+1)
	for key, value := range c.Detail {
		detail[key] = value
	}
	detail["measured"] = c.Measured
	c.Detail = detail
	return c
}

// Tick faz uma rodada: avalia, sincroniza obs.alerts e notifica.
func (a *Alerts) Tick(ctx context.Context, now time.Time) error {
	metrics, err := a.store.AlertMetrics(ctx, now)
	if err != nil {
		return err
	}
	conds := evaluateRules(metrics)
	open, err := a.store.ListAlerts(ctx, "open", 500)
	if err != nil {
		return err
	}
	plan := planSync(conds, open)
	items, err := a.applyPlan(ctx, plan, now)
	if err != nil {
		return err
	}
	return a.dispatch(ctx, items, now)
}

func (a *Alerts) applyPlan(ctx context.Context, plan syncPlan, now time.Time) ([]notifyItem, error) {
	var items []notifyItem
	for _, c := range plan.Insert {
		c = withMeasured(c)
		id, err := a.store.InsertAlert(ctx, c, now)
		if err != nil {
			return nil, err
		}
		row := alertRow{ID: id, Rule: c.Rule, Severity: c.Severity, Area: c.Area, Title: c.Title,
			Detail: c.Detail, Status: "open", FirstSeen: now, LastSeen: now}
		items = append(items, notifyItem{Alert: row, Reason: reasonOpened})
	}
	for _, update := range plan.Update {
		c := withMeasured(update.Cond)
		reason := notifyReason(update.Row, c.Severity, now, a.cfg.Renotify)
		if err := a.store.UpdateAlert(ctx, update.Row.ID, c, now); err != nil {
			return nil, err
		}
		row := update.Row
		row.Severity, row.Title, row.LastSeen = c.Severity, c.Title, now
		row.Detail = c.Detail
		if raw, ok := update.Row.Detail["_notify"]; ok {
			row.Detail["_notify"] = raw
		}
		if reason != "" {
			items = append(items, notifyItem{Alert: row, Reason: reason})
		}
	}
	for _, row := range plan.Resolve {
		if err := a.store.ResolveAlert(ctx, row.ID, now); err != nil {
			return nil, err
		}
		if notifyResolved(row, now) {
			resolvedAt := now
			row.ResolvedAt = &resolvedAt
			items = append(items, notifyItem{Alert: row, Reason: reasonResolved})
		}
	}
	return items, nil
}

// dispatch envia um único e-mail com todos os itens da rodada.
func (a *Alerts) dispatch(ctx context.Context, items []notifyItem, now time.Time) error {
	if len(items) == 0 {
		return nil
	}
	if !a.Configured() {
		for _, item := range items {
			if item.Reason == reasonResolved {
				continue
			}
			if current := item.Alert.LastNotifyError; current != nil && *current == emailNotConfigured {
				continue
			}
			if err := a.store.MarkNotifyError(ctx, item.Alert, emailNotConfigured, now); err != nil {
				return err
			}
		}
		return nil
	}
	subject, body := buildDigest(items, a.cfg.panelLink())
	sendErr := a.sender.Send(ctx, a.cfg.Recipients, subject, body)
	for _, item := range items {
		if err := a.record(ctx, item, sendErr, now); err != nil {
			return err
		}
	}
	if sendErr != nil {
		return fmt.Errorf("send alert email: %w", sendErr)
	}
	return nil
}

func (a *Alerts) record(ctx context.Context, item notifyItem, sendErr error, now time.Time) error {
	if sendErr != nil {
		return a.store.MarkNotifyError(ctx, item.Alert, truncateError(sendErr), now)
	}
	if item.Reason == reasonResolved {
		return nil
	}
	return a.store.MarkNotified(ctx, item.Alert.ID, item.Alert.Severity, now)
}

func truncateError(err error) string {
	text := err.Error()
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}

// SendTest envia o e-mail de teste (POST /alerts/test).
func (a *Alerts) SendTest(ctx context.Context) error {
	if !a.Configured() {
		return ErrAlertsNotConfigured
	}
	subject, body := testDigest(a.cfg.panelLink(), time.Now())
	return a.sender.Send(ctx, a.cfg.Recipients, subject, body)
}
