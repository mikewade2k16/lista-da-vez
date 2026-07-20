package omnichannel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// F9 — Persistencia do RUNTIME da triagem: run, contagem mensal, contexto do prompt
// ============================================================================
//
// Toda query filtra por account_id (defesa em profundidade). Nada aqui escreve
// conversation.state nem queue_id — a maquina/motor da F8 sao donos disso. Esta camada so
// grava ai_runs, le o contexto do prompt e faz o MERGE dos extracted_fields (o motor le).

// aiRunInsert e a linha a gravar em ai_runs (uma por TENTATIVA, inclusive as sem modelo).
// Ponteiros nos ids opcionais: simulate nao tem conversa/mensagem; gate 2 nem tem run.
type aiRunInsert struct {
	AccountID        string
	ConversationID   *string
	AgentID          *string
	AgentVersionID   *string
	MessageID        *string
	Status           string
	Provider         string
	Model            string
	SchemaVersion    string
	Input            json.RawMessage // MASCARADO antes de chegar aqui (§10)
	Output           json.RawMessage
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostUSD          float64
	LatencyMs        int
	Error            string
}

// InsertRun grava um run e devolve o id. input JA vem mascarado; error JAMAIS carrega prompt/
// chave (so a classe do erro). E a base do custo por conta da F13.
func (s *Store) InsertRun(ctx context.Context, in aiRunInsert) (string, error) {
	input := in.Input
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	output := in.Output
	if len(output) == 0 {
		output = json.RawMessage(`{}`)
	}
	var id string
	err := s.pool.QueryRow(ctx, `insert into messaging.ai_runs
		(account_id, conversation_id, agent_id, agent_version_id, message_id, status,
		 provider, model, schema_version, input, output, prompt_tokens, completion_tokens,
		 total_tokens, cost_usd, latency_ms, error)
		values ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6, $7, $8, $9,
		        $10::jsonb, $11::jsonb, $12, $13, $14, $15, $16, $17)
		returning id::text`,
		in.AccountID, in.ConversationID, in.AgentID, in.AgentVersionID, in.MessageID,
		in.Status, in.Provider, in.Model, in.SchemaVersion, input, output,
		in.PromptTokens, in.CompletionTokens, in.TotalTokens, in.CostUSD, in.LatencyMs,
		in.Error).Scan(&id)
	return id, err
}

// CountRunsThisMonth conta os runs que EFETIVAMENTE consumiram o modelo neste mes (status ok),
// para o gate de limite mensal (C9.6 gate 3). blocked/limit_exceeded/no_agent nao chamaram o
// modelo — nao contam. A janela e o mes-calendario corrente (date_trunc('month', now())).
func (s *Store) CountRunsThisMonth(ctx context.Context, accountID string) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `select count(*) from messaging.ai_runs
		where account_id = $1::uuid and status = 'ok'
		  and created_at >= date_trunc('month', now())`, accountID).Scan(&n)
	return n, err
}

// ActiveAgent devolve o agente HABILITADO da conta com active_version_id apontado (gate 2 e a
// resolucao de HasActiveAgent da F8). Sem agente ativo => (_, false, nil): a conversa roteia
// direto (nota 1 da maquina). Se houver mais de um, o mais antigo vence (determinismo).
func (s *Store) ActiveAgent(ctx context.Context, accountID string) (agentRow, bool, error) {
	row, err := scanAgent(s.pool.QueryRow(ctx, `select `+agentCols+`
		from messaging.ai_agents
		where account_id = $1::uuid and enabled and active_version_id is not null
		order by created_at, id limit 1`, accountID))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return agentRow{}, false, nil
	case err != nil:
		return agentRow{}, false, err
	default:
		return row, true, nil
	}
}

// convTriage e o minimo que o dispatch precisa da conversa: o state (gate 1) e os campos que
// alimentam o RoutingContext do motor (contact_phone, instance_scope_key).
type convTriage struct {
	State            string
	ContactPhone     *string
	ContactName      *string
	InstanceScopeKey string
	ExtractedFields  json.RawMessage
	Found            bool
}

// ConvTriageContext le o contexto da conversa para o dispatch. Fora de escopo => Found=false.
func (s *Store) ConvTriageContext(ctx context.Context, accountID, convID string) (convTriage, error) {
	var c convTriage
	err := s.pool.QueryRow(ctx, `select state, contact_phone, contact_name, instance_scope_key,
		coalesce(extracted_fields, '{}'::jsonb)
		from messaging.conversations
		where account_id = $1::uuid and id = $2::uuid`, accountID, convID).
		Scan(&c.State, &c.ContactPhone, &c.ContactName, &c.InstanceScopeKey, &c.ExtractedFields)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return convTriage{Found: false}, nil
	case err != nil:
		return convTriage{}, err
	default:
		c.Found = true
		return c, nil
	}
}

// RecentMessages devolve as ultimas `limit` mensagens da conversa em ordem cronologica (a
// janela da camada 7 do prompt). role: contact (INBOUND) | agent (OUTBOUND). Filtra por account.
func (s *Store) RecentMessages(ctx context.Context, accountID, convID string, limit int) ([]SimMessage, error) {
	rows, err := s.pool.Query(ctx, `select direction, content from (
		select direction, content, created_at, id from messaging.messages
		where account_id = $1::uuid and conversation_id = $2::uuid
		order by created_at desc, id desc limit $3
	) recent order by created_at asc, id asc`, accountID, convID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SimMessage, 0, limit)
	for rows.Next() {
		var direction, content string
		if err := rows.Scan(&direction, &content); err != nil {
			return nil, err
		}
		role := "agent"
		if direction == "INBOUND" {
			role = "contact"
		}
		out = append(out, SimMessage{Role: role, Text: content})
	}
	return out, rows.Err()
}

// catalogTarget e uma fila destino candidata (camada 4 do prompt: setores/filas — so para a
// IA SUGERIR; o painel nao pode inventar uma fila que nao existe).
type catalogTarget struct {
	DepartmentSlug string
	DepartmentName string
	QueueSlug      string
	QueueName      string
}

// RoutingCatalog lista as filas ativas de setores ativos da conta (camada 4). Ordem estavel.
func (s *Store) RoutingCatalog(ctx context.Context, accountID string) ([]catalogTarget, error) {
	rows, err := s.pool.Query(ctx, `select d.slug, d.name, q.slug, q.name
		from messaging.queues q
		join messaging.departments d on d.id = q.department_id and d.account_id = q.account_id
		where q.account_id = $1::uuid and q.is_active and d.is_active
		order by d.slug, q.slug`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]catalogTarget, 0)
	for rows.Next() {
		var t catalogTarget
		if err := rows.Scan(&t.DepartmentSlug, &t.DepartmentName, &t.QueueSlug, &t.QueueName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MergeExtractedFields funde os campos extraidos pela IA em conversation.extracted_fields
// (jsonb || jsonb: preserva os anteriores e sobrescreve as chaves novas). Filtra por account.
// NAO muda state/queue — o motor da F8 e quem roteia lendo estes campos.
func (s *Store) MergeExtractedFields(ctx context.Context, accountID, convID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `update messaging.conversations
		set extracted_fields = coalesce(extracted_fields, '{}'::jsonb) || $3::jsonb,
		    updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, convID, raw)
	return err
}

// RuleSummary devolve nome/prioridade de uma regra (para o traco do simulate: matchedRule).
// Filtra por account. Regra fora de escopo => ok=false (sem erro).
func (s *Store) RuleSummary(ctx context.Context, accountID, ruleID string) (name string, priority int, ok bool, err error) {
	err = s.pool.QueryRow(ctx, `select name, priority from messaging.routing_rules
		where account_id = $1::uuid and id = $2::uuid`, accountID, ruleID).Scan(&name, &priority)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", 0, false, nil
	case err != nil:
		return "", 0, false, err
	default:
		return name, priority, true, nil
	}
}

// modelPrice e o preco por 1k tokens de um modelo (input/output), lido de platform_settings.
type modelPrice struct {
	InputPer1kUSD  float64 `json:"inputPer1kUsd"`
	OutputPer1kUSD float64 `json:"outputPer1kUsd"`
}

// ModelPricing resolve o preco (provider, model) de core.platform_settings key
// 'ai_model_pricing' — shape { "<provider>": { "<model>": {inputPer1kUsd, outputPer1kUsd} } }.
// Ausente => (_, false): custo 0 (a F13 e a dona da precificacao; aqui e so a base). NUNCA
// hardcode de preco — a fonte e o banco (principio 1).
func (s *Store) ModelPricing(ctx context.Context, provider, model string) (modelPrice, bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `select config from core.platform_settings
		where key = 'ai_model_pricing'`).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return modelPrice{}, false, nil
	case err != nil:
		return modelPrice{}, false, err
	}
	var table map[string]map[string]modelPrice
	if err := json.Unmarshal(raw, &table); err != nil {
		return modelPrice{}, false, nil // config malformada nao derruba a triagem
	}
	models, ok := table[provider]
	if !ok {
		return modelPrice{}, false, nil
	}
	price, ok := models[model]
	return price, ok, nil
}
