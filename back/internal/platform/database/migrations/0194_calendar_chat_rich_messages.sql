-- Chat do calendario: metadados ricos e propostas duraveis por mensagem.
-- proposal guarda o pedido estruturado; proposal_status so muda por acao explicita
-- do usuario. calendar_items e um snapshot de eventos reais validado pelo backend.

alter table calendar.chat_messages
  add column if not exists proposal jsonb,
  add column if not exists proposal_status text not null default 'none',
  add column if not exists calendar_items jsonb not null default '[]'::jsonb;

do $$
begin
  if not exists (
    select 1 from pg_constraint
    where conname = 'calendar_chat_messages_proposal_status_check'
      and conrelid = 'calendar.chat_messages'::regclass
  ) then
    alter table calendar.chat_messages
      add constraint calendar_chat_messages_proposal_status_check
      check (proposal_status in ('none', 'pending', 'accepted', 'rejected'));
  end if;
end $$;
