# Customer Intelligence — especificações gerais

- **Status:** DRAFT
- **Versão:** 0.2
- **Data-base:** 2026-07-23
- **Governança:** [GOVERNANCA.md](GOVERNANCA.md)

> Este documento é o blueprint para gerar todas as specs executáveis numa única rodada de
> especificação. Gerar tudo de uma vez não significa implementar tudo de uma vez: execução,
> revisão e cutover permanecem atômicos.

## 1. Resultado da futura rodada única

Quando o owner solicitar “gere todas as specs de Customer Intelligence”, a rodada deverá:

1. reler o estado real do repositório e as instruções aplicáveis;
2. reconciliar a governança com código, migrations, APIs, front e workflows existentes;
3. resolver ou marcar explicitamente todas as decisões pendentes;
4. criar specs campo a campo, sem implementar código;
5. criar matriz de dependências e paralelismo;
6. criar allowlist de escrita por pacote;
7. criar contrato comum de execução;
8. criar checklist de revisão independente;
9. registrar divergências entre plano e disco;
10. entregar um índice completo para validação do owner.

Saídas esperadas:

```text
docs/customer-intelligence/specs/
  README.md
  CONTRATO_EXECUCAO_AGENTES.md
  MATRIZ_DEPENDENCIAS.md
  CATALOGO_DESPACHO_AGENTES.md
  CHECKLIST_REVISAO.md
  CI-00_GOVERNANCA_CONTRATOS.md
  CI-01_BINDING_CANAL_CLIENTE.md
  CI-02_IDENTIDADE_RELACIONAMENTOS.md
  CI-03_CUSTOMER_DATA.md
  CI-04_INTELLIGENCE_BANK.md
  CI-05_FONTES_CONECTORES.md
  CI-06_RUNTIME_IA.md
  CI-07_INTEGRACAO_OMNICHANNEL.md
  CI-08_FRONTEND_UX.md
  CI-09_RECOMENDACOES_PORTFOLIO.md
  CI-10_HARDENING_CUTOVER.md
```

Arquivos ainda não são criados nesta versão geral para evitar congelar contratos campo a campo
antes da validação das decisões `CI-DEC-*`.

### 1.1 Disparo canônico da rodada única

O owner pode usar:

> Leia `GOVERNANCA.md` e `SPECS_GERAIS.md`, reconcilie-os com o estado atual do repositório e gere
> todas as specs CI-00 a CI-10, contrato de execução, matriz de dependências, catálogo de pacotes e
> checklist de revisão. Não implemente código, migration, workflow ou deploy. Não esconda decisões
> abertas: aplique o default recomendado como proposta `DRAFT` ou marque a spec dependente como
> `BLOCKED`. Entregue um único índice para minha validação.

Essa rodada não deve interromper a geração para pedir decisões uma a uma; todas as escolhas abertas
e recomendações ficam consolidadas no índice final para validação conjunta.

## 2. Ordem obrigatória de leitura para gerar as specs

1. `AGENT.md`;
2. skills `principios-engenharia` e `omnichannel-hibrido`;
3. `docs/customer-intelligence/GOVERNANCA.md`;
4. `docs/omnichannel/PLANO_TECNICO_EVOLUCAO.md`;
5. `docs/omnichannel/ARQUITETURA_HIBRIDA_N8N.md`;
6. `docs/omnichannel/ESTADO.md`;
7. `docs/omnichannel/evolucao/CONTRATO_EXECUCAO_AGENTES.md`;
8. `back/internal/modules/omnichannel/AGENT.md`;
9. `back/internal/modules/crm/AGENT.md` e `crm/erp/AGENT.md`;
10. `back/internal/modules/calendar/AGENT.md`;
11. `back/internal/modules/site/AGENT.md`;
12. `back/internal/platform/events/AGENT.md` e `platform/modules/AGENT.md`;
13. `web/AGENT.md` e AGENTs locais do Omnichannel/CRM;
14. `automation/AGENT.md`, apenas para preservar ownership.

O disco vence qualquer inventário desatualizado. Divergência não pode ser resolvida por suposição.

## 3. Invariantes comuns a todas as specs

- PostgreSQL é fonte de verdade.
- Rotas autenticadas derivam escopo do Principal; webhooks públicos e gateways internos usam
  resolução server-side e autenticação própria, nunca `account_id` confiado do body.
- Sem SQL cross-module.
- Sem dual-write permanente.
- Sem exclusão em pacote de criação/cutover.
- Chat funciona sem Customer Intelligence.
- Customer Intelligence funciona sem painel ou conversa ativa.
- A IA produz a resposta; o Omnichannel executa o envio.
- Toda resposta automática passa por validação Go, mensagem `PENDING`, outbox e adapter.
- n8n não envia, não grava PostgreSQL e não mantém memória autoritativa.
- Tool/fonte é registrada e allowlisted; modelo não escolhe SQL/URL/segredo livre.
- Fato LLM não vence dado verificado.
- Contexto é filtrado por cliente e relacionamento.
- Cross-client individual permanece desabilitado até policy explícita.
- Todo evento repetível tem idempotency key.
- Toda fase inclui teste negativo de tenant.
- Toda rota/página autenticada nova ou alterada tem gate de módulo/permissão. Webhooks e gateways
  internos têm assinatura/token, rate limit e escopo equivalente, sem gate de UI.
- APIs antigas e novas nunca escrevem em fontes diferentes.
- Nenhum workflow de Automation, Calendar, Operação ou outro owner pode ser alterado.

## 4. Mapa das especificações

### CI-00 — Governança, vocabulário e contratos

**Objetivo:** transformar as decisões propostas em contratos aprovados.

Entregas:

- ADR de separação dos bounded contexts;
- IDs definitivos dos módulos, schemas e rotas;
- decisão `RequiresModules`/`OptionalModules` e modo degradado por conta;
- decisão de `account_id` físico versus alias de domínio `owner_account_id`;
- topologia `owner→subject` e `relationship(owner, client, subject)`, sem inventar FK pai→filho;
- invariantes para agência, cliente, mesma organização e conta standalone;
- glossário account/client/subject/relationship;
- ownership de dados e workflows;
- envelopes versionados `ContextRequest`, `ContextEnvelope` e `InteractionDecision`;
- catálogo inicial de eventos;
- permissões e feature flags;
- registro das decisões `CI-DEC-*`.

Aceite:

- nenhuma palavra “cliente” permanece ambígua nos contratos;
- chat, CRM determinístico e inteligência possuem owners distintos;
- decisão de envio está representada como proposta IA + execução Omnichannel;
- decisões não resolvidas bloqueiam apenas as specs dependentes.

Permissões a congelar, sem herdar acesso cross-client de uma chave antiga:

- `customer_data.subjects.view/manage`, merge e consentimentos;
- `customer_intelligence.profile.view`;
- `customer_intelligence.sources.view/manage`;
- `customer_intelligence.agents.manage`;
- `customer_intelligence.audit.view`;
- `customer_intelligence.portfolio.view/manage`.

O mapeamento temporário de `omnichannel.agents.manage` e `omnichannel.audit.view` não pode conceder
automaticamente fontes, portfólio ou acesso cross-client.

### CI-01 — Binding canal ↔ cliente

**Objetivo:** tornar o cliente proprietário de uma conversa independente do perfil de IA.

Entregas:

- modelo histórico de binding instância/canal↔`client_account_id`;
- migration aditiva e índices;
- backfill a partir de `automation_profiles`;
- relatório de órfãos, duplicidades e ambiguidades;
- conversa/touchpoint com escopo histórico explícito;
- outbox local na transação de inbound; resolução de subject/relationship fora da transação;
- mudança de `AutomationClientForInstance` e `AutomationConversationScope`;
- testes com perfil de IA inexistente ou desabilitado.

Aceite:

- ativar/desativar IA não muda ownership da conversa;
- troca futura de instância não reatribui histórico;
- chat humano funciona sem `automation_profile`;
- conta/cliente incompatível retorna 404;
- backfill não escolhe vínculo ambíguo silenciosamente.

### CI-02 — Identidade, matching e relacionamentos

**Objetivo:** separar pessoa deduplicável de sua relação com cada cliente.

Entregas de contrato, sem criar um segundo writer:

- schema lógico de `subject`, `relationship`, identities, external refs e candidatos de match;
- `subject_type` e atributos próprios para pessoa/empresa;
- regras fortes/fracas de matching;
- revisão, merge e undo auditáveis;
- tratamento de contatos já misturados;
- escopo de consentimento, preferência e lifecycle por relacionamento;
- fixtures e cenários de aceitação para a implementação da CI-03.

Aceite:

- fixtures cobrem mesma pessoa com duas relações isoladas;
- contrato proíbe merge por nome/fuzzy match;
- chaves fortes incluem cliente/conector quando necessário;
- matriz de permissão impede relação A em contexto B;
- match forte em cliente distinto gera candidato restrito, não vínculo automático;
- sem DDL ou writer concorrente antes da CI-03.

### CI-03 — Customer Data

**Objetivo:** criar a fronteira determinística consumida por canais e inteligência.

Entregas:

- módulo/package, `AGENT.md`, metadata, permissões e wiring;
- decisão implementada entre módulo próprio e `crm/customerdata`;
- DDL e writer únicos de subjects, relações, identidades, notas e consentimentos;
- services/repositories para subjects, relações, notas e consentimentos;
- APIs `/v1/customer-data/*`;
- projeção determinística de Customer Data;
- compatibilidade para endpoints atuais de contato/CRM;
- plano de migração das tabelas `messaging.*` sem big-bang.

Aceite:

- existe um único escritor autoritativo por entidade;
- campos determinísticos da fachada antiga e da API Customer Data são equivalentes; a fachada
  composta antiga pode acrescentar Intelligence sem fingir que ela pertence a Customer Data;
- paginação e índices atendem hot paths;
- módulo pode ser habilitado/desabilitado sem quebrar recebimento de canal;
- front reidrata pela resposta autoritativa.

### CI-04 — Intelligence Bank

**Objetivo:** persistir evidências e inteligência derivada com proveniência.

Entregas:

- source configs, observations, claims, facts e fact evidence;
- summary versions, recommendations e context snapshots;
- ingestion runs, source suggestions e audit events;
- precedência, conflito, supersede e revisão manual;
- projeção compatível com `messaging.contact_intelligence`;
- retenção e classificação de sensibilidade;
- tombstone, anonimização/crypto-shredding, legal hold, backups e bloqueio de reingestão;
- criptografia/HMAC para identificadores e proteção/TTL de context snapshots;

Aceite:

- cada fato mostra origem, instante, confiança, método e estado;
- resumo aponta exatamente as evidências usadas;
- conflito não apaga valor anterior;
- correção manual verificada prevalece;
- authority policy varia por tipo de fato, fonte, frescor e validade;
- reprocessar a mesma origem não duplica observação;
- `contact_intelligence` não permanece como segunda verdade após cutover.

### CI-05 — Fontes e conectores

**Objetivo:** alimentar o banco por adapters tipados, configuráveis e auditáveis.

Ordem:

1. Omnichannel;
2. manual/offline;
3. ERP;
4. Calendário;
5. Site;
6. BI on-demand, começando pela conexão/dataset configurado da Pérola.

Entregas:

- registry de `source_key`;
- portas separadas para evidência do subject, contexto empresarial, agregado de portfólio e tool
  de escrita;
- configuração por cliente, finalidade, campos, retenção e prioridade;
- health, cursor, sync, retry e dead-letter;
- adapters no composition root;
- outbox/evento durável para ingestão;
- unificação dos fluxos de lead Site/Omnichannel;
- source suggestions aguardando aprovação;
- semântica de source disable: ingestão, uso de evidência histórica, invalidação e retenção;

Aceite:

- falha de conector não bloqueia o chat;
- segredo não aparece em banco de config aberto, resposta, reidratação do front, log ou workflow;
- nenhuma fonte aceita SQL/URL/tabela livre do modelo;
- ERP genérico/CPF inválido não faz match automático;
- tracking de sessão não vira pessoa sem conversão;
- desabilitar fonte interrompe novas leituras;
- fonte desabilitada deixa explícito se evidência histórica pode compor contexto;
- dado ausente ou stale aparece no resultado e não vira fallback silencioso.

### CI-06 — Runtime de IA headless

**Objetivo:** executar contexto, modelo, tools e aprendizado fora do ownership do chat.

Entregas:

- módulo/package Customer Intelligence;
- agentes, versões, prompts, modelos, credenciais, tools e knowledge;
- IDs/referências de compatibilidade para consumidores operacionais ainda ligados às tabelas
  `messaging.ai_*`;
- context builder determinístico, paginado e token-budgeted;
- execução da decisão inteligente, runs, custo, timeout e cancelamento cooperativo;
- saída versionada com resposta, handoff e memória candidata;
- execução nativa/n8n sob o mesmo contrato;
- API/job que opera sem frontend.

Aceite:

- perfil pode ser criado a partir de ERP/manual sem inbox;
- modelo recebe apenas contexto autorizado;
- resultado atrasado/cancelado não produz efeito operacional;
- tool inválida ou insegura é rejeitada;
- provider/modelo indisponível gera fallback auditado;
- prompt/chave brutos não aparecem em logs ou execução persistida do n8n.

Credencial digitada por um administrador pode existir transitoriamente no input e request TLS.
Ela nunca retorna da API, reidrata no formulário, persiste em store/local storage, aparece em
log/telemetria ou permanece no campo após salvar.

O dispatch durável de uma conversa (`messaging.ai_dispatches`), sua lease e o cancelamento por
takeover continuam no Omnichannel. Esta spec possui a execução da decisão inteligente e os jobs
headless de ingestão, não uma segunda fila autoritativa da conversa.

### CI-07 — Integração Omnichannel ↔ Customer Intelligence

**Objetivo:** desacoplar runtime inteligente preservando FSM, handoff e envio.

Entregas:

- dispatcher opcional no `InboundService`;
- separação de `CommitAITriageWithIntelligence`;
- outbox de aprendizado inserida na mesma transação que aceita `state + ai_generation`;
- portas para contexto, decisão, handoff e resposta;
- preservação de `messaging.ai_dispatches`, `ai_generation` e `ai_close_evaluations` no Omnichannel;
- migration aditiva, backfill e troca de referências antes de desacoplar `ai_runs`,
  `ai_agent_versions`, `ai_credentials` ou `automation_profiles`;
- shadow mode por cliente/canal;
- compatibilidade de `ai_dispatches`, runs e URLs atuais;
- testes de generation lease, humano concorrente e outbox.

Aceite:

- Omnichannel inicia e opera sem módulo de Inteligência;
- IA ativa produz um único efeito interno idempotente de mensagem/outbox; resposta ambígua do
  provider é reconciliada e eventual duplicata externa é detectada;
- humano assumindo invalida resultado atrasado;
- mensagem automática prova `decision → PENDING → outbox → provider`;
- nenhuma resposta é enviada pelo n8n;
- rollback troca contexto/runtime, nunca o sender.

### CI-08 — Frontend e experiência

**Objetivo:** retirar o perfil completo e a configuração inteligente do inbox.

Entregas:

- workspace `/inteligencia-clientes`;
- rotas de detalhe, fontes, atendimentos, auditoria e portfólio sob
  `/inteligencia-clientes/*`;
- lista, perfil, timeline, fatos, evidências, recomendações e auditoria;
- página de fontes/conectores;
- atendimentos/intervenções e configuração de agentes;
- cartão compacto no inbox;
- split de `AutomationAiConfigDrawer`;
- domain/composables/store próprios;
- seams substitutos para `useOmnichannelCRM`, `useOmnichannelGlobalAI`,
  `OmnichannelCRMProfilePanel` e o drawer misto;
- gates e testes em `workspaces.ts`, `permissions.ts`, `nav.config.ts` e
  `module-enabled.global.ts`;
- lazy-load por aba e gates de permissão;
- compatibilidade/redirect autorizado de `/omnichannel/automacao`, preservando conta/query e sem
  loop quando o módulo destino não estiver disponível.

Aceite:

- inbox permanece utilizável sem Customer Intelligence;
- perfil completo possui uma única casa;
- após cutover, o inbox não importa tipos/composables do perfil antigo;
- cada fato mostra origem/confiança/estado;
- conta trocada limpa e reidrata todo estado;
- módulo desabilitado não dispara fetch de inteligência;
- `/crm`, `/automation` e `/inteligencia` existentes não sofrem regressão;
- chave digitada não retorna, reidrata, persiste localmente ou aparece em telemetria;
- loading, vazio, erro, stale, mobile, tema, troca de conta e papéis distintos são inspecionados
  no browser.

### CI-09 — Recomendações, marketing e portfólio

**Objetivo:** transformar inteligência em ações úteis sem misturar fatos e recomendações.

Entregas:

- follow-up e cadência;
- produtos/serviços sugeridos com referência de catálogo;
- datas importantes;
- next best action;
- feedback de aceite/rejeição/resultado;
- segmentos e oportunidades de portfólio;
- política de uso cross-client;
- aprovação explícita para qualquer ativação individual.

Aceite:

- recomendação possui racional, evidências, validade e expiração;
- recomendação não altera fato histórico;
- portfólio é agregado/anônimo por padrão;
- coorte mínima e supressão contra reidentificação são aplicadas;
- nenhum contato individual atravessa cliente sem gate, finalidade e auditoria;
- controlador, operadores, aprovador, base legal e revogação estão documentados;
- opt-out bloqueia ação incompatível;
- resultado real alimenta avaliação da recomendação.

### CI-10 — Hardening, cutover, rollback e retirada

**Objetivo:** liberar gradualmente e retirar legado somente com prova.

Entregas:

- métricas quantitativas do shadow;
- policy do shadow com PII, fornecedor/modelo, finalidade, retenção e custo;
- dashboards de backlog, custo, latência, duplicata, handoff e vazamento;
- testes de carga e queries/índices;
- plano LGPD de retenção, retificação, anonimização e exclusão;
- rollout por workspace/cliente/canal;
- state machine do writer por cliente/entidade, watermark e checksum do backfill;
- rollback ensaiado;
- métricas de uso de APIs antigas;
- headers `Deprecation`/`Sunset`;
- prova de que FKs de dispatch, fechamento, handoff, routing e media analysis já não apontam para
  tabelas candidatas a drop;
- smoke de texto, mídia, áudio, reply, `fromMe`, duplicata, falha do provider, atraso e handoff;
- pacotes separados de remoção.

Aceite:

- zero vazamento cross-tenant nos testes e no shadow;
- um único efeito interno de mensagem/outbox; duplicidade/ambiguidade do provider é detectada,
  reconciliada e observável;
- rollback não reprocessa eventos deduplicados;
- escritor antigo e novo nunca ficam ativos juntos;
- compatibilidade atinge tráfego zero antes da remoção;
- owner aprova cada exclusão material.

## 5. Dependências e paralelismo

```text
CI-00
  -> CI-01
      -> CI-02
          -> CI-03
          -> CI-04
          -> CI-05 (adapters podem avançar em paralelo após contratos)
          -> CI-08 (shell e fixtures podem avançar em paralelo)

CI-04 + CI-05
  -> CI-06

CI-01 + CI-06
  -> CI-07

CI-04 + CI-06
  -> CI-09

CI-03..CI-09
  -> CI-10
```

Trilhas após CI-00/CI-01:

| Trilha | Escopo |
|---|---|
| A — Omnichannel | binding, dispatcher opcional, FSM/outbox e compatibilidade |
| B — Customer Data | identidade, relações, consentimentos e APIs |
| C — Intelligence | evidências, contexto, runtime e recomendações |
| D — Front/Connectors | workspace, adapters, fontes e observabilidade |

Contratos e fixtures são congelados antes de permitir implementação paralela. Integração e revisão
final pertencem ao orquestrador, não aos executores isolados.

## 6. Pacotes atômicos mínimos por spec

Cada spec detalhada deve separar, quando aplicável:

- `DOC/ADR`: decisão e contrato;
- `DB`: DDL aditivo;
- `BACKFILL`: processamento e relatório, separado de DDL pesado;
- `BE-DOMAIN`: modelos, policies e services;
- `BE-STORE`: repositories e SQL;
- `API`: handlers, DTOs e permissions;
- `ADAPTER`: integração cross-module;
- `JOB`: outbox, worker, retry e idempotência;
- `FE-DOMAIN`: tipos/API/store;
- `FE-UI`: página/componentes;
- `N8N`: somente workflow do owner correto;
- `QA`: revisão/teste independente;
- `CUTOVER`: ativação/rollback;
- `REMOVE`: retirada separada e explicitamente aprovada.

Um pacote não deve misturar criação e exclusão. Migration e backfill pesado não ficam no mesmo
executor. Workflow n8n e policy Go também não ficam no mesmo executor.

## 7. Conteúdo obrigatório de cada spec detalhada

1. resultado único e verificável;
2. status e owner;
3. decisões já tomadas;
4. dependências e bloqueados;
5. inventário atual medido no disco;
6. arquivos que pode ler;
7. allowlist exata de escrita;
8. arquivos proibidos;
9. contratos de entrada/saída campo a campo;
10. tabelas, índices, constraints e estratégia de backfill;
11. fluxos de sucesso, duplicata, falha e concorrência;
12. tenant/permissões;
13. observabilidade e evidência de auditoria;
14. compatibilidade e deprecação;
15. rollout e rollback;
16. testes/comandos exatos;
17. critérios de aceite;
18. stop conditions;
19. handoff obrigatório.

## 8. Critérios globais de aceite

- [ ] Chat recebe, envia, transfere e encerra com Inteligência ausente.
- [ ] Inteligência forma perfil com ERP/manual sem chat ou painel.
- [ ] IA ativa responde automaticamente pelo caminho Go/outbox.
- [ ] Nenhum workflow chama provider de canal.
- [ ] Toda informação derivada possui fonte, data, confiança, método e estado.
- [ ] Correção manual verificada não é sobrescrita por LLM.
- [ ] Conflitos permanecem auditáveis.
- [ ] Repetir evento não duplica observação, fato, dispatch ou mensagem.
- [ ] Escopo negativo retorna 404 e não confirma existência.
- [ ] Uma pessoa pode ter relações isoladas com clientes distintos.
- [ ] Cross-client é agregado/anônimo por padrão.
- [ ] Painel não é dependência do runtime.
- [ ] Chave pode transitar no request de cadastro, mas nunca retorna, reidrata ou persiste no front.
- [ ] Contexto respeita orçamento, paginação e fontes habilitadas.
- [ ] APIs legadas e novas não criam duas fontes de verdade.
- [ ] Cutover e rollback são demonstrados antes de remoção.

## 9. Decisões que bloqueiam a geração final

A rodada completa pode produzir specs em `DRAFT`, mas não pode marcá-las `READY` enquanto não
forem validadas:

- IDs definitivos `customer_data` e `customer_intelligence`;
- decisão entre módulo `customer_data` próprio ou subdomínio `crm/customerdata`;
- escopo definitivo de `subject_id`;
- owner definitivo de subject/relationship;
- precedência de fontes;
- retenção por categoria;
- categorias permitidas em inteligência cross-client;
- mecanismo concreto de outbox/evento de ingestão;
- métricas mínimas para shadow→produção;
- estratégia de contatos historicamente misturados;
- semântica completa de source disable e exclusão de derivados;
- política de privacidade/LGPD e papéis aplicáveis;
- criptografia/HMAC, context snapshot e tratamento de backups;
- regra de matching cross-client e conta standalone;
- coorte mínima/supressão do portfólio.

O default recomendado está em `GOVERNANCA.md` §17. Qualquer divergência decidida pelo owner deve
ser registrada no histórico antes de gerar specs `READY`.

## 10. Checklist de validação do owner

Ao receber todas as specs numa rodada, o owner deve conseguir validar em um único índice:

- [ ] nomes e limites dos módulos;
- [ ] quem é agência, cliente e pessoa;
- [ ] fontes iniciais e prioridade;
- [ ] telas e rotas;
- [ ] comportamento automático da IA;
- [ ] fallback humano;
- [ ] dados que podem ou não cruzar clientes;
- [ ] retenção e consentimento;
- [ ] ordem de implementação;
- [ ] funcionalidades preservadas;
- [ ] itens que serão movidos;
- [ ] itens criados;
- [ ] itens candidatos a exclusão;
- [ ] custos/riscos operacionais;
- [ ] rollout e rollback.

## 11. Histórico

| Versão | Data | Alteração | Aprovação |
|---|---|---|---|
| 0.1 | 2026-07-23 | blueprint inicial das specs CI-00 a CI-10 | pendente |
| 0.2 | 2026-07-23 | revisão de dependências, ownership, privacidade, cutover, APIs e UX | pendente |
