# AGENT — Customer Intelligence

## Escopo

Estas instruções valem para `back/internal/modules/customerintelligence`.

## Ownership

Customer Intelligence possui contexto inteligente, fontes allowlisted, observações, fatos,
sínteses, recomendações, prompts por processo, agentes/modelos/credenciais e runs de IA.

Não possui webhook, FSM, lease de conversa, mensagem, outbox de canal, adapter ou sender.
A IA produz `InteractionDecision`; somente o Omnichannel pode aceitar a proposta e criar
`PENDING` + outbox.

## Fonte de verdade e isolamento

- PostgreSQL, schema `intelligence`, é a fonte autoritativa.
- Toda entidade de negócio repete `account_id` e, quando aplicável, `client_account_id`.
- O service valida owner/client/relationship por authorizers pequenos; o repository repete o
  escopo nas queries.
- Request HTTP nunca define `account_id`; ele vem do Principal.
- Owner e `platform_admin` não têm bypass de permissões account-scoped. Permissões
  `*.platform_manage` são gates explícitos e exclusivos de `platform_admin`.
- Recurso fora do escopo retorna not found/forbidden conforme a borda, sem fallback cross-client.

## Runtime conversacional

- Entrada: `interaction.request.v1`.
- Saída válida: `interaction.decision.v1`.
- Pipeline inicial: `conversation.respond`, versão publicada resolvida antes da execução.
- Processos ativos nesta fase:
  - `conversation.triage`;
  - `conversation.reply`.
- Os onze processos headless possuem schemas v2 fechados e validação Go. O refresh de
  relacionamento executa e persiste cinco writers: `profile.summary`,
  `recommendation.follow_up`, `recommendation.offer`,
  `recommendation.important_dates` e `source.suggest`.
- `conversation.handoff_summary`, `memory.extract`, `portfolio.opportunity`,
  `media.image_analysis`, `media.document_analysis` e `quality.review` permanecem sem entrypoint
  e writer operacional; publicar prompt não os habilita implicitamente.
- Cada `ProcessRunRef` registra run, modo, process definition/config, binding, versões de todas as
  camadas de prompt, agent, model, context snapshot e output schema.
- `processRunRefs` é o nome wire canônico. Um resultado operacional exige lista não vazia.

### Falhas

Falha técnica nunca vira `no_reply` bem-sucedido. `RuntimeFailure` expõe somente:

- `disabled`;
- `not_authorized`;
- `invalid_input`;
- `timeout`;
- `temporarily_unavailable`;
- `invalid_result`;
- `budget_exceeded`;
- `permanent_failure`;
- `shadow_no_effect`.

O erro é seguro para log e não inclui prompt, input, output ou segredo. Consumidores usam
`RuntimeFailureDetails`; não analisam texto. `temporarily_unavailable`/`timeout` permitem retry
bounded. Configuração/schema inválidos seguem fail-open sem aceitar outcome.

### Shadow

`shadow` executa os prompts e persiste runs com `execution_mode=shadow`, mas devolve
`RuntimeFailureShadowNoEffect`. A decisão validada pode ser recuperada apenas para comparação por
`RuntimeShadowDecision`. Ela nunca deve chegar ao commit operacional.

### Prompt e dados não confiáveis

Mensagem, contexto, ERP, documento e tool output ficam exclusivamente no `UserPrompt` como JSON.
Placeholders do prompt de sistema são compilados para referências simbólicas
`user_payload.<campo>`; bytes do cliente nunca são interpolados no system prompt.

Prompts controlam semântica. Tenant, RBAC, schema, source/tool allowlist, consentimento, FSM,
idempotência e sender continuam invariantes Go/PostgreSQL.

## Ingestão

- Registry de sources é fechado em `catalog.go`.
- `source_ingestion_runs` é idempotente por
  `(account_id, client_account_id, idempotency_key)`.
- Busca/replay de run repete account + client.
- Jobs carregam IDs; adapters retornam observações allowlisted.
- Falha de source nunca bloqueia webhook ou resposta humana.

## Candidate claims

- Claim extraído por LLM entra somente como `status=candidate`,
  `verification_state=unverified` e `extraction_method=llm`.
- Outcome e claim repetem `account_id + client_account_id + subject_id + relationship_id`.
  A idempotência do outcome e da claim inclui o client; nunca deduplicar apenas por account.
- A outbox aceita somente descritores tipados da claim. O valor não é duplicado no evento:
  Customer Intelligence reidrata o output cifrado do `runtime_run` ativo e bem-sucedido usando
  todas as referências de processo/prompt/runtime antes de persistir a claim.
- `fact_key` e `value_type` precisam corresponder a uma definição publicada e ativa. Prompt não
  cria chave de fato nem altera o catálogo.
- Evidência só é vinculada quando a observação já existe no mesmo
  `account + client + subject + relationship`; IDs fora do escopo são descartados.
- `POST /v1/customer-intelligence/claims/{id}/review` aceita ou rejeita com optimistic revision.
  Aceitar muda somente o status da claim; não cria fato, não marca como verificado e nunca
  substitui valor manual/verificado. Materialização futura exige fluxo separado e explícito.
- `GET /v1/customer-intelligence/relationships/{relationshipId}/claims` lista claims no escopo
  do relacionamento e descriptografa o valor apenas na borda autorizada.

## Migrations

- `0242_intelligence_foundation.sql`: capabilities, sources, evidências, fatos, contexto,
  recomendações e auditoria.
- `0243_intelligence_prompt_runtime.sql`: registry de processos/prompts, agents/models/credentials,
  pipelines e runs.
- `0246_customer_intelligence_runtime_hardening.sql`: idempotência por client, refs de pipeline,
  modo shadow e schemas v2 fechados dos processos conversacionais.
- `0248_customer_intelligence_candidate_claims.sql`: idempotência de outcome por client, origem e
  revisão das candidate claims e FKs completas para outcome, prompt binding e runtime run.
- `0250_intelligence_observation_audit.sql`: constraints de classificação/escopo e evento
  metadata-only por observação ingerida.
- `0251_intelligence_observation_retention.sql`: binding de policy publicada, TTL fechado,
  tombstone/crypto-shred e worker de retenção metadata-only.
- `0252_customer_intelligence_headless_processes.sql`: schemas v2 fechados dos onze processos
  headless e pipeline publicado sem prompt/binding/agente habilitado implicitamente.
- `0253_customer_intelligence_headless_results.sql`: proveniência e idempotência dos cinco
  writers de refresh de relacionamento.
- `0254_intelligence_retention_governance.sql`: lifecycle obrigatório draft→publish com
  revisão/aprovação e legal hold imutável por observação.
- `0255_intelligence_context_snapshot_retention.sql`: crypto-shred idempotente do payload expirado
  de context snapshot, tombstone metadata-only e legal hold auditável por snapshot.

Migrations são append-only. Não editar
0242/0243/0246/0248/0250/0251/0252/0253/0254/0255 após aplicação.

## Validação mínima

```powershell
cd back
go test ./internal/modules/customerintelligence/...
go vet ./internal/modules/customerintelligence/...
golangci-lint run ./internal/modules/customerintelligence/...
go test ./internal/modules/omnichannel/...
go test ./internal/platform/app/...
```

Migration nova exige banco efêmero/cópia autorizada, aplicação repetida e teste de constraints.
`go test -race` exige CGO e compilador C no ambiente.

## Auditoria paginada

- `GET /v1/customer-intelligence/audit-events` devolve
  `{ "items": [...], "nextCursor": "..." }`; nao devolve mais um array nu.
- Os filtros server-side sao `action`, `entityType`, `occurredFrom`, `occurredTo`, `cursor` e
  `limit`. Datas usam RFC3339, o intervalo e inclusivo e o limite aceito e `1..200` (default 50).
- A ordem canonica e `occurred_at desc, id desc`. O cursor e base64url opaco, sem padding, e
  representa exatamente essa tupla; cliente nao deve interpretar nem construir o valor.
- Account/client, filtros temporais e filtros de tipo sempre sao aplicados no PostgreSQL antes do
  `LIMIT`. Eventos globais (`client_account_id is null`) continuam visiveis dentro da account.
- `Service.AuditEvents` e `FoundationRepository.ListAuditEvents` permanecem como compatibilidade
  interna; novas listagens HTTP usam `AuditEventPage`.

## Fontes owner-owned implementadas (2026-07-23)

- A composition root registra adapters para `calendar.client_profile`, `erp`,
  `site` e `bi.perola`; cada adapter consome uma fachada publica do modulo
  proprietario. Customer Intelligence nao faz SQL cross-module.
- `calendar.client_profile`: Business Context do `client_account_id`, somente
  `on_demand`, sem `subject_id` ou `relationship_id`.
- `erp` e `site`: Subject Evidence somente quando Customer Data devolve
  `source_link` deterministico para a mesma relacao. Sem link, o adapter nao
  consulta o owner; nome/fuzzy match sao proibidos.
- `bi.perola`: somente `on_demand`; valida configuracao no registry fechado do
  BI e permanece indisponivel com
  `deterministic_subject_link_unavailable` ate existir vinculo deterministico.
- `SourceDescriptor` fecha config, modos, capabilities e campos permitidos.
  Observacoes Business e Subject possuem escopos distintos; Business nunca
  herda o relacionamento recebido pelo job.
- Cada observacao ingerida usa exatamente a `purpose_key` da `source_config`; adapter nao pode
  ampliar finalidade. A leitura traduz a finalidade solicitada para uma allowlist fechada e o
  PostgreSQL filtra `purpose_key` antes de `ORDER BY/LIMIT`.
- `BuildContext` inclui observacoes allowlisted, separa a classificacao
  `customer_relationship|client_business_context`, aplica um unico budget de
  itens/tokens e recompõe provenance apos truncamento. A matriz de finalidade
  aceita `customer_service -> customer_service|customer_profile|customer_relationship`,
  `profile_view -> customer_profile|customer_relationship`; demais finalidades
  exigem igualdade exata. Mismatch e omitido com warning auditavel.
- Campos de snapshot usam as chaves canônicas `snake_case` do descriptor. Em especial,
  `omnichannel` e `manual.offline` não podem voltar a produzir camelCase incompatível com
  `safeKeyPattern` e a field allowlist.
- `GET /relationships/{relationshipId}/observations` exige `profile.view`, repete owner/client e
  relacionamento e omite classificação `restricted`. O detalhe `/observations/{id}` exige
  `audit.view`; restricted continua mascarado. Nenhuma resposta contém ciphertext,
  idempotency key ou payload upstream irrestrito.
- O endpoint de lista de observacoes exige capability `customer_intelligence.profile` e usa
  finalidade fixa `profile_view` antes do limite. `personal`, `sensitive` e `restricted`
  devolvem somente labels estruturais com `displayValue` mascarado no servidor. Reveal exige
  `audit.view`, motivo de catálogo fechado e gravação metadata-only bem-sucedida antes de
  devolver os campos allowlisted; falha da auditoria bloqueia a revelação.
- `provenanceRef`/`EvidenceRef.locator` nunca carregam `source_entity_id`. O service deriva
  `obsref:v1` por HMAC escopado em account, client, source e observation id usando chave separada
  por dominio da chave mestra. A mesma referencia opaca aparece no painel e no ContextEnvelope.
- Falhas permanentes tipadas encerram o run com `error_code` auditavel, sem
  retry. O run atualiza tambem a saude da source config. Nenhuma falha de fonte
  pode interromper a conversa omnichannel.

## Governança de retenção

- Configuração de fonte nunca cria nem publica policy implicitamente. Ela só vincula uma versão
  `published` que corresponda exatamente a key, TTL e ação; ausência retorna
  `retention_policy_approval_required`.
- Policy nasce exclusivamente por
  `POST /retention-policies/{policyKey}/drafts`. Publicação é uma mutação separada sob
  `customer_intelligence.sources.manage`, com `expectedRevision`, `reasonCode` e
  `approvalReference`. A versão publicada permanece imutável e não reponta fontes existentes.
- Legal hold vive em `intelligence.observation_legal_holds`, repete account+client+observation,
  preserva histórico e aceita apenas a transição auditada `active→released`.
- Scheduler e worker excluem hold ativo; trigger PostgreSQL impede tombstone/crypto-shred mesmo
  se um caminho de escrita futuro esquecer o filtro.
- `context_snapshots` expirados preservam IDs e metadados mínimos de proveniência, mas apagam
  ciphertext, versão de chave e hash, marcando `crypto_shredded + tombstoned_at`. O mesmo job
  legado de retenção drena observações e snapshots em lotes limitados e idempotentes.
- `context_snapshot_legal_holds` protege um snapshot explicitamente. Um hold de observação também
  protege snapshots do mesmo subject/relationship; contexto empresarial sob hold protege todos
  os snapshots daquele cliente. Locks transacionais e trigger mantêm a barreira sob concorrência.
