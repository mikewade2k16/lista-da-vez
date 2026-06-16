# meta-ads-assistant (agent-runner)

Servico Node interno do modulo meta_ads (fase MA1 do plano canonico
[docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md, secao 12](../docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md)).
Roda o Claude headless via `@anthropic-ai/claude-agent-sdk` com o MCP oficial da
Meta (`https://mcp.facebook.com/ads`) para criar/editar campanhas a partir de
comandos de texto enviados pelo painel (via API Go, fase MA2).

Regras do projeto: [AGENT_RULES.md](../AGENT_RULES.md) e
[docs/ENGINEERING_PRINCIPLES.md](../docs/ENGINEERING_PRINCIPLES.md).

## Como rodar (modo primario: no HOST)

O OAuth do MCP da Meta (login Facebook) fica cacheado nas credenciais do Claude
do host (`~/.claude`). Por isso, em dev o runner roda direto no host, nao em
container:

```
cd meta-ads-assistant
npm install
set META_ADS_ASSISTANT_TOKEN=dev-meta-assistant-token   (PowerShell: $env:META_ADS_ASSISTANT_TOKEN="dev-meta-assistant-token")
npm start
```

O api (container) alcanca o runner por `http://host.docker.internal:8765`
(`META_ADS_ASSISTANT_RUNNER_URL` no compose).

### Pre-requisitos (uma unica vez)

1. **Login do Claude**: o host ja logado no Claude Code serve (`~/.claude`).
   Para container/VPS, gere um token de longa duracao com `claude setup-token`
   e exporte como `CLAUDE_CODE_OAUTH_TOKEN`.
2. **OAuth do MCP da Meta**: autentique o servidor `meta-ads` uma vez de forma
   interativa no Claude Code (comando `/mcp`, login Facebook). O token fica
   cacheado nas credenciais e o runner headless reaproveita.

## Endpoints

- `GET /healthz` -> `{ ok, claudeAuth, detail }`. `claudeAuth` e best-effort
  (presenca de `CLAUDE_CODE_OAUTH_TOKEN`/`ANTHROPIC_API_KEY` ou de
  `~/.claude/.credentials.json`); nao faz chamada de rede.
- `POST /run` (exige `Authorization: Bearer <META_ADS_ASSISTANT_TOKEN>`)
  - body: `{ "prompt": string, "history"?: [{"role":"user"|"assistant","content":string}], "adAccountId"?: string }`
  - resposta: `{ "reply": string, "actions": [{"tool","summary","status":"ok"|"error"}] }`
  - sem token configurado -> `503 runner_not_configured`; token errado -> `401`;
    estouro de `META_ADS_ASSISTANT_TIMEOUT_MS` -> `504 assistant_timeout`.

## Variaveis de ambiente

| Variavel | Default | Descricao |
|---|---|---|
| `META_ADS_ASSISTANT_PORT` | `8765` | Porta do servico (bind 0.0.0.0) |
| `META_ADS_ASSISTANT_TOKEN` | (vazio) | Token Bearer exigido no /run; vazio desliga o runner |
| `META_ADS_ASSISTANT_TIMEOUT_MS` | `120000` | Timeout por execucao do agente |
| `META_ADS_ASSISTANT_MAX_TURNS` | `25` | Limite de turnos por execucao |
| `META_ADS_MCP_URL` | `https://mcp.facebook.com/ads` | URL do MCP oficial da Meta |
| `META_ADS_ASSISTANT_MODEL` | (vazio = default da assinatura) | Override de modelo |
| `CLAUDE_CODE_OAUTH_TOKEN` | (vazio) | Auth do Claude p/ container/VPS (`claude setup-token`) |

## Modelo de seguranca

- Servico **interno** (rede do compose / localhost). Nunca expor em proxy publico.
- `POST /run` exige Bearer token com comparacao em tempo constante (espelha o
  `bearerEquals` do modulo automation no Go).
- O agente roda **sem nenhuma tool built-in** (`tools: []` + `disallowedTools`):
  apenas as tools `mcp__meta-ads__*` sao permitidas e auto-aprovadas; qualquer
  outra e negada sem prompt (`canUseTool`). Settings/CLAUDE.md do host nao sao
  carregados (`settingSources: []`, `strictMcpConfig: true`).
- Campanhas criadas via MCP nascem PAUSADAS; o system prompt exige confirmacao
  explicita do usuario antes de qualquer escrita e proibe ativar sem pedido.
- Tokens nunca sao logados.

## Container (VPS/futuro)

`docker compose --profile meta-ads-assistant up -d meta-ads-assistant` builda
esta pasta. Necessario `CLAUDE_CODE_OAUTH_TOKEN` no `.env` (nao ha `~/.claude`
no container) e repetir o OAuth do MCP uma vez no ambiente da VPS.
