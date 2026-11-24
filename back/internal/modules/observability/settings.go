package observability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // fuso IANA mesmo em imagens sem tzdata
)

// Chaves de obs.settings editáveis pelo painel (PUT /settings/{key}).
const (
	settingsKeyAlerts       = "alerts"
	settingsKeyEmail        = "email"
	settingsKeyWeekly       = "weeklyReport"
	settingsKeyWeeklyState  = "weeklyReport.state" // interna, só o motor grava
	defaultTimezone         = "America/Sao_Paulo"
	defaultSubjectPrefix    = "[Omni]"
	maxRecipients           = 20
	maxFooterLength         = 500
	maxSubjectPrefixLength  = 40
	minRenotifyMinutes      = 15
	maxRenotifyMinutes      = 1440
	severityAuto            = "auto"
	sectionSummary          = "summary"
	sectionDetails          = "details"
	sectionLink             = "link"
	weeklyDefaultHour       = 8
	weeklyDefaultWeekday    = int(time.Monday)
	settingsFieldsErrorCode = "invalid_settings"
)

var clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// ruleCatalogItem descreve uma regra do motor para a tela de configuração. O limite
// padrão é o do código (nunca muda ao salvar outra coisa).
type ruleCatalogItem struct {
	Rule             string  `json:"rule"`
	Label            string  `json:"label"`
	Area             string  `json:"area"`
	HasThreshold     bool    `json:"hasThreshold"`
	DefaultThreshold float64 `json:"defaultThreshold"`
	Unit             string  `json:"unit"`
	Min              float64 `json:"min"`
	Max              float64 `json:"max"`
}

// rulesCatalog lista as 19 regras do motor, com os limites atuais do código.
var rulesCatalog = []ruleCatalogItem{
	{Rule: "api_5xx_rate", Label: "Taxa de erros 5xx da API", Area: "api", HasThreshold: true, DefaultThreshold: 5, Unit: "% de 5xx em 5 min", Min: 1, Max: 100},
	{Rule: "route_errors", Label: "Rota com erros 5xx", Area: "api", HasThreshold: true, DefaultThreshold: 5, Unit: "respostas 5xx em 5 min", Min: 1, Max: 1000},
	{Rule: "slow_route", Label: "Rota lenta (p95)", Area: "api", HasThreshold: true, DefaultThreshold: 2000, Unit: "ms", Min: 200, Max: 60000},
	{Rule: "goroutine_growth", Label: "Goroutines em excesso ou crescendo", Area: "api", HasThreshold: true, DefaultThreshold: 10000, Unit: "goroutines", Min: 1000, Max: 1000000},
	{Rule: "panic_detected", Label: "Panic recuperado na API", Area: "api"},
	{Rule: "collector_silent", Label: "Coletor de métricas parado", Area: "api"},
	{Rule: "pool_saturation", Label: "Pool de conexões saturado", Area: "database"},
	{Rule: "pg_idle_in_tx", Label: "Transações paradas (idle in transaction)", Area: "database"},
	{Rule: "pg_lock_wait", Label: "Consultas esperando lock", Area: "database"},
	{Rule: "pg_deadlock", Label: "Deadlock no Postgres", Area: "database"},
	{Rule: "pg_connections", Label: "Conexões do Postgres perto do limite", Area: "database", HasThreshold: true, DefaultThreshold: 85, Unit: "% das conexões", Min: 50, Max: 99},
	{Rule: "job_backlog", Label: "Fila de jobs com atraso", Area: "workers", HasThreshold: true, DefaultThreshold: 300, Unit: "s de atraso", Min: 30, Max: 86400},
	{Rule: "job_dead", Label: "Jobs mortos", Area: "workers"},
	{Rule: "container_down", Label: "Container fora do ar", Area: "containers"},
	{Rule: "container_restarts", Label: "Container reiniciando", Area: "containers"},
	{Rule: "agent_silent", Label: "Agente de containers parado", Area: "containers"},
	{Rule: "disk_low", Label: "Pouco espaço em disco", Area: "containers", HasThreshold: true, DefaultThreshold: 85, Unit: "% usado", Min: 50, Max: 99},
	{Rule: "tls_expiring", Label: "Certificado TLS perto de vencer", Area: "containers", HasThreshold: true, DefaultThreshold: 14, Unit: "dias", Min: 1, Max: 90},
	{Rule: "frontend_errors", Label: "Erros de JavaScript no front", Area: "pages", HasThreshold: true, DefaultThreshold: 20, Unit: "erros em 15 min", Min: 1, Max: 10000},
}

func catalogItem(rule string) (ruleCatalogItem, bool) {
	for _, item := range rulesCatalog {
		if item.Rule == rule {
			return item, true
		}
	}
	return ruleCatalogItem{}, false
}

// defaultThreshold é o limite do código para a regra (0 se não tem limite).
func defaultThreshold(rule string) float64 {
	item, ok := catalogItem(rule)
	if !ok || !item.HasThreshold {
		return 0
	}
	return item.DefaultThreshold
}

// RuleSetting é a configuração de uma regra. Enabled nil = ligada.
type RuleSetting struct {
	Enabled   *bool    `json:"enabled"`
	Severity  string   `json:"severity"`
	Threshold *float64 `json:"threshold"`
}

func (r RuleSetting) on() bool { return r.Enabled == nil || *r.Enabled }

// QuietHours é o horário de silêncio dos e-mails.
type QuietHours struct {
	Enabled       bool   `json:"enabled"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Timezone      string `json:"timezone"`
	AllowCritical bool   `json:"allowCritical"`
}

// AlertSettings é a chave `alerts` de obs.settings.
type AlertSettings struct {
	Recipients      []string               `json:"recipients"`
	RenotifyMinutes int                    `json:"renotifyMinutes"`
	QuietHours      QuietHours             `json:"quietHours"`
	Rules           map[string]RuleSetting `json:"rules"`
}

// EmailSettings é a chave `email`: layout do e-mail de alertas.
type EmailSettings struct {
	SubjectPrefix   string   `json:"subjectPrefix"`
	Group           bool     `json:"group"`
	Sections        []string `json:"sections"`
	IncludeResolved bool     `json:"includeResolved"`
	Footer          string   `json:"footer"`
}

// WeeklySettings é a chave `weeklyReport`.
type WeeklySettings struct {
	Enabled    bool     `json:"enabled"`
	Weekday    int      `json:"weekday"` // 0 = domingo ... 6 = sábado (time.Weekday)
	Hour       int      `json:"hour"`
	Timezone   string   `json:"timezone"`
	Recipients []string `json:"recipients"`
}

// weeklyState é a chave interna `weeklyReport.state`.
type weeklyState struct {
	LastSentAt    *time.Time `json:"lastSentAt,omitempty"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
}

func defaultAlertSettings(env AlertConfig) AlertSettings {
	minutes := int(env.Renotify / time.Minute)
	if minutes < minRenotifyMinutes {
		minutes = 60
	}
	return AlertSettings{
		Recipients:      []string{},
		RenotifyMinutes: min(minutes, maxRenotifyMinutes),
		QuietHours:      QuietHours{Start: "22:00", End: "07:00", Timezone: defaultTimezone, AllowCritical: true},
		Rules:           map[string]RuleSetting{},
	}
}

func defaultEmailSettings() EmailSettings {
	return EmailSettings{
		SubjectPrefix:   defaultSubjectPrefix,
		Group:           true,
		Sections:        []string{sectionSummary, sectionDetails, sectionLink},
		IncludeResolved: true,
	}
}

func defaultWeeklySettings() WeeklySettings {
	return WeeklySettings{Weekday: weeklyDefaultWeekday, Hour: weeklyDefaultHour, Timezone: defaultTimezone, Recipients: []string{}}
}

// fieldErrors: caminho do campo -> mensagem (vai em error.details.fields no 400).
type fieldErrors map[string]string

func (f fieldErrors) add(field, message string) {
	if _, exists := f[field]; !exists {
		f[field] = message
	}
}

// strictDecode lê o JSON proposto rejeitando campos desconhecidos.
func strictDecode(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("conteúdo extra após o objeto")
	}
	return nil
}

// loadZone resolve o fuso; o do Brasil tem reserva fixa (sem horário de verão desde 2019).
func loadZone(name string) (*time.Location, error) {
	loc, err := time.LoadLocation(name)
	if err == nil {
		return loc, nil
	}
	if name == defaultTimezone {
		return time.FixedZone("BRT", -3*60*60), nil
	}
	return nil, err
}

func validateTimezone(f fieldErrors, field, name string) {
	if strings.TrimSpace(name) == "" {
		f.add(field, "Informe o fuso horário (ex.: America/Sao_Paulo).")
		return
	}
	if _, err := loadZone(name); err != nil {
		f.add(field, "Fuso horário desconhecido.")
	}
}

// normalizeRecipients valida e-mails (sem duplicatas, minúsculos, máx. 20).
func normalizeRecipients(f fieldErrors, field string, list []string) []string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		parsed, err := mail.ParseAddress(item)
		_, domain, hasAt := strings.Cut(item, "@")
		if err != nil || parsed.Address != item || strings.ContainsAny(item, "\r\n<>\"") || !hasAt || !strings.Contains(domain, ".") {
			f.add(field, fmt.Sprintf("E-mail inválido: %s.", truncateText(item, 60)))
			continue
		}
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	if len(out) > maxRecipients {
		f.add(field, fmt.Sprintf("No máximo %d destinatários.", maxRecipients))
		out = out[:maxRecipients]
	}
	return out
}

func clockMinutes(value string) (int, bool) {
	if !clockPattern.MatchString(value) {
		return 0, false
	}
	var hour, minute int
	_, _ = fmt.Sscanf(value, "%d:%d", &hour, &minute)
	return hour*60 + minute, true
}

// validateAlertSettings valida e normaliza a chave `alerts`.
func validateAlertSettings(in AlertSettings) (AlertSettings, fieldErrors) {
	f := fieldErrors{}
	out := in
	out.Recipients = normalizeRecipients(f, "recipients", in.Recipients)
	if in.RenotifyMinutes < minRenotifyMinutes || in.RenotifyMinutes > maxRenotifyMinutes {
		f.add("renotifyMinutes", fmt.Sprintf("Informe de %d a %d minutos.", minRenotifyMinutes, maxRenotifyMinutes))
	}
	q := &out.QuietHours
	q.Timezone = strings.TrimSpace(q.Timezone)
	if q.Timezone == "" {
		q.Timezone = defaultTimezone
	}
	validateTimezone(f, "quietHours.timezone", q.Timezone)
	start, startOK := clockMinutes(q.Start)
	end, endOK := clockMinutes(q.End)
	if !startOK {
		f.add("quietHours.start", "Use o formato HH:MM (ex.: 22:00).")
	}
	if !endOK {
		f.add("quietHours.end", "Use o formato HH:MM (ex.: 07:00).")
	}
	if q.Enabled && startOK && endOK && start == end {
		f.add("quietHours.end", "O fim deve ser diferente do início.")
	}
	out.Rules = map[string]RuleSetting{}
	for key, rule := range in.Rules {
		item, known := catalogItem(key)
		if !known {
			f.add("rules."+key, "Regra desconhecida.")
			continue
		}
		enabled := rule.on()
		clean := RuleSetting{Enabled: &enabled, Severity: strings.TrimSpace(rule.Severity)}
		if clean.Severity == "" {
			clean.Severity = severityAuto
		}
		if clean.Severity != severityAuto && severityRank(clean.Severity) == 0 {
			f.add("rules."+key+".severity", "Use auto, critical, high, medium ou low.")
		}
		switch {
		case rule.Threshold != nil && !item.HasThreshold:
			f.add("rules."+key+".threshold", "Esta regra não tem limite configurável.")
		case rule.Threshold != nil && (*rule.Threshold < item.Min || *rule.Threshold > item.Max):
			f.add("rules."+key+".threshold", fmt.Sprintf("Informe de %s a %s (%s).", trimFloat(item.Min), trimFloat(item.Max), item.Unit))
		case rule.Threshold != nil:
			value := *rule.Threshold
			clean.Threshold = &value
		}
		out.Rules[key] = clean
	}
	return out, f
}

func trimFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

var validSections = []string{sectionSummary, sectionDetails, sectionLink}

// validateEmailSettings valida e normaliza a chave `email`.
func validateEmailSettings(in EmailSettings) (EmailSettings, fieldErrors) {
	f := fieldErrors{}
	out := in
	out.SubjectPrefix = strings.TrimSpace(in.SubjectPrefix)
	switch {
	case strings.ContainsAny(out.SubjectPrefix, "\r\n"):
		f.add("subjectPrefix", "O prefixo deve ter uma única linha.")
	case len([]rune(out.SubjectPrefix)) > maxSubjectPrefixLength:
		f.add("subjectPrefix", fmt.Sprintf("No máximo %d caracteres.", maxSubjectPrefixLength))
	}
	out.Footer = strings.TrimSpace(strings.ReplaceAll(in.Footer, "\r\n", "\n"))
	if len([]rune(out.Footer)) > maxFooterLength {
		f.add("footer", fmt.Sprintf("No máximo %d caracteres.", maxFooterLength))
	}
	chosen := map[string]bool{}
	for _, section := range in.Sections {
		valid := false
		for _, candidate := range validSections {
			valid = valid || candidate == section
		}
		if !valid {
			f.add("sections", "Seção desconhecida: "+truncateText(section, 30)+".")
			continue
		}
		chosen[section] = true
	}
	out.Sections = out.Sections[:0:0]
	for _, section := range validSections { // ordem canônica
		if chosen[section] {
			out.Sections = append(out.Sections, section)
		}
	}
	if len(out.Sections) == 0 {
		f.add("sections", "Escolha ao menos uma seção.")
	}
	return out, f
}

// validateWeeklySettings valida e normaliza a chave `weeklyReport`.
func validateWeeklySettings(in WeeklySettings) (WeeklySettings, fieldErrors) {
	f := fieldErrors{}
	out := in
	out.Recipients = normalizeRecipients(f, "recipients", in.Recipients)
	if in.Weekday < 0 || in.Weekday > 6 {
		f.add("weekday", "Informe de 0 (domingo) a 6 (sábado).")
	}
	if in.Hour < 0 || in.Hour > 23 {
		f.add("hour", "Informe de 0 a 23.")
	}
	out.Timezone = strings.TrimSpace(in.Timezone)
	if out.Timezone == "" {
		out.Timezone = defaultTimezone
	}
	validateTimezone(f, "timezone", out.Timezone)
	return out, f
}

// parseSetting valida o JSON proposto de uma chave; devolve o valor normalizado
// (para gravar) ou os erros por campo. Chave desconhecida: ok=false.
func parseSetting(key string, raw []byte) (value any, f fieldErrors, ok bool) {
	var decodeErr error
	switch key {
	case settingsKeyAlerts:
		var in AlertSettings
		if decodeErr = strictDecode(raw, &in); decodeErr == nil {
			value, f = validateAlertSettings(in)
		}
	case settingsKeyEmail:
		var in EmailSettings
		if decodeErr = strictDecode(raw, &in); decodeErr == nil {
			value, f = validateEmailSettings(in)
		}
	case settingsKeyWeekly:
		var in WeeklySettings
		if decodeErr = strictDecode(raw, &in); decodeErr == nil {
			value, f = validateWeeklySettings(in)
		}
	default:
		return nil, nil, false
	}
	if decodeErr != nil {
		return nil, fieldErrors{"_": "JSON inválido: " + truncateText(decodeErr.Error(), 120)}, true
	}
	return value, f, true
}

// effectiveAlerts completa o que veio do banco com o padrão: regras sem entrada
// ficam ligadas, em severidade automática e com o limite do código.
func effectiveAlerts(stored AlertSettings) AlertSettings {
	out := stored
	out.Rules = make(map[string]RuleSetting, len(rulesCatalog))
	for _, item := range rulesCatalog {
		rule := stored.Rules[item.Rule]
		enabled := rule.on()
		eff := RuleSetting{Enabled: &enabled, Severity: rule.Severity}
		if eff.Severity == "" {
			eff.Severity = severityAuto
		}
		if item.HasThreshold {
			value := item.DefaultThreshold
			if rule.Threshold != nil {
				value = *rule.Threshold
			}
			eff.Threshold = &value
		}
		out.Rules[item.Rule] = eff
	}
	if out.Recipients == nil {
		out.Recipients = []string{}
	}
	if out.QuietHours.Timezone == "" {
		out.QuietHours.Timezone = defaultTimezone
	}
	return out
}

// inQuietHours: `now` cai dentro de [start, end) no fuso configurado (a janela pode
// atravessar a meia-noite).
func inQuietHours(q QuietHours, now time.Time) bool {
	if !q.Enabled {
		return false
	}
	start, startOK := clockMinutes(q.Start)
	end, endOK := clockMinutes(q.End)
	loc, err := loadZone(q.Timezone)
	if !startOK || !endOK || err != nil || start == end {
		return false
	}
	local := now.In(loc)
	minutes := local.Hour()*60 + local.Minute()
	if start < end {
		return minutes >= start && minutes < end
	}
	return minutes >= start || minutes < end
}

// splitQuiet separa o que sai agora do que fica retido no horário de silêncio:
// críticos passam se allowCritical.
func splitQuiet(items []notifyItem, q QuietHours, now time.Time) (send, held []notifyItem) {
	if !inQuietHours(q, now) {
		return items, nil
	}
	for _, item := range items {
		if q.AllowCritical && item.Alert.Severity == sevCritical {
			send = append(send, item)
		} else {
			held = append(held, item)
		}
	}
	return send, held
}

// applyRuleSettings remove condições de regras desligadas e aplica a severidade fixa.
func applyRuleSettings(conds []condition, rules map[string]RuleSetting) []condition {
	if len(rules) == 0 {
		return conds
	}
	out := conds[:0:0]
	for _, c := range conds {
		rule, ok := rules[c.Rule]
		if ok && !rule.on() {
			continue
		}
		if ok && rule.Severity != "" && rule.Severity != severityAuto && severityRank(rule.Severity) > 0 {
			c.Severity = rule.Severity
		}
		out = append(out, c)
	}
	return out
}

// ruleThresholds extrai só os limites personalizados (para as regras puras).
func ruleThresholds(rules map[string]RuleSetting) map[string]float64 {
	out := map[string]float64{}
	for key, rule := range rules {
		if rule.Threshold != nil {
			out[key] = *rule.Threshold
		}
	}
	return out
}
