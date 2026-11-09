-- CT-15 fase C (P15-22/P15-33): curadoria do acervo público usado pelo chat Finance.
-- Coleta de fontes oficiais de um catálogo fechado no servidor → trechos em rascunho →
-- revisões registradas (dono, IA externa, contador) → publicação explícita pelo dono.
-- Nada é publicado automaticamente; o acervo continua sem tenant porque só recebe
-- material público. Sem binário no banco: guarda hash, tamanho e URL do original.

create table if not exists knowledge.public_collections (
    id uuid primary key default gen_random_uuid(),
    catalog_key text not null check (catalog_key ~ '^[a-z0-9][a-z0-9-]{1,60}$'),
    version integer not null check (version > 0),
    title text not null check (length(title) between 1 and 300),
    source_ref text not null check (source_ref like 'https://%' and length(source_ref) <= 1000),
    content_type text not null check (content_type in ('text/html', 'application/pdf')),
    sha256 text not null check (sha256 ~ '^[0-9a-f]{64}$'),
    byte_size bigint not null check (byte_size > 0),
    pages integer check (pages is null or pages > 0),
    chunk_count integer not null check (chunk_count >= 0),
    status text not null default 'collected' check (status in ('collected', 'published', 'superseded', 'discarded')),
    metadata jsonb not null default '{}'::jsonb,
    collected_by uuid not null references core.users(id) on delete restrict,
    collected_at timestamptz not null default clock_timestamp(),
    published_by uuid references core.users(id) on delete restrict,
    published_at timestamptz,
    unique (catalog_key, version),
    check (status <> 'published' or (published_by is not null and published_at is not null))
);
create index if not exists knowledge_public_collections_key_idx on knowledge.public_collections(catalog_key, version desc);

-- Tentativas de coleta (inclusive falhas), para o painel mostrar o último resultado.
create table if not exists knowledge.public_collection_attempts (
    id bigint generated always as identity primary key,
    catalog_key text not null,
    outcome text not null check (outcome in ('collected', 'unchanged', 'failed')),
    error_code text check (error_code is null or length(error_code) <= 80),
    collection_id uuid references knowledge.public_collections(id) on delete set null,
    actor_id uuid not null references core.users(id) on delete restrict,
    created_at timestamptz not null default clock_timestamp()
);
create index if not exists knowledge_public_collection_attempts_key_idx on knowledge.public_collection_attempts(catalog_key, created_at desc);

alter table knowledge.public_sources add column if not exists collection_id uuid references knowledge.public_collections(id) on delete restrict;
alter table knowledge.public_sources add column if not exists chunk_index integer check (chunk_index is null or chunk_index >= 0);
create index if not exists knowledge_public_sources_collection_idx on knowledge.public_sources(collection_id, chunk_index) where collection_id is not null;

create table if not exists knowledge.public_reviews (
    id uuid primary key default gen_random_uuid(),
    collection_id uuid not null references knowledge.public_collections(id) on delete restrict,
    reviewer_kind text not null check (reviewer_kind in ('owner', 'ai', 'accountant')),
    reviewer_name text not null check (length(reviewer_name) between 1 and 120),
    verdict text not null check (verdict in ('approved', 'changes_requested', 'rejected')),
    notes text not null default '' check (length(notes) <= 4000),
    actor_id uuid not null references core.users(id) on delete restrict,
    created_at timestamptz not null default clock_timestamp()
);
create index if not exists knowledge_public_reviews_collection_idx on knowledge.public_reviews(collection_id, created_at desc);

create or replace function knowledge.public_reviews_immutable() returns trigger
language plpgsql as $$
begin
    raise exception 'knowledge.public_reviews is append-only';
end;
$$;
drop trigger if exists knowledge_public_reviews_immutable on knowledge.public_reviews;
create trigger knowledge_public_reviews_immutable
    before update or delete on knowledge.public_reviews
    for each row execute function knowledge.public_reviews_immutable();
