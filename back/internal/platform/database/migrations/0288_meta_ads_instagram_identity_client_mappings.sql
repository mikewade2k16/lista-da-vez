-- 0288 - vinculo autoritativo de identidades Page/Instagram da conexao
-- central da agencia com uma account-cliente da mesma organizacao.
--
-- A Graph continua sendo a fonte das identidades existentes. Esta tabela
-- persiste somente a atribuicao tenant-scoped; o service cruza os dois IDs
-- atuais da Graph antes de gravar e antes de expor posts em client scope.

create unique index if not exists meta_ads_connections_account_id_pair_idx
    on meta_ads.connections (account_id, id);

create table if not exists meta_ads.instagram_identity_client_mappings (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    connection_id uuid not null,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    ig_user_id text not null,
    page_id text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint meta_ads_instagram_identity_mapping_distinct_accounts
        check (account_id <> client_account_id),
    constraint meta_ads_instagram_identity_mapping_ig_id_valid
        check (length(ig_user_id) between 1 and 64 and ig_user_id ~ '^[0-9]+$'),
    constraint meta_ads_instagram_identity_mapping_page_id_valid
        check (length(page_id) between 1 and 64 and page_id ~ '^[0-9]+$'),
    constraint meta_ads_instagram_identity_mapping_ig_unique
        unique (account_id, ig_user_id),
    constraint meta_ads_instagram_identity_mapping_page_unique
        unique (account_id, page_id),
    constraint meta_ads_instagram_identity_mapping_connection_fk
        foreign key (account_id, connection_id)
        references meta_ads.connections(account_id, id) on delete cascade
);

create index if not exists meta_ads_instagram_identity_mapping_client_idx
    on meta_ads.instagram_identity_client_mappings (account_id, client_account_id);
