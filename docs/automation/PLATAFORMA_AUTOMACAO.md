# Plataforma de Automacoes (multi-tenant) — Visao e Roadmap

> Status: **design** (pending). Pensar grande, ir com calma.
> Visao canonica de para onde a automacao WhatsApp/IA evolui dentro do Omni: de
> "1 bot atendimento no n8n" para uma **plataforma onde cada cliente cria suas
> automacoes**. Generaliza o [PLANO_INTEGRACAO_OMNI.md](PLANO_INTEGRACAO_OMNI.md)
> (que vira o detalhe da PRIMEIRA automacao, "atendimento").
> Espelhado em [roadmap-data.ts](../../web/app/components/roadmap/roadmap-data.ts) e
> [automation/AGENT.md](../../automation/AGENT.md). Criado 2026-06-08.
> **Bloqueio:** as fases que viram modulo Go/banco aguardam o fechamento da
> `refactor/multi-tenant-complete` (regra do MULTITENANT_COMPLETION_PLAN). Este doc e o
> desenho para quando liberar; a infra (containers local+prod) ja esta pronta.

---

## 1. Visao

Transformar a automacao de WhatsApp/IA numa **plataforma multi-tenant** dentro do Omni:

- Cada **account** (cliente) pode ter **N automacoes** (robos) — se a plataforma liberar.
- Cada **automacao** = 1 numero de WhatsApp conectado + comportamento proprio (persona/
  instrucoes/knowledge) + modelos de IA por etapa + tools/conexoes opcionais + liga/desliga.
- **Tipos de automacao** sao templates (modulos): hoje so `atendimento`; depois outros
  (cobranca, pos-venda, SDR, agendamento...), cada um com seu workflow/template.
- Tudo configurado pelo **painel** — o cliente nunca edita o n8n na mao.
- **BYOK (bring your own key):** cada account usa a propria chave de provedor (OpenAI/
  Anthropic/...) e os **proprios creditos**. Modo "chave gerenciada" da plataforma (a gente
  cobra) fica como opcao futura.
- **Integra ao banco do Omni:** o agente puxa CRM/ERP/metricas de outros modulos via tools
  na API Go, cruzando dados — base para um **"super robo"** interno do time.
- **Multi-tenant + RBAC desde o primeiro commit** do modulo (`automation_id` central),
  mesmo shippando 1 automacao primeiro. Nada de construir single-tenant e refatorar depois.

### Principio condutor
O **comportamento vem do banco**, nao dos nos do n8n. O n8n (ou, no futuro, um motor Go)
e so o executor: a cada mensagem ele pergunta a API Go "quem e essa sessao, qual a config?"
e age. Trocar persona/modelo/knowledge/tools e mexer em linha de banco pelo painel.

---

## 2. Conceitos (glossario)

| Termo | O que e |
|---|---|
| **account** | O cliente/tenant (ja existe em `core.accounts`). Dono das automacoes. |
| **automation** | O "robo". 1 numero de WhatsApp + 1 comportamento. N por account. **Entidade central.** |
| **automation_type** | Template do robo (`atendimento`, futuros). Define o workflow base e quais campos sao configuraveis. |
| **channel** | Conexao do canal (WAHA session = 1 numero). 1 automacao -> 1 channel na V1 (extensivel a N). |
| **persona / instrucoes** | O "comportamento" estilo GPT (system prompt + guardrails) por automacao. |
| **knowledge base** | Arquivos/conhecimento da automacao -> RAG (pgvector). O "Knowledge" do GPT. |
| **model_config** | Modelo de IA por etapa/no (chat, visao, audio, classificador, triagem...). |
| **provider_credential** | Chave de API do provedor por account (BYOK), criptografada. |
| **tool / conexao** | Acesso a dados (CRM/ERP/metricas do Omni) ou API externa, escopado por account. |
| **service_token** | Token que o n8n usa no runtime; resolve a automacao/account (nunca vem do body). |
| **entitlement** | Liberacao por account: pode criar automacao? quantas? quais tipos? (gating do platform admin). |

---

## 3. Modelo de dados (automation-centric, schema `automation.*`)

> Mudanca-chave vs. o plano antigo: entra **`automations`** como entidade central; tudo
> (persona, canal, CRM, modelos, knowledge) passa a ser escopado por `automation_id`
> (+ `account_id` para o tenant). DDL final vira migration idempotente na implementacao.

| Tabela | Campos principais | Nota |
|---|---|---|
| `automations` | `id`, `account_id` (FK core.accounts), `type`, `name`, `slug`, `status` (draft/active/paused), `enabled`, `created_by`, timestamps | **Central.** N por account. |
| `entitlements` | `account_id` (unico), `enabled`, `max_automations`, `allowed_types text[]` | Gating do platform admin. |
| `channels` | `id`, `automation_id`, `account_id`, `provider` (`waha`), `session_name` (unico global), `connected_phone`, `status` | Mapa sessao WAHA -> automacao. Resolve o resto. |
| `personas` | `id`, `automation_id`, `account_id`, `name`, `system_prompt`, `is_active` | 1 ativa por automacao. |
| `guardrails` | `account_id`/global default + override por automacao, `body` | PT-BR, texto puro, baloes. |
| `model_catalog` | `id`, `provider` (openai/anthropic/...), `kind`, `label`, flags (responsesApi/temperature/vision) | Global, seedado. Provider-agnostico. |
| `automation_models` | `automation_id`, `role` (chat/vision/audio/classifier/triage/summarizer), `model_id`, `params jsonb` | "Varios nos com IAs diferentes". |
| `provider_credentials` | `id`, `account_id`, `provider`, `label`, `key_ciphertext`, `key_last4`, `revoked_at` | **BYOK**, criptografado at-rest. |
| `pipeline_config` | `automation_id`, `config jsonb` (debounce_s, backlog_cutoff_min, triagem on/off, baloes, naturalidade...) | Configurar o pipeline pelo painel. |
| `tools` | `id`, `automation_id`, `account_id`, `kind` (crm/erp/metrics/http), `config jsonb`, `enabled` | Conexoes opcionais (dados Omni/externo). |
| `knowledge_bases` / `knowledge_documents` / `knowledge_chunks` | KB por automacao; chunks com `embedding vector` (pgvector) + `metadata` | RAG por automacao. |
| `service_tokens` | `id`, `account_id`, `automation_id`, `token_hash`, `revoked_at` | Auth do n8n no runtime. Rotacionavel. |
| `contacts` / `messages` / `lead_state` / `long_memory` / `follow_ups` / `purchases` | todos com `automation_id` + `account_id` | Mini-CRM por automacao. |
| `usage_log` | `account_id`, `automation_id`, `provider`, `model`, `role`, `tokens_in/out`, `cost_estimate`, `created_at` | Metricas/limites (cobrado na chave deles; a gente so mede). |

Indices por `account_id` + `automation_id` nos hot paths; `channels.session_name` unico.
**Cadeia de resolucao no runtime:** webhook WAHA traz `session` -> `channels.session_name`
-> `automation_id` -> persona + guardrails + modelos + pipeline + tools + knowledge + enabled.

---

## 4. Decisoes de arquitetura (recomendacoes)

**D1. Provedor de WhatsApp (multi-numero) — decisao de custo/risco, importante.**
Conectar varios numeros por cliente = varias **sessoes**. O **WAHA Core** (free) opera
**1 sessao**; multi-sessao (2-100) e so no **WAHA Plus** (pago). Alternativa **grátis e open
source**: **Evolution API** (Apache-2.0, multi-instancia free, Baileys + Cloud API, Docker,
integra com n8n) — resolve multi-numero sem licenca. Outras libs OSS: WPPConnect, Baileys
(raw), whatsapp-web.js (sao bibliotecas, mais codigo nosso). Comparativo em [SETUP.md] /
mensagem de 2026-06-08.
**Risco a registrar:** WAHA/Evolution/Baileys usam protocolo **nao-oficial** (QR) -> risco de
**ban do numero** e violacao de ToS. So a **WhatsApp Cloud API oficial** (Meta) e ToS-safe,
mas exige verificacao de Business e templates para mensagem ativa.
**Recomendo:** `channel.provider` **pluggable** (waha | evolution | cloud_api) desde o P1;
rodar **WAHA Core na V1** (1 numero, ja montado) e migrar para **Evolution API** (free) no P11
(multi-numero). Oferecer **Cloud API oficial** como trilha "premium/segura" por cliente depois.

**D2. Topologia n8n -> motor Go (norte).**
1 **workflow parametrizado por TIPO** de automacao (ex.: 1 workflow "atendimento" serve todos
os clientes; o comportamento vem do `runtime-config` por sessao). Cliente nunca edita n8n.
Tipo novo = template novo. **Norte:** quando o pipeline estabilizar, migrar a orquestracao
para um **motor Go nativo configuravel** (n8n opcional) — da controle total dos "nos" pelo
painel e remove a dependencia do n8n. Comecar no n8n, abstraindo a config para trocar o motor.

**D3. Knowledge/RAG — pgvector no Postgres do Omni.**
Comportamento estilo GPT = instrucoes + knowledge. **Recomendo RAG self-hosted com pgvector**
(escopado por automacao, provider-agnostico, dado fica no nosso banco — single-source-of-truth):
upload -> chunk -> embed -> `knowledge_chunks` -> retrieval top-k por `automation_id`.
Alternativa: Vector Store/Assistants do provedor (mais simples, mas vendor lock-in e o dado
sai do banco). A "Opcao 1" da sua referencia (instructions + RAG) e exatamente o caminho pgvector.
**Reforco (decisao BYOK):** com BYOK, o Vector Store do provedor viveria na conta OpenAI **do
cliente** (a gente nao controla, custo na chave dele, limpeza por automacao vira bagunca). O
pgvector evita isso (knowledge fica no nosso banco; embeddings sao so chamadas na chave dele)
e ainda permite **cruzar knowledge com CRM/ERP** relacional (objetivo do P9). **Decidido: pgvector.**

**D4. BYOK — chaves criptografadas, creditos do cliente.**
`provider_credentials` guarda a chave **criptografada at-rest** (AES-256-GCM, envelope com
master key em env `AUTOMATION_CRED_ENC_KEY`, **nunca em log**, painel mostra so os ultimos 4).
Cada automacao escolhe provider + credencial. Uso roda nos creditos do cliente; a gente
**mede** tokens em `usage_log` (visibilidade + cota opcional). Modo "chave gerenciada" depois.

**D5. Modelos — V1 so OpenAI (decidido), catalogo ja provider-aware.**
V1 usa **so OpenAI** (decisao 2026-06-08). Mas `model_catalog` ja carrega `provider`
(openai/anthropic/...) e o codigo trata provider como dado, para **somar Claude/outros depois
sem refatorar**. Flags por modelo (Responses API, aceita temperature, suporta visao) aplicadas
pelo painel sozinhas (heranca do MODELOS.md).

**D6. Multi-tenant + RBAC desde o dia 1.**
`automation_id` central; tudo escopado. `entitlements` controla quem cria/quantas/quais tipos
(platform admin libera). Permissoes em `core.permissions` (seedadas pelo Module Registry):
`automation.manage`, `automation.view`, `automation.crm.view`, `automation.whatsapp.manage`,
`automation.platform.admin` (super-user: entitlements, super-robo, metricas cross-account).

---

## 5. Super-robo (time interno)

Um `automation_type = 'super'` interno (nao do cliente), gated por `automation.platform.admin`:
cruza dados de varios modulos/accounts (CRM, ERP, metricas, vendas), responde perguntas de
negocio do time. Reusa a mesma engine + tools, mas com escopo de leitura ampliado e auditado.
Pode nem ser WhatsApp (surface no proprio painel). Fase posterior (depende de tools + metricas).

---

## 6. Roadmap faseado (ir com calma; multi-tenant correto desde o inicio)

> Convencao: P0 feito. P1+ aguardam multitenant-completion (modulo Go/banco). As fases
> A1-A10 do PLANO_INTEGRACAO_OMNI se **dobram** aqui (P1=A1 ampliado, etc.).

| Cod | Entrega | Depende |
|---|---|---|
| **P0** | Infra: containers local (`docker-compose.yml`) + prod (`docker-compose.prod.yml`), Redis disponivel, runbook. **FEITO 2026-06-08.** | — |
| **P1** | Migration `automation.*` **automation-centric** (automations, entitlements, channels, personas, guardrails, model_catalog/models, provider_credentials, pipeline_config, tools, service_tokens, CRM tables, usage_log) + seeds | multitenant-completion |
| **P2** | Modulo Go `automation` (Module Registry): CRUD automations + channels + entitlements; **runtime-config** (sessao->automacao->config); service_tokens; BYOK (cripto). Permissoes | P1 |
| **P3** | n8n consome runtime-config: o workflow "atendimento" vira config-driven por sessao (persona/modelos/pipeline/enabled dinamicos). Valida troca de modelo por expression | P2 |
| **P4** | Painel `/automation`: **listar/criar automacoes** por account; Status (WhatsApp connect/QR por channel); liga/desliga; contexto temporario | P2 |
| **P5** | Painel: Personas/Instrucoes + guardrails (comportamento estilo GPT). Modal e board card espelhados | P2 |
| **P6** | Painel: Modelos por etapa/no + gestao de **credenciais BYOK** (catalogo + regras MODELOS.md) | P2 |
| **P7** | CRM persistente por automacao (contacts/messages/lead_state/long_memory); n8n grava via API | P2 |
| **P8** | **Knowledge/RAG** (pgvector): upload -> chunk -> embed -> retrieval por automacao | P2 |
| **P9** | **Tools / cruzar dados**: agente consulta CRM/ERP/metricas do Omni via API Go, escopo por account | P7 |
| **P10** | Motor proativo (follow-up/pos-venda/nurture) | P7 |
| **P11** | **Multi-numero** (WAHA Plus multi-sessao; N channels por account) | P3, P4 |
| **P12** | **Super-robo** interno (cross-account, admin-gated) | P9 |
| **P13** | Metering/cotas + visibilidade de uso (usage_log no painel) | P6 |
| **P14** | Norte: motor de orquestracao em Go (n8n opcional) | P3..P9 |

---

## 7. Decisoes fechadas (2026-06-08)

1. **RAG:** **pgvector** (self-hosted no Postgres do Omni). Reforcado pelo BYOK e pelo objetivo
   de cruzar dados (D3).
2. **BYOK:** **obrigatorio na V1** (cliente poe a chave, usa os creditos); modo "chave
   gerenciada" (a gente cobra) fica como **opcao futura** (D4).
3. **Provedores V1:** **so OpenAI** por enquanto; catalogo ja provider-aware para somar Claude
   depois sem refatorar (D5).
4. **Super-robo:** **depois** (perto do P12), nao e prioridade da V1 (secao 5).
5. **Provedor WhatsApp:** `channel.provider` pluggable; **WAHA Core na V1** (1 numero, ja
   montado), **Evolution API** (free, OSS) como alvo de multi-numero no P11 (D1).

### Ainda a confirmar
- No **P11**, fechar **Evolution API** (free, mais rework) vs **WAHA Plus** (pago, menos rework)
  para multi-numero — decidir quando chegar la, com o produto ja validado.
- Avaliar oferecer **WhatsApp Cloud API oficial** (ToS-safe) como trilha premium por cliente.

---

## 8. Notas de Deploy (acumular conforme implementa)

- **Infra (P0, feito):** ver [SETUP.md](SETUP.md) §8 (prod) e §3 (local). Tag WAHA correta:
  `devlikeapro/waha:gows-<versao>` (engine GOWS).
- **P1+ (futuro):** migration `automation.*` idempotente; permissoes seedadas no boot
  (`CORE_V2_ENABLED`); rebuild da API (`docker compose up -d --build api`).
- **Env novas a prever:** `AUTOMATION_CRED_ENC_KEY` (master key BYOK — **secreta, rotacao
  planejada**), provider keys ficam no banco (BYOK, nao em env), `WAHA_API_KEY`/license se Plus.
- **pgvector (P8):** a imagem `postgres:16-alpine` do Omni **nao traz pgvector**. Trocar por
  `pgvector/pgvector:pg16` (ou buildar a extensao) ANTES da migration que faz `CREATE EXTENSION
  vector`. Mesma troca nos 2 compose (dev+prod); o volume de dados e compativel.

---

## 9. Referencia cruzada

- Detalhe da 1a automacao (atendimento) -> [PLANO_INTEGRACAO_OMNI.md](PLANO_INTEGRACAO_OMNI.md)
- Infra/containers/runbook -> [../../automation/AGENT.md](../../automation/AGENT.md) · [SETUP.md](SETUP.md)
- Bot no n8n (workflow atual) -> [WORKFLOW.md](WORKFLOW.md) · [ROADMAP.md](ROADMAP.md)
- Regras de modelos -> [MODELOS.md](MODELOS.md)
- Bloqueio (multitenant) -> [../MULTITENANT_COMPLETION_PLAN.md](../MULTITENANT_COMPLETION_PLAN.md)
- Principios/regras -> [../ENGINEERING_PRINCIPLES.md](../ENGINEERING_PRINCIPLES.md) · [../../AGENT_RULES.md](../../AGENT_RULES.md)
