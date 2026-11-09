package finance

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// CT-15 pacote 8 (P15-31): opt-in latency measurement of the chat flows added in
// packages 3–6 with representative synthetic volume. It is a local measurement on
// a disposable database, not an SLA. Run with FINANCE_PERF=1 (no -race).
func TestConversationLatencyWithRepresentativeVolume(t *testing.T) {
	if os.Getenv("FINANCE_PERF") != "1" {
		t.Skip("set FINANCE_PERF=1 to measure chat latency")
	}
	f := newReceiptFixture(t)
	ctx := f.ctx
	pool := f.documents.store.pool
	// 240 sheets (12 months x 20 clients) with 40 lines each.
	for month := 1; month <= 12; month++ {
		for client := 0; client < 20; client++ {
			title, period := fmt.Sprintf("Cliente %02d", client), fmt.Sprintf("2026-%02d", month)
			lines := make([]Line, 40)
			for i := range lines {
				lines[i] = Line{Description: fmt.Sprintf("Linha %d", i), Amount: float64(10 + i)}
			}
			if _, err := f.service.CreateSheet(ctx, f.account, SheetInput{Title: &title, Period: &period, Saidas: lines[:20], Entradas: lines[20:]}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := pool.Exec(ctx, `insert into finance.fiscal_customers(account_id,user_id,name,document,service_description,amount)
 select $1::uuid,$2::uuid,'Cliente Perf '||g,lpad(g::text,11,'0'),'Serviço '||g,100+g from generate_series(1,500) g`, f.account, f.user); err != nil {
		t.Fatal(err)
	}
	profileFixture := FiscalProfile{LegalName: "Perf", Document: "12345678000195", IssuerType: "mei", TaxRegime: "MEI sintético", Municipality: "Aracaju", State: "SE", Street: "Rua", Number: "1", District: "Centro", PostalCode: "49000000", PortalURL: nationalPortalLogin}
	view, _ := f.documents.fiscal.GetProfile(ctx, f.account, f.user)
	if _, err := f.documents.fiscal.SaveProfile(ctx, f.account, f.user, FiscalProfileInput{Profile: profileFixture, ExpectedUpdatedAt: view.UpdatedAt}); err != nil {
		t.Fatal(err)
	}
	n := 5000
	timed := func(text string) time.Duration {
		n++
		start := time.Now()
		if _, err := f.chat.Send(ctx, f.account, f.user, AccessConversationInput{RequestID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Text: text}); err != nil {
			t.Fatal(text, err)
		}
		return time.Since(start)
	}
	cases := map[string][]string{
		"leitura com período (20 planilhas no mês → escolha)": {"resumo 2026-11"},
		"escolha numerada + leitura":                          {"resumo 2026-10", "3"},
		"pedido de nota com cliente salvo (500 clientes)":     {"faz uma nota pro Cliente Perf 250"},
		"consulta lateral com pergunta pendente":              {"criar nota", "quanto gastei em 2026-09?"},
		"pedido composto":                                     {"mostra o saldo e depois faz uma nota"},
	}
	report := map[string]any{"date": time.Now().UTC().Format(time.RFC3339), "volume": map[string]int{"sheets": 240, "linesPerSheet": 40, "customers": 500}, "repetitions": 15, "aiCalls": 0, "note": "local disposable PostgreSQL in Docker; not an SLA"}
	results := map[string]map[string]float64{}
	for name, steps := range cases {
		samples := []float64{}
		for range 15 {
			for i, step := range steps {
				d := timed(step)
				if i == len(steps)-1 {
					samples = append(samples, float64(d.Microseconds())/1000)
				}
			}
		}
		sort.Float64s(samples)
		results[name] = map[string]float64{"p50ms": samples[len(samples)/2], "p95ms": samples[(len(samples)*95+99)/100-1]}
	}
	start := time.Now()
	for range 15 {
		if _, err := f.chat.List(ctx, f.account, f.user); err != nil {
			t.Fatal(err)
		}
	}
	results["carregar histórico (60 mensagens)"] = map[string]float64{"meanMs": float64(time.Since(start).Microseconds()) / 1000 / 15}
	report["results"] = results
	raw, _ := json.MarshalIndent(report, "", "  ")
	if out := os.Getenv("FINANCE_PERF_OUTPUT"); out != "" {
		if err := os.WriteFile(out, append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(strings.TrimSpace(string(raw)))
}
