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
- `repository_postgres.go`

## MVP atual

- parser CSV nativo em Go implementado para `item`, `customer`, `employee`, `order` e `ordercanceled`
- source abstraction implementada com `local`, `ftp`, `sftp` e `ftps`
- ingestion manual via `POST /v1/erp/sync` e `POST /v1/erp/backfill` reaproveitando `erp_sync_runs`, `erp_sync_files` e tabelas raw atuais
- scheduler automatico implementado no bootstrap do app via `ERP_SYNC_AUTOMATIC_ENABLED`, `ERP_SYNC_INTERVAL`, `ERP_SYNC_HOUR_UTC` e `ERP_SYNC_DRY_RUN_DEFAULT`
- projeção de `erp_item_current` agora considera `source_extracted_at` como critério de desempate
- bootstrap markdown legado permanece ativo por compatibilidade e deve ser tratado como caminho em transição
- o FTP real em `extract_files` já foi validado com arquivos `item`, `customer`, `employee`, `order` e `ordercanceled`
- o codigo `184` aparece nos arquivos observados do ERP, mas o modulo nao deve tratar isso como escopo fixo de UI nem como tenant separado
- `GET /v1/erp/crm` agrega vendas ERP por loja comercial e consultor no escopo raiz do ERP, resolvendo a loja nesta ordem: `store_id_raw`, cadastro interno do vendedor (`users` + `consultants`/`user_store_roles`), loja dominante do historico ERP do vendedor e `store_cnpj` como ultimo fallback; ver tambem `docs/ERP_CRM_STORE_ATTRIBUTION.md`

## Invariantes novos

- nunca mutar a origem remota; apenas listar e abrir arquivos
- usar ordenação cronológica por `ExtractedAt` do nome do arquivo quando presente; no layout real do FTP, usar `ModTime` da listagem remota como fallback
- idempotência continua baseada em `(tenant_id, store_id, data_type, source_name, checksum_sha256)`
- o parser CSV deve calcular checksum em cima dos bytes originais do arquivo