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

## Saúde do sistema (fase 6) — contratos

Migration `0330_observability_health.sql` (tabelas `obs.system_samples`,
`obs.container_samples`, `obs.presence`, `obs.alerts`, `obs.security_checks`,
`obs.findings`). Retenção: as três primeiras entram na limpeza de `OBS_RETENTION_DAYS`
(presença: apagar `last_seen` com mais de 1 dia).

### Coletor de sistema na API (`platform/observability/system*.go`)

A cada `OBS_SYSTEM_INTERVAL` (padrão 15s) grava uma linha `source='api'` em
`obs.system_samples.metrics`:

```jsonc
{
  "go":    { "goroutines": 0, "heapAllocBytes": 0, "heapSysBytes": 0, "numGC": 0, "gcPauseP99Ms": 0, "gomaxprocs": 0 },
  "pool":  { "max": 0, "total": 0, "acquired": 0, "idle": 0, "acquireCountDelta": 0, "emptyAcquireDelta": 0,
             "canceledAcquireDelta": 0, "acquireWaitMsDelta": 0, "utilization": 0.0 },
  "pg":    { "connections": 0, "maxConnections": 0, "active": 0, "idleInTransaction": 0,
             "idleInTransactionOver60s": 0, "waitingOnLock": 0, "longestQuerySeconds": 0,
             "longestQuery": "<normalizado>", "deadlocksDelta": 0, "cacheHitRatio": 0.0, "dbSizeBytes": 0 },
  "jobs":  [ { "table": "schema.tabela", "pending": 0, "due": 0, "oldestDueLagSeconds": 0, "processing": 0,
               "stuckProcessing": 0, "dead": 0, "deadLastHour": 0 } ]
}
```

`jobs`: descobre via `information_schema` as tabelas com colunas `status` e `run_after`
(filas do `platform/jobs`, status `pending|processing|done|dead`). Consultas do coletor
usam `WithoutTracing` e timeout curto; falha numa parte não derruba as outras.

### Agente de host (`back/cmd/obs-agent`, container `obs-agent`)

A cada `OBS_AGENT_INTERVAL` (padrão 30s): lê o Docker pelo socket (somente leitura) e
grava uma linha por container em `obs.container_samples` (inclui `restart_policy` e `exit_code` do inspect), mais uma linha
`source='agent'` em `obs.system_samples` com
`{ "disk": [{ "path": "/", "totalBytes": 0, "freeBytes": 0, "usedPct": 0 }], "docker": { "containers": 0, "running": 0 } }`.

### Presença

O front envia `kind: "heartbeat"` (campo `name` = rota atual) pelo `POST /v1/obs/collect`
a cada 30s com a aba visível e a cada troca de rota. A API faz upsert em `obs.presence`
(`page_view_id`, `user_id` do principal). Online = `last_seen` nos últimos 90s.

### Alertas (`platform/observability/alerts*.go`)

Avaliados a cada 60s. Canal atual: **e-mail** (SMTP já configurado no projeto,
destinatários em `OBS_ALERT_EMAILS`, separados por vírgula). WhatsApp: **planejado**,
não implementado. Anti-spam: notifica ao abrir, ao subir de severidade e no máximo
1 vez a cada `OBS_ALERT_RENOTIFY_MINUTES` (padrão 60) enquanto aberto; avisa ao resolver.
Regras (alvo entre parênteses compõe o fingerprint). As regras e contagens de 5xx/latência lidas de `obs.http_requests` consideram só `source = 'live'` (tráfego do runner fica nas execuções). Alertas abertos cuja condição deixa de valer são resolvidos na rodada seguinte. A área `containers` do /health conta só containers esperados no ar (ex.: "14 de 14 containers esperados rodando, 5 parados de propósito").

| regra | condição (janela) | severidade |
|---|---|---|
| `api_5xx_rate` | 5xx > 5% e >= 20 req em 5 min | high (critical se > 20%) |
| `route_errors` (rota) | rota com >= 5 respostas 5xx em 5 min | medium |
| `slow_route` (rota) | p95 > 2000 ms com >= 10 req em 15 min | medium |
| `pool_saturation` | utilização >= 0.9 em 3 amostras seguidas, ou canceledAcquireDelta > 0 em 2 seguidas, ou espera média (acquireWaitMsDelta / acquireCountDelta) > 50 ms em 3 seguidas (emptyAcquireDelta não conta: é só criação de conexão nova) | high |
| `pg_idle_in_tx` | idleInTransactionOver60s > 0 | medium |
| `pg_lock_wait` | waitingOnLock > 0 por 2 amostras | medium |
| `pg_deadlock` | deadlocksDelta > 0 | high |
| `pg_connections` | connections / maxConnections >= 0.85 | high |
| `goroutine_growth` | goroutines > 10000, ou crescimento > 50% **e** >= +300 em 30 min sem queda | medium |
| `job_backlog` (tabela) | oldestDueLagSeconds > 300 | medium (high > 1800) |
| `job_dead` (tabela) | deadLastHour > 0 | medium |
| `collector_silent` | sem amostra `api` há 2 min | high |
| `agent_silent` | sem amostra `agent` há 3 min (só se já houve alguma) | medium |
| `container_down` (container) | (restart_policy em always\|unless-stopped\|on-failure E state != running) ou health = unhealthy. Policy `no`/vazia parada é intencional e não alerta; containers sem amostra há > 10 min (removidos) são ignorados | critical |
| `container_restarts` (container) | restart_count subiu em 1 h | high |
| `disk_low` (path) | usedPct >= 85 (critical >= 95) | high |
| `frontend_errors` | >= 20 erros de JS em 15 min | medium |
| `contract_errors` (rota) | >= 2 respostas 400/422 `invalid_body`/`invalid_json` em 15 min (tráfego `live`): front e back fora de contrato | high |
| `route_4xx_spike` (rota) | >= 50% de 400/422 em 15 min, com >= 10 chamadas (rota já em `contract_errors` não duplica) | medium |
| `deploy_regression` (rota) | último deploy `success` terminado há <= 2 h: erros (5xx + 400/422) nos últimos 15 min após o fim >= 3 e > 2x a média de 15 min dos 30 min antes do início; resolve quando os erros param | high |

### Endpoints novos (todos `RequireRoles(platform_admin)`, envelope `{ data }`)

- `GET /v1/admin/observability/health` →
```jsonc
{
  "generatedAt": "iso",
  "overall": "ok|warn|critical",
  "areas": [ { "key": "api|database|containers|pages|workers|security", "label": "API", "status": "ok|warn|critical|unknown", "summary": "texto curto" } ],
  "issues": [ { "severity": "critical|high|medium|low", "area": "api", "title": "texto", "detail": "texto", "since": "iso", "link": { "tab": "endpoints|db|pages|errors|security|system", "query": {} } } ],
  "errors": { "lastHour": { "api5xx": 0, "frontend": 0 }, "last24h": { "api5xx": 0, "frontend": 0 }, "top": [ { "source": "api|front", "signature": "", "count": 0 } ] },
  "online": { "count": 0, "users": [ { "userId": "", "name": "", "page": "", "lastSeen": "iso", "tabs": 1 } ] },
  "system": { "api": { "at": "iso", "goroutines": 0, "heapMB": 0, "poolUtilization": 0, "pgConnections": 0, "pgMaxConnections": 0 } | null,
              "agent": { "at": "iso", "disk": [], "containers": 0, "running": 0 } | null },
  "alerts": { "channel": "email", "configured": true, "recipients": 0, "whatsapp": "planned", "open": [ { "id": 0, "severity": "", "title": "", "since": "iso", "notifiedAt": "iso|null" } ] },
  "security": { "checks": { "pass": 0, "fail": 0, "partial": 0, "not_tested": 0 }, "findings": { "open": { "critical": 0, "high": 0, "medium": 0, "low": 0 }, "fixed": 0 } }
}
```
  `issues` = alertas abertos + achados críticos/altos abertos + checagens de segurança
  `fail`, em ordem de severidade (máximo 15).
- `GET /v1/admin/observability/security` → `{ checks: [ {key, category, title, standard, status, detail, checkedAt, source} ], findings: [ {id, severity, title, area, location, status, updatedAt} ] }`
- `GET /v1/admin/observability/system?hours=6` → `{ api: [ {at, metrics} ], agent: [ {at, metrics} ], containers: [ {container, service, state, health, restartCount, cpuPct, memBytes, memLimitBytes, at} ] }` (containers = última amostra de cada).
- `GET /v1/admin/observability/alerts?status=open|resolved|all` → `[ {id, rule, severity, area, title, detail, status, firstSeen, lastSeen, resolvedAt, notifiedAt, notifyCount, lastNotifyError} ]`
- `POST /v1/admin/observability/alerts/test` → envia um e-mail de teste aos destinatários (204; 409 `alerts_not_configured` sem SMTP/destinatários).

### Segurança

`observability/config/security-catalog.json` lista os ataques/controles (OWASP Top 10,
OWASP API Top 10 e outros), cada um com `key, category, title, standard, howTested`.
`npm run obs:security -- --sync` grava o catálogo em `obs.security_checks` (sem
sobrescrever resultado já testado) e importa os achados da AUDITORIA em `obs.findings`
(sem sobrescrever `status` editado). A fase `security` do runner (`--only=security`)
executa as checagens automáticas e atualiza `status/detail/evidence/checked_at`.
Nunca roda ataque destrutivo; contra produção só checagens passivas (headers, TLS).

### Deploys (fase 6)

Tabela `obs.deploys` (migration 0330). Fonte: `.tmp/deploy/<id>/summary.json` gerado por
`scripts/deploy/deploy-report.mjs` (duração de ponta a ponta pelo relógio monotônico da
máquina que disparou: do início do launcher até os smoke tests na VPS; `stages[]` com
`name, depth, status, durationMs`).

- Gravação automática: ao terminar `run-deploy.mjs` (sucesso ou falha), o resumo é
  inserido no banco do ambiente alvo (prod/staging pela mesma conexão SSH do deploy;
  upsert por `id`). Falha ao gravar só gera aviso, nunca muda o exit code do deploy.
- Importação: `npm run obs:deploys -- --sync [--target=local|prod|staging]` importa
  todos os `summary.json` ainda não gravados (idempotente).
- `GET /v1/admin/observability/deploys?limit=30` (platform_admin) →
```jsonc
{ "items": [ { "id": "", "command": "fast-prod", "environment": "prod", "startedAt": "iso", "finishedAt": "iso",
               "durationMs": 0, "exitCode": 0, "status": "success|failed|interrupted|unknown",
               "gitSha": "", "gitBranch": "", "imageTag": "", "verdict": "ok|slow|failed",
               "stages": [ { "name": "", "depth": 0, "status": "", "durationMs": 0 } ] } ],
  "stats": { "count": 0, "successRate": 0.0, "medianMs": 0, "p90Ms": 0, "slowThresholdMs": 0, "lastSuccessAt": "iso|null" } }
```
  `medianMs`/`p90Ms` sobre os últimos 10 com sucesso do mesmo `command`;
  `slow` = duração > 1,5 × mediana (mínimo 3 deploys de referência).
- `/health`: área `deploy` (último deploy: ok/lento/falhou e há quanto tempo) e issue
  quando o último deploy falhou ou foi lento.

## Etapa 1 do backlog (tasks 1.3–1.7) — contratos

Migration `0332_obs_alert_controls.sql` (aditiva): `obs.alerts` ganha
`acknowledged_at timestamptz`, `acknowledged_by text`, `silenced_until timestamptz`,
`silenced_by text`. Reversão em `observability/migrations/0332_obs_alert_controls.down.sql`.

**1.3 Reconhecer/silenciar** (platform_admin):
- `POST /v1/admin/observability/alerts/{id}/ack` → 204 (grava quem/quando; alerta
  reconhecido não é reavisado; o e-mail de resolução continua saindo).
- `POST /v1/admin/observability/alerts/{id}/silence` `{ "hours": 1..168 }` → 204
  (nenhum e-mail do alerta até `silenced_until`).
- `POST /v1/admin/observability/alerts/{id}/unsilence` → 204.
- Respostas de `/alerts` e `/health.alerts.open` incluem `acknowledgedAt`,
  `acknowledgedBy` (nome), `silencedUntil`.
- Alerta que resolve e reabre (novo registro) volta a avisar normalmente.

**1.5 TLS** (obs-agent): `OBS_AGENT_TLS_HOSTS` (lista `host[:porta]`, vírgula; vazio =
não checa). A amostra `source='agent'` ganha
`"tls": [{ "host": "", "expiresAt": "iso", "daysLeft": 0, "error": "" }]` (checagem a
cada 6 h, cache entre ciclos). Regra `tls_expiring` (alvo host): `daysLeft <= 14`
high, `<= 3` critical; `error` não vazio por 2 checagens → medium.

**1.7 Panic**: a amostra `source='api'` ganha `"panics": { "total": 0, "delta": 0 }`,
somando panics recuperados no middleware HTTP (Recover), em `platform/background`
e em `platform/jobs`. Regra `panic_detected`: `delta > 0` → high (detail com total).

**1.6 Integrações**: `GET /v1/admin/observability/integrations?days=7` →
`[{ "host": "", "calls": 0, "errors": 0, "errorRate": 0.0, "avgMs": 0, "p95Ms": 0, "lastErrorAt": "iso|null", "lastError": "" }]`
de `obs.external_calls` (ordenado por calls desc). Aba "Integrações" no painel.

**1.4 Uptime externo**: `.github/workflows/uptime.yml`, cron a cada 5 min, checa
`https://omni.crowvisuals.com.br/healthz` e a home; em falha abre (ou comenta) uma issue
com label `uptime`; na volta, fecha a issue. Sem segredos (usa `GITHUB_TOKEN`).

## Etapa 2 do backlog (governança, tasks 2.1–2.4) — contratos

Migration `0333_obs_governance.sql`: tabela `obs.settings (key, value jsonb, updated_by,
updated_at)` e colunas `owner`, `due_date`, `note`, `updated_by` em `obs.findings`.

**2.1 Configuração dos alertas** — chave `alerts` em `obs.settings`:
```jsonc
{ "recipients": ["a@b.com"],            // vazio = OBS_ALERT_EMAILS
  "renotifyMinutes": 60,                // 15..1440
  "quietHours": { "enabled": false, "start": "22:00", "end": "07:00", "timezone": "America/Sao_Paulo", "allowCritical": true },
  "rules": { "<rule>": { "enabled": true, "severity": "auto|critical|high|medium|low", "threshold": 0 } } }
```
`threshold` só para regras com limite numérico (ex.: `api_5xx_rate` %, `slow_route` ms,
`disk_low` %, `job_backlog` s, `tls_expiring` dias); o padrão é o do código. Horário de
silêncio segura e-mails (exceto critical se `allowCritical`) e manda o acumulado ao fim.
- `GET /v1/admin/observability/settings` → `{ alerts, email, weeklyReport, rulesCatalog: [{rule, label, area, hasThreshold, defaultThreshold, unit}] }` (valores efetivos = banco ou padrão).
- `PUT /v1/admin/observability/settings/{key}` (`alerts|email|weeklyReport`) → valida e grava; 400 com mensagem por campo.
O motor relê as configurações a cada rodada (cache de 60 s).

**2.2 Layout do e-mail** — chave `email`:
```jsonc
{ "subjectPrefix": "[Omni]", "group": true, "sections": ["summary","details","link"], "includeResolved": true, "footer": "" }
```
`POST /v1/admin/observability/settings/email/preview` (corpo = valor proposto) →
`{ "subject": "", "text": "" }` com alertas de exemplo. Texto simples (sem HTML).

**2.3 Relatório semanal** — chave `weeklyReport`:
```jsonc
{ "enabled": false, "weekday": 1, "hour": 8, "timezone": "America/Sao_Paulo", "recipients": [] }   // vazio = mesmos dos alertas
```
O motor envia quando chega o dia/hora e ainda não enviou na semana (marca em
`obs.settings` chave `weeklyReport.state` `{lastSentAt}`). Conteúdo: p95 e taxa de 5xx da
semana × semana anterior, top 5 rotas que pioraram, alertas abertos/abertos na semana,
achados abertos por severidade e vencidos, deploys (quantidade, mediana, falhas),
notas do último run completo. Endpoints: `GET /reports/weekly/preview` → `{subject,text}`,
`POST /reports/weekly/send` → 204 (envio imediato).

**Detalhes implementados (2.1 a 2.3)**
- `GET /settings` devolve também `channel` (`smtpConfigured`, `envRecipientCount`,
  `alertRecipientCount`, `weeklyRecipientCount`, `cacheSeconds`) e `weeklyState`
  (`lastSentAt`, `lastError`). `recipients` volta como gravado (vazio = ambiente); `alerts.rules`
  volta completo (22 regras, padrão do código quando não gravado); `rulesCatalog` traz também
  `min`/`max` do limite. `PUT /settings/{key}` grava e devolve o mesmo corpo do GET.
- Erros: 400 `invalid_settings` com `error.details.fields` = `{ "<caminho>": "mensagem" }`
  (`recipients`, `renotifyMinutes`, `quietHours.start`, `rules.<regra>.threshold`...; `_` = JSON
  inválido ou campo desconhecido); 404 chave desconhecida; 503 `settings_unavailable`.
- Limites aplicados pelo motor: `api_5xx_rate` (crítico em 4x o limite), `route_errors`,
  `slow_route`, `pg_connections`, `goroutine_growth` (limite absoluto), `job_backlog` (alto em 6x),
  `disk_low` (crítico em limite+10, máx. 99), `tls_expiring` (crítico em min(3, limite) dias),
  `frontend_errors`, `contract_errors`, `route_4xx_spike` (%), `deploy_regression`. Regra desligada some das condições (os alertas abertos dela resolvem).
  Severidade diferente de `auto` substitui a do código.
- Horário de silêncio: e-mails não críticos (ou todos, sem `allowCritical`) ficam retidos sem
  marcar o alerta como avisado; o motor os reavalia a cada rodada e eles saem juntos no fim.
  Resoluções ocorridas durante o silêncio ficam em memória e saem no fim (um reinício da API as perde).
- `email`: `sections` = `summary` (frase de abertura), `details` (onde/desde/medido por alerta; sem
  ele só o título), `link` (painel). `group=false` envia um e-mail por alerta (assunto
  `<prefixo> <SEVERIDADE> - <título>`). `includeResolved=false` não avisa resoluções.
- `weeklyReport.weekday` segue `time.Weekday` (0 = domingo ... 6 = sábado). O envio ocorre entre o
  horário marcado e 24 h depois (recuperação se a API estava fora), uma vez por semana; falha
  grava `lastAttemptAt`/`lastError` e tenta de novo após 15 min. `POST /reports/weekly/send` conta
  como o envio da semana; 409 `alerts_not_configured` sem SMTP/destinatários; 502 se o SMTP falhar.
- Relatório: p95 do histograma de `obs.http_route_hourly` (inclui tráfego do runner) e taxa de 5xx de
  `obs.http_requests` `source='live'`, semana × semana anterior; rotas que pioraram/melhoraram
  (mín. 50 chamadas nas duas semanas, variação >= 20% e >= 50 ms); alertas, achados abertos e
  vencidos (`due_date` < hoje no fuso do relatório), deploys e notas do último run completo.

**2.4 Achados como fila** — `PATCH /v1/admin/observability/findings/{id}`
`{ "status": "open|fixed|accepted", "owner": "", "dueDate": "YYYY-MM-DD|null", "note": "" }`
(campos opcionais; grava `updated_by` = usuário). `/security` devolve `owner`, `dueDate`,
`note`, `updatedAt`, `updatedBy`, `overdue` (dueDate < hoje e status open). O sync da
AUDITORIA nunca sobrescreve esses campos. Aba Segurança: edição inline e filtro
"abertos / vencidos / meus".

## Etapa 3 do backlog (banco, tasks 3.1, 3.2, 3.3, 3.5) — contratos

Migration `0334_obs_db_insights.sql`: `obs.table_stats` (uma linha por tabela por hora)
e `obs.query_plans` (plano genérico por fingerprint lento).

**3.1 Crescimento e manutenção.** Coletor na API (pacote `platform/observability`,
de hora em hora, `OBS_TABLE_STATS_INTERVAL` padrão 1h, primeira coleta 2 min após o boot)
grava em `obs.table_stats` a partir de `pg_stat_user_tables` + `pg_total_relation_size`
/`pg_relation_size`/`pg_indexes_size` (exclui schema `obs` e `pg_toast`). Retenção:
90 dias (independente de `OBS_RETENTION_DAYS`).
`GET /v1/admin/observability/db/tables?sort=size|growth|dead|seqscan` →
`[{ schema, table, totalBytes, tableBytes, indexBytes, liveTuples, deadTuples, deadRatio, growthBytes7d, growthPct7d, seqScanDelta24h, idxScanDelta24h, lastVacuumAt, lastAnalyzeAt, needsVacuum, needsAnalyze }]`
(máx 100). `needsVacuum` = deadTuples > 50 000 e deadRatio > 0,2; `needsAnalyze` =
modsSinceAnalyze > 10% das linhas e > 10 000. Regra de alerta `table_needs_vacuum`
(alvo schema.tabela, medium) e `db_growth` (banco cresceu > 20% em 7 dias, medium).

**3.2 Índices de FK que pesam.** `GET /v1/admin/observability/db/index-advice` →
`[{ schema, table, constraint, columns: [], tableBytes, seqScanDelta24h, matchingQueries, slowQueries, score, suggestion }]`
— FKs sem índice cobrindo as colunas (pg_constraint × pg_index), cruzadas com
`obs.db_query_stats`/`obs.db_slow_queries` (statement cita a tabela e a coluna) e
com `seq_scan` de `obs.table_stats`. `suggestion` = DDL sugerido
(`create index concurrently if not exists ... on schema.tabela (cols);`), só texto,
nunca executado. Ordenado por score; só itens com score > 0 por padrão (`all=1` lista tudo).

**3.3 EXPLAIN automático.** A cada hora, para os fingerprints lentos (top 20 por tempo
total em `obs.db_query_stats` nos últimos 7 dias, mais todos de `db_slow_queries`) sem
plano nas últimas 24 h: converte `?` do statement normalizado em `$n` e roda
`EXPLAIN (GENERIC_PLAN, FORMAT JSON)` (Postgres ≥ 16; não executa a consulta) com
`WithoutTracing`, timeout 5 s, `SET LOCAL statement_timeout`, numa transação
`READ ONLY` com rollback; só `select`/`with ... select` (nunca escrita). Achados do plano
em `findings`: `seq_scan_large` (Seq Scan em tabela > 10 000 linhas com Filter → sugere
índice nas colunas do filtro), `sort_without_index`, `nested_loop_large`,
`hash_join_large`, `function_on_column` (filtro com função na coluna impede índice).
Erro de plano vai em `error` (sem derrubar o ciclo).
`GET /v1/admin/observability/db/plans` → `[{ fingerprint, statement, totalCost, findings: [{kind, relation, detail, suggestion}], error, capturedAt, avgMs, calls }]`;
`POST /v1/admin/observability/db/plans/{fingerprint}/refresh` → 202 (recaptura agora).

**3.5 Ganho medido.** `GET /v1/admin/observability/db/optimizations?days=7` →
`{ deployAt, items: [{ fingerprint, statement, route, beforeAvgMs, afterAvgMs, beforeCalls, afterCalls, deltaPct, verdict: "melhorou|piorou|igual" }] }`
comparando `obs.db_query_stats` na janela antes × depois do último deploy com sucesso
(`obs.deploys`), mínimo 20 chamadas em cada lado e variação ≥ 20% e ≥ 5 ms para não ser
"igual". Sem deploy registrado: compara a última semana com a anterior.

Front (aba Banco): seções "Tabelas" (3.1), "Índices sugeridos" (3.2), "Planos das
consultas lentas" (3.3) e "Antes × depois do último deploy" (3.5).
