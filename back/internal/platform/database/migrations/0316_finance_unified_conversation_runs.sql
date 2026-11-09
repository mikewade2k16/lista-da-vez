-- A personal conversation has no corporate issuer. Keep the existing usage ledger.
alter table finance.plus_llm_runs alter column issuer_id drop not null;
