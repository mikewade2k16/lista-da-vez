package core

import "errors"

var (
	// ErrUserNotFound e retornado quando core.users nao tem o id.
	ErrUserNotFound = errors.New("core: user not found")

	// ErrAccountNotFound e retornado quando core.accounts nao tem o id.
	ErrAccountNotFound = errors.New("core: account not found")

	// ErrAccountNotMember e retornado quando o user nao tem membership ativa
	// na account informada (defesa em profundidade contra spoofing de
	// X-Account-Id).
	ErrAccountNotMember = errors.New("core: user is not a member of the account")

	// ErrOrganizationNotFound e retornado quando organization_id nao existe
	// (geralmente porque a account nao esta vinculada a nenhuma organization).
	ErrOrganizationNotFound = errors.New("core: organization not found")

	// ErrFeatureDisabled e retornado quando o endpoint v2 e chamado com
	// CORE_V2_ENABLED desligado. Defesa explicita em runtime.
	ErrFeatureDisabled = errors.New("core: CORE_V2_ENABLED feature flag is disabled")

	// ErrRoleNotFound e retornado quando core.roles nao tem o id ou code.
	ErrRoleNotFound = errors.New("core: role not found")

	// ErrTemplateNotFound e retornado quando core.role_templates nao tem o id.
	ErrTemplateNotFound = errors.New("core: role template not found")

	// ErrRoleCodeConflict e retornado quando ja existe um role com o mesmo code
	// na account (unique constraint account_id + code).
	ErrRoleCodeConflict = errors.New("core: role code already exists in this account")

	// ErrRoleIsLocked e retornado ao tentar deletar um role com is_locked=true.
	ErrRoleIsLocked = errors.New("core: role is locked and cannot be deleted")

	// ErrAccountSlugConflict e retornado quando ja existe uma account com o mesmo slug.
	ErrAccountSlugConflict = errors.New("core: account slug already exists")

	// ErrAdminUserNotFound e retornado ao criar account quando o adminEmail
	// nao corresponde a nenhum usuario ativo em core.users.
	ErrAdminUserNotFound = errors.New("core: admin user not found in core.users")

	// ErrUserEmailConflict e retornado ao criar/atualizar user quando o email
	// (lowercased) ja existe em core.users.
	ErrUserEmailConflict = errors.New("core: user email already exists")

	// ErrLastPlatformAdmin e retornado ao tentar rebaixar/desativar o ultimo
	// platform_admin ativo. Safeguard para evitar perder acesso total.
	ErrLastPlatformAdmin = errors.New("core: cannot remove the last active platform admin")

	// ErrInvalidRole e retornado quando o papel informado (criar user ou trocar
	// nivel de membership) nao e um valor aceito. Mapeado para HTTP 400.
	ErrInvalidRole = errors.New("core: invalid role")

	// ErrAccountIsAgency e retornado ao tentar MOVER um usuario para uma conta
	// is_agency=true via PUT /v1/admin/users/{id}/account. Esse endpoint e so
	// para conta-cliente. Mapeado para HTTP 400.
	ErrAccountIsAgency = errors.New("core: target account is an agency")

	// ErrOrganizationSlugConflict e retornado quando ja existe uma organization
	// com o mesmo slug.
	ErrOrganizationSlugConflict = errors.New("core: organization slug already exists")
)
