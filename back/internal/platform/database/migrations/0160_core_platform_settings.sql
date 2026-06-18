-- 0160 — Configurações GLOBAIS da plataforma (singleton por chave).
--
-- core.platform_settings guarda configuração de NÍVEL PLATAFORMA, NÃO por account
-- e NÃO por usuário. É um key-value singleton (uma linha por chave). A primeira
-- chave é 'menu_layout': a organização global do menu (quais itens vão no header
-- vs sidebar), definida por platform_admin e lida por TODOS os usuários.
--
-- EXCEÇÃO CONSCIENTE à regra "toda tabela tem account_id": esta tabela é
-- deliberadamente platform-global. Não há coluna account_id porque a config é
-- única para a plataforma inteira (igual ao catálogo de módulos). Escrita
-- restrita a platform_admin na camada de serviço/HTTP.
--
-- REGRAS (AGENT_RULES.md + memória do projeto):
--   - O migrator roda o arquivo .sql INTEIRO no boot. SEM marcadores -- +goose e
--     SEM DROP: SQL plano e idempotente (CREATE TABLE IF NOT EXISTS), seguro para
--     rodar 2x sem efeito colateral.
--   - Schema sempre qualificado (core.*).
--   - config é jsonb; updated_by referencia core.users(id) e pode ser NULL
--     (default vazio quando a linha ainda não existe / escrita do sistema).

create table if not exists core.platform_settings (
    key        text primary key,
    config     jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null default now(),
    updated_by uuid references core.users(id)
);
