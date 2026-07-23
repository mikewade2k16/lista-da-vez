-- 0232_messaging_ai_unlimited_turns.sql
--
-- `max_ai_turns = 0` significa sem limite por conversa. Versoes publicadas existentes
-- permanecem imutaveis; a configuracao ativa passa a 0 quando o painel salvar uma nova
-- versao. Novos agentes/versoes usam 0 como default.

alter table messaging.ai_agent_versions
    alter column max_ai_turns set default 0;

alter table messaging.ai_agent_versions
    drop constraint if exists messaging_ai_versions_turns_ck;

alter table messaging.ai_agent_versions
    add constraint messaging_ai_versions_turns_ck
    check (max_ai_turns between 0 and 100);
