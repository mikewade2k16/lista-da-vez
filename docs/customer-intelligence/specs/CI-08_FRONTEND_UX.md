# CI-08 — Frontend e experiência de Customer Intelligence

- **Status:** READY — implementação local autorizada
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Frontend/Produto
- **Depende de:** CI-03, CI-04 e fixtures/contratos de CI-06; `CI08-SOURCES-03` depende também do
  handoff frontend de CI-05
- **Integra com:** CI-07 e CI-09
- **Desbloqueia:** CI-10
- **Autoriza implementação:** sim; acesso segue módulos e permissões

> Esta spec define a superfície administrativa e operacional futura. Enquanto estiver em `DRAFT`,
> não autoriza criar páginas, alterar navegação, trocar APIs, redirecionar rotas, remover telas ou
> publicar prompts.

## 1. Resultado único e verificável

Entregar o workspace `/inteligencia-clientes` como a superfície única e permissionada para:

- visualizar o perfil determinístico e a inteligência explicável de um cliente;
- rastrear fatos, claims, evidências e fontes até sua origem;
- habilitar e configurar fontes allowlisted;
- administrar agentes, policies e prompts específicos de cada processo;
- criar e operar segmentos CRM/marketing determinísticos com versões, preview e materialização;
- validar, simular, publicar, colocar em canary e fazer rollback de versões;
- consultar runs, custo, latência, falhas e auditoria;
- expor apenas um snapshot compacto no inbox do Omnichannel;
- manter o runtime totalmente funcional sem o painel aberto.

A entrega só é aceita quando não existir um segundo Prompt Studio concorrente: a experiência atual
de `ConfigAiAgent.vue` e `ConfigAiAgentVersions.vue` deve ser evoluída/reutilizada, com façade de
compatibilidade, até que equivalência, telemetria e cutover permitam depreciar a entrada antiga.

## 2. Princípios de UX e ownership

| Área | Owner da verdade | Papel da UI |
|---|---|---|
| pessoa, identidade, relação, consentimento | Customer Data | editar apenas por API tipada e permissão |
| segmento, versão, evaluation run, materialização e export | Customer Data | builder/preview/publish/export por gates distintos |
| evidence, claim, fact, summary, run | Customer Intelligence | explicar, revisar e auditar |
| prompt, binding, agent, model, source/tool policy | Customer Intelligence | draft/test/publish/rollback |
| conversa, mensagem, fila, handoff, sender | Omnichannel | operar atendimento |
| catálogo/produto/serviço | CRM/ERP | referenciar; nunca copiar como verdade nova |
| evento/data de calendário | Calendar | referenciar; nunca mutar por tela de perfil |

Regras:

- a UI não faz join de entidades no browser para reconstruir uma autoridade paralela;
- módulos retornam read models compostos por gateways backend autorizados;
- nenhum ID de account/client enviado pelo browser substitui o Principal;
- toda configuração segura de produto deve ser tipada, persistida no PostgreSQL e editável no
  painel pela permissão correta;
- prompt é a principal camada de comportamento semântico, específico por processo;
- prompt não controla tenant, RBAC, FSM, lease, idempotência, schema, consentimento, allowlist,
  retenção, outbox ou sender;
- segredos são write-only: o frontend recebe apenas ID, nome, provider, status e `last4`;
- páginas usam `AdminPageHeader` e os padrões visuais existentes do dashboard.

## 3. Dependências, bloqueios e estados de página

| Dependência | Contrato exigido |
|---|---|
| CI-03 | subject, relationship, identity, consent, segmentos e client scope |
| CI-04 | evidence/claim/fact/summary/profile e provenance |
| CI-05 | catálogo/API e componentes-base de fontes, exigidos apenas por `CI08-SOURCES-03` |
| CI-06 | processos, prompts, bindings, agentes, fontes, runs, evals e fixtures |
| CI-07 | snapshot compacto e navegação contextual a partir do inbox |
| CI-09 | recomendações e portfólio, sem misturá-los a fatos |

Estados obrigatórios por página/componente:

- `loading`: skeleton estável, sem mostrar dados do escopo anterior;
- `empty`: explica requisito, permissão ou próxima ação segura;
- `partial`: identifica fontes indisponíveis/stale sem fingir completude;
- `forbidden`: não revela existência de subject/client;
- `disabled`: módulo/capability desligado e nenhuma chamada de dados é iniciada;
- `error`: reason code traduzido, retry explícito e correlação segura;
- `ready`: dados com `asOf`, origem e status;
- `dirty`: edição local protegida de reidratação automática;
- `saving/testing/publishing/rollingBack`: ação bloqueada contra repetição.

Bloqueios para `READY`:

- rotas e permissões CI-00 aprovadas;
- read models de CI-03/04/06 congelados;
- decisão sobre façade das APIs legadas de agente;
- inventário completo de recursos atuais de `ConfigAiAgent*`;
- critérios de equivalência e telemetria definidos em CI-10;
- requisitos jurídicos para evidência, consentimento e portfólio aprovados;
- exportação de segmento permanece `off|shadow` até finalidade, retenção, TTL e field sets serem
  aprovados.

## 4. Inventário real e compatibilidade obrigatória

O frontend atual já contém:

- `/omnichannel/automacao` e configurações de IA dentro do contexto Omnichannel;
- `ConfigAiAgent.vue`, com lista, ativação e criação de agentes;
- `ConfigAiAgentVersions.vue`, com `systemPrompt`, layers, provider, credencial, modelo,
  temperatura, debounce, contexto, limites, handoff e mídia;
- simulator, tools/knowledge, credentials, advanced settings e publish/rollback relacionados;
- `AutomationAiConfigDrawer.vue` e drawer de configuração do Omnichannel;
- `OmnichannelCRMProfilePanel.vue` e `useOmnichannelCRM.ts`, acoplando inteligência do contato ao
  inbox;
- rotas `/crm` e `/inteligencia` com significados existentes e que não podem ser reaproveitadas.

Consequências:

- não criar um editor de prompt paralelo copiando componentes ou regras;
- não apagar, ocultar nem redirecionar o fluxo atual antes da equivalência comprovada;
- não transformar `/inteligencia` analítica em Customer Intelligence;
- não transformar `/crm` comercial em perfil inteligente;
- manter deep links e permissões existentes durante a fase de façade;
- toda regressão de capability atual bloqueia cutover.

## 5. Arquitetura de informação

### 5.1 Workspace novo

| Rota proposta | Conteúdo | Owner de implementação |
|---|---|---|
| `/inteligencia-clientes` | visão geral, busca e cobertura | CI-08 |
| `/inteligencia-clientes/:subjectId` | perfil 360 por relação/client scope | CI-08 |
| `/inteligencia-clientes/segmentos` | segmentos CRM/marketing, versões, runs e export separado | CI-03/CI-08 |
| `/inteligencia-clientes/fontes` | catálogo, health e configuração de fontes | CI-08 |
| `/inteligencia-clientes/prompts` | Prompt Studio único | CI-08 |
| `/inteligencia-clientes/atendimentos` | runs, custo, latência e erros | CI-08 |
| `/inteligencia-clientes/auditoria` | mudanças, origem e decisões administrativas | CI-08 |
| `/inteligencia-clientes/portfolio` | oportunidades agregadas | CI-09 |
| `/omnichannel?config=channel-client-bindings` | vínculo canal→cliente e exceções | CI-01/Omnichannel |

`subjectId` sozinho não concede acesso. A relação deve ser resolvida no client scope selecionado e
validado pelo backend.

### 5.2 Navegação

- workspace visível somente com módulo habilitado e alguma permissão de visualização;
- abas internas aparecem por capability e permissão, não por role hardcoded;
- `segmentos` exige `customer_data.segmentation != off`, `segments.view` e client explícito;
- `atendimentos` exige `customer_intelligence.runs.view`;
- `auditoria` exige `customer_intelligence.audit.view`;
- `portfolio` exige todos os gates cross-client e não aparece por default;
- link vindo do inbox leva ao perfil e preserva somente parâmetros allowlisted;
- link de cobertura sem binding abre a aba canônica do Omnichannel; CI-08 não duplica o editor;
- troca de account/client limpa estado, cancela requests e reidrata permissões antes de buscar;
- item atual de IA no Omnichannel permanece até cutover, apontando para a mesma experiência
  canônica por wrapper/link contextual.

### 5.3 Segmentação CRM/marketing

`/inteligencia-clientes/segmentos` é uma superfície de Customer Data composta no workspace. Usa
`AdminPageHeader`, breadcrumbs/tabs existentes e nunca consulta tabela nem reconstrói membership no
browser. Em account agência, o client selector é obrigatório e não oferece “todos os clientes”;
trocar client limpa lista, draft, amostra, runs, materializações e export intents antes do novo
fetch.

A página possui cinco áreas:

| Área | Conteúdo/ação | Gate |
|---|---|---|
| lista | nome, status, versão ativa, último `asOf`, freshness e count bucket | `segments.view` |
| builder | grupos AND/OR e predicates tipados vindos de `/segment-fields` | `segments.manage` |
| versões | draft, validação, diff, publish e rollback por binding | `segments.manage` + `segments.publish` |
| avaliações | preview, runs, diagnóstico e materializações | `segments.evaluate` |
| exportação | elegibilidade, exclusões, finalidade, TTL e download intent | `segments.export` |

O builder oferece somente `fieldKey`, operador e editor de valor fornecidos pelo catálogo backend.
Não existe textarea de SQL, JSON/JSONPath, expressão, URL ou regex livre. O AST pode ser exibido em
modo técnico somente leitura, sanitizado. Profundidade, quantidade de condições, listas, strings e
custo mostram os caps retornados pelo servidor; a UI antecipa validação, mas o backend continua
autoritativo.

IA de sugestão de filtro não integra esta tela: a capability permanece **BLOCKED** até nova decisão
de `process_key`, schemas, prompts, evals, thresholds, permissões e rollout. Nenhum botão oculto,
feature flag frontend ou chamada ao runtime inteligente antecipa essa decisão.

Fluxo de definição:

1. criar registro estável e primeiro draft no client atual;
2. editar com `revision`, preservando formulário dirty;
3. validar schema, field catalog, sources, custo e policy;
4. solicitar preview assíncrono e acompanhar `evaluationRunId`;
5. comparar count/amostra mascarada e diagnósticos com `asOf`;
6. publicar versão imutável mediante permissão/confirmação;
7. materializar versão publicada e acompanhar freshness;
8. rollback escolhe versão publicada anterior; nunca abre published para edição.

Preview não mostra “membros do segmento” como uma lista persistida e nunca produz export. A amostra
é bounded, mascarada e desaparece ao trocar scope/run; sem permissões cumulativas de subject/relação,
a UI exibe apenas contagens. Runs `partial|failed` mostram fonte, freshness e reason code, não
convertem indisponibilidade em zero resultados. Polling respeita `pollAfterMs`, cap, visibility e
cancel; não há loop por watcher.

Materialização mostra versão, `asOf`, source snapshots, count, freshness e estado
`building|current|superseded|expired|failed`. A UI nunca adiciona/remove membro manualmente. O
badge “corresponde ao segmento” é visualmente distinto de “elegível para contato”.

Exportação é diálogo/stepper separado da materialização:

- exige finalidade, canal, formato e field set entre opções backend;
- apresenta candidate/eligible/excluded counts e reason codes antes do download;
- exige confirmação do escopo/client e, quando a policy determinar, aprovação/motivo;
- só habilita download por intent curta e nunca guarda URL/PII em store persistida;
- informa que membership não é consentimento e que revogação/staleness pode regenerar o arquivo;
- em `segment_exports=shadow`, mostra apenas relatório de elegibilidade, sem ação de download;
- nunca oferece “enviar agora”, criar campanha ou chamar WhatsApp/Instagram.

Criar/editar/listar segmentos e executar preview continuam funcionais sem Customer Intelligence.
Exportação indisponível não bloqueia definição, publicação ou materialização.

### 5.4 Atendimentos e runs

`/inteligencia-clientes/atendimentos` é leitura operacional de
`GET /v1/customer-intelligence/runs`, owner CI-06. Usa `AdminPageHeader` e só inicia fetch com
`customer_intelligence` habilitado e `customer_intelligence.runs.view` efetiva na account.

A página oferece filtros allowlisted/cursor-based retornados pelo contrato CI-06, incluindo client,
período, status, process/pipeline e executor quando suportados. Busca não aceita query de prompt,
mensagem ou PII. Lista/drawer mostram somente:

- status, processo/pipeline e refs de versões/bindings;
- início/fim, duração e tentativas;
- usage/custo/moeda e latência sanitizados;
- executor/provider/model por IDs/nome permitido, nunca credential;
- source/tool counts/status e reason/error code sanitizado, nunca arguments/results;
- correlation/context refs opacas permitidas pelo backend.

Não exibe input/output, prompt compilado, mensagem, tool trace, evidence ou payload bruto. Um link
contextual só aparece quando o backend devolve a ref e a permissão correspondente; a UI não monta
IDs ou consulta outro módulo para enriquecer a linha. Refresh usa paginação, cancelamento e intervalo
bounded; a página não reexecuta, cancela nem altera run nesta fase.

### 5.5 Auditoria e observações

`/inteligencia-clientes/auditoria` lê
`GET /v1/customer-intelligence/audit-events` e, a partir de ref autorizada,
`GET /v1/customer-intelligence/observations/{observationId}`, owner CI-04. Usa `AdminPageHeader`,
capability do módulo e `customer_intelligence.audit.view`.

Filtros são allowlisted/cursor-based por client, tempo, ação, entity type e status/proveniência que
o backend suportar. A lista mostra evento, ator sanitizado, ação, entidade/ref opaca, instante,
reason/correlation code, old/new hash e diff allowlisted. O painel de observação mostra somente o
snapshot já minimizado/mascarado pelo backend, source/provenance refs, sensitivity, purpose,
retention state e timestamps autorizados.

Não existe botão “mostrar JSON bruto”, reveal genérico, export ad hoc ou busca por conteúdo. Prompt,
mensagem, documento, source payload, segredo e PII não autorizada nunca entram em DOM, analytics ou
store. Ausência de permissão retorna `forbidden` sem confirmar que evento/observação existe. Esta
página é read-only; revisão/contestação continua na tela e API owner correspondentes.

## 6. Perfil do cliente

### 6.1 Cabeçalho

Exibe:

- nome canônico e aliases autorizados;
- client/relationship em contexto explícito;
- lifecycle/estágio e owner comercial;
- canais/identidades mascarados;
- consentimento/opt-out e alertas;
- `asOf`, qualidade e cobertura;
- ações permitidas, sem inferir permissão pelo fato de o botão estar visível.

### 6.2 Seções

| Seção | Conteúdo | Regras |
|---|---|---|
| resumo | síntese versionada, estilo e próximos cuidados | informar prompt/context/version e `asOf` |
| fatos | valores confirmados/candidatos/contestados | separar fato de claim e mostrar confiança |
| evidências | trecho minimizado, fonte, instante e hash/ref | conteúdo sensível mascarado por default |
| histórico | conversas online/offline e eventos autorizados | paginação server-side |
| preferências | contato, linguagem, produto e canal | indicar origem e possibilidade de edição |
| datas | evidência da data e recomendação separadas | nunca apresentar inferência como data confirmada |
| recomendações | follow-up, oferta e próxima ação | status/rationale/evidence; owner CI-09 |
| fontes | cobertura, freshness, erro e última coleta | links somente para módulos permitidos |
| auditoria | mudanças de fato/resumo/configuração | diff sanitizado |

### 6.3 Ações

- corrigir dado determinístico chama Customer Data;
- contestar claim/fact registra motivo e auditoria;
- aceitar/rejeitar recomendação não converte automaticamente em fato;
- solicitar refresh cria job idempotente, não executa polling descontrolado;
- registrar/revisar interação offline usa Customer Data e continua disponível sem IA;
- abrir conversa navega ao Omnichannel, sem enviar mensagem;
- merge/undo exige permissão específica e confirmação;
- exportar/apagar segue workflow LGPD, nunca download ad hoc no browser.

## 7. Catálogo de fontes

Cada fonte deve apresentar:

| Campo | Regra |
|---|---|
| `sourceKey` | chave registrada em código, imutável |
| `name`, `description` | metadata, sem HTML inseguro |
| `moduleId` | módulo owner |
| `status` | `available`, `disabled`, `degraded`, `error`, `unauthorized` |
| `enabled` | configuração por owner/client |
| `capabilities` | tipos de dado e modos read-only/mutável |
| `dataClasses` | classificação, inclusive sensível |
| `purposeKeys` | finalidades permitidas |
| `freshness` | `lastSuccessAt`, lag e policy |
| `coverage` | sujeitos/relações cobertos, sem expor PII agregada insegura |
| `credentialRef` | ID/last4/status; nunca valor |
| `configSchema` | campos tipados e bounds |
| `healthReasonCode` | código traduzível e correlação |

A UI só permite habilitar fonte já registrada e compatível com módulo, permissão, finalidade,
consentimento e data class. Prompt/modelo pode sugerir uma `sourceKey`, mas não habilitá-la nem
fornecer URL/SQL/credencial.

Configuração segue `draft → validate → save → effective`; fontes com impacto inteligente podem
exigir teste e rollout. Toda mudança mostra escopo, diff e efeitos previstos.

## 8. Prompt Studio único

### 8.1 Estratégia de evolução, não duplicação

A árvore visual canônica deverá ser extraída/evoluída a partir dos componentes existentes. Durante
a transição:

1. `ConfigAiAgent.vue` e `ConfigAiAgentVersions.vue` continuam acessíveis;
2. a lógica de formulário/versionamento passa por composables e API façade compartilhados;
3. o novo `/inteligencia-clientes/prompts` usa a mesma árvore canônica;
4. wrappers legados mostram banner de origem/destino e preservam deep link;
5. nenhuma alteração fica salva somente na API antiga;
6. depois de equivalência, tráfego e erros são medidos;
7. somente CI-10 pode trocar entrada, congelar façade e autorizar remoção.

Não é permitido manter dois formulários que editem o mesmo binding por APIs diferentes.

### 8.2 Mapeamento de funcionalidades

| Função atual | Evolução canônica |
|---|---|
| nome/ativação do agente | agente versionado + capability/policy separada |
| sequência triage→reply/close | pipeline estruturado/versionado, sem lógica escondida no prompt |
| `systemPrompt` | versão do `process_prompt`, nunca blob global implícito |
| `layers.identity` e demais layers | editor/visualização de camadas tipadas e precedência |
| provider/model/temperature | binding e policy estruturada allowlisted |
| credencial de resposta | secret reference write-only |
| debounce/context/turns/confidence | policies tipadas com bounds server-side |
| handoff on error/limit | fallback policy; FSM permanece no Omnichannel |
| imagem/documento | bindings próprios para `media.image_analysis` e `media.document_analysis` |
| `transcription`/áudio e `video_summary` | card read-only “legado gerenciado no Omnichannel” + CTA para `ConfigAiAgentMediaSettings` |
| tools/knowledge | source/tool binding allowlisted |
| simulator | test case/eval versionados e sem efeito |
| versões/publicação/rollback | lifecycle imutável com canary e auditoria |

Nenhuma capability atual pode desaparecer durante a migração. Se um campo não possuir destino
canônico, o cutover para até haver decisão explícita de preservação, transformação ou depreciação.

### 8.3 Experiência por processo

O Studio começa pelo catálogo de processos, não por um prompt universal:

- `conversation.triage`;
- `conversation.reply`;
- `conversation.handoff_summary`;
- `memory.extract`;
- `profile.summary`;
- `recommendation.follow_up`;
- `recommendation.offer`;
- `recommendation.important_dates`;
- `source.suggest`;
- `portfolio.opportunity`;
- `media.image_analysis`;
- `media.document_analysis`;
- `quality.review`.

Essas continuam sendo as 13 process keys novas; imagem e documento permanecem no catálogo. Áudio
(`transcription`) e vídeo (`video_summary`) **não** aparecem como process keys nem ganham draft,
publish ou API writer novos. O Studio mostra uma seção de compatibilidade “Legado gerenciado no
Omnichannel” com:

- capability, owner `omnichannel`, status read-only e indicação de que usa `media_config` legado;
- status vindo da façade/read model já existente; se indisponível, “status desconhecido”, sem
  inferir `off`;
- CTA “Configurar no Omnichannel”, usando o deep link atual que monta
  `ConfigAiAgentMediaSettings`;
- explicação de que migração futura depende de decisão, schemas, prompts, thresholds, fixtures,
  bindings e shadow próprios.

O card não duplica campos, não envia PATCH e não reserva/ativa as chaves candidatas. Toda edição
continua na configuração legada até novo despacho de governança.

Uma aba “Pipelines” administra entrypoints estruturados como `conversation.respond`: versão
ativa/draft, etapas registradas, branches allowlisted, hard caps, escopo agency/client/agent,
shadow/canary e rollback. Não existe editor de código, expressão, SQL ou URL; a UI só oferece
combinações aceitas pelo catálogo Go. Cada etapa continua apontando para seu prompt próprio.

Para cada processo, mostra:

- descrição, owner, input/output schema e status;
- binding efetivo por agency/client/agent;
- herança e precedência das camadas;
- versão ativa, draft, canary e anterior;
- editor com variáveis tipadas e autocomplete allowlisted;
- modelo/parâmetros, tools, fontes e limites em controles estruturados;
- diff semântico/textual;
- fixtures, simulação, assertions, custo e latência;
- evals de qualidade, segurança, PII e schema;
- histórico de publicação/rollback e auditoria.

Para cada pipeline, mostra também diff de ordem/branch, process config versions efetivas, fixture
end-to-end e `processRunRefs` esperados. Simulação evidencia que triage intermediária não gera
efeito e que closure proposal preserva resposta final para revalidação do Omnichannel.

### 8.4 Lifecycle e proteções

- draft é mutável com revision/ETag; published é imutável;
- conflito de revision não sobrescreve edição de outro operador;
- variável inexistente, schema incompatível ou source/tool fora da allowlist bloqueia validação;
- teste não chama tool mutável nem sender;
- publish exige `customer_intelligence.prompts.publish`, confirmação e evals obrigatórios;
- alteração de `platform_guardrail` exige permissão platform;
- canary mostra escopo, percentual/allowlist, orçamento e rollback;
- rollback seleciona versão publicada anterior e registra motivo;
- logs comuns exibem IDs/checksum, não prompt integral;
- conteúdo vindo de conversa/documento é visualmente marcado como dado não confiável.

### 8.5 Migração legada e split revisável

O Prompt Studio inclui a visão “Migração legada” definida por CI-06. `agentVersionStatus`,
`promptVersionStatus`, `processVersionStatus` e `pipelineVersionStatus` usam consistentemente
`draft|validated|published|archived`; cada campo permanece separado de
`migrationState=inventoried|imported|review_required|validated|shadow|cutover|failed`.
`PromptLegacyMigrationPanel` lista por account/client/agente/versão e
`PromptLegacyMappingDiff` apresenta mapping/diff mascarado.

Os oito componentes precisam aparecer individualmente, sem colapsar o mega-prompt:

| # | Origem legada | Destino/revisão na UI |
|---|---|---|
| 1 | `layers.identity` | overrides distintos de triage/reply; copied/materialized |
| 2 | `layers.goal` | process prompt de triage |
| 3 | `layers.context` | agency policy distinta por processo; nunca platform guardrail |
| 4 | catálogo server-side de destinos | `routing.catalog` e source binding estruturados |
| 5 | `collect_field_defs` | variável/config tipada de coleta |
| 6 | `layers.guardrails` | trechos marcados nos drafts distintos de triage/reply |
| 7 | mensagens/contexto CRM server-side | context resolvers; conteúdo não é importado no prompt |
| 8 | `output_schema`/`schema_version` | schemas registrados distintos; mega-schema só compara |

O painel exibe writer state e migration state separadamente, source/schema/builder/hash,
`transformVersion`, target hashes, modos `copied|materialized|transformed|manual`,
`unmappedPaths`, reason codes, diff triage/reply, eval/shadow e histórico. Nunca mostra segredo,
conversa real ou payload bruto.

Permissões/ações:

- `customer_intelligence.prompts.view`: lista, detalhe e diff mascarado;
- `customer_intelligence.prompts.manage`: inventory/import/retry, mapear, manter legado ou descartar com justificativa,
  aceitar split e validate, sempre conforme APIs CI-06 e `expectedRevision`;
- `customer_intelligence.prompts.publish`: iniciar shadow e cutover apenas quando os gates
  CI-06/CI-10 permitirem;
- ausência de mapping completo, target/hash, aceite do split, estado compatível ou eval mantém a
  ação bloqueada com reason code.

Source path, target ID e estado não são campos livres; a UI escolhe opções devolvidas pelo backend.
Import cria targets funcionais como drafts e não os publica. Conflito CAS preserva a decisão local
para comparação; retry é idempotente. “Descartar” exige motivo e preserva a decisão auditável no
mapping. Nenhuma ação cria dual-write ou edita target published.

## 9. Contratos frontend/backend

### 9.1 Escopo comum de request

O frontend envia apenas:

| Campo | Regra |
|---|---|
| `clientAccountId` | selecionado entre clientes acessíveis; backend revalida |
| `relationshipId` | quando operação for da relação; backend cruza subject/client |
| filtros/paginação | allowlisted, bounded e normalizados |
| `revision`/`etag` | obrigatório em update concorrente |
| payload tipado | sem campos extras aceitos silenciosamente |

`ownerAccountId` vem do Principal/account ativa e não é autoridade quando enviado no body.

### 9.2 `CustomerProfileView`

| Campo | Tipo |
|---|---|
| `subject` | ID, displayName, aliases autorizados, merge status |
| `relationship` | ID, clientAccountId, lifecycle, owner, tags, timestamps |
| `identities` | tipo, maskedValue, verified status, provenance |
| `consents` | purpose/channel/status/granted/revoked/expiry refs |
| `summary` | ID, text sanitizado, version, `asOf`, prompt/context refs |
| `facts` | lista paginada com type/value/status/confidence/evidence refs |
| `recommendations` | lista separada com type/status/validity/rationale |
| `sourceStatuses` | sourceKey/status/freshness/reason code |
| `quality` | coverage/staleness/conflicts |
| `permissions` | capabilities efetivas do recurso |

Ausência de permissão remove a seção no backend; não deve retornar dado e pedir que o Vue o esconda.

### 9.3 `PromptStudioProcessView`

| Campo | Tipo/regra |
|---|---|
| `processKey` | chave registrada |
| `definitionId` | UUID |
| `inputSchemaVersion`, `outputSchemaVersion` | versões imutáveis |
| `allowedVariables` | key/type/classification/source/maxLength |
| `maxTools`, `maxSources`, `maxModels` | catálogos autorizados |
| `effectiveBinding` | binding ID, revision e layer version refs |
| `draft` | conteúdo/configuração somente com permissão |
| `published` | metadata imutável |
| `rollout` | mode/scope/from/to/status |
| `evaluations` | status/scores/violations/cost/latency |
| `canEdit`, `canTest`, `canPublish`, `canRollback` | derivados no backend |

O catálogo do Studio também pode retornar `legacyManagedCapabilities` read-only para
`transcription`/áudio e `video_summary`, usando a façade de compatibilidade existente. Esse bloco
contém apenas key legada, owner, status sanitizado e deep-link ref; não é `ProcessDefinition`, não
possui `processKey` canônica e não aceita mutação.

### 9.4 `SegmentWorkspaceView`

| Campo | Tipo/regra |
|---|---|
| `scope` | clientAccountId resolvido e modo standalone/agência |
| `fieldCatalog` | version, fields, types, operators, value schemas, caps e availability |
| `segments` | página com stable ID/key, metadata, active version/materialization e revision |
| `selectedSegment` | definição, capabilities efetivas e refs de auditoria |
| `versions` | metadata/diff refs; AST somente quando autorizado |
| `draft` | AST/policy/revision/validation; nunca SQL compilado |
| `evaluationRuns` | mode/status/asOf/counts/source statuses/reason codes/pollAfter |
| `previewSample` | bounded/masked e omitida sem permissões cumulativas |
| `materializations` | version/asOf/freshness/count/status, sem PII |
| `exports` | finalidade/canal/field set/status/counts/expiry; sem URL permanente |
| `permissions` | canView/canManage/canPublish/canEvaluate/canExport derivados no backend |
| `capabilities` | segmentation/export effective mode e motivos de disable |

Tipos frontend discriminam `group` e `predicate`; branches desconhecidos falham fechado e pedem
upgrade, não são descartados ou serializados de volta. `fieldCatalogVersion`, `definitionHash`,
`asOf`, `segmentId`, `versionId`, `runId` e `materializationId` permanecem opacos.

### 9.5 `LegacyPromptMigrationView`

| Campo | Tipo/regra |
|---|---|
| IDs/scope | mapping, client, agent e legacy version refs opacas |
| states | writer state e migration state em campos distintos |
| lineage | legacy schema/builder/source hash, transform version e target hash |
| components | exatamente oito mappings com source path registrado, target ref, mode e hashes |
| unmapped | path/reason/allowed decisions retornados pelo backend |
| split | diff triage/reply mascarado, acceptance/reviewer e eval/shadow refs |
| concurrency | mapping revision e capabilities efetivas |

Paths/targets/states são opções fechadas da API CI-06. Conteúdo integral de prompt só aparece
quando a permissão específica já o autorizaria no Studio; segredo, conversa e payload bruto nunca
entram nesse read model.

### 9.6 `RuntimeRunListItem`

| Campo | Tipo/regra |
|---|---|
| refs | run, client permitido, process/pipeline, binding/schema/model e correlation opacos |
| state | status, executor e reason/error code sanitizado |
| timing | queued/started/finished, duration/latency e attempts |
| usage | token/unit counts, custo/moeda conforme contrato; sem input/output |
| dependencies | source/tool status/counts, sem arguments/results |
| navigation | refs contextuais já autorizadas e capabilities de link |

O cursor e os filtros vêm de CI-06. Account nunca é autoridade do body/query; client é revalidado
no backend. Não existe mutation DTO em `CI08-RUNS-08`.

### 9.7 `IntelligenceAuditEventView`

| Campo | Tipo/regra |
|---|---|
| event | ID, action, entity type/ref, timestamp e client permitido |
| actor | type/ref/display sanitizado |
| change | old/new hashes e diff allowlisted |
| provenance | source/observation refs, reason e correlation code |
| observation | snapshot minimizado/mascarado, sensitivity/purpose/retention/timestamps |
| capabilities | canOpenObservation/canNavigate derivados no backend |

Audit list e observation detail usam os endpoints CI-04. Campo não autorizado é omitido no backend;
o Vue não recebe raw payload para redigir. Não existe mutation DTO em `CI08-AUDIT-09`.

### 9.8 Mutações

Toda mutação retorna ID, revision, status, actor/time e configuração efetiva sanitizada. Erros
usam reason codes estáveis para:

- conflito de revision;
- permissão/módulo/capability;
- variável/schema inválido;
- tool/source/model não permitido;
- segredo ausente/inválido;
- eval obrigatório reprovado;
- versão imutável;
- rollout concorrente;
- escopo inválido;
- field/operator/value/AST inválido;
- versão de field catalog stale;
- budget/custo excedido;
- run/materialização stale ou concorrente;
- finalidade/consentimento/field set/export indisponível.

## 10. Estado, reatividade e isolamento

- um store dedicado não mistura dados com Omnichannel/Analytics;
- cache key inclui account, client, subject/relationship ou segment/version/run/materialization,
  rota e filtros;
- troca de account/client incrementa generation local, aborta fetch e limpa todos os dados;
- resposta antiga não pode hidratar escopo novo;
- módulo/permissão desligado impede fetch no middleware, store e componente;
- watchers não sobrescrevem formulário `dirty`;
- draft local é reidratado somente após save/discard/confirm;
- request repetida é deduplicada; busca/paginação usam debounce/cancel;
- erro de uma aba não limpa dados válidos de outra;
- SSR/hydration não serializa token, prompt bruto ou evidência sensível;
- `localStorage` não guarda prompt, PII, segredo ou fonte de verdade.
- AST dirty pode permanecer apenas em memória; versão salva é sempre reidratada da API
  autoritativa.

### 10.1 Persistência, DDL e backfill

CI-08 não cria tabela, índice, constraint, migration ou backfill. Toda persistência pertence às
APIs autoritativas de CI-03/04/05/06. O frontend guarda somente estado efêmero de formulário e
cache; drafts duráveis, revisions, versões e auditoria ficam no PostgreSQL do módulo owner.

Se uma tela exigir campo que o contrato backend não possui, o pacote para e devolve a alteração à
spec owner. Não se cria coluna por conveniência dentro de pacote frontend nem se usa
`localStorage` como migração.

### 10.2 Fluxos de sucesso, duplicata, falha e concorrência

- **sucesso de leitura:** resolver gates → cancelar geração anterior → buscar read model → validar
  escopo/revision → substituir estado da mesma cache key;
- **sucesso de mutação:** validar dirty/revision → desabilitar repetição → enviar payload tipado →
  aceitar resposta autoritativa → limpar dirty → atualizar cache;
- **duplicata:** clique/retry usa idempotency key quando a operação não for naturalmente
  idempotente e mantém um único pending local;
- **falha:** preserva draft local, mostra reason/correlation code e não inventa estado de sucesso;
- **concorrência:** `409/revision conflict` não faz last-write-wins; oferece recarregar, comparar ou
  recriar draft;
- **troca de escopo:** aborta request, invalida generation/cache e remove toda PII anterior antes
  da nova busca;
- **resposta tardia:** generation/escopo divergente é descartado sem toast enganoso;
- **publish/rollback:** a UI aguarda resposta backend e nunca altera o binding ativo por otimismo.
- **preview/materialize/export:** criação usa idempotency key, acompanha o run retornado e nunca
  infere sucesso por timeout, count local ou toast.

## 11. Snapshot compacto no inbox

O Omnichannel mantém dados operacionais locais. A sidebar/painel recebe duas projeções explícitas:

1. `OperationalContactSnapshot`: nome do participante, canal, tags/estado operacionais;
2. `IntelligenceCompactSnapshot` opcional: subject/relationship ref, resumo curto autorizado,
   alertas, próxima recomendação e `asOf`.

Regras:

- indisponibilidade da inteligência não bloqueia lista, conversa ou resposta humana;
- painel mostra `indisponível`, `parcial` ou `desatualizado` sem apagar o operacional;
- edição completa abre `/inteligencia-clientes/:subjectId`;
- nenhuma evidência extensa, configuração de prompt ou dado cross-client entra no inbox;
- o inbox não faz chamadas diretas a repositories/APIs internas de Intelligence;
- o link só aparece com módulo, permissão e relação válida;
- a IA pode propor reply; a UI nunca chama provider para enviá-la automaticamente.

## 12. Acessibilidade, segurança visual e conteúdo

- todos os controles funcionam por teclado e possuem nome acessível;
- status não depende apenas de cor;
- confirmação informa escopo e efeito de publish/rollback/source enable;
- diff de prompt escapa HTML e marca dados não confiáveis;
- evidência/PII é mascarada por default e possui ação auditada para revelar;
- campos sensíveis não entram em autocomplete do browser;
- credencial existente nunca é reidratada em input;
- mensagens de erro não exibem stack, endpoint interno ou segredo;
- paginação e virtualização evitam carregar históricos inteiros;
- datas usam timezone explícita e formato local sem perder UTC;
- texto gerado por IA é identificado como síntese/recomendação, não fato humano;
- grupos/condições do segment builder possuem nome, ordem e erro acessíveis sem depender de
  drag-and-drop;
- count de segmento, elegibilidade e consentimento usam rótulos distintos, não apenas cor.

## 13. Configuração pelo painel: cobertura obrigatória

Devem ser configuráveis, quando seguras:

- persona, tom, estratégia e instruções por processo;
- fields/questions que o processo tenta coletar;
- agent/process bindings por agency/client;
- modelo, parâmetros, orçamento, timeout e retry dentro de bounds;
- fontes, tools e knowledge allowlisted;
- confidence thresholds e fallback policies;
- follow-up, cadência, quiet hours e limites;
- rollout, canary e rollback;
- retenção e finalidade somente entre opções aprovadas;
- alertas, freshness e comportamento em dado parcial.
- segmentos: nome/descrição, AST por builder, schedule/freshness/budget dentro dos caps;
- exportação: finalidade, canal, formato e field set somente entre opções aprovadas.

Devem aparecer como invariantes não editáveis por prompt:

- isolamento tenant/client;
- RBAC;
- FSM/lease/takeover;
- dedupe/idempotência;
- schemas e limites máximos;
- consentimento/opt-out obrigatório;
- catálogo máximo de tool/source/model;
- catálogo máximo de field/operator e caps do AST;
- membership não equivale a consentimento;
- canal/provider window;
- mensagem `PENDING`, outbox e sender.

Se um comportamento seguro ainda só existir em código, a UI deve sinalizar “não configurável nesta
versão” e abrir dívida explícita; não deve simular customização local.

## 14. Observabilidade e auditoria da UI

Eventos de produto sanitizados:

- workspace/page aberta por capability;
- busca/filtro/paginação com contagens, sem query PII;
- draft criado/salvo/descartado;
- validate/test/publish/canary/rollback iniciado e concluído;
- conflito de revision;
- fonte habilitada/desabilitada;
- CTA de capability de mídia legada aberto, sem conteúdo de `media_config`;
- inventory/import/review/validate/shadow/cutover legado por state/reason code, sem prompt/diff;
- segmento/draft/validate/publish/rollback solicitado e concluído;
- preview/materialization/export run por status/reason code e count bucket;
- download intent criado/expirado, sem URL ou identidade;
- runs page/filter/drawer aberto por status/count bucket, sem run/context ID;
- audit page/filter/observation drawer aberto por action/type bucket, sem entity/observation ID;
- navegação legado→canônico;
- uso residual de rota/API legada;
- erro por reason code e correlation ID.

Não enviar para analytics:

- prompt integral;
- telefone, e-mail, nome ou mensagem;
- evidence/raw payload;
- segredo/credential;
- resposta de tool/documento;
- client/subject ID em plataforma que não possua contrato de dados aprovado.

Auditoria autoritativa permanece no backend. Toast/evento frontend não prova mutação.

## 15. Pacotes atômicos e allowlists

### CI08-SHELL-01 — Workspace, gates e navegação

**Resultado:** shell navegável, sem tela falsa de feature ainda indisponível.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/index.vue` (novo);
- `web/app/components/customer-intelligence/CustomerIntelligenceWorkspace.vue` (novo);
- `web/app/components/customer-intelligence/CustomerIntelligenceNav.vue` (novo);
- `web/app/composables/customer-intelligence/useCustomerIntelligenceAccess.ts` (novo);
- `web/app/domain/customer-intelligence/api.ts` (novo);
- `web/app/domain/customer-intelligence/types.ts` (novo);
- `web/app/stores/customer-intelligence.ts` (novo);
- `web/app/utils/workspaces.ts`;
- `web/app/domain/utils/permissions.ts`;
- `web/layers/queue/nav.config.ts`;
- `web/app/middleware/module-enabled.global.ts`;
- testes novos diretamente correspondentes.

**Proibido:** `/crm`, `/inteligencia`, página de portfólio, Prompt Studio e inbox.

### CI08-PROFILE-02 — Perfil e proveniência

**Resultado:** perfil 360 explicável, sem editar módulos por atalhos.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/[subjectId].vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerProfileHeader.vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerProfileSummary.vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerFactsPanel.vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerEvidencePanel.vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerHistoryPanel.vue` (novo);
- `web/app/components/customer-intelligence/profile/CustomerSourceCoverage.vue` (novo);
- `web/app/composables/customer-intelligence/useCustomerProfile.ts` (novo);
- `web/app/domain/customer-intelligence/api.ts`;
- `web/app/domain/customer-intelligence/types.ts`;
- `web/app/stores/customer-intelligence.ts`;
- testes novos correspondentes.

**Proibido:** mutar ERP/Calendar/Omnichannel diretamente ou implementar recomendações CI-09.

### CI08-OFFLINE-03 — Timeline, entrada e import offline

**Resultado:** registrar, anexar, importar e consultar interação offline pelo perfil, mantendo
Customer Data autoritativo e Inteligência opcional.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/[subjectId].vue`;
- `web/app/components/customer-data/offline/OfflineInteractionTimeline.vue` (novo);
- `web/app/components/customer-data/offline/OfflineInteractionDrawer.vue` (novo);
- `web/app/components/customer-data/offline/OfflineAttachmentUpload.vue` (novo);
- `web/app/components/customer-data/offline/OfflineImportDialog.vue` (novo);
- `web/app/composables/customer-data/useOfflineInteractions.ts` (novo);
- `web/app/domain/customer-data/offline-api.ts` (novo);
- `web/app/domain/customer-data/offline-types.ts` (novo);
- testes novos correspondentes.

Gates usam `customer_data.offline_interactions.view|manage|import` por ação. É proibido upload
direto sem intent, marcar antivírus/processamento no front, enviar para LLM/n8n ou depender de
Customer Intelligence para salvar/listar.

### CI08-SOURCES-03 — Catálogo e administração de fontes

**Resultado:** integrar e validar no workspace os artefatos de fonte entregues por CI-05, sem criar
uma segunda implementação.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/fontes.vue` (entregue por CI-05);
- `web/app/components/customer-intelligence/sources/IntelligenceSourcesCatalog.vue` (entregue por CI-05);
- `web/app/components/customer-intelligence/sources/IntelligenceSourceConfigDrawer.vue` (entregue por CI-05);
- `web/app/components/customer-intelligence/sources/IntelligenceSourceHealth.vue` (entregue por CI-05);
- `web/app/composables/customer-intelligence/useCustomerIntelligenceSources.ts` (entregue por CI-05);
- `web/app/domain/customer-intelligence/sources.ts` (entregue por CI-05);
- `web/app/stores/customer-intelligence-sources.ts` (entregue por CI-05);
- testes novos correspondentes.

**Proibido:** connector backend, segredo, URL/SQL livre e módulo fonte.

### CI08-PROMPTS-04 — Prompt Studio e migração visual/API

**Resultado:** uma única experiência canônica, preservando integralmente o fluxo atual até cutover.

**Allowlist máxima nova:**

- `web/app/pages/inteligencia-clientes/prompts.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptStudio.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptProcessList.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptEditor.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptLayersPanel.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptVersionsPanel.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptEvaluationPanel.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptRolloutPanel.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptPipelinePanel.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptLegacyMediaNotice.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptLegacyMigrationPanel.vue` (novo);
- `web/app/components/customer-intelligence/prompts/PromptLegacyMappingDiff.vue` (novo);
- `web/app/composables/customer-intelligence/usePromptStudio.ts` (novo);
- `web/app/domain/customer-intelligence/prompt-api.ts` (novo);
- `web/app/domain/customer-intelligence/prompt-types.ts` (novo).

**Allowlist máxima de compatibilidade/reuso existente:**

- `web/app/components/omnichannel/config/ConfigAiAgent.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentCard.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentVersions.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentSimulator.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentAdvancedSettings.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentClientScope.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentMediaSettings.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentModelSelect.vue`;
- `web/app/components/omnichannel/config/ConfigAiRoleModelSelect.vue`;
- `web/app/components/omnichannel/config/ConfigAiToolsKnowledge.vue`;
- `web/app/components/omnichannel/config/ConfigAiCredentials.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentProviderKeys.vue`;
- `web/app/components/omnichannel/automation/AutomationAiConfigDrawer.vue`;
- `web/app/domain/omnichannel/config-api.ts`;
- `web/app/domain/omnichannel/config-types.ts`;
- `web/app/domain/omnichannel/ai-configuration-api.ts`;
- testes novos/atuais diretamente correspondentes.

O despacho deve reduzir essa allowlist ao menor conjunto necessário depois do inventário de
equivalência. É proibido apagar o fluxo legado neste pacote.

### CI08-INBOX-05 — Snapshot compacto

**Resultado:** inbox operacional independente, com inteligência opcional e link contextual.

**Allowlist máxima:**

- `web/app/components/omnichannel/OmnichannelInboxModule.vue`;
- `web/app/components/omnichannel/OmnichannelCRMProfilePanel.vue`;
- `web/app/components/omnichannel/inbox/InboxConversationsSidebar.vue`;
- `web/app/composables/omnichannel/useOmnichannelCRM.ts`;
- `web/app/composables/omnichannel/useOmnichannelCRM.test.ts`;
- `web/app/domain/customer-intelligence/compact-api.ts` (novo);
- `web/app/domain/customer-intelligence/compact-types.ts` (novo);
- testes novos correspondentes.

**Proibido:** envio, composer, realtime, fila, FSM e configuração completa de inteligência.

### CI08-QA-06 — Contratos e regressão visual

**Resultado:** provar isolamento, acessibilidade, equivalência e ausência de duplicação.

**Allowlist máxima:**

- arquivos `*.test.ts`/`*.spec.ts` novos junto aos componentes/composables CI-08;
- fixtures sanitizadas sob diretório de teste frontend aprovado;
- configuração de teste somente se o despacho comprovar necessidade exclusiva desta spec.

**Proibido:** produção, snapshots com PII, update indiscriminado de snapshots.

### CI08-SEGMENTS-07 — Segmentos CRM/marketing

**Resultado:** entregar `/inteligencia-clientes/segmentos` com builder determinístico, versões,
preview/materialização e exportação separada, consumindo somente APIs CI-03.

**Depende de:** `CI03-API-SEGMENTS`, `CI03-JOB-SEGMENTS`; a área de export depende também de
`CI03-API-SEGMENT-EXPORTS` e gates jurídicos. Sem export, o restante da página continua utilizável.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/segmentos.vue` (novo);
- `web/app/components/customer-data/segments/CustomerSegmentsWorkspace.vue` (novo);
- `web/app/components/customer-data/segments/SegmentList.vue` (novo);
- `web/app/components/customer-data/segments/SegmentBuilder.vue` (novo);
- `web/app/components/customer-data/segments/SegmentConditionGroup.vue` (novo);
- `web/app/components/customer-data/segments/SegmentVersionsPanel.vue` (novo);
- `web/app/components/customer-data/segments/SegmentEvaluationPanel.vue` (novo);
- `web/app/components/customer-data/segments/SegmentMaterializationsPanel.vue` (novo);
- `web/app/components/customer-data/segments/SegmentExportDialog.vue` (novo);
- `web/app/composables/customer-data/useCustomerSegments.ts` (novo);
- `web/app/domain/customer-data/segment-api.ts` (novo);
- `web/app/domain/customer-data/segment-types.ts` (novo);
- `web/app/stores/customer-segments.ts` (novo);
- testes novos diretamente correspondentes.

O despacho reduz componentes se o design puder compor a mesma experiência com menos arquivos.
Todas as páginas usam `AdminPageHeader`. Gates por ação usam
`customer_data.segments.view|manage|publish|evaluate|export`; ausência de export não desabilita
builder/preview. É proibido criar editor SQL/JSON livre, chamar LLM, persistir AST/PII no browser,
implementar backend/migration, campanha ou sender.

### CI08-RUNS-08 — Atendimentos e runs

**Resultado:** entregar `/inteligencia-clientes/atendimentos` como leitura sanitizada e paginada do
`GET /v1/customer-intelligence/runs` definido por CI-06.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/atendimentos.vue` (novo);
- `web/app/components/customer-intelligence/runs/IntelligenceRunsWorkspace.vue` (novo);
- `web/app/components/customer-intelligence/runs/IntelligenceRunsFilters.vue` (novo);
- `web/app/components/customer-intelligence/runs/IntelligenceRunsTable.vue` (novo);
- `web/app/components/customer-intelligence/runs/IntelligenceRunDrawer.vue` (novo);
- `web/app/composables/customer-intelligence/useIntelligenceRuns.ts` (novo);
- `web/app/domain/customer-intelligence/runs-api.ts` (novo);
- `web/app/domain/customer-intelligence/runs-types.ts` (novo);
- testes novos diretamente correspondentes.

Usa `AdminPageHeader` e gate `customer_intelligence.runs.view`. É pacote GET-only: não inventa
run detail/retry/cancel, não chama provider e não recebe/exibe prompt, input/output, mensagem,
credential, tool arguments/results, evidence ou payload bruto.

### CI08-AUDIT-09 — Auditoria e observações

**Resultado:** entregar `/inteligencia-clientes/auditoria` sobre `audit-events`/`observations`
CI-04, com filtros, diff e proveniência sanitizados.

**Allowlist máxima:**

- `web/app/pages/inteligencia-clientes/auditoria.vue` (novo);
- `web/app/components/customer-intelligence/audit/IntelligenceAuditWorkspace.vue` (novo);
- `web/app/components/customer-intelligence/audit/IntelligenceAuditFilters.vue` (novo);
- `web/app/components/customer-intelligence/audit/IntelligenceAuditEventList.vue` (novo);
- `web/app/components/customer-intelligence/audit/IntelligenceAuditDiffPanel.vue` (novo);
- `web/app/components/customer-intelligence/audit/IntelligenceObservationDrawer.vue` (novo);
- `web/app/composables/customer-intelligence/useIntelligenceAudit.ts` (novo);
- `web/app/domain/customer-intelligence/audit-api.ts` (novo);
- `web/app/domain/customer-intelligence/audit-types.ts` (novo);
- testes novos diretamente correspondentes.

Usa `AdminPageHeader` e gate `customer_intelligence.audit.view`. É pacote GET-only: não cria
endpoint, reveal genérico, export, revisão ou mutação e não recebe/exibe prompt, mensagem,
documento, source payload, segredo, PII não autorizada ou JSON bruto.

## 16. Arquivos e áreas proibidos

Sem novo despacho/owner, CI-08 não altera:

- backend, migrations, n8n ou provider;
- `web/app/pages/crm.vue`;
- `web/app/pages/inteligencia.vue`;
- lógica de mensagem, composer, realtime ou sender;
- telas de ERP, Calendar, Site ou Analytics;
- página/algoritmo de portfólio pertencente a CI-09;
- qualquer algoritmo/backend de segmentação pertencente a CI-03;
- módulo legado `automation` fora dos wrappers de IA listados;
- qualquer arquivo de `socialpublishing`/`social-publishing`;
- `.env`, secrets ou credenciais.

## 17. Rollout, migração e rollback

### 17.1 Migração visual/API

1. inventariar campos, ações, permissões, estados e deep links atuais;
2. congelar matriz de equivalência;
3. criar composable/API façade canônica;
4. adaptar componentes atuais à façade sem mudar entrada;
5. montar o mesmo núcleo no workspace novo;
6. executar testes e telemetria paralela sem dual-write;
7. liberar workspace novo por capability;
8. apontar links antigos para o núcleo canônico;
9. medir uso, erro e paridade;
10. CI-10 decide redirect/depreciação; remoção fica em pacote separado.

Se `/omnichannel/automacao` também contiver automações operacionais, somente a subseção de IA pode
ser migrada. Não se redireciona a rota inteira sem provar que todo o restante possui destino.

### 17.2 Rollback

- desabilitar a capability do workspace novo;
- manter façade e fluxo antigo funcionais;
- reverter somente binding/entrada visual, sem editar versão publicada;
- limpar store/cache no account switch;
- não reativar API writer antiga depois do writer cutover sem estratégia CI-10;
- nenhum rollback reenvia mensagem ou repete mutação.

### 17.3 Segmentos

1. registrar rota/nav atrás de `customer_data.segmentation=off`;
2. validar types/fixtures do field catalog e AST sem backend mock em produção;
3. liberar lista/builder para client canary;
4. liberar preview em `shadow` e comparar counts/diagnósticos legados;
5. liberar publish/materialização somente após CI-03 READY;
6. manter export `off`, depois `shadow` apenas para relatório de elegibilidade;
7. liberar download canary somente após consentimento/revogação/storage/TTL aprovados e testados.

Rollback desliga a capability da rota, cancela polling e limpa store. Não edita published, não
reativa dual-write e não reaproveita export intent. A definição/materialização autoritativa
permanece no backend para retomada auditável.

### 17.4 Runs e auditoria

1. congelar fixtures sanitizadas dos GETs CI-06/CI-04;
2. provar gates negativos e field omission no backend;
3. registrar páginas/nav sem fetch quando módulo/permissão estiver ausente;
4. liberar GET-only para client canary;
5. medir paginação, erro e payload size sem registrar filtros/IDs sensíveis.

Rollback remove/desabilita apenas a entrada frontend e cancela requests; não altera run, evento,
observação, API ou writer backend.

## 18. Testes e comandos

Comandos futuros a partir de `web/`, ajustados aos scripts reais no despacho:

```text
npm run lint
npm run typecheck
npm test -- --run
npm run build
```

Cenários obrigatórios:

- módulo desabilitado gera zero fetch;
- usuário sem permissão não recebe conteúdo protegido;
- account/client switch aborta resposta antiga e limpa PII;
- out-of-scope não revela subject;
- formulário dirty não é sobrescrito por watcher/reload;
- conflito de revision preserva edição e mostra resolução;
- prompt inválido não publica;
- published não é editável;
- catálogo mantém exatamente as 13 process keys novas; áudio/transcrição e vídeo não viram
  `ProcessDefinition`;
- card de mídia legada é read-only, mostra status/owner e CTA abre
  `ConfigAiAgentMediaSettings` sem chamar API writer nova;
- writer state, version status e migration state aparecem separados;
- painel cobre os oito componentes, split triage/reply e unmapped paths;
- prompts.view não ganha ações; prompts.manage usa expected revision e motivo; prompts.publish
  continua necessário para shadow/cutover;
- unmapped/hash divergente/split sem aceite bloqueiam validate/publish e conflito CAS não
  sobrescreve revisão;
- diff de migração não contém segredo, conversa ou payload bruto;
- segredo nunca é reidratado;
- simulator/test não causa tool mutável nem envio;
- fluxo atual e novo leem/escrevem pela mesma façade;
- todos os campos atuais possuem destino ou depreciação explícita;
- inbox funciona com Customer Intelligence indisponível;
- snapshot compacto não contém evidência extensa/cross-client;
- offline create/list funciona com Inteligência ausente;
- upload/import offline respeita intent, scan, dry-run, relatório e permissões;
- rota de segmentos gera zero fetch quando módulo/capability/permissão estiver ausente;
- agência nunca oferece client implícito nem opção cross-client;
- field catalog desconhecido/atualizado falha fechado e preserva draft para reconciliação;
- builder serializa somente AST tipado e não oferece SQL/JSON/JSONPath/URL/regex livre;
- published é read-only; rollback troca binding sem copiar/editar versão;
- preview acompanha run, usa `asOf`, não cria membership e limpa amostra no scope switch;
- materialização mostra partial/stale/failure sem substituir snapshot corrente no client;
- amostra/membros respeitam permissões cumulativas e identidades mascaradas;
- membership e elegibilidade/consentimento aparecem distintos;
- export shadow não baixa, intent expira, URL/PII não persiste e nenhum fluxo envia mensagem;
- retry/clique duplo converge por idempotency e polling para em unmount/visibility/cap;
- `/atendimentos` sem `runs.view` gera zero fetch e não confirma runs existentes;
- runs pagina com cursor estável, descarta resposta de outro scope e nunca renderiza
  prompt/input/output/message/tool args/results;
- `/auditoria` sem `audit.view` gera zero fetch e não confirma eventos/observações;
- audit diff/observation usa somente projeção mascarada e não oferece JSON bruto/export/reveal;
- runs usa somente o GET de lista CI-06, sem inventar detail/retry/cancel; audit usa somente os
  GETs `audit-events`/`observations` CI-04, sem inventar review/mutação;
- navegação por teclado, focus e status acessíveis;
- `/crm` e `/inteligencia` continuam com comportamento anterior.

## 19. Critérios de aceite

- [ ] workspace, rotas e owners estão inequívocos;
- [ ] profile separa fato, claim, evidência, resumo e recomendação;
- [ ] fontes mostram origem, freshness, health e configuração allowlisted;
- [ ] interação offline possui timeline/form/import utilizáveis e Customer Data autoritativo;
- [ ] `/inteligencia-clientes/segmentos` usa `AdminPageHeader`, client scope e gates por ação;
- [ ] segment builder deriva fields/operators do catálogo e não possui SQL/JSON livre;
- [ ] versões, preview runs, materializações e rollback são explicáveis e auditáveis;
- [ ] membership, consentimento, elegibilidade e exportação são estados visualmente separados;
- [ ] exportação usa workflow/intents próprios e nunca oferece envio/campanha;
- [ ] a tela de segmentos é determinística e não depende de IA;
- [ ] Prompt Studio usa prompt específico por processo;
- [ ] imagem/documento permanecem nos 13 processos novos;
- [ ] áudio/transcrição e vídeo aparecem somente como legado read-only com CTA ao Omnichannel;
- [ ] nenhuma process key/API writer foi inventada para mídia legada;
- [ ] migração mostra oito mappings, split, hashes, estados separados e blockers revisáveis;
- [ ] revisão/import/validate/shadow/cutover respeitam permissions, CAS e lifecycle CI-06;
- [ ] layers, bindings, modelos, tools, sources, evals e rollout são explicáveis;
- [ ] toda customização segura prevista possui controle tipado e persistência backend;
- [ ] invariantes de segurança/operação não são apresentados como prompt editável;
- [ ] fluxo `ConfigAiAgent*` foi reutilizado/evoluído, não duplicado;
- [ ] equivalência funcional e telemetria precedem redirect/remoção;
- [ ] published é imutável e rollback troca binding;
- [ ] módulo/permissão/account/client isolam fetch e cache;
- [ ] `/atendimentos` usa somente GET runs CI-06 e exibe custo/latência/status/refs sanitizados;
- [ ] `/auditoria` usa somente audit-events/observations CI-04 e omite payload bruto/PII;
- [ ] ambas as páginas usam `AdminPageHeader`, cursor, states e gates backend;
- [ ] inbox continua operacional sem inteligência;
- [ ] segredos e PII não vazam em DOM, store persistida ou analytics;
- [ ] nenhuma área fora da allowlist foi alterada.

## 20. Stop conditions

Parar e devolver ao orquestrador se:

- CI-03/04/06 não tiver contrato estável ou fixtures sanitizadas;
- a UI precisar consultar tabelas ou combinar APIs sem gateway autorizado;
- for necessário criar um segundo editor de prompt para ganhar velocidade;
- áudio/transcrição ou vídeo exigirem process key, editor ou writer novo sem decisão canônica;
- migração legada misturar version status com migration state, omitir um dos oito mappings ou
  permitir publish com unmapped/split não aceito;
- algum campo/capability atual não tiver destino na matriz de equivalência;
- API antiga e nova puderem escrever simultaneamente a mesma entidade;
- permissão existir apenas no frontend;
- account/client switch não puder cancelar/invalidar requests;
- segredo precisar ser reidratado;
- prompt for usado para habilitar source/tool, ignorar consentimento ou controlar sender;
- segment builder precisar de SQL/JSONPath/URL/field arbitrário ou LLM não aprovado;
- página aceitar client implícito em agência ou manter cache/amostra de outro client;
- preview persistir membership, published precisar ser editado ou rollback copiar versão;
- exportação misturar membership com consentimento, expor URL durável/PII ou acionar campanha;
- runs/audit exigirem novo endpoint/mutation backend ou receberem prompt/input/output/payload bruto;
- redirecionamento causar loop, quebrar deep link ou esconder automação não-IA;
- arquivo da allowlist estiver sujo por outra trilha;
- surgir necessidade de tocar `socialpublishing`, backend, migration ou workflow;
- testes encontrarem cross-tenant, PII em cache/analytics ou regressão do inbox.

## 21. Handoff obrigatório

Cada pacote deve informar:

- resultado e rotas entregues;
- arquivos lidos/alterados e confirmação da allowlist;
- matriz de equivalência atualizada;
- APIs/façades e revisions utilizadas;
- field catalog/AST versions, segment bindings e evaluation/materialization/export run IDs;
- lineage/mapping revisions, oito componentes e states da migração de prompts;
- contratos GET/cursors e campos sanitizados de runs/audit;
- permissões/capabilities testadas;
- testes, build e evidências acessíveis/visuais;
- telemetria de rota/API antiga e nova;
- riscos, feature flags e rollback;
- confirmação de que não existe segundo Prompt Studio;
- confirmação de zero backend/migration/workflow/sender;
- próximo pacote desbloqueado ou blocker objetivo.
