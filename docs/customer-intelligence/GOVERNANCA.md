# Governança da Inteligência do Cliente

- **Status:** DRAFT — aguardando validação do product owner
- **Versão:** 0.2
- **Data-base:** 2026-07-23
- **Decisor do produto:** Mike
- **Escopo:** Omnichannel, Customer Data, Customer Intelligence e integrações de contexto
- **Especificação companheira:** [SPECS_GERAIS.md](SPECS_GERAIS.md)

> Este documento registra intenção, limites, evidências, mudanças propostas e critérios de
> auditoria. Enquanto estiver em `DRAFT`, ele não autoriza migration, mudança de código, exclusão,
> ativação de workflow, deploy ou tratamento cross-client em produção.

## 1. Decisão executiva

O produto será dividido em três contextos centrais:

1. **Omnichannel:** conversa, canais e execução operacional;
2. **Customer Data:** identidade confiável da pessoa e sua relação com cada cliente da agência;
3. **Customer Intelligence:** contexto, LLM, evidências derivadas, fatos, resumos e recomendações.

ERP, Calendário, Site, BI e fontes offline continuam donos de seus dados. Eles expõem informações
por contratos pequenos e adapters registrados. Nenhum módulo consulta diretamente o schema privado
de outro módulo.

### 1.1 A IA responde, mas o Omnichannel envia

Não existe contradição entre resposta automática e a proibição de envio direto:

- **A IA produz a resposta:** decide se deve responder, gera o texto e devolve uma proposta
  estruturada.
- **O Omnichannel executa o envio:** valida escopo, estado da conversa, geração da IA, política,
  janela/capacidade do canal, idempotência e tamanho; depois grava a mensagem `PENDING` e a outbox.
- **O worker do Omnichannel entrega:** chama o adapter Evolution, WhatsApp Cloud ou Instagram e
  reconcilia o resultado do provider.

Para o consumidor, foi a IA que respondeu. Tecnicamente, somente o Omnichannel possui autoridade
para publicar no canal.

```text
mensagem inbound
  -> Omnichannel autentica, deduplica e persiste
  -> Customer Intelligence monta contexto e chama o modelo/n8n
  -> resposta proposta, versionada e não confiável
  -> Omnichannel revalida e decide enviar, silenciar ou transferir
  -> mensagem PENDING + outbox
  -> adapter do canal
```

Estados esperados:

| Situação | Comportamento |
|---|---|
| IA ativa e decisão válida | resposta automática segue por mensagem + outbox do Omnichannel |
| IA desativada | chat continua humano, sem despacho automático |
| IA indisponível, timeout ou saída inválida | fail-open para atendimento humano |
| humano assumiu ou geração mudou | resultado atrasado da IA é descartado |
| canal indisponível | outbox aplica retry/dead-letter sem pedir nova resposta ao LLM |

## 2. Objetivos

- Permitir chat totalmente funcional sem IA.
- Permitir inteligência headless, sem depender do inbox ou de um painel aberto.
- Construir perfil 360° com conversas online, ERP, Calendário, Site, BI e fontes offline.
- Mostrar de onde cada dado veio e como foi transformado.
- Produzir contexto versionado para LLM com orçamento e fontes autorizadas.
- Produzir resumo, abordagem recomendada, follow-up, ofertas, datas importantes e próxima ação.
- Permitir que a IA sugira novas fontes sem habilitar acesso por conta própria.
- Suportar inteligência de portfólio da agência sem vazar dados entre clientes.
- Liberar desenvolvimento paralelo por contratos estáveis e ownership explícito.

## 3. Fora de escopo

- Permitir que LLM ou n8n chamem diretamente Evolution, Meta ou Instagram.
- Permitir SQL, URL, tabela, credencial ou tool arbitrária escolhida pelo modelo.
- Transformar o n8n em banco, CRM, fonte de verdade ou sender de canal.
- Reaproveitar silenciosamente o módulo legado `automation`, WAHA ou seus workflows.
- Renomear silenciosamente `/crm`, `/automation` ou `/inteligencia`, que já têm outros usos.
- Compartilhar PII ou histórico bruto entre clientes por padrão.
- Fazer merge automático de pessoas por nome, comportamento ou fuzzy match.
- Mover tabelas vivas ou excluir compatibilidade num único cutover.

## 4. Glossário e escopo

| Termo | Identificador proposto | Significado |
|---|---|---|
| Organização | `organization_id` | agrupamento organizacional existente em `core` |
| Workspace da agência | `owner_account_id` | conta autenticada que governa dados e permissões |
| Cliente da agência | `client_account_id` | Pérola, Dr. Lucas, Dr. Antonio etc. |
| Pessoa/empresa atendida | `subject_id` | consumidor, lead ou organização identificada |
| Relação comercial | `relationship_id` | vínculo do subject com um cliente específico |
| Fonte | `source_key` | adapter registrado: omnichannel, ERP, site, calendário etc. |
| Observação | `observation_id` | evidência imutável ou referência auditável à origem |
| Claim | `claim_id` | valor candidato extraído de uma ou mais observações |
| Fato | `fact_id` | valor resolvido para uma relação, com estado e proveniência |
| Síntese | `summary_id` | resumo versionado produzido a partir de evidências identificadas |
| Recomendação | `recommendation_id` | follow-up, oferta ou ação proposta, nunca fato histórico |

Escopo alvo:

```text
organization_id
  ├─ owner_account_id (workspace autorizado)
  └─ client_account_id(s)

owner_account_id
  └─ subject_id

relationship_id
  = owner_account_id + client_account_id + subject_id
```

Uma pessoa pode ser reconhecida dentro do workspace autorizado da agência, mas seus fatos,
consentimentos, histórico e recomendações permanecem separados por `relationship_id`.

Essa árvore representa escopo lógico, não uma FK pai→filho já existente. Hoje agência e cliente
são `core.accounts` relacionados à organização por memberships. O binding futuro deve validar
mesma organização e catálogo permission-scoped; nunca inferir parentesco inexistente.

Nas tabelas tenant-scoped, `account_id` continua sendo o nome físico canônico. O termo
`owner_account_id` é usado neste documento para eliminar ambiguidade de negócio; CI-00 decidirá
se ele será apenas um alias de domínio/DTO. Não serão criadas duas colunas concorrentes para o
mesmo escopo.

Invariantes ainda a congelar em CI-00:

- owner e client devem pertencer ao mesmo escopo organizacional autorizado;
- visão cross-client exige owner agência e permissão específica;
- conta standalone só usa relação consigo mesma se essa regra for explicitamente adotada;
- um client não é inferido apenas por estar na mesma organização;
- um diretório permission-scoped de clientes substitui inferências por perfil de IA.

## 5. Estado atual comprovado em 2026-07-23

### 5.1 Base já existente

| Capacidade | Estado atual |
|---|---|
| contatos e identidade de canal | `messaging.contacts` e `messaging.contact_identities` |
| atribuição/origem | `messaging.contact_touchpoints` |
| notas, segmentos, consentimentos e merge | já existem no CRM 360 do Omnichannel |
| memória derivada | `messaging.contact_intelligence` |
| agentes, versões, runs, tools e knowledge | atualmente dentro do módulo Omnichannel |
| dispatch durável da conversa | `messaging.ai_dispatches`, protegido por estado e `ai_generation` |
| decisão final de fechamento | `messaging.ai_close_evaluations`, aplicada pelo Go |
| cliente ↔ número ↔ agente | `messaging.automation_profiles` |
| perfil estratégico do cliente | `calendar.client_profiles` |
| contexto do Calendário | adapter em `back/internal/platform/app/omnichannel_calendar_adapter.go` |
| dados ERP | módulo `back/internal/modules/crm/erp` |
| leads e tracking | schema/módulo `site` |
| CRM 360 no front | acoplado ao inbox do Omnichannel |

### 5.2 Lacunas que motivam a mudança

- `messaging.contact_intelligence` é um snapshot JSON sem proveniência por fato, histórico,
  verificação, conflitos, finalidade ou validade.
- O merge de `facts` e `preferences` é realizado junto do commit operacional da triagem.
- Contato e memória são escopados por `account_id`, mas não explicitam a relação com cada
  `client_account_id`.
- A conversa deriva o cliente pelo `automation_profile`; perfil desativado pode eliminar esse elo.
- O CRM determinístico, a memória LLM e a operação de canal convivem no mesmo módulo.
- O event bus atual é in-process; ingestão crítica exige mecanismo durável.
- O painel do inbox mistura canal, filas, agentes, modelos, tools, knowledge e CRM.
- `/crm` já é CRM Comercial/ERP; `/automation` é legado WAHA; `/inteligencia` já é inteligência
  operacional da fila.

### 5.3 Baseline documental

- A maior migration no disco na inspeção foi `0237_social_publishing_foundation.sql`.
- Nenhum número de migration fica reservado por este plano.
- Migrations existentes são imutáveis; toda alteração futura será aditiva.
- O plano preserva os workflows e recursos de outros módulos.

## 6. Ownership da arquitetura alvo

| Domínio | Deve possuir | Não pode possuir |
|---|---|---|
| Omnichannel | webhooks, providers, participante/identidade local do canal, mensagens, mídia, dispatch/lease, FSM, filas, handoff, outbox e envio | memória global, ERP, fatos cross-source ou recomendação de portfólio |
| Customer Data | subjects, relações, identidades canônicas multiorigem, external refs, notas, consentimentos e merge/undo | execução de LLM, envio a canal ou cópia integral de ERP/mensagens |
| Customer Intelligence | catálogo de fontes, observações, claims, fatos, sínteses, contexto, agentes, tools e recomendações | FSM de conversa, sender de canal ou escrita direta em módulos-fonte |
| ERP/CRM Comercial | clientes, pedidos, itens e demais dados ERP autoritativos | decidir identidade ou enviar mensagens |
| Calendário | perfil estratégico, itens e demais entidades do calendário | memória privada do contato |
| Site | leads, formulário, atribuição e tracking autoritativos | merge global de identidade |
| n8n | orquestração configurável, modelo, multimodal e tools autorizadas | banco, credenciais persistidas, permissão, FSM ou envio |

IDs técnicos propostos:

- boundary: `customer_data`; CI-00 decide entre módulo/package próprio e subdomínio
  `crm/customerdata`;
- módulo: `customer_intelligence`; package Go: `customerintelligence`;
- rota de produto: `/inteligencia-clientes`;
- APIs: `/v1/customer-data/*` e `/v1/customer-intelligence/*`.

Esses nomes permanecem propostos até a validação deste documento.

## 7. Contratos entre módulos

Interfaces finais serão congeladas nas specs. Fontes com escopos distintos não usam uma porta
genérica indistinguível:

```go
type SubjectEvidenceSource interface {
    ReadSubjectEvidence(ctx context.Context, req SubjectEvidenceRequest) (EvidencePage, error)
}

type BusinessContextSource interface {
    ReadBusinessContext(ctx context.Context, req BusinessContextRequest) (BusinessContext, error)
}

type PortfolioAggregateSource interface {
    QueryAggregate(ctx context.Context, req AggregateRequest) (AggregateResult, error)
}

type AuthorizedActionTool interface {
    ProposeOrExecute(ctx context.Context, req ActionRequest) (ActionResult, error)
}

type IntelligenceDispatcher interface {
    Dispatch(ctx context.Context, req InteractionRequest) (InteractionDecision, error)
}

type CustomerContextProvider interface {
    BuildContext(ctx context.Context, req ContextRequest) (ContextEnvelope, error)
}
```

Regras:

- a interface é declarada pelo consumidor;
- adapters concretos são montados em `platform/app`;
- não existe SQL cross-module;
- payloads levam IDs, versão, cursor e finalidade; não carregam bancos inteiros;
- tools e fontes vêm de registry allowlisted;
- falha de uma fonte degrada o contexto e não bloqueia o recebimento da conversa.

Dependência proposta: Omnichannel consome Customer Data e Customer Intelligence opcionalmente e
mantém seu participante local quando ambos estão ausentes. Customer Intelligence requer a
capacidade Customer Data para identificar `subject`/`relationship`. CI-00 deverá congelar
`RequiresModules`, `OptionalModules` e o modo degradado antes das specs executáveis.

## 8. Identidade, matching e relacionamento

Ordem segura de matching:

1. referência externa verificada e identidade exata do provider;
2. telefone E.164 ou e-mail verificado no escopo permitido;
3. ID ERP/loja e documento validado, sempre com escopo de cliente/conector;
4. nome, nascimento, endereço e comportamento apenas produzem candidatos;
5. `visitor_id` e `session_id` só viram identidade após conversão explícita.

Regras:

- confiança de matching, confiança do fato e confiança da recomendação são métricas diferentes;
- fuzzy match nunca executa merge automático;
- merges exigem auditoria, revisão e undo;
- nome do WhatsApp é uma observação candidata;
- nome manual verificado ou fonte autoritativa vence nome decorativo do canal;
- contatos atualmente fundidos entre clientes entram em relatório de exceções, não em correção
  automática.

Matching cross-client proposto: uma identidade forte já presente em outro cliente gera candidato
restrito para revisão da agência; não vincula automaticamente uma nova relação e não compartilha
fatos.

## 9. Banco de evidências e inteligência

Modelo lógico proposto:

| Entidade | Função |
|---|---|
| `source_configs` | fonte habilitada, finalidade, escopo, prioridade, retenção e referência de segredo |
| `source_observations` | snapshot sanitizado e/ou referência imutável à origem, com hash |
| `claims` | valores candidatos produzidos por fonte, regra, humano ou modelo |
| `facts` | projeção resolvida, sem apagar claims conflitantes |
| `fact_evidence` | liga fato às observações que o sustentam |
| `summary_versions` | sínteses versionadas com fingerprint das entradas |
| `recommendations` | follow-up, oferta, data, próxima ação e oportunidade |
| `context_snapshots` | pacote token-budgeted entregue a uma execução |
| `ingestion_runs` | sincronização, cursor, duração, erro e contagem |
| `source_suggestions` | novas fontes sugeridas, aguardando decisão humana |
| `audit_events` | revisão, correção, compartilhamento e ação sensível |

O dado bruto autoritativo permanece no módulo de origem. A Inteligência armazena:

- referência estável à origem;
- hash/versionamento;
- campos allowlisted necessários para auditoria;
- snapshot sanitizado quando a origem pode mudar ou desaparecer.

O produto deve mostrar “de onde veio” sem replicar indiscriminadamente payloads com PII.

Imutabilidade lógica não elimina retenção e direitos de exclusão. A policy detalhada definirá:

- tombstone e invalidação dos derivados;
- anonimização ou crypto-shredding quando aplicável;
- legal hold e expurgo de backups;
- bloqueio de reingestão;
- reconstrução de fatos, summaries, recommendations e context snapshots.

CPF, telefone e e-mail não usam hash simples enumerável. A spec de segurança definirá criptografia
e/ou fingerprint com HMAC e chaves rotacionáveis.

### 9.1 Resolução de autoridade

A autoridade será definida por tipo de fato, fonte, frescor, validade e escopo. Como default de
segurança, manual verificado e fonte autoritativa validada vencem inferência LLM; isso não cria uma
ordem global em que ERP sempre vence. Nome preferido, endereço fiscal e preferência comportamental,
por exemplo, podem ter autoridades diferentes.

Valores divergentes continuam registrados. Resolver um fato cria nova versão ou supersede o valor;
nunca apaga silenciosamente a evidência anterior.

## 10. Fontes e conectores

| Fonte | Owner | Primeira capacidade |
|---|---|---|
| Omnichannel | `omnichannel` | mensagens, identidades, touchpoints e resultados de atendimento |
| Manual/offline | Customer Data | reunião, ligação, importação, nota e correção verificada |
| ERP | `crm/erp` | cadastro, pedidos, itens, cancelamentos e produtos |
| Calendário | `calendar` | perfil estratégico do cliente e ações explicitamente autorizadas |
| Site | `site` | lead, formulário, consentimento, campanha, UTM e tracking |
| BI (conexão/dataset Pérola) | `bi` | consultas caras e limitadas, preferencialmente on-demand |

Configuração de fonte:

- usa `source_key` registrado em código;
- é habilitada por workspace e cliente;
- declara campos/categorias, finalidade e retenção;
- declara se é obrigatória ou opcional, SLA de frescor e comportamento quando stale;
- guarda referência de segredo, nunca segredo em claro;
- possui health, última sincronização, cursor e erro;
- não aceita tabela, query, URL privada ou tool ID livre escolhido pelo modelo.

A IA pode sugerir uma fonte com justificativa e lacunas esperadas. Somente um usuário autorizado
pode habilitar, configurar ou ampliar seu escopo.

Desabilitar uma fonte exige decisões separadas:

1. parar nova ingestão;
2. incluir ou excluir evidências já coletadas de novos contextos;
3. invalidar/recalcular fatos e sínteses dependentes;
4. reter, anonimizar ou apagar evidências conforme policy.

### 10.1 Escrita em outros módulos

Ler contexto e editar dados são operações distintas:

- a Inteligência pode ler o Calendário pelo adapter permitido;
- uma alteração no Calendário é uma ação/tool tipada, permissionada e auditada;
- o Calendário valida e persiste em seu próprio service/repository;
- a Inteligência nunca executa `UPDATE calendar.*` diretamente;
- o mesmo vale para ERP, Site, tarefas e futuras integrações.

## 11. Segurança, privacidade e cross-client

- Em rotas autenticadas, o escopo deriva do Principal. Webhooks públicos resolvem a conta no
  servidor a partir do identificador da rota e da credencial/assinatura do provider; gateways
  internos usam token/claims server-to-server, nunca `account_id` confiado do body.
- Recurso fora do escopo retorna `404`.
- Operador de um cliente não enxerga relações de outro cliente.
- `platform_admin` não implica compartilhamento automático.
- Prompt, resultado do modelo e conteúdo externo são input não confiável.
- Dados sensíveis ficam fora da memória LLM salvo política, finalidade e proteção específicas.
- Logs não contêm prompt bruto, payload bruto, chave, documento, pagamento ou PII desnecessária.
- Retenção é configurada por categoria e fonte.
- Desabilitar uma fonte aplica as quatro decisões da seção 10; exclusão da trilha segue política
  legal/auditável.

Antes de qualquer uso cross-client, uma revisão jurídica/privacidade registra os papéis aplicáveis
— controlador, operador, suboperador, titular e usuário aprovador — além de base legal, finalidade,
revogação, compartilhamento e tratamento de categorias sensíveis, especialmente saúde. Uma
autorização de produto não é presumida como base legal suficiente.

Inteligência de portfólio:

1. começa agregada ou anonimizada;
2. produz afinidade de público, segmento e oportunidade;
3. não revela que uma pessoa específica é cliente de outra marca;
4. ativação individual exige base/finalidade compatível, opt-out, autorização explícita e auditoria.

Agregados terão limiar mínimo de coorte e supressão contra reidentificação definidos em CI-09.

Exemplo Pérola → Dr. Lucas:

- permitido inicialmente: “segmento X da Pérola apresenta afinidade Y com o serviço Z”;
- proibido por padrão: entregar nomes/telefones individuais da Pérola ao Dr. Lucas;
- eventual campanha individual exige uma spec/política própria e gate de aprovação.

## 12. Evidência mínima de auditoria

Cada execução relevante deve permitir correlacionar, sem expor conteúdo desnecessário:

- `request_id`, `ai_run_id` e versão do schema;
- organização, workspace, cliente, subject e relacionamento;
- fontes consultadas, ignoradas e motivo;
- IDs/hashes das observações;
- versão do agente, prompt, modelo, extrator e tools;
- confiança de matching, fatos e decisão;
- proposta da IA;
- decisão final do Go: enviar, não responder ou transferir;
- mensagem criada, outbox, provider e status final;
- tokens, custo, latência e erro classificado.

Resumos registram as evidências utilizadas. Recomendações registram racional, expiração, estado e
feedback de aceite/rejeição.

## 13. Inventário de mudanças propostas

### 13.0 Ownership durante a transição

| Entidade | Writer atual | Writer após cutover | Compatibilidade |
|---|---|---|---|
| participante local do canal | Omnichannel | Omnichannel | `messaging.contacts` permanece |
| identidade específica do provider | Omnichannel | Omnichannel | projetada assincronamente |
| subject canônico | inexistente | Customer Data | mapeia para participante local |
| identidade canônica multiorigem | CRM no Omnichannel | Customer Data | fachada/read model temporário |
| notas, consentimentos e merge | CRM no Omnichannel | Customer Data | writer antigo congela antes da troca |
| evidência bruta de conversa/touchpoint | Omnichannel | Omnichannel | Intelligence guarda referência/projeção |
| fatos e sínteses | `messaging.contact_intelligence` | Customer Intelligence | fachada temporária, nunca dual-write permanente |

O estado do writer é controlado por cliente e entidade (`legacy`, `shadow`, `new`), com watermark,
checksum e relatório de comparação. A spec detalhada define o ponto sem retorno de cada cutover.

### 13.1 Manter no Omnichannel

| Área/arquivo | Decisão |
|---|---|
| `back/internal/modules/omnichannel/channel/**` | manter providers e contrato de envio |
| `service_inbound.go` | manter ingestão; trocar apenas o acoplamento direto com a IA |
| `store_webhook_events.go` | manter dedupe, participante/binding local e outbox; resolução de subject/relationship é assíncrona |
| `service_outbound.go:SendAIMessageWithResult` | manter validação e produção da resposta automática |
| `store_ai_outbound.go:CreateAIOutboundMessage` | manter mensagem `PENDING` + outbox sob lease |
| `outbound_handler.go:OutboundHandler` | manter entrega e reconciliação com o provider |
| FSM, filas, routing e handoff | manter autoridade operacional |
| `messaging.ai_dispatches` | manter agendamento, debounce durável, cancelamento e lease da conversa |
| `messaging.ai_close_evaluations` | manter auditoria da decisão final de fechamento do Go |
| `messaging.contact_ai_restrictions` | manter gate que cancela resposta pendente |
| `messaging.contact_suppressions` | manter ocultação e cutoff operacional do inbox |
| webhooks WhatsApp/Instagram | manter autenticação, normalização e escopo |
| `/v1/omnichannel/conversations/**` e envio/mídia | manter |
| bloqueio de IA por conversa | manter como controle operacional |

### 13.2 Alterar ou extrair progressivamente

| Alvo atual | Mudança proposta | Regra de compatibilidade |
|---|---|---|
| `service_inbound.go:InboundService` | depender de dispatcher opcional, não de `*AIService` concreto | chat deve iniciar sem módulo de Inteligência |
| `service_triage.go:AIService` | extrair contexto/modelo/prompt para Customer Intelligence | wrapper temporário mantém chamadas atuais |
| `store_ai_runtime.go:CommitAITriageWithIntelligence` | separar commit da conversa de aprendizado | transação aceita `state + ai_generation` e insere outbox de integração; publicação ocorre após commit |
| `store_ai_runtime.go:GetContactIntelligence` | trocar por projeção/fachada da nova fonte | uma única fonte autoritativa por etapa |
| `ai_dispatch_job.go` | manter claim/lease e trocar execução concreta por gateway | handoff, fechamento e envio continuam no Omnichannel |
| `ai_prompt.go`, `brain_*`, `ai_tool_*` | migrar ownership para Customer Intelligence | não mover tudo antes de testes de paridade |
| `automation_store.go:AutomationClientForInstance` | deixar de ser fonte de ownership cliente↔canal | usar binding operacional independente da IA |
| `automation_attendances.go:AutomationConversationScope` | usar escopo explícito da conversa/binding | perfil de IA pode não existir |
| `automation_model.go:AutomationBusinessContext*` | generalizar como fonte de contexto | adapter do Calendário passa a atender o novo contrato |
| `http_contacts_crm.go`, `service_crm.go`, `store_crm.go` | migrar para Customer Data | rotas antigas viram fachada temporária |
| `module.go` | wiring opcional dos novos módulos e adapters | ausência da Inteligência não falha o Omnichannel |

No primeiro cutover, `messaging.contacts` e `contact_identities` permanecem como participante e
identidade local do canal. A FK de `conversations.contact_id` não será quebrada. Campos CRM e
relações multiorigem migram por fachada/backfill; o inbound não fará chamada síncrona a outro
módulo dentro da transação de webhook.

Divisão proposta de `messaging.contacts`:

| Permanece como projeção local do canal no primeiro cutover | Migra para Customer Data |
|---|---|
| `name`, `phone`, `avatar_url` | `relationship_status`, `tags`, `custom_fields` |
| `first_seen_at`, `last_seen_at` | `primary_email`, `owner_user_id` |
| `first_channel`, `last_channel` | classificação, qualificação, archive e merge autoritativos |

Notas, consentimentos, external refs, segmentos e eventos de merge migram depois de backfill e
troca do writer. Touchpoints diretamente ligados a mensagem/conversa podem permanecer como
evidência bruta no Omnichannel enquanto Customer Data mantém sua projeção multiorigem.

### 13.3 Criar

| Alvo proposto | Finalidade |
|---|---|
| `back/internal/modules/customerdata/**` **ou** `back/internal/modules/crm/customerdata/**` | alternativa de CI-00 para subjects, relações, identidades, consentimentos e merge |
| `back/internal/modules/customerintelligence/**` | fontes, evidências, fatos, contexto, agentes e recomendações |
| `AGENT.md` em cada módulo | ownership, contratos, tabelas, rotas e falhas conhecidas |
| adapters em `back/internal/platform/app/**` | composição sem import/SQL cross-module |
| binding canal↔cliente no domínio Omnichannel | ownership independente de `automation_profiles` |
| evento/outbox de ingestão durável | aprendizado e sync idempotentes |
| APIs `/v1/customer-data/*` | perfil determinístico |
| APIs `/v1/customer-intelligence/*` | perfil derivado, fontes, runs e recomendações |
| workspace `/inteligencia-clientes` | perfil completo fora do inbox |
| painel de fontes/evidências | ativação, health, origem, conflito e revisão |

### 13.4 Frontend

Manter:

- `web/app/pages/omnichannel/index.vue`;
- `web/app/components/omnichannel/inbox/**`;
- componentes/composables de conversa, histórico, mídia, realtime, envio, filas e handoff;
- configuração de números e capacidades de WhatsApp/Instagram.

Decompor:

- `OmnichannelCRMProfilePanel.vue`: cadastro, notas, identidades e merge vão para Customer Data;
  fatos, evidências, resumo e recomendações vão para Customer Intelligence;
- `useOmnichannelCRM.ts`: tipos/chamadas determinísticos e inteligentes viram contratos separados;
- `pages/omnichannel/automacao.vue`;
- `components/omnichannel/automation/**`, exceto controles operacionais;
- agentes, modelos, credenciais de IA, tools, knowledge, runs e contexto estratégico.

Criar:

- `web/app/components/customer-intelligence/**`;
- `web/app/composables/customer-intelligence/**`;
- `web/app/domain/customer-intelligence/**`;
- `web/app/domain/customer-data/**`;
- `web/app/stores/customer-intelligence.ts`;
- páginas `/inteligencia-clientes`, `/inteligencia-clientes/:subjectId`,
  `/inteligencia-clientes/fontes`, `/inteligencia-clientes/auditoria`,
  `/inteligencia-clientes/atendimentos` e `/inteligencia-clientes/portfolio`.

O inbox compõe dois contratos opcionais, sem criar nova fonte de verdade:

- `OperationalContactSnapshot`, do Omnichannel: nome/canal/estado local;
- `IntelligenceCompactSnapshot`, de Customer Intelligence: resumo, fatos confiáveis e próxima ação.

Se Customer Intelligence estiver indisponível/desabilitado, o primeiro snapshot mantém o chat
funcional e o segundo não é buscado.

`messaging.media_analyses` também permanece no primeiro cutover por suas FKs operacionais.
Customer Intelligence assume gradualmente modelo, credencial, execução, tokens e custo; a mídia
binária e sua autorização continuam no storage privado do Omnichannel.

### 13.5 Compatibilidade, deprecação e exclusão

Nenhuma exclusão é autorizada por este documento.

Candidatos futuros:

- merge JSON direto em `messaging.contact_intelligence`;
- endpoints CRM/IA antigos sob `/v1/omnichannel`;
- página `/omnichannel/automacao`;
- componentes/composables duplicados após migração;
- tabelas antigas de inteligência, nunca mensagens/outbox/FSM.

Classificação de APIs na transição:

| Destino | APIs/ações |
|---|---|
| migram para Customer Intelligence | agentes, credenciais de IA, knowledge, tools, usage e runs |
| leitura pode aparecer na nova workspace | intervenções e atendimentos de automação |
| permanecem comandos Omnichannel | `pause-ai`, `reply-with-ai`, fechar/reabrir e `ai-restriction` |
| permanece Omnichannel | upload/stream da mídia binária e entrega ao canal |

O redirect futuro de `/omnichannel/automacao` só ocorre quando o módulo e a permissão de Customer
Intelligence estiverem ativos. Deve preservar cliente/query permitidos, impedir loop e não expor
o destino a quem possui apenas a permissão antiga.

Inventário de símbolos candidatos a retirada, sempre após paridade e consumidor zero:

| Símbolo/arquivo atual | Destino antes da retirada |
|---|---|
| `contact_intelligence.go:ContactIntelligenceView` | DTO/projeção de Customer Intelligence |
| `Store.GetContactIntelligence` | reader/fachada para a nova projeção |
| `normalizeContactMemory` | sanitização/versionamento no novo módulo |
| `withContactMemoryOutputSchema` | contrato de saída versionado do novo runtime |
| `buildUserPromptWithContactIntelligence` | context builder de Customer Intelligence |
| trecho inteligente de `CommitAITriageWithIntelligence` | evento `interaction.outcome.accepted` |
| `service_triage.go:AIService` | gateway e decision service após paridade |
| `omnichannelCalendarContextAdapter` | source adapter registrado no novo composition root |

Tabelas candidatas a deprecação gradual:

- `messaging.contact_intelligence`;
- agentes, versões, campos coletáveis, runs e credenciais de IA em `messaging.*`;
- bindings/runs/approvals de tools e tabelas de knowledge em `messaging.*`;
- `messaging.automation_profiles`, somente após existirem binding operacional e binding de
  inteligência separados;
- tabelas/campos CRM em `messaging.*`, somente após Customer Data assumir o writer.

Antes de retirar `ai_runs`, `ai_agent_versions`, `ai_credentials` ou `automation_profiles`, uma
migration aditiva precisa criar novas referências, fazer backfill e trocar as FKs atuais de
`ai_dispatches`, `ai_close_evaluations`, `handoffs`, `routing_decisions` e `media_analyses`.
Nenhum drop acontece enquanto uma dessas referências apontar para o schema legado.

Mesmo no alvo final, não são candidatos à exclusão por esta iniciativa:

- `messaging.ai_dispatches`;
- `messaging.ai_close_evaluations`;
- `messaging.contact_ai_restrictions`;
- `messaging.messages`, `conversations`, `outbox` e dados de handoff/routing.

Gate para excluir:

1. consumidor inventariado;
2. backfill comparado por conta/cliente;
3. escritor antigo congelado;
4. tráfego da compatibilidade em zero durante janela aprovada;
5. rollback testado;
6. retenção/auditoria resolvidas;
7. aprovação explícita do owner;
8. remoção em pacote separado.

Não tocar no módulo legado `automation`, WAHA ou workflows de Calendar/Operação/outros owners.

## 14. Migração, cutover e rollback

1. Congelar vocabulário, ownership e contratos.
2. Criar estruturas aditivas.
3. Inventariar e backfillar instância↔cliente; produzir relatório de órfãos/ambiguidades.
4. Introduzir relação cliente↔subject e revisar contatos potencialmente misturados.
5. Ingerir evidências em shadow mode, sem alterar resposta ao consumidor.
6. Comparar contexto legado e novo por cliente, fonte, latência e vazamento de escopo.
7. Ativar leitura nova por feature flag server-side, cliente e canal.
8. Manter um único escritor autoritativo em cada etapa; dual-write permanente é proibido.
9. Migrar frontend e transformar rotas antigas em fachadas.
10. Observar tráfego e somente então iniciar deprecação.

Shadow mode que chama LLM/n8n com dados reais possui os mesmos gates de fornecedor, finalidade,
retenção, custo e auditoria da produção. “Não enviar ao consumidor” não elimina tratamento de PII.

O smoke de cutover inclui texto, mídia, áudio, reply, `fromMe`, duplicata, falha do provider,
resultado atrasado e handoff humano.

Rollback:

- antes da troca do writer, pode voltar a leitura/contexto legado por feature flag;
- depois da troca do writer, mantém o writer novo e usa projeção/fachada compatível; reativar uma
  memória legada congelada exige reconciliação reversa explícita;
- preserva dados novos para análise;
- não reprocessa webhook/evento já deduplicado;
- nunca troca o caminho de envio: continua Omnichannel/outbox;
- não reativa escritor antigo sem desativar o novo.

## 15. Gates de liberação

| Gate | Prova mínima |
|---|---|
| arquitetura | nenhum sender no n8n; nenhum SQL cross-module |
| chat independente | receber, responder humano, transferir e encerrar com Inteligência ausente |
| inteligência headless | formar perfil a partir de ERP/manual sem inbox aberto |
| tenant | testes negativos entre organização, workspace, cliente, subject e relação |
| identidade | ambiguidades não causam merge automático; undo provado |
| fatos | origem, data, confiança, estado e conflito visíveis |
| IA | timeout, schema inválido, confiança abaixo do limite configurado e tool insegura fazem fail-open; resposta forçada autenticada segue policy própria |
| idempotência | repetir evento/source record não duplica observação, ação ou envio |
| LGPD | finalidade, retenção, consentimento e exclusão aprovados antes de cross-client |
| cutover | shadow comparado e rollback ensaiado |
| operação | backlog, duplicatas, latência, handoff, custo e falha de provider monitorados |
| compatibilidade | APIs antigas e novas não escrevem em fontes diferentes |

## 16. Processo de governança

Status dos documentos e pacotes:

| Status | Significado |
|---|---|
| `DRAFT` | decisão ou contrato ainda pode mudar |
| `READY` | dependências e decisões fechadas; implementação pode ser despachada |
| `IN_PROGRESS` | executor iniciou dentro da allowlist |
| `BLOCKED` | bloqueio objetivo registrado |
| `IMPLEMENTED` | código e testes do autor passaram |
| `VERIFIED` | revisor independente conferiu diff e evidências |
| `DONE` | integração, documentação e demonstração aceitas pelo owner |

Todo pacote deve registrar:

- baseline e worktree antes;
- arquivos permitidos e alterados;
- decisão atendida;
- migration/backfill, se houver;
- contratos/API/eventos afetados;
- testes e seus resultados;
- critérios provados e não provados;
- rollout e rollback;
- consumidores/deprecações;
- confirmação de que nenhum workflow/recurso externo foi alterado.

O autor de uma implementação não marca sozinho seu pacote como `VERIFIED` ou `DONE`.

## 17. Decisões para validação

| ID | Proposta recomendada | Status |
|---|---|---|
| CI-DEC-001 | módulo novo chama-se `customer_intelligence` | proposto |
| CI-DEC-002 | dados determinísticos usam boundary `customer_data`; decidir entre módulo próprio ou `crm/customerdata`, sem reaproveitar silenciosamente `/crm` | pendente de validação |
| CI-DEC-003 | `subject_id` é deduplicável apenas dentro de `owner_account_id` | proposto |
| CI-DEC-004 | fatos e consentimentos são sempre separados por `relationship_id` | proposto |
| CI-DEC-005 | binding canal↔cliente pertence ao Omnichannel e independe da IA | proposto |
| CI-DEC-006 | autoridade é configurada por tipo de fato/fonte/frescor; LLM não vence fonte verificada | proposto |
| CI-DEC-007 | ingestão crítica usa outbox durável, não apenas event bus in-process | proposto |
| CI-DEC-008 | dados cross-client individuais ficam desabilitados por padrão | proposto |
| CI-DEC-009 | observação guarda referência/hash + snapshot allowlisted, não payload bruto irrestrito | proposto |
| CI-DEC-010 | métricas quantitativas de shadow serão fechadas antes do rollout | pendente de spec |
| CI-DEC-011 | retenção por categoria/fonte será aprovada antes de produção | pendente de política |
| CI-DEC-012 | contatos já misturados entram em quarentena/revisão, sem correção automática | proposto |
| CI-DEC-013 | match forte entre clientes apenas gera candidato restrito; não vincula automaticamente | proposto |
| CI-DEC-014 | definir conta standalone e invariantes owner/client/organização | pendente |
| CI-DEC-015 | definir controlador, operadores, base legal, revogação e dados sensíveis | pendente jurídico |
| CI-DEC-016 | definir tombstone, anonimização/crypto-shredding, legal hold e backups | pendente |
| CI-DEC-017 | definir criptografia/HMAC e rotação para identificadores | pendente |
| CI-DEC-018 | `subject_type` suporta pessoa e empresa com atributos próprios | proposto |
| CI-DEC-019 | context snapshots têm criptografia, TTL e conteúdo minimizado | proposto |
| CI-DEC-020 | portfólio exige limiar de coorte/supressão contra reidentificação | proposto |

## 18. Histórico de revisão

| Versão | Data | Autor | Alteração | Aprovação |
|---|---|---|---|---|
| 0.1 | 2026-07-23 | Codex | primeira governança baseada no estado real do repositório | pendente |
| 0.2 | 2026-07-23 | Codex + revisão adversarial | corrigidos topologia de identidade, transação/outbox, FKs, ownership de transição, privacidade, rollback e garantia de envio | pendente |
