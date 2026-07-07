# AC-17 — Load test (k6 via Docker) + EXPLAIN ANALYZE dos endpoints quentes

> Spec de implementação · Prioridade **P2** · Esforço **M** · Impacto **alto**
> Origem: limitação declarada do diagnóstico 2026-07 ("sem load test, sem EXPLAIN") · roadmap
> `ac-fixes-2026-07` → task `ac-17-load-test-explain`

## 1. Contexto

**O achado:** toda a análise de performance do diagnóstico foi ESTÁTICA (código+configs) e o
`qa-bot/perf_audit.py` mede navegação de UI (Playwright), não carga de API. Nota de performance ≥9
exige prova sob carga + planos de query reais. Não existe k6/artillery/locust no repo.

Alvos (endpoints de lista/agregação mais pesados):
- `GET /v1/reports/results` — histórico paginado (teto 5000/página 200, AC-06;
  `back/internal/modules/queue/reports/service.go:18-24`), lê a view `operation_service_history`
  (`store_postgres.go:94`).
- `GET /v1/reports/overview` e `GET /v1/analytics/ranking|data|intelligence` — agregações sem
  paginação (`queue/analytics/http.go`; params `storeId,tenantId,dateFrom,dateTo`).

**Guarda-corpo central: NUNCA apontar carga para a VPS de produção.** Alvo = build de prod LOCAL
(`npm run prod:up`, compose prod na sua máquina).

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Harness k6 versionado em `qa-bot/load/` (login + 3 cenários), rodando 100% via
   `docker run grafana/k6` (zero install no host).
2. EXPLAIN (ANALYZE, BUFFERS) das queries quentes com os MESMOS filtros do load + procedimento
   `auto_explain` temporário e reversível.
3. Relatório `docs/relatorios/2026-07/AC-17-load-test.md` (p50/p95/p99, RPS, findings, ações).

**Não-objetivos (FORA):** rodar contra VPS/domínio público (PROIBIDO — abortar no script);
otimizar queries (as AÇÕES viram backlog; esta spec só MEDE); seeding sintético de dados (rodar com
o dataset local e DECLARAR os row counts como limitação); CI (roda manual).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. NUNCA inventar credenciais: usuário/senha de teste vêm por env
  (`K6_USER`/`K6_PASSWORD`) fornecidos pelo dono na hora de rodar — a spec NÃO contém senha.
- Confirmar o nome da rede do compose prod local antes de rodar (`docker network ls | grep omni`).
- `qa-bot/load/artifacts/` no `.gitignore` (brutos ficam fora do repo).
- `auto_explain`: SEMPRE reverter ao final (passo 4.5) — deixar ligado degrada prod local e polui log.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `qa-bot/load/lib/auth.js`

```javascript
import http from 'k6/http';
import { fail } from 'k6';

// GUARDA-CORPO: só alvos locais. Jamais a VPS/domínio público.
const ALLOWED = [/^http:\/\/api:8080$/, /^http:\/\/host\.docker\.internal:\d+$/, /^http:\/\/127\.0\.0\.1:\d+$/];
export function assertLocalBase(baseUrl) {
  if (!ALLOWED.some((re) => re.test(baseUrl))) {
    fail(`BASE_URL proibida para load test: ${baseUrl} (só api:8080 / host.docker.internal / 127.0.0.1)`);
  }
}

// Login 1x no setup(); o token é distribuído aos VUs via data.
export function login(baseUrl) {
  assertLocalBase(baseUrl);
  const user = __ENV.K6_USER;
  const password = __ENV.K6_PASSWORD;
  if (!user || !password) fail('K6_USER/K6_PASSWORD não definidos (o dono fornece; nunca hardcodar).');
  const res = http.post(`${baseUrl}/v1/auth/login`, JSON.stringify({ username: user, password }), {
    headers: { 'Content-Type': 'application/json' },
  });
  if (res.status !== 200) fail(`login falhou: ${res.status} ${res.body}`);
  const body = res.json();
  const token = body.accessToken || body.token;
  if (!token) fail('login sem accessToken no corpo — conferir o shape real de /v1/auth/login');
  return { token, accountId: __ENV.K6_ACCOUNT_ID || '' };
}

export function authHeaders(session) {
  const h = { Authorization: `Bearer ${session.token}` };
  if (session.accountId) h['X-Account-Id'] = session.accountId; // rotas multi-tenant exigem (memória do projeto)
  return h;
}
```

**Nota:** conferir o SHAPE real do login (campo do token e se o username é e-mail) lendo
`back/internal/modules/auth/http*.go` antes de rodar — ajustar `login()` se divergir.

### 4.2 CRIAR os 3 cenários em `qa-bot/load/scenarios/`

`smoke.js` (valida o harness): 1 VU, 30s. `baseline.js`: 10 VUs, 3min. `stress.js`: ramp
0→50 VUs em 5min. Os três compartilham o corpo (importar de `../lib/requests.js` se preferir):

```javascript
// baseline.js — esqueleto (smoke/stress mudam só o options)
import http from 'k6/http';
import { group, check, sleep } from 'k6';
import { login, authHeaders, assertLocalBase } from '../lib/auth.js';

const BASE = __ENV.BASE_URL || 'http://api:8080';
const RANGE = `dateFrom=${__ENV.K6_DATE_FROM || '2026-06-01'}&dateTo=${__ENV.K6_DATE_TO || '2026-07-03'}`;

export const options = {
  vus: 10,
  duration: '3m',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{group:::results}':   ['p(95)<600'],
    'http_req_duration{group:::overview}':  ['p(95)<800'],
    'http_req_duration{group:::analytics}': ['p(95)<1000'],
  },
};

export function setup() { assertLocalBase(BASE); return login(BASE); }

export default function (session) {
  const h = { headers: authHeaders(session) };
  group('results', () => {
    const r = http.get(`${BASE}/v1/reports/results?${RANGE}&pageSize=200`, h);
    check(r, { 'results 200': (x) => x.status === 200 });
  });
  group('overview', () => {
    const r = http.get(`${BASE}/v1/reports/overview?${RANGE}`, h);
    check(r, { 'overview 200': (x) => x.status === 200 });
  });
  group('analytics', () => {
    for (const ep of ['ranking', 'data', 'intelligence']) {
      const r = http.get(`${BASE}/v1/analytics/${ep}?${RANGE}`, h);
      check(r, { [`${ep} 200`]: (x) => x.status === 200 });
    }
  });
  sleep(1);
}
```

Thresholds são INICIAIS — calibrar 1x após o smoke com justificativa escrita no relatório
(não afrouxar silenciosamente).

### 4.3 CRIAR `qa-bot/load/README.md` — comandos de execução

```bash
# 1) prod build local no ar (NUNCA a VPS):
npm run prod:up
# 2) nome da rede (compose prod local; conferir):
docker network ls | grep -E "omni|listaatendimento"
# 3) rodar (a rede coloca o k6 falando com o serviço api direto):
docker run --rm -i --network <rede_do_compose_prod> \
  -v "$PWD/qa-bot/load:/scripts" \
  -e BASE_URL=http://api:8080 -e K6_USER=<user> -e K6_PASSWORD=<senha> -e K6_ACCOUNT_ID=<uuid> \
  grafana/k6 run /scripts/scenarios/smoke.js
# baseline e stress: trocar o arquivo. Saída JSON: adicionar
#   --summary-export /scripts/artifacts/baseline-summary.json
```

Incluir no README o guarda-corpo ("se BASE_URL não for local o script aborta") e
`qa-bot/load/artifacts/` no `.gitignore` da raiz.

### 4.4 CRIAR `qa-bot/load/explain/hot-queries.sql`

`EXPLAIN (ANALYZE, BUFFERS)` das queries REAIS extraídas dos stores (copiar o SQL de
`reports/store_postgres.go:94+` e `store_postgres_filters.go:70+` e das agregações de
`analytics/store_postgres.go`, substituindo os parâmetros pelos MESMOS filtros do load — array de
store_ids e range de datas usados nos cenários). Cabeçalho do arquivo documenta de qual função Go
cada query veio. Executar:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < qa-bot/load/explain/hot-queries.sql \
  > qa-bot/load/artifacts/explain-$(date +%Y%m%d).txt
```

### 4.5 Procedimento `auto_explain` (ligar → medir → REVERTER)

```sql
-- ligar (psql no postgres do compose prod local):
alter database omni set session_preload_libraries = 'auto_explain';
alter database omni set auto_explain.log_min_duration = '250ms';
alter database omni set auto_explain.log_analyze = on;
```
`docker compose restart api` (recicla o pool → novas sessões carregam) → rodar `baseline.js` →
coletar `docker compose logs postgres --since 10m > artifacts/auto-explain.log` → **reverter**:
```sql
alter database omni reset session_preload_libraries;
alter database omni reset auto_explain.log_min_duration;
alter database omni reset auto_explain.log_analyze;
```
+ `docker compose restart api`.

### 4.6 CRIAR `docs/relatorios/2026-07/AC-17-load-test.md` (template)

Seções: Setup (máquina, dataset com **row counts** das tabelas quentes — declarar como limitação),
Resultados por cenário (tabela p50/p95/p99/RPS/erro% por group), Findings dos plans (seq scan, sort
em disco, índice ausente — cada um com evidência do EXPLAIN), Ações recomendadas (backlog, com
prioridade), Veredito vs thresholds.

## 5. Critérios de aceite

1. `smoke.js` passa (0 erros) contra o prod build local com credenciais fornecidas pelo dono.
2. `baseline.js` completa e o summary JSON está em artifacts; thresholds avaliados (pass ou fail
   JUSTIFICADO no relatório).
3. `hot-queries.sql` roda limpo e os planos estão no relatório com pelo menos 3 findings analisados
   (mesmo que a conclusão seja "plano saudável").
4. `auto_explain` REVERTIDO (conferir `alter database ... reset` aplicado: `select setconfig from pg_db_role_setting` vazio para o db).
5. Relatório publicado em `docs/relatorios/2026-07/AC-17-load-test.md`; brutos fora do git.
6. NENHUMA requisição de carga saiu para domínio público (guarda-corpo testado: rodar 1x com
   `BASE_URL=https://omni.crowvisuals.com.br` e confirmar que o script ABORTA no setup).

## 6. Validação

Os próprios cenários (4.3) + item 6 do aceite. Sem build de código de produto.

## 7. Notas de Deploy

Nada vai para produção (tooling local + docs). Nenhuma migration/env de app.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `qa-bot/load/lib/auth.js` + `scenarios/{smoke,baseline,stress}.js` | criar |
| `qa-bot/load/explain/hot-queries.sql` + `qa-bot/load/README.md` | criar |
| `.gitignore` (raiz) | editar (`qa-bot/load/artifacts/`) |
| `docs/relatorios/2026-07/AC-17-load-test.md` | criar (relatório) |

**Conflitos potenciais:** nenhum com as demais specs. Sinergia: os findings alimentam o backlog de
performance (05) e podem gerar specs próprias de índice/query.
