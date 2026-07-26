# CI-01 — Binding canal ↔ cliente

- **Status:** READY — implementação local aditiva autorizada
- **Versão:** 0.1
- **Data-base:** 2026-07-23
- **Owner proposto:** Omnichannel
- **Dependências:** CI-00; catálogo permission-scoped de clientes; `messaging.*`
- **Autoriza implementação:** sim; cutover e backfill de produção permanecem bloqueados

> Esta spec descreve criação aditiva e cutover futuro. Não reserva número de migration, não
> autoriza backfill no banco e não altera o worktree atual.

## 1. Resultado único e verificável

Tornar o cliente proprietário do canal, conversa e touchpoint independente de
`messaging.automation_profiles`, preservando histórico quando um número/account de Instagram for
reatribuído.

Ao final:

- habilitar/desabilitar ou excluir configuração de IA não muda ownership do canal;
- uma conversa nova recebe `client_account_id` do binding operacional ativo;
- uma conversa antiga conserva o binding que valia quando foi criada;
- chat humano funciona sem automation profile;
- binding ambíguo ou ausente nunca escolhe cliente silenciosamente.

## 2. Estado atual medido no disco

| Entidade | Estado atual |
|---|---|
| WhatsApp | `messaging.whatsapp_instances(account_id,id,provider,is_active,...)` |
| Instagram | `messaging.instagram_accounts(account_id,id,is_active,...)` |
| conversa | possui `account_id`, `instance_id`, `instance_scope_key`; não possui cliente explícito |
| touchpoint | possui `account_id`, contato/conversa/origem; não possui cliente explícito |
| perfil de automação | `messaging.automation_profiles(account_id,client_account_id,whatsapp_instance_id,ai_agent_id,enabled,...)` |
| unicidade MVP | um profile por cliente e um profile por WhatsApp instance |
| resolução atual | `AutomationClientForInstance`/`AutomationConversationScope` dependem do profile |
| catálogo de clientes | `tenants.Service.ListAccessible(... ModuleID:"omnichannel")` via adapter |
| sender | continua no Omnichannel e não é alterado por esta spec |

Não existe binding equivalente para `messaging.instagram_accounts`. Não foram obtidas contagens de
profiles, órfãos ou conversas; o pacote de inventário deve produzi-las antes do backfill.

### 2.1 Leitura obrigatória para execução

- migrations `0200`, `0211`, `0217`, `0227`, `0228`, `0229` e migrations posteriores que alterem
  essas entidades;
- `back/internal/modules/omnichannel/AGENT.md`;
- `module.go`, `service_inbound.go`, `store_webhook_events.go`;
- `automation_model.go`, `automation_store.go`, `automation_attendances.go`;
- stores/models de conversation, touchpoint, WhatsApp instance e Instagram account;
- adapter do catálogo de clientes no composition root;
- CI-00 e estado real do banco alvo.

Leitura de arquivo adicional para rastrear consumidor é permitida; escrita continua limitada aos
pacotes da seção 10.

## 3. Requisitos derivados das decisões canônicas

Estes requisitos materializam principalmente `CI-DEC-005`, `CI-DEC-007`, `CI-DEC-012` e
`CI-DEC-014`; não criam uma série paralela de decisões:

- binding pertence ao Omnichannel e não ao Customer Data/Intelligence;
- binding é histórico por intervalo; reatribuição encerra um registro e cria outro;
- conversa e touchpoint guardam snapshot de `client_account_id` e referência ao binding;
- `automation_profiles` vira consumidor do binding; nunca sua fonte;
- novo canal não pode ser ativado sem binding válido;
- durante backfill, linha sem resolução fica `unresolved`; não recebe IA nem Customer Data;
- unresolved não impede persistência/dedupe do inbound nem acesso humano owner-scoped;
- cliente do binding vem do catálogo permission-scoped e mesma organização;
- mudança de binding exige motivo, idempotency key e optimistic revision;
- nenhuma reatribuição retroativa ocorre por default.

## 4. Modelo candidato

### 4.1 `messaging.channel_client_bindings`

| Coluna | Tipo | Regra |
|---|---|---|
| `id` | uuid PK | `gen_random_uuid()` |
| `account_id` | uuid not null | FK `core.accounts`; workspace owner |
| `client_account_id` | uuid not null | FK `core.accounts`; validado pelo service |
| `channel` | text not null | `WHATSAPP` ou `INSTAGRAM` |
| `whatsapp_instance_id` | uuid nullable | preenchido somente para WhatsApp |
| `instagram_account_id` | uuid nullable | preenchido somente para Instagram |
| `effective_from` | timestamptz not null | início inclusivo |
| `effective_to` | timestamptz nullable | fim exclusivo; null = ativo |
| `source` | text not null | `manual`, `automation_profile_backfill`, `standalone_default` |
| `source_ref` | text nullable | ID externo/legado sanitizado |
| `reason` | text not null | motivo auditável, com limite |
| `revision` | bigint not null | começa em 1, optimistic locking |
| `created_by_user_id` | uuid nullable | FK `core.users` |
| `ended_by_user_id` | uuid nullable | FK `core.users` |
| `created_at`, `updated_at` | timestamptz | server-side |

Constraints:

- CHECK exige exatamente a FK do recurso correspondente ao `channel`;
- FKs compostas `(account_id, whatsapp_instance_id)` e
  `(account_id, instagram_account_id)`;
- ação `on delete` de owner/client permanece bloqueada por `CI-DEC-016`; candidato inicial é
  `restrict` para não apagar ownership histórico silenciosamente;
- `effective_to is null or effective_to > effective_from`;
- unique partial de binding ativo por `(account_id, whatsapp_instance_id)`;
- unique partial equivalente por Instagram;
- unique `(account_id,id)` para FKs compostas;
- unique `(account_id,client_account_id,id)` para o snapshot validar também o client;
- nenhuma constraint tenta inferir mesma organização; isso é service + catálogo.

Índices:

- `(account_id, client_account_id, effective_to, effective_from desc)`;
- `(account_id, channel, effective_to, updated_at desc)`;
- índices parciais ativos dos dois recursos.

### 4.2 `messaging.channel_client_binding_events`

Trilha imutável das operações administrativas e sua idempotência:

| Coluna | Regra |
|---|---|
| `id`, `account_id`, `binding_id` | IDs tenant-scoped |
| `successor_binding_id` | preenchido em reatribuição |
| `event_type` | `created`, `reassigned`, `ended` |
| `reason` | obrigatório e bounded |
| `idempotency_key` | unique por account |
| `actor_user_id` | usuário autorizado |
| `snapshot` | IDs/intervalo/revisions, sem credencial/PII |
| `occurred_at` | server-side |

Repetir create/reassign/end com a mesma key devolve o resultado desse evento. Uma key nunca pode
ser reutilizada com outro payload/hash.

### 4.3 Snapshot na conversa

Adicionar de forma inicialmente nullable em `messaging.conversations`:

| Coluna | Regra |
|---|---|
| `client_account_id uuid` | FK `core.accounts`, snapshot histórico |
| `channel_client_binding_id uuid` | FK composta `(account_id,id)` |
| `client_binding_state text` | `resolved`, `unresolved`, `quarantined` |
| `client_bound_at timestamptz` | instante real da resolução |

Regras:

- `resolved` exige client e binding;
- `unresolved` exige ambos nulos;
- `quarantined` preserva referências encontradas, mas bloqueia projeção/IA;
- FK composta `(account_id,client_account_id,channel_client_binding_id)` impede snapshot apontar
  para o client errado;
- após backfill 100% e janela aprovada, migration separada pode tornar os campos obrigatórios para
  novos registros; histórico unresolved não é apagado;
- mudança de binding ativo não altera conversa existente.

Índices:

- `(account_id, client_account_id, last_message_at desc)`;
- `(account_id, client_binding_state, last_message_at desc)`;
- `(account_id, channel_client_binding_id)`.

### 4.4 Snapshot no touchpoint

Adicionar inicialmente nullable em `messaging.contact_touchpoints`:

- `client_account_id uuid`;
- `channel_client_binding_id uuid`;
- `client_binding_state`;

Touchpoint com conversa herda seu snapshot. Touchpoint independente resolve pelo recurso/canal do
evento. Não usa o profile de IA. O mesmo FK composto account+client+binding é obrigatório quando o
estado é `resolved`.

### 4.4.1 `messaging.channel_client_binding_repair_jobs`

Job administrativo durável para preview/aplicação:

- id/account/channel resource/client/binding alvo;
- `mode=preview|apply`, `status=queued|processing|completed|partial|failed|cancelled`;
- filtros fechados e watermark temporal;
- `preview_job_id`, `preview_checksum` e `idempotency_key`;
- contagens scanned/eligible/repaired/quarantined/skipped;
- cursor/attempts/lease/error code;
- `report_ref` privada sem PII no log;
- actor, reason e timestamps.

Apply exige preview concluído/checksum igual e nunca altera linha que ficou inelegível, recebeu
resposta humana protegida ou mudou revision desde o preview. Unique account + idempotency key e
worker batch idempotente impedem reparo duplicado.

### 4.5 Auditoria e outbox

Não criar segunda outbox sem decisão explícita. Primeira proposta:

- inserir job/evento ID-only no `messaging.outbox` na mesma transação do binding/conversa;
- `kind` versionado e handler próprio, sem chamar provider de canal;
- `ordering_key = "integration:"+aggregate_id`;
- `idempotency_key = topic+":"+event_id`;
- consumidor registra `eventId` único antes de projetar.

Se a inspeção de CI-05 demonstrar que compartilhar `messaging.outbox` prejudica FIFO/latência do
sender, parar e aprovar uma outbox de integração separada. Não trocar silenciosamente.

## 5. Regras de resolução

### 5.1 Novo inbound

1. autenticar provider e resolver `account_id` no servidor;
2. deduplicar evento;
3. localizar recurso de canal dentro de `account_id`;
4. localizar exatamente um binding ativo no instante do evento;
5. copiar binding/client para conversa e touchpoint;
6. persistir mensagem;
7. inserir evento de integração na mesma transação;
8. somente após commit, despachar processamento assíncrono.

Resultados:

| Condição | Efeito |
|---|---|
| um binding válido | conversa `resolved`; fluxo humano/IA segue seus gates |
| nenhum binding | `unresolved`; inbox humano owner-scoped, IA e projeção externa bloqueadas |
| mais de um binding | erro de integridade + `quarantined`; nunca escolhe o mais recente |
| client inativo/fora do catálogo | `quarantined`, alerta e sem IA |
| profile de IA ausente/desabilitado | ownership permanece; chat humano funciona |

### 5.2 Conta standalone

Para account ativa não-agência:

- criação/ativação do recurso pode criar binding explícito `client_account_id=account_id`;
- o registro continua obrigatório e auditável;
- não existe fallback runtime invisível para `account_id`.

### 5.3 Agência

- cliente é obrigatório no formulário/API;
- deve estar ativo, acessível e na mesma organização autorizada;
- pertencer à mesma organização não é suficiente: precisa constar no catálogo permission-scoped;
- reatribuição nunca move conversas/touchpoints antigos.

## 6. Backfill e relatório de exceções

### 6.1 Fonte inicial

Para WhatsApp, `messaging.automation_profiles` fornece somente candidato:

```text
account_id + whatsapp_instance_id -> client_account_id
```

O candidato só vira binding se:

- owner/client existem e estão ativos;
- pertencem ao escopo organizacional autorizado;
- o recurso pertence ao mesmo account;
- não existe binding conflitante;
- não há outro dado contraditório inventariado.

Instagram exige configuração manual ou fonte autoritativa aprovada; não deriva de username/nome.

### 6.2 Classificação obrigatória

| Classe | Ação |
|---|---|
| `resolvable_unique` | pode entrar no backfill |
| `orphan_instance` | relatório; nenhuma escolha |
| `missing_client` | relatório; nenhuma escolha |
| `cross_scope` | quarentena e incidente |
| `conflicting_candidate` | revisão manual |
| `instagram_unbound` | configuração manual |
| `conversation_without_resource` | revisão/manual |
| `contact_mixed_clients` | encaminha para CI-02; sem correção automática |

Relatório por account/client:

- quantidade de recursos, profiles e bindings candidatos;
- active/inactive;
- conversas/touchpoints por classe;
- primeiro/último timestamp;
- IDs opacos necessários para revisão;
- checksum e watermark;
- zero PII bruta.

### 6.3 Ordem

1. inventário read-only;
2. DDL aditivo;
3. binding em `shadow`;
4. backfill de bindings;
5. backfill de conversa/touchpoint em batches;
6. comparação;
7. bloquear ativação de novo recurso sem binding;
8. trocar resolução nova para binding;
9. trocar `AutomationClientForInstance`/`AutomationConversationScope`;
10. somente depois avaliar constraints finais.

## 7. APIs

Todas usam account do Principal/header já validado; body nunca aceita `accountId`.

### 7.1 Listar

`GET /v1/omnichannel/channel-client-bindings`

Query allowlisted:

- `clientAccountId`;
- `channel`;
- `state=active|ended`;
- `cursor`;
- `limit` com cap.

Resposta:

```json
{
  "items": [{
    "id": "uuid",
    "clientAccountId": "uuid",
    "channel": "WHATSAPP",
    "channelResource": {
      "type": "whatsapp_instance",
      "id": "uuid",
      "label": "string"
    },
    "effectiveFrom": "RFC3339",
    "effectiveTo": null,
    "source": "manual",
    "reason": "string",
    "revision": 1,
    "createdAt": "RFC3339",
    "updatedAt": "RFC3339"
  }],
  "hasMore": false,
  "nextCursor": ""
}
```

Permissão: `omnichannel.instances.manage`. Fora do client scope retorna 404.

### 7.2 Criar

`POST /v1/omnichannel/channel-client-bindings`

```json
{
  "clientAccountId": "uuid",
  "channel": "WHATSAPP",
  "channelResourceId": "uuid",
  "effectiveFrom": "RFC3339 opcional",
  "reason": "string",
  "idempotencyKey": "string"
}
```

- o service valida recurso, catálogo, intervalo e conflito;
- repetição da mesma idempotency key retorna o mesmo resultado;
- conflito com binding ativo retorna 409 sem revelar recurso fora de escopo.

### 7.3 Reatribuir

`POST /v1/omnichannel/channel-client-bindings/{id}/reassign`

```json
{
  "targetClientAccountId": "uuid",
  "effectiveAt": "RFC3339",
  "reason": "string",
  "expectedRevision": 1,
  "idempotencyKey": "string"
}
```

Uma transação encerra o binding anterior e cria o sucessor. Não executa UPDATE retroativo em
conversas/touchpoints.

### 7.4 Encerrar

`POST /v1/omnichannel/channel-client-bindings/{id}/end`

Body: `effectiveAt`, `reason`, `expectedRevision`, `idempotencyKey`.

Canal ativo não pode ficar sem binding por erro administrativo: o service exige desativação
coordenada do recurso ou sucessor na mesma operação.

### 7.5 Exceções e reparo assistido

| Método/rota | Resultado |
|---|---|
| `GET /v1/omnichannel/channel-client-binding-exceptions` | recursos/conversas `unresolved|quarantined`, contagens e reason codes sem PII |
| `POST /v1/omnichannel/channel-client-binding-exceptions/resolve` | cria binding para o recurso e evita novos órfãos |
| `POST /v1/omnichannel/channel-client-binding-repair-previews` | calcula impacto/checksum/ambiguidades sem escrever |
| `POST /v1/omnichannel/channel-client-binding-repair-jobs` | aplica somente linhas aprovadas do preview, em batches idempotentes |
| `GET /v1/omnichannel/channel-client-binding-repair-jobs/{id}` | progresso, contagens e relatório |

Todas exigem `omnichannel.instances.manage`. Resolve recebe channel/resource/client, reason,
effectiveAt e idempotency key; não reassocia histórico silenciosamente. Repair exige
`previewId/checksum`, confirmação, revision e mantém ambiguidades em quarentena. Conversa que
recebeu resposta humana não muda client sem política/relatório explícitos.

### 7.6 Painel administrativo

O drawer de configuração do Omnichannel possui uma aba “Clientes por canal”, independente de IA,
com:

- channel resource, provider, binding efetivo, cliente, vigência e revision;
- criar, reatribuir e encerrar com diff/impacto/reason/confirm;
- fila `unresolved|quarantined` com reason codes e filtros;
- preview/reparo em job com contagens, checksum e download de relatório sem PII;
- loading, vazio, erro, stale, conflito de revision, retry e troca de account;
- link contextual opcional no Customer Intelligence, sem duplicar formulário/API.

O frontend nunca escolhe account por body, corrige conversa localmente ou usa IA para inferir
cliente. A aba continua funcional com Customer Intelligence ausente/desabilitada.

## 8. Eventos

| Tópico | Payload ID-only |
|---|---|
| `omnichannel.channel_binding.created` | binding, client, resource type/id |
| `omnichannel.channel_binding.reassigned` | old/new binding e old/new client |
| `omnichannel.channel_binding.ended` | binding, client, resource |
| `omnichannel.conversation.client_bound` | conversation, contact, binding, client |

Todos incluem event/account/client/correlation/idempotency/schema version. Dados de conversa,
telefone, nome, prompt e mensagem não entram no payload.

## 9. Permissões, tenant e concorrência

- handler valida UUID/formato; service valida autorização, catálogo e invariantes;
- repository repete `account_id` em SELECT/UPDATE/FK;
- recurso fora de account/client scope retorna 404;
- create/reassign/end usam transação e optimistic revision;
- unique partial impede dois bindings ativos mesmo com requests concorrentes;
- webhook e alteração administrativa concorrentes usam o binding válido pelo `occurred_at` do
  evento, com regra de borda `[effective_from,effective_to)`;
- cache key inclui `account_id`, canal, resource ID e versão;
- invalidar cache somente após commit.

## 10. Pacotes atômicos e allowlists de escrita

`<NEXT_*>` precisa ser resolvido para um nome numérico exato pelo orquestrador após reinspecionar
o disco. Executor não inicia com placeholder.

### CI01-DB-ADDITIVE

- **Escreve somente:**
  - `back/internal/platform/database/migrations/<NEXT_BINDING>_messaging_channel_client_bindings.sql`
  - `back/database/ERD.md`
  - `back/database/AGENT.md`
- **Entrega:** tabela, colunas nullable, constraints e índices aditivos.
- **Proibido:** backfill, NOT NULL final, drop, rename e qualquer arquivo social-publishing.
- **Stop:** ERD/AGENT ainda sujos por outra trilha.

### CI01-BE-DOMAIN

- **Escreve somente:**
  - `back/internal/modules/omnichannel/channel_client_binding_model.go`
  - `back/internal/modules/omnichannel/channel_client_binding_service.go`
  - `back/internal/modules/omnichannel/channel_client_binding_store.go`
  - `back/internal/modules/omnichannel/channel_client_binding_service_test.go`
  - `back/internal/modules/omnichannel/channel_client_binding_store_test.go`
  - `back/internal/modules/omnichannel/AGENT.md`
- **Proibido:** inbound, automation, module wiring e migration.

### CI01-API

- **Escreve somente:**
  - `back/internal/modules/omnichannel/http_channel_client_bindings.go`
  - `back/internal/modules/omnichannel/http_channel_client_bindings_test.go`
  - `back/internal/modules/omnichannel/module.go`
  - `back/internal/modules/omnichannel/AGENT.md`
- **Dependência:** domain aprovado.

### CI01-REPAIR

- **Escreve somente:**
  - `back/internal/modules/omnichannel/channel_client_binding_repair.go`
  - `back/internal/modules/omnichannel/channel_client_binding_repair_job.go`
  - `back/internal/modules/omnichannel/http_channel_client_binding_repairs.go`
  - respectivos testes
  - `back/internal/modules/omnichannel/module.go`
  - `back/internal/modules/omnichannel/AGENT.md`
- **Dependência:** DB/domain/API aprovados.
- **Proibido:** reparo sem preview/checksum, DDL, IA, sender ou reatribuição histórica silenciosa.

### CI01-FE-BINDING

- **Escreve somente:**
  - `web/app/components/omnichannel/config/OmnichannelConfigDrawer.vue`
  - `web/app/components/omnichannel/config/ConfigChannelClientBindings.vue` (novo)
  - `web/app/components/omnichannel/config/ConfigChannelClientBindingExceptions.vue` (novo)
  - `web/app/composables/omnichannel/useOmnichannelChannelClientBindings.ts` (novo)
  - `web/app/domain/omnichannel/channel-client-binding-api.ts` (novo)
  - `web/app/domain/omnichannel/channel-client-binding-types.ts` (novo)
  - testes correspondentes
- **Entrega:** aba independente de IA para listar/criar/reatribuir/encerrar/reparar.
- **Proibido:** fonte local autoritativa, inferência de client por IA, sender e alteração de
  `AutomationAiConfigDrawer`.

### CI01-BACKFILL

- **Escreve somente:**
  - `back/cmd/channel-client-binding-backfill/main.go`
  - `back/internal/modules/omnichannel/channel_client_binding_backfill.go`
  - `back/internal/modules/omnichannel/channel_client_binding_backfill_test.go`
  - `docs/customer-intelligence/evidence/CI-01_BACKFILL.md`
- **Regra:** dry-run default; escrita exige flag, DB alvo confirmado e relatório aprovado.
- **Proibido:** DDL e cutover.

### CI01-INBOUND

- **Escreve somente:**
  - `back/internal/modules/omnichannel/service_inbound.go`
  - `back/internal/modules/omnichannel/store_webhook_events.go`
  - testes existentes/novos diretamente correspondentes nesses dois packages/files
  - `back/internal/modules/omnichannel/AGENT.md`
- **Entrega:** resolução/snapshot/outbox na transação.
- **Proibido:** sender/outbound, IA, n8n e automation profile.

### CI01-AUTOMATION-SEAM

- **Escreve somente:**
  - `back/internal/modules/omnichannel/automation_store.go`
  - `back/internal/modules/omnichannel/automation_attendances.go`
  - respectivos testes
  - `back/internal/modules/omnichannel/AGENT.md`
- **Entrega:** ler client da conversa/binding, mantendo profile apenas como configuração IA.

### CI01-CUTOVER

- **Escreve somente após shadow aprovado:**
  - capability/policy server-side já aprovada;
  - `docs/customer-intelligence/evidence/CI-01_CUTOVER.md`;
  - eventual migration `<NEXT_ENFORCE>` resolvida exatamente para constraints finais.
- **Proibido:** drop de `automation_profiles`.

## 11. Compatibilidade

- `messaging.automation_profiles` permanece e continua configurando agente/close policy;
- APIs `/v1/omnichannel/automation/*` continuam funcionando;
- criação/edição do profile passa a exigir binding compatível;
- conversas antigas sem snapshot usam fachada legacy somente durante shadow;
- não há dual-write de ownership: após cutover, somente binding é autoridade;
- nenhuma FK de dispatch/close/handoff é alterada nesta spec.

## 12. Testes e comandos

A partir de `back/`:

```text
go test ./internal/modules/omnichannel/... -run 'ChannelClientBinding|Automation'
go test ./internal/modules/omnichannel/...
go test ./internal/platform/app/...
go test ./...
```

Quando `CI01-FE-BINDING` for despachado:

```text
npm --prefix web run test
npm --prefix web run lint
npm --prefix web run build
```

Cenários mínimos:

- profile desabilitado/ausente e conversa humana funcional;
- WhatsApp e Instagram com binding;
- standalone explícito;
- client fora da organização/catálogo → 404;
- dois creates concorrentes → um binding ativo;
- reassign preserva histórico anterior;
- webhook exatamente no instante de corte;
- retry idempotente não duplica binding/evento;
- ausência/ambiguidade → unresolved/quarantine sem IA;
- troca de account não reutiliza cache;
- backfill dry-run, órfão, conflito e checksum.
- painel funciona com IA/Customer Intelligence ausentes;
- create/reassign/end exigem confirmação, reason, revision e resposta autoritativa;
- exceção preview/apply mantém ambígua em quarentena;
- account switch cancela requests e não reutiliza binding anterior.

## 13. Observabilidade e auditoria

Métricas:

- bindings ativos/unresolved/quarantined;
- inbound sem binding por account/channel;
- divergência profile↔binding;
- atraso/backlog da outbox de integração;
- conflitos de unique/revision;
- recursos ativos sem binding.

Logs estruturados: operation, account, client quando conhecido, binding/resource ID, reason code,
correlation e erro. Sem telefone, mensagem, token ou body bruto.

## 14. Rollout

1. DDL nullable;
2. inventário dry-run;
3. bindings shadow;
4. backfill em batches;
5. comparar checksum e exceções;
6. exigir binding na criação/ativação de recurso novo;
7. habilitar leitura por client/channel;
8. trocar automation seam;
9. monitorar pelo menos a janela definida em CI-10;
10. avaliar constraint final separada.

## 15. Rollback

- antes do cutover: capability volta a `legacy`; bindings permanecem para análise;
- depois da troca de leitura: voltar leitura exige relatório de reconciliação, sem reescrever
  histórico;
- writer novo e antigo nunca ficam ativos juntos;
- não remover colunas/tabela no rollback;
- não reprocessar webhooks deduplicados;
- sender/outbox de canal não muda.

## 16. Critérios de aceite

- [ ] IA/profile não define ownership;
- [ ] chat humano funciona sem profile;
- [ ] todo recurso novo ativo possui binding explícito;
- [ ] conversa/touchpoint preservam client histórico;
- [ ] reatribuição não move histórico;
- [ ] backfill ambíguo não escolhe client;
- [ ] unresolved não alimenta IA/Customer Data;
- [ ] painel permite administrar bindings e fila de exceções sem depender de IA;
- [ ] reparo exige preview/checksum e deixa trilha auditável;
- [ ] account/client incompatível retorna 404;
- [ ] evento é durável e idempotente;
- [ ] nenhum provider, sender ou workflow foi alterado;
- [ ] nenhum drop foi executado.

## 17. Stop conditions

Parar se:

- CI-00 não estiver aprovada;
- DB alvo não estiver confirmado;
- próximo número de migration não estiver reconciliado;
- ERD/AGENT/migration estiver sob edição de outra trilha;
- catálogo não provar owner/client autorizados;
- existir mais de um candidato de client;
- implementation precisar bloquear/dropar mensagem inbound para “resolver” ownership;
- compartilhar `messaging.outbox` ameaçar FIFO/latência do sender sem decisão;
- reatribuição exigir UPDATE retroativo;
- automation profile precisar ser apagado para o binding funcionar;
- houver qualquer necessidade de tocar `social-publishing`, `automation` legado ou n8n.

## 18. Handoff obrigatório

Registrar por pacote:

- baseline/worktree e DB alvo;
- migration exata, tabelas/índices e status;
- contagens por classe e checksum;
- exceções não resolvidas;
- testes/resultados;
- capability efetiva por client/channel;
- prova de chat sem IA;
- rollback ensaiado;
- confirmação de zero drop/workflow/provider.
