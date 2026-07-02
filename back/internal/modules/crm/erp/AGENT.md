# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/erp`.

## Objetivo

Este modulo concentra a integracao ERP/FTP da aplicacao.
Na fase 1, ele precisa sustentar:

- ingestao idempotente do consolidado ERP da loja ativa
- persistencia raw exata do layout FTP
- projecoes rapidas para busca de produtos
- endpoints de status e listagem via HTTP em dev e HTTPS em prod; trigger manual somente em dev

## Regras do modulo

- manter `raw` separado da projecao `current`
- preservar `tenant_id`, `store_id`, `store_code` e `store_cnpj`
- mutacao manual de sync deve continuar bloqueada fora de dev/opt-in
- arquivos binarios/CSV continuam fora do PostgreSQL; o banco guarda metadados, checksums e controle de processamento

## Shape preferido

- `model.go`
- `errors.go`
- `parser.go`
- `service.go`
- `http.go`
- `repository_postgres.go` apenas com struct/construtor
- `repository_scope.go`, `repository_status.go`, `repository_items.go`, `repository_raw_records.go`
- `repository_crm*.go` para agregacoes e helpers CRM
- `repository_import_*.go`, `repository_sync_*.go`, `repository_raw_mirror.go`

## MVP atual

- parser CSV nativo em Go implementado para `item`, `customer`, `employee`, `order` e `ordercanceled`
- source abstraction implementada com `local`, `ftp`, `sftp` e `ftps`
- ingestion manual via `POST /v1/erp/sync` e `POST /v1/erp/backfill` reaproveitando `erp_sync_runs`, `erp_sync_files` e tabelas raw atuais
- scheduler automatico implementado no bootstrap do app via `ERP_SYNC_AUTOMATIC_ENABLED`, `ERP_SYNC_INTERVAL`, `ERP_SYNC_HOUR_UTC` e `ERP_SYNC_DRY_RUN_DEFAULT`
- projeção de `erp_item_current` agora considera `source_extracted_at` como critério de desempate
- bootstrap markdown legado permanece ativo por compatibilidade e deve ser tratado como caminho em transição
- o FTP real em `extract_files` já foi validado com arquivos `item`, `customer`, `employee`, `order` e `ordercanceled`
- o codigo `184` aparece nos arquivos observados do ERP, mas o modulo nao deve tratar isso como escopo fixo de UI nem como tenant separado
- `ResolveDefaultTenantID` para `platform_admin` filtra `tenants` por `exists (... stores ativas)` antes do `limit 2`. Isso evita 404 "Loja nao encontrada" quando existem tenants vazios (sem stores) que apareceriam alfabeticamente antes do tenant real do ERP. A regra de "exatamente 1 tenant acessivel ou `ErrTenantRequired`" continua valendo apos o filtro
- `GET /v1/erp/crm` agrega vendas ERP por loja comercial e consultor no escopo raiz do ERP, resolvendo a loja nesta ordem: `store_id_raw`, override especial de multi-loja, loja dominante do historico ERP do vendedor, cadastro interno do vendedor (`users` + `consultants`/`core.user_module_settings(queue)` lojas do usuario; migrado de `user_store_roles` no U4a) e `store_cnpj` como ultimo fallback; ver tambem `docs/ERP_CRM_STORE_ATTRIBUTION.md`
- repositorio PostgreSQL fatiado por responsabilidade; manter novos metodos perto do arquivo tematico correspondente em vez de voltar a concentrar em `repository_postgres.go`
- `GET /v1/erp/status` (`GetStatus` em `repository_status.go`): o "ultimo run" e o "ultimo arquivo" de TODOS os data types sao buscados em 2 queries agregadas (`getLastRunByType`/`getLastFileByType`, `distinct on (data_type)`), nao 2 por tipo. Antes eram 10 round-trips sequenciais (5 tipos x 2) — N+1 que pesava no boot do `/erp`. Os helpers antigos `getLastRun`/`getLastFile` por tipo foram removidos. Ao adicionar data type novo, basta incluir em `supportedDataTypes`; nao reintroduzir lookups por tipo em loop

## Aba Compras — data real da compra, exclusao de cancelados e ordenacao (2026-06-30)

Contexto: a aba "Compras" antes filtrava por data de importacao do lote e incluia pedidos cancelados, divergindo do CRM e oscilando entre ambientes; agora filtra pela data REAL da compra e exclui cancelados, servindo de base exata para o filtro de sorteio ("compras do dia X ao Y acima de R$ V"). Plano: `planos/glowing-jingling-oasis.md`. Afeta `GET /v1/erp/stats` (`GetRecordsStats`, `repository_records_stats.go`) e `GET /v1/erp/records`/`/v1/erp/orders` (`ListRawRecords`, `repository_raw_records.go`).

- Novos parametros de query (ambos endpoints, em `RawRecordsQuery`/`RecordsStatsQuery` no `model.go`):
  - `dateField`: `order_date` (PADRAO — data real da compra) ou `batch_date` (data do lote importado, para auditoria). So afeta `order`/`ordercanceled`; para `customer`/`employee` o filtro de periodo e sempre o lote (`source_batch_date`). Sanitizado em `normalizeDateField` (`http.go`): qualquer valor que nao seja `batch_date` vira `order_date`. A coluna real e o estilo do predicado saem de `resolveOrderDateColumn` + `dateRangeClause` (`repository_records_stats.go`); modo lote compara `::date` nas duas pontas inclusivas, modo `order_date` usa fim exclusivo (`< dia+1`) por ser `timestamptz`.
  - `minValueCents`: filtro de valor minimo da compra em centavos (`0` = sem filtro; `parseOptionalCents` no `http.go` tolera vazio/invalido/negativo => 0). Filtra pelo total bruto da nota `coalesce(nullif(max(total_amount_cents),0), sum(amount_cents))`. So se aplica a `order`/`ordercanceled`; na lista, so vale no modo dedup (que e o que a UI usa). Posicoes de parametro: stats `$11`; dedup da lista `$11` (count) e `$13` (list).
- Invariante: pedidos cancelados sao excluidos de TODA query de pedido ATIVO (cards e lista da aba Compras), via anti-join contra `erp_order_canceled_raw` por `order_id` (helper `activeOrderCancelClause` em `repository_records_stats.go`, `nullif(trim(...))` nas duas pontas). A aba "Cancelados" (`dataType=ordercanceled`) NAO recebe o anti-join (mostra os cancelados de proposito) — o helper so retorna a clausula para `DataTypeOrder`.
- Ordenacao: a allowlist em `resolveRawRecordsSortColumn` (`repository_raw_records.go`) agora TRADUZ o id da coluna do front para a coluna real — `total_amount_raw` => `total_amount_cents` (numerico) e `order_date_raw` => `order_date` (cronologico, com `nulls last`). Antes `total_amount_raw` nao estava na allowlist e a ordenacao caia no fallback `source_batch_date` (a tabela "nao ordenava por valor"). O CTE de dedup (`latest_orders`/`grouped_orders`) agora projeta tambem `order_date` para permitir a ordenacao cronologica.

## Busca por nome + contato do cliente na aba Compras/Cancelados (2026-07-01)

Contexto: `erp_order_raw`/`erp_order_canceled_raw` NAO guardam nome/contato do cliente — so o `customer_id` (CPF). Nome e contato sao enriquecidos pos-query pelo CPF (`enrichOrderRecords`, join com `erp_customer_raw.identifier`, lote mais recente vence). Por isso a busca geral de pedidos so achava por CPF; digitar o nome nao retornava nada.

- Busca por nome: a `searchCondition` de `order`/`ordercanceled` inclui um semi-join contra `erp_customer_raw` pelo nome/apelido, via helper unico `customerNameMatchSubquery(likePlaceholder)` (em `repository_records_stats.go`). CRITICO: casa SO com o nome do LOTE MAIS RECENTE por CPF (`DISTINCT ON (identifier)` guiado pelo indice de recencia `0184`), o mesmo que a coluna "Nome" exibe. Sem esse recorte, o CPF generico de "consumidor sem CPF" (`82541150016`: 10.4k linhas, 3.6k nomes) faria buscar "Bruna" trazer pedidos rotulados "Iure". NAO reintroduzir `NOT EXISTS` aqui: o planner o resolvia como anti-join PARALELO que hasheava as ~345k linhas e estourava o `/dev/shm` (64MB) do container Postgres — `SQLSTATE 53100 could not resize shared memory segment / No space left on device`, quebrando lista e stats em TODA busca. Filtra por `tenant_id` (defesa em profundidade). Subquery NAO-correlacionada com o pedido => materializa 1x (semi-join), sem N+1.
- ATENCAO: a `searchCondition` de pedido usa o helper nos DOIS arquivos que precisam andar juntos, senao lista e cards de estatistica divergem: `repository_raw_records.go` (lista, placeholder `$4`) e `repository_records_stats.go` (`GetRecordsStats`, `$6`).
- Contato: `loadCustomerContactsByIdentifier` (era `loadCustomerNamesByIdentifier`) retorna `customerContact{Name,Email,Phone,Mobile}` do lote mais recente; `enrichOrderRecords` popula `customer_name`/`customer_email`/`customer_phone`/`customer_mobile`. Colunas no front (`ERP_ORDER_COLUMNS`): Celular+Email visiveis, Telefone oculto (storage-key `erp-*-grid-columns-v7`). AVISO: para o CPF generico o contato tambem e' do ultimo lote (arbitrario); so e' confiavel para CPF real.
- Performance: `name ilike '%termo%'` tem wildcard a esquerda (nao usa B-tree), mas roda so com termo de busca e escopado por tenant. O recorte "lote mais recente" e o enrich usam o indice de recencia `erp_customer_raw_tenant_identifier_recency_idx` (migration `0184`, `(tenant_id, identifier, source_batch_date desc, created_at_imported desc)`). Se o `ilike` de nome ficar lento em tenant grande, avaliar indice trigram (`gin (name gin_trgm_ops)`, requer `pg_trgm`) — ainda nao aplicado.

## Filtro por loja e consultor na aba Compras/Cancelados (2026-07-01)

Filtro server-side (base toda, com paginacao), so para `order`/`ordercanceled`. Novos params de query em `/v1/erp/orders`, `/v1/erp/orders/canceled` e `/v1/erp/stats` (`storeFilter`, `employeeFilter` em `RawRecordsQuery`/`RecordsStatsQuery`):
- `storeFilter`: uma ou mais CHAVES de loja separadas por virgula. A chave e' `coalesce(nullif(store_id_raw,''), store_cnpj)` — o MESMO valor que resolve o label exibido (`resolveERPStoreLabel`), entao o filtro casa exatamente com a coluna "Loja". Um label com varios CNPJs (ex.: Treze = 2 CNPJs) vira uma opcao com as chaves juntas.
- `employeeFilter`: `employee_id` exato.
- Clausula unica `storeEmployeeFilterClause(storePlaceholder, employeePlaceholder)` (em `repository_records_stats.go`). Placeholders posicionais passados como os ULTIMOS args de cada query: stats `$12`/`$13` (apos minvalue `$11`); lista dedup `$12`/`$13` no count e `$14`/`$15` na list (montados via `countFilter`/`listFilter` = dateFilter + clausula, no ramo dedup de `repository_raw_records.go`). So aplicado no ramo dedup order/cancel (que e' o que a UI usa) e no stats; ramo default/customer nao referencia esses placeholders. Ao mexer na numeracao de params dessas queries, conferir a ordem de append dos args.
- Facetas (opcoes dos dropdowns): `GET /v1/erp/records/facets?dataType=&dateFrom=&dateTo=&dateField=&storeFilter=` (`RecordsFacets` no service, `repository_records_facets.go`). Retorna `{ stores:[{value,label,count}], employees:[{value,label,count}] }` dos DADOS reais do periodo (nunca lista hardcoded); lojas agrupadas por label no Go, consultores 1 por `employee_id` com nome resolvido em lote (`loadEmployeeNamesByOriginalID`). Vazio para dataType != order/ordercanceled.
  - Cascata loja -> consultor: quando `storeFilter` vem preenchido, as facetas de CONSULTOR sao restritas a quem vendeu na(s) loja(s) escolhida(s) (`loadEmployeeFacets` adiciona a clausula de loja como ultimo placeholder). As facetas de LOJA seguem SEMPRE completas (para permitir trocar de loja) — o `storeFilter` nunca filtra `loadStoreFacets`. No front, trocar a loja reseta o consultor selecionado e recarrega as facetas.

## CRM 360 — Indicadores integrados com a fila (2026-05-21)

- `GET /v1/erp/crm` agora retorna também `queueStats` com dados de atendimento da fila (`operation_service_history`) para o mesmo período e loja
- `queueStats.byConsultant` agrupa por `person_id/person_name`; o merge com `consultants` (ERP) usa `consultant_erp_links`, `users.employee_code`, nome normalizado unico e fallback por nome no frontend
- `erpCancellations` / `erpCancellationRate` adicionados em `CRMSummary` e `CRMStoreMetric` a partir de `erp_order_canceled_raw`
- novos arquivos: `repository_crm_queue.go` (queries de fila), `repository_crm_types.go` (tipos internos adicionados)
- taxa de conversão da fila = `finish_outcome = 'compra'` / total atendimentos; taxa de cancelamento = `cancel_reason preenchido` / total
- o uso da lista no frontend e cobertura por consultor/loja (`atendimentos >= pedidos ERP`), nao a razao bruta `atendimentos / pedidos`, para evitar KPIs acima de 100%
- `queueStats.byConsultant[].queueCancellationRate` (ja calculado em `repository_crm_queue.go:buildQueueStats`) e a UNICA fonte da taxa de cancelamento por consultor do card de consultor. A `consultants` store (front) consome esse mesmo payload do `GET /v1/erp/crm`, mergeia por `(storeId, personId)` com fallback por nome e propaga como `cancellationRate` ate o `ConsultantPlayerCard`/drawer da visao integrada. Nao criar DTO/endpoint paralelo nem recalcular no front; o ranking de consultor (`buildRankingRows`, client-side a partir do snapshot) NAO computa cancelamento

## Comissao por atingimento de meta (payout embutido no /v1/erp/crm) (2026-06-17)

- O calculo de comissao ("Recebimento por atingimento de meta") vive em
  `back/internal/modules/queue/commission` (pacote folha, fonte unica). O
  `/v1/erp/crm` apenas CARREGA os insumos, chama `commission.Calculate` e
  EMBUTE o resultado no DTO existente — sem endpoint/DTO paralelo, sem recalcular no front.
- Insumos carregados 1x por request, tenant-scoped (`repository_crm_payout.go`):
  politica (`tenant_operation_core_settings.crm_goal_payout_policy`), metas
  mensais por consultor (`queue.operation_goal_targets`, batch por `tenant_id` +
  `target_month`) e `store_type`/metas de loja (ja vem em `listCRMStoreTargets`,
  agora lendo `queue.stores.store_type`). Sem N+1.
- `target_month` do payout = mes do `dateFrom` da query (ou mes atual se vazio).
- Campos JSON novos no payload (consumidos pela Trilha C):
  - cada item de `consultants[]` (byConsultant) ganha:
    - `payout` = `{ amount, ratePercent, base, group, ruleLabel, penaltyApplied }`.
      `group` = `"consultant"|"manager"|"support"`; `ratePercent` ja com penalidade;
      `base` = valor sobre o qual incidiu; `ruleLabel` curto; `penaltyApplied` em
      pontos percentuais. (so quando ha dado; `omitempty`).
    - `monthlyGoalCents` e `goalProgress` do consultor (preenchidos quando ha meta propria).
  - cada item de `stores[]` ganha:
    - `storeType` (`"shopping"|"bairro"`),
    - `managerPayout` = `{ amount, ratePercent, ruleLabel }` (ja resolvido pelo
      `store_type` da loja),
    - `supportPayout` = `{ amount, ratePercent, ruleLabel }`.
    `storeSold`/`storeGoal`/`storeProgress` ja existiam como `salesCents`/
    `monthlyGoalCents`/`goalProgress`.
- O payout de loja so e calculado para loja `mapped` (com target). Loja nao
  mapeada nao recebe `managerPayout`/`supportPayout`.

## Flags de gap — aviso acionavel inline (2026-06-17)

- O payload do `GET /v1/erp/crm` expoe "flags de gap" que dizem ao front qual
  config de meta esta faltando (regra AGENT_RULES "Config/dado faltando = aviso
  ACIONAVEL inline"). Plano canonico: `docs/INLINE_QUICK_EDIT_PLAN.md` (fase
  `crm-c10`). Tudo DERIVADO do que `applyCRMPayouts` ja carrega — sem recalcular,
  sem endpoint/migration/query nova.
- Cada item de `stores[]` ganha:
  - `storeGoalSource`: `"own"` (loja tem `monthly_goal` proprio) | `"consultant-sum"`
    (meta caiu na soma das metas dos consultores) | `"none"` (sem meta alguma).
  - `missingStoreGoal` (bool): loja sem `monthly_goal` proprio cadastrado.
  - `missingTicketGoal` / `missingPaGoal` (bool): loja sem meta de ticket/PA
    (com essas faltando, a penalidade de qualidade fica desligada).
  - `splitConsultantCount` (int): nº de consultores na loja (divisor da mensagem
    "meta da loja R$ X ÷ N").
- Cada item de `consultants[]` ganha:
  - `goalSource`: `"own"` (meta mensal propria) | `"store-split"` (herdou a meta da
    loja dividida entre N consultores) | `"none"`.
  - `missingMonthlyGoal` (bool): consultor sem meta mensal propria.
  - `missingTicketGoal` / `missingPaGoal` (bool): sem meta de ticket/PA, ja
    considerando a HERANCA da loja (so marca missing quando nem o consultor nem a
    loja tem a meta).
- Preenchimento em `repository_crm_payout.go`: flags de loja no loop de lojas
  (junto de `storeGoal`/`storeProgress`); flags de consultor em
  `applyConsultantPayout` (reusa `cg`/`sg`/`monthlyGoal`/`ticketGoalCents`/`paGoal`).

## Ponte de meta para o snapshot da Operacao (2026-06-17)

- `Service.GoalStatsByConsultant(ctx, principal, tenantID, month)` (em `service.go`)
  e uma PONTE server-side para o modulo `queue/operations` embutir o atingimento
  de meta CANONICO por consultor no snapshot/overview da Lista da vez. Devolve
  `map[string]ConsultantGoalStat` indexado pelo `ProfileConsultantID` (o mesmo
  `consultant.ID` de perfil / `person.id` do front); consultores sem
  `ProfileConsultantID` sao pulados.
- Diferente de `CRMOverview`, este metodo NAO passa pelo gate `canViewERP`: o
  numero e enriquecimento, e a decisao de produto e "todos os operadores veem a
  meta de todos". O escopo continua restrito ao tenant — o caller (adapter na
  composition root) injeta um principal `platform_admin` escopado ao `tenantID`.
- Reusa exatamente o caminho da pagina de consultores: `resolveERPScope` +
  `repository.GetCRMOverview` com `allowedStoreIDs` tenant-wide quando o papel nao
  exige filtro por loja. A janela do mes (`monthWindow`, `service.go`) e do
  primeiro ao ultimo dia do mes em UTC, date-only — IGUAL ao default de
  `web/app/domain/utils/consultant-transforms.ts` — para o numero bater com o
  `goalProgress` do `/v1/erp/crm`. `month` vazio => mes corrente.
- Mapeamento `CRMConsultantMetric` -> `ConsultantGoalStat` (em reais): `MonthlyGoal
  = MonthlyGoalCents/100`, `SoldValue = SalesCents/100`, `RemainingToGoal =
  maxCRMRemaining(MonthlyGoalCents, SalesCents)/100`, `Progress = GoalProgress`,
  `HasGoal = MonthlyGoalCents > 0`.
- O cache (TTL 120s por `(tenant, mes)`) NAO fica aqui: vive no adapter da
  composition root (`back/internal/platform/app/operations_goal_progress_adapter.go`),
  para `operations` nao importar `crm/erp`.

## Invariantes novos

- nunca mutar a origem remota; apenas listar e abrir arquivos
- usar ordenação cronológica por `ExtractedAt` do nome do arquivo quando presente; no layout real do FTP, usar `ModTime` da listagem remota como fallback
- idempotência continua baseada em `(tenant_id, store_id, data_type, source_name, checksum_sha256)`
- o parser CSV deve calcular checksum em cima dos bytes originais do arquivo
