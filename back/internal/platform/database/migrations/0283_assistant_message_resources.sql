-- Cards read-only do Assistente 360. O modelo devolve apenas resourceIds;
-- o backend cruza com um registry autorizado e persiste o snapshot sanitizado.

alter table calendar.chat_messages
    add column if not exists resources jsonb not null default '[]'::jsonb,
    add column if not exists context_modules jsonb not null default '[]'::jsonb;

comment on column calendar.chat_messages.resources is
    'Snapshots read-only autorizados de instagram_post, meta_campaign e meta_ad_account; nunca payload livre do LLM.';

comment on column calendar.chat_messages.context_modules is
    'Modulos de contexto efetivamente anexados pelo backend a resposta; usado para invalidar historico apos revogacao de capability.';
