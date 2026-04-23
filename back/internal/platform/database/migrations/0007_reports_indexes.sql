create index if not exists operation_service_history_store_finished_idx
	on operation_service_history (store_id, finished_at desc);

create index if not exists operation_service_history_store_person_finished_idx
	on operation_service_history (store_id, person_id, finished_at desc);

create index if not exists operation_service_history_store_outcome_finished_idx
	on operation_service_history (store_id, finish_outcome, finished_at desc);

create index if not exists operation_service_history_visit_reasons_gin_idx
	on operation_service_history using gin (visit_reasons_json);

create index if not exists operation_service_history_customer_sources_gin_idx
	on operation_service_history using gin (customer_sources_json);
