# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/users`.

## Responsabilidade do modulo

O modulo `users` cuida da administracao de usuarios da plataforma dentro do modelo multitenant atual.

Hoje ele deve responder por:

- listar usuarios acessiveis
- criar usuario com papel, escopo e onboarding por convite
- criar usuario com senha inicial definida pelo admin quando esse for o fluxo desejado
- atualizar dados basicos, papel e escopo
- reenviar/gerar convite inicial quando aplicavel
- redefinir senha temporaria de forma administrativa
- inativar usuario

Ele nao deve cuidar de:

- login e emissao de token
- leitura operacional da fila
- configuracoes da loja

## Contrato atual

- `GET /v1/users`
- `POST /v1/users`
- `PATCH /v1/users/{id}`
- `POST /v1/users/{id}/invite`
- `POST /v1/users/{id}/reset-password`
- `POST /v1/users/{id}/archive`

## Fonte única de usuários — unificação 2026-06 (CONCLUÍDA)

Historicamente havia DUAS tabelas de usuário que divergiam:
- `public.users` (legado, 0001) + `consultants` → lida por este módulo.
- `core.users` (0100) → lida pelo admin global (`/manage/users`, módulo core).

**Unificação (core.users = fonte única) — FINALIZADA (itens 2&3, migration 0136):**
- Todo o Go (auth, consultants, users, bootstrap) lê/escreve `core.users` direto — `store_postgres.go` agora faz `from core.users`/`update core.users`/`insert into core.users`.
- A VIEW `public.users` + triggers `INSTEAD OF` foram **DROPADOS** (0136). Não existe mais camada de compat — fonte única é `core.users`.
- `0131_backfill_users_into_core.sql` consolidou o drift histórico antes do drop.
- Resultado: as duas telas (`/manage/users` e `/operacao/usuarios`) leem a MESMA verdade (cada uma com sua projeção), sem view legada no meio.

**U3 (2026-06-05): `/operacao/usuarios` le core.**
- Listagem (`GET /v1/users`) monta usuarios a partir de `core.account_users`, `core.users`, `core.user_role_assignments`, `core.roles` e `core.user_module_settings(module_id='queue')`.
- O papel coarse reutiliza o mapeamento exportado de `auth.CoarseRoleFromCoreRole` / `auth.CoreRoleCodesForCoarse`; nao duplicar esse mapeamento aqui.
- `storeIds` da Fila vem de `core.user_module_settings.config.storeIdsByAccount[accountId]`.
- `employee_code` e `job_title` continuam em `core.users`.
- `consultants` ainda fornece a indicacao `managedBy=consultants` / `managedResourceId` para contas vinculadas ao roster.
- Create/update ainda dual-gravam `user_tenant_roles`/`user_store_roles`/`user_platform_roles` por compatibilidade ate U4, mas tambem garantem `core.account_users`, `core.roles` compat, `core.user_role_assignments` e `core.user_module_settings`.

## Regras de escopo

- `platform_admin` pode administrar usuarios de qualquer tenant, inclusive outros `platform_admin`
- `owner` pode administrar usuarios do proprio tenant
- `owner` nao pode criar nem editar `platform_admin`
- `manager`, `consultant`, `marketing` e `director` nao administram usuarios
- `store_terminal` nao administra usuarios

## Regras de modelagem

- o sistema trabalha com um papel efetivo por usuario
- papeis efetivos da listagem usam `core.user_role_assignments` + `core.roles`
- papeis de tenant ainda sao dual-gravados em `user_tenant_roles` ate U4
- papeis de loja ainda sao dual-gravados em `user_store_roles` ate U4
  - `consultant`
  - `manager`
  - `store_terminal`
- `platform_admin` usa `core.users.is_platform_admin` e ainda dual-grava `user_platform_roles` ate U4
- mutacoes devem limpar atribuicoes antigas de compatibilidade e regravar apenas o escopo valido para o novo papel
- papeis de loja devem ficar vinculados a uma unica loja por usuario nesta fase
- criar usuario sem senha deve preferir convite, nao senha placeholder
- criar usuario com senha manual nao deve gerar convite
- definicao manual e reset administrativo de senha ficam restritos a `platform_admin`; `owner` segue no fluxo de convite
- criar usuario com senha manual deve marcar a conta com senha temporaria quando o papel for individual
- convite so deve ser gerado para usuario ativo e sem senha definida
- se o admin definir senha manualmente ou inativar a conta, convites pendentes devem ser revogados
- reset administrativo de senha deve marcar `must_change_password = true`, exceto para papeis de terminal fixo quando essa regra nao se aplicar
- o CRUD administrativo de usuarios deve viver em area propria do frontend, separado de `multiloja`
- autoedicao do proprio perfil nao pertence a este modulo; fica em `auth`
- consultores nao devem nascer por este modulo; o fluxo correto e `consultants`
- contas com papel `consultant` e vinculo de roster nao devem ser editadas, convidadas nem inativadas por este modulo
- para contas de consultor, este modulo pode apenas listar e executar reset administrativo de senha
- `platform_admin` pode usar override administrativo para manutencao/debug de contas `consultant`, inclusive mudanca de papel por PATCH quando isso for explicitamente necessario
- esse override administrativo nao cria roster; ele apenas altera o acesso do usuario e deixa o sincronismo do consultor vinculado agir quando houver `consultants.user_id`
