package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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

// CountAIOutboundTurns conta respostas da IA já persistidas nesta conversa. Mensagens FAILED
// não consomem um turno; PENDING/SENT/ACK contam porque o texto já foi autorizado e enfileirado.
// A consulta é tenant-scoped e serve somente à policy, nunca ao front.
func (s *Store) CountAIOutboundTurns(ctx context.Context, accountID, conversationID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from messaging.messages message
		where account_id = $1::uuid and conversation_id = $2::uuid
		  and direction = 'OUTBOUND' and origin = 'ai' and status <> 'FAILED'
		  and message.created_at > coalesce((select suppression.history_cleared_at
		      from messaging.conversations conversation
		      join messaging.contact_suppressions suppression
		        on suppression.account_id=conversation.account_id and suppression.contact_id=conversation.contact_id
		      where conversation.account_id=message.account_id and conversation.id=message.conversation_id),
		      '-infinity'::timestamptz)`,
		accountID, conversationID).Scan(&n)
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
	AIGeneration     int64
	InstanceID       *string
	ContactID        *string
	ContactPhone     *string
	ContactName      *string
	Channel          string
	ExternalID       string
	InstanceScopeKey string
	ExtractedFields  json.RawMessage
	Found            bool
}

// ConvTriageContext le o contexto da conversa para o dispatch. Fora de escopo => Found=false.
func (s *Store) ConvTriageContext(ctx context.Context, accountID, convID string) (convTriage, error) {
	var c convTriage
	err := s.pool.QueryRow(ctx, `select state, ai_generation, instance_id::text, contact_id::text, contact_phone, contact_name,
		channel, external_id, instance_scope_key, coalesce(extracted_fields, '{}'::jsonb)
		from messaging.conversations
		where account_id = $1::uuid and id = $2::uuid`, accountID, convID).
		Scan(&c.State, &c.AIGeneration, &c.InstanceID, &c.ContactID, &c.ContactPhone, &c.ContactName,
			&c.Channel, &c.ExternalID, &c.InstanceScopeKey, &c.ExtractedFields)
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
	rows, err := s.pool.Query(ctx, `select id::text, direction,
		case when media_context='' then content
			when content='' then media_context
			else content || E'\n' || media_context end as content
		from (
		select message.id, message.direction, message.content, message.created_at,
			coalesce((select string_agg(
				case analysis.analysis_kind
					when 'transcription' then '[Áudio transcrito]: ' || coalesce(analysis.result_text,'')
					when 'vision' then '[Imagem interpretada]: ' || coalesce(analysis.result_text,'')
					when 'video_summary' then '[Vídeo interpretado]: ' || coalesce(analysis.result_text,'')
					when 'document_text' then '[Documento interpretado]: ' || coalesce(analysis.result_text,'')
				end, E'\n' order by analysis.created_at)
			from messaging.media_analyses analysis
			join messaging.ai_agent_versions version
			  on version.account_id=analysis.account_id and version.id=analysis.agent_version_id
			where analysis.account_id=message.account_id and analysis.message_id=message.id
			  and analysis.status='completed'
			  and coalesce((version.media_config->>'includeInReply')::boolean,true)), '') as media_context
		from messaging.messages message
		where message.account_id = $1::uuid and message.conversation_id = $2::uuid
		  and message.created_at > coalesce((select suppression.history_cleared_at
		      from messaging.conversations conversation
		      join messaging.contact_suppressions suppression
		        on suppression.account_id=conversation.account_id and suppression.contact_id=conversation.contact_id
		      where conversation.account_id=message.account_id and conversation.id=message.conversation_id),
		      '-infinity'::timestamptz)
		order by created_at desc, id desc limit $3
	) recent order by created_at asc, id asc`, accountID, convID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SimMessage, 0, limit)
	for rows.Next() {
		var id, direction, content string
		if err := rows.Scan(&id, &direction, &content); err != nil {
			return nil, err
		}
		role := "agent"
		if direction == "INBOUND" {
			role = "contact"
		}
		out = append(out, SimMessage{ID: id, Role: role, Text: content})
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
func (s *Store) CommitAITriage(ctx context.Context, accountID, convID string, generation int64, fields map[string]any) (bool, error) {
	raw, err := json.Marshal(fields)
	if err != nil {
		return false, err
	}
	result, err := s.pool.Exec(ctx, `update messaging.conversations
		set extracted_fields = coalesce(extracted_fields, '{}'::jsonb) || $3::jsonb,
		    updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		  and state = 'ai_active' and ai_generation = $4`, accountID, convID, raw, generation)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

func (s *Store) CommitAITriageWithIntelligence(ctx context.Context, accountID, convID string, generation int64, runID string, out TriageOutput) (bool, error) {
	raw, err := json.Marshal(out.ExtractedFields)
	if err != nil {
		return false, err
	}
	memory := normalizeContactMemory(out.ContactMemory)
	facts := contactMemoryJSON(memory.Facts)
	preferences := contactMemoryJSON(memory.Preferences)
	summary := ""
	if memory.Summary != nil {
		summary = *memory.Summary
	}
	learned := summary != "" || len(memory.Facts) > 0 || len(memory.Preferences) > 0
	var committed bool
	err = s.pool.QueryRow(ctx, `with updated as (
			update messaging.conversations
			set extracted_fields = coalesce(extracted_fields, '{}'::jsonb) || $3::jsonb,
			    updated_at = now()
			where account_id = $1::uuid and id = $2::uuid
			  and state = 'ai_active' and ai_generation = $4
			returning contact_id
		), intelligence as (
			insert into messaging.contact_intelligence (
				account_id, contact_id, summary, facts, preferences, interaction_count,
				ai_reply_count, handoff_count, last_intent, last_sentiment, last_confidence,
				last_outcome, last_conversation_id, last_ai_run_id, last_learned_at
			)
			select $1::uuid, contact_id, $5, $6::jsonb, $7::jsonb, 1,
				case when $8 then 1 else 0 end, case when $9 then 1 else 0 end,
				$10, $11, $12, $13, $2::uuid, nullif($14,'')::uuid,
				case when $15 then now() else null end
			from updated where contact_id is not null
			on conflict (account_id, contact_id) do update set
				summary = case when excluded.summary <> '' then excluded.summary else contact_intelligence.summary end,
				facts = contact_intelligence.facts || excluded.facts,
				preferences = contact_intelligence.preferences || excluded.preferences,
				interaction_count = contact_intelligence.interaction_count + 1,
				ai_reply_count = contact_intelligence.ai_reply_count + excluded.ai_reply_count,
				handoff_count = contact_intelligence.handoff_count + excluded.handoff_count,
				last_intent = excluded.last_intent,
				last_sentiment = excluded.last_sentiment,
				last_confidence = excluded.last_confidence,
				last_outcome = excluded.last_outcome,
				last_conversation_id = excluded.last_conversation_id,
				last_ai_run_id = excluded.last_ai_run_id,
				last_learned_at = case when $15 then now() else contact_intelligence.last_learned_at end,
				updated_at = now()
			returning contact_id
		)
		select exists(select 1 from updated)`,
		accountID, convID, raw, generation, summary, facts, preferences,
		strings.TrimSpace(out.ReplyDraft) != "", out.NeedsHuman, strings.TrimSpace(out.Intent),
		normalizeContactSentiment(out.Sentiment), out.Confidence, triageOutcomeLabel(out), runID, learned,
	).Scan(&committed)
	return committed, err
}

func triageOutcomeLabel(out TriageOutput) string {
	switch {
	case out.NeedsHuman:
		return "handoff"
	case out.CloseRequested:
		return "close_requested"
	case strings.TrimSpace(out.ReplyDraft) != "":
		return "replied"
	default:
		return "no_reply"
	}
}

// GetContactIntelligence returns both the conservative personal name and the
// accumulated structured memory. The contact and every joined row are scoped
// again by account_id as defense in depth.
func (s *Store) GetContactIntelligence(ctx context.Context, accountID, contactID string) (ContactIntelligenceView, error) {
	var out ContactIntelligenceView
	var contactName, phone string
	var identityName *string
	err := s.pool.QueryRow(ctx, `select contact.name, coalesce(contact.phone,''), contact.relationship_status,
			contact.tags, identity.display_name,
			coalesce(intelligence.summary,''), coalesce(intelligence.facts,'{}'::jsonb),
			coalesce(intelligence.preferences,'{}'::jsonb),
			coalesce(intelligence.interaction_count,0), coalesce(intelligence.ai_reply_count,0),
			coalesce(intelligence.handoff_count,0), coalesce(intelligence.last_intent,''),
			coalesce(intelligence.last_sentiment,'unknown'), intelligence.last_confidence::float8,
			coalesce(intelligence.last_outcome,''), intelligence.last_conversation_id::text,
			intelligence.last_learned_at, intelligence.updated_at
		from messaging.contacts contact
		left join messaging.contact_intelligence intelligence
		  on intelligence.account_id=contact.account_id and intelligence.contact_id=contact.id
		left join lateral (
			select display_name from messaging.contact_identities
			where account_id=contact.account_id and contact_id=contact.id and display_name is not null
			order by last_seen_at desc, id desc limit 1
		) identity on true
		where contact.account_id=$1::uuid and contact.id=$2::uuid and contact.archived_at is null`,
		accountID, contactID).Scan(
		&contactName, &phone, &out.RelationshipStatus, &out.Tags, &identityName,
		&out.Summary, &out.Facts, &out.Preferences, &out.InteractionCount,
		&out.AIReplyCount, &out.HandoffCount, &out.LastIntent, &out.LastSentiment,
		&out.LastConfidence, &out.LastOutcome, &out.LastConversationID,
		&out.LastLearnedAt, &out.UpdatedAt,
	)
	if err != nil {
		return ContactIntelligenceView{}, err
	}
	identity := ""
	if identityName != nil {
		identity = *identityName
	}
	name, source := safePreferredPersonalName(contactName, identity)
	if name != "" && name != phone {
		out.PreferredName = &name
	}
	out.NameSource = source
	if out.Facts == nil {
		out.Facts = json.RawMessage(`{}`)
	}
	if out.Preferences == nil {
		out.Preferences = json.RawMessage(`{}`)
	}
	if out.Tags == nil {
		out.Tags = json.RawMessage(`[]`)
	}
	out.RelationshipStatus = crmStatus(out.RelationshipStatus)
	return out, nil
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
