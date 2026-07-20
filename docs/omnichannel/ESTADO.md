# Estado da implementação — Atendimento WhatsApp

> **Documento de trabalho, não canônico.** O canônico é [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md);
> o espelho de fases é `web/app/components/roadmap/data/phases-part7.ts`.
> Este arquivo existe só para **retomar de onde parou** — apagar quando o piloto P0 fechar.
>
> Última atualização: **2026-07-20**

---

## Atualização de direção — 2026-07-20

- Arquitetura híbrida aprovada: Go/PostgreSQL seguem autoritativos; n8n executa debounce,
  contexto, modelo, multimodal, tools e decisão estruturada. Envio final continua somente
  pelo outbox/adapters Go.
- Executor `OMNI_AI_EXECUTOR=native|n8n` implementado, com `Omnichannel Brain` stateless,
  configuração/chave vindas do banco/Go e saída revalidada.
- CRM base multicanal entregue nas migrations 0211/0212: contatos, identidades,
  touchpoints, notas e backfill das conversas existentes.
- Multi-turno inicial ligado: resposta válida vira mensagem `PENDING` + outbox;
  `needs_human=false` mantém IA e `true` segue para handoff; falha faz fail-open.
- Workflow legado `Whatsapp`/WAHA desativado no n8n local e retirado do export/import.
  WAHA permanece apenas como dependência transitória da tela antiga `/automation`.
- Plano executivo posterior ao port: [`PLANO_TECNICO_EVOLUCAO.md`](PLANO_TECNICO_EVOLUCAO.md).

P0 atual: mídia inbound real, quote, espelho `fromMe`, job idempotente da IA, debounce e
contrato versionado `continue_ai|handoff|no_reply`. Os detalhes históricos abaixo ainda são
úteis como evidência, mas estados antigos de “pendente” podem ter sido superados por esta
atualização e pelo AGENT do módulo.

---

## 0. PILOTO P0 — CODE-COMPLETE (2026-07-18)

Todas as fases do piloto (F0–F10 + F13-mínimo) estão **em código, compilando/testando verde, e no ar** (api sobe, migrations 0200–0209 aplicadas, mock 100%). O que falta NÃO é código do piloto:

- **Smoke visual no browser** (do dono): logar, abrir as telas de config, ver mensagem fluir ao vivo. Não invento login.
- **Pareamento WhatsApp real**: só na VPS — o WebSocket do WhatsApp dá timeout desta máquina local (rede, não código; ver §Evolution).
- **Follow-ups de código (passos próprios, documentados abaixo)**:
  1. **Auto-disparo da IA no inbound** (F5↔F9↔F8) — a IA rodar sozinha em toda mensagem. Caminho quente; passo isolado com teste. Ver §F5+F9.
  2. **Estender `channel.Provider`** com `SendReaction`/`DeleteForAll` (+ evolution) — hoje reaction/delete-for-all respondem 409 honesto. Ver §F6+F7.
  3. **Gaps de backend que a F10 apontou**: `GET .../capabilities`, `provider` na InstanceView de gestão, DELETE de instância. A tela degrada sem eles.
  4. **F13 P1** (fora do piloto): export/anonimização do titular — hoje manual por psql.
  5. **Notas de deploy F13**: rodar o purge em **dry-run** na 1ª subida em prod; seed de `platform_settings['ai_model_pricing']`; volume de mídia no backup.
  6. Sincronizar os 3 docs (AGENT.md do módulo, canônico, phases-part7.ts) com o estado final — os subagentes deixaram para o orquestrador para não colidir.

**F13 LGPD-mínimo ✅** — retenção por classe (365/180/90/30, config por conta→platform_settings→default Go) + job de purge no MESMO worker (batches ≤500, sempre `account_id`+cutoff, nunca DROP), purge de mídia do disco, masking auditado (nenhum log vaza conteúdo/telefone), custo de LLM por conta (`GET /v1/omnichannel/ai/usage`). Migration `0209` (purge_runs). **Wire feito por mim**: purge registrado no worker + scheduler no boot + rota de custo.

---

## 0.1 🚀 BRIDGE PARA O TESTE REAL NA VPS (checklist de deploy — o dono revisa e roda)
O real (parear o WhatsApp) não conecta LOCAL (WebSocket do WhatsApp dá timeout desta máquina — rede, não código). Na VPS (egress de datacenter) deve conectar. Para o teste real lá, falta preparar o deploy (nada disto é código do módulo; é infra de prod, o dono aplica/revisa — **não commitar segredo**):

1. **`docker-compose.prod.yml` — adicionar os serviços `evolution` + `evolution-db`** (espelhar do `docker-compose.yml` linhas ~464-540, profile `omnichannel`), MAS prod-safe: **sem default inseguro** no `AUTHENTICATION_API_KEY` (obrigar via env), porta do host **não exposta** (ou só `127.0.0.1`), `EVOLUTION_DB_PASSWORD`/`EVOLUTION_API_KEY` fortes. **CRÍTICO:** manter a imagem **`evolution-api:v2.3.7`** E a env **`CONFIG_SESSION_PHONE_VERSION=2,3000,1025205472`** — sem elas o WhatsApp trava o handshake e o QR nunca gera (ver §Evolution; foi o que travava local).
2. **Serviço `api` do `compose.prod` — adicionar as envs** (hoje não tem): `OMNI_SECRETS_KEY` (**obrigatória** — a api NÃO sobe sem, fail-fast), `EVOLUTION_BASE_URL=http://evolution:8080`, `EVOLUTION_API_KEY`, `WEBHOOK_RECEIVER_BASE_URL=`**{URL pública da api}** (por onde a Evolution devolve o webhook — em prod é o domínio, não `http://api:8080`), `OMNI_DEFAULT_WHATSAPP_PROVIDER=evolution`.
3. **Caddy (VPS)** — adicionar `handle /v1/webhooks/*` roteando pra api (webhook é PÚBLICO, sem JWT). ARMADILHA da casa: `cat >`+reload não pega no inode do bind-mount órfão → `docker restart` do container do Caddy (ver `project_vps_shortlink_caddy_routing`).
4. **`.env` de prod** — gerar e setar `OMNI_SECRETS_KEY` (`openssl rand -base64 32`), `EVOLUTION_API_KEY` (`openssl rand -hex 24`), `EVOLUTION_DB_PASSWORD`. **Não commitar.**
5. **Migrations** — deploy roda `docker compose build --no-cache api` (as migrations 0200-0209 são embed.FS; cache de camada pode não re-embutir).
6. **F13** — 1ª subida em prod: rodar o purge em **dry-run** antes do delete real; seed de `platform_settings['ai_model_pricing']` (senão custo=0).
7. **Subir o Evolution na VPS**: `docker compose --profile omnichannel up -d evolution` → parear o número **42984138129** lendo o QR pelo painel (`docs/omnichannel/EVOLUTION_SETUP.md`).

**Topologia real da VPS** (memória): projeto `listaatendimento`, Caddy compartilhado em `/opt/omnichannel` (NUNCA parar), conta-agência slug `crow`. Ver `project_vps_real_topology`.

## 0.2 ✅ TELA COMPLETA LOCAL COM MOCK (2026-07-18) — para o dono aprovar
O dono precisa VER o Atendimento completo rodando local antes de aprovar. O pareamento real não conecta local (WS do WhatsApp), então a tela roda no provider **mock**. Fechei os 3 gaps que a F10 apontou (degradavam a tela) e populei dados demo pelo pipeline real.

**Gaps de backend fechados (build/vet/test verdes; rebuild da api aplicado):**
1. **`GET /tenant/whatsapp/instances/{id}/capabilities`** — nova rota (`http_instances.go` → `handleInstanceCapabilities` → `SessionService.InstanceCapabilities` → resolve provider por id via `ResolveInstanceForOps` → `registry.Get(provider).Capabilities()`). **Json tags camelCase** adicionadas em `channel.Capabilities` (`provider.go`) senão o front (camelCase) não casava. **Sem isto todo número mostrava banner "DEGRADADO"** (`ConfigNumberCapabilities.vue`).
2. **`provider` na `InstanceView`** (`model.go`) + `instanceCols`/`scanInstance` (`store_postgres_admin.go`) — a coluna já existia, só a projeção de leitura a omitia.
3. **`/status` e `/qrcode` aceitam `instanceId`** — o inbox verbatim manda `?instanceId=<UUID>`, o back só lia `instanceName` → banner de conexão sempre degradado. Novo `Store.GetSessionInstanceByRef` (id → nome → default); `assertInstance`/`resolveForSession`/`Status`/`QRCode` passaram a receber `(instanceID, instanceName)`; handlers passam os dois params. Empty+empty → a default (o inbox mostra a conexão da default quando nada está selecionado).

**Setup de DEMO local (conta `crow`, id `80caf5d5-…`, instância `omni-main` id `cee78ac9-…`):**
- ⚠️ **`omni-main` trocada de `evolution` → `mock`** por UPDATE direto (`update messaging.whatsapp_instances set provider='mock' where instance_name='omni-main'`). É **mudança LOCAL de demo** (evolution não conecta local); **reverter para prod**: `set provider='evolution'` (o bridge §0.1 já diz prod=evolution). Reversível, aditiva, conta-agência de teste.
- **3 conversas demo** injetadas pelo webhook mock público (`POST /v1/webhooks/omnichannel/mock/crow`, `instance:"omni-main"`): Maria Silva, João Pereira, Ana Souza (4 msgs INBOUND). Provado no banco: `state=queued` (o auto-disparo F8 rodou: msg.inbound → routing → SystemRoute). O `ingestOne` resolve a instância por NOME e não valida provider, por isso o webhook mock popula a `omni-main`.
- Badge da página reescrito honesto ("PILOTO — MOCK LOCAL", flag de demo) — princípio 4.

**PASSO 2 (documentado, NÃO bloqueia ver a tela — degradações engolidas por try/catch):**
- **`GET /users`** (retorna `TenantUser[]`) — picker de atribuição fica vazio sem ele (fonte = membros da conta). Há também um `POST /users` (criar usuário) do legado — decidir se porta ou roteia pro cadastro central.
- **`GET /tenant`** (retorna `TenantSettings`: limite de upload + instância default) — hoje 404, cai no default de upload (engolido).
- **`DELETE /tenant/whatsapp/instances/{id}`** — spec F10 pede, mas o front NÃO chama (usa PATCH `isActive:false` "Desativar"). FK `conversations.instance_id`: hard-delete só sem conversas (senão 409). Puro spec-completeness.

### Smoke do DONO (ver a tela completa) — 2026-07-18
1. Login normal (platform_admin) → selecionar a conta **crow** (ativa) → abrir **/omnichannel**.
2. Esperado: banner de conexão **conectado** (mock), **sem** "DEGRADADO"; lista com **3 conversas** (Maria/João/Ana); abrir uma → mensagem inbound; enviar resposta (mock ecoa SENT). 
3. Ver mensagem chegar AO VIVO (realtime F5): `POST /v1/omnichannel/webhooks... mock/crow` com novo `messageId` (ou repetir o comando de injeção do ESTADO) → a conversa sobe sozinha.

## 0.3 🔴 BACKLOG (funcionalidades > aparência; decisão do dono 2026-07-18)
Com WhatsApp real pareado, o dono achou 3 bugs de FUNCIONALIDADE. **Prioridade: fechar
funcionalidades ANTES de polir aparência/bugs.** Detalhe técnico no `back/internal/modules/omnichannel/AGENT.md` (seção Backlog):
1. **Mídia recebida (imagem) não renderiza** ("Carregando preview..." infinito) — caminho de mídia inbound.
2. **"Responder" (quote) não vai pro WhatsApp real** — falta passar `quoted` pro Evolution no send.
3. **Mensagens enviadas PELO celular (fromMe) não aparecem na plataforma** — falta ingerir `messages.upsert` fromMe=true como outbound.

## 1. Onde estamos

| Fase | Estado | Evidência |
|---|---|---|
| **F0** Decisões + fundação | ✅ **feita** | 7 decisões (D-A..D-G) no canônico §2; 15 specs em `specs/`; roadmap espelhado |
| **F1** Front verbatim | ✅ **código em disco** | 72 arquivos em `web/app/**` (49 composables + 22 componentes + 1 página), byte-identidade conferida por SHA-256 |
| **F2** Schema + Go + leitura | ✅ **código em disco** | migration `0200_messaging_schema.sql` (9 tabelas); módulo `back/internal/modules/omnichannel/`; wire no `app.go` |
| **F3** Infra transversal | ✅ **código em disco + testado** | `platform/jobs` (FIFO **provado sob concorrência** contra Postgres real — `TestWorkerFIFO` 30s), `platform/secretbox` (AES-256-GCM, nonce aleatório), `platform/llm` (client OpenAI-compat + schema + allowlist SSRF), `platform/modules/limits.go`. `go build`/`vet`/`test` verdes |
| **F4** Canal + webhook | ✅ **código em disco, build combinado verde** | interface `channel.Provider` + adapter `mock` + adapter **Evolution** (`channel/evolution/`, 17 testes) + webhook público protegido + sessão/QR + `number_guard` + wiring do `secretbox`/`OMNI_SECRETS_KEY` no app.go. Migration `0201`. Containers `evolution`+`evolution-db` no compose (profile `omnichannel`). Guia `EVOLUTION_SETUP.md` |
| **F8** Domínio | ✅ **código em disco, testado** | departments/queues/queue_members/routing_rules/routing_decisions + **máquina de estados 7×12=84 transições provada** (nenhuma implícita) + projeção state→status + motor de roteamento determinístico + gate de fila (WHERE no repo). Migration `0205`. Rotas costuradas no module.go |
| F4–F14 | ⬜ não iniciadas | — |

**Nada foi commitado.** HEAD continua em `21eeaba`.

**Pilha no ar (2026-07-17):** api buildada `--no-cache` e de pé — `migration_up_ok`, `module built
module_id=omnichannel schema=messaging`, 9 tabelas `messaging.*` no banco, `CHECK` de `state` com os 7
valores (D-E enforçada pela constraint). Web buildado (com `@emoji-mart/data`) e de pé — `/omnichannel`
responde 200, sem erro de import. Rota da api protegida: `400 missing_account_id` sem conta,
`403` para conta sem membership/módulo.

**Falta a prova que exige LOGIN do dono** (não invento credencial): (1) a página renderizar o inbox
logado; (2) o smoke cross-tenant de dois usuários — logar como conta A, mandar `X-Account-Id: B` e
esperar 403/404. O código do fix está no caminho e o middleware rejeita não-membro; falta a prova
empírica de duas contas reais.

### Estado F4+F8 (2026-07-17, tarde) — reconciliação FEITA, build da api em andamento
Ambas fecharam. **Reconciliação da costura aplicada por mim:**
1. `RegisterDomainRoutes` (F8) wireada no `module.go` (a F8 não podia editar o arquivo; a F4 editou pro adapter Evolution) — sem isso o CRUD de setores/filas responde 404.
2. **Fix de isolamento:** o índice de dedup do webhook (migration 0201) era global `(provider, external_event_id)` — troquei para `(account_id, provider, external_event_id)` + o `ON CONFLICT` do Go casando. Evita colisão cross-tenant (evento da conta B sumir como "duplicado" da A). Migration 0201 ainda **não aplicada** — fix feito antes de aplicar.
3. **`OMNI_SECRETS_KEY` setada no `.env`** (`openssl rand -base64 32`) — o app.go tem fail-fast (`secretbox.FromEnv`, app.go:427): **sem a env a api NÃO sobe**. Já está no `.env` local; **em prod/staging precisa ser setada também** (não commitar).
4. `go build ./...` + `go vet` + `go test ./internal/modules/omnichannel/...` todos **verdes** com as duas fases juntas.
5. **Build da api feito, api NO AR.** Boot limpo: `module built module_id=omnichannel schema=messaging`, sem panic (fail-fast do `OMNI_SECRETS_KEY` passou). **Migrations 0201+0205 APLICADAS** — as 6 tabelas novas existem no banco. ✅

### F6 (envio) + F7 (ações) — 2026-07-18 — NO AR, wireadas por mim
**F6 envio ✅** — `POST /conversations/{id}/messages` sobre o outbox da F3 (worker iniciado no boot em module.go), `GET .../media` (stream+Range, anti-SSRF, mídia em disco), migration `0207` (media_storage_key). Idempotência por conta (D-G). Rota viva (404 com code = handler). **Wire feito**: worker `Start` no Build + `Close` no shutdown + `RegisterSendRoutes`.
**F7 ações ✅** — reaction/forward/delete/status/assign + group/sync/contatos, migration `0208` (audit CHECK += MESSAGE_FORWARDED/DELETED_FOR_ALL). status/assign passam PELA FSM (F8), nunca escrevem state na mão. Escopo de instância CORRIGIDO (o ternário morto do legado). **Wire feito**: `RegisterActionRoutes` + `NewActionsService`.
> **Gap honesto (follow-up F4):** a interface `channel.Provider` só tem 5 métodos — reaction/delete-for-all/group/sync NÃO chegam no provider ainda; respondem **409 `provider_action_unavailable`** (nunca sucesso fingido). forward/status/assign/delete-for-me/open-conversation funcionam ponta a ponta. Falta: estender `channel.Provider` com `SendReaction`/`DeleteForAll` + implementar no evolution + trocar os 409 pela chamada real. Passo próprio (toca o pacote channel/F4).

### F5 (realtime) + F9 (triagem IA) — 2026-07-17, noite — CÓDIGO em disco, rotas wireadas
**F5 realtime ✅** — canal Go `GET /v1/realtime/omnichannel` (padrão ticket, fora do gate), Publisher injetado no app.go (`WithPublisher(realtimeService)`), `message.created` publicado no inbound pós-commit (com o id interno, senão o front duplica no merge), mídia sanitizada (data URL→null). Front `useOmnichannelInboxRealtime.ts` reescrito sobre `useRealtimeSocket` (accountId pela fonte do REST — evita o loop 1006 do platform_admin). Testes de transporte verdes; WS route: sem ticket→401. **Nota:** F5 só emite `message.created`; `conversation.updated`/`message.updated` completos dependem de leitura de conversa (F6/F7) — os 3 shapes e o transporte já existem.

**F9 triagem IA ✅ (rotas), ⏳ (auto-disparo)** — migration `0206` (ai_agents/versions/runs/collect_field_defs + FK routing_decisions.ai_run_id), triagem nativa sobre `platform/llm`, saída schema-validada, `human_active` bloqueia, limite degrada (não derruba). **Rotas costuradas por mim** no module.go (`NewAIService(store, llm.New(...), secretBox, limits, logger)` + `RegisterAIRoutes`). Config/CRUD/publish-rollback/simulate **funcionam**.

#### ✅ Auto-disparo da IA (2026-07-18) — FEITO e PROVADO end-to-end contra o mock
`service_transition.go`: `transitionContextFor` resolve `HasActiveAgent` via `store.ActiveAgent` (só em `msg.inbound`) + `SystemTransition`/`SystemRoute` (ator sistema, sem gate). `service_inbound.go`: `InboundService` recebeu `ai`+`domain`; `runAutoTriage` em goroutine (ctx 90s, recover) pós-publish. Provado: SEM agente → routing direto (0 ai_runs); COM agente → 1 ai_run amarrado à conversa+msg. Fire-and-forget: 202 antes da triagem, msg gravada mesmo com IA falhando, sem panic. Resíduo de teste limpo.
#### ✅ channel.Provider reaction/delete (2026-07-18) — FEITO. `SendReaction`/`DeleteForAll` na interface + evolution (`/message/sendReaction`, `/chat/deleteMessageForEveryone`) + mock no-op. F7 deixou de responder 409 nesses dois (gate de Capabilities mantido). `WithActionsSecretBox(m.secretBox)` costurado no module.go por mim.
_(histórico da pendência abaixo)_

#### 🔧 Wire pendente F9 — auto-disparo da IA no inbound (F5↔F9↔F8) — ~~NÃO feito~~ FEITO acima
Faz a IA **rodar sozinha** quando chega mensagem. Requer, com cuidado (fire-and-forget, nunca bloquear a persistência da mensagem):
1. `InboundService` receber o `*AIService` — reordenar o `Build` (construir `aiSvc` antes de `NewInboundService`) e passar na assinatura.
2. Em `service_inbound.go`, após `s.publishInboundMessage(...)` (linha ~210), se `res.MessageID != ""` e `!res.Duplicate`: `go`/inline `s.ai.Dispatch(ctx, TriageInput{AccountID, ConversationID: res.ConversationID, MessageID: res.MessageID})` — erro da IA NUNCA falha o webhook.
3. Após o Dispatch, disparar a transição F8 (`ai.triage.done`/`ai.triage.failed` → `RouteConversation`).
4. `service_transition.go:107` `transitionContextFor` fixa `HasActiveAgent:false` — resolver `store.ActiveAgent(accountID)` (precisa do store + accountID; hoje é função pura). Sem isso a máquina nunca entra em `ai_active`.
Testar com o mock: mensagem inbound → run em ai_runs → conversa roteada. **Fazer isolado, com teste, por ser o caminho de toda mensagem recebida.**

### Estado Evolution (2026-07-17, noite) — 3 bugs de webhook RESOLVIDOS + blocker de infra do QR
**Provado funcionando (testei via API + curl direto na Evolution v2.1.1):**
- Mock: 100% (bootstrap/connect/status → `connected:true`).
- Evolution container UP, api **cria a instância na Evolution** (confirmado via `/instance/fetchInstances`).
- `OMNI_DEFAULT_WHATSAPP_PROVIDER=evolution` → bootstrap volta `provider:"evolution"`.

**✅ Os 3 bugs do adapter Evolution — RESOLVIDOS e provados ao vivo:**
1. **BUG 1 — webhook sem `headers` → 401.** `webhookConfig` (client.go) agora embute `headers:{apikey:<token>}`
   (token = `c.apiKey == expectedToken(cred)`) no create E no set. **Prova:** inbound com token correto → 202;
   com token errado → 401 (fail-closed).
2. **BUG 2 — auto-set do webhook não disparava.** Causa raiz REAL: o `createInstance` mandava o webhook
   **duplo-envelopado** (`body.webhook.webhook.url`) → Evolution devolvia **400 "Invalid url"**, o adapter
   ENGOLIA o 400 (idempotência por nome), a instância nunca era criada e o `setWebhook` seguinte dava **500**
   (instância inexistente). Fix: `webhookConfig` devolve o objeto FLAT; o create embute direto, o `/webhook/set`
   envolve em `{"webhook":{...}}`. O erro do `setWebhook` agora é logado em Warn (era `_ =`). **Prova:**
   `/webhook/find/omni-main` mostra url + `headers.apikey` + `enabled:true` + os 4 eventos; zero
   `evolution_set_webhook_failed` no log.
3. **BUG 3 — QR do webhook não cacheava.** Causa raiz: o `ingestOne` só tratava `message_received`; o
   `QRCODE_UPDATED` virava ignored e o `qrCache` do `InboundService` nem existia (era exclusivo do
   `SessionService`). Fix: `qrCache` **compartilhado** (module.go) + `ingestOne` grava o QR do
   `QRCODE_UPDATED` (extrai `data.qrcode.base64` OU `data.base64`) e limpa no `CONNECTION_UPDATE state=open`.
   **Prova:** POST sintético de `qrcode.updated` no webhook (token correto) → `/qrcode` devolve o data URL
   exato (nested e flat); `state=open` limpa.

**✅ RESOLVIDO (2026-07-18) — o QR real GERA local. NÃO era infra/rede: era CONFIG faltando.**
A conclusão anterior ("timeout do WebSocket é da máquina/rede") estava **ERRADA**. O `whats-test` (omni
antigo) conectava Evolution local nesta MESMA máquina — a diferença era config:
```
EVOLUTION_IMAGE = evoapicloud/evolution-api:v2.3.7        (nós usávamos v2.2.3)
CONFIG_SESSION_PHONE_VERSION = 2,3000,1025205472          (nós NÃO setávamos)
```
O `CONFIG_SESSION_PHONE_VERSION` é a versão do WhatsApp Web que o Baileys **anuncia** no handshake. O
WhatsApp **rejeita/trava versões velhas** — o "Timed Out at validateConnection" era isso, não a rede.
Copiei a config pro `docker-compose.yml` (serviço `evolution`). **PROVADO ao vivo:** com v2.3.7 +
`CONFIG_SESSION_PHONE_VERSION`, `POST /instance/create` volta `qrcode.code` e `GET /instance/connect`
devolve o **QR base64 real** (antes: `{count:0}` + timeout). Instância de teste criada/pareável/deletada
via curl direto na Evolution :8085.
**Como parear (o dono):** tela → "Conectar por QR" → o QR aparece → ler no WhatsApp. Conta `crow`
(80caf5d5-...), instância `omni-main` (revertida p/ `evolution`), número do dono 42984138129.
**ARMADILHA quando o WA deprecar a versão:** o QR volta a não gerar; atualizar `CONFIG_SESSION_PHONE_VERSION`
(ver a que o Evolution/WhatsApp Web usam no momento).

### ✅ RESOLVIDO (2026-07-17, tarde) — remap de rotas + superfície de instância do F4
- **6 rotas remapeadas** de `/whatsapp/session/*` e `/whatsapp/instances` → `/tenant/whatsapp/*` (o path do front verbatim, D-B). `http_session.go` + `http.go`.
- **5 handlers novos** que o front chamava e não existiam: `POST/PATCH tenant/whatsapp/instances`, `PUT .../{id}/users`, `POST tenant/whatsapp/validate-endpoints`, `POST tenant/whatsapp/conversations/clear` (`http_instances.go` + `service_instances.go` + `service_instance_ops.go` + `store_instances.go`). `assignedUserIds`/`userScopePolicy` persistem em `provider_config jsonb` (sem migration nova); `validate-endpoints` valida config (probe vivo é F6). LEGADO item `f` resolvido.
- `go build`/`vet`/`test` verdes; rebuild da api aplicando. **O smoke do mock connect deve funcionar** — o dono confirma no browser.

_Histórico do bloqueador (abaixo, mantido como registro):_

### 🔴 (RESOLVIDO) descasamento de path front↔back (D-B violada)
O front verbatim (que É a especificação, D-B) chama, relativo a `/v1/omnichannel`:
`tenant/whatsapp/{bootstrap, connect, conversations, instances, logout, qrcode, status, validate-endpoints}`.
Mas o back registrou em paths DIFERENTES:
- **F4** (`http_session.go`): `/v1/omnichannel/whatsapp/session/{bootstrap,connect,status,qrcode,logout}` — falta `tenant/`, sobra `session/`.
- **F2** (`http.go`): `/v1/omnichannel/whatsapp/instances` (+ `/access`) — falta `tenant/`.
- **Sem dono ainda**: `tenant/whatsapp/conversations`, `tenant/whatsapp/validate-endpoints`.
**Prova:** log da api → `GET /v1/omnichannel/tenant/whatsapp/status → 404`. É por isso que o painel diz
"Nenhum WhatsApp configurado / Status temporariamente indisponível".
**CORREÇÃO (próximo passo — D-B: o Go se adapta ao front, NÃO mexer no front):** remapear as rotas Go
para `tenant/whatsapp/*` em `http_session.go` (F4) e `http.go` (F2); criar handlers p/ `conversations` e
`validate-endpoints`. Confirmar o conjunto exato lendo `web/app/composables/omnichannel/useOmnichannelAdmin.ts`
e afins. Depois `docker compose up -d --build api` (SEM `--no-cache` — só código Go, sem migration nova) e
reconferir: os 404 viram 200/401. **Só então o smoke com mock funciona.**

#### Mapa de execução do remap (extraído do front — aplicar direto)
Todos os paths são relativos a `/v1/omnichannel` (o `useApi` prefixa). Fonte confirmada: `useOmnichannelAdmin.ts`,
`useOmnichannelAdminOperationalOps.ts`, `useOmnichannelWhatsAppSession.ts`. **Regra: mudar o path no Go para
o do front; NÃO mexer no front.**

| Método | Path que o FRONT chama | Onde o Go registrou hoje | Ação |
|---|---|---|---|
| GET | `/tenant/whatsapp/status?...` | `/whatsapp/session/status` (`http_session.go`) | renomear |
| GET | `/tenant/whatsapp/qrcode?...` | `/whatsapp/session/qrcode` | renomear |
| POST | `/tenant/whatsapp/bootstrap` | `/whatsapp/session/bootstrap` | renomear |
| POST | `/tenant/whatsapp/connect` | `/whatsapp/session/connect` | renomear |
| POST | `/tenant/whatsapp/logout` | `/whatsapp/session/logout` | renomear |
| GET | `/tenant/whatsapp/instances` | `/whatsapp/instances` (`http.go`, F2) | renomear (+ `/access`?) |
| POST | `/tenant/whatsapp/instances` | — (F2 tem GET/PATCH; ver se há POST) | conferir/criar |
| PATCH | `/tenant/whatsapp/instances/{id}` | conferir F2 | conferir |
| PUT | `/tenant/whatsapp/instances/{id}/users` | — | criar handler (assigned users) |
| POST | `/tenant/whatsapp/validate-endpoints` | — | **novo handler** (validação de config) |
| POST | `/tenant/whatsapp/conversations/clear` | — | **novo handler** (limpar conversas) |

Passos: (1) editar os `mux.Handle(...)` em `http_session.go` (5 rotas de sessão) e `http.go` (instances) para
o prefixo `tenant/whatsapp/`; (2) escrever handlers de `instances/{id}/users`, `validate-endpoints` e
`conversations/clear` (ver o body/response esperado no shape TS do front: `WhatsAppInstanceRecord`,
`WhatsAppStatusResponse`, `WhatsAppBootstrapResponse`, `WhatsAppQrCodeResponse`); (3) `docker compose up -d
--build api` (sem `--no-cache`); (4) `curl` cada rota → 401 (sem token) ou 200, nenhum 404. Depois o smoke do mock.
**Não esquecer:** as rotas de sessão/instância continuam sob `RequireAuthWithAccount` (membership) — não regredir p/ `RequireAuth`.

**Pendências de reconciliação que NÃO fiz (follow-ups nomeados, não bloqueiam o smoke do mock):**
- **Gate de fila na lista principal**: a F8 aplicou o `visibilityWhere` só nas leituras dela; a `ListConversations` da F2 (`store_postgres.go`) ainda **não** filtra por queue-membership. Isso NÃO é furo de isolamento (o filtro de `account_id` está lá) — é a restrição extra "atendente vê só suas filas", que entra quando a F7 (assign) ficar viva. Fazer com cuidado ao costurar a F7.
- **Webhook: fallback pra chave global** (`adapter.go:238`): conta sem credencial própria cai no `EVOLUTION_API_KEY` global. Endurecer para exigir credencial por instância em prod (LEGADO).
- **rate_limit.go em memória**: buckets nunca removidos (cresce) + sem o "block 5min" do legado. Follow-up (Redis/F14).
- **Paridade prod**: `docker-compose.prod.yml` NÃO recebeu os serviços evolution/evolution-db nem as envs. Replicar antes de subir em prod.
- **Caddy prod**: rota pública `/v1/webhooks/*` precisa de `handle` no Caddyfile.
- **Tag da imagem Evolution**: `atendai/evolution-api:v2.1.1` marcada "confirmar versão" no compose.
- **AGENT.md do módulo**: as duas fases encostaram; conferir se não houve clobber.

### Smoke quando a api subir (o que o DONO faz)
- **Com mock (sem número, destrava tudo):** logar como admin → `/omnichannel` → Configurar WhatsApp → provider `mock` → conectar (conecta na hora). Ou `POST /v1/webhooks/omnichannel/mock/{slug}` com um evento de teste.
- **Com Evolution real:** `docker compose --profile omnichannel up -d evolution` (sobe evolution+evolution-db) → parear o número de teste **42984138129** lendo o QR pelo painel. Precisa: `EVOLUTION_API_KEY` e `EVOLUTION_DB_PASSWORD` no `.env`, e confirmar a tag da imagem.

---

## 2. Bloqueadores para o piloto rodar (em ordem)

### 2.1 Duas deps que o port precisa — ✅ RESOLVIDAS (2026-07-17)
O port trouxe dois imports sem dep declarada: `@emoji-mart/data` (`useInboxChatEmojiCatalog.ts:118`) e
`sass-embedded` (1 componente usa `<style lang="scss">`). Ambos entraram no `web/package.json` + lock.

**Como foi feito (padrão para o host Windows — o lock é cross-platform, `npm install` no host quebra o
`npm ci` do container):** regenerar só o lock, num container montando o dir do host, com
`--package-lock-only` (não instala binário nativo):
```
MSYS_NO_PATHCONV=1 docker compose run --rm --no-deps \
  -v "//c/Users/Mike/Documents/Projects/fila-atendimento/web:/hostweb" -w /hostweb \
  web npm install --package-lock-only <pacote>
```
Depois `docker compose up -d --build --force-recreate web`.

### 2.2 Conflito **verbatim × pre-commit** — resolver ANTES do primeiro `git add`
`lint-staged.config.mjs` manda `web/**/*.{ts,vue,js,mjs}` para `eslint --fix` **e** `prettier --write`.
No primeiro commit, os **72 arquivos portados seriam reformatados** — destruindo exatamente a propriedade
byte-a-byte que justifica o port (D-B) e inviabilizando o `diff` contra o `web-reference`.

**Precisa de um ignore** para `web/app/{composables,components}/omnichannel/**` e
`web/app/pages/omnichannel/**` no lint-staged, até a F14 (que é quando o refactor é deliberado).

### 2.4 ⚠️ Bundle stale do web — `up -d --build web` NÃO basta
`compose watch` **não faz sync inicial** (só sincroniza mudança feita com o watch rodando). Código
que foi pro disco com o watch desligado **não entra no container**, e `up -d --build web` pode
rebuildar a imagem sem recriar o container (ficou "Up 22h" servindo a página de **demo** velha em vez
do inbox real). **Diagnóstico:** `docker compose exec web sh -c "cat /app/app/pages/omnichannel/index.vue | grep -c OmnichannelInboxModule"` — 0 = container stale. **Cura:** `docker compose up -d --build --force-recreate web`. Depois, **hard-refresh no browser** (Ctrl+Shift+R) — pode haver cache do bundle antigo.

### 2.3 Ordem de build (o orquestrador roda, **um de cada vez** — build paralelo trava o Docker Desktop)
```
docker compose build --no-cache api && docker compose up -d api    # --no-cache: migration nova é embed.FS
docker compose up -d --build web
```

---

## 3. Correções já aplicadas (não regredir)

### 3.1 🔴 Vazamento cross-tenant nas 10 rotas — **corrigido**
A F2 nasceu com `RequireAuth` (que **não valida membership**) + `accountScope` lendo `X-Account-Id`
**cru do cliente**. Qualquer usuário autenticado de qualquer conta leria conversas, mensagens e contatos
de **outra conta** trocando o header. O gate de módulo **não** fecha isso — ele checa se a conta do header
contratou o módulo, nunca se o usuário pertence a ela. Conversa de WhatsApp = dado pessoal (LGPD).

**Fix:** `RequireAuthWithAccount` (valida membership em `core.account_users`; o checker cobre
`platform_admin` e `agency_owner` — `auth/account_checker.go:23`) + `accountScope` lê só
`principal.AccountID`, que o middleware carimba.

**Causa raiz — vale para quem escrever módulo novo:** a casa tem **dois padrões** e o inseguro é o mais
copiado. `calendar/http.go` usa `RequireAuth` + header cru; `calendar/http_secrets.go:26` usa
`RequireAuthWithAccount` e o comentário descreve o ataque exato. O agente copiou o primeiro.

> **Fora do escopo deste módulo, mas registrado:** `calendar/http.go`, `automation/http.go`,
> `bio/http.go` e `cardapio/http.go` usam o mesmo padrão. **Não foi auditado** — pode haver defesa em
> outra camada. Merece verificação própria.

### 3.2 Badge da página mentia ao contrário — corrigido
Dizia "SEM BACKEND (F1) — os requests devolvem 404 de propósito", mas a F2 já entregou as rotas de leitura.
Painel dizendo que dado real é fake inverte o princípio 4 e treina o admin a ignorar o badge. Agora é
**"PARCIAL (F2)"**: leitura real; canal, tempo real, envio e ações ainda 404 (F4–F7).
**Atualizar a cada fase; remover ao fechar a F7.**

---

## 4. Pendências conhecidas (não são bugs — são dívida nomeada)

| Item | Onde | Alvo |
|---|---|---|
| **Permission keys declaradas, não aplicadas** — as 9 keys `omnichannel.*` existem mas nenhuma rota as exige; a plataforma **não tem middleware de permissão** (só `RequireAuth`/`RequireAuthWithAccount`/`RequireRoles`). Hoje qualquer membro da conta lê o inbox inteiro | `modules/omnichannel/module.go` | decidir na F6/F7 (middleware novo × check no service) |
| `docs/LEGADO.md` **não recebeu os vestígios da F1** (entrega 8): front sem backend, os 5 adaptadores de costura, arquivos >450 linhas, módulo fora de layer, e `assignedUserIds`/`userScopePolicy` hardcoded (A3 da F2) | `docs/LEGADO.md` | antes do commit |
| `socket.io-client` **não existe** no web: o import verbatim derruba o build do Vite. A F1 trocou por um **stub inerte** preservando a lógica dos 3 eventos | `useOmnichannelInboxRealtime.ts` | **F5** reescreve o arquivo sobre WS nativo |
| 460 warnings de `max-lines` no tree portado + 2 erros `no-useless-escape` herdados do legado | `web/app/**/omnichannel/**` | **F14** (esperado e consciente — não "consertar" antes) |
| ~~Teste de concorrência FIFO (risco 5)~~ | ~~`platform/jobs`~~ | ✅ **feito e passa** contra Postgres real (`TEST_DATABASE_URL=postgres://omni:omni_dev@localhost:5432/omni` `go test ./internal/platform/jobs/ -run FIFO`). Skippa sem a env — não é falha |
| `llm` sem client concreto e limits sem leitor | `platform/llm`, `platform/modules/limits.go` | ✅ **completados nesta sessão** (client OpenAI-compat + testes; leitor de limites + 409 `limit_exceeded`) |

---

## 5. Armadilhas de processo desta rodada (para a próxima)

- **Não afirmar autorização do dono dentro de prompt de subagente.** O classificador bloqueou 4 agentes por
  *instruction poisoning* — corretamente: da perspectiva do subagente, "o dono liberou X" é indistinguível
  de um ataque. Os docs já registram as decisões; o agente lê o doc. A asserção no prompt é desnecessária.
- **Agente bloqueado ≠ agente que falhou.** A F3 foi bloqueada e **não escreveu nada**, mas o workflow
  reportou "concluído". Conferir arquivo em disco antes de dar fase por feita.
- **Retorno estruturado pode falhar com o arquivo já escrito.** A `OMNI-F14.md` existe (29 KB) apesar de o
  agente ter estourado o retry cap do schema. Checar o disco antes de re-executar.
