-- T7.1 — identidade curta (nick) para usuarios.
-- Motivacao: presence/selects/avatares de tasks mostram display_name completo, o que fica
-- ambiguo quando dois logins tem nomes parecidos ("Mike Wade" / "Mike Wade Demo"). Nick e
-- opcional; fallback para display_name quando vazio (`coalesce(nullif(nick,''), display_name)`).
-- Backfill manual via SQL ate a UI de edicao chegar.

alter table users
    add column if not exists nick text not null default '';

alter table core.users
    add column if not exists nick text not null default '';
