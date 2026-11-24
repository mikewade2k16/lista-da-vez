package observability

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const settingsCacheTTL = 60 * time.Second

// settingsSource lê uma chave de obs.settings (implementado por *Store).
type settingsSource interface {
	GetSetting(ctx context.Context, key string) ([]byte, bool, error)
}

// effectiveSettings são os valores em vigor: o que está no banco sobre o padrão
// (variáveis de ambiente e código).
type effectiveSettings struct {
	Alerts AlertSettings
	Email  EmailSettings
	Weekly WeeklySettings
}

// settingsProvider lê as três chaves e guarda o resultado por 60 s. Valor gravado
// inválido (editado à mão no banco) cai no padrão daquela chave.
type settingsProvider struct {
	src settingsSource
	env AlertConfig

	mu     sync.Mutex
	cached effectiveSettings
	at     time.Time
	loaded bool
}

func newSettingsProvider(src settingsSource, env AlertConfig) *settingsProvider {
	return &settingsProvider{src: src, env: env}
}

// Get devolve as configurações em vigor, relendo do banco depois de 60 s. Se a
// leitura falhar, mantém a última leitura (ou o padrão) e devolve o erro.
func (p *settingsProvider) Get(ctx context.Context, now time.Time) (effectiveSettings, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loaded && now.Sub(p.at) < settingsCacheTTL && now.After(p.at.Add(-settingsCacheTTL)) {
		return p.cached, nil
	}
	loaded, err := p.load(ctx)
	if err != nil {
		if !p.loaded {
			return p.defaults(), err
		}
		return p.cached, err
	}
	p.cached, p.at, p.loaded = loaded, now, true
	return loaded, nil
}

// Invalidate força a releitura na próxima chamada (depois de um PUT).
func (p *settingsProvider) Invalidate() {
	p.mu.Lock()
	p.loaded = false
	p.mu.Unlock()
}

// Peek devolve a última leitura sem consultar o banco (padrão se ainda não leu).
func (p *settingsProvider) Peek() effectiveSettings {
	if p == nil {
		return effectiveSettings{Alerts: effectiveAlerts(defaultAlertSettings(AlertConfig{})), Email: defaultEmailSettings(), Weekly: defaultWeeklySettings()}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loaded {
		return p.cached
	}
	return p.defaults()
}

func (p *settingsProvider) defaults() effectiveSettings {
	return effectiveSettings{
		Alerts: effectiveAlerts(defaultAlertSettings(p.env)),
		Email:  defaultEmailSettings(),
		Weekly: defaultWeeklySettings(),
	}
}

func (p *settingsProvider) load(ctx context.Context) (effectiveSettings, error) {
	out := p.defaults()
	alerts := defaultAlertSettings(p.env)
	if found, err := p.read(ctx, settingsKeyAlerts, &alerts); err != nil {
		return out, err
	} else if found {
		if clean, errs := validateAlertSettings(alerts); len(errs) == 0 {
			out.Alerts = effectiveAlerts(clean)
		}
	}
	email := defaultEmailSettings()
	if found, err := p.read(ctx, settingsKeyEmail, &email); err != nil {
		return out, err
	} else if found {
		if clean, errs := validateEmailSettings(email); len(errs) == 0 {
			out.Email = clean
		}
	}
	weekly := defaultWeeklySettings()
	if found, err := p.read(ctx, settingsKeyWeekly, &weekly); err != nil {
		return out, err
	} else if found {
		if clean, errs := validateWeeklySettings(weekly); len(errs) == 0 {
			out.Weekly = clean
		}
	}
	return out, nil
}

// read decodifica sobre o valor padrão já presente em dst (campos ausentes ficam
// no padrão). JSON ilegível conta como não encontrado.
func (p *settingsProvider) read(ctx context.Context, key string, dst any) (bool, error) {
	raw, found, err := p.src.GetSetting(ctx, key)
	if err != nil || !found {
		return false, err
	}
	return json.Unmarshal(raw, dst) == nil, nil
}

// recipientsFor: destinatários do alerta (banco) ou, vazios, os de OBS_ALERT_EMAILS.
func recipientsFor(stored []string, env []string) []string {
	if len(stored) > 0 {
		return stored
	}
	return env
}
