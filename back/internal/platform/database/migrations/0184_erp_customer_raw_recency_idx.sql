-- 0184: indice de recencia (tenant_id, identifier, source_batch_date desc,
-- created_at_imported desc) em public.erp_customer_raw.
--
-- A busca por nome na aba Compras passou a casar SO com o nome do lote mais recente
-- de cada CPF (customerNameMatchSubquery, via NOT EXISTS "existe linha mais nova").
-- O enriquecimento de nome/contato (email, telefone, celular) tambem le o lote mais
-- recente por identifier (loadCustomerContactsByIdentifier). Este indice ordena a
-- recencia dentro de cada (tenant_id, identifier) para esses recortes virarem range
-- scan em vez de varrer/ordenar as ~345k linhas de clientes. Complementa o
-- 0180 (tenant_id, identifier), que sozinho nao cobre a ordenacao por data do lote.
--
-- Idempotente (IF NOT EXISTS); migrator roda o arquivo inteiro (sem goose Up/Down).
create index if not exists erp_customer_raw_tenant_identifier_recency_idx
    on public.erp_customer_raw (tenant_id, identifier, source_batch_date desc, created_at_imported desc)
    where identifier <> '';

analyze public.erp_customer_raw;
