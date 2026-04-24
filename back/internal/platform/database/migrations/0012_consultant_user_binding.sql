alter table if exists consultants
	add column if not exists user_id uuid references users(id) on delete restrict;

create unique index if not exists consultants_user_id_uidx
	on consultants (user_id)
	where user_id is not null;

do $$
declare
	rec record;
	generated_email text;
	inserted_user_id uuid;
begin
	for rec in
		select
			c.id,
			c.store_id,
			c.name,
			s.code
		from consultants c
		join stores s on s.id = c.store_id
		where c.user_id is null
			and c.is_active = true
	loop
		generated_email := lower(
			concat(
				trim(both '.' from regexp_replace(rec.name, '[^a-zA-Z0-9]+', '.', 'g')),
				'.',
				trim(both '.' from regexp_replace(rec.code, '[^a-zA-Z0-9]+', '', 'g')),
				'.',
				right(replace(rec.id::text, '-', ''), 4),
				'@acesso.omni.local'
			)
		);

		insert into users (
			email,
			display_name,
			password_hash,
			is_active
		)
		values (
			generated_email,
			rec.name,
			crypt('Omni@123', gen_salt('bf', 10)),
			true
		)
		returning id into inserted_user_id;

		insert into user_store_roles (user_id, store_id, role)
		values (inserted_user_id, rec.store_id, 'consultant')
		on conflict (user_id, store_id, role) do nothing;

		update consultants
		set
			user_id = inserted_user_id,
			updated_at = now()
		where id = rec.id
			and user_id is null;
	end loop;
end $$;
