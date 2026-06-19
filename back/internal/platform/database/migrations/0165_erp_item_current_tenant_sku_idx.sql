-- 0165: indice (tenant_id, sku) em queue.erp_item_current.
--
-- A tool de catalogo do Omni Chat (back/internal/modules/automation/store_product.go)
-- enriquece site.products pelo ERP via o codigo do produto (sku == split_part(code,'_',1)),
-- buscando por (tenant_id, sku). A PK da tabela e' (tenant_id, store_id, sku); buscar
-- (tenant_id, sku) SEM store_id nao casa o prefixo do indice e varria todos os itens do
-- tenant (~360k na Perola) por lookup -> a query do catalogo levava ~8s (timeout).
-- Com este indice o lookup vira ponto -> ~60ms. Util tambem p/ qualquer busca por sku
-- escopada por tenant (sem saber a loja).
--
-- Idempotente (IF NOT EXISTS); migrator roda o arquivo inteiro (sem goose Up/Down).
create index if not exists erp_item_current_tenant_sku_idx
    on queue.erp_item_current (tenant_id, sku);
