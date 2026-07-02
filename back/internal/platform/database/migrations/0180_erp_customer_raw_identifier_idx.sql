-- 0180: indice (tenant_id, identifier) em public.erp_customer_raw.
--
-- A aba Compras passou a enriquecer cada pagina com o NOME do cliente, casando
-- order.customer_id (o CPF) com erp_customer_raw.identifier em lote
-- (identifier = any($array)). Sem indice em identifier, esse lookup varria as
-- ~345k linhas de clientes a cada pagina. Com o indice vira lookup por ponto.
-- Mesmo padrao do resolver bulk (relations_resolver.go).
--
-- Idempotente (IF NOT EXISTS); migrator roda o arquivo inteiro (sem goose Up/Down).
create index if not exists erp_customer_raw_tenant_identifier_idx
    on public.erp_customer_raw (tenant_id, identifier)
    where identifier <> '';

analyze public.erp_customer_raw;
