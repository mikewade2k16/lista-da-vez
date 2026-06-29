-- Modulo cardapio: banner por categoria do cardapio. Alem da capa (image_url, WS-F)
-- e do subtitulo (description, ja existente), a categoria agora carrega um banner
-- (banner_url) — imagem larga de topo da secao no site publico (TAVOLA).
-- Idempotente, schema-qualificada, sem -- +goose Down (o migrator roda o arquivo
-- inteiro; um Down com DROP se auto-destruiria).

alter table cardapio.categories
    add column if not exists banner_url text not null default '';
