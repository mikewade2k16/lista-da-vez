-- Public curated material only. Private tenant documents never enter this table.
create schema if not exists knowledge;
create table if not exists knowledge.public_sources (
 id uuid primary key default gen_random_uuid(),
 source_key text not null,
 version integer not null check(version>0),
 title text not null,
 source_ref text not null check(source_ref like 'https://%'),
 excerpt text not null check(length(excerpt) between 1 and 1600),
 metadata jsonb not null default '{}'::jsonb,
 status text not null default 'draft' check(status in ('draft','published','withdrawn')),
 reviewed_at timestamptz,
 search_vector tsvector generated always as (to_tsvector('simple', translate(lower(title || ' ' || excerpt),'áàâãäéèêëíìîïóòôõöúùûüç','aaaaaeeeeiiiiooooouuuuc'))) stored,
 unique(source_key,version),
 check(status<>'published' or reviewed_at is not null)
);
create index if not exists knowledge_public_sources_search_idx on knowledge.public_sources using gin(search_vector);
alter table finance.access_messages add column if not exists public_evidence jsonb not null default '[]'::jsonb check(jsonb_typeof(public_evidence)='array');

create or replace function knowledge.protect_public_source_version() returns trigger language plpgsql as $$
begin
 if old.status in ('published','withdrawn') and
    (new.id,new.source_key,new.version,new.title,new.source_ref,new.excerpt,new.metadata,new.reviewed_at)
    is distinct from (old.id,old.source_key,old.version,old.title,old.source_ref,old.excerpt,old.metadata,old.reviewed_at) then
  raise exception 'published public source versions are immutable';
 end if;
 return new;
end;
$$;
drop trigger if exists knowledge_public_source_immutable on knowledge.public_sources;
create trigger knowledge_public_source_immutable before update on knowledge.public_sources for each row execute function knowledge.protect_public_source_version();
