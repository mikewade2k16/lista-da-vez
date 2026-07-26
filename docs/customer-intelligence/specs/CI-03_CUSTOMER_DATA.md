# CI-03 — Customer Data

- **Status:** READY — implementação local autorizada
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** módulo `customer_data`
- **Dependências:** CI-00 validada; CI-01 em shadow; CI-02 aceita
- **Autoriza implementação:** sim; capabilities nascem desligadas

> Esta spec propõe a fronteira determinística. Não cria migrations nem módulo nesta rodada. Todo
> nome `<NEXT_*>` deve ser resolvido no despacho após reinspecionar o disco.

## 1. Resultado único e verificável

Criar um módulo determinístico, independente de UI e IA, que seja o writer único de:

- subjects e seus perfis determinísticos;
- relações por cliente;
- identidades canônicas multiorigem;
- source links;
- notas e consentimentos;
- candidates, merge e undo;
- segmentos CRM/marketing determinísticos, suas versões, avaliações, materializações e auditoria.

O Omnichannel continua dono de participante, mensagens, conversa, touchpoint bruto e sender.
Customer Data não copia mensagens/ERP e não executa LLM.

## 2. Decisão de boundary

Proposta que resolve `CI-DEC-002`, ainda DRAFT:

| Propriedade | Valor |
|---|---|
| módulo | `customer_data` |
| package | `back/internal/modules/customerdata` (`package customerdata`) |
| schema | `customer_data` |
| API | `/v1/customer-data/*` |
| UI completa | composta futuramente em `/inteligencia-clientes`; sem rota própria obrigatória nesta CI |
| Requires | `core` |
| Optional | `omnichannel`, `crm`, `site` |
| sort order | 44 |

Não usar `crm/customerdata`: o módulo `crm` atual possui semântica CRM Comercial/ERP, permissões e
rotas próprias. Customer Data precisa operar sem ERP e ser consumido por Omnichannel e
Customer Intelligence.

## 3. Ownership

| Entidade | Writer durante legacy | Writer após cutover |
|---|---|---|
| participante/identidade local de canal | Omnichannel | Omnichannel |
| subject/relação canônicos | inexistente | Customer Data |
| identidade multiorigem | CRM Omnichannel | Customer Data |
| lifecycle/tags/custom fields | `messaging.contacts` | Customer Data |
| notas/consentimentos/external refs | `messaging.contact_*` | Customer Data |
| merge/undo canônico | `messaging.contact_merge_events` | Customer Data |
| definição/versão/materialização de segmento | `messaging.contact_segments` apenas para definição legada | Customer Data |
| elegibilidade/exportação de segmento | inexistente | Customer Data; envio continua fora do módulo |
| mensagem/conversa/touchpoint bruto | Omnichannel | Omnichannel |
| fatos/sínteses/recomendações | memória Omnichannel | Customer Intelligence, não Customer Data |

`messaging.contacts` não é removida: permanece projeção/participante operacional, inclusive FK de
`messaging.conversations.contact_id`.

### 3.1 Leitura obrigatória para execução

- CI-00, CI-01 e CI-02 aprovadas;
- migrations/tabelas listadas na seção 9;
- `back/internal/modules/omnichannel/{crm_model.go,service_crm.go,store_crm.go,http_contacts_crm.go}`;
- module metadata/AGENTs de Core, Omnichannel, CRM/ERP e Site;
- `back/internal/platform/modules/*`, composition root e authorizer RBAC;
- front consumidor atual apenas para preservar o contrato da fachada;
- estado e contagens do banco alvo.

Leitura de consumidores adicionais é permitida. Escrita obedece exclusivamente à seção 11.

## 4. Estrutura do módulo

Arquivos candidatos:

```text
back/internal/modules/customerdata/
  AGENT.md
  module.go
  model.go
  errors.go
  service.go
  service_identity.go
  service_merge.go
  service_segments.go
  service_segment_evaluations.go
  service_segment_exports.go
  repository.go
  repository_postgres.go
  repository_subjects.go
  repository_relationships.go
  repository_identities.go
  repository_notes_consents.go
  repository_matching_merge.go
  repository_segments.go
  repository_segment_exports.go
  repository_writer_state.go
  segment_filter.go
  segment_field_catalog.go
  http.go
  http_subjects.go
  http_relationships.go
  http_identity.go
  http_matching.go
  http_segments.go
  http_segment_exports.go
  events.go
  worker.go
  worker_segments.go
```

Camadas obrigatórias:

```text
HTTP -> service/policy -> repository -> customer_data.*
source adapter -> service -> repository
```

Customer Data declara interfaces pequenas para o que consome. Adapters concretos vivem em
`back/internal/platform/app`. Nenhum repository consulta `messaging.*`, `erp.*` ou `site.*`.

## 5. Schema candidato

Todas as tabelas:

- usam schema qualificado;
- possuem `account_id uuid not null references core.accounts(id)`;
- possuem unique `(account_id,id)` quando referenciadas;
- repetem account/client nas FKs/índices de hot path;
- usam timestamps server-side;
- não possuem FK/cascade para schemas de módulos-fonte;
- não armazenam segredo em claro.

A ação de hard delete nas FKs de Core depende de `CI-DEC-016` (retenção, anonimização, legal hold e
backups). Nenhum executor escolhe `cascade`, `restrict` ou `set null` silenciosamente.

### 5.1 `customer_data.subjects`

| Coluna | Tipo/regra |
|---|---|
| `id` | uuid PK |
| `account_id` | uuid not null |
| `subject_type` | `person`, `organization` |
| `status` | `active`, `merged`, `anonymized` |
| `merged_into_subject_id` | uuid nullable, mesmo account |
| `revision` | bigint > 0 |
| `created_by_user_id`, `updated_by_user_id` | uuid nullable |
| `created_at`, `updated_at` | timestamptz |

Constraints:

- merge target diferente da origem;
- `status=merged` exige target; outros estados não usam target;
- prevenção de ciclo no service transacional;
- FK composta `(account_id,merged_into_subject_id)`.

Índices:

- `(account_id,status,updated_at desc,id)`;
- `(account_id,subject_type,status,updated_at desc,id)`;
- `(account_id,merged_into_subject_id)` partial.

### 5.2 Perfis tipados

`customer_data.subject_person_profiles`:

- PK/FK composta account + subject;
- `legal_name`, `preferred_name`, `birth_date`, `locale`, `timezone`;
- verification source ref/at;
- revision/timestamps.

`customer_data.subject_organization_profiles`:

- PK/FK composta account + subject;
- `legal_name`, `trade_name`, `registration_country`;
- `registration_id_ciphertext`, `registration_id_fingerprint`, `key_version`;
- verification source ref/at;
- revision/timestamps.

Somente subject do tipo correspondente recebe o perfil; o service valida. Campos owner-scoped não
aparecem automaticamente em DTO de client.

### 5.3 `customer_data.relationships`

| Coluna | Tipo/regra |
|---|---|
| `id` | uuid PK |
| `account_id`, `client_account_id`, `subject_id` | uuid not null |
| `display_name` | text not null, bounded |
| `preferred_name` | text nullable |
| `lifecycle_status` | `lead`, `prospect`, `customer`, `inactive` |
| `classification_source` | `manual`, `erp`, `rule`, `backfill` |
| `classification_confidence` | numeric 0..1 nullable |
| `owner_user_id` | uuid nullable |
| `tags` | jsonb array bounded |
| `custom_fields` | jsonb object validado por schema |
| `first_seen_at`, `last_seen_at`, `last_qualified_at` | timestamptz nullable |
| `archived_at` | timestamptz nullable |
| `revision` | bigint > 0 |
| actor/timestamps | auditáveis |

Constraints/índices:

- unique `(account_id,client_account_id,subject_id)`;
- unique `(account_id,client_account_id,id)` e
  `(account_id,client_account_id,subject_id,id)` para FKs tenant/client-scoped;
- FK composta account+subject;
- FKs owner/client para Core;
- `(account_id,client_account_id,archived_at,updated_at desc,id)`;
- `(account_id,client_account_id,lifecycle_status,last_seen_at desc,id)`;
- GIN em tags/custom fields somente após query plan e allowlist de filtros; não criar por hábito.

### 5.4 `customer_data.subject_identities`

| Coluna | Tipo/regra |
|---|---|
| IDs | account, client, relationship, subject |
| `identity_kind` | enum da CI-02 |
| `issuer` | source/provider registrado |
| `value_ciphertext` | text nullable |
| `value_fingerprint` | text not null, HMAC |
| `key_version` | text not null |
| `masked_value` | text not null |
| `verification_status` | `unverified`, `verified`, `revoked` |
| `verification_method` | enum allowlisted |
| `source_ref_type`, `source_ref_id` | referência tipada |
| `metadata` | jsonb allowlisted por kind |
| first/last/verified timestamps | origem real |
| `revision`, actor/timestamps | optimistic/audit |

Constraints:

- FK composta
  `(account_id,client_account_id,subject_id,relationship_id)` garante que identity, subject e
  relationship pertencem ao mesmo account/client;
- unique `(account_id,client_account_id,identity_kind,issuer,value_fingerprint)` para identidade
  ativa;
- valor revogado continua auditável e não resolve match;
- nenhum índice contém telefone/e-mail/documento em claro.

### 5.5 `customer_data.subject_source_links`

Colunas:

- id, account, client, subject, relationship;
- `source_module`, `source_key`, `source_entity_type`, `source_entity_id`;
- `source_version`, `source_hash`;
- `link_method`, `match_confidence`, `status`;
- `idempotency_key`;
- linked/reviewed actor e timestamps.

Unique:

- `(account_id,client_account_id,source_module,source_entity_type,source_entity_id)`;
- `(account_id,idempotency_key)`.

FK composta para relationship repete account, client e subject.

O primeiro link é `messaging.contact`. IDs são referências, não FKs cross-module.

### 5.6 `customer_data.relationship_notes`

- id, account, client, relationship;
- `content` bounded;
- `context_source_module/entity_type/entity_id` opcionais;
- author user;
- `revision`, `archived_at`, timestamps.

Não existe FK para conversation. Edição atualiza revision e gera audit; archive substitui delete
comum.

FK `(account_id,client_account_id,relationship_id)` impede nota cross-client.

Índice: `(account_id,client_account_id,relationship_id,created_at desc,id)`.

### 5.6.1 `customer_data.offline_interactions`

Registro determinístico de reunião, ligação, conversa fora do canal, visita ou nota importada:

- `id`, `account_id`, `client_account_id`, `relationship_id`;
- `interaction_type=meeting|call|offline_chat|visit|note|other`;
- `occurred_at`, `timezone`, `duration_seconds` nullable;
- `title` bounded;
- `content_sanitized` somente para `public|internal`, ou `content_ciphertext` +
  `cipher_key_version` para `personal|sensitive|restricted`;
- `sensitivity`, `purpose_key`, `source_external_ref` nullable;
- `status=active|archived`, `revision`;
- `created_by_user_id`, `updated_by_user_id`, timestamps;
- `idempotency_key` obrigatório.

Unique `(account_id,client_account_id,idempotency_key)`. FK
`(account_id,client_account_id,relationship_id)` e índice de timeline impedem mistura de cliente.
Editar cria audit/outbox; não reescreve observação inteligente já aceita — emite nova versão para
supersede/rebuild.

### 5.6.2 `customer_data.offline_interaction_attachments`

Guarda apenas metadados e referência ao storage privado:

- id/account/client/offline interaction;
- `storage_object_ref` opaca, `original_name_masked`, MIME allowlisted, bytes e hash;
- `scan_status=pending|clean|blocked|failed`;
- sensibilidade/finalidade, uploader e timestamps;
- unique account + storage ref e FK composta para a interação.

Arquivo nunca entra em JSON/base64/PostgreSQL nem fica disponível antes de antivírus/MIME/size.
Análise por IA exige `scan_status=clean`, capability de mídia, finalidade e source binding próprios.

### 5.7 `customer_data.relationship_consents`

- id, account, client, relationship;
- `purpose`, `channel`, `status=granted|revoked|unknown`;
- source module/ref e evidence hash;
- `effective_at`, `expires_at`;
- actor/timestamps;
- `idempotency_key`.

Cada mudança é nova linha/evento; “estado atual” é projeção pela data/precedência. Revogar não
apaga concessão histórica.

FK `(account_id,client_account_id,relationship_id)` impede consentimento cross-client.

Índice: `(account_id,client_account_id,relationship_id,purpose,channel,effective_at desc,id)`.

### 5.8 `customer_data.match_candidates`

Campos do contrato CI-02 mais:

- incoming source key/type/id/version;
- candidate subject/relationship;
- method/confidence/risk flags/evidence refs;
- status/decision/reviewer;
- expires/revision/timestamps;
- idempotency.

Índices:

- fila `(account_id,client_account_id,status,created_at,id)`;
- candidate `(account_id,candidate_subject_id,status)`;
- unique account + idempotency key.

### 5.9 `customer_data.merge_events`

- id, account;
- source/target subject;
- affected relationship IDs allowlisted;
- reason, actor, idempotency;
- snapshot sanitizado/cifrado conforme policy;
- `event_kind=merge|undo`;
- `reverses_event_id` nullable;
- createdAt.

Unique account + idempotency. FKs compostas no mesmo account. Eventos são imutáveis.

### 5.10 Segmentação CRM/marketing

O domínio preserva a capacidade atual de filtro salvo, mas separa identidade estável, regra
versionada, execução, membership derivada e exportação. Todo segmento pertence a exatamente um
`account_id + client_account_id`, ambos `not null`. Em standalone, `client_account_id=account_id`.
Agência exige um cliente ativo e acessível; não existe segmento individual cross-client. Análises
agregadas de portfólio permanecem em CI-09.

#### 5.10.1 `customer_data.segments`

Registro estável de negócio:

| Coluna | Tipo/regra |
|---|---|
| `id` | uuid PK; identidade estável |
| `account_id`, `client_account_id` | uuid not null; scope imutável |
| `segment_key` | text normalizado, estável e nunca reutilizado no mesmo client |
| `name`, `description` | text bounded; nome pode mudar |
| `status` | `active`, `archived` |
| `active_version_id` | uuid nullable; sempre versão publicada do mesmo segmento/scope |
| `current_materialization_id` | uuid nullable; snapshot corrente do mesmo segmento/scope |
| `revision` | bigint > 0 para edição concorrente do registro estável |
| owner/actor/timestamps | criador, owner opcional, updater, archive e audit |

Unique `(account_id,client_account_id,segment_key)`. Arquivar não apaga versão, run, membership ou
export histórico. Trocar client ou reutilizar `segment_key` arquivada é proibido; cria-se outro
segmento.

#### 5.10.2 `customer_data.segment_versions`

Cada mudança de comportamento possui versão própria:

| Coluna | Tipo/regra |
|---|---|
| IDs | id, account, client e segment por FK composta |
| `version_number` | inteiro crescente por segmento |
| `status` | `draft`, `validated`, `published`, `archived` |
| `filter_schema_version` | inicialmente `segment.filter.v1` |
| `field_catalog_version` | snapshot lógico do catálogo usado na validação |
| `filter_ast` | JSONB validado pelo schema fechado da seção 5.10.3 |
| `evaluation_policy` | JSONB tipado: schedule, freshness, budgets e comportamento parcial |
| `definition_hash` | hash canônico da regra/policy |
| `revision` | optimistic lock enquanto mutável |
| lifecycle | change summary, actors e timestamps de create/validate/publish/archive |

Unique `(account_id,client_account_id,segment_id,version_number)` e no máximo um draft aberto por
segmento. Editar depois de validar retorna a versão a `draft` e invalida a validação anterior.
Publicar exige validação vigente e torna AST, policy, catálogo, hash e número imutáveis. Não existe
PATCH/DELETE de versão publicada. Rollback atualiza `segments.active_version_id` para versão
publicada anterior e registra motivo/audit; nunca reescreve a versão. Um novo draft parte por cópia
explícita e recebe novo ID/número.

#### 5.10.3 `segment.filter.v1` e catálogo de campos

Envelope lógico:

```json
{
  "schemaVersion": "segment.filter.v1",
  "root": {
    "type": "group",
    "operator": "and",
    "children": [
      {
        "type": "predicate",
        "fieldKey": "relationship.lifecycle_status",
        "operator": "in",
        "value": ["lead", "prospect"]
      }
    ]
  }
}
```

Gramática v1:

- node `group` aceita somente `and|or` e filhos não vazios;
- node `predicate` aceita somente `fieldKey`, `operator` e valor compatível;
- operadores comuns: `eq`, `neq`, `in`, `not_in`, `exists`, `not_exists`;
- operadores adicionais são allowlisted por tipo: string (`contains`, `prefix`), número
  (`gt`, `gte`, `lt`, `lte`, `between`), data (`before`, `after`, `between`, `within_last`) e
  boolean (`is_true`, `is_false`);
- `within_last` usa exclusivamente o `asOf` gravado no run; nunca depende de relógio implícito no
  meio da paginação;
- null, normalização, timezone e case sensitivity são definidos pelo descriptor do campo.

`SegmentFieldDefinition` é registry server-side versionado e contém `fieldKey`, owner/source,
data type, operadores, schema do valor, classificação/sensibilidade, finalidades permitidas,
freshness, modo de resolução e capacidade de preview/materialização. O AST nunca aceita nome de
tabela/coluna, SQL, fragmento de WHERE, JSONPath, URL, tool, template, regex livre ou expressão.
Profundidade, número de nodes, tamanho de strings/listas e custo estimado possuem hard caps
server-side.

Campos locais compilam somente para query parametrizada sobre o repository de Customer Data.
Campo externo exige resolver registrado por interface tipada, que devolve IDs no mesmo
account/client e source snapshot/freshness; o repository não consulta schema de outro módulo.
Field/operator desconhecido, valor incompatível, source ausente ou estimativa acima do budget
falha fechado. O query plan/SQL compilado não é exposto na API nem persistido em log.

IA de sugestão de filtro não faz parte desta entrega. Essa capability permanece **BLOCKED** até
nova decisão que aprove `process_key`, schemas, prompts, evals, thresholds, permissões e rollout.
O builder manual é determinístico e não chama Customer Intelligence.

#### 5.10.4 `customer_data.segment_evaluation_runs`

Toda avaliação, inclusive preview, cria um run auditável:

| Grupo | Campos/regras |
|---|---|
| scope | id, account, client, segment e version por FKs compostas |
| execução | mode em `preview`, `materialize`, `recompute`; trigger em `manual`, `schedule`, `source_change`, `backfill` |
| estado | `queued`, `running`, `completed`, `partial`, `failed`, `cancelled` |
| determinismo | `as_of`, definition/input fingerprint, field catalog version e source snapshot refs |
| resultado | matched/excluded/error counts, bounded sample count e reason codes sanitizados |
| controle | idempotency key, budget/cost class, attempts, lease owner/expiry, cancel request |
| auditoria | requested/started/finished actors e timestamps, correlation/causation IDs |

Unique `(account_id,client_account_id,idempotency_key)`. Claim usa lease e retry classificado;
replay retorna o mesmo run. Preview aceita draft somente depois de validar seu AST, nunca grava
membership e retorna contagem mais amostra mascarada/bounded conforme permissões cumulativas.
Materialização/recompute aceita apenas versão publicada. Fonte indisponível não vira zero
silencioso: o run fica `partial` ou `failed` segundo a policy publicada e informa freshness/reason
code. Nenhum run executa LLM nem sender.

#### 5.10.5 Materializações e memberships

`customer_data.segment_materializations` registra snapshot derivado:

- id/account/client/segment/version/evaluation run por FKs compostas;
- `as_of`, definition/input fingerprint, field catalog/source snapshot refs;
- `status=building|current|superseded|expired|failed`;
- member count, expiry/freshness e timestamps;
- unique por run e fingerprint de entrada.

`customer_data.segment_memberships` contém somente account, client, materialization, segment,
version, relationship e subject IDs, match fingerprint e `matched_at`. FKs compostas garantem que a
relação pertence ao mesmo client; unique por materialization+relationship. Não copia telefone,
e-mail, nome, mensagem, consentimento ou payload de fonte.

Uma materialização completa é publicada atomicamente como `current`; a anterior vira
`superseded`. Falha preserva o snapshot corrente. Membership é projeção reconstruível e não pode
ser editada manualmente. Rollback de versão requer materialização compatível ainda fresh ou agenda
novo run antes de trocar o snapshot corrente.

#### 5.10.6 Exportação e consentimento separados

Membership significa apenas “a regra correspondeu em `asOf`”. Não significa opt-in, base legal,
elegibilidade de canal nem autorização de campanha. Exportação usa workflow próprio:

`customer_data.segment_exports`:

- id/account/client/segment/version/materialization;
- `purpose_key`, `channel`, `format_key`, `field_set_key` e policy refs allowlisted;
- status `requested|evaluating|ready|denied|expired|failed|cancelled`;
- candidate/eligible/excluded counts e breakdown por reason code;
- idempotency, requester/approver, correlation e timestamps;
- `storage_object_ref` privada, hash, bytes e `expires_at`; nunca URL pública.

`customer_data.segment_export_items`:

- account/client/export/relationship por FK composta;
- `eligibility=eligible|excluded`;
- reason code, consent event/policy refs e `evaluated_at`;
- sem identidade ou PII copiada.

O job resolve a projeção de contato somente ao gerar o objeto privado, revalida consentimento atual
por finalidade/canal, opt-out, status da relação, freshness e fields permitidos. Download intent é
curto, auditado, exige nova autorização e invalida/regenera o artefato quando consentimento ou
materialização ficou stale. Em `customer_data.segment_exports=shadow`, calcula apenas o relatório
de elegibilidade e nunca cria objeto baixável.

Exportar não envia, não agenda campanha e não chama provider. Um módulo de campanha ou
Omnichannel que venha a consumir a exportação mantém sua própria permissão e revalida novamente
consentimento, janela/template/capacidade antes de qualquer mensagem; o único caminho de envio
continua mensagem `PENDING` + outbox + sender do Omnichannel.

Produção de export depende das decisões jurídicas `CI-DEC-015`/`CI-DEC-016` e de policy aprovada de
retenção/TTL/field sets. Sem elas, capability permanece `off` ou `shadow`.

#### 5.10.7 Constraints e índices mínimos

- todas as FKs entre segment/version/run/materialization/membership/export repetem
  `(account_id,client_account_id)`;
- ponteiro ativo/corrente só aceita filho do mesmo segmento e scope;
- índices de lista usam account+client+status+updated/created+id;
- claim de run/export usa account+status+run_after e `skip locked`;
- members usam account+client+materialization+relationship e cursor estável;
- índices de predicate/GIN não são genéricos: cada field local precisa de query plan e pacote DDL
  explícito;
- nenhum índice contém PII crua.

### 5.11 `customer_data.inbox_events`

Consumo idempotente de eventos duráveis:

- event id + account como chave;
- topic/schema version;
- aggregate/client IDs;
- status `received|processing|done|failed|dead`;
- attempts, next run, error class sanitizada;
- received/processed timestamps.

Payload bruto não é memória. Guardar somente envelope ID-only e buscar o owner por adapter.

### 5.12 `customer_data.writer_states`

Controla um writer por client/entidade:

| Campo | Regra |
|---|---|
| account/client/entity key | unique |
| `mode` | `legacy`, `shadow`, `new` |
| `watermark` | cursor opaco |
| `source_checksum`, `target_checksum` | comparação |
| `approved_by_user_id`, `approved_at` | exigidos em `new` |
| `revision`, timestamps | optimistic/audit |

Entidades mínimas: `relationship`, `identity`, `note`, `consent`, `merge`,
`segment_definition`. Materialização/exportação não têm writer legado; permanecem `off` até a
definição estar em `new`.

### 5.13 `customer_data.outbox_events`

Publicação durável na mesma transação de Customer Data:

- event/account/client/aggregate IDs;
- topic e schema version;
- payload ID-only;
- correlation/causation/idempotency;
- status `pending|processing|done|failed|dead`;
- attempts/lease/run-after/error code sanitizado;
- timestamps.

Unique `(account_id,idempotency_key)` e índice de claim por status/run-after. Se uma outbox
durável compartilhada de plataforma for aprovada antes da implementação, CI-03 deve registrar ADR
e usar exatamente uma delas; nunca publica crítico somente no `InMemoryBus`.

### 5.14 `customer_data.audit_events`

Campos: event/account/client/subject/relationship, actor type/id, action, entity type/id, old/new
hash, reason, correlation, timestamp. Sem segredo ou PII bruta.

## 6. Domínio e ports

### 6.1 Interfaces exportadas

```go
type Service interface {
    ListSubjects(ctx context.Context, scope Scope, filter SubjectFilter) (SubjectPage, error)
    GetProfile(ctx context.Context, scope Scope, relationshipID string) (DeterministicProfile, error)
    ResolveSubject(ctx context.Context, req ResolveSubjectRequest) (ResolveSubjectResult, error)
    UpdateRelationship(ctx context.Context, scope Scope, id string, patch RelationshipPatch) (Relationship, error)
    RecordConsent(ctx context.Context, scope Scope, req ConsentInput) (Consent, error)
    ListSegments(ctx context.Context, scope Scope, filter SegmentListFilter) (SegmentPage, error)
    RequestSegmentEvaluation(ctx context.Context, scope Scope, req SegmentEvaluationRequest) (SegmentEvaluationRun, error)
    RequestSegmentExport(ctx context.Context, scope Scope, req SegmentExportRequest) (SegmentExport, error)
}
```

Interfaces efetivas devem ser menores por consumidor; este bloco é catálogo de capabilities, não
obrigação de uma interface gigante.

### 6.2 Portas de entrada

- HTTP autenticado;
- `ChannelParticipantSource` adapter do Omnichannel;
- `ERPSubjectSource` adapter do CRM/ERP;
- `SiteLeadSource` adapter do Site;
- importação manual autorizada;
- consumidor de evento durável.

### 6.3 Portas de saída

Customer Data não escreve em módulos fonte. Pode publicar eventos ID-only:

- `customer_data.subject.resolved`;
- `customer_data.relationship.changed`;
- `customer_data.consent.changed`;
- `customer_data.identity.changed`;
- `customer_data.merge.completed`;
- `customer_data.merge.undone`;
- `customer_data.segment.version_published`;
- `customer_data.segment.materialized`;
- `customer_data.segment.export_ready`.

O source owner aplica qualquer correção pedida por sua própria tool/service; não há UPDATE
cross-schema.

## 7. DTOs e APIs

Convenções:

- camelCase;
- sem `tenantId` legado nas APIs novas;
- account nunca vem no body;
- `clientAccountId` é obrigatório em agência e omitível somente em standalone;
- paginação cursor-based com cap;
- `revision`/`expectedRevision` em mutações;
- idempotency key em create/consent/merge/review/evaluate/materialize/export;
- fora do escopo → 404.

### 7.1 Lista de subjects

`GET /v1/customer-data/subjects`

Filtros:

- `clientAccountId`;
- `q` bounded;
- `subjectType`;
- `lifecycleStatus`;
- `tag`;
- `ownerUserId`;
- `archived`;
- `updatedAfter`;
- `cursor`, `limit`.

Permissões base cumulativas: `customer_data.subjects.view` +
`customer_data.relationships.view`. `primaryIdentities` só é incluído quando o principal também
possui `customer_data.identities.view`; sem ela, o backend omite o campo e informa a capability
sanitizada correspondente. O frontend não recebe identidade para “esconder” depois.

Item:

```json
{
  "subjectId": "uuid",
  "subjectType": "person",
  "relationship": {
    "id": "uuid",
    "clientAccountId": "uuid",
    "displayName": "string",
    "preferredName": "string|null",
    "lifecycleStatus": "lead",
    "tags": [],
    "ownerUserId": null,
    "firstSeenAt": "RFC3339|null",
    "lastSeenAt": "RFC3339|null",
    "revision": 1,
    "updatedAt": "RFC3339"
  },
  "primaryIdentities": [{
    "id": "uuid",
    "kind": "whatsapp",
    "maskedValue": "***1234",
    "verificationStatus": "verified"
  }]
}
```

Não retorna outras relações do subject.

### 7.2 Criar subject/relação manual

`POST /v1/customer-data/subjects`

```json
{
  "clientAccountId": "uuid",
  "subjectType": "person",
  "profile": {
    "preferredName": "string",
    "locale": "pt-BR",
    "timezone": "America/Sao_Paulo"
  },
  "relationship": {
    "displayName": "string",
    "lifecycleStatus": "lead",
    "tags": [],
    "customFields": {}
  },
  "identities": [],
  "idempotencyKey": "string"
}
```

Permissões cumulativas conforme conteúdo: `customer_data.subjects.manage`,
`customer_data.relationships.manage` e `customer_data.identities.manage`.

### 7.3 Detalhe determinístico

`GET /v1/customer-data/relationships/{relationshipId}/profile`

Resposta:

- subject mínimo;
- somente perfil owner-scoped permitido;
- relationship;
- identities, somente com `customer_data.identities.view`;
- source links;
- notes page, somente com `customer_data.notes.view`;
- offline interactions/attachment metadata, somente com
  `customer_data.offline_interactions.view`;
- consents atuais + histórico paginado, somente com `customer_data.consents.view`;
- referências a touchpoints, sem mensagens;
- writer state/compatibility somente para administrador.

Não inclui fatos, resumo ou recomendação; a workspace compõe Customer Intelligence separadamente.
O endpoint exige `customer_data.subjects.view` + `customer_data.relationships.view` e faz
field/section-level authorization no backend. Se a UI precisar que todas as seções estejam
presentes, ela solicita as permissões cumulativas; ausência de uma permissão nunca vaza o campo com
valor vazio, mascarado ou redigido que confirme a existência do dado.

### 7.4 Atualizar subject

`PATCH /v1/customer-data/subjects/{subjectId}`

Campos allowlisted por subject type e `expectedRevision`. Alterar type não é suportado por PATCH.
Permissão `customer_data.subjects.manage`. Subject sem relação acessível retorna 404.

### 7.5 Atualizar relação

`PATCH /v1/customer-data/relationships/{relationshipId}`

Campos:

- display/preferred name;
- lifecycle;
- owner;
- tags/custom fields;
- archive;
- expected revision.

`classificationSource` é derivado do ator/fluxo, não aceito livremente.

### 7.6 Identidades

| Método | Rota | Ação |
|---|---|---|
| GET | `/relationships/{id}/identities` | listar projeção mascarada |
| POST | `/relationships/{id}/identities` | adicionar identidade |
| POST | `/identities/{id}/verify` | registrar verificação/evidência |
| POST | `/identities/{id}/revoke` | revogar sem apagar histórico |

Valor cru pode entrar por TLS no POST e é cifrado/fingerprinted no service; nunca retorna da API,
fica em log ou store do frontend.

### 7.7 Notas

| Método | Rota |
|---|---|
| GET | `/relationships/{id}/notes` |
| POST | `/relationships/{id}/notes` |
| PATCH | `/notes/{id}` |
| POST | `/notes/{id}/archive` |

Body aceita content bounded, context source ref tipada, expected revision/idempotency quando
aplicável.

### 7.7.1 Interações offline e importação

| Método | Rota | Permissão |
|---|---|---|
| GET | `/relationships/{id}/offline-interactions` | `customer_data.offline_interactions.view` |
| POST | `/relationships/{id}/offline-interactions` | `customer_data.offline_interactions.manage` |
| PATCH | `/offline-interactions/{id}` | `customer_data.offline_interactions.manage` |
| POST | `/offline-interactions/{id}/archive` | `customer_data.offline_interactions.manage` |
| POST | `/offline-interactions/{id}/attachment-upload-intents` | `customer_data.offline_interactions.manage` |
| POST | `/offline-attachments/{id}/complete` | `customer_data.offline_interactions.manage` |
| POST | `/offline-interaction-imports` | `customer_data.offline_interactions.import` |
| GET | `/offline-interaction-imports/{id}` | `customer_data.offline_interactions.import` |
| POST | `/offline-interaction-imports/{id}/apply` | `customer_data.offline_interactions.import` |

Criação recebe `clientAccountId`, tipo, `occurredAt`, timezone, título/conteúdo classificados,
purpose, refs de anexos já emitidas e `idempotencyKey`. Account/relationship vêm do Principal e da
rota; fora do escopo retorna 404.

Upload intent limita MIME/bytes e gera referência privada de TTL curto. `complete` apenas agenda
scan server-side; o cliente nunca informa `clean`. CSV usa storage ref já escaneada, mapping
versionado e `dryRun`; apply exige relatório revisado, revision e idempotency. Linha ambígua fica
quarantined e nunca faz merge automático.

O commit cria outbox durável `customer_data.offline_interaction.changed.v1`. Customer Intelligence
consome por adapter quando a source manual estiver habilitada; falha/ausência da Inteligência não
impede registrar nem consultar a interação offline.

### 7.8 Consentimentos

| Método | Rota |
|---|---|
| GET | `/relationships/{id}/consents` |
| POST | `/relationships/{id}/consents` |

POST registra um novo evento com purpose, channel, status, source/evidence, effective/expires e
idempotency. Nunca altera linha histórica.

### 7.9 Matching e merge

Rotas definidas em CI-02:

- GET candidates/list/detail;
- POST decision;
- POST subject merge;
- POST merge undo.

Permissão `customer_data.merge.manage`.

### 7.10 Segmentos CRM/marketing

Todas as rotas abaixo ficam sob `/v1/customer-data`, exigem `customer_data` habilitado,
`customer_data.segmentation != off`, account permission efetiva e client scope revalidado:

Na tabela, `segments.*` abrevia exclusivamente `customer_data.segments.*`; não é uma segunda
família de permissions.

| Método | Rota | Permissão cumulativa | Resultado |
|---|---|---|---|
| GET | `/segment-fields` | `segments.view` | catálogo versionado de fields/operators permitido no client |
| GET | `/segments` | `segments.view` | lista cursor-based por client/status |
| POST | `/segments` | `segments.manage` | registro estável + primeiro draft |
| GET | `/segments/{segmentId}` | `segments.view` | definição, binding ativo e capabilities efetivas |
| PATCH | `/segments/{segmentId}` | `segments.manage` | metadata/status com `expectedRevision` |
| POST | `/segments/{segmentId}/archive` | `segments.manage` | archive auditado, sem delete |
| GET | `/segments/{segmentId}/versions` | `segments.view` | histórico e metadata; AST conforme permissão |
| POST | `/segments/{segmentId}/versions` | `segments.manage` | novo draft, opcionalmente baseado em versão publicada |
| PATCH | `/segment-versions/{versionId}` | `segments.manage` | altera somente draft com `expectedRevision` |
| POST | `/segment-versions/{versionId}/validate` | `segments.manage` | schema/catalog/custo/compatibilidade |
| POST | `/segment-versions/{versionId}/preview` | `segments.view` + `segments.evaluate` | cria evaluation run sem membership |
| POST | `/segment-versions/{versionId}/publish` | `segments.manage` + `segments.publish` | publica versão imutável |
| POST | `/segments/{segmentId}/rollback` | `segments.publish` | troca binding para versão publicada anterior |
| POST | `/segments/{segmentId}/materializations` | `segments.view` + `segments.evaluate` | cria run para versão publicada |
| GET | `/segment-evaluation-runs/{runId}` | `segments.view` | estado/contagens/freshness/diagnóstico sanitizado |
| GET | `/segments/{segmentId}/materializations` | `segments.view` | snapshots paginados |
| GET | `/segment-materializations/{materializationId}/members` | ver regra abaixo | IDs/projeção mínima, cursor-based |
| POST | `/segment-exports` | `segments.view` + `segments.export` | cria workflow de elegibilidade/export |
| GET | `/segment-exports/{exportId}` | `segments.view` + `segments.export` | status/counts/reason codes/expiry |
| POST | `/segment-exports/{exportId}/download-intents` | `segments.export` | intent privada curta após revalidação |
| POST | `/segment-exports/{exportId}/cancel` | `segments.export` | cancela quando o estado permitir |

`clientAccountId` é obrigatório na listagem, catálogo e criação para agência; em standalone o
service normaliza para account. Rotas por ID resolvem o recurso no mesmo scope e retornam 404 fora
dele. Nenhuma rota aceita `accountId`, SQL, coluna, JSONPath, URL, expressão ou field definition
fornecida pelo cliente.

Criação:

```json
{
  "clientAccountId": "uuid",
  "segmentKey": "leads-inativos",
  "name": "Leads inativos",
  "description": "string",
  "draft": {
    "filterSchemaVersion": "segment.filter.v1",
    "fieldCatalogVersion": "opaque-version",
    "filterAst": {"schemaVersion": "segment.filter.v1", "root": {}},
    "evaluationPolicy": {}
  },
  "idempotencyKey": "opaque"
}
```

Campos extras falham. O backend seleciona o catálogo efetivo; uma versão enviada pelo frontend é
apenas precondition e não autoriza field antigo/revogado. Create/PATCH retorna `segmentId`,
`versionId`, status, revision, validation state e capabilities derivadas.

Preview/materialização retornam `202` com:

```json
{
  "evaluationRunId": "uuid",
  "mode": "preview",
  "status": "queued",
  "asOf": "RFC3339",
  "definitionHash": "hex",
  "pollAfterMs": 1000
}
```

O GET do run retorna counts, source statuses/freshness, partial/error reason codes e, somente para
preview concluído, amostra bounded. Exibir nome/perfil da amostra exige cumulativamente
`subjects.view + relationships.view`; sem isso retorna apenas count. Identidade nunca entra sem
`identities.view` e permanece mascarada. O endpoint de members exige
`segments.view + subjects.view + relationships.view` e não devolve identidade por default.

Publish recebe `expectedRevision`, validation ID/hash, motivo e idempotency key. Rollback recebe
target published version, expected segment revision, motivo e idempotency. Materialização recebe
published `versionId`, `asOf` opcional bounded e idempotency; schedule não é criado pelo browser, é
derivado da `evaluation_policy` publicada.

Export recebe materialization, purpose/channel, `formatKey`, `fieldSetKey`, motivo, policy
preconditions e idempotency. Field set é catálogo fechado e nunca array livre de colunas. O backend
aplica consentimento mesmo se o chamador não possuir `consents.view`; essa permissão adicional
apenas permite ver detalhes de consentimento autorizados. Resposta nunca contém objeto/URL antes de
`ready` e download intent, nunca retorna PII excluída e nunca inicia envio.

### 7.11 Auditoria

`GET /v1/customer-data/audit`

Filtros allowlisted por client/subject/relationship/action/time, cursor e cap. Permissão
`customer_data.audit.view`. Payload antigo/novo é hash/projeção, não PII integral.

## 8. Escopo, autorização e módulos

1. middleware autentica e valida account ativa;
2. module guard exige `customer_data`;
3. authorizer resolve permissão efetiva naquela account;
4. service deriva standalone/agência e valida client no catálogo;
5. service valida subject/relationship/client;
6. repository repete account + client;
7. response projeta somente campos permitidos.

Customer Intelligence recebe apenas `DeterministicProfile` pelo adapter/purpose; não ganha acesso
ao repository.

Para segmentos, o mesmo fluxo é repetido em definição, versão, run, materialização, membership e
export. O account/client do segmento é resolvido antes do field catalog/compilação, incorporado à
query pelo repository e nunca pode ser removido, negado ou substituído pelo AST.

## 9. Migração do CRM atual

### 9.1 Mapeamento

| Legado | Destino/projeção |
|---|---|
| `messaging.contacts` | permanece participante; gera subject source link |
| name/phone/avatar/seen/channel | permanecem projeção operacional; Customer Data pode compor leitura |
| relationship status/tags/custom fields/email/owner | `customer_data.relationships`/identities |
| `contact_identities` | provider identity + source link |
| `contact_notes` | relationship notes |
| `contact_consents` | relationship consents |
| `contact_external_refs` | identities/source links |
| `contact_merge_events` | merge events após revisão |
| `contact_segments` | registro estável + versão draft/publicada somente após tradução validada |
| `contact_touchpoints` | permanece bruto; leitura por adapter/projeção |
| `contact_intelligence` | não migra para Customer Data |

### 9.2 Elegibilidade de backfill

Depende do snapshot client da CI-01:

| Classe | Ação |
|---|---|
| contato com exatamente um client resolvido | pode criar subject/relação/link |
| contato standalone resolvido | client=account |
| contato com mais de um client | quarentena/revisão CI-02 |
| contato sem client | permanece legacy; relatório |
| identidade conflitante | candidate/quarentena |
| merge legado envolvendo classes diferentes | revisão manual |

IDs antigos não viram automaticamente subject IDs. `subject_source_links` mantém a tradução.

Segmentos legados exigem classificação própria porque `messaging.contact_segments` possui
`account_id`, mas não `client_account_id`:

| Caso legado | Ação |
|---|---|
| account standalone | client=account; traduzir filtros reconhecidos e publicar somente após revisão |
| agência com exatamente um client comprovado | gerar candidato de import para revisão explícita |
| agência com mais de um client ou sem evidência | quarentena; não duplicar nem criar segmento cross-client |
| field/operator/valor desconhecido | quarentena com reason code; não preservar JSON como regra executável |
| segmento inativo | importar como archived/draft, nunca ativar por default |

O tradutor aceita somente os campos legados comprovados no código:
`search`, `channel`, `status`, `tag`, `ownerId`, `source`, `lastSeenAfter` e `lastSeenBefore`.
Cada um possui mapping explícito para `fieldKey/operator/value` v1, fixture e relatório. O
`filter_json` original pode permanecer apenas como hash/referência auditável no relatório de
backfill; não entra no compiler. Antes do cutover, comparar listagem legada e preview novo por
client, com divergências explicadas.

### 9.3 Writer state

Para cada client/entidade:

```text
legacy -> shadow -> new
```

- `legacy`: API antiga escreve `messaging.*`; Customer Data apenas compara;
- `shadow`: continua um writer legado; projeção nova é reconstruída/backfillada, nunca atende
  comando autoritativo;
- `new`: APIs novas e fachada antiga chamam Customer Data service; escrita CRM legada congela.

Não existe modo `dual`.

### 9.4 Fachada antiga

Rotas atuais permanecem temporariamente:

- `/v1/omnichannel/contacts/crm`;
- `/v1/omnichannel/contacts/{id}/profile`;
- PATCH CRM;
- notes;
- merge/undo;
- `/v1/omnichannel/settings/contact-segments` GET/POST/PATCH.

Após `writer_state=new`:

1. Omnichannel resolve contact→subject/relationship pelo adapter;
2. chama Customer Data service;
3. compõe `CRMContactProfileView`;
4. se habilitado, compõe Intelligence separadamente;
5. nunca escreve novamente nos campos CRM legacy.

Para segmentos após `segment_definition=new`, a rota antiga adapta apenas o subconjunto legado
fechado para o mesmo Customer Data service. Ela não ganha preview, materialização, exportação ou
novos operadores e não grava `messaging.contact_segments`. Não há dual-write.

O cutover exige relatório de impacto e sincronização explícita de roles account-scoped. Permissão
legada de contatos não concede `customer_data.segments.*`: durante `legacy|shadow`, a rota antiga
mantém seu enforcement atual; em `new`, a façade chama o authorizer novo e o endpoint canônico
sempre exige as permissions da seção 7.10. Não há fallback silencioso por role.

Headers `Deprecation`/`Sunset` entram somente na CI-10 após consumidores/UX preparados.

## 10. Eventos e idempotência

Customer Data consome inicialmente:

- `omnichannel.channel_binding.*`;
- `omnichannel.conversation.client_bound`;
- eventos de lead/site e ERP quando CI-05 definir adapters.

Cada evento:

- entra em inbox com unique account+event ID;
- claim usa lock/lease;
- retry exponencial classificado;
- source inexistente/stale não inventa dados;
- replay retorna done sem duplicar subject/link/relação;
- dead-letter gera alerta e reprocessamento administrativo idempotente.

Publicações de Customer Data nascem na mesma transação da mudança autoritativa ou em outbox local
aprovada. O bus in-process pode invalidar cache, nunca ser a única entrega crítica.

## 11. Pacotes atômicos e allowlists de escrita

Nenhum pacote executa com placeholder de migration ou arquivo dirty de outra trilha.

### CI03-DOC-ADR

- **Pode escrever somente:**
  - `docs/customer-intelligence/specs/CI-03_CUSTOMER_DATA.md`
  - `docs/customer-intelligence/adr/ADR-CI-001-customer-data-boundary.md`
- **Proibido:** código/DDL/workflow.

### CI03-DB-FOUNDATION

- **Pode escrever somente após resolver `<NEXT_CD>`:**
  - `back/internal/platform/database/migrations/<NEXT_CD>_customer_data_foundation.sql`
  - `back/database/ERD.md`
  - `back/database/AGENT.md`
- **Entrega:** schema/tabelas/constraints/índices aditivos.
- **Proibido:** backfill, delete/drop, alteração `messaging.*`.
- **Stop:** ERD/AGENT sujo por outra trilha.

### CI03-DB-SEGMENTS

- **Depende de:** `CI03-DB-FOUNDATION` e contrato CI-00/CI-03 READY.
- **Pode escrever somente após resolver `<NEXT_CD_SEGMENTS>`:**
  - `back/internal/platform/database/migrations/<NEXT_CD_SEGMENTS>_customer_data_segments.sql`
  - `back/database/ERD.md`
  - `back/database/AGENT.md`
- **Entrega:** segmentos, versões, evaluation runs, materializações e memberships com FKs
  account/client-scoped; nenhum backfill.
- **Proibido:** export tables, SQL de filtro livre, GIN genérico, alteração/drop de
  `messaging.contact_segments`.

### CI03-DB-SEGMENT-EXPORTS

- **Depende de:** `CI03-DB-SEGMENTS` e decisões/policies jurídicas de finalidade, retenção e TTL.
- **Pode escrever somente após resolver `<NEXT_CD_SEGMENT_EXPORTS>`:**
  - `back/internal/platform/database/migrations/<NEXT_CD_SEGMENT_EXPORTS>_customer_data_segment_exports.sql`
  - `back/database/ERD.md`
  - `back/database/AGENT.md`
- **Entrega:** workflow de export e eligibility items, sem PII copiada.
- **Proibido:** storage público, sender, campanha, default jurídico silencioso ou ativação.

### CI03-BE-MODULE

- **Pode criar somente:**
  - `back/internal/modules/customerdata/AGENT.md`
  - `back/internal/modules/customerdata/module.go`
  - `back/internal/modules/customerdata/model.go`
  - `back/internal/modules/customerdata/errors.go`
  - `back/internal/modules/customerdata/service.go`
  - `back/internal/modules/customerdata/repository.go`
  - `back/internal/modules/customerdata/repository_postgres.go`
  - respectivos arquivos `_test.go`
- **Pode alterar somente:**
  - `back/internal/platform/app/app.go`
- **Proibido:** Omnichannel, front, migration e sources.

### CI03-BE-IDENTITY

- **Pode criar somente dentro de `back/internal/modules/customerdata/`:**
  - `service_identity.go`
  - `service_identity_test.go`
  - `repository_subjects.go`
  - `repository_relationships.go`
  - `repository_identities.go`
  - testes correspondentes
  - atualização de `AGENT.md`
- **Proibido:** HTTP, backfill e outros módulos.

### CI03-BE-NOTES-CONSENTS

- **Pode criar somente dentro do módulo:**
  - `repository_notes_consents.go`
  - `service_notes_consents.go`
  - respectivos testes
  - atualização de `AGENT.md`

### CI03-BE-OFFLINE

- **Pode criar somente dentro do módulo:**
  - `repository_offline_interactions.go`
  - `service_offline_interactions.go`
  - `service_offline_imports.go`
  - `worker_offline_imports.go`
  - interfaces pequenas de storage/scanner consumidas pelo módulo
  - respectivos testes
  - atualização de `AGENT.md`
- **Proibido:** binário/base64 no banco, scanner falso, merge automático e escrita em Intelligence.

### CI03-BE-MATCH-MERGE

- **Pode criar somente dentro do módulo:**
  - `service_merge.go`
  - `repository_matching_merge.go`
  - respectivos testes
  - fixtures sintéticas sob package
  - atualização de `AGENT.md`

### CI03-BE-SEGMENTS

- **Depende de:** `CI03-DB-SEGMENTS`.
- **Pode criar somente dentro do módulo:**
  - `service_segments.go`
  - `service_segment_evaluations.go`
  - `repository_segments.go`
  - `segment_filter.go`
  - `segment_field_catalog.go`
  - respectivos testes/fixtures sanitizadas
  - atualização de `model.go`, `repository.go` e `AGENT.md`
- **Entrega:** lifecycle estável/imutável, validator/compiler AST e evaluation orchestration.
- **Proibido:** LLM, SQL/JSONPath/URL livre, query cross-schema, export e sender.

### CI03-BE-SEGMENT-EXPORTS

- **Depende de:** DB de export, `CI03-BE-SEGMENTS` e gates jurídicos aprovados.
- **Pode criar somente dentro do módulo:**
  - `service_segment_exports.go`
  - `repository_segment_exports.go`
  - ports mínimos de storage privado
  - respectivos testes
  - atualização de `model.go`, `repository.go` e `AGENT.md`
- **Pode alterar somente:** adapter de storage nominalmente despachado no composition root.
- **Proibido:** provider de canal, campaign sender, URL pública, export sem consent check atual ou
  segredo/PII em log.

### CI03-API

- **Pode criar somente dentro do módulo:**
  - `http.go`
  - `http_subjects.go`
  - `http_relationships.go`
  - `http_identity.go`
  - `http_matching.go`
  - respectivos testes
  - atualização de `module.go`/`AGENT.md`
- **Pode alterar somente:** wiring/gating exato em `back/internal/platform/app/app.go` se necessário.
- **Proibido:** fachada Omnichannel.

### CI03-API-OFFLINE

- **Pode criar somente dentro do módulo:**
  - `http_offline_interactions.go`
  - `http_offline_imports.go`
  - respectivos testes
  - atualização de `http.go`/`module.go`/`AGENT.md`
- **Pode alterar somente:** wiring dos adapters de storage/scanner nominalmente despachados.
- **Proibido:** provider de mídia, LLM, sender e endpoint sem permission/idempotency.

### CI03-API-SEGMENTS

- **Depende de:** `CI03-BE-SEGMENTS`.
- **Pode criar somente dentro do módulo:**
  - `http_segments.go`
  - testes correspondentes
  - atualização de `http.go`, `module.go` e `AGENT.md`
- **Entrega:** fields, definitions, versions, validate/publish/rollback, runs, materializations e
  members da seção 7.10.
- **Proibido:** aceitar account/SQL/field definition do browser, export ou fachada Omnichannel.

### CI03-API-SEGMENT-EXPORTS

- **Depende de:** `CI03-BE-SEGMENT-EXPORTS`.
- **Pode criar somente dentro do módulo:**
  - `http_segment_exports.go`
  - testes correspondentes
  - atualização de `http.go`, `module.go` e `AGENT.md`
- **Entrega:** request/status/cancel/download intent; nenhum endpoint de envio.
- **Proibido:** URL permanente, campos livres, bypass de consentimento ou integração de campanha.

### CI03-EVENT-JOB

- **Pode criar somente dentro do módulo:**
  - `events.go`
  - `worker.go`
  - `repository_writer_state.go`
  - respectivos testes
  - atualização de `module.go`/`AGENT.md`
- **Pode criar adapter somente em:**
  - `back/internal/platform/app/customer_data_omnichannel_adapter.go`
  - teste correspondente.

### CI03-JOB-SEGMENTS

- **Depende de:** `CI03-BE-SEGMENTS`; export worker depende também de
  `CI03-BE-SEGMENT-EXPORTS`.
- **Pode criar somente dentro do módulo:**
  - `worker_segments.go`
  - testes correspondentes
  - atualização mínima de `worker.go`, `module.go` e `AGENT.md`
- **Entrega:** claim/lease/retry/cancel de evaluation/materialization e export eligibility.
- **Pode alterar somente:** wiring nominal no composition root e scheduler/worker registration
  aprovado.
- **Proibido:** cron/n8n como autoridade, scan sem scope, polling sem cap, sender e campanha.

### CI03-BACKFILL

- **Pode criar somente:**
  - `back/cmd/customer-data-backfill/main.go`
  - `back/internal/modules/customerdata/backfill.go`
  - `back/internal/modules/customerdata/backfill_test.go`
  - `docs/customer-intelligence/evidence/CI-03_BACKFILL.md`
- dry-run default; escrita exige DB alvo e approval.
- **Proibido:** DDL, cutover, source update.

### CI03-BACKFILL-SEGMENTS

- **Depende de:** `CI03-DB-SEGMENTS`, tradutor/fixtures aprovados e snapshot CI-01.
- **Pode criar somente:**
  - `back/cmd/customer-data-segment-backfill/main.go`
  - `back/internal/modules/customerdata/backfill_segments.go`
  - `back/internal/modules/customerdata/backfill_segments_test.go`
  - `docs/customer-intelligence/evidence/CI-03_SEGMENT_BACKFILL.md`
- dry-run é default; relatório lista mapping, client resolvido, divergência e quarentena.
- **Proibido:** auto-publicar ambíguo, duplicar por client, preservar JSON desconhecido como AST,
  DDL, cutover ou alterar tabela legada.

### CI03-LEGACY-FACADE

- **Somente após shadow aprovado. Pode alterar:**
  - `back/internal/modules/omnichannel/http_contacts_crm.go`
  - `back/internal/modules/omnichannel/service_crm.go`
  - `back/internal/modules/omnichannel/crm_model.go`
  - testes diretamente correspondentes
  - `back/internal/modules/omnichannel/AGENT.md`
  - adapter exato no composition root
- **Proibido:** `store_webhook_events.go`, sender, FSM, IA e drop.

### CI03-QA-SEGMENTS

- **Depende de:** pacotes segmentação escolhidos no despacho.
- **Pode escrever somente:** testes/fixtures focados no módulo e
  `docs/customer-intelligence/evidence/CI-03_SEGMENTS_QA.md`.
- **Entrega:** matriz adversarial de AST, tenant/client, imutabilidade, runs, consentimento,
  exportação, idempotência, carga e compatibilidade legada.
- **Proibido:** alterar produção para fazer o teste passar, snapshots com PII, migration, capability
  on, sender ou workflow.

### CI03-CUTOVER

- **Depende de:** QA dos pacotes selecionados e gates de rollout atendidos.
- **Pode escrever somente:**
  - writer/capability state pelos comandos administrativos aprovados;
  - `docs/customer-intelligence/evidence/CI-03_CUTOVER.md`;
  - eventual migration de constraint final com nome exato resolvido.
- **Proibido:** remoção de tabela/campo/rota.

## 12. Compatibilidade e deprecação

- APIs novas são a autoridade após writer cutover;
- fachada velha adapta IDs e chama o mesmo service;
- front antigo pode continuar consumindo composição;
- não criar view SQL que leia schema privado de outro módulo;
- `messaging.contact_intelligence` não entra em Customer Data;
- nenhuma rota é removida nesta CI;
- nenhum campo/tabela legacy é dropado;
- métricas contabilizam tráfego de fachada para CI-10.

## 13. Testes e comandos

A partir de `back/`:

```text
go test ./internal/modules/customerdata/...
go test ./internal/modules/omnichannel/... -run 'CRM|Contact|Merge'
go test ./internal/platform/app/...
go test ./...
golangci-lint run ./internal/modules/customerdata/... ./internal/platform/app/...
```

Contratos mínimos:

- account/client negativo → 404;
- agency sem client → validação sem default;
- standalone → client=account explícito;
- subject com duas relações não vaza a segunda;
- identity replay não duplica;
- match fraco não merge;
- external ID é escopado por issuer/client;
- merge concorrente/idempotente/undo;
- consentimento append-only;
- optimistic revision → 409;
- paginação estável com empate de timestamp;
- facade e API nova retornam campos determinísticos equivalentes;
- módulo desabilitado não recebe fetch/command;
- Omnichannel recebe/responde humano com Customer Data ausente;
- segmento A nunca lista/versiona/avalia/materializa/exporta relação do client B;
- permissão Omnichannel legada sozinha não acessa API nova de segmentos;
- `segmentId` permanece estável; published rejeita PATCH/DELETE e rollback apenas troca binding;
- AST rejeita field/operator/node extra, tipo inválido, cap de profundidade/nodes/lista/string e
  payloads de SQL/JSONPath/regex/URL;
- compiler usa parâmetros e scope obrigatório mesmo quando o valor contém metacaracteres;
- campo externo passa apenas por resolver tipado e não gera SQL cross-schema;
- `within_last` e paginação usam o mesmo `asOf` do run;
- preview cria um único run idempotente e não cria materialization/membership/export;
- materialize aceita somente published, mantém snapshot anterior em falha e converge sob workers
  concorrentes/retry;
- source stale/indisponível resulta `partial|failed` conforme versão, nunca count zero silencioso;
- amostra/member respeita permissões cumulativas e identidade mascarada;
- revogação/expiração de consentimento após materialização exclui no export;
- export usa field set fechado, objeto privado/TTL/download intent e não chama sender;
- `segment_exports=shadow` não cria artefato baixável;
- backfill traduz somente oito campos legados conhecidos e quarentena client/filtro ambíguo;
- fachada legada e API nova convergem ao mesmo writer sem dual-write.

Teste de migration exige antes:

- host/port/database/container confirmados;
- `migrate status`;
- schema/constraints/índices;
- FKs compostas e ponteiros version/materialization/export no mesmo account/client/segment;
- imutabilidade de published e unicidade de draft/idempotency;
- explain de hot paths com dataset representativo;
- nenhum lock pesado junto de backfill.

## 14. Observabilidade

Métricas:

- subjects/relationships por client/status;
- candidates/quarantine/merge/undo;
- eventos inbox pending/failed/dead/lag;
- source links unresolved;
- writer mode por entity/client;
- divergence/checksum legacy↔new;
- tráfego/error da fachada;
- 404 scope negative e conflitos de revision sem IDs sensíveis;
- segmentos/drafts/published/archived por client, sem nomes/IDs em label;
- evaluation runs por mode/status/reason, queue lag, lease retry, duração e matched count bucket;
- materialization age/freshness/member count bucket e source partial/stale;
- exports por status/purpose/channel, eligible/excluded reason bucket, expiry e download intent;
- rejeições AST por reason code e cost/budget, nunca o valor do predicate.

Auditoria correlaciona request/event/source/subject/relationship/actor. Logs não carregam value
cru, nota, documento, prompt ou mensagem.

## 15. Rollout

1. aprovação CI-00/CI-02;
2. DDL aditivo;
3. módulo registrado, mas account modules/capabilities off;
4. testes e adapters read-only;
5. inventário CI-01 e classificação;
6. backfill dry-run;
7. shadow por client/entidade;
8. comparação de campos/contagens/checksum;
9. ativar API nova somente para client canary;
10. trocar fachada/writer por entidade;
11. observar;
12. expandir client a client;
13. somente CI-10 considera deprecação.

Segmentação percorre uma trilha independente por client:

```text
segmentation off -> shadow (traduz/preview/compara) -> on (novo writer/materialização)
segment_exports off -> shadow (eligibilidade sem arquivo) -> on (somente após gates jurídicos)
```

Publicação de definição não liga schedule/export automaticamente. Primeiro validar field catalog e
fixtures, depois preview, backfill dry-run, comparação legada, materialização canary e observação de
custo/freshness. Exportação só avança depois de teste de revogação e storage privado/TTL.

## 16. Rollback

- antes de `new`: retornar leitura a legacy e preservar projeção;
- após `new`: manter Customer Data writer e reativar apenas fachada de leitura; não religar writer
  legado sem reconciliação reversa aprovada;
- capacidade pode ficar off sem apagar dados;
- eventos já processados não são reexecutados sem idempotency;
- migrations aditivas não são revertidas por drop;
- Omnichannel participante/conversa/sender continuam intactos;
- rollback de segmento aponta para versão publicada anterior e materialização compatível; nunca
  edita versão nem restaura membership por cópia manual;
- desligar export cancela novos jobs/intents, expira objetos conforme policy e preserva auditoria.

## 17. Critérios de aceite

- [ ] módulo/ID/schema/route aprovados;
- [ ] subject e relação possuem writer único;
- [ ] relação A nunca aparece em contexto B;
- [ ] chat humano funciona com módulo ausente/desabilitado;
- [ ] intelligence pode consumir perfil determinístico sem SQL cross-module;
- [ ] toda identidade possui issuer/escopo/verificação/proteção;
- [ ] consentimento é append-only e relationship-scoped;
- [ ] merge/undo são auditáveis/idempotentes;
- [ ] backfill quarentena ambiguidades;
- [ ] API nova e fachada convergem ao mesmo writer;
- [ ] paginação/índices cobrem hot paths;
- [ ] front reidrata resposta autoritativa;
- [ ] nenhum dado inteligente é fingido como Customer Data;
- [ ] cada segmento pertence a um único account/client e possui ID estável;
- [ ] published é imutável, draft usa revision e rollback troca somente o binding;
- [ ] AST é fechado/versionado, fields/operators vêm do registry e não há SQL arbitrário;
- [ ] preview/materialização/recompute são runs auditáveis, idempotentes e com `asOf`;
- [ ] membership derivada não contém PII nem é tratada como consentimento;
- [ ] exportação possui permission/finalidade/consentimento/TTL próprios e não envia;
- [ ] capacidade legada foi traduzida ou quarentenada sem dual-write;
- [ ] nenhuma exclusão/workflow/provider foi alterado.

## 18. Stop conditions

Parar se:

- CI-00/CI-02 não estiverem aprovadas;
- `customer_data` próprio for rejeitado pelo owner;
- policy de criptografia/HMAC não estiver definida antes de persistir identidade real;
- authorizer backend por account não estiver disponível;
- backfill não conseguir determinar client sem ambiguidade;
- writer state permitir dual;
- API/fachada escreverem em stores diferentes;
- repository precisar consultar `messaging.*`, ERP ou Site;
- source event carregar PII/mensagem bruta;
- AST/compiler aceitar SQL, coluna, JSONPath, URL, expressão, regex livre ou field externo sem
  resolver registrado;
- segmento aceitar client nulo/implícito em agência ou membro cross-client;
- published precisar ser editado ou preview persistir membership;
- exportação tratar membership como opt-in, gerar URL pública, ignorar revogação ou chamar sender;
- decisões jurídicas/retention/TTL estiverem pendentes quando export sair de `off|shadow`;
- cutover exigir apagar/modificar conversa/mensagem;
- ERD/AGENT/migration estiverem em conflito com outra trilha;
- próximo número de migration não estiver confirmado;
- qualquer execução precisar tocar `social-publishing`, `automation` legado, n8n ou sender.

## 19. Handoff obrigatório

Cada pacote registra:

- baseline/worktree/DB alvo;
- allowlist efetivamente usada;
- migration e status;
- contratos/API/eventos;
- field catalog/schema/hash e lifecycle de versões;
- runs/materializações/exports, `asOf`, counts e reason codes;
- contagens/checksums/exceções;
- writer states antes/depois;
- testes e resultados;
- compatibilidade/tráfego;
- rollout/rollback;
- decisões não provadas;
- confirmação de zero SQL cross-module, dual-write, drop e workflow.
