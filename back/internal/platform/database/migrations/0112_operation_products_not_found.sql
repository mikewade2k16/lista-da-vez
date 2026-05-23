-- Adiciona campo products_not_found_json para registrar produtos que o cliente
-- queria mas a loja não tinha — separado de products_seen_json (produto visto/experimentado).
-- A view public.operation_service_history replica automaticamente por ser SELECT *.
alter table queue.operation_service_history
    add column if not exists products_not_found_json jsonb not null default '[]'::jsonb;
