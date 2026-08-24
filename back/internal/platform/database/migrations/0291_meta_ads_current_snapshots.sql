-- Meta Ads: cada leitura operacional usa apenas o snapshot que pertence a
-- conexao/token atual. Linhas antigas permanecem para preservar IDs, vinculos e
-- auditoria, mas deixam de ser elegiveis para painel, IA e execucao.

alter table meta_ads.connections
    add column if not exists revision uuid;

update meta_ads.connections
set revision = gen_random_uuid()
where revision is null;

alter table meta_ads.connections
    alter column revision set default gen_random_uuid();

alter table meta_ads.connections
    alter column revision set not null;

-- Nao ha como provar que um cache criado antes desta migration ainda pertence
-- aos grants atuais. O backfill e deliberadamente fail-closed; a proxima
-- descoberta/sincronizacao publica somente o snapshot confirmado na Graph.
do $migration$
begin
    if not exists (
        select 1 from information_schema.columns
        where table_schema = 'meta_ads'
          and table_name = 'ad_accounts'
          and column_name = 'is_current'
    ) then
        alter table meta_ads.ad_accounts
            add column is_current boolean not null default false;
        update meta_ads.ad_accounts set is_current = false where is_current;
    end if;
    if not exists (
        select 1 from information_schema.columns
        where table_schema = 'meta_ads'
          and table_name = 'campaigns'
          and column_name = 'is_current'
    ) then
        alter table meta_ads.campaigns
            add column is_current boolean not null default false;
        update meta_ads.campaigns set is_current = false where is_current;
    end if;
end
$migration$;

create index if not exists meta_ads_ad_accounts_current_idx
    on meta_ads.ad_accounts (account_id, name, meta_ad_account_id)
    where is_current;

create index if not exists meta_ads_campaigns_current_idx
    on meta_ads.campaigns (account_id, ad_account_id, name, meta_campaign_id)
    where is_current;
