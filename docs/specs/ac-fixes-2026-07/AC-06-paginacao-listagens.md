# AC-06 — Paginação/teto em listagens sem limite (reports, analytics, BI)

Prioridade P1 · Esforço M · Impacto alto · Achado canônico AC-06 do diagnóstico 2026-07
(`scratchpad/fatos.json → achados_canonicos.AC-06`).

## 1. Contexto

Três caminhos de leitura carregam volumes sem teto no banco/upstream e devolvem tudo num payload:

**Caso 1 — Reports (queue/reports).** `listHistoryQuery` monta o SELECT de
`operation_service_history` com filtros dinâmicos mas **sem LIMIT**
(`back/internal/modules/queue/reports/store_postgres.go:39-150`; o ORDER BY na linha 150 encerra a
query com `;` e nada mais). Os endpoints `GET /v1/reports/overview|results|recent-services|multistore-overview`
(`reports/http.go:15,37,59,103`) chamam `Service.loadEntries` (`service_loading.go:14`), que traz o
histórico INTEIRO da loja (ou de TODAS as lojas do tenant), filtra e ordena em memória e só então
pagina em memória via `paginateRows` (`service_charts.go:136`). `Filters` já tem `Page/PageSize`
(`model.go:19-20`, clamp em `service_loading.go:124-134`, teto `maxPageSize=200` em `service.go:18`)
— mas isso pagina o ARRAY já carregado, não a query. O front pede `pageSize=200` sem `dateFrom`
default (`web/app/stores/reports.ts:9,120-128`; `domain/utils/reports.ts:360` → `dateFrom: ''`), ou
seja, a rota `/relatorios` de uma loja com 50k atendimentos puxa 50k linhas com ~40 colunas + 10
campos JSONB decodificados por linha.

**Caso 2 — Analytics (queue/analytics).** `loadServiceHistory` carrega o histórico COMPLETO da loja
(`back/internal/modules/queue/operations/store_postgres.go:565-707`, sem LIMIT nem janela de data)
dentro de `LoadSnapshot` (`store_postgres.go:322-362`, chamada na linha 348). O analytics usa
`LoadSnapshot` por loja em `loadStoreBundle` (`analytics/service.go:172`) para os endpoints
`GET /v1/analytics/ranking|data|intelligence` (`analytics/http.go`). O Go então **só agrega**:
`buildRankingRows` filtra por mês-corrente/hoje/range explícito (`service_ranking.go:36-70`) e
`Data/Intelligence` agregam contagens (`service_data.go`, `service_intelligence.go`) — o histórico
bruto nunca é devolvido ao cliente. No escopo integrado (tenant), isso multiplica: N lojas × histórico
completo em memória por request. O front já manda `dateFrom/dateTo` (mês corrente por default,
`web/app/stores/analytics.ts:19-25,43-44,66-81`), mas o back ignora as datas no CARREGAMENTO (usa só
na agregação, e apenas no ranking).

**Caso 3 — BI (bi).** `defaultPerolaPageLimit=100` × `defaultPerolaMaxPages=50`
(`back/internal/modules/bi/service.go:23-24`) permitem até 5.000 registros/dataset num payload único
do `GET /v1/bi/perola/overview` (`bi/http.go:53`). **Divergência verificada nesta leitura**: as 4
definições de dataset hoje fixam `MaxPages: 1` (`bi/service.go:697,722,748,772`), então o overview
efetivo busca ≤100 registros/dataset (25 no inventário) — o fallback de 50 páginas
(`service.go:383-386` → `service.options.MaxPages`, default do env `PEROLA_BI_MAX_PAGES=50` em
`config/config.go:153`) fica como bomba armada para qualquer dataset futuro sem `MaxPages` explícito.
A flag `Truncated` já existe e é calculada (`service.go:416,443`; `model.go:79`).

Por que importa: são as rotas de leitura mais pesadas do painel; sem teto, o custo cresce linear com
o histórico (memória do processo Go, tempo de scan, payload JSON), e o pool tem `MaxConns=10`.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Reports: LIMIT no SQL com teto default + parâmetro `limit` documentado + metadados de truncamento
   (`historyWindow`) nas respostas, SEM mudar o shape existente (só adição de campos).
2. Analytics: janela de data no carregamento do histórico (default 90 dias; range explícito do
   ranking respeitado quando mais antigo), já que o Go só agrega e o front já trabalha com mês corrente.
3. BI: reduzir o teto default do fallback de páginas (50→5) mantendo a flag `Truncated` existente.
4. Testes unitários por caso nos `_test.go` dos módulos.

**Não-objetivos (NÃO fazer):**
- NÃO alterar o front (stores `reports.ts`, `analytics.ts`, `bi.ts`) — segue funcionando sem mudança;
  ajustes de UI ficam como follow-ups (seção 8).
- NÃO mudar `GET /v1/operations/snapshot` (a operação ao vivo também envia `serviceHistory` completo
  ao navegador via `snapshot.go:90-111` — é um 4º caso, fora do escopo deste AC; registrado como follow-up).
- NÃO mover os filtros em memória do reports (`filterHistory`, `service_loading.go:188`) para SQL.
- NÃO mexer em `POST /v1/bi/perola/find` (proxy com body controlado pelo cliente) nem nas
  `MaxPages: 1` das definições de dataset.
- NÃO criar migration (índice `operation_service_history_store_finished_idx (store_id, finished_at desc)`
  já existe — `migrations/0007_reports_indexes.sql:1-2`).
- NÃO janelar `loadSessions` (`operations/store_postgres.go:533`) — também sem teto, fica p/ follow-up.

## 3. Mudanças

### Regras de execução (obrigatórias para o implementador)

- NENHUM comando git (sessão multi-agente — só o usuário roda git).
- NÃO rodar npm/build/generate do web. Validação do back: `docker compose up -d --build api`
  DEVE ser executada ao final (back/ muda nesta spec).
- Máx 450 linhas por arquivo novo; arquivos já acima (operations/store_postgres.go 1.060,
  bi/service.go 1.932) devem DIMINUIR ou manter (as movimentações abaixo já reduzem).
- Não remover funcionalidade: todos os campos de resposta atuais permanecem; só há adição.
- Zero mock/legado novo. Go: sem lib uuid externa (string + `::uuid`), nullable com `*string`.
- Portas fixas (api 9091 host); NUNCA tocar em password_hash/dados de usuário; não inventar credenciais.
- Atualizar os AGENT.md dos módulos tocados ao final (seção 3.5).

### 3.1 Caso 1 — Reports: LIMIT no SQL + `historyWindow`

**a) `back/internal/modules/queue/reports/model.go`**
- `Filters` (linha 3): adicionar campo `Limit int \`json:"limit,omitempty"\`` (após `PageSize`).
- `repositoryFilters` (linha 23): adicionar campo `Limit int`.
- Novo tipo (junto aos responses):

```go
// HistoryWindow descreve a janela bruta de historico lida do banco antes dos
// filtros em memoria. Total e' o count(*) com os MESMOS filtros SQL (sem os
// filtros em memoria); Truncated indica que ha mais linhas do que o teto.
type HistoryWindow struct {
	Limit     int  `json:"limit"`
	Fetched   int  `json:"fetched"`
	Total     int  `json:"total"`
	Truncated bool `json:"truncated"`
}
```
- Adicionar `HistoryWindow HistoryWindow \`json:"historyWindow"\`` em: `OverviewResponse`,
  `ResultsResponse`, `RecentServicesResponse`, `MultiStoreOverviewResponse`. Campos existentes intactos.

**b) `back/internal/modules/queue/reports/http.go`**
- Em `parseFilters` (linha 126): parsear `limit` com o `parseOptionalInt` já existente (linha 197) e
  preencher `Filters.Limit`. Erro de parse → `ErrValidation` (mesmo padrão de `page`/`pageSize`).

**c) `back/internal/modules/queue/reports/service.go`**
- Constantes (linha 16-20): adicionar
  `defaultHistoryFetchLimit = 2000` e `maxHistoryFetchLimit = 5000`.
  Racional decidido: 2000 cobre o mês de uma loja grande sem filtro de data; 5000 é o máximo aceito
  via `?limit=`; o front pede `pageSize` ≤200, que pagina DENTRO dessa janela.
- Interface `Repository` (linha 27): adicionar método
  `CountHistory(ctx context.Context, storeIDs []string, filters repositoryFilters) (int, error)`.
- `Overview/Results/RecentServices`: receber a `HistoryWindow` do `loadEntries` (assinatura muda —
  item d) e preencher o campo novo da resposta.
- `MultiStoreOverview` (linha 128): após `ListHistoryByStores` (linha 157), computar a janela:

```go
window := HistoryWindow{Limit: repositoryInput.Limit, Fetched: len(history), Total: len(history)}
if repositoryInput.Limit > 0 && len(history) >= repositoryInput.Limit {
	total, countErr := service.repository.CountHistory(ctx, storeIDs, repositoryInput)
	if countErr != nil {
		return MultiStoreOverviewResponse{}, countErr
	}
	window.Total = total
	window.Truncated = total > window.Fetched
}
```
  e devolver `HistoryWindow: window` na resposta. O count SÓ roda quando a janela bateu no teto.

**d) `back/internal/modules/queue/reports/service_loading.go`**
- `normalizeFilters` (linha 105): após o clamp de `PageSize` (linhas 128-134), clampar `Limit`:

```go
if normalized.Limit <= 0 {
	normalized.Limit = defaultHistoryFetchLimit
}
if normalized.Limit > maxHistoryFetchLimit {
	normalized.Limit = maxHistoryFetchLimit
}
```
  e, junto dos repasses da linha 160-162, `repositoryInput.Limit = normalized.Limit`.
- `loadEntries` (linha 14): mudar retorno para
  `(reportScope, Filters, []operations.ServiceHistoryEntry, HistoryWindow, error)`.
  Nos DOIS ramos (multi-loja linha 44 e loja única linha 78), logo após obter `history` (ANTES de
  `filterHistory` — a janela mede o resultado bruto do SQL), computar a `HistoryWindow` com o mesmo
  bloco do item c (no ramo de loja única, `storeIDs = []string{store.ID}`). Ajustar os 3 call sites
  (`Overview`, `Results`, `RecentServices`) e os retornos de erro (`HistoryWindow{}`).

**e) `back/internal/modules/queue/reports/store_postgres.go`**
- `listHistoryQuery` (linha 39): substituir o bloco de filtros das linhas 100-148 (o miolo
  `args/position` inteiro) por uma chamada ao helper novo `appendHistoryFilters` (item f) e aplicar
  o LIMIT trocando a linha 150 por:

```go
args := []any{storeIDs}
args = appendHistoryFilters(&query, args, filters)

query.WriteString(" order by h.finished_at desc, h.created_at desc")
if filters.Limit > 0 {
	fmt.Fprintf(&query, " limit $%d", len(args)+1)
	args = append(args, filters.Limit)
}
query.WriteString(";")
```
  Atenção: usar `len(args)+1` (o código atual esquece o `position++` no ramo `MaxSaleAmount`,
  linhas 145-148 — o helper com `len(args)+1` elimina essa classe de bug). Ordenação
  `finished_at desc` garante que o truncamento preserva os registros MAIS RECENTES (índice 0007 cobre).

**f) NOVO `back/internal/modules/queue/reports/store_postgres_filters.go`** (~70 linhas)

```go
package reports

// appendHistoryFilters escreve os predicados dinamicos do historico (mesma
// semantica do antigo miolo de listHistoryQuery) numerando placeholders por
// len(args)+1. Compartilhado entre listagem e count para nunca divergirem.
func appendHistoryFilters(query *strings.Builder, args []any, filters repositoryFilters) []any
```
  Corpo: os 8 ifs atuais (`FinishedAtFrom`, `FinishedAtTo`, `ConsultantIDs`, `Outcomes`,
  `StartModes`, `IsExistingCustomer`, `MinSaleAmount`, `MaxSaleAmount`) com
  `fmt.Fprintf(query, " and h.finished_at >= $%d", len(args)+1)` etc. (NÃO incluir `Limit` aqui).

```go
// CountHistory conta o universo SQL-filtrado (sem join com queue.stores — o
// join da listagem existe so para store_name; store_id tem FK).
func (repository *PostgresRepository) CountHistory(
	ctx context.Context, storeIDs []string, filters repositoryFilters,
) (int, error)
```
  Corpo: `select count(*) from operation_service_history h where h.store_id::text = any($1)` +
  `appendHistoryFilters` + `QueryRow(...).Scan(&total)`. `len(storeIDs)==0` → `0, nil`.

**g) NOVO `back/internal/modules/queue/reports/service_pagination_test.go`**
Fakes locais implementando `Repository` e `StoreFinder` (não existem fakes no pacote hoje — só
`permissions_test.go`). Testes mínimos:
- `TestNormalizeFiltersClampsLimit`: `Limit` 0→2000, 9999→5000, 300→300.
- `TestResultsMarksTruncatedWindow`: fake devolve exatamente `Limit` entradas em `ListHistory`,
  `CountHistory` devolve `Limit+500` → resposta com `HistoryWindow.Truncated=true`,
  `Total=Limit+500`, `Fetched=Limit`; e `Rows` continua respeitando `PageSize`.
- `TestResultsSkipsCountWhenBelowLimit`: fake devolve `Limit-1` entradas e faz `CountHistory`
  dar `t.Fatal` se chamado → `Truncated=false`, `Total==Fetched`.
Principal dos testes: `auth.Principal{Role: auth.RoleOwner}` (sem permissões resolvidas — passa em
`canViewReports`, `service.go:275-282`). Filtro com `StoreID` setado para cair no ramo de loja única.

### 3.2 Caso 2 — Analytics: janela de 90 dias no carregamento do histórico

**a) NOVO `back/internal/modules/queue/operations/store_postgres_history.go`** (~200 linhas)
MOVER de `store_postgres.go` (deletar lá) as funções `LoadSnapshot` (linhas 322-362) e
`loadServiceHistory` (linhas 565-707), reorganizadas assim:

```go
package operations

// LoadSnapshot mantem o contrato atual (historico completo) — usado pela
// operacao ao vivo (service.go:437 via loadSnapshotState).
func (repository *PostgresRepository) LoadSnapshot(ctx context.Context, storeID string) (SnapshotState, error) {
	return repository.LoadSnapshotWithHistorySince(ctx, storeID, 0)
}

// LoadSnapshotWithHistorySince carrega o snapshot com o historico janelado por
// finished_at >= historySinceMillis (0 = sem janela). Consumido pelo analytics.
func (repository *PostgresRepository) LoadSnapshotWithHistorySince(ctx context.Context, storeID string, historySinceMillis int64) (SnapshotState, error)

// buildServiceHistoryQuery e' pura para ser testavel sem banco.
func buildServiceHistoryQuery(storeID string, sinceMillis int64) (string, []any)

func (repository *PostgresRepository) loadServiceHistory(ctx context.Context, storeID string, sinceMillis int64) ([]ServiceHistoryEntry, error)
```
- `LoadSnapshotWithHistorySince` = corpo atual de `LoadSnapshot`, trocando a linha 348 por
  `repository.loadServiceHistory(ctx, storeID, historySinceMillis)`.
- `buildServiceHistoryQuery`: SQL atual (linhas 566-618) com `where store_id = $1::uuid` +
  (`sinceMillis > 0` → `" and finished_at >= $2"`, args `[]any{storeID, sinceMillis}`) + o mesmo
  `order by started_at asc, created_at asc;`.
- `loadServiceHistory`: usa a query construída; scan/decodes idênticos aos atuais (692-702).
- A interface `Repository` de operations (`model.go:398`) NÃO muda (o fake de
  `service_parallel_test.go:72` continua válido). `store_postgres.go` cai de 1.060 p/ ~880 linhas.

**b) `back/internal/modules/queue/analytics/model.go`**
- Na interface `Repository` (linha 23), substituir
  `LoadSnapshot(ctx context.Context, storeID string) (operations.SnapshotState, error)` por
  `LoadSnapshotWithHistorySince(ctx context.Context, storeID string, historySinceMillis int64) (operations.SnapshotState, error)`.
  (Interface interna do pacote; único implementador é o repositório abaixo.)

**c) `back/internal/modules/queue/analytics/store_postgres.go`**
- Trocar o delegate das linhas 26-28 por:

```go
func (repository *PostgresRepository) LoadSnapshotWithHistorySince(ctx context.Context, storeID string, historySinceMillis int64) (operations.SnapshotState, error) {
	return repository.operations.LoadSnapshotWithHistorySince(ctx, storeID, historySinceMillis)
}
```

**d) `back/internal/modules/queue/analytics/helpers.go`**
- Adicionar (o arquivo já importa `time` e define `analyticsLocation` na linha 13):

```go
const defaultHistoryWindowDays = 90

// historySinceMillis define a janela de carga do historico: 90 dias por
// default; se o dateFrom explicito for mais antigo que a janela, respeita-o
// (o ranking com range antigo nao pode perder dados).
func historySinceMillis(dateFrom string, now time.Time) int64 {
	windowStart := now.AddDate(0, 0, -defaultHistoryWindowDays)
	if parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateFrom), analyticsLocation); err == nil && parsed.Before(windowStart) {
		return parsed.UnixMilli()
	}
	return windowStart.UnixMilli()
}
```

**e) `back/internal/modules/queue/analytics/service.go`**
- `loadBundles` (linha 125) e `loadStoreBundle` (linha 171) ganham parâmetro final
  `historySinceMillis int64`, repassado até `repository.LoadSnapshotWithHistorySince(ctx, store.ID, historySinceMillis)`
  (substitui a linha 172).
- Call sites: `Ranking` (linha 46) passa `historySinceMillis(dateFrom, time.Now().In(analyticsLocation))`;
  `Data` (linha 67) e `Intelligence` (linha 93) passam `historySinceMillis("", time.Now().In(analyticsLocation))`.
- Decisão registrada (comportamento muda de propósito): `/v1/analytics/data` e
  `/v1/analytics/intelligence` passam a agregar sobre os últimos 90 dias em vez de todo o histórico.
  O front já exibe essas telas com mentalidade de mês corrente (`analytics.ts:31,43-44` manda
  `dateFrom/dateTo` do mês, que o back dessas 2 rotas nem parseia hoje — `analytics/http.go:38-57,59-78`).
  Janela cobre mês corrente + 2 anteriores; ranking com range explícito antigo continua íntegro.

**f) NOVO `back/internal/modules/queue/analytics/service_window_test.go`**
- `TestHistorySinceMillisDefaultsTo90Days`: `dateFrom=""` → `now-90d` (comparar UnixMilli com
  tolerância exata usando `now` fixo passado por parâmetro).
- `TestHistorySinceMillisRespectsOlderDateFrom`: `dateFrom` 200 dias atrás → retorna o próprio dateFrom.
- `TestHistorySinceMillisIgnoresRecentDateFrom`: `dateFrom` 10 dias atrás → retorna `now-90d`.

**g) NOVO `back/internal/modules/queue/operations/store_postgres_history_test.go`**
- `TestBuildServiceHistoryQueryWithoutWindow`: `sinceMillis=0` → SQL NÃO contém `finished_at >=`,
  args `== 1`.
- `TestBuildServiceHistoryQueryWithWindow`: `sinceMillis>0` → SQL contém `and finished_at >= $2`,
  args `== 2`, e mantém `order by started_at asc`.

### 3.3 Caso 3 — BI: teto default do fallback 50→5 páginas

**a) `back/internal/modules/bi/service.go`**
- Linha 24: `defaultPerolaMaxPages = 50` → `defaultPerolaMaxPages = 5` (com comentário
  `// teto de seguranca do fallback; datasets do overview fixam MaxPages:1`).
- NÃO mexer em `defaultPerolaPageLimit` (100) nem nas definições (`MaxPages: 1` fica).
- A flag `Truncated` (linhas 416 e 443; `model.go:79`) permanece intacta — já sinaliza corte.

**b) `back/internal/platform/config/config.go`**
- Linha 153: `PerolaBIMaxPages: getEnvInt("PEROLA_BI_MAX_PAGES", 50)` → default `5`
  (env continua permitindo subir de volta se o cliente precisar; ver Notas de Deploy).

**c) `back/internal/modules/bi/service_test.go`** (já existe; adicionar no fim)
- `TestNewServiceDefaultMaxPagesIsCapped`: `NewService()` → `options.MaxPages == 5` e
  `options.PageLimit == 100`.
- `TestOverviewDatasetDefinitionsKeepSinglePage`: todas as definições de
  `perolaDatasetDefinitions()` com `IncludeInOverview` têm `MaxPages == 1` (trava de regressão
  contra reativar o fan-out de 50 páginas no overview).

### 3.4 Compat com o front (garantias)

- Reports: respostas mantêm TODOS os campos atuais; `historyWindow` e `filters.limit` são adições
  que `reports.ts` ignora (leitura por campos nomeados, linhas 178-182/242-246). Comportamento só
  muda quando o histórico filtrado passa de 2000 linhas — e aí o corte fica explícito no metadado.
- Analytics: shapes de `RankingResponse/DataResponse/IntelligenceResponse` intocados.
- BI: overview inalterado na prática (datasets já capam em 1 página).

### 3.5 AGENT.md (obrigatório, ao final)

- `back/internal/modules/queue/reports/AGENT.md`: documentar `?limit=` (default 2000, máx 5000),
  o objeto `historyWindow` e o contrato "total = universo SQL, sem filtros em memória".
- `back/internal/modules/queue/operations/AGENT.md`: `LoadSnapshotWithHistorySince` + arquivo novo
  `store_postgres_history.go`; nota de que `LoadSnapshot` (operação ao vivo) segue sem janela.
- `back/internal/modules/queue/analytics/AGENT.md`: janela default de 90 dias e regra do dateFrom antigo.
- `back/internal/modules/bi/AGENT.md`: novo default `PEROLA_BI_MAX_PAGES=5` (fallback) e datasets a 1 página.

## 4. Critérios de aceite

1. `GET /v1/reports/results?storeId=X` sem `limit` executa SQL com `limit $N` = 2000 e a resposta tem
   `historyWindow` com `limit:2000`; `?limit=100` aplica 100; `?limit=99999` clampa em 5000;
   `?limit=abc` → 400 `validation_error`.
2. Quando o número de linhas no banco (com filtros SQL) excede o teto, `historyWindow.truncated=true`
   e `historyWindow.total` = count real; abaixo do teto, `CountHistory` NÃO é executado e
   `total==fetched`.
3. Os 4 responses de reports mantêm todos os campos pré-existentes (diff de JSON: só adições).
4. `/v1/analytics/ranking|data|intelligence` chamam `LoadSnapshotWithHistorySince` com
   `since = now-90d` (ou `dateFrom` se mais antigo, só no ranking); nenhum caminho do analytics chama
   mais `LoadSnapshot` sem janela.
5. `GET /v1/operations/snapshot` (operação ao vivo) permanece byte-idêntico: `LoadSnapshot` continua
   passando `0` (sem janela).
6. `defaultPerolaMaxPages==5`, `PEROLA_BI_MAX_PAGES` default 5, datasets do overview seguem `MaxPages:1`,
   flag `truncated` continua emitida.
7. Todos os testes novos passam e nenhum teste existente quebra
   (`service_parallel_test.go`, `service_alerts_test.go`, `permissions_test.go`, `bi/service_test.go`).
8. Nenhum arquivo NOVO acima de 450 linhas; `operations/store_postgres.go` termina MENOR que as
   1.060 linhas atuais.
9. AGENT.md dos 4 módulos atualizados.

## 5. Validação

```powershell
# 1. Testes unitários dos módulos tocados (host, a partir de back/)
cd back
go test ./internal/modules/queue/reports/... ./internal/modules/queue/operations/... ./internal/modules/queue/analytics/... ./internal/modules/bi/...
go vet ./internal/modules/queue/... ./internal/modules/bi/...

# 2. Rebuild da api (OBRIGATÓRIO — back/ mudou; o implementador RODA isso)
cd ..
docker compose up -d --build api
docker logs --tail 50 fila-atendimento-api-1   # sem panics; migrations ok

# 3. Smoke manual (deixar listado p/ o usuário — NÃO inventar credenciais; pedir token/login)
# curl -H "Authorization: Bearer <TOKEN>" "http://localhost:9091/v1/reports/results?storeId=<STORE>&limit=5"
#   → historyWindow.limit=5 e truncated=true se a loja tiver >5 atendimentos
# curl -H "Authorization: Bearer <TOKEN>" "http://localhost:9091/v1/analytics/data?storeId=<STORE>"
#   → 200 com shape idêntico ao atual
```

Validação web (SÓ listar; usuário aprova antes de rodar): `docker compose run --rm web npm run test`
— nenhum teste de front depende dos campos novos.

## 6. Notas de Deploy

- **Migrations:** nenhuma (índice `(store_id, finished_at desc)` já existe desde 0007).
- **Env vars:** default de `PEROLA_BI_MAX_PAGES` muda 50→5. Efeito prático nulo no overview (datasets
  fixam 1 página); se algum ambiente depender do fallback alto para o proxy de datasets futuros,
  setar `PEROLA_BI_MAX_PAGES` explicitamente no `.env`/compose da VPS. Sem outras vars.
- **Rebuild:** `docker compose up -d --build api` (obrigatório — Go mudou). Web não precisa.
- **Ordem:** só o build da api; sem passos de dados.
- **Comportamento visível:** relatórios de lojas com >2000 atendimentos no filtro passam a computar
  métricas sobre os 2000 mais recentes (com `truncated:true`); analytics passa a janela de 90 dias.
  Avisar no changelog interno.

## 7. Arquivos tocados

Editar:
- `back/internal/modules/queue/reports/model.go`
- `back/internal/modules/queue/reports/http.go`
- `back/internal/modules/queue/reports/service.go`
- `back/internal/modules/queue/reports/service_loading.go`
- `back/internal/modules/queue/reports/store_postgres.go`
- `back/internal/modules/queue/reports/AGENT.md`
- `back/internal/modules/queue/operations/store_postgres.go` (remoção das funções movidas)
- `back/internal/modules/queue/operations/AGENT.md`
- `back/internal/modules/queue/analytics/model.go`
- `back/internal/modules/queue/analytics/store_postgres.go`
- `back/internal/modules/queue/analytics/service.go`
- `back/internal/modules/queue/analytics/helpers.go`
- `back/internal/modules/queue/analytics/AGENT.md`
- `back/internal/modules/bi/service.go`
- `back/internal/modules/bi/service_test.go`
- `back/internal/modules/bi/AGENT.md`
- `back/internal/platform/config/config.go`

Criar:
- `back/internal/modules/queue/reports/store_postgres_filters.go`
- `back/internal/modules/queue/reports/service_pagination_test.go`
- `back/internal/modules/queue/operations/store_postgres_history.go`
- `back/internal/modules/queue/operations/store_postgres_history_test.go`
- `back/internal/modules/queue/analytics/service_window_test.go`

## 8. Follow-ups (fora deste AC — não implementar)

1. Front reports: exibir aviso quando `historyWindow.truncated` ("mostrando os 2000 mais recentes —
   refine o período") em `/relatorios`.
2. Analytics back: parsear `dateFrom/dateTo` também em `/v1/analytics/data|intelligence`
   (o front já envia; hoje são ignorados).
3. Front BI: badge "parcial" quando `source.truncated` em `/bi`.
4. `GET /v1/operations/snapshot`: janelar o `serviceHistory` enviado à operação ao vivo (4º caso).
5. `loadSessions` (`operations/store_postgres.go:533`): janela de data análoga.
6. Cursor/keyset real (`finished_at < $cursor`) nos reports se o teto de 2000 apertar.
