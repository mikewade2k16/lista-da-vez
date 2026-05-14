-- Fase 4B — Domínio operacional no schema queue
--
-- Move tabelas de operação (fila, atendimento, histórico), configurações
-- operacionais por tenant e feedback para queue.*.
-- Requer 0104 aplicada (queue.stores e queue.consultants devem existir).
--
-- FKs intra-queue: store_id → queue.stores, consultant_id → queue.consultants.
-- FKs cross-schema: tenant_id → public.tenants, user_id → public.users
--   (permanecem em public.* até core.accounts ser a fonte de verdade — Fase 4+).

-- ============================================================================
-- Configurações operacionais por tenant
-- ============================================================================

create table queue.tenant_operation_settings
    (like public.tenant_operation_settings including all);

alter table queue.tenant_operation_settings
    add constraint queue_top_settings_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

insert into queue.tenant_operation_settings
    select * from public.tenant_operation_settings;

alter table public.tenant_operation_settings rename to tenant_operation_settings_v0;
create view public.tenant_operation_settings as
    select * from queue.tenant_operation_settings;
drop table public.tenant_operation_settings_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.tenant_setting_options
    (like public.tenant_setting_options including all);

alter table queue.tenant_setting_options
    add constraint queue_tsetting_options_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

insert into queue.tenant_setting_options
    select * from public.tenant_setting_options;

alter table public.tenant_setting_options rename to tenant_setting_options_v0;
create view public.tenant_setting_options as
    select * from queue.tenant_setting_options;
drop table public.tenant_setting_options_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.tenant_catalog_products
    (like public.tenant_catalog_products including all);

alter table queue.tenant_catalog_products
    add constraint queue_tcatalog_products_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

insert into queue.tenant_catalog_products
    select * from public.tenant_catalog_products;

alter table public.tenant_catalog_products rename to tenant_catalog_products_v0;
create view public.tenant_catalog_products as
    select * from queue.tenant_catalog_products;
drop table public.tenant_catalog_products_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.tenant_operation_core_settings
    (like public.tenant_operation_core_settings including all);

alter table queue.tenant_operation_core_settings
    add constraint queue_top_core_settings_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

insert into queue.tenant_operation_core_settings
    select * from public.tenant_operation_core_settings;

alter table public.tenant_operation_core_settings rename to tenant_operation_core_settings_v0;
create view public.tenant_operation_core_settings as
    select * from queue.tenant_operation_core_settings;
drop table public.tenant_operation_core_settings_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.tenant_finish_modal_settings
    (like public.tenant_finish_modal_settings including all);

alter table queue.tenant_finish_modal_settings
    add constraint queue_tfinish_modal_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

insert into queue.tenant_finish_modal_settings
    select * from public.tenant_finish_modal_settings;

alter table public.tenant_finish_modal_settings rename to tenant_finish_modal_settings_v0;
create view public.tenant_finish_modal_settings as
    select * from queue.tenant_finish_modal_settings;
drop table public.tenant_finish_modal_settings_v0 cascade;

-- ============================================================================
-- Tabelas de operação em tempo real
-- ============================================================================

create table queue.operation_queue_entries
    (like public.operation_queue_entries including all);

alter table queue.operation_queue_entries
    add constraint queue_oqe_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

alter table queue.operation_queue_entries
    add constraint queue_oqe_consultant_fk
        foreign key (consultant_id) references queue.consultants(id) on delete cascade;

insert into queue.operation_queue_entries
    select * from public.operation_queue_entries;

alter table public.operation_queue_entries rename to operation_queue_entries_v0;
create view public.operation_queue_entries as
    select * from queue.operation_queue_entries;
drop table public.operation_queue_entries_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.operation_active_services
    (like public.operation_active_services including all);

alter table queue.operation_active_services
    add constraint queue_oas_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

alter table queue.operation_active_services
    add constraint queue_oas_consultant_fk
        foreign key (consultant_id) references queue.consultants(id) on delete cascade;

insert into queue.operation_active_services
    select * from public.operation_active_services;

alter table public.operation_active_services rename to operation_active_services_v0;
create view public.operation_active_services as
    select * from queue.operation_active_services;
drop table public.operation_active_services_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.operation_paused_consultants
    (like public.operation_paused_consultants including all);

alter table queue.operation_paused_consultants
    add constraint queue_opc_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

alter table queue.operation_paused_consultants
    add constraint queue_opc_consultant_fk
        foreign key (consultant_id) references queue.consultants(id) on delete cascade;

insert into queue.operation_paused_consultants
    select * from public.operation_paused_consultants;

alter table public.operation_paused_consultants rename to operation_paused_consultants_v0;
create view public.operation_paused_consultants as
    select * from queue.operation_paused_consultants;
drop table public.operation_paused_consultants_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.operation_current_status
    (like public.operation_current_status including all);

alter table queue.operation_current_status
    add constraint queue_ocs_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

alter table queue.operation_current_status
    add constraint queue_ocs_consultant_fk
        foreign key (consultant_id) references queue.consultants(id) on delete cascade;

insert into queue.operation_current_status
    select * from public.operation_current_status;

alter table public.operation_current_status rename to operation_current_status_v0;
create view public.operation_current_status as
    select * from queue.operation_current_status;
drop table public.operation_current_status_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.operation_status_sessions
    (like public.operation_status_sessions including all);

alter table queue.operation_status_sessions
    add constraint queue_oss_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

alter table queue.operation_status_sessions
    add constraint queue_oss_consultant_fk
        foreign key (consultant_id) references queue.consultants(id) on delete cascade;

insert into queue.operation_status_sessions
    select * from public.operation_status_sessions;

alter table public.operation_status_sessions rename to operation_status_sessions_v0;
create view public.operation_status_sessions as
    select * from queue.operation_status_sessions;
drop table public.operation_status_sessions_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.operation_service_history
    (like public.operation_service_history including all);

alter table queue.operation_service_history
    add constraint queue_osh_store_fk
        foreign key (store_id) references queue.stores(id) on delete cascade;

alter table queue.operation_service_history
    add constraint queue_osh_person_fk
        foreign key (person_id) references queue.consultants(id) on delete cascade;

insert into queue.operation_service_history
    select * from public.operation_service_history;

alter table public.operation_service_history rename to operation_service_history_v0;
create view public.operation_service_history as
    select * from queue.operation_service_history;
drop table public.operation_service_history_v0 cascade;

-- ============================================================================
-- Feedback
-- ============================================================================

create table queue.user_feedback
    (like public.user_feedback including all);

alter table queue.user_feedback
    add constraint queue_ufeedback_tenant_fk
        foreign key (tenant_id) references public.tenants(id) on delete cascade;

-- store_id é nullable (migração 0029); FK só quando não nulo — sem FK declarativa.
-- Mantemos consistência referencial via aplicação.

alter table queue.user_feedback
    add constraint queue_ufeedback_user_fk
        foreign key (user_id) references public.users(id) on delete cascade;

insert into queue.user_feedback select * from public.user_feedback;

alter table public.user_feedback rename to user_feedback_v0;
create view public.user_feedback as select * from queue.user_feedback;
drop table public.user_feedback_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.feedback_messages
    (like public.feedback_messages including all);

alter table queue.feedback_messages
    add constraint queue_fmessages_feedback_fk
        foreign key (feedback_id) references queue.user_feedback(id) on delete cascade;

alter table queue.feedback_messages
    add constraint queue_fmessages_author_fk
        foreign key (author_user_id) references public.users(id) on delete cascade;

insert into queue.feedback_messages select * from public.feedback_messages;

alter table public.feedback_messages rename to feedback_messages_v0;
create view public.feedback_messages as select * from queue.feedback_messages;
drop table public.feedback_messages_v0 cascade;

-- ----------------------------------------------------------------------------

create table queue.feedback_read_states
    (like public.feedback_read_states including all);

alter table queue.feedback_read_states
    add constraint queue_fread_states_feedback_fk
        foreign key (feedback_id) references queue.user_feedback(id) on delete cascade;

alter table queue.feedback_read_states
    add constraint queue_fread_states_user_fk
        foreign key (user_id) references public.users(id) on delete cascade;

insert into queue.feedback_read_states select * from public.feedback_read_states;

alter table public.feedback_read_states rename to feedback_read_states_v0;
create view public.feedback_read_states as select * from queue.feedback_read_states;
drop table public.feedback_read_states_v0 cascade;
