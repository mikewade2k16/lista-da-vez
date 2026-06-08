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

> Plano canonico da integracao (schema, endpoints, painel, n8n config-driven, deploy VPS,
> fases A1-A10): [docs/automation/PLANO_INTEGRACAO_OMNI.md](../docs/automation/PLANO_INTEGRACAO_OMNI.md).
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
- `systemMessage` do AI Agent = persona (`gpt-tony.md`) + `guardrails-resposta.md`
  (sempre anexar os guardrails ao re-sincronizar). Guardrails forcam resposta em PT-BR e
  texto puro (sem markdown), e definem quando dividir em baloes.
- Midia da WAHA tem TTL curto: baixar na hora do webhook.

## Segredos

`automation/.mcp.json` e `automation/export/credentials.decrypted.json` contem chaves
(OpenAI etc.). Ja ignorados pelo `.gitignore` (do modulo e da raiz). Ao migrar de
ambiente, considerar rotacionar as chaves.

## Notas de Deploy

- A stack `automation` esta apenas no `docker-compose.yml` (dev local). NAO foi
  adicionada ao `docker-compose.prod.yml` — deploy na VPS e fase posterior (decisao em
  aberto em docs/automation/AGENTS.md).
- Sem migration Go nesta etapa (so infra + pastas + docs). A Etapa 2 (schema
  `automation.*`) tera migration propria quando for implementada.
- Vars novas (opcionais, com default): `AUTOMATION_N8N_PORT`, `AUTOMATION_WAHA_PORT`,
  `AUTOMATION_REDIS_PORT`, `AUTOMATION_REDIS_PASSWORD` — documentadas em `.env.docker.example`.
