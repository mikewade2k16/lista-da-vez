-- Fase 3 — RBAC dinâmico (Item 1)
-- Adiciona is_locked em core.role_templates. Roles clonados de templates com
-- is_locked=true recebem is_locked=true em core.roles e não podem ser deletados.
-- Exemplo: core.owner — o proprietário da account deve sempre existir.
alter table core.role_templates
    add column if not exists is_locked boolean not null default false;
