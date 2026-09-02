package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// ============================================================================
// F13 — Custo de LLM por conta (C7). AGREGA o que a F9 gravou; nao recalcula
// ============================================================================
//
// Fonte: messaging.ai_runs (F9). cost_usd e CONGELADO no dispatch (com o preco vigente
// naquele instante) e NUNCA recalculado na leitura — recalcular reescreveria a fatura do mes
// passado quando o preco muda (C7). Aqui so se SOMA o que esta gravado.
//
// "priced" e derivado no READ da tabela de preco viva (core.platform_settings key
// 'ai_model_pricing', a mesma que a F9 usa no dispatch — store_ai_runtime.go:ModelPricing):
// resposta autoritativa "existe preco para este modelo?", sem a heuristica (tokens>0 e
// custo=0) que a spec proibe. Modelo sem preco entra em unpricedModels e a tela avisa
// "preco nao cadastrado", jamais "US$ 0,00" (principio 5).

const pricingKey = "ai_model_pricing"

// UsageTotals soma o periodo inteiro.
type UsageTotals struct {
	Runs             int64   `json:"runs"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	CostUsd          float64 `json:"costUsd"`
}

// ModelUsage e a quebra por (provider, model). priced = existe preco cadastrado hoje.
type ModelUsage struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Runs        int64   `json:"runs"`
	TotalTokens int64   `json:"totalTokens"`
	CostUsd     float64 `json:"costUsd"`
	Priced      bool    `json:"priced"`
}

// UsageLimit casa o teto mensal (monthly_ai_runs, leitor da F3) com o consumo — custo e teto
// na mesma tela. nil no report => "Sem limite cadastrado" (nunca 0).
type UsageLimit struct {
	MonthlyAiRuns int64  `json:"monthlyAiRuns"`
	Used          int64  `json:"used"`
	Source        string `json:"source"`
}

// UsageReport e a resposta de GET /ai/usage (C7).
type UsageReport struct {
	Totals         UsageTotals  `json:"totals"`
	ByModel        []ModelUsage `json:"byModel"`
	UnpricedModels []string     `json:"unpricedModels"`
	Limit          *UsageLimit  `json:"limit"`
}

// modelAgg e uma linha crua da agregacao (antes de casar com o preco).
type modelAgg struct {
	Provider         string
	Model            string
	Runs             int64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CostUsd          float64
}

// CostService agrega o custo por conta. So leitura. accountID vem SEMPRE do Principal.
type CostService struct {
	pool   *pgxpool.Pool
	store  *Store
	limits *modules.LimitReader
}

// NewCostService monta o servico. limits = o leitor de teto da F3 (monthly_ai_runs).
func NewCostService(pool *pgxpool.Pool, limits *modules.LimitReader) *CostService {
	return &CostService{pool: pool, store: NewStore(pool), limits: limits}
}

// Usage agrega ai_runs no periodo [from, toExclusive) da conta e monta o report. Usa o indice
// messaging_ai_runs_account_created_idx (account_id, created_at) — sem varredura.
func (s *CostService) Usage(ctx context.Context, accountID, userID string, from, toExclusive time.Time) (UsageReport, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, userID)
	if err != nil {
		return UsageReport{}, err
	}
	if !scope.Eligible || !scope.allowsPermission("omnichannel.audit.view") {
		return UsageReport{}, ErrForbidden
	}
	groups, err := s.aggregate(ctx, accountID, from, toExclusive)
	if err != nil {
		return UsageReport{}, err
	}
	priced, err := s.pricedModels(ctx)
	if err != nil {
		return UsageReport{}, err
	}
	limit, err := s.limit(ctx, accountID)
	if err != nil {
		return UsageReport{}, err
	}
	return assembleUsage(groups, priced, limit), nil
}

// aggregate soma ai_runs por (provider, model). SEM filtro de status: a fatura conta toda
// tentativa que gerou custo, e as tentativas sem modelo (0 tokens) aparecem honestas.
func (s *CostService) aggregate(ctx context.Context, accountID string, from, toExclusive time.Time) ([]modelAgg, error) {
	rows, err := s.pool.Query(ctx, `select provider, model, count(*),
		coalesce(sum(prompt_tokens),0), coalesce(sum(completion_tokens),0),
		coalesce(sum(total_tokens),0), coalesce(sum(cost_usd),0)::float8
		from messaging.ai_runs
		where account_id = $1::uuid and created_at >= $2 and created_at < $3
		group by provider, model
		order by coalesce(sum(total_tokens),0) desc, provider, model`,
		accountID, from, toExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]modelAgg, 0)
	for rows.Next() {
		var g modelAgg
		if err := rows.Scan(&g.Provider, &g.Model, &g.Runs, &g.PromptTokens,
			&g.CompletionTokens, &g.TotalTokens, &g.CostUsd); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// pricedModels devolve o conjunto {provider/model} que TEM preco hoje (config viva). Ausente/
// ilegivel => conjunto vazio (tudo vira "preco nao cadastrado" — aviso honesto, nao 0 mudo).
func (s *CostService) pricedModels(ctx context.Context) (map[string]bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `select config from core.platform_settings where key = $1`, pricingKey).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return map[string]bool{}, nil
	case err != nil:
		return nil, err
	}
	var table map[string]map[string]json.RawMessage
	if json.Unmarshal(raw, &table) != nil {
		return map[string]bool{}, nil
	}
	set := make(map[string]bool)
	for provider, models := range table {
		for model := range models {
			set[provider+"/"+model] = true
		}
	}
	return set, nil
}

// limit resolve monthly_ai_runs (F3) + o consumido no mes corrente. Sem teto configurado =>
// nil (a tela mostra "Sem limite cadastrado", nunca 0).
func (s *CostService) limit(ctx context.Context, accountID string) (*UsageLimit, error) {
	lim, err := s.limits.Resolve(ctx, accountID, "omnichannel", "monthly_ai_runs")
	if err != nil {
		return nil, err
	}
	if !lim.Set {
		return nil, nil
	}
	var used int64
	err = s.pool.QueryRow(ctx, `select count(*) from messaging.ai_runs
		where account_id = $1::uuid and status = 'ok' and created_at >= date_trunc('month', now())`,
		accountID).Scan(&used)
	if err != nil {
		return nil, err
	}
	return &UsageLimit{MonthlyAiRuns: lim.Value, Used: used, Source: lim.Source}, nil
}

// assembleUsage monta o report a partir das linhas cruas + o conjunto de precos. Funcao PURA
// (sem IO) — o nucleo testavel da precificacao/soma.
func assembleUsage(groups []modelAgg, priced map[string]bool, limit *UsageLimit) UsageReport {
	report := UsageReport{
		ByModel:        make([]ModelUsage, 0, len(groups)),
		UnpricedModels: make([]string, 0),
		Limit:          limit,
	}
	for _, g := range groups {
		key := g.Provider + "/" + g.Model
		isPriced := priced[key]
		report.ByModel = append(report.ByModel, ModelUsage{
			Provider: g.Provider, Model: g.Model, Runs: g.Runs,
			TotalTokens: g.TotalTokens, CostUsd: g.CostUsd, Priced: isPriced,
		})
		report.Totals.Runs += g.Runs
		report.Totals.PromptTokens += g.PromptTokens
		report.Totals.CompletionTokens += g.CompletionTokens
		report.Totals.TotalTokens += g.TotalTokens
		report.Totals.CostUsd += g.CostUsd
		// So marca "sem preco" quando o modelo REALMENTE consumiu (tokens>0): tentativa sem
		// modelo (gate) tem 0 tokens e nao e um preco faltando.
		if !isPriced && g.TotalTokens > 0 {
			report.UnpricedModels = append(report.UnpricedModels, key)
		}
	}
	return report
}
