alter table users
	add column if not exists avatar_path text not null default '';
