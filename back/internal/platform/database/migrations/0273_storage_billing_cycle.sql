-- Alinha o ledger e as consultas de Analytics ao ciclo de faturamento exibido
-- pela Cloudflare. O dia fica no PostgreSQL para poder ser corrigido sem rebuild.

alter table storage.settings
    add column if not exists billing_cycle_day smallint not null default 27;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'storage_settings_billing_cycle_day_check'
          and conrelid = 'storage.settings'::regclass
    ) then
        alter table storage.settings
            add constraint storage_settings_billing_cycle_day_check
            check (billing_cycle_day between 1 and 28);
    end if;
end $$;

comment on column storage.settings.billing_cycle_day is
    'Dia inicial do ciclo mensal de faturamento do R2, conforme o painel Cloudflare.';
