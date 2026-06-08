-- 0129 — Site tracking events.
--
-- Recebe lotes assinados do site da Perola via:
-- POST /v1/webhooks/tracking/{sourceSlug}

create schema if not exists site;

do $$
begin
    if exists (
        select 1
        from pg_constraint
        where conrelid = 'site.webhook_sources'::regclass
          and conname = 'webhook_sources_entity_type_check'
    ) then
        alter table site.webhook_sources drop constraint webhook_sources_entity_type_check;
    end if;
end $$;

alter table site.webhook_sources
    add constraint webhook_sources_entity_type_check
    check (entity_type in ('leads', 'products', 'tracking'));

create table if not exists site.tracking_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    source_id uuid references site.webhook_sources(id) on delete set null,
    source_label text not null default '',
    source text not null default '',
    batch_id text not null default '',
    source_event_id text not null default '',
    visitor_id text not null default '',
    session_id text not null default '',
    event_type text not null default '',
    event_name text not null default '',
    page_url text not null default '',
    page_path text not null default '',
    page_title text not null default '',
    page_group text not null default '',
    page_name text not null default '',
    referrer text not null default '',
    element_tag text not null default '',
    element_text text not null default '',
    element_href text not null default '',
    element_id text not null default '',
    element_classes text not null default '',
    element_role text not null default '',
    product_code text not null default '',
    active_seconds integer,
    scroll_depth integer,
    screen_width integer,
    screen_height integer,
    viewport_width integer,
    viewport_height integer,
    device_type text not null default '',
    browser_lang text not null default '',
    timezone text not null default '',
    utm_source text not null default '',
    utm_medium text not null default '',
    utm_campaign text not null default '',
    utm_term text not null default '',
    utm_content text not null default '',
    event_data jsonb,
    raw_payload jsonb,
    ip text not null default '',
    user_agent text not null default '',
    sent_at timestamptz,
    received_at timestamptz not null default now()
);

create index if not exists site_tracking_events_account_received_idx
    on site.tracking_events (account_id, received_at desc);
create index if not exists site_tracking_events_account_event_idx
    on site.tracking_events (account_id, event_type, event_name, received_at desc);
create index if not exists site_tracking_events_account_page_idx
    on site.tracking_events (account_id, page_path, received_at desc);
create index if not exists site_tracking_events_account_session_idx
    on site.tracking_events (account_id, session_id, received_at desc);
create index if not exists site_tracking_events_account_visitor_idx
    on site.tracking_events (account_id, visitor_id, received_at desc);
create unique index if not exists site_tracking_events_source_event_unique
    on site.tracking_events (source_id, source_event_id)
    where source_id is not null and source_event_id <> '';

create or replace view public.site_tracking_events as
    select * from site.tracking_events;
