package observability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Erros de configuração traduzidos pelo handler.
var (
	ErrUnknownSetting      = errors.New("unknown setting key")
	ErrSettingsUnavailable = errors.New("settings unavailable")
)

func (s *Service) provider() (*settingsProvider, error) {
	if s.alerts == nil || s.alerts.settings == nil {
		return nil, ErrSettingsUnavailable
	}
	return s.alerts.settings, nil
}

// Settings devolve as configurações em vigor (banco sobre o padrão), o catálogo de
// regras e o estado do canal de e-mail.
func (s *Service) Settings(ctx context.Context) (map[string]any, error) {
	provider, err := s.provider()
	if err != nil {
		return nil, err
	}
	provider.Invalidate() // a tela sempre mostra o que está gravado
	now := time.Now().UTC()
	st, err := provider.Get(ctx, now)
	if err != nil {
		return nil, err
	}
	state, err := s.store.GetWeeklyState(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"alerts":       st.Alerts,
		"email":        st.Email,
		"weeklyReport": st.Weekly,
		"rulesCatalog": rulesCatalog,
		"channel": map[string]any{
			"email":                "active",
			"whatsapp":             "planned",
			"smtpConfigured":       s.alerts.sender != nil && s.alerts.sender.Configured(),
			"envRecipientCount":    len(s.alerts.cfg.Recipients),
			"alertRecipientCount":  len(s.alerts.recipients(st)),
			"weeklyRecipientCount": len(s.alerts.weeklyRecipients(st)),
			"cacheSeconds":         int(settingsCacheTTL / time.Second),
		},
		"weeklyState": map[string]any{"lastSentAt": state.LastSentAt, "lastError": state.LastError},
	}, nil
}

// PutSetting valida e grava uma chave. Erros de campo voltam em fieldErrors (400).
func (s *Service) PutSetting(ctx context.Context, key string, raw []byte, actor string) (fieldErrors, error) {
	provider, err := s.provider()
	if err != nil {
		return nil, err
	}
	value, errs, known := parseSetting(key, raw)
	if !known {
		return nil, ErrUnknownSetting
	}
	if len(errs) > 0 {
		return errs, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal setting: %w", err)
	}
	if err := s.store.PutSetting(ctx, key, encoded, actor); err != nil {
		return nil, err
	}
	provider.Invalidate()
	return nil, nil
}

// PreviewEmail monta o e-mail com o layout proposto e alertas de exemplo.
func (s *Service) PreviewEmail(raw []byte) (outgoingEmail, fieldErrors, error) {
	if s.alerts == nil {
		return outgoingEmail{}, nil, ErrSettingsUnavailable
	}
	value, errs, _ := parseSetting(settingsKeyEmail, raw)
	if len(errs) > 0 {
		return outgoingEmail{}, errs, nil
	}
	layout, _ := value.(EmailSettings)
	return previewEmail(layout, s.alerts.cfg.panelLink(), time.Now()), nil, nil
}

// WeeklyPreview monta o relatório semanal com os dados de agora.
func (s *Service) WeeklyPreview(ctx context.Context) (map[string]string, error) {
	if s.alerts == nil || s.alerts.settings == nil {
		return nil, ErrSettingsUnavailable
	}
	subject, text, err := s.alerts.PreviewWeekly(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{"subject": subject, "text": text}, nil
}

// WeeklySend envia o relatório semanal agora.
func (s *Service) WeeklySend(ctx context.Context) error {
	if s.alerts == nil || s.alerts.settings == nil {
		return ErrSettingsUnavailable
	}
	return s.alerts.SendWeeklyNow(ctx)
}
