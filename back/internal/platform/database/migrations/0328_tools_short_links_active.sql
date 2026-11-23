-- Encurtador: liga/desliga sem excluir + ultimo clique.
-- is_active = false faz /s/{slug} responder 404 sem contar clique, e o link
-- volta a funcionar ao reativar (mesmo comportamento do QR em 0197).
-- last_hit_at registra o ultimo clique contado (preview de app/robo nao conta).
-- Links existentes nascem ativos (default), entao nada muda para eles.
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (ver 0197_tools_module.sql).

alter table tools.short_links
    add column if not exists is_active boolean not null default true;

alter table tools.short_links
    add column if not exists last_hit_at timestamptz;
