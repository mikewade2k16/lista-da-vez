-- 0231_messaging_contact_suppressions.sql
--
-- Ocultacao global de um contato nas superficies do atendimento. A linha nao apaga
-- mensagens: `history_cleared_at` funciona como corte logico, preservando integridade,
-- auditoria e midias enquanto impede que o historico anterior volte ao restaurar.

create table if not exists messaging.contact_suppressions (
    account_id uuid not null references core.accounts(id) on delete cascade,
    contact_id uuid not null,
    is_hidden boolean not null default true,
    hidden_at timestamptz not null default now(),
    hidden_by_user_id uuid not null references core.users(id) on delete restrict,
    history_cleared_at timestamptz,
    history_cleared_by_user_id uuid references core.users(id) on delete set null,
    restored_at timestamptz,
    restored_by_user_id uuid references core.users(id) on delete set null,
    updated_at timestamptz not null default now(),
    primary key (account_id, contact_id),
    constraint messaging_contact_suppressions_contact_fk
        foreign key (account_id, contact_id)
        references messaging.contacts(account_id, id)
        on delete cascade
);

create index if not exists messaging_contact_suppressions_hidden_idx
    on messaging.contact_suppressions (account_id, hidden_at desc)
    where is_hidden = true;

-- A permissao e deliberadamente separada das roles/template do modulo. Ela so pode
-- existir como override explicito por usuario; nenhum papel administrativo a recebe.
insert into core.modules (id, schema_name, label, description, is_core, sort_order)
values ('omnichannel', 'messaging', 'Omnichannel',
        'Atendimento WhatsApp: inbox, setores/filas e triagem por IA.', false, 47)
on conflict (id) do nothing;

insert into core.permissions (key, module_id, label, description, scope)
values ('omnichannel.conversations.privacy.manage', 'omnichannel',
        'Gerenciar contatos ocultos',
        'Ocultar, restaurar e limpar o historico visivel de contatos do atendimento.',
        'account')
on conflict (key) do update set
    module_id = excluded.module_id,
    label = excluded.label,
    description = excluded.description,
    scope = excluded.scope,
    deprecated_at = null,
    updated_at = now();

-- Solicitacao operacional explicita: somente este usuario recebe o poder sensivel,
-- em cada conta ativa da qual ja e membro. Novos grants continuam administraveis via
-- RBAC; o backend nao contem checagem hardcoded de e-mail.
insert into core.user_permission_overrides (
    account_id, user_id, permission_key, effect, note, is_active, created_by_user_id
)
select au.account_id, u.id, 'omnichannel.conversations.privacy.manage', 'allow',
       'Grant exclusivo solicitado pelo titular para privacidade de conversas.', true, u.id
from core.users u
join core.account_users au on au.user_id = u.id and au.is_active = true
where lower(u.email) = 'mikewade2k16@gmail.com'
  and u.is_active = true
  and not exists (
      select 1
      from core.user_permission_overrides existing
      where existing.account_id = au.account_id
        and existing.user_id = u.id
        and existing.permission_key = 'omnichannel.conversations.privacy.manage'
        and existing.is_active = true
  );
