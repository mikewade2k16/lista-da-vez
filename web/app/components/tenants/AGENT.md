# AGENT — web/app/components/tenants

## Escopo

Admin global de **accounts** (`core.accounts`, via `/v1/admin/accounts`). Para
platform_admin. Não confundir com o gestor legado de `public.tenants`
(`TenantsWorkspace.vue`, módulo Fila).

## Peças

- `ClientsAdminWorkspace.vue` — workspace principal. Duas visões alternáveis:
  **tabela** (`OmniDataTable`, edição inline célula a célula) e **board** (cards).
  CRUD via `useClientsManager()`.
- `account-fields.ts` — **fonte única** da definição de campos do account
  (`ACCOUNT_FIELDS`, `ACCOUNT_FIELD_GROUPS`, `accountCardFields()`). Modal de
  detalhe e board card consomem a mesma lista.
- `AccountDetailModal.vue` — modal de detalhe/edição, agrupado por
  `ACCOUNT_FIELD_GROUPS`. Editáveis emitem `update-field` → `updateField`.
- `AccountBoardCard.vue` — card compacto do board, exibe `accountCardFields()`.
  `@open` abre o detail modal (botão ou duplo clique).
- `AccountCreateModal.vue` — form de criar conta (name, slug, planCode,
  adminEmail) → `POST /v1/admin/accounts` via `createClient(payload)`.

## Regra do espelho (ENGINEERING_PRINCIPLES §4)

**Modal de detalhe e board card são espelhados.** Isso é garantido por construção:
ambos derivam de `account-fields.ts`. Para mudar o que aparece, edite a definição
em um lugar só — os dois refletem. NÃO duplicar lista de campos em componente.

## Contrato

Todos os campos editáveis existem em `core.accounts` e persistem via
`PATCH /v1/admin/accounts/:id` (mapeados em `useClientsManager.FIELD_TO_PATCH`).
`status` → `active` (bool). `modules` usa endpoint dedicado `/modules` (diff
enable/disable). Agregados (`userCount`, `projectCount`, ...) são read-only.

## Quando atualizar este AGENT.md

- Adicionar/remover campo em `account-fields.ts`.
- Novo componente de account ou nova visão no workspace.
- Mudança no contrato de `useClientsManager` / endpoints admin de account.
