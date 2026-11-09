create table if not exists finance.assistant_configs (
 account_id uuid primary key references core.accounts(id) on delete cascade,
 data jsonb not null default '{}'::jsonb,
 updated_at timestamptz not null default clock_timestamp()
);
alter table calendar.chat_conversations drop constraint if exists calendar_chat_conversations_entry_surface_check;
alter table calendar.chat_conversations add constraint calendar_chat_conversations_entry_surface_check check(entry_surface in ('calendar','meta_ads','global','finance'));
alter table calendar.chat_ask_requests drop constraint if exists chat_ask_requests_entry_surface_check;
alter table calendar.chat_ask_requests add constraint chat_ask_requests_entry_surface_check check(entry_surface in ('calendar','meta_ads','global','finance'));
