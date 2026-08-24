package calendar

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
)

const (
	testAgencyID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testClientOne = "11111111-1111-1111-1111-111111111111"
	testClientTwo = "22222222-2222-2222-2222-222222222222"
	testForeign   = "ffffffff-ffff-ffff-ffff-ffffffffffff"
)

func TestResolveChatContextIntersectsCalendarOrganizationAndRBAC(t *testing.T) {
	t.Parallel()
	store := &assistantAccessStore{
		scope: CalendarScope{
			StorageAccountID: testAgencyID,
			CanSelect:        true,
			Clients: []CalendarScopeClient{
				{ID: testClientOne, Name: "Cliente canonico 1"},
				{ID: testClientTwo, Name: "Cliente canonico 2"},
			},
		},
		isAgency: true,
	}
	service := NewService(store, nil).WithClientScope(assistantScopeLister{clients: []tenants.TenantView{
		{ID: testClientTwo, Name: "Nome forjado pelo catalogo"},
		{ID: testForeign, Name: "Outra organizacao"},
	}})

	access, clients, err := service.resolveChatContext(context.Background(), auth.Principal{UserID: testClientOne}, testAgencyID)
	if err != nil {
		t.Fatal(err)
	}
	if !access.IsAgency || access.StorageAccountID != testAgencyID {
		t.Fatalf("acesso canonico inesperado: %#v", access)
	}
	if len(clients) != 1 || clients[0].ID != testClientTwo || clients[0].Name != "Cliente canonico 2" {
		t.Fatalf("intersecao organization+RBAC inesperada: %#v", clients)
	}
	if len(access.VisibleClientIDs) != 1 || access.VisibleClientIDs[0] != testClientTwo {
		t.Fatalf("ids visiveis inesperados: %#v", access.VisibleClientIDs)
	}
}

func TestResolveChatContextKeepsClientAccountLocked(t *testing.T) {
	t.Parallel()
	store := &assistantAccessStore{
		scope: CalendarScope{
			StorageAccountID: testAgencyID,
			LockedClientID:   testClientOne,
			Clients:          []CalendarScopeClient{{ID: testClientOne, Name: "Cliente 1"}},
		},
		// Mesmo um agency_owner/platform admin nao transforma uma account-cliente
		// ativa em seletor multi-cliente.
		isAgency: true,
	}
	service := NewService(store, nil).WithClientScope(assistantScopeLister{clients: []tenants.TenantView{
		{ID: testClientOne, Name: "Cliente 1"}, {ID: testForeign, Name: "Outra org"},
	}})

	access, _, err := service.resolveChatContext(context.Background(), auth.Principal{UserID: testClientOne}, testClientOne)
	if err != nil {
		t.Fatal(err)
	}
	if access.IsAgency || access.canSelectScope() || access.lockedClientID() != testClientOne {
		t.Fatalf("account-cliente deveria permanecer travada: %#v", access)
	}
	if access.calendarAccountID(testClientOne) != testAgencyID {
		t.Fatalf("storage do calendario nao foi reaproveitada: %#v", access)
	}
}

func TestAssistantSurfaceRequiresItsPrimaryCapability(t *testing.T) {
	t.Parallel()
	calendarOnly := []AssistantCapability{{Module: "calendar", Available: true, EffectiveMode: assistantModeRead}}
	if assistantSurfaceCapabilityAllowed(AssistantSurfaceMetaAds, calendarOnly) {
		t.Fatal("surface meta_ads nao pode herdar apenas calendar.read")
	}
	if !assistantSurfaceCapabilityAllowed(AssistantSurfaceCalendar, calendarOnly) ||
		!assistantSurfaceCapabilityAllowed(AssistantSurfaceGlobal, calendarOnly) {
		t.Fatal("calendar/global deveriam aceitar a capability efetiva")
	}
}

func TestConversationAccessIsRevokedAcrossListGetAndDelete(t *testing.T) {
	t.Parallel()
	conversation := ChatConversation{
		ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", AccountID: testAgencyID,
		CreatedByUserID: testClientOne, EntrySurface: AssistantSurfaceMetaAds,
		ScopeMode: chatScopeClient, ScopeClientID: stringPointer(testClientOne),
	}
	store := &assistantAccessStore{
		scope: CalendarScope{StorageAccountID: testAgencyID, CanSelect: true,
			Clients: []CalendarScopeClient{{ID: testClientOne, Name: "Cliente 1"}}},
		conversations: []ChatConversation{conversation}, conversation: conversation,
	}
	service := assistantAccessService(store, map[string]map[string]string{
		AssistantSurfaceMetaAds: {"calendar": assistantModeRead, "meta_ads": assistantModeOff},
	})
	principal := assistantAccessPrincipal()

	listed, err := service.ListChatConversations(context.Background(), testAgencyID, principal)
	if err != nil || len(listed) != 0 {
		t.Fatalf("list deve esconder surface revogada: %#v, %v", listed, err)
	}
	if _, err = service.GetChatConversation(context.Background(), testAgencyID, conversation.ID, principal); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get revogado deveria ser 404, veio %v", err)
	}
	if err = service.DeleteChatConversation(context.Background(), testAgencyID, conversation.ID, principal); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete revogado deveria ser 404, veio %v", err)
	}
	if store.softDeleteCalls != 0 {
		t.Fatalf("delete nao autorizado chegou ao store: %d", store.softDeleteCalls)
	}
}

func TestListConversationsHidesSavedScopeAfterClientAccessLoss(t *testing.T) {
	t.Parallel()
	conversation := ChatConversation{
		ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", AccountID: testAgencyID,
		CreatedByUserID: testClientOne, EntrySurface: AssistantSurfaceGlobal,
		ScopeMode: chatScopeClient, ScopeClientID: stringPointer(testClientTwo),
		Title: "Titulo que nao pode vazar",
	}
	store := &assistantAccessStore{
		scope: CalendarScope{StorageAccountID: testAgencyID, CanSelect: true,
			Clients: []CalendarScopeClient{{ID: testClientOne, Name: "Cliente 1"}}},
		conversations: []ChatConversation{conversation},
	}
	service := assistantAccessService(store, map[string]map[string]string{
		AssistantSurfaceGlobal: {"calendar": assistantModeRead},
	})

	listed, err := service.ListChatConversations(context.Background(), testAgencyID, assistantAccessPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("list vazou conversa de cliente revogado: %#v", listed)
	}
}

func TestGlobalHistoryDropsAssistantMessagesWithRevokedMetaResources(t *testing.T) {
	t.Parallel()
	conversation := ChatConversation{
		ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", AccountID: testAgencyID,
		CreatedByUserID: testClientOne, EntrySurface: AssistantSurfaceGlobal,
		ScopeMode: chatScopeClient, ScopeClientID: stringPointer(testClientOne),
	}
	store := &assistantAccessStore{
		scope: CalendarScope{StorageAccountID: testAgencyID, CanSelect: true,
			Clients: []CalendarScopeClient{{ID: testClientOne, Name: "Cliente 1"}}},
		conversation: conversation,
		messages: []ChatMessage{
			{ID: "user", Role: chatRoleUser, Content: "Mostre a campanha"},
			{ID: "meta-with-card", Role: chatRoleAssistant, Content: "Campanha secreta",
				Resources: []AssistantResource{{ID: "meta_campaign:one", Kind: assistantResourceMetaCampaign, Title: "Campanha"}}},
			{ID: "meta-without-card", Role: chatRoleAssistant, Content: "Outra campanha secreta",
				ContextModules: []string{"meta_ads"}},
			{ID: "safe", Role: chatRoleAssistant, Content: "Resposta geral"},
		},
	}
	service := assistantAccessService(store, map[string]map[string]string{
		AssistantSurfaceGlobal: {"calendar": assistantModeRead, "meta_ads": assistantModeOff},
	})

	detail, err := service.GetChatConversation(context.Background(), testAgencyID, conversation.ID, assistantAccessPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 2 || detail.Messages[0].ID != "user" || detail.Messages[1].ID != "safe" {
		t.Fatalf("mensagem Meta revogada vazou no GET: %#v", detail.Messages)
	}
	history := toHistory(filterChatMessagesForCapabilities(store.messages,
		[]AssistantCapability{{Module: "calendar", Available: true, EffectiveMode: assistantModeRead}}))
	if len(history) != 2 || history[1].Content != "Resposta geral" {
		t.Fatalf("mensagem Meta revogada vazou no history: %#v", history)
	}
}

func TestGlobalHistoryInvalidatesEveryRevokedContextModule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		revoked string
		message ChatMessage
	}{
		{name: "calendar marker", revoked: "calendar", message: ChatMessage{Role: chatRoleAssistant, ContextModules: []string{"calendar"}}},
		{name: "tasks marker", revoked: "tasks", message: ChatMessage{Role: chatRoleAssistant, ContextModules: []string{"tasks"}}},
		{name: "users marker", revoked: "users", message: ChatMessage{Role: chatRoleAssistant, ContextModules: []string{"users"}}},
		{name: "meta marker without card", revoked: "meta_ads", message: ChatMessage{Role: chatRoleAssistant, ContextModules: []string{"meta_ads"}}},
		{name: "legacy calendar item", revoked: "calendar", message: ChatMessage{Role: chatRoleAssistant, CalendarItems: []AIContextEvent{{ID: "event"}}}},
		{name: "legacy meta resource", revoked: "meta_ads", message: ChatMessage{Role: chatRoleAssistant,
			Resources: []AssistantResource{{ID: "meta_campaign:one", Kind: assistantResourceMetaCampaign, Title: "Campanha"}}}},
		{name: "legacy task proposal", revoked: "tasks", message: ChatMessage{Role: chatRoleAssistant,
			Proposals: []StoredProposal{{Kind: "task", Fields: ChatProposalFields{Title: "Task"}}}}},
		{name: "legacy people-backed proposal", revoked: "users", message: ChatMessage{Role: chatRoleAssistant,
			Proposals: []StoredProposal{{Kind: "event", Fields: ChatProposalFields{ResponsibleID: testClientOne}}}}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.message.Content = "dado protegido"
			filtered := filterChatMessagesForCapabilities([]ChatMessage{tt.message}, assistantCapabilitiesExcept(tt.revoked))
			if len(filtered) != 0 {
				t.Fatalf("modulo %s revogado ainda devolveu mensagem: %#v", tt.revoked, filtered)
			}
		})
	}
}

type assistantScopeLister struct{ clients []tenants.TenantView }

func (l assistantScopeLister) ListAccessible(context.Context, auth.Principal, tenants.ListInput) ([]tenants.TenantView, error) {
	return l.clients, nil
}

type assistantAccessStore struct {
	calendarStore
	scope           CalendarScope
	isAgency        bool
	conversations   []ChatConversation
	conversation    ChatConversation
	messages        []ChatMessage
	softDeleteCalls int
}

func (s *assistantAccessStore) ResolveCalendarScope(context.Context, string) (CalendarScope, error) {
	return s.scope, nil
}

func (s *assistantAccessStore) IsAgencyOfAccount(context.Context, string, string) (bool, error) {
	return s.isAgency, nil
}

func (s *assistantAccessStore) ListConversations(context.Context, string, string, bool) ([]ChatConversation, error) {
	return s.conversations, nil
}

func (s *assistantAccessStore) GetConversation(context.Context, string, string) (ChatConversation, error) {
	return s.conversation, nil
}

func (s *assistantAccessStore) ListMessages(context.Context, string, string) ([]ChatMessage, error) {
	return s.messages, nil
}

func (s *assistantAccessStore) SoftDeleteConversation(context.Context, string, string) error {
	s.softDeleteCalls++
	return nil
}

func assistantAccessService(store *assistantAccessStore, matrix map[string]map[string]string) *Service {
	return NewService(store, nil).
		WithClientScope(assistantScopeLister{clients: []tenants.TenantView{{ID: testClientOne, Name: "Cliente 1"}}}).
		WithAssistantRuntimeProvider(func(context.Context, string) (AssistantRuntime, error) {
			return AssistantRuntime{
				Enabled: true, Provider: "openai", Model: "gpt-test", APIKey: "secret",
				SurfaceModules: matrix,
			}, nil
		}).
		WithAssistantModuleAvailability(func(context.Context, string, string) (bool, error) { return true, nil })
}

func assistantAccessPrincipal() auth.Principal {
	return auth.Principal{
		UserID: testClientOne, Role: auth.RoleMarketing, PermissionsResolved: true,
		Permissions: []string{"calendar.view"},
	}
}

func assistantCapabilitiesExcept(revoked string) []AssistantCapability {
	modules := []string{"calendar", "tasks", "users", "meta_ads"}
	out := make([]AssistantCapability, 0, len(modules))
	for _, module := range modules {
		mode := assistantModeRead
		available := true
		if module == revoked {
			mode = assistantModeOff
			available = false
		}
		out = append(out, AssistantCapability{Module: module, EffectiveMode: mode, Available: available})
	}
	return out
}

func stringPointer(value string) *string { return &value }
