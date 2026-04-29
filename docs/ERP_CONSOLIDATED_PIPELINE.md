# ERP Consolidated Pipeline (Markdown Source)

## Scope

This document describes the ERP ingestion flow using consolidated markdown files in:

- `Controlle10 - ftp/consolidados/<store_code>/*.md`

No direct CSV read is required for ingestion in this flow.

## Supported Data Types

- `item`
- `customer`
- `employee`
- `order`
- `ordercanceled`

## API Endpoints

- `POST /v1/erp/bootstrap/items`
  - Backward-compatible endpoint for `item` only.
- `POST /v1/erp/bootstrap`
  - Generic endpoint for all supported data types.
  - Request body:
    - `tenantId` (optional)
    - `storeCode` (required)
    - `dataType` (required: item|customer|employee|order|ordercanceled)
    - `sourcePath` (optional, defaults from env)
- `GET /v1/erp/products`
  - Paginated projection from `erp_item_current`.
  - Query params: `tenantId?`, `storeCode`, `identifierPrefix?`, `search?`, `page?`, `pageSize?`.
- `GET /v1/erp/records`
  - Paginated raw listing for non-product tabs.
  - Query params: `tenantId?`, `storeCode`, `dataType` (`customer|employee|order|ordercanceled`), `search?`, `page?`, `pageSize?`.
  - Returns `items` as a JSON array of selected raw columns for the requested type.

## Environment Variables

- `ERP_BOOTSTRAP_ITEM_FILE`
- `ERP_BOOTSTRAP_CUSTOMER_FILE`
- `ERP_BOOTSTRAP_EMPLOYEE_FILE`
- `ERP_BOOTSTRAP_ORDER_FILE`
- `ERP_BOOTSTRAP_ORDER_CANCELED_FILE`
- `ERP_ALLOW_MANUAL_SYNC`

## Storage Model

Each import creates entries in:

- `erp_sync_runs`
- `erp_sync_files`

Raw data tables used by type:

- `item` -> `erp_item_raw` and projection/upsert in `erp_item_current`
- `customer` -> `erp_customer_raw`
- `employee` -> `erp_employee_raw`
- `order` -> `erp_order_raw`
- `ordercanceled` -> `erp_order_canceled_raw`

Frontend tabs consume these sources:

- `Produtos` -> `GET /v1/erp/products` (`erp_item_current` projection)
- `Clientes` -> `GET /v1/erp/records?dataType=customer` (`erp_customer_raw`)
- `Funcionarios` -> `GET /v1/erp/records?dataType=employee` (`erp_employee_raw`)
- `Pedidos` -> `GET /v1/erp/records?dataType=order` (`erp_order_raw`)
- `Cancelados` -> `GET /v1/erp/records?dataType=ordercanceled` (`erp_order_canceled_raw`)

## Idempotency

A file is deduplicated by `erp_sync_files` unique key:

- `tenant_id`
- `store_id`
- `data_type`
- `source_name`
- `checksum_sha256`

If already imported, the batch is skipped and counted in `filesSkipped`.

## Consolidated Markdown Contract

Each file must contain:

1. Header metadata (store and type)
2. `## Ordem das Colunas`
3. `## Lote ...`
4. `### DadosCSV`
5. Semicolon-delimited rows

## Identifier Semantics (Store 184)

From consolidated markdown inspection:

- 14-digit values are CNPJ (store/sub-store identifiers)
- 11-digit values are CPF (customer identifiers)
- `184` is the internal store code (Perola root)

Known CNPJs observed in 184 consolidated order data:

- `12583959000186` (main/general)
- `56173889000163` (sub-store)
- `31327524000115` (sub-store)

Note: commercial names (e.g., Jardins/Riomar/Treze/Garcia) require an explicit mapping table by business confirmation.

## Product Scope Rule (Store 184 Root)

This rule is mandatory for ERP product screens.

- The ERP product panel is always anchored in store code `184` (root scope).
- Root scope `184` is the source of truth for catalog, imported files, run counters, and product listing.
- Sub-stores (JAR, RIO, GAR, TRE) are children of the same 184 scope and can only be used as a filter dimension.

### Store Selector Semantics

- The selector exists only to filter records inside the 184 scope.
- It must not switch the panel away from store 184 scope.
- It must not contain an option that changes source scope to a non-184 root.
- Option `184 - Loja 184` should not be shown as a selectable filter value.
- The default state should represent `all sub-stores under 184`.

### API Contract for This Screen

- Requests for ERP products and related counters must keep `storeCode=184` as fixed scope.
- Optional filtering by sub-store must be passed as an additional filter field, never replacing root `storeCode`.
- If no sub-store is selected, API must still return the complete 184 dataset.

### Acceptance Criteria (Local and VPS)

- Local and VPS must show the same behavior for selector options and default value.
- Opening ERP panel with no sub-store selected must load 184 data (not empty state).
- Selecting JAR/RIO/GAR/TRE filters within the same 184 dataset.
- Removing sub-store filter returns to full 184 dataset.
