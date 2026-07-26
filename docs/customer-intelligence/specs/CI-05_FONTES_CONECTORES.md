# CI-05 — Fontes e conectores

- **Status:** READY — implementação local autorizada; fontes nascem desligadas
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** `customer_intelligence`
- **Dependências:** CI-00, CI-01, CI-02, CI-03 e núcleo de CI-04
- **Bloqueia:** CI-06, CI-08, CI-09 e CI-10
- **Governança:** [../GOVERNANCA.md](../GOVERNANCA.md), versão vigente
- **Blueprint:** [../SPECS_GERAIS.md](../SPECS_GERAIS.md), versão vigente

> Esta spec não autoriza implementação, credencial, sync, workflow ou consulta externa. Ela
> especifica adapters tipados e o painel administrativo. O dado continua pertencendo ao módulo
> de origem; Customer Intelligence recebe apenas o recorte autorizado.

## 1. Resultado único e verificável

Disponibilizar um registry allowlisted de fontes e quatro portas distintas para:

1. evidência de subject/relationship;
2. contexto empresarial do cliente;
3. agregados de portfólio;
4. ações tipadas explicitamente autorizadas.

Um administrador autorizado deve conseguir, pelo painel, configurar finalidade, campos, frescor,
retenção e comportamento histórico; testar saúde; iniciar sync; acompanhar cursor/runs; desabilitar
com semântica explícita; e aceitar/rejeitar sugestões sem jamais fornecer SQL, URL, tabela, tool ou
segredo ao modelo.

O resultado é demonstrado quando Omnichannel, manual/offline, ERP, Calendário, Site e BI podem ser
registrados de forma independente, uma fonte falha sem interromper o chat, e cada observação
resultante aponta `source_key`, entidade e versão de origem.

## 2. Ordem de entrega

1. `omnichannel`;
2. `manual.offline`;
3. `erp`;
4. `calendar.client_profile`;
5. `site`;
6. `bi.perola`, somente on-demand e com filtros seletivos.

Essa ordem não autoriza um pacote a tocar outro módulo. Cada adapter tem pacote e allowlist próprios.

## 3. Decisões congeladas como proposta DRAFT

- módulo ID `customer_intelligence`, package `customerintelligence`, schema `intelligence`;
- API `/v1/customer-intelligence`;
- UI `/inteligencia-clientes/fontes`;
- `account_id` físico é o alias de domínio de `owner_account_id`;
- conta não-agência usa `client_account_id = account_id`;
- agência exige cliente explícito, mesma organização e catálogo permission-scoped;
- Customer Intelligence requer `customer_data`;
- Omnichannel, CRM, Calendar e Site são módulos opcionais;
- BI continua source adapter allowlisted, mas ainda não entra em `Metadata.OptionalModules` porque
  `back/internal/modules/bi` não possui hoje ID/module metadata estáveis no Registry;
- `source_key` é estável, registrado em código e pode ser deprecado, nunca reutilizado;
- nenhuma configuração aceita endpoint, query ou credential arbitrários;
- evidência crítica entra por mecanismo durável e idempotente;
- o event bus in-memory pode invalidar cache, mas não é garantia de ingestão;
- leitura e escrita em módulo de origem usam portas diferentes;
- sugestão de fonte nunca habilita ou amplia acesso;
- desabilitar uma fonte não equivale automaticamente a apagar seu histórico.

## 4. Estado atual medido no disco

| Fonte/capacidade | Estado atual relevante |
|---|---|
| Omnichannel | mensagens, identidades, touchpoints, CRM local e resultados em `messaging.*` |
| Calendário | `calendar.client_profiles`; adapter atual em `platform/app/omnichannel_calendar_adapter.go` |
| ERP | `crm/erp` possui raw/current e APIs; customer traz nome, apelido, CPF, telefone, celular, e-mail, nascimento, endereço e tags |
| Site | leads, consentimento, UTM/campanha e tracking; visitor/session não equivalem a pessoa |
| BI/Pérola | dataset registry tipado, limites e filtros obrigatórios; Nota/Inventário são fontes caras |
| manual/offline | notas CRM existem no Omnichannel; ainda não há source adapter genérico |
| eventos | `platform/events` é síncrono e não persistente |
| tools | `AIToolRegistry` atual já recusa tool não registrada e valida argumentos |

Riscos comprovados:

- CPF genérico `82541150016` aparece associado a muitas pessoas no ERP e não pode causar match;
- busca nominal/fuzzy não identifica pessoa;
- Inventário aberto da Pérola excedeu 35 segundos mesmo com `limit: 1`;
- Nota contém PII fiscal e exige recorte/intenção;
- o contexto do Calendário atual é cliente/business context, não fato individual;
- o inbound não pode chamar outro módulo dentro da transação de webhook.

## 5. Registry de fontes

### 5.1 Descriptor imutável

```go
type SourceDescriptor struct {
    Key                    string
    OwnerModuleID          string // vazio quando o owner ainda não possui Module ID estável
    OwnerPackage           string
    Label                  string
    Capabilities           []SourceCapability
    EntityTypes            []string
    Modes                   []SourceMode
    ConfigSchema           json.RawMessage
    FieldCatalog           []SourceField
    RequiredFilterSets     [][]string
    DefaultPageSize        int
    MaxPageSize            int
    DefaultFreshness       time.Duration
    MaxLookback            time.Duration
    SupportsCursor         bool
    SupportsTest           bool
    SupportsHistoricalUse  bool
    SensitivityCeiling     string
    SecretMode             string
}
```

Enums:

- `SourceCapability`: `subject_evidence`, `business_context`, `portfolio_aggregate`,
  `authorized_action`;
- `SourceMode`: `event`, `scheduled`, `on_demand`, `manual`;
- `SecretMode`: `none`, `source_owned`, `secret_ref`, nunca `inline`.

`SourceField` declara chave, tipo, categoria, sensibilidade, se pode persistir snapshot, se pode
entrar em contexto, filtros/ordenações e tamanho máximo.

### 5.2 Regras do registry

- registro explícito no composition root;
- duplicate `source_key` falha no boot;
- descriptor não muda durante uma execução;
- config é validada contra schema fechado;
- campo/filtro/ordenação desconhecido é rejeitado;
- teto do descriptor vence valor do painel;
- source ausente por módulo opcional aparece como `unavailable`, não como dado vazio;
- descriptor deprecado permanece legível para histórico, mas bloqueia config nova;
- modelo e prompt recebem apenas capabilities/bindings já resolvidos, nunca o registry inteiro.

Catálogo inicial:

| `source_key` | Owner | Capabilities | Modo inicial |
|---|---|---|---|
| `omnichannel` | Omnichannel | `subject_evidence` | evento |
| `manual.offline` | Customer Data | `subject_evidence` | manual |
| `erp` | CRM/ERP | `subject_evidence`, `business_context` | schedule/on-demand |
| `calendar.client_profile` | Calendar | `business_context`, `authorized_action` | on-demand |
| `site` | Site | `subject_evidence`, agregado limitado | evento/schedule |
| `bi.perola` | BI | `business_context`, `portfolio_aggregate` | on-demand |

Para `bi.perola`, `OwnerPackage=bi` e `OwnerModuleID` permanece vazio até o BI possuir ID estável.
Isso não remove gates de autenticação/permissão do adapter nem autoriza inventar um Module ID.

## 6. Portas e contratos campo a campo

As interfaces são declaradas pelo consumidor em Customer Intelligence; adapters concretos vivem
em `back/internal/platform/app`. Nenhuma interface retorna repository, SQL row ou DTO privado.

### 6.1 Evidência de subject

```go
type SubjectEvidenceRequest struct {
    AccountID, ClientAccountID string
    SubjectID, RelationshipID  *string
    PurposeKey                 string
    EntityTypes                []string
    Fields                     []string
    ExternalRefs               []ExternalRef
    Since, Until, AsOf         *time.Time
    Cursor                     string
    Limit                      int
}

type ExternalRef struct {
    Namespace, ValueFingerprint, VerificationState string
}

type EvidenceRecord struct {
    SourceKey, EntityType, EntityID, SourceVersion string
    IdempotencyKey                                 string
    OccurredAt, ObservedAt                         time.Time
    SubjectCandidate                              *SubjectCandidate
    Fields                                        map[string]SourceValue
    Sensitivity, PurposeKey                       string
}

type SourceValue struct {
    Type string
    Value json.RawMessage
    Verified bool
    ValidFrom, ValidUntil *time.Time
}

type EvidencePage struct {
    Records []EvidenceRecord
    NextCursor string
    HasMore bool
    Freshness SourceFreshness
    Warnings []string
}

type SubjectEvidenceSource interface {
    ReadSubjectEvidence(context.Context, SubjectEvidenceRequest) (EvidencePage, error)
}
```

Regras:

- `Limit` default 50, máximo do descriptor;
- `Cursor` é opaco e limitado;
- `Fields` precisa ser subconjunto da config e do descriptor;
- resposta não pode conter segredo, body bruto ou campo não solicitado;
- matching é realizado por Customer Data, não pelo adapter;
- `SubjectCandidate` é candidato, não merge/vínculo.

### 6.2 Contexto empresarial

```go
type BusinessContextRequest struct {
    AccountID, ClientAccountID, PurposeKey string
    Sections []string
    AsOf *time.Time
    MaxBytes int
}

type BusinessContext struct {
    SourceKey, SchemaVersion string
    ClientAccountID string
    Sections map[string]json.RawMessage
    UpdatedAt *time.Time
    Freshness SourceFreshness
    Warnings []string
}

type BusinessContextSource interface {
    ReadBusinessContext(context.Context, BusinessContextRequest) (BusinessContext, error)
}
```

Esse contrato não leva `subject_id` e não transforma posicionamento/brand voice em fato pessoal.

### 6.3 Agregado de portfólio

```go
type AggregateRequest struct {
    AccountID, PurposeKey, DatasetKey string
    ClientAccountIDs []string
    Dimensions, Metrics []string
    Filters []TypedFilter
    Period DateRange
    Cursor string
    Limit int
}

type AggregateResult struct {
    SchemaVersion string
    Rows []map[string]json.RawMessage
    CohortSize int
    Suppressed bool
    SuppressionReason string
    NextCursor string
    Freshness SourceFreshness
}

type PortfolioAggregateSource interface {
    QueryAggregate(context.Context, AggregateRequest) (AggregateResult, error)
}
```

CI-09 define coorte/supressão. Até lá, essa capability não pode ser publicada a usuário final.

### 6.4 Ação autorizada

```go
type ActionRequest struct {
    AccountID, ClientAccountID, PurposeKey string
    ActorUserID string
    ToolKey, Operation string
    IdempotencyKey string
    Arguments json.RawMessage
    ApprovalID *string
}

type ActionResult struct {
    Status string
    EntityRef *ExternalEntityRef
    OutputMasked json.RawMessage
    AuditRef string
}

type AuthorizedActionTool interface {
    ProposeOrExecute(context.Context, ActionRequest) (ActionResult, error)
}
```

Uma leitura nunca ganha permissão de escrita implicitamente. O owner valida schema, permissão,
consentimento, revision e idempotência em seu próprio service/repository.

## 7. Ingestão durável

### 7.1 Eventos

Envelope mínimo:

```json
{
  "schemaVersion": "source.event.v1",
  "eventId": "uuid",
  "topic": "omnichannel.interaction.accepted",
  "occurredAt": "RFC3339",
  "accountId": "server-resolved",
  "clientAccountId": "validated",
  "sourceKey": "omnichannel",
  "entityType": "conversation_interaction",
  "entityId": "uuid",
  "sourceVersion": "opaque",
  "purposeKey": "customer_profile"
}
```

Payload leva IDs, não conteúdo bruto. O adapter carrega a projeção autorizada depois do commit.

### 7.2 `intelligence.source_ingestion_jobs`

CI-05 propõe uma migration aditiva separada:

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `client_account_id`, `source_config_id` | escopo/FKs |
| `event_id`, `topic`, `entity_type`, `entity_id`, `source_version` | referência |
| `idempotency_key` | obrigatório e único por source config |
| `status` | `queued`, `processing`, `completed`, `failed`, `dead_letter`, `cancelled` |
| `attempts`, `max_attempts` | bounds |
| `run_after`, `locked_at`, `completed_at` | lease |
| `last_error_code`, `last_error_masked` | sem PII |
| `created_at`, `updated_at` | timestamps |

Índices:

- unique `(account_id, source_config_id, idempotency_key)`;
- claim `(status, run_after, created_at, id)`;
- histórico `(account_id, source_config_id, created_at desc, id desc)`.

### 7.3 Regras de worker

- `FOR UPDATE SKIP LOCKED` ou engine `platform/jobs` compatível;
- lease com timeout e recuperação;
- retry apenas para erro classificado transitório;
- `not_found` de origem após retenção vira dead-letter auditável;
- evento repetido retorna completed existente;
- grava observação/claim por services de CI-04;
- não chama LLM na transação de ingestão;
- falha nunca bloqueia webhook, mensagem ou outbox do canal.

## 8. Configuração e lifecycle de fonte

### 8.1 Criar/editar

Input administrativo:

```json
{
  "clientAccountId": "uuid",
  "sourceKey": "erp",
  "connectionKey": "default",
  "mode": "scheduled",
  "purposeKey": "customer_profile",
  "fieldAllowlist": ["customer.name", "order.summary"],
  "categoryAllowlist": ["identity", "commerce"],
  "priority": 800,
  "required": false,
  "freshnessSlaSeconds": 86400,
  "historicalUseMode": "include",
  "retentionPolicyKey": "customer_profile.default",
  "config": {"scheduleKey": "daily"},
  "secretInput": null
}
```

`sourceKey`, fields, mode, finalidade, schedule e config são validados pelo descriptor. Segredo,
quando aplicável, é write-only e enviado por campo separado; a resposta retorna apenas
`{set,last4,updatedAt}` ou referência mascarada.

### 8.2 Saúde

```json
{
  "status": "healthy|degraded|stale|error|disabled|unavailable|unknown",
  "lastAttemptAt": null,
  "lastSuccessAt": null,
  "dataAsOf": null,
  "freshnessSlaSeconds": 86400,
  "ageSeconds": null,
  "lastRunId": null,
  "lastErrorCode": "",
  "nextScheduledAt": null
}
```

Ausência e stale são explícitos. Nenhum fallback vazio pode parecer dado atual.

### 8.3 Desabilitar

O painel exige quatro decisões separadas:

```json
{
  "expectedRevision": 7,
  "stopIngestion": true,
  "historicalEvidence": "include|exclude",
  "derivatives": "keep|recompute|invalidate",
  "retentionAction": "policy|retain|anonymize_request|delete_request",
  "reason": "texto obrigatório"
}
```

Fluxo:

1. `preview-disable` calcula impacto;
2. confirmação com revision e reason;
3. status muda e novos jobs são recusados/cancelados;
4. rebuild/invalidação roda em job separado;
5. exclusão/anônimo vira pedido sujeito a policy/legal hold, nunca cascade imediato.

## 9. Adapters iniciais

### 9.1 Omnichannel

Insumos permitidos:

- IDs e metadados sanitizados de mensagem/conversa;
- identidade local do canal;
- touchpoints/campanha;
- resultado aceito de atendimento, handoff e outcome.

Regras:

- integração começa somente após commit;
- `messaging.ai_dispatches`, FSM e outbox continuam no Omnichannel;
- conteúdo é buscado por adapter com janela/finalidade, não copiado no evento;
- resultado atrasado/cancelado não gera aprendizagem aceita;
- mídia binária não sai do storage; análise autorizada usa referência;
- source failure não altera a conversa.

CI-07 possui a inserção transacional do evento/outbox; CI-05 possui o consumer/adapter.

### 9.2 Manual/offline

Casos:

- reunião presencial;
- ligação;
- conversa fora do canal;
- nota;
- correção verificada;
- importação controlada.

Regras:

- operador precisa de permissão Customer Data;
- toda entrada registra autor, horário real, origem e finalidade;
- correção manual produz observação e claim, nunca `UPDATE facts` direto;
- anexos ficam em storage privado com MIME/tamanho/antivírus; não em JSON/base64;
- CSV não executa merge automático e produz relatório de linhas aceitas/rejeitadas;
- conteúdo offline não entra em LLM se classificação/finalidade não permitirem.

Contrato de entrada canônico pertence ao Customer Data:

```json
{
  "clientAccountId": "uuid",
  "interactionType": "meeting",
  "occurredAt": "RFC3339",
  "timezone": "America/Sao_Paulo",
  "title": "string",
  "content": "string",
  "sensitivity": "personal",
  "purposeKey": "customer_relationship",
  "attachmentRefs": ["opaque-upload-id"],
  "sourceExternalRef": null,
  "idempotencyKey": "caller-key"
}
```

`POST /v1/customer-data/relationships/{relationshipId}/offline-interactions` valida e persiste o
registro determinístico. O adapter CI-05 consome
`customer_data.offline_interaction.changed.v1`, busca a projeção permitida pela porta pública e
cria observação/claim sob a source `manual.offline`.

Importação usa `POST /v1/customer-data/offline-interaction-imports`, storage ref escaneada, mapping
versionado, `dryRun` obrigatório antes de apply e relatório linha a linha. Anexo só é elegível após
scanner server-side `clean`; arquivo nunca passa no evento, JSON ou prompt. Permissões, DTOs,
endpoints e packages estão congelados na CI-03 §5.6.1/§7.7.1.

### 9.3 ERP

Capacidades iniciais:

- cadastro identificado;
- pedidos/itens/cancelamentos;
- datas e agregados comerciais;
- produtos/catálogos por referência.

Matching:

- ID ERP dentro do conector/cliente;
- documento validado e não genérico;
- telefone/e-mail normalizado e verificado;
- nome, endereço e nascimento apenas candidatos;
- CPF `82541150016`, inválido, placeholder ou compartilhado nunca faz auto-match;
- divergência vira review de Customer Data.

O adapter consome service/repository público do ERP; não consulta `erp_*` diretamente.

### 9.4 Calendário

Primeiro adapter traduz `calendar.client_profiles` para `BusinessContext`.

Seções iniciais:

- segmento, posicionamento, descrição e histórico;
- site/Instagram/endereço;
- objetivos, brand voice, público, oferta, pilares, cadência, restrições, performance e assets.

Regras:

- é contexto do cliente, não memória do subject;
- leitura on-demand tem timeout e freshness;
- edição usa `AuthorizedActionTool` separado;
- action tool nasce `propose_write`, com aprovação;
- Calendar valida e persiste em seu próprio service;
- nunca `UPDATE calendar.*` pelo Intelligence.

### 9.5 Site

Insumos:

- lead/formulário;
- consentimento;
- página/campanha/UTM/referrer;
- eventos de tracking limitados e agregados.

Regras:

- `visitor_id` e `session_id` não viram subject;
- conversão explícita ou identidade forte gera candidato de match;
- consentimento da origem é evidência, não permissão universal;
- tracking bruto não é copiado integralmente para o prompt;
- integração Site/Omnichannel usa idempotência e atribuição temporal.

### 9.6 BI/Pérola

Somente on-demand no primeiro rollout:

- usa o dataset registry já tipado pelo BI;
- uma entidade/página por consulta;
- filtros e limites do BI são preservados ou reduzidos;
- Inventário exige `itemSaldoId`;
- Nota exige ID/documento ou período fechado máximo de 31 dias;
- nenhuma abertura da UI dispara fan-out;
- nenhuma sincronização integral;
- credencial/JWT permanece no BI e não chega ao Intelligence/n8n/front;
- PII fiscal não entra em contexto sem finalidade/campo explícitos;
- timeout/falha devolve `stale/unavailable`, sem retry recursivo.

## 10. Sugestões de fonte

O processo `source.suggest` recebe:

- catálogo de capabilities disponível;
- campos/fatos faltantes;
- fontes já habilitadas e sua saúde;
- finalidade atual.

Ele devolve somente:

```json
{
  "sourceKey": "erp",
  "missingCapabilities": ["purchase_history"],
  "rationale": "string",
  "confidence": 0.82
}
```

Go rejeita `sourceKey` desconhecida. Aceitar cria config `draft`; habilitar exige ação humana
separada, permissões, finalidade, retenção e secret/config válidos.

## 11. APIs administrativas

| Método e rota | Permissão | Resultado |
|---|---|---|
| `GET /v1/customer-intelligence/sources/catalog` | `customer_intelligence.sources.view` | descriptors disponíveis no account/client |
| `GET /v1/customer-intelligence/sources` | `customer_intelligence.sources.view` | configs paginadas + health |
| `POST /v1/customer-intelligence/sources` | `customer_intelligence.sources.manage` | cria draft |
| `PATCH /v1/customer-intelligence/sources/{id}` | `customer_intelligence.sources.manage` | update com `expectedRevision` |
| `POST /v1/customer-intelligence/sources/{id}/test` | `customer_intelligence.sources.manage` | teste limitado, sem ingestão |
| `POST /v1/customer-intelligence/sources/{id}/sync` | `customer_intelligence.sources.manage` | agenda run idempotente |
| `POST /v1/customer-intelligence/sources/{id}/preview-disable` | `customer_intelligence.sources.manage` | impacto |
| `POST /v1/customer-intelligence/sources/{id}/disable` | `customer_intelligence.sources.manage` | quatro decisões |
| `POST /v1/customer-intelligence/sources/{id}/enable` | `customer_intelligence.sources.manage` | valida dependências e revision |
| `POST /v1/customer-intelligence/sources/{id}/retention-policy-preview` | `customer_intelligence.sources.manage` | impacto da nova versão |
| `POST /v1/customer-intelligence/sources/{id}/retention-policy-rebind` | `customer_intelligence.sources.manage` | CAS para versão published |
| `POST /v1/customer-intelligence/sources/{id}/retention-policy-rollback` | `customer_intelligence.sources.manage` | reponta versão anterior |
| `GET /v1/customer-intelligence/sources/{id}/runs` | `customer_intelligence.sources.view` | cursor |
| `GET /v1/customer-intelligence/source-suggestions` | `customer_intelligence.sources.view` | pendentes/decididas |
| `POST /v1/customer-intelligence/source-suggestions/{id}/decide` | `customer_intelligence.sources.manage` | aceita/rejeita; não habilita |

`clientAccountId` é selecionado do catálogo permitido e revalidado; não define `account_id`.
Recursos fora do escopo retornam 404.

As APIs de entrada manual são `/v1/customer-data/...` da CI-03 e exigem
`customer_data.offline_interactions.view|manage|import`. `http_sources.go` não cria uma rota
concorrente para conteúdo offline; apenas expõe health/run da ingestão `manual.offline`.

## 12. Painel `/inteligencia-clientes/fontes`

### 12.1 Estrutura

- `AdminPageHeader` compartilhado;
- seletor de cliente permission-scoped;
- cards por categoria, colapsados por default;
- resumo no cabeçalho: status, data atual, modo, última sync;
- estados loading, vazio, erro, stale, unavailable e disabled;
- nenhum fetch quando módulo/permissão não permite;
- lazy-load de runs e detalhe.

Seções por fonte:

1. capacidade e owner;
2. campos/categorias;
3. finalidade e retenção;
4. modo/agendamento;
5. segredo mascarado, quando aplicável;
6. health e freshness;
7. runs/cursor/dead-letter;
8. ações testar, sincronizar, habilitar/desabilitar.

### 12.2 Segurança e estado

- segredo digitado existe somente no input/request TLS;
- após salvar, o campo é limpo;
- API/store/localStorage nunca reidratam chave;
- draft dirty não é substituído por polling;
- trocar conta/cliente cancela requests e limpa estado;
- confirmação de disable mostra as quatro decisões;
- UI nunca oferece campo de URL/SQL/tabela/tool livre;
- ações perigosas exibem impacto e exigem reason;
- menu é cosmético; URL direta repete gates.

### 12.3 Contratos front

Paths propostos:

```text
web/app/pages/inteligencia-clientes/fontes.vue
web/app/components/customer-intelligence/sources/IntelligenceSourcesCatalog.vue
web/app/components/customer-intelligence/sources/IntelligenceSourceConfigDrawer.vue
web/app/components/customer-intelligence/sources/IntelligenceSourceHealth.vue
web/app/composables/customer-intelligence/useIntelligenceSources.ts
web/app/domain/customer-intelligence/source-api.ts
web/app/domain/customer-intelligence/source-types.ts
```

O composable guarda somente DTOs autoritativos, filtros, cursors e drafts durante a sessão; não
cria cache definitivo de evidências nem source registry paralelo.

### 12.4 Entrada manual/offline

O perfil 360° oferece timeline e drawer “Registrar interação offline” quando as permissões
Customer Data permitirem. O card da source `manual.offline` em `/fontes` mostra health/cobertura e
leva ao perfil/importador; ele não tenta gravar conteúdo pela API de sources.

A UI inclui:

- formulário tipado de tipo/data/timezone/finalidade/classificação;
- upload por intent privado com progresso e estado de scan;
- import CSV com mapping, dry-run, relatório de aceitas/rejeitadas/quarentena e apply confirmado;
- estados dirty, conflito de revision, erro parcial, retry e troca de account/client;
- indicação separada de “registro salvo” e “inteligência processada”.

Sem Customer Intelligence, registrar/listar continua funcional e a UI mostra processamento
inteligente como indisponível, não como falha do cadastro.

## 13. Falhas e modo degradado

| Falha | Comportamento |
|---|---|
| owner module ausente | source `unavailable`; contexto segue sem ela |
| timeout/stale | warning explícito; required source pode impedir somente o processo dependente |
| cursor inválido | run falha sem avançar cursor |
| segredo ausente | config acionável; nunca fallback para env de outro cliente |
| payload fora do schema | registro rejeitado e auditado |
| relationship ambígua | quarentena; sem fato |
| rate limit externo | retry classificado com backoff |
| dead-letter | painel mostra código/contagem; replay exige ação autorizada |
| BI caro sem filtro | rejeição antes do transporte |
| Calendar write sem aprovação | proposta pendente; nada persistido |
| source disable concorrente | revision conflict; nenhuma decisão parcial |

O chat recebe/persiste e encaminha ao humano mesmo com todas as fontes indisponíveis.

## 14. Tenant, permissões e segredos

Permissões:

- `customer_intelligence.sources.view`;
- `customer_intelligence.sources.manage`;
- `customer_intelligence.audit.view` para snapshots/runs mais sensíveis;
- permissões do módulo owner continuam cumulativas para ação escrita.

Gates:

- account do Principal;
- client do catálogo, mesma organização;
- módulo Customer Intelligence;
- módulo fonte opcional habilitado;
- finalidade/categoria/retention;
- field allowlist;
- permission da ação.

Segredos:

- ficam no owner ou cofre/secretbox;
- config guarda somente `secret_ref`;
- nunca aparecem em response, reidratação, log, fixture ou workflow;
- rotação não altera histórico de observações.

## 15. Observabilidade

Métricas:

- health/status/freshness por source key e cliente;
- backlog/age/retry/dead-letter;
- registros vistos/criados/duplicados/rejeitados;
- bytes e duração;
- cursor advancement;
- campo/categoria negados;
- source unavailable/stale no contexto;
- custo/latência BI;
- suggestions aceitas/rejeitadas.

Logs registram `source_key`, account/client, run/job, endpoint lógico/dataset key, página, quantidade,
duração e error code. Nunca token, query externa bruta, argumento sensível, documento ou payload.

## 16. Pacotes atômicos e allowlists

### Leitura permitida

- `AGENT.md`, skills aplicáveis e AGENTs de Customer Intelligence, Customer Data, Omnichannel,
  CRM/ERP, Calendar, Site, BI, platform/app, events e modules;
- `docs/customer-intelligence/GOVERNANCA.md`, `SPECS_GERAIS.md` e specs CI-00 a CI-05;
- documentos canônicos Omnichannel e contrato de execução;
- migrations/tabelas já inventariadas de messaging, ERP, Calendar e Site;
- interfaces/services públicos dos módulos fonte e adapters existentes em `platform/app`;
- `back/internal/modules/omnichannel/ai_tool_registry.go` e validação de tools como precedente;
- dataset registry e testes do BI/Pérola;
- domain/composables de fontes Customer Intelligence criados por CI-08, quando já existirem.

É proibido ler segredo bruto, `.env`, credential export, execution data n8n, mídia privada sem
fixture autorizada, payload de produção ou arquivos social-publishing. A inspeção de um módulo
fonte serve para definir porta pública; não autoriza importar seu repository.

### CI05-BE-REGISTRY-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/source_descriptor.go`;
- `back/internal/modules/customerintelligence/source_registry.go`;
- `back/internal/modules/customerintelligence/source_contracts.go`;
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI05-DB-01

**Escrita permitida:**

- `back/internal/platform/database/migrations/<NNNN>_customer_intelligence_source_jobs.sql`;
- `back/database/ERD.md`;
- `back/database/AGENT.md`;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI05-BE-SERVICE-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/model_source.go`;
- `back/internal/modules/customerintelligence/service_source.go`;
- `back/internal/modules/customerintelligence/store_source.go`;
- `back/internal/modules/customerintelligence/job_source_ingestion.go`;
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI05-API-01

**Escrita permitida:**

- `back/internal/modules/customerintelligence/http_sources.go`;
- testes HTTP correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

### CI05-ADAPTER-OMNI-01

**Escrita permitida:**

- `back/internal/platform/app/customer_intelligence_omnichannel_adapter.go`;
- teste correspondente;
- interfaces mínimas do Omnichannel explicitamente autorizadas pelo orquestrador;
- AGENTs dos dois módulos.

Não edita inbound/outbox; essa integração transacional pertence a CI-07.

### CI05-ADAPTER-MANUAL-01

**Escrita permitida:**

- `back/internal/platform/app/customer_intelligence_customer_data_adapter.go`;
- interfaces mínimas em `customerdata` listadas pelo pacote;
- testes e AGENTs correspondentes.

### CI05-ADAPTER-ERP-01

**Escrita permitida:**

- `back/internal/platform/app/customer_intelligence_erp_adapter.go`;
- nova interface/DTO público mínimo em `back/internal/modules/crm/erp/`;
- testes e AGENT do ERP.

Não edita raw nem queries existentes sem pacote separado do owner.

### CI05-ADAPTER-CALENDAR-01

**Escrita permitida:**

- `back/internal/platform/app/customer_intelligence_calendar_adapter.go`;
- interface/DTO mínimo em `back/internal/modules/calendar/`;
- testes e AGENT do Calendar.

Não altera workflow do Calendar.

### CI05-ADAPTER-SITE-01

**Escrita permitida:**

- `back/internal/platform/app/customer_intelligence_site_adapter.go`;
- interface/DTO mínimo em `back/internal/modules/site/`;
- testes e AGENT do Site.

### CI05-ADAPTER-BI-01

**Escrita permitida:**

- `back/internal/platform/app/customer_intelligence_bi_adapter.go`;
- interface/DTO mínimo em `back/internal/modules/bi/`;
- testes e AGENT do BI.

Preserva dataset registry, filtros, bloqueio de chamadas e limites atuais.

### CI05-FE-CONTRACT-01

Este é o contrato de handoff para `CI08-SOURCES-03`; não deve ser executado como uma segunda
implementação frontend.

**Escrita permitida quando CI-08 despachar:**

- `web/app/domain/customer-intelligence/source-api.ts`;
- `web/app/domain/customer-intelligence/source-types.ts`;
- `web/app/composables/customer-intelligence/useIntelligenceSources.ts`;
- testes correspondentes.

### CI05-FE-UI-CONTRACT-01

**Escrita permitida quando CI-08 despachar `CI08-SOURCES-03`:**

- `web/app/pages/inteligencia-clientes/fontes.vue`;
- `web/app/components/customer-intelligence/sources/IntelligenceSourcesCatalog.vue`;
- `web/app/components/customer-intelligence/sources/IntelligenceSourceConfigDrawer.vue`;
- `web/app/components/customer-intelligence/sources/IntelligenceSourceHealth.vue`;
- testes correspondentes.

Gates globais/nav só entram num pacote CI-08 explicitamente coordenado.

### CI05-FE-MANUAL-CONTRACT-01

**Escrita permitida quando CI-08 despachar a experiência offline:**

- `web/app/components/customer-data/offline/OfflineInteractionTimeline.vue`;
- `web/app/components/customer-data/offline/OfflineInteractionDrawer.vue`;
- `web/app/components/customer-data/offline/OfflineAttachmentUpload.vue`;
- `web/app/components/customer-data/offline/OfflineImportDialog.vue`;
- `web/app/composables/customer-data/useOfflineInteractions.ts`;
- `web/app/domain/customer-data/offline-api.ts`;
- `web/app/domain/customer-data/offline-types.ts`;
- testes correspondentes.

Usa somente APIs Customer Data da CI-03. É proibido chamar LLM/n8n/storage diretamente ou marcar
scan/processamento como concluído no frontend.

### CI05-QA-01

**Escrita permitida:** testes dentro das áreas acima e evidências sob
`docs/customer-intelligence/evidence/CI-05/`.

### CI05-CUTOVER-01

**Escrita permitida:** feature flags/bindings do composition root listados nominalmente,
configuração de módulo e evidências. Não ativa sync externo em produção sem aprovação.

### Sempre proibido

- qualquer arquivo de `socialpublishing`;
- workflows n8n nesta spec;
- sender/provider/outbox de canal;
- `automation/workflow-whatsapp.json`, WAHA e workflows Calendar/Operação;
- migrations aplicadas;
- secrets, `.env`, volumes e dados reais;
- proxy aberto, SQL ou URL livre.

## 17. Testes e comandos

Backend:

```powershell
cd back
go test ./internal/modules/customerintelligence/...
go test ./internal/modules/crm/erp/... ./internal/modules/calendar/... ./internal/modules/site/... ./internal/modules/bi/...
go test ./internal/platform/app/...
go test -race ./internal/modules/customerintelligence/...
go test ./...
golangci-lint run ./...
```

Frontend, somente após pacote UI:

```powershell
npm --prefix web run test
npm --prefix web run lint
npm --prefix web run build
```

Cenários:

- duplicate source key no boot;
- campo/filtro/config desconhecido;
- owner module ausente;
- retry/event replay;
- offline create/list funciona sem Customer Intelligence;
- upload offline bloqueia MIME/size/scan inválidos e nunca usa JSON/base64;
- CSV offline dry-run/apply relata aceita/rejeitada/quarentena sem auto-merge;
- outbox offline repetida não duplica observação/claim;
- lease/dead-letter;
- source disabled durante run;
- stale/required/optional;
- CPF genérico/invalid;
- visitor/session sem conversão;
- BI sem filtro e período >31 dias;
- Calendar write sem approval;
- segredo write-only;
- account/client/relationship negativos;
- troca de conta/cliente no browser;
- disable preview/revision/rebuild;
- abrir página sem disparar fonte externa.

Browser obrigatório por papel-alvo, desktop/mobile/tema, loading/vazio/erro/stale/disabled.

## 18. Rollout

1. registry e descriptors sem chamadas;
2. configs em draft e catálogo no painel;
3. adapter com test/health somente;
4. ingestão shadow por uma fonte/cliente;
5. comparação de observações/dedupe/freshness;
6. habilitar sync limitado;
7. fonte seguinte somente após métricas da anterior;
8. BI permanece on-demand;
9. suggestions apenas informativas;
10. ações escritas continuam desabilitadas até policy própria.

## 19. Rollback

- desabilitar config interrompe novos jobs;
- jobs já claimed respeitam cancelamento cooperativo;
- evidências coletadas seguem a decisão histórica/retention, não são apagadas;
- cursor só avança após sucesso;
- adapter anterior pode ser reativado sem duplicar pela idempotency key;
- falha de painel não afeta worker headless;
- rollback nunca muda sender/canal;
- módulo owner continua autoritativo.

## 20. Critérios de aceite

- [ ] Todas as fontes vêm de registry allowlisted.
- [ ] Interfaces de evidência, business context, agregado e ação são distintas.
- [ ] Nenhum adapter executa SQL cross-module.
- [ ] Repetir evento não duplica observação.
- [ ] Entrada/UI offline é utilizável independentemente do runtime inteligente.
- [ ] Anexos usam storage privado + scan e nunca payload/base64.
- [ ] Import offline exige dry-run, revisão e relatório por linha.
- [ ] Falha de fonte não bloqueia o chat.
- [ ] Stale/ausente/unavailable aparecem explicitamente.
- [ ] CPF genérico/nome não fazem auto-match.
- [ ] Tracking não vira pessoa sem conversão.
- [ ] BI não faz fan-out, busca aberta ou sync integral.
- [ ] Segredo nunca retorna ou reidrata.
- [ ] Desabilitar separa ingestão, histórico, derivados e retenção.
- [ ] Sugestão aceita não habilita fonte.
- [ ] Painel é totalmente configurável dentro do descriptor/policies.
- [ ] Página aberta não chama fonte externa automaticamente.

## 21. Stop conditions

Parar quando:

- CI-00/CI-03 divergir dos IDs/escopos usados;
- source owner não fornecer porta pequena e a solução exigir SQL privado;
- descriptor precisar aceitar URL/query/tabela livre;
- segredo precisar aparecer em response/workflow/log;
- ERP match exigir nome/CPF genérico;
- BI exigir buscar tudo ou remover filtro;
- Calendar write não possuir aprovação/policy;
- ingestão depender somente do bus in-memory;
- implementação tocar worktree do usuário fora da allowlist;
- pacote exigir workflow, canal ou social-publishing;
- teste tenant negativo falhar.

## 22. Handoff obrigatório

- baseline/worktree;
- descriptor e owner de cada source key;
- arquivos permitidos/alterados;
- módulos opcionais presentes/ausentes;
- config/schema/fields permitidos;
- testes, smokes e chamadas externas realmente executadas;
- volume, página, filtro, duração e custo observados;
- contagens de jobs/runs/dedupe/dead-letter;
- segredo validado como write-only;
- clientes/fontes ainda em shadow;
- rollout/rollback;
- confirmação de nenhum workflow, sender, secret ou social-publishing alterado.
