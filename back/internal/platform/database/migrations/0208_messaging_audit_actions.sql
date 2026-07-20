-- Modulo Omnichannel — F7 (Acoes do inbox): estende o CHECK de messaging.audit_events.
-- Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md (§9.2 F7). Spec: docs/omnichannel/specs/OMNI-F7.md (C5).
--
-- A F2 (0200) modelou event_type como CHECK com 5 tipos e NENHUM para acao destrutiva
-- (Prisma AuditEvent:40-46). A F7 acrescenta os dois tipos das acoes irreversiveis/externas:
--   MESSAGE_DELETED_FOR_ALL — apagar-para-todos (irreversivel, visivel ao cliente final).
--   MESSAGE_FORWARDED       — encaminhar (publica conteudo em outra conversa).
-- delete-for-me NAO entra: e ocultacao por usuario (hidden_messages), sem efeito externo.
--
-- O nome da constraint e o auto-gerado do Postgres para um CHECK de coluna inline
-- (0200: `event_type text not null check (...)`) => `<tabela>_<coluna>_check`, sem schema no
-- nome: audit_events_event_type_check. Confirmado contra 0200_messaging_schema.sql:217.
--
-- Idempotente (drop if exists antes do add: re-rodar dropa o recem-criado e recria), schema
-- qualificado, SEM `-- +goose Down` (o migrator roda o arquivo inteiro; um Down se
-- auto-destruiria — ver 0200/0187). account_id ja e NOT NULL na tabela (0200), nada a alterar.

alter table messaging.audit_events
    drop constraint if exists audit_events_event_type_check;

alter table messaging.audit_events
    add constraint audit_events_event_type_check
    check (event_type in (
        'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
        'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
        'MESSAGE_FORWARDED', 'MESSAGE_DELETED_FOR_ALL'
    ));
