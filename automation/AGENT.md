# AGENT — Modulo `automation` (Assistente WhatsApp/IA)

## Escopo

Automacao de atendimento de WhatsApp com IA, originalmente construida em n8n + WAHA
(projeto "n8n Whatsapp", instalacao separada) e trazida para dentro do Omni em
2026-06-04. Hoje roda como stack de containers do proprio projeto (profile
`automation` no `docker-compose.yml` da raiz). O objetivo da migracao e, por fases,
conectar o bot ao banco/back/front do Omni (CRM, catalogo, ERP) em vez de manter o
n8n isolado.

Persona ativa: **Tony** (consultor objetivo, estilo WhatsApp real). Persona alternativa
documentada: **Perola Buyer** (consultor de compras de joalheria).

## O que e (runtime)

Assistente proativa (nao um chatbot reativo). Pipeline do workflow n8n:

```
Webhook (WAHA) -> normaliza -> dedupe (por id) -> filtro (so conversa 1:1; ignora
grupo/canal/broadcast e mensagens proprias) -> roteia por tipo:
  texto  -> direto
  audio  -> baixa midia -> Whisper transcreve (com aviso "pode ter erro")
  imagem -> baixa -> visao (gpt-4o) descreve -> junta com a legenda
-> DEBOUNCE 7s (agrupa mensagens rapidas via Redis; responde 1x)
-> CLASSIFICADOR DE CONTEXTO (gpt-4o-mini decide ASSUNTO NOVO x CONTINUACAO; reseta
   memoria curta por "segmento")
-> MEMORIA LONGA (resumo por contato no staticData; persiste entre assuntos)
-> AI AGENT (Tony) com memoria Redis por chatId_<segmento>
-> NATURALIDADE ("digitando", delay proporcional, divide a resposta em baloes)
-> RESUMIDOR atualiza a memoria longa.
```

Modelos: chat `gpt-5.3-chat-latest` - visao `gpt-4o` - audio `whisper-1` -
classificador/resumidor `gpt-4o-mini`. Regras de troca de modelo em
[docs/automation/MODELOS.md](../docs/automation/MODELOS.md).

## Estrutura

```
automation/
  AGENT.md                         <- este arquivo
  .gitignore                       <- protege segredos do modulo
  .mcp.json                        <- config MCP do n8n (SEGREDO; regerar key no novo n8n)
  docker-compose.reference.yml     <- recipe ORIGINAL standalone (so referencia historica)
  export/
    workflow-whatsapp.json         <- o workflow completo (36 nos; persona+guardrails embutidos)
    credentials.decrypted.json     <- credenciais do n8n (SEGREDO em texto puro)

docs/automation/                   <- documentacao detalhada (migrada de docs/n8n/)
  SETUP.md       runbook de subida (ja adaptado para o profile automation)
  AGENTS.md      decisoes/historico do projeto n8n
  WORKFLOW.md    arquitetura alvo + roadmap por etapa do bot
  ROADMAP.md     quadro de status do bot (n8n)
  MODELOS.md     requisitos por modelo e como trocar sem quebrar
  gpt-tony.md / gpt-perola-buyer-assistant.md / guardrails-resposta.md   (fontes dos prompts)
  HANDOFF.md     prompt de handoff original
  roadmap.html / roadmap-server.js   dashboard do roadmap do bot
```

> A recipe canonica dos containers e o `docker-compose.yml` da RAIZ (profile
> `automation`). O `docker-compose.reference.yml` aqui e so o original standalone,
> mantido como referencia — nao usar para subir.

## Como rodar (local)

A stack NAO sobe no dev normal. Sobe sob demanda:

```bash
docker compose --profile automation up -d        # sobe n8n + waha + redis (+ api/web/postgres)
docker compose --profile automation logs -f n8n  # logs do n8n
docker compose --profile automation down          # derruba (sem -v: preserva volumes)
```

Passo a passo completo (instalar community node, importar credenciais + workflow,
escanear QR, ativar) em [docs/automation/SETUP.md](../docs/automation/SETUP.md).

> Ativar o workflow faz o bot responder no WhatsApp real conectado na WAHA.
> Confirmar com o Mike antes de ativar.

### Portas (host -> container)

| Servico | Host | Container | Env var |
|---|---|---|---|
| n8n | 5680 | 5678 | `AUTOMATION_N8N_PORT` |
| WAHA | 3010 | 3000 | `AUTOMATION_WAHA_PORT` |
| Redis | 6380 | 6379 | `AUTOMATION_REDIS_PORT` |

Escolhidas para nao colidir com o Omni (api 9091, web 3003, postgres 5432). Nao
mudar as portas do Omni. Defaults documentados em `.env.docker.example`.

**Producao (VPS):** mesma stack no `docker-compose.prod.yml` (profile `automation`),
**mesmos nomes de servico** (`redis`/`waha`/`n8n`) para o workflow/credenciais rodarem
igual local e prod. Sem portas publicas: `n8n`/`waha` na rede `proxy` (aliases
`automation-n8n`/`automation-waha`) atras do Caddy com basic auth; `redis` so na rede
`app` (`redis:6379`, disponivel para a API do Omni depois). Binds `127.0.0.1:15680/13010/
16380` so para tunel SSH. Vars no `.env.production.example` (bloco `AUTOMATION_*`).
Runbook: docs/automation/SETUP.md secao 8.

## Integracao com o Omni

- **Rede:** mesmo projeto Compose (`omni`) -> mesma rede default. A WAHA fala com o
  n8n por `http://n8n:5678` (sem `host.docker.internal`). O n8n alcanca o Postgres do
  Omni por `postgres:5432`.
- **Volumes proprios** (namespace `automation_*`): `automation_n8n_data` (config/SQLite
  do n8n), `automation_waha_sessions`, `automation_waha_media`, `automation_redis`.
  Sao volumes NOVOS do projeto `omni` — a migracao comeca do zero (reimportar
  workflow/credenciais e reescanear o QR; ver SETUP secao "O que NAO migra").
- **Acesso a dados do Omni (futuro, por fase):** o caminho alinhado com a arquitetura
  e o bot consultar produto/estoque/preco/CRM via **tools do agente batendo na API Go**
  (`/v1/...` de `crm/catalog`, `erp`), com auth e escopo por account/tenant — nao SQL
  cru direto nas tabelas. Respeita RBAC, multi-tenant e single-source-of-truth.

## Fases futuras (resumo)

> **Visao de plataforma (multi-tenant):** cada cliente cria N automacoes (robos), BYOK,
> RAG, super-robo — [docs/automation/PLATAFORMA_AUTOMACAO.md](../docs/automation/PLATAFORMA_AUTOMACAO.md).
> Modelo de dados **automation-centric** (`automation_id` central). Construir multi-tenant
> desde o dia 1.
> Detalhe da 1a automacao (atendimento) — schema, endpoints, painel, n8n config-driven,
> deploy VPS, fases A1-A10: [docs/automation/PLANO_INTEGRACAO_OMNI.md](../docs/automation/PLANO_INTEGRACAO_OMNI.md).
> Visao do bot no n8n: docs/automation/WORKFLOW.md. Espelho de status: roadmap-data.ts.

- **Etapa 2 — mini-CRM no Postgres do Omni:** schema dedicado `automation.*` (tenant-aware,
  `account_id` NOT NULL com FK para `core.accounts`) com contatos, mensagens, estado do
  lead, follow-ups, compras. Hoje a memoria longa e uma versao "lite" no staticData do n8n.
- **Tools do agente:** consultar catalogo/estoque/preco e registrar lead/pedido via API Go.
- **Etapa 3 — motor proativo:** follow-up de quem nao respondeu (cadencia), pos-venda,
  nurture/upsell (depende de estado persistente da Etapa 2).
- **Etapa 5 — RAG dos dados do ERP:** para o Perola analisar estoque/vendas de verdade.
- **Etapa 7 — painel de configuracao no front do Omni:** escolher modelos, gerenciar
  personas/prompts, ligar/desligar, contexto temporario.

> Bloqueio: as fases que viram modulo Go/banco aguardam o fechamento da
> `multitenant-completion` (regra do MULTITENANT_COMPLETION_PLAN: nenhuma fase nova de
> modulo satelite avanca antes). A migracao de infra/pastas feita em 2026-06-04 nao
> mexe no core multi-tenant.

## Gotchas tecnicos (do projeto n8n — nao esquecer)

- Escrita de workflow via n8n-MCP esta quebrada nesta versao do n8n (PUT publico
  estrito). Editar via PUT direto com payload limpo `{name, nodes, connections,
  settings:{executionOrder}, staticData}`. Nunca montar expressoes com `$json` dentro de
  `node -e` inline (o shell come o `$`); usar arquivo `.js`.
- Modelos de raciocinio (gpt-5*, o-series) exigem Responses API e nao aceitam
  `temperature`; nao funcionam no no de imagem (por isso a visao usa gpt-4o).
- Expressoes do n8n nao suportam optional chaining (`?.`); usar `(obj || {}).campo`.
- `systemMessage` do AI Agent = **so a persona/prompt do painel** (+ knowledge docs habilitados).
  Decisao "prompt e a lei" (2026-06-19): o no `Montar systemMessage` **NAO anexa mais** os guardrails
  fixos (`guardrails-resposta.md`). Regras de idioma/formato (PT-BR, texto puro, baloes) agora tem que
  estar DENTRO do proprio prompt do painel. Ver registro de falhas "2026-06-20" no
  back/internal/modules/automation/AGENT.md.
- Midia da WAHA tem TTL curto: baixar na hora do webhook.

## Segredos

`automation/.mcp.json` e `automation/export/credentials.decrypted.json` contem chaves
(OpenAI etc.). Ja ignorados pelo `.gitignore` (do modulo e da raiz). Ao migrar de
ambiente, considerar rotacionar as chaves.

## Notas de Deploy

- A stack `automation` esta no `docker-compose.yml` (dev local) **e** no
  `docker-compose.prod.yml` (profile `automation`, infra preparada 2026-06-08). O deploy
  na VPS em si (Caddy, DNS, QR, ativacao, backups) e do Mike — passo a passo em
  docs/automation/SETUP.md secao 8.
- Decisao de prod (2026-06-08): exposicao via **Caddy + basic auth** (subdominios `n8n.`/
  `waha.`); **Redis so disponivel** na rede `app` (sem mexer na API Go ainda).
- Sem migration Go nesta etapa (so infra + docs). A Etapa 2/A1 (schema `automation.*`)
  tera migration propria quando for implementada — **bloqueada pela multitenant-completion**.
- Vars novas: local em `.env.docker.example` (`AUTOMATION_N8N_PORT`, `AUTOMATION_WAHA_PORT`,
  `AUTOMATION_REDIS_PORT`, `AUTOMATION_REDIS_PASSWORD`); prod em `.env.production.example`
  (`AUTOMATION_N8N_HOST`, `AUTOMATION_N8N_WEBHOOK_URL`, `AUTOMATION_N8N_ENCRYPTION_KEY`,
  `AUTOMATION_WAHA_HOST`, `AUTOMATION_WAHA_DASHBOARD_USER/PASSWORD`, aliases de proxy).

## Pendencias acumuladas (env / VPS / modulo) — ir marcando

> Checklist vivo: tudo que precisa ser feito conforme o modulo evolui. Detalhe e ordem na
> [PLATAFORMA_AUTOMACAO.md](../docs/automation/PLATAFORMA_AUTOMACAO.md) §6/§8.

**Ambiente / env**
- [x] Ajustes 1/2/4 **aplicados no `workflow-whatsapp.json`** (2026-06-08, patch programatico):
      nó Dados +`msgTimestamp`/`isSticker`/`isForwarded`/`isReply`/`quotedText`; Dedupe +boot
      cutoff 5min; Juntar +prefixo de contexto (figurinha/encaminhada/reply); AI Agent emoji
      endurecido (re-sync). Mesma mudanca em guardrails-resposta.md (fonte).
- [ ] **Validar ao vivo** apos importar: 1 figurinha + 1 reply + 1 msg antiga (boot). Os campos
      `isForwarded`/`isReply` dependem do que o engine **GOWS** entrega no payload (`_data.*`);
      se nao detectar, ajustar o caminho no nó Dados (guards `(obj||{})` ja evitam erro).
- [ ] Se o n8n ja tiver o workflow importado, **re-importar** para pegar os ajustes acima.
- [ ] Prod: preencher `AUTOMATION_*` no `.env.production` (gerar `N8N_ENCRYPTION_KEY`, nao mudar depois).
- [ ] Futuro (BYOK): `AUTOMATION_CRED_ENC_KEY` (master key de cripto das chaves dos clientes).

**VPS / deploy**
- [ ] Rotas Caddy `n8n.`/`waha.` (basic auth) no projeto do proxy + DNS dos subdominios.
- [ ] Subir `--profile automation` na prod, importar workflow/credenciais, escanear QR, ativar (confirmar com o Mike).
- [ ] **Apos importar (ARMADILHA real 2026-06-19):** as credenciais do n8n vem com **host de DEV**
      (`host.docker.internal`) — reapontar **Redis** (`redis:6379` + `AUTOMATION_REDIS_PASSWORD`) e
      **WAHA** (`http://waha:3000`); **reimportar o `workflow-whatsapp.json` atual** (o que ja estava
      no n8n pode ser uma versao velha sem os fixes de conexao) e **instalar o community node
      `n8n-nodes-waha`** no volume `~/.n8n/nodes`; `docker restart` do n8n a cada troca. Sem isso o
      bot conecta mas **nao responde** (execucoes travam no Redis / erram em "Ler memoria" / envio
      WAHA falha). Detalhe: back/internal/modules/automation/AGENT.md "Registro de falhas 2026-06-19 (6)".
- [ ] Backup dos volumes `automation_n8n_data` e `automation_waha_sessions`.
- [ ] Multi-numero: avaliar **WAHA Plus** (licenca) quando precisar de >1 sessao por account.

**Modulo Go / banco (multitenant-completion fechada — desbloqueado 2026-06-09)**
- [x] **M1 entregue (2026-06-09):** modulo Go `automation` (Module Registry) + migration
      `0140_automation_schema.sql` (automations/channels) + painel `/automation`
      (Status/Conectar WhatsApp via proxy WAHA + liga/desliga). Gated `platform_admin`.
      Doc: back/internal/modules/automation/AGENT.md.
- [x] **M2 entregue (2026-06-09):** runtime-config — persona vive no banco (`automation.personas`,
      seed Tony/Crow via go:embed) + guardrails montados no back; n8n consome
      `GET /v1/runtime/automation/config` (no `Get runtime config` + `Bot ligado?` gate);
      on/off passa a valer. Persona fonte: docs/automation/persona-tony-crowvisuals.md (verbatim).
      Ativacao exige rebuild da api + `AUTOMATION_RUNTIME_TOKEN` (ver back AGENT.md Notas de Deploy).
- [x] **M3 entregue (2026-06-09):** editor de **persona pelo painel** (`/automation` -> card
      Comportamento: nome + system_prompt). `GET/PUT /v1/automation/persona`; salvar muda o bot
      sem tocar no n8n (runtime-config le do banco). Guardrails seguem anexados pelo back.
- [ ] M3+: separar `knowledge_documents` por doc no painel + RAG (P8) p/ knowledge grande.
- [ ] Persona/knowledge da Crow: embutida no system_prompt (~4k tokens). RAG so quando o knowledge crescer.
- [ ] P8: pgvector — trocar imagem `postgres:16-alpine` -> `pgvector/pgvector:pg16` (dev+prod)
      antes da migration `CREATE EXTENSION vector`. Esboco: docs/automation/schema_automation_sketch.sql.
- [ ] Redis ja disponivel (`redis:6379` na rede app) — decidir primeiro uso pela API Go.
