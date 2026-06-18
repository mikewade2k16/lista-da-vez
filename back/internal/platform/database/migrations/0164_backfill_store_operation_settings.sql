-- 0164 — Backfill de store_operation_settings para lojas sem linha
--
-- Contexto: GET /v1/analytics/data varre todas as lojas do tenant e chama
-- LoadSettings por loja. Loja sem linha em store_operation_settings fazia o
-- QueryRow().Scan() retornar ErrNoRows -> 500 no endpoint inteiro. O codigo Go
-- ja passou a tratar ErrNoRows como defaults; este backfill complementa,
-- garantindo que toda loja tenha settings persistidos e editaveis na tela.
--
-- ADITIVO E IDEMPOTENTE: insere apenas o store_id faltante. Todas as demais
-- colunas assumem os DEFAULTs do schema (todas NOT NULL DEFAULT). Nunca atualiza
-- nem remove linhas existentes -> zero risco a settings ja configurados.

insert into public.store_operation_settings (store_id)
select s.id
from public.stores s
where not exists (
  select 1
  from public.store_operation_settings o
  where o.store_id = s.id
);
