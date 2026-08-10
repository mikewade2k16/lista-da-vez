package planning

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type repositoryStub struct {
	consultantCount int
	savedSchedule   RepositoryScheduleInput
	contracts       []StaffContract
	weeklyGoal      float64
	configuration   *StoreConfiguration
}

func (stub *repositoryStub) FindConfiguration(context.Context, string, string) (*StoreConfiguration, error) {
	return stub.configuration, nil
}
func (stub *repositoryStub) FindSchedule(context.Context, string, string, time.Time) (*Schedule, error) {
	return nil, nil
}
func (stub *repositoryStub) ListScheduleRevisions(context.Context, string, string, string) ([]ScheduleRevision, error) {
	return []ScheduleRevision{}, nil
}
func (stub *repositoryStub) ListContracts(context.Context, string, string) ([]StaffContract, error) {
	return stub.contracts, nil
}
func (stub *repositoryStub) UpsertConfiguration(_ context.Context, _, storeID, _ string, value json.RawMessage, _ []StaffContract) (StoreConfiguration, error) {
	return StoreConfiguration{StoreID: storeID, Configuration: value}, nil
}
func (stub *repositoryStub) SaveSchedule(_ context.Context, input RepositoryScheduleInput) (Schedule, error) {
	stub.savedSchedule = input
	return Schedule{StoreID: input.StoreID, WeekStart: input.WeekStart.Format("2006-01-02"), Status: input.Status, Shifts: input.Shifts, GoalAllocations: input.Allocations}, nil
}
func (stub *repositoryStub) ReopenSchedule(context.Context, string, string, string, time.Time, int64) (Schedule, error) {
	return Schedule{}, nil
}
func (stub *repositoryStub) FindStoreWeeklyGoal(context.Context, string, string, time.Time, int) (float64, error) {
	return stub.weeklyGoal, nil
}
func (stub *repositoryStub) CountStoreConsultants(context.Context, string, string, []string) (int, error) {
	return stub.consultantCount, nil
}

type storeFinderStub struct{}

type notifierStub struct{ actions []string }

func (stub *notifierStub) PublishContextEvent(_ context.Context, _, resource, action, _ string, _ time.Time) {
	stub.actions = append(stub.actions, resource+":"+action)
}

func (storeFinderStub) FindAccessible(_ context.Context, _ auth.Principal, storeID string) (stores.StoreView, error) {
	return stores.StoreView{ID: storeID, TenantID: "11111111-1111-1111-1111-111111111111", StoreType: "shopping"}, nil
}

func managerPrincipal() auth.Principal {
	return auth.Principal{UserID: "22222222-2222-2222-2222-222222222222", Role: auth.RoleManager}
}

func testPlanningConfiguration() *StoreConfiguration {
	return &StoreConfiguration{Configuration: json.RawMessage(`{
		"activePolicyId":"default",
		"operatingHoursByLocationType":{"shopping":[
			{"weekday":"mon","isOpen":true,"opensAt":"10:00","closesAt":"22:00"},
			{"weekday":"tue","isOpen":true,"opensAt":"10:00","closesAt":"22:00"},
			{"weekday":"wed","isOpen":true,"opensAt":"10:00","closesAt":"22:00"},
			{"weekday":"thu","isOpen":true,"opensAt":"10:00","closesAt":"22:00"},
			{"weekday":"fri","isOpen":true,"opensAt":"10:00","closesAt":"22:00"},
			{"weekday":"sat","isOpen":true,"opensAt":"10:00","closesAt":"22:00"},
			{"weekday":"sun","isOpen":true,"opensAt":"10:00","closesAt":"22:00"}
		]},
		"shiftTemplatesByLocationType":{"shopping":[
			{"id":"opening","name":"Abertura","startsAt":"10:00","endsAt":"19:00"},
			{"id":"middle","name":"Intermediário","startsAt":"11:00","endsAt":"20:00"},
			{"id":"closing","name":"Fechamento","startsAt":"13:00","endsAt":"22:00"}
		]},
		"policies":[{"id":"default","maxDailyHours":8,"maxConsecutiveDays":6,"minDaysOff":1,"breakAfterHours":6,"minBreakMinutes":60}]
	}`)}
}

func TestSaveScheduleAcceptsScopedConsultantAndMondayWeek(t *testing.T) {
	repository := &repositoryStub{
		consultantCount: 1,
		weeklyGoal:      25_000,
		contracts:       []StaffContract{{ConsultantID: "44444444-4444-4444-4444-444444444444", WeeklyHours: 44, MaxDailyHours: 8, TargetWeight: 1, AvailableWeekdays: []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}}},
		configuration:   testPlanningConfiguration(),
	}
	service := NewService(repository, storeFinderStub{})
	result, err := service.SaveSchedule(context.Background(), managerPrincipal(), SaveScheduleInput{
		StoreID: "33333333-3333-3333-3333-333333333333", WeekStart: "2026-07-27", TargetMonth: "2026-08", GoalWeek: 1, Status: "saved",
		Shifts: []Shift{{StaffID: "44444444-4444-4444-4444-444444444444", ISODate: "2026-08-02", TemplateID: "closing", StartsAt: "14:00", EndsAt: "22:00", BreakMinutes: 60}},
	})
	if err != nil {
		t.Fatalf("SaveSchedule() error = %v", err)
	}
	if result.Status != "saved" || repository.savedSchedule.WeekStart.Format("2006-01-02") != "2026-07-27" {
		t.Fatalf("unexpected persisted schedule: %#v", result)
	}
	if len(result.GoalAllocations) != 1 || result.GoalAllocations[0].Target != 25_000 {
		t.Fatalf("unexpected goal allocations: %#v", result.GoalAllocations)
	}
}

func TestSaveScheduleRejectsConsultantFromAnotherStore(t *testing.T) {
	service := NewService(&repositoryStub{consultantCount: 0}, storeFinderStub{})
	_, err := service.SaveSchedule(context.Background(), managerPrincipal(), SaveScheduleInput{
		StoreID: "33333333-3333-3333-3333-333333333333", WeekStart: "2026-07-27", TargetMonth: "2026-07", GoalWeek: 4, Status: "saved",
		Shifts: []Shift{{StaffID: "44444444-4444-4444-4444-444444444444", ISODate: "2026-07-27", TemplateID: "opening", StartsAt: "09:00", EndsAt: "17:00", BreakMinutes: 60}},
	})
	if err != ErrValidation {
		t.Fatalf("SaveSchedule() error = %v, want ErrValidation", err)
	}
}

func TestGenerateSchedulePersistsExactHoursAndIndividualGoals(t *testing.T) {
	staffID := "44444444-4444-4444-4444-444444444444"
	repository := &repositoryStub{
		weeklyGoal: 25_000,
		contracts: []StaffContract{{
			ConsultantID: staffID, WeeklyHours: 44, MaxDailyHours: 8, TargetWeight: 1,
			AvailableWeekdays: []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"},
		}},
		configuration: testPlanningConfiguration(),
	}
	notifier := &notifierStub{}
	service := NewService(repository, storeFinderStub{}, notifier)

	result, err := service.GenerateSchedule(context.Background(), managerPrincipal(), GenerateScheduleInput{
		StoreID: "33333333-3333-3333-3333-333333333333", WeekStart: "2026-07-27",
		TargetMonth: "2026-08", GoalWeek: 1,
	})
	if err != nil {
		t.Fatalf("GenerateSchedule() error = %v", err)
	}
	totalMinutes := 0
	for _, shift := range result.Shifts {
		totalMinutes += engineShiftMinutes(shift)
	}
	if totalMinutes != 44*60 {
		t.Fatalf("generated minutes = %d, want %d", totalMinutes, 44*60)
	}
	if len(repository.savedSchedule.Allocations) != 1 || repository.savedSchedule.Allocations[0].Target != 25_000 {
		t.Fatalf("individual goals were not persisted: %#v", repository.savedSchedule.Allocations)
	}
	if len(notifier.actions) != 2 || notifier.actions[0] != "planning:schedule.generated" || notifier.actions[1] != "operationgoal:generated" {
		t.Fatalf("unexpected realtime events: %#v", notifier.actions)
	}
}

func TestSaveConfigurationRequiresJSONObject(t *testing.T) {
	service := NewService(&repositoryStub{}, storeFinderStub{})
	_, err := service.SaveConfiguration(context.Background(), managerPrincipal(), SaveConfigurationInput{
		StoreID: "33333333-3333-3333-3333-333333333333", Configuration: json.RawMessage(`[]`),
	})
	if err != ErrValidation {
		t.Fatalf("SaveConfiguration() error = %v, want ErrValidation", err)
	}
}

func TestResolvedPermissionsOverrideManagerRole(t *testing.T) {
	principal := managerPrincipal()
	principal.PermissionsResolved = true
	principal.Permissions = []string{}
	if canView(principal) || canEdit(principal) {
		t.Fatal("resolved permissions without planning grants must deny access")
	}
}

func TestPlanningUsesDedicatedResolvedPermissions(t *testing.T) {
	principal := managerPrincipal()
	principal.PermissionsResolved = true
	principal.Permissions = []string{accesscontrol.PermissionMultiStoreEdit}
	if canView(principal) || canEdit(principal) {
		t.Fatal("multi-store permissions must not grant planning access")
	}

	principal.Permissions = []string{accesscontrol.PermissionPlanningEdit}
	if !canView(principal) || !canEdit(principal) {
		t.Fatal("planning edit permission must grant planning view and edit access")
	}
}
