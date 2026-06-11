# SETUP — Subir e validar o modulo automation (n8n + WAHA)

> Runbook para subir a stack do bot DENTRO do projeto Omni (profile `automation`) e
> deixar o workflow pronto. Migrado do projeto standalone "n8n Whatsapp" em 2026-06-04.
> A recipe dos containers agora vive no `docker-compose.yml` da RAIZ do Omni (servicos
> `n8n`, `waha`, `redis` sob `profiles: ["automation"]`).
> Visao/decisoes: [AGENTS.md](AGENTS.md) - plano: [WORKFLOW.md](WORKFLOW.md) - status:
> [ROADMAP.md](ROADMAP.md) - modelos: [MODELOS.md](MODELOS.md) - modulo:
> [../../automation/AGENT.md](../../automation/AGENT.md).

## 1. Versoes (fixadas para reprodutibilidade)

| Componente | Versao | Imagem / pacote |
|---|---|---|
| n8n | **2.23.2** | `n8nio/n8n:2.23.2` |
| WAHA | **2026.5.1** (engine GOWS, CORE) | `devlikeapro/waha:gows-2026.5.1` |
| Redis | 8.x | `redis:8-alpine` |
| Community node (n8n) | **2024.11.5** | `n8n-nodes-waha` |

> O Postgres dedicado do projeto standalone foi descartado na migracao (o n8n guarda a
> config em SQLite, no volume `automation_n8n_data`). Quando a Etapa 2 (mini-CRM) chegar,
> o bot usa o Postgres do proprio Omni.
> Se algum tag nao existir no registry, caia para `latest` — mas convem manter o **n8n**
> em 2.23.2 (compatibilidade dos nos do workflow).
> **Atencao ao tag da WAHA:** os tags sao prefixados pelo engine — use `gows-<versao>`
> (ex.: `gows-2026.5.1`), NAO `<versao>` puro (esse manifest nao existe e o pull falha).
> Engine GOWS = `gows-*`; alternativas: `latest-*` (WEBJS), `noweb-*`, `chrome-*`.

## 2. Estrutura no Omni

```
(raiz do Omni)
├─ docker-compose.yml          <- recipe canonica (servicos n8n/waha/redis no profile automation)
├─ .env.docker.example         <- AUTOMATION_N8N_PORT / WAHA / REDIS / REDIS_PASSWORD (opcionais)
├─ automation/
│  ├─ AGENT.md                          <- doc do modulo
│  ├─ .mcp.json                         <- config MCP/Claude do n8n (SEGREDO; regerar a key)
│  ├─ .gitignore                        <- protege segredos do modulo
│  ├─ docker-compose.reference.yml      <- recipe ORIGINAL standalone (so referencia)
│  └─ export/
│     ├─ workflow-whatsapp.json         <- O WORKFLOW completo (36 nos; persona+guardrails embutidos)
│     └─ credentials.decrypted.json     <- credenciais do n8n (SEGREDOS em texto puro)
└─ docs/automation/
   ├─ SETUP.md (este)  AGENTS.md  WORKFLOW.md  ROADMAP.md  MODELOS.md  HANDOFF.md
   ├─ gpt-tony.md  gpt-perola-buyer-assistant.md  guardrails-resposta.md   (fontes dos prompts)
   └─ roadmap.html  roadmap-server.js                                       (dashboard do roadmap)
```

> Os comandos `docker compose ...` rodam a partir da **raiz do Omni** (onde esta o
> `docker-compose.yml`). Dashboard do roadmap do bot:
> `node docs/automation/roadmap-server.js` -> http://localhost:8088/roadmap.html.

## 3. Passo a passo

### 3.1 Subir os containers (profile automation)
```bash
docker compose --profile automation up -d
```
Sobe `n8n`, `waha`, `redis` (e tambem `api`/`web`/`postgres` do Omni, se ainda nao
estiverem de pe). Portas no host: n8n `5680` - WAHA `3010` - Redis `6380`. Para mudar,
defina `AUTOMATION_*_PORT` no `.env`.

> Os volumes do projeto Omni sao novos (`automation_n8n_data`, `automation_waha_sessions`,
> `automation_waha_media`, `automation_redis`). A stack comeca do zero: e preciso
> reimportar workflow/credenciais e reescanear o QR (ver secao 5).

### 3.2 Instalar o community node WAHA (OBRIGATORIO antes de importar)
Sem isso os nos WAHA e a credencial `wahaApi` nao existem e a importacao quebra.
- Abra o n8n em http://localhost:5680 -> **Settings -> Community Nodes -> Install** ->
  pacote `n8n-nodes-waha` (versao `2024.11.5`).
- Reinicie o n8n: `docker compose --profile automation restart n8n`.

### 3.3 Importar credenciais (antes do workflow)
```bash
docker compose --profile automation cp automation/export/credentials.decrypted.json n8n:/tmp/creds.json
docker compose --profile automation exec n8n n8n import:credentials --input=/tmp/creds.json
```
> Os IDs das credenciais sao preservados, entao o workflow ja vai apontar para elas.
> Se a OpenAI key tiver expirado/rotacionada, atualize a credencial "OpenAI account" no n8n.
> A credencial "Redis account" usa host `redis`, porta `6379`, senha = `AUTOMATION_REDIS_PASSWORD`
> (default `default`). Se voce mudar a senha no `.env`, atualize a credencial.

### 3.4 Importar o workflow
```bash
docker compose --profile automation cp automation/export/workflow-whatsapp.json n8n:/tmp/wf.json
docker compose --profile automation exec n8n n8n import:workflow --input=/tmp/wf.json
```

### 3.5 Conectar o WhatsApp (WAHA)
- Abra o dashboard da WAHA em http://localhost:3010 (sem senha).
- Inicie a sessao `default` e **escaneie o QR Code** com o WhatsApp.
- O webhook ja aponta para o n8n pela rede interna do Compose:
  `WHATSAPP_HOOK_URL=http://n8n:5678/webhook/webhook` (definido no compose; nao usa mais
  `host.docker.internal`).

### 3.6 Ativar
- No n8n, abra o workflow **"Whatsapp"** e clique em **Active**.
- Mande uma mensagem de teste de outro numero.

> Ativar faz o bot responder no WhatsApp real conectado. Confirmar com o Mike antes.

### 3.7 (Opcional) Religar o MCP / Claude
- No n8n: **Settings -> n8n API -> Create API key**.
- Cole a key em `automation/.mcp.json` (campo `N8N_API_KEY`) e recarregue o Claude Code.
  (Ja tem `WEBHOOK_SECURITY_MODE=moderate` para liberar localhost.)
- A key antiga do `.mcp.json` e da instancia standalone e NAO funciona no n8n novo.

## 4. Credenciais (referencia)

| Nome | Tipo | Config |
|---|---|---|
| OpenAI account | `openAiApi` | API key da plataforma platform.openai.com (com creditos) |
| WAHA account | `wahaApi` | URL `http://waha:3000` (rede interna), sem API key (`WAHA_NO_API_KEY=true`) |
| Redis account | `redis` | host `redis`, porta `6379`, senha `AUTOMATION_REDIS_PASSWORD` (default `default`) |
| Google Gemini(PaLM) Api account | `googlePalmApi` | legado/Gemini — **nao usado** atualmente |

## 5. O que NAO migra (esperado)

A migracao para o projeto Omni usa volumes novos, entao comeca limpo:
- **Sessao do WhatsApp** (`automation_waha_sessions`): re-escaneie o QR.
- **Config/workflow/credenciais do n8n** (`automation_n8n_data`, SQLite): reimporte
  (passos 3.2 a 3.4) e reinstale o community node.
- **Memoria de conversa** (Redis + `staticData` do workflow): comeca zerada — segmentos,
  memoria longa e buffer de debounce sao runtime, nao config.
- **Execucoes** antigas.

## 6. Seguranca

- `automation/.mcp.json` e `automation/export/credentials.decrypted.json` contem
  **segredos** (OpenAI key etc.). Ja ignorados pelo `.gitignore` do modulo e da raiz.
  Considere **rotacionar** as chaves depois da migracao.

## 7. Arquitetura atual (resumo)

```
Webhook(WAHA) -> Dados -> Dedupe -> Switch(filtro 1:1) -> Tipo(mimetype):
   audio  -> Baixar -> Whisper -> Texto do audio --+
   imagem -> Baixar -> Visao(gpt-4o) -> Texto da imagem --+
   texto  -----------------------------------------------+
        -> [DEBOUNCE: Fila push/token -> Wait 7s -> Eh ultima? -> junta] (Redis)
        -> Ctx: ler(memoria longa) -> Ctx: classificar(gpt-4o-mini NOVO x CONTINUA) -> Ctx: aplicar(segmento)
        -> AI Agent (Tony, gpt-5.3-chat-latest) <- Redis Chat Memory (chatId_segmento)
        -> Send Seen -> Dividir(baloes) -> Loop[ Digitando + Wait + Send Text ] -> Resumir(memoria longa) -> salvar
```
Modelos: chat `gpt-5.3-chat-latest` - visao `gpt-4o` - audio `whisper-1` -
classificador/resumidor `gpt-4o-mini`.

## 8. Producao (VPS) — rodar online

> A stack `automation` ja esta no `docker-compose.prod.yml` (profile `automation`),
> preparada em 2026-06-08. Os servicos tem os **mesmos nomes do dev** (`redis`/`waha`/
> `n8n`), entao o workflow e as credenciais importadas funcionam **igual local e prod**.
> O deploy em si (subir na VPS, escanear o QR, ativar) e do Mike — aqui esta o passo a passo.

### 8.1 Modelo de rede (igual ao resto do Omni)
- `n8n` e `waha` entram na rede `proxy` (externa, `omnichannel-mvp_default`) com aliases
  `automation-n8n` / `automation-waha`. O **Caddy** roteia por subdominio com **basic auth**.
- `redis` fica **so na rede `app`** (nunca publico) e ja disponivel para a API do Omni
  usar no futuro em `redis:6379`.
- O **webhook WAHA -> n8n e interno** (`http://n8n:5678/webhook/webhook`) — nunca sai da rede.
- As portas `127.0.0.1:15680/13010/16380` sao so para **tunel SSH/debug**, nao publicas.

### 8.2 Variaveis (.env.production)
Copie do `.env.production.example` e preencha o bloco `AUTOMATION_*`:
```
AUTOMATION_N8N_HOST=n8n.crowvisuals.com.br
AUTOMATION_N8N_WEBHOOK_URL=https://n8n.crowvisuals.com.br
AUTOMATION_N8N_ENCRYPTION_KEY=<openssl rand -hex 24>   # gere 1x e NAO mude depois
AUTOMATION_WAHA_HOST=waha.crowvisuals.com.br
AUTOMATION_WAHA_DASHBOARD_USER=admin
AUTOMATION_WAHA_DASHBOARD_PASSWORD=<senha forte>
AUTOMATION_REDIS_PASSWORD=<senha forte>               # = credencial "Redis account" no n8n
```

### 8.3 Caddy (no projeto do proxy, fora deste repo)
Gere o hash do basic auth com `caddy hash-password` e adicione duas rotas:
```
n8n.crowvisuals.com.br {
    basic_auth { admin <BCRYPT_HASH> }
    reverse_proxy automation-n8n:5678
}
waha.crowvisuals.com.br {
    basic_auth { admin <BCRYPT_HASH> }
    reverse_proxy automation-waha:3000
}
```
Aponte os DNS `n8n.` e `waha.` para a VPS. Apos editar, recarregue o Caddy.

### 8.4 Subir e configurar
```bash
docker compose -f docker-compose.prod.yml --profile automation up -d
```
Depois, **uma vez** (a stack comeca limpa, como no local). Container do n8n na prod:
`omni-n8n-1` (project `omni`).
1. Crie a conta dona do n8n (primeiro acesso em `https://n8n.crowvisuals.com.br`).
2. Instale o community node `n8n-nodes-waha` (2024.11.5) pela UI (Settings -> Community Nodes).
3. **Credenciais (segredo, NAO vem no git):** `credentials.decrypted.json` e gitignored, entao
   nao chega na VPS pelo `git pull`. Copie a parte:
   ```bash
   scp automation/export/credentials.decrypted.json USER@VPS:/srv/omni/automation/export/
   ```
   (O `workflow-whatsapp.json` **e** versionado, esse vem no git.)
4. Importe credenciais + workflow (na VPS, Linux — sem o problema de path do Windows):
   ```bash
   docker cp automation/export/credentials.decrypted.json omni-n8n-1:/tmp/creds.json
   docker exec omni-n8n-1 n8n import:credentials --input=/tmp/creds.json
   docker cp automation/export/workflow-whatsapp.json omni-n8n-1:/tmp/wf.json
   docker exec omni-n8n-1 n8n import:workflow --input=/tmp/wf.json
   docker exec omni-n8n-1 n8n update:workflow --all --active=false   # garante inativo
   ```
   Se a OpenAI key estiver expirada/rotacionada, atualize a credencial "OpenAI account" no n8n.
5. Escaneie o QR no dashboard da WAHA (`https://waha.crowvisuals.com.br`) — sessao `default`.
6. Ative o workflow **so quando for pra valer** — responde no WhatsApp real.

### 8.6 Licoes do teste local (aplicar no prod) — 2026-06-08
- **Use um numero DEDICADO para o bot, nao o pessoal.** Quando ativo, o bot responde
  **automaticamente qualquer pessoa** que mandar mensagem 1:1 (o filtro so ignora grupo/
  canal/broadcast e as proprias mensagens `fromMe`). Num numero pessoal isso responde teus
  contatos reais.
- **Tag da WAHA:** `devlikeapro/waha:gows-<versao>` (engine GOWS), nunca `<versao>` puro.
- **Importar deixa o workflow inativo por padrao** (rode o `update:workflow --all --active=false`
  por seguranca); so ative apos QR conectado e com intencao de ir ao ar.

### 8.7 M2 — comportamento config-driven (persona do banco)
A persona vive em `automation.personas` (seed Tony/Crow) e o n8n a consome via
`runtime-config`. Para ativar (vale dev e prod):
1. Defina **`AUTOMATION_RUNTIME_TOKEN`** no `.env` (prod: forte, `openssl rand -hex 24`).
   api e n8n usam o MESMO valor.
2. `docker compose ... up -d --build api` — sobe o modulo + migrations `0140`/`0141`.
3. Restart do n8n (pega o `$env`): `docker compose ... up -d n8n`.
4. **Re-importe o workflow** (`automation/export/workflow-whatsapp.json` ja traz o no
   `Get runtime config` + o gate `Bot ligado?`): `n8n import:workflow --input=...`.
5. Ative e teste. A partir daqui, editar a persona/o liga-desliga no banco muda o bot sem
   tocar no n8n. (Ate o passo 4, o bot usa a persona embutida no workflow — quick-win.)

### 8.5 Backups (volumes)
Os dados vivem em volumes nomeados: `automation_n8n_data` (config/workflow/credenciais
do n8n), `automation_waha_sessions` (sessao do WhatsApp — perder = reescanear QR),
`automation_waha_media`, `automation_redis` (memoria de conversa, descartavel). Inclua os
tres primeiros na rotina de backup da VPS.
