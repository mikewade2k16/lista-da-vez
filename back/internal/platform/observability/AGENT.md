# AGENT — platform/observability

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
