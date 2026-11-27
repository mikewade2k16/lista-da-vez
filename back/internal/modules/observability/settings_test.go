package observability

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRulesCatalogMatchesCode(t *testing.T) {
	if len(rulesCatalog) != 22 {
		t.Fatalf("catálogo com %d regras, esperava 22", len(rulesCatalog))
	}
	want := map[string]float64{
		"api_5xx_rate": 5, "route_errors": 5, "slow_route": 2000, "goroutine_growth": 10000,
		"pg_connections": 85, "job_backlog": 300, "disk_low": 85, "tls_expiring": 14, "frontend_errors": 20,
		"contract_errors": 2, "route_4xx_spike": 50, "deploy_regression": 3,
	}
	seen := map[string]bool{}
	for _, item := range rulesCatalog {
		if seen[item.Rule] || item.Label == "" || item.Area == "" {
			t.Errorf("regra repetida ou sem rótulo/área: %+v", item)
		}
		seen[item.Rule] = true
		if value, ok := want[item.Rule]; ok != item.HasThreshold || (ok && value != item.DefaultThreshold) {
			t.Errorf("%s: limite padrão %v/%v", item.Rule, item.HasThreshold, item.DefaultThreshold)
		}
		if item.HasThreshold && (item.Unit == "" || item.DefaultThreshold < item.Min || item.DefaultThreshold > item.Max) {
			t.Errorf("%s: unidade ou faixa inválida", item.Rule)
		}
	}
}

func TestValidateAlertSettings(t *testing.T) {
	good := AlertSettings{
		Recipients:      []string{" Ana@Exemplo.com ", "ana@exemplo.com", ""},
		RenotifyMinutes: 30,
		QuietHours:      QuietHours{Enabled: true, Start: "22:00", End: "07:00", Timezone: "America/Sao_Paulo"},
		Rules:           map[string]RuleSetting{"slow_route": {Severity: "high", Threshold: ptr(3000.0)}},
	}
	out, errs := validateAlertSettings(good)
	if len(errs) != 0 {
		t.Fatalf("válido rejeitado: %v", errs)
	}
	if len(out.Recipients) != 1 || out.Recipients[0] != "ana@exemplo.com" {
		t.Errorf("destinatários: %v", out.Recipients)
	}
	if rule := out.Rules["slow_route"]; !rule.on() || *rule.Threshold != 3000 {
		t.Errorf("regra normalizada: %+v", rule)
	}

	bad := AlertSettings{
		Recipients:      []string{"sem-arroba", "x@y"},
		RenotifyMinutes: 5,
		QuietHours:      QuietHours{Enabled: true, Start: "25:00", End: "07:00", Timezone: "Marte/Olympus"},
		Rules: map[string]RuleSetting{
			"nao_existe":   {},
			"pg_deadlock":  {Threshold: ptr(1.0)},
			"slow_route":   {Threshold: ptr(10.0), Severity: "high"},
			"disk_low":     {Severity: "urgente"},
			"api_5xx_rate": {Threshold: ptr(500.0)},
		},
	}
	_, errs = validateAlertSettings(bad)
	for _, field := range []string{"recipients", "renotifyMinutes", "quietHours.start", "quietHours.timezone",
		"rules.nao_existe", "rules.pg_deadlock.threshold", "rules.slow_route.threshold", "rules.disk_low.severity", "rules.api_5xx_rate.threshold"} {
		if errs[field] == "" {
			t.Errorf("sem erro para %s: %v", field, errs)
		}
	}
	same := good
	same.QuietHours.End = "22:00"
	if _, errs = validateAlertSettings(same); errs["quietHours.end"] == "" {
		t.Errorf("início = fim deveria falhar: %v", errs)
	}
	many := good
	many.Recipients = nil
	for index := 0; index < 21; index++ {
		many.Recipients = append(many.Recipients, strings.Repeat("a", index+1)+"@x.com")
	}
	if _, errs = validateAlertSettings(many); errs["recipients"] == "" {
		t.Errorf("mais de 20 destinatários deveria falhar")
	}
}

func TestValidateEmailSettings(t *testing.T) {
	out, errs := validateEmailSettings(EmailSettings{SubjectPrefix: " [X] ", Group: true, Sections: []string{"link", "summary", "link"}, Footer: " rodapé "})
	if len(errs) != 0 {
		t.Fatalf("erros: %v", errs)
	}
	if out.SubjectPrefix != "[X]" || out.Footer != "rodapé" || strings.Join(out.Sections, ",") != "summary,link" {
		t.Errorf("normalização: %+v", out)
	}
	_, errs = validateEmailSettings(EmailSettings{SubjectPrefix: "a\nb", Sections: []string{"banner"}, Footer: strings.Repeat("x", 501)})
	for _, field := range []string{"subjectPrefix", "sections", "footer"} {
		if errs[field] == "" {
			t.Errorf("sem erro para %s: %v", field, errs)
		}
	}
	if _, errs = validateEmailSettings(EmailSettings{}); errs["sections"] == "" {
		t.Errorf("sem seções deveria falhar")
	}
}

func TestValidateWeeklySettings(t *testing.T) {
	if _, errs := validateWeeklySettings(WeeklySettings{Weekday: 1, Hour: 8, Timezone: "America/Sao_Paulo"}); len(errs) != 0 {
		t.Fatalf("erros: %v", errs)
	}
	_, errs := validateWeeklySettings(WeeklySettings{Weekday: 7, Hour: 24, Timezone: "x", Recipients: []string{"nope"}})
	for _, field := range []string{"weekday", "hour", "timezone", "recipients"} {
		if errs[field] == "" {
			t.Errorf("sem erro para %s: %v", field, errs)
		}
	}
}

func TestParseSetting(t *testing.T) {
	if _, _, ok := parseSetting("outra", []byte(`{}`)); ok {
		t.Errorf("chave desconhecida deveria ser rejeitada")
	}
	if _, errs, ok := parseSetting("email", []byte(`{"subjectPrefix":"a","campoNovo":1}`)); !ok || errs["_"] == "" {
		t.Errorf("campo desconhecido deveria dar erro geral: %v", errs)
	}
	value, errs, _ := parseSetting("weeklyReport", []byte(`{"enabled":true,"weekday":1,"hour":8,"timezone":"America/Sao_Paulo","recipients":[]}`))
	if len(errs) != 0 || !value.(WeeklySettings).Enabled {
		t.Errorf("weekly: %v %+v", errs, value)
	}
}

func TestEffectiveAlertsFillsDefaults(t *testing.T) {
	stored := AlertSettings{RenotifyMinutes: 60, Rules: map[string]RuleSetting{"slow_route": {Enabled: ptr(false), Threshold: ptr(3000.0)}}}
	eff := effectiveAlerts(stored)
	if len(eff.Rules) != len(rulesCatalog) {
		t.Fatalf("regras efetivas: %d", len(eff.Rules))
	}
	if eff.Rules["slow_route"].on() || *eff.Rules["slow_route"].Threshold != 3000 {
		t.Errorf("slow_route: %+v", eff.Rules["slow_route"])
	}
	if !eff.Rules["disk_low"].on() || *eff.Rules["disk_low"].Threshold != 85 || eff.Rules["disk_low"].Severity != severityAuto {
		t.Errorf("disk_low: %+v", eff.Rules["disk_low"])
	}
	if eff.Rules["pg_deadlock"].Threshold != nil {
		t.Errorf("regra sem limite não deve ter threshold")
	}
}

func TestQuietHours(t *testing.T) {
	q := QuietHours{Enabled: true, Start: "22:00", End: "07:00", Timezone: "America/Sao_Paulo", AllowCritical: true}
	at := func(hour, minute int) time.Time { // hora de Brasília (UTC-3)
		return time.Date(2026, 11, 24, hour+3, minute, 0, 0, time.UTC)
	}
	cases := []struct {
		hour, minute int
		want         bool
	}{{21, 59, false}, {22, 0, true}, {23, 30, true}, {3, 0, true}, {6, 59, true}, {7, 0, false}, {12, 0, false}}
	for _, tc := range cases {
		if got := inQuietHours(q, at(tc.hour, tc.minute)); got != tc.want {
			t.Errorf("%02d:%02d: got %v", tc.hour, tc.minute, got)
		}
	}
	q.Enabled = false
	if inQuietHours(q, at(23, 0)) {
		t.Errorf("desligado nunca silencia")
	}
	day := QuietHours{Enabled: true, Start: "09:00", End: "18:00", Timezone: "America/Sao_Paulo"}
	if !inQuietHours(day, at(12, 0)) || inQuietHours(day, at(20, 0)) {
		t.Errorf("janela no mesmo dia")
	}
}

func TestSplitQuiet(t *testing.T) {
	q := QuietHours{Enabled: true, Start: "22:00", End: "07:00", Timezone: "America/Sao_Paulo", AllowCritical: true}
	night := time.Date(2026, 11, 25, 2, 0, 0, 0, time.UTC) // 23:00 em Brasília
	items := []notifyItem{
		{Alert: alertRow{ID: 1, Severity: sevCritical}, Reason: reasonOpened},
		{Alert: alertRow{ID: 2, Severity: sevHigh}, Reason: reasonOpened},
		{Alert: alertRow{ID: 3, Severity: sevCritical}, Reason: reasonResolved},
	}
	send, held := splitQuiet(items, q, night)
	if len(send) != 2 || len(held) != 1 || held[0].Alert.ID != 2 {
		t.Fatalf("send=%v held=%v", send, held)
	}
	q.AllowCritical = false
	if send, held = splitQuiet(items, q, night); len(send) != 0 || len(held) != 3 {
		t.Fatalf("sem allowCritical tudo fica retido: %v %v", send, held)
	}
	day := time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC)
	if send, held = splitQuiet(items, q, day); len(send) != 3 || len(held) != 0 {
		t.Fatalf("fora do silêncio tudo sai")
	}
}

func TestApplyRuleSettings(t *testing.T) {
	conds := []condition{
		{Rule: "slow_route", Target: "GET /x", Severity: sevMedium},
		{Rule: "disk_low", Target: "/", Severity: sevHigh},
		{Rule: "pg_deadlock", Severity: sevHigh},
	}
	rules := map[string]RuleSetting{
		"slow_route":  {Enabled: ptr(false)},
		"disk_low":    {Severity: sevCritical},
		"pg_deadlock": {Severity: severityAuto},
	}
	out := applyRuleSettings(conds, rules)
	if len(out) != 2 || findRule(out, "slow_route", "GET /x") != nil {
		t.Fatalf("regra desligada deveria sair: %+v", out)
	}
	if findRule(out, "disk_low", "/").Severity != sevCritical || findRule(out, "pg_deadlock", "").Severity != sevHigh {
		t.Errorf("severidade: %+v", out)
	}
}

func TestEvaluateRulesWithThresholds(t *testing.T) {
	m := baseMetrics()
	m.HTTPTotal5m, m.HTTP5xx5m = 100, 8 // 8%
	m.SlowRoutes15m = []routeLatency{{Route: "GET /x", P95Ms: 2500, Calls: 20}}
	m.RouteErrors5m = []routeCount{{Route: "GET /y", Count: 6}}
	m.Disks = []diskMetric{{Path: "/", UsedPct: 88}}
	m.FrontendJSErr15 = 25
	m.APISamples = []apiSample{sampleAt(0, func(a *apiMetrics) { a.Jobs = []jobMetrics{{Table: "q", OldestDueLagSeconds: 400}} })}

	// padrão do código
	conds := evaluateRulesWith(m, nil)
	for _, want := range [][2]string{{"api_5xx_rate", ""}, {"slow_route", "GET /x"}, {"route_errors", "GET /y"}, {"disk_low", "/"}, {"frontend_errors", ""}, {"job_backlog", "q"}} {
		if findRule(conds, want[0], want[1]) == nil {
			t.Errorf("padrão deveria abrir %v", want)
		}
	}
	// limites mais altos silenciam; um desligado some
	rules := map[string]RuleSetting{
		"api_5xx_rate":    {Threshold: ptr(10.0)},
		"slow_route":      {Threshold: ptr(3000.0)},
		"route_errors":    {Threshold: ptr(10.0)},
		"disk_low":        {Threshold: ptr(90.0)},
		"frontend_errors": {Enabled: ptr(false)},
		"job_backlog":     {Threshold: ptr(600.0)},
	}
	if conds = evaluateRulesWith(m, rules); len(conds) != 0 {
		t.Errorf("com limites maiores nada deveria abrir: %+v", conds)
	}
	// limites mais baixos abrem mais cedo e escalam a severidade proporcionalmente
	m.HTTP5xx5m = 3 // 3%
	low := map[string]RuleSetting{"api_5xx_rate": {Threshold: ptr(2.0)}, "disk_low": {Threshold: ptr(80.0)}}
	conds = evaluateRulesWith(m, low)
	if c := findRule(conds, "api_5xx_rate", ""); c == nil || c.Severity != sevHigh {
		t.Errorf("3%% com limite 2%%: %+v", c)
	}
	m.HTTP5xx5m = 9 // 9% > 2*4%
	if c := findRule(evaluateRulesWith(m, low), "api_5xx_rate", ""); c == nil || c.Severity != sevCritical {
		t.Errorf("9%% com limite 2%% deveria ser crítico: %+v", c)
	}
	if c := findRule(conds, "disk_low", "/"); c == nil || c.Severity != sevHigh {
		t.Errorf("disk_low 88%% com limite 80: %+v", c)
	}
	m.Disks[0].UsedPct = 91
	if c := findRule(evaluateRulesWith(m, low), "disk_low", "/"); c == nil || c.Severity != sevCritical {
		t.Errorf("disk_low 91%% com limite 80 (crítico em 90): %+v", c)
	}
}

type fakeSettingsSource struct {
	values map[string]string
	reads  int
	err    error
}

func (f *fakeSettingsSource) GetSetting(_ context.Context, key string) ([]byte, bool, error) {
	f.reads++
	if f.err != nil {
		return nil, false, f.err
	}
	value, ok := f.values[key]
	return []byte(value), ok, nil
}

func TestSettingsProviderCacheAndDefaults(t *testing.T) {
	src := &fakeSettingsSource{values: map[string]string{
		"alerts": `{"renotifyMinutes":120,"recipients":["a@b.com"]}`,
		"email":  `{"subjectPrefix":"[Teste]","group":false,"sections":["summary"],"includeResolved":false}`,
	}}
	env := AlertConfig{Renotify: 45 * time.Minute}
	p := newSettingsProvider(src, env)
	st, err := p.Get(context.Background(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if st.Alerts.RenotifyMinutes != 120 || len(st.Alerts.Recipients) != 1 || st.Email.Group || st.Email.SubjectPrefix != "[Teste]" {
		t.Errorf("valores do banco: %+v %+v", st.Alerts, st.Email)
	}
	if st.Weekly.Enabled || st.Weekly.Hour != 8 || st.Weekly.Weekday != 1 {
		t.Errorf("padrão semanal: %+v", st.Weekly)
	}
	reads := src.reads
	if _, err = p.Get(context.Background(), t0.Add(59*time.Second)); err != nil || src.reads != reads {
		t.Errorf("dentro de 60 s deve usar o cache (reads %d -> %d)", reads, src.reads)
	}
	src.values["alerts"] = `{"renotifyMinutes":30}`
	if st, _ = p.Get(context.Background(), t0.Add(61*time.Second)); st.Alerts.RenotifyMinutes != 30 {
		t.Errorf("depois de 60 s relê: %+v", st.Alerts)
	}
	p.Invalidate()
	src.values["alerts"] = `{"renotifyMinutes":90}`
	if st, _ = p.Get(context.Background(), t0.Add(62*time.Second)); st.Alerts.RenotifyMinutes != 90 {
		t.Errorf("Invalidate relê: %+v", st.Alerts)
	}
}

func TestSettingsProviderFallbacks(t *testing.T) {
	src := &fakeSettingsSource{values: map[string]string{"alerts": `{"renotifyMinutes":1}`, "email": `{lixo`}}
	p := newSettingsProvider(src, AlertConfig{Renotify: 45 * time.Minute})
	st, err := p.Get(context.Background(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if st.Alerts.RenotifyMinutes != 45 || st.Email.SubjectPrefix != defaultSubjectPrefix {
		t.Errorf("inválido cai no padrão (env 45 min): %+v", st)
	}
	src.err = errors.New("banco fora")
	if _, err = p.Get(context.Background(), t0.Add(2*time.Minute)); err == nil {
		t.Errorf("erro de leitura deve ser devolvido")
	}
	if got := p.Peek(); got.Alerts.RenotifyMinutes != 45 {
		t.Errorf("mantém a última leitura: %+v", got.Alerts)
	}
}

func TestRecipientsFor(t *testing.T) {
	if got := recipientsFor(nil, []string{"env@x.com"}); len(got) != 1 || got[0] != "env@x.com" {
		t.Errorf("vazio usa env: %v", got)
	}
	if got := recipientsFor([]string{"db@x.com"}, []string{"env@x.com"}); got[0] != "db@x.com" {
		t.Errorf("banco vence: %v", got)
	}
}
