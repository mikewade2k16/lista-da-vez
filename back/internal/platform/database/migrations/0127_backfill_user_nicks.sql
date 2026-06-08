-- 0127 — Backfill de core.users.nick a partir de display_name.
--
-- Motivacao: a tela /usuarios (modulo Fila, queue) sempre gerou nicks via
-- `buildNickname(displayName)` no front. Esses nicks nunca foram persistidos em
-- core.users.nick — quando uma tela admin nova (/manage/users) le diretamente
-- do banco, todos aparecem vazios. C14 padronizou o auto-nick em
-- AdminUserService.CreateUser; esta migration popula os users existentes.
--
-- Regra (espelha core.BuildNickname e domain/utils/person-display.ts):
--   primeiro_nome + ' ' + UPPER(primeira_letra_segundo_token) + '.'
--   se so houver primeiro_nome, mantem ele puro.
--   se result > 18 chars, encurta first com '...' (raro — ignorado aqui;
--   o trim natural do display_name ja resolve nos casos reais).
--
-- Idempotente: only updates rows where nick = '' or nick is null. Rodar de novo
-- nao altera quem ja tem nick definido (UI manual prevalece).

update core.users
set nick = case
    -- Sem displayName → mantem vazio.
    when coalesce(nullif(trim(display_name), ''), '') = '' then ''
    -- So um token → primeiro nome puro.
    when array_length(string_to_array(trim(display_name), ' '), 1) = 1
        then split_part(trim(display_name), ' ', 1)
    -- 2+ tokens → "Primeiro X." (onde X = inicial maiuscula do segundo).
    else split_part(trim(display_name), ' ', 1)
        || ' '
        || upper(substring(split_part(trim(display_name), ' ', 2), 1, 1))
        || '.'
    end,
    updated_at = now()
where coalesce(nick, '') = '';

-- Espelhar no users legado (public.users) caso ainda exista (alguns
-- ambientes mantem ate Fase 4).
do $$
begin
    if exists (
        select 1 from information_schema.columns
        where table_schema = 'public' and table_name = 'users' and column_name = 'nick'
    ) then
        update public.users
        set nick = case
            when coalesce(nullif(trim(display_name), ''), '') = '' then ''
            when array_length(string_to_array(trim(display_name), ' '), 1) = 1
                then split_part(trim(display_name), ' ', 1)
            else split_part(trim(display_name), ' ', 1)
                || ' '
                || upper(substring(split_part(trim(display_name), ' ', 2), 1, 1))
                || '.'
            end
        where coalesce(nick, '') = '';
    end if;
end $$;
