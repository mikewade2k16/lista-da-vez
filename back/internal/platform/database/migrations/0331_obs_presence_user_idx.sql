-- Presenca por usuario: /manage/users (platform_admin) consulta o heartbeat mais
-- recente de cada usuario da pagina para "online agora" e "ultimo acesso".
create index if not exists presence_user_last_seen_idx
  on obs.presence (user_id, last_seen desc);
