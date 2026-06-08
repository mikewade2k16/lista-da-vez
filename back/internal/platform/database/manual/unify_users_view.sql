-- ============================================================================
-- MANUAL — NAO roda no boot (fica fora de migrations/). Aplicar deliberadamente.
-- ============================================================================
-- Converte public.users (TABELA) em VIEW sobre core.users (fonte unica de
-- verdade de identidade). Escritas legadas (insert/update/delete em "users")
-- passam a cair em core.users via triggers INSTEAD OF, sem reescrever o codigo
-- Go legado. As ~20 FKs que apontavam para users(id) sao re-apontadas para
-- core.users(id) dinamicamente (mesmos ids, seguro apos o backfill da 0131).
--
-- PRE-REQUISITOS (em ordem):
--   1. Migration 0131 aplicada (core.users + memberships backfilled).
--   2. BACKUP feito (ver bloco abaixo). Esta operacao DROPa a tabela users —
--      sem backup, sem volta facil.
--   3. Banco certo: :5433 (omni-postgres-1). NUNCA :5432 (postgres nativo).
--
-- BACKUP (rodar ANTES, no shell — nao e SQL):
--   pg_dump "postgres://omni:omni_dev@localhost:5433/omni?sslmode=disable" \
--     -Fc -f omni_backup_pre_users_view.dump
--   # rede de seguranca extra dentro do banco:
--   psql "$DB" -c "create table if not exists users_backup as select * from users;"
--
-- APLICAR:
--   psql "postgres://omni:omni_dev@localhost:5433/omni?sslmode=disable" \
--     -v ON_ERROR_STOP=1 -f back/internal/platform/database/manual/unify_users_view.sql
--
-- Idempotente: se public.users ja for VIEW, o bloco principal e pulado.
-- ============================================================================

begin;

do $$
declare
  r record;
begin
  -- Idempotencia: so age se public.users ainda e TABELA base.
  if not exists (
    select 1 from information_schema.tables
    where table_schema = 'public' and table_name = 'users' and table_type = 'BASE TABLE'
  ) then
    raise notice 'public.users ja nao e tabela base — pulando a conversao.';
    return;
  end if;

  -- 1. Garante que todo users.id existe em core.users (pre-req das FKs).
  insert into core.users (
      id, email, display_name, password_hash, must_change_password,
      avatar_path, is_platform_admin, is_active, nick, created_at, updated_at
  )
  select
      u.id, u.email, u.display_name, u.password_hash,
      coalesce(u.must_change_password, false), coalesce(u.avatar_path, ''),
      exists (select 1 from public.user_platform_roles upr where upr.user_id = u.id),
      u.is_active, coalesce(u.nick, ''), u.created_at, u.updated_at
  from public.users u
  on conflict (id) do nothing;

  -- 2. Re-aponta TODA FK que referencia public.users -> core.users.
  --    Preserva colunas e ON DELETE/UPDATE via replace na definicao.
  for r in
    select con.conname,
           con.conrelid::regclass::text as tbl,
           pg_get_constraintdef(con.oid)  as def
    from pg_constraint con
    join pg_class     ref on ref.oid = con.confrelid
    join pg_namespace n   on n.oid   = ref.relnamespace
    where con.contype = 'f' and ref.relname = 'users' and n.nspname = 'public'
  loop
    execute format('alter table %s drop constraint %I', r.tbl, r.conname);
    execute format(
      'alter table %s add constraint %I %s',
      r.tbl, r.conname,
      replace(
        replace(r.def, 'REFERENCES users(',        'REFERENCES core.users('),
                       'REFERENCES public.users(',  'REFERENCES core.users('
      )
    );
    raise notice 'FK re-apontada: %.% -> core.users', r.tbl, r.conname;
  end loop;

  -- 3. Reconcilia colunas: core.users PRECISA ter TODA coluna que public.users
  --    tinha, senao a view some colunas e queries legadas quebram (500).
  --    employee_code/job_title existiam so em public.users (bootstrap/consultores).
  alter table core.users add column if not exists employee_code text not null default '';
  alter table core.users add column if not exists job_title    text not null default '';
  update core.users c
  set employee_code = coalesce(b.employee_code, ''), job_title = coalesce(b.job_title, '')
  from public.users b where b.id = c.id;

  -- 4. Remove a tabela (indices proprios vao junto). FKs ja re-apontadas.
  drop table public.users;
  raise notice 'public.users (tabela) removida.';
end $$;

-- 5. VIEW public.users sobre core.users — TODAS as colunas historicas.
create or replace view public.users as
select id, email, display_name, password_hash, is_active,
       created_at, updated_at, avatar_path, must_change_password, nick,
       employee_code, job_title
from core.users;

-- 5. INSTEAD OF triggers: escritas legadas em "users" caem em core.users.
create or replace function public.users_view_insert() returns trigger
language plpgsql as $fn$
begin
  if new.id is null then
    new.id := gen_random_uuid();
  end if;
  -- View nao tem DEFAULT: setar no NEW (nao so no insert) para que o RETURNING
  -- created_at/updated_at devolva valor e nao NULL. Sem isso, inserts legados
  -- com `... returning created_at` (ex.: users module) quebram no Scan (time.Time).
  if new.created_at is null then new.created_at := now(); end if;
  if new.updated_at is null then new.updated_at := now(); end if;
  insert into core.users (
      id, email, display_name, password_hash, is_active,
      avatar_path, must_change_password, nick, employee_code, job_title,
      created_at, updated_at
  ) values (
      new.id, new.email, new.display_name, new.password_hash,
      coalesce(new.is_active, true), coalesce(new.avatar_path, ''),
      coalesce(new.must_change_password, false), coalesce(new.nick, ''),
      coalesce(new.employee_code, ''), coalesce(new.job_title, ''),
      new.created_at, new.updated_at
  );
  return new;
end $fn$;

create or replace function public.users_view_update() returns trigger
language plpgsql as $fn$
begin
  update core.users set
    email                = new.email,
    display_name         = new.display_name,
    password_hash        = new.password_hash,
    is_active            = new.is_active,
    avatar_path          = coalesce(new.avatar_path, ''),
    must_change_password = coalesce(new.must_change_password, false),
    nick                 = coalesce(new.nick, ''),
    employee_code        = coalesce(new.employee_code, ''),
    job_title            = coalesce(new.job_title, ''),
    updated_at           = now()
  where id = old.id;
  return new;
end $fn$;

create or replace function public.users_view_delete() returns trigger
language plpgsql as $fn$
begin
  delete from core.users where id = old.id;
  return old;
end $fn$;

drop trigger if exists users_view_insert_trg on public.users;
drop trigger if exists users_view_update_trg on public.users;
drop trigger if exists users_view_delete_trg on public.users;

create trigger users_view_insert_trg instead of insert on public.users
  for each row execute function public.users_view_insert();
create trigger users_view_update_trg instead of update on public.users
  for each row execute function public.users_view_update();
create trigger users_view_delete_trg instead of delete on public.users
  for each row execute function public.users_view_delete();

commit;

-- ============================================================================
-- VALIDACAO (rodar apos aplicar):
--   -- public.users agora e VIEW:
--   select table_type from information_schema.tables
--   where table_schema='public' and table_name='users';        -- VIEW
--   -- contagem bate com core.users:
--   select (select count(*) from public.users) as via_view,
--          (select count(*) from core.users)   as core;        -- iguais
--   -- nenhuma FK ainda aponta para a tabela antiga (todas em core.users):
--   select conrelid::regclass, conname from pg_constraint
--   where contype='f' and confrelid = 'core.users'::regclass;  -- ~20 linhas
-- ROLLBACK: restaurar do dump (pg_restore) — a conversao nao tem undo trivial.
-- ============================================================================
