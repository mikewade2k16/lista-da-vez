create temp table tmp_tenant_seed_users on commit drop as
select distinct u.id
from users u
where exists (
	select 1
	from user_tenant_roles utr
	where utr.user_id = u.id
		and utr.tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid
)
or exists (
	select 1
	from user_store_roles usr
	join stores s on s.id = usr.store_id
	where usr.user_id = u.id
		and s.tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid
);

update users u
set
	password_hash = crypt('desde1967', gen_salt('bf', 10)),
	must_change_password = case
		when exists (
			select 1
			from user_store_roles usr
			where usr.user_id = u.id
				and usr.role = 'store_terminal'
		) then false
		else true
	end,
	updated_at = now()
where u.id in (select id from tmp_tenant_seed_users);

update user_invitations ui
set
	status = 'revoked',
	revoked_at = now(),
	updated_at = now()
where ui.user_id in (select id from tmp_tenant_seed_users)
	and ui.status = 'pending';