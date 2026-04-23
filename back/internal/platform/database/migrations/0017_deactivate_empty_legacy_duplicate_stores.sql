update stores s
set
	is_active = false,
	updated_at = now()
where s.tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid
	and s.code in ('PJ-GARCIA', 'PJ-TRE')
	and not exists (
		select 1
		from user_store_roles usr
		where usr.store_id = s.id
	)
	and not exists (
		select 1
		from consultants c
		where c.store_id = s.id
	);