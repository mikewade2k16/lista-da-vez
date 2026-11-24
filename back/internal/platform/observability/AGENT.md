# AGENT — platform/observability

> Backlog e estado da observabilidade (o que foi feito e o que falta): `observability/AGENT.md`.

Instrumentação de toda a API: requisições, consultas ao banco, chamadas externas e
eventos do front, gravados de forma assíncrona no schema `obs` (migration
`0329_observability.sql`). Contratos: `observability/SPEC.md`. Painel e coletor:
`internal/modules/observability`.

## Peças

- `config.go`: `ConfigFromEnv(env)`. `OBSERVABILITY_ENABLED=false` faz `New` devolver
  `nil`; todo método de `*Runtime` é nil-safe (sem middleware, tracer, coletor nem jobs).
  Também `OBS_SAMPLE_RATE`, `OBS_SLOW_QUERY_MS`, `OBS_SLOW_REQUEST_MS`,
  `OBS_NPLUSONE_THRESHOLD`, `OBS_RETENTION_DAYS`, `OBS_BUFFER_SIZE`.
- `middleware.go`: `Runtime.Middleware(mux)`. Fica logo após `httpapi.RequestID` no
  Chain. Resolve a rota pelo padrão do mux (`mux.Handler(r)`, baixa cardinalidade;
  sem rota = `UNMATCHED`). Ignora WebSocket, preflight e o próprio coletor.
- `context.go`: estado por requisição no contexto. `SetPrincipal` (chamado pelo
  `auth.Middleware`), `NoteError` (chamado por `httpapi.WriteError*`) e `NotePanic`
  (chamado pelo `httpapi.Recover`, com stack) enriquecem o mesmo registro.
  `WithoutTracing(ctx)` desliga o tracer, usado pelo gravador e pelo painel.
- `tracer.go`: `pgx.QueryTracer` ligado ao pool de runtime
  (`database.OpenAppPoolWithTracer`). Conta consultas por requisição, agrega por
  fingerprint, grava as lentas e as falhas. Também tem `InstrumentDefaultTransport`,
  que envolve `http.DefaultTransport`. Clientes com `Transport` próprio, como o SDK
  AWS/R2, não são medidos.
- `sqlnorm.go`: troca literais por `?`, colapsa listas `IN (...)` e `VALUES`, gera
  fingerprint FNV. Nenhum valor de parâmetro é gravado.
- `sanitize.go`: sanitização central (tokens/JWT, e-mail, CPF/CNPJ, telefone, cartão
  com Luhn, pares chave=valor sensíveis, query string). Usada pelo logger, pelos erros
  e pelo coletor do front.
- `logging.go`: `NewLogger`, JSON por padrão (`LOG_FORMAT=text` volta ao antigo).
  Acrescenta `request_id` quando o log recebe contexto. Chaves `*_id` não são
  mascaradas.
- `recorder.go` / `writer.go`: fila limitada (`OBS_BUFFER_SIZE`). Fila cheia descarta
  e conta `droppedEvents`, nunca bloqueia a requisição. A cada 5 s grava os brutos via
  COPY e os agregados (`http_route_hourly`, `db_query_stats`) via upsert, numa
  transação.
- `maintenance.go`: a cada hora apaga dado bruto mais velho que `OBS_RETENTION_DAYS`,
  em lotes de 5000. Agregados e execuções do runner ficam.

## Regras

- Amostragem só afeta a linha bruta de `http_requests`. Agregados por rota contam
  100% das requisições (histograma de latência em `buckets`). 5xx, lentas e as do
  runner (`X-Obs-Source: runner`) sempre gravam a linha bruta.
- Resposta 5xx com o contexto da requisição cancelado (cliente desconectou) é gravada
  como 499 `client_closed_request`: não conta como erro do servidor. O cliente não
  recebe esse status; ele existe só na telemetria.
- Nunca gravar corpo de requisição/resposta, header de auth ou valor de parâmetro SQL.
  Texto livre passa por `SanitizeText`.
- O schema `obs` não é tenant-scoped: guarda ids opacos de usuário/conta. A leitura é
  restrita a `platform_admin`.

## Saúde do sistema (fase 6)

- `system.go`: coletor ligado por `Runtime.Start`, parado por `Stop`. A cada
  `OBS_SYSTEM_INTERVAL` (padrão 15s; aceita `15s` ou inteiro em segundos, mínimo 5s)
  grava uma linha `source='api'` em `obs.system_samples` (formato no `SPEC.md`).
  Deltas são contra a amostra anterior em memória (primeira = 0). Contexto
  `WithoutTracing`, timeout por ciclo, `recover` no loop; parte que falha some do JSON.
- `system_runtime.go`: runtime Go (`MemStats`, p99 das pausas de GC) e `pgxpool.Stat`.
- `system_pg.go`: `pg_stat_activity` (sem a conexão do coletor), `pg_stat_database`,
  tamanho e `max_connections` (cache). `longestQuery` normalizada e sanitizada.
- `system_jobs.go`: descobre via `information_schema` as tabelas com `status` e
  `run_after`, redescobre a cada 10 min; identificadores só `[a-z0-9_]` e quotados;
  uma consulta por tabela (3s), erro numa tabela não derruba as outras.
- `presence.go`: heartbeat do front (`kind: "heartbeat"`, `name` = rota) vira upsert em
  `obs.presence` no flush em lote (último por `page_view_id`); não vai para
  `obs.frontend_events`. Sem usuário autenticado não grava.
- `maintenance.go` também limpa `system_samples`/`container_samples` pela retenção e
  `presence` com `last_seen` > 1 dia.
- Panics (task 1.7): `internal/platform/panics` e um pacote folha com o contador atomico
  `Record(source)`/`Total()`. Quem recupera panic chama `panics.Record`: `httpapi.Recover`
  ("http"), `background.safeRun` e o closer ("background"), `jobs.safeCycle` e
  `jobs.safeHandle` ("jobs"). O coletor de sistema (`system.go`) grava
  `"panics": {"total", "delta"}` na amostra `source='api'`; o delta e contra a amostra
  anterior (0 na primeira amostra ou se o processo reiniciou). A regra `panic_detected`
  fica em `modules/observability`. Pacote separado porque httpapi/jobs/background nao
  devem importar `platform/observability` (risco de ciclo).
