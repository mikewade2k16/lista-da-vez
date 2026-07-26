-- Customer Data idempotency is scoped by the relationship owner and client.
-- An agency may legitimately receive the same connector/request key for two
-- different client accounts; one client must never replay the other's result.

alter table customer_data.subject_source_links
    drop constraint if exists subject_source_links_account_id_idempotency_key_key;
alter table customer_data.subject_source_links
    add constraint subject_source_links_account_client_idempotency_key
    unique (account_id, client_account_id, idempotency_key);

alter table customer_data.match_candidates
    drop constraint if exists match_candidates_account_id_idempotency_key_key;
alter table customer_data.match_candidates
    add constraint match_candidates_account_client_idempotency_key
    unique (account_id, client_account_id, idempotency_key);

alter table customer_data.merge_events
    drop constraint if exists merge_events_account_id_idempotency_key_key;
alter table customer_data.merge_events
    add constraint merge_events_account_client_idempotency_key
    unique (account_id, client_account_id, idempotency_key);

alter table customer_data.outbox_events
    drop constraint if exists outbox_events_account_id_idempotency_key_key;
alter table customer_data.outbox_events
    add constraint outbox_events_account_client_idempotency_key
    unique (account_id, client_account_id, idempotency_key);
