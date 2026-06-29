package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PostgresAdminUserLinksRepository cuida dos VINCULOS de um usuario: membership
// em conta-cliente (account_users) e cargo em organization (organization_users).
// Reaproveita o pool e os helpers (enrollUserInAccount, agencyAccountRole) do
// PostgresAdminUserRepository. Operacoes destrutivas correm em transacao.
type PostgresAdminUserLinksRepository struct {
	*PostgresAdminUserRepository
}

// NewPostgresAdminUserLinksRepository cria a implementacao Postgres dos vinculos.
func NewPostgresAdminUserLinksRepository(base *PostgresAdminUserRepository) *PostgresAdminUserLinksRepository {
	return &PostgresAdminUserLinksRepository{PostgresAdminUserRepository: base}
}

// AccountLinkInfo descreve a conta destino de um vinculo de cliente para
// validar antes de matricular. OrganizationID e "" quando a account nao esta
// vinculada a nenhuma org (coluna nullable).
type AccountLinkInfo struct {
	Exists         bool
	IsActive       bool
	IsAgency       bool
	OrganizationID string
}

// FindAccountLinkInfo carrega o estado da account destino (existe/ativa/agencia/
// org) para o service decidir 404 (nao existe/inativa) vs 400 (e agencia) e, no
// caso de conta-agencia, aplicar a autoridade de organizacao (M2).
func (r *PostgresAdminUserLinksRepository) FindAccountLinkInfo(ctx context.Context, accountID string) (AccountLinkInfo, error) {
	var info AccountLinkInfo
	var orgID *string
	err := r.pool.QueryRow(ctx, `
		select is_active, is_agency, organization_id::text from core.accounts where id = $1::uuid
	`, accountID).Scan(&info.IsActive, &info.IsAgency, &orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccountLinkInfo{Exists: false}, nil
		}
		return AccountLinkInfo{}, err
	}
	if orgID != nil {
		info.OrganizationID = *orgID
	}
	info.Exists = true
	return info, nil
}

// AddMembership matricula o usuario na conta-cliente (membership + papel + perms),
// reusando enrollUserInAccount. O upsert com `do update set is_active = true`
// reativa/nao duplica — adiciona o vinculo SEM remover os outros (diferente de
// MoveUserAccount). Nao toca demais vinculos.
func (r *PostgresAdminUserLinksRepository) AddMembership(ctx context.Context, accountID, userID, role string) error {
	return enrollUserInAccount(ctx, r.pool, accountID, userID, role)
}

// DeactivateMembership desativa o vinculo de cliente numa transacao: marca
// account_users.is_active=false (preserva joined_at/historico) e remove os
// user_role_assignments do usuario naquela conta. Idempotente.
func (r *PostgresAdminUserLinksRepository) DeactivateMembership(ctx context.Context, accountID, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		update core.account_users
		set is_active = false
		where account_id = $1::uuid and user_id = $2::uuid
	`, accountID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		delete from core.user_role_assignments
		where account_id = $1::uuid and user_id = $2::uuid
	`, accountID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// OrganizationLinkInfo descreve a org destino de um vinculo de agencia.
type OrganizationLinkInfo struct {
	Exists   bool
	IsActive bool
}

// FindOrganizationLinkInfo carrega o estado da org destino (existe/ativa).
func (r *PostgresAdminUserLinksRepository) FindOrganizationLinkInfo(ctx context.Context, organizationID string) (OrganizationLinkInfo, error) {
	var info OrganizationLinkInfo
	err := r.pool.QueryRow(ctx, `
		select is_active from core.organizations where id = $1::uuid
	`, organizationID).Scan(&info.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrganizationLinkInfo{Exists: false}, nil
		}
		return OrganizationLinkInfo{}, err
	}
	info.Exists = true
	return info, nil
}

// LinkUserToOrganization vincula o usuario a uma organization (cargo de agencia),
// reusando linkUserToOrganization (extraido de CreateUser para DRY). O cargo de
// agencia tambem matricula o usuario na conta-agencia da org para que ele consiga
// logar e enxergar os clientes da org (visao ampla).
func (r *PostgresAdminUserLinksRepository) LinkUserToOrganization(ctx context.Context, organizationID, userID, orgRole string) error {
	return linkUserToOrganization(ctx, r.pool, organizationID, userID, orgRole)
}

// CountAgencyOwners conta os agency_owner ativos da org (safeguard do ultimo dono).
func (r *PostgresAdminUserLinksRepository) CountAgencyOwners(ctx context.Context, organizationID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		select count(*) from core.organization_users
		where organization_id = $1::uuid and org_role = 'agency_owner'
	`, organizationID).Scan(&count)
	return count, err
}

// IsAgencyOwner diz se o usuario e agency_owner da org (para o safeguard saber
// se a remocao pode deixar a org sem dono).
func (r *PostgresAdminUserLinksRepository) IsAgencyOwner(ctx context.Context, organizationID, userID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.organization_users
			where organization_id = $1::uuid and user_id = $2::uuid and org_role = 'agency_owner'
		)
	`, organizationID, userID).Scan(&ok)
	return ok, err
}

// UnlinkUserFromOrganization remove o vinculo de agencia numa transacao: deleta
// a linha organization_users e desativa a membership do usuario na conta-agencia
// da org (ele perde a visao ampla). Preserva joined_at via is_active=false.
func (r *PostgresAdminUserLinksRepository) UnlinkUserFromOrganization(ctx context.Context, organizationID, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		delete from core.organization_users
		where organization_id = $1::uuid and user_id = $2::uuid
	`, organizationID, userID); err != nil {
		return err
	}
	// Desativa a membership na(s) conta(s)-agencia da org + remove role_assignments
	// dela. Mantem a linha (is_active=false) para preservar joined_at.
	if _, err := tx.Exec(ctx, `
		update core.account_users au
		set is_active = false
		from core.accounts a
		where a.id = au.account_id
			and a.organization_id = $1::uuid
			and a.is_agency = true
			and au.user_id = $2::uuid
	`, organizationID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		delete from core.user_role_assignments ura
		using core.accounts a
		where a.id = ura.account_id
			and a.organization_id = $1::uuid
			and a.is_agency = true
			and ura.user_id = $2::uuid
	`, organizationID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// linkUserToOrganization e o bloco de vinculo de agencia EXTRAIDO de CreateUser
// (admin_users_repository.go) para DRY: insere/atualiza organization_users e, se
// existir conta-agencia ativa na org, matricula o usuario nela com o papel
// correspondente (owner para dono, director para membro) para que ele consiga
// logar. Recebe pgxExec (pool OU tx). orgRole ja validado pelo service.
func linkUserToOrganization(ctx context.Context, exec pgxExec, organizationID, userID, orgRole string) error {
	if _, err := exec.Exec(ctx, `
		insert into core.organization_users (organization_id, user_id, org_role, joined_at)
		values ($1::uuid, $2::uuid, $3, now())
		on conflict (organization_id, user_id) do update set org_role = $3
	`, organizationID, userID, orgRole); err != nil {
		return err
	}
	// Cargo de agencia precisa logar: vira membro da conta-agencia (is_agency=true)
	// da org com papel conforme o cargo. Sem isto o usuario de agencia nao resolve
	// papel e o login falha.
	agencyAccountID, err := lookupAgencyAccountID(ctx, exec, organizationID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(agencyAccountID) != "" {
		if err := enrollUserInAccount(ctx, exec, agencyAccountID, userID, agencyAccountRole(orgRole)); err != nil {
			return err
		}
	}
	return nil
}

// lookupAgencyAccountID resolve a conta-agencia (is_agency=true, ativa) mais
// antiga da org, ou "" quando a org nao tem conta-agencia. Usa pgxExec via uma
// query escalar — pgxExec so expoe Exec, entao usamos o pool/tx subjacente.
func lookupAgencyAccountID(ctx context.Context, exec pgxExec, organizationID string) (string, error) {
	q, ok := exec.(pgxQuerier)
	if !ok {
		return "", errors.New("core: exec does not support queries")
	}
	var agencyAccountID string
	err := q.QueryRow(ctx, `
		select id::text
		from core.accounts
		where organization_id = $1::uuid and is_agency = true and is_active = true
		order by created_at asc
		limit 1
	`, organizationID).Scan(&agencyAccountID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	return agencyAccountID, nil
}

// pgxQuerier expoe QueryRow, implementado por *pgxpool.Pool e pgx.Tx. Usado para
// reusar lookupAgencyAccountID dentro e fora de transacao sem importar pgxpool
// aqui.
type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
