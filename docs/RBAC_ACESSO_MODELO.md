# Modelo de Acesso (RBAC multi-tenant) — Usuários, Perfis, Módulos e Páginas

Doc canônico do modelo de acesso confirmado pelo dono do produto (2026-06-18).
Espelhado no roadmap como fase `rbac-acesso` (RBAC). Trabalho casado com
`docs/USUARIOS_ACESSOS_FIX_PLAN.md` (a leva anterior, de senha/agência/salvar módulos).

## Modelo em 4 camadas

### Camada 1 — Módulo × Página (distinção obrigatória)
- **Módulo** = unidade de topo contratável: Fila, CRM, ERP, Tasks, Cardápio, Site,
  Meta Ads, Automação… + módulos **dev/internos**.
- **Página** = tela dentro de um módulo. Ex.: módulo **Fila** → Operação, Dados,
  Ranking, Relatórios, Usuários, Configurações. Hoje a lista (`WORKSPACE_ACCESS_DEFINITIONS`)
  mistura módulo e página num nível só — precisa virar **módulo → páginas** explícito.

### Camada 2 — O que a CONTA tem (teto)
- Cada cliente/agência tem um conjunto de módulos: `core.account_modules` (editável em
  manage/clientes-web, com `AccountModulesGuard` no back).
- **Crow (agência)** = todos os módulos **menos os de dev**. **Dev/platform_admin** = todos.
- Regra: usuário **nunca** acessa módulo que a conta dele não tem. Override só dev/admin.

### Camada 3 — Perfis customizáveis por conta
- Cada cliente/agência cria **seus próprios perfis** (ex.: "Financeiro", "Marketing",
  "Gerente"). Cada perfil libera/restringe no nível de **página** dentro dos módulos
  (um "Financeiro" pode não ver certas páginas, até dentro do próprio financeiro).
- Base existente: `core.roles` já é **por conta** (clonado de `core.role_templates`).
  Falta a UI de editar permissões por página por perfil por conta.

### Camada 4 — Usuário
- Recebe um perfil (dentro do que a conta tem) + **overrides individuais** (`access`
  user overrides) para limitar ainda mais.

## O que já existe (reaproveitar)
- `core.account_modules` + `AccountModulesGuard` = camada 2.
- `core.roles` por conta (clonados de template) = base da camada 3.
- Módulo `access`: `workspace.*` (view/edit) + overrides por usuário + matriz de papéis
  (`/v1/access/*`). platform_admin usa cross-account.
- `manage/users` (AdminUsersWorkspace): lista, inline edit, criar, senha, memberships (RO).

## Gaps (o que falta)
- Mapa **módulo → páginas** explícito (hoje flat).
- **Perfis custom por conta com permissão por página** (hoje perfis são globais/coarse).
- **Coerência usuário ⊆ `account_modules`** no back (com override dev/admin).
- **Drawer de edição por usuário** em manage/users (atribuir nível por vínculo +
  dar/remover módulo/página por usuário).

## Plano de fases

### Fase 1 — Drawer de edição em manage/users
Valor imediato com os papéis que já existem.

**Fase 1A — FEITA e testada ao vivo (2026-06-18):**
- Drawer abre por usuário (`AdminUserEditDrawer.vue`, botão de lápis na linha).
- Edita básico (nome/nick/email/ativo/platform admin) + senha.
- **Vínculos (cliente/agência) com o papel/nível, e troca o nível por vínculo.**
  - Back: `GET memberships` devolve `role` + `isAgency`; `PATCH /v1/admin/users/{id}/
    memberships/{accountId}` troca o papel (replace de `user_role_assignments`;
    owner/director/marketing; inválido → 400 `invalid_role`; não-membro → 404).
  - Testado: trocar owner→director e o login do usuário passa a resolver `director`.

**Fase 1B — PENDENTE (próxima etapa):**
- **Módulos/páginas por usuário** dentro do drawer: reaproveita o painel de overrides
  do `access` (`/v1/access/users/{id}`). Placeholder no drawer aponta isso hoje.
- O teto (`account_modules`) e a distinção módulo×página entram na Fase 2.

### Fase 2 — Estrutura módulo→páginas + coerência no back
- Definir o mapa `module_id → páginas/workspaces`.
- Enforcement: acesso do usuário ⊆ `account_modules` (override só dev/admin).
- UI esconde/desabilita o que a conta não tem.

### Fase 3 — Perfis customizáveis por conta
- Construtor de perfis por cliente/agência (permissão por página).
- Atribuição de perfil ao usuário, com overrides individuais por cima.

## Notas de Deploy
- Fase 1 muda `back/` (memberships + endpoint de papel) → exige
  `docker compose up -d --build api`. Sem migration nova prevista (usa tabelas core
  existentes: `account_users`, `user_role_assignments`, `roles`, `role_templates`).
