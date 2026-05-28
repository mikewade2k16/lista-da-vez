-- Fase 4A — Fundação do schema queue
--
-- Move tabelas estáveis (stores, consultants, store_settings) de public.* para
-- queue.*. Views de compatibilidade em public.* garantem retrocompatibilidade
-- zero-code-change para os módulos Go legados.
--
-- Ordem obrigatória: stores → consultants → store_settings (dependências de FK).
-- CASCADE nos DROPs remove apenas FK constraints de tabelas ainda em public.*;
-- os dados nessas tabelas permanecem intactos.
--
-- IDEMPOTENTE: se schema queue já existir (re-execução), as CREATE TABLE falham
-- cedo com erro legível antes de modificar qualquer dado. Execute em transação.

-- ============================================================================
-- Schema
-- ============================================================================

create schema if not exists queue;

-- ============================================================================
-- queue.stores
-- ============================================================================

create table queue.stores (like public.stores including all);

alter table queue.stores
    add constraint queue_stores_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

insert into queue.stores select * from public.stores;

alter table public.stores rename to stores_v0;
create view public.stores as select * from queue.stores;
drop table public.stores_v0 cascade;

-- ============================================================================
-- queue.consultants
-- ============================================================================

create table queue.consultants (like public.consultants including all);

alter table queue.consultants
    add constraint queue_consultants_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

alter table queue.consultants
    add constraint queue_consultants_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

insert into queue.consultants select * from public.consultants;

alter table public.consultants rename to consultants_v0;
create view public.consultants as select * from queue.consultants;
drop table public.consultants_v0 cascade;

-- ============================================================================
-- queue.store_settings  (skip: tabela `public.store_settings` nao existe neste
-- schema; o equivalente em uso e `public.store_operation_settings`, que vai
-- ser tratado na 0105 junto com o restante das settings tenant/store).
-- ============================================================================
