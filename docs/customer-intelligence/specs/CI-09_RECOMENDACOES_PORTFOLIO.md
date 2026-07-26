# CI-09 — Recomendações, próxima ação e inteligência de portfólio

- **Status:** READY — implementação local autorizada; portfólio individual bloqueado
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Customer Intelligence
- **Depende de:** CI-04, CI-05 e CI-06
- **Integra com:** CI-03, CI-07 e CI-08
- **Desbloqueia:** CI-10
- **Autoriza implementação:** sim, com capabilities off e dados agregados/anônimos

> Esta spec define propostas e gates. Enquanto estiver em `DRAFT`, não autoriza gerar campanhas
> reais, compartilhar dados entre clientes, enviar mensagens, criar migration, publicar prompt ou
> ativar portfólio.

## 1. Resultado único e verificável

Entregar um motor explicável que:

- gere recomendações individuais de follow-up, oferta e data importante;
- escolha uma próxima ação entre recomendações elegíveis por policy determinística;
- use prompts específicos por processo, binding e cliente;
- registre evidências, contexto, versão de prompt/modelo e validade;
- permita aprovação, rejeição, expiração, execução autorizada e outcome;
- crie oportunidades de portfólio apenas a partir de agregados protegidos;
- nunca transforme recomendação em fato nem execute canal/módulo externo diretamente;
- mantenha dados individuais cross-client desabilitados por padrão.

O resultado é verificável quando uma recomendação pode ser explicada até suas evidências e versões,
produto/data inexistente é rejeitado, opt-out/quiet hours vencem o prompt e nenhuma consulta de
portfólio permite inferir ou exportar uma pessoa.

## 2. Requisitos locais derivados da governança

Os IDs `CI09-REQ-*` são requisitos desta spec. Novas decisões continuam sendo registradas somente
na sequência `CI-DEC-*` de `GOVERNANCA.md`.

| ID | Requisito |
|---|---|
| CI09-REQ-001 | recomendação é derivada, temporal e revogável; nunca é fato |
| CI09-REQ-002 | cada tipo gerado por LLM usa um `process_key` e output schema próprios |
| CI09-REQ-003 | prompt controla rationale/estratégia; policies estruturadas controlam elegibilidade e limites |
| CI09-REQ-004 | execução externa pertence ao módulo owner e exige nova validação/idempotência |
| CI09-REQ-005 | produto/serviço/data são referências a catálogos/fatos; o modelo não cria autoridade |
| CI09-REQ-006 | próxima ação é ranking determinístico de recomendações elegíveis no MVP |
| CI09-REQ-007 | portfólio começa agregado, anônimo, auditado e sem subject/relationship |
| CI09-REQ-008 | compartilhar ou ativar indivíduo entre clientes permanece `off` |
| CI09-REQ-009 | dado sensível e saúde possuem bloqueio adicional que prompt não pode relaxar |
| CI09-REQ-010 | toda customização segura é persistida, versionada e administrável pelo painel |

## 3. Dependências e bloqueios

| Dependência | Entrega exigida |
|---|---|
| CI-04 | recommendations, `CI04-DB-03` policies/bindings, evidence/facts, context snapshots, runtime/prompt refs e audit |
| CI-05 | `PortfolioAggregateSource`, source registry, freshness e supressão indicada pelo owner |
| CI-06 | runtime, Prompt Registry, evals, jobs e schemas por processo |
| CI-03 | subject/relationship/consent/client scope autoritativos |
| CI-08 | perfil, Prompt Studio e shell do workspace |

### 3.1 Alinhamento de persistência de portfólio

CI-04 já restringe `intelligence.recommendations` a
`follow_up|offer|important_date|next_action` no escopo
`client_account_id + subject_id + relationship_id` e declara que oportunidade cross-client usa
entidade própria em CI-09. Esta spec detalha essa entidade sem reabrir a tabela individual:

- persistir oportunidades agregadas em `intelligence.portfolio_opportunities`;
- persistir clientes-alvo permitidos em `intelligence.portfolio_opportunity_targets`;
- nunca persistir contributing subject IDs;
- incorporar essas tabelas na migration ainda não executada de CI-04 ou despachar uma migration
  aditiva própria e separada antes de CI09-BE-01.

CI-09 não autoriza esconder portfólio em JSON individual, usar UUID fictício ou tornar colunas de
escopo opcionais silenciosamente.

### 3.2 Outros bloqueios

- base legal, finalidade e papéis jurídicos;
- categorias de dado permitidas;
- piso de coorte e algoritmo de supressão;
- proteção contra differencing/repeated queries;
- retenção, anonimização, legal hold e backups;
- enforcement de `customer_intelligence.profile.manage` e das permissões cumulativas do owner;
- thresholds de confiança/qualidade/custo;
- catálogo e versionamento de produtos/serviços;
- contrato de ação autorizada por módulo owner.

Enquanto esses pontos estiverem abertos, `customer_intelligence.portfolio=off`.

## 4. Inventário real e fronteira

O estado documental atual já propõe:

- `intelligence.recommendations` com tipo, status, payload, rationale, confiança, validade e refs;
- `intelligence.source_observations`, claims, facts, summaries e context snapshots;
- `intelligence.prompt_bindings`, evaluations e rollouts;
- `PortfolioAggregateSource.QueryAggregate` com dimensions, metrics, filters, cohort size,
  suppression e freshness;
- processos `recommendation.follow_up`, `recommendation.offer`,
  `recommendation.important_dates` e `portfolio.opportunity`;
- `source.suggest` e tabela própria de sugestões de fonte.

Esta spec não possui:

- sender, scheduler de canal ou criação de campanha;
- autoridade de produto, pedido, calendário ou consentimento;
- SQL direto em ERP/BI/Calendar/Omnichannel;
- permissão para mover contato entre clientes;
- autorização para individual cross-client;
- um novo prompt genérico que substitua processos existentes.

## 5. Tipos e processos

| `recommendation_type` | Processo gerador | Resultado |
|---|---|---|
| `follow_up` | `recommendation.follow_up` | quando, por qual canal elegível e por quê retomar |
| `offer` | `recommendation.offer` | referências de produtos/serviços adequados e rationale |
| `important_date` | `recommendation.important_dates` | oportunidade relacionada a data sustentada por evidência |
| `next_action` | nenhum processo novo no MVP | ranking determinístico das recomendações elegíveis |
| `source` | `source.suggest` | permanece em `intelligence.source_suggestions`, não é ação comercial |
| `portfolio` | `portfolio.opportunity` | oportunidade agregada sem pessoa contribuinte |

Criar `recommendation.next_action` exigiria novo item no catálogo canônico, schema, permission,
evals e decisão `CI-DEC-*`. Até isso ocorrer, a IA não produz próxima ação livre; o Go escolhe entre
recomendações já validadas usando policy versionada.

## 6. Contrato comum de recomendação individual

O registro físico continua sendo o definido pela CI-04. O service expõe:

| Campo | Tipo/regra |
|---|---|
| `id` | UUID |
| `accountId`, `clientAccountId` | escopo derivado/validado |
| `subjectId`, `relationshipId` | obrigatórios |
| `recommendationType` | enum registrada |
| `status` | `proposed`, `approved`, `rejected`, `executed`, `expired`, `invalidated` |
| `payloadSchemaVersion` | versão fechada por tipo, dentro de `payload` se não houver coluna |
| `payload` | objeto fechado e limitado |
| `rationale` | texto sanitizado e limitado |
| `rationaleCodes` | códigos registrados |
| `confidence` | decimal 0..1 |
| `validFrom`, `expiresAt` | janela obrigatória |
| `evidenceRefs` | IDs permitidos, limitados e ordenados |
| `factRefs` | fatos/versões usados |
| `sourceRefs` | source key/snapshot/freshness |
| `promptDefinitionId` | definição do processo |
| `promptBindingId` | binding efetivo |
| `promptVersionRefs` | layers efetivas |
| `runtimeRunId` | run gerador |
| `contextSnapshotId` | snapshot utilizado |
| `modelRef`, `schemaVersion` | execução reproduzível |
| `recommendationPolicyBindingId` | binding estruturado efetivo |
| `recommendationPolicyVersionId` | versão imutável efetiva |
| `policyVersionRefs` | guards adicionais revalidados pelo owner |
| `approvedByUserId`, `decidedAt` | decisão humana quando aplicável |
| `outcome` | resultado observado sanitizado, não fato implícito |
| `createdAt`, `updatedAt` | auditoria |

CI04-DB-03 cria as colunas próprias e precisa estar concluída antes da geração. Binding/version,
prompt/model/context e evidência nunca ficam escondidos no payload.

## 7. Payloads fechados por tipo

### 7.1 `follow_up.v1`

| Campo | Regra |
|---|---|
| `recommendedAt` | RFC3339 e timezone |
| `windowStart`, `windowEnd` | dentro de policy |
| `suggestedChannel` | canal registrado e elegível |
| `cadencePolicyRef` | versão aplicada |
| `reasonCodes` | catálogo fechado |
| `conversationBrief` | objetivo curto, não mensagem pronta para provider |
| `evidenceRefs` | obrigatórias |
| `constraintsSnapshot` | quiet hours, cap, consent e channel eligibility avaliados |

O momento pode ser sugerido pelo processo, mas Go recalcula limites no momento da aprovação e
execução.

### 7.2 `offer.v1`

| Campo | Regra |
|---|---|
| `catalogOwnerModule` | chave registrada |
| `catalogItems` | `{itemType,itemId,versionRef}` limitados |
| `fitReasonCodes` | catálogo fechado |
| `fitNarrative` | texto limitado |
| `excludedItemReasonCodes` | opcional |
| `priceContextRef` | referência opcional; nunca preço autoritativo copiado |
| `validityCheckedAt` | instante da leitura |
| `evidenceRefs`, `factRefs` | obrigatórias |

Antes de exibir/usar, o backend revalida existência, disponibilidade, client scope, restrições e
versão no owner do catálogo.

### 7.3 `important_date.v1`

| Campo | Regra |
|---|---|
| `dateFactId`, `dateFactVersion` | fato tipado que sustenta a data |
| `dateValue` | eco normalizado para apresentação |
| `dateKind` | chave registrada |
| `verificationState` | `verified`, `resolved`, `contested` |
| `suggestedWindow` | janela de ação, não evento criado |
| `reasonCodes` | catálogo fechado |
| `evidenceRefs` | obrigatórias |
| `requiresReview` | true para conflito/baixa confiança |

Claim candidato sem fato resolvido pode gerar alerta de revisão, mas não recomendação acionável.
Uma data nunca é inventada pelo modelo nem confirmada só por aparecer em resumo.

### 7.4 `next_action.v1`

| Campo | Regra |
|---|---|
| `candidateRecommendationIds` | somente recomendações válidas da mesma relationship |
| `selectedRecommendationId` | uma das candidatas ou null |
| `rankingPolicyVersionId` | obrigatório |
| `scoreBreakdown` | fatores/códigos estruturados |
| `blockedReasonCodes` | consent, cap, stale, conflito, expiração etc. |
| `computedAt`, `expiresAt` | obrigatórios |

O ranking não altera o status das candidatas e não executa a selecionada.

## 8. Geração e lifecycle

### 8.1 Geração

1. request autenticado identifica relationship e tipos permitidos;
2. service valida módulo, permissão, consentimento, purpose e policy;
3. job durável recebe IDs e idempotency key;
4. CI-06 monta snapshot minimizado e resolve binding publicado por processo;
5. modelo retorna output estrito;
6. validator rejeita catálogo/data/source/evidence inexistente;
7. policy aplica elegibilidade e validade;
8. recomendação nasce `proposed` com refs completas;
9. evento/audit nasce na mesma transação;
10. UI recebe status por leitura/polling bounded ou realtime autorizado.

Nenhuma chamada LLM ocorre dentro de transação PostgreSQL.

### 8.2 Estados

```text
proposed -> approved -> executed
        \-> rejected
        \-> expired
        \-> invalidated

approved -> expired | invalidated
executed -> permanece histórico; outcome é anexado/versionado
```

Regras:

- `approved` não significa enviado/agendado;
- `executed` significa que o owner aceitou a ação, não que houve conversão;
- nova evidência pode invalidar/expirar, mas não reescreve histórico;
- decisão duplicada usa revision/idempotency key;
- rejeição exige reason code e pode alimentar eval sem treinar implicitamente;
- autoaprovação só pode existir para categoria de baixo risco explicitamente allowlisted;
- portfólio e dado sensível sempre exigem revisão humana no rollout inicial.

### 8.3 Execução

Uma ação aprovada vira `ActionRequest` para o módulo owner:

- permission do Customer Intelligence e do owner são cumulativas;
- owner revalida catálogo, consentimento, janela, estado e idempotência;
- Customer Intelligence nunca chama provider diretamente;
- WhatsApp/Instagram, quando aplicável, passam por Omnichannel, mensagem `PENDING` e outbox;
- Calendar/ERP/CRM gravam por seus services;
- falha externa registra status/outcome e permite retry classificado, sem duplicar.

## 9. Prompts, bindings e customização

Cada processo possui:

- definition e input/output schema próprios;
- prompt específico editável no Prompt Studio;
- agency/client/process/agent layers permitidas;
- binding, modelo, parâmetros, sources e tools versionados;
- fixtures e evals próprios;
- rollout e rollback independentes.

Prompts podem controlar:

- objetivo e estratégia da recomendação;
- tom/rationale;
- tipos de sinal a priorizar;
- perguntas/lacunas que devem virar sugestão de fonte;
- forma de comparar fit dentro do contexto autorizado.

Policies estruturadas configuráveis pelo painel controlam:

- confidence mínimo;
- validade;
- cadência/frequency cap;
- quiet hours;
- canais elegíveis;
- quantidade máxima de itens;
- categorias/produtos bloqueados;
- revisão/autoaprovação;
- budgets e rollout;
- fontes/tools permitidos;
- thresholds de staleness.

Essas policies usam `recommendation_policy_definitions`, versões imutáveis e bindings
agency/client/agent da CI-04. Alteração cria draft; validate/test/publish/rollback usam CAS,
auditoria e preview. O runtime registra a versão efetiva e o ranking `next_action` usa
`rankingPolicyVersionId` igual à policy version persistida, nunca uma configuração local.

Invariantes Go/PostgreSQL controlam account/client/relationship, permissão, consentimento,
opt-out, schema, idempotência, catálogo máximo, PII, outbox e sender.

Default prompt não pode ficar hardcoded como fallback oculto. O bootstrap inicial deve criar
versões controladas pelo mecanismo de CI-06; se não houver binding publicado, a geração falha com
reason code e não produz recomendação.

## 10. Portfólio agregado

### 10.1 Casos permitidos

Uma agência pode receber, por exemplo, uma sugestão de campanha para um cliente-alvo baseada em
padrões agregados do portfólio, desde que:

- não revele pessoa ou empresa contribuinte além do cliente-alvo autorizado;
- não transfira lead/contato entre clientes;
- use apenas dimensions/metrics registradas;
- tenha finalidade e base legal aprovadas;
- respeite coorte, supressão, retenção e categorias;
- registre query fingerprint, fontes, freshness, prompt e policy.

### 10.2 `PortfolioOpportunity.v1`

| Campo | Regra |
|---|---|
| `id`, `accountId` | owner agência |
| `opportunityType` | chave registrada |
| `status` | `proposed`, `approved`, `rejected`, `expired`, `invalidated`, `executed` |
| `targetClientAccountIds` | clientes acessíveis que podem receber a ação |
| `purposeKey` | finalidade aprovada |
| `aggregateSnapshotId` | snapshot protegido e temporário |
| `datasetKeys`, `sourceKeys` | allowlisted |
| `dimensionKeys`, `metricKeys` | allowlisted |
| `period` | intervalo fechado |
| `cohortClass` | bucket seguro; tamanho exato só se policy permitir |
| `suppressionApplied` | boolean + reason codes |
| `rationale`, `reasonCodes` | limitados |
| `campaignBrief` | sugestão, não campanha criada |
| `promptBindingId`, `promptVersionRefs` | obrigatórios |
| `runtimeRunId`, `contextSnapshotId` | obrigatórios |
| `policyVersionRefs` | coorte, privacy, eligibility e rollout |
| `validFrom`, `expiresAt` | obrigatórios |
| `approvedBy`, `decidedAt`, `outcome` | lifecycle |

O registro não possui `subjectId`, `relationshipId`, telefone, e-mail, documento, mensagem,
source row ID ou lista de contribuidores.

### 10.3 Persistência proposta

`intelligence.portfolio_opportunities`:

- PK UUID e `account_id not null`;
- campos do contrato acima em colunas/JSON fechado;
- unique por `(account_id, input_fingerprint, prompt_binding_id, policy_version)` durante validade;
- índices por `(account_id,status,created_at desc,id desc)` e expiração;
- prompt/context/runtime refs;
- sem nullable tenant global.

`intelligence.portfolio_opportunity_targets`:

- `(account_id, opportunity_id, client_account_id)` como chave composta;
- client validado no catálogo da organização;
- nunca contém contributors.

Se essas tabelas não forem incorporadas pela CI-04 antes de sua migration, uma nova spec/pacote DB
deve ser aprovado. CI09-BE-01 não pode criar DDL incidental.

## 11. Coorte, supressão e proteção contra reidentificação

O valor exato do piso de coorte continua pendente de decisão jurídica/privacidade. Até a aprovação:

- capability de portfólio permanece `off`;
- painel não oferece default editável;
- nenhuma query real é executada.

Policy final deve garantir cumulativamente:

- `cohortSize >= platformMinimumCohort`;
- cliente não consegue reduzir o piso da plataforma;
- small cells e combinações raras são suprimidas;
- dimensions de alta cardinalidade são bloqueadas;
- filtros sucessivos/differencing possuem orçamento e janela;
- queries equivalentes/repetidas são correlacionadas;
- resultados podem exigir bucketização, arredondamento ou ruído conforme categoria;
- exportação row-level não existe;
- supressão do source não pode ser revertida pelo processo/prompt;
- cache key inclui policy/query fingerprint e não cruza owner;
- erro/supressão não revela contagem limítrofe.

O output para o modelo recebe apenas o agregado já protegido. O modelo nunca recebe linhas para
“anonimizar depois”.

## 12. Cross-client individual e dados sensíveis

### 12.1 Default obrigatório

- leitura e ativação individual cross-client desabilitadas;
- nenhuma recomendação individual usa evidence/fact de outro `client_account_id`;
- nenhum cliente vê que uma pessoa também pertence a outro cliente;
- `core.organization.consolidated_read` sozinho não concede esse uso;
- portfólio agrega primeiro e descarta contributors antes do runtime.

### 12.2 Eventual ativação futura

Exigiria spec separada, capability própria e gates cumulativos:

- categoria explicitamente autorizada;
- purpose/base legal documentada;
- consentimento e opt-out aplicáveis;
- permissão platform e do owner;
- aprovação humana identificada;
- trilha de auditoria/export;
- retenção e revogação;
- política anti-reidentificação;
- contrato com o cliente de origem/destino quando juridicamente necessário.

Esta spec não autoriza esse fluxo.

### 12.3 Saúde e outras categorias sensíveis

- health/sensitive/restricted são excluídas de portfólio por default;
- clínicas/profissionais de saúde não permitem inferir condição, tratamento ou interesse de pessoa;
- prompt não pode recategorizar dado para contornar bloqueio;
- aggregate source e service repetem classificação;
- uso futuro exige revisão jurídica específica, DPIA/relatório equivalente e decisão canônica.

## 13. APIs propostas

Leitura individual usa `customer_intelligence.profile.view`. Gerar, aceitar, rejeitar, invalidar ou
superseder recomendação usa a permissão canônica `customer_intelligence.profile.manage` definida
na CI-00. Execução exige ainda a permissão do módulo owner; nenhuma permissão nova é criada nesta
spec.

| Método e rota | Permissão | Resultado |
|---|---|---|
| `GET /v1/customer-intelligence/relationships/{id}/recommendations` | `customer_intelligence.profile.view` | cursor, filtros fechados |
| `POST /v1/customer-intelligence/relationships/{id}/recommendation-runs` | `customer_intelligence.profile.manage` | agenda tipos/processos allowlisted |
| `GET /v1/customer-intelligence/recommendation-runs/{id}` | `customer_intelligence.profile.view` | status/ref sanitizada |
| `POST /v1/customer-intelligence/recommendations/{id}/approve` | `customer_intelligence.profile.manage` | revision + reason |
| `POST /v1/customer-intelligence/recommendations/{id}/reject` | `customer_intelligence.profile.manage` | revision + reason code |
| `POST /v1/customer-intelligence/recommendations/{id}/execute` | `customer_intelligence.profile.manage` + owner | ação idempotente no owner |
| `POST /v1/customer-intelligence/recommendations/{id}/invalidate` | `customer_intelligence.profile.manage` | reason + revision |
| `GET /v1/customer-intelligence/recommendation-policies` | `customer_intelligence.profile.view` | definitions/versions/binding efetivo |
| `POST /v1/customer-intelligence/recommendation-policies/{policyKey}/drafts` | `customer_intelligence.profile.manage` | cria/clona draft |
| `PATCH /v1/customer-intelligence/recommendation-policy-versions/{id}` | `customer_intelligence.profile.manage` | draft + `expectedRevision` |
| `POST /v1/customer-intelligence/recommendation-policy-versions/{id}/validate` | `customer_intelligence.profile.manage` | bounds/catálogos/fixtures |
| `POST /v1/customer-intelligence/recommendation-policy-versions/{id}/publish` | `customer_intelligence.profile.manage` | versão imutável |
| `POST /v1/customer-intelligence/recommendation-policy-bindings` | `customer_intelligence.profile.manage` | seleciona versão/scope por CAS |
| `POST /v1/customer-intelligence/recommendation-policy-bindings/{id}/rollback` | `customer_intelligence.profile.manage` | reponta published anterior |
| `GET /v1/customer-intelligence/portfolio/opportunities` | `customer_intelligence.portfolio.view` + gates | agregados protegidos |
| `POST /v1/customer-intelligence/portfolio/runs` | `customer_intelligence.portfolio.manage` + gates | job agregado allowlisted |
| `POST /v1/customer-intelligence/portfolio/opportunities/{id}/decide` | `customer_intelligence.portfolio.manage` + gates | aprova/rejeita |

Regras:

- account vem do Principal;
- client/relationship são revalidados por Customer Data;
- out-of-scope retorna 404;
- lista usa cursor estável e limite bounded;
- response nunca inclui contributors;
- geração/execução são jobs idempotentes, não request síncrona longa;
- `execute` retorna accepted/job ref, não “enviado” antes do owner confirmar.

## 14. Frontend

### 14.1 Perfil individual

`CustomerRecommendationsPanel`:

- separa follow-up, oferta, data e próxima ação;
- mostra status, validade, confiança, rationale, evidências e freshness;
- identifica conteúdo gerado por IA;
- oferece aprovar/rejeitar/executar conforme permissões;
- exibe preview de constraints e revalidação necessária;
- não cria mensagem/campanha no browser;
- não transforma aceite em fato;
- preserva dirty/revision e account/client switching de CI-08.

### 14.2 Portfólio

Página `/inteligencia-clientes/portfolio`:

- só monta/faz fetch depois de todos os gates;
- mostra finalidade, policy, freshness e proteção aplicada;
- filtros vêm do descriptor, nunca campo livre;
- não mostra tamanho exato quando a policy usa bucket;
- não oferece drill-down até indivíduo;
- apresenta clientes-alvo sem revelar contributors;
- mostra prompt/binding/model/eval/custo conforme permissão;
- aprovação exige confirmação e reason;
- criação de campanha/ação ocorre no owner, em fluxo separado.

### 14.3 Customização

O painel permite configurar prompts por processo, policies de validade/ranking/cadência,
thresholds, fontes, modelos, revisão e rollout dentro dos limites de plataforma. Piso de coorte,
categorias proibidas, consentimento, tenant e anti-reidentificação não podem ser reduzidos pelo
cliente.

`RecommendationPolicyPanel` mostra definition, versão efetiva/herança, binding, draft, diff,
preview de impacto, fixtures, validate, publish e rollback. Campos são controles tipados pelo
schema da policy; não há JSON/expressão livre. A UI exibe separadamente “comportamento do prompt” e
“limites/policy”, para não induzir que texto possa vencer consentimento ou catálogo.

## 15. Concorrência, duplicata e falhas

| Caso | Comportamento |
|---|---|
| mesmo fingerprint/binding/policy válido | retornar recomendação existente |
| geração concorrente | unique/lock e um único vencedor |
| evidência muda durante run | validar `asOf`/fingerprint; descartar ou marcar stale |
| consentimento revogado | invalidar/bloquear; não executar |
| produto removido | invalidar offer na revalidação |
| data contestada | bloquear ação e pedir revisão |
| source stale/partial | policy decide gerar com warning ou recusar |
| prompt/output inválido | run falha; zero recomendação |
| owner indisponível ao executar | retry bounded; não duplicar |
| query suprimida | nenhum modelo recebe dado; registrar reason code |
| client sai do escopo | invalidar target e cache; não revelar histórico indevido |
| rollout pausado | não iniciar novos runs candidatos |

## 16. Observabilidade, feedback e avaliação

Métricas individuais:

- geradas, deduplicadas, aprovadas, rejeitadas, expiradas, invalidadas e executadas;
- tempo até decisão/execução;
- acceptance/rejection por process, binding, model e reason code;
- catálogo/data inválidos;
- stale/partial/consent blocks;
- custo/latência/tokens;
- outcome observado separado de conversão causal.

Métricas de portfólio:

- queries tentadas, suprimidas e aprovadas;
- cohort class, nunca contributor;
- budget de differencing;
- target clients/opportunities;
- aprovações/rejeições/outcomes;
- custo/freshness e policy version.

Feedback:

- rejeição exige reason code allowlisted;
- comentário livre é limitado/classificado;
- outcome não edita recomendação histórica;
- feedback alimenta evals versionados;
- nenhum feedback inicia fine-tuning/treino externo implicitamente.

Evals por processo:

- output schema e refs válidas;
- zero produto/data/source inventados;
- consentimento/quiet hours/frequency cap;
- evidência suficiente e rationale fiel;
- prompt injection;
- PII leakage;
- reidentificação/differencing;
- qualidade funcional em fixtures;
- custo e latência.

## 17. Pacotes atômicos e allowlists

### CI09-POLICY-01 — lifecycle e resolução de policies

**Resultado:** policy versionada/editável realmente governa elegibilidade, validade, cadência e
ranking sem configuração escondida.

**Pré-condição:** `CI04-DB-03` concluído.

**Allowlist máxima:**

- `back/internal/modules/customerintelligence/model_recommendation_policy.go` (novo);
- `back/internal/modules/customerintelligence/service_recommendation_policy.go` (novo);
- `back/internal/modules/customerintelligence/store_recommendation_policy.go` (novo);
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

**Proibido:** DDL incidental, policy em prompt/constante, sender e alteração de owner externo.

### CI09-BE-01 — Domínio, jobs e persistência

**Resultado:** recomendações individuais explicáveis e ranking determinístico.

**Allowlist máxima:**

- `back/internal/modules/customerintelligence/model_recommendation.go` (novo);
- `back/internal/modules/customerintelligence/policy_recommendation.go` (novo);
- `back/internal/modules/customerintelligence/service_recommendation.go` (novo);
- `back/internal/modules/customerintelligence/store_recommendation.go` (novo);
- `back/internal/modules/customerintelligence/job_recommendation.go` (novo);
- `back/internal/modules/customerintelligence/service_portfolio.go` (novo, somente após schema);
- `back/internal/modules/customerintelligence/store_portfolio.go` (novo, somente após schema);
- testes correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

**Proibido:** DDL incidental, prompt hardcoded, sender e SQL cross-module.

### CI09-PROMPTS-02 — Processos, schemas e evals

**Resultado:** bindings separados e outputs fechados para os quatro processos canônicos.

**Allowlist máxima proposta, reutilizando o runtime CI-06 e a congelar após seu handoff:**

- `back/internal/modules/customerintelligence/prompt_catalog.go`;
- `back/internal/modules/customerintelligence/process_schemas.go`;
- `back/internal/modules/customerintelligence/processors/recommendation_follow_up.go` (novo);
- `back/internal/modules/customerintelligence/processors/recommendation_offer.go` (novo);
- `back/internal/modules/customerintelligence/processors/recommendation_important_dates.go` (novo);
- `back/internal/modules/customerintelligence/processors/portfolio_opportunity.go` (novo);
- fixtures/testes sintéticos correspondentes;
- `back/internal/modules/customerintelligence/AGENT.md`.

Conteúdo default de prompt deve entrar pelo lifecycle controlado de CI-06, não como constante de
produção. É proibido adicionar `recommendation.next_action` sem decisão canônica.

### CI09-API-03 — APIs de recomendação e portfólio

**Resultado:** leitura, geração, decisão e execução autorizada.

**Allowlist máxima:**

- `back/internal/modules/customerintelligence/http_recommendations.go` (novo);
- `back/internal/modules/customerintelligence/http_recommendations_test.go` (novo);
- `back/internal/modules/customerintelligence/http_recommendation_policies.go` (novo);
- `back/internal/modules/customerintelligence/http_recommendation_policies_test.go` (novo);
- `back/internal/modules/customerintelligence/http_portfolio.go` (novo);
- `back/internal/modules/customerintelligence/http_portfolio_test.go` (novo);
- `back/internal/modules/customerintelligence/module.go`;
- `back/internal/modules/customerintelligence/AGENT.md`.

**Proibido:** endpoint row-level cross-client, sender e bypass do owner action service.

### CI09-FE-04 — Perfil e portfólio

**Resultado:** recomendações explicáveis no perfil e oportunidades agregadas em rota protegida.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/[subjectId].vue`;
- `web/app/pages/inteligencia-clientes/portfolio.vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerRecommendationsPanel.vue` (novo);
- `web/app/components/customer-intelligence/recommendations/RecommendationPolicyPanel.vue` (novo);
- `web/app/components/customer-intelligence/portfolio/PortfolioOpportunities.vue` (novo);
- `web/app/components/customer-intelligence/portfolio/PortfolioOpportunityDrawer.vue` (novo);
- `web/app/components/customer-intelligence/portfolio/PortfolioPolicySummary.vue` (novo);
- `web/app/composables/customer-intelligence/useRecommendations.ts` (novo);
- `web/app/composables/customer-intelligence/useRecommendationPolicies.ts` (novo);
- `web/app/composables/customer-intelligence/usePortfolioOpportunities.ts` (novo);
- `web/app/domain/customer-intelligence/recommendation-api.ts` (novo);
- `web/app/domain/customer-intelligence/recommendation-types.ts` (novo);
- `web/app/domain/customer-intelligence/recommendation-policy-api.ts` (novo);
- `web/app/domain/customer-intelligence/recommendation-policy-types.ts` (novo);
- `web/app/domain/customer-intelligence/portfolio-api.ts` (novo);
- `web/app/domain/customer-intelligence/portfolio-types.ts` (novo);
- `web/app/components/customer-intelligence/CustomerIntelligenceNav.vue`;
- testes correspondentes.

### CI09-QA-05 — Segurança, qualidade e integração

**Resultado:** provar contratos, policies, cross-client e ausência de execução direta.

**Allowlist máxima:**

- novos `*_test.go` no package Customer Intelligence diretamente relacionados;
- novos `*.test.ts`/`*.spec.ts` nos paths frontend CI-09;
- fixtures sintéticas e evidências em `docs/customer-intelligence/evidence/CI-09/`.

**Proibido:** dados reais, snapshot com PII e alteração de produção.

### Pacote DB predecessor, se necessário

Caso CI-04 não incorpore a persistência de portfólio, o orquestrador deve primeiro criar uma spec e
um pacote DB separado, com número reservado, ERD, migration apenas aditiva, índices/constraints e
rollback lógico. Nenhum pacote CI09 acima recebe autorização implícita para isso.

## 18. Áreas proibidas

CI-09 não altera, sem novo despacho:

- Omnichannel, adapters de provider, mensagem, outbox ou sender;
- ERP/CRM/Calendar/Site/BI internals;
- workflow n8n;
- migrations CI-04/05 já executadas;
- Prompt Studio CI-08 fora dos bindings/APIs aprovados;
- módulo legado `automation`;
- qualquer arquivo de `socialpublishing`/`social-publishing`;
- secrets, `.env` ou dado real.

## 19. Rollout e rollback

Rollout individual:

1. schemas/evals com fixtures sintéticas;
2. geração shadow, sem persistir recomendação visível;
3. persistência `proposed`, somente interna;
4. revisão humana obrigatória por cliente/processo;
5. execução manual via owner;
6. automação de baixo risco somente após métricas e decisão explícita.

Rollout de portfólio:

1. capability `off`;
2. policies jurídica/coorte/supressão aprovadas;
3. queries sintéticas;
4. shadow com agregado protegido;
5. acesso somente platform/agência interna;
6. oportunidade proposta com revisão humana;
7. nenhuma ativação individual nesta spec.

Rollback:

- pausar rollout/binding e impedir novos jobs;
- manter histórico/auditoria;
- expirar/invalidate recomendações afetadas com reason code;
- cancelar jobs não claimed e cooperar com claimed;
- não apagar evidence/outcome fora da retention policy;
- owner mantém idempotency keys de ações já aceitas;
- rollback de prompt reponta binding;
- nunca reexecutar mensagem/campanha.

## 20. Testes e comandos

Backend futuro, a partir de `back/`:

```text
go test ./internal/modules/customerintelligence/...
go test -race ./internal/modules/customerintelligence/...
go test ./internal/platform/app/...
go test ./...
```

Frontend futuro, a partir da raiz:

```text
npm --prefix web run test
npm --prefix web run lint
npm --prefix web run build
```

Cenários obrigatórios:

- process/binding inexistente produz zero recomendação;
- retry/fingerprint não duplica;
- account/client/relationship negativos;
- claim de data sem fato não vira ação;
- produto removido/inválido é rejeitado;
- opt-out, quiet hours, cap e consentimento vencem prompt;
- recommendation policy draft/publish/binding/rollback preserva histórico;
- toda recomendação fixa policy binding/version e ranking usa a mesma versão;
- próxima ação não cruza relationship;
- aprovação concorrente respeita revision;
- execução repetida não duplica no owner;
- portfólio abaixo do piso é suprimido sem revelar contagem;
- differencing e alta cardinalidade são bloqueados;
- output não contém contributor/PII;
- health/sensitive não entra no agregado;
- prompt injection não altera source/tool/policy;
- abrir página sem gates gera zero fetch;
- troca de account/client limpa estado;
- n8n/frontend não enviam mensagem.

## 21. Critérios de aceite

- [ ] tipos e processos estão separados;
- [ ] recomendação, fato e ação possuem owners diferentes;
- [ ] payloads são fechados e versionados;
- [ ] todas as recomendações carregam evidence/context/prompt/model/policy refs;
- [ ] prompt específico por processo é editável no painel;
- [ ] policy estruturada vence prompt;
- [ ] policy possui definition/version/binding, API e painel próprios;
- [ ] recomendação fixa policy binding/version em colunas auditáveis;
- [ ] próxima ação é determinística no MVP;
- [ ] data/produto inexistente não pode ser recomendado como válido;
- [ ] lifecycle e concorrência são auditáveis;
- [ ] execução passa pelo módulo owner e por idempotência;
- [ ] portfólio não contém contributors nem PII;
- [ ] piso de coorte/supressão bloqueia release até aprovação;
- [ ] cross-client individual permanece desligado;
- [ ] health/sensitive possui bloqueio adicional;
- [ ] schema de portfólio foi resolvido sem UUID fictício;
- [ ] nenhum sender, workflow ou arquivo fora da allowlist foi alterado.

## 22. Stop conditions

Parar e devolver ao orquestrador se:

- a entidade física de portfólio não tiver sido entregue por pacote DB predecessor aprovado;
- `customer_intelligence.profile.manage` não tiver enforcement backend por account/client;
- coorte/supressão/base legal continuarem abertas na tentativa de ativar portfólio;
- aggregate source retornar linha, subject, contributor ou filtro livre;
- prompt precisar criar produto, data, source key ou tool;
- action owner não fornecer porta idempotente;
- houver proposta de dual-write ou envio direto;
- dado sensível puder entrar por configuração/prompt;
- migration for necessária dentro de pacote não-DB;
- arquivo permitido estiver sujo por outra trilha;
- surgir necessidade de tocar `socialpublishing`, workflow ou sender;
- teste tenant/reidentificação/duplicata falhar.

## 23. Handoff obrigatório

Cada pacote registra:

- escopo e arquivos alterados;
- processos/bindings/schemas/policies usados;
- permissões e capabilities efetivas;
- migrations predecessoras e schema realmente disponível;
- fixtures/evals, custos e resultados;
- contagens por lifecycle e dedupe;
- proteção de coorte/supressão testada;
- confirmação de zero contributor/PII em portfólio;
- ações externas chamadas e idempotency keys, quando autorizadas;
- rollout/rollback e flags;
- confirmação de zero sender/workflow/social-publishing;
- próximo pacote ou blocker objetivo.
