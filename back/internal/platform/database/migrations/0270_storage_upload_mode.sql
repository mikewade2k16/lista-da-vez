-- Seletor operacional reversivel para a migracao gradual dos uploads ao R2.
-- A infraestrutura continua controlada por R2_ENABLED; este valor decide o
-- destino de novos uploads sem impedir a leitura dos objetos R2 existentes.

alter table storage.settings
    add column if not exists uploads_enabled boolean not null default false;

comment on column storage.settings.uploads_enabled is
    'Quando true, novos uploads dos adapters integrados usam R2; quando false, preservam o storage local legado.';
