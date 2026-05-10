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
)
