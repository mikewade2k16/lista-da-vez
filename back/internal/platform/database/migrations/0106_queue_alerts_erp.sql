-- Fase 4C - Alertas e ERP no schema queue
--
-- Move tabelas de alertas operacionais e sincronizacao ERP para queue.*.
-- Views de compatibilidade em public.* preservam os queries legados.
--
-- Importante: as tabelas ERP podem ter milhoes de linhas. Esta migration move
-- as tabelas existentes entre schemas em vez de copiar dados para tabelas novas.
-- FKs novas para queue.stores sao adicionadas como NOT VALID para evitar scan
-- completo no boot; a validacao pode virar migration dedicada quando necessario.

-- ============================================================================
-- Configuracoes e instancias de alertas
-- ============================================================================

alter table public.tenant_alert_settings set schema queue;
create view public.tenant_alert_settings as
	select * from queue.tenant_alert_settings;

-- ----------------------------------------------------------------------------

alter table public.tenant_operational_alert_rules set schema queue;
create view public.tenant_operational_alert_rules as
	select * from queue.tenant_operational_alert_rules;

-- ----------------------------------------------------------------------------

alter table public.alert_instances set schema queue;

alter table queue.alert_instances
	add constraint queue_alert_instances_store_fk
		foreign key (store_id) references queue.stores(id) on delete cascade not valid;

create view public.alert_instances as
	select * from queue.alert_instances;

-- ----------------------------------------------------------------------------

alter table public.alert_actions set schema queue;

alter table queue.alert_actions
	add constraint queue_alert_actions_store_fk
		foreign key (store_id) references queue.stores(id) on delete cascade not valid;

create view public.alert_actions as
	select * from queue.alert_actions;

-- ============================================================================
-- ERP - sincronizacao de dados externos
-- ============================================================================

alter table public.erp_sync_runs set schema queue;

alter table queue.erp_sync_runs
	add constraint queue_erp_sync_runs_store_fk
		foreign key (store_id) references queue.stores(id) on delete cascade not valid;

create view public.erp_sync_runs as
	select * from queue.erp_sync_runs;

-- ----------------------------------------------------------------------------

alter table public.erp_item_raw set schema queue;

alter table queue.erp_item_raw
	add constraint queue_erp_item_raw_store_fk
		foreign key (store_id) references queue.stores(id) on delete cascade not valid;

create view public.erp_item_raw as
	select * from queue.erp_item_raw;

-- ----------------------------------------------------------------------------

alter table public.erp_item_current set schema queue;

alter table queue.erp_item_current
	add constraint queue_erp_item_current_store_fk
		foreign key (store_id) references queue.stores(id) on delete cascade not valid;

create view public.erp_item_current as
	select * from queue.erp_item_current;
