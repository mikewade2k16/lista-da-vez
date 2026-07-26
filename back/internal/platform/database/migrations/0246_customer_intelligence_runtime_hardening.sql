-- 0246_customer_intelligence_runtime_hardening.sql
--
-- Hardening aditivo do runtime Customer Intelligence:
-- - idempotencia inclui client_account_id;
-- - runs registram pipeline e modo shadow/active;
-- - somente os dois processos conversacionais com schemas fechados ficam ativos.
-- Nenhuma capability, binding, agente, provider ou writer e habilitado aqui.

alter table intelligence.source_ingestion_runs
    drop constraint if exists source_ingestion_runs_account_id_idempotency_key_key;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_source_ingestion_runs_client_idempotency_key'
          and conrelid = 'intelligence.source_ingestion_runs'::regclass
    ) then
        alter table intelligence.source_ingestion_runs
            add constraint intelligence_source_ingestion_runs_client_idempotency_key
            unique (account_id, client_account_id, idempotency_key);
    end if;
end $$;

alter table intelligence.runtime_runs
    add column if not exists pipeline_definition_id uuid,
    add column if not exists pipeline_version_id uuid,
    add column if not exists execution_mode text not null default 'active';

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_runtime_runs_execution_mode_ck'
          and conrelid = 'intelligence.runtime_runs'::regclass
    ) then
        alter table intelligence.runtime_runs
            add constraint intelligence_runtime_runs_execution_mode_ck
            check (execution_mode in ('active', 'shadow'));
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_runtime_runs_pipeline_definition_fk'
          and conrelid = 'intelligence.runtime_runs'::regclass
    ) then
        alter table intelligence.runtime_runs
            add constraint intelligence_runtime_runs_pipeline_definition_fk
            foreign key (pipeline_definition_id)
            references intelligence.pipeline_definitions(id)
            on delete restrict;
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_runtime_runs_pipeline_version_fk'
          and conrelid = 'intelligence.runtime_runs'::regclass
    ) then
        alter table intelligence.runtime_runs
            add constraint intelligence_runtime_runs_pipeline_version_fk
            foreign key (pipeline_version_id, pipeline_definition_id)
            references intelligence.pipeline_versions(id, pipeline_definition_id)
            on delete restrict;
    end if;
end $$;

alter table intelligence.runtime_runs
    drop constraint if exists runtime_runs_account_id_request_id_process_key_key;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_runtime_runs_client_request_process_key'
          and conrelid = 'intelligence.runtime_runs'::regclass
    ) then
        alter table intelligence.runtime_runs
            add constraint intelligence_runtime_runs_client_request_process_key
            unique (account_id, client_account_id, request_id, process_key);
    end if;
end $$;

create index if not exists intelligence_runtime_runs_pipeline_idx
    on intelligence.runtime_runs (
        account_id,
        client_account_id,
        pipeline_version_id,
        execution_mode,
        created_at desc
    );

with conversational_processes(process_key, input_schema, output_schema, schema_version) as (
    values
    (
        'conversation.triage'::text,
        '{
          "type":"object",
          "required":["context","input","locale","purpose","asOf","operationalState","routingCatalog","channelCapabilities"],
          "properties":{
            "context":{"type":"object"},
            "input":{"type":"object"},
            "locale":{"type":"string","maxLength":32},
            "purpose":{"type":"string","minLength":1,"maxLength":120},
            "asOf":{"type":"string","minLength":1,"maxLength":64},
            "operationalState":{"type":"object"},
            "routingCatalog":{"type":"object"},
            "channelCapabilities":{"type":"object"}
          },
          "additionalProperties":false
        }'::jsonb,
        '{
          "type":"object",
          "required":["intent","categories","leadStage","needsHuman","reasonCode","confidence","extractedClaims","departmentId","queueId","closure"],
          "properties":{
            "intent":{"type":"string","maxLength":160},
            "categories":{"type":"array","maxItems":20,"items":{"type":"string","maxLength":120}},
            "leadStage":{"type":"string","maxLength":120},
            "needsHuman":{"type":"boolean"},
            "reasonCode":{"type":"string","maxLength":160},
            "confidence":{"type":"number","minimum":0,"maximum":1},
            "extractedClaims":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["factKey","valueType","value","confidence","evidenceObservationIds"],
                "properties":{
                  "factKey":{"type":"string","minLength":1,"maxLength":160},
                  "valueType":{"type":"string","enum":["string","integer","decimal","boolean","date","timestamp","enum","string_list","object_closed"]},
                  "value":{"type":["string","integer","number","boolean","array","object","null"]},
                  "confidence":{"type":"number","minimum":0,"maximum":1},
                  "evidenceObservationIds":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":64}},
                  "validFrom":{"type":["string","null"],"maxLength":64},
                  "validUntil":{"type":["string","null"],"maxLength":64}
                },
                "additionalProperties":false
              }
            },
            "departmentId":{"type":["string","null"],"maxLength":64},
            "queueId":{"type":["string","null"],"maxLength":64},
            "closure":{
              "type":["object","null"],
              "properties":{
                "requested":{"type":"boolean"},
                "reasonCode":{"type":"string","maxLength":160},
                "reason":{"type":"string","maxLength":500},
                "confidence":{"type":"number","minimum":0,"maximum":1},
                "humanRequested":{"type":"boolean"},
                "sensitiveTopic":{"type":"boolean"}
              },
              "required":["requested","confidence","humanRequested","sensitiveTopic"],
              "additionalProperties":false
            }
          },
          "additionalProperties":false
        }'::jsonb,
        'conversation.triage.result.v2'::text
    ),
    (
        'conversation.reply'::text,
        '{
          "type":"object",
          "required":["context","input","locale","purpose","asOf","operationalState","routingCatalog","channelCapabilities"],
          "properties":{
            "context":{"type":"object"},
            "input":{"type":"object"},
            "locale":{"type":"string","maxLength":32},
            "purpose":{"type":"string","minLength":1,"maxLength":120},
            "asOf":{"type":"string","minLength":1,"maxLength":64},
            "operationalState":{"type":"object"},
            "routingCatalog":{"type":"object"},
            "channelCapabilities":{"type":"object"}
          },
          "additionalProperties":false
        }'::jsonb,
        '{
          "type":"object",
          "required":["replyDraft","confidence","warnings","closure"],
          "properties":{
            "replyDraft":{"type":["string","null"],"maxLength":16000},
            "confidence":{"type":"number","minimum":0,"maximum":1},
            "warnings":{"type":"array","maxItems":20,"items":{"type":"string","maxLength":160}},
            "closure":{
              "type":["object","null"],
              "properties":{
                "requested":{"type":"boolean"},
                "reasonCode":{"type":"string","maxLength":160},
                "reason":{"type":"string","maxLength":500},
                "confidence":{"type":"number","minimum":0,"maximum":1},
                "humanRequested":{"type":"boolean"},
                "sensitiveTopic":{"type":"boolean"}
              },
              "required":["requested","confidence","humanRequested","sensitiveTopic"],
              "additionalProperties":false
            }
          },
          "additionalProperties":false
        }'::jsonb,
        'conversation.reply.result.v2'::text
    )
)
insert into intelligence.process_config_versions (
    process_definition_id,
    version,
    status,
    input_schema,
    output_schema,
    schema_version,
    allowed_variables,
    failure_mode,
    max_input_tokens,
    max_output_tokens,
    timeout_ms,
    published_at
)
select
    definition.id,
    2,
    'published',
    process.input_schema,
    process.output_schema,
    process.schema_version,
    '["context","input","locale","purpose","asOf","operationalState","routingCatalog","channelCapabilities"]'::jsonb,
    case when process.process_key = 'conversation.triage'
         then 'handoff'
         else 'no_effect'
    end,
    8000,
    1200,
    60000,
    now()
from conversational_processes process
join intelligence.process_definitions definition
  on definition.process_key = process.process_key
on conflict (process_definition_id, version) do nothing;

update intelligence.process_config_versions config
set status = 'archived'
from intelligence.process_definitions definition
where config.process_definition_id = definition.id
  and definition.process_key in ('conversation.triage', 'conversation.reply')
  and config.version < 2
  and config.status = 'published';

update intelligence.process_definitions definition
set status = 'registered',
    active_config_version_id = config.id,
    updated_at = now()
from intelligence.process_config_versions config
where config.process_definition_id = definition.id
  and config.version = 2
  and config.status = 'published'
  and definition.process_key in ('conversation.triage', 'conversation.reply');

update intelligence.process_definitions
set status = 'deprecated',
    active_config_version_id = null,
    updated_at = now()
where process_key not in ('conversation.triage', 'conversation.reply');

update intelligence.process_config_versions config
set status = 'archived'
from intelligence.process_definitions definition
where config.process_definition_id = definition.id
  and definition.process_key not in ('conversation.triage', 'conversation.reply')
  and config.status = 'published';
