package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresRepository) ListProcesses(ctx context.Context) ([]ProcessDefinition, error) {
	rows, err := s.pool.Query(ctx, `
		select d.id, d.process_key, d.label, d.description, d.status,
		       coalesce(v.schema_version, ''), coalesce(v.input_schema, '{}'::jsonb),
		       coalesce(v.output_schema, '{}'::jsonb), coalesce(v.allowed_variables, '[]'::jsonb)
		from intelligence.process_definitions d
		left join intelligence.process_config_versions v
		  on v.id = d.active_config_version_id
		order by d.process_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProcessDefinition, 0)
	for rows.Next() {
		var item ProcessDefinition
		var input, output, variables []byte
		if err := rows.Scan(
			&item.ID, &item.Key, &item.Label, &item.Description, &item.Status,
			&item.SchemaVersion, &input, &output, &variables,
		); err != nil {
			return nil, err
		}
		item.InputSchema = input
		item.OutputSchema = output
		item.AllowedVariables = decodeStrings(variables)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) ListPromptVersions(
	ctx context.Context,
	accountID, clientAccountID, processKey string,
) ([]PromptVersion, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.account_id, coalesce(p.client_account_id::text, ''),
		       p.process_key, p.layer_kind, p.version, p.status, p.content,
		       p.variables, coalesce(c.output_schema, '{}'::jsonb), p.revision,
		       p.created_at, p.validated_at, p.published_at
		from intelligence.prompt_versions p
		join intelligence.process_definitions d on d.id = p.process_definition_id
		left join intelligence.process_config_versions c on c.id = d.active_config_version_id
		where p.account_id = $1
		  and p.client_account_id is not distinct from nullif($2, '')::uuid
		  and ($3 = '' or p.process_key = $3)
		order by p.process_key, p.layer_kind, p.version desc`,
		accountID, clientAccountID, processKey,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PromptVersion, 0)
	for rows.Next() {
		var item PromptVersion
		var variables, output []byte
		if err := rows.Scan(
			&item.ID, &item.AccountID, &item.ClientAccountID, &item.ProcessKey,
			&item.Layer, &item.Version, &item.Status, &item.Content, &variables,
			&output, &item.Revision, &item.CreatedAt, &item.ValidatedAt,
			&item.PublishedAt,
		); err != nil {
			return nil, err
		}
		item.Variables = decodeStrings(variables)
		item.OutputSchema = output
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) ListPromptBindings(
	ctx context.Context,
	accountID, clientAccountID, processKey string,
) ([]PromptBinding, error) {
	rows, err := s.pool.Query(ctx, `
		select id, account_id, coalesce(client_account_id::text, ''),
		       process_key, process_prompt_version_id, agent_version_id,
		       status, revision, coalesce(published_at, created_at)
		from intelligence.prompt_bindings
		where account_id = $1
		  and client_account_id is not distinct from nullif($2, '')::uuid
		  and ($3 = '' or process_key = $3)
		order by process_key, published_at desc nulls last, created_at desc`,
		accountID, clientAccountID, processKey,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PromptBinding, 0)
	for rows.Next() {
		var item PromptBinding
		if err := rows.Scan(
			&item.ID, &item.AccountID, &item.ClientAccountID,
			&item.ProcessKey, &item.ProcessPromptVersionID,
			&item.AgentVersionID, &item.Status, &item.Revision,
			&item.PublishedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) CreatePromptDraft(
	ctx context.Context,
	accountID, actorID string,
	input PromptDraftInput,
	variables []string,
) (PromptVersion, error) {
	vars, _ := json.Marshal(variables)
	var item PromptVersion
	var rawVars, output []byte
	err := s.pool.QueryRow(ctx, `
		with selected as (
		    select d.id as process_definition_id, p.id as prompt_definition_id,
		           c.output_schema
		    from intelligence.process_definitions d
		    join intelligence.prompt_definitions p
		      on p.process_definition_id = d.id and p.layer_kind = $4
		    join intelligence.process_config_versions c
		      on c.id = d.active_config_version_id and c.status = 'published'
		    where d.process_key = $3 and d.status = 'registered'
		),
		next_version as (
		    select coalesce(max(v.version), 0) + 1 as version
		    from intelligence.prompt_versions v
		    join selected x on x.prompt_definition_id = v.prompt_definition_id
		    where v.account_id = $1
		      and v.client_account_id is not distinct from nullif($2, '')::uuid
		      and v.layer_kind = $4
		)
		insert into intelligence.prompt_versions (
		    account_id, client_account_id, process_definition_id,
		    prompt_definition_id, process_key, layer_kind, version, status,
		    content, content_hash, variables, based_on_version_id, created_by_user_id
		)
		select $1, nullif($2, '')::uuid, x.process_definition_id,
		       x.prompt_definition_id, $3, $4, n.version, 'draft',
		       $5, encode(digest($5, 'sha256'), 'hex'), $6,
		       nullif($7, '')::uuid, nullif($8, '')::uuid
		from selected x cross join next_version n
		returning id, account_id, coalesce(client_account_id::text, ''),
		          process_key, layer_kind, version, status, content, variables,
		          (select output_schema from selected), revision, created_at,
		          validated_at, published_at`,
		accountID, input.ClientAccountID, input.ProcessKey, input.Layer,
		input.Content, vars, input.BasedOnVersionID, actorID,
	).Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID, &item.ProcessKey,
		&item.Layer, &item.Version, &item.Status, &item.Content, &rawVars,
		&output, &item.Revision, &item.CreatedAt, &item.ValidatedAt,
		&item.PublishedAt,
	)
	item.Variables = decodeStrings(rawVars)
	item.OutputSchema = output
	return item, repositoryError(err)
}

func (s *PostgresRepository) GetPromptVersion(
	ctx context.Context,
	accountID, id string,
) (PromptVersion, error) {
	var item PromptVersion
	var variables, output []byte
	err := s.pool.QueryRow(ctx, `
		select p.id, p.account_id, coalesce(p.client_account_id::text, ''),
		       p.process_key, p.layer_kind, p.version, p.status, p.content,
		       p.variables, c.output_schema, p.revision, p.created_at,
		       p.validated_at, p.published_at
		from intelligence.prompt_versions p
		join intelligence.process_definitions d on d.id = p.process_definition_id
		join intelligence.process_config_versions c on c.id = d.active_config_version_id
		where p.account_id = $1 and p.id = $2`,
		accountID, id,
	).Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID, &item.ProcessKey,
		&item.Layer, &item.Version, &item.Status, &item.Content, &variables,
		&output, &item.Revision, &item.CreatedAt, &item.ValidatedAt,
		&item.PublishedAt,
	)
	item.Variables = decodeStrings(variables)
	item.OutputSchema = output
	return item, repositoryError(err)
}

func (s *PostgresRepository) UpdatePromptDraft(
	ctx context.Context,
	accountID, actorID, id, content string,
	variables []string,
	expectedRevision int64,
) (PromptVersion, error) {
	vars, _ := json.Marshal(variables)
	tag, err := s.pool.Exec(ctx, `
		update intelligence.prompt_versions
		set content = $3, content_hash = encode(digest($3, 'sha256'), 'hex'),
		    variables = $4, status = 'draft', validated_by_user_id = null,
		    validated_at = null, revision = revision + 1,
		    updated_by_user_id = nullif($6, '')::uuid
		where account_id = $1 and id = $2 and status in ('draft', 'validated')
		  and revision = $5`,
		accountID, id, content, vars, expectedRevision, actorID,
	)
	if err != nil {
		return PromptVersion{}, repositoryError(err)
	}
	if tag.RowsAffected() == 0 {
		return PromptVersion{}, ErrConflict
	}
	return s.GetPromptVersion(ctx, accountID, id)
}

func (s *PostgresRepository) MarkPromptValidated(
	ctx context.Context,
	accountID, actorID, id string,
	variables []string,
) (PromptVersion, error) {
	vars, _ := json.Marshal(variables)
	tag, err := s.pool.Exec(ctx, `
		update intelligence.prompt_versions
		set status = 'validated', variables = $3,
		    validated_by_user_id = nullif($4, '')::uuid,
		    updated_by_user_id = nullif($4, '')::uuid,
		    validated_at = now(), revision = revision + 1
		where account_id = $1 and id = $2 and status in ('draft', 'validated')`,
		accountID, id, vars, actorID,
	)
	if err != nil {
		return PromptVersion{}, repositoryError(err)
	}
	if tag.RowsAffected() == 0 {
		return PromptVersion{}, ErrConflict
	}
	return s.GetPromptVersion(ctx, accountID, id)
}

func (s *PostgresRepository) CreatePromptEvaluation(
	ctx context.Context,
	accountID, actorID, promptVersionID, status string,
	reasonCodes []string,
	scores json.RawMessage,
) (PromptEvaluation, error) {
	reasons, _ := json.Marshal(reasonCodes)
	var item PromptEvaluation
	var rawScores, rawReasons []byte
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.prompt_evaluations (
		    account_id, client_account_id, process_definition_id,
		    prompt_version_id, status, scores, reason_codes, created_by_user_id
		)
		select p.account_id, p.client_account_id, p.process_definition_id,
		       p.id, $3, $4, $5, nullif($6, '')::uuid
		from intelligence.prompt_versions p
		where p.account_id = $1 and p.id = $2
		returning id, status, scores, reason_codes, created_at`,
		accountID, promptVersionID, status, normalizedJSON(scores, `{}`),
		reasons, actorID,
	).Scan(&item.ID, &item.Status, &rawScores, &rawReasons, &item.CreatedAt)
	item.Scores = rawScores
	item.ReasonCodes = decodeStrings(rawReasons)
	return item, repositoryError(err)
}

func (s *PostgresRepository) ListPromptEvaluations(
	ctx context.Context,
	accountID, promptVersionID string,
	limit int,
) ([]PromptEvaluation, error) {
	rows, err := s.pool.Query(ctx, `
		select id, status, scores, reason_codes, created_at
		from intelligence.prompt_evaluations
		where account_id = $1 and prompt_version_id = $2
		order by created_at desc, id desc
		limit $3`,
		accountID, promptVersionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PromptEvaluation, 0)
	for rows.Next() {
		var item PromptEvaluation
		var scores, reasons []byte
		if err := rows.Scan(
			&item.ID, &item.Status, &scores, &reasons, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Scores = scores
		item.ReasonCodes = decodeStrings(reasons)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) PublishPrompt(
	ctx context.Context,
	accountID, actorID, promptVersionID string,
	input PublishPromptInput,
) (PromptBinding, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PromptBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var prompt PromptVersion
	var processDefinitionID, promptDefinitionID string
	err = tx.QueryRow(ctx, `
		select p.id, p.account_id, coalesce(p.client_account_id::text, ''),
		       p.process_key, p.layer_kind, p.version, p.status, p.content,
		       p.variables, c.output_schema, p.revision, p.created_at,
		       p.validated_at, p.published_at, p.process_definition_id,
		       p.prompt_definition_id
		from intelligence.prompt_versions p
		join intelligence.process_definitions d on d.id = p.process_definition_id
		join intelligence.process_config_versions c on c.id = d.active_config_version_id
		where p.account_id = $1 and p.id = $2
		for update of p`,
		accountID, promptVersionID,
	).Scan(
		&prompt.ID, &prompt.AccountID, &prompt.ClientAccountID, &prompt.ProcessKey,
		&prompt.Layer, &prompt.Version, &prompt.Status, &prompt.Content,
		new([]byte), &prompt.OutputSchema, &prompt.Revision, &prompt.CreatedAt,
		&prompt.ValidatedAt, &prompt.PublishedAt, &processDefinitionID,
		&promptDefinitionID,
	)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if prompt.Status != "validated" && prompt.Status != "published" {
		return PromptBinding{}, ErrPromptNotValidated
	}
	if prompt.Layer != "process_prompt" {
		return PromptBinding{}, unexpectedState("layer", prompt.Layer)
	}
	if input.ClientAccountID != "" && prompt.ClientAccountID != input.ClientAccountID {
		return PromptBinding{}, ErrInvalidInput
	}

	var processConfigID, platformPromptID string
	err = tx.QueryRow(ctx, `
		select c.id, platform.id
		from intelligence.process_definitions d
		join intelligence.process_config_versions c
		  on c.id = d.active_config_version_id and c.status = 'published'
		join intelligence.prompt_definitions pd
		  on pd.process_definition_id = d.id and pd.layer_kind = 'platform_guardrail'
		join intelligence.platform_prompt_versions platform
		  on platform.prompt_definition_id = pd.id and platform.status = 'published'
		where d.id = $1
		order by platform.version desc
		limit 1`,
		processDefinitionID,
	).Scan(&processConfigID, &platformPromptID)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	var agentVersionStatus string
	err = tx.QueryRow(ctx, `
		select version.status
		from intelligence.ai_agent_versions version
		join intelligence.ai_agents agent
		  on agent.account_id = version.account_id
		 and agent.id = version.agent_id
		where version.account_id = $1 and version.id = $2
		  and agent.client_account_id
		      is not distinct from nullif($3, '')::uuid`,
		accountID, input.AgentVersionID, prompt.ClientAccountID,
	).Scan(&agentVersionStatus)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if agentVersionStatus != "published" {
		return PromptBinding{}, ErrAgentNotPublished
	}

	if _, err = tx.Exec(ctx, `
		update intelligence.prompt_bindings
		set status = 'archived', revision = revision + 1
		where account_id = $1
		  and client_account_id is not distinct from nullif($2, '')::uuid
		  and process_definition_id = $3 and status = 'published'`,
		accountID, prompt.ClientAccountID, processDefinitionID,
	); err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		update intelligence.prompt_versions
		set status = 'published', published_by_user_id = nullif($3, '')::uuid,
		    updated_by_user_id = nullif($3, '')::uuid,
		    published_at = coalesce(published_at, now()), revision = revision + 1
		where account_id = $1 and id = $2`,
		accountID, promptVersionID, actorID,
	); err != nil {
		return PromptBinding{}, repositoryError(err)
	}

	var binding PromptBinding
	err = tx.QueryRow(ctx, `
		insert into intelligence.prompt_bindings (
		    account_id, client_account_id, process_definition_id,
		    process_config_version_id, process_key, platform_prompt_version_id,
		    process_prompt_version_id, agent_version_id, source_policy, tool_policy,
		    knowledge_policy, runtime_policy, status, published_by_user_id,
		    published_at
		)
		values (
		    $1, nullif($2, '')::uuid, $3, $4, $5, $6, $7, $8,
		    $9, $10, $11, $12, 'published', nullif($13, '')::uuid, now()
		)
		returning id, account_id, coalesce(client_account_id::text, ''),
		          process_key, process_prompt_version_id, agent_version_id,
		          status, revision, published_at`,
		accountID, prompt.ClientAccountID, processDefinitionID, processConfigID,
		prompt.ProcessKey, platformPromptID, promptVersionID, input.AgentVersionID,
		normalizedJSON(input.SourcePolicy, `[]`), normalizedJSON(input.ToolPolicy, `[]`),
		normalizedJSON(input.KnowledgePolicy, `[]`), normalizedJSON(input.RuntimePolicy, `{}`),
		actorID,
	).Scan(
		&binding.ID, &binding.AccountID, &binding.ClientAccountID,
		&binding.ProcessKey, &binding.ProcessPromptVersionID,
		&binding.AgentVersionID, &binding.Status, &binding.Revision,
		&binding.PublishedAt,
	)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, actor_user_id, event_type,
		    aggregate_type, aggregate_id, reason_code, metadata
		)
		values (
		    $1, nullif($2, '')::uuid, nullif($3, '')::uuid,
		    'prompt_binding_published', 'prompt_binding', $4,
		    'prompt_published',
		    jsonb_build_object(
		        'processKey', $5::text,
		        'promptVersionId', $6::text,
		        'agentVersionId', $7::text
		    )
		)`,
		accountID, binding.ClientAccountID, actorID, binding.ID,
		binding.ProcessKey, binding.ProcessPromptVersionID,
		binding.AgentVersionID,
	); err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PromptBinding{}, err
	}
	return binding, nil
}

func (s *PostgresRepository) RollbackPrompt(
	ctx context.Context,
	accountID, actorID, bindingID string,
	input RollbackPromptInput,
) (PromptBinding, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PromptBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current PromptBinding
	var processDefinitionID, processConfigID, platformPromptID string
	var sourcePolicy, toolPolicy, knowledgePolicy, runtimePolicy []byte
	err = tx.QueryRow(ctx, `
		select id, account_id, coalesce(client_account_id::text, ''), process_key,
		       process_prompt_version_id, agent_version_id, status, revision,
		       coalesce(published_at, created_at), process_definition_id,
		       process_config_version_id, platform_prompt_version_id,
		       source_policy, tool_policy, knowledge_policy, runtime_policy
		from intelligence.prompt_bindings
		where account_id = $1 and id = $2 and status = 'published'
		for update`,
		accountID, bindingID,
	).Scan(
		&current.ID, &current.AccountID, &current.ClientAccountID,
		&current.ProcessKey, &current.ProcessPromptVersionID,
		&current.AgentVersionID, &current.Status, &current.Revision,
		&current.PublishedAt, &processDefinitionID, &processConfigID,
		&platformPromptID, &sourcePolicy, &toolPolicy, &knowledgePolicy,
		&runtimePolicy,
	)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	var targetProcessID, targetProcessKey, targetClientID, targetStatus string
	err = tx.QueryRow(ctx, `
		select process_definition_id, process_key,
		       coalesce(client_account_id::text, ''), status
		from intelligence.prompt_versions
		where account_id = $1 and id = $2 and layer_kind = 'process_prompt'`,
		accountID, input.TargetPromptVersionID,
	).Scan(&targetProcessID, &targetProcessKey, &targetClientID, &targetStatus)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if targetProcessID != processDefinitionID || targetProcessKey != current.ProcessKey ||
		targetClientID != current.ClientAccountID || targetStatus != "published" {
		return PromptBinding{}, ErrInvalidInput
	}
	if _, err = tx.Exec(ctx, `
		update intelligence.prompt_bindings
		set status = 'archived', revision = revision + 1
		where account_id = $1 and id = $2`,
		accountID, bindingID,
	); err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	var rolled PromptBinding
	err = tx.QueryRow(ctx, `
		insert into intelligence.prompt_bindings (
		    account_id, client_account_id, process_definition_id,
		    process_config_version_id, process_key, platform_prompt_version_id,
		    process_prompt_version_id, agent_version_id, source_policy, tool_policy,
		    knowledge_policy, runtime_policy, status, based_on_binding_id,
		    published_by_user_id, published_at
		)
		values (
		    $1, nullif($2, '')::uuid, $3, $4, $5, $6, $7, $8,
		    $9, $10, $11, $12, 'published', $13, nullif($14, '')::uuid, now()
		)
		returning id, account_id, coalesce(client_account_id::text, ''),
		          process_key, process_prompt_version_id, agent_version_id,
		          status, revision, published_at`,
		accountID, current.ClientAccountID, processDefinitionID, processConfigID,
		current.ProcessKey, platformPromptID, input.TargetPromptVersionID,
		current.AgentVersionID, sourcePolicy, toolPolicy, knowledgePolicy,
		runtimePolicy, bindingID, actorID,
	).Scan(
		&rolled.ID, &rolled.AccountID, &rolled.ClientAccountID,
		&rolled.ProcessKey, &rolled.ProcessPromptVersionID,
		&rolled.AgentVersionID, &rolled.Status, &rolled.Revision,
		&rolled.PublishedAt,
	)
	if err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, actor_user_id, event_type,
		    aggregate_type, aggregate_id, reason_code, metadata
		)
		values (
		    $1, nullif($2, '')::uuid, nullif($3, '')::uuid,
		    'prompt_binding_rolled_back', 'prompt_binding', $4, $5,
		    jsonb_build_object(
		        'processKey', $6,
		        'previousBindingId', $7,
		        'targetPromptVersionId', $8
		    )
		)`,
		accountID, rolled.ClientAccountID, actorID, rolled.ID,
		input.ReasonCode, rolled.ProcessKey, current.ID,
		input.TargetPromptVersionID,
	); err != nil {
		return PromptBinding{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PromptBinding{}, err
	}
	return rolled, nil
}

func (s *PostgresRepository) ListModels(ctx context.Context, accountID string) ([]AIModel, error) {
	rows, err := s.pool.Query(ctx, `
		select id, provider, model, base_url,
		       case when is_enabled then 'enabled' else 'disabled' end,
		       jsonb_build_object('capabilities', capabilities), revision
		from intelligence.ai_models
		where account_id is null or account_id = $1
		order by account_id nulls first, provider, model`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AIModel, 0)
	for rows.Next() {
		var item AIModel
		var config []byte
		if err := rows.Scan(
			&item.ID, &item.Provider, &item.Model, &item.BaseURL,
			&item.Status, &config, &item.Revision,
		); err != nil {
			return nil, err
		}
		item.Config = config
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) UpsertModel(
	ctx context.Context,
	accountID, actorID string,
	model AIModel,
) (AIModel, error) {
	capabilities := json.RawMessage(`[]`)
	if len(model.Config) > 0 {
		var decoded struct {
			Capabilities json.RawMessage `json:"capabilities"`
		}
		if json.Unmarshal(model.Config, &decoded) == nil && len(decoded.Capabilities) > 0 {
			capabilities = decoded.Capabilities
		}
	}
	var enabled bool
	var rawCapabilities []byte
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.ai_models (
		    account_id, provider, model, base_url, capabilities, is_enabled,
		    revision, created_by_user_id, updated_by_user_id
		)
		values ($1, $2, $3, $4, $5, $6, 1, nullif($7, '')::uuid, nullif($7, '')::uuid)
		on conflict (account_id, provider, model)
		do update set base_url = excluded.base_url,
		              capabilities = excluded.capabilities,
		              is_enabled = excluded.is_enabled,
		              revision = intelligence.ai_models.revision + 1,
		              updated_by_user_id = excluded.updated_by_user_id,
		              updated_at = now()
		where $8 > 0 and intelligence.ai_models.revision = $8
		returning id, provider, model, base_url, is_enabled, capabilities, revision`,
		accountID, model.Provider, model.Model, model.BaseURL, capabilities,
		model.Status == "enabled", actorID, model.Revision,
	).Scan(
		&model.ID, &model.Provider, &model.Model, &model.BaseURL,
		&enabled, &rawCapabilities, &model.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIModel{}, ErrConflict
	}
	if enabled {
		model.Status = "enabled"
	} else {
		model.Status = "disabled"
	}
	config, _ := json.Marshal(map[string]json.RawMessage{"capabilities": rawCapabilities})
	model.Config = config
	return model, repositoryError(err)
}

func (s *PostgresRepository) ListCredentials(
	ctx context.Context,
	accountID string,
) ([]credentialRecord, error) {
	rows, err := s.pool.Query(ctx, `
		select id, provider, name, secret_ciphertext, secret_last4, status, updated_at
		from intelligence.ai_credentials
		where account_id = $1
		order by name`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]credentialRecord, 0)
	for rows.Next() {
		var item credentialRecord
		if err := rows.Scan(
			&item.ID, &item.Provider, &item.Label, &item.Ciphertext,
			&item.Last4, &item.Status, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) UpsertCredential(
	ctx context.Context,
	accountID, actorID string,
	input CredentialInput,
	ciphertext, last4 string,
) (credentialRecord, error) {
	var item credentialRecord
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.ai_credentials (
		    account_id, name, provider, secret_ciphertext, secret_last4,
		    status, created_by_user_id, updated_by_user_id
		)
		values (
		    $1, $2, $3, $4, $5, 'active',
		    nullif($6, '')::uuid, nullif($6, '')::uuid
		)
		on conflict (account_id, name)
		do update set provider = excluded.provider,
		              secret_ciphertext = excluded.secret_ciphertext,
		              secret_last4 = excluded.secret_last4,
		              status = 'active',
		              updated_by_user_id = excluded.updated_by_user_id,
		              updated_at = now()
		returning id, provider, name, secret_ciphertext, secret_last4,
		          status, updated_at`,
		accountID, input.Label, input.Provider, ciphertext, last4, actorID,
	).Scan(
		&item.ID, &item.Provider, &item.Label, &item.Ciphertext, &item.Last4,
		&item.Status, &item.UpdatedAt,
	)
	return item, repositoryError(err)
}

func (s *PostgresRepository) RevokeCredential(
	ctx context.Context,
	accountID, actorID, id string,
) error {
	tag, err := s.pool.Exec(ctx, `
		update intelligence.ai_credentials
		set status = 'revoked', secret_ciphertext = '', secret_last4 = '',
		    updated_by_user_id = nullif($3, '')::uuid, updated_at = now()
		where account_id = $1 and id = $2`,
		accountID, id, actorID,
	)
	if err != nil {
		return repositoryError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresRepository) ListAgents(
	ctx context.Context,
	accountID, clientAccountID string,
) ([]AIAgent, error) {
	rows, err := s.pool.Query(ctx, `
		select id, name, slug,
		       case when enabled then 'enabled' else 'disabled' end,
		       coalesce(active_version_id::text, ''), updated_at, revision
		from intelligence.ai_agents
		where account_id = $1
		  and client_account_id is not distinct from nullif($2, '')::uuid
		order by name`,
		accountID, clientAccountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AIAgent, 0)
	for rows.Next() {
		var item AIAgent
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Purpose, &item.Status,
			&item.ActiveVersionID, &item.UpdatedAt, &item.Revision,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) CreateAgent(
	ctx context.Context,
	accountID, actorID, clientAccountID, slug, name string,
) (AIAgent, error) {
	var item AIAgent
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.ai_agents (
		    account_id, client_account_id, slug, name, enabled,
		    created_by_user_id, updated_by_user_id
		)
		values (
		    $1, nullif($2, '')::uuid, $3, $4, false,
		    nullif($5, '')::uuid, nullif($5, '')::uuid
		)
		returning id, name, slug, 'disabled', coalesce(active_version_id::text, ''), updated_at, revision`,
		accountID, clientAccountID, slug, name, actorID,
	).Scan(
		&item.ID, &item.Name, &item.Purpose, &item.Status,
		&item.ActiveVersionID, &item.UpdatedAt, &item.Revision,
	)
	return item, repositoryError(err)
}

func (s *PostgresRepository) AgentClientScope(
	ctx context.Context,
	accountID, agentID string,
) (string, error) {
	var clientAccountID string
	err := s.pool.QueryRow(ctx, `
		select coalesce(client_account_id::text, '')
		from intelligence.ai_agents
		where account_id = $1 and id = $2`,
		accountID, agentID,
	).Scan(&clientAccountID)
	return clientAccountID, repositoryError(err)
}

func (s *PostgresRepository) AgentVersionClientScope(
	ctx context.Context,
	accountID, versionID string,
) (string, error) {
	var clientAccountID string
	err := s.pool.QueryRow(ctx, `
		select coalesce(a.client_account_id::text, '')
		from intelligence.ai_agent_versions v
		join intelligence.ai_agents a
		  on a.account_id = v.account_id and a.id = v.agent_id
		where v.account_id = $1 and v.id = $2`,
		accountID, versionID,
	).Scan(&clientAccountID)
	return clientAccountID, repositoryError(err)
}

func (s *PostgresRepository) UpdateAgent(
	ctx context.Context,
	accountID, actorID, id string,
	input AgentPatchInput,
) (AIAgent, error) {
	var item AIAgent
	tag, err := s.pool.Exec(ctx, `
		update intelligence.ai_agents
		set name = case when $3 = '' then name else $3 end,
		    enabled = coalesce($4, enabled),
		    revision = revision + 1,
		    updated_by_user_id = nullif($5, '')::uuid,
		    updated_at = now()
		where account_id = $1 and id = $2 and revision = $6
		  and (coalesce($4, enabled) = false or active_version_id is not null)`,
		accountID, id, input.Name, input.Enabled, actorID, input.ExpectedRevision,
	)
	if err != nil {
		return AIAgent{}, repositoryError(err)
	}
	if tag.RowsAffected() == 0 {
		return AIAgent{}, ErrConflict
	}
	err = s.pool.QueryRow(ctx, `
		select id, name, slug,
		       case when enabled then 'enabled' else 'disabled' end,
		       coalesce(active_version_id::text, ''), updated_at, revision
		from intelligence.ai_agents
		where account_id = $1 and id = $2`,
		accountID, id,
	).Scan(
		&item.ID, &item.Name, &item.Purpose, &item.Status,
		&item.ActiveVersionID, &item.UpdatedAt, &item.Revision,
	)
	return item, repositoryError(err)
}

func (s *PostgresRepository) CreateAgentVersion(
	ctx context.Context,
	accountID, actorID, agentID string,
	input AIAgentVersionInput,
) (AIAgentVersion, error) {
	var item AIAgentVersion
	var config []byte
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.ai_agent_versions (
		    account_id, agent_id, version, status, model_id, credential_id,
		    temperature, max_output_tokens, timeout_ms, prompt_override,
		    config, created_by_user_id
		)
		select
		    $1, a.id,
		    (select coalesce(max(v.version), 0) + 1
		     from intelligence.ai_agent_versions v
		     where v.account_id = $1 and v.agent_id = a.id),
		    'draft', $4, nullif($5, '')::uuid, $6, $7, $8, $9, $10,
		    nullif($3, '')::uuid
		from intelligence.ai_agents a
		where a.account_id = $1 and a.id = $2
		  and exists (
		      select 1 from intelligence.ai_models m
		      where m.id = $4 and (m.account_id is null or m.account_id = $1)
		  )
		  and (
		      $5 = '' or exists (
		          select 1 from intelligence.ai_credentials c
		          where c.account_id = $1 and c.id = nullif($5, '')::uuid and c.status = 'active'
		      )
		  )
		returning id, agent_id, version, status, model_id,
		          coalesce(credential_id::text, ''), temperature,
		          max_output_tokens, timeout_ms, prompt_override, config`,
		accountID, agentID, actorID, input.ModelID, input.CredentialID,
		input.Temperature, input.MaxOutputTokens, input.TimeoutMS,
		input.PromptOverride, normalizedJSON(input.Config, `{}`),
	).Scan(
		&item.ID, &item.AgentID, &item.Version, &item.Status, &item.ModelID,
		&item.CredentialID, &item.Temperature, &item.MaxOutputTokens,
		&item.TimeoutMS, &item.PromptOverride, &config,
	)
	item.Config = config
	return item, repositoryError(err)
}

func (s *PostgresRepository) PublishAgentVersion(
	ctx context.Context,
	accountID, actorID, versionID string,
) (AIAgentVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AIAgentVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var item AIAgentVersion
	var config []byte
	err = tx.QueryRow(ctx, `
		update intelligence.ai_agent_versions v
		set status = 'published', validated_by_user_id = nullif($3, '')::uuid,
		    published_by_user_id = nullif($3, '')::uuid,
		    validated_at = coalesce(validated_at, now()),
		    published_at = coalesce(published_at, now()),
		    revision = v.revision + 1
		from intelligence.ai_models m
		where v.account_id = $1 and v.id = $2
		  and v.status in ('draft', 'validated', 'published')
		  and m.id = v.model_id and m.is_enabled = true
		  and (
		      v.credential_id is null or exists (
		          select 1 from intelligence.ai_credentials c
		          where c.account_id = v.account_id and c.id = v.credential_id
		            and c.status = 'active'
		      )
		  )
		returning v.id, v.agent_id, v.version, v.status, v.model_id,
		          coalesce(v.credential_id::text, ''), v.temperature,
		          v.max_output_tokens, v.timeout_ms, v.prompt_override, v.config`,
		accountID, versionID, actorID,
	).Scan(
		&item.ID, &item.AgentID, &item.Version, &item.Status, &item.ModelID,
		&item.CredentialID, &item.Temperature, &item.MaxOutputTokens,
		&item.TimeoutMS, &item.PromptOverride, &config,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AIAgentVersion{}, ErrConflict
		}
		return AIAgentVersion{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		update intelligence.ai_agent_versions
		set status = 'archived', revision = revision + 1
		where account_id = $1 and agent_id = $2 and id <> $3 and status = 'published'`,
		accountID, item.AgentID, item.ID,
	); err != nil {
		return AIAgentVersion{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		update intelligence.ai_agents
		set active_version_id = $3, enabled = true, revision = revision + 1,
		    updated_by_user_id = nullif($4, '')::uuid, updated_at = now()
		where account_id = $1 and id = $2`,
		accountID, item.AgentID, item.ID, actorID,
	); err != nil {
		return AIAgentVersion{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AIAgentVersion{}, err
	}
	item.Config = config
	return item, nil
}

func (s *PostgresRepository) ResolveExecutionPlan(
	ctx context.Context,
	scope Scope,
	processKey string,
) (ExecutionPlan, error) {
	var plan ExecutionPlan
	var output, variables, sourcePolicy, toolPolicy, knowledgePolicy, runtimePolicy []byte
	err := s.pool.QueryRow(ctx, `
		select d.id, c.id, d.process_key, c.schema_version, c.output_schema,
		       c.allowed_variables, b.id, platform.id,
		       coalesce(agency.id::text, ''), coalesce(client.id::text, ''),
		       process.id, platform.content,
		       coalesce(agency.content, ''), coalesce(client.content, ''),
		       process.content, av.id, m.id, m.provider, m.model, m.base_url,
		       coalesce(cred.secret_ciphertext, ''), av.temperature,
		       av.max_output_tokens, av.timeout_ms, av.prompt_override,
		       b.source_policy, b.tool_policy, b.knowledge_policy, b.runtime_policy
		from intelligence.prompt_bindings b
		join intelligence.process_definitions d
		  on d.id = b.process_definition_id and d.process_key = $3
		     and d.status = 'registered'
		join intelligence.process_config_versions c
		  on c.id = b.process_config_version_id and c.status = 'published'
		     and d.active_config_version_id = c.id
		join intelligence.platform_prompt_versions platform
		  on platform.id = b.platform_prompt_version_id and platform.status = 'published'
		left join intelligence.prompt_versions agency
		  on agency.account_id = b.account_id and agency.id = b.agency_prompt_version_id
		left join intelligence.prompt_versions client
		  on client.account_id = b.account_id and client.id = b.client_prompt_version_id
		join intelligence.prompt_versions process
		  on process.account_id = b.account_id and process.id = b.process_prompt_version_id
		     and process.status = 'published'
		join intelligence.ai_agent_versions av
		  on av.account_id = b.account_id and av.id = b.agent_version_id
		     and av.status = 'published'
		join intelligence.ai_agents agent
		  on agent.account_id = av.account_id and agent.id = av.agent_id
		     and agent.client_account_id is not distinct from b.client_account_id
		     and agent.enabled = true and agent.active_version_id = av.id
		join intelligence.ai_models m
		  on m.id = av.model_id and m.is_enabled = true
		     and (m.account_id is null or m.account_id = b.account_id)
		left join intelligence.ai_credentials cred
		  on cred.account_id = av.account_id and cred.id = av.credential_id
		     and cred.status = 'active'
		where b.account_id = $1 and b.status = 'published'
		  and (b.client_account_id = $2 or b.client_account_id is null)
		order by (b.client_account_id = $2) desc, b.published_at desc
		limit 1`,
		scope.AccountID, scope.ClientAccountID, processKey,
	).Scan(
		&plan.ProcessDefinitionID, &plan.ProcessConfigVersionID, &plan.ProcessKey,
		&plan.SchemaVersion, &output, &variables, &plan.PromptBindingID,
		&plan.PlatformPromptVersionID, &plan.AgencyPromptVersionID,
		&plan.ClientPromptVersionID, &plan.ProcessPromptVersionID,
		&plan.PlatformPrompt, &plan.AgencyPrompt, &plan.ClientPrompt,
		&plan.ProcessPrompt, &plan.AgentVersionID, &plan.ModelID,
		&plan.Provider, &plan.Model, &plan.BaseURL,
		&plan.CredentialCiphertext, &plan.Temperature,
		&plan.MaxOutputTokens, &plan.TimeoutMS, &plan.PromptOverride,
		&sourcePolicy, &toolPolicy, &knowledgePolicy, &runtimePolicy,
	)
	plan.OutputSchema = output
	plan.AllowedVariables = decodeStrings(variables)
	plan.SourcePolicy = normalizedJSON(sourcePolicy, `[]`)
	plan.ToolPolicy = normalizedJSON(toolPolicy, `[]`)
	plan.KnowledgePolicy = normalizedJSON(knowledgePolicy, `[]`)
	plan.RuntimePolicy = normalizedJSON(runtimePolicy, `{}`)
	if errors.Is(err, pgx.ErrNoRows) {
		return ExecutionPlan{}, ErrPromptNotPublished
	}
	return plan, repositoryError(err)
}

func (s *PostgresRepository) ResolvePipelineVersion(
	ctx context.Context,
	pipelineKey string,
) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		select version.id
		from intelligence.pipeline_definitions definition
		join intelligence.pipeline_versions version
		  on version.id = definition.active_version_id
		     and version.pipeline_definition_id = definition.id
		     and version.status = 'published'
		where definition.pipeline_key = $1
		  and definition.status = 'registered'`,
		pipelineKey,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPromptNotPublished
	}
	return id, repositoryError(err)
}

func (s *PostgresRepository) FindRuntimeResult(
	ctx context.Context,
	scope Scope,
	requestID, processKey string,
) (RuntimeResult, error) {
	var result RuntimeResult
	var warnings []byte
	err := s.pool.QueryRow(ctx, `
		select run.id, run.status, run.output_ciphertext, run.warning_codes,
		       run.prompt_tokens, run.completion_tokens, run.total_tokens, run.latency_ms,
		       run.execution_mode, run.process_definition_id,
		       run.process_config_version_id, run.prompt_binding_id,
		       binding.platform_prompt_version_id,
		       coalesce(binding.agency_prompt_version_id::text, ''),
		       coalesce(binding.client_prompt_version_id::text, ''),
		       binding.process_prompt_version_id, run.agent_version_id,
		       run.model_id, coalesce(run.context_snapshot_id::text, ''),
		       run.output_schema_version
		from intelligence.runtime_runs run
		join intelligence.prompt_bindings binding
		  on binding.account_id = run.account_id
		     and binding.id = run.prompt_binding_id
		where run.account_id = $1 and run.client_account_id = $2
		  and run.request_id = $3 and run.process_key = $4`,
		scope.AccountID, scope.ClientAccountID, requestID, processKey,
	).Scan(
		&result.RunRef.RunID, &result.Status, &result.OutputCiphertext, &warnings,
		&result.Usage.PromptTokens, &result.Usage.CompletionTokens,
		&result.Usage.TotalTokens, &result.Usage.LatencyMs,
		&result.RunRef.ExecutionMode, &result.RunRef.ProcessDefinitionID,
		&result.RunRef.ProcessConfigVersionID, &result.RunRef.PromptBindingID,
		&result.RunRef.PlatformPromptVersionID, &result.RunRef.AgencyPromptVersionID,
		&result.RunRef.ClientPromptVersionID, &result.RunRef.ProcessPromptVersionID,
		&result.RunRef.AgentVersionID, &result.RunRef.ModelID,
		&result.RunRef.ContextSnapshotID, &result.RunRef.OutputSchemaVersion,
	)
	result.RunRef.ProcessKey = processKey
	result.RunRef.Status = result.Status
	result.WarningCodes = decodeStrings(warnings)
	return result, repositoryError(err)
}

func (s *PostgresRepository) StartRuntimeRun(
	ctx context.Context,
	input RuntimeRunInput,
) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.runtime_runs (
		    request_id, interaction_id, account_id, client_account_id,
		    subject_id, relationship_id, conversation_id, pipeline_definition_id,
		    pipeline_version_id, process_definition_id,
		    process_config_version_id, process_key, prompt_binding_id,
		    agent_version_id, model_id, context_snapshot_id,
		    output_schema_version, execution_mode, status, input_fingerprint, started_at
		)
		values (
		    $1, $2, $3, $4, nullif($5, '')::uuid, nullif($6, '')::uuid,
		    nullif($7, '')::uuid,
		    (select pipeline_definition_id
		       from intelligence.pipeline_versions
		      where id = $8),
		    $8, $9, $10, $11, $12, $13, $14,
		    nullif($15, '')::uuid, $16, $17, 'running', $18, now()
		)
		on conflict (account_id, client_account_id, request_id, process_key) do nothing
		returning id`,
		input.Request.RequestID, input.Request.InteractionID,
		input.Request.AccountID, input.Request.ClientAccountID,
		input.Request.SubjectID, input.Request.RelationshipID,
		input.Request.ConversationID, input.PipelineVersionID,
		input.Plan.ProcessDefinitionID,
		input.Plan.ProcessConfigVersionID, input.ProcessKey,
		input.Plan.PromptBindingID, input.Plan.AgentVersionID,
		input.Plan.ModelID, input.ContextID, input.Plan.SchemaVersion,
		input.ExecutionMode, input.InputHash,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			select id from intelligence.runtime_runs
			where account_id = $1 and client_account_id = $2
			  and request_id = $3 and process_key = $4`,
			input.Request.AccountID, input.Request.ClientAccountID,
			input.Request.RequestID, input.ProcessKey,
		).Scan(&id)
		return id, false, repositoryError(err)
	}
	return id, true, repositoryError(err)
}

func (s *PostgresRepository) CompleteRuntimeRun(
	ctx context.Context,
	input RuntimeRunCompletion,
) error {
	if input.WarningCodes == nil {
		input.WarningCodes = []string{}
	}
	warnings, _ := json.Marshal(input.WarningCodes)
	tag, err := s.pool.Exec(ctx, `
		update intelligence.runtime_runs
		set status = $3, output_ciphertext = $4, output_hash = $5,
		    warning_codes = $6, error_code = $7, prompt_tokens = $8,
		    completion_tokens = $9, total_tokens = $10, latency_ms = $11,
		    completed_at = now()
		where account_id = $1 and id = $2`,
		input.AccountID, input.RunID, input.Status, input.OutputCiphertext,
		input.OutputHash, warnings, input.ErrorCode, input.Usage.PromptTokens,
		input.Usage.CompletionTokens, input.Usage.TotalTokens,
		input.Usage.LatencyMs,
	)
	if err != nil {
		return repositoryError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresRepository) ListRuntimeRuns(
	ctx context.Context,
	scope Scope,
	limit int,
) ([]RuntimeRunView, error) {
	rows, err := s.pool.Query(ctx, `
		select id, request_id, client_account_id, process_key, prompt_binding_id,
		       agent_version_id, model_id, output_schema_version, status,
		       error_code, prompt_tokens, completion_tokens, total_tokens,
		       latency_ms, created_at, completed_at
		from intelligence.runtime_runs
		where account_id = $1 and client_account_id = $2
		order by created_at desc, id desc
		limit $3`,
		scope.AccountID, scope.ClientAccountID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RuntimeRunView, 0)
	for rows.Next() {
		var item RuntimeRunView
		if err := rows.Scan(
			&item.ID, &item.RequestID, &item.ClientAccountID, &item.ProcessKey,
			&item.PromptBindingID, &item.AgentVersionID, &item.ModelID,
			&item.OutputSchemaVersion, &item.Status, &item.ErrorCode,
			&item.Usage.PromptTokens, &item.Usage.CompletionTokens,
			&item.Usage.TotalTokens, &item.Usage.LatencyMs, &item.CreatedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var _ PromptRepository = (*PostgresRepository)(nil)
var _ RuntimeRepository = (*PostgresRepository)(nil)

func normalizeLayer(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func validProvider(value string) bool {
	return validMode(strings.TrimSpace(strings.ToLower(value)), "openai", "gemini", "glm")
}
