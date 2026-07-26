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
  É o **host** do manager: chama `provideAdminUsersContext()` UMA vez (provide/inject), e o
  drawer + panels descendentes compartilham a MESMA instância (estado/ações unificados).
  Host fino: criação/senha/ações de linha foram fatiadas em subcomponentes (abaixo).
- `admin/users/AdminUserCreateDialog.vue` — modal de criação (form/validação/submit isolados);
  recebe `accountOptions`/`organizationOptions` por prop, cria via manager compartilhado, emite `created`.
- `admin/users/AdminUserPasswordDialog.vue` — modal de definir/resetar senha (só platform_admin).
- `admin/users/AdminUsersActionsCell.vue` — célula de ações da linha (popover de memberships +
  popover de detalhes + botões editar/senha/excluir); estado dos popovers é LOCAL da célula.
- `AdminUserEditDrawer.vue` — drawer de edição em ABAS, montado sobre `OmniEntityDrawer`
  (`components/ui/` — TEMPLATE-CORE de modal: header fechar/expandir-toggle/popover de modo,
  resize no modo lado, modos lado/centro/fullscreen; ver `docs/frontend/MODAL_TEMPLATE.md`).
  Abre mais largo que o default global (≈960px) via `v-model:width` no nível do drawer — NÃO
  altera o `SIDE_DEFAULT_WIDTH` (720) global do `OmniEntityDrawer`, que outros drawers usam; o
  drawer continua redimensionável (handle) e respeita o `SIDE_MAX_CAP` (1120).
  Abas: **Dados · Vínculos · Papéis · Módulos · Páginas · Senha**. Cada aba é um painel em
  `admin/users/`. A aba Senha só aparece para platform_admin (identity-global).
- `admin/users/AdminUserDataPanel.vue` — identidade (nome/nick/email/ativo/platform admin);
  campos desabilitados quando `canEditIdentity=false` (só platform_admin edita identidade).
- `admin/users/AdminUserMembershipsPanel.vue` — adicionar/remover cliente; vincular/desvincular
  organização (com `orgRole` + confirmação obrigatória de acesso amplo de agência).
- `admin/users/AdminUserRolesPanel.vue` — papéis `core.roles` por ESCOPO (multi-select +
  `setUserRoles`); embute o `AdminRoleMatrixEditor`. O escopo inclui clientes E a conta-agência
  (`isAgency`, badge "Organização"), para gerenciar papéis de usuário só-organização (sem cliente).
  Cada escopo é um bloco colapsável padronizado no `OmniCollapse` (não mais markup próprio): título =
  nome da conta, resumo = "X de Y papéis", slot `actions` = badge "Organização" + "pendente".
- `admin/users/AdminRoleMatrixEditor.vue` — CRUD de papel custom + matriz de permissões
  (catálogo `available` reaproveitado do `getOverrides`). Concentra o ESTADO (seleção, rascunho
  de edição, persistência); a apresentação foi fatiada em `AdminRoleCreateForm.vue` (form de
  criação) e `AdminRolePermissionMatrix.vue` (grade de checkboxes agrupada por módulo).
- `admin/users/AdminUserModulesPanel.vue` — overrides por usuário por account
  (Herdar/Permitir/Negar por permissão), via `core.user_permission_overrides`. O seletor de escopo
  inclui clientes E a conta-agência (overrides de módulo também valem na conta-agência). Componente
  FINO: o estado de edição (rascunho + snapshot salvo) e o de VIEW (busca/filtro/lote) ficam no
  composable `useModuleOverridesEditor`; o painel só carrega/salva pela API e renderiza. Permissões
  agrupadas por `moduleId` em blocos colapsáveis (`OmniCollapse`, colapsados por padrão; abrem se há
  busca/filtro ativo). Toolbar: busca (`AppSearchInput`, debounce, filtra label OU key tipo
  `tasks.boards.manage`) + filtro de efeito (`AppSegmentedFilter`, sentinela `'all'`) + filtro por
  módulo (select com sentinela `'all'`). Ações em lote são POR MÓDULO (`AdminModuleGroupActions` no
  TOPO DO CORPO do collapse, acima da lista — só aparece com o módulo expandido; o header tem só o
  nome): Permitir/Negar/Herdar agem só nas permissões VISÍVEIS daquele módulo e "Restaurar" reverte
  as pendências daquele módulo ao último salvo. Há um "Restaurar tudo" global
  discreto no rodapé. Busca/filtro/lote são client-side puro: NÃO mudam o contrato de salvar (PUT
  `.../overrides` = replace do tri-estado); fonte de verdade = retorno do backend (re-hidrata após
  salvar). Pendências marcadas visualmente (badge "pendente" + borda na linha).
- `admin/users/AdminUserPagesPanel.vue` — visibilidade de PÁGINA por usuário (Herdar/Mostrar/Ocultar
  por workspace). ATENÇÃO: o gating de página do menu resolve do módulo LEGADO `access`
  (`auth.permissionKeys` ← `/v1/me/context`), então este painel grava nos overrides de access
  (store `access-control` → `PUT /v1/access/users/{id}/overrides`, chaves `workspace.*.view`), NÃO no
  core — só assim o menu muda de fato. Registrado em `docs/LEGADO.md` (item 5). Tem busca
  (`AppSearchInput`) + filtro de efeito (`AppSegmentedFilter`, sentinela `'all'`); as páginas
  visíveis são agrupadas em seções colapsáveis (`OmniCollapse`) por situação ("Com override" vs
  "Herdando o papel") via o composable `usePageOverridesView`. Estado de view client-side; não muda
  o contrato de salvar.
- `admin/users/AdminTriStateControl.vue` — controle tri-estado compartilhado (Herdar/allow/deny;
  rótulos configuráveis — Mostrar/Ocultar nas páginas) reusado por Módulos e Páginas.
- `admin/users/AdminModuleGroupActions.vue` — ações em lote POR MÓDULO (Permitir/Negar/Herdar/
  Restaurar) no header do collapse de cada módulo. Só altera rascunho/dirty (não auto-salva).
- `AdminOrganizationsWorkspace.vue` — CRUD de organizações (agências).
- `role-templates/AdminRoleTemplatesWorkspace.vue` — área de **papéis-padrão** (role templates):
  catálogo GLOBAL de templates que as contas novas clonam (`/manage/role-templates`, página
  `pages/manage/role-templates.vue`, workspace `role_templates_admin`). **SOMENTE platform_admin**
  (gate de rota/menu `agencyOnly` + `workspaceId` + reforço `auth.role === 'platform_admin'` no host;
  fail-closed: sem fetch para não-admin). Host concentra o ESTADO (seleção/abertura); lista separa
  Customizados (editáveis/removíveis) de Sistema (`isSystem||isLocked` → read-only com cadeado).
- `role-templates/AdminRoleTemplateCreateForm.vue` — form de criação (id slug sugerido do label via
  `slugify` enquanto não tocado; charset `[a-z0-9._-]`), label, descrição + matriz. Emite `submit`.
- `role-templates/AdminRoleTemplateListItem.vue` — uma linha: cabeçalho + editor colapsável. Custom:
  edita label/descrição (PATCH) e matriz (PUT) + remove (DELETE, com confirmação). Sistema: read-only.
  Rascunho re-hidrata do template autoritativo a cada abertura/mudança (fonte = backend).
- `role-templates/AdminRoleTemplateMatrix.vue` — matriz BINÁRIA (on/off) com toolbar de busca
  (`AppSearchInput`) + filtro por módulo (`AppSegmentedFilter`, sentinela `'all'`); reusa
  `admin/users/AdminRolePermissionMatrix.vue` na grade. Em readonly mostra só o resumo das permissões.
- `LegacyMarker.vue` — badge "LEGADO"/"MOCK" (só platform_admin).
- `ExperimentalFeaturesWorkspace.vue` — painel global exclusivo de
  `platform_admin` em `/manage/experimental-features`. Usa
  `usePlatformFeaturesStore`, carrega `GET /v1/platform/experimental-features`
  antes de habilitar qualquer switch e reidrata toda escrita pelo retorno
  autoritativo do `PUT`. O primeiro rollout é
  `attendanceAudioRecording`; o toggle não inicia captura de microfone neste
  bloco.

## Camada de dados

- `useAdminUsersManager()` (+ auxiliar `useAdminUserLinks`) — CRUD de usuário, `fetchMemberships`,
  `updateMembershipRole`, `moveUserAccount`, `addMembership`/`removeMembership`,
  `linkOrganization`/`unlinkOrganization`, `getOverrides`/`setOverrides`. **Unificado via
  provide/inject**: `useAdminUsersManager()` resolve a instância compartilhada provida pelo host
  (`provideAdminUsersContext()`); sem contexto, cai num fallback que cria instância local (compat).
- `useAccountRolesManager()` — `listRoles`, `getRole`, `createRole`, `updateRole`, `deleteRole`,
  `getUserRoles`, `setUserRoles`. Envia `X-Account-Id = accountId` explícito (escopo do recurso).
- `useAdminRoleTemplatesManager()` — catálogo GLOBAL de papéis-padrão via `/v1/admin/role-templates`:
  `fetchTemplates` (GET `{ templates, available }`), `createTemplate` (POST), `updateTemplate`
  (PATCH metadados), `updatePermissions` (PUT `.../permissions`, substitui a matriz), `deleteTemplate`
  (DELETE) + `isSaving(id, 'meta'|'perms'|'delete')`. Rota platform-global (sem `X-Account-Id` próprio).
  Fonte de verdade = resposta do backend (re-lê via `fetchTemplates` após cada escrita). Tipos em
  `web/types/admin-role-templates.ts` (reusa `AvailablePermission`).
- `useClientsManager()`/`useAdminOrganizationsManager()` — listas de clientes/organizações.
- `useInlineEditManager()` — mecânica COMPARTILHADA de edição inline (`savingMap`/`setSaving`/
  `rowIsSaving` + debounce `schedulePatch`/`cancelPatch` + cleanup de timers no unmount), reusada
  pelos 6 managers de grade. Cada manager segue dono do seu `applyPatch`/`patchLocal`/`persistPatch`
  (lista/endpoint próprios) e só delega o saving-map + a agenda de debounce.

## Delegação e gating (multi-tenant)

- `/v1/admin/users*` é DELEGADO: platform_admin OU agency_owner OU `core.users.manage` resolvido
  na account (validado NO BACKEND, 404/403 fora do escopo). O front gateia com
  `isPlatformAdmin || auth.permissionKeys.includes('core.users.manage')` (padrão `isPlatformAdmin || has`).
- **Identity-global** (criar/excluir usuário, senha, is_platform_admin, email, nome) → só
  platform_admin (o backend devolve 403 `forbidden_field` para ator delegado). Por isso o botão
  "Novo usuário"/excluir/aba Senha ficam restritos a platform_admin no front.
- **Papéis-padrão (`/v1/admin/role-templates*`)** → SOMENTE platform_admin (catálogo GLOBAL, não
  por-conta). Wiring de gate em 4 lugares (espelhar ao mover/renomear): workspace
  `role_templates_admin` em `app/utils/workspaces.ts` + `WORKSPACE_ACCESS_DEFINITIONS` e
  `ROLE_WORKSPACES.platform_admin` em `app/domain/utils/permissions.ts`; item de menu
  `manage-role-templates` (`agencyOnly: true` + `workspaceId`) em `layers/queue/nav.config.ts`; path
  `/manage/role-templates` em `AGENCY_ONLY_PATHS` de `app/middleware/module-enabled.global.ts`.
  Templates `isSystem=true` (ou `isLocked`) são read-only no front (o backend também bloqueia
  PATCH/PUT/DELETE).

## Quando atualizar este AGENT.md

- Novo painel/aba no drawer, novo componente admin, ou mudança no contrato de
  `useAdminUsersManager`/`useAccountRolesManager`/`useAdminRoleTemplatesManager`/endpoints
  `/v1/admin/*` ou `/v1/accounts/{id}/roles*`.
- Mudança no contrato ou na navegação de recursos experimentais globais.
