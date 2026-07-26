# CI-10 — Hardening, rollout, cutover, depreciação e retirada

- **Status:** READY-PREPROD — hardening local autorizado; cutover não autorizado
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Plataforma/Release
- **Owners aprovadores:** Omnichannel, Customer Data, Customer Intelligence, Frontend,
  Segurança/Privacidade, SRE/Operação e Produto
- **Depende de:** CI-07, CI-08 e CI-09 concluídas; todas as predecessoras em estado compatível
- **Autoriza implementação ou cutover:** implementação local sim; cutover/deploy/exclusão não

> Esta spec é um runbook de decisão ainda em `DRAFT`. Não autoriza deploy, feature flag, backfill,
> writer switch, redirect, `DROP`, alteração de workflow ou acesso a dados de produção.

## 1. Resultado único e verificável

Conduzir a evolução para Customer Data + Customer Intelligence sem:

- indisponibilizar o chat;
- criar dois writers para a mesma entidade;
- duplicar mensagem, campanha, evento ou ação;
- perder proveniência, prompt version ou auditoria;
- vazar dado entre account/client;
- remover fluxo/API/tabela ainda consumido;
- tornar painel, n8n ou LLM autoridades operacionais.

O cutover só é concluído quando cada entidade possui owner/writer único, as fachadas convergem para
a nova autoridade, shadow/canary atingem gates aprovados, rollback foi ensaiado e a retirada de cada
artefato legado ocorre em pacote destrutivo independente.

## 2. Requisitos locais derivados da governança

`CI10-REQ-*` serve para rastreabilidade desta spec; decisões novas permanecem exclusivamente na
sequência `CI-DEC-*` de `GOVERNANCA.md`.

| ID | Requisito |
|---|---|
| CI10-REQ-001 | transição de writer é `legacy → shadow → new`, sem estado dual |
| CI10-REQ-002 | shadow compara e mede; nunca produz novo efeito |
| CI10-REQ-003 | cutover é por owner/client/entity/process/channel, não big-bang |
| CI10-REQ-004 | sender permanece no Go/Omnichannel em todos os estados |
| CI10-REQ-005 | versão publicada de prompt é imutável; rollback reponta binding |
| CI10-REQ-006 | APIs e telas antigas viram façades antes de serem depreciadas |
| CI10-REQ-007 | equivalência, telemetria e janela de sunset precedem remoção |
| CI10-REQ-008 | DDL, backfill, cutover, depreciação e remoção são pacotes separados |
| CI10-REQ-009 | cada pacote de remoção possui um único alvo e aprovação explícita |
| CI10-REQ-010 | falha de Customer Intelligence nunca derruba recebimento/resposta humana |

## 3. Pré-condições globais

Nenhuma etapa de produção começa sem:

- CI-00 e decisões canônicas aplicáveis aprovadas;
- CI-01 a CI-09 com handoffs, testes e débitos conhecidos;
- owner e runbook de incidente por módulo;
- inventário de banco, volumes, FKs, consumidores, rotas e jobs na instância alvo;
- migrations numeradas após reinspeção do disco;
- backups/restauração testados de acordo com a política da plataforma;
- retenção, exclusão, legal hold e crypto-shredding aprovados;
- permissions/capabilities sincronizadas e testadas por account;
- dashboards, alerts e correlation IDs em operação;
- feature/capability flags server-side com histórico;
- rollback técnico e decisão de negócio ensaiados;
- janela, responsáveis e canal de comunicação definidos.

Portfólio possui gates adicionais de base legal, coorte, supressão e anti-reidentificação. A
aprovação do restante não liga portfólio implicitamente.

## 4. Unidades de cutover e writer matrix

O estado é persistido por `account_id + client_account_id + entity_key`; quando necessário inclui
channel/process. UI não escolhe writer por estado local.

| `entity_key` | Writer `legacy` | Shadow | Writer `new` |
|---|---|---|---|
| `channel_client_binding` | perfil/mapeamento legado | binding novo compara | binding autoritativo CI-01 |
| `subject_identity` | `messaging.contacts`/origens | Customer Data reconstrói | Customer Data |
| `relationship_profile` | campos CRM legados | Customer Data compara | Customer Data |
| `contact_intelligence` | `messaging.contact_intelligence` | Intelligence Bank observa | Customer Intelligence |
| `agent_prompt_config` | `messaging.ai_*` | Prompt Registry compara | Customer Intelligence |
| `conversation_pipeline_config` | fluxo combinado do Brain atual | pipeline triage/reply compara | Pipeline Registry |
| `interaction_decision` | runtime IA atual | novo runtime sem efeitos | Customer Intelligence produz; Omni aceita |
| `compact_inbox_projection` | CRM panel legado | snapshot novo paralelo | façade lê nova projeção |
| `recommendations` | inexistente/experimental | geração invisível | Customer Intelligence |
| `portfolio` | inexistente | agregado interno protegido | somente após gates próprios |

Invariantes:

- exatamente um writer por linha/unidade;
- shadow não é writer de negócio;
- façades antigas podem escrever somente chamando o writer efetivo;
- não há trigger de dual-write;
- read fallback não pode virar write fallback;
- transição usa revision/compare-and-swap e auditoria;
- um estado desconhecido ou inconsistente falha fechado;
- estado `new` não volta a `legacy` sem reconciliação reversa aprovada.

## 5. Sequência de dependências

```text
binding de canal/cliente
    -> subject/identity/relationship
        -> Intelligence Bank + fontes
            -> Prompt Registry/runtime
                -> bridge Omnichannel
                    -> frontend/façades
                        -> recomendações
                            -> portfólio (gates próprios)
                                -> depreciações
                                    -> remoções unitárias
```

Não promover uma etapa porque “a tela parece funcionar”. Cada nó exige provas de dados,
concorrência, observabilidade e modo degradado.

## 6. Hardening multi-tenant, segurança e privacidade

### 6.1 Multi-tenant

- Principal define `account_id`;
- client vem de catálogo permission-scoped e é revalidado;
- repository repete account em filtros e FKs compostas;
- out-of-scope retorna 404;
- cache/job/outbox/idempotency key incluem o escopo;
- account/client switch limpa stores e cancela requests;
- eventos carregam account/client e consumer confirma ambos;
- testes negativos cobrem leitura, escrita, merge, prompt, run, source e portfólio;
- nenhum modo degradado remove filtro tenant.

### 6.2 Prompt, tool e source

- prompt injection de mensagem/documento/tool é tratado como dado não confiável;
- process/input/output schema são fixados por versão;
- source/tool/model vêm de registry allowlisted;
- SQL, URL, path e operação livre são recusados;
- tools mutáveis exigem modo, approval e idempotência;
- secrets são write-only, cifrados e nunca reidratados;
- logs/traces não contêm prompt compilado, mensagem integral ou segredo;
- budgets limitam tokens, calls, bytes, tempo e custo;
- published é imutável;
- rollback troca binding/rollout.

### 6.3 LGPD

Antes de produção real, deve existir:

- inventário de data class por tabela/campo/source;
- finalidade e base legal por processo/fonte;
- consentimento/opt-out efetivos;
- retention policy versionada;
- export/retificação/exclusão por subject/relationship;
- tombstone que bloqueia reingestão;
- propagação idempotente de delete/anonymize;
- legal hold;
- política para backups e prazo de expurgo;
- crypto-shredding e rotação de chave onde necessário;
- auditoria de acesso/reveal/export;
- procedimento de incidente.

Delete não faz cascade cego de mensagens autoritativas nem quebra trilha legal. Cada owner executa
sua ação e confirma por evento/outcome durável.

### 6.4 Cross-client

- individual permanece desligado;
- agregado é produzido antes de entrar no modelo;
- coorte/supressão/differencing são hard gates;
- categorias sensitive/restricted são bloqueadas por default;
- autorização de portfólio exige organização, owner agência, permissões, purpose e policy;
- nenhuma telemetria inclui contributors.

## 7. Observabilidade mínima

### 7.1 Identificadores

Toda cadeia nova registra, conforme aplicável:

- request/correlation/causation/idempotency IDs;
- account/client/subject/relationship IDs internos;
- source/job/run/context snapshot;
- pipeline/version, process run, prompt definition/version/binding;
- agent/model/schema/tool/source refs;
- rollout/bucket;
- dispatch/generation/conversation/message/outbox;
- writer state e façade utilizada;
- reason/error codes sanitizados.

### 7.2 Métricas

| Domínio | Métricas mínimas |
|---|---|
| Customer Data | match/merge/review, unresolved, writer state, divergence, latency |
| Intelligence Bank | observations/claims/facts/conflicts, freshness, rebuild, tombstone |
| fontes | backlog, lag, retry, dead-letter, bytes, custo e stale |
| pipelines/prompts/runtime | branch/process refs, schema validity, eval, latency, tokens, custo, model/tool/source errors |
| Omnichannel bridge | dispatch, retry, reject, generation mismatch, takeover, accepted outcome |
| sender | pending/sent/failed, attempts e provider latency, sem duplicar owner |
| frontend/façade | rota/API legada, nova, erros, conflitos de revision |
| recomendações | lifecycle, acceptance, invalidation, execution e outcome |
| portfólio | query/suppression/privacy budget, sem contributors |

### 7.3 Alertas absolutos

Alertas bloqueadores:

- qualquer acesso cross-tenant confirmado;
- qualquer envio fora do Omnichannel;
- qualquer efeito duplicado;
- dois writers ativos;
- decisão vencida que causou efeito;
- segredo/PII proibida em log;
- invalid schema aceito;
- outbox sem progresso acima do limite aprovado;
- tombstone reingerido;
- portfólio retornando contributor/row-level.

## 8. Evals, shadow e critérios de promoção

### 8.1 Tipos de prova

1. fixtures determinísticas por processo;
2. corpus sintético adversarial;
3. regressão com casos históricos autorizados/cifrados;
4. schema/policy/security evals;
5. human review;
6. model judge somente como sinal adicional;
7. shadow online sem efeitos;
8. canary real limitado;
9. comparação de custo/latência/qualidade.

### 8.2 Métricas e fórmulas

| Gate | Fórmula/condição |
|---|---|
| tenant safety | violações confirmadas = 0 |
| sender ownership | chamadas de provider fora do worker Omni = 0 |
| duplicate effect | efeitos com mesma chave lógica >1 = 0 |
| trace completeness | decisões sem pipeline/process run/prompt/context/model/schema refs = 0 |
| schema safety | outputs inválidos aceitos = 0 |
| writer safety | unidades com dois writers = 0 |
| shadow mutation | mensagens/outboxes/handoffs/closes/profile writes do candidato = 0 |
| backfill integrity | source eligible = migrated + quarantined explicada |
| semantic agreement | matriz por outcome/process, não média global |
| quality | score por assertions obrigatórias e revisão humana |
| latency | p50/p95/p99 por processo versus baseline |
| cost | custo por run aceito, cliente, processo e modelo |
| human takeover | taxa e motivo por rollout |
| source coverage | `ok/partial/stale/error` por processo |

Os seis primeiros gates são absolutos. Os thresholds quantitativos de qualidade, amostra, latência,
custo e divergência continuam `DRAFT` até aprovação canônica. Não inserir números silenciosos no
código/painel.

### 8.3 Matriz de thresholds a preencher

| Processo | Amostra mínima | Qualidade mínima | p95 máximo | Custo máximo | Divergência máxima | Aprovador |
|---|---:|---:|---:|---:|---:|---|
| conversation.triage | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Omni |
| conversation.reply | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Omni |
| conversation.handoff_summary | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Operação |
| memory.extract | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Data/Privacidade |
| profile.summary | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto |
| recommendation.follow_up | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Operação |
| recommendation.offer | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Marketing |
| recommendation.important_dates | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Privacidade |
| source.suggest | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Data/Privacidade |
| portfolio.opportunity | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Jurídico/Produto |
| media.image_analysis | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Privacidade |
| media.document_analysis | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/Privacidade |
| quality.review | PENDENTE | PENDENTE | PENDENTE | PENDENTE | PENDENTE | Produto/QA |

Não se aceita wildcard: cada um dos treze processos possui threshold e aprovação próprios. Sem
preenchimento e aprovação, o processo não passa de shadow.

`media.audio_transcription` e `media.video_summary` não integram este catálogo inicial: permanecem
capacidades legadas do Omnichannel conforme CI-DEC-031. Se forem aprovadas como processos novos,
CI-00, CI-06, CI-08 e esta matriz ganham linhas próprias antes de qualquer shadow/cutover.

### 8.4 Stop automático de rollout

Rollout pausa automaticamente quando:

- gate absoluto falha;
- taxa/latência/custo ultrapassa threshold aprovado;
- eval required falha;
- provider/source apresenta incidente;
- backlog ameaça SLO;
- consentimento/policy não pode ser resolvido;
- versão/configuração é revogada;
- owner aciona kill switch.

Pausar impede novas alocações; não abandona jobs/outboxes já autoritativos sem protocolo.

## 9. Performance, carga e capacidade

### 9.1 Planos obrigatórios

- `EXPLAIN (ANALYZE, BUFFERS)` com dataset representativo e sanitizado;
- índices tenant-first para hot paths;
- paginação por cursor, nunca offset irrestrito;
- backfill em batches com pausa/resume;
- limite de context bytes/tokens;
- limites de tools/sources por run;
- timeouts/circuit breakers;
- pool/conexões sob carga;
- outbox/worker claim com `SKIP LOCKED` ou protocolo equivalente;
- teste de retry storm, provider rate limit e source unavailable;
- frontend sem fan-out ao abrir página;
- BI/portfólio sem consulta aberta;
- retenção/particionamento avaliados para observations/runs/audit.

### 9.2 SLOs a aprovar

SLOs precisam existir por:

- webhook/inbound persistido;
- envio humano;
- dispatch IA;
- construção de contexto;
- decisão IA;
- aceitação/outbox;
- ingestão de fonte;
- perfil/read model;
- Prompt Studio publish/rollback;
- recommendation job;
- portfolio query.

Cada SLO define p95/p99, error budget, janela e modo degradado. Customer Intelligence nunca entra no
SLO crítico de persistência inbound/resposta humana.

## 10. Backfill, reconciliação e cutover de writer

### 10.1 Contrato administrativo `CutoverPlan.v1`

O plano é persistido/auditado antes da janela. Campos mínimos:

| Campo | Regra |
|---|---|
| `schemaVersion` | literal `cutover.plan.v1` |
| `planId`, `idempotencyKey` | opacos e únicos |
| `accountId`, `clientAccountId` | escopo validado; nunca livres do body |
| `entityKey` | unidade registrada na writer matrix |
| `channelId`, `processKey` | opcionais conforme granularidade |
| `fromState`, `toState` | somente transição permitida |
| `expectedRevision` | compare-and-swap obrigatório |
| `candidateVersionRefs` | código, migration, prompt/binding/config efetivos |
| `preflightEvidenceRefs` | inventário, dry-run, backup, segurança e QA |
| `thresholdPolicyVersion` | gates quantitativos aprovados |
| `windowStart`, `windowEnd` | janela aprovada |
| `dryRun` | obrigatório antes do modo efetivo |
| `requestedBy`, `approvedBy` | atores distintos quando policy exigir |
| `reason` | texto limitado e auditável |
| `rollbackPlanRef` | runbook testado |

### 10.2 Contrato `CutoverResult.v1`

| Campo | Regra |
|---|---|
| `schemaVersion` | literal `cutover.result.v1` |
| `planId`, `operationId` | correlação e dedupe |
| `status` | `validated`, `blocked`, `running`, `completed`, `rolled_back`, `failed` |
| `beforeState`, `afterState`, `revision` | estado autoritativo |
| `countSummary` | eligible/migrated/quarantined/invalid, sem PII |
| `checksumRefs` | fingerprints e relatórios, não payload bruto |
| `gateResults` | gate ID, resultado, threshold version e evidence ref |
| `smokeEvidenceRefs` | old/new façade, leitura/escrita e modo degradado |
| `startedAt`, `completedAt` | timestamps |
| `rollbackStatus`, `rollbackEvidenceRef` | prova quando acionado |
| `incidentRefs`, `warnings` | IDs/reason codes sanitizados |

Repetir o mesmo `planId + idempotencyKey` retorna a operação existente. Uma segunda operação
concorrente para a mesma unidade falha por lease/revision; nunca executa dois switches.

### 10.3 Preflight

Para cada entidade:

- contar total, elegível, inválido, órfão, ambíguo e já migrado;
- registrar máximo ID/timestamp inicial;
- confirmar owner/source;
- validar client/relationship;
- estimar tempo, locks, I/O e espaço;
- executar dry-run;
- gerar checksum/fingerprint reproduzível;
- criar relatório de exceções sem PII;
- confirmar rollback.

### 10.4 Execução

- migration aditiva já aplicada separadamente;
- backfill não cria DDL;
- batches pequenos, ordenação estável e watermark;
- idempotency key por linha/entidade;
- lease para impedir dois runners;
- pause/resume;
- retries classificados;
- nenhuma escolha automática em ambiguidade;
- source não é alterada;
- erro fica em quarentena;
- métricas e checkpoint duráveis.

### 10.5 Reconciliação

Provas mínimas:

```text
eligible_count = migrated_count + quarantined_count
duplicate_logical_keys = 0
cross_tenant_links = 0
unexplained_checksum_divergence = 0
writer_state_dual = 0
```

Além de contagem:

- comparar amostra estratificada;
- comparar joins/FKs e órfãos;
- validar timestamps/timezone;
- validar consentimentos/tombstones;
- conferir versões/fingerprints;
- testar façade antiga e API nova sobre a mesma entidade.

### 10.6 Switch

1. congelar mudança de configuração somente no escopo afetado, se necessário;
2. drenar jobs relevantes;
3. executar delta catch-up;
4. reconciliar novamente;
5. compare-and-swap `shadow → new`;
6. apontar façade ao novo writer;
7. executar smoke de leitura/escrita;
8. liberar tráfego gradualmente;
9. observar janela aprovada;
10. descongelar;
11. preservar legado read-only até sunset.

O switch não apaga dado nem FK.

## 11. Cutover do runtime de IA e Omnichannel

### 11.1 Ordem

1. Customer Intelligence disponível, mas bridge `off`;
2. adapter/contratos CI-07 com Noop testado;
3. shadow por process/client/channel;
4. pipeline e prompt bindings candidatos publicados e evals aprovados;
5. canary determinístico;
6. Omnichannel continua revalidando geração/FSM/takeover;
7. somente decisão aceita cria `PENDING` + outbox;
8. outcome event é consumido idempotentemente;
9. expansão por gates;
10. runtime legado congelado atrás de façade.

### 11.2 Falhas

- timeout/erro do Intelligence não derruba chat;
- resultado tardio não causa efeito;
- worker/outbox preserva idempotência;
- n8n nunca envia;
- fallback é policy tipada, não prompt oculto;
- um número/provider possui um sender ativo;
- rollback não reprocessa webhook nem mensagem.

## 12. Equivalência e migração do painel de IA

Antes de redirect/depreciação, a matriz deve provar equivalência de:

- criação, ativação e escopo do agente;
- `systemPrompt` e layers;
- pipeline triage→reply/closure, branches e hard caps;
- provider/model/temperature;
- credenciais write-only;
- debounce/context/turns/confidence;
- handoff on error/limit;
- mídia por papel;
- tools/knowledge;
- simulator;
- versões, publish e rollback;
- permissões, erros, loading e account switch;
- deep links.

Estratégia:

- `ConfigAiAgent.vue` e `ConfigAiAgentVersions.vue` evoluem/reutilizam o núcleo canônico;
- API antiga vira façade para Customer Intelligence após writer cutover;
- Prompt Studio não é duplicado;
- telemetria distingue rota/API/componentes legados;
- banner/link orienta migração sem esconder funções;
- redirect só ocorre quando módulo/permissão de destino estão ativos;
- query/client permitido pode ser preservado; token/PII não;
- não há loop;
- se `/omnichannel/automacao` tiver funções não-IA, não redirecionar a rota inteira.

Remoção visual/API exige equivalência aprovada, zero uso legítimo por janela definida e rollback
ensaiado.

## 13. Compatibilidade de API e depreciação

### 13.1 Fachadas

- endpoints atuais mantêm shape/status/error contract durante janela;
- façade autentica e autoriza novamente;
- façade chama service público novo; não lê tabela privada;
- um único writer efetivo;
- resposta inclui IDs legados apenas como mapeamento estável;
- métricas registram caller/rota/resultado sem PII;
- documentação aponta substituto.

### 13.2 Headers e sunset

Quando aplicável:

```text
Deprecation: true
Sunset: <HTTP-date aprovado>
Link: </docs/...>; rel="deprecation"
```

Sunset só é publicado após:

- substituto disponível e documentado;
- clientes internos migrados;
- telemetria confiável;
- janela aprovada pelo owner;
- suporte/rollback definidos.

Resposta `410 Gone` ou remoção de rota exige pacote posterior e prova de zero uso permitido.

### 13.3 Compatibilidade de dados

- `messaging.contacts` permanece participante operacional;
- `messaging.ai_dispatches` permanece dispatch operacional;
- mensagens, conversas, handoffs, filas, restrições e outbox permanecem Omnichannel;
- `messaging.contact_intelligence` pode virar façade/projeção read-only temporária;
- IDs legados de agent/run/version recebem mapeamento;
- published history e audit não são descartados pela migração.

## 14. Inventário de legado e pré-requisitos de retirada

### 14.1 Nunca retirar por esta iniciativa

Sem nova decisão de ownership, preservar:

- `messaging.contacts`;
- `messaging.conversations`;
- `messaging.messages`;
- `messaging.outbox`;
- `messaging.ai_dispatches`;
- routing decisions, filas e handoffs;
- restrições/supressões;
- close evaluations;
- channel/provider instances e sender;
- media metadata/binary ownership operacional.

### 14.2 Candidatos condicionais

| Alvo legado | Provas adicionais antes de sequer propor remoção |
|---|---|
| `messaging.contact_intelligence` | writer congelado, façade nova, backfill/retenção e zero leitor direto |
| `messaging.collect_field_defs` | field schema/process bindings equivalentes e zero FK/reader |
| `messaging.ai_tool_approvals` | approvals expiradas/migradas, ciphertext tratado por retention e FKs desacopladas |
| `messaging.ai_tool_runs` | histórico migrado/retido e referências de dispatch/run preservadas |
| `messaging.ai_tool_bindings` | tools/bindings novos ativos e runs já desacoplados |
| `messaging.ai_knowledge_bindings` | source/knowledge bindings novos e zero reader |
| `messaging.knowledge_chunks` | documentos migrados/retidos e search novo equivalente |
| `messaging.knowledge_documents` | chunks removidos, provenance preservada |
| `messaging.knowledge_bases` | bindings/documentos removidos e façade sem uso |
| `messaging.ai_runs` | routing/tool/close refs migradas e auditoria/retention preservadas |
| `messaging.ai_agent_versions` | dispatch/media/profile refs migradas, Prompt Registry equivalente |
| `messaging.ai_credentials` | version/media refs migradas e segredo novo validado |
| `messaging.ai_agents` | versions/tools/knowledge/profiles desacoplados e API façade sem uso |
| `messaging.automation_profiles` | binding CI-01 + agent/policy nova equivalentes; auto-close preservado no owner correto |

`messaging.media_analyses` não é automaticamente candidata: antes deve haver decisão explícita
sobre metadata operacional versus análise inteligente. A migration atual contém FKs para
`ai_agent_versions` e `ai_credentials`, que precisam ser substituídas antes de remover os alvos.

### 14.3 Grafo de referências que precisa ser provado

Antes de remover agente/version/run/credential, inventariar no catálogo PostgreSQL e no código:

- `messaging.ai_dispatches.agent_version_id`;
- `messaging.ai_dispatches.result_run_id`;
- `messaging.media_analyses.agent_version_id`;
- `messaging.media_analyses.credential_id`;
- `messaging.routing_decisions.ai_run_id`;
- `messaging.ai_close_evaluations.ai_run_id`;
- `messaging.ai_close_evaluations.automation_profile_id`;
- `messaging.ai_tool_approvals.tool_run_id`;
- `messaging.ai_tool_approvals.binding_id`;
- `messaging.ai_tool_approvals.agent_id`;
- `messaging.ai_tool_runs.ai_run_id`;
- `messaging.ai_tool_runs.dispatch_id`;
- close evaluations e outras migrations posteriores;
- `messaging.automation_profiles.ai_agent_id`;
- `messaging.ai_agent_versions.response_credential_id`;
- todas as referências encontradas via `pg_constraint`, `rg` e telemetria.

Lista documental não substitui inspeção na instância alvo.

## 15. Regras de remoção

Cada remoção:

- possui owner aprovador;
- trata um único alvo lógico;
- tem migration nova, número reservado e sem `Down` autoexecutável;
- ocorre após depreciação e janela de observação;
- registra contagens, FKs, readers/writers e retention;
- tem backup/restore e plano de forward-fix;
- não compartilha pacote com criação, backfill ou cutover;
- atualiza ERD/AGENT/documentação;
- executa testes completos antes/depois;
- nunca usa `CASCADE` para “resolver” dependências desconhecidas.

Uma coluna/FK pode precisar de pacote de transição próprio antes do `DROP TABLE`. A remoção de uma
tabela não autoriza remover outra.

## 16. Rollout hierárquico

Ordem de escopo:

1. ambiente local/teste;
2. conta interna sintética;
3. owner/workspace interno;
4. um client allowlisted;
5. um channel;
6. um process/prompt binding;
7. percentual canary determinístico;
8. expansão por clients/processes;
9. default novo;
10. legado read-only;
11. sunset;
12. remoções.

O bucket é estável por chave aprovada e versionada. Mudar percentual não reatribui de forma
imprevisível. Capability/módulo/permissão/purpose continuam cumulativos.

Portfólio segue rollout separado e não herda promoção do runtime conversacional.

## 17. Rollback por fase

| Fase | Rollback |
|---|---|
| antes de shadow | desliga capability; sem dados de negócio |
| shadow | para candidato; mantém métricas/auditoria |
| canary antes de writer `new` | retorna binding/runtime ao baseline |
| depois de writer `new` | mantém novo writer; façade retorna caminho compatível |
| depois de façade | reabre entrada antiga sobre o mesmo writer |
| após depreciação | cancela sunset e mantém façade |
| após `DROP` | não há rollback simples; restore/forward-fix conforme runbook aprovado |

Regras universais:

- não religar writer legado sem reconciliação reversa;
- não reprocessar webhook/evento deduplicado;
- não reenviar mensagem/campanha;
- preservar IDs/idempotency keys/tombstones;
- drenar ou transferir leases/outboxes com protocolo;
- rollback de prompt reponta binding publicado anterior;
- registrar ator, motivo, escopo e tempo.

## 18. Pacotes atômicos e allowlists

### CI10-OBS-01 — Métricas, dashboards e alertas

**Resultado:** tornar gates observáveis antes de canary.

**Allowlist máxima proposta:**

- `back/internal/modules/customerdata/observability.go` (novo);
- `back/internal/modules/customerdata/observability_test.go` (novo);
- `back/internal/modules/customerintelligence/observability.go` (novo);
- `back/internal/modules/customerintelligence/observability_test.go` (novo);
- `back/internal/modules/omnichannel/intelligence_observability.go` (novo);
- `back/internal/modules/omnichannel/intelligence_observability_test.go` (novo);
- arquivos de dashboard/alerta somente após o despacho identificar os paths reais;
- evidências sob `docs/customer-intelligence/evidence/CI-10/observability/`.

Nenhum pacote inventa label com PII/account de alta cardinalidade sem revisão.

### CI10-LOAD-02 — Carga, planos e caos controlado

**Resultado:** provar capacidade e modo degradado.

**Allowlist máxima:**

- testes `*_test.go` de carga/integração nos módulos Customer Data, Customer Intelligence e
  Omnichannel;
- scripts read-only novos sob `back/scripts/customer-intelligence-load/`, listados nominalmente no
  despacho;
- evidências sanitizadas em `docs/customer-intelligence/evidence/CI-10/load/`.

**Proibido:** produção, dado real, DDL, alteração de índice e stress sem janela.

### CI10-CUT-03 — Execução do runbook de cutover

**Resultado:** promover uma unidade previamente aprovada.

**Allowlist:**

- estado/configuração alterados somente por APIs/comandos administrativos já aprovados;
- evidência em
  `docs/customer-intelligence/evidence/CI-10/cutovers/<account-client-entity-timestamp>/`;
- este documento, somente para registrar resultado aprovado sem reescrever critérios.

Código ou DDL necessários significam que o cutover para e volta à spec predecessora. Este pacote não
“corrige durante a janela”.

### CI10-DEPRECATE-04 — Façades, telemetria e avisos

**Resultado:** marcar uma rota/tela/API como depreciada sem removê-la.

**Allowlist máxima, reduzida por alvo no despacho:**

- façade HTTP específica em `back/internal/modules/omnichannel/`;
- façade/adapters específicos em `back/internal/platform/app/`;
- `web/app/components/omnichannel/config/ConfigAiAgent.vue`;
- `web/app/components/omnichannel/config/ConfigAiAgentVersions.vue`;
- `web/app/components/omnichannel/automation/AutomationAiConfigDrawer.vue`;
- `web/app/pages/omnichannel/automacao.vue`, somente se equivalência de toda a rota for provada;
- documentação/evidência CI-10.

Um pacote não deprecia vários alvos não relacionados. É proibido remover neste pacote.

### CI10-REMOVE-*

**Resultado:** remover exatamente uma tabela, coluna, FK, rota ou componente já aprovado.

Para alvo de banco, allowlist:

- `back/internal/platform/database/migrations/<N_RESERVADO>_<alvo_unico>.sql`;
- `back/database/ERD.md`;
- `back/database/AGENT.md`;
- AGENT do módulo owner;
- evidência específica CI-10.

Para alvo de código/UI, allowlist contém somente o alvo e testes/importadores diretamente
dependentes, todos enumerados no despacho. Não existe wildcard de módulo.

Exemplos de IDs, ainda não autorizados:

- `CI10-REMOVE-CONTACT-INTELLIGENCE`;
- `CI10-REMOVE-COLLECT-FIELD-DEFS`;
- `CI10-REMOVE-AI-TOOL-APPROVALS`;
- `CI10-REMOVE-AI-TOOL-RUNS`;
- `CI10-REMOVE-AI-TOOL-BINDINGS`;
- `CI10-REMOVE-AI-KNOWLEDGE-BINDINGS`;
- `CI10-REMOVE-KNOWLEDGE-CHUNKS`;
- `CI10-REMOVE-KNOWLEDGE-DOCUMENTS`;
- `CI10-REMOVE-KNOWLEDGE-BASES`;
- `CI10-REMOVE-AI-RUNS`;
- `CI10-REMOVE-AI-AGENT-VERSIONS`;
- `CI10-REMOVE-AI-CREDENTIALS`;
- `CI10-REMOVE-AI-AGENTS`;
- `CI10-REMOVE-AUTOMATION-PROFILES`;
- `CI10-REMOVE-LEGACY-AI-ROUTE`;
- `CI10-REMOVE-LEGACY-AI-COMPONENT`.

Ordem é calculada pelo grafo real de dependências; esta lista não é uma ordem de execução.

### CI10-QA-05 — Auditoria independente

**Resultado:** verificar gates e produzir parecer go/no-go.

**Allowlist:**

- testes faltantes diretamente relacionados;
- consultas read-only aprovadas;
- evidências em `docs/customer-intelligence/evidence/CI-10/qa/`;
- parecer final sob `docs/customer-intelligence/evidence/CI-10/GO_NO_GO.md`.

O revisor não implementa correção dentro do pacote QA.

## 19. Áreas sempre proibidas

Sem spec/owner próprio, CI-10 não altera:

- sender/provider adapters;
- workflow n8n;
- módulo legado `automation`;
- dados/módulos ERP, Calendar, Site ou BI;
- arquivos de `socialpublishing`/`social-publishing`;
- migrations aplicadas;
- secrets, `.env`, backups ou volumes;
- qualquer tabela com `CASCADE` não inventariado;
- produção fora da janela/aprovação.

## 20. Testes e comandos

Backend, conforme pacotes realmente implementados:

```text
go test ./internal/modules/customerdata/...
go test ./internal/modules/customerintelligence/...
go test ./internal/modules/omnichannel/...
go test ./internal/platform/app/...
go test -race ./internal/modules/customerdata/...
go test -race ./internal/modules/customerintelligence/...
go test -race ./internal/modules/omnichannel/...
go test ./...
golangci-lint run ./...
```

Frontend:

```text
npm --prefix web run test
npm --prefix web run lint
npm --prefix web run build
```

Banco/integração, somente em ambiente autorizado:

- migration em banco vazio;
- migration em cópia representativa;
- queries de FK/órfão/duplicata;
- dry-run/backfill/resume;
- `EXPLAIN (ANALYZE, BUFFERS)`;
- lock/timeout;
- backup/restore;
- façade old/new;
- outbox/retry/idempotência;
- kill switch/rollback.

Testes ponta a ponta:

- inbound e resposta humana com Intelligence off/down;
- IA ativa produz decisão, Omnichannel cria `PENDING`/outbox e envia uma vez;
- takeover/geração vencida gera zero efeito;
- account/client negativos em todas as APIs/jobs/events;
- draft/test/publish/canary/rollback de cada prompt;
- draft/test/publish/canary/rollback do pipeline e seus branches;
- migração legada cobre os oito componentes com hashes, split revisado e zero publicação automática;
- triage intermediária sem efeito e closure com resposta final preservada;
- equivalência completa de `ConfigAiAgent*`;
- áudio/transcrição e vídeo permanecem visíveis/configuráveis pelo caminho legado até spec própria;
- segmentos rejeitam AST livre, preservam tenant/versão e separam membership de consentimento/export;
- fonte stale/error/disabled;
- consentimento/opt-out/tombstone;
- cross-client/coorte/differencing;
- caos de provider/source/database parcial;
- rollback em cada fase;
- zero caller após janela de depreciação.

## 21. Critérios de aceite e go/no-go

- [ ] todas as predecessoras possuem handoff e blockers resolvidos;
- [ ] writer matrix está completa por unidade;
- [ ] nenhum estado dual é possível;
- [ ] gates absolutos estão em zero;
- [ ] thresholds por processo foram aprovados e atingidos;
- [ ] dashboards/alerts e kill switches foram testados;
- [ ] backfill fecha contagens/checksums e quarentena;
- [ ] chat humano funciona com Intelligence indisponível;
- [ ] sender continua somente no Omnichannel;
- [ ] prompts/bindings/context/model/schema são rastreáveis;
- [ ] Prompt Studio preserva/evolui todo o fluxo atual;
- [ ] mapping dos prompts legados está completo/revisado e estado de import não contamina lifecycle;
- [ ] áudio/transcrição e vídeo atuais continuam acessíveis pelo caminho legado documentado;
- [ ] segmentos, preview, materialização e exportação respeitam AST, tenant e gates aprovados;
- [ ] façade old/new converge ao mesmo writer;
- [ ] uso legado foi medido por janela aprovada;
- [ ] LGPD/retention/delete/tombstone/backups foram testados;
- [ ] cross-client individual permanece desligado;
- [ ] rollback foi ensaiado;
- [ ] remoções estão separadas e cada alvo tem aprovação;
- [ ] não há alteração fora da allowlist.

Go/no-go é assinado pelos owners técnicos, Produto, Operação e Privacidade quando o escopo tocar
PII/portfólio. Aprovação parcial não vale para outro client/process/entity.

## 22. Stop conditions

Parar imediatamente se:

- surgir cross-tenant, PII/segredo em log ou reidentificação;
- dois writers puderem operar;
- IA/n8n/frontend puder enviar diretamente;
- mensagem/outbox/evento puder ficar parcialmente persistido;
- shadow causar efeito;
- gate absoluto falhar;
- threshold obrigatório estiver pendente ou reprovado;
- backfill não fechar contagem/checksum/quarentena;
- houver órfão/FK/reader não inventariado;
- equivalência do painel/API não estiver completa;
- prompt legado tiver path sem mapping/decisão ou split triage/reply sem revisão;
- segmento aceitar SQL/expressão livre ou exportar sem revalidar finalidade/consentimento;
- telemetria de uso legado for ausente/inconfiável;
- rollback não tiver sido ensaiado;
- retenção/legal hold/backups não estiverem resolvidos;
- migration não tiver número reservado após reinspeção;
- pacote misturar criação, backfill, cutover e remoção;
- alguém propuser `CASCADE` para contornar dependência;
- arquivo permitido estiver sujo por outra trilha;
- for necessário tocar `socialpublishing`, workflow ou owner externo;
- qualquer teste de tenant, idempotência, sender ou restauração falhar.

Em stop condition, manter capability segura (`off`, `shadow` ou baseline), preservar evidências e
abrir correção na spec owner. Não improvisar durante janela.

## 23. Handoff obrigatório

Cada pacote/onda registra:

- escopo exato: account/client/entity/channel/process/binding;
- owners e aprovações;
- baseline de código, migrations e configuração;
- writer states antes/depois;
- contagens, watermark, checksums, órfãos e quarentena;
- métricas/evals/thresholds e duração observada;
- prompt/binding/model/schema/source/tool refs;
- arquivos/configurações alterados;
- testes e comandos com resultados;
- dashboards/alerts/incidentes;
- façade/rota/API e uso legado;
- rollback executado ou prova do ensaio;
- LGPD/retention/tombstone/backup;
- confirmação de sender exclusivo no Omnichannel;
- confirmação de zero alteração fora da allowlist;
- decisão go/no-go e próxima ação.
