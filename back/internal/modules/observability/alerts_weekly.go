package observability

import (
	"context"
	"log/slog"
	"time"
)

const weeklyTimeout = 60 * time.Second

// weeklyTick envia o relatório semanal quando chega o dia e a hora configurados
// (uma vez por semana). Roda depois de cada rodada de alertas; falhas só registram
// o estado e tentam de novo depois de 15 min, dentro de 24 h do horário.
func (a *Alerts) weeklyTick(ctx context.Context, now time.Time) {
	defer func() {
		if recovered := recover(); recovered != nil {
			a.logger.Error("obs_weekly_report_panic", slog.Any("panic", recovered))
		}
	}()
	st := a.current(ctx, now)
	if !st.Weekly.Enabled {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	state, err := a.store.GetWeeklyState(readCtx)
	cancel()
	if err != nil {
		a.logger.Warn("obs_weekly_state_read_failed", slog.Any("error", err))
		return
	}
	if !weeklyDue(st.Weekly, state, now) {
		return
	}
	sendCtx, cancelSend := context.WithTimeout(ctx, weeklyTimeout)
	defer cancelSend()
	if err := a.sendWeekly(sendCtx, st, now); err != nil {
		a.logger.Warn("obs_weekly_report_failed", slog.Any("error", err))
		state.LastAttemptAt, state.LastError = &now, truncateError(err)
		if putErr := a.store.PutWeeklyState(sendCtx, state); putErr != nil {
			a.logger.Warn("obs_weekly_state_write_failed", slog.Any("error", putErr))
		}
	}
}

// weeklyRecipients: os do relatório ou, vazios, os dos alertas.
func (a *Alerts) weeklyRecipients(st effectiveSettings) []string {
	return recipientsFor(st.Weekly.Recipients, a.recipients(st))
}

// PreviewWeekly monta o relatório com os dados de agora (sem enviar).
func (a *Alerts) PreviewWeekly(ctx context.Context) (subject, text string, err error) {
	now := time.Now().UTC()
	return a.buildWeekly(ctx, a.current(ctx, now), now)
}

func (a *Alerts) buildWeekly(ctx context.Context, st effectiveSettings, now time.Time) (subject, text string, err error) {
	loc, err := loadZone(st.Weekly.Timezone)
	if err != nil {
		return "", "", err
	}
	data, err := a.store.WeeklyReportData(ctx, now, now.In(loc).Format("2006-01-02"))
	if err != nil {
		return "", "", err
	}
	subject, text = buildWeeklyReport(data, st.Email.SubjectPrefix, a.cfg.panelLink(), loc)
	return subject, text, nil
}

// SendWeeklyNow envia o relatório agora (POST /reports/weekly/send) e o conta como o
// desta semana.
func (a *Alerts) SendWeeklyNow(ctx context.Context) error {
	now := time.Now().UTC()
	return a.sendWeekly(ctx, a.current(ctx, now), now)
}

func (a *Alerts) sendWeekly(ctx context.Context, st effectiveSettings, now time.Time) error {
	to := a.weeklyRecipients(st)
	if a.sender == nil || !a.sender.Configured() || len(to) == 0 {
		return ErrAlertsNotConfigured
	}
	subject, text, err := a.buildWeekly(ctx, st, now)
	if err != nil {
		return err
	}
	if err := a.sender.Send(ctx, to, subject, text); err != nil {
		return err
	}
	return a.store.PutWeeklyState(ctx, weeklyState{LastSentAt: &now})
}
