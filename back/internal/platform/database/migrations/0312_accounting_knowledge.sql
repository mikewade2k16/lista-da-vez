-- CT-06: canonical corpus stays in messaging. No account/corpus is activated.
create table if not exists accounting.knowledge_bindings (
 id uuid primary key default gen_random_uuid(),
 account_id uuid not null references core.accounts(id),
 issuer_id uuid not null,
 knowledge_base_id uuid not null,
 is_enabled boolean not null default false,
 version bigint not null default 1 check(version>0),
 created_at timestamptz not null default now(),
 updated_at timestamptz not null default now(),
 foreign key(account_id,issuer_id) references accounting.issuers(account_id,id),
 foreign key(account_id,knowledge_base_id) references messaging.knowledge_bases(account_id,id),
 unique(account_id,issuer_id,knowledge_base_id),
 unique(account_id,knowledge_base_id)
);
create index if not exists accounting_knowledge_binding_scope_idx on accounting.knowledge_bindings(account_id,issuer_id,is_enabled,knowledge_base_id);
create table if not exists messaging.knowledge_document_applicability (
 account_id uuid not null references core.accounts(id),
 document_id uuid not null,
 version integer not null check(version>0),
 metadata jsonb not null check(jsonb_typeof(metadata)='object'),
 reviewed_by uuid references core.users(id),
 reviewed_at timestamptz,
 status text not null default 'draft' check(status in ('draft','reviewed','inactive')),
 original_storage_id uuid not null,
 checksum text not null,
 version_lock bigint not null default 1 check(version_lock>0),
 created_at timestamptz not null default now(),
 updated_at timestamptz not null default now(),
 primary key(account_id,document_id,version),
 foreign key(account_id,document_id) references messaging.knowledge_documents(account_id,id)
);
create index if not exists messaging_knowledge_applicability_scope_idx on messaging.knowledge_document_applicability(account_id,status,(metadata->>'jurisdiction'),(metadata->>'validFrom'));
alter table messaging.knowledge_chunks add column if not exists location jsonb not null default '{}'::jsonb;
-- Once published, referenced text and metadata cannot be rewritten by either consumer.
create or replace function messaging.guard_knowledge_document() returns trigger language plpgsql as $$
begin
 if old.status in ('published','archived') and
 (tg_op='DELETE' or new.title is distinct from old.title or new.source_ref is distinct from old.source_ref or new.checksum is distinct from old.checksum or new.metadata is distinct from old.metadata or new.version is distinct from old.version or new.knowledge_base_id is distinct from old.knowledge_base_id or new.status not in ('published','archived')) then
 raise exception 'immutable knowledge version' using errcode='23514';
 end if;
 if tg_op='DELETE' then return old; end if;
 return new;
end $$;
drop trigger if exists knowledge_document_immutable on messaging.knowledge_documents;
create trigger knowledge_document_immutable before update or delete on messaging.knowledge_documents for each row execute function messaging.guard_knowledge_document();
create or replace function messaging.guard_knowledge_chunk() returns trigger language plpgsql as $$
declare parent_status text;
begin
 if tg_op<>'INSERT' then
  select status into parent_status from messaging.knowledge_documents where account_id=old.account_id and id=old.document_id for update;
  if parent_status in ('published','archived') then raise exception 'immutable knowledge chunks' using errcode='23514'; end if;
 end if;
 if tg_op<>'DELETE' then
  select status into parent_status from messaging.knowledge_documents where account_id=new.account_id and id=new.document_id for update;
  if parent_status in ('published','archived') then raise exception 'immutable knowledge chunks' using errcode='23514'; end if;
  return new;
 end if;
 return old;
end $$;
drop trigger if exists knowledge_chunk_immutable on messaging.knowledge_chunks;
create trigger knowledge_chunk_immutable before insert or update or delete on messaging.knowledge_chunks for each row execute function messaging.guard_knowledge_chunk();
