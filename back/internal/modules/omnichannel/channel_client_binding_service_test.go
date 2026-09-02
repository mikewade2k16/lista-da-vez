package omnichannel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	bindingTestAccount  = "10000000-0000-0000-0000-000000000001"
	bindingTestClient   = "20000000-0000-0000-0000-000000000002"
	bindingTestOther    = "30000000-0000-0000-0000-000000000003"
	bindingTestResource = "40000000-0000-0000-0000-000000000004"
	bindingTestID       = "50000000-0000-0000-0000-000000000005"
)

type bindingPermissionFake struct {
	key string
	err error
}

func (f *bindingPermissionFake) requirePermission(_ context.Context, _ string, _ auth.Principal, key string) error {
	f.key = key
	return f.err
}

func (f *bindingPermissionFake) requireInstanceAccess(context.Context, string, string, string, string, InstanceGrantLevel) error {
	return f.err
}

func (f *bindingPermissionFake) assertConversationAccess(context.Context, string, string, string, string, InstanceGrantLevel) error {
	return f.err
}

type bindingClientCatalogFake struct {
	items []AutomationClientRef
	err   error
}

func (f bindingClientCatalogFake) ListAccessible(context.Context, auth.Principal) ([]AutomationClientRef, error) {
	return f.items, f.err
}

type bindingRepoFake struct {
	eligible       map[string]bool
	resourceExists bool
	resourceActive bool
	createInput    channelClientBindingWrite
	rows           []ChannelClientBindingView
	view           ChannelClientBindingView
}

func (f *bindingRepoFake) ListChannelClientBindings(context.Context, string, ChannelClientBindingFilter) ([]ChannelClientBindingView, error) {
	return f.rows, nil
}
func (f *bindingRepoFake) GetChannelClientBinding(context.Context, string, string) (ChannelClientBindingView, error) {
	if f.view.ID == "" {
		return ChannelClientBindingView{}, ErrNotFound
	}
	return f.view, nil
}
func (f *bindingRepoFake) ChannelBindingClientEligible(_ context.Context, _, clientID string) (bool, error) {
	return f.eligible[clientID], nil
}
func (f *bindingRepoFake) ChannelBindingResourceExists(context.Context, string, string, string) (bool, bool, error) {
	return f.resourceExists, f.resourceActive, nil
}
func (f *bindingRepoFake) CreateChannelClientBinding(_ context.Context, _ string, in channelClientBindingWrite) (string, error) {
	f.createInput = in
	return bindingTestID, nil
}
func (f *bindingRepoFake) ReassignChannelClientBinding(context.Context, string, string, string, ReassignChannelClientBindingInput, string) (string, error) {
	return bindingTestID, nil
}
func (f *bindingRepoFake) EndChannelClientBinding(context.Context, string, string, string, EndChannelClientBindingInput, string) (string, error) {
	return bindingTestID, nil
}
func (f *bindingRepoFake) ListChannelClientBindingExceptions(context.Context, string) ([]ChannelClientBindingExceptionView, error) {
	return nil, nil
}
func (f *bindingRepoFake) GetChannelClientBindingPolicy(context.Context, string) (ChannelClientBindingPolicyView, error) {
	return ChannelClientBindingPolicyView{
		ChannelBindingMode: "shadow", CustomerIntelligenceMode: "off",
		CustomerIntelligenceFailurePolicy: "retry_then_handoff", Revision: 1,
	}, nil
}
func (f *bindingRepoFake) UpdateChannelClientBindingPolicy(_ context.Context, _ string, in ChannelClientBindingPolicyInput) (ChannelClientBindingPolicyView, error) {
	return ChannelClientBindingPolicyView{
		ChannelBindingMode:                in.ChannelBindingMode,
		CustomerIntelligenceMode:          in.CustomerIntelligenceMode,
		CustomerIntelligenceFailurePolicy: in.CustomerIntelligenceFailurePolicy,
		Revision:                          in.ExpectedRevision + 1,
	}, nil
}
func (f *bindingRepoFake) CreateChannelClientBindingRepairPreview(context.Context, string, auth.Principal, ChannelClientBindingRepairPreviewInput, string) (ChannelClientBindingRepairJobView, error) {
	return ChannelClientBindingRepairJobView{}, nil
}
func (f *bindingRepoFake) ApplyChannelClientBindingRepair(context.Context, string, auth.Principal, ChannelClientBindingRepairApplyInput, string) (ChannelClientBindingRepairJobView, error) {
	return ChannelClientBindingRepairJobView{}, nil
}
func (f *bindingRepoFake) GetChannelClientBindingRepairJob(context.Context, string, string) (ChannelClientBindingRepairJobView, error) {
	return ChannelClientBindingRepairJobView{}, nil
}

func TestChannelClientBindingCreateUsesPermissionCatalogAndServerScope(t *testing.T) {
	fixedNow := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	repo := &bindingRepoFake{
		eligible:       map[string]bool{bindingTestClient: true},
		resourceExists: true,
		resourceActive: true,
		view: ChannelClientBindingView{
			ID: bindingTestID, ClientAccountID: bindingTestClient,
		},
	}
	permissions := &bindingPermissionFake{}
	svc := NewChannelClientBindingService(
		repo,
		permissions,
		bindingClientCatalogFake{items: []AutomationClientRef{{ID: bindingTestClient}}},
	)
	svc.now = func() time.Time { return fixedNow }

	out, err := svc.Create(context.Background(), bindingTestAccount, auth.Principal{
		UserID:    "60000000-0000-0000-0000-000000000006",
		AccountID: bindingTestAccount,
	}, CreateChannelClientBindingInput{
		ClientAccountID:   bindingTestClient,
		Channel:           "whatsapp",
		ChannelResourceID: bindingTestResource,
		Reason:            "Número dedicado ao cliente",
		IdempotencyKey:    "binding:create:1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if out.ID != bindingTestID {
		t.Fatalf("id=%q", out.ID)
	}
	if permissions.key != channelBindingManagePermission {
		t.Fatalf("permission=%q", permissions.key)
	}
	if repo.createInput.ClientAccountID != bindingTestClient ||
		repo.createInput.Channel != "WHATSAPP" ||
		repo.createInput.ResourceID != bindingTestResource ||
		repo.createInput.EffectiveFrom != fixedNow {
		t.Fatalf("write inesperado: %#v", repo.createInput)
	}
	if len(repo.createInput.RequestHash) != 64 {
		t.Fatalf("request hash invalido: %q", repo.createInput.RequestHash)
	}
}

func TestChannelClientBindingCreateFailsClosedOutsideClientCatalog(t *testing.T) {
	repo := &bindingRepoFake{
		eligible:       map[string]bool{bindingTestClient: true},
		resourceExists: true,
		resourceActive: true,
	}
	svc := NewChannelClientBindingService(
		repo,
		&bindingPermissionFake{},
		bindingClientCatalogFake{items: []AutomationClientRef{{ID: bindingTestOther}}},
	)
	_, err := svc.Create(context.Background(), bindingTestAccount, auth.Principal{}, CreateChannelClientBindingInput{
		ClientAccountID:   bindingTestClient,
		Channel:           "WHATSAPP",
		ChannelResourceID: bindingTestResource,
		Reason:            "motivo",
		IdempotencyKey:    "key",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("esperava 404 sem enumeracao, recebeu %v", err)
	}
	if repo.createInput.ClientAccountID != "" {
		t.Fatal("repository de escrita nao deveria ser chamado")
	}
}

func TestChannelClientBindingListFiltersClientsNotAccessibleToCaller(t *testing.T) {
	repo := &bindingRepoFake{
		eligible: map[string]bool{bindingTestClient: true, bindingTestOther: true},
		rows: []ChannelClientBindingView{
			{ID: bindingTestID, ClientAccountID: bindingTestClient},
			{ID: "70000000-0000-0000-0000-000000000007", ClientAccountID: bindingTestOther},
		},
	}
	svc := NewChannelClientBindingService(
		repo,
		&bindingPermissionFake{},
		bindingClientCatalogFake{items: []AutomationClientRef{{ID: bindingTestClient}}},
	)
	out, err := svc.List(context.Background(), bindingTestAccount, auth.Principal{}, ChannelClientBindingFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].ClientAccountID != bindingTestClient {
		t.Fatalf("escopo vazou: %#v", out.Items)
	}
}

func TestChannelClientBindingPolicyIsPanelConfigurableButClosed(t *testing.T) {
	svc := NewChannelClientBindingService(
		&bindingRepoFake{},
		&bindingPermissionFake{},
		bindingClientCatalogFake{},
	)
	out, err := svc.UpdatePolicy(context.Background(), bindingTestAccount, auth.Principal{}, ChannelClientBindingPolicyInput{
		ChannelBindingMode:                " ENFORCED ",
		CustomerIntelligenceMode:          " SHADOW ",
		CustomerIntelligenceFailurePolicy: " RETRY_THEN_HANDOFF ",
		ExpectedRevision:                  3,
	})
	if err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}
	if out.ChannelBindingMode != "enforced" ||
		out.CustomerIntelligenceMode != "shadow" ||
		out.CustomerIntelligenceFailurePolicy != "retry_then_handoff" ||
		out.Revision != 4 {
		t.Fatalf("policy inesperada: %#v", out)
	}

	_, err = svc.UpdatePolicy(context.Background(), bindingTestAccount, auth.Principal{}, ChannelClientBindingPolicyInput{
		ChannelBindingMode:                "prompt_decides",
		CustomerIntelligenceMode:          "on",
		CustomerIntelligenceFailurePolicy: "legacy_fallback",
		ExpectedRevision:                  4,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("modo fora do enum deveria falhar: %v", err)
	}

	_, err = svc.UpdatePolicy(context.Background(), bindingTestAccount, auth.Principal{}, ChannelClientBindingPolicyInput{
		ChannelBindingMode:                "shadow",
		CustomerIntelligenceMode:          "on",
		CustomerIntelligenceFailurePolicy: "prompt_decides",
		ExpectedRevision:                  4,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("failure policy fora do enum deveria falhar: %v", err)
	}
}

func TestChannelClientBindingPolicyDefaultsFailurePolicyForRollingClients(t *testing.T) {
	svc := NewChannelClientBindingService(
		&bindingRepoFake{},
		&bindingPermissionFake{},
		bindingClientCatalogFake{},
	)
	out, err := svc.UpdatePolicy(
		context.Background(),
		bindingTestAccount,
		auth.Principal{},
		ChannelClientBindingPolicyInput{
			ChannelBindingMode:       "shadow",
			CustomerIntelligenceMode: "off",
			ExpectedRevision:         1,
		},
	)
	if err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}
	if out.CustomerIntelligenceFailurePolicy != "retry_then_handoff" {
		t.Fatalf("default inesperado: %#v", out)
	}
}
