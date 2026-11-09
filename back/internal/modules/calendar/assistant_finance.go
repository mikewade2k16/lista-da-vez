package calendar

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const AssistantSurfaceFinance = "finance"

type FinanceAssistantContextProvider func(context.Context, string, auth.Principal, string, string) (any, error)
type financeSurfaceContextKey struct{}

func WithFinanceAssistant(runtime AssistantRuntimeProvider, provider FinanceAssistantContextProvider) Option {
	return func(m *Module) { m.financeAssistantRuntime = runtime; m.financeAssistantContext = provider }
}
func (s *Service) WithFinanceAssistant(runtime AssistantRuntimeProvider, provider FinanceAssistantContextProvider) *Service {
	s.financeAssistantRuntime = runtime
	s.financeAssistantContext = provider
	return s
}
func financeSurfaceContext(ctx context.Context) bool {
	enabled, _ := ctx.Value(financeSurfaceContextKey{}).(bool)
	return enabled
}
func financeChatAccess(p auth.Principal, account string) (ChatAccess, error) {
	allowed := p.Role == auth.RolePlatformAdmin || (p.PermissionsResolved && (containsPermission(p.Permissions, "finance.sheets.view") || (p.Role == auth.RoleOwner && containsPermission(p.Permissions, "core.account.manage"))))
	if !allowed || strings.TrimSpace(account) == "" {
		return ChatAccess{}, ErrForbidden
	}
	return ChatAccess{StorageAccountID: account, VisibleClientIDs: []string{account}, VisibleClients: []ChatScopeClient{{ID: account, Name: "Conta financeira ativa"}}}, nil
}

// Finance local transcription is independent of conversation model activation.
type FinanceTranscriptionAuthorizer func(context.Context, string, auth.Principal) error

func WithFinanceTranscriptionAuthorization(authorize FinanceTranscriptionAuthorizer) Option {
	return func(m *Module) { m.financeTranscriptionAuthorize = authorize }
}
func (s *Service) WithFinanceTranscriptionAuthorization(authorize FinanceTranscriptionAuthorizer) *Service {
	s.financeTranscriptionAuthorize = authorize
	return s
}
