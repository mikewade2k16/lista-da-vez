package omnichannel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	automationTestAccount  = "11111111-1111-4111-8111-111111111111"
	automationTestClient   = "22222222-2222-4222-8222-222222222222"
	automationOtherClient  = "33333333-3333-4333-8333-333333333333"
	automationTestInstance = "44444444-4444-4444-8444-444444444444"
	automationTestAgent    = "55555555-5555-4555-8555-555555555555"
	automationTestConv     = "77777777-7777-4777-8777-777777777777"
)

type automationPermissionFake struct{ err error }

func (f automationPermissionFake) requirePermission(context.Context, string, auth.Principal, string) error {
	return f.err
}

func (f automationPermissionFake) requireInstanceAccess(context.Context, string, string, string, string, InstanceGrantLevel) error {
	return f.err
}

func (f automationPermissionFake) assertConversationAccess(context.Context, string, string, string, string, InstanceGrantLevel) error {
	return f.err
}

type automationCatalogFake struct {
	clients []AutomationClientRef
	err     error
}

func (f automationCatalogFake) ListAccessible(context.Context, auth.Principal) ([]AutomationClientRef, error) {
	return f.clients, f.err
}

type automationContextFake struct {
	profile   AutomationBusinessContext
	available bool
	err       error
}

type automationDomainFake struct {
	request HandoffRequest
	calls   int
	err     error
}

func (f *automationDomainFake) RequestAutomationHandoff(_ context.Context, _, _, _ string, in HandoffRequest) (HandoffView, error) {
	f.calls++
	f.request = in
	return HandoffView{}, f.err
}

func (f automationContextFake) Load(context.Context, string, string) (AutomationBusinessContext, bool, error) {
	return f.profile, f.available, f.err
}

type automationRepositoryFake struct {
	rows          []automationProfileRow
	row           automationProfileRow
	getErr        error
	readiness     automationBindingReadiness
	write         automationProfileWrite
	putCalls      int
	interventions []automationInterventionRow
	attendances   []automationAttendanceRow
	scope         automationConversationScope
	dispatch      AIDispatchRecord
	actionErr     error
}

func (f *automationRepositoryFake) ListAutomationInterventions(context.Context, string, string, int) ([]automationInterventionRow, error) {
	return f.interventions, nil
}

func (f *automationRepositoryFake) ListAutomationAttendances(context.Context, string, string, int) ([]automationAttendanceRow, error) {
	return f.attendances, f.actionErr
}

func (f *automationRepositoryFake) AutomationConversationScope(context.Context, string, string) (automationConversationScope, error) {
	return f.scope, f.actionErr
}

func (f *automationRepositoryFake) ReplayAutomationWithAI(context.Context, string, string, string, string) (AIDispatchRecord, error) {
	return f.dispatch, f.actionErr
}

func (f *automationRepositoryFake) ListAutomationProfiles(context.Context, string) ([]automationProfileRow, error) {
	return f.rows, nil
}

func (f *automationRepositoryFake) GetAutomationProfile(context.Context, string, string) (automationProfileRow, error) {
	return f.row, f.getErr
}

func (f *automationRepositoryFake) AutomationBindingReadiness(context.Context, string, string, string, string) (automationBindingReadiness, error) {
	return f.readiness, nil
}

func (f *automationRepositoryFake) UpsertAutomationProfile(_ context.Context, _, _ string, _ string, in automationProfileWrite) (automationProfileRow, error) {
	f.putCalls++
	f.write = in
	return f.row, nil
}

func automationTestPrincipal() auth.Principal {
	return auth.Principal{UserID: "66666666-6666-4666-8666-666666666666", AccountID: automationTestAccount}
}

func automationTestClientRef() AutomationClientRef {
	return AutomationClientRef{ID: automationTestClient, Slug: "cliente", Name: "Cliente"}
}

func TestNormalizeAutomationClosePolicyUsesSafeDefaults(t *testing.T) {
	policy, err := normalizeAutomationClosePolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	if policy.AutoCloseEnabled || policy.MinimumConfidence != 0.90 || !policy.RequireAllRequiredFields ||
		!policy.BlockOnHumanRequest || !policy.BlockSensitiveTopics || !policy.ValidGenerationRequired {
		t.Fatalf("defaults inseguros: %+v", policy)
	}
}

func TestAutomationPutRejectsClientOutsideVisibleScope(t *testing.T) {
	repo := &automationRepositoryFake{}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	_, err := svc.PutProfile(context.Background(), automationTestAccount, automationTestPrincipal(), automationOtherClient, AutomationProfileInput{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, esperado not found", err)
	}
	if repo.putCalls != 0 {
		t.Fatalf("store chamado fora do escopo: %d", repo.putCalls)
	}
}

func TestAutomationPutRequiresReadyBindingWhenEnabling(t *testing.T) {
	repo := &automationRepositoryFake{readiness: automationBindingReadiness{
		InstanceFound: true, AgentFound: true, InstanceReady: true, AgentReady: false, BindingReady: true,
	}}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	_, err := svc.PutProfile(context.Background(), automationTestAccount, automationTestPrincipal(), automationTestClient,
		AutomationProfileInput{WhatsAppInstanceID: automationTestInstance, AIAgentID: automationTestAgent, Enabled: true})
	if !errors.Is(err, ErrAutomationNotReady) {
		t.Fatalf("err=%v, esperado automation not ready", err)
	}
	if repo.putCalls != 0 {
		t.Fatalf("perfil inseguro foi persistido: %d", repo.putCalls)
	}
}

func TestAutomationPutRejectsBindingFromAnotherClient(t *testing.T) {
	repo := &automationRepositoryFake{readiness: automationBindingReadiness{
		InstanceFound: true, AgentFound: true, InstanceReady: true, AgentReady: true, BindingReady: false,
	}}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	_, err := svc.PutProfile(context.Background(), automationTestAccount, automationTestPrincipal(), automationTestClient,
		AutomationProfileInput{WhatsAppInstanceID: automationTestInstance, AIAgentID: automationTestAgent, Enabled: true})
	if !errors.Is(err, ErrAutomationBindingMismatch) {
		t.Fatalf("err=%v, esperado binding mismatch", err)
	}
	if repo.putCalls != 0 {
		t.Fatalf("perfil divergente foi persistido: %d", repo.putCalls)
	}
}

func TestAutomationPutPersistsPolicyAndReusesStrategicContext(t *testing.T) {
	now := time.Now()
	repo := &automationRepositoryFake{
		readiness: automationBindingReadiness{InstanceFound: true, AgentFound: true, InstanceReady: true, AgentReady: true, BindingReady: true},
		row: automationProfileRow{ID: "77777777-7777-4777-8777-777777777777", ClientAccountID: automationTestClient,
			WhatsAppInstanceID: automationTestInstance, AIAgentID: automationTestAgent, Enabled: true,
			AutoCloseMinConfidence: 0.82, AutoCloseRequireAllFields: true,
			AutoCloseBlockHumanRequest: true, AutoCloseBlockSensitive: true,
			InstanceName: "cliente-wa", InstanceProvider: "evolution", InstanceActive: true,
			AgentName: "Recepcao", AgentEnabled: true, AgentReady: true, BindingReady: true, Revision: 1,
			CreatedAt: now, UpdatedAt: now},
	}
	minimum := 0.82
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, automationContextFake{available: true, profile: AutomationBusinessContext{
		ClientID: automationTestClient, Segment: "Saude", BrandVoice: "Acolhedora",
	}})
	out, err := svc.PutProfile(context.Background(), automationTestAccount, automationTestPrincipal(), automationTestClient,
		AutomationProfileInput{WhatsAppInstanceID: automationTestInstance, AIAgentID: automationTestAgent, Enabled: true,
			ClosePolicy: &AutomationClosePolicyInput{MinimumConfidence: &minimum}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.putCalls != 1 || repo.write.AutoCloseMinConfidence != minimum || !repo.write.AutoCloseRequireAllFields ||
		!repo.write.AutoCloseBlockHumanRequest || !repo.write.AutoCloseBlockSensitive {
		t.Fatalf("write=%+v calls=%d", repo.write, repo.putCalls)
	}
	if out.StrategicContext == nil || !out.StrategicContext.Available || !out.StrategicContext.Filled ||
		out.StrategicContext.Profile.Segment != "Saude" {
		t.Fatalf("contexto=%+v", out.StrategicContext)
	}
	if !out.ClosePolicy.ValidGenerationRequired {
		t.Fatal("lease de geracao deixou de ser obrigatoria")
	}
}

func TestAutomationGetReturnsUnconfiguredDefaults(t *testing.T) {
	repo := &automationRepositoryFake{getErr: pgx.ErrNoRows}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, automationContextFake{available: true, profile: AutomationBusinessContext{ClientID: automationTestClient}})
	out, err := svc.GetProfile(context.Background(), automationTestAccount, automationTestPrincipal(), automationTestClient)
	if err != nil {
		t.Fatal(err)
	}
	if out.Configured || out.ClosePolicy.AutoCloseEnabled || out.ClosePolicy.MinimumConfidence != 0.90 {
		t.Fatalf("perfil default=%+v", out)
	}
}

func TestAutomationInterventionsFilterInvisibleClientsAndExposeOnlyFieldKeys(t *testing.T) {
	repo := &automationRepositoryFake{interventions: []automationInterventionRow{
		{ID: "handoff-1", ClientAccountID: automationTestClient, ConversationID: "conversation-1",
			CollectedFields: []byte(`{"phone":"5511","name":"Ana"}`), WaitingSince: time.Now()},
		{ID: "handoff-2", ClientAccountID: automationOtherClient, ConversationID: "conversation-2",
			CollectedFields: []byte(`{"secret":"hidden"}`), WaitingSince: time.Now()},
	}}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	out, err := svc.ListInterventions(context.Background(), automationTestAccount, automationTestPrincipal(), "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != "handoff-1" {
		t.Fatalf("unexpected interventions: %#v", out)
	}
	if len(out[0].CollectedFieldKeys) != 2 || out[0].CollectedFieldKeys[0] != "name" || out[0].CollectedFieldKeys[1] != "phone" {
		t.Fatalf("field keys not normalized: %#v", out[0].CollectedFieldKeys)
	}
}

func TestAutomationInterventionsRejectClientOutsideVisibleScope(t *testing.T) {
	repo := &automationRepositoryFake{}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	_, err := svc.ListInterventions(context.Background(), automationTestAccount, automationTestPrincipal(), automationOtherClient, 50)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, want not found", err)
	}
}

func TestAutomationAttendancesExposeActiveAndStoppedModes(t *testing.T) {
	now := time.Now()
	repo := &automationRepositoryFake{attendances: []automationAttendanceRow{
		{ConversationID: "active", ClientAccountID: automationTestClient,
			ConversationState: string(StateAIActive), UnansweredCount: 1, ActivitySince: now},
		{ConversationID: "human", ClientAccountID: automationTestClient,
			ConversationState: string(StateHumanActive), UnansweredCount: 1, ActivitySince: now},
		{ConversationID: "stopped", ClientAccountID: automationTestClient,
			ConversationState: string(StateQueued), ReasonCode: HandoffReasonModel, ActivitySince: now},
		{ConversationID: "hidden", ClientAccountID: automationOtherClient, ActivitySince: now},
	}}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	out, err := svc.ListAttendances(context.Background(), automationTestAccount, automationTestPrincipal(), "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0].Mode != automationAttendanceAIActive ||
		out[1].Mode != automationAttendanceHuman || out[1].ReasonCode != "human_active" ||
		out[2].Mode != automationAttendanceAIStopped {
		t.Fatalf("attendances=%#v", out)
	}
}

func TestAutomationPauseCreatesOperatorHandoff(t *testing.T) {
	repo := &automationRepositoryFake{scope: automationConversationScope{
		ClientAccountID: automationTestClient, ConversationState: string(StateAIActive),
	}}
	domain := &automationDomainFake{}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil, domain)
	out, err := svc.PauseAI(context.Background(), automationTestAccount, automationTestPrincipal(), automationTestConv,
		AutomationActionInput{IdempotencyKey: "action-1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.State != string(StateQueued) || domain.calls != 1 ||
		domain.request.ReasonCode != HandoffReasonOperatorPause ||
		domain.request.IdempotencyKey != "operator-pause:action-1" {
		t.Fatalf("out=%+v domain=%+v", out, domain)
	}
}

func TestAutomationReplyWithAIRejectsInvisibleClient(t *testing.T) {
	repo := &automationRepositoryFake{scope: automationConversationScope{
		ClientAccountID: automationOtherClient, ConversationState: string(StateQueued),
	}}
	svc := NewAutomationService(repo, automationPermissionFake{}, automationCatalogFake{
		clients: []AutomationClientRef{automationTestClientRef()},
	}, nil)
	_, err := svc.ReplyWithAI(context.Background(), automationTestAccount, automationTestPrincipal(), automationTestConv,
		AutomationActionInput{IdempotencyKey: "action-2"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, want not found", err)
	}
}
