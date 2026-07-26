package app

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/transcriptions"
)

// queueTranscriptionCredentialAdapter is the composition boundary between
// Queue and the account-scoped AI credential vault owned by Omnichannel.
type queueTranscriptionCredentialAdapter struct {
	service func() *omnichannel.AIService
}

func (adapter queueTranscriptionCredentialAdapter) ResolveCredential(
	ctx context.Context,
	accountID string,
	credentialID string,
) (transcriptions.RuntimeCredential, error) {
	service := adapter.service()
	if service == nil {
		return transcriptions.RuntimeCredential{}, transcriptions.ErrCredentialUnavailable
	}
	credential, err := service.ResolveRuntimeCredential(ctx, accountID, credentialID)
	if err != nil {
		return transcriptions.RuntimeCredential{}, transcriptions.ErrCredentialUnavailable
	}
	return transcriptions.RuntimeCredential{
		ID:       credential.ID,
		Provider: credential.Provider,
		APIKey:   credential.APIKey,
	}, nil
}
