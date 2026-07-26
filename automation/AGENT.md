# AGENT — Modulo `automation` (Assistente WhatsApp/IA)

## Escopo

Automacao de atendimento de WhatsApp com IA, originalmente construida em n8n + WAHA
(projeto "n8n Whatsapp", instalacao separada) e trazida para dentro do Omni em
2026-06-04. Hoje roda como stack de containers do proprio projeto (profile
`automation` no `docker-compose.yml` da raiz). O objetivo da migracao e, por fases,
conectar o bot ao banco/back/front do Omni (CRM, catalogo, ERP) em vez de manter o
n8n isolado.

## Propriedade de workflows — regra obrigatoria

Cada workflow pertence ao modulo que o criou. Uma tarefa so pode editar, importar,
exportar, ativar, desativar ou remover workflows do modulo explicitamente em escopo.

- `workflow-whatsapp.json` pertence ao modulo **automation** e usa WAHA legitimamente.
- `workflow-calendar-*` pertence ao modulo **calendar**.
- `workflow-omni-chat.json` pertence ao chat interno da Operacao. Recebe do Go
  o prompt, historico, provider, modelo, temperatura e chave global resolvida
  server-side; nao possui credencial/modelo fixo no workflow.
- `workflow-omnichannel-brain.json` e `workflow-instagram-first-contact.json` pertencem
  ao modulo **omnichannel**.

Trabalhar no omnichannel nunca autoriza tocar no runtime, provider, credenciais ou estado
ativo dos workflows de automation, calendar ou Operacao. Mudanca em script compartilhado
de import/export deve preservar todos os owners e aplicar validacoes especificas por id.

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
    workflow-omni-chat.json        <- chat interno da Operacao (Webhook -> AI Agent -> Respond)
    workflow-omnichannel-brain.json <- cerebro stateless do atendimento; sem envio de canal
    workflow-instagram-first-contact.json <- orquestracao Instagram; sem envio Meta
    workflow-calendar-omni.json    <- IA do Calendario: plano estrategico do mes (SPEC-W3)
    workflow-calendar-chat.json    <- IA do Calendario: chat flutuante (SPEC-W1; C7)
    workflow-calendar-transcribe.json <- voz do Calendario: OpenAI Whisper/Gemini (SPEC-W2)
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

> Ativar o workflow da automacao faz o bot responder no WhatsApp real conectado na WAHA.
> Confirmar com o Mike antes de alterar seu estado.

### Par `n8n:import` <-> `n8n:export` (o n8n roda do BANCO, nao do arquivo)

O n8n guarda os workflows no PROPRIO banco (SQLite). O repo (`export/workflow-*.json`)
so reflete o n8n quando alguem sincroniza nos DOIS sentidos. Import, export, check de runtime e
sync exigem sempre owner e key exatos; operacao global implicita foi removida:

| comando | sentido | o que faz |
|---|---|---|
| `npm run n8n:import:<key>` | arquivo -> n8n | `scripts/dev/n8n-import.ps1 -Owner <owner> -Only <key>`: valida ownership/IDs, copia um arquivo, importa, reativa e reinicia. |
| `npm run n8n:export:<key>` | n8n -> arquivo | `scripts/dev/n8n-export.ps1 -Owner <owner> -Only <key>`: traz um workflow no shape canônico. |
| `npm run n8n:export:<key>:check` | runtime -> comparação | consulta somente o alvo owner-scoped e retorna `10` se divergir. |
| `npm run n8n:export:<key>:sync` | runtime -> arquivo | sincroniza somente o alvo owner-scoped; nunca usar alias de outro owner numa tarefa de módulo. |
| `npm run n8n:export:check` | somente local | valida registro, IDs e hashes dos sete arquivos; não chama Docker, não consulta runtime e não cria temporário. |

Aliases canônicos ficam em `package.json`. Omnichannel usa somente
`n8n:{import,export}:omnichannel-brain` e
`n8n:{import,export}:instagram-first-contact` (mais os sufixos `:check`/`:sync` existentes).
Calendar, Operação e Automação usam seus próprios aliases/owners.

**Mapa FIXO id -> arquivo:** `n8n-workflow-registry.ps1` e a fonte única de key, owner, ID e
caminho. Import/export nunca derivam arquivo por glob para escrever. O `deploy-pull.ps1` antigo
ainda consome o glob como operação explícita de plataforma; ele não pode ser usado por tarefa
isolada do Omnichannel, Calendar, Operação ou Automação.

| arquivo | id |
|---|---|
| `workflow-calendar-chat.json` | `calendarchat0001` |
| `workflow-calendar-omni.json` | `calendaromni0001` |
| `workflow-calendar-transcribe.json` | `calendartrans001` |
| `workflow-instagram-first-contact.json` | `instafirst000001` |
| `workflow-omnichannel-brain.json` | `omnibrain0000001` |
| `workflow-omni-chat.json` | `omnichatmvp00001` |
| `workflow-whatsapp.json` | `lzhb5JjN5kdcVuRR` |

O nome do arquivo e ESTAVEL (nao derivar do nome do workflow: renomear no n8n nao
renomeia o arquivo, so evita ruido de diff).

**Garantia anti-credencial e anti-dado de runtime:** `n8n export:workflow` NAO exporta credenciais
decriptadas (os nos trazem `credentials` so como `{id,name}`). O
`n8n-workflow-normalize.js`, chamado por `n8n-export.ps1`, ABORTA o arquivo (sem gravar) se algum
`node.credentials[*]` tiver campo fora de `id`/`name`. O normalizador tambem FORCA
`pinData={}` e `staticData=null`, mesmo quando o n8n exporta payload de webhook ou memoria
preenchidos. Definicao portavel vai ao repo; credencial, amostra e memoria de conversa nao.

## Workflow "Omnichannel Brain"

`export/workflow-omnichannel-brain.json` (id `omnibrain0000001`) e o executor stateless
da triagem. O Go resolve provider/modelo/temperatura/prompt/schema no banco e chama o endpoint
interno com `brain.request.v2` ou `brain.request.v3`, conforme a versão publicada do agente;
credencial persistente nunca entra no request, export ou log.
O workflow exige `X-Omni-Internal-Token` validado contra `OMNI_N8N_INTERNAL_TOKEN`, chama
somente o gateway interno em `OMNI_LLM_GATEWAY_URL` e revalida `brain.result.v2/v3` no Go.
O mesmo arquivo possui o webhook `omnichannel-brain-media`: ele recebe uma chave descriptografada
somente durante a execução, baixa a mídia privada com token curto e chama OpenAI/Gemini conforme
o papel versionado. O resultado volta estruturado ao Go; o workflow não grava estado nem envia
mensagem ao canal. O workflow `workflow-whatsapp.json` da Automação não participa desse fluxo.
No v3, `close` continua sendo apenas proposta: os gates configuráveis e a lease obrigatória são
avaliados sob lock no Go. Ele nao acessa Postgres, Evolution, WAHA ou Meta e nao possui node de envio. As quatro opcoes
`saveData*` ficam desligadas para o token efemero e o contexto do cliente nao permanecerem no
banco do n8n. O arquivo local esta preparado/inativo; importacao e ativacao dependem do aceite
externo do E1 e da implementacao do gateway.

Quando o gateway e o aceite E1 estiverem prontos, a ativacao desse workflow isolado e segura
porque ele nao recebe webhook de canal nem envia mensagem. Isso nao autoriza alterar o workflow
`workflow-whatsapp.json`, que pertence ao modulo automation e tem ciclo operacional proprio.

`workflow-instagram-first-contact.json` (id `instafirst000001`) e a orquestracao separada
de DM/comentarios. Ela prepara origem/politica/historico e chama o cerebro comum, mas
responde sempre com `deliveryPolicy=go_outbox_only`: nao possui token Meta nem node de
publicacao. O adapter Meta no Go e pre-requisito para qualquer entrega real.

**Guardas automáticos atuais:**
- **git (pre-commit, `-Check`):** valida somente o inventário local e bloqueia registro/ID inválido;
  não consulta Docker e não afirma se o runtime está alinhado.
- **deploy rápido normal:** `deploy:fast:prod` sobe API/web e não aciona Automação implicitamente.
- **deploy de Automação:** `-DeployAutomation` exige `-WorkflowOwner` e `-WorkflowOnly` para o sync
  local, mas o `deploy-pull.ps1` downstream ainda importa o conjunto global. Portanto esse caminho
  é operação de plataforma, exige autorização explícita do dono e é proibido numa tarefa isolada
  do Omnichannel até existir deploy owner-scoped de ponta a ponta.

**Follow-ups (evolucao, NAO no MVP):**
- (B) trigger dentro do proprio n8n ("workflow salvo" -> `n8n export:workflow` para um
  volume mapeado em `export/`) — mais tempo real, mas o n8n OSS nao expoe hook de save
  trivial (provavel polling) + exige volume/permissao no host.
- (C) watcher no host (`n8n:watch`) fazendo `-Check` em intervalo e exportando ao detectar
  diff — mais um daemon para lembrar de rodar.
- (VPS) exportar o que roda em PRODUCAO de volta pro repo (hoje o fluxo e uma direcao so:
  dev -> repo -> deploy -> prod). Exige rodar via o tunel/ssh do deploy e decidir a politica
  de fonte da verdade (dev x prod).

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

### Limites e healthchecks (AC-11, 2026-07)

Os dois compose agora limitam memoria por servico do stack: `redis` 256m, `waha` 2g
(`AUTOMATION_WAHA_MEMORY_LIMIT`, elevado apos OOM em 2026-07-22) e `n8n` 768m
(`mem_reservation` proporcional; no prod tambem `cpus`). WAHA esta fixado em
`gows-2026.7.1`, restaura sessoes persistidas no boot e ignora stories/status por default.
Healthchecks: `waha` = `GET /ping` (`/health` e WAHA Plus → 422); `n8n` = `GET /healthz`;
`redis` = `redis-cli ping` autenticado (`$$REDIS_PASSWORD`, dois cifroes = env do
container). `depends_on`: `n8n→redis service_healthy`, `waha→n8n service_started`
(so ordena o boot; a WAHA sobe mesmo com n8n quebrado). Como Docker nao reinicia apenas por
`unhealthy`, o healthcheck da WAHA encerra PID 1 apos tres falhas consecutivas de `/ping`, mas so
depois de aquele boot ja ter ficado saudavel; `restart: unless-stopped` entao recupera a sessao.
Log rotation (prod, AC-16): json-file 10m×3 por servico.

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

## Workflow "Calendar Omni" (IA do Calendario — SPEC-W3)

Workflow separado (`export/workflow-calendar-omni.json`, id `calendaromni0001`) que gera o
**plano estrategico de conteudo do mes** do modulo Calendario. O n8n **nao fala com o Postgres
direto** e e um EXECUTOR BURRO: quem orquestra e a fonte da verdade da IA (provider, modelo,
baseUrl, prompt, temperatura E a key) e o back Go do Calendario (`POST /v1/calendar/ai/plan`), que
dispara o webhook e recebe o resultado por callback.

- **Webhook** POST path `calendar-omni` (responde na hora) — recebe o payload C5 (planId, month,
  `ai` provider/model/baseUrl/systemPrompt/temperature **+ apiKey**, clients+perfil, holidays do
  mes, monthNotes, callbackUrl, serviceToken).
- **Montar prompt** (Code) — system (config OU DEFAULT pt-BR que exige JSON estrito no shape
  C4.content) + user (mes/feriados/clientes/notas); resolve baseUrl default por provider (mapa C2,
  agora com `openai`/`gemini` OpenAI-compat e `glm` = z.ai internacional, igual ao Calendar Chat),
  clamp de temperature 0..1. **Expoe `apiKey = body.ai.apiKey`** (resolvida server-side pelo Go a
  partir do banco, contrato PAY) para os nos HTTP.
- **Switch provider** — `claude` -> HTTP `POST {baseUrl}/v1/messages` (header `x-api-key:
  {{ $json.apiKey }}` + `anthropic-version`); demais (openai/gemini/deepseek/qwen/kimi/glm/custom)
  -> HTTP `POST {baseUrl}/chat/completions` (header `Authorization: Bearer {{ $json.apiKey }}`). A
  key vem SEMPRE do payload — **sem credential/`$env` no n8n** (mudanca SPEC-W3, 2026-07-04,
  removendo as credentials `calendar-ai-claude`/`calendar-ai-openai-compat`). LLM nodes com
  `onError: continueRegularOutput`.
- **Extrair JSON** (Code) — normaliza Anthropic (`content[0].text`) vs OpenAI
  (`choices[0].message.content`), remove cercas de codigo, `JSON.parse` com try/catch ->
  `{status:'done', content}` ou `{status:'error', error}`. O `apiKey` NAO trafega para o callback
  (so planId/status/content/error/callbackUrl/serviceToken) — a key nunca vaza no loop de volta.
- **Callback** — POST `{{callbackUrl}}` com header `X-Service-Token: {{serviceToken}}`.

**Sem credential/`$env` no n8n para a IA do plano** — a key crua trafega no payload server-to-server
(Go -> n8n, rede docker) e nunca e logada nem persistida. Kill switch (`ai.enabled=false`) e
"sem key" sao tratados no Go ANTES de disparar (contrato PAY: `ai_disabled`/`ai_key_missing` 409
acionaveis). Envs do back: `CALENDAR_AI_WEBHOOK_URL`
(`http://n8n:5678/webhook/calendar-omni`), `CALENDAR_AI_SERVICE_TOKEN`, `CALENDAR_AI_CALLBACK_BASE`
(as envs `CALENDAR_AI_KEY_*` deixam de ser lidas por este workflow). Como o `versionId` mudou
(`...calendaromni03`), **reimportar** para pegar a versao sem credential. Import + teste manual:
**pendente** (workflow autorado, validado fora do n8n). Runbook completo:
[docs/automation/CALENDAR_OMNI_WORKFLOW.md](../docs/automation/CALENDAR_OMNI_WORKFLOW.md).

## Workflow "Calendar Chat" (chat de IA do Calendario — SPEC-W1)

Workflow separado (`export/workflow-calendar-chat.json`, id `calendarchat0001`) que responde o
**chat de IA** do Calendario. Contrato C7 (payload agora carrega a KEY, contrato PAY da wave 3). O
back Go (`POST /v1/calendar/chat/ask`) monta o payload (`question` + `ai` + `context`, o `context`
identico ao agregado C9 via a MESMA funcao `BuildAIContext`) e faz um POST **sincrono** ao webhook,
esperando `{ "answer": "", "eventIds": [], "proposals": [] }` de volta. O n8n nao fala com o Postgres direto e e um EXECUTOR BURRO:
o painel/back e a fonte da verdade da IA (provider, modelo, baseUrl, prompt, temperatura E a key).

- **Webhook** POST path `calendar-chat`, `responseMode: responseNode` (responde pelo no Respond).
- **Montar contexto** (Code) — system = `ai.systemPrompt` OU DEFAULT pt-BR ("assistente de
  calendario de conteudo") + serializa o `context` (perfil do cliente, feriados, eventos ricos com
  ID/horario/cliente/midias, notas, tasks reais do board configurado,
  planos) no system; resolve `provider`/`model`/`baseUrl`/`temperature` (mapa `DEFAULT_BASE` com
  **gemini** OpenAI-compatible `https://generativelanguage.googleapis.com/v1beta/openai` e **glm**
  z.ai; clamp 0..1) + guard modelo x provider (cai no default do provider se o modelo salvo nao
  casa, evita 404). **A API key vem no payload** (`body.ai.apiKey`, resolvida server-side pelo Go a
  partir do banco) e e exposta como `apiKey` no output — a key crua nunca fica no n8n. `keyEnv`
  continua no output so como referencia/diagnostico. **Historico da conversa** (mudanca SPEC-W5,
  2026-07-04): le `body.history` (array `{role,content}`, so `user`/`assistant` com content;
  itens invalidos descartados) e monta a array `messages` do LLM na ordem
  `[system+context, ...history, {user: question}]`, exposta como `messages` no output. O n8n **nao
  guarda memoria** — a fonte do historico e o banco (persistido pelo Go), so repassado no payload.
- **Chamar LLM** (HTTP `POST {baseUrl}/chat/completions`, OpenAI-compatible) — header
  `Authorization: Bearer {{ $json.apiKey }}` (a key do payload). Body usa a array `messages` ja
  montada no no anterior (`messages: $json.messages` = system + historico + pergunta; antes era
  system+user fixo). **Nao le mais `$env`/credential** (mudanca SPEC-W1, 2026-07-04, revertendo o
  `$env[keyEnv]` da wave anterior). `onError: continueRegularOutput`.
- **Extrair resposta** (Code) - normaliza `choices[0].message.content` -> `{ answer, eventIds, proposals[] }`; `eventIds` identifica os eventos que o Go monta como cards somente com dados
  autoritativos. `proposals[].fields` preserva campos de evento/task (`priority`, `description`,
  `responsibleId`, `involvedIds`, `taskId`/`targetId`, datas etc.). Quando houver `eventIds`, o
  `answer` deve ser resumo curto (sem repetir a lista item a item); o Go ainda compacta
  defensivamente respostas longas antes de persistir. Item de erro do HTTP node vira mensagem
  acionavel pt-BR.
- **Respond to Webhook** - `{ "answer": ..., "eventIds": [...], "proposals": [...] }`.

**Sem credential/`$env` no n8n para a IA do chat** — a key trafega no payload server-to-server (Go
-> n8n, rede docker) e nunca e logada nem persistida. Env do back: `CALENDAR_CHAT_WEBHOOK_URL`
(`http://n8n:5678/webhook/calendar-chat`); vazio -> back responde 503 `chat_not_configured`. Kill
switch e "sem key" sao tratados no Go ANTES de disparar (contrato PAY: `ai_disabled`/`ai_key_missing`
409 acionaveis). **Memoria de conversa (SPEC-W5, 2026-07-04):** o workflow recebe o `history` no
payload (persistido no banco pelo Go, ultimas N mensagens) e o inclui na array `messages` entre o
system e a pergunta — o n8n segue stateless (sem memoria propria). **Limitacoes:** tools desligadas
(o assistente responde so com o `context` autoritativo do system + o historico). Como o `versionId` mudou
(`...calendarcht07`), **reimportar** para pegar a versao com campos completos de proposta e `context.tasks` do board configurado. Import + teste manual:
**pendente** (autorado, validado fora do n8n: parse + ordem da array messages).
Runbook com curl de teste:
[docs/automation/CALENDAR_CHAT_WORKFLOW.md](../docs/automation/CALENDAR_CHAT_WORKFLOW.md).

## Workflow "Calendar Transcribe" (voz do Calendario — SPEC-W2)

Workflow separado (`export/workflow-calendar-transcribe.json`, id `calendartrans001`) que
**transcreve o audio** gravado no chat de voz do Calendario (SPEC-F7) em texto. Contrato C8 +
PAY (wave 3). O back Go (`POST /v1/calendar/chat/transcribe`) aplica o kill switch (`ai.enabled`),
resolve provider/model/**apiKey** no banco (`resolveAIKey`), valida mime + tamanho (max 15 MiB) e
**repassa o multipart** ao webhook com esses campos; o n8n devolve `{ "text": "..." }` e o Go
entrega ao painel (o texto entra no input, o usuario revisa antes de enviar). Nada e gravado em
disco. **Multi-provider sem credential do n8n** — o painel/back e a fonte da verdade da IA; a key
crua vem no payload (contrato PAY), nunca no n8n/`.env`/log.

- **Webhook** POST path `calendar-transcribe`, `responseMode: responseNode`. Campos `provider`/
  `apiKey`/`model` chegam em `$json.body`; o arquivo do campo `file` vira binario na propriedade
  `data0` (webhook do n8n grava multipart como prefixo `data` + indice `0`).
- **Preparar** (Code) — normaliza `provider` para `openai` OU `gemini` (outro -> `gemini`) e
  reatacha o binario `data0` para os ramos.
- **Switch por provider** (v3.2) — output 0 `openai`, output 1 `gemini`.
- **Ramo openai** — HTTP `POST https://api.openai.com/v1/audio/transcriptions` (multipart:
  `file` = binario `data0` + `model` = `body.model || whisper-1`; header `Authorization: Bearer
  {{ body.apiKey }}`). OpenAI devolve `{ text }` nativo -> Respond.
- **Ramo gemini** — Code `Montar base64` (le `data0` via `getBinaryDataBuffer` -> base64; monta
  `contents.parts` com `inline_data` mime+base64 + prompt "Transcreva...") -> HTTP `POST
  {geminiBase}/models/{model}:generateContent` (header `x-goog-api-key: {{ apiKey }}`; geminiBase
  default `https://generativelanguage.googleapis.com/v1beta`, model default `gemini-2.5-flash`) ->
  Code `Extrair texto` (`candidates[0].content.parts[].text`) -> Respond.
- **Respond to Webhook** — `{ "text": ... }` (ambos os ramos convergem).

Env do back: `CALENDAR_TRANSCRIBE_WEBHOOK_URL` (`http://n8n:5678/webhook/calendar-transcribe`);
vazio -> back responde 503 `transcribe_not_configured`. Kill switch/sem-key sao tratados no Go
ANTES de disparar (409 `ai_disabled`/`ai_key_missing`). **Whisper LOCAL** documentado como
alternativa futura (enum previsto, NAO implementado — 3o ramo `local` apontando um faster-whisper
self-host OpenAI-compativel; CUIDADO memoria da VPS/AC-11). Groq tambem citado. Import + teste
manual: **pendente** (autorado, validado fora do n8n: parse, ramos, sintaxe dos Code). Como o
`versionId` mudou, **reimportar** para pegar a versao multi-provider. Runbook completo com curl de
teste: [docs/automation/CALENDAR_TRANSCRIBE_WORKFLOW.md](../docs/automation/CALENDAR_TRANSCRIBE_WORKFLOW.md).

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

- Calendario em prod (2026-07-07): `Calendar Chat` (`calendarchat0001`), `Calendar Omni`
  (`calendaromni0001`) e `Calendar Transcribe` (`calendartrans001`) importados, publicados e
  validados estruturalmente contra os JSONs locais; n8n `2.23.2` healthy. As cinco envs
  `CALENDAR_*` foram configuradas na VPS e passaram a ser repassadas pela API no
  `docker-compose.prod.yml`. Backup anterior: `backups/n8n/workflows-before-calendar-20260707_124922.json`.
- A stack `automation` esta no `docker-compose.yml` (dev local) **e** no
  `docker-compose.prod.yml` (profile `automation`, infra preparada 2026-06-08). O deploy
  na VPS em si (Caddy, DNS, QR, ativacao, backups) e do Mike — passo a passo em
  docs/automation/SETUP.md secao 8.
- Decisao de prod (2026-06-08): exposicao via **Caddy + basic auth** (subdominios `n8n.`/
  `waha.`); **Redis so disponivel** na rede `app` (sem mexer na API Go ainda).
- Hardening WAHA aplicado em prod (2026-07-22): `gows-2026.7.1`, 2 GiB, autostart, filtro de
  status e auto-restart por falha de `/ping`; sessao persistida voltou `WORKING` sem QR e ficou
  estavel, sem OOM. Backup/rollback e evidencias estao em `docs/DEPLOY_VPS.md`.
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
      no Dados +`msgTimestamp`/`isSticker`/`isForwarded`/`isReply`/`quotedText`; Dedupe +boot
      cutoff 5min; Juntar +prefixo de contexto (figurinha/encaminhada/reply); AI Agent emoji
      endurecido (re-sync). Mesma mudanca em guardrails-resposta.md (fonte).
- [ ] **Validar ao vivo** apos importar: 1 figurinha + 1 reply + 1 msg antiga (boot). Os campos
      `isForwarded`/`isReply` dependem do que o engine **GOWS** entrega no payload (`_data.*`);
      se nao detectar, ajustar o caminho no no Dados (guards `(obj||{})` ja evitam erro).
- [ ] Se o n8n ja tiver o workflow importado, **re-importar** para pegar os ajustes acima.
- [ ] Prod: preencher `AUTOMATION_*` no `.env.production` (gerar `N8N_ENCRYPTION_KEY`, nao mudar depois).
- [ ] Futuro (BYOK): `AUTOMATION_CRED_ENC_KEY` (master key de cripto das chaves dos clientes).

**VPS / deploy**
- [ ] Rotas Caddy `n8n.`/`waha.` (basic auth) no projeto do proxy + DNS dos subdominios.
- [ ] Subir `--profile automation` na prod, importar workflow/credenciais, escanear QR, ativar (confirmar com o Mike).
- [ ] **Apos importar (ARMADILHA real 2026-06-19):** as credenciais do n8n vem com **host de DEV**
      (`host.docker.internal`) — reapontar **Redis** (`redis:6379` + `AUTOMATION_REDIS_PASSWORD`) e
      **WAHA** (`http://waha:3000`); reimportar o `workflow-whatsapp.json` atual e instalar o
      community node `n8n-nodes-waha` no volume `~/.n8n/nodes`; `docker restart` do n8n a cada troca.
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
