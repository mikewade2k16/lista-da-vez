alter table users
	add column if not exists must_change_password boolean not null default false;

update users
set must_change_password = true
where id in (
	select c.user_id
	from consultants c
	where c.user_id is not null
);
