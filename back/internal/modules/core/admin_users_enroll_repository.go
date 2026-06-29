package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Este arquivo concentra a matricula/membership (enroll) de usuarios em accounts.
// Foi separado do CRUD em admin_users_repository.go por coesao e limite de linhas;
// como tudo vive no mesmo pacote core, mover de arquivo NAO altera comportamento.

// pgxExec abstrai os metodos de execucao compartilhados por *pgxpool.Pool e
// pgx.Tx, permitindo reusar enrollUserInAccount tanto fora quanto dentro de uma
// transacao (MoveUserAccount). So expomos o que o enroll usa (Exec).
type pgxExec interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// enrollUserInAccount garante membership + papel CORE do usuario numa account:
// (1) core.account_users, (2) clona o role queue.<papel> do template e atribui em
// core.user_role_assignments, (3) copia as role_permissions do template. Reaproveitado
// pela conta-cliente e pela conta-agencia. Mapeamento owner/director->queue.supervisor,
// marketing->queue.consultant (igual a 0133 + auth core_role_resolver). Recebe um
// pgxExec (pool OU tx) para poder rodar dentro de uma transacao.
func enrollUserInAccount(ctx context.Context, exec pgxExec, accountID, userID, role string) error {
	// on conflict reativa a membership (is_active=true): garante que re-enroll de
	// um vinculo previamente desativado (ex.: MoveUserAccount para uma conta onde
	// o usuario ja teve membership) volte a ficar ativo, sem perder joined_at.
	if _, err := exec.Exec(ctx, `
		insert into core.account_users (account_id, user_id, is_active, joined_at)
		values ($1::uuid, $2::uuid, true, now())
		on conflict (account_id, user_id) do update set is_active = true
	`, accountID, userID); err != nil {
		return err
	}
	if _, err := exec.Exec(ctx, `
		with ins as (
			insert into core.roles (account_id, cloned_from_template_id, code, label, description, is_locked)
			select $1::uuid, rt.id, 'queue.' || $3, rt.label, rt.description, rt.is_locked
			from core.role_templates rt
			where rt.id = case $3::text when 'marketing' then 'queue.consultant' else 'queue.supervisor' end
			on conflict (account_id, code) do nothing
			returning id
		),
		resolved as (
			select id from ins
			union
			select id from core.roles where account_id = $1::uuid and code = 'queue.' || $3
		)
		insert into core.user_role_assignments (account_id, user_id, role_id)
		select $1::uuid, $2::uuid, (select id from resolved limit 1)
		where (select id from resolved limit 1) is not null
		on conflict (account_id, user_id, role_id) do nothing
	`, accountID, userID, role); err != nil {
		return err
	}
	if _, err := exec.Exec(ctx, `
		insert into core.role_permissions (role_id, permission_key)
		select r.id, rtp.permission_key
		from core.roles r
		join core.role_template_permissions rtp on rtp.role_template_id = r.cloned_from_template_id
		where r.account_id = $1::uuid and r.code = 'queue.' || $2
		on conflict do nothing
	`, accountID, role); err != nil {
		return err
	}
	return nil
}

// agencyAccountRole mapeia o cargo de agencia para o papel na conta-agencia:
// agency_owner -> owner (acesso total da agencia); demais (agency_member) -> director
// (acesso limitado, tenant-scoped, sem exigir vinculo de loja).
func agencyAccountRole(orgRole string) string {
	if strings.TrimSpace(orgRole) == "agency_owner" {
		return "owner"
	}
	return "director"
}

// SetUserAccountRole remove os papeis atuais do usuario naquela conta e atribui o
// novo (reaproveitando enrollUserInAccount, que garante membership + role + perms).
func (r *PostgresAdminUserRepository) SetUserAccountRole(ctx context.Context, accountID, userID, role string) error {
	if _, err := r.pool.Exec(ctx, `
		delete from core.user_role_assignments
		where account_id = $1::uuid and user_id = $2::uuid
	`, accountID, userID); err != nil {
		return err
	}
	return enrollUserInAccount(ctx, r.pool, accountID, userID, role)
}

// ============================================================================
// MoveUserAccount (transacao: remove vinculos de cliente atuais + matricula no destino)
// ============================================================================

// MoveUserAccount MOVE o usuario para a conta-cliente destino, tudo numa
// transacao para nao deixar o usuario sem vinculo se algo falhar no meio:
//  1. valida que o destino existe, esta ATIVO e NAO e agencia (is_agency=false);
//  2. remove os user_role_assignments das contas-CLIENTE nao-agencia atuais;
//  3. desativa as memberships account_users nao-agencia atuais;
//  4. matricula no destino reusando enrollUserInAccount (membership + papel + perms).
//
// NAO toca vinculos de agencia (account_users de contas is_agency=true nem os
// respectivos role_assignments) — este endpoint e exclusivo de cliente.
func (r *PostgresAdminUserRepository) MoveUserAccount(ctx context.Context, userID, targetAccountID, role string) (AdminUserView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AdminUserView{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// (1) Valida o destino: existe + ativo + nao-agencia. A query separa "nao existe
	// / inativo" (ErrAccountNotFound, 404 — nao vaza existencia) de "e agencia"
	// (ErrAccountIsAgency, 400 — endpoint so para cliente).
	var isAgency bool
	err = tx.QueryRow(ctx, `
		select is_agency from core.accounts
		where id = $1::uuid and is_active = true
	`, targetAccountID).Scan(&isAgency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminUserView{}, ErrAccountNotFound
		}
		return AdminUserView{}, err
	}
	if isAgency {
		return AdminUserView{}, ErrAccountIsAgency
	}

	// (2) Remove os papeis das contas-CLIENTE (nao-agencia) atuais do usuario.
	if _, err := tx.Exec(ctx, `
		delete from core.user_role_assignments ura
		using core.accounts a
		where a.id = ura.account_id
			and ura.user_id = $1::uuid
			and a.is_agency = false
	`, userID); err != nil {
		return AdminUserView{}, err
	}

	// (3) Desativa as memberships de cliente (nao-agencia) atuais. Mantemos a linha
	// (is_active=false) em vez de deletar para preservar joined_at/historico — o
	// enroll no destino faz upsert e reativa se a conta destino ja existir aqui.
	if _, err := tx.Exec(ctx, `
		update core.account_users au
		set is_active = false
		from core.accounts a
		where a.id = au.account_id
			and au.user_id = $1::uuid
			and a.is_agency = false
	`, userID); err != nil {
		return AdminUserView{}, err
	}

	// (4) Matricula no destino (membership ativa + papel + perms) na mesma tx.
	if err := enrollUserInAccount(ctx, tx, targetAccountID, userID, role); err != nil {
		return AdminUserView{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AdminUserView{}, err
	}
	return r.FindAdminUser(ctx, userID)
}
