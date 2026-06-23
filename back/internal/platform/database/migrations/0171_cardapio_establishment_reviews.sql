-- Modulo cardapio (Fase 9, F2): avaliacoes de estabelecimento. Ate aqui toda
-- review pertencia a um produto (product_id NOT NULL). Agora um review pode ser
-- do ESTABELECIMENTO (product_id NULL) e um review de produto pode ser marcado
-- para aparecer tambem na vitrine do estabelecimento (show_on_establishment).
-- Idempotente, schema-qualificado, sem -- +goose Down (o migrator roda o arquivo
-- inteiro). Plano canonico: docs/cardapio/PLANO_CARDAPIO_GESTAO_UX.md (F2).

alter table cardapio.reviews
    alter column product_id drop not null;

alter table cardapio.reviews
    add column if not exists show_on_establishment boolean not null default false;
