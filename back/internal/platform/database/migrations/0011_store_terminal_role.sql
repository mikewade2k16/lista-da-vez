alter table if exists user_store_roles
	drop constraint if exists user_store_roles_role_check;

alter table if exists user_store_roles
	add constraint user_store_roles_role_check
	check (role in ('consultant', 'manager', 'store_terminal'));
