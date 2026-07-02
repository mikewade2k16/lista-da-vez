-- 0179: indice (tenant_id, store_id, order_date) em public.erp_order_raw.
--
-- A aba Compras do ERP passou a filtrar o periodo pela DATA REAL da compra
-- (order_date) em vez da data do lote importado (source_batch_date) — base exata
-- do filtro de sorteio. So que existia indice em (tenant_id, store_id,
-- source_batch_date) e NENHUM em order_date: o filtro `order_date >= X and
-- order_date < Y` caia em Seq Scan dos ~770k registros (custo ~93k no EXPLAIN),
-- repetido 4-5x pelo CTE inline -> a tela "carregava infinito".
--
-- Com este indice o filtro de periodo vira Index Range Scan (le so as linhas do
-- intervalo). Serve tanto para os cards (GetRecordsStats) quanto para a lista
-- (ListRawRecords) e para ORDER BY order_date. Espelha o erp_order_raw_tenant_
-- store_idx ja existente (que cobre source_batch_date).
--
-- Idempotente (IF NOT EXISTS); migrator roda o arquivo inteiro (sem goose Up/Down).
create index if not exists erp_order_raw_tenant_store_order_date_idx
    on public.erp_order_raw (tenant_id, store_id, order_date);

-- ANALYZE imediato: CREATE INDEX nao recalcula o histograma de order_date, e sem
-- stats frescas o planner ignora o indice e volta ao seq scan (medido: 2.3s antes
-- do analyze vs 83ms depois). Garante plano bom ja no deploy, antes do autoanalyze.
analyze public.erp_order_raw;
