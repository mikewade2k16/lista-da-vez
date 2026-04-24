alter table operation_paused_consultants
	add column if not exists kind text not null default 'pause';

alter table operation_paused_consultants
	drop constraint if exists operation_paused_consultants_kind_check;

alter table operation_paused_consultants
	add constraint operation_paused_consultants_kind_check
	check (kind in ('pause', 'assignment'));

update operation_paused_consultants
set kind = 'pause'
where kind is null
   or btrim(kind) = '';
