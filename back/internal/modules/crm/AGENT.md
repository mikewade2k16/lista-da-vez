# AGENT — module `crm`

## Escopo

Módulo `back/internal/modules/crm/`. Domínio CRM/ERP da plataforma:
integração FTP/CSV com sistema ERP, dashboard CRM 360 (vendas × fila),
busca de produtos no catálogo.

Branch alvo: `refactor/multi-tenant-core`. Plano canônico:
`docs/MULTITENANT_COMPLETION_PLAN.md`.

## Estrutura de arquivos

```
crm/
  module.go        — implementa modules.Module (ID="crm", Metadata, Permissions, RoleTemplates, Build)
  AGENT.md
  erp/             — subpacote crm/erp (ingestao FTP/CSV, CRM 360)
    model.go
    errors.go
    parser.go
    service.go
    service_helpers.go
    service_sync.go
    http.go
    source.go
    source_local.go
    ftp_client.go
    csv_parser.go
    relations_resolver.go
    repository_postgres.go
    repository_scope.go
    repository_status.go
    repository_items.go
    repository_raw_records.go
    repository_raw_mirror.go
    repository_sync_runs.go
    repository_sync_files.go
    repository_sync_recovery.go
    repository_import_customers.go
    repository_import_employees.go
    repository_import_items.go
    repository_import_orders.go
    repository_crm.go
    repository_crm_aggregates.go
    repository_crm_links.go
    repository_crm_queue.go
    repository_crm_scope.go
    repository_crm_types.go
    repository_records_stats.go
    AGENT.md
  catalog/         — subpacote crm/catalog (busca de produtos)
    model.go
    errors.go
    service.go
    http.go
    repository_postgres.go
    source_registry.go
    AGENT.md
```

## Estado atual (C5 — 2026-05-29)

### C5 concluído

- `erp/` e `catalog/` movidos de `back/internal/modules/{erp,catalog}` para `back/internal/modules/crm/{erp,catalog}`.
- Package names mantidos iguais (`package erp`, `package catalog`).
- Import paths atualizados em `app.go` e `catalog_store_finder_adapter.go`.
- `crm/module.go` criado implementando `modules.Module`:
  - `ID() = "crm"`
  - 5 permissões declaradas (ver seção abaixo)
  - 2 role templates: `crm.manager` e `crm.analyst`
  - `Build()` retorna handle vazio — rotas ainda montadas pelo wiring legado em `app.go`
- `registry.MustRegister(crm.New())` adicionado em `app.go`.
- `queue/catalog_adapter.go` criado com interface `CatalogResolver` + `CatalogAdapter` (fallback local).
- `go build ./...` confirmado limpo.

### Wiring de rotas (legado) + gating ativo (C20, 2026-06-04)

Os endpoints HTTP de `erp` (`/v1/erp/*`) e `catalog` (`/v1/catalog/*`) continuam
registrados diretamente em `app.go`, MAS agora são **gateados por contratação**
via `AccountModulesGuard.RequireModuleByPath` (prefixo → `crm`) no `Chain`.
Desabilitar o módulo `crm` de uma account → `403 module_disabled` em `/v1/erp/*`
e `/v1/catalog/*`. O front envia `X-Account-Id` automaticamente (`createApiRequest`).
Ver [platform/app/AGENT.md](../../platform/app/AGENT.md).

## Permissões declaradas

| Key | Scope | Descrição |
|---|---|---|
| `crm.erp.sync` | account | Disparar sync manual FTP/CSV do ERP |
| `crm.erp.read` | account | Ver registros sincronizados do ERP (clientes, pedidos, itens) |
| `crm.dashboard.read` | account | Dashboard CRM 360 (vendas × fila) |
| `crm.catalog.read` | store | Buscar produtos no catálogo (ERP atual ou interno) |
| `crm.analytics.read` | account | Analytics de vendas por consultor/loja/período |

## Role templates declarados

| ID | Label | Permissões |
|---|---|---|
| `crm.manager` | Gerente CRM | Todas as 5 permissões |
| `crm.analyst` | Analista CRM | erp.read + dashboard.read + catalog.read + analytics.read |

## Import paths

Antes da reorganização (C5):
```go
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/erp"
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/catalog"
```

Após (correto):
```go
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/erp"
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/catalog"
```

## CatalogAdapter (queue/catalog_adapter.go)

A interface `CatalogResolver` vive em `queue/catalog_adapter.go`:

```go
type CatalogResolver interface {
    SearchProducts(ctx context.Context, accountID, storeID, term string, limit int) ([]ProductRef, error)
}
```

`ErrCRMNotEnabled` é retornado quando CRM não está habilitado para a account.
`CatalogAdapter.Search()` faz fallback automático para função local quando recebe `ErrCRMNotEnabled`.

## Notas de deploy

Nenhuma migration nova em C5. Apenas reorganização de pacotes Go.
Sem novas variáveis de ambiente ou alterações em Dockerfile.

## Quando atualizar este AGENT.md

- Quando rotas migrarem do wiring legado (app.go) para `RequireModule("crm")`.
- Quando `CatalogResolver` for efetivamente wired entre crm/catalog e queue/operations.
- Quando permissões ou role templates forem alterados.
- Quando novos subpacotes entrarem no crm/ (ex: crm/dashboard).
