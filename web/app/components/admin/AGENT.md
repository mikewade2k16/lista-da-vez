# AGENT — web/app/components/admin

## Escopo

Admin GLOBAL de usuários e organizações (painel `/manage/*`), batendo na API real
`core` (`/v1/admin/users*`, `/v1/admin/organizations*`, `/v1/accounts/{id}/roles*`).
Para platform_admin **e** admins delegados (org admin / admin de cliente). Não confundir
com `/operacao/usuarios` (módulo Fila legado, `back/internal/modules/users` + `/v1/access/*`).

## Peças

- `AdminUsersWorkspace.vue` — tabela cross-account de usuários (`OmniDataTable`), filtros
  server-side, paginação. Coluna "Cliente" permite atribuir/mover cliente inline (inclui
  usuário SEM cliente — reusa `moveUserAccount`). Abre o drawer de edição pelo lápis.
- `AdminUserEditDrawer.vue` — drawer de edição em ABAS, montado sobre `OmniEntityDrawer`
  (`components/ui/` — TEMPLATE-CORE de modal: header fechar/expandir-toggle/popover de modo,
  resize no modo lado, modos lado/centro/fullscreen; ver `docs/frontend/MODAL_TEMPLATE.md`).
  Abas: **Dados · Vínculos · Papéis · Módulos · Páginas · Senha**. Cada aba é um painel em
  `admin/users/`. A aba Senha só aparece para platform_admin (identity-global).
- `admin/users/AdminUserDataPanel.vue` — identidade (nome/nick/email/ativo/platform admin);
  campos desabilitados quando `canEditIdentity=false` (só platform_admin edita identidade).
- `admin/users/AdminUserMembershipsPanel.vue` — adicionar/remover cliente; vincular/desvincular
  organização (com `orgRole` + confirmação obrigatória de acesso amplo de agência).
- `admin/users/AdminUserRolesPanel.vue` — papéis `core.roles` por ESCOPO (multi-select +
  `setUserRoles`); embute o `AdminRoleMatrixEditor`. O escopo inclui clientes E a conta-agência
  (`isAgency`, badge "Organização"), para gerenciar papéis de usuário só-organização (sem cliente).
- `admin/users/AdminRoleMatrixEditor.vue` — CRUD de papel custom + matriz de permissões
  (catálogo `available` reaproveitado do `getOverrides`).
- `admin/users/AdminUserModulesPanel.vue` — overrides por usuário por account
  (Herdar/Permitir/Negar por permissão), via `core.user_permission_overrides`. O seletor de escopo
  inclui clientes E a conta-agência (overrides de módulo também valem na conta-agência).
- `admin/users/AdminUserPagesPanel.vue` — visibilidade de PÁGINA por usuário (Herdar/Mostrar/Ocultar
  por workspace). ATENÇÃO: o gating de página do menu resolve do módulo LEGADO `access`
  (`auth.permissionKeys` ← `/v1/me/context`), então este painel grava nos overrides de access
  (store `access-control` → `PUT /v1/access/users/{id}/overrides`, chaves `workspace.*.view`), NÃO no
  core — só assim o menu muda de fato. Registrado em `docs/LEGADO.md` (item 5).
- `AdminOrganizationsWorkspace.vue` — CRUD de organizações (agências).
- `LegacyMarker.vue` — badge "LEGADO"/"MOCK" (só platform_admin).

## Camada de dados

- `useAdminUsersManager()` (+ auxiliar `useAdminUserLinks`) — CRUD de usuário, `fetchMemberships`,
  `updateMembershipRole`, `moveUserAccount`, `addMembership`/`removeMembership`,
  `linkOrganization`/`unlinkOrganization`, `getOverrides`/`setOverrides`.
- `useAccountRolesManager()` — `listRoles`, `getRole`, `createRole`, `updateRole`, `deleteRole`,
  `getUserRoles`, `setUserRoles`. Envia `X-Account-Id = accountId` explícito (escopo do recurso).
- `useClientsManager()`/`useAdminOrganizationsManager()` — listas de clientes/organizações.

## Delegação e gating (multi-tenant)

- `/v1/admin/users*` é DELEGADO: platform_admin OU agency_owner OU `core.users.manage` resolvido
  na account (validado NO BACKEND, 404/403 fora do escopo). O front gateia com
  `isPlatformAdmin || auth.permissionKeys.includes('core.users.manage')` (padrão `isPlatformAdmin || has`).
- **Identity-global** (criar/excluir usuário, senha, is_platform_admin, email, nome) → só
  platform_admin (o backend devolve 403 `forbidden_field` para ator delegado). Por isso o botão
  "Novo usuário"/excluir/aba Senha ficam restritos a platform_admin no front.

## Quando atualizar este AGENT.md

- Novo painel/aba no drawer, novo componente admin, ou mudança no contrato de
  `useAdminUsersManager`/`useAccountRolesManager`/endpoints `/v1/admin/*` ou `/v1/accounts/{id}/roles*`.
