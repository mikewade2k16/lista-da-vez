# AGENT.md — meta-ads-assistant (agent-runner, fase MA1)

Leia antes: [@AGENT_RULES.md](../AGENT_RULES.md) e
[@docs/ENGINEERING_PRINCIPLES.md](../docs/ENGINEERING_PRINCIPLES.md).
Plano canonico: [docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md secao 12](../docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md).

## O que e

Sidecar Node (sem framework, so `node:http`) que roda o **Claude headless**
(`@anthropic-ai/claude-agent-sdk`) restrito ao **MCP oficial da Meta**
(`https://mcp.facebook.com/ads`). O Go (fase MA2) chama `POST /run` por rede
interna; o painel nunca fala direto com o runner.

## Arquivos

| Arquivo | Responsabilidade |
|---|---|
| `src/server.mjs` | HTTP (`/healthz`, `/run`, `/auth/start`, `/auth/complete`), auth Bearer em tempo constante, parsing/validacao (inclui `accountId`) |
| `src/session.mjs` | `MetaSession`: UMA `query()` persistente; turnos serializados; `ensure()` faz refresh OAuth + recria se model/prompt mudou; injeta `accountId` no contexto via `setAccountContext` |
| `src/agent.mjs` | Opcoes do `query()` (2 MCP: `meta-ads` http + `omni` in-process), header OAuth na conexao Meta, coleta de tool_use/tool_result de AMBOS os prefixos -> `actions[]`, trava anti-invencao |
| `src/oauth.mjs` | OAuth proprio do runner: discovery (RFC 9728/8414), DCR (RFC 7591), PKCE S256, listener do callback, troca/refresh de tokens |
| `src/oauth-store.mjs` | Persistencia em `.auth/` (client.json + tokens.json, dir 0700/arquivo 0600); nunca loga token |
| `src/omni-tools.mjs` | Servidor MCP in-process `omni` (`instagram_get_accounts`, `instagram_get_recent_posts`) que chama o bridge Go; `accountId` vem do contexto do turno, nao e parametro |
| `src/auth.mjs` | Login: tenta OAuth proprio (persistente) primeiro; fallback in-session via modelo se discovery/DCR falharem |
| `src/system-prompt.mjs` | System prompt PT-BR (confirmacao antes de write, campanha nasce pausada, fluxo do feed do Instagram) |
| `src/config.mjs` | Env vars + `claudeAuthStatus()` + `metaAuthStatus()` (oauth/session/none) do /healthz |
| `Dockerfile` | Imagem p/ VPS (profile `meta-ads-assistant`); dev roda no host |

## Contrato HTTP (congelado p/ MA2)

- `GET /healthz` -> `{ ok: true, claudeAuth: boolean, detail: string, metaAuth: 'oauth'|'session'|'none' }`
  - `oauth`: token proprio do runner valido em `.auth/tokens.json` (persiste no restart).
  - `session`: sem token em disco, mas a sessao MCP viva tem tools (fallback via modelo, NAO persiste).
  - `none`: deslogado.
- `POST /run` + `Authorization: Bearer <META_ADS_ASSISTANT_TOKEN>`
  - in: `{ prompt, history?: [{role:'user'|'assistant', content}], adAccountId?, accountId? }`
    - `adAccountId`: conta de anuncios da Meta (numerica), passada nas tools meta-ads.
    - `accountId`: conta do painel (`core.accounts`, uuid), usada pelas tools `omni` (feed IG).
  - out: `{ reply: string, actions: [{ tool, summary, status: 'ok'|'error' }] }`
  - erros: `503 runner_not_configured`, `401 unauthorized`, `400 invalid_body`,
    `413 body_too_large`, `504 assistant_timeout`, `502 assistant_error`
- `POST /auth/start` -> `{ url, mode: 'oauth'|'session', alreadyAuthed }`. mode=`oauth` =
  login persistente (token em disco). Sobe listener em `127.0.0.1:<callback port>` que
  captura o redirect automaticamente.
- `POST /auth/complete` + `{ callbackUrl? }` -> `{ ok, detail }`. callbackUrl pode vir
  vazio (o listener ja capturou code+state). No OAuth proprio, troca code por tokens
  (PKCE) e recria a sessao MCP para nascer com o header.

## OAuth proprio do runner (Problema 1 — token persistente)

Login antigo era feito PELO MODELO (tool `authenticate` do MCP) e valia so na conexao
viva — restart ou troca de model/prompt deslogava. Agora o runner faz o OAuth padrao de
MCP sozinho:

1. **Discovery**: `GET /.well-known/oauth-protected-resource/ads` (RFC 9728) ->
   `authorization_servers`; metadata do AS via `/.well-known/oauth-authorization-server`
   (RFC 8414) -> `authorization_endpoint`, `token_endpoint`, `registration_endpoint`.
2. **DCR** (RFC 7591): registra `omni-meta-ads-runner` (`token_endpoint_auth_method:none`),
   persiste `client_id` em `.auth/client.json` (reusa entre boots).
3. **PKCE S256** + `state`; `redirect_uri = http://127.0.0.1:<META_ADS_OAUTH_CALLBACK_PORT>/oauth/callback`.
4. **/auth/start**: monta a URL de autorizacao (sem modelo) e sobe o listener do callback.
5. **/auth/complete**: troca `code` por tokens, salva em `.auth/tokens.json` (0600),
   recria a sessao MCP -> proxima conexao nasce com `Authorization: Bearer`.
6. **Refresh**: antes de cada sessao, se o token expira em < 60s e ha `refresh_token`,
   renova; se falhar, apaga os tokens (volta a deslogado / fallback).
7. **Fallback**: se discovery/DCR falharem, mantem o fluxo legado via modelo (token nao
   persiste; `detail` avisa).

`.auth/` esta no `.gitignore` e `.dockerignore` — tokens nunca versionados nem na imagem.

## Bridge do Instagram (Problema 2 — MCP `omni` in-process)

O MCP da Meta nao expoe o feed do Instagram. O backend Go expoe um bridge interno e o
runner publica como tools custom (`createSdkMcpServer` + `tool`, schema zod):

- `instagram_get_accounts()` -> `GET {META_ADS_API_BASE}/internal/meta-ads/runner/instagram/accounts?accountId=`
- `instagram_get_recent_posts({ limit?:1..20=5, igUserId? })` -> `.../instagram/media?accountId=&limit=&igUserId=`
- Auth: `Authorization: Bearer <META_ADS_RUNNER_BRIDGE_TOKEN>`. Token vazio => tools
  respondem "bridge nao configurada" (nunca chamam sem auth).
- `accountId` NAO e parametro (o modelo nao escolhe a conta): vem do contexto do turno
  (`setAccountContext` no `execTurn`). Erros da bridge viram texto amigavel no tool result.

## Decisoes de SDK (verificadas contra sdk.d.ts 0.3.x instalado)

- `tools: []` zera as built-in; `disallowedTools` bloqueia por nome (cinto e
  suspensorio); `allowedTools: ['mcp__meta-ads', 'mcp__meta-ads__*']` pre-aprova
  o servidor; `canUseTool` nega qualquer outra coisa sem prompt (headless);
  `permissionMode: 'default'` (decisoes caem no `canUseTool`).
- `strictMcpConfig: true` + `settingSources: []` isolam o runner do `.mcp.json`
  e settings do host; o MCP usado e SO o passado em `mcpServers`.
- Multi-turn: runner **stateless**; o historico chega no body e e prefixado como
  transcript (`<historico_da_conversa>`) no prompt do usuario. Sem resume/sessao.
- Timeout via `abortController` + `AssistantTimeoutError` -> 504.
- Auth do Claude: herda o login do host (`~/.claude`) ou `CLAUDE_CODE_OAUTH_TOKEN`
  (gerado com `claude setup-token`). Tokens nunca logados.

## Env vars

`META_ADS_ASSISTANT_PORT` (8765), `META_ADS_ASSISTANT_TOKEN` (obrigatoria p/ /run),
`META_ADS_ASSISTANT_TIMEOUT_MS` (120000), `META_ADS_ASSISTANT_MAX_TURNS` (25),
`META_ADS_MCP_URL` (`https://mcp.facebook.com/ads`), `META_ADS_ASSISTANT_MODEL`,
`CLAUDE_CODE_OAUTH_TOKEN`.

Novas (OAuth proprio + bridge IG):
- `META_ADS_OAUTH_CALLBACK_PORT` (8766) — porta do listener local do callback OAuth.
- `META_ADS_API_BASE` (`http://localhost:9091`) — base da API Go do bridge do Instagram.
- `META_ADS_RUNNER_BRIDGE_TOKEN` (sem default) — Bearer do bridge; vazio = tools `omni`
  respondem "bridge nao configurada".

No compose, o servico `api` recebe `META_ADS_ASSISTANT_RUNNER_URL` e
`META_ADS_ASSISTANT_TOKEN` para o proxy MA2.

## Guardrails de produto

1. Toda escrita (create/update/activate) exige confirmacao explicita do usuario
   no chat antes de executar (regra no system prompt).
2. Campanha criada via MCP nasce PAUSADA (guardrail nativo da Meta); ativar so
   com pedido explicito.
3. Runner interno: Bearer token, sem exposicao publica, sem tools de
   arquivo/terminal/web na sessao do agente.

## Setup unico (humano)

1. Login Claude no host (ou `claude setup-token` p/ container/VPS).
2. OAuth do MCP da Meta pelo painel (aba Conexoes -> "Conectar Meta"): abre a URL
   de autorizacao (login Facebook), o callback e capturado em
   `127.0.0.1:<META_ADS_OAUTH_CALLBACK_PORT>` e o token fica em `.auth/tokens.json`.
   Persiste no restart e no recreate da sessao (nao precisa relogar). Se o servidor
   MCP nao suportar OAuth padrao, cai no login legado via modelo (nao persiste).
