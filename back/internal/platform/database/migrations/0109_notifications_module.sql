-- Fase 12 / Tasks T3 — notifications
--
-- Cria o schema de notificacoes in-app com preferencias, mutes e delivery log.

create schema if not exists notifications;

create table if not exists notifications.user_notifications (
	id uuid primary key default gen_random_uuid(),
	account_id uuid not null references core.accounts(id) on delete cascade,
	user_id uuid not null references core.users(id) on delete cascade,
	source_module text not null,
	source_event text not null,
	title text not null,
	body text not null default '',
	link_path text not null default '',
	payload jsonb not null default '{}'::jsonb,
	read_at timestamptz,
	archived_at timestamptz,
	created_at timestamptz not null default now()
);

create index if not exists notifications_user_notifications_user_idx
	on notifications.user_notifications (account_id, user_id, archived_at, created_at desc);

create index if not exists notifications_user_notifications_unread_idx
	on notifications.user_notifications (account_id, user_id, created_at desc)
	where read_at is null and archived_at is null;

create table if not exists notifications.notification_channels (
	account_id uuid not null references core.accounts(id) on delete cascade,
	user_id uuid not null references core.users(id) on delete cascade,
	channel text not null check (channel in ('in_app', 'email', 'whatsapp', 'push')),
	source_module text not null default '',
	source_event text not null default '',
	enabled boolean not null default true,
	updated_at timestamptz not null default now(),
	primary key (account_id, user_id, channel, source_module, source_event)
);

create index if not exists notifications_notification_channels_lookup_idx
	on notifications.notification_channels (account_id, user_id, channel, source_module, source_event);

create table if not exists notifications.delivery_log (
	id bigserial primary key,
	notification_id uuid references notifications.user_notifications(id) on delete cascade,
	channel text not null check (channel in ('in_app', 'email', 'whatsapp', 'push')),
	status text not null check (status in ('sent', 'failed', 'not_configured', 'muted', 'skipped')),
	error text not null default '',
	attempted_at timestamptz not null default now()
);

create index if not exists notifications_delivery_log_notification_idx
	on notifications.delivery_log (notification_id, attempted_at desc);

create table if not exists notifications.mutes (
	account_id uuid not null references core.accounts(id) on delete cascade,
	user_id uuid not null references core.users(id) on delete cascade,
	resource_type text not null,
	resource_id text not null,
	until_at timestamptz not null,
	created_at timestamptz not null default now(),
	primary key (account_id, user_id, resource_type, resource_id)
);

create index if not exists notifications_mutes_active_idx
	on notifications.mutes (account_id, user_id, until_at desc);