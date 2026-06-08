# Changelog — Unificação de usuários + guard (2026-06-05, banco LOCAL)

Registro de TODAS as operações aplicadas ao banco local (`omni-postgres-1`, container, user `omni`) durante a unificação de usuários e ativação do guard. Inclui operações ad-hoc via `psql` que NÃO viraram migration versionada — listadas aqui para rastreabilidade e reprodução em outros ambientes.

> Backup feito antes de tudo: `C:\tmp\omni_backup_pre_unify.dump` (pg_dump custom, 260MB) + tabela `users_backup` (cópia de `public.users` pré-mudança). Rollback total: `pg_restore -d omni --clean --if-exists C:\tmp\omni_backup_pre_unify.dump`.

## Ordem aplicada

1. **Backup** — `pg_dump -Fc` + `create table users_backup as select * from public.users`.

2. **Migration `0131_backfill_users_into_core.sql`** (versionada, auto no boot, idempotente) — re-sincroniza `public.users → core.users` + memberships de `user_tenant_roles`, `user_store_roles` e **`consultants.tenant_id`** (esta fonte faltava e deixava 6 consultores da Pérola sem membership). Resultado: drift 7→0, `core.account_users` 29→36.

3. **API rebuild + `--force-recreate`** — o `up --build` sozinho NÃO recriou o container (rodava binário de 2 dias). Com `--force-recreate` subiu o binário novo: guard `RequireModuleByPath` ATIVO + `queue`/`crm` registrados no Registry (SyncCatalog popula `core.modules`).

4. **AD-HOC — reseed de `core.account_modules`** (não-versionado): 
   ```sql
   insert into core.account_modules (account_id, module_id, enabled, config)
   select a.id, m.id, true, '{}'::jsonb
   from core.accounts a cross join core.modules m
   where a.is_active = true and m.id in ('queue','tasks','crm')
   on conflict (account_id, module_id) do nothing;
   ```
   Antes só havia 2 linhas (`tasks`), porque queue/crm não estavam registrados quando a 0124 rodou. Depois: Pérola e Duby com `crm,queue,tasks`. (Idempotente; equivale a re-rodar a seed 0124.)

5. **Script manual `manual/unify_users_view.sql`** — `public.users` (tabela) → **VIEW** sobre `core.users` + triggers `INSTEAD OF` (insert/update/delete) + re-aponta 20 FKs para `core.users(id)`. Transacional/idempotente. Aplicado com a api parada.

6. **AD-HOC — restauração de senha/dados** (não-versionado, CORREÇÃO de regressão): o view-swap passou a servir o `password_hash` STALE do `core.users` (congelado desde o seed 0101) em vez do hash VIVO de `public.users` → admin trancado fora. Corrigido:
   ```sql
   update core.users c
   set password_hash=b.password_hash, display_name=b.display_name, nick=b.nick,
       avatar_path=b.avatar_path, must_change_password=b.must_change_password,
       is_active=b.is_active, updated_at=now()
   from users_backup b where b.id=c.id
     and (c.password_hash is distinct from b.password_hash or ...);
   ```
   3 usuários tinham hash divergente; restaurados do `users_backup`. **Lição registrada no AGENT_RULES.md (nunca sobrescrever senha sem permissão; comparar DADOS antes de view-swap).**

7. **AD-HOC — colunas faltantes `employee_code`/`job_title`** (CORREÇÃO de regressão; agora também no script manual): o view-swap esqueceu 2 colunas que `public.users` tinha e `core.users` não → query de consultores quebrava (500 "Erro ao processar o consultor"). Corrigido:
   ```sql
   alter table core.users add column if not exists employee_code text not null default '';
   alter table core.users add column if not exists job_title    text not null default '';
   update core.users c set employee_code=coalesce(b.employee_code,''), job_title=coalesce(b.job_title,'')
   from users_backup b where b.id=c.id;
   -- + recriar view e triggers incluindo as 2 colunas (ver unify_users_view.sql atualizado)
   ```

## Estado final validado (HTTP, ao vivo)
- Login `mikewade2k16@gmail.com` / `123123456` → 200. `/v1/me/context` 200, `/v1/users` 200, `/v1/consultants?storeId=...` 200.
- Guard: rota gateada sem `X-Account-Id` → 400; conta sem módulo → 403 `module_disabled`; com módulo → passa.
- `public.users` é VIEW; insert/update/delete via view caem em `core.users` (triggers validados).

## Correções pós-aplicação (mesma sessão) — regressões do view-swap + bugs pré-existentes

8. **Multiselect de módulos (front)** — `ClientsAdminWorkspace.moduleSelectOptions` usava lista hardcoded fake (core_panel, atendimento, indicators, finance, kanban) que não existem no backend → clicar não toggla. Trocado pelo catálogo real (`account.modules`). E **`useClientsManager.updateField`**: o campo `modules` passava por `patchLocal` (corrompia `account.modules` de objeto p/ string[]) → `persistModules` zerava o `currentEnabled` → diff de `disable` nunca disparava (adicionava mas não removia). Fix: `modules` não passa por `patchLocal`.

9. **`/v1/users` 500 (regressão do view-swap)** — insert legado faz `... returning created_at, updated_at`; a view não tem DEFAULT, e o trigger `INSTEAD OF` não setava `new.created_at`/`new.updated_at` no NEW → RETURNING NULL → Scan em `time.Time` falhava. Fix no trigger (setar no NEW). Aplicado ao vivo + no `unify_users_view.sql`.

10. **`/v1/admin/users` 500 (bug pré-existente)** — `AdminUserService.hasher` era nil porque `app.go` nunca passava `PasswordHasher` no `registry.Build(Dependencies{...})` (afirmação do C14 era falsa). Fix Go: `PasswordHasher: hasher`. Exigiu rebuild.

11. **Feature: vínculo de cliente/agência no manage/users** — `AdminCreateUserInput` ganhou `accountId`/`organizationId`; `admin_users_repository.CreateUser` cria membership em `core.account_users`/`core.organization_users`. Front: 2 selects no modal de criação (carregados de `/v1/admin/accounts` e `/v1/admin/organizations`). `/operacao/usuarios` não precisa (já está dentro de um cliente).

> Nota: o reseed de account_modules (item 4) e o `--force-recreate` foram necessários porque `docker compose up --build api` NÃO recria o container sozinho (rodava binário antigo). Sempre usar `--force-recreate` ao rebuildar a api localmente.

12. **Divergência de PAPÉIS (não só identidade) — manage-create não logava** — a identidade foi unificada (users→view sobre core.users), mas o modelo de PAPEL continua DOIS: legado (`user_tenant_roles`/`user_store_roles`/`user_platform_roles`, usado pelo **auth login** e por `/operacao/usuarios`) vs core (`core.account_users`, usado por manage + módulos). Um user criado no manage tinha só membership core, **sem papel legado** → o `Login` (auth resolve papel pelo legado) retornava erro → **500 "Erro ao autenticar."**; e não aparecia na operação. Fix: `AdminCreateUserInput` ganhou `role` (owner/director/marketing); `admin_users_repository.CreateUser` agora cria **ambos** — `core.account_users` (módulos) E `user_tenant_roles` (papel legado, `accountId == tenantId`). Front: select de Papel no modal (quando há cliente). Validado: criar com cliente+papel → core_membership + tenant_role → **login 200**. (filipe@perola.com existente foi corrigido à mão: `insert user_tenant_roles owner`.)

### Pendências conhecidas desta área (não bloqueiam, mas avisar)
- **Direção reversa:** `/operacao/usuarios` (users module) cria papel legado mas NÃO cria `core.account_users` → user criado lá aparece na operação/loga, mas `/manage/users` mostra `accountCount=0`. Para consistência total, o users module deveria também criar membership core (fase futura).
- **Auth frágil p/ user sem papel:** `LoadUserForAuth`/`Login` quebra (500) para user em core.users SEM nenhum papel legado, em vez de erro limpo. Com o fix acima isso não acontece no fluxo normal (todo manage-user ganha papel), mas é um hardening pendente.
- **Unificação real do modelo de papel** (auth ler de core em vez de legado) continua sendo a dívida arquitetural maior — hoje mantemos os dois em sync na criação.

## Pendências de reprodutibilidade
- As operações **4, 6, 7** foram ad-hoc. Para outros ambientes: (4) re-rodar a seed 0124; (5+7) rodar o `unify_users_view.sql` ATUALIZADO (já inclui as 2 colunas); (6) só é necessário se houver divergência de senha core×public — comparar antes.
