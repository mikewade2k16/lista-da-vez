# Plano — Módulo de Atendimento WhatsApp (Omni)

**Documento canônico — fonte de verdade do módulo de Atendimento WhatsApp do Omni.**

- Criado: 2026-07-16
- Status: **PLANO** (nada implementado)
- Espelhado em: `web/app/components/roadmap/data/phases-part7.ts`
- Specs por fase: `docs/omnichannel/specs/OMNI-F*.md`
- Anexo técnico do front: [`PLANO_PORT_OMNICHANNEL.md`](PLANO_PORT_OMNICHANNEL.md) + [`SPECS_PORT_OMNICHANNEL.md`](SPECS_PORT_OMNICHANNEL.md)

> ## LIBERADO PARA IMPLEMENTAÇÃO — 2026-07-17 (decisão do dono)
>
> **A branch `refactor/multi-tenant-complete` fechou e o módulo está liberado para
> implementação.** O congelamento que vigorava aqui — "nenhuma fase começa antes do core sair
> de obra", a mesma regra que bloqueava o A1+ da automação
> (`docs/automation/PLANO_INTEGRACAO_OMNI.md`) — **não vale mais** (D-D, §2). Este documento
> deixa de ser desenho para depois: **é o plano de execução.** As fases correm respeitados
> apenas os *blockers* técnicos de cada uma (§9.2).

---

## 1. Objetivo e o que este documento é

Especificar o módulo de **Atendimento WhatsApp** do Omni: inbox humano, setores/filas com
atribuição, triagem por IA e multi-provider de canal — com o Go como fonte de verdade.

Este plano nasce da **fusão de dois trilhos que existiam separados**:

| Trilho | Origem | Estado | Papel na fusão |
|---|---|---|---|
| Port do inbox legado | `docs/omnichannel/` | PLANO, nada implementado | Vira o **FRONT** do inbox → rebaixado a **anexo técnico** |
| Spec externa MVP | `Omni_Atendimento_Spec_MVP_WhatsApp_v0.1.md` (não versionada) | Proposta | Rege o **BACKEND**: domínio, filas, triagem, segurança |

**Regra de leitura:** o que é contrato do front verbatim (paginação `limit`+`beforeId`, os 3
shapes de `message.updated`, as proteções do webhook, o mapa de rotas Node→Go) **não é
duplicado aqui** — vive no `PLANO_PORT_OMNICHANNEL.md` §7/§8 e no `SPECS_PORT_OMNICHANNEL.md`
F2/F4. Este doc **remete**. Duplicar contrato é criar duas verdades (princípio 1).

---

## 2. Decisões do dono — 2026-07-16 e 2026-07-17 (vinculantes)

Registradas com data para **ninguém re-decidir depois**. Quem discordar reabre com o dono,
não no código. **D-A/D-B/D-C são de 2026-07-16; D-D/D-E/D-F/D-G são de 2026-07-17.**

### D-A · Provider = adapter multi-provider

`meta_whatsapp_cloud` (oficial) + `evolution` / `waha` (não-oficial) + `mock`, com **escolha
por conta/número**.

- **Revisa a D1 do port** (`PLANO_PORT_OMNICHANNEL.md` §6), que recomendava Evolution
  single-provider. A D1 fica **superada** — o port não decide mais o provider.
- **Racional:** conta séria quer o número oficial (sem risco de ban); conta pequena/piloto
  quer o não-oficial (sem app review, sem custo por conversa). Cravar um provider força a
  escolha errada para metade da base. Um adapter custa uma interface; um provider errado
  custa uma migração.

### D-B · Fusão: port = front, spec = backend

- O **port verbatim** segue como caminho do **FRONT** do inbox (**67** arquivos byte a byte +
  5 repontados = **72 copiados**, mais a costura — contagem verificada no disco, ver
  `specs/OMNI-F1.md` C1). Inalterado quanto ao método — ver anexo técnico §5.
- A **spec externa** rege o **BACKEND**: domínio, setores/filas, triagem, segurança/LGPD, e
  as **telas novas de config** (que não existem no legado e por isso nascem no design system
  da casa, não verbatim).
- **Racional:** o front legado é código maduro em produção que ninguém aqui escreveu —
  reescrever enquanto porta é trocar dois problemas por quatro. Já o backend do legado é
  Fastify+Prisma com auth N+1 por request e base64 no Postgres: não é o que se quer manter.

### D-C · Arquitetura híbrida Go/PostgreSQL + n8n — revisada em 2026-07-20

Provider/modelo/chave/prompt/schema continuam no painel e no PostgreSQL. O Go autentica,
persiste, monta o contexto autoritativo, chama e revalida. O n8n executa a parte configurável
da inteligência: debounce, contexto evolutivo, modelo, multimodal, tools e produção da decisão
estruturada.

- Go/PostgreSQL continuam donos de dedupe, CRM, mensagens, estados, routing, auditoria,
  outbox e envio final.
- O n8n nunca recebe o webhook público autoritativo, nunca escreve no banco do produto e
  nunca envia para Evolution/Meta.
- `OMNI_AI_EXECUTOR=native|n8n` mantém o client nativo como rollback operacional enquanto
  o cérebro n8n amadurece; não cria duas fontes de configuração.
- O workflow é stateless e exportável: `pinData={}`, `staticData=null`, execução não salva e
  nenhuma credencial/chave versionada.
- **Racional:** a orquestração visual acelera a evolução do atendimento, mas as garantias
  críticas continuam no Go. Contrato: `ARQUITETURA_HIBRIDA_N8N.md`.

### D-D · Congelamento liberado — 2026-07-17

A branch `refactor/multi-tenant-complete` **fechou**. O congelamento sai: **o módulo está
liberado para implementação**.

- O **aviso ativo de bloqueio** some de todos os documentos do módulo (o do topo deste plano
  já saiu). O registro histórico de que houve congelamento permanece **só** onde explica uma
  decisão passada — o que morre é o bloqueio vigente.
- **Consequência:** o *blocker* "multi-tenant-complete" da **F0** (§9.2) deixa de existir.
  Restam apenas os *blockers* técnicos entre fases.
- **Racional:** a regra do congelamento era "módulo satélite não avança com o core em obra".
  O core saiu da obra; a premissa da regra acabou, e a regra com ela. Documento que mantém um
  bloqueio já revogado **mente** — e quem lê passa a não confiar em nenhum outro aviso dele.

### D-E · `PENDING` = 7º estado (opção A do Contrato 3.1 da OMNI-F8) — 2026-07-17

`pending` vira o **7º `state`** da máquina, com o **12º evento `human.pending`**
(`PATCH /conversations/{id}/status` → `PENDING`). Projeta `pending → PENDING` (§7.3).

- **Saídas de `pending`:** `msg.outbound.human` → `human_active`; `human.assign` →
  `human_active`; `conv.close` → `closed`.
- **`msg.inbound` em `pending` = `self`** (fica em `pending`): o rótulo é do **OPERADOR** — o
  cliente não o limpa.
- **Racional (verificado no legado):** `PENDING` é **rótulo manual do operador** — "parei
  nesta, estou esperando algo". Não tem produtor automático nem limpeza automática: é
  **ortogonal ao roteamento**. O candidato **`queued → PENDING` está descartado com
  evidência**: `queued` é produzido pelo **motor**, então mapeá-lo trocaria "filtro sempre
  vazio" por "filtro sempre cheio" — o rótulo perderia o significado que o operador lhe dá.
- **Consequências (aplicar, não re-decidir):** §7.2 (o `CHECK` nasce com 7 valores na **F2** —
  a F8 **não faz `ALTER`**); §7.3 (a projeção ganha `pending → PENDING`); a matriz do
  **Contrato 2 da F8** vira **7 × 12 = 84 pares** (era 6 × 11 = 66), e os contadores da F8
  (Entrega 8 e Verificável 8) vão de 66 para **84**; o **409 `invalid_transition` interino**
  para `PATCH status → PENDING` **deixa de existir** — virou transição válida.

### D-F · Código morto do port (D4 do port) fica de fora — 2026-07-17

`OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts` **não são portados**.

- **Revisa a D4 do port** (`PLANO_PORT_OMNICHANNEL.md` §6), que era decisão em aberto e
  travava a F0/F1 do port. A D4 sai de "pendente" e vira **DECIDIDA: fora**. A **F1 deixa de
  ter esse blocker**.
- **Consequência de contagem:** os arquivos copiados **byte a byte caem 2** — **67 verbatim**
  + 5 repontados = **72 copiados**. Quem cita contagem tem que bater. A conta, em uma linha:
  78 no disco (50 composables + 23 componentes + 5 pages) − **4 redirects** (a F1.1 manda não
  copiar) − **2 do D4** = **72 copiados** − 5 repontados = **67 byte a byte**. O "73 verbatim"
  do `PLANO_PORT_OMNICHANNEL.md` (§5.3, §9) parte de 78 − 5 repontados e **não desconta os 4
  redirects**: use **67/72**, número cravado e conferido no disco em `specs/OMNI-F1.md` C1.
- **Racional:** o componente **nunca renderiza, nem no legado** — as páginas que o chamariam
  redirecionam para fora do módulo. Não é remover funcionalidade (princípio 3): é **não
  importar código inalcançável**. Bônus: era a única dependência de
  `~/components/docs/ProjectDocsModule.vue`, que deixa de ser arrastado junto.

### D-G · `idempotency_key` é por conta — 2026-07-17

`outbox` usa **`unique(account_id, idempotency_key)`** — **não** UNIQUE global.

- **Racional:** a chave vem do **cliente**. Com UNIQUE global, a conta A colide com a chave da
  conta B e **suprime o envio dela** — fere o **princípio 2** (isolamento multi-tenant).
- A spec da **F3** já tinha divergido do canônico exatamente assim: **a divergência virou a
  norma** e o **§7.1 muda** (deixa de dizer `idempotency_key UNIQUE` global).
- Onde alguma spec exigia **prefixar a chave com o `account_id`** como mitigação do UNIQUE
  global, isso vira **desnecessário** — remover.

---

## 3. O que foi descartado da spec externa (e por quê)

A spec externa é sólida e alinhada aos princípios da casa (Go como verdade, n8n periférico,
adapter de canal, IA com saída validada + roteamento determinístico). **Mas re-especifica
metade de uma plataforma que já existe.** Descartado:

| Descartado da spec | Porque a plataforma já tem | Onde vive hoje |
|---|---|---|
| Modelo de `tenants` | `core.accounts` (tenants foi consolidada) | `core.accounts` + `X-Account-Id` no Principal |
| Modelo de `users` | `core.users` + sessão/JWT | módulo `core` |
| RBAC próprio | RBAC declarativo no banco, permissões seedadas pelo Module Registry no boot | `core.permissions` / `core.roles` |
| `memberships` | `core.account_users` + papéis/overrides | módulo `core` |
| Gate de módulo por conta | `core.account_modules` + `moduleGatingRules()` | `app.go:518` |

**Aceitar a spec inteira criaria uma segunda plataforma dentro da plataforma** — duas
tabelas de usuário, dois RBAC, duas verdades (fere o princípio 1 e o 2 de uma vez).

### Gaps reais que a spec expõe (e que a plataforma NÃO tem)

Isto é o que sobrou da spec — e é exatamente o valor dela:

| Gap | Fase | Nota |
|---|---|---|
| **Outbox/fila durável** | F3 | Nenhum módulo tem. O port previa a tabela só para o envio (F5); vira infra transversal |
| **Cifragem de segredos em repouso** | F3 | **Confirmado no código:** `calendar/secrets.go` mascara na saída (`{set,last4}`) mas grava a chave **crua**. Mascaramento é de saída, não é cifragem |
| **Setores / filas / atribuição** | F8 | O legado só tem `assign` manual — o port declara isso em §14 |
| **Política de mídia** | F13 | Retenção por classe, purge, masking |

---

## 4. Fronteira do módulo — independência

**UM módulo `omnichannel`** (schema `messaging.*`) cobre inbox + setores/filas + triagem IA.
Gate Go `/v1/omnichannel`; front `/omnichannel`; permissões `omnichannel.*`.

```
  ┌─ MÓDULO omnichannel (NOVO) ──────────────────┐
  │  schema messaging.*                          │
  │  inbox + filas + triagem IA no Go            │
  │  gate /v1/omnichannel                        │
  │  front /omnichannel                          │
  └──────────────────────────────────────────────┘
```

- **Independente por construção.** O módulo não lê, não escreve, não consulta e não depende
  do schema, da API nem do runtime de nenhum outro módulo. Todo dado dele nasce e vive em
  `messaging.*`.
- **`messaging.*` é a verdade do atendimento:** conversas, mensagens, contatos, instâncias,
  setores, filas, decisões de roteamento e execuções de IA.
- **Validação de número — interna, por conta:** o mesmo número de WhatsApp **não pode ser
  cadastrado em duas instâncias da mesma conta**. `UNIQUE(account_id, phone_number)`,
  validado **no cadastro da instância** (F4), não no runtime. É a única trava de número que
  este módulo implementa.
- **Integração com outros módulos é decisão futura, explicitamente fora deste plano** —
  quando o módulo fechar, se for preciso integrar, isso vira um plano próprio.

---

## 5. Governança — as 4 perguntas do dono (respondidas 2026-07-16)

### 5.1 Quais clientes acessam o módulo

`core.account_modules` — habilitado via **`PUT /v1/admin/accounts/{id}/modules`** (painel
admin, **já existe**: `back/internal/modules/core/admin_http.go:31`, exige `platform_admin`).

A conta-agência **auto-habilita no boot**: `EnableAllModulesOnAgencyAccounts`
(`back/internal/platform/modules/catalog_postgres.go:147`) roda a cada `SyncCatalog` como
invariante — CROSS JOIN `accounts(is_agency=true) × core.modules`, idempotente, não toca
contas-cliente. Ou seja: **o módulo novo aparece sozinho para a agência**, e a habilitação
por cliente continua sendo decisão de negócio no painel.

### 5.2 Quais usuários, e o que cada um faz

Permission keys `omnichannel.*`, seedadas pelo Module Registry no boot:

| Key | O que libera |
|---|---|
| `omnichannel.conversations.view` | Ver o inbox |
| `omnichannel.conversations.reply` | Responder |
| `omnichannel.conversations.assign` | Atribuir/transferir |
| `omnichannel.conversations.close` | Encerrar |
| `omnichannel.contacts.manage` | CRUD de contatos |
| `omnichannel.instances.manage` | Números/instâncias/providers |
| `omnichannel.settings.manage` | Setores, filas, regras de roteamento |
| `omnichannel.agents.manage` | Editor de agente de IA (publish/rollback) |
| `omnichannel.audit.view` | Trilha de auditoria |

Role templates: `attendant` · `supervisor` · `manager`.

> **Regra central — permissão gateia FEATURE; fila gateia DADO.**
> `conversations.view` não é "vê tudo": o atendente vê **só** as conversas das filas onde é
> `queue_member` **+** as atribuídas a ele. O filtro é **no repositório** (defesa em
> profundidade, princípio 2) — não só no service, não só no front.
> Conversa fora do escopo → **404, nunca 403**.

Nota de implementação (confirmada no código): a validação de chaves de permissão contra os
módulos habilitados já é um JOIN `core.permissions × core.account_modules` em
`InvalidPermissionKeys` (`back/internal/modules/core/rbac_repository.go:385`) — filtra por
`am.enabled = true` e `p.deprecated_at is null`. Permissão de módulo desabilitado é inválida
de graça, sem código novo.

**Armadilha conhecida:** `platform_admin` tem `has()` = false no front. Todo gating de menu /
seção precisa de `isPlatformAdmin || has(...)` — senão o módulo some justamente para quem
administra.

### 5.3 Quantos números por cliente

Limite em **`core.account_modules.config jsonb`** — a coluna **já existe**
(`0100_core_schema.sql:120`, `config jsonb not null default '{}'::jsonb`). Sem migration nova
para isso.

```json
{ "max_whatsapp_numbers": 2, "monthly_ai_runs": 5000 }
```

Defaults em `core.platform_settings`. Estouro de limite → **409** com erro acionável
(princípio 5: aviso honesto, não falha silenciosa).

### 5.4 A camada tradutora (`msg.text` e o resto)

Interface **`ChannelProvider`** + eventos canônicos. **O front e o domínio só veem o shape
canônico.** Mudança da Meta ou troca de provedor = ajustar **1 adapter**, zero mudança no
domínio, zero no front.

| Método | Papel |
|---|---|
| `VerifyWebhook` | Autenticidade do inbound (assinatura/token) — por provider |
| `ParseWebhook` | Payload bruto → evento canônico |
| `SendMessage` | Envio canônico → chamada do provider |
| `DownloadMedia` | Mídia do provider → disco |
| `Capabilities` | O que este provider/número sabe fazer (templates, janela 24h, reação, sticker…) |

`Capabilities()` é o que sustenta o multi-provider na UI: a tela **degrada por número**
(§12, risco 2), em vez de mentir que todo número faz tudo.

---

## 6. Arquitetura

```
 Meta Cloud ─┐
 Evolution  ─┼─webhook─> GO (verdade: Postgres) ─realtime─> PAINEL (front verbatim)
 WAHA       ─┤            │  ChannelProvider (adapter)          │
 mock       ─┘            │  outbox ─> worker ─> SendMessage    └── telas de config (novas)
                          │  LLM nativo (triagem) ─> motor determinístico decide
                          │
                          └──> n8n (só integrações periféricas — NUNCA no caminho crítico)
```

- **Go = a verdade.** Recebe o webhook, persiste, emite realtime, decide, envia. Todo dado e
  toda config no Postgres.
- **IA sugere; o motor decide.** A LLM devolve JSON validado contra schema versionado. **Quem
  roteia é código determinístico**, lendo `routing_rules`. LLM não escolhe fila sozinha —
  ela preenche campos; a regra decide. Isso é o que torna o roteamento auditável
  (`routing_decisions`) e testável sem chamar modelo.
- **`human_active` bloqueia a IA (hard-block).** Atribuiu a um humano, a IA cala. Substitui o
  `paused_until` da spec externa: janela de tempo expira sozinha e o bot volta a falar por
  cima do atendente — estado é mais honesto que timer.
- **n8n = periferia.** Sem lógica, sem prompt, sem config — e **nunca** no caminho crítico.
  Derrubar o n8n não para o atendimento nem a triagem.

---

## 7. Domínio (`messaging.*`)

**Migrations a partir de 0200** (última no disco: `0199_calendar_drop_day_media.sql`).
Regras — **SQL plano idempotente** (`IF NOT EXISTS`), schema-qualificado, **SEM
`-- +goose Down`**: o migrator roda o arquivo **inteiro** e o bloco Down se auto-destrói
(criou e dropou no mesmo boot — falha real, ver `0147_automation_contacts_fix.sql`).

`account_id uuid NOT NULL REFERENCES core.accounts(id)` **em todas**. `account_id` **sempre**
do Principal, **nunca** do body. Fora de escopo → **404**.

### 7.1 Tabelas novas (deste plano)

| Tabela | Papel | Chave |
|---|---|---|
| `departments` | Setor (Vendas, Suporte…) | `UNIQUE(account_id, slug)` |
| `queues` | Fila dentro do setor | `UNIQUE(account_id, department_id, slug)` |
| `queue_members` | Quem atende cada fila | `UNIQUE(queue_id, user_id)` — **é o gate de dado** |
| `routing_rules` | Regra determinística de roteamento | ordenadas por prioridade |
| `routing_decisions` | **Auditoria de cada decisão** (entrada, regra que casou, saída) | por conversa |
| `ai_agents` | Agente de triagem | |
| `ai_agent_versions` | Versão publicada (publish/rollback) | |
| `ai_runs` | Execução: input, output, schema, **usage/custo** | base do custo por conta (F13) |
| `collect_field_defs` | Campos que a IA extrai | |
| `webhook_events` | Dedupe inbound | **`UNIQUE(provider, external_event_id)`** |
| `outbox` | Envio durável | **`unique(account_id, idempotency_key)`** (D-G — **não** é UNIQUE global), `ordering_key = conversation_id` |

### 7.2 Tabelas do port, alteradas — **nascem assim na F2**

Não é ALTER depois. A migration da F2 **já cria com as colunas de estado/fila/provider** —
menos migration, menos backfill, menos janela de inconsistência.

| Tabela | Colunas que nascem junto |
|---|---|
| `whatsapp_instances` | `provider` (**CHECK** `meta_whatsapp_cloud\|evolution\|waha\|mock`), `provider_config jsonb`, `credentials_ciphertext` |
| `conversations` | `state` (máquina da spec — **CHECK com os 7 valores**, ver abaixo), `department_id`, `queue_id`, `assigned_user_id`, `extracted_fields jsonb` |

**O `CHECK` de `conversations.state` nasce com os 7 valores** (D-E, 2026-07-17):
`new` · `ai_active` · `routing` · `queued` · `human_active` · **`pending`** · `closed`.
O `pending` entra **aqui, na F2** — a **F8 NÃO faz `ALTER`** para adicioná-lo. É a mesma regra
da seção: coluna que o domínio vai precisar nasce na migration, não vira backfill depois.

As demais tabelas do port (`messages`, `contacts`, `saved_stickers`, `audit_events`,
`hidden_messages`, `account_config`) vêm como estão no anexo técnico §7 — **não repetidas
aqui**.

### 7.3 `state` é a fonte de verdade; `status` é projeção derivada

O front verbatim conhece `status` — **3 valores: `OPEN` / `PENDING` / `CLOSED`** — e **não
pode ser tocado** (D-B). O domínio precisa da máquina de estados da spec. Solução: **`state`
manda; `status` é derivado na serialização.**

| `state` (verdade) | `status` (projeção p/ o front) |
|---|---|
| `new` · `ai_active` · `routing` · `queued` | `OPEN` |
| `human_active` | `OPEN` + `assignedTo` preenchido |
| **`pending`** | **`PENDING`** |
| `closed` | `CLOSED` |

- `assign` ⇒ `state = human_active` ⇒ **hard-block da IA** (substitui o `paused_until`).
- **`pending` é o 7º state** (D-E, 2026-07-17), alimentado pelo evento `human.pending`
  (`PATCH /conversations/{id}/status` → `PENDING`). É **rótulo manual do operador**,
  **ortogonal ao roteamento** — nenhum produtor automático o liga, nenhuma regra o desliga.
  Sai dele por `msg.outbound.human` / `human.assign` (→ `human_active`) ou `conv.close`
  (→ `closed`); `msg.inbound` em `pending` é **`self`** (o cliente não limpa o rótulo do
  operador).
- **Este é o ponto mais frágil da fusão** (§12, risco 4). A spec da **F8 tabela TODAS as
  transições** — sem exceção, sem "e os outros casos análogos".

---

## 8. Infra transversal (F3) — nasce em `platform/`, não dentro do módulo

Três peças que **não são do omnichannel** — são da plataforma. Nascem em `back/internal/platform/`
porque o segundo consumidor já é previsível (o calendário para segredos e LLM).

| Pacote | O que faz | Detalhe que não pode faltar |
|---|---|---|
| `platform/jobs` | Outbox + worker | `FOR UPDATE SKIP LOCKED`; retry/backoff **classificado**; **FIFO por `ordering_key`**; dead-letter |
| `platform/secretbox` | Cifragem em repouso | **AES-256-GCM**; chave via env `OMNI_SECRETS_KEY`; prefixo **`v1:`** para rotação; saída **sempre `{set,last4}`** |
| `platform/llm` | Client LLM | adapters `openai`/`gemini`/`glm`; **structured output validado contra schema versionado**; `usage` → `ai_runs` |

**Retry classificado** (herdado do legado, anexo técnico §9 e SPECS F5): transitório → 5;
401/403/404/405 e 400/422 conhecidos → **1 (unrecoverable)**; 429 → 5; 5xx → 4; sem status →
4; outros → 3. Monitor de presas >10 min **com filtro de conta** (o legado varre a tabela
inteira, sem tenant — não portar esse comportamento).

**Sobre o `secretbox` e o calendário:** `calendar/secrets.go` **já entrega `{set,last4}`** —
o contrato de saída está certo e é o modelo a seguir. O que ele **não** faz é cifrar em
repouso (grava a chave crua). **Migrar os segredos do calendário para o `secretbox` é
pendência registrada, NÃO bloqueante** deste plano — mas é a razão de o pacote nascer em
`platform/` e não em `omnichannel/`.

---

## 9. Fases F0–F14

Substituem OMNI-F0..F9 do port. **Tudo `pending`** — renumerar é seguro, nada foi
implementado. Cada fase tem entregável **verificável no browser**, não só compilando.

| # | Fase | Escopo | Prio |
|---|---|---|---|
| **F0** | Decisões + fundação | Registrar as 7 decisões (D-A…D-G), publicar este plano, LEGADO, roadmap | P0 |
| **F1** | Front verbatim + costura | **Inalterada do port** (**67** verbatim + 5 repontados = 72 copiados + badge SEM BACKEND) | P0 |
| **F2** | Go: schema `messaging.*` + leitura | Port + colunas de estado/fila/provider **já nascem na migration** | P0 |
| **F3** | **[NOVA]** Infra transversal | `platform/jobs` · `platform/secretbox` · `platform/llm` · limites | P0 |
| **F4** | ChannelProvider + adapters + webhook inbound | Interface da spec; adapters `mock` + `evolution` (1º real) | P0 |
| **F5** | Realtime | `/v1/realtime/omnichannel`, canal `omnichannel:account:{id}` | P0 |
| **F6** | Envio via outbox + mídia | `idempotency_key`, FIFO por conversa, mídia disco + stream `Range` | P0 |
| **F7** | Ações do inbox | reaction/forward/delete/status/assign via máquina de estados | P0 |
| **F8** | **[NOVA]** Domínio de atendimento | departments/queues/members/routing_rules/decisions + máquina + handoff | P0 |
| **F9** | **[NOVA]** Triagem IA híbrida | Go autoritativo + executor nativo/n8n + JSON schema-validado | P0 |
| **F10** | **[NOVA]** Telas de config | Números/providers, setores/filas/regras, editor de agente + simulador | P0 |
| **F11** | **[NOVA]** Meta WhatsApp Cloud | HMAC, verify token, templates + janela 24h, capabilities | P1 |
| **F12** | Stickers/GIF/avatar | **Inalterado do port** | P1 |
| **F13** | LGPD + observabilidade | Retenção + purge, masking, export/anonimização, custo LLM | **P0 mín.** |
| **F14** | Refactor | Split >450, virar layer, remover os 6 adaptadores de costura | P1 |

**Piloto P0 = F0 → F10 + F13-mínimo.**
**Paralelização:** F1 ∥ F2 ∥ F3 (independentes). F8 ∥ F4–F7 (domínio não depende do canal).

### 9.1 Mapa de renumeração (port → fusão)

Para não se perder ao consultar o anexo técnico:

| Port (`PLANO_PORT_OMNICHANNEL.md` §9) | Fusão | Mudou? |
|---|---|---|
| OMNI-F0 Decisões + fundação | **F0** | D1 revisada (multi-provider) |
| OMNI-F1 Front verbatim + costura | **F1** | Não |
| OMNI-F2 Go: schema + leitura | **F2** | Colunas de estado/fila/provider nascem junto |
| OMNI-F3 Evolution + inbound | **F4** | Vira `ChannelProvider` + adapters (não Evolution cravado) |
| OMNI-F4 Realtime | **F5** | Não |
| OMNI-F5 Envio + mídia | **F6** | Outbox sai para `platform/jobs` (F3) |
| OMNI-F6 Ações | **F7** | Passa pela máquina de estados (F8) |
| OMNI-F7 Stickers/GIF/avatar | **F12** | Não |
| OMNI-F8 IA no n8n | **F9** | Go autoritativo; n8n executa a orquestração configurável (D-C revisada) |
| OMNI-F9 Refactor | **F14** | Não |
| — | **F3, F8, F10, F11, F13** | **Novas** (da spec externa) |

### 9.2 Fase a fase

**F0 — Decisões + fundação.** Registrar **D-A…D-G** com data (as 4 novas são de 2026-07-17);
publicar este plano; `docs/LEGADO.md` (itens do §14); roadmap `phases-part7.ts` + `groups.ts`
+ `modules.ts`. *Verificável:* roadmap mostra F0–F14; LEGADO lista os adaptadores.
*Blockers:* **nenhum** — o bloqueio "multi-tenant-complete" caiu com a **D-D** (2026-07-17).

**F1 — Front verbatim + costura.** **Inalterada** no método — ver `SPECS_PORT_OMNICHANNEL.md`
F1 (F1.1 a F1.6); são **67** arquivos verbatim de **72 copiados** (a **D-F** tirou os 2 do
audit; ver a conta em `specs/OMNI-F1.md` C1). *Verificável:*
`/omnichannel` abre, parece o legado, requests 404, **badge "SEM BACKEND" visível para admin**.
*Blockers:* **nenhum** — a D4 do port está decidida (**D-F**: fora) e não trava mais; pode
correr em paralelo.

**F2 — Go: schema + leitura.** Migrations `messaging.*` a partir de **0200** (§7) + rotas de
leitura. Contratos e índices: anexo técnico §7/§8 e `SPECS_PORT_OMNICHANNEL.md` F2 —
**não duplicados aqui**. *Verificável:* inbox lista do banco; `X-Account-Id` de outra conta →
404. *Blockers:* nenhum.

**F3 — Infra transversal.** §8. *Verificável:* **teste de concorrência dedicado** provando
FIFO por conversa com N workers (§12, risco 5); segredo gravado cifrado (`v1:`) e lido; LLM
devolve JSON validado. *Blockers:* nenhum.

**F4 — ChannelProvider + adapters + webhook inbound.** Interface (§5.4); adapters `mock` e
`evolution`; webhook inbound + dedupe (`webhook_events`); **`UNIQUE(account_id, phone_number)`
validado no cadastro da instância** (§4). Proteções do webhook: `SPECS_PORT_OMNICHANNEL.md` F3.
*Verificável:* QR no painel, conectar, mandar msg do celular e ela existir em
`messaging.messages`; cadastrar o mesmo número 2× na conta → 409; webhook sem assinatura →
401; evento repetido não duplica. *Blockers:* F2, F3.

**F5 — Realtime.** `/v1/realtime/omnichannel`, canal `omnichannel:account:{id}`. Os 3 eventos
e os shapes por call-site: `SPECS_PORT_OMNICHANNEL.md` F4 — **replicar sem unificar**.
*Verificável:* duas abas em contas diferentes; msg aparece ao vivo só na conta certa.
*Blockers:* F4.

**F6 — Envio via outbox + mídia.** `POST .../messages` → outbox (`idempotency_key`, FIFO por
conversa); mídia em **disco + stream com `Range`** (D2 do port). *Verificável:* responder do
painel e chegar no celular; derrubar o provider → FAILED após os retries; enviar 2× com a
mesma `idempotency_key` → 1 mensagem. *Blockers:* F3, F4.

**F7 — Ações do inbox.** reaction/forward/delete-for-me/delete-for-all/status/assign — **via
máquina de estados** (F8), não escrevendo `status` na mão. *Verificável:* cada botão da UI
funciona. *Blockers:* F6, F8.

**F8 — Domínio de atendimento.** `departments`/`queues`/`queue_members`/`routing_rules`/
`routing_decisions`; **máquina de estados com TODAS as transições tabeladas**; projeção
`state → status`; handoff IA→humano. *Verificável:* conversa entra → cai na fila certa por
regra; atendente de outra fila **não vê** (404); `routing_decisions` explica cada decisão.
*Blockers:* F2. **Pode correr ∥ F4–F7.**

**F9 — Triagem IA híbrida.** `ai_agents`/`ai_agent_versions`/`ai_runs`; prompt em 8 camadas;
saída **JSON schema-validado**; **IA sugere → motor Go decide**; `human_active` = hard-block.
O executor principal evolui no n8n stateless, enquanto o Go resolve configuração/chave,
limita contexto, revalida, audita e aplica FSM/routing/outbox. O client nativo permanece como
rollback via `OMNI_AI_EXECUTOR`. *Verificável:* msg → IA extrai campos → regra roteia; trocar
prompt/modelo/chave no painel muda o comportamento; workflow exportado não contém segredo,
memória nem node de canal; atribuir a humano → IA cala; trocar para `native` restaura o caminho
de contingência sem mudar dados. *Blockers:* F3, F8.

**F10 — Telas de config.** Números/providers, setores/filas/regras, editor de agente com
publish/rollback + **simulador mínimo**. **Design system da casa** (são telas novas — o
verbatim não se aplica). *Verificável:* configurar um número, um setor, uma regra e um agente
sem tocar no banco. *Blockers:* F4, F8, F9.

**F11 — Meta WhatsApp Cloud.** Adapter: **`X-Hub-Signature-256`** + verify token; templates +
janela 24h; `Capabilities()`. **Embedded Signup → P2** (exige app review). *Verificável:*
número oficial recebe e envia; fora da janela 24h a UI exige template. *Blockers:* F4.

**F12 — Stickers/GIF/avatar.** **Inalterado** — `SPECS_PORT_OMNICHANNEL.md` F7.

**F13 — LGPD + observabilidade.** §10. *Verificável:* job de purge apaga o que passou da
retenção; log não contém payload bruto; custo LLM por conta na tela. *Blockers:* F9.

**F14 — Refactor.** Split >450; `web/app/**` → layer; **remover os 6 adaptadores de costura**
(§14.2). *Verificável:* nenhum arquivo do módulo >450 linhas; o módulo carrega como layer; os
adaptadores não existem mais e o inbox segue funcionando. *Blockers:* F0–F13 verdes.

---

## 10. Segurança e LGPD

| Item | Regra |
|---|---|
| **Autenticidade inbound** | `VerifyWebhook` **por provider**. Meta → HMAC `X-Hub-Signature-256`. Evolution/WAHA → token com comparação **constant-time** |
| **Dedupe / idempotência** | Inbound: `webhook_events UNIQUE(provider, external_event_id)`. Outbound: `outbox unique(account_id, idempotency_key)` — **por conta, nunca global** (D-G): a chave vem do cliente, e UNIQUE global deixa uma conta suprimir o envio da outra |
| **Payload bruto** | **Mascarado. Nunca em log.** Nem em erro, nem em trace |
| **Retenção por classe** | 365 / 180 / 90 / 30 dias (default) + **job de purge**. Export/anonimização → P1 |
| **Rate limit por número** | Mitigação de ban do não-oficial — **mitigação, não garantia** (§12, risco 3) |
| **Credenciais** | **Só via `secretbox`.** Nunca em coluna crua, nunca em log, nunca de volta pro front (só `{set,last4}`) |
| **Escopo** | `account_id` do Principal; repositório filtra também; fora de escopo → **404** |

**Padrão de HMAC — o que existe hoje:** `back/internal/modules/site/http_ingest.go` já faz
webhook público autenticado por HMAC SHA-256 do body, com `hmac.Equal` (constant-time) e
`MaxBytesReader`. **É o modelo a seguir** — mas note a diferença: lá o header é
`X-Signature: sha256=<hex>` (padrão próprio, estilo GitHub/Stripe); a **Meta exige
`X-Hub-Signature-256`**. Mesma mecânica, header diferente — o adapter da F11 tem o seu.

---

## 11. Wire do módulo (checklist para as specs)

Sincronizar **todos** os pontos — senão dá drift (o menu esconde mas a rota abre, ou o
módulo some para quem administra). Todos confirmados no código:

### Go

| Ponto | Onde | Nota |
|---|---|---|
| `registry.MustRegister(omnichannel.New(...))` | `back/internal/platform/app/app.go` (~364+) | Padrão dos módulos existentes |
| `moduleGatingRules()` += `{Prefix: "/v1/omnichannel", ModuleID: "omnichannel"}` | `app.go:518` | `platform_admin` tem bypass; conta sem o módulo → 403 `module_disabled` |
| **Webhooks e runtime FORA do gate** | — | Precedente confirmado: `/v1/public/*` (bio, cardápio) e `/s/{slug}`, `/q/{slug}` (tools) não estão em `moduleGatingRules` |

`SyncCatalog` no boot registra as permissões **e** auto-habilita nas contas `is_agency`
(`catalog_postgres.go:147`).

### Front

| Ponto | Onde | Nota |
|---|---|---|
| `MODULE_PATH_GUARDS` += `{ prefix: '/omnichannel', moduleId: 'omnichannel' }` | `web/app/middleware/module-enabled.global.ts` | Rota direta também gated |
| `workspaces.ts` | `web/app/utils/workspaces.ts` | `icon` é **chave do `NAV_ICON_MAP`**, não nome livre — `messages` existe |
| `permissions.ts` (prefixo `omnichannel.`) | `web/app/domain/utils/permissions.ts` | `WORKSPACE_ACCESS_DEFINITIONS` + `ROLE_WORKSPACES` + `MODULE_WORKSPACE_PERMISSION_PREFIXES` |
| `nav.config.ts` — item já existe `hidden: true` → `beta` | `web/layers/queue/nav.config.ts:9-13` | **Confirmado no disco**: o item `omnichannel` existe e está escondido |
| `nuxt.config.ts` | `web/nuxt.config.ts:58` | Tem `'/omnichannel': { ssr: false }`; **falta `/omnichannel/**`** |
| Limpar o demo | `web/app/pages/omnichannel.vue` + `web/app/utils/demo-pages.ts:22` | Placeholder atual — remover na F1 |

**Não há AGENT.md a sincronizar ainda:** o módulo **não existe** em `back/internal/modules/`
nem em `web/app/components/`. O `AGENT.md` **nasce junto com o código** na F1/F2 — não antes.

---

## 12. Riscos

1. **Pricing e política da Meta.** Cobrança **por conversa** + quality rating que degrada o
   número. **Embedded Signup exige app review** → empurrado para **P2**. Risco de produto,
   não de código: quem decide preço é a Meta.
2. **Assimetria da janela de 24h.** Cloud **exige template** fora da janela; o não-oficial
   não tem essa restrição. **Resolver via `Capabilities()`** — a UI **degrada por número**,
   nunca oferece o que aquele número não faz. Sem isso, o atendente descobre o limite quando
   a mensagem falha.
3. **Ban do não-oficial.** Evolution/WAHA usam WhatsApp não-oficial: **conta séria usa
   Cloud.** Rate limit por número é **mitigação, não garantia**. Isto entra no contrato com o
   cliente, não só no código.
4. **A projeção `state → status` continua o ponto mais frágil da fusão.** O front verbatim não
   pode mudar (D-B) e o domínio precisa da máquina nova. Um estado que projete errado = inbox
   mostrando conversa fechada como aberta. **Mitigação: a spec da F8 tabela TODAS as
   transições** — nenhuma implícita. **A lacuna do `PENDING` foi FECHADA pela D-E**
   (2026-07-17): o terceiro valor de `status` deixou de ser um destino sem origem — `pending`
   é o 7º `state` e projeta `pending → PENDING` (§7.3), com o `CHECK` já nascendo com os 7
   valores na F2. O que **resta** de frágil é a cobertura: **7 × 12 = 84 pares** na matriz da
   F8, todos tabelados — nenhum "caso análogo".
5. **FIFO por conversa com múltiplos workers.** `SKIP LOCKED` dá throughput, mas duas
   mensagens da mesma conversa em workers diferentes podem inverter a ordem — o cliente vê a
   resposta antes da pergunta. **Mitigação: `ordering_key = conversation_id` + teste de
   concorrência dedicado na spec F3.**
6. **Colisão de número com sistema externo — risco OPERACIONAL, não gate de código.** A norma
   **um número = um cérebro** continua valendo como **regra de operação**: apontar o mesmo
   número de WhatsApp para dois sistemas ao mesmo tempo faz dois robôs responderem o mesmo
   cliente — incidente visível para o cliente final. **Este módulo não valida isso** — ele não
   consulta sistema externo (§4). A única trava que existe é interna:
   `UNIQUE(account_id, phone_number)` no cadastro da instância (F4). **Mitigação: é
   responsabilidade de quem opera** não apontar o mesmo número para dois sistemas — entra no
   procedimento de onboarding do número, não no código.

---

## 13. Notas de Deploy

**Ordem exata:** migrations → env vars → **build api (`--no-cache`)** → build web → container
do provider → Caddy.

| # | Item | Fase | Detalhe |
|---|---|---|---|
| 1 | Migrations `messaging.*` | F2+ | A partir de **0200** (última no disco: `0199`). Idempotentes, **sem `-- +goose Down`** |
| 2 | **`OMNI_SECRETS_KEY`** | F3 | **OBRIGATÓRIA — sem ela o módulo NÃO SOBE.** Fail-fast no boot, nunca default. AES-256 (32 bytes). Perder a chave = perder os segredos cifrados |
| 3 | `EVOLUTION_BASE_URL` · `EVOLUTION_API_KEY` · `EVOLUTION_WEBHOOK_TOKEN` | F4 | Avaliar API key **por conta** (multi-tenant) em vez de global do ambiente |
| 4 | `META_APP_SECRET` + verify token | F11 | Verify token do webhook da Meta |
| 5 | `WEBHOOK_RECEIVER_BASE_URL` | F4 | URL que o provider chama de volta |
| 6 | Container `evolution` | F4 | Novo serviço no compose (**profile próprio**), volumes + backup, **Caddy se precisar de rota pública** |
| 7 | Volume de mídia | F6 | Disco (D2). **Backup precisa incluir** |
| 8 | Rebuild da API | F2+ | Toda fase que mexe em `back/` → `docker compose up -d --build api` |

> **Armadilha que já queimou tempo — migration nova exige `docker compose build --no-cache api`.**
> As migrations são **`embed.FS`**: o cache da camada `go build` pode **não re-embutir** o
> `.sql` novo. Sintoma: `migrate status` para na migration anterior, sem erro.

**Higiene observada (não bloqueia, mas registrar):** existem **dois arquivos com o número
0197** — `0197_operation_validation_reason.sql` e `0197_tools_module.sql`. Não afeta este
plano (0200+ está livre), mas confirma que a numeração não é validada por ninguém: **conferir
o disco antes de numerar a 0200**, não presumir.

---

## 14. Legado/mock a marcar (princípio 4)

Entra em `docs/LEGADO.md` na F0/F1, com **badge admin visível na tela**:

1. **F1 = front sem backend.** Badge **"SEM BACKEND"** enquanto F2/F4 não fecharem.
2. **Os 6 arquivos de costura** — adaptadores, alvo de remoção em **F14**.
3. **Arquivos >450 linhas** (o port chega a 1.467) — violação **consciente**, alvo F14.
4. **Módulo em `web/app/` em vez de layer** — alvo F14.
5. **Segredos do calendário sem cifragem em repouso** — `calendar/secrets.go` grava a chave
   crua. Alvo: migrar para `platform/secretbox` **após a F3**. Não bloqueante.

---

## 15. O que este plano NÃO cobre

- **Instagram / Email / Webchat.** O legado só tem WhatsApp (o enum `INSTAGRAM` existe e não
  é implementado). O `ChannelProvider` **não fecha a porta**, mas não é escopo.
- **Embedded Signup da Meta** → P2 (exige app review).
- **Integração com outros módulos do Omni** → decisão futura: este módulo é independente e não
  conhece nenhum outro; se for preciso integrar depois que ele fechar, isso vira plano próprio.
- **`DELIVERED` / `READ`** — não existe no legado (`MessageStatus` é só
  `PENDING|SENT|FAILED`). É **feature nova**, não port.
- **Distribuição automática por carga/round-robin** — a F8 entrega roteamento por **regra
  determinística**; balanceamento é fase futura.
- **Migração dos segredos do calendário** — pendência registrada (§14.5), não bloqueante.

---

## Referência cruzada

- Anexo técnico do front → [`PLANO_PORT_OMNICHANNEL.md`](PLANO_PORT_OMNICHANNEL.md) · [`SPECS_PORT_OMNICHANNEL.md`](SPECS_PORT_OMNICHANNEL.md)
- Branch que **bloqueava** este plano (fechada — D-D, 2026-07-17) → [`../MULTITENANT_COMPLETION_PLAN.md`](../MULTITENANT_COMPLETION_PLAN.md)
- Princípios → [`../ENGINEERING_PRINCIPLES.md`](../ENGINEERING_PRINCIPLES.md) · skill `principios-engenharia`
- Roadmap (espelho) → `web/app/components/roadmap/data/phases-part7.ts`
