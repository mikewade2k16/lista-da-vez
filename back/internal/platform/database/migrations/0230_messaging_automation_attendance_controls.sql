-- Omnichannel MVP: distinguish model and operator handoffs in operational cards.
-- No workflow or channel ownership changes; PostgreSQL remains authoritative.

do $$
begin
    if exists (
        select 1 from pg_constraint
        where conname = 'handoffs_reason_code_check'
          and conrelid = 'messaging.handoffs'::regclass
    ) then
        alter table messaging.handoffs drop constraint handoffs_reason_code_check;
    end if;
end $$;

alter table messaging.handoffs
    add constraint handoffs_reason_code_check check (reason_code in (
        'requested','low_confidence','max_turns','tool_failed','policy','error',
        'model_handoff','operator_paused'
    ));
