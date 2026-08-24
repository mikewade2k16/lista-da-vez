# Módulo `omnichannel` — Atendimento omnichannel

Inbox humano, CRM multicanal, setores/filas e primeiro atendimento por IA, com o Go e o
PostgreSQL como fontes de verdade.
Schema `messaging.*`. Plano canônico: `docs/omnichannel/PLANO_ATENDIMENTO.md`.
Specs por fase: `docs/omnichannel/specs/OMNI-F*.md`.

## Direção vigente — MVP de automação (2026-07-21)

O recorte prioritário atual é
`docs/omnichannel/MVP_AUTOMACAO_ATENDIMENTO.md`: somente WhatsApp, sem resposta pelo painel,
com IA iniciando o primeiro atendimento, sugerindo encerramento e transferindo para humano.
A primeira versão da página `/omnichannel/automacao` e sua API base estão implementadas em
`automation_model.go`, `automation_store.go`, `automation_service.go` e
`http_automation.go`, com migration `0228_messaging_automation_profiles.sql`.

Contrato do MVP-01:

- um perfil por cliente e um número por cliente;
- cliente vem do mesmo catálogo permission-scoped de `/v1/tenants` usado pelo Calendário, mas o
  adapter solicita `ListInput.ModuleID="omnichannel"`; somente contas com
  `core.account_modules.enabled=true` para o módulo aparecem e um `clientId` fora desse catálogo
  continua retornando 404;
- perfil estratégico continua autoritativo em `calendar.client_profiles` e chega somente pelo
  adapter de composição `platform/app/omnichannel_calendar_adapter.go`; é proibido SQL direto;
- profile aponta para a instância lógica, portanto Evolution e WhatsApp Cloud reutilizam o vínculo;
- o perfil habilitado é obrigatório para cada número; não existe fallback para agente global.
  Perfil ausente/desabilitado ou instância inativa falha fechado antes do modelo, antes da outbox e
  imediatamente antes do provider;
- desligar o perfil é o kill switch autoritativo: incrementa a lease, cancela dispatches e saídas
  pendentes e tira conversas de `ai_active` no mesmo commit;
- JID WhatsApp `@g.us` é grupo, não contato: permanece visível no inbox separado de pessoas, não
  cria identidade de CRM e nunca chama triagem, análise multimodal ou envio por IA;
- encerramento automático nasce desligado. Threshold e gates são configuráveis, mas
  `conversations.ai_generation` válida é uma invariante obrigatória e não pode virar toggle;
- `PUT /v1/omnichannel/agents/{id}/configuration` é o salvamento simples do MVP: normaliza,
  persiste e ativa a configuração no mesmo commit. O banco mantém versões para auditoria e
  rollback, promove um draft idêntico quando existir, arquiva drafts antigos e não cria nova
  versão quando o conteúdo já é o ativo. As rotas draft/publish permanecem apenas compatíveis;
- nenhuma migration é aplicada, container reiniciado ou workflow importado por consequência
  desta fase. Essas ações precisam de autorização e alvo explícitos.

### Incidente de rollout — sessão duplicada e perfil ausente (2026-07-22)

- Sintoma: webhook e mensagens inbound chegavam em produção, mas não existiam `ai_dispatches` ou
  `ai_runs`; a visão geral mostrava zero clientes configurados e a conversa estava
  `human_active`.
- Causa provada: o agente só foi publicado depois das mensagens de teste, nenhum
  `messaging.automation_profiles` vinculava cliente+número+agente e mensagens anteriores enviadas
  pelo aparelho (`origin=provider_device`) fizeram takeover humano. O mesmo número também estava
  conectado em Evolutions independentes local e produção.
- Correção operacional: criar/habilitar o perfil somente após número e agente estarem prontos;
  manter um único ambiente consumidor por número; parar o outro ambiente preservando volumes;
  encerrar a conversa de teste antes de validar um novo inbound.
- Diagnóstico preventivo: correlacionar nesta ordem `webhook_events/messages → conversations.state
  → automation_profiles → ai_dispatches → ai_runs → outbox`. Webhook 202 sozinho não prova que a
  automação estava elegível, e ausência de dispatch significa gate anterior ao n8n.

Contrato do MVP-02:

- `brain.request/result.v3` acrescenta `close` e metadados de handoff; `brain.v2` permanece
  aceito por versões antigas e nunca pode encerrar conversa;
- a IA apenas propõe `close`; somente `SystemTryAutoClose` pode persistir `closed`;
- o fechamento final, a mensagem de despedida e sua outbox são atômicos sob lock da conversa;
- fechamento automático habilitado, confiança, campos obrigatórios, pedido humano e assunto
  sensível são gates configuráveis por perfil; `ai_generation` válida é sempre obrigatória;
- cada tentativa de close é auditada em `messaging.ai_close_evaluations` (migration `0229`),
  sem prompt, chave ou conteúdo da conversa;
- `fromMe` novo faz takeover no mesmo commit da mensagem; duplicata/eco nunca incrementa de novo;
- `handoff` usa `messaging.handoffs`; o n8n não escolhe fila, não muda estado e não envia canal;
- cards operacionais consultam `/v1/omnichannel/automation/attendances` e projetam do PostgreSQL
  conversas `ai_active`, `human_active` e handoffs parados, com contagem/preview limitado das mensagens
  inbound posteriores à última resposta. `pause-ai` cria handoff auditado e invalida a lease;
  `reply-with-ai` fecha o handoff, volta para `ai_active` e cria dispatch/outbox para as mensagens
  já persistidas no mesmo commit. Em `human_active`, a mesma ação autenticada permite ao operador
  devolver a conversa à IA quando há inbound pendente. Nenhuma dessas ações chama n8n ou Evolution diretamente;
- `Retomar nas próximas` continua usando `PATCH /conversations/{id}/status` com `CLOSED` somente
  quando não há mensagem pendente. O inbound seguinte reabre pelo fluxo canônico.
- `reply-with-ai` é uma ordem manual autenticada para uma resposta: o dispatch marca `ForceReply`,
  ignora `min_confidence`, `max_ai_turns`, `needs_human`/`close` sugeridos pelo modelo e reforça no
  prompt que `reply_draft` deve ser preenchido. Não ignora configuração ausente, falha do provider,
  limite mensal, schema inválido, resposta vazia, escopo tenant nem lease de `ai_generation`;
- modelos OpenAI-compatible legados podem aceitar chat e aparecer em `/models`, mas rejeitar
  `response_format=json_object` com HTTP 400. O client tenta uma única vez sem essa dica e mantém a
  validação do JSON Schema no Go; outros status não repetem a chamada e nenhum corpo de erro é logado;
- motivos de parada permanecem específicos de ponta a ponta. `max_ai_turns` gera handoff
  `max_turns`, confiança abaixo do mínimo gera `low_confidence` e limite mensal cai em `policy`.
  A projeção dos cards recupera handoffs antigos rotulados incorretamente consultando o último
  `ai_run`, e expõe confiança/mínimo/máximo sem prompt ou conteúdo adicional.
- `max_ai_turns=0` significa **sem limite de respostas automáticas** de ponta a ponta. O default
  do schema, a normalização da policy e a UI devem preservar zero; somente valores positivos
  aplicam o gate `max_turns`.
- ocultação de contato é uma supressão lógica account-scoped em
  `messaging.contact_suppressions`: remove o contato e suas conversas das projeções do inbox,
  CRM e automação sem apagar auditoria. `clearHistory=true` avança o cutoff; depois de restaurar,
  apenas mensagens posteriores ao cutoff reaparecem. A gestão exige o grant explícito
  `omnichannel.conversations.privacy.manage`; nem `platform_admin` ignora esse gate.
- `AIVersionInput.minConfidence` distingue campo ausente de `0`: ausente aplica o default `0.65`;
  zero é configuração explícita válida e significa não interromper resposta automática por confiança.

### Roteiro técnico anterior

O roteiro executivo atual é
`docs/omnichannel/PLANO_TECNICO_EVOLUCAO.md`. O plano F0–F14 acima continua como histórico
do port e dos contratos já entregues. Para qualquer tarefa neste módulo, o Codex deve usar
as skills pessoais `$principios-engenharia` e `$omnichannel-hibrido` e ler este AGENT antes
de alterar código. A execução E0–E10 é detalhada em
`docs/omnichannel/evolucao/README.md`: um agente executor recebe o contrato comum e somente o
pacote atômico autorizado; ler uma fase inteira não concede permissão para implementá-la inteira.

### Fronteira Go/PostgreSQL x n8n

| Go/PostgreSQL | n8n |
|---|---|
| adapters, webhooks e assinatura | debounce e agrupamento |
| dedupe e idempotência | montagem do contexto |
| contatos, identidades, CRM e atribuição | chamada ao modelo configurado |
| mensagens, mídias e estados | transcrição e visão |
| setores, filas, responsáveis e RBAC | tools autorizadas pelo Go |
| auditoria, custos, retenção e LGPD | decisão estruturada IA/handoff |
| outbox e envio final | orquestração configurável |

Regra absoluta: workflow n8n não envia diretamente para Evolution, WhatsApp Cloud ou
Instagram e não grava no banco do produto. O Go decifra a chave somente para a chamada
server-to-server, valida novamente a saída, aplica FSM/routing e cria mensagem `PENDING` +
outbox. `OMNI_AI_EXECUTOR=native|n8n` é chave de rollout/rollback, não uma segunda fonte de
configuração.

O cofre `messaging.ai_credentials` e global por conta e pode ser consumido por
outros modulos somente por fachada server-side explicita, injetada no composition
root. O Omnichannel nao consulta schemas desses consumidores e o segredo nunca
entra em views HTTP, logs ou configuracoes de workflow.
Para consumidores server-side da mesma organizacao, uma credencial pertencente a
account agencia ativa pode atender accounts clientes ativas com a mesma
`organization_id`, mas somente quando a owner e a conta-agencia canonica (ativa mais
antiga, desempate por id). A rota nativa continua account-scoped. A superficie neutra
`/v1/assistant/ai-credentials` lista credenciais proprias e herdadas mascaradas com
`ownedByAccount`, `ownerName` e `readOnly`; GET/models aceita gestores das superficies,
enquanto criar/rotacionar/importar/excluir exige administracao transversal e muta apenas
a conta ativa. A fachada de runtime aplica o mesmo escopo canonico e nunca devolve a chave.
As FKs RESTRICT da migration `0284` impedem excluir credencial ainda referenciada por
Automation ou pela analise de atendimentos, sem o Omnichannel consultar schemas consumidores.

### Canais e CRM

- Evolution é a ponte do piloto; WhatsApp Cloud API é o destino oficial.
- Um número só pode ter um provider ativo. Nunca Evolution e Cloud API simultâneas.
- Instagram possui adapter e workflow próprios para DM/comentários, compartilhando inbox,
  contatos, identidade, touchpoints, cérebro e regras de handoff.
- `messaging.contacts` é a entidade CRM; telefone não é identidade universal.
- `contact_identities`, `contact_touchpoints` e `contact_notes` são autoritativos.
- Cliente existente vem de regra/CRM/ERP, nunca de inferência livre do modelo.
- Landing page, campanha, anúncio e UTMs entram como touchpoints persistidos pelo Go.

### Ownership n8n — não cruzar módulos

Este módulo é dono somente de `workflow-omnichannel-brain.json` (`omnibrain0000001`) e
`workflow-instagram-first-contact.json` (`instafirst000001`). `workflow-whatsapp.json`,
WAHA e o módulo Go `automation` pertencem a outro produto e permanecem intactos. Calendar e
Operação também têm workflows próprios. Nunca editar, importar, exportar, ativar, desativar,
remover ou aplicar regras do omnichannel a ids de outro módulo.

### Estado atual resumido

- Entregue: schema, inbox, contatos/identidades/touchpoints, mídia privada, realtime,
  outbox/FIFO, FSM, filas/routing, adapter mock/Evolution, agente versionado, auditoria de
  custo, executor n8n e multi-turno inicial.
- E7/E8 code-complete local: adapters Meta Cloud/Instagram, templates/janela, moderação e painel;
  smoke/cutover com credenciais reais ainda pendente.
- E9/E10 ficam pausadas enquanto o MVP de automação WhatsApp é validado. Comentários/menções
  nunca publicam sem aprovação humana e todas as ações continuam na outbox Go.

## 🔴 Backlog de funcionalidades + bugs conhecidos (a pedido do dono, 2026-07-18)
**Prioridade do dono: FUNCIONALIDADES primeiro; aparência + estes bugs DEPOIS.** NÃO polir UI nem
corrigir isto antes de fechar as funcionalidades pendentes. Descobertos com WhatsApp **real** pareado
(conversa ao vivo — o pareamento local já funciona, ver `docs/omnichannel/ESTADO.md`).

1. **Mídia recebida (imagem) não renderiza** — a bolha fica em "Foto recebido. Carregando preview...".
   A imagem inbound não chega ao front. Investigar o caminho de mídia inbound: adapter Evolution
   (`getBase64FromMediaMessage`/`webhookBase64` em `channel/evolution/client.go`) + persistência
   (`media_storage_key`, migration 0207) + o `GET /conversations/{cid}/messages/{mid}/media` que o
   front consome. Provável: a mídia não é baixada/armazenada no inbound, ou o stream falha.
2. **"Responder" (quote) não vai pro WhatsApp real** — o reply aparece só na plataforma; no WhatsApp
   de verdade chega como mensagem normal, sem citar. O envio outbound não passa o `quoted`/contexto de
   reply pro provider. Falta: mapear a mensagem citada no send → campo `quoted` do Evolution
   (`/message/sendText`), em `service_outbound.go` + `channel/evolution` (SendMessage).
3. **Mensagens enviadas PELO celular (fromMe) não aparecem na plataforma** — o que o dono manda direto
   do WhatsApp pareado não é ingerido. `ParseWebhook`/`ingestOne` só tratam inbound (fromMe=false); o
   Evolution manda `messages.upsert` com `key.fromMe=true` nos envios do device. Falta ingerir esses
   como **OUTBOUND** (espelhar no histórico) SEM duplicar os que a própria plataforma já enviou
   (dedupe por external id).
4. **Ajustes visuais menores** — pendentes; só depois das funcionalidades (decisão do dono). O
   alinhamento base ao design system já foi feito (bolhas viraram card, tokens corrigidos).

> **Estado: F4 (canal provider-agnóstico + webhook inbound + sessão/QR).** Entrou a camada
> tradutora `channel.Provider` + eventos canônicos, o **registry** de providers (só o `mock`
> registrado; `evolution` pluga na fase seguinte), o **webhook inbound público** (dedupe por
> `messaging.webhook_events`) e o **ciclo de sessão/QR** (bootstrap/connect/status/qrcode/
> logout). **F5 (realtime) ligado**: o webhook inbound publica `message.created` no canal
> `omnichannel:account:{id}` (transporte `realtime`, injetado via `WithPublisher`); ainda
> **sem envio** (F6). Com `provider='mock'` grava inbound sem número real.

> **F5 — realtime (canal `omnichannel:account:{id}`).** `publisher.go` define a interface
> `Publisher` (`PublishOmnichannelEvent(RealtimeEvent)`) + os 3 literais de evento
> (`RealtimeEvent{MessageCreated,MessageUpdated,ConversationUpdated}`); o módulo `realtime` a
> implementa e o `app.go` injeta `omnichannel.WithPublisher(realtimeService)`. O **call-site
> monta o subconjunto** de cada evento (spec F5 "shapes por call-site", nunca unificar). Em F5 o
> único produtor é o **webhook inbound**: `service_inbound.go` publica `message.created` com o id
> INTERNO persistido (por isso `PersistInbound`/`writeInboundMessage` devolvem `inboundResult`
> com `ConversationID`/`MessageID`) — `realtime.go` monta o payload e **sanitiza mídia data: →
> ausente** (`sanitizeMediaURLForRealtime`; nunca base64 no WS; o publisher repete a checagem).
> Publica FORA da transação (persiste → commita → publica). `message.updated`/
> `conversation.updated` do webhook ficam para a F6. Detalhe do transporte + autorização (404 p/
> não-membro, 403 p/ permissão faltando) em `back/internal/modules/realtime/AGENT.md`.

> **F6 — envio via outbox + mídia (`http_send.go`, `service_outbound.go`, `outbound_handler.go`,
> `service_media.go`, `store_outbound.go`, `media_storage.go`, `ssrf.go`).** `POST
> /conversations/{id}/messages` cria a mensagem `PENDING`/`OUTBOUND`, grava mídia em **disco
> (raiz privada, fora de `UPLOADS_DIR`)**, **enfileira em `platform/jobs`** (`ordering_key =
> conversation_id`, FIFO por conversa) e publica `message.created`. O **worker da F3** chama o
> `OutboundHandler` → `channel.Provider.SendMessage` → `SENT`/`FAILED` + `message.updated`
> (mínimo). `GET .../messages/{mid}/media` faz **stream com `Range`** (`http.ServeContent`, sem
> `io.ReadAll`), exclui `hidden_messages` (→404) e rehidrata mídia inbound sob demanda. **Não
> reimplementa fila** (claim/retry/dead-letter/monitor são da F3). **Wiring pendente** (o
> orquestrador liga no `module.go`/`app.go`): ver §Wiring pendente (F6).

### E3 multimodal (migrations 0219 e 0234)

- `ai_agent_versions.media_config` é versionado e validado pelo service; aceita somente as
  seções audio/image/video/document, limites, provider/model e `credentialId`. Segredo cru,
  token e senha são rejeitados. `response_credential_id` fixa a chave usada para responder.
- `messaging.ai_credentials` é o cofre nomeado account-scoped. Segredos são cifrados pelo Go,
  nunca retornam ao navegador e podem ser reutilizados por múltiplos agentes. O keyring legado
  por agente permanece apenas para rollback e para a importação explícita/idempotente.
- `messaging.media_analyses` é a fonte autoritativa de status, resultado limitado, tentativas,
  tokens/custo e retenção. O unique tenant-scoped impede cobrança duplicada; FKs compostas
  impedem associar mensagem/conversa/versão de outra conta.
- `GET /conversations/{cid}/messages/{mid}/media/analyses` retorna somente metadados derivados
  ao operador autorizado. O gateway interno `GET /v1/runtime/omnichannel/media/{messageId}`
  exige token cifrado curto `media-stream.v1`, valida account+analysis+message e nunca expõe
  storage key. O `media.fetch` chama somente o webhook `omnichannel-brain-media`; o workflow
  interpreta áudio/imagem/vídeo/documento e devolve JSON, mas somente o Go valida/persiste e
  continua o dispatch/outbox. Nenhum workflow da Automação foi alterado.
- No workflow multimodal, o sandbox do Code node não oferece o construtor global `URL`: a
  validação do `mediaUrl` usa parser restrito e aceita HTTP somente para hosts internos. Os
  payloads OpenAI/Gemini são montados em `Preparar midia para IA`; os HTTP nodes recebem apenas
  `JSON.stringify($json.providerBody)`. Não remontar objetos multimodais complexos na expressão
  `jsonBody`, pois o parser do n8n rejeita a sintaxe antes de chamar o provider.

O bloqueio de IA por contato (`PUT /v1/omnichannel/conversations/{id}/ai-restriction`) é
account-scoped e fail-closed: ao bloquear, incrementa a geração e cancela mensagens/outbox/
dispatches AI pendentes no mesmo commit; `allow` apenas remove a restrição persistida.

### Memória e nome confiável do contato (migration 0236)

- O nome exibido pelo canal continua preservado para o operador, mas somente
  `safePreferredPersonalName` pode liberar um nome à IA para saudação. Frases como
  `Deus é fiel`, empresas, handles e telefones resultam em saudação sem nome.
- `messaging.contact_intelligence` é a memória estruturada account-scoped do CRM: resumo,
  fatos, preferências, intenção/sentimento/confiança e contadores operacionais.
- O resultado do modelo é não confiável. O Go remove chaves sensíveis, objetos aninhados e
  excesso de tamanho antes do upsert; histórico bruto, prompt e segredo nunca entram na tabela.
- A atualização da conversa e da inteligência compartilha a mesma validação de
  `state='ai_active' + ai_generation`; resultado atrasado não aprende nem responde.
- A memória volta ao próximo prompt como contexto não confiável. Ela nunca prevalece sobre
  prompt administrativo, regras, CRM manual ou contexto estratégico do cliente atendido.

### E6 tools e conhecimento (migrations 0222–0225)

- `messaging.ai_tool_bindings` é configuração por agente/conta; nasce desabilitado, sem credencial,
  e aceita apenas `read`, `propose_write` ou `approved_write`. As rotas de configuração são
  `/agents/{id}/tool-bindings` e exigem `omnichannel.agents.manage`.
- `POST /v1/internal/omnichannel/ai/tool-calls` é server-to-server: só aceita o token cifrado curto
  do brain, timestamp/HMAC, `dispatch_id`/`generation` correspondentes e binding do agente da
  versão despachada. Não usa JWT nem account vindo do body. `call_id` é único por dispatch; retry
  devolve o mesmo resultado e nunca repete um handler.
- `POST /v1/internal/omnichannel/ai/tool-call-signatures` é a única rota que assina chamadas do
  orquestrador n8n. Ela valida o binding habilitado, operação e schema sob o dispatch antes de
  devolver a assinatura HMAC; nunca devolve a chave do provider. O workflow próprio usa essa rota
  e depois chama `tool-calls`, sem credencial estática n8n.
- O registry é explícito e injetável (`WithAIToolRegistry`). Sem adaptador registrado, a chamada é
  negada/auditada; não existe fallback para SQL, HTTP, credencial ou URL escolhida pelo modelo.
  Modo de escrita sempre retorna `approvalRequired` até haver aprovação humana persistida.
- Argumentos/output passam por limites, schema estrutural, masking e timeout; auditoria usa somente
  IDs, operação, status e código. Eventos `AI_TOOL_REQUESTED|COMPLETED|DENIED|FAILED|TIMEOUT` e a
  trilha `AI_TOOL_APPROVAL_REQUESTED|AI_TOOL_APPROVED|AI_TOOL_REJECTED` são aceitos pelas migrations 0223/0224/0225.
  Propostas mutáveis ficam em `messaging.ai_tool_approvals`, com argumentos cifrados no Go; o painel
  recebe somente a projeção mascarada. As rotas de evidência são `/agents/{id}/tool-runs` e
  `/agents/{id}/tool-approvals`; aprovar/rejeitar exige `omnichannel.agents.manage` e nunca executa
  provider diretamente.
- Knowledge bases, documents, chunks e `ai_knowledge_bindings` vivem em `messaging.*`. Publicar
  documento exige chunks; FTS retorna somente evidências de documentos publicados e bases/bindings
  habilitados, com `topK`/`minScore` escopados por agente/conta. A base manual e o loop n8n assinado
  estão fechados localmente na E6; importação/ativação no runtime continua uma operação de rollout
  separada. O registry default possui apenas `knowledge.search`, que consulta o PostgreSQL no
  escopo account+agent e devolve evidências limitadas; integrações corporativas sem contrato estável
  precisam ser registradas explicitamente e binding sem handler falha fechado.
- As credenciais de IA do agente usam um keyring cifrado e versionado
  (`ai-provider-keys.v1`) dentro de `ai_agents.provider_key_ciphertext`, com slots independentes
  para `gemini`, `glm` e `openai`. O backend aceita o formato legado de chave única somente para
  leitura/migração transparente; nenhuma chave crua sai da API.
- `GET /agents/{id}/provider-keys` devolve somente `{set,last4}` por provider. `PUT` e `DELETE`
  em `/agents/{id}/provider-keys/{provider}` trocam ou limpam exclusivamente o slot solicitado,
  preservando as outras chaves e respeitando a allowlist fechada de providers.
- `GET /agents/{id}/models?provider=` alimenta o select do painel com a chave do slot solicitado.
  A chave é decifrada somente durante a chamada server-side, o provider usa uma allowlist de
  endpoints canônicos e a resposta contém apenas IDs de modelos de chat.

## Fronteira — o módulo é independente

Não lê, não escreve e não conhece o schema, a API nem o runtime de **nenhum outro módulo
satélite**. Zero `automation.*` (a D3 do port está superada/fora de escopo). Todo dado dele
nasce e vive em `messaging.*`.

As únicas tabelas de fora que ele lê são as da **plataforma**, e por contrato explícito da
spec C4: `core.accounts` (id/slug/name da conta), `core.users` (responsável da instância,
membros), `core.account_modules.config` (limites), `core.platform_settings` (default dos
limites). Nunca duplicar esses dados em `messaging.*`.

## Schema (migration `0200_messaging_schema.sql`)

9 tabelas. As 8 do port vêm do Prisma do legado
(`whats-test/apps/atendimento-online-api/prisma/schema.prisma`), camelCase → snake_case e
`tenantId` → `account_id`. A 9ª (`outbox`) não vem do port.

| Tabela | Origem | Nota |
|---|---|---|
| `whatsapp_instances` | `WhatsAppInstance` | `UNIQUE(account_id, instance_name)`. `provider`/`provider_config`/`credentials_ciphertext` são **novas** (D-A) |
| `conversations` | `Conversation` | `UNIQUE(account_id, external_id, channel, instance_scope_key)`. `state`/`department_id`/`queue_id`/`assigned_user_id`/`extracted_fields` são **novas** |
| `messages` | `Message` | `(account_id, created_at)`, `(conversation_id, created_at)`. **F6 (0207)**: `media_storage_key` (path relativo à raiz privada, nunca no JSON) + `media_source_kind` (`disk`\|`url_encrypted`) |
| `contacts` | `Contact` | Desde 0211, telefone opcional e único quando preenchido; identidade multicanal fica em `contact_identities` |
| `saved_stickers` | `SavedSticker` | poda FIFO 200/conta = regra de service (F12) |
| `audit_events` | `AuditEvent` | |
| `hidden_messages` | `HiddenMessageForUser` | "apagar para mim"; `UNIQUE(user_id, message_id)` |
| `account_config` | `AtendimentoTenantConfig` | `retention_days` 15 / `max_upload_mb` 500 (defaults do legado) |
| `outbox` | **nova** | envio durável. Contrato: `specs/OMNI-F3.md` §F3.2 |

`account_id uuid not null references core.accounts(id) on delete cascade` em **todas**.

### As três armadilhas do schema (não “consertar” sem ler)

1. **`conversations.status` NÃO é coluna.** `state` é a verdade; `status` é **projeção
   derivada na serialização** (canônico §7.3). O Prisma tem `status`, e ele **não é
   portado** — coluna + projeção = duas verdades. Ver `projectStatus` em `model.go`.
2. **`state` já nasce com os 7 valores**, `pending` incluído (D-E, 2026-07-17). A **F8 não
   faz `ALTER`**. Quem escreve `pending` é a F8 (evento `human.pending`); a F2 só garante
   que a coluna aceita e projeta.
3. **`department_id`/`queue_id` são `uuid` SEM FK.** As tabelas alvo só existem na F8. Quem
   adicionar a constraint lá: `ADD CONSTRAINT IF NOT EXISTS` **não existe no Postgres** —
   precisa de `DO $$ ... pg_constraint ... $$` para seguir idempotente.

Migration é **SQL plano idempotente, sem `-- +goose Down`** (o migrator roda o arquivo
inteiro e o Down se auto-destrói — falha real em `0147_automation_contacts_fix.sql`).
**Migration nova exige `docker compose build --no-cache api`**: as migrations são `embed.FS`
e o cache da camada `go build` pode não re-embutir o `.sql` (sintoma: `migrate status` para
na anterior, **sem erro**).

### CRM multicanal e executor n8n (migration `0211`)

- Webhook inbound faz upsert de contato + `contact_identities` + `contact_touchpoints`
  na mesma transação da deduplicação/conversa/mensagem. Não mover essa escrita ao n8n.
- `contacts.phone` é nullable para Instagram; toda leitura legada projeta `coalesce(phone,'')`.
  Upsert por telefone precisa repetir o predicado do índice parcial no `ON CONFLICT`.
- `contact_notes` é a persistência canônica das notas humanas; o front não deve manter nota
  somente em estado reativo.
- `OMNI_AI_EXECUTOR=native|n8n`: provider/modelo/chave continuam no banco/painel. O modo `n8n`
  usa o contrato versionado brain.v2/brain.v3 e a rota Go `POST /v1/runtime/omnichannel/llm-gateway`,
  que aceita somente `brain.request.v2` e `brain.request.v3`; a chave é
  selada em token curto pelo `secretbox` e nunca aparece no payload/export do n8n. Sem
  `OMNI_N8N_INTERNAL_TOKEN` o boot permanece native. O workflow nunca toca canal, estado, fila ou
  outbox.
- `reply_draft` sai somente por `SendService.SendAIMessage` (mensagem PENDING + outbox).
  `needs_human=false` mantém `ai_active` para o próximo turno; `true` envia a transição e
  segue para routing. Falha de enqueue faz fail-open para humano.
- Contrato e roadmap: `docs/omnichannel/ARQUITETURA_HIBRIDA_N8N.md`.

### `outbox` — a tabela é daqui, o engine não

A F2 é dona do **DDL**. O **engine** (claim `FOR UPDATE SKIP LOCKED`, retry classificado,
worker, dead-letter, monitor de presas) é a **F3** (`platform/jobs`, sobre uma interface
`Store`). O **produtor** de job é a **F6**. Não implementar worker aqui.

`unique (account_id, idempotency_key)` — **nunca UNIQUE global** (D-G). A chave vem do
cliente: com UNIQUE global a conta A colide com a chave da conta B e **suprime o envio
dela**. Como o UNIQUE global saiu, **prefixar a chave com o `account_id` é desnecessário**.

## Rotas (F2 — só leitura)

Prefixo `/v1/omnichannel`, `RequireAuth` + gate de módulo no Chain (`moduleGatingRules` em
`app.go`). Mapa Node→Go completo: `PLANO_PORT_OMNICHANNEL.md` §8.

| Rota | Nota |
|---|---|
| `GET /conversations` | `instanceId?`; ordena `last_message_at DESC`; **sem paginação**; **array direto** |
| `GET /conversations/{id}/messages` | `limit` 1..200 (default 100) + `beforeId` |
| `GET /conversations/{cid}/messages/{mid}` | |
| `GET /contacts` · `POST /contacts` · `PATCH /contacts/{id}` | POST devolve `{contact, conversation}` |
| `GET /account` · `PATCH /account` | shape = `mapTenantResponse` do legado |
| `GET /tenant` | **alias de `GET /account`** — o front verbatim chama os dois; mesmo shape `TenantSettings`/`AccountSettingsView`. O inbox usa só `maxUploadMb`; a config usa o objeto inteiro |
| `GET /users` | membros ATIVOS da conta (picker de atribuição do inbox), shape `TenantUser[]`. Reusa `ListAssignableUsers` (`core.account_users`); `role` = valor neutro (GAP DECLARADO do RBAC custom, ver `GetInstanceManagement`) |
| `GET /tenant/whatsapp/instances` · `GET /tenant/whatsapp/instances/access` | filtro **corrigido** (ver A2) |
| `POST /conversations/{id}/messages` | **F6** envio. Body do legado + `idempotencyKey?`. TEXT exige `content` (≤4000); mídia exige `mediaUrl`. **200** enfileirado · **202** falhou ao enfileirar (mensagem `FAILED`) · **403** VIEWER · **413/415** upload · **422/403** anti-SSRF de `mediaUrl` http(s) |
| `GET /conversations/{cid}/messages/{mid}/media` | **F6** stream com `Range` (`http.ServeContent`). `disposition=inline\|attachment`, `download`. Exclui `hidden_messages`→404. `Cache-Control: private, max-age=60` + `nosniff` |

### Rotas de gestão de instância (escrita) — `http_instances.go`

`RequireAuthWithAccount` + gate de módulo + **só admin** (checado no service). `account_id`
do Principal, nunca do body. Erros por `writeSessionError` (409 `number_in_use` /
`instance_name_conflict` / `channel_limit`; 404 fora de escopo; 403 `forbidden`).

| Rota | Nota |
|---|---|
| `POST /tenant/whatsapp/instances` | cria; body = formulário do front (`instanceName`, `displayName?`, `phoneNumber?`, `evolutionApiKey?`, `queueLabel?`, `userScopePolicy`, `responsibleUserId?`, `isDefault`, `isActive`). Provider default `evolution`. Devolve `WhatsAppInstanceRecord`. **201** |
| `PATCH /tenant/whatsapp/instances/{id}` | atualiza; full-replace do formulário. `evolutionApiKey` é **só-se-presente** (ausente = mantém a credencial). `isDefault=true` → `PromoteDefault`. **200** |
| `PUT /tenant/whatsapp/limits` | body `{ maxChannels }`, faixa 1–100. **Somente `platform_admin`**; grava `core.account_modules.config.max_whatsapp_numbers`, preserva as outras chaves e rejeita teto abaixo do total ativo. **200** |
| `GET /tenant/whatsapp/instances/{id}/capabilities` | resolve o provider da instância por id → `registry.Get(provider).Capabilities()`. Devolve `OmniCapabilities` (camelCase: `supportsTemplates`/`requires24hWindow`/`supportsReaction`/`supportsSticker`/`supportsGroups`/`maxMediaBytes`). Metadado do provider (não exige admin). Sem esta rota o front mostra "capacidades DEGRADADAS" por número. **200** |
| `DELETE /tenant/whatsapp/instances/{id}` | remove a instância. **BLOQUEIA com 409 `instance_has_conversations`** se houver conversas atreladas (o front usa "Desativar"=PATCH `isActive:false` no caso comum; delete duro só sem histórico). Só admin; fora de escopo → 404. **204** |
| `PUT /tenant/whatsapp/instances/{id}/users` | body = `{ userIds: string[] }` (o front manda `userIds`, não `assignedUserIds`). Filtra p/ membros ativos da conta. **200** |
| `POST /tenant/whatsapp/validate-endpoints` | body `{ instanceId?, instanceName? }`; valida **config** (não faz probe vivo — ver gap 7). **200** |
| `POST /tenant/whatsapp/conversations/clear` | body `{ instanceId? }`; `instanceId` ausente = escopo **tenant** (conta toda), presente = **instance**. Apaga audit→messages→conversations numa tx e devolve as contagens. **200** |

Semânticas duras: `phoneNumber` passa pelo `number_guard` (um número, uma instância);
`instance_name` colide no índice único → 409 `instance_name_conflict`; `responsibleUserId`
e `assignedUserIds` são validados contra `core.account_users` (isolamento — usuário de
outra conta nunca entra). Reativar uma instância também consulta o `LimitReader`; não é possível
contornar `max_whatsapp_numbers` criando inativa e ativando depois.

**Sessão/QR aceitam `instanceId` (2026-07-18):** `GET /tenant/whatsapp/status` e `.../qrcode`
resolvem por `instanceId` (o id que o inbox verbatim manda no query) OU `instanceName`, senão a
instância **default** da conta (`Store.GetSessionInstanceByRef`, mesma regra do `ResolveInstanceForOps`).
A `InstanceView` agora expõe `provider`. Detalhe em docs/omnichannel/ESTADO.md §0.2.

**Webhooks e runtime ficam FORA do gate** quando chegarem (F4): precedente confirmado —
`/v1/public/*` e `/s/{slug}` não estão em `moduleGatingRules`.

### Rotas F12 — figurinhas · GIF · avatar (2026-07-18)
Código novo (house-standard), spec `docs/omnichannel/specs/OMNI-F12.md`. Front do GIF/sticker já usa `apiFetch` (repontado na F1); o avatar foi repontado p/ a rota pública. Migration **0210** (`saved_stickers.storage_key` + `data_url` nullable).

| Rota | Nota |
|---|---|
| `GET\|POST\|DELETE /stickers` | Figurinhas por conta. `dataUrl` na resposta é **base64 REAL** relido do disco (o front DESCARTA em silêncio o que não começa com `data:image/`). Bytes em disco (`media_storage` da F6, `{acct}/stickers/{rand}.{ext}`), coluna `storage_key`; poda **FIFO >200** (linha+arquivo). Teto **~1MB** decodificado (sniff de mime→415; acima→413). Cache `private`. Perms: GET=`conversations.view`, POST/DELETE=`conversations.reply`. Fora de escopo→404 |
| `GET /gif/search?q=&limit=` | Tenor v2. Erro é **SOFT (200 + `error`)**, nunca 4xx/5xx (senão apaga a msg acionável no `catch` do front). `limit` 1..40 default 24; `q` vazio→`items:[]` sem chamar o Tenor. Perm `conversations.reply` |
| `GET /gif/media?url=` | Proxy do GIF: allowlist 4 hosts Tenor + anti-SSRF F6 (no-redirect) + stream, cache 1h. 400 host/URL, 502 upstream |
| `GET\|PUT /gif/settings` | Chave do Tenor em `core.platform_settings['omnichannel_gif']`, **CIFRADA** (secretbox `v1:` — não repete o gap do calendar que grava cru). Só `platform_admin`; saída `{set,last4}`. É **daqui** que a chave entra (princípio 1: nunca de env) |
| `GET /v1/public/omnichannel/avatar?url=` | Proxy do avatar WhatsApp **PÚBLICO** (`http_avatar.go`, C4 opção A): o front põe a URL no `<img src>`, que não manda token → fora do gate. Allowlist dos 4 hosts WA + anti-SSRF F6 + **no-redirect** + rate-limit por IP. 403 host, 422 esquema, 400 sem url, 204 falha/vazio (cai nas iniciais) |

Costura no `module.go`: `NewGifService(store, secretBox)` + `NewStickerService(store, media, logger)` no `Build()`; `RegisterGifRoutes`/`RegisterStickerRoutes` (gated) + `registerAvatarRoutes` (público) no `RegisterRoutes`.

### Contratos do front que não se pode divergir

O contrato é `web-reference/app/types/index.ts`. **Divergir um campo quebra o front.**

- **Paginação é `limit` + `beforeId`, NÃO cursor.** Resolve `beforeId → created_at`, filtra
  `created_at <`, ordena `DESC`, `take limit`, **inverte o array** (devolve ASC). `hasMore` =
  existe mensagem mais antiga que a **primeira** da página. Divergir quebra o scroll infinito.
- **`Message` e `Contact` têm `tenantId` OBRIGATÓRIO**; `Conversation` **não tem**. Como
  mapeamos `tenantId → account_id`, esses dois serializam **`account_id` de volta como
  `tenantId`**.
- **`MessageStatus` = só `PENDING|SENT|FAILED`.** Não existe `DELIVERED`/`READ` no legado
  (não há tracking de ACK). Se um dia quisermos, é **feature nova**, não port.
- **`ConversationStatus` = `OPEN|PENDING|CLOSED`** — os 3, e o front renderiza o caso
  `PENDING`. O serializador nunca emite string fora dessa lista.
- **`POST /contacts` devolve `{contact, conversation}`**, não o contato pelado
  (`useOmnichannelInboxContactActions.ts:41-56`). `conversation` só vem com `conversationId`
  no body; `null` faz o front recarregar a lista.
- `instanceScopeKey` = o **`instance_name`**, não o id. É a chave real de particionamento.

## Isolamento multi-tenant (defesa em profundidade)

- `account_id` **sempre** do Principal (`accountScope` em `http.go`: `X-Account-Id` → fallback
  `principal.TenantID`; vazio → 403 `no_account`). **Nunca do body** — nem em `POST /contacts`,
  nem em `PATCH`.
- O **repositório filtra por conta TAMBÉM**, em toda query, inclusive nas que já receberam id
  validado no service (`where account_id = $1::uuid`).
- **Fora de escopo → 404, nunca 403.** `translate()` no `service.go` mapeia `pgx.ErrNoRows`
  (que cobre "existe, mas é de outra conta") para `ErrNotFound`. 403 confirmaria que o
  recurso existe (enumeration).
- Conta sem o módulo → 403 `module_disabled` (gate no Chain); `platform_admin` tem bypass.

### A2 — o filtro de instância do legado está QUEBRADO; aqui é portado corrigido

`whats-test/.../services/whatsapp-instances.ts:681-683`:

```ts
const accessibleInstances = isTenantAdmin || activeInstances.length <= 1
    ? activeInstances
    : activeInstances;   // <- os dois ramos devolvem a MESMA coisa
```

Ou seja: **todo usuário vê todas as instâncias**. É isolamento (princípio 2), então o port
manda corrigir. Regra desta fase: não-admin vê `responsible_user_id is null or
responsible_user_id = <principal>` (o único vínculo por usuário que existe de fato). Sem
acesso a nenhuma instância ⇒ inbox **vazio** (fail-close), nunca o inbox inteiro.

**O gate de dado definitivo é `queue_members` e chega na F8** — não inventar um segundo gate.

**F7 fecha a correção (`store_postgres_scope.go`, `AccessibleScopeKeys`).** A F2 filtrava só
`responsible_user_id = <principal>`; a F7 reconstrói a tabela completa do C3, incluindo o guard
`<= 1` que o próprio código morto do legado tinha e que evita trancar todos fora numa conta com
um só número:

| Ator | Instâncias visíveis |
|---|---|
| admin da conta / `platform_admin` | todas as ativas (`unrestricted`) |
| qualquer usuário, conta com **≤ 1** instância ativa | a única ativa |
| demais | ativas com `responsible_user_id = <principal>` |
| resultado vazio | **vê nada** (fail-close), nunca "vê tudo" |

Toda rota da F7 resolve a conversa por **DOIS gates somados com `AND`** (`resolveConversation`
em `service_actions.go`): o de instância (F7, acima) **e** o de fila (F8,
`GetVisibleConversation`). O mais restritivo vence; unir com `OR` reabriria o furo. Fora de
qualquer um → **404**. **DIVERGE de propósito do legado** (registrado em `docs/LEGADO.md`).
*Pendência:* as rotas de **leitura** (F2, `service.go:accessibleScopeKeys`) ainda usam a versão
parcial — unificar com `AccessibleScopeKeys` num passo futuro (a F7 não edita `service.go`).

## Gaps honestos desta fase (não são bugs — são dívida declarada)

Registrados também em `docs/LEGADO.md`.

1. **Não existe middleware de permissão por key no Go.** O disco só tem `RequireAuth` /
   `RequireAuthWithAccount` / `RequireRoles` e o gate de módulo por path; `modules.Dependencies`
   **não expõe** serviço de permissões (`ResolveEffectivePermissions` vive no módulo `access`).
   As 9 keys `omnichannel.*` hoje gateiam o **front**; a F2 as **declara**. Enforcement por key
   vira load-bearing na **escrita** (F6/F7). **Não fingir que a key protege a rota agora.**
2. **`assignedUserIds` e `userScopePolicy` — agora PERSISTEM** (deixaram de ser constantes
   fixas). O front tem controles reais para os dois (seletor de política + `PUT .../users`);
   mantê-los fixos deixaria esses controles mortos (o save reverteria na releitura). Ficam em
   `whatsapp_instances.provider_config` (jsonb, chaves `userScopePolicy`/`assignedUserIds`) —
   **coluna existente, NÃO tabela nova** (armadilha A3 respeitada; o merge `|| jsonb_build_object`
   preserva as outras chaves, ex.: `baseURL`). `scanInstance` lê via `parseInstanceScope` com
   fallback nos defaults do legado. O `/access` (operador) segue emitindo `assignedUserIds: []`.
3. **`users[].role` e `atendimentoAccess`** (array do `WhatsAppInstanceManagementResponse`)
   saem com valor neutro (`AGENT`/`true`). Os papéis do Omni são **custom por conta**
   (`core.roles` + `core.user_role_assignments` + overrides) e quem os resolve é o `access` —
   refazer esse JOIN aqui criaria uma **segunda verdade de RBAC**. Fonte real = F10.
4. **`webhookUrl` sai vazio** — a URL vem de `WEBHOOK_RECEIVER_BASE_URL`, env da **F4**
   (canônico §13). Inventar uma URL aqui seria mentir para o painel.
5. **`credentials_ciphertext` nasce e fica VAZIA.** Quem cifra é `platform/secretbox` (**F3**,
   prefixo `v1:`). **Nunca gravar chave crua nela** — repetiria exatamente o gap do
   `calendar/secrets.go` (grava a chave em texto puro; `{set,last4}` é máscara de **saída**,
   não cifragem), que é o gap que este plano existe para não repetir.
6. **`assigned_user_id` (novo, → `core.users`) coexiste com `assigned_to_id`** (texto, do
   Prisma, servido ao front com esse nome). **Não fundir**: o front verbatim lê `assignedToId`.
   **RESOLVIDO na F7:** a FSM (`ApplyTransition`) grava só `assigned_user_id`; a F7
   (`SyncAssignedToID` em `store_postgres_scope.go`) espelha esse valor em `assigned_to_id`
   após `assign`/`unassign` — o inbox mostra o responsável. Não é escrita de `state`/`status`
   (risco 4): `assigned_to_id` é projeção de exibição, não ciclo de vida.
7. **`validate-endpoints` valida CONFIG, não faz probe vivo** (`service_instance_ops.go`). Não
   há método de "ping" na `channel.SessionManager`/`Provider` e a única rota de envio
   (`SendMessage`) é F6 — probá-la de verdade enviaria mensagens. Então a validação é da
   **configuração**: `provider`, `baseURL` (`provider_config.baseURL` → `EVOLUTION_BASE_URL`) e
   presença de credencial (`credentials_ciphertext` → `EVOLUTION_API_KEY`). Status honesto por
   endpoint (`missing_route`/`auth_error`/`provider_error`/`ok`), `httpStatus: null`, e a
   mensagem do caso `ok` **não afirma** conectividade viva (é validada no envio). Probe real =
   fase de envio (F6).

## Arquivos

| Arquivo | Papel |
|---|---|
| `module.go` | catálogo (9 permissões + 3 role templates), `Build`, `handle` |
| `model.go` | views do front, projeção `state → status`, erros do domínio |
| `http.go` | rotas, `accountScope`/`callerFrom`, mapa de erros |
| `service.go` | conversas e mensagens (paginação, escopo A2) |
| `service_contacts.go` | contatos (CRUD do inbox) + helpers de telefone/nome |
| `service_admin.go` | conta e instâncias (+ mapa `legacyRole`) |
| `store_postgres.go` | conversas e mensagens |
| `store_postgres_admin.go` | contatos, config, conta e instâncias |
| `channel/provider.go` | **F4** interface `Provider` (5 métodos) + `SessionManager` + eventos canônicos + `Capabilities` + `Credentials` |
| `channel/registry.go` | **F4** registry de providers por chave (`Get`/`Session`/`Has`) |
| `channel/mock/mock.go` | **F4** adapter `mock` (sem rede: QR falso, conecta na hora, ecoa envio) |
| `service_inbound.go` | **F4** webhook: resolve conta, verify, parse, mascara, dedupe+persiste; **F5** publica `message.created` pós-commit |
| `store_webhook_events.go` | **F4** dedupe+domínio na MESMA tx; resolve conta por slug; instância por nome; **F5** devolve `inboundResult` (ids internos) |
| `http_webhook.go` | **F4** rota PÚBLICA `/v1/webhooks/omnichannel/{provider}/{accountSlug}` + proteções |
| `rate_limit.go` | **F4** limitador em memória por `provider:slug:ip` (rota pública) |
| `service_session.go` | **F4** ciclo de sessão/QR + grava credencial cifrada (secretbox) |
| `http_session.go` | **F4** rotas do painel `/v1/omnichannel/whatsapp/session/*` + credenciais |
| `number_guard.go` | **F4** "um número, uma instância" INTERNO (só a própria conta) |
| `qr_cache.go` | **F4** cache de QR em memória (TTL 120s; não sobrevive a restart). **Compartilhado** entre `SessionService` (connect síncrono + `/qrcode`) e `InboundService` (QR via webhook `QRCODE_UPDATED`) |
| `publisher.go` | **F5** interface `Publisher` (`PublishOmnichannelEvent`) + `RealtimeEvent` + 3 literais + `noopPublisher` |
| `realtime.go` | **F5** `sanitizeMediaURLForRealtime` (data: → "") + `publishInboundMessage` (monta `message.created` do webhook) |
| `http_send.go` | **F6** `RegisterSendRoutes` + `POST /messages` + `GET /media`; `writeSendError` (413/415/422/403/404); `callerCanReply` (VIEWER→403) |
| `service_outbound.go` | **F6** `SendService`: escopo→reply→upload→cria PENDING→enfileira→`message.created`→audita; `Enqueuer` (fatia do outbox); `OutboundJobKind` |
| `outbound_handler.go` | **F6** `OutboundHandler` (jobs.Handler): resolve provider/credencial→`SendMessage`→`SENT`/`FAILED`→`message.updated`; `isTerminalJobError` espelha o engine |
| `service_media.go` | **F6** `MediaService`: escopo + `hidden`→404 + rehidratação one-shot; abre o arquivo p/ `ServeContent` |
| `store_outbound.go` | **F6** cria OUTBOUND + colunas de mídia + `last_message_at`; `Mark{Sent,Failed}`; descriptor da mídia; `InsertAudit`; `GetMaxUploadBytes`; leitura do outbox p/ idempotência |
| `media_storage.go` | **F6** `DiskMediaStorage` (raiz **privada**): `Save` (data URL/http streaming), `SaveReader` (rehidratação), `Open` (containment); allowlist de mime |
| `ssrf.go` | **F6** guarda anti-SSRF reutilizável (F12 reusa): IP resolvido no `Control` do dialer, sem redirect; `validatePublicURL` (422 scheme / 403 host interno) |
| `http_actions.go` | **F7** `RegisterActionRoutes` (11 rotas) + handlers + `writeActionError` (409 acionável); `actionScope` (accountID+Principal) |
| `service_actions.go` | **F7** `ActionsService`: status/assign via FSM, open-conversation, `resolveConversation` (gate instância AND fila), realtime `conversation.updated`, auditoria |
| `service_actions_messages.go` | **F7** reaction/forward/delete-for-me/delete-for-all; `forward` reusa o `SendService` (outbox F6) |
| `store_postgres_scope.go` | **F7** `AccessibleScopeKeys` (escopo de instância corrigido, C3) + `SyncAssignedToID` (reconcilia a coluna do front) |
| `store_postgres_actions.go` | **F7** `HideMessages` (delete-for-me), `ListActionMessages`, `ConversationChannelTarget`, `Find/CreateContactConversation` |

## F7 — ações do inbox

- **Status/assign SEMPRE via `Service.Transition` (F8)** — nunca `update ... set state/status`
  (risco 4 do canônico). `SetStatus` projeta o status do front no evento (`statusEvent`,
  Contrato 3): `CLOSED`→`conv.close`, `PENDING`→`human.pending` (D-E), `OPEN` numa conversa
  `closed`→`conv.reopen`, `OPEN` já aberta→**no-op 200**. `Assign` mapeia `assignedToId` com
  valor→`human.assign` (⇒ `human_active` ⇒ hard-block da IA), `null`→`human.unassign`. Destino
  não-atribuível/fora da conta → **404** (guarda da nota 8 na F8).
- `conv.close` também fecha qualquer handoff aberto no mesmo `ApplyTransition`; isso evita card
  de intervenção órfão e faz o reset operacional ser atômico. Fechar uma conversa já fechada é
  idempotente e ainda reconcilia handoffs antigos que tenham ficado abertos.
- **Reconciliação `assigned_to_id`** após assign/unassign (a FSM só grava `assigned_user_id`;
  ver gap #6). **`conversation.updated` COM `instanceName`** (a `ConversationView` já o carrega);
  `mediaUrl` `data:` do preview → `null`. Auditoria **só quando muda** (`before != after`):
  `CONVERSATION_STATUS_CHANGED` / `CONVERSATION_ASSIGNED` com `{before, after, changedBy}`.
- **Mensagens**: `delete-for-me` grava em `hidden_messages` (ocultação por usuário; a mensagem
  **fica** em `messaging.messages`) → `{deletedIds, skippedIds, conversation}`. `forward` (1..100)
  reusa o **outbox da F6** (uma `SendMessage` por mensagem no destino) — valida origem **e**
  destino no escopo (404 cada); `queuedCount`/`failedToQueueCount` vêm do desfecho do enqueue.
- **Escopo**: toda rota resolve por `resolveConversation` = instância (F7) **AND** fila (F8) →
  404 fora do escopo. `account_id` sempre do Principal.
- **Auditoria estendida** (migration `0208`): o `CHECK` de `messaging.audit_events` ganha
  `MESSAGE_FORWARDED` e `MESSAGE_DELETED_FOR_ALL` (`delete-for-me` não audita — sem efeito externo).
- **`reaction` / `delete-for-all` LIGADAS no provider (F4 estendida)**: `channel.Provider` ganhou
  `SendReaction`/`DeleteForAll` (+ `ReactionInput`/`DeleteInput`), implementados no `evolution`
  (`POST /message/sendReaction/{i}`; `DELETE /chat/deleteMessageForEveryone/{i}` — v2 confirmada
  no legado `EVOLUTION_DELETE_FOR_ALL_PATH`) e no `mock` (no-op). `reaction` gateia por
  `Capabilities().SupportsReaction` (número sem suporte → 409), exige `external_message_id` (→ 409)
  e mapeia falha de transporte/HTTP do provider → **502** (`provider_unavailable`). `delete-for-all`
  executa por-id: sucesso → `updatedIds`, falha do provider → `failedIds` (**200**, nunca 502 no
  multi-id — contrato do legado). `remoteJid` = `conversations.external_id` (novo campo em
  `ConversationChannelTarget`); `fromMe` vem da `direction`. Credencial resolvida por instância
  (`FindProviderCredential` + `secretBox`) — ver Wiring item 3.
- **Gap da F4 remanescente (honesto, `docs/LEGADO.md`)**: `group-participants`, `sync-open`,
  `sync-history` e `import-whatsapp` seguem **409 acionável** (`provider_action_unavailable`) até a
  F4 expor os métodos correspondentes — **nunca sucesso fingido**.

### Wiring pendente (F7) — a fazer no `module.go` (a F7 não o edita)

1. **Service** (no `Build`, onde `store`/`registry`/`m.publisher`/`deps.Logger` existem, após o
   `send`): `actions := NewActionsService(store, m.handle.service, m.handle.send, registry, m.publisher, deps.Logger)`.
   Guardar em `handle` (novo campo `actions *ActionsService`).
2. **Rotas** (no `handle.RegisterRoutes`): `RegisterActionRoutes(mux, h.actions, h.authMiddleware)`.
3. **F4 reaction/delete-for-all — FEITO**: `channel.Provider` estendida (`SendReaction`/`DeleteForAll`
   + `ReactionInput`/`DeleteInput`), implementada em `evolution`+`mock`; os pontos
   `ErrProviderActionUnavailable` de `reaction`/`delete-for-all` viraram a chamada real (502 na
   falha de transporte da reaction; `failedIds` por-id no delete). **Pendente de wiring no
   `module.go`**: passar `omnichannel.WithActionsSecretBox(m.secretBox)` como argumento final de
   `NewActionsService` para as ações síncronas decifrarem a credencial **por instância** (sem isso,
   caem em `provider_config` + fallback de ambiente `EVOLUTION_*` do adapter). As demais sync ops
   (group/sync/import) seguem 409 até a F4 expor os métodos.

- **Nomes de grupos**: a UI identifica grupos exclusivamente pelo JID `@g.us` e nunca deriva
  rótulos a partir dos últimos dígitos. O adapter Evolution implementa a capacidade opcional
  `channel.GroupMetadataProvider`, consulta o `subject` oficial em `group/findGroupInfos` e o
  ingest persiste esse nome em `conversations.contact_name` de forma best-effort. Falha de
  metadata não interrompe o webhook; o front exibe `Grupo sem nome` até o provedor informar o
  nome real. `pushName` de participante nunca é usado como nome de grupo.
4. **Migration `0208`** exige `docker compose build --no-cache api` (embed.FS não re-embute com cache).

## F4 — canal, webhook e sessão

- **Camada tradutora**: domínio e front só veem o shape canônico de `channel` (`Event`,
  `InboundMessage`, ...). Trocar de provider = 1 adapter novo; zero mudança no domínio.
- **Registry**: `channel.NewRegistry(mock.New(), evolution.New(EVOLUTION_BASE_URL, EVOLUTION_API_KEY, deps.Logger))`.
  `provider` da instância (`whatsapp_instances.provider`) resolve o adapter. Chave sem adapter → erro claro.
  O `logger` (variádico, opcional) só é usado para logar falha de `setWebhook` em Warn (sem vazar token).
- **Adapter `evolution`** (`channel/evolution/{client,adapter,parse}.go`): 1º adapter REAL (D-A),
  integração WHATSAPP-BAILEYS, header `apikey`, timeout 30s. Implementa `channel.Provider` +
  `channel.SessionManager`. `baseURL`/`apiKey` vêm da **instância** (`provider_config.baseURL` +
  `credentials_ciphertext`); `EVOLUTION_BASE_URL`/`EVOLUTION_API_KEY` são só **fallback de ambiente**.
  Em produção, `docker-compose.prod.yml` mantém `evolution` + `evolution-db` isolados no profile
  `omnichannel`, com volumes próprios e bind de troubleshooting apenas em `127.0.0.1`.
  `VerifyWebhook` compara `x-webhook-token` (fallback `apikey`) constant-time (`hmac.Equal`), fail-closed.
  `ParseWebhook` é defensivo: event de `event ?? type ?? data.event` normalizado; `MESSAGES_UPSERT`→
  message_received (pula `fromMe`), `MESSAGES_UPDATE`(ack)→message_status, `QRCODE_UPDATED`→qr_updated
  (extrai `data.qrcode.base64` OU `data.base64`), `CONNECTION_UPDATE`→session_status, resto→ignored.
  `ExternalEventID` = `{instância}:{tipo}:{id}` (mata colisão entre contas — armadilha 2). Mídia inbound:
  só mimetype/fileName/caption entram no domínio; o binário é reidratado por `DownloadMedia` (F6).
  Nunca loga a apiKey nem o body cru.
- **Auto-set do webhook** (`connect`): o adapter (re)configura o webhook da instância embutindo
  `headers:{apikey:<token>}` — o MESMO valor que o `VerifyWebhook` espera (`c.apiKey == expectedToken(cred)`),
  senão a Evolution não devolve token e todo inbound volta 401. O `webhookConfig` monta o OBJETO de config
  (sem envelope): o `create` embute-o direto em `body.webhook` (o envelope duplo `body.webhook.webhook.url`
  devolve 400 "Invalid url"); o `/webhook/set` o envolve em `{"webhook":{...}}`. A `webhookUrl` chega ao
  adapter via `cred.Config["webhookUrl"]`, montada no `credentialsFor` a partir de `WEBHOOK_RECEIVER_BASE_URL`
  + slug da conta. Falha de `setWebhook` é logada em Warn (não aborta o connect; url logável, token não).
- **Webhook** `POST /v1/webhooks/omnichannel/{provider}/{accountSlug}` — **público, FORA do gate**
  (não está sob `/v1/omnichannel`). Proteções na ordem: rate-limit `provider:slug:ip` → 429;
  content-type → 415; content-length (`MaxBytesReader`) → 413; **resolve conta por slug** → 404;
  `VerifyWebhook` (constant-time no adapter) → 401; `ParseWebhook` + **dedupe por
  `webhook_events UNIQUE(provider, external_event_id)`** na MESMA tx da escrita de domínio →
  202 `{status: accepted|duplicate|ignored}`. **Deviação documentada**: a conta resolve ANTES do
  verify (credencial é por instância — D-A; sem conta não há o que comparar; mesmo padrão do
  `site/http_ingest.go`). Instância desconhecida → **ignored** (nunca auto-cria — armadilha 1).
- **`account_id` do webhook** vem SEMPRE do `{accountSlug}` do path, resolvido no server; slug
  inexistente / conta inativa / módulo desabilitado → **404** (nunca 403).
- **Sessão** (`/v1/omnichannel/whatsapp/session/*`, `RequireAuthWithAccount`, só admin):
  `bootstrap` (limite de canais via `platform/modules.LimitReader` → 409; cria instância; promove
  default), `connect`, `status`, `qrcode`, `logout`. QR em cache de **memória** (sem Redis).
- **Estado de conexão autoritativo**: `connected` e `connectionState.instance.state` vêm
  exclusivamente do `SessionState` consultado no provider. `phone_number` é cadastro/identificador
  e nunca prova pareamento; preencher telefone manualmente não pode esconder o botão ou o QR.
- **QR via webhook**: a Evolution empurra o QR async por `QRCODE_UPDATED` (o `/instance/connect` nem
  sempre traz base64). O `qrCache` é **compartilhado** (criado no `module.go`, injetado no `SessionService`
  E no `InboundService`): o `SessionService` grava o QR síncrono do connect e lê no `/qrcode`; o
  `InboundService.ingestOne` grava o QR do webhook e limpa no `CONNECTION_UPDATE state=open`. Sem o cache
  compartilhado o QR do webhook nunca chegaria ao endpoint que o painel lê.
- **Credenciais**: gravadas cifradas (`credentials_ciphertext` via `platform/secretbox`), lidas
  decifradas só no service, expostas ao painel só como `{set,last4}`. Chave crua nunca em log/front.
- **Um número, uma instância**: validação INTERNA (`number_guard.go`) + índice único parcial
  `(account_id, phone_number)` (migration `0201`). Não consulta nenhum outro módulo.
- **Wiring**: `app.go` faz fail-fast do `OMNI_SECRETS_KEY` (`secretbox.FromEnv`) e injeta o Box
  no módulo (`omnichannel.New(omnichannel.WithSecretBox(box))`). Sem a env a api não sobe.

## F6 — envio via outbox e mídia

- **Produtor, não fila.** A tabela `messaging.outbox` é da **F2** e o engine (claim `FOR UPDATE
  SKIP LOCKED`, retry classificado, dead-letter, monitor de presas) é da **F3**
  (`platform/jobs`). A F6 só **produz** job (`Enqueuer.Enqueue`) e é dona do **handler de envio**.
- **FIFO por conversa**: `ordering_key = conversation_id` (o engine garante a ordem; não burlar).
- **Idempotência escopada por conta**: `idempotency_key` vai **cru** — o `unique (account_id,
  idempotency_key)` do outbox é o mecanismo (não prefixar com `account_id`). Chave do cliente:
  pré-checagem antes de gravar mídia/mensagem (reenvio → **mesma** mensagem, sem linha nova).
  Chave ausente → o servidor deriva uma aleatória por requisição. Corrida (2 POST simultâneos com
  a mesma chave): o `created=false` do `Enqueue` remove o órfão `PENDING` e devolve a vencedora.
- **`message` FAILED na falha terminal**: o engine só conhece o outbox. O handler **espelha**
  `jobs.Worker.settleFailure` (`isTerminalJobError`) e, quando a falha é terminal (unrecoverable
  ou tentativas esgotadas), marca a mensagem `FAILED` + audita **antes** de devolver o erro
  (que manda o job à dead-letter). Falha transitória → devolve o erro e a mensagem segue `PENDING`.
- **Mídia em disco, raiz PRIVADA** (`OMNICHANNEL_MEDIA_DIR`, default `data/media/omnichannel`):
  **fora de `UPLOADS_DIR`** — sob o `http.FileServer` de `/uploads` qualquer um baixaria a mídia
  sem token. `{root}/{accountId}/{conversationId}/{random}.{ext}`, `MkdirAll 0o750`, arquivo
  `0o600`, allowlist de mime. Decodificação **streaming** (`io.Copy` do `base64.Decoder`) com teto
  = `account_config.max_upload_mb`; estouro → **413**, mime fora do allowlist → **415**.
- **Serialização de `mediaUrl`** (spec C2): quando há `media_storage_key`, a coluna `media_url`
  guarda a **URL do endpoint** `/v1/omnichannel/conversations/{cid}/messages/{mid}/media` — nunca
  data URL, nunca o path de disco. `media_storage_key`/`media_source_kind` **não** entram no
  `messageCols` (nunca no JSON).
- **`GET /media`**: `http.ServeContent` (1º uso do repo) resolve `Range`/`206`/`Content-Range` sem
  `io.ReadAll` (D2). Content-Type explícito do mime salvo + `nosniff` + `Cache-Control: private,
  max-age=60`. Rehidratação one-shot (`requiresMediaDecrypt`/`url_encrypted`) via
  `provider.DownloadMedia` → grava disco → `message.updated` **completo** (sem `correlationId`).
- **Anti-SSRF** (`ssrf.go`, F12 reusa): valida o **IP resolvido** (não o hostname) no `Control` do
  dialer (fecha TOCTOU/DNS rebinding), sem seguir redirect. Bloqueia loopback/privado/link-local/
  CGNAT/metadata. `mediaUrl` http(s) no envio passa por aqui: scheme inválido → **422**, host
  interno → **403**.
- **Shapes de realtime por call-site**: `message.created` (envio HTTP) = Message completo +
  `correlationId` (= id da mensagem, nunca `sync-history:`); `message.updated` (worker) = mínimo
  `{id, status, externalMessageId, updatedAt, correlationId}`. `mediaUrl` data: → `null` (nunca
  base64 no WS). Auditoria: `MESSAGE_OUTBOUND_QUEUED|SENT|FAILED`.

### Wiring pendente (F6) — a fazer no `module.go`/`app.go` (a F6 não os edita)

A F6 entrega os arquivos novos e as funções de registro; a costura no `handle`/boot é do
orquestrador (outras fases são donas de `module.go`/`app.go`/`http.go`). Ordem:

1. **Storage de mídia**: `media := omnichannel.NewDiskMediaStorage(omnichannel.MediaDirFromEnv())`
   (ou `cfg.OmnichannelMediaDir` se o `config.go` ganhar a env `OMNICHANNEL_MEDIA_DIR`).
2. **Outbox store + worker** (no `Build`, com `deps.Pool`/`deps.Logger`):
   `outboxStore, _ := jobs.NewPostgresStore(deps.Pool, jobs.DefaultTable)` ·
   `worker := jobs.New(outboxStore, jobs.Config{Logger: deps.Logger, WorkerID: hostname+pid})`.
3. **Services** (o `registry`/`secretBox`/`publisher` já existem no `Build`):
   `send := omnichannel.NewSendService(store, outboxStore, media, m.publisher, deps.Logger)` ·
   `mediaSvc := omnichannel.NewMediaService(store, media, registry, m.secretBox, m.publisher, deps.Logger)` ·
   `outHandler := omnichannel.NewOutboundHandler(store, registry, m.secretBox, m.publisher, deps.Logger)`.
4. **Registrar o handler + iniciar o worker**:
   `worker.Register(omnichannel.OutboundJobKind, outHandler)` · `worker.Start(bootCtx)` no boot ·
   `worker.Close()` no `Handle.Close()` (parada limpa, como `Module.Close()`).
5. **Rotas**: no `handle.RegisterRoutes`, `omnichannel.RegisterSendRoutes(mux, send, mediaSvc, h.authMiddleware)`.
6. **Env/volume**: `OMNICHANNEL_MEDIA_DIR` (default `data/media/omnichannel`) — **fora de
   `UPLOADS_DIR`**; volume novo no compose (dev+prod) e **no backup** (a mídia não está no Postgres).
7. **Migration `0207`** exige `docker compose build --no-cache api` (embed.FS não re-embute com cache).

Camadas **estritas**: `handler → service → repository`. O handler não toca o banco; o
repository não conhece HTTP. **Teto de ~450 linhas/arquivo vale integralmente aqui** (é código
novo — a violação consciente do port é só do front, canônico §14.3).

**Sem pacote de uuid** (padrão da casa): `string` + cast no SQL (`$1::uuid`). Coluna nullable →
scan em `*string`, nunca no tipo puro. `jsonb` → `json.RawMessage`, nunca scan direto em struct.

## Ao mexer em `back/`

`docker compose up -d --build api`. **Migration nova → `docker compose build --no-cache api`**
e depois `up -d api`. Portas são fixas (api=9091) — não alterar.

## E1 — entrega, reply, midia inbound e takeover

- `fromMe=true` e sempre `OUTBOUND/provider_device`; nunca dispara triagem IA. Dedupe e eco de
  envio convergem pela unique `(account_id, instance_scope_key, external_message_id)`.
- ACK do provider so avanca `PENDING -> SENT -> DELIVERED -> READ`; erro/delecao sao estados
  explicitos e `provider_error_code` aceita apenas codigo seguro.
- Reply outbound resolve a mensagem na mesma conta+conversa e envia a referencia externa pelo
  adapter. Reply inbound pode guardar external fallback e deve reconciliar a FK local depois.
- Midia inbound e job `omnichannel.media.fetch` com payload contendo apenas `messageId`. O handler
  rele instancia/credencial/limite no PostgreSQL, publica o arquivo por temp+fsync+rename, persiste
  hash/estado e emite realtime. `GET /media` apenas serve arquivo privado `ready`; nunca baixa do
  provider no request. Retry rearma exclusivamente esse kind/idempotency key. O diretorio
  `/app/data/media/omnichannel` usa o volume exclusivo `api_omnichannel_media`; nunca montar sob
  `/app/data/uploads`. A migration 0214 autoriza somente os eventos de auditoria
  `MESSAGE_MEDIA_READY|FAILED|RETRY` adicionados pela E1.
- `ai_generation` e capturada antes da chamada ao modelo. Merge e criacao atomica de mensagem IA
  + outbox exigem conversa `ai_active` e a mesma geracao. Evento humano incrementa a geracao e
  cancela saidas IA pendentes dentro da transacao.

## E5 — handoff, SLA e policies determinísticas

- `messaging.handoffs` é o snapshot autoritativo da transferência: sempre criado sob lock da
  conversa, com idempotência por conta, resumo/campos sanitizados e invalidação de dispatch/outbox
  de IA na mesma transação. `policy_id` e `policy_snapshot` preservam a decisão mesmo quando a
  configuração muda depois.
- `messaging.handoff_policies` é CRUD administrativo sob `omnichannel.settings.manage`. Conditions
  aceitam somente chaves fechadas (`reasonCode`, `sourceState`, `departmentId`, `channel`,
  `intent`, `relationshipStatus`, `lifecycle`, `tag`, `slaRisk`, `confidenceMin/Max`, `hourUtc`);
  o backend rejeita chaves desconhecidas, segredos, regex e expressões livres.
- A avaliação é ordenada por `priority asc, id asc`, usa estado/contato/extracted_fields sob lock e
  escolhe target ativo, depois fallback ativo, depois o alvo sugerido/default. Nem n8n nem o modelo
  escolhem fila, escrevem `messaging.*` ou enviam aviso; qualquer aviso futuro deve passar pela
  outbox e capability do adapter.
- A aba `Handoff` em `OmnichannelConfigDrawer` apenas configura as policies; o inbox lê handoff/SLA
  pela API autoritativa. Não criar estado booleano paralelo (`ai_active`, `assigned`) no frontend.

## CI-01/CI-07 — cliente por canal e inteligência opcional

- `messaging.channel_client_bindings` é o ownership histórico do recurso WhatsApp/Instagram.
  `automation_profiles` continua sendo configuração de IA e nunca substitui esse binding.
- Inbound resolve pela borda `[effective_from,effective_to)` no timestamp do provider e grava
  snapshot em conversa/touchpoint. Ausência não bloqueia persistência humana; vira `unresolved`.
  Ambiguidade ou client inválido vira `quarantined`.
- APIs `/v1/omnichannel/channel-client-bindings*`, exceptions, preview/apply de reparo e policy
  exigem `omnichannel.instances.manage`, account do Principal e client do catálogo permissionado.
- Reassign encerra o intervalo e cria sucessor; não altera histórico. End só é aceito depois de o
  recurso ser desativado. Toda mutação exige reason, idempotency key e optimistic revision.
- Policy administrável: `channelBindingMode=legacy|shadow|enforced`,
  `customerIntelligenceMode=off|shadow|on` e
  `customerIntelligenceFailurePolicy=legacy_fallback|retry_then_handoff|immediate_handoff`.
  O default é `shadow/off/retry_then_handoff`; `on` exige `customer_data` e
  `customer_intelligence` habilitados. `enforced` exige todo recurso ativo resolvido.
- A porta `CustomerIntelligenceBridge` só devolve proposta. Mesmo no modo `on`, este módulo
  revalida generation/FSM/policy e é o único que cria `OUTBOUND/PENDING` + outbox. Falha técnica
  nunca vira `no_reply`: conforme a policy, usa o legado, faz retry limitado e depois handoff, ou
  transfere imediatamente. Shadow executa e audita, mas nunca produz efeito de canal.
- A aceitação durável preserva `pipelineVersionId` e as referências completas de cada process run
  (process/config, binding e camadas de prompt, agente, modelo, snapshot e schema). O payload não
  carrega segredo, prompt efetivo nem mensagem bruta adicional.
- Candidate claims seguem no outcome apenas como descritores tipados (`ordinal`, `factKey`,
  `valueType`, confiança, evidências, validade e referências de process run). O valor extraído não
  é duplicado na outbox nem em `extracted_fields`; Customer Intelligence o reidrata do output
  cifrado do runtime e sempre o persiste como `candidate/unverified/llm`.
- Aceitar uma claim no Customer Intelligence não materializa fato e não permite que a IA
  substitua valor manual ou verificado. O Omnichannel não implementa atalho para essa regra.
- Todo inbound novo com snapshot de binding `resolved` cria, na mesma transação da mensagem,
  um evento em `messaging.customer_data_outbox`. O payload é ID-only e deve permanecer sem nome,
  telefone, conteúdo, prompt ou credencial.
- `customer_data_outbox` é uma lane própria do `platform/jobs`, separada do sender e de
  `intelligence_outbox`. O worker reidrata a evidência autoritativa e chama Customer Data apenas
  pela bridge do composition root; IA desligada não desativa essa ingestão.
- Binding `unresolved|quarantined`, grupo, eco `fromMe` e canal não suportado não alimentam
  Customer Data, mas continuam sendo persistidos e exibidos pelo chat humano.

## Cofre compartilhado do Assistente 360 — Anthropic (0292)

- `messaging.ai_credentials` aceita credenciais nomeadas `openai|anthropic|gemini|glm` para o
  Assistente 360. O catálogo é account-scoped, mascarado e pode incluir credencial herdada da
  agência canônica; segredo nunca volta ao frontend.
- O keyring legado dos agentes Omnichannel continua aceitando apenas os providers já suportados
  por esses agentes. Adicionar Anthropic ao cofre compartilhado não altera o brain/outbox/canais.
- A listagem de modelos Anthropic usa `GET /v1/models` com `x-api-key` e
  `anthropic-version: 2023-06-01`; nunca envia `Authorization: Bearer` para esse provider.
