# Observabilidade — contratos entre as partes

Fonte única dos formatos usados pelo inventário, pela instrumentação (Go), pelo
runner (Node) e pela página `/manage/observabilidade`. Mudou um contrato? Atualize
aqui primeiro.

## Decisões

- **Armazenamento:** schema `obs` no próprio Postgres do projeto (migration
  `back/internal/platform/database/migrations/0329_observability.sql`; reversão em
  `observability/migrations/0329_observability.down.sql`, fora do glob do migrator).
  Motivo: o pool pgx e os GRANTs da role `omni_app` já existem (o `SyncAppRoleGrants`
  cobre schemas novos), o painel lê com SQL e o runner grava direto via `pg`. Nenhum
  serviço novo para operar.
- **Backend:** pacote `back/internal/platform/observability` (instrumentação) +
  módulo `back/internal/modules/observability` (rotas de leitura do painel e coletor).
- **Runner/inventário:** Node 24 (já existe no host), pacote próprio em
  `observability/` com dependências `pg`, `playwright-core` (usa o Google Chrome
  instalado via `channel: 'chrome'`, sem baixar navegador) e `autocannon` (opcional,
  só `--load`).
- **Front:** plugin client-only `web/app/plugins/observability.client.ts` com
  medição de Web Vitals por `PerformanceObserver` (sem dependência nova no web).

## Correlation ID

- Header `X-Request-ID`. O front gera `<pageViewId>-<seq>` (pageViewId = 16 hex por
  navegação). A API aceita o valor recebido se casar `^[A-Za-z0-9._-]{8,80}$`; senão
  gera um novo. O ID volta no header de resposta e em `error.requestId`.
- O ID aparece em: `obs.http_requests.correlation_id`, consultas lentas/N+1,
  chamadas externas, eventos do front (`api_call`) e nos logs (`request_id`).

## Tabelas `obs.*`

| tabela | conteúdo | retenção |
|---|---|---|
| `http_requests` | 1 linha por requisição amostrada: correlation_id, method, route (padrão do mux, ex. `GET /v1/x/{id}`), path_module, status, duration_ms, response_bytes, user_id, account_id, query_count, query_ms, external_count, error_code, error_message (sanitizada), error_stack (só panic), source (`live`/`runner`) | bruta, `OBS_RETENTION_DAYS` |
| `http_route_hourly` | agregado por hora × rota: calls, errors_4xx, errors_5xx, total_ms, max_ms, p50/p95/p99, avg_queries | permanente |
| `db_query_stats` | agregado por hora × fingerprint × rota: calls, total_ms, max_ms, errors, statement normalizado | permanente |
| `db_slow_queries` | consulta acima de `OBS_SLOW_QUERY_MS`: correlation_id, route, fingerprint, statement normalizado, duration_ms, error | bruta |
| `db_n_plus_one` | fingerprint repetido ≥ `OBS_NPLUSONE_THRESHOLD` vezes numa requisição | bruta |
| `external_calls` | integration (host), method, path (sem query), status, duration_ms, timeout, error | bruta |
| `frontend_events` | page_view_id, page, kind (`vital`,`pageload`,`js_error`,`api_call`,`api_failure`), name, value, status, correlation_id, detail jsonb (sanitizado), user_id | bruta |
| `inventory_items` | (kind, key) → module, data jsonb. Preenchida por `npm run obs:inventory -- --sync` | substituída |
| `runs` | uma execução do runner: id (`YYYYMMDD-HHMMSS`), started_at, finished_at, git_sha, branch, env, base_url, filters, summary, coverage, scores, regressions, exit_code | permanente |
| `run_results` | run_id, kind (`api`,`page`,`db`,`load`,`health`,`authz`), module, target, status (`pass`,`fail`,`warn`,`skip`), reason, metrics jsonb | permanente |
| `run_requests` | fila do botão "rodar": module, only, requested_by, status (`queued`,`running`,`done`,`failed`), run_id, log | permanente |
| `workers` | heartbeat do `npm run obs:worker` (id, host, last_seen) | — |

## Variáveis de ambiente (API)

| var | padrão | efeito |
|---|---|---|
| `OBSERVABILITY_ENABLED` | `true` | `false` desliga middleware, tracer, coletor e jobs (zero gravação) |
| `OBS_SAMPLE_RATE` | `1` | fração de requisições gravadas (erros 5xx e lentas sempre gravadas) |
| `OBS_SLOW_QUERY_MS` | `200` | limite de consulta lenta |
| `OBS_SLOW_REQUEST_MS` | `1000` | requisição lenta (sempre gravada) |
| `OBS_NPLUSONE_THRESHOLD` | `10` | repetições do mesmo fingerprint numa requisição |
| `OBS_RETENTION_DAYS` | `30` | apaga dado bruto mais velho; mantém agregados |
| `OBS_BUFFER_SIZE` | `5000` | fila em memória; cheia = descarta e conta (`obs_dropped`) |
| `LOG_FORMAT` | `json` | `text` volta ao formato antigo |

## API do painel (somente `platform_admin`; envelope `{ "data": ... }`)

- `GET /v1/admin/observability/overview?days=7` → `{ live: {requests, errorRate, p95Ms, droppedEvents, enabled}, modules: [{module, score, components, coverage}], trend: [{runId, startedAt, scores, coverage}] }`
- `GET /v1/admin/observability/endpoints?days=7&module=&sort=p95|errors|calls|queries` → `[{route, method, module, calls, errorRate, p50Ms, p95Ms, p99Ms, avgQueries, maxQueries}]`
- `GET /v1/admin/observability/db?days=7` → `{ slow: [...], nPlusOne: [{route, fingerprint, statement, occurrences, maxRepetitions, lastCorrelationId}], top: [{fingerprint, statement, calls, totalMs, maxMs, avgMs}], checks: <db do último run> }`
- `GET /v1/admin/observability/pages?days=7` → `{ live: [{page, samples, lcpP75, inpP75, clsP75, ttfbP75, loadP75, jsErrors, apiFailures}], lastRun: [run_results kind=page] }`
- `GET /v1/admin/observability/errors?days=7` → `[{source:'api'|'front', signature, route|page, count, lastSeen, correlationIds: [...]}]`
- `GET /v1/admin/observability/trace/{correlationId}` → `{ request, slowQueries, nPlusOne, externalCalls, frontendEvents }`
- `GET /v1/admin/observability/runs?limit=30` → `[{id, startedAt, finishedAt, env, filters, summary, coverage: {endpoints, pages, tables} (%), scores: {modulo: nota}, exitCode}]` (resumido; a lista de não testados só vem em `runs/{id}`)
- `GET /v1/admin/observability/runs/{id}` → `{ run, results: [...] }`
- `GET /v1/admin/observability/runs/compare?a=<id>&b=<id>` → `{ a, b, changes: [{kind, target, metric, before, after, deltaPct, verdict:'regression'|'improvement'|'same'}] }`
- `GET /v1/admin/observability/run-requests` → `{ canRun, workerOnline, items: [...] }`
- `POST /v1/admin/observability/run-requests` `{module?, only?}` → 201 / 403 `runner_disabled_in_production`
- `POST /v1/obs/collect` (exige login; 64 KB; até 100 eventos) `{ pageViewId, page, events: [{kind, name, value?, status?, correlationId?, detail?, at?}] }` → 202

## inventory.json (gerado por `observability/scripts/build-inventory.mjs`)

```jsonc
{
  "schemaVersion": 1, "generatedAt": "...", "git": { "sha": "...", "branch": "..." },
  "stack": { "backend": {...}, "frontend": {...}, "database": {...}, "infra": {...}, "deploy": {...} },
  "modules": [{ "id": "finance", "backendDirs": [...], "endpoints": 12, "pages": 3, "tables": 20 }],
  "endpoints": [{
    "id": "GET /v1/finance/x/{id}", "method": "GET", "path": "/v1/finance/x/{id}",
    "module": "finance", "file": "back/internal/modules/finance/http.go", "line": 42,
    "auth": "public|auth|auth_account|role|unknown", "roles": [], "permissions": [],
    "access": "read|write", "pathParams": ["id"], "queryParams": ["q"],
    "deps": { "db": true, "external": ["meta"] }, "notes": []
  }],
  "pages": [{ "path": "/manage/users", "file": "web/app/pages/manage/users.vue", "module": "core",
              "requiresLogin": true, "layout": "dashboard", "workspaceId": "...", "dynamic": false,
              "apis": ["GET /v1/..."] }],
  "tables": [{ "schema": "finance", "name": "x", "module": "finance", "kind": "table|view",
               "columns": [{"name","type","nullable","default"}], "primaryKey": [], "indexes": [...],
               "foreignKeys": [...], "constraints": [...], "rowEstimate": 0, "sizeBytes": 0 }],
  "jobs": [{ "name", "kind": "ticker|worker|cron|webhook|n8n", "module", "file", "line", "interval" }],
  "integrations": [{ "id", "kind", "envVars": [], "files": [] }],
  "queries": { "<module>": { "sqlCallSites": 0, "files": 0 } },
  "gaps": [{ "kind", "item", "reason" }]
}
```

## Saída do runner (`observability/runs/<runId>/`)

- `results.json`: `{ runId, startedAt, finishedAt, git, env, baseUrl, filters, results: [run_results...], db: {...}, health: {...}, coverage: { endpoints|pages|tables: { total, tested, pct, untested: [{ key, reason }] } }, scores: { <module>: { score, components } }, regressions: [...], improvements: [...], exitCode }`
- `report.html`: estático, sem dependências externas.
- `screenshots/*.jpg` (qualidade 50, viewport 1280×800, para poupar disco).
