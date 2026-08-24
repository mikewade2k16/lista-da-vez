# AGENT.md - meta-ads-assistant

Leia antes: [AGENT_RULES.md](../AGENT_RULES.md) e
[docs/ENGINEERING_PRINCIPLES.md](../docs/ENGINEERING_PRINCIPLES.md).
Plano canonico: [docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md](../docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md).

## Escopo e fronteira de confianca

Sidecar Node **legado/compatibilidade interna** que roda o Claude Agent SDK com o
MCP oficial da Meta e o bridge Instagram do Go. O painel nunca chama este
processo diretamente. O Assistente 360 e o executor first-party funcionam sem
este runner; nao o adicionar ao caminho oficial de produto ou ao compose de
producao sem uma nova decisao arquitetural.

- O Go autenticado resolve membership, RBAC e `Principal.AccountID` e envia o
  UUID ao runner com Bearer de servico.
- O runner valida e normaliza o UUID em toda rota, mas nao substitui a checagem
  de membership do Go.
- Nunca expor o runner em proxy publico.
- `/run` e estritamente **somente leitura**. Escritas da Meta permanecem negadas
  em `canUseTool`; so poderao existir em endpoint separado com proposta
  persistida, aprovacao e idempotencia verificadas pelo Go.

## Arquivos

| Arquivo                  | Responsabilidade                                                                       |
| ------------------------ | -------------------------------------------------------------------------------------- |
| `src/server.mjs`         | HTTP, Bearer em tempo constante, limite de body e validacao obrigatoria de `accountId` |
| `src/account-id.mjs`     | normalizacao/validacao unica de UUID                                                   |
| `src/session.mjs`        | `MetaSession` persistente por account e integracao com o pool                          |
| `src/session-pool.mjs`   | pool LRU limitado; lease protege sessao ativa de eviccao; `close()` no descarte        |
| `src/agent.mjs`          | opcoes do Agent SDK, MCP Meta + Omni e gate final read-only                            |
| `src/oauth.mjs`          | discovery, DCR global, PKCE, troca e refresh por account                               |
| `src/oauth-callback.mjs` | listener local e roteamento de callbacks pelo `state`                                  |
| `src/oauth-pending.mjs`  | registry de fluxos PKCE pendentes por account                                          |
| `src/oauth-store.mjs`    | DCR global e tokens em cofre separado por account                                      |
| `src/omni-tools.mjs`     | MCP in-process; `accountId` capturado por closure imutavel                             |
| `src/auth.mjs`           | OAuth proprio e fallback in-session, ambos tenant-scoped                               |
| `src/config.mjs`         | env e status Claude/Meta da account                                                    |
| `src/system-prompt.mjs`  | prompt PT-BR; explicita que o chat atual nao escreve na Meta                           |
| `test/*.test.mjs`        | contrato HTTP, UUID, storage, PKCE, pool, bridge e trava read-only                     |

## Contrato HTTP atual

Todas as rotas abaixo exigem
`Authorization: Bearer <META_ADS_ASSISTANT_TOKEN>`. Token de servico ausente
retorna `503 runner_not_configured`; Bearer invalido retorna `401 unauthorized`.

### `GET /healthz?accountId=<uuid>`

`accountId` e obrigatorio na query. Resposta:

```json
{
  "ok": true,
  "claudeAuth": true,
  "detail": "...",
  "metaAuth": "oauth"
}
```

`metaAuth` e calculado somente para a account pedida: `oauth`, `session` ou
`none`. Health nao chama o modelo nem a Meta.

### `POST /run`

```json
{
  "accountId": "uuid-obrigatorio",
  "prompt": "texto obrigatorio",
  "history": [{ "role": "user", "content": "..." }],
  "adAccountId": "id-meta-opcional",
  "model": "opcional",
  "systemPrompt": "opcional"
}
```

Resposta: `{ reply, actions: [{ tool, summary, status }] }`.
Timeout retorna `504 assistant_timeout`; pool totalmente ocupado retorna
`503 session_capacity`. Toda tool de escrita recebe `deny`, mesmo que o modelo a
solicite. As unicas tools Omni liberadas sao
`instagram_get_accounts` e `instagram_get_recent_posts`.

### `POST /auth/start`

Body: `{ accountId, model?, systemPrompt? }`, com UUID obrigatorio. Resposta:
`{ url, mode: "oauth"|"session", alreadyAuthed }`.

Fluxos OAuth de varias accounts podem coexistir: um listener local unico roteia
o callback pelo `state` aleatorio para o registry da account correta. Repetir o
start da mesma account reaproveita a URL ainda pendente. Se a porta estiver
ocupada por outro processo, falha fechado com `409 oauth_callback_conflict`.

### `POST /auth/complete`

Body: `{ accountId, callbackUrl?, model?, systemPrompt? }`, com UUID
obrigatorio. `callbackUrl` vazio preserva o fluxo em que o listener local ja
capturou o redirect; a URL completa colada continua suportada. O `state` e
obrigatoriamente comparado ao fluxo daquela account. Fluxo ausente/expirado
retorna `409 auth_session_gone`.

Em qualquer rota, `accountId` ausente/malformado retorna
`400 invalid_account_id` antes de abrir sessao, acessar disco ou chamar rede.

## Isolamento multi-tenant

### OAuth em disco

- DCR global: `.auth/client.json`. E global porque identifica este processo e o
  mesmo redirect URI; o registro inclui o redirect usado no criterio de reuso.
- Tokens: `.auth/accounts/<accountId>/tokens.json`, incluindo o `client_id` e o
  `token_endpoint` usados naquele fluxo; refresh nao consulta o DCR mais recente
  de outra account.
- Diretorios tentam `0700`; arquivos tentam `0600`.
- O UUID estrito e a checagem de path confinado impedem traversal.
- O antigo `.auth/tokens.json` global e **ignorado** e nao e atribuido
  automaticamente a nenhuma account. Cada account deve reconectar uma vez.
- `currentAccessToken(accountId)`, `refreshIfNeeded(accountId)` e todas as
  operacoes do store exigem o escopo explicitamente.

### Sessoes Claude/MCP

- Uma `MetaSession` por account; nunca ha `query()`, auth in-session, tool state
  ou fila compartilhada entre tenants.
- Turnos da mesma account sao serializados.
- Pool default: ate 24 sessoes; ociosidade default: 15 minutos.
- Ao atingir o limite, a sessao LRU sem lease e fechada. Se todas estiverem
  ocupadas, falha com `session_capacity`; sessao ativa nunca e removida.
- Alterar modelo/prompt ou concluir OAuth recria somente a sessao da account.

### Bridge Omni

`createOmniMcpServer(accountId)` cria as tools com UUID capturado por closure.
O modelo nao recebe `accountId` como argumento de tool e nao consegue escolher
ou trocar tenant. Nao existe setter nem contexto global mutavel.

## OAuth

Fluxo preferencial: discovery RFC 9728/8414, DCR RFC 7591, Authorization Code +
PKCE S256, troca, persistencia tenant-scoped e refresh. O fallback legado chama
`authenticate`/`complete_authentication` pelo modelo, mas fica preso a sessao da
account e nao persiste no restart.

O listener usa `127.0.0.1:<META_ADS_OAUTH_CALLBACK_PORT>`. Nunca logar callback,
code, state, access token, refresh token, headers, prompts ou mensagem crua de
erro. Logs operacionais incluem somente nome/codigo do erro e contagens.

## Variaveis de ambiente

| Variavel                             |                        Default | Uso                                  |
| ------------------------------------ | -----------------------------: | ------------------------------------ |
| `META_ADS_ASSISTANT_PORT`            |                         `8765` | porta HTTP interna                   |
| `META_ADS_ASSISTANT_TOKEN`           |                          vazio | Bearer obrigatorio em todas as rotas |
| `META_ADS_ASSISTANT_TIMEOUT_MS`      |                       `120000` | timeout de turno                     |
| `META_ADS_ASSISTANT_MAX_TURNS`       |                           `25` | limite de tool turns                 |
| `META_ADS_ASSISTANT_MAX_SESSIONS`    |                           `24` | limite de accounts com sessao viva   |
| `META_ADS_ASSISTANT_SESSION_IDLE_MS` |                       `900000` | TTL ocioso do pool                   |
| `META_ADS_MCP_URL`                   | `https://mcp.facebook.com/ads` | MCP oficial                          |
| `META_ADS_ASSISTANT_MODEL`           |                          vazio | modelo default opcional              |
| `META_ADS_OAUTH_CALLBACK_PORT`       |                         `8766` | callback local PKCE                  |
| `META_ADS_API_BASE`                  |        `http://localhost:9091` | bridge Go                            |
| `META_ADS_RUNNER_BRIDGE_TOKEN`       |                          vazio | Bearer do bridge Go                  |
| `CLAUDE_CODE_OAUTH_TOKEN`            |                          vazio | credencial Claude em container/VPS   |

## Validacao

```bash
npm test
node --check src/server.mjs
```

O projeto e Docker-first; se Node nao existir no host, rode os mesmos comandos
em uma imagem Node com esta pasta montada. Nao executar login OAuth real em
teste automatizado e nunca versionar `.auth/`.
