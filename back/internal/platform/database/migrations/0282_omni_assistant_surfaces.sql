-- Perfis de capacidades do assistente 360 por superficie. A configuracao vive
-- junto da fonte canonica do Omni Chat; nao cria uma quarta pilha de IA.

alter table automation.omni_chat_configs
    add column if not exists surface_modules jsonb not null default '{
      "calendar": {"calendar": "write", "tasks": "write", "meta_ads": "off", "users": "read"},
      "meta_ads": {"calendar": "off", "tasks": "off", "meta_ads": "write", "users": "off"},
      "global": {"calendar": "read", "tasks": "read", "meta_ads": "read", "users": "read"}
    }'::jsonb;

alter table automation.omni_chat_configs
    drop constraint if exists omni_chat_configs_surface_modules_object;

alter table automation.omni_chat_configs
    add constraint omni_chat_configs_surface_modules_object
    check (jsonb_typeof(surface_modules) = 'object');

comment on column automation.omni_chat_configs.surface_modules is
    'Modos solicitados (off/read/write) por surface e modulo; o backend sempre intersecta com contratacao e RBAC.';

-- A conversa continua na fonte unica existente do Calendar durante a extracao
-- do motor, mas passa a registrar de qual pagina nasceu. Isso preserva o contexto
-- quando o usuario navega entre modulos usando a mesma janela global.
alter table calendar.chat_conversations
    add column if not exists entry_surface text not null default 'calendar';

alter table calendar.chat_conversations
    drop constraint if exists calendar_chat_conversations_entry_surface_check;

alter table calendar.chat_conversations
    add constraint calendar_chat_conversations_entry_surface_check
    check (entry_surface in ('calendar', 'meta_ads', 'global'));

create index if not exists calendar_chat_conversations_surface_updated_idx
    on calendar.chat_conversations (account_id, entry_surface, updated_at desc)
    where deleted_at is null;

comment on column calendar.chat_conversations.entry_surface is
    'Surface de origem da conversa compartilhada; nao concede capability nem permissao.';
