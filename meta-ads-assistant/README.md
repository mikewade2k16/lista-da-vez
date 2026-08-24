# meta-ads-assistant

Sidecar Node legado/compatibilidade interna do modulo Meta Ads. Ele executa o Claude Agent SDK com o
MCP oficial da Meta e o bridge de Instagram do Omni. O runner e tenant-scoped e
o endpoint `/run` e estritamente somente leitura. O Assistente 360 usa
`/v1/assistant/chat/*` e as escritas confirmadas usam o executor Graph first-party
no backend Go; este processo nao e um chat ou executor alternativo.

O painel nunca chama este servico diretamente. A API Go resolve a account pelo
Principal autenticado, aplica RBAC e envia `accountId` com Bearer interno.

## Executar

```bash
cd meta-ads-assistant
npm install
npm start
```

Variaveis principais:

- `META_ADS_ASSISTANT_TOKEN`: Bearer interno obrigatorio em todas as rotas.
- `META_ADS_ASSISTANT_PORT`: porta HTTP, default `8765`.
- `META_ADS_ASSISTANT_TIMEOUT_MS`: timeout de turno, default `120000`.
- `META_ADS_ASSISTANT_MAX_TURNS`: limite do agente, default `25`.
- `META_ADS_ASSISTANT_MAX_SESSIONS`: pool por account, default `24`.
- `META_ADS_ASSISTANT_SESSION_IDLE_MS`: TTL ocioso, default `900000`.
- `META_ADS_MCP_URL`: default `https://mcp.facebook.com/ads`.
- `META_ADS_OAUTH_CALLBACK_PORT`: callback PKCE local, default `8766`.
- `META_ADS_API_BASE` e `META_ADS_RUNNER_BRIDGE_TOKEN`: bridge interno Go.
- `CLAUDE_CODE_OAUTH_TOKEN`: autenticacao Claude para container/VPS.

## Endpoints

Todos exigem `Authorization: Bearer <META_ADS_ASSISTANT_TOKEN>`:

- `GET /healthz?accountId=<uuid>`
- `POST /run` com `{ accountId, prompt, history?, adAccountId?, model?, systemPrompt? }`
- `POST /auth/start` com `{ accountId, model?, systemPrompt? }`
- `POST /auth/complete` com `{ accountId, callbackUrl?, model?, systemPrompt? }`

`accountId` e obrigatorio e deve ser UUID. O health retorna o `metaAuth` daquela
account. Os tokens vivem em `.auth/accounts/<accountId>/tokens.json`; o registro
DCR permanece global em `.auth/client.json`. O arquivo global legado
`.auth/tokens.json` nao e reutilizado, evitando atribuir uma credencial antiga ao
tenant errado. Na primeira execucao desta versao, cada account precisa
reconectar uma vez para criar seu proprio cofre. Cada arquivo tenant-scoped tambem
guarda o `client_id` e o token endpoint usados no fluxo, para que refresh de uma
account nunca dependa do ultimo DCR gravado por outra.

No Compose, `/app/.auth` fica no volume nomeado `meta_ads_assistant_auth`, portanto
restart/rebuild do container preserva os tokens por account. O healthcheck usa
Bearer interno e um UUID tecnico apenas para validar o processo; esse UUID nao
recebe token nem contexto de cliente.

O callback colado continua aceito. Um listener local unico multiplexa callbacks
simultaneos pelo `state` de cada account. Porta ocupada por outro processo falha
fechado com `409 oauth_callback_conflict`.

## Seguranca

- Sessao Claude/MCP e fila separadas por account, com limite LRU e fechamento.
- `accountId` do bridge e capturado por closure; nao existe global mutavel.
- Sem tools built-in de arquivo, terminal ou web.
- Gate final usa allowlist fechada para as cinco leituras Meta conhecidas,
  autenticacao e as duas leituras do Instagram; toda outra tool recebe `deny`.
- Nenhum token, callback, header, prompt ou mensagem crua de erro vai para logs.
- `.auth/` e ignorado pelo Git e pelo build context; no container ele e montado
  exclusivamente pelo volume persistente.

Detalhes de contrato e manutencao: [AGENT.md](./AGENT.md).

## Testes

```bash
npm test
node --check src/server.mjs
```
