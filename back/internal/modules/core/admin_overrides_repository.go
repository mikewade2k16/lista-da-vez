package core

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAdminOverridesRepository implementa AdminOverridesRepository contra
// core.user_permission_overrides + core.permissions + core.account_modules.
type PostgresAdminOverridesRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAdminOverridesRepository cria a implementacao Postgres dos overrides.
func NewPostgresAdminOverridesRepository(pool *pgxpool.Pool) *PostgresAdminOverridesRepository {
	return &PostgresAdminOverridesRepository{pool: pool}
}

// IsAccountMember diz se o usuario-alvo e membro (account_users, ativo OU inativo)
// da account — override pressupoe vinculo. So checa o vinculo do alvo, nao do ator.
func (r *PostgresAdminOverridesRepository) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid
		)
	`, accountID, userID).Scan(&ok)
	return ok, err
}

// ListActiveOverrides retorna os overrides ATIVOS do usuario na account.
func (r *PostgresAdminOverridesRepository) ListActiveOverrides(ctx context.Context, accountID, userID string) ([]UserPermissionOverride, error) {
	const query = `
		select permission_key, effect, note
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		order by permission_key asc
	`
	rows, err := r.pool.Query(ctx, query, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserPermissionOverride, 0)
	for rows.Next() {
		var o UserPermissionOverride
		if err := rows.Scan(&o.PermissionKey, &o.Effect, &o.Note); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListAvailablePermissions retorna as permissoes aplicaveis a overrides na
// account: de modulos HABILITADOS (core.account_modules.enabled=true), nao
// deprecated e com scope != 'platform' (override de permissao de plataforma e
// bloqueado por design).
func (r *PostgresAdminOverridesRepository) ListAvailablePermissions(ctx context.Context, accountID string) ([]AvailablePermission, error) {
	const query = `
		select p.key, p.label, p.module_id, p.scope
		from core.permissions p
		join core.account_modules am
			on am.module_id = p.module_id
			and am.account_id = $1::uuid
			and am.enabled = true
		where p.deprecated_at is null
		  and p.scope <> 'platform'
		order by p.module_id asc, p.key asc
	`
	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AvailablePermission, 0)
	for rows.Next() {
		var p AvailablePermission
		if err := rows.Scan(&p.Key, &p.Label, &p.ModuleID, &p.Scope); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PlatformScopedKeys retorna, dentre as keys informadas, as que tem
// scope='platform' (bloqueadas para override). Parametrizado via unnest.
func (r *PostgresAdminOverridesRepository) PlatformScopedKeys(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}
	const query = `
		select p.key
		from core.permissions p
		join unnest($1::text[]) as k(key) on k.key = p.key
		where p.scope = 'platform'
		order by p.key asc
	`
	rows, err := r.pool.Query(ctx, query, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ReplaceUserOverrides substitui os overrides do usuario na account numa
// transacao: (1) desativa TODOS os ativos atuais (is_active=false libera o indice
// unico parcial (account_id,user_id,permission_key) where is_active); (2) insere
// os novos com is_active=true e created_by_user_id=actorUserID. account_id e
// user_id vem do path (parametros $1/$2), nunca do body.
func (r *PostgresAdminOverridesRepository) ReplaceUserOverrides(ctx context.Context, accountID, userID, actorUserID string, overrides []UserPermissionOverride) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// (1) Desativa os ativos atuais. Mantemos a linha (historico/auditoria) — o
	// indice unico parcial so considera is_active=true, entao reinserir a mesma
	// key nao conflita.
	if _, err := tx.Exec(ctx, `
		update core.user_permission_overrides
		set is_active = false, updated_at = now()
		where account_id = $1::uuid and user_id = $2::uuid and is_active = true
	`, accountID, userID); err != nil {
		return err
	}

	// (2) Insere os novos. created_by_user_id audita quem aplicou (actorUserID do
	// Principal). effect/key ja validados no service.
	if len(overrides) > 0 {
		keys := make([]string, len(overrides))
		effects := make([]string, len(overrides))
		notes := make([]string, len(overrides))
		for i, o := range overrides {
			keys[i] = o.PermissionKey
			effects[i] = o.Effect
			notes[i] = o.Note
		}
		// unnest com varios arrays em paralelo expande coluna-a-coluna na mesma
		// ordem (key[i], effect[i], note[i]). Parametrizado, sem concatenacao.
		if _, err := tx.Exec(ctx, `
			insert into core.user_permission_overrides
				(account_id, user_id, permission_key, effect, note, is_active, created_by_user_id)
			select $1::uuid, $2::uuid, t.key, t.effect, t.note, true, $6::uuid
			from unnest($3::text[], $4::text[], $5::text[]) as t(key, effect, note)
		`, accountID, userID, keys, effects, notes, actorUserID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
