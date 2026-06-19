-- Modulo cardapio (Fase 2, WS-C): campos faltantes do restaurante (paridade com
-- o cadastro antigo lojatop). Segmento, redes sociais extras (facebook/youtube) e
-- estatisticas (GA/Pixel/HTML adicional). Endereco extra (numero/complemento/
-- ponto de referencia) entra no address jsonb existente — sem coluna.
-- Idempotente, schema-qualificado, sem -- +goose Down (o migrator roda o arquivo
-- inteiro). Plano canonico: docs/cardapio/PLANO_CARDAPIO_FASE2.md secao WS-C.

alter table cardapio.restaurants
    add column if not exists segment             text not null default '',
    add column if not exists facebook            text not null default '',
    add column if not exists youtube             text not null default '',
    add column if not exists google_analytics_id text not null default '',
    add column if not exists facebook_pixel_id   text not null default '',
    add column if not exists custom_head_html    text not null default '';
