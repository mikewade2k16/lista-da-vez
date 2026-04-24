create temp table tmp_legacy_demo_users on commit drop as
select u.id
from users u
join user_store_roles usr on usr.user_id = u.id
where usr.role in ('consultant', 'store_terminal')
	and (
		lower(u.email) like 'convite.%@demo.local'
		or lower(u.email) in (
			'teste.senha.4247@demo.local',
			'teste@teste.com',
			'terminal.jardins.teste@demo.local'
		)
	);

delete from consultants
where user_id in (select id from tmp_legacy_demo_users);

delete from users
where id in (select id from tmp_legacy_demo_users);

update users
set
	avatar_path = '',
	updated_at = now()
where lower(email) in (
	'betaniaconceicao681@gmail.com',
	'lane.olivieravcxz@gmail.com',
	'tonyw.right@outlook.com',
	'days.matos@gmail.com',
	'mikewade2k16@gmail.com',
	'talia.sts10@hotmail.com',
	'alexsandrapaz@gmail.com.br',
	'terminal.riomar@acesso.omni.local',
	'terminal.jardins@acesso.omni.local',
	'terminal.garcia@acesso.omni.local',
	'terminal.treze@acesso.omni.local',
	'roseli.a.paixao@gmail.com',
	'diancampos638@gmail.com',
	'caroline17silva@gmail.com',
	'nielaoliveira@hotmail.com',
	'daysepaiva.sp@hotmail.com',
	'rafialmengo01@gmail.com',
	'ray.tsaraujo@gmail.com',
	'hitanabatista1@gmail.com',
	'nutrilarad@gmail.com',
	'fabiomenezes80@hotmail.com',
	'daianecaroline340@gmail.com',
	'ritadamaris1@gmail.com',
	'tauvaniyassemin@gmail.com',
	'everlandalves38@gmail.com',
	'fabianarafaellaviana2@gmail.com',
	'acilenejeronimo1@hotmail.com',
	'gardenia.lobo@hotmail.com',
	'mirelamirelasilvarodrigues@gmail.com'
);