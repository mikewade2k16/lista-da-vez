-- Plus reuses the canonical personal chat; no model/config/history mirror.
alter table calendar.chat_conversations add column if not exists assistant_mode text not null default 'legacy' check(assistant_mode in ('legacy','plus'));
alter table calendar.chat_conversations add column if not exists issuer_id uuid;
alter table calendar.chat_conversations add column if not exists contract_version text not null default '1.0';
alter table calendar.chat_messages add column if not exists plus_envelope jsonb;
alter table calendar.chat_ask_requests add column if not exists assistant_mode text not null default 'legacy' check(assistant_mode in ('legacy','plus'));
create index if not exists calendar_plus_conversation_context_idx on calendar.chat_conversations(account_id,created_by_user_id,issuer_id,updated_at desc) where deleted_at is null and assistant_mode='plus';
create unique index if not exists calendar_plus_turn_message_uidx on calendar.chat_messages(account_id,conversation_id,(plus_envelope->>'requestId'),role) where plus_envelope is not null;
