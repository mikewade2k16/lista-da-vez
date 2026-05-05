package operations

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testOperationsRepository struct {
	storeID                            string
	storeName                          string
	storeExists                        bool
	maxConcurrentServices              int
	maxConcurrentServicesPerConsultant int
	roster                             []ConsultantProfile
	snapshot                           SnapshotState
	persisted                          []PersistInput
}

type testStoreScopeProvider struct {
	stores []StoreScopeView
}

func (provider testStoreScopeProvider) ListAccessible(context.Context, AccessContext, StoreScopeFilter) ([]StoreScopeView, error) {
	return append([]StoreScopeView{}, provider.stores...), nil
}

func (repository *testOperationsRepository) StoreExists(context.Context, string) (bool, error) {
	return repository.storeExists, nil
}

func (repository *testOperationsRepository) GetStoreName(context.Context, string) (string, error) {
	return repository.storeName, nil
}

func (repository *testOperationsRepository) GetMaxConcurrentServices(context.Context, string) (int, error) {
	return repository.maxConcurrentServices, nil
}

func (repository *testOperationsRepository) GetMaxConcurrentServicesPerConsultant(context.Context, string) (int, error) {
	return repository.maxConcurrentServicesPerConsultant, nil
}

func (repository *testOperationsRepository) ListStoresWithActiveServices(context.Context) ([]string, error) {
	if len(repository.snapshot.ActiveServices) == 0 {
		return nil, nil
	}
	return []string{repository.storeID}, nil
}

func (repository *testOperationsRepository) ListStoresWithActiveServicesByTenant(context.Context, string) ([]string, error) {
	return repository.ListStoresWithActiveServices(context.Background())
}

func (repository *testOperationsRepository) ListRoster(context.Context, string) ([]ConsultantProfile, error) {
	return append([]ConsultantProfile{}, repository.roster...), nil
}

func (repository *testOperationsRepository) LoadSnapshot(context.Context, string) (SnapshotState, error) {
	return SnapshotState{
		StoreID:                    repository.snapshot.StoreID,
		WaitingList:                cloneQueueStateItems(repository.snapshot.WaitingList),
		ActiveServices:             cloneActiveServiceStates(repository.snapshot.ActiveServices),
		PausedEmployees:            clonePausedStateItems(repository.snapshot.PausedEmployees),
		ConsultantActivitySessions: cloneSessions(repository.snapshot.ConsultantActivitySessions),
		ConsultantCurrentStatus:    cloneCurrentStatus(repository.snapshot.ConsultantCurrentStatus),
		ServiceHistory:             cloneHistory(repository.snapshot.ServiceHistory),
	}, nil
}

func (repository *testOperationsRepository) Persist(_ context.Context, input PersistInput) error {
	repository.persisted = append(repository.persisted, PersistInput{
		StoreID:          input.StoreID,
		WaitingList:      cloneQueueStateItems(input.WaitingList),
		ActiveServices:   cloneActiveServiceStates(input.ActiveServices),
		PausedEmployees:  clonePausedStateItems(input.PausedEmployees),
		CurrentStatus:    cloneCurrentStatus(input.CurrentStatus),
		AppendedSessions: cloneSessions(input.AppendedSessions),
		AppendedHistory:  cloneHistory(input.AppendedHistory),
	})

	repository.snapshot = SnapshotState{
		StoreID:                    input.StoreID,
		WaitingList:                cloneQueueStateItems(input.WaitingList),
		ActiveServices:             cloneActiveServiceStates(input.ActiveServices),
		PausedEmployees:            clonePausedStateItems(input.PausedEmployees),
		ConsultantActivitySessions: append(cloneSessions(repository.snapshot.ConsultantActivitySessions), cloneSessions(input.AppendedSessions)...),
		ConsultantCurrentStatus:    cloneCurrentStatus(input.CurrentStatus),
		ServiceHistory:             append(cloneHistory(repository.snapshot.ServiceHistory), cloneHistory(input.AppendedHistory)...),
	}

	return nil
}

func TestStartParallelRejectsWhenPerConsultantLimitIsOne(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	repository.maxConcurrentServicesPerConsultant = 1
	repository.snapshot.ActiveServices = []ActiveServiceState{testPrimaryActiveService(consultantID)}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	_, err := service.StartParallel(context.Background(), testAccessContext(repository.storeID), StartParallelCommandInput{
		StoreID:  repository.storeID,
		PersonID: consultantID,
	})
	if !errors.Is(err, ErrConcurrentServiceLimitPerConsultantReached) {
		t.Fatalf("expected ErrConcurrentServiceLimitPerConsultantReached, got %v", err)
	}
	if len(repository.persisted) != 0 {
		t.Fatalf("expected no persistence when per-consultant limit blocks the action")
	}
}

func TestStartParallelCreatesNewServiceAndKeepsConsultantInService(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	repository.maxConcurrentServicesPerConsultant = 2
	repository.snapshot.ActiveServices = []ActiveServiceState{testPrimaryActiveService(consultantID)}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	ack, err := service.StartParallel(context.Background(), testAccessContext(repository.storeID), StartParallelCommandInput{
		StoreID:  repository.storeID,
		PersonID: consultantID,
	})
	if err != nil {
		t.Fatalf("expected StartParallel to succeed, got %v", err)
	}
	if ack.ServiceID == "" {
		t.Fatalf("expected StartParallel to return a new service id")
	}
	if repository.snapshot.ConsultantCurrentStatus[consultantID].Status != statusService {
		t.Fatalf("expected consultant to remain in service, got %s", repository.snapshot.ConsultantCurrentStatus[consultantID].Status)
	}

	parallelService := findActiveServiceState(t, repository.snapshot.ActiveServices, ack.ServiceID)
	if parallelService.StartMode != startModeParallel {
		t.Fatalf("expected new service to be parallel, got %s", parallelService.StartMode)
	}
	if parallelService.ParallelStartIndex == nil || *parallelService.ParallelStartIndex != 2 {
		t.Fatalf("expected parallelStartIndex=2, got %#v", parallelService.ParallelStartIndex)
	}
	if parallelService.ParallelGroupID == "" {
		t.Fatalf("expected parallelGroupId to be populated")
	}
	if len(parallelService.SiblingServiceIDs) != 1 || parallelService.SiblingServiceIDs[0] != "service-primary" {
		t.Fatalf("expected siblingServiceIds to reference the original service, got %#v", parallelService.SiblingServiceIDs)
	}
	if parallelService.StartOffsetMs <= 0 {
		t.Fatalf("expected positive startOffsetMs, got %d", parallelService.StartOffsetMs)
	}
	if parallelService.QueuePositionAtStart == nil || *parallelService.QueuePositionAtStart != 1 {
		t.Fatalf("expected parallel service to inherit queuePositionAtStart=1, got %#v", parallelService.QueuePositionAtStart)
	}
	if parallelService.QueueJoinedAt != repository.snapshot.ActiveServices[0].QueueJoinedAt {
		t.Fatalf("expected parallel service to inherit queueJoinedAt, got %d want %d", parallelService.QueueJoinedAt, repository.snapshot.ActiveServices[0].QueueJoinedAt)
	}

	snapshot, err := service.Snapshot(context.Background(), testAccessContext(repository.storeID), repository.storeID)
	if err != nil {
		t.Fatalf("expected Snapshot to succeed, got %v", err)
	}
	projectedService := findActiveServiceView(t, snapshot.ActiveServices, ack.ServiceID)
	if projectedService.StartMode != startModeParallel {
		t.Fatalf("expected snapshot to preserve parallel start mode, got %s", projectedService.StartMode)
	}
	if projectedService.ParallelStartIndex == nil || *projectedService.ParallelStartIndex != 2 {
		t.Fatalf("expected snapshot parallelStartIndex=2, got %#v", projectedService.ParallelStartIndex)
	}
	if projectedService.ParallelGroupID == "" {
		t.Fatalf("expected snapshot to preserve parallelGroupId")
	}
	if len(projectedService.SiblingServiceIDs) != 1 || projectedService.SiblingServiceIDs[0] != "service-primary" {
		t.Fatalf("expected snapshot siblingServiceIds to be preserved, got %#v", projectedService.SiblingServiceIDs)
	}
}

func TestFinishParallelBeforeLastKeepsConsultantOutOfQueue(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	repository.snapshot.ActiveServices = []ActiveServiceState{
		testPrimaryActiveService(consultantID),
		{
			ConsultantID:       consultantID,
			ServiceID:          "service-parallel",
			ServiceStartedAt:   time.Now().Add(-1 * time.Minute).UTC().UnixMilli(),
			QueueJoinedAt:      time.Now().Add(-1 * time.Minute).UTC().UnixMilli(),
			QueueWaitMs:        0,
			StartMode:          startModeParallel,
			ParallelGroupID:    "group-1",
			ParallelStartIndex: intPtr(2),
			SiblingServiceIDs:  []string{"service-primary"},
			StartOffsetMs:      60000,
		},
	}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	_, err := service.Finish(context.Background(), testAccessContext(repository.storeID), FinishCommandInput{
		StoreID:   repository.storeID,
		ServiceID: "service-parallel",
		Outcome:   "compra",
	})
	if err != nil {
		t.Fatalf("expected Finish to succeed, got %v", err)
	}
	if len(repository.snapshot.WaitingList) != 0 {
		t.Fatalf("expected consultant to stay out of queue while another service is active")
	}
	if repository.snapshot.ConsultantCurrentStatus[consultantID].Status != statusService {
		t.Fatalf("expected consultant to remain in service, got %s", repository.snapshot.ConsultantCurrentStatus[consultantID].Status)
	}
	if len(repository.snapshot.ActiveServices) != 1 {
		t.Fatalf("expected one remaining active service, got %d", len(repository.snapshot.ActiveServices))
	}
	if len(repository.snapshot.ServiceHistory) != 1 {
		t.Fatalf("expected one history entry, got %d", len(repository.snapshot.ServiceHistory))
	}
	historyEntry := repository.snapshot.ServiceHistory[0]
	if historyEntry.ParallelGroupID != "group-1" {
		t.Fatalf("expected history to preserve parallelGroupId, got %s", historyEntry.ParallelGroupID)
	}
	if historyEntry.ParallelStartIndex == nil || *historyEntry.ParallelStartIndex != 2 {
		t.Fatalf("expected history parallelStartIndex=2, got %#v", historyEntry.ParallelStartIndex)
	}
	if len(historyEntry.SiblingServiceIDs) != 1 || historyEntry.SiblingServiceIDs[0] != "service-primary" {
		t.Fatalf("expected history siblingServiceIds to be preserved, got %#v", historyEntry.SiblingServiceIDs)
	}
	if historyEntry.StartOffsetMs != 60000 {
		t.Fatalf("expected history startOffsetMs=60000, got %d", historyEntry.StartOffsetMs)
	}
	if historyEntry.QueuePositionAtStart == nil || *historyEntry.QueuePositionAtStart != 1 {
		t.Fatalf("expected history to preserve queuePositionAtStart from the sequence, got %#v", historyEntry.QueuePositionAtStart)
	}
	if historyEntry.DurationMs != historyEntry.FinishedAt-historyEntry.StartedAt {
		t.Fatalf("expected durationMs to use the sequential end timestamp, got %d", historyEntry.DurationMs)
	}
}

func TestFinishPrimaryServiceUsesNextSequentialStartAsEndTime(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	primaryService := testPrimaryActiveService(consultantID)
	parallelStartedAt := time.Now().Add(-1 * time.Minute).UTC().UnixMilli()
	repository.snapshot.ActiveServices = []ActiveServiceState{
		primaryService,
		{
			ConsultantID:         consultantID,
			ServiceID:            "service-parallel",
			ServiceStartedAt:     parallelStartedAt,
			QueueJoinedAt:        primaryService.QueueJoinedAt,
			QueueWaitMs:          primaryService.QueueWaitMs,
			QueuePositionAtStart: intPtr(1),
			StartMode:            startModeParallel,
			ParallelGroupID:      "group-1",
			ParallelStartIndex:   intPtr(2),
			SiblingServiceIDs:    []string{"service-primary"},
			StartOffsetMs:        parallelStartedAt - primaryService.ServiceStartedAt,
		},
	}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	_, err := service.Finish(context.Background(), testAccessContext(repository.storeID), FinishCommandInput{
		StoreID:   repository.storeID,
		ServiceID: "service-primary",
		Outcome:   "compra",
	})
	if err != nil {
		t.Fatalf("expected Finish to succeed, got %v", err)
	}
	if len(repository.snapshot.ServiceHistory) != 1 {
		t.Fatalf("expected one history entry, got %d", len(repository.snapshot.ServiceHistory))
	}
	historyEntry := repository.snapshot.ServiceHistory[0]
	if historyEntry.FinishedAt != parallelStartedAt {
		t.Fatalf("expected primary service timing to stop at the next sequential start, got %d want %d", historyEntry.FinishedAt, parallelStartedAt)
	}
	if historyEntry.DurationMs != parallelStartedAt-primaryService.ServiceStartedAt {
		t.Fatalf("expected primary service duration to stop at the next sequential start, got %d", historyEntry.DurationMs)
	}
}

func TestSnapshotAndOverviewExposeStoppedServiceTiming(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	startedAt := time.Now().Add(-10 * time.Minute).UTC().UnixMilli()
	stoppedAt := startedAt + (3 * time.Minute).Milliseconds()
	repository.snapshot.ActiveServices = []ActiveServiceState{
		{
			ConsultantID:     consultantID,
			ServiceID:        "service-stopped",
			ServiceStartedAt: startedAt,
			QueueJoinedAt:    startedAt - 30000,
			QueueWaitMs:      30000,
			StartMode:        startModeQueue,
			StoppedAt:        stoppedAt,
			StopReason:       "cliente ausente",
		},
	}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: startedAt},
	}

	service := NewService(repository, nil, testStoreScopeProvider{
		stores: []StoreScopeView{{ID: repository.storeID, Name: repository.storeName}},
	})
	snapshot, err := service.Snapshot(context.Background(), testAccessContext(repository.storeID), repository.storeID)
	if err != nil {
		t.Fatalf("expected Snapshot to succeed, got %v", err)
	}
	projectedService := findActiveServiceView(t, snapshot.ActiveServices, "service-stopped")
	if projectedService.StoppedAt != stoppedAt {
		t.Fatalf("expected snapshot stoppedAt=%d, got %d", stoppedAt, projectedService.StoppedAt)
	}
	if projectedService.EffectiveFinishedAt != stoppedAt {
		t.Fatalf("expected snapshot effectiveFinishedAt=%d, got %d", stoppedAt, projectedService.EffectiveFinishedAt)
	}

	overview, err := service.Overview(context.Background(), testAccessContext(repository.storeID))
	if err != nil {
		t.Fatalf("expected Overview to succeed, got %v", err)
	}
	if len(overview.ActiveServices) != 1 {
		t.Fatalf("expected one overview active service, got %d", len(overview.ActiveServices))
	}
	if overview.ActiveServices[0].StoppedAt != stoppedAt {
		t.Fatalf("expected overview stoppedAt=%d, got %d", stoppedAt, overview.ActiveServices[0].StoppedAt)
	}
	if overview.ActiveServices[0].EffectiveFinishedAt != stoppedAt {
		t.Fatalf("expected overview effectiveFinishedAt=%d, got %d", stoppedAt, overview.ActiveServices[0].EffectiveFinishedAt)
	}
}

func TestSnapshotAndOverviewExposeSequentialEffectiveFinishedAtFromHistory(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	startedAt := time.Now().Add(-10 * time.Minute).UTC().UnixMilli()
	nextStartedAt := startedAt + (4 * time.Minute).Milliseconds()
	repository.snapshot.ActiveServices = []ActiveServiceState{
		{
			ConsultantID:       consultantID,
			ServiceID:          "service-primary",
			ServiceStartedAt:   startedAt,
			QueueJoinedAt:      startedAt - 30000,
			QueueWaitMs:        30000,
			StartMode:          startModeQueue,
			ParallelGroupID:    "group-1",
			ParallelStartIndex: intPtr(1),
		},
	}
	repository.snapshot.ServiceHistory = []ServiceHistoryEntry{
		{
			ServiceID:          "service-next",
			StoreID:            repository.storeID,
			PersonID:           consultantID,
			PersonName:         "Ana",
			StartedAt:          nextStartedAt,
			FinishedAt:         nextStartedAt + 60000,
			DurationMs:         60000,
			FinishOutcome:      "compra",
			StartMode:          startModeParallel,
			ParallelGroupID:    "group-1",
			ParallelStartIndex: intPtr(2),
		},
	}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: startedAt},
	}

	service := NewService(repository, nil, testStoreScopeProvider{
		stores: []StoreScopeView{{ID: repository.storeID, Name: repository.storeName}},
	})
	snapshot, err := service.Snapshot(context.Background(), testAccessContext(repository.storeID), repository.storeID)
	if err != nil {
		t.Fatalf("expected Snapshot to succeed, got %v", err)
	}
	projectedService := findActiveServiceView(t, snapshot.ActiveServices, "service-primary")
	if projectedService.EffectiveFinishedAt != nextStartedAt {
		t.Fatalf("expected snapshot effectiveFinishedAt=%d, got %d", nextStartedAt, projectedService.EffectiveFinishedAt)
	}

	overview, err := service.Overview(context.Background(), testAccessContext(repository.storeID))
	if err != nil {
		t.Fatalf("expected Overview to succeed, got %v", err)
	}
	if len(overview.ActiveServices) != 1 {
		t.Fatalf("expected one overview active service, got %d", len(overview.ActiveServices))
	}
	if overview.ActiveServices[0].EffectiveFinishedAt != nextStartedAt {
		t.Fatalf("expected overview effectiveFinishedAt=%d, got %d", nextStartedAt, overview.ActiveServices[0].EffectiveFinishedAt)
	}
}

func TestFinishLastParallelReturnsConsultantToQueue(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	repository.snapshot.ActiveServices = []ActiveServiceState{
		{
			ConsultantID:       consultantID,
			ServiceID:          "service-last",
			ServiceStartedAt:   time.Now().Add(-1 * time.Minute).UTC().UnixMilli(),
			QueueJoinedAt:      time.Now().Add(-1 * time.Minute).UTC().UnixMilli(),
			QueueWaitMs:        0,
			StartMode:          startModeParallel,
			ParallelGroupID:    "group-1",
			ParallelStartIndex: intPtr(2),
			SiblingServiceIDs:  []string{"service-primary"},
			StartOffsetMs:      60000,
		},
	}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	_, err := service.Finish(context.Background(), testAccessContext(repository.storeID), FinishCommandInput{
		StoreID:   repository.storeID,
		ServiceID: "service-last",
		Outcome:   "compra",
	})
	if err != nil {
		t.Fatalf("expected Finish to succeed, got %v", err)
	}
	if len(repository.snapshot.ActiveServices) != 0 {
		t.Fatalf("expected no active services remaining, got %d", len(repository.snapshot.ActiveServices))
	}
	if len(repository.snapshot.WaitingList) != 1 || repository.snapshot.WaitingList[0].ConsultantID != consultantID {
		t.Fatalf("expected consultant to return to queue, got %#v", repository.snapshot.WaitingList)
	}
	if repository.snapshot.ConsultantCurrentStatus[consultantID].Status != statusQueue {
		t.Fatalf("expected consultant status to return to queue, got %s", repository.snapshot.ConsultantCurrentStatus[consultantID].Status)
	}
}

func TestStartParallelHonorsStoreLimit(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	repository.maxConcurrentServices = 1
	repository.maxConcurrentServicesPerConsultant = 2
	repository.snapshot.ActiveServices = []ActiveServiceState{testPrimaryActiveService(consultantID)}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	_, err := service.StartParallel(context.Background(), testAccessContext(repository.storeID), StartParallelCommandInput{
		StoreID:  repository.storeID,
		PersonID: consultantID,
	})
	if !errors.Is(err, ErrConcurrentServiceLimitReached) {
		t.Fatalf("expected ErrConcurrentServiceLimitReached, got %v", err)
	}
	if len(repository.persisted) != 0 {
		t.Fatalf("expected no persistence when store-level limit blocks the action")
	}
}

func TestStartParallelReusesParallelGroupAcrossOverlaps(t *testing.T) {
	consultantID := "consultant-1"
	repository := newParallelTestRepository(consultantID)
	repository.maxConcurrentServicesPerConsultant = 3
	repository.snapshot.ActiveServices = []ActiveServiceState{testPrimaryActiveService(consultantID)}
	repository.snapshot.ConsultantCurrentStatus = map[string]ConsultantStatus{
		consultantID: {Status: statusService, StartedAt: time.Now().Add(-2 * time.Minute).UTC().UnixMilli()},
	}

	service := NewService(repository, nil, nil)
	firstAck, err := service.StartParallel(context.Background(), testAccessContext(repository.storeID), StartParallelCommandInput{
		StoreID:  repository.storeID,
		PersonID: consultantID,
	})
	if err != nil {
		t.Fatalf("expected first StartParallel to succeed, got %v", err)
	}
	secondAck, err := service.StartParallel(context.Background(), testAccessContext(repository.storeID), StartParallelCommandInput{
		StoreID:  repository.storeID,
		PersonID: consultantID,
	})
	if err != nil {
		t.Fatalf("expected second StartParallel to succeed, got %v", err)
	}

	firstParallel := findActiveServiceState(t, repository.snapshot.ActiveServices, firstAck.ServiceID)
	secondParallel := findActiveServiceState(t, repository.snapshot.ActiveServices, secondAck.ServiceID)
	if firstParallel.ParallelGroupID == "" || secondParallel.ParallelGroupID == "" {
		t.Fatalf("expected both parallel services to carry a parallelGroupId")
	}
	if firstParallel.ParallelGroupID != secondParallel.ParallelGroupID {
		t.Fatalf("expected overlapping parallel services to share the same parallelGroupId, got %s and %s", firstParallel.ParallelGroupID, secondParallel.ParallelGroupID)
	}
	if secondParallel.ParallelStartIndex == nil || *secondParallel.ParallelStartIndex != 3 {
		t.Fatalf("expected third active service to have parallelStartIndex=3, got %#v", secondParallel.ParallelStartIndex)
	}
	if !containsString(secondParallel.SiblingServiceIDs, "service-primary") || !containsString(secondParallel.SiblingServiceIDs, firstAck.ServiceID) {
		t.Fatalf("expected third active service to reference both sibling services, got %#v", secondParallel.SiblingServiceIDs)
	}
}

func newParallelTestRepository(consultantID string) *testOperationsRepository {
	storeID := "store-1"
	return &testOperationsRepository{
		storeID:                            storeID,
		storeName:                          "Loja Teste",
		storeExists:                        true,
		maxConcurrentServices:              10,
		maxConcurrentServicesPerConsultant: 2,
		roster: []ConsultantProfile{
			{
				ID:       consultantID,
				StoreID:  storeID,
				Name:     "Ana",
				Role:     "Consultora",
				Initials: "AN",
				Color:    "#123456",
			},
		},
		snapshot: SnapshotState{
			StoreID:                    storeID,
			WaitingList:                []QueueStateItem{},
			ActiveServices:             []ActiveServiceState{},
			PausedEmployees:            []PausedStateItem{},
			ConsultantActivitySessions: []ConsultantSession{},
			ConsultantCurrentStatus:    map[string]ConsultantStatus{},
			ServiceHistory:             []ServiceHistoryEntry{},
		},
	}
}

func testAccessContext(storeID string) AccessContext {
	return AccessContext{
		Role:     RoleManager,
		StoreIDs: []string{storeID},
	}
}

func testPrimaryActiveService(consultantID string) ActiveServiceState {
	startedAt := time.Now().Add(-2 * time.Minute).UTC().UnixMilli()
	return ActiveServiceState{
		ConsultantID:         consultantID,
		ServiceID:            "service-primary",
		ServiceStartedAt:     startedAt,
		QueueJoinedAt:        startedAt - 30000,
		QueueWaitMs:          30000,
		QueuePositionAtStart: intPtr(1),
		StartMode:            startModeQueue,
		SkippedPeople:        []SkippedPerson{},
		SiblingServiceIDs:    []string{},
	}
}

func cloneQueueStateItems(items []QueueStateItem) []QueueStateItem {
	if len(items) == 0 {
		return []QueueStateItem{}
	}
	cloned := make([]QueueStateItem, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, QueueStateItem{
			ConsultantID:  item.ConsultantID,
			QueueJoinedAt: item.QueueJoinedAt,
		})
	}
	return cloned
}

func cloneActiveServiceStates(items []ActiveServiceState) []ActiveServiceState {
	if len(items) == 0 {
		return []ActiveServiceState{}
	}
	cloned := make([]ActiveServiceState, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, ActiveServiceState{
			ConsultantID:         item.ConsultantID,
			ServiceID:            item.ServiceID,
			ServiceStartedAt:     item.ServiceStartedAt,
			QueueJoinedAt:        item.QueueJoinedAt,
			QueueWaitMs:          item.QueueWaitMs,
			QueuePositionAtStart: cloneOptionalInt(item.QueuePositionAtStart),
			StartMode:            item.StartMode,
			SkippedPeople:        cloneSkippedPeople(item.SkippedPeople),
			ParallelGroupID:      item.ParallelGroupID,
			ParallelStartIndex:   cloneOptionalInt(item.ParallelStartIndex),
			SiblingServiceIDs:    cloneStringSlice(item.SiblingServiceIDs),
			StartOffsetMs:        item.StartOffsetMs,
			StoppedAt:            item.StoppedAt,
			StopReason:           item.StopReason,
		})
	}
	return cloned
}

func clonePausedStateItems(items []PausedStateItem) []PausedStateItem {
	if len(items) == 0 {
		return []PausedStateItem{}
	}
	cloned := make([]PausedStateItem, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, PausedStateItem{
			ConsultantID: item.ConsultantID,
			Reason:       item.Reason,
			Kind:         item.Kind,
			StartedAt:    item.StartedAt,
		})
	}
	return cloned
}

func cloneOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func findActiveServiceState(t *testing.T, items []ActiveServiceState, serviceID string) ActiveServiceState {
	t.Helper()
	for _, item := range items {
		if item.ServiceID == serviceID {
			return item
		}
	}
	t.Fatalf("service %s not found in active services", serviceID)
	return ActiveServiceState{}
}

func findActiveServiceView(t *testing.T, items []ActiveService, serviceID string) ActiveService {
	t.Helper()
	for _, item := range items {
		if item.ServiceID == serviceID {
			return item
		}
	}
	t.Fatalf("service %s not found in snapshot view", serviceID)
	return ActiveService{}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
