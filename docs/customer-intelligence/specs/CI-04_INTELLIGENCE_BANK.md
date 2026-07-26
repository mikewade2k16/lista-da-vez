# CI-04 — Intelligence Bank

- **Status:** READY — implementação local autorizada
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** módulo `customer_intelligence`
- **Dependências:** CI-00, CI-02 e contratos estáveis de CI-03
- **Bloqueia:** CI-05, CI-06, CI-08, CI-09 e CI-10
- **Governança:** [../GOVERNANCA.md](../GOVERNANCA.md), versão vigente
- **Blueprint:** [../SPECS_GERAIS.md](../SPECS_GERAIS.md), versão vigente

> Esta spec descreve contratos e pacotes futuros. Ela não autoriza código, migration, backfill,
> workflow, deploy ou exclusão. Todos os nomes de schema/módulo permanecem propostos até CI-00
> validar as decisões `CI-DEC-*`.

## 1. Resultado único e verificável

Criar uma fonte autoritativa, versionada e tenant-safe para:

1. observações imutáveis e auditáveis vindas de fontes registradas;
2. claims candidatos produzidos por fonte, regra, humano ou LLM;
3. fatos resolvidos sem apagar divergências;
4. sínteses que indiquem exatamente seus insumos;
5. snapshots de contexto minimizados e temporários;
6. registro de prompts por processo, com versões, bindings, testes, avaliações e rollouts;
7. projeções de recomendação e auditoria consumidas pelas fases posteriores.

O resultado é considerado demonstrado quando uma mesma evidência pode ser reprocessada sem
duplicação, dois claims conflitantes permanecem visíveis, uma correção manual verificada vence
pela policy aplicável e a síntese resultante aponta seus fatos/observações de origem.

## 2. Decisões já tomadas

- PostgreSQL é a fonte de verdade.
- O dado bruto autoritativo permanece no módulo de origem.
- O banco de inteligência guarda referência estável, hash e somente snapshot allowlisted.
- `account_id` físico continua canônico; `owner_account_id` é alias de domínio até CI-00 decidir.
- Fatos, consentimentos, sínteses e recomendações são escopados por `relationship_id`.
- Observação, claim, fato, síntese e contexto são entidades diferentes.
- Claim ou saída de LLM é input não confiável.
- Fato inferido por LLM não vence correção manual verificada nem fonte autoritativa aplicável.
- Conflito é preservado; resolver não apaga evidência anterior.
- Prompt publicado é imutável.
- Cada `process_key` possui prompt, variáveis, schema, sources/tools e rollout próprios.
- O painel edita comportamento seguro, mas não altera tenant, RBAC, consentimento, FSM, schema
  obrigatório, allowlist, outbox ou sender.
- `messaging.contact_intelligence` é compatibilidade transitória, nunca segunda verdade permanente.
- CPF, telefone, e-mail ou outro identificador enumerável não usam hash simples.

## 3. Decisões abertas e bloqueios

| Decisão | Default DRAFT desta spec | Efeito se divergir |
|---|---|---|
| schema físico | `intelligence` | renomeia todas as tabelas e stores antes de `READY` |
| módulo Go | `customerintelligence` | ajusta allowlists e imports |
| owner de `subject/relationship` | Customer Data | altera validação/FKs, não o modelo de evidência |
| FK para Customer Data | somente se CI-03 expuser contrato composto estável | sem FK, o service valida pela porta e mantém IDs escopados |
| retenção por categoria | policy obrigatória ainda a aprovar | bloqueia produção com PII |
| HMAC/crypto-shredding | provider de chave rotacionável a definir | bloqueia identificadores e snapshots reais |
| legal hold/backups | policy jurídica pendente | bloqueia exclusão/anonimização |
| catálogo final de `process_key` | catálogo inicial da governança | impede publicar processo não catalogado |
| critérios mínimos de eval | 100% segurança/schema; funcional configurável | bloqueia publish/canary até CI-06 congelar |

Enquanto esta spec e suas decisões estiverem `DRAFT`, nenhum pacote pode produzir DDL, service,
backfill ou outra implementação. A execução só começa depois de validação explícita do owner e de o
pacote atômico aplicável passar para `READY`; autorização genérica ou branch isolada não substitui
esse gate.

## 4. Estado atual medido no disco

| Evidência atual | Limite relevante |
|---|---|
| `0236_messaging_contact_intelligence.sql` | snapshot 1:1 por contato com `summary`, `facts` e `preferences` JSON; não há proveniência por fato |
| `store_ai_runtime.go:CommitAITriageWithIntelligence` | conversa e memória são atualizadas sob o mesmo lease `state + ai_generation` |
| `contact_intelligence.go` | sanitiza até 40 chaves escalares, mas não representa fonte, conflito, validade ou verificação |
| `0206_messaging_ai_agents.sql` | agentes, versões e runs pertencem hoje a `messaging.*` |
| `0216_messaging_ai_dispatches.sql` | dispatch operacional e lease pertencem corretamente ao Omnichannel |
| `0222_messaging_ai_tools_knowledge.sql` | tools e knowledge já têm bindings, auditoria e FKs tenant-safe no schema legado |
| `0234_messaging_ai_credentials_and_roles.sql` | credenciais nomeadas são cifradas e write-only |
| `ai_prompt.go` | prompt atual combina triagem e resposta em camadas do agente |
| `service_triage.go` | contexto, prompt, chamada LLM e commit inteligente estão no Omnichannel |
| `platform/events` | bus atual é síncrono e in-memory; não serve como entrega crítica |
| `web/app/components/omnichannel/config/ConfigAiAgentVersions.vue` | UI real de versões, publish, rollback e seleção ativa |

Não existe no disco um schema `intelligence.*`, Prompt Registry por `process_key`, evidência
atômica, policy de autoridade versionada ou contexto criptografado com TTL.

## 5. Fronteira de ownership

### 5.1 O Intelligence Bank possui

- catálogo de tipos de fato e policies de autoridade;
- configurações e referências de fontes;
- observações sanitizadas;
- claims e seus insumos;
- projeções versionadas de fatos;
- sínteses e suas evidências;
- snapshots de contexto;
- recomendações derivadas, sem executar a ação;
- Prompt Registry;
- runs de ingestão e auditoria de inteligência.

### 5.2 O Intelligence Bank não possui

- mensagem, mídia binária, webhook, FSM, fila, handoff, outbox ou envio;
- cadastro bruto de ERP, Calendário, Site ou BI;
- subject, identidade ou consentimento determinístico autoritativo;
- segredo cru de fonte ou provider;
- decisão final de merge;
- payload bruto irrestrito;
- estado autoritativo de n8n.

## 6. Modelo de dados proposto

### 6.1 Convenções comuns

Todas as tabelas tenant-scoped:

- usam UUID como PK;
- possuem `account_id uuid not null references core.accounts(id)`;
- possuem unique composto `(account_id, id)` quando são alvo de FK composta;
- repetem `account_id` em toda FK tenant-safe;
- usam `timestamptz`;
- usam enums fechados por `CHECK`;
- têm `created_at`; entidades mutáveis têm `updated_at`;
- nunca confiam em `client_account_id`, `subject_id` ou `relationship_id` vindos de body sem
  validação no service;
- não armazenam segredo, prompt compilado em claro ou PII bruta desnecessária.

`client_account_id`, `subject_id` e `relationship_id` são obrigatórios conforme a granularidade:

| Granularidade | `client_account_id` | `subject_id` | `relationship_id` |
|---|---:|---:|---:|
| business context do cliente | sim | não | não |
| observação ainda não identificada | sim | não | não |
| observação de pessoa identificada | sim | sim | sim |
| fato/síntese/recomendação individual | sim | sim | sim |
| portfólio agregado | não no registro raiz; escopos em tabela própria | não | não |

Não se cria FK cross-module antes de CI-03 expor o contrato. A ausência de FK não autoriza aceitar
IDs livres: Customer Data valida o escopo por porta pequena e o repository repete `account_id`.

### 6.2 `intelligence.fact_definitions`

Catálogo estável de fatos possíveis; prompt pode sugerir um valor, mas não inventar uma chave.

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id` | UUID obrigatório |
| `fact_key` | texto normalizado, 1..160, único por conta |
| `catalog_status` | `registered`, `deprecated` |
| `active_version_id` | versão published efetiva, nullable durante criação |
| `revision` | bigint para compare-and-swap do ponteiro |
| `created_by_user_id`, `updated_by_user_id` | UUID nullable |
| `created_at`, `updated_at`, `deprecated_at` | timestamps |

`fact_key` nunca muda nem é reutilizada com outro significado. Tipo, schema, sensibilidade e
demais semântica ficam em versão imutável:

`intelligence.fact_definition_versions`:

| Coluna | Tipo/regra |
|---|---|
| `id`, `account_id`, `fact_definition_id` | PK/FK composta |
| `version` | inteiro positivo |
| `status` | `draft`, `validated`, `published`, `archived` |
| `label` | texto 1..200 |
| `value_type` | `string`, `integer`, `decimal`, `boolean`, `date`, `timestamp`, `enum`, `string_list`, `object_closed` |
| `value_schema` | JSON Schema fechado e limitado |
| `sensitivity` | `public`, `internal`, `personal`, `sensitive`, `restricted` |
| `relationship_scoped` | `true` no MVP |
| `context_allowed` | se pode entrar em contexto LLM |
| `cross_client_allowed` | `false` por default |
| `manual_verification_allowed` | boolean |
| `revision` | bigint somente enquanto draft |
| `based_on_version_id` | lineage nullable |
| autores e timestamps | criação/validação/publicação |

Índices:

- unique `(account_id, fact_key)`;
- unique `(account_id, fact_definition_id, version)`;
- `(account_id, catalog_status, fact_key)`;
- `(account_id, sensitivity, status)` na tabela de versões.

Publicar valida compatibilidade e atualiza `fact_definitions.active_version_id` por CAS na mesma
transação. Versão published não recebe `UPDATE`. Mudança incompatível de tipo/schema exige nova
`fact_key` ou migration/backfill explícitos; não reinterpreta claim/fato histórico.

### 6.3 `intelligence.authority_policy_versions`

Policy estruturada e imutável de resolução; não é prompt.

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id` | UUID obrigatório |
| `fact_definition_id` | FK composta |
| `fact_definition_version_id` | semântica validada pela policy |
| `version` | inteiro positivo |
| `status` | `draft`, `published`, `archived` |
| `rules` | array JSON fechado: source/method, prioridade, confiança, frescor, validade e verificação |
| `conflict_strategy` | `keep_current`, `highest_authority`, `newest_valid`, `manual_review` |
| `published_at`, `published_by_user_id` | nullable |
| `created_at` | timestamp |

Constraints/índices:

- unique `(account_id, fact_definition_id, version)`;
- versão publicada não recebe `UPDATE`.

`intelligence.authority_policy_bindings` resolve a versão efetiva sem ambiguidade:

| Coluna | Tipo/regra |
|---|---|
| `id`, `account_id`, `client_account_id` | escopo; client nullable para default da agência |
| `fact_definition_id`, `fact_definition_version_id` | definição/semântica |
| `authority_policy_version_id` | versão published |
| `status` | `draft`, `published`, `archived` |
| `revision`, autores e timestamps | CAS/auditoria |

Unique `(account_id, client_account_id, fact_definition_id)` com tratamento explícito de `NULL`.
Resolver tenta client exato e depois default da account; nunca outra account. Publish/rollback faz
CAS do binding para uma versão published e não edita policies/runs/fatos históricos.

Cada regra contém somente chaves registradas:

```json
{
  "sourceKey": "erp",
  "extractionMethod": "source_direct",
  "priority": 900,
  "minConfidence": 1,
  "maxAgeSeconds": null,
  "requiresVerifiedSource": true,
  "validityMode": "source"
}
```

### 6.4 `intelligence.source_configs`

A estrutura nasce em CI-04; registry, adapters e semântica operacional são CI-05.

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id` | UUID obrigatório |
| `client_account_id` | UUID obrigatório |
| `source_key` | texto 1..120, deve existir no registry Go |
| `connection_key` | slug estável 1..120; permite mais de uma conexão registrada da mesma source |
| `status` | `draft`, `enabled`, `disabled`, `error` |
| `mode` | `event`, `scheduled`, `on_demand`, `manual` |
| `purpose_key` | texto allowlisted |
| `field_allowlist` | array JSON de chaves do descriptor |
| `category_allowlist` | array JSON fechado |
| `priority` | inteiro 0..1000 |
| `required` | boolean |
| `freshness_sla_seconds` | inteiro positivo nullable |
| `historical_use_mode` | `include`, `exclude`, `policy` |
| `retention_policy_key` | texto registrado |
| `retention_policy_version_id` | versão published efetiva |
| `secret_ref` | referência opaca; nunca segredo |
| `config` | objeto fechado conforme schema do descriptor |
| `revision` | bigint |
| `created_by_user_id`, `updated_by_user_id` | UUID nullable |
| `created_at`, `updated_at` | timestamps |

Constraints/índices:

- unique `(account_id, client_account_id, source_key, connection_key)`;
- `(account_id, status, source_key)`;
- `(account_id, client_account_id, status)`;
- JSON limitado em quantidade de campos e bytes pelo service.

### 6.4.1 `intelligence.retention_policy_versions`

Retenção é configuração estruturada e versionada, não prompt:

| Coluna | Tipo/regra |
|---|---|
| `id`, `account_id` | UUID/FK |
| `policy_key` | chave estável por conta |
| `version` | inteiro positivo |
| `status` | `draft`, `published`, `archived` |
| `category_rules` | objeto fechado por data class/source/fact type |
| `snapshot_ttl_seconds` | bound obrigatório |
| `on_expiry` | `tombstone`, `anonymize`, `crypto_shred`, `review` |
| `legal_hold_behavior` | chave de policy aprovada |
| `block_reingestion` | boolean/policy por categoria |
| `published_by_user_id`, `published_at`, `created_at` | auditoria |

Unique `(account_id, policy_key, version)`. Publicada é imutável. `source_configs` funciona como
binding e referencia simultaneamente key + versão published; publicar uma nova policy não altera
fontes existentes. Rebind/rollback atualiza `retention_policy_version_id` por CAS e auditoria, com
preview de impacto. Nenhuma policy pode prometer expurgo de backup ou dado em legal hold sem o
fluxo jurídico aprovado.

### 6.5 `intelligence.ingestion_runs`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | escopo obrigatório |
| `source_config_id` | FK composta |
| `source_config_revision` | configuração efetiva congelada |
| `retention_policy_version_id` | versão efetiva copiada no início |
| `trigger` | `event`, `schedule`, `manual`, `replay`, `backfill` |
| `status` | `queued`, `processing`, `completed`, `partial`, `failed`, `dead_letter`, `cancelled` |
| `cursor_before`, `cursor_after` | texto opaco limitado |
| `idempotency_key` | texto obrigatório |
| `records_seen`, `records_created`, `records_duplicated`, `records_rejected` | inteiros não negativos |
| `error_code`, `error_detail_masked` | texto seguro e limitado |
| `attempts` | inteiro não negativo |
| `locked_at`, `started_at`, `completed_at` | nullable |
| `created_at`, `updated_at` | timestamps |

Índices:

- unique `(account_id, source_config_id, idempotency_key)`;
- claim `(status, created_at, id)` para jobs pendentes;
- histórico `(account_id, source_config_id, created_at desc, id desc)`.

### 6.6 `intelligence.source_observations`

Observação é append-only quanto ao conteúdo. Apenas lifecycle legal pode tombstonar/inutilizar.

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | obrigatórios |
| `subject_id`, `relationship_id` | nullable apenas enquanto o match está pendente |
| `source_config_id`, `source_key` | origem registrada |
| `source_entity_type` | tipo allowlisted pelo descriptor |
| `source_entity_id` | referência opaca do owner; nunca CPF/telefone/e-mail cru |
| `source_version` | versão/cursor/checksum da origem |
| `idempotency_key` | chave canônica obrigatória |
| `occurred_at` | instante do fato na origem |
| `observed_at` | instante em que o Omni observou |
| `content_hash` | HMAC/fingerprint versionado |
| `hash_key_version` | versão da chave |
| `snapshot_schema_version` | contrato do snapshot |
| `snapshot_sanitized` | JSON fechado somente para `public/internal` |
| `snapshot_ciphertext`, `cipher_key_version` | conteúdo `personal/sensitive/restricted` cifrado |
| `sensitivity` | classificação máxima do conteúdo |
| `purpose_key` | finalidade autorizada |
| `valid_from`, `valid_until` | nullable |
| `expires_at` | retenção |
| `ingestion_run_id` | FK composta nullable |
| `retention_policy_version_id` | versão usada para `expires_at`/lifecycle |
| `tombstoned_at`, `tombstone_reason` | lifecycle legal |
| `superseded_by_observation_id` | nova versão da mesma origem |
| `created_at` | timestamp |

Constraints/índices:

- unique `(account_id, source_config_id, idempotency_key)`;
- lookup de origem `(account_id, source_key, source_entity_type, source_entity_id, occurred_at desc)`;
- timeline `(account_id, relationship_id, occurred_at desc, id desc)`;
- retenção `(account_id, expires_at)` parcial;
- unresolved `(account_id, client_account_id, created_at)` onde `relationship_id is null`;
- exatamente uma forma de snapshot é preenchida conforme sensibilidade;
- snapshot decifrado deve ser objeto e respeitar teto de bytes.

Retry com payload igual retorna a observação existente. Mesma entidade com `source_version` ou hash
novo cria observação nova e aponta supersede; nunca reescreve o snapshot anterior.

### 6.7 `intelligence.claims` e `intelligence.claim_evidence`

`claims`:

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id`, `subject_id`, `relationship_id` | obrigatórios |
| `fact_definition_id`, `fact_definition_version_id`, `fact_key` | chave e semântica congeladas |
| `value_type` | deve coincidir com a definição |
| `value_normalized` | JSON validado somente para `public/internal` |
| `value_ciphertext`, `cipher_key_version` | valor `personal/sensitive/restricted` cifrado |
| `value_fingerprint` | HMAC quando necessário para comparação |
| `extraction_method` | `source_direct`, `rule`, `manual`, `llm` |
| `extractor_key`, `extractor_version` | regra/processo/modelo versionado |
| `prompt_binding_id`, `runtime_run_id` | nullable; obrigatórios quando método `llm` |
| `confidence` | numeric 0..1 |
| `verification_state` | `unverified`, `verified`, `rejected`, `contested` |
| `valid_from`, `valid_until` | nullable |
| `sensitivity` | classificação |
| `status` | `candidate`, `accepted`, `superseded`, `invalidated`, `rejected` |
| `superseded_by_claim_id` | nullable |
| `created_by_user_id` | preenchido para manual |
| `created_at`, `updated_at` | timestamps |

`claim_evidence`:

| Coluna | Tipo/regra |
|---|---|
| `account_id`, `claim_id`, `observation_id` | PK/FKs compostas |
| `role` | `supports`, `contradicts`, `context` |
| `created_at` | timestamp |

Índices principais:

- `(account_id, relationship_id, fact_key, status, created_at desc)`;
- `(account_id, runtime_run_id)` parcial;
- `(account_id, observation_id)` em `claim_evidence`.

Claim manual sem observação cria primeiro uma observação `manual.offline` auditável; não existe
claim sem proveniência. Valor em claro e ciphertext são mutuamente exclusivos; API autorizada
decifra somente no service e nunca entrega ciphertext.

### 6.8 `intelligence.facts` e `intelligence.fact_evidence`

`facts` é a projeção resolvida e versionada, não um segundo log de claims.

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id`, `subject_id`, `relationship_id` | obrigatórios |
| `fact_definition_id`, `fact_definition_version_id`, `fact_key` | chave e semântica congeladas |
| `version` | inteiro positivo |
| `value_type`, `value_resolved` | valor validado somente para `public/internal` |
| `value_ciphertext`, `cipher_key_version` | projeção `personal/sensitive/restricted` cifrada |
| `winning_claim_id` | claim que sustenta o valor |
| `authority_policy_version_id` | policy usada |
| `confidence` | numeric 0..1 |
| `resolution_state` | `resolved`, `verified`, `contested`, `invalidated`, `superseded` |
| `resolution_reason_code` | código fechado |
| `valid_from`, `valid_until` | nullable |
| `effective_at` | quando passou a valer |
| `superseded_by_fact_id` | nullable |
| `resolved_by_user_id` | nullable |
| `created_at` | timestamp |

Constraints/índices:

- unique `(account_id, relationship_id, fact_definition_id, version)`;
- unique parcial de uma projeção corrente por
  `(account_id, relationship_id, fact_definition_id)` onde estado é
  `resolved`, `verified` ou `contested`;
- timeline `(account_id, relationship_id, effective_at desc, id desc)`;
- fatos ativos `(account_id, client_account_id, fact_key, effective_at desc)`.

`fact_evidence`:

| Coluna | Tipo/regra |
|---|---|
| `account_id`, `fact_id`, `observation_id` | PK/FKs compostas |
| `claim_id` | FK composta nullable |
| `role` | `winning`, `supporting`, `conflicting` |
| `created_at` | timestamp |

Resolver novamente cria uma nova versão do fato, marca a anterior como `superseded` na mesma
transação e preserva todos os claims/evidências. A mesma exclusividade clear/ciphertext dos claims
vale para a projeção resolvida.

### 6.9 `intelligence.summary_versions` e `intelligence.summary_evidence`

`summary_versions`:

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id`, `subject_id`, `relationship_id` | obrigatórios |
| `summary_type` | `relationship_profile`, `handoff`, `conversation`, `portfolio_aggregate` |
| `version` | inteiro positivo |
| `status` | `draft`, `published`, `superseded`, `invalidated` |
| `content_ciphertext`, `sections_ciphertext` | síntese e seções cifradas |
| `content_hash`, `cipher_key_version` | integridade e rotação |
| `input_fingerprint` | hash determinístico dos insumos ordenados |
| `as_of` | corte temporal |
| `prompt_binding_id`, `runtime_run_id` | referências da geração |
| `confidence` | nullable 0..1 |
| `expires_at` | nullable |
| `superseded_by_summary_id` | nullable |
| `created_at`, `published_at` | timestamps |

`summary_evidence`:

| Coluna | Tipo/regra |
|---|---|
| `account_id`, `summary_id` | escopo/FK |
| `observation_id` | nullable |
| `fact_id` | nullable |
| `ordinal` | ordem usada no contexto |
| `role` | `source`, `supporting`, `conflicting`, `omitted_after_budget` |

Constraint exige exatamente um entre `observation_id` e `fact_id`. O mesmo fingerprint e binding
retornam a síntese existente, salvo pedido explícito de reavaliação.

### 6.9.1 Policies de recomendação

`intelligence.recommendation_policy_definitions` é catálogo estável platform-wide:

| Coluna | Tipo/regra |
|---|---|
| `id`, `policy_key` | UUID + chave estável |
| `recommendation_type` | `follow_up`, `offer`, `important_date`, `next_action` |
| `rules_schema_version` | schema fechado registrado em Go |
| `catalog_status`, timestamps | lifecycle |

`intelligence.recommendation_policy_versions` guarda configuração administrativa:

| Coluna | Tipo/regra |
|---|---|
| `id`, `account_id`, `client_account_id` | escopo; client nullable para default da agência |
| `policy_definition_id`, `version` | definição/versão |
| `status` | `draft`, `validated`, `published`, `archived` |
| `rules` | validade, cadência, thresholds, ranking, revisão e catálogo em objeto fechado |
| `rules_schema_version`, `checksum` | validação/reprodução |
| `based_on_version_id`, `revision` | lineage/CAS de draft |
| autores e timestamps | auditoria |

`intelligence.recommendation_policy_bindings` seleciona a versão efetiva:

| Coluna | Tipo/regra |
|---|---|
| `id`, `account_id`, `client_account_id`, `agent_id` | escopo; agent nullable |
| `policy_definition_id`, `recommendation_policy_version_id` | versão published |
| `process_key` | processo canônico correspondente |
| `status`, `revision` | `draft`, `published`, `archived` + CAS |
| `rollout_policy` | off/shadow/canary/full |
| autores e timestamps | auditoria |

As regras nunca desligam consentimento, quiet hours obrigatórias, opt-out, permissão, allowlist,
FSM ou sender. Resolução segue agência → cliente → agente somente nos campos delegáveis e registra
binding/version em cada recomendação. Publish/rollback reponta binding; não reescreve histórico.

### 6.10 `intelligence.recommendations`

CI-04 cria a estrutura mínima; geração, feedback e portfólio são detalhados em CI-09.

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id`, `subject_id`, `relationship_id` | escopo individual |
| `recommendation_type` | `follow_up`, `offer`, `important_date`, `next_action` |
| `status` | `proposed`, `approved`, `rejected`, `executed`, `expired`, `invalidated` |
| `payload_ciphertext` | objeto fechado por tipo, cifrado |
| `rationale_ciphertext`, `cipher_key_version` | racional cifrado e versão de chave |
| `confidence` | 0..1 |
| `valid_from`, `expires_at` | timestamps |
| `prompt_binding_id`, `runtime_run_id`, `context_snapshot_id` | proveniência |
| `recommendation_policy_binding_id`, `recommendation_policy_version_id` | policy efetiva |
| `approved_by_user_id`, `decided_at` | nullable |
| `outcome` | objeto fechado e sanitizado |
| `created_at`, `updated_at` | timestamps |

Recomendação nunca atualiza um fato histórico nem executa ação externa por conta própria.
Sugestão de fonte permanece em `source_suggestions`; oportunidade cross-client usa entidade própria
de CI-09 e não compartilha a tabela individual.

### 6.11 `intelligence.context_snapshots`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | obrigatórios |
| `subject_id`, `relationship_id` | conforme processo |
| `purpose_key`, `process_key` | allowlisted |
| `schema_version` | contrato do envelope |
| `input_fingerprint` | hash dos insumos e policies |
| `content_ciphertext` | snapshot minimizado cifrado |
| `cipher_key_version` | versão de chave |
| `content_hash` | integridade sem revelar conteúdo |
| `token_budget`, `estimated_tokens` | inteiros não negativos |
| `source_keys`, `omission_codes` | arrays fechados |
| `prompt_binding_id` | binding resolvido |
| `expires_at` | obrigatório |
| `tombstoned_at` | nullable |
| `created_at` | timestamp |

Regras:

- nunca persistir conteúdo em claro;
- TTL obrigatório e curto por process policy;
- acesso exige finalidade, escopo e permissão;
- logs carregam somente ID/fingerprint;
- recomputação após tombstone/invalidação não pode reusar snapshot antigo.

### 6.12 `intelligence.source_suggestions`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | obrigatórios |
| `subject_id`, `relationship_id` | nullable |
| `suggested_source_key` | precisa existir no catálogo ou status `catalog_missing` |
| `missing_capabilities` | array fechado |
| `rationale` | texto limitado |
| `evidence_refs` | somente IDs internos, limitados |
| `confidence` | 0..1 |
| `status` | `pending`, `accepted`, `rejected`, `expired`, `catalog_missing` |
| `runtime_run_id`, `decided_by_user_id`, `decided_at` | nullable |
| `created_at`, `expires_at` | timestamps |

Aceitar sugestão não habilita fonte. Apenas abre um draft de configuração para decisão humana.

### 6.13 `intelligence.audit_events`

Append-only para ações administrativas e sensíveis:

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | escopo |
| `subject_id`, `relationship_id` | nullable |
| `event_type` | catálogo fechado |
| `actor_type`, `actor_id` | `user`, `system`, `runtime`, `source` |
| `entity_type`, `entity_id` | alvo |
| `request_id`, `correlation_id`, `causation_id` | correlação |
| `before_masked`, `after_masked` | diffs mínimos |
| `reason_code` | obrigatório em ação sensível |
| `created_at` | timestamp |

Eventos mínimos: observação criada/duplicada/tombstonada, claim criado/rejeitado/verificado, fato
resolvido/contestado/corrigido, síntese publicada/invalidada, source suggestion decidida, prompt
publicado/rollback, snapshot acessado e export/compartilhamento autorizado.

## 7. Process, Pipeline e Prompt Registry — persistência

### 7.1 Separação de escopos

Para não misturar configuração de plataforma com dado tenant:

- `intelligence.process_definitions` é o catálogo estável de processos platform-wide;
- `intelligence.process_config_versions` versiona schema, variáveis, capabilities e comportamento
  estruturado de cada processo;
- `intelligence.pipeline_definitions`, `intelligence.pipeline_versions` e os bindings de agente
  versionam a composição estruturada entre processos sem criar mega-prompt;
- `intelligence.prompt_definitions` cataloga os slots de camada de cada processo, sem conteúdo
  tenant ou PII;
- `intelligence.platform_prompt_versions` guarda somente `platform_guardrail`, administrado por
  `platform_admin` com permissão específica;
- versões de agência/cliente/processo/agente ficam em tabelas com `account_id not null`;
- bindings tenant-scoped referenciam uma configuração de processo e versões imutáveis das cinco
  camadas.

Essa separação evita linha tenant com `account_id null` e permite reproduzir uma execução mesmo
depois que schemas, variáveis ou defaults do processo evoluírem.

### 7.2 `intelligence.process_definitions`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `process_key` | chave estável única |
| `owner_module_id` | owner registrado |
| `label`, `description` | textos administrativos |
| `catalog_status` | `registered`, `deprecated`; desabilitação operacional não altera história |
| `replacement_process_key` | nullable, somente para depreciação explícita |
| `created_at`, `updated_at`, `deprecated_at` | timestamps |

`process_key` e owner são imutáveis. Label pode evoluir com auditoria, mas esta tabela não guarda
prompt, endpoint, schema mutável, threshold ou configuração de runtime.

### 7.3 `intelligence.process_config_versions`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `process_definition_id` | processo estável |
| `version` | inteiro positivo |
| `status` | `draft`, `validated`, `published`, `archived` |
| `input_schema_version`, `output_schema_version` | contratos registrados em código |
| `variable_contract_version` | versão do catálogo tipado |
| `allowed_layer_kinds` | array fechado |
| `allowed_source_capabilities`, `required_source_capabilities` | arrays fechados |
| `allowed_tool_capabilities`, `allowed_knowledge_capabilities` | arrays fechados |
| `allowed_invocation_modes` | subconjunto de `conversation`, `headless`, `simulation`, `replay` |
| `default_failure_mode` | código fechado |
| `runtime_policy` | objeto fechado, validado dentro dos hard caps de CI-06 |
| `config_checksum` | fingerprint canônico |
| `based_on_version_id` | nullable |
| `revision` | bigint somente enquanto draft |
| `created_by_user_id`, `published_by_user_id` | auditoria |
| `created_at`, `updated_at`, `published_at` | timestamps |

Unique `(process_definition_id, version)`. Publicada é imutável. A tabela versiona configuração
executável, não código: não aceita SQL, URL, resolver, template ou capability fora dos catálogos Go.
Configuração tenant mais restritiva fica no binding/policy estruturada; nunca amplia os máximos
desta versão nem os invariantes de código.

### 7.4 `intelligence.pipeline_definitions` e `intelligence.pipeline_versions`

`pipeline_definitions`:

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `pipeline_key` | chave estável única, inicialmente `conversation.respond` |
| `owner_module_id` | `customer_intelligence` |
| `entrypoint_schema_version` | contrato de alto nível |
| `allowed_process_keys` | catálogo máximo fechado |
| `catalog_status`, timestamps | lifecycle auditável |

`pipeline_versions`:

| Coluna | Tipo/regra |
|---|---|
| `id`, `pipeline_definition_id`, `version` | identidade imutável após publish |
| `account_id` | obrigatório; versão administrativa nunca usa tenant nulo |
| `client_account_id` | nullable para default da agência; quando presente, pertence ao account |
| `status` | `draft`, `validated`, `published`, `archived` |
| `steps` | grafo fechado de process keys e schemas registrados |
| `branch_policy_keys` | condições estruturadas allowlisted, nunca código/expressão livre |
| `process_config_version_ids` | versões compatíveis por etapa |
| `max_runs`, `max_duration_ms`, `max_cost_units` | hard bounds |
| `config_checksum`, `based_on_version_id`, `revision` | concorrência/lineage |
| autores e timestamps | auditoria |

Unique `(account_id, client_account_id, pipeline_definition_id, version)` com tratamento explícito
de `NULL`. A versão publicada é imutável. A resolução segue default de agência → cliente → binding
de agente somente nos campos delegáveis; ausência não herda pipeline de outro account. O pipeline
`conversation.respond` começa com triagem, policy estruturada e resposta condicional; cada etapa
gera seu próprio `ProcessResult.v1`. Ele não pode remover revalidação do Omnichannel, incluir
processo/tool/source fora do catálogo, executar sender nem armazenar prompt embutido.

### 7.5 `intelligence.prompt_definitions`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `process_definition_id` | processo estável |
| `layer_kind` | uma das cinco camadas canônicas |
| `slot_key` | chave estável administrativa |
| `required` | boolean conforme camada/processo |
| `catalog_status` | `registered`, `deprecated` |
| `created_at`, `updated_at` | timestamps |

Unique `(process_definition_id, layer_kind)`. Prompt definition é somente o slot; não guarda
conteúdo, schema mutável nem endpoint.

### 7.6 `intelligence.platform_prompt_versions`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `prompt_definition_id` | slot `platform_guardrail` do processo |
| `validated_process_config_version_id` | configuração usada na validação |
| `version` | inteiro positivo |
| `status` | `draft`, `validated`, `published`, `archived` |
| `content` | guardrail em texto |
| `content_hash` | fingerprint |
| `variable_contract_version` | contrato tipado |
| `revision` | bigint somente enquanto draft |
| `validated_by_user_id`, `validated_at` | nullable; obrigatórios em validated/published |
| `published_by_user_id`, `published_at` | nullable |
| `created_at` | timestamp |

Unique `(prompt_definition_id, version)`. Editar draft validada a devolve a `draft` e invalida a
validação; publicada é imutável.

### 7.7 `intelligence.prompt_versions`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id` | UUID obrigatório |
| `process_definition_id`, `prompt_definition_id`, `process_key` | processo e slot |
| `validated_process_config_version_id` | configuração usada na validação |
| `layer_kind` | `agency_policy`, `client_policy`, `process_prompt`, `agent_override` |
| `client_account_id` | obrigatório para `client_policy`; nullable nos demais conforme binding |
| `agent_id` | obrigatório para `agent_override` |
| `version` | inteiro positivo dentro do escopo |
| `status` | `draft`, `validated`, `published`, `archived` |
| `content` | texto limitado |
| `content_hash` | fingerprint |
| `change_summary` | texto |
| `based_on_version_id` | nullable |
| `revision` | bigint somente enquanto draft |
| `created_by_user_id`, `published_by_user_id` | UUID nullable |
| `created_at`, `updated_at`, `published_at` | timestamps |

Published não recebe update. Edição de published sempre cria draft novo.

### 7.8 `intelligence.prompt_variables`

Catálogo tipado por processo, não valores persistidos da execução:

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `process_definition_id`, `process_config_version_id` | processo e configuração |
| `variable_key` | única por versão de configuração |
| `value_type` | tipo fechado descrito em CI-06 |
| `resolver_key` | resolver Go allowlisted; nunca SQL/URL/path livre |
| `required` | boolean |
| `missing_behavior` | `fail`, `omit`, `default` |
| `default_value` | validado e sem segredo |
| `sensitivity` | classificação |
| `max_items`, `max_chars`, `token_priority` | limites |
| `allowed_layers` | array fechado |
| `active` | boolean |

Placeholder desconhecido, variável em camada não permitida ou campo obrigatório sem resolver
bloqueiam validação/publicação.

### 7.9 `intelligence.prompt_bindings`

Binding é o bundle imutável efetivamente resolvido:

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id` | UUID obrigatório; nunca existe binding tenant com account nula |
| `client_account_id` | nullable somente no default account-wide |
| `agent_id` | nullable; exige `client_account_id` quando preenchido |
| `process_definition_id`, `process_key` | processo |
| `process_config_version_id` | configuração published obrigatória |
| `version` | versão do binding |
| `status` | `draft`, `published`, `archived` |
| `platform_guardrail_version_id` | obrigatório |
| `agency_policy_version_id` | nullable |
| `client_policy_version_id` | nullable |
| `process_prompt_version_id` | obrigatório |
| `agent_override_version_id` | nullable |
| `output_schema_version` | contrato fixado |
| `source_policy` | chaves/capacidades allowlisted |
| `tool_policy` | chaves/operações allowlisted |
| `model_policy_ref` | referência à configuração versionada de CI-06 |
| `provisioning_manifest_version` | nullable; origem do default materializado |
| `compiled_fingerprint` | hash das camadas e policies |
| `published_by_user_id`, `published_at`, `created_at` | auditoria |

As uniques são parciais e explícitas:

- default account-wide: `(account_id, process_definition_id, version)` quando client/agent são null;
- default do cliente: `(account_id, client_account_id, process_definition_id, version)` quando
  client não é null e agent é null;
- override do agente: `(account_id, client_account_id, agent_id, process_definition_id, version)`
  quando agent não é null.

`client_account_id is null` exige `agent_id is null`; agent exige client. Default de plataforma não
é linha runtime com `account_id null`: o enable/provisionamento pode materializar, transacionalmente,
um default account-wide tenant-owned e registrar `provisioning_manifest_version`. Sem binding
materializado/publicado, o processo fica `not_configured` e segue fallback seguro.

Publicação exige todas as referências published e compatíveis.
`output_schema_version` deve ser idêntica à configuração fixada; a duplicação serve apenas para
consulta/auditoria e recebe constraint ou validação transacional.

### 7.10 `intelligence.prompt_test_cases`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | escopo |
| `process_definition_id`, `process_config_version_id`, `process_key` | processo/configuração |
| `name`, `description`, `tags` | metadados |
| `fixture_schema_version` | contrato |
| `fixture_ciphertext` | fixture sensível cifrada ou referência sintética |
| `fixture_hash` | integridade |
| `assertions` | DSL fechada de eval |
| `enabled`, `required_for_publish` | booleans |
| `created_by_user_id`, `created_at`, `updated_at` | auditoria |

Nenhum teste pode carregar segredo. Fixture histórica exige permissão, finalidade e retenção.

### 7.11 `intelligence.prompt_evaluations`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id` | escopo |
| `process_definition_id`, `process_config_version_id` | processo/configuração |
| `prompt_binding_id`, `prompt_test_case_id` | referências |
| `runtime_run_id` | nullable |
| `evaluator_type`, `evaluator_version` | `deterministic`, `human`, `model_judge` |
| `status` | `queued`, `running`, `passed`, `failed`, `error`, `cancelled` |
| `scores` | objeto fechado |
| `failure_codes` | array fechado |
| `output_masked` | resultado limitado |
| `prompt_tokens`, `completion_tokens`, `cost_usd`, `latency_ms` | métricas |
| `created_at`, `completed_at` | timestamps |

### 7.12 `intelligence.prompt_rollouts`

| Coluna | Tipo/regra |
|---|---|
| `id` | UUID PK |
| `account_id`, `client_account_id`, `agent_id` | escopo |
| `process_definition_id`, `process_key` | processo |
| `baseline_binding_id`, `candidate_binding_id` | published |
| `mode` | `shadow`, `canary`, `full`, `paused`, `rolled_back`, `stopped` |
| `traffic_percent` | 0..100 |
| `bucket_seed_version` | hashing determinístico |
| `entry_criteria`, `stop_criteria` | policies fechadas |
| `started_by_user_id`, `started_at`, `ended_at` | auditoria |
| `rollback_reason` | nullable |

Um processo/escopo tem no máximo um rollout ativo. Rollback reponta resolução futura ao baseline;
não reescreve binding nem runs passados.

## 8. Contratos de serviço

```go
type ObservationInput struct {
    AccountID, ClientAccountID string
    SubjectID, RelationshipID  *string
    SourceConfigID, SourceKey  string
    SourceEntityType           string
    SourceEntityID             string
    SourceVersion              string
    IdempotencyKey             string
    OccurredAt                 time.Time
    SnapshotSchemaVersion      string
    SnapshotSanitized          json.RawMessage
    Sensitivity, PurposeKey    string
}

type ClaimInput struct {
    AccountID, ClientAccountID, SubjectID, RelationshipID string
    FactKey, ExtractionMethod, ExtractorKey, ExtractorVersion string
    Value json.RawMessage
    Confidence float64
    ObservationIDs []string
    RuntimeRunID, PromptBindingID *string
}

type ResolveFactRequest struct {
    AccountID, ClientAccountID, SubjectID, RelationshipID, FactKey string
    Cause string
}

type BuildSummaryRequest struct {
    AccountID, ClientAccountID, SubjectID, RelationshipID string
    SummaryType, ProcessKey string
    AsOf time.Time
}
```

Regras de transação:

1. ingestão deduplica observação antes de extrair;
2. claim e `claim_evidence` nascem na mesma transação;
3. resolução trava a projeção corrente por conta/relação/fato;
4. nova versão e supersede da anterior são atômicos;
5. evento/outbox de rebuild nasce na transação da alteração aceita;
6. nenhum LLM é chamado dentro da transação PostgreSQL.

## 9. APIs propostas

Todas sob módulo/permissão e escopo derivado do Principal:

| Método e rota | Permissão | Contrato |
|---|---|---|
| `GET /v1/customer-intelligence/relationships/{relationshipId}/facts` | `customer_intelligence.profile.view` | cursor, `factKey`, `state`, `asOf` |
| `GET /v1/customer-intelligence/facts/{factId}` | `customer_intelligence.profile.view` | fato + claims/evidências permitidas |
| `POST /v1/customer-intelligence/facts/{factId}/verify` | `customer_intelligence.profile.manage` | `{decision, reason, expectedVersion}` |
| `POST /v1/customer-intelligence/relationships/{relationshipId}/facts/manual` | `customer_intelligence.profile.manage` | observação manual + claim, idempotency key |
| `GET /v1/customer-intelligence/relationships/{relationshipId}/timeline` | `customer_intelligence.profile.view` | cursor temporal |
| `GET /v1/customer-intelligence/relationships/{relationshipId}/summaries` | `customer_intelligence.profile.view` | tipo/status |
| `GET /v1/customer-intelligence/observations/{observationId}` | `customer_intelligence.audit.view` | snapshot mascarado conforme sensibilidade |
| `GET /v1/customer-intelligence/audit-events` | `customer_intelligence.audit.view` | filtros allowlisted e cursor |
| `GET /v1/customer-intelligence/fact-definitions` | `customer_intelligence.profile.view` | catálogo e policy efetiva |
| `POST /v1/customer-intelligence/fact-definitions` | `customer_intelligence.profile.manage` | cria key estável + versão 1 draft |
| `POST /v1/customer-intelligence/fact-definitions/{id}/versions` | `customer_intelligence.profile.manage` | clona nova versão draft |
| `PATCH /v1/customer-intelligence/fact-definition-versions/{id}` | `customer_intelligence.profile.manage` | altera somente draft + `expectedRevision` |
| `POST /v1/customer-intelligence/fact-definition-versions/{id}/validate` | `customer_intelligence.profile.manage` | schema/compatibilidade |
| `POST /v1/customer-intelligence/fact-definition-versions/{id}/publish` | `customer_intelligence.profile.manage` | publica e reponta `active_version_id` por CAS |
| `POST /v1/customer-intelligence/fact-definitions/{id}/version-rollback` | `customer_intelligence.profile.manage` | reponta versão published anterior |
| `POST /v1/customer-intelligence/fact-definitions/{id}/authority-policy-drafts` | `customer_intelligence.profile.manage` | cria policy draft |
| `POST /v1/customer-intelligence/authority-policy-versions/{id}/publish` | `customer_intelligence.profile.manage` | publica versão imutável; não muda binding |
| `POST /v1/customer-intelligence/authority-policy-bindings` | `customer_intelligence.profile.manage` | aponta scope para versão published por CAS |
| `POST /v1/customer-intelligence/authority-policy-bindings/{id}/rollback` | `customer_intelligence.profile.manage` | reponta versão anterior |
| `GET /v1/customer-intelligence/retention-policies` | `customer_intelligence.sources.view` | versões e policy efetiva |
| `POST /v1/customer-intelligence/retention-policies/{policyKey}/drafts` | `customer_intelligence.sources.manage` | cria draft |
| `POST /v1/customer-intelligence/retention-policy-versions/{id}/publish` | `customer_intelligence.sources.manage` | publica após gates jurídicos |
| `POST /v1/customer-intelligence/sources/{id}/retention-policy-preview` | `customer_intelligence.sources.manage` | impacto do rebind |
| `POST /v1/customer-intelligence/sources/{id}/retention-policy-rebind` | `customer_intelligence.sources.manage` | reponta versão published por CAS |
| `POST /v1/customer-intelligence/sources/{id}/retention-policy-rollback` | `customer_intelligence.sources.manage` | reponta versão anterior |
| `GET /v1/customer-intelligence/processes` | `customer_intelligence.prompts.view` | catálogo sem prompt sensível |
| `GET /v1/customer-intelligence/pipelines` | `customer_intelligence.prompts.view` | entrypoints/versões estruturadas |
| `GET /v1/customer-intelligence/prompts?processKey=...` | `customer_intelligence.prompts.view` | definições/versões/bindings do escopo autorizado |

Listas usam cursor estável `(created_at,id)` ou `(effective_at,id)`, `limit` default 50, máximo
200. Fora de escopo retorna 404. Snapshot restrito pode retornar metadados e `contentRestricted:
true`, nunca degradar para conteúdo sem autorização.

Definições e authority policies aparecem no painel como configuração estruturada, com draft,
diff, validação, publish e rollback; nunca como instrução escondida no prompt. Correção de cadastro
determinístico continua em Customer Data com `customer_data.subjects.manage`; os endpoints acima
governam somente claims/fatos derivados.

### 9.1 Compatibilidade do Prompt Registry

`messaging.ai_agent_versions` já é a fonte autoritativa de layers, output schema, versão,
publish/rollback e versão ativa do agente. O novo registry não pode fingir que essa base não existe
nem começar como segundo writer.

Estado transitório por `account_id + client_account_id + agent_id + process_key`:

| Estado | Writer | Leitura/execução |
|---|---|---|
| `legacy` | `messaging.ai_agent_versions` e APIs Omnichannel | runtime/UI atuais |
| `shadow` | legado | Prompt Registry importa por referência/hash e compara; não ativa |
| `new` | `intelligence.prompt_*` | novo resolver; APIs/UI antigas são fachada/read-only |

Regras:

- mappings preservam `legacy_agent_id` e `legacy_agent_version_id`;
- uma versão legada importada registra hash, versão e source ref;
- o prompt combinado atual gera drafts separados para `conversation.triage` e
  `conversation.reply`, mas nenhum deles é publicado automaticamente;
- publish/rollback atuais continuam com a mesma semântica enquanto o writer for `legacy`;
- `ConfigAiAgentVersions.vue` permanece funcional até CI-08 provar paridade e compatibilidade;
- dual-read é permitido para comparação; dual-write permanente é proibido;
- congelar o writer antigo, executar delta/checksum e trocar o resolver precedem a nova escrita;
- rollback depois da troca mantém o writer novo e serve fachada antiga; não reativa dois writers.

## 10. Compatibilidade com `messaging.contact_intelligence`

### 10.1 Projeção temporária

Enquanto o writer legado for autoritativo:

- adapter lê `messaging.contact_intelligence` pelo service Omnichannel;
- gera observação `omnichannel.contact_memory.legacy`;
- `facts`/`preferences` viram claims `llm` com confiança e proveniência explicitamente limitadas;
- `summary` vira síntese `legacy_imported`, não fato;
- `last_*` vira metadado de observação;
- nenhum backfill decide conflitos automaticamente.

### 10.2 State machine do writer

| Estado | Writer | Leitura |
|---|---|---|
| `legacy` | `messaging.contact_intelligence` | fachada legado |
| `shadow` | legado; novo banco apenas observa/compara | legado + relatório interno |
| `new` | Intelligence Bank | nova projeção; fachada antiga lê adapter |

É proibido manter ambos como writers. A transição por cliente/relação exige watermark, checksum,
contagem e relatório de divergência.

### 10.3 Retirada

`messaging.contact_intelligence` só vira candidata a retirada em CI-10 depois de:

- escritor legado congelado;
- zero consumidor direto;
- fachada antiga lendo a nova fonte;
- retenção resolvida;
- rollback ensaiado;
- autorização explícita do owner.

## 11. Retenção, retificação e exclusão

Cada categoria define:

- prazo de observação;
- prazo de claim/fato/síntese;
- TTL de context snapshot;
- se hash/fingerprint pode permanecer;
- regra de legal hold;
- efeito em backups;
- bloqueio de reingestão.

Fluxo de exclusão:

1. registrar pedido e escopo;
2. bloquear nova ingestão para as identidades/fontes afetadas;
3. tombstonar observações;
4. invalidar claims/fatos/sínteses/recomendações;
5. apagar ou crypto-shred snapshots/ciphertexts;
6. recomputar projeções sem os insumos removidos;
7. registrar resultado e pendências de backup/legal hold.

Não existe `DELETE CASCADE` administrativo disparado diretamente pelo painel.

## 12. Tenant, permissões e privacidade

Permissões mínimas propostas:

- `customer_intelligence.profile.view`;
- `customer_intelligence.profile.manage`;
- `customer_intelligence.audit.view`;
- `customer_intelligence.agents.manage`;
- permissão platform-only para guardrail;
- permissões de fontes e portfólio permanecem separadas.

Testes negativos devem cobrir:

- outra `account_id`;
- cliente fora do catálogo permission-scoped;
- subject correto com relationship de outro cliente;
- ID válido de outra organização;
- observação restrita sem `customer_intelligence.audit.view`;
- usuário com permissão antiga `omnichannel.agents.manage` sem permissão nova;
- platform admin sem finalidade cross-client aprovada.

## 13. Observabilidade

Métricas sem PII:

- observações criadas/duplicadas/rejeitadas por `source_key`;
- backlog e idade de ingestão;
- claims por método/estado;
- conflitos e tempo até revisão;
- resoluções por policy/version;
- sínteses geradas/reutilizadas/invalidadas;
- snapshots criados/expirados/acessados;
- dedupe hit rate;
- latência e erro de store/job;
- prompt versions/bindings/rollouts por processo, sem conteúdo.

Logs estruturados carregam operação, account, client, IDs internos, request/correlation, código de
erro e duração. Não carregam valor do fato, snapshot, prompt, documento ou conteúdo de mensagem.

## 14. Pacotes atômicos e allowlists

Cada pacote executa preflight, registra baseline e não toca arquivo fora da allowlist.

### Leitura permitida

- `AGENT.md`, skills aplicáveis e AGENTs de database/Omnichannel/Customer Data;
- `docs/customer-intelligence/GOVERNANCA.md`, `SPECS_GERAIS.md` e specs CI-00 a CI-03;
- documentos canônicos em `docs/omnichannel/`;
- migrations `0206`, `0216`, `0219`, `0222`–`0225`, `0228`, `0234` e `0236`;
- `back/internal/modules/omnichannel/ai_*`, `brain_*`, `contact_intelligence.go`,
  `service_triage.go`, `store_ai*.go` e testes correspondentes;
- contratos públicos implementados por `customerdata`;
- `back/internal/platform/modules/**`, `events/**`, `database/**` e `secretbox/**`;
- `back/database/ERD.md`;
- `web/app/components/omnichannel/config/ConfigAiAgentVersions.vue` e seu client tipado, somente
  para inventário de compatibilidade.

Leitura de `.env`, credentials, execution data, volumes, mídia privada ou payload real é proibida.
Outro módulo só pode ser lido quando necessário para contrato público; seu internals não viram
dependência do runtime.

### CI04-DOC-01 — congelar modelo

**Escrita permitida:**

- `docs/customer-intelligence/specs/CI-04_INTELLIGENCE_BANK.md`;
- ADR específico criado por CI-00, somente se o pacote for despachado conjuntamente.

**Proibido:** código, migration, workflow e ERD.

### CI04-DB-01 — núcleo do banco

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_bank.sql`;
- `back/database/ERD.md`;
- `back/database/AGENT.md`;
- `back/internal/modules/customerintelligence/AGENT.md`.

Inclui fact definitions/policies, source configs/runs, observations, claims, facts, summaries,
recommendations, context snapshots, suggestions e audit. Número é reservado pelo orquestrador.

### CI04-DB-02 — Process, Pipeline e Prompt Registry

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_prompt_registry.sql`;
- `back/database/ERD.md`;
- `back/database/AGENT.md`;
- `back/internal/modules/customerintelligence/AGENT.md`.

Não compartilha migration com CI04-DB-01 se o lock/volume dificultar revisão.
Inclui process definitions/config versions, pipeline definitions/versions, prompt
slots/versions/variables, bindings, test cases, evaluations e rollouts.

### CI04-DB-03 — policies de recomendação

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_recommendation_policies.sql`;
- `back/database/ERD.md`;
- `back/database/AGENT.md`;
- `back/internal/modules/customerintelligence/AGENT.md`.

Cria definitions/versions/bindings e refs colunares em recommendations. Deve executar antes de
qualquer pacote CI-09 que gere recomendação; não compartilha migration com portfólio.

### CI04-BE-DOMAIN-01 — evidência e fatos

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_bank.go`;
- `back/internal/modules/customerintelligence/errors.go`;
- `back/internal/modules/customerintelligence/policy_authority.go`;
- `back/internal/modules/customerintelligence/service_bank.go`;
- testes correspondentes `*_test.go`;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI04-BE-STORE-01 — persistência

**Escrita permitida:**

- `back/internal/modules/customerintelligence/store.go`;
- `back/internal/modules/customerintelligence/store_observations.go`;
- `back/internal/modules/customerintelligence/store_claims.go`;
- `back/internal/modules/customerintelligence/store_facts.go`;
- `back/internal/modules/customerintelligence/store_summaries.go`;
- `back/internal/modules/customerintelligence/store_audit.go`;
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI04-BE-PROMPTS-01 — persistência do registry

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_prompt.go`;
- `back/internal/modules/customerintelligence/service_prompt.go`;
- `back/internal/modules/customerintelligence/store_prompt.go`;
- `back/internal/modules/customerintelligence/policy_prompt.go`;
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

Não implementa execução LLM; isso pertence a CI-06.

### CI04-BE-PIPELINES-01 — persistência de pipelines

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_pipeline.go`;
- `back/internal/modules/customerintelligence/service_pipeline.go`;
- `back/internal/modules/customerintelligence/store_pipeline.go`;
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

Não executa processos, LLM, sender ou efeito operacional; apenas lifecycle, versão e binding
estruturados.

### CI04-API-01 — leitura e revisão

**Escrita permitida:**

- `back/internal/modules/customerintelligence/http_bank.go`;
- `back/internal/modules/customerintelligence/http_prompt.go`;
- `back/internal/modules/customerintelligence/http_pipeline.go`;
- `back/internal/modules/customerintelligence/http_helpers.go`;
- testes HTTP correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

Wiring/metadata ficam bloqueados até pacote de integração aprovado.

### CI04-JOB-01 — resolução/rebuild

**Escrita permitida:**

- `back/internal/modules/customerintelligence/job_resolution.go`;
- `back/internal/modules/customerintelligence/store_jobs.go`;
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

Não usa o bus in-memory como garantia de entrega.

### CI04-BACKFILL-01 — legado

**Escrita permitida:**

- `back/cmd/customer-intelligence-backfill/main.go`;
- `back/internal/modules/customerintelligence/backfill_contact_intelligence.go`;
- testes correspondentes;
- relatório gerado sob `docs/customer-intelligence/evidence/`;
- `back/internal/modules/customerintelligence/AGENT.md`.

Leitura de `messaging.*` ocorre por adapter/service aprovado; se for necessário SQL de migração
controlada, ele fica documentado e isolado neste pacote, nunca no runtime normal.

### CI04-QA-01 — revisão independente

**Escrita permitida:** somente testes faltantes dentro dos arquivos `*_test.go` do módulo e
evidências sob `docs/customer-intelligence/evidence/CI-04/`.

### CI04-CUTOVER-01

**Escrita permitida:**

- feature flags/writer state aprovados no módulo Customer Intelligence;
- adapter de compatibilidade explicitamente listado pelo orquestrador;
- documentação/evidências de cutover.

Não autoriza `DROP`, remoção de rota ou edição de workflow.

### Arquivos sempre proibidos nesta spec

- `automation/export/**`;
- `automation/workflow-whatsapp.json`;
- qualquer workflow Calendar/Operação;
- `back/internal/modules/socialpublishing/**`;
- `docs/social-publishing/**`;
- migrations existentes `0001`–`0238`;
- canal, outbox, FSM e providers do Omnichannel;
- secrets, `.env`, volumes e dados reais.

## 15. Fluxos obrigatórios

### Sucesso

```text
evento/fonte
  -> idempotency check
  -> observation append-only
  -> extractor cria claim(s)
  -> resolver aplica authority policy
  -> fato novo/supersede atômico
  -> outbox de rebuild
  -> síntese/contexto é invalidado e reconstruído fora da transação
```

### Duplicata

Mesma `source_config_id + idempotency_key` retorna a observação existente; não recria claim,
fato, síntese ou evento downstream.

### Conflito

Claim divergente é persistido, policy decide `resolved` ou `contested`, e a UI/auditoria mostra os
dois valores conforme permissão. Nenhum valor anterior é apagado.

### Falha

- snapshot inválido: observação rejeitada, run parcial/falha auditada;
- relação não resolvida: observação fica em quarentena, sem criar fato;
- policy ausente: fato permanece sem resolução e cria alerta;
- rebuild falha: job retry/dead-letter, fato continua válido e síntese fica stale;
- crypto indisponível: não persiste snapshot sensível nem contexto em claro.

### Concorrência

Resolução usa lock/CAS pela projeção corrente. Dois claims simultâneos produzem uma sequência
determinística de versões; unique parcial impede dois fatos correntes.

## 16. Testes e comandos

Durante execução real:

```powershell
cd back
go test ./internal/modules/customerintelligence/...
go test -race ./internal/modules/customerintelligence/...
go test ./internal/platform/database/...
go test ./...
golangci-lint run ./...
```

Banco:

```powershell
cd back
go run ./cmd/migrate status
go run ./cmd/migrate up
```

Cenários mínimos:

- banco vazio e upgrade da versão anterior;
- dedupe/replay;
- conflito e correção manual;
- policy diferente por tipo de fato;
- versão de fact definition publicada não reinterpreta claim/fato histórico;
- publish/rollback de authority binding preserva versão gravada em cada fato;
- ingestion run/observation fixam source config revision e retention policy version;
- rebind/rollback de retenção não altera `expires_at` histórico silenciosamente;
- tenant/client/relationship negativo;
- tombstone e rebuild;
- context snapshot expirado/cifrado;
- versão de prompt publicada imutável;
- placeholder desconhecido;
- binding cross-tenant;
- rollback de binding sem reescrever histórico;
- backfill com órfão/ambiguidade;
- query principal com índice em volume representativo.

## 17. Rollout

1. DDL aditivo sem tráfego.
2. Services/stores atrás de capability server-side.
3. Backfill somente leitura e relatório.
4. Shadow por cliente, mantendo writer legado.
5. Comparar contagens, fingerprints, conflitos, latência e escopo.
6. Habilitar nova leitura para administradores.
7. Trocar writer de uma entidade/cliente por vez.
8. Manter fachada de leitura antiga.
9. Observar janela aprovada antes de qualquer deprecação.

Métricas quantitativas finais são congeladas em CI-10; sem elas o rollout não passa de shadow.

## 18. Rollback

- Antes do writer novo: desabilitar shadow e continuar legado.
- Depois do writer novo: manter writer novo e servir fachada compatível.
- Nunca reativar writer legado sem reconciliação reversa explícita.
- Preservar observações/runs novos para análise.
- Rollback de prompt reponta binding published anterior; não altera versões nem runs.
- Não reprocessar origem já deduplicada.
- Não apagar schema/tabelas como rollback operacional.

## 19. Critérios de aceite

- [ ] Cada fato expõe origem, instante, método, confiança, estado e policy.
- [ ] Claim/fato fixa a versão semântica da fact definition.
- [ ] Authority policy efetiva vem de binding versionado sem resolução ambígua.
- [ ] Run/observação fixa a retention policy usada.
- [ ] Síntese aponta exatamente fatos/observações usados.
- [ ] Conflito não apaga valor anterior.
- [ ] Correção manual verificada prevalece conforme policy.
- [ ] Mesma origem reprocessada não duplica observação ou derivados.
- [ ] Relações de clientes distintos permanecem isoladas.
- [ ] Snapshot de contexto é cifrado, minimizado e expira.
- [ ] Prompt publicado é imutável e separado por `process_key`.
- [ ] Binding registra as cinco camadas e suas versões.
- [ ] Variável inválida bloqueia publish.
- [ ] `messaging.contact_intelligence` não é segunda verdade após cutover.
- [ ] Nenhum payload bruto/segredo entra no banco ou log.
- [ ] Nenhum SQL cross-module aparece no runtime normal.

## 20. Stop conditions

O executor para antes de escrever quando:

- CI-00 escolher IDs/schema diferentes e a spec não tiver sido reconciliada;
- CI-03 não fornecer validação estável de relationship;
- houver migration concorrente sem número reservado;
- houver mudança prévia do usuário na allowlist que seria sobrescrita;
- a policy de retenção/crypto for necessária para dado real e continuar indefinida;
- DDL criar writer concorrente ou editar migration aplicada;
- uma tabela proposta duplicar entidade semântica já criada por outra spec;
- teste revelar vazamento cross-tenant;
- implementação exigir tocar workflow, sender, social-publishing ou módulo não autorizado.

## 21. Handoff obrigatório

Cada pacote entrega:

- baseline e `git status --short`;
- arquivos lidos/alterados;
- migration reservada e `migrate status`, quando houver;
- decisões `CI-DEC-*` aplicadas;
- tabelas/índices/constraints efetivamente criados;
- testes e resultados;
- contagens de backfill, órfãos, conflitos e checksums;
- prova de imutabilidade/dedupe/tenant;
- rollout/rollback e ponto sem retorno;
- consumidores ainda no legado;
- confirmação de que nenhum workflow, canal, secret ou social-publishing foi alterado.
