# AGENT

## Escopo

Estas instrucoes valem para tudo que for banco de dados no backend:

- schema
- migrations
- convencoes SQL
- modelagem tenant/store
- documentacao estrutural do banco

## Versoes padrao desta base

- Go: `1.24.0`
- Toolchain Go: `1.24.3`
- Nuxt no frontend integrado: `4.4.2`
- Pinia no frontend integrado: `3.0.4`
- Driver PostgreSQL no backend: `pgx/v5 5.7.6`
- PostgreSQL alvo: `16`
- Tag recomendada para Docker futuro: `postgres:16-alpine`

## Localizacao atual

- migrations SQL: `back/internal/platform/database/migrations`
- pool/conexao: `back/internal/platform/database/pool.go`
- runner de migrations: `back/internal/platform/database/migrator.go`
- comando de migration: `back/cmd/migrate/main.go`
- visualizacao humana: `back/database/ERD.md`
- visualizacao local no Windows: `back/scripts/postgres/open-pgadmin.ps1`

## Regras de modelagem

### 1. Multi-tenant

- toda entidade de negocio futura deve considerar `tenant_id` quando fizer sentido
- tudo que for operacional por loja deve considerar tambem `store_id`
- `platform_admin` fica fora de tenant
- contas de loja/operacao com escopo de store devem tender a uma unica loja por usuario
  - hoje isso vale para `consultant`, `manager` e `store_terminal`

### 2. IDs

- usar `uuid` como PK nas tabelas principais
- evitar PK numerica incremental como identificador externo

### 3. Datas

- usar `timestamptz`
- toda tabela principal deve ter pelo menos `created_at`
- tabelas mutaveis devem ter `updated_at`

### 4. Soft delete vs archive

- preferir `is_active`/`archived_at` quando a regra pedir historico preservado
- nao apagar dado operacional importante sem motivo forte

### 5. Email e unicidade

- email deve ser unico case-insensitive
- no PostgreSQL atual estamos tratando isso com indice em `lower(email)`
- onboarding de usuario por convite deve usar tabela dedicada
  - `user_invitations`
  - token persistido apenas em hash
  - `users.password_hash` pode ficar nulo enquanto o convite nao for aceito
- senhas temporarias administrativas devem usar flag explicita
  - `users.must_change_password`
  - nao depender de comparar hash ou tentar inferir pela senha padrao
- o modelo de acesso operacional deve suportar:
  - conta individual do consultor
  - conta fixa do terminal da loja
  - conta gerencial por loja
  - conta tenant-wide para `owner` e `marketing`
- foto de perfil do usuario:
  - o banco guarda apenas `users.avatar_path`
  - o arquivo binario nao deve ir para coluna blob nesta fase
  - no Docker, o storage local deve ficar em volume dedicado para nao perder arquivo no recreate do container

### 6. Regra de payload e mutacao

- banco nao deve ser tratado como destino de bundles gigantes para alteracoes pequenas
- uma alteracao pequena deve gerar:
  - payload pequeno na API
  - SQL pequeno e previsivel
  - escrita focada apenas nas linhas/colunas afetadas
- campos opcionais ou nao aplicaveis devem ser omitidos no JSON sempre que o backend puder assumir zero-value com seguranca
- evitar estrategia de `delete all + insert all` quando a intencao do usuario for adicionar, editar ou remover um unico item
- manter endpoints bulk apenas para cenarios de importacao, seed controlada, template ou substituicao total intencional
- sempre considerar custo de I/O, lock, WAL, rede e observabilidade ao desenhar mutacoes
- exclusoes administrativas precisam validar dependencias de negocio antes de executar `delete`
- `on delete cascade` deve ser tratado como protecao de integridade, nao como politica primaria de remocao
- leituras administrativas expandidas devem ficar separadas das leituras operacionais normais para manter o bootstrap leve
- seguranca por loja/dispositivo ainda e backlog, mas a modelagem deve preservar espaco para:
  - vinculo entre conta de terminal e loja
  - auditoria de sessao por dispositivo
  - futuras restricoes de login por origem/estacao
- para leitura analitica, preferir indices e consultas coerentes com o recorte real da tela:
  - `store_id`
  - intervalo de tempo
  - consultor
  - desfecho
- para visao operacional integrada multi-loja:
  - consultas precisam continuar baratas por `tenant_id + store_id`
  - o dado devolvido ao frontend deve carregar identificacao visual minima da loja (`store_id`, `store_name`, opcionalmente `store_code`)
  - a tabela `operation_paused_consultants` deve preservar o tipo do afastamento (`pause` vs `assignment`)
  - a configuracao operacional vive em tabelas `tenant_*` desde a migration `0024_tenant_operation_settings.sql`:
    - `tenant_operation_settings`
    - `tenant_setting_options`
    - `tenant_catalog_products`
  - a tabela `tenant_setting_options` precisa manter `kind` alinhado com os catalogos vivos da UI:
    - `visit_reason`
    - `customer_source`
    - `pause_reason`
    - `queue_jump_reason`
    - `loss_reason`
    - `profession`
  - as tabelas legadas `store_operation_settings`, `store_setting_options` e `store_catalog_products` permanecem no banco apenas como fonte de backfill durante a transicao; novas escritas vao para o escopo do tenant
- quando o historico tiver campos estruturados em JSON, a agregacao deve priorizar a colecao estruturada como fonte de verdade
  - exemplo: `productsClosed[]` antes de `productClosed`

### 7. Resiliencia futura

- a modelagem deve facilitar:
  - reprocessamento idempotente
  - fila de sincronizacao offline
  - auditoria de mutacoes
  - recuperacao apos falha de rede ou de servico
- isso ainda nao e a prioridade de implementacao, mas ja e uma restricao arquitetural da base

## Regras de migrations

- migrations sao append-only
- nao editar migration ja aplicada em ambiente compartilhado
- nomear com prefixo ordenavel:
  - `0001_init.sql`
  - `0002_seed_demo_auth.sql`
- migration deve ser idempotente quando possivel para facilitar setup local

## Comandos uteis

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
```

```powershell
.\scripts\postgres\status-local.ps1
.\scripts\postgres\open-pgadmin.ps1
```

Ambiente esperado:

- `DATABASE_URL`
- opcionalmente `DATABASE_MIN_CONNS`
- opcionalmente `DATABASE_MAX_CONNS`

## O que consultar antes de mexer no schema

1. `back/PLAN.md`
2. `back/README.md`
3. `back/database/ERD.md`
4. `../docs/operacao/operations.md`
5. `../docs/NUXT_FULL_REFERENCE.md`

## Proxima evolucao esperada

- campos administrativos completos por loja
  - template operacional padrao
  - metas por loja
- consolidacao de leitura para multiloja e usuarios/acessos
- convites e onboarding de usuarios ja modelados com `user_invitations`
- vinculo 1:1 entre consultor operacional e conta autenticada
- politica de primeiro login e senha temporaria ja usa `users.must_change_password`
- conta `store_terminal` por loja para computador fixo da unidade
- visao integrada da operacao ja exige que o banco sustente leitura cross-store leve para `owner` e `platform_admin`
- sessoes persistidas de auth
- auditoria/eventos para realtime e resiliencia offline
- estrategia futura de backup automatizado, restore testado e redundancia do banco

## Omnichannel E1 (migrations 0213-0215)

- `messaging.conversations.ai_generation` e o lease autoritativo de takeover humano; nao criar
  flag paralela em JSON ou no n8n.
- Reply de mensagem usa `reply_to_message_id` quando a original local existe e
  `reply_to_external_message_id` somente como fallback reconciliavel.
- `messaging.messages.origin` e os estados de ACK possuem vocabulario fechado pela migration;
  transicao de ACK deve permanecer monotonica no store.
- A unique parcial de external ID e a barreira de idempotencia do canal por conta+instancia.
- Midia nunca vira blob no PostgreSQL nem URL temporaria autoritativa; arquivo fica em storage
  privado e o banco guarda metadados/chave.
- A migration `0214_messaging_media_audit_events.sql` amplia o vocabulario fechado de
  `messaging.audit_events.event_type` somente com `MESSAGE_MEDIA_READY`,
  `MESSAGE_MEDIA_FAILED` e `MESSAGE_MEDIA_RETRY`.
- A migration `0215_messaging_delivery_reconciliation.sql` persiste o ACK canonico e
  sanitizado em `messaging.webhook_events` mesmo quando a mensagem ainda nao existe. O replay
  sempre filtra `account_id + provider + instance_name + external_message_id` e ordena por
  `provider_status_at + id`; nunca usa payload cru, memoria ou Redis como fonte de verdade.
- `messaging_messages_content_trgm_idx` e o indice GIN da busca existente por
  `lower(messages.content) LIKE '%...%'`. Ele reutiliza `pg_trgm`, instalado pela migration
  `0034_erp_ftp_foundation.sql`; nao criar mecanismo de busca ou copia de conteudo paralela.

## Social publishing (migration 0237)

- `social_publishing.connections` guarda historico imutavel e no maximo uma conexao ativa por
  conta; token cru nunca e persistido, e disconnect faz soft revoke para preservar posts e
  auditoria sem redirecionar publicacoes antigas.
- `social_publishing.posts` e a fonte unica do agendamento/publicacao. Integracoes futuras usam
  `source_type + source_ref`; nao existe FK nem SQL direto para `calendar.*`.
- Analytics corrente vive em `post_analytics`; o historico append-only fica em
  `analytics_snapshots`, deduplicado pelo `job_key` imutavel da outbox. O external media id
  continua somente em `posts`, evitando drift.
- `publish_attempted_at` e gravado antes do efeito externo; resultado ambiguo nunca autoriza retry
  automatico nem edicao/reagendamento ate existir reconciliacao explicita.
- `social_publishing.publish_outbox` e `social_publishing.analytics_outbox` implementam
  separadamente o contrato de `platform/jobs`; rajadas de insights nunca disputam claim/worker
  com publicacoes e nenhuma lane reutiliza `messaging.outbox`.

## Omnichannel CRM intelligence (migration 0236)

- `messaging.contact_intelligence` e uma extensao 1:1 de `messaging.contacts`, sempre com
  `account_id + contact_id` e FKs compostas tenant-safe.
- A tabela guarda somente memoria derivada limitada e metricas. Historico bruto permanece em
  `messaging.messages`; prompt, credencial, documento e dado de pagamento nao podem ser gravados.
- Atualizacao de memoria precisa compartilhar o gate de `state + ai_generation` da conversa para
  impedir aprendizado vindo de uma resposta atrasada ou cancelada.

## Omnichannel -> Customer Data (migration 0245)

- `messaging.customer_data_outbox` é a lane durável da ingestão determinística de um inbound com
  binding de cliente resolvido. Ela satisfaz o contrato do `platform/jobs`, possui FIFO por
  contato e não compartilha claim com envio ao canal ou execução de inteligência.
- A linha nasce na mesma transação de `messages` e `contact_touchpoints`. Seu payload aceita
  somente IDs, canal, provider e `occurredAt`, com checks que os amarram às colunas estruturadas;
  nome, telefone, conteúdo, prompt e credencial são proibidos.
- As FKs compostas repetem `account_id` e, no binding, `client_account_id`. O consumidor deve
  reidratar e revalidar a evidência no PostgreSQL antes de resolver o relacionamento.
- O vínculo `unresolved|quarantined` não produz evento; a conversa humana continua funcionando.
  Falha do consumidor usa retry/dead-letter desta lane e nunca pode disputar a outbox do sender.

## Customer Intelligence runtime e evidências (migrations 0242, 0243, 0246, 0248, 0250, 0251, 0254 e 0255)

- `intelligence.source_ingestion_runs` deduplica por
  `(account_id, client_account_id, idempotency_key)`; queries de replay repetem o client.
- `intelligence.runtime_runs` registra `pipeline_definition_id`, `pipeline_version_id` e
  `execution_mode=active|shadow`, alem das refs de process/config/binding/agent/model/context.
- Run conversacional deduplica por
  `(account_id, client_account_id, request_id, process_key)`.
- Somente `conversation.triage` e `conversation.reply` possuem config ativa nesta fase, ambas com
  schema v2 fechado. Processo sem schema definitivo fica `deprecated` e sem active config.
- Shadow pode persistir run para comparacao, mas nunca autoriza mensagem, handoff, FSM ou outbox.
- `accepted_outcomes` deduplica event/decision por
  `(account_id, client_account_id, event_id|decision_id)`.
- `claims` conserva origem (`source_outcome_event_id + source_claim_ordinal`), prompt/runtime refs,
  revisão e reviewer. Extração LLM nasce `candidate/unverified/llm`; aceitar a revisão não escreve
  em `facts` nem altera `verification_state` para verificado.
- `claim_evidence` só pode apontar para observação reidratada no mesmo
  account/client/subject/relationship. Valor da claim permanece cifrado e não é copiado para a
  outbox operacional.
- A migration `0250_intelligence_observation_audit.sql` audita cada insert de observação com
  `aggregate_type=source_observation`. O evento guarda somente source key, entity type,
  sensibilidade e finalidade; snapshot, ciphertext, external entity id e idempotency key nunca
  entram em `intelligence.audit_events`. A mesma migration fecha os pares válidos de
  classificação/escopo: `customer_relationship` exige subject+relationship e
  `client_business_context` exige ambos nulos.
- Leitura de observação repete `account_id + client_account_id`, valida o relacionamento e reaplica
  a allowlist atual antes de descriptografar. Registro expirado ou fonte desabilitada não entra no
  contexto LLM.
- A migration `0251_intelligence_observation_retention.sql` congela a policy publicada no run e na
  observação, calcula `expires_at` e preserva a linha como tombstone/crypto-shred metadata-only.
- A migration `0254_intelligence_retention_governance.sql` proíbe criar policy já publicada,
  exige transição draft→published com revisão e referência de aprovação e cria
  `intelligence.observation_legal_holds`.
- Hold ativo nunca permite tombstone/crypto-shred: scheduler e worker filtram por
  account+client+observation e um trigger PostgreSQL repete a barreira. Hold é liberado por
  transição auditada; excluir hold ativo ou reescopar um hold é proibido.
- A migration `0255_intelligence_context_snapshot_retention.sql` transforma snapshot expirado em
  tombstone metadata-only: mantém a linha referenciada, mas limpa ciphertext, versão da chave e
  hash. `context_snapshot_legal_holds` usa o mesmo lifecycle `active→released`; holds de observação
  relacionados também protegem o contexto, e advisory locks serializam hold e crypto-shred.

## Gravacao experimental de atendimentos (migration 0256)

- `queue.attendance_recordings` e a fonte autoritativa dos metadados e estados
  de gravacao/transcricao.
- `queue.attendance_recording_chunks` registra cada parte de forma idempotente
  por conta, gravacao e sequencia.
- O audio nao fica no PostgreSQL: as tabelas guardam somente storage key,
  MIME, tamanho e SHA-256; a entrega ocorre por endpoint autenticado.
- As duas tabelas repetem `account_id` e nao criam FK de `queue.*` para
  `core.*`.
- A migration `0257` torna a transcricao um job duravel na propria gravacao:
  solicitacao, proxima tentativa, lease, worker e contador ficam no PostgreSQL.
  O claim usa indice parcial apenas sobre audios prontos solicitados.
- A migration `0260` adiciona a previa quase ao vivo em
  `attendance_live_transcription_segments`: janelas duraveis de 25 segundos,
  sobreposicao util de 2,5 segundos, retry/lease independentes e merge em
  `attendance_recordings.live_transcript_text`. O texto integral produzido no
  encerramento continua autoritativo e e o unico enviado para analise.
