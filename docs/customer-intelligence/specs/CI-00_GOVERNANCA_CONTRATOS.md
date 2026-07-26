# CI-00 — Governança, vocabulário e contratos

- **Status:** READY — validada pelo product owner em 2026-07-23
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Arquitetura/Plataforma
- **Dependências:** [GOVERNANCA.md](../GOVERNANCA.md) e
  [SPECS_GERAIS.md](../SPECS_GERAIS.md), nas versões vigentes
- **Autoriza implementação:** sim, local e aditiva; sem deploy/cutover/exclusão

> O product owner aprovou os nomes, boundaries, contratos e execução do pacote. Permanecem
> bloqueados workflow/import, deploy, cutover, exclusão e decisões jurídicas/privacidade ainda
> marcadas como gates de produção.

## 1. Resultado único e verificável

Produzir um contrato sem ambiguidades para que Omnichannel, Customer Data e Customer Intelligence
possam ser implementados em paralelo, mantendo:

- PostgreSQL e Go como autoridades;
- chat funcional sem Customer Intelligence;
- inteligência headless sem painel ou conversa ativa;
- prompt específico por processo como principal camada de comportamento sem permitir que prompt
  substitua segurança, escopo, policy operacional ou sender;
- um único owner e writer por entidade em cada etapa.

O resultado é considerado validado quando o owner aprovar explicitamente as decisões desta spec e
as registrar como `READY`. A aprovação não libera todas as CI seguintes automaticamente; cada uma
mantém seus próprios bloqueios.

## 2. Baseline comprovada no disco

| Item | Estado em 2026-07-23 |
|---|---|
| módulo operacional | `omnichannel`, schema `messaging`, sort order 47 |
| CRM comercial | módulo `crm`, schema lógico `crm`, APIs atuais `/v1/erp/*` e `/v1/catalog/*` |
| contato/CRM 360 | ainda em `messaging.contacts` e tabelas `messaging.contact_*` |
| IA de atendimento | ainda construída dentro de `omnichannel.Module.Build` |
| vínculo cliente/IA/canal | `messaging.automation_profiles` |
| envio | mensagem `PENDING` + `messaging.outbox` + adapter Omnichannel |
| bus entre módulos | `platform/events.InMemoryBus`, síncrono e não durável |
| dependências de módulo | `Metadata.RequiresModules` e `OptionalModules` já existem |
| catálogo de clientes | adapter de `tenants.Service.ListAccessible`, permission-scoped |
| permissão consolidada | `core.organization.consolidated_read` já existe |
| última migration observada | `0238_social_publishing_reliability.sql`; nenhum próximo número é reservado |

Não foram consultadas contagens de banco nesta rodada documental. Qualquer spec de backfill deve
medir a instância alvo antes de sair de `DRAFT`.

### 2.1 Leitura obrigatória para execução

- `AGENT.md`;
- `docs/customer-intelligence/GOVERNANCA.md`;
- `docs/customer-intelligence/SPECS_GERAIS.md`;
- documentos canônicos em `docs/omnichannel/`;
- AGENTs de Omnichannel, CRM/ERP, Calendar, Site, `platform/events` e `platform/modules`;
- `back/internal/platform/modules/module.go`;
- module metadata e migrations atuais;
- `automation/AGENT.md` somente para preservar ownership.

Outros arquivos podem ser lidos para reconciliar consumidores. Ler não amplia a allowlist de
escrita.

## 3. Resolução proposta das decisões canônicas

Esta spec não cria uma segunda sequência de decisões. Ela resolve ou mantém pendentes os IDs
`CI-DEC-*` de `GOVERNANCA.md` §17. “Proposta congelada” significa apenas que as specs CI-01 a CI-10
devem usar a mesma proposta enquanto o owner avalia; não significa aprovação nem implementação.

| ID canônico | Resolução proposta nesta spec | Estado |
|---|---|---|
| `CI-DEC-001` | ID `customer_intelligence`, package `customerintelligence`, schema `intelligence` | proposta DRAFT |
| `CI-DEC-002` | módulo próprio `customer_data`, package `customerdata`, schema `customer_data`; não será subdomínio de `crm` | proposta DRAFT |
| `CI-DEC-003` | subject deduplicável somente dentro do `account_id` owner | proposta DRAFT |
| `CI-DEC-004` | lifecycle, fatos, consentimentos, histórico e recomendações isolados por `relationship_id` | proposta DRAFT |
| `CI-DEC-005` | binding operacional pertence ao Omnichannel e independe de IA | proposta DRAFT |
| `CI-DEC-006` | authority varia por tipo/fonte/frescor; LLM não vence dado verificado | proposta DRAFT |
| `CI-DEC-007` | evento crítico usa outbox durável e consumidor idempotente | proposta DRAFT |
| `CI-DEC-008` | indivíduo cross-client desabilitado; portfólio começa agregado/anônimo | proposta DRAFT |
| `CI-DEC-009` | observação usa referência/hash e snapshot allowlisted | proposta DRAFT |
| `CI-DEC-010` | métricas numéricas ficam para CI-10 antes de produção | pendente |
| `CI-DEC-011` | retenção por categoria/fonte exige policy aprovada | pendente |
| `CI-DEC-012` | contatos misturados entram em quarentena/revisão | proposta DRAFT |
| `CI-DEC-013` | match forte cross-client gera candidato restrito, sem vínculo automático | proposta DRAFT |
| `CI-DEC-014` | standalone não-agência usa client=account; agência exige client explícito, ativo, acessível e da mesma organização | proposta DRAFT |
| `CI-DEC-015` | papéis jurídicos/base legal/revogação | pendente jurídico |
| `CI-DEC-016` | tombstone, anonimização, legal hold e backups | pendente de policy |
| `CI-DEC-017` | criptografia/HMAC e rotação | pendente de segurança |
| `CI-DEC-018` | `subject_type` suporta `person` e `organization` | proposta DRAFT |
| `CI-DEC-019` | context snapshot minimizado, criptografado e com TTL | proposta DRAFT; parâmetros pendentes |
| `CI-DEC-020` | portfólio exige coorte mínima e supressão | proposta DRAFT; números pendentes |
| `CI-DEC-021` | prompt separado por `process_key` estável/depreciável | proposta DRAFT |
| `CI-DEC-022` | configuração segura é PostgreSQL + painel | proposta DRAFT |
| `CI-DEC-023` | platform guardrail não é sobrescrito por tenant/prompt | proposta DRAFT |
| `CI-DEC-024` | version usa draft→validated→published→archived; eval e rollout/canary/rollback são máquinas separadas; published é imutável | proposta DRAFT |
| `CI-DEC-025` | prompt, schema, tools, fontes, modelo e policy formam binding versionado | proposta DRAFT |
| `CI-DEC-026` | pipeline estruturado/versionado compõe processos com `ProcessResult` próprio | proposta DRAFT |
| `CI-DEC-027` | closure preserva final reply e só é aceita/executada pelo Omnichannel | proposta DRAFT |
| `CI-DEC-028` | segmento pertence a Customer Data, usa AST fechado e versões imutáveis; LLM/SQL livre não o executam | proposta DRAFT |
| `CI-DEC-029` | membership não equivale a consentimento; exportação/marketing têm permissão, finalidade e gates próprios | proposta DRAFT |
| `CI-DEC-030` | migração de prompts separa lifecycle funcional do estado de import e exige mapping/split revisável antes de publish | proposta DRAFT |
| `CI-DEC-031` | áudio/transcrição e vídeo permanecem no runtime legado até process keys, schemas, prompts e thresholds próprios aprovados | proposta DRAFT |

Requisitos derivados, sem criar novos IDs de decisão:

- APIs: `/v1/customer-data/*` e `/v1/customer-intelligence/*`;
- workspace: `/inteligencia-clientes`;
- `account_id` é o nome físico; `owner_account_id` é somente alias de domínio/DTO;
- `relationship_id` vincula exatamente account, client e subject;
- Customer Intelligence requer Customer Data; fontes são opcionais;
- Omnichannel trata os dois módulos novos como capacidades opcionais;
- IA propõe; Omnichannel valida, grava `PENDING` e envia;
- `process_key` publicado nunca é reutilizado com outro significado;
- preview, materialização e exportação são operações distintas, auditáveis e sem envio.

### 3.1 Decisões ainda externas a esta spec

Permanecem bloqueadores de produção, mesmo após a aprovação arquitetural:

- papéis jurídicos, base legal, revogação e tratamento de dados sensíveis;
- retenção por categoria/fonte, legal hold, backup, anonimização e crypto-shredding;
- algoritmo e rotação de criptografia/HMAC;
- limiar de coorte e supressão de reidentificação;
- métricas quantitativas de shadow→canary→produção;
- conteúdos permitidos em eventual ativação individual cross-client.

Esses itens não autorizam defaults silenciosos. CI-03 pode estruturar campos e gates, mas não pode
habilitar produção com PII/cross-client sem a decisão correspondente.

## 4. Módulos, dependências e modo degradado

| ID | Schema | `RequiresModules` | `OptionalModules` | Sort order proposto |
|---|---|---|---|---:|
| `customer_data` | `customer_data` | `core` | `omnichannel`, `crm`, `site` | 44 |
| `customer_intelligence` | `intelligence` | `core`, `customer_data` | `omnichannel`, `crm`, `calendar`, `site` | 45 |
| `omnichannel` | `messaging` | sem nova obrigatória | `customer_data`, `customer_intelligence` | 47, preservado |

Regras:

- `RequiresModules` trata disponibilidade estrutural no Registry. Habilitação por account continua
  em `core.account_modules`.
- Customer Intelligence não pode ser habilitado para uma account sem Customer Data habilitado.
- Desabilitar Customer Intelligence não desabilita Customer Data nem Omnichannel.
- Desabilitar Customer Data impede novos perfis inteligentes, mas não bloqueia webhook, mensagem,
  fila, resposta humana ou outbox do canal.
- Omnichannel mantém participante local em `messaging.contacts` durante a transição.
- Falha de uma fonte opcional produz contexto parcial com `sourceStatus`; nunca inventa fallback.
- IDs e sort orders só entram no catálogo depois de aprovação, pois `Module.ID()` é estável.
- BI já pode ser source adapter allowlisted, mas o package atual não implementa
  `modules.Module`; portanto `bi` não entra em `OptionalModules` até possuir ID estável no Registry.

## 5. Vocabulário contratual

| Nome de negócio | Campo físico/DTO | Regra |
|---|---|---|
| workspace owner | DB `account_id`; DTO `ownerAccountId` apenas em envelopes cross-module | sempre derivado do Principal ou gateway autenticado |
| cliente da agência | `client_account_id` / `clientAccountId` | validado no catálogo permission-scoped |
| pessoa/empresa | `subject_id` / `subjectId` | escopo máximo = workspace owner |
| relação | `relationship_id` / `relationshipId` | escopo obrigatório de dados de cliente |
| participante local | `contact_id` / `contactId` | entidade operacional do canal, não substitui subject |
| canal conectado | `channelResourceId` | WhatsApp instance ou Instagram account tipado |
| fonte | `source_key` / `sourceKey` | chave estável registrada em código |
| processo de IA | `process_key` / `processKey` | chave estável registrada no Prompt Registry |

Nenhum contrato novo usará apenas `client`, `customer`, `contact` ou `tenant` quando houver risco de
confundir cliente da agência, consumidor final, participante do canal ou workspace.

## 6. Catálogo de permissões

Todas as permissões abaixo precisam de enforcement no backend por account selecionada. Não basta
ler `Principal.Permissions`, pois o estado atual documenta que a RBAC efetiva é resolvida por
account. O consumidor deve declarar uma porta pequena, implementada pelo Core no composition root:

```go
type AccountPermissionAuthorizer interface {
    HasAccountPermission(ctx context.Context, principal auth.Principal, accountID, key string) (bool, error)
}
```

Ausência/erro do authorizer nega mutações e dados sensíveis; nunca permite por fallback de papel.

### 6.1 Customer Data

| Key | Scope | Autoriza |
|---|---|---|
| `customer_data.subjects.view` | account | listar/detalhar subjects no client scope permitido |
| `customer_data.subjects.manage` | account | criar/editar subject e perfil determinístico |
| `customer_data.relationships.view` | account | ver relação específica do cliente |
| `customer_data.relationships.manage` | account | editar lifecycle, owner, tags e campos |
| `customer_data.identities.view` | account | ver identidades permitidas, mascaradas por padrão |
| `customer_data.identities.manage` | account | adicionar/verificar/revogar identidade |
| `customer_data.notes.view` | account | ver notas permitidas no relationship scope |
| `customer_data.notes.manage` | account | criar/editar/arquivar notas |
| `customer_data.offline_interactions.view` | account | ver interações offline e metadados de anexos permitidos |
| `customer_data.offline_interactions.manage` | account | registrar/editar/arquivar interação offline |
| `customer_data.offline_interactions.import` | account | executar import controlado e revisar relatório |
| `customer_data.consents.view` | account | ver estado/histórico de consentimento permitido |
| `customer_data.consents.manage` | account | registrar concessão, revogação e evidência |
| `customer_data.merge.manage` | account | revisar match, merge e undo |
| `customer_data.segments.view` | account | ver definições, versões, runs e membros permitidos no client scope |
| `customer_data.segments.manage` | account | criar/editar/arquivar segmento e seu draft estruturado |
| `customer_data.segments.publish` | account | publicar versão imutável e fazer rollback do binding ativo |
| `customer_data.segments.evaluate` | account | executar preview e materialização dentro dos budgets |
| `customer_data.segments.export` | account | solicitar/baixar exportação separada após gates de finalidade e consentimento |
| `customer_data.audit.view` | account | consultar auditoria determinística |

### 6.2 Customer Intelligence

| Key | Scope | Autoriza |
|---|---|---|
| `customer_intelligence.profile.view` | account | fatos, sínteses e recomendações da relação |
| `customer_intelligence.profile.manage` | account | revisar, aceitar, rejeitar ou superseder claims/fatos/recomendações derivadas |
| `customer_intelligence.sources.view` | account | catálogo, health e cobertura, sem segredo |
| `customer_intelligence.sources.manage` | account | habilitar/desabilitar/configurar fonte allowlisted |
| `customer_intelligence.agents.manage` | account | agentes, modelo e bindings permitidos |
| `customer_intelligence.prompts.view` | account | ver processo, versões, diff e resultados permitidos |
| `customer_intelligence.prompts.manage` | account | criar/editar draft e casos de teste |
| `customer_intelligence.prompts.publish` | account | publicar, canary e rollback no escopo permitido |
| `customer_intelligence.prompts.platform_manage` | platform | alterar guardrail/base global |
| `customer_intelligence.runs.view` | account | runs, custo, latência e erro sanitizado |
| `customer_intelligence.audit.view` | account | auditoria inteligente |
| `customer_intelligence.portfolio.view` | account | agregado organizacional com gates cumulativos |
| `customer_intelligence.portfolio.manage` | account | gerar/revisar oportunidade agregada com gates cumulativos |
| `customer_intelligence.portfolio.platform_manage` | platform | guardrails globais de coorte, categorias e supressão |

Portfólio exige cumulativamente:

1. account selecionada é agência ativa e pertence à organização autorizada;
2. principal possui `core.organization.consolidated_read` nessa organização;
3. principal possui `customer_intelligence.portfolio.view|manage` na account de agência;
4. finalidade/policy aprovada;
5. targets pertencem à mesma organização e estão no escopo consolidado permitido;
6. consulta agregada; indivíduo continua bloqueado por default.

As permissões `portfolio.view/manage` são concedíveis por role/override account-scoped. O guardrail
`portfolio.platform_manage` não é concedível a role de account e continua exclusivo de
`platform_admin`; ele não é requisito para o uso normal dentro dos limites globais.

Nenhum mapeamento temporário de `omnichannel.agents.manage`, `omnichannel.contacts.manage` ou
`omnichannel.audit.view` concede automaticamente segmentos, fontes, prompts, portfólio ou
auditoria nova.

### 6.3 Role templates propostos

Templates só podem ser sincronizados depois da validação final, pois os já existentes não são
sobrescritos pelo Registry.

| ID | Permissões |
|---|---|
| `customer_data.viewer` | subjects.view, relationships.view, identities.view, notes.view, offline_interactions.view, consents.view, segments.view |
| `customer_data.steward` | todas as permissões Customer Data, exceto `segments.export` |
| `customer_data.segment_analyst` | subjects.view, relationships.view, consents.view, segments.view, segments.evaluate |
| `customer_data.segment_manager` | segment_analyst + segments.manage + segments.publish |
| `customer_intelligence.viewer` | profile.view, sources.view, prompts.view, runs.view |
| `customer_intelligence.manager` | viewer + profile.manage + sources.manage + agents.manage + prompts.manage + prompts.publish + audit.view |

Permissões de portfólio não entram nos templates account default, mas podem ser concedidas
explicitamente na account de agência. `portfolio.platform_manage` e `prompts.platform_manage`
permanecem fora de qualquer role/override de account.

`customer_data.segments.export` também fica fora dos templates default e exige concessão explícita
na account. Todas as cinco permissões de segmento continuam account-scoped e são insuficientes
sozinhas: o service revalida `client_account_id` pelo catálogo permission-scoped em toda leitura,
mutação, avaliação e exportação. Não existe permissão que transforme um segmento individual em
cross-client.

### 6.4 Contrato transversal de segmentação

Segmentação CRM/marketing é uma capability determinística de Customer Data:

1. `segmentId` é a identidade estável de negócio; nome, descrição e binding ativo podem evoluir;
2. cada alteração de regra cria ou edita um draft com revision; publicar cria uma versão imutável;
3. rollback apenas aponta o binding ativo para uma versão publicada anterior e nunca a edita;
4. o filtro usa AST fechado e versionado, com `fieldKey` e operador vindos de catálogo server-side;
5. SQL, JSONPath, URL, regex sem catálogo, expressão ou nome físico de tabela/coluna são proibidos;
6. preview e materialização sempre criam `evaluationRunId`, `asOf` e trilha de auditoria;
7. preview não persiste membros; materialização persiste projeção derivada reconstruível;
8. membership não comprova consentimento, base legal, elegibilidade de canal ou autorização de
   envio;
9. exportação é workflow separado, com permissão própria e revalidação atual de finalidade,
   consentimento, expiração, campos e client scope;
10. exportar nunca envia mensagem nem aciona campanha.

Uma IA de sugestão de filtro é capability futura e permanece **BLOCKED** nesta spec. Ela só poderá
entrar após nova decisão de catálogo, com `process_key`, input/output schema, prompts, evals,
thresholds, permissões e rollout próprios. Até lá, nenhum prompt ou runtime propõe AST. Customer
Data e a tela de segmentos são integralmente determinísticos e independem de Customer Intelligence.

## 7. Configurabilidade: painel, prompt, policy e invariante

### 7.1 Regra de produto

Toda decisão de comportamento segura para administração deve:

1. possuir configuração tipada;
2. ser persistida no PostgreSQL;
3. ser editável no painel por permissão específica;
4. suportar draft/teste/publicação/rollback quando afeta IA;
5. registrar autor, escopo, diff, versão e instante;
6. funcionar sem o painel aberto.

Configuração não pode ficar apenas em Vue, store, `localStorage`, n8n, env ou prompt oculto.

### 7.2 Matriz obrigatória

| Exemplo | Prompt versionado | Policy estruturada | Invariante Go/PostgreSQL |
|---|---:|---:|---:|
| persona, linguagem, tom, abordagem | sim | não | output validado |
| objetivo do processo e estratégia da resposta | sim | opcional | processo/saída registrados |
| perguntas e campos que a IA deve tentar coletar | sim | schema de campos | tipos/tamanho/sensibilidade |
| quando sugerir produto ou follow-up | sim | janela/cadência/limite | consentimento e canal |
| confiança mínima para automação | prompt pode explicar | sim | comparação no Go |
| horários, máximo de turnos, timeout e tokens | não | sim | bounds no Go |
| modelo, temperatura e rollout | não | sim | catálogo/bounds e auditoria |
| tools, fontes e knowledge | prompt pode orientar uso | binding allowlisted | registry e autorização |
| tenant/client/subject/relationship | não | não | derivação e filtro obrigatório |
| permissão, autenticação e módulo habilitado | não | não | obrigatório |
| FSM, lease, `ai_generation` e takeover humano | não | não | obrigatório |
| dedupe, idempotência, mensagem `PENDING` e outbox | não | não | obrigatório |
| janela/capacidade real do provider e sender | não | policy tipada | Omnichannel decide/envia |
| consentimento, opt-out, retenção e dado sensível | prompt apenas recebe restrição | policy aprovada | gate obrigatório |
| schema de saída e limites de payload | não | versão selecionada | validação obrigatória |
| regra determinística de segmento | não | AST versionado + policy de avaliação | account/client e compiler fechado |
| agenda, budget e freshness da materialização | não | sim | caps, lease e idempotência |
| exportação de membros | não | finalidade/canal/campos/TTL aprovados | permissão e consentimento atuais |

Se uma necessidade não se encaixar claramente, a implementação para e registra decisão em ADR;
não esconde comportamento em prompt ou `if` hardcoded.

### 7.3 Inventário de cobertura administrativa

Cada comportamento/capability de produto deve aparecer em um catálogo de cobertura com:

- `behaviorKey` estável;
- processo/módulo owner;
- classificação `prompt`, `structured_policy`, `capability`, `invariant`;
- escopos editáveis (`platform`, `agency`, `client`, `agent`, `channel`);
- permissão de view/manage/publish;
- tela/campo do painel;
- tabela/versão autoritativa;
- default e bounds;
- efeito de disable;
- dependências e modo degradado;
- audit event;
- teste e rollback.

Comportamento seguro sem superfície administrativa ou sem justificativa de invariante bloqueia a
liberação. Invariante deve ser visível/documentado no painel de auditoria, mas não editável por
tenant. Isso torna “tudo customizável” verificável sem transformar segurança em configuração.

## 8. Prompt Registry

### 8.1 Catálogo inicial de `process_key`

| `process_key` | Owner | Resultado principal |
|---|---|---|
| `conversation.triage` | Customer Intelligence | intenção, etapa, campos e necessidade de humano |
| `conversation.reply` | Customer Intelligence | texto proposto ao consumidor |
| `conversation.handoff_summary` | Customer Intelligence | resumo/motivo para atendente |
| `memory.extract` | Customer Intelligence | claims candidatos |
| `profile.summary` | Customer Intelligence | síntese versionada da relação |
| `recommendation.follow_up` | Customer Intelligence | instante/canal/justificativa |
| `recommendation.offer` | Customer Intelligence | referências de produto/serviço |
| `recommendation.important_dates` | Customer Intelligence | datas candidatas e evidências |
| `source.suggest` | Customer Intelligence | lacunas e fonte allowlisted sugerida |
| `portfolio.opportunity` | Customer Intelligence | oportunidade agregada |
| `media.image_analysis` | Customer Intelligence | descrição/extração autorizada |
| `media.document_analysis` | Customer Intelligence | extração documental limitada |
| `quality.review` | Customer Intelligence | avaliação e feedback |

Regras:

- cada chave possui input/output schema próprios;
- adicionar chave exige catálogo em código, permissão, owner, schema, teste e documentação;
- renomear é criar chave nova e depreciar a antiga;
- processos diferentes não compartilham prompt implícito;
- `conversation.triage` não pode produzir texto como substituto silencioso de
  `conversation.reply`;
- processos de mídia não recebem binário ou URL sem autorização do owner da mídia.

### 8.2 Camadas e precedência

```text
platform_guardrail
  + agency_policy
  + client_policy
  + process_prompt
  + agent_override permitido
  + runtime_context minimizado
```

- `platform_guardrail` possui autoridade máxima e só é editável com
  `customer_intelligence.prompts.platform_manage`.
- `agency_policy` e `client_policy` neste diagrama são camadas textuais versionadas; thresholds,
  horários e limites continuam na structured policy da seção 7.
- Para comportamento não protegido, a especificidade é:
  `agent_override > client_policy > agency_policy > process_prompt`.
- Uma camada pode restringir a anterior; ampliar tool, fonte, dado sensível ou capacidade exige
  binding/policy estruturada autorizada, não apenas texto.
- O compilador usa delimitadores/roles determinísticos e registra todas as versões.
- Instrução encontrada em mensagem, documento, tool ou source é dado não confiável e nunca camada
  de prompt.

### 8.3 Contrato lógico do registry

| Entidade | Campos mínimos |
|---|---|
| `ProcessDefinition` | `processKey`, owner, status, input/output schema versions, variables allowlisted, tools/sources máximas |
| `PipelineDefinition` | `pipelineKey`, owner, entrypoint schema, allowed process keys e invariantes |
| `PipelineVersion` | owner/client, pipeline, versão, grafo/ordem fechados, policy refs, status, revision, checksum e autor |
| `AgentPipelineBinding` | owner/client/agent, pipeline version, rollout, status e revision |
| `PromptVersion` | id, process key, layer, scope type/id, version, content, status, checksum, author, createdAt, publishedAt |
| `PromptBinding` | id, owner/client/agent, process key, IDs de cada layer, model/policy/schema, tools/sources, status, revision |
| `PromptVariable` | key, type, required, classification, source, max length, default permitido |
| `PromptTestCase` | id, process key, fixture reference, expected assertions, PII mode, author |
| `PromptEvaluation` | binding/version, test case, status, scores, violations, cost, latency, evaluator version |
| `PromptRollout` | binding, from/to version, mode, percentage/allowlist, started/ended, actor, rollback reason |

As máquinas de estado são separadas:

```text
Prompt/Process/Pipeline/Agent Version:
draft --validate--> validated --publish--> published --archive--> archived
validated --edit--> draft

PromptEvaluation:
queued -> running -> passed | failed | error | cancelled
queued -------------------------------------> cancelled

PromptRollout:
shadow -> canary -> full
shadow | canary | full -> paused -> modo anterior auditado
shadow | canary | full | paused -> rolled_back | stopped
```

`tested`, `canary`, `active`, `rolled_back` e `deprecated` não são status de versão. Test/eval
produz `PromptEvaluation`; shadow/canary/full/pause/rollback pertencem a `PromptRollout`; binding
efetivo é resolvido separadamente. Draft é mutável com `revision`; editar uma versão validada a
devolve a draft e invalida a avaliação anterior. Published é imutável: alteração cria nova versão.
Rollback reponta o binding para published anterior e encerra o rollout, sem marcar nem editar a
versão como `rolled_back`. Archive preserva conteúdo/lineage e só ocorre sem binding efetivo.

### 8.4 Superfície administrativa contratada

| Método | Rota proposta | Permissão |
|---|---|---|
| GET | `/v1/customer-intelligence/processes` | `customer_intelligence.prompts.view` |
| GET | `/v1/customer-intelligence/pipelines` | `customer_intelligence.prompts.view` |
| POST | `/v1/customer-intelligence/pipelines/{pipelineKey}/drafts` | `customer_intelligence.prompts.manage` |
| POST | `/v1/customer-intelligence/pipeline-versions/{id}/publish` | `customer_intelligence.prompts.publish` |
| GET | `/v1/customer-intelligence/prompts` | `customer_intelligence.prompts.view` |
| POST | `/v1/customer-intelligence/prompts/{processKey}/drafts` | `customer_intelligence.prompts.manage` |
| PATCH | `/v1/customer-intelligence/prompt-versions/{id}` | `customer_intelligence.prompts.manage` |
| POST | `/v1/customer-intelligence/prompt-versions/{id}/validate` | `customer_intelligence.prompts.manage` |
| POST | `/v1/customer-intelligence/prompt-versions/{id}/test` | `customer_intelligence.prompts.manage` |
| POST | `/v1/customer-intelligence/prompt-versions/{id}/publish` | `customer_intelligence.prompts.publish` |
| POST | `/v1/customer-intelligence/prompt-bindings/{id}/rollback` | `customer_intelligence.prompts.publish` |

Nenhuma resposta retorna segredo. Prompt bruto só é retornado a quem pode vê-lo e com o escopo
correto; runs/logs usam IDs/checksums, não conteúdo integral.

## 9. Envelopes versionados entre módulos

### 9.1 `ContextRequest.v1`

| Campo | Tipo | Regra |
|---|---|---|
| `schemaVersion` | `"context.request.v1"` | obrigatório |
| `requestId` | UUID/string opaca | idempotência/correlação |
| `ownerAccountId` | UUID | derivado/autenticado, nunca confiado de UI |
| `clientAccountId` | UUID | validado no catálogo |
| `subjectId` | UUID nullable | ausente somente durante resolução |
| `relationshipId` | UUID nullable | obrigatório para fatos/contexto de cliente |
| `conversationId` | UUID nullable | referência, não FK cross-module |
| `processKey` | chave registrada | obrigatório |
| `purpose` | enum registrado | finalidade autorizada |
| `channel` | enum nullable | canal operacional |
| `asOf` | RFC3339 | corte temporal |
| `sourceKeys` | string[] | subconjunto allowlisted |
| `maxItems` | int | bound server-side |
| `maxTokens` | int | bound server-side |
| `locale` | string | normalizada |
| `correlationId` | string | cadeia auditável |

### 9.2 `ContextEnvelope.v1`

| Campo | Tipo | Regra |
|---|---|---|
| `schemaVersion` | `"context.envelope.v1"` | obrigatório |
| `requestId` | string | eco validado |
| IDs de escopo | UUIDs | iguais ao request autorizado |
| `snapshotId` | UUID | versão persistida/minimizada |
| `asOf` | RFC3339 | instante real do snapshot |
| `sections` | lista tipada | cada item informa classificação e origem |
| `sourceStatuses` | lista | `ok`, `partial`, `stale`, `disabled`, `error` + reason code |
| `evidenceRefs` | lista de IDs/hashes | sem payload bruto indiscriminado |
| `budget` | objeto | tokens/itens usados e truncamento |
| `promptBindingId` | UUID | binding resolvido |
| `expiresAt` | RFC3339 | TTL |
| `warnings` | reason codes[] | nunca texto com segredo |

### 9.3 `InteractionRequest.v1`

É o pedido de alto nível do Omnichannel para o pipeline de conversa. Não é uma chamada direta a
um prompt e, por isso, não carrega `processKey`.

| Campo | Tipo | Regra |
|---|---|---|
| `schemaVersion` | `"interaction.request.v1"` | obrigatório |
| `requestId`, `interactionId` | strings opacas | idempotência/correlação |
| IDs de escopo | UUIDs | owner/client/subject/relationship/conversation validados |
| `pipelineKey` | `"conversation.respond"` | entrypoint estruturado, não prompt |
| `aiGeneration` | bigint | geração capturada pelo Omnichannel |
| `message` | objeto fechado | mensagem agregada e minimizada |
| `operationalState` | objeto fechado | estado/FSM/lease/policy permitidos |
| `routingCatalog` | lista fechada | fila/setor válidos, somente para sugestão |
| `channelCapabilities` | objeto fechado | limites reais do canal |
| `purpose`, `locale`, `asOf` | valores normalizados | finalidade e corte temporal |
| `sourceKeys` | string[] | subconjunto allowlisted |
| `maxItems`, `maxTokens` | inteiros | bounds server-side |
| `deadlineAt`, `correlationId` | timestamp/string | cancelamento e tracing |

O coordenador do Customer Intelligence resolve uma versão publicada de pipeline e cria
internamente um `ContextRequest.v1` separado para cada `processKey`. O pipeline inicial é:

```text
conversation.triage
  -> policy estruturada
  -> conversation.reply quando permitido ou necessário para fechamento
  -> composição de InteractionDecision.v1
```

Pipeline é configuração estruturada e versionada pelo painel; não é mega-prompt. Uma versão não
pode remover a revalidação final do Omnichannel nem acrescentar processo/tool/source fora dos
catálogos autorizados.

O request não contém credencial persistível, URL livre, SQL, sender ou permissão de mutação.

### 9.4 `ProcessResult.v1`

Envelope interno do runtime para manter cada prompt independente:

| Campo | Tipo | Regra |
|---|---|---|
| `schemaVersion` | `"process.result.v1"` | obrigatório |
| `requestId`, `runId`, `interactionId` | strings opacas | correlação |
| IDs de escopo | UUIDs | iguais ao request autorizado |
| `processKey` | chave registrada | exatamente um processo |
| `outputSchemaVersion` | chave registrada | compatível com o processo |
| `output` | objeto fechado | validado pelo schema próprio |
| `promptBindingId` | UUID | binding efetivo |
| `promptVersionRefs` | lista | todas as layers efetivas |
| `modelRef`, `contextSnapshotId` | referências | sem segredo/payload bruto |
| `usage`, `warnings` | objetos tipados | custo/latência/degradação |

`conversation.triage` produz `conversation.triage.result.v1`, sem `replyDraft` e sem comando
operacional. `conversation.reply` produz `conversation.reply.result.v1`, com texto candidato e sem
sender. Resultado intermediário nunca é aceito como mensagem, handoff ou fechamento; o coordenador
compõe a decisão final e o Omnichannel ainda a revalida.

### 9.5 `InteractionDecision.v1`

| Campo | Tipo | Regra |
|---|---|---|
| `schemaVersion` | `"interaction.decision.v1"` | obrigatório |
| `requestId`, `decisionId` | string | correlação/idempotência |
| IDs de escopo | UUIDs | devem coincidir com request |
| `pipelineKey`, `pipelineVersionId` | referências | pipeline efetivo |
| `processRunRefs` | lista não vazia | run/process/binding/versions/schema de cada etapa |
| `aiGeneration` | bigint | revalidada pelo Omnichannel |
| `outcome` | `reply`, `handoff`, `no_reply` | proposta, não comando de provider |
| `replyDraft` | string nullable | limite configurado e validado |
| `needsHuman` | bool | coerente com outcome |
| `reasonCode` | chave allowlisted | sem texto arbitrário operacional |
| `departmentId`, `queueId` | UUID nullable | revalidados no catálogo local |
| `intent`, `categories`, `leadStage` | valores versionados | revalidados |
| `confidence` | 0..1 | não decide sozinho |
| `extractedClaims` | lista tipada | candidata, nunca fato direto |
| `toolResults` | refs sanitizadas | apenas tools autorizadas |
| `closure` | objeto nullable | `requested`, `reasonCode`, confiança, `humanRequested` e `sensitiveTopic`; nunca fecha sozinho |
| `usage` | tokens/custo/latência | auditável |
| `warnings` | reason codes[] | degradado/partial/stale |

Quando `closure.requested=true`, o pipeline também produz `replyDraft` final quando a policy exigir.
O Omnichannel passa a proposta para seu fluxo `SystemTryAutoClose`, que revalida geração, estado,
humano, sensibilidade e policy e grava mensagem/outbox/avaliação atomicamente. Não se converte
fechamento em `no_reply` nem se perde a resposta final do contrato `brain.result.v3`.

O envelope não possui `provider`, `send`, `sendNow`, token ou endpoint. Omnichannel pode rejeitar
uma decisão válida por estado, takeover, lease, policy, consentimento ou capacidade do canal.

## 10. Eventos iniciais

Envelope comum:

```json
{
  "eventId": "uuid",
  "schemaVersion": "event.v1",
  "topic": "module.entity.verb_past",
  "accountId": "uuid",
  "clientAccountId": "uuid",
  "aggregateId": "uuid",
  "occurredAt": "RFC3339",
  "causationId": "opaque",
  "correlationId": "opaque",
  "idempotencyKey": "opaque",
  "payload": {"idsOnly": true}
}
```

Catálogo inicial:

| Tópico | Owner | Consumidor inicial |
|---|---|---|
| `omnichannel.channel_binding.created` | Omnichannel | Customer Data |
| `omnichannel.channel_binding.reassigned` | Omnichannel | Customer Data |
| `omnichannel.channel_binding.ended` | Omnichannel | Customer Data |
| `omnichannel.conversation.client_bound` | Omnichannel | Customer Data |
| `omnichannel.interaction.accepted` | Omnichannel | Customer Intelligence |
| `customer_data.subject.resolved` | Customer Data | Customer Intelligence |
| `customer_data.relationship.changed` | Customer Data | Customer Intelligence |
| `customer_data.consent.changed` | Customer Data | Customer Intelligence |
| `customer_data.segment.version_published` | Customer Data | materializador/read models |
| `customer_data.segment.materialized` | Customer Data | read models/consumidores autorizados |
| `customer_data.segment.export_ready` | Customer Data | UI/notificação administrativa |
| `customer_intelligence.profile.updated` | Customer Intelligence | read models/UI |

Eventos carregam IDs e reason codes. O consumidor consulta o owner por porta tipada. Publicação
crítica nasce em outbox na mesma transação do estado autoritativo; consumo registra `eventId` único.

## 11. Feature/capability flags

Além de `core.account_modules`, o PostgreSQL deverá persistir capabilities server-side:

| Key | Escopo | Valores |
|---|---|---|
| `customer_data.identity_resolution` | owner/client | `off`, `shadow`, `on` |
| `customer_data.legacy_facade` | owner/client | `legacy`, `shadow`, `new` |
| `customer_data.segmentation` | owner/client | `off`, `shadow`, `on` |
| `customer_data.segment_exports` | owner/client | `off`, `shadow`, `on` |
| `customer_intelligence.profile` | owner/client | `off`, `shadow`, `on` |
| `customer_intelligence.runtime` | owner/client/channel | `off`, `shadow`, `canary`, `on` |
| `customer_intelligence.portfolio` | organization/owner | `off`, `shadow`, `on` |
| `omnichannel.customer_data_bridge` | owner/client/channel | `off`, `shadow`, `on` |
| `omnichannel.customer_intelligence_bridge` | owner/client/channel | `off`, `shadow`, `canary`, `on` |

Um flag não concede módulo, permissão, finalidade ou acesso a fonte. O valor efetivo é a interseção
de todos os gates.

## 12. Pacotes atômicos e allowlists

### CI00-DOC-APPROVAL

- **Resultado:** registrar aprovação/rejeição de cada decisão.
- **Pode escrever somente:**
  - `docs/customer-intelligence/GOVERNANCA.md`
  - `docs/customer-intelligence/SPECS_GERAIS.md`
  - `docs/customer-intelligence/specs/CI-00_GOVERNANCA_CONTRATOS.md`
- **Proibido:** qualquer arquivo em `back/`, `web/`, `automation/` ou migrations.

### CI00-MODULE-CATALOG

- **Executa somente após `READY`.**
- **Pode escrever somente:**
  - `back/internal/modules/customerdata/module.go`
  - `back/internal/modules/customerdata/AGENT.md`
  - `back/internal/modules/customerintelligence/module.go`
  - `back/internal/modules/customerintelligence/AGENT.md`
  - `back/internal/platform/app/app.go`
  - testes novos diretamente correspondentes nesses packages
- **Proibido:** migrations, workflows e módulos `automation`/`socialpublishing`.
- **Observação:** a criação funcional dos módulos pertence a CI-03 e CI-06; este pacote não deve
  deixar handle falso parecendo pronto.

### CI00-PROMPT-CONTRACT

- **Executa em CI-04/CI-06, não nesta spec.**
- **Pode escrever somente após despacho resolver os nomes exatos:**
  - modelos/validators do Prompt/Pipeline Registry dentro de
    `back/internal/modules/customerintelligence/`;
  - testes correspondentes no mesmo package;
  - documentação `AGENT.md` do módulo.
- **Proibido:** n8n, sender, provider de canal e `messaging.*`.

### CI00-SEGMENT-CONTRACT

- **Resultado:** congelar permissions, AST, lifecycle de versão, runs, materialização e separação
  entre membership, consentimento e exportação.
- **Pode escrever somente:**
  - documentos CI-00/CI-03/CI-08 nominalmente listados no despacho;
  - fixtures contratuais sanitizadas somente em pacote futuro específico.
- **Proibido:** código, migration, SQL livre, backfill, arquivo de exportação, campanha, sender,
  workflow ou ativação de capability.

## 13. Testes e provas exigidos

Comandos futuros, a partir de `back/`:

```text
go test ./internal/modules/customerdata/...
go test ./internal/modules/customerintelligence/...
go test ./internal/modules/omnichannel/...
go test ./internal/platform/app/...
go test ./...
```

Provas contratuais:

- matriz automatizada de permissões por account/client;
- módulo Customer Intelligence recusa enable sem Customer Data;
- Omnichannel constrói e recebe/responde humano com os módulos opcionais ausentes;
- cada `process_key` resolve exatamente um binding publicado;
- version, evaluation e rollout rejeitam status pertencente às outras state machines;
- pipeline resolve versão publicada e um `ProcessResult` por etapa;
- triage intermediária não causa efeito nem vira `no_reply`;
- closure preserva resposta final e segue para `SystemTryAutoClose`;
- draft não muda produção;
- placeholder/schema/tool/source inválido bloqueia publish;
- prompt não altera `accountId`, `clientAccountId`, FSM, sender ou catálogo;
- resultado atrasado com `aiGeneration` divergente não produz mensagem/outbox.
- AST de segmento rejeita field/operator desconhecido, profundidade/lista acima do cap e campo
  físico/SQL/JSONPath;
- preview cria run auditável, não persiste membership e usa `asOf` fixo;
- materialização aceita somente versão publicada e é idempotente por escopo/input;
- revogação posterior à materialização exclui a relação da exportação;
- exportação exige permissão própria, objeto privado com TTL e nunca aciona sender.

## 14. Rollout e rollback

Esta spec não possui rollout de runtime. Após aprovação:

1. sincronizar IDs/permissões somente em ambiente de desenvolvimento;
2. verificar catálogo sem deprecar keys não relacionadas;
3. manter capabilities `off`;
4. executar CI-01 e CI-02;
5. habilitar cada capability por client/channel.

Rollback do catálogo marca novos módulos/capabilities como desabilitados; não renomeia IDs nem
apaga permissões. Prompt publicado é revertido por binding para versão anterior, nunca editado.

## 15. Critérios de aceite

- [ ] owner aprovou nomes, schemas, rotas e topologia de IDs;
- [ ] nenhuma palavra “cliente” permanece ambígua nos envelopes;
- [ ] modo standalone e agência estão explícitos;
- [ ] dependências e modo degradado estão explícitos;
- [ ] catálogo de permissões foi aceito sem herança cross-client silenciosa;
- [ ] Prompt Registry possui processos separados e lifecycle auditável;
- [ ] version/evaluation/rollout possuem state machines separadas e published é imutável;
- [ ] Pipeline Registry compõe processos sem mega-prompt nem efeito intermediário;
- [ ] prompt, policy e invariante estão diferenciados;
- [ ] painel é superfície administrativa, não dependência do runtime;
- [ ] decisão IA e envio Omnichannel aparecem como autoridades distintas;
- [ ] segmento possui identidade estável, versão publicada imutável e rollback por binding;
- [ ] AST, preview, materialização, consentimento e exportação possuem contratos separados;
- [ ] nenhuma permissão ou consulta de segmento permite indivíduo cross-client;
- [ ] eventos críticos usam outbox/consumo idempotente;
- [ ] nenhuma exclusão ou alteração de workflow foi autorizada.

## 16. Stop conditions

Parar a implementação se:

- owner rejeitar ou renomear módulo/processo após catálogo sincronizado;
- não existir authorizer backend por account;
- a única opção for confiar em `account_id`/`client_account_id` do body;
- Customer Intelligence precisar ser obrigatório para boot/recebimento do Omnichannel;
- algum processo exigir SQL/URL/tool/source arbitrário;
- segmento exigir SQL/JSONPath/field físico arbitrário ou client implícito;
- membership for tratada como consentimento ou exportação for acoplada a envio;
- versão, evaluation e rollout compartilharem o mesmo enum/status;
- prompt for usado para substituir consentimento, FSM, idempotência ou sender;
- uma migration for numerada sem reinspecionar o maior número no disco;
- arquivo de allowlist estiver sujo ou sob ownership de outra trilha;
- qualquer executor precisar tocar `social-publishing`, módulo legado `automation` ou workflow de
  outro owner.

## 17. Handoff obrigatório

Ao concluir a validação, registrar:

- decisões aprovadas/rejeitadas e suas justificativas;
- catálogo final de permissões/processos;
- decisões jurídicas e de retenção ainda abertas;
- diff documental;
- confirmação de zero código/migration/workflow;
- specs desbloqueadas e ainda bloqueadas.
