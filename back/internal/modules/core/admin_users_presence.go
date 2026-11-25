package core

// Presenca na listagem de /manage/users (so platform_admin).
//
// Fontes, ambas ja existentes no banco:
//   - core.user_sessions: created_at = login; last_seen_at = ultimo uso da sessao
//     (atualizado pelo auth a cada cache miss do Principal). Retroativo desde a
//     criacao das sessoes; antes do Touch ser ligado, last_seen_at == created_at.
//   - obs.presence: heartbeat do front (30s com aba visivel), retido por 24h.
//
// "Online" = ultimo sinal dentro de adminUserOnlineWindow.
const adminUserOnlineWindow = "2 minutes"

const adminUserPresenceSelect = `
			presence_sess.last_login_at,
			greatest(presence_sess.session_seen_at, presence_obs.heartbeat_at),
			greatest(presence_sess.session_seen_at, presence_obs.heartbeat_at)
				>= now() - interval '` + adminUserOnlineWindow + `'`

const adminUserPresenceNullSelect = `
			null::timestamptz, null::timestamptz, null::boolean`

const adminUserPresenceJoin = `
		left join lateral (
			select max(s.created_at) as last_login_at, max(s.last_seen_at) as session_seen_at
			from core.user_sessions s
			where s.user_id = u.id
		) presence_sess on true
		left join lateral (
			select max(p.last_seen) as heartbeat_at
			from obs.presence p
			where p.user_id = u.id::text
		) presence_obs on true`

func adminUserPresenceSQL(include bool) (selectSQL, joinSQL string) {
	if !include {
		return adminUserPresenceNullSelect, ""
	}
	return adminUserPresenceSelect, adminUserPresenceJoin
}
