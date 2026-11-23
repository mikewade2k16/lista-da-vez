-- 0328 — Id estável por linha de roteiro.
-- A edição passou a ser por célula (PATCH + trava de presença por célula). Índice de linha
-- muda quando alguém insere/remove/move linhas; o id não. Este backfill só ACRESCENTA a
-- chave "id" (uuid) nas linhas que ainda não têm; conteúdo e ordem ficam intactos.
-- Idempotente: roteiros cujas linhas já têm id não são tocados.
update tools.scripts s
set rows = (
    select coalesce(jsonb_agg(
        case when x.r ? 'id' then x.r else x.r || jsonb_build_object('id', gen_random_uuid()::text) end
        order by x.ord), '[]'::jsonb)
    from jsonb_array_elements(s.rows) with ordinality as x(r, ord)
)
where exists (
    select 1 from jsonb_array_elements(s.rows) as e(r) where not e.r ? 'id'
);
