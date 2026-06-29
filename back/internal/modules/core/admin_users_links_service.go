package core

import (
	"context"
	"strings"
)

// AddMembership adiciona um vinculo de cliente ao usuario SEM remover os outros.
// Escopo: o ator precisa administrar a account destino (CanManageAccount), senao
// 404 (ErrAccountNotFound — nao vaza existencia). Valida que a account existe,
// esta ativa e NAO e agencia (membership de cliente nao matricula em agencia).
// Retorna as memberships atualizadas (mesmo shape do GET).
func (s *AdminUserService) AddMembership(ctx context.Context, actorUserID, userID string, input AddMembershipInput) (AdminMembershipsResponse, error) {
	accountID := strings.TrimSpace(input.AccountID)
	if accountID == "" {
		return AdminMembershipsResponse{}, ErrAccountNotFound
	}
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		role = "owner"
	}
	switch role {
	case "owner", "director", "marketing":
	default:
		return AdminMembershipsResponse{}, ErrInvalidRole
	}

	// Escopo: fora do escopo -> 404 (mesma resposta de "nao existe").
	can, err := s.scope.CanManageAccount(ctx, actorUserID, accountID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	if !can {
		return AdminMembershipsResponse{}, ErrAccountNotFound
	}

	info, err := s.links.FindAccountLinkInfo(ctx, accountID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	if !info.Exists || !info.IsActive {
		return AdminMembershipsResponse{}, ErrAccountNotFound
	}
	if info.IsAgency {
		return AdminMembershipsResponse{}, ErrAccountIsAgency
	}

	if err := s.links.AddMembership(ctx, accountID, userID, role); err != nil {
		return AdminMembershipsResponse{}, err
	}
	return s.getMembershipsScoped(ctx, actorUserID, userID)
}

// RemoveMembership desativa o vinculo de cliente. Escopo: CanManageAccount senao
// 404; se a conta for agencia, exige autoridade de organizacao (M2). Convive com
// o PATCH no mesmo path (ServeMux 1.22 e method-aware).
func (s *AdminUserService) RemoveMembership(ctx context.Context, actorUserID, userID, accountID string) (AdminMembershipsResponse, error) {
	accountID = strings.TrimSpace(accountID)
	if err := s.ensureCanMutateAccountLink(ctx, actorUserID, accountID); err != nil {
		return AdminMembershipsResponse{}, err
	}
	if err := s.links.DeactivateMembership(ctx, accountID, userID); err != nil {
		return AdminMembershipsResponse{}, err
	}
	return s.getMembershipsScoped(ctx, actorUserID, userID)
}

// ensureCanMutateAccountLink e o gate de mutacao de vinculo numa account (M2,
// consistencia de autoridade). Sempre exige CanManageAccount (404 senao). Se a
// account for is_agency=true, mutar membro da agencia exige a MESMA autoridade do
// fluxo de organizacao: platform_admin OU agency_owner daquela org
// (CanManageOrganization) — NAO basta core.users.manage account-scoped. Sem isto
// um admin de cliente com permissao resolvida na conta-agencia mexeria em membros
// da agencia por fora do fluxo restrito. Fora de escopo -> 404 (nao vaza).
func (s *AdminUserService) ensureCanMutateAccountLink(ctx context.Context, actorUserID, accountID string) error {
	can, err := s.scope.CanManageAccount(ctx, actorUserID, accountID)
	if err != nil {
		return err
	}
	if !can {
		return ErrAccountNotFound
	}
	info, err := s.links.FindAccountLinkInfo(ctx, accountID)
	if err != nil {
		return err
	}
	if !info.Exists {
		return ErrAccountNotFound
	}
	if info.IsAgency {
		// Conta-agencia sem org (caso degenerado) -> so platform_admin pode mutar
		// (CanManageOrganization com id vazio nao casta para uuid).
		if strings.TrimSpace(info.OrganizationID) == "" {
			isAdmin, err := s.scope.IsPlatformAdmin(ctx, actorUserID)
			if err != nil {
				return err
			}
			if !isAdmin {
				return ErrAccountNotFound
			}
			return nil
		}
		canOrg, err := s.scope.CanManageOrganization(ctx, actorUserID, info.OrganizationID)
		if err != nil {
			return err
		}
		if !canOrg {
			return ErrAccountNotFound
		}
	}
	return nil
}

// LinkOrganization vincula uma agencia (organization) a um usuario existente.
// Escopo restrito: SO platform_admin OU agency_owner da PROPRIA org
// (CanManageOrganization) — admin de cliente -> 404. Virar membro de agencia da
// visao de TODOS os clientes da org -> exige confirmAgencyWideAccess=true senao
// 422 confirmation_required. Retorna o AdminUserView atualizado (mesmo shape do
// PATCH/PUT account) para o front aplicar na linha da tabela via applyPatch.
func (s *AdminUserService) LinkOrganization(ctx context.Context, actorUserID, userID, organizationID string, input LinkOrganizationInput) (AdminUserView, error) {
	organizationID = strings.TrimSpace(organizationID)
	orgRole := strings.TrimSpace(input.OrgRole)
	if orgRole == "" {
		orgRole = "agency_member"
	}
	switch orgRole {
	case "agency_owner", "agency_member":
	default:
		return AdminUserView{}, ErrInvalidOrgRole
	}

	can, err := s.scope.CanManageOrganization(ctx, actorUserID, organizationID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !can {
		return AdminUserView{}, ErrOrganizationNotFound
	}

	info, err := s.links.FindOrganizationLinkInfo(ctx, organizationID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !info.Exists || !info.IsActive {
		return AdminUserView{}, ErrOrganizationNotFound
	}

	if !input.ConfirmAgencyWideAccess {
		return AdminUserView{}, ErrConfirmationRequired
	}

	if err := s.links.LinkUserToOrganization(ctx, organizationID, userID, orgRole); err != nil {
		return AdminUserView{}, err
	}
	return s.repo.FindAdminUser(ctx, userID)
}

// UnlinkOrganization remove o vinculo de agencia. Mesmo escopo do link
// (platform_admin OU agency_owner da org). Safeguard ErrLastAgencyOwner (409):
// nao remove o ultimo agency_owner (deixaria a org sem dono). Retorna o
// AdminUserView atualizado (mesmo shape do PATCH/PUT account).
func (s *AdminUserService) UnlinkOrganization(ctx context.Context, actorUserID, userID, organizationID string) (AdminUserView, error) {
	organizationID = strings.TrimSpace(organizationID)
	can, err := s.scope.CanManageOrganization(ctx, actorUserID, organizationID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !can {
		return AdminUserView{}, ErrOrganizationNotFound
	}

	// Safeguard: nao remover o ultimo agency_owner ativo.
	isOwner, err := s.links.IsAgencyOwner(ctx, organizationID, userID)
	if err != nil {
		return AdminUserView{}, err
	}
	if isOwner {
		count, err := s.links.CountAgencyOwners(ctx, organizationID)
		if err != nil {
			return AdminUserView{}, err
		}
		if count <= 1 {
			return AdminUserView{}, ErrLastAgencyOwner
		}
	}

	if err := s.links.UnlinkUserFromOrganization(ctx, organizationID, userID); err != nil {
		return AdminUserView{}, err
	}
	return s.repo.FindAdminUser(ctx, userID)
}

// getMembershipsScoped carrega as memberships do usuario e FILTRA pelas accounts
// administraveis pelo ator (senao um agency_owner veria vinculos do usuario em
// clientes de OUTRA agencia — vazamento). platform_admin ve tudo.
func (s *AdminUserService) getMembershipsScoped(ctx context.Context, actorUserID, userID string) (AdminMembershipsResponse, error) {
	memberships, err := s.repo.GetMemberships(ctx, userID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	isAdmin, err := s.scope.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	if isAdmin {
		return AdminMembershipsResponse{Memberships: memberships}, nil
	}
	filtered := make([]AccountMembershipView, 0, len(memberships))
	for _, m := range memberships {
		can, err := s.scope.CanManageAccount(ctx, actorUserID, m.AccountID)
		if err != nil {
			return AdminMembershipsResponse{}, err
		}
		if can {
			filtered = append(filtered, m)
		}
	}
	return AdminMembershipsResponse{Memberships: filtered}, nil
}
