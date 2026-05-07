# ERP FTP Ingestion

## Estado atual

O backend ERP já suporta ingestão nativa de CSV em Go para os tipos `item`, `customer`, `employee`, `order` e `ordercanceled`, sem depender do pipeline Python para a rotina nova.

Entradas disponíveis hoje:
- `POST /v1/erp/sync`
- `POST /v1/erp/backfill`
- `GET /v1/erp/runs`
- `GET /v1/erp/overview`

Ambos reaproveitam:
- `erp_sync_runs`
- `erp_sync_files`
- tabelas raw `erp_*_raw`
- projeção `erp_item_current`

## Origem dos arquivos

O módulo suporta quatro kinds de source:
- `local`
- `ftp`
- `sftp`
- `ftps`

Configuração por ambiente:
- `ERP_SOURCE_KIND`
- `ERP_LOCAL_SOURCE_DIR`
- `ERP_FTP_HOST`
- `ERP_FTP_PORT`
- `ERP_FTP_USER`
- `ERP_FTP_PASSWORD`
- `ERP_FTP_KEY_PATH`
- `ERP_FTP_REMOTE_DIR`
- `ERP_FTP_HOST_KEY`
- `ERP_SYNC_AUTOMATIC_ENABLED`
- `ERP_SYNC_INTERVAL`
- `ERP_SYNC_HOUR_UTC`
- `ERP_SYNC_DRY_RUN_DEFAULT`

## Fluxo

1. `ErpSource.List()` retorna os arquivos CSV candidatos da loja.
2. O nome do arquivo é validado por regex e convertido em `csvFileMetadata`.
3. Os arquivos são ordenados por `ExtractedAt` crescente quando o nome carrega esse campo; no layout real do FTP (`storeCode-storeCNPJ-dataType-dataReference.csv`), o fallback é o `modtime` retornado pela listagem remota.
4. `StreamCSV()` detecta UTF-8 ou fallback CP1252, valida header/colunas e produz `*RawRecord` por linha.
5. O repositório grava metadados em `erp_sync_files`, persiste nas tabelas raw e projeta `erp_item_current`.
6. A projeção de item usa `source_extracted_at` como tiebreak adicional para decidir a versão mais nova.

## Validacao real do FTP

Validado em 2026-05-05 e 2026-05-06:
- a pasta remota `extract_files` está acessível via `ftp`
- o conjunto atual contém arquivos dos cinco tipos esperados: `item`, `customer`, `employee`, `order` e `ordercanceled`
- o layout real dos nomes no FTP não traz prefixo de `ExtractedAt`; o parser agora aceita esse formato
- os CSVs reais usam `;` como separador e headers compatíveis com o parser Go
- em Docker dev, a API foi validada com `ERP_SOURCE_KIND=ftp` e `ERP_FTP_*` injetados no container
- foi executado um sync controlado real de `employee` com `maxFiles=1`, importando com sucesso 1 arquivo FTP e 9 linhas no banco
- depois disso, o mesmo teste controlado foi executado com sucesso para `item`, `customer`, `order` e `ordercanceled`
- evidencias persistidas em `erp_sync_files` para os imports controlados:
  - `item`: `record_count=2231`
  - `customer`: `record_count=126`
  - `employee`: `record_count=9`
  - `order`: `record_count=233`
  - `ordercanceled`: `record_count=6`

## Limites conhecidos

- alertas de FTP inacessível / arquivo esperado ausente ainda precisam de continuação
- abort e reprocessamento ainda precisam de continuação
- o frontend de sincronização agora já consome histórico real de runs e um overview operacional do FTP atual; ainda não há drill-down completo de arquivos por run

## Overview operacional

O endpoint `GET /v1/erp/overview` compara o inventário remoto atual do FTP com os arquivos já marcados como importados no banco para a loja consultada.

No workspace ERP do frontend, a chamada agora pode omitir `storeCode`: o backend resolve automaticamente o escopo ERP raiz do tenant e a UI apresenta isso como `Sistema completo`, sem reaproveitar a subloja operacional do header como fonte do módulo.

Ele expõe:
- `automatic.enabled`, `interval` e `hourUtc`
- totais de arquivos atuais no FTP, já importados e ainda pendentes
- resumo por entidade (`item`, `customer`, `employee`, `order`, `ordercanceled`)
- lista explícita dos CSVs faltantes
- referência técnica do módulo (`back/internal/modules/erp/AGENT.md`) com espelho público em `/erp-agent.md`

Validação real em Docker dev para `storeCode=184`:
- `35` arquivos atuais no FTP
- `35` arquivos atuais já importados
- `0` arquivos pendentes
- `automatic.enabled=false`
- `POST /v1/erp/sync` executado em `2026-05-06 13:08`, importando os `31` CSVs pendentes do FTP atual (`filesImported=31`, `filesSkipped=4`, `rowsImported=9964`)
- em `2026-05-07 11:35`, um novo lote reabriu o overview para `30/35` (`5` CSVs pendentes) e a sincronização manual disparada pela UI `/erp` importou os `5` arquivos novos, retornando o overview para `35/35` e `0` pendentes
- esse lote novo entrou como `1` arquivo por tipo, com `item=1007`, `customer=144`, `employee=10`, `order=217` e `ordercanceled=19` linhas importadas

Observação operacional importante:
- `rowsInBank` e `searchableRows` mostram o que já existe no banco hoje, inclusive cargas legadas anteriores
- a cobertura do overview considera apenas o conjunto remoto atual do FTP, então a pendência pode reabrir quando o FTP publicar um novo lote, mesmo com muitas linhas já persistidas no banco

## Operação local

Para validar contra o FTP real em Docker dev, o `api` precisa subir com as envs ERP remotas efetivamente declaradas no `docker-compose.yml` e preenchidas na sessao de execucao.

Exemplo de payload para sync manual:

```json
{
  "storeCode": "<codigo-da-loja-erp>"
}
```

Exemplo de payload para backfill:

```json
{
  "storeCode": "<codigo-da-loja-erp>",
  "triggeredBy": "backfill"
}
```

Observacao:

- o `storeCode` deve refletir a loja ERP que existe no contexto autenticado e nos nomes de arquivo da origem remota
- o fato de o FTP atual expor arquivos com codigo `184` nao transforma esse codigo em escopo fixo da UI ou tenant especial

## Próximo passo obrigatório

Fechar a camada operacional em cima do fluxo já funcional: alertas quando o FTP falhar ou quando um CSV esperado não chegar, além de endpoints completos de runs/reprocessamento para observabilidade e suporte.