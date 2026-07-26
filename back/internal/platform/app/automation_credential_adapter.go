package app

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/automation"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

// automationCredentialAdapter liga o Omni Chat ao cofre global account-scoped
// sem permitir que o modulo Automation consulte ou decifre messaging.*.
type automationCredentialAdapter struct {
	service func() *omnichannel.AIService
}

func (adapter automationCredentialAdapter) ResolveCredential(
	ctx context.Context,
	accountID string,
	credentialID string,
) (automation.OmniChatRuntimeCredential, error) {
	service := adapter.service()
	if service == nil {
		return automation.OmniChatRuntimeCredential{}, automation.ErrOmniChatCredentialUnavailable
	}
	credential, err := service.ResolveRuntimeCredential(ctx, accountID, credentialID)
	if err != nil {
		return automation.OmniChatRuntimeCredential{}, automation.ErrOmniChatCredentialUnavailable
	}
	return automation.OmniChatRuntimeCredential{
		ID: credential.ID, Provider: credential.Provider, APIKey: credential.APIKey,
	}, nil
}
