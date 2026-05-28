alter table user_feedback
    alter column store_id drop not null,
    alter column tenant_id drop not null;
