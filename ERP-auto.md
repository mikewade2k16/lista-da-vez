# ERP Auto

## Objetivo

Este documento registra o estado real da automacao de ingestao do ERP via FTP, o que foi implementado, o que foi validado em 2026-05-05 e onde o trabalho parou para continuarmos depois.

## Resumo executivo

O fluxo deixou de depender da ideia de consolidar arquivos manualmente no workspace.

O backend agora foi preparado para:

- ler CSVs do ERP diretamente de uma origem remota
- processar os 5 tipos de arquivo do ERP: `item`, `customer`, `employee`, `order` e `ordercanceled`
- importar de forma idempotente
- registrar execucoes e arquivos importados
- permitir sync manual, backfill e agendamento automatico

Importante:

- o FTP real foi validado com sucesso
- os arquivos faltantes do ERP foram encontrados no FTP
- nesta etapa, o que fizemos foi validar acesso e preparar o codigo para consumir direto do FTP
- nao houve download manual definitivo desses arquivos para dentro do repositorio
- ja foi executado um ciclo controlado real no ambiente local de validacao, com a API apontando para o FTP real e importando 1 arquivo de cada um dos 5 tipos ERP
- ainda nao foi executado um ciclo final confirmado de importacao completa no banco alvo com a aplicacao rodando configurada no ambiente definitivo
- o codigo `184` aparece no layout dos arquivos ERP observados, mas nao deve ser tratado na aplicacao como uma loja fixa ou como um tenant separado

## O que foi feito

### 1. Base de dados

Foi criada a migration `0057_erp_csv_metadata.sql` para sustentar a ingestao nativa de CSV com metadados de origem.

Entraram, entre outros pontos:

- `triggered_by` em `erp_sync_runs`
- extensao de `mode` para `csv_ftp`
- metadados como `source_extracted_at`, `source_data_reference`, `source_size_bytes` e `error_message`
- suporte adicional em `erp_item_current` para desempate por origem mais recente

### 2. Parser CSV em Go

Foi implementado parser nativo em Go para:

- `item`
- `customer`
- `employee`
- `order`
- `ordercanceled`

O parser cobre:

- separador `;`
- BOM
- UTF-8 e fallback CP1252
- validacao de header
- validacao de quantidade de colunas
- parse tipado de campos
- checksum SHA-256 em cima dos bytes originais

### 3. Source abstraction

Foi criada a abstracao de origem ERP com suporte a:

- `local`
- `ftp`
- `sftp`
- `ftps`

O caso importante aqui foi o `ftp`, porque o host real respondeu via FTP comum.

Tambem ficou protegido:

- path traversal
- redacao de senha em mensagens de erro
- retry basico na conexao remota

### 4. Ingest service

O service do ERP agora consegue:

- listar arquivos da origem remota por loja
- filtrar por tipo
- ordenar cronologicamente
- abrir cada CSV
- transformar em batches tipados
- reaproveitar os imports existentes do repositorio
- gravar progresso em `erp_sync_runs`
- registrar arquivos em `erp_sync_files`

### 5. Endpoints

Entraram os endpoints:

- `POST /v1/erp/sync`
- `POST /v1/erp/backfill`
- `GET /v1/erp/runs`
- `GET /v1/erp/overview`

Eles usam o novo fluxo CSV/FTP e mantem o bootstrap markdown legado apenas como compatibilidade temporaria.

### 6. Scheduler automatico

Foi ligado um scheduler basico no bootstrap da aplicacao.

Ele usa:

- `ERP_SYNC_AUTOMATIC_ENABLED`
- `ERP_SYNC_INTERVAL`
- `ERP_SYNC_HOUR_UTC`
- `ERP_SYNC_DRY_RUN_DEFAULT`

Com isso, a aplicacao ja ficou pronta para disparar sync automatico quando estiver configurada no ambiente correto.

### 7. Frontend

Ja existe a aba de sincronizacao no ERP admin com:

- status
- botao de sync
- botao de backfill
- historico de runs reais via `GET /v1/erp/runs`
- resumo operacional via `GET /v1/erp/overview`, respondendo se o FTP atual ja foi coberto, se o scheduler automatico esta ligado, quantos CSVs faltam e qual entidade ainda esta pendente
- link tecnico publico para o AGENT do modulo em `/erp-agent.md`
- resolucao automatica do escopo ERP raiz do sistema quando a UI nao informa `storeCode`, evitando herdar `JAR`/subloja operacional do topo como origem do modulo
- apresentacao visual ajustada para `Sistema completo`, deixando explicito que a tela ERP nao segue a subloja operacional selecionada no header global

Essa parte melhorou, mas ainda nao esta fechada em observabilidade fina por arquivo/reprocessamento/abort.

## Validacao real feita

### FTP validado

Foi validado acesso ao host remoto informado pelo cliente, na pasta `extract_files`.

Resultado pratico:

- o host respondeu por `ftp`
- a pasta remota listou arquivos corretamente
- os cinco tipos de arquivo esperados estavam presentes

Exemplos observados no FTP:

- `184-12583959000186-customer-20260505010059.csv`
- `184-12583959000186-employee-20260429010206.csv`
- arquivos `item`
- arquivos `order`
- arquivos `ordercanceled`

Observacao de dominio:

- esses nomes refletem o codigo de origem presente no ERP/arquivo
- isso nao significa que a aplicacao deva tratar `184` como loja fixa do painel ou como tenant proprio
- o painel deve operar sobre a loja ativa/contexto ERP atual, sem especial-casing na UI

### Ajuste importante descoberto no FTP real

O layout real do nome do arquivo no FTP nao veio com prefixo de `ExtractedAt`.

Formato real encontrado:

- `storeCode-storeCNPJ-dataType-dataReference.csv`

Por isso, o parser foi ajustado para aceitar esse formato e, quando `ExtractedAt` nao existir no nome, a ordenacao passar a usar o `modtime` retornado pela listagem remota.

## Testes e validacoes executadas

Passaram:

- `go test ./internal/modules/erp`
- `go test ./internal/modules/erp ./internal/platform/config ./internal/platform/app`

Tambem houve validacao manual do FTP real, confirmando que a origem remota existe e que o layout real dos arquivos bate com o parser atual.

Em 2026-05-06 tambem foi validado, em Docker dev:

- ajuste do `docker-compose.yml` para repassar `ERP_SOURCE_KIND`, `ERP_FTP_*` e flags de scheduler para o container `api`
- `dryRun` real de `employee` com `maxFiles=1`, retornando `filesSeen=1`, `filesImported=1`, `rowsRead=9`
- sync real de `employee` com `maxFiles=1`, primeiro falhando por bug de persistencia em `erp_sync_files.record_count`
- correção do SQL em `repository_postgres.go`, removendo coercao texto/integer indevida no insert de `erp_sync_files`
- nova execucao do mesmo sync real com sucesso, gravando 1 arquivo FTP (`source_kind=ftp`) e `record_count=9`
- `dryRun` controlado adicional dos tipos `item`, `customer`, `order` e `ordercanceled`, todos encontrando e parseando 1 arquivo corretamente
- sync real controlado desses mesmos 4 tipos com sucesso, fechando validacao ponta a ponta dos 5 tipos ERP
- evidencias persistidas em `erp_sync_files`:
	- `item`: `184-12583959000186-item-20260430010001.csv` com `record_count=2231`
	- `customer`: `184-12583959000186-customer-20260430010218.csv` com `record_count=126`
	- `employee`: `184-12583959000186-employee-20260429010206.csv` com `record_count=9`
	- `order`: `184-12583959000186-order-20260430010221.csv` com `record_count=233`
	- `ordercanceled`: `184-12583959000186-ordercanceled-20260430010224.csv` com `record_count=6`
- endpoint `GET /v1/erp/runs?storeCode=184&page=1&pageSize=10` validado no ambiente Docker dev, retornando historico real de runs `csv_ftp`, inclusive run falho e runs corrigidos/sucedidos
- endpoint `GET /v1/erp/overview?storeCode=184` validado no ambiente Docker dev, retornando cobertura do FTP atual vs banco:
	- `35` arquivos atuais no FTP
	- `35` arquivos atuais ja importados
	- `0` arquivos atuais ainda pendentes
	- `automatic.enabled=false` no ambiente atual
	- totais por entidade mostrando junto `rowsInBank` e `searchableRows`, para separar claramente o que ja existe no banco do que ainda falta cobrir no FTP atual
	- `POST /v1/erp/sync` executado em `2026-05-06 13:08`, importando os `31` CSVs pendentes do FTP atual (`filesImported=31`, `filesSkipped=4`, `rowsImported=9964`)
	- em `2026-05-07 11:35`, um novo lote publicado no FTP reabriu temporariamente o overview para `30/35` (`5` CSVs pendentes) e a sincronizacao manual pela propria UI `/erp` importou esses `5` arquivos, fechando novamente em `35/35` e `0` pendentes
	- o lote de `2026-05-07 11:35` entrou como `1` arquivo por tipo, com `item=1007`, `customer=144`, `employee=10`, `order=217` e `ordercanceled=19` linhas importadas
	- em `2026-05-07` tambem foi validado um modo temporario de backfill historico local via `docker-compose.erp-local-temp.yml`, montando `C:/Users/Mike/Downloads/processed/loja_184` como source `local` sem copiar arquivos para o repositorio
	- nesse modo local, o inventario historico da loja `184` apareceu como `4370` arquivos no total; o overview inicialmente mostrou apenas `115` pendentes porque `4255` nomes de arquivo ja estavam cobertos por runs antigos persistidos como `bootstrap_markdown`, e o overview considera cobertura por `source_name` ja importado no banco
	- o sync historico local terminou em `6m24s` com status `200` e o overview final fechou em `4370/4370` e `0` pendentes para a loja `184`
	- observacao importante: dois runs locais (`item` e `customer`) acusaram mismatch de layout em arquivos antigos (`14` vs `16` colunas em `item`, `21` vs `22` em `customer`), mas esses mesmos arquivos ja estavam marcados como cobertos por imports anteriores; se precisarmos reprocessar todo o historico exclusivamente pelo parser novo, ainda falta suportar esses layouts legados

## Onde paramos

Paramos no ponto em que:

- o codigo para consumo direto do FTP esta implementado
- o scheduler automatico basico esta implementado
- o parser e a source remota foram ajustados para o FTP real
- a documentacao tecnica do modulo foi atualizada
- o fluxo real FTP -> parser -> persistencia ja foi provado em lote controlado pequeno para os 5 tipos ERP
- a UI do ERP agora tambem tem um resumo operacional mais direto para responder “ja puxamos tudo?”, “esta automatico?” e “o que falta?”

Mas ainda nao fechamos a operacao completa de producao.

## O que ainda nao foi feito

### Nao foi feito ainda

- rodar a aplicacao no ambiente alvo com as envs definitivas do FTP
- disparar e conferir um sync/backfill real mais amplo no banco alvo final
- validar contagens finais no banco apos ingestao real completo por tipo/loja
- fechar endpoints completos de listagem detalhada de runs e reprocessamento
- criar alertas para FTP indisponivel
- criar alertas para ausencia de CSV esperado
- criar abort/reprocess com observabilidade completa

### Sobre os itens faltantes do ERP

Sim, conseguimos confirmar que os itens faltantes existem na origem remota do ERP.

O ponto exato e:

- antes, o workspace local nao tinha os CSVs brutos completos
- depois, validamos o FTP real e encontramos os tipos que faltavam
- entao o bloqueio deixou de ser “onde estao os arquivos?”
- o bloqueio passou a ser operacional: rodar o fluxo final no ambiente com configuracao efetiva e concluir a camada de observabilidade/controle

## Proximos passos

### Passo 1. Configurar ambiente

Subir a API com envs do ERP configuradas, sem gravar segredo no repositorio.

Variaveis relevantes:

- `ERP_SOURCE_KIND=ftp`
- `ERP_FTP_HOST`
- `ERP_FTP_PORT=21`
- `ERP_FTP_USER`
- `ERP_FTP_PASSWORD`
- `ERP_FTP_REMOTE_DIR=extract_files`
- `ERP_ALLOW_MANUAL_SYNC=true` no ambiente de validacao

Opcional para automacao:

- `ERP_SYNC_AUTOMATIC_ENABLED=true`
- `ERP_SYNC_HOUR_UTC`
- `ERP_SYNC_INTERVAL`
- `ERP_SYNC_DRY_RUN_DEFAULT=false`

### Passo 2. Executar sync real

Executar um sync manual real mais amplo e confirmar resultado em:

- `erp_sync_runs`
- `erp_sync_files`
- `erp_item_raw`
- `erp_customer_raw`
- `erp_employee_raw`
- `erp_order_raw`
- `erp_order_canceled_raw`
- `erp_item_current`

Observacao:

- esse passo nao e mais sobre provar se o pipeline funciona; isso ja foi validado com sucesso em lote controlado para todos os tipos
- o objetivo agora passa a ser validar volume maior, cobertura historica e comportamento operacional no banco alvo

### Passo 3. Executar backfill controlado

Rodar backfill real por loja e verificar:

- quantidade de arquivos vistos
- quantidade de arquivos importados
- falhas registradas
- quantidade de linhas importadas por tipo

### Passo 4. Fechar operacao

Implementar o que falta para a automacao ficar realmente operacional:

- endpoints completos de runs
- detalhe por arquivo
- reprocessamento manual
- abort
- alertas de FTP inacessivel
- alertas de arquivo esperado ausente

## Arquivos principais tocados

- `back/internal/platform/database/migrations/0057_erp_csv_metadata.sql`
- `back/internal/modules/erp/csv_parser.go`
- `back/internal/modules/erp/csv_parser_test.go`
- `back/internal/modules/erp/source.go`
- `back/internal/modules/erp/source_local.go`
- `back/internal/modules/erp/source_local_test.go`
- `back/internal/modules/erp/source_ftp_test.go`
- `back/internal/modules/erp/ftp_client.go`
- `back/internal/modules/erp/service.go`
- `back/internal/modules/erp/repository_postgres.go`
- `back/internal/modules/erp/http.go`
- `back/internal/platform/config/config.go`
- `back/internal/platform/app/app.go`
- `web/app/components/erp/ErpWorkspace.vue`
- `web/app/components/erp/ErpSyncStatus.vue`
- `web/app/components/erp/ErpSyncRunsTable.vue`
- `web/app/components/erp/ErpSyncRunDetail.vue`
- `web/app/stores/erp.ts`

## Estado final deste checkpoint

Estado em 2026-05-05:

- conseguimos localizar e validar a origem real no FTP
- conseguimos adaptar o backend para consumir essa origem
- conseguimos deixar a automacao basica preparada
- ainda falta rodar e homologar o ciclo final no banco/ambiente alvo

Em outras palavras:

- a parte de descoberta e implementacao tecnica principal foi resolvida
- a parte de operacao final e fechamento ainda ficou para a proxima sessao