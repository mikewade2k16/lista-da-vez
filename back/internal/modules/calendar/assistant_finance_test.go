package calendar

import (
	"context"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFinanceScopeAndConversationPrivacy(t *testing.T) {
	p := auth.Principal{UserID: testClientOne, AccountID: testAgencyID, PermissionsResolved: true, Permissions: []string{"finance.sheets.view"}}
	access, err := financeChatAccess(p, testAgencyID)
	if err != nil || access.IsAgency || access.lockedClientID() != testAgencyID {
		t.Fatalf("scope: %+v %v", access, err)
	}
	p.Permissions = nil
	if _, err = financeChatAccess(p, testAgencyID); !errors.Is(err, ErrForbidden) {
		t.Fatal("unprivileged finance access")
	}
	store := &assistantAccessStore{conversation: ChatConversation{ID: testClientTwo, EntrySurface: AssistantSurfaceFinance, CreatedByUserID: testClientTwo}}
	svc := NewService(store, nil)
	if _, err = svc.authorizeConversation(context.Background(), ChatAccess{IsAgency: true}, testAgencyID, testClientTwo, testClientOne); !errors.Is(err, ErrNotFound) {
		t.Fatal("agency bypassed personal finance history")
	}
	store.conversation.CreatedByUserID = testClientOne
	if _, err = svc.authorizeConversation(context.Background(), ChatAccess{IsAgency: true}, testAgencyID, testClientTwo, testClientOne); err != nil {
		t.Fatal(err)
	}
}
func TestFinanceRuntimeIsIndependentAndReadOnly(t *testing.T) {
	p := auth.Principal{PermissionsResolved: true, Permissions: []string{"finance.sheets.view"}}
	svc := NewService(nil, nil).WithFinanceAssistant(func(context.Context, string) (AssistantRuntime, error) {
		return AssistantRuntime{Enabled: true, APIKey: "test-only", Provider: "openai", Model: "test", SurfaceModules: map[string]map[string]string{"finance": {"finance": "write", "calendar": "off"}}}, nil
	}, nil).WithAssistantModuleAvailability(func(context.Context, string, string) (bool, error) { return true, nil })
	_, caps, err := svc.resolveAssistantRuntime(context.Background(), testAgencyID, AssistantSurfaceFinance, p)
	if err != nil || assistantCapabilityMode(caps, "finance") != "read" || !assistantSurfaceCapabilityAllowed(AssistantSurfaceFinance, caps) {
		t.Fatalf("capabilities %v %v", caps, err)
	}
	if assistantCapabilityMode(caps, "calendar") != "off" {
		t.Fatal("calendar leaked into finance context")
	}
	ctx := context.WithValue(context.Background(), financeSurfaceContextKey{}, true)
	if _, err = svc.buildChatContext(ctx, testAgencyID, p, ChatAccess{}, chatScopeClient, testAgencyID, "2026-11", assistantContextPolicy{}); err != nil {
		t.Fatal("finance accessed Calendar storage")
	}
}

func TestFinancePreflightDoesNotRequireCalendarScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	svc := NewService(nil, nil).WithChat(server.URL+"/webhook/calendar-chat", "").WithFinanceAssistant(func(context.Context, string) (AssistantRuntime, error) {
		return AssistantRuntime{Enabled: true, APIKey: "test", Provider: "openai", Model: "test", SurfaceModules: map[string]map[string]string{"finance": {"finance": "read"}}}, nil
	}, nil).WithAssistantModuleAvailability(func(_ context.Context, _ string, module string) (bool, error) { return module == "finance", nil })
	principal := auth.Principal{UserID: testClientOne, AccountID: testAgencyID, PermissionsResolved: true, Permissions: []string{"finance.sheets.view"}}
	status, err := svc.AssistantChatStatus(context.Background(), testAgencyID, principal, "finance", "client", testAgencyID)
	if err != nil || !status.Available {
		t.Fatalf("finance preflight %+v %v", status, err)
	}
}
