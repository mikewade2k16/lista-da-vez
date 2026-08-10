-- 0267_account_attendance_recording_feature.sql
--
-- O rollout de gravacao pertence a account operacional. A flag global antiga
-- e usada somente no backfill para preservar o comportamento no momento da
-- migracao; depois disso, queue.attendance_recording_settings e a fonte unica.

create table if not exists queue.attendance_recording_settings (
    account_id uuid primary key,
    enabled boolean not null default false,
    updated_by uuid,
    updated_at timestamptz not null default now()
);

insert into queue.attendance_recording_settings (account_id, enabled)
select
    account.id,
    coalesce((platform.config ->> 'attendanceAudioRecording')::boolean, false)
from core.accounts account
left join core.platform_settings platform
  on platform.key = 'experimental_features'
where account.is_active = true
on conflict (account_id) do nothing;

comment on table queue.attendance_recording_settings is
    'Controle por account para iniciar novas gravacoes de atendimentos.';

comment on column queue.attendance_recording_settings.enabled is
    'Quando false, impede novas gravacoes sem ocultar o historico existente.';
