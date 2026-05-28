# Modulo `roadmap`

Inventario editavel de modulos/paginas pendentes e regras canonicas para agentes.

Plugado no Module Registry quando `CORE_V2_ENABLED=true`. Schema `roadmap.*` criado em [0115_roadmap_schema.sql](../../platform/database/migrations/0115_roadmap_schema.sql).

## Modelos

### `Module`
Linha em `roadmap.modules`. Representa um modulo/pagina do produto (Tasks, Tracking, Omnichannel, etc.).

Campos relevantes:
- `source_id` — identificador estavel (`tasks`, `editor`, `tracking`...). Usado pra fazer override por account.
- `account_id` — `NULL` para registros globais (seed); preenchido quando uma account customiza.
- `status` — `pending` / `in_progress` / `beta` / `done`.
- `priority` — `P0` / `P1` / `P2` / `P3`.
- `scope` e `depends_on` — arrays JSON.

### `Rule`
Linha em `roadmap.rules`. Regra canonica que agentes devem seguir.

Campos relevantes:
- `category` — `frontend` / `backend` / `banco` / `linguagens` / `deploy` / `padroes-gerais`.
- `body` — descricao principal.
- `why` — motivacao.
- `applies_when` — quando aplicar.

## Estrategia de override

Registros com `account_id IS NULL` sao globais (seed compartilhado). Quando uma account edita um registro global, o servico cria um override (mesmo `source_id`, `account_id` setado). A listagem dedupa por `source_id` preferindo o override.

`DELETE` em registros globais retorna 403 `cannot_delete_global` — globais so podem ser editados (vira override).

## Permissoes

- `roadmap.view` — listar modulos/regras e exportar markdown
- `roadmap.manage` — criar/editar/apagar overrides da account

Role templates:
- `roadmap.viewer` — so `roadmap.view`
- `roadmap.admin` — `view` + `manage`

## Endpoints

| Metodo | Rota | Permissao | Descricao |
|---|---|---|---|
| GET | `/v1/roadmap/modules` | `roadmap.view` | Lista modulos visiveis pra account (globais + overrides) |
| POST | `/v1/roadmap/modules` | `roadmap.manage` | Upsert por `source_id` na account |
| PUT | `/v1/roadmap/modules/{id}` | `roadmap.manage` | Atualiza (cria override se for global) |
| DELETE | `/v1/roadmap/modules/{id}` | `roadmap.manage` | Apaga override (403 se for global) |
| GET | `/v1/roadmap/rules` | `roadmap.view` | Lista regras |
| POST | `/v1/roadmap/rules` | `roadmap.manage` | Upsert |
| PUT | `/v1/roadmap/rules/{id}` | `roadmap.manage` | Atualiza |
| DELETE | `/v1/roadmap/rules/{id}` | `roadmap.manage` | Apaga override |
| GET | `/v1/roadmap/rules.md` | `roadmap.view` | Exporta AGENT_RULES.md agregado |

## Layout

```
roadmap/
  AGENT.md
  errors.go              -- ErrForbidden, ErrInvalid, ErrNotFound, ErrCannotDeleteGlobal
  http.go                -- Handlers REST + register
  model.go               -- Tipos Module/Rule + AccessContext + constantes
  module.go              -- Implementa modules.Module (Registry)
  repository_postgres.go -- Repository (account_users, permissions, modules, rules)
  service.go             -- Regras (validar, override, dedup, exportar markdown)
```

## Notas de implementacao

- Listagem usa `select distinct on (source_id)` com `order by source_id, (account_id is not null) desc` para preferir override sobre global.
- Scope e dependsOn em JSONB; service normaliza arrays vazios pra `[]` (nunca `null`).
- Markdown gerado em [service.go:BuildMarkdown](service.go) — mesma estrutura do AGENT_RULES.md na raiz; ordem das categorias e fixa.
- Quando `roadmap.modules` ganhar nova categoria/status, atualizar:
  - constraint `check` na migration
  - constantes em [model.go](model.go)
  - validadores em [service.go](service.go)
