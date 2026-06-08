-- Multi-Tenant Completion (C3)
-- Branch: refactor/multi-tenant-complete
-- Plano: docs/MULTITENANT_COMPLETION_PLAN.md secao C3
--
-- Adiciona billing_amount em queue.stores para suporte ao modo per_store.
-- Quando billing_mode = 'per_store' na account, cada loja tem valor proprio.
-- Recria a view public.stores para expor a nova coluna ao codigo legado.

alter table queue.stores
    add column if not exists billing_amount numeric(12, 2) not null default 0;

do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'queue_stores_billing_amount_check'
    ) then
        alter table queue.stores
            add constraint queue_stores_billing_amount_check
            check (billing_amount >= 0);
    end if;
end $$;

-- Recriar view public.stores para incluir billing_amount.
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
        billing_amount
    from queue.stores;
