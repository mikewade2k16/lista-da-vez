# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/erp`.

## Objetivo

Este modulo concentra a integracao ERP/FTP da aplicacao.
Na fase 1, ele precisa sustentar:

- ingestao idempotente do consolidado de `item` da loja 184
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

- somente `item` da loja 184 entra no pipeline funcional
- `customer`, `employee`, `order` e `ordercanceled` entram no schema e no contrato, mas ficam como placeholders de dominio