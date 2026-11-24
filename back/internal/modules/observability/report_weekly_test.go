package observability

import (
	"strings"
	"testing"
	"time"
)

func TestWeeklyDue(t *testing.T) {
	loc, _ := loadZone("America/Sao_Paulo")
	cfg := WeeklySettings{Enabled: true, Weekday: int(time.Monday), Hour: 8, Timezone: "America/Sao_Paulo"}
	monday := func(hour, minute int) time.Time { return time.Date(2026, 11, 23, hour, minute, 0, 0, loc).UTC() }
	cases := []struct {
		name  string
		now   time.Time
		state weeklyState
		want  bool
	}{
		{"antes do horário", monday(7, 59), weeklyState{}, false},
		{"no horário", monday(8, 0), weeklyState{}, true},
		{"minutos depois", monday(8, 30), weeklyState{}, true},
		{"já enviou hoje", monday(9, 0), weeklyState{LastSentAt: ptr(monday(8, 1))}, false},
		{"enviou na semana passada", monday(9, 0), weeklyState{LastSentAt: ptr(monday(8, 0).AddDate(0, 0, -7))}, true},
		{"enviou antes do horário (manual)", monday(9, 0), weeklyState{LastSentAt: ptr(monday(7, 0))}, true},
		{"falhou há 5 min", monday(9, 0), weeklyState{LastAttemptAt: ptr(monday(8, 55))}, false},
		{"falhou há 20 min", monday(9, 20), weeklyState{LastAttemptAt: ptr(monday(9, 0))}, true},
		{"fora da janela de 24 h", monday(8, 0).Add(25 * time.Hour), weeklyState{}, false},
		{"dentro da janela de 24 h", monday(8, 0).Add(23 * time.Hour), weeklyState{}, true},
		{"quarta, sem envio", monday(8, 0).AddDate(0, 0, 2), weeklyState{}, false},
	}
	for _, tc := range cases {
		if got := weeklyDue(cfg, tc.state, tc.now); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
	cfg.Enabled = false
	if weeklyDue(cfg, weeklyState{}, monday(8, 0)) {
		t.Errorf("desligado nunca envia")
	}
	cfg.Enabled, cfg.Timezone = true, "Marte/Olympus"
	if weeklyDue(cfg, weeklyState{}, monday(8, 0)) {
		t.Errorf("fuso inválido não envia")
	}
}

func TestLastScheduledAcrossWeek(t *testing.T) {
	loc, _ := loadZone("America/Sao_Paulo")
	cfg := WeeklySettings{Weekday: int(time.Sunday), Hour: 20}
	// segunda 08:00 -> o último domingo 20:00
	got := lastScheduled(cfg, time.Date(2026, 11, 23, 8, 0, 0, 0, loc))
	if want := time.Date(2026, 11, 22, 20, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
	// domingo 19:00 -> o domingo anterior
	got = lastScheduled(cfg, time.Date(2026, 11, 22, 19, 0, 0, 0, loc))
	if want := time.Date(2026, 11, 15, 20, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func weekOf(buckets ...int64) routeWeek {
	var calls int64
	for _, value := range buckets {
		calls += value
	}
	return routeWeek{Calls: calls, Buckets: buckets}
}

func TestCompareRoutes(t *testing.T) {
	// buckets: índice cresce com a latência (LatencyBucketsMS); tudo em um bucket para p95 estável
	fast := weekOf(100)                         // p95 baixo
	slow := weekOf(0, 0, 0, 0, 0, 0, 0, 0, 100) // p95 alto
	cur := map[string]routeWeek{"GET /a": slow, "GET /b": fast, "GET /c": slow, "GET /novo": slow, "GET /poucos": weekOf(0, 0, 0, 0, 0, 0, 0, 0, 10)}
	prev := map[string]routeWeek{"GET /a": fast, "GET /b": slow, "GET /c": slow, "GET /poucos": fast}
	worse, better := compareRoutes(cur, prev)
	if len(worse) != 1 || worse[0].Route != "GET /a" || worse[0].CurP95 <= worse[0].PrevP95 {
		t.Errorf("piorou: %+v", worse)
	}
	if len(better) != 1 || better[0].Route != "GET /b" {
		t.Errorf("melhorou: %+v", better)
	}
	many := map[string]routeWeek{}
	manyPrev := map[string]routeWeek{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		many["GET /"+name], manyPrev["GET /"+name] = slow, fast
	}
	if worse, _ = compareRoutes(many, manyPrev); len(worse) != 5 {
		t.Errorf("top 5: %d", len(worse))
	}
}

func TestSummarizeDeploys(t *testing.T) {
	from := t0.AddDate(0, 0, -7)
	dur := func(ms float64) *float64 { return &ms }
	rows := []deployRow{
		{StartedAt: t0.Add(-time.Hour), Status: "success", DurationMs: dur(120000)},
		{StartedAt: t0.Add(-2 * time.Hour), Status: "failed"},
		{StartedAt: t0.Add(-3 * time.Hour), Status: "success", DurationMs: dur(60000)},
		{StartedAt: t0.Add(-4 * time.Hour), Status: "running"},
		{StartedAt: from.Add(-time.Hour), Status: "success", DurationMs: dur(1)},
	}
	week := summarizeDeploys(rows, from, t0)
	if week.Count != 4 || week.Failed != 1 || week.MedianMs != 90000 {
		t.Errorf("deploys: %+v", week)
	}
}

func TestBuildWeeklyReport(t *testing.T) {
	loc, _ := loadZone("America/Sao_Paulo")
	data := weeklyData{
		From: time.Date(2026, 11, 17, 8, 0, 0, 0, loc), To: time.Date(2026, 11, 24, 8, 0, 0, 0, loc),
		Cur:    trafficWeek{Calls: 1000, P95Ms: 440, LiveTotal: 900, Live5xx: 9},
		Prev:   trafficWeek{Calls: 800, P95Ms: 400, LiveTotal: 700, Live5xx: 7},
		Worse:  []routeDelta{{Route: "GET /lenta", PrevP95: 300, CurP95: 900, Calls: 120}},
		Better: []routeDelta{{Route: "GET /rapida", PrevP95: 900, CurP95: 300, Calls: 80}},

		AlertsOpened:   map[string]int64{sevHigh: 2, sevCritical: 1},
		AlertsOpenNow:  1,
		AlertsResolved: 2,
		AlertsTopRules: []namedCount{{Name: "slow_route", Count: 2}},
		FindingsOpen:   map[string]int64{sevHigh: 3, sevLow: 1}, FindingsOverdue: 2,
		Deploys: deployWeek{Count: 4, Failed: 1, MedianMs: 200000},
		Run:     &runNotes{ID: "20261123-120510", At: time.Date(2026, 11, 23, 12, 5, 0, 0, time.UTC), Scores: []moduleScore{{"crm", 90}, {"bi", 70}}},
	}
	subject, text := buildWeeklyReport(data, "[Omni]", "https://app/x", loc)
	if subject != "[Omni] Relatório semanal - 17/11 a 24/11" {
		t.Errorf("assunto: %q", subject)
	}
	for _, want := range []string{
		"p95: 440 ms (semana anterior: 400 ms, +10%)", "Erros 5xx: 1.00% (9 de 900)", "1. GET /lenta: 300 ms -> 900 ms (+200%, 120 chamadas)",
		"ROTAS QUE MELHORARAM", "Abertos na semana: 3 (1 critico, 2 alto)", "Resolvidos na semana: 2; abertos agora: 1",
		"Slow", "Abertos: 4 (3 alto, 1 baixo)", "Com prazo vencido: 2", "4 deploy(s), 1 com falha", "Nota média: 80", "  - bi: 70", "Painel: https://app/x",
	} {
		if want == "Slow" {
			want = "Rota lenta (p95): 2 vez(es)"
		}
		if !strings.Contains(text, want) {
			t.Errorf("sem %q:\n%s", want, text)
		}
	}
	empty, emptyText := buildWeeklyReport(weeklyData{From: data.From, To: data.To, AlertsOpened: map[string]int64{}, FindingsOpen: map[string]int64{}}, "", "", loc)
	if strings.HasPrefix(empty, " ") || !strings.Contains(emptyText, "Sem dados de requisições") || !strings.Contains(emptyText, "Nenhum deploy") || !strings.Contains(emptyText, "Nenhuma execução completa") {
		t.Errorf("relatório vazio:\n%s\n%s", empty, emptyText)
	}
}

func TestPctChange(t *testing.T) {
	if pctChange(110, 100) != "+10%" || pctChange(50, 100) != "-50%" || pctChange(10, 0) != "sem base de comparação" {
		t.Errorf("pctChange")
	}
}
