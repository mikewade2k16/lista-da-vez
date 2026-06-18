-- Comissao por atingimento de meta v2 (Trilha A)
-- Plano: C:/Users/Mike/.claude/plans/vamos-fazer-altera-es-em-purrfect-pony.md
--
-- store_type (Shopping/Bairro) e atributo de 1a classe da loja (CHECK), usado
-- pelo calculo de comissao do gerente (faixas managerShopping vs managerBairro).
-- Default 'bairro'. Recria a view public.stores para expor a coluna nova.
--
-- SQL plano, idempotente, schema-qualificado, SEM marcadores goose.

alter table queue.stores
    add column if not exists store_type text not null default 'bairro';

do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'queue_stores_store_type_check'
    ) then
        alter table queue.stores
            add constraint queue_stores_store_type_check
            check (store_type in ('shopping', 'bairro'));
    end if;
end $$;

-- Recriar view public.stores para incluir store_type.
-- Lista de colunas copiada da 0126 (billing_amount) + store_type.
-- OR REPLACE e idempotente: atualiza a definicao sem dropar dependentes.
create or replace view public.stores as
    select
        id,
        tenant_id,
        code,
        name,
        city,
        is_active,
        created_at,
        updated_at,
        default_template_id,
        monthly_goal,
        weekly_goal,
        avg_ticket_goal,
        conversion_goal,
        pa_goal,
        billing_amount,
        store_type
    from queue.stores;
