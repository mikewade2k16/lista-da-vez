# CI-07 — Integração Customer Intelligence ↔ Omnichannel

- **Status:** READY — implementação local/canary-off autorizada
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Plataforma/Integrações
- **Owners participantes:** Omnichannel e Customer Intelligence
- **Depende de:** CI-00, CI-01, CI-03 e CI-06
- **Desbloqueia:** CI-10
- **Autoriza implementação:** sim; sender e cutover continuam no Omnichannel

> Esta spec descreve a integração futura. Enquanto estiver em `DRAFT`, não autoriza código,
> migration, workflow, backfill, alteração de sender, cutover ou remoção de legado.

## 1. Resultado único e verificável

Entregar uma fronteira versionada na qual:

1. Omnichannel continua recebendo, persistindo e enviando mensagens mesmo sem Customer
   Intelligence;
2. Customer Intelligence recebe contexto autorizado e produz uma decisão estruturada;
3. a resposta produzida pela IA só vira mensagem de canal depois de nova validação do Omnichannel;
4. mensagem aceita é persistida como `PENDING` junto do outbox transacional do canal;
5. o resultado operacional aceito gera, na mesma transação autoritativa, um evento durável para
   aprendizado posterior;
6. toda execução registra os IDs das versões de prompt, binding, contexto, modelo e rollout;
7. modo shadow compara decisões sem alterar FSM, fila, handoff, mensagem ou sender;
8. nenhuma etapa depende do painel, do browser ou de envio direto pelo n8n.

O resultado é verificável por testes de contrato, concorrência, idempotência e integração que
provem uma única mensagem/outbox por decisão válida e nenhuma mensagem para decisão vencida,
duplicada, rejeitada ou executada em shadow.

## 2. Requisitos locais derivados da governança

Os IDs abaixo são requisitos rastreáveis desta spec, não uma sequência concorrente de decisões.
Decisões de arquitetura continuam sendo registradas exclusivamente como `CI-DEC-*` na seção 17 de
`GOVERNANCA.md`.

| ID | Requisito derivado |
|---|---|
| CI07-REQ-001 | Omnichannel é o único owner de webhook, conversa, FSM, lease, `ai_generation`, mensagem, outbox do canal, adapter e sender |
| CI07-REQ-002 | Customer Intelligence é o único owner de contexto inteligente, prompts, agentes, ferramentas, avaliações, fatos candidatos e recomendações |
| CI07-REQ-003 | “A IA responde” significa que ela produz `InteractionDecision`; não significa acesso ao provider |
| CI07-REQ-004 | a interface consumida pelo Omnichannel pertence ao package Omnichannel; o adapter concreto é montado no composition root |
| CI07-REQ-005 | o binding e as versões efetivas de prompt são resolvidos pelo Customer Intelligence, não pelo Omnichannel |
| CI07-REQ-006 | resultado aceito e evento de aprendizado nascem na mesma transação do estado operacional aceito |
| CI07-REQ-007 | evento crítico usa outbox durável e consumidor idempotente; o bus in-process não é suficiente |
| CI07-REQ-008 | n8n pode orquestrar uma execução autorizada, mas não grava PostgreSQL nem chama WhatsApp/Instagram |
| CI07-REQ-009 | ausência, desabilitação ou falha do Customer Intelligence degrada somente automação inteligente; chat humano permanece funcional |
| CI07-REQ-010 | shadow nunca cria mensagem, outbox de canal, handoff, fechamento, mudança de fila ou mutação de perfil |

Estes requisitos herdam os invariantes e o vocabulário da CI-00. Qualquer conflito deve parar o
pacote; não deve ser resolvido por adaptação silenciosa.

## 3. Dependências, bloqueios e pré-condições

| Dependência | Entrega necessária |
|---|---|
| CI-00 | IDs de módulos, permissões, `process_key`, envelopes e capability flags aprovados |
| CI-01 | vínculo determinístico de participante, subject e relationship com escopo validado |
| CI-03 | Customer Data e projeções mínimas disponíveis por interfaces tipadas |
| CI-06 | runtime de contexto, Prompt Registry, agentes, tools e execução headless estáveis |
| Omnichannel | contrato atual de lease, FSM, dispatch, mensagem `PENDING` e outbox preservado |

Bloqueios para sair de `DRAFT`:

- contrato final de `InteractionRequest` e `InteractionDecision` aprovado pelos dois owners;
- `ProcessResult.v1`, pipeline `conversation.respond` e composição final de CI-00 aprovados;
- compatibilidade field-by-field de `brain.result.v3`/auto-close provada, preservando final reply,
  closure proposal e todos os gates;
- tabela/contrato do outbox de integração definido sem reutilizar indevidamente o outbox de canal;
- política de retry, timeout, orçamento e fallback aprovada por processo;
- estratégia de compatibilidade de `messaging.ai_dispatches.agent_version_id` definida;
- números de migration reservados somente no despacho de cada pacote;
- métricas mínimas de shadow e promoção aprovadas na CI-10;
- confirmação de que CI-06 persiste versões publicadas de prompt de forma imutável.

## 4. Inventário real que a implementação deverá respeitar

Na data-base desta spec:

- `InboundService` ainda recebe implementações concretas de IA e dispara `maybeAutoTriage`;
- `messaging.ai_dispatches` já oferece dispatch durável, geração, retry e idempotency key;
- o handler de dispatch chama o serviço de IA e depois aplica reply, handoff, fechamento ou retry;
- `CreateAIOutboundMessage` já revalida conversa/geração e grava `messaging.messages` com
  `direction=OUTBOUND`, `status=PENDING`, `origin=ai` e `messaging.outbox` na mesma transação;
- `CommitAITriageWithIntelligence` ainda mistura atualização operacional com o snapshot legado
  `messaging.contact_intelligence`;
- os tipos Go `BrainRequestV2`/`BrainResultV2` aceitam os wire schemas
  `brain.request.v2|v3` e `brain.result.v2|v3`;
- o executor valida resultado e o Go mantém FSM, restrições e envio;
- o n8n não pode se tornar sender nem autoridade de banco;
- o bus em `platform/events` é síncrono/in-memory e não atende entrega crítica.

Este inventário não autoriza ampliar o package Omnichannel com novas responsabilidades. A
integração deve extrair o acoplamento, não apenas renomeá-lo.

## 5. Fronteiras e fluxo alvo

```text
webhook/provider
      |
      v
Omnichannel: dedupe -> mensagem inbound -> dispatch durável
      |
      | InteractionRequest.v1 (porta tipada)
      v
Customer Intelligence: pipeline -> contextos -> prompts separados -> decisão composta
      |
      | InteractionDecision.v1 (proposta)
      v
Omnichannel: lock -> tenant -> lease/generation -> policy/FSM -> aceitar/rejeitar
      |
      +--> mensagem PENDING + outbox do canal --> worker --> adapter/provider
      |
      +--> outbox de integração --> consumidor idempotente --> banco inteligente
```

Regras:

- o retorno da IA nunca é uma confirmação de envio;
- apenas o commit do Omnichannel pode marcar a decisão como aceita;
- o provider só é chamado pelo worker do outbox de canal;
- o consumidor do evento inteligente só lê projeções autorizadas por porta pública;
- payloads duráveis levam IDs e códigos estáveis, não dumps de conversa ou prompts;
- Customer Intelligence pode funcionar headless com entradas de ERP, calendário, offline ou
  outras fontes mesmo sem inbox;
- Omnichannel pode funcionar com IA `off`, módulo ausente ou adapter indisponível.

## 6. Porta consumida pelo Omnichannel

A interface deve ser pequena, declarada pelo consumidor e não importar tipos internos de Customer
Intelligence:

```go
type IntelligenceDispatcher interface {
    Decide(ctx context.Context, input InteractionRequest) (InteractionDecision, error)
}
```

Regras de construção:

- `nil` ou implementação `Noop` representa capability ausente/desabilitada e não impede boot;
- o adapter concreto fica no composition root e traduz para a API pública de Customer Intelligence;
- nenhum repository de um módulo é injetado diretamente no outro;
- nenhum módulo consulta tabelas do outro por SQL;
- erros devem ser tipados: `disabled`, `not_authorized`, `invalid_input`, `timeout`,
  `temporarily_unavailable`, `invalid_result`, `budget_exceeded` e `permanent_failure`;
- o caller decide retry/handoff conforme policy estruturada; texto de prompt não decide retry;
- deadline e cancelamento do contexto devem ser respeitados por adapter, executor e tools.

## 7. `InteractionRequest.v1`

CI-00 §9.3 é a única definição canônica. CI-07 não acrescenta campos ao mesmo nome/versão. O
adapter materializa exatamente:

| Campo canônico | Tipo/regra nesta integração |
|---|---|
| `schemaVersion` | literal `interaction.request.v1` |
| `requestId` | idempotência/correlação opaca |
| `interactionId` | referência lógica da interação; no adapter Omnichannel mapeia de forma estável ao dispatch |
| IDs de escopo | owner/client/subject/relationship/conversation validados server-side |
| `pipelineKey` | literal inicial `conversation.respond`; não é prompt/process key |
| `aiGeneration` | geração capturada, revalidada na aceitação |
| `message` | mensagem agregada/minimizada no schema congelado por CI-00/CI-06 |
| `operationalState` | projeção allowlisted de estado, lease, restrições e campos |
| `routingCatalog` | filas/setores permitidos, somente para sugestão |
| `channelCapabilities` | capacidades/limites reais informados pelo Omnichannel |
| `purpose`, `locale`, `asOf`, `sourceKeys`, budgets | contexto autorizado e bounded |
| `deadlineAt`, `correlationId` | deadline e tracing server-side |

`dispatchId`, idempotency key, runtime mode, force command e tentativa continuam no registro
operacional `messaging.ai_dispatches`/policy do Omnichannel. Não são campos novos de
`InteractionRequest.v1`; `interactionId` é a correlação canônica.

O request aciona uma versão publicada de pipeline. Customer Intelligence cria internamente um
`ContextRequest.v1`/`ProcessResult.v1` por processo, mantendo run, binding, schema e auditoria
separados. O Omnichannel não monta prompt, não escolhe branch e não recebe triage intermediária
como decisão final.

Não entram no request senha, chave, token, SQL, URL livre, sender, provider command, prompt montado
no Omnichannel, outro cliente não autorizado ou payload bruto ilimitado.

## 8. `InteractionDecision.v1`

CI-00 §9.5 é a definição única. O adapter rejeita campos desconhecidos e aplica este contrato sem
alias estrutural:

| Campo canônico | Tipo | Regra |
|---|---|---|
| `schemaVersion` | literal `interaction.decision.v1` | obrigatório |
| `requestId`, `decisionId` | strings opacas | correlação/idempotência |
| IDs de escopo | UUIDs | iguais ao request autorizado |
| `pipelineKey`, `pipelineVersionId` | referências | iguais ao entrypoint/binding resolvidos |
| `processRunRefs` | lista não vazia | run/process/binding/versions/schema de cada etapa |
| `aiGeneration` | bigint | eco exato; divergência rejeita tudo |
| `outcome` | enum | `reply`, `handoff` ou `no_reply` |
| `replyDraft` | string nullable | proposta; tamanho/conteúdo revalidados |
| `needsHuman` | boolean | coerente com outcome |
| `reasonCode` | chave allowlisted | sem comando/texto operacional livre |
| `departmentId`, `queueId` | UUID nullable | revalidados no catálogo local |
| `intent`, `categories`, `leadStage` | valores versionados | classificações, não comandos |
| `confidence` | decimal 0..1 | não decide sozinho |
| `extractedClaims` | lista tipada | candidatos; nunca fatos diretos |
| `toolResults` | referências sanitizadas | somente tools autorizadas |
| `closure` | objeto nullable | proposta com `requested`, reason, confidence, `humanRequested` e `sensitiveTopic`; nunca comando |
| `usage` | tokens/custo/latência | auditável |
| `warnings` | reason codes[] | parcial, stale, truncado ou fallback |

Validações cruzadas mínimas:

- `outcome=reply` exige texto não vazio e proíbe `needsHuman=true`;
- `outcome=handoff` não pode criar texto para envio automático;
- `outcome=no_reply` não pode carregar sender/action;
- `closure.requested=true` exige `replyDraft` final quando a policy vigente assim determinar;
- closure não muda FSM; somente `SystemTryAutoClose` pode aceitar/rejeitar;
- `departmentId`/`queueId` pertencem ao account e catálogo recebidos;
- todo item em `processRunRefs` possui process key, binding, version refs e schema;
- qualquer ID de escopo divergente invalida o resultado inteiro;
- campo desconhecido invalida o contrato estrito.

`promptDefinitionId`, rollout, agent/model, context snapshot, source/tool run, evaluation e trace
continuam obrigatórios nos `intelligence.runtime_runs` referenciados e ligados ao `decisionId`. O
evento aceito carrega `decisionId`; o consumidor recarrega a auditoria pela porta pública
autorizada.

`replyDraft` é string, não objeto; a relação com a mensagem respondida é escolhida/revalidada pelo
Omnichannel. Sentiment, detalhes de handoff e resumo continuam outputs de processos próprios e não
são contrabandeados em `InteractionDecision.v1`.

A façade de `brain.result.v3` mapeia closure proposal e final reply sem perda para o contrato acima.
O handler chama o mesmo `SystemTryAutoClose`, que preserva `humanRequested`, `sensitiveTopic`,
geração, policy, avaliação, mensagem `PENDING` e outbox atômicos. Mapeamento incompleto permanece
em shadow/legado e bloqueia cutover.

O envelope não possui `send`, `sendNow`, provider token, endpoint, template livre ou comando de
gravação.

## 9. Resolução de prompts e customização pelo painel

Customer Intelligence resolve, nesta ordem determinística:

```text
platform_guardrail
  + agency_policy
  + client_policy
  + process_prompt
  + agent_override permitido
  + runtime_context autorizado e minimizado
```

Para cada processo, a execução deve persistir:

- `process_key`;
- `prompt_definition_id`;
- `prompt_binding_id`;
- IDs de todas as versões de camada;
- checksum do prompt compilado, sem conteúdo em log comum;
- versão de schema de input/output;
- modelo e parâmetros estruturados;
- tools e fontes efetivamente autorizadas;
- rollout/canary;
- snapshot de contexto;
- resultado de validação/eval.

O painel poderá configurar capability, timeout, retry, orçamento, modelo allowlisted, binding,
fontes, tools, thresholds e fallback por escopo permitido. Prompts específicos por processo
controlam tom, estratégia, objetivo e conteúdo semântico. Continuam fora do alcance de prompt:

- tenant/client/subject/relationship;
- RBAC e módulo habilitado;
- lease, FSM, takeover humano e `ai_generation`;
- dedupe, idempotência e schema;
- consentimento, opt-out e retenção;
- allowlist de source/tool/model;
- janela e capacidade real do canal;
- criação de `PENDING`, outbox, adapter e sender.

Uma alteração do painel só afeta produção depois do lifecycle aplicável:
`draft → validate → test → publish → canary → active`. Publicado é imutável; rollback troca binding
para versão anterior.

## 10. Transação de aceitação e outboxes

### 10.1 Resultado `reply`

Uma única transação do Omnichannel deve:

1. localizar e bloquear dispatch/conversa no `account_id` correto;
2. confirmar módulo/capability, estado `ai_active`, lease e `ai_generation`;
3. confirmar que não houve takeover humano, supressão ou restrição posterior;
4. revalidar tamanho, janela, template, destino e catálogo operacional;
5. registrar a decisão aceita e seus IDs de inteligência;
6. inserir exatamente uma mensagem `OUTBOUND/TEXT/PENDING/origin=ai`;
7. inserir exatamente um item em `messaging.outbox`;
8. inserir exatamente um evento no outbox de integração;
9. concluir o dispatch com o mesmo resultado lógico;
10. fazer commit.

Se qualquer passo falhar, nenhuma das três saídas — mensagem, outbox do canal e evento de
integração — pode ficar parcialmente persistida.

### 10.2 `handoff`, `no_reply` e proposta de fechamento

- revalidam os mesmos IDs, lease, estado e geração;
- aplicam somente transições permitidas pela FSM;
- não criam mensagem/outbox de canal quando não há reply aceita;
- registram o resultado aceito e evento de integração na mesma transação;
- uma decisão inválida ou vencida é registrada como rejeitada para telemetria, sem aprendizado
  positivo e sem efeito operacional.

`brain.result.v3 decision=close` não pode ser reduzido a `outcome=no_reply` nem mapeado para uma
versão incompleta do contrato. Enquanto CI-00 permanecer `DRAFT`, esse caminho continua no
legado/shadow. Depois que o `InteractionDecision.v1` final de CI-00 ficar `READY` e a equivalência
for provada, a façade mapeia field-by-field closure, final reply, confidence, human/sensitive
flags, generation e idempotency para esse contrato. O handler continua chamando
`SystemTryAutoClose`, que avalia policy e, quando aceita, cria final reply/outbox e mudança de
estado na transação Omnichannel.

### 10.3 Outbox de integração

CI-07 não deve reutilizar `messaging.outbox` se esse contrato estiver restrito a envio de canal. O
owner deverá escolher, antes de `READY`, entre uma infraestrutura comum já definida por CI-01/CI-02
ou uma tabela de integração proprietária do Omnichannel.

Campos mínimos:

| Campo | Regra |
|---|---|
| `event_id` | UUID único |
| `topic` | `omnichannel.interaction.accepted` |
| `schema_version` | versão explícita |
| `account_id`, `client_account_id` | obrigatórios e indexados |
| `aggregate_id` | `conversation_id` ou `decision_id`, conforme contrato final |
| `idempotency_key` | unique dentro do producer |
| `occurred_at` | instante do estado autoritativo |
| `causation_id`, `correlation_id` | opacos e sem PII |
| `payload` | IDs, outcome e reason codes; sem conversa/prompt bruto |
| `available_at`, `attempts`, `locked_at` | entrega com retry/lease |
| `published_at`, `last_error_code` | conclusão e erro sanitizado |

O consumidor Customer Intelligence:

- registra `event_id` único antes/após aplicação conforme protocolo transacional;
- carrega detalhes por portas autorizadas e com o mesmo escopo;
- grava evidence/claim/run de forma idempotente;
- não reabre dispatch nem cria mensagem;
- trata tombstone/consentimento revogado antes de enriquecer;
- move falhas permanentes para inspeção/DLQ sem bloquear sender.

## 11. Modo degradado, retry e fallback

| Situação | Comportamento obrigatório |
|---|---|
| módulo ausente/desabilitado | não criar novo dispatch inteligente; fluxo humano normal |
| timeout/transiente | requeue bounded conforme policy; nenhuma mensagem |
| resultado inválido | rejeitar, registrar reason code e aplicar fallback estruturado |
| lease/geração vencida | cancelar resultado, sem mensagem, handoff ou aprendizado positivo |
| fonte parcial | executar somente se process policy permitir; registrar `sourceStatus` |
| prompt sem binding publicado | bloquear execução; não usar prompt oculto/default de código |
| orçamento excedido | parar tools/model e aplicar fallback configurado |
| provider de IA indisponível | manter chat e sender humanos; eventual handoff requer policy/FSM |
| consumer do evento indisponível | outbox acumula; envio de canal não espera consumo inteligente |

Retry deve reutilizar a mesma chave lógica e uma tentativa identificável. Fallback nunca troca
silenciosamente de cliente, prompt, modelo não autorizado ou fonte não permitida.

## 12. Shadow e comparação

Em `shadow`:

- o caminho legado continua sendo o único candidato a efeitos;
- o caminho novo recebe o mesmo snapshot autorizado, respeitando consentimento, retenção e custo;
- a decisão nova é validada, mas descartada antes de qualquer chamada de aceitação operacional;
- não há mensagem `PENDING`, outbox de canal, handoff, close, roteamento ou update de perfil;
- comparação usa IDs de prompt/binding/context/model e reason codes;
- conteúdo bruto só pode ser armazenado onde a política de PII permitir;
- divergências são classificadas por processo, cliente, canal, tipo e severidade;
- shadow não duplica tools com efeitos colaterais;
- tools mutáveis ficam bloqueadas ou substituídas por fixtures/read-only.

Métricas, thresholds e promoção pertencem à CI-10. Um toggle de painel não pode promover shadow
sem gates server-side e permissão de publicação/rollout.

## 13. Multi-tenant, LGPD e permissões

Toda chamada e persistência repete `account_id` e `client_account_id`; IDs do body nunca definem
escopo. Regras adicionais:

- `subject_id` e `relationship_id` são resolvidos/validados por Customer Data;
- out-of-scope retorna 404 quando aplicável;
- logs não registram prompt efetivo, mensagem integral, telefone, e-mail ou segredo;
- contexto é minimizado por finalidade e processo;
- consentimento revogado e opt-out vencem prompt e recommendation;
- cross-client individual permanece desabilitado;
- portfólio agregado não participa de `conversation.reply` sem um gate e contrato explícitos;
- dado sensível não entra em prompt/tool por simples configuração de usuário;
- permissões mínimas seguem CI-00, com enforcement backend por account.

O operador pode visualizar IDs, versões, origem, status e reason codes conforme permissão. Ver
conteúdo de prompt, evidência ou mensagem exige a permissão específica e o mesmo escopo do dado.

## 14. Observabilidade e auditoria

Métricas mínimas:

- dispatches criados, executados, requeued, cancelados e vencidos;
- latência por processo, binding, modelo, cliente e canal;
- taxa de schema inválido e resultado rejeitado;
- decisões por outcome e reason code;
- mensagens/outboxes criados por decisão aceita;
- duplicatas evitadas por idempotency key;
- backlog/idade/attempts do outbox de integração;
- consumo idempotente e DLQ;
- custo/tokens por binding e rollout;
- divergência shadow e takeover humano;
- contagem de runs sem prompt/context refs, que deve ser zero para o caminho novo.

Auditoria mínima:

- ator ou principal técnico;
- account/client/subject/relationship quando aplicável;
- dispatch, request, decision, conversation e message IDs;
- binding, prompt versions, model, schema, tools, sources e rollout;
- estado anterior/novo e reason code;
- horário, correlação e idempotency key;
- decisão aceita/rejeitada, sem payload sensível em log comum.

## 15. Pacotes atômicos e allowlists

Cada pacote abaixo tem PR, validação e handoff próprios. Arquivo fora da allowlist exige novo
despacho. Os nomes de arquivos novos podem ser ajustados somente na passagem para `READY`, quando
deverão ser congelados de forma exata.

### CI07-GATEWAY-01 — Porta, adapter e wiring

**Resultado:** substituir dependência concreta por porta opcional, preservando compatibilidade.

**Allowlist máxima:**

- `back/internal/modules/omnichannel/intelligence_gateway.go` (novo);
- `back/internal/modules/omnichannel/intelligence_gateway_test.go` (novo);
- `back/internal/modules/omnichannel/service_inbound.go`;
- `back/internal/modules/omnichannel/ai_dispatch_job.go`;
- `back/internal/modules/omnichannel/module.go`;
- `back/internal/platform/app/customer_intelligence_runtime_adapter.go` (criado/entregue por CI-06);
- `back/internal/platform/app/customer_intelligence_runtime_adapter_test.go`;
- `back/internal/platform/app/app.go`;
- arquivos públicos do contrato em `back/internal/modules/customerintelligence/` congelados pela
  CI-06 no despacho.

**Proibido no pacote:** DDL, backfill, remoção, web, sender/provider e workflow n8n.

### CI07-OUTBOX-02 — Aceitação e evento durável

**Resultado:** estado aceito e evento `omnichannel.interaction.accepted` atômicos.

**Allowlist máxima:**

- `back/internal/platform/database/migrations/<N_RESERVADO>_omnichannel_intelligence_outbox.sql`;
- `back/internal/modules/omnichannel/store_intelligence_outcome.go` (novo);
- `back/internal/modules/omnichannel/store_intelligence_outcome_test.go` (novo);
- `back/internal/modules/omnichannel/service_ai.go`;
- `back/internal/modules/omnichannel/service_ai_test.go`;
- `back/internal/modules/omnichannel/integration_outbox_job.go` (novo);
- `back/internal/modules/omnichannel/integration_outbox_job_test.go` (novo);
- `back/internal/modules/omnichannel/module.go`.

`<N_RESERVADO>` não é autorização para escolher número. O orquestrador deve reinspecionar
migrations, reservar o número e substituir o placeholder na allowlist do despacho.

**Proibido no pacote:** mudar adapter/provider, frontend, n8n ou escrever tabelas de Intelligence.

### CI07-FK-03 — Referências aditivas e backfill

**Resultado:** adicionar referências novas sem remover FKs/colunas legadas.

**Allowlist máxima:**

- `back/internal/platform/database/migrations/<N_RESERVADO>_omnichannel_intelligence_refs.sql`;
- `back/cmd/customer-intelligence-ref-backfill/main.go` (novo);
- `back/cmd/customer-intelligence-ref-backfill/main_test.go` (novo);
- `back/internal/modules/omnichannel/store_ai_runtime.go`;
- `back/internal/modules/omnichannel/store_ai_runtime_test.go`.

DDL e backfill devem ser despachos/commits separados mesmo dentro da trilha. O backfill exige
watermark, dry-run, checksum, resume e relatório de exceções. Nenhuma FK antiga é removida aqui.

### CI07-SHADOW-04 — Execução comparativa sem efeitos

**Resultado:** comparar legado e novo runtime sem segundo writer/sender.

**Allowlist máxima:**

- `back/internal/modules/omnichannel/intelligence_shadow.go` (novo);
- `back/internal/modules/omnichannel/intelligence_shadow_test.go` (novo);
- `back/internal/modules/omnichannel/ai_dispatch_job.go`;
- `back/internal/modules/customerintelligence/shadow.go` (novo);
- `back/internal/modules/customerintelligence/shadow_test.go` (novo);
- `back/internal/platform/app/customer_intelligence_runtime_adapter.go`;
- migration nova somente se CI-06 não tiver criado armazenamento de eval e após novo número
  reservado no despacho.

**Proibido no pacote:** sender, outbox do canal, mutação de FSM, handoff, close ou perfil.

### CI07-QA-05 — Provas integradas

**Resultado:** provar lease, takeover humano, idempotência, outbox e ausência de envio direto.

**Allowlist máxima:**

- novos arquivos `*_test.go` diretamente relacionados em
  `back/internal/modules/omnichannel/`;
- novos arquivos `*_test.go` diretamente relacionados em
  `back/internal/modules/customerintelligence/`;
- `back/internal/platform/app/customer_intelligence_runtime_adapter_test.go`;
- fixtures novas sob diretório de teste já aprovado pela CI-06.

**Proibido no pacote:** produção, migration, workflow e snapshot massivo não revisado.

## 16. Arquivos proibidos em toda a CI-07

Sem novo despacho e ownership explícito, nenhum pacote desta spec pode alterar:

- adapters Evolution, WhatsApp Cloud ou Instagram;
- implementação do sender e worker de provider, salvo teste de caixa-preta;
- schemas/tabelas de outro módulo por SQL direto;
- `automation/` e qualquer workflow n8n;
- módulo legado `automation`;
- módulos/páginas de CRM comercial;
- calendário, ERP ou site;
- qualquer arquivo de `socialpublishing`/`social-publishing`;
- migrations de remoção;
- secrets, `.env` ou credenciais.

## 17. Compatibilidade e depreciação

Wire schemas `brain.request.v2|v3` e `brain.result.v2|v3`, endpoints atuais e
`messaging.contact_intelligence` permanecem compatíveis durante shadow/canary. A tradução deve
ocorrer em adapter/façade, com um único writer por entidade e sem perder semântica v3.

Regras:

- não quebrar assinatura pública antes de telemetria provar zero uso;
- não fazer dual-write livre;
- `messaging.contact_intelligence` vira projeção/fachada de compatibilidade, não fonte nova;
- IDs legados de agent/version permanecem até CI-10 provar migração de todas as FKs;
- depreciação expõe métricas e, em HTTP, headers `Deprecation`, `Sunset` e
  `Link: <...>; rel="deprecation"` quando aplicável;
- remoção só ocorre em pacote `CI10-REMOVE-*` independente.

## 18. Rollout e rollback

Rollout:

1. contratos e adapter com capability `off`;
2. testes com Customer Intelligence ausente;
3. shadow interno;
4. shadow por workspace/client/channel/process;
5. canary allowlisted;
6. expansão gradual após gates CI-10;
7. caminho novo ativo, legado congelado como façade;
8. depreciação e remoções posteriores.

Rollback antes do writer cutover desliga o bridge e retorna ao runtime legado sem reprocessar
webhooks. Rollback depois do writer cutover mantém o novo writer e reverte leitura/binding por
fachada; não reativa writer legado sem reconciliação reversa aprovada.

Em qualquer rollback:

- sender continua no Omnichannel;
- mensagens já `SENT` não são reemitidas;
- idempotency keys e tombstones são preservados;
- prompt rollback troca binding, nunca edita versão publicada;
- outbox pendente continua drenável pelo owner correto.

## 19. Testes e comandos de validação

Comandos futuros, executados a partir de `back/`:

```text
go test ./internal/modules/omnichannel/...
go test ./internal/modules/customerintelligence/...
go test ./internal/platform/app/...
go test -race ./internal/modules/omnichannel/...
go test ./...
```

Cenários obrigatórios:

- módulo/adapter ausente e chat humano funcional;
- reply válida cria uma mensagem, um outbox de canal e um evento de integração;
- repetição da mesma decisão não cria duplicata;
- geração divergente, lease vencido e takeover humano criam zero efeitos;
- decisão de outro account/client é rejeitada;
- prompt sem binding/version refs é rejeitado;
- triage intermediária não é confundida com `no_reply`;
- `brain.result.v3 close` fica legado/shadow até o contrato final aprovado e então mapeia closure,
  final reply e todos os gates sem perda;
- handoff/close inválido não altera FSM;
- falha do consumer não impede envio já aceito;
- falha da transação não deixa mensagem/outbox parcial;
- shadow produz métricas e zero mutação;
- n8n/executor não possui caminho de provider;
- logs e eventos não contêm segredo, telefone ou prompt bruto;
- corrida entre resposta humana e IA conserva um único vencedor operacional.

## 20. Critérios de aceite

- [ ] contrato IA produz/Omnichannel aceita e envia foi aprovado pelos dois owners;
- [ ] porta é opcional e declarada pelo consumidor;
- [ ] request/decision são versionados e validados campo a campo;
- [ ] prompt definition/version/binding/context/model/rollout são rastreáveis;
- [ ] cada processo usa prompt específico publicado;
- [ ] configuração segura é administrável pelo painel sem transformar prompt em mecanismo de segurança;
- [ ] mensagem `PENDING` e outbox do canal continuam atômicos;
- [ ] evento de aprendizado nasce em outbox durável no mesmo commit aceito;
- [ ] consumo é idempotente e não bloqueia sender;
- [ ] shadow não causa efeitos;
- [ ] tenancy, consentimento e PII possuem testes negativos;
- [ ] compatibilidade e rollback não criam dois writers;
- [ ] nenhum workflow, sender ou arquivo fora da allowlist foi alterado.

## 21. Stop conditions

Parar e devolver ao orquestrador se:

- CI-00/CI-06 mudar envelope, módulos, processo ou ownership;
- a implementação exigir Customer Intelligence para boot/recebimento do Omnichannel;
- o adapter precisar importar repository/tipo interno do provider;
- não for possível aceitar estado e publicar evento de forma transacional;
- houver proposta de usar apenas o bus in-memory em caminho crítico;
- n8n precisar gravar PostgreSQL ou enviar ao provider;
- uma decisão da IA puder ignorar FSM, geração, takeover, consentimento ou outbox;
- triage intermediária ou auto-close v3 exigir tradução lossy, ou o `InteractionDecision.v1` final
  ainda não estiver aprovado/equivalente;
- shadow executar tool mutável ou sender;
- uma migration não tiver número reservado após inspeção do disco;
- arquivo permitido estiver sujo por outra trilha e não houver coordenação;
- surgir necessidade de tocar `socialpublishing`, módulo legado `automation` ou owner externo;
- testes detectarem cross-tenant, duplicata, envio sem outbox ou log de segredo.

## 22. Handoff obrigatório

Cada pacote deve registrar:

- pacote/resultado executado e commit/diff correspondente;
- arquivos lidos e alterados;
- decisões e placeholders resolvidos;
- contratos e migrations efetivamente usados;
- testes executados, resultados e provas de concorrência/idempotência;
- métricas de shadow, quando aplicável;
- confirmação de zero sender direto fora do Omnichannel;
- confirmação de zero alteração fora da allowlist;
- riscos, débitos, flags e rollback;
- próxima spec desbloqueada ou motivo objetivo do bloqueio.
