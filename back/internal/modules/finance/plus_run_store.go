package finance

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"time"
)

type plusRunStore struct{ assistant *AssistantService }

func (s plusRunStore) reserve(ctx context.Context, sc PlusScope, cfg AssistantConfig, runtime AssistantRuntime, evidence []PlusEvidence, cost *string) (string, error) {
	tx, e := s.assistant.pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, "finance:plus-budget:"+sc.AccountID); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, `delete from finance.plus_llm_runs where account_id=$1::uuid and expires_at<now()`, sc.AccountID); e != nil {
		return "", e
	}
	var seen bool
	if e = tx.QueryRow(ctx, `select exists(select 1 from finance.plus_llm_runs where account_id=$1::uuid and actor_id=$2::uuid and request_id=$3::uuid)`, sc.AccountID, sc.ActorID, sc.RequestID).Scan(&seen); e != nil {
		return "", e
	}
	if seen {
		return "", plusError("request_in_progress", 409)
	}
	var count int
	if e = tx.QueryRow(ctx, `select count(*) from finance.plus_llm_runs where account_id=$1::uuid and created_at>=date_trunc('day',now() at time zone 'UTC') at time zone 'UTC'`, sc.AccountID).Scan(&count); e != nil {
		return "", e
	}
	if count >= cfg.Plus.Explanation.DailyRequests {
		return "", plusError("budget_exhausted", 429)
	}
	refs := []map[string]any{}
	for _, v := range evidence {
		refs = append(refs, map[string]any{"documentId": v.DocumentID, "version": v.Version, "chunkId": v.ChunkID})
	}
	raw, e := json.Marshal(refs)
	if e != nil {
		return "", e
	}
	var price *string
	if p := cfg.Plus.Explanation.Pricing; p != nil {
		price = &p.Version
	}
	var pricingSnapshot json.RawMessage
	if cfg.Plus.Explanation.Pricing != nil {
		pricingSnapshot, e = json.Marshal(cfg.Plus.Explanation.Pricing)
		if e != nil {
			return "", e
		}
	}
	var id string
	e = tx.QueryRow(ctx, `insert into finance.plus_llm_runs(account_id,actor_id,issuer_id,request_id,provider,model,task_class,prompt_version,policy_version,config_version,evidence_refs,reserved_cost_usd,pricing_version,pricing_snapshot)values($1::uuid,$2::uuid,nullif($3,'')::uuid,$4::uuid,$5,$6,$13,1,$7,$8,$9::jsonb,$10::numeric,$11,$12::jsonb)returning id::text`, sc.AccountID, sc.ActorID, sc.IssuerID, sc.RequestID, runtime.Provider, runtime.Model, cfg.Plus.ToolPolicyVersion, cfg.UpdatedAt, raw, cost, price, pricingSnapshot, cfg.Plus.Explanation.TaskClass).Scan(&id)
	if e != nil {
		return "", e
	}
	return id, tx.Commit(ctx)
}
func (s plusRunStore) finish(ctx context.Context, sc PlusScope, id, status string, response llm.Response, latency int, pricing *ExplanationPricing) (PlusRunUsage, error) {
	out := PlusRunUsage{RunID: id, Model: response.Model, Status: status, LatencyMS: latency}
	var input, output, total *int
	if response.UsageKnown && response.Usage.PromptTokens >= 0 && response.Usage.CompletionTokens >= 0 && response.Usage.TotalTokens >= 0 {
		input = &response.Usage.PromptTokens
		output = &response.Usage.CompletionTokens
		total = &response.Usage.TotalTokens
		out.Tokens = total
		out.CostUSD = priceUSD(pricing, *input, *output, time.Now())
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	result, e := s.assistant.pool.Exec(bounded, `update finance.plus_llm_runs set status=$5,prompt_tokens=$6,completion_tokens=$7,total_tokens=$8,latency_ms=$9,cost_usd=$10::numeric,completed_at=now() where account_id=$1::uuid and actor_id=$2::uuid and issuer_id is not distinct from nullif($3,'')::uuid and id=$4::uuid and status='reserved'`, sc.AccountID, sc.ActorID, sc.IssuerID, id, status, input, output, total, latency, out.CostUSD)
	if e == nil && result.RowsAffected() != 1 {
		e = errors.New("run completion conflict")
	}
	return out, e
}
