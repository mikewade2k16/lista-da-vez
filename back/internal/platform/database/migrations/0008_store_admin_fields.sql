alter table stores
  add column if not exists default_template_id text not null default '',
  add column if not exists monthly_goal numeric(14, 2) not null default 0,
  add column if not exists weekly_goal numeric(14, 2) not null default 0,
  add column if not exists avg_ticket_goal numeric(14, 2) not null default 0,
  add column if not exists conversion_goal numeric(6, 2) not null default 0,
  add column if not exists pa_goal numeric(8, 2) not null default 0;
