-- Dev-only seed: keep the local platform_admin root on a known password.
-- This is skipped in production by migrator.go.

update users u
set
	password_hash = crypt('Mvp@2026!', gen_salt('bf', 10)),
	must_change_password = false,
	is_active = true,
	updated_at = now()
where lower(u.email) = 'mikewade2k16@gmail.com'
	and exists (
		select 1
		from user_platform_roles upr
		where upr.user_id = u.id
			and upr.role = 'platform_admin'
	);