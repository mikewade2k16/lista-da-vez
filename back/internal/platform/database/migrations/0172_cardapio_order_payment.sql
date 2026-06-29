-- Modulo cardapio: forma de pagamento escolhida pelo cliente no checkout do
-- cardapio publico (TAVOLA). Antes o pagamento era apenas INFORMATIVO em
-- settings.payment; agora o pedido carrega a forma escolhida e, quando dinheiro
-- em entrega, o troco solicitado. payment_method e um token livre
-- (pix/cash/debit/credit/ticket/other); change_for_cents so e relevante em
-- entrega + dinheiro (zero nos demais casos).
-- Idempotente, schema-qualificada, sem -- +goose Down (o migrator roda o arquivo
-- inteiro).

alter table cardapio.orders
    add column if not exists payment_method   text   not null default '';

alter table cardapio.orders
    add column if not exists change_for_cents bigint not null default 0;
