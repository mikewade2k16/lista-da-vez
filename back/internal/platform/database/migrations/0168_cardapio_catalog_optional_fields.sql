-- Modulo cardapio (Fase 2, WS-F): campos opcionais do contrato da API do site
-- (TAVOLA) que faltavam. image_url na categoria (foto representativa) e
-- compare_at_price_cents no produto (preco cheio para exibicao riscada).
-- productCount NAO tem coluna: e derivado no service_public (conta produtos
-- disponiveis por categoria). Idempotente, schema-qualificado, sem -- +goose Down
-- (o migrator roda o arquivo inteiro). Plano: docs/cardapio/PLANO_CARDAPIO_FASE2.md (WS-F).

alter table cardapio.categories
    add column if not exists image_url text not null default '';

alter table cardapio.products
    add column if not exists compare_at_price_cents bigint not null default 0;
