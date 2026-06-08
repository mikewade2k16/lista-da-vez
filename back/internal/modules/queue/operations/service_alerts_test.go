package operations

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeAlertCoordinator struct {
	rules          OperationalAlertRules
	loadErr        error
	receiveErr     error
	loadedStoreIDs []string
	received       [][]OperationalAlertSignal
}

func (coordinator *fakeAlertCoordinator) LoadOperationalRules(_ context.Context, storeID string) (OperationalAlertRules, error) {
	coordinator.loadedStoreIDs = append(coordinator.loadedStoreIDs, storeID)
	if coordinator.loadErr != nil {
		return OperationalAlertRules{}, coordinator.loadErr
	}
	return coordinator.rules, nil
}

func (coordinator *fakeAlertCoordinator) ReceiveOperationalSignals(_ context.Context, signals []OperationalAlertSignal) error {
	batch := make([]OperationalAlertSignal, len(signals))
	copy(batch, signals)
	coordinator.received = append(coordinator.received, batch)
	return coordinator.receiveErr
}

func TestPersistAndAckEmitsLongOpenSignal(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	coordinator := &fakeAlertCoordinator{rules: OperationalAlertRules{LongOpenServiceMinutes: 25}}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	startedAt := time.Now().Add(-30 * time.Minute).UTC().UnixMilli()
	_, err := service.persistAndAck(context.Background(), repository.storeID, "queue", "consultant-2", SnapshotState{
		StoreID: repository.storeID,
		ActiveServices: []ActiveServiceState{{
			ConsultantID:     "consultant-1",
			ServiceID:        "service-1",
			ServiceStartedAt: startedAt,
			QueueWaitMs:      120000,
			StartMode:        startModeQueue,
		}},
		ConsultantCurrentStatus: map[string]ConsultantStatus{},
	}, nil, nil)
	if err != nil {
		t.Fatalf("expected persistAndAck to succeed, got %v", err)
	}
	if len(coordinator.loadedStoreIDs) != 1 || coordinator.loadedStoreIDs[0] != repository.storeID {
		t.Fatalf("expected LoadOperationalRules to run for %q, got %#v", repository.storeID, coordinator.loadedStoreIDs)
	}
	if len(coordinator.received) != 1 {
		t.Fatalf("expected one signal batch, got %d", len(coordinator.received))
	}
	if len(coordinator.received[0]) != 1 {
		t.Fatalf("expected one signal, got %d", len(coordinator.received[0]))
	}
	signal := coordinator.received[0][0]
	if signal.SignalType != SignalLongOpenServiceTriggered {
		t.Fatalf("expected %q, got %q", SignalLongOpenServiceTriggered, signal.SignalType)
	}
	if signal.ServiceID != "service-1" {
		t.Fatalf("expected service-1, got %q", signal.ServiceID)
	}
}

func TestPersistAndAckEmitsResolveSignalFromHistory(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	coordinator := &fakeAlertCoordinator{rules: OperationalAlertRules{LongOpenServiceMinutes: 25}}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	finishedAt := time.Now().UTC().UnixMilli()
	_, err := service.persistAndAck(context.Background(), repository.storeID, actionFinish, "consultant-1", SnapshotState{
		StoreID:                 repository.storeID,
		ConsultantCurrentStatus: map[string]ConsultantStatus{},
	}, []ServiceHistoryEntry{{
		ServiceID:     "service-1",
		PersonID:      "consultant-1",
		FinishedAt:    finishedAt,
		FinishOutcome: "compra",
	}}, nil)
	if err != nil {
		t.Fatalf("expected persistAndAck to succeed, got %v", err)
	}
	if len(coordinator.received) != 1 || len(coordinator.received[0]) != 1 {
		t.Fatalf("expected one resolve signal, got %#v", coordinator.received)
	}
	if coordinator.received[0][0].SignalType != SignalLongOpenServiceResolved {
		t.Fatalf("expected %q, got %q", SignalLongOpenServiceResolved, coordinator.received[0][0].SignalType)
	}
}

func TestPersistAndAckIgnoresAlertCoordinatorFailure(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	coordinator := &fakeAlertCoordinator{
		rules:      OperationalAlertRules{LongOpenServiceMinutes: 25},
		receiveErr: errors.New("boom"),
	}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	_, err := service.persistAndAck(context.Background(), repository.storeID, "queue", "consultant-1", SnapshotState{
		StoreID: repository.storeID,
		ActiveServices: []ActiveServiceState{{
			ConsultantID:     "consultant-1",
			ServiceID:        "service-1",
			ServiceStartedAt: time.Now().Add(-30 * time.Minute).UTC().UnixMilli(),
		}},
		ConsultantCurrentStatus: map[string]ConsultantStatus{},
	}, nil, nil)
	if err != nil {
		t.Fatalf("expected persistAndAck to ignore alert coordinator errors, got %v", err)
	}
}

func TestFinishCancelEmitsResolveSignalWithoutHistory(t *testing.T) {
	repository := &testOperationsRepository{
		storeID:     "store-1",
		storeName:   "Loja 1",
		storeExists: true,
		roster: []ConsultantProfile{{
			ID:      "consultant-1",
			StoreID: "store-1",
			Name:    "Consultor 1",
		}},
		snapshot: SnapshotState{
			StoreID: "store-1",
			ActiveServices: []ActiveServiceState{{
				ConsultantID:     "consultant-1",
				ServiceID:        "service-1",
				ServiceStartedAt: time.Now().Add(-5 * time.Minute).UTC().UnixMilli(),
				QueueJoinedAt:    time.Now().Add(-6 * time.Minute).UTC().UnixMilli(),
				QueueWaitMs:      60000,
				StartMode:        startModeQueue,
			}},
			ConsultantCurrentStatus: map[string]ConsultantStatus{
				"consultant-1": {Status: statusService, StartedAt: time.Now().Add(-5 * time.Minute).UTC().UnixMilli()},
			},
		},
	}
	coordinator := &fakeAlertCoordinator{rules: OperationalAlertRules{LongOpenServiceMinutes: 25}}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	_, err := service.Finish(context.Background(), testAccessContext(repository.storeID), FinishCommandInput{
		StoreID:      repository.storeID,
		ServiceID:    "service-1",
		Action:       actionCancel,
		CancelReason: "cliente desistiu",
	})
	if err != nil {
		t.Fatalf("expected cancel Finish to succeed, got %v", err)
	}
	if len(coordinator.received) != 1 || len(coordinator.received[0]) != 1 {
		t.Fatalf("expected one resolve signal batch on cancel, got %#v", coordinator.received)
	}
	signal := coordinator.received[0][0]
	if signal.SignalType != SignalLongOpenServiceResolved {
		t.Fatalf("expected %q, got %q", SignalLongOpenServiceResolved, signal.SignalType)
	}
	if signal.ServiceID != "service-1" {
		t.Fatalf("expected service-1, got %q", signal.ServiceID)
	}
	if signal.Metadata["action"] != actionCancel {
		t.Fatalf("expected cancel metadata action, got %#v", signal.Metadata)
	}
	if signal.Metadata["cancelReason"] != "cliente desistiu" {
		t.Fatalf("expected cancel reason metadata, got %#v", signal.Metadata)
	}
}

func TestFinishStopEmitsResolveSignalWithoutRetrigger(t *testing.T) {
	repository := &testOperationsRepository{
		storeID:     "store-1",
		storeName:   "Loja 1",
		storeExists: true,
		roster: []ConsultantProfile{{
			ID:      "consultant-1",
			StoreID: "store-1",
			Name:    "Consultor 1",
		}},
		snapshot: SnapshotState{
			StoreID: "store-1",
			ActiveServices: []ActiveServiceState{{
				ConsultantID:     "consultant-1",
				ServiceID:        "service-1",
				ServiceStartedAt: time.Now().Add(-30 * time.Minute).UTC().UnixMilli(),
				QueueJoinedAt:    time.Now().Add(-31 * time.Minute).UTC().UnixMilli(),
				QueueWaitMs:      60000,
				StartMode:        startModeQueue,
			}},
			ConsultantCurrentStatus: map[string]ConsultantStatus{
				"consultant-1": {Status: statusService, StartedAt: time.Now().Add(-30 * time.Minute).UTC().UnixMilli()},
			},
		},
	}
	coordinator := &fakeAlertCoordinator{rules: OperationalAlertRules{LongOpenServiceMinutes: 25}}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	_, err := service.Finish(context.Background(), testAccessContext(repository.storeID), FinishCommandInput{
		StoreID:    repository.storeID,
		ServiceID:  "service-1",
		Action:     actionStop,
		StopReason: "cliente ausente",
	})
	if err != nil {
		t.Fatalf("expected stop Finish to succeed, got %v", err)
	}
	if len(coordinator.received) != 1 || len(coordinator.received[0]) != 1 {
		t.Fatalf("expected one resolve signal batch on stop, got %#v", coordinator.received)
	}
	if coordinator.received[0][0].SignalType != SignalLongOpenServiceResolved {
		t.Fatalf("expected %q, got %q", SignalLongOpenServiceResolved, coordinator.received[0][0].SignalType)
	}

	coordinator.received = nil
	if err := service.ProcessTimedAlerts(context.Background()); err != nil {
		t.Fatalf("expected ProcessTimedAlerts after stop to succeed, got %v", err)
	}
	if len(coordinator.received) != 0 {
		t.Fatalf("expected stopped service to be ignored by timed alerts, got %#v", coordinator.received)
	}
}

func TestProcessTimedAlertsEmitsLongOpenSignalOnce(t *testing.T) {
	repository := &testOperationsRepository{
		storeID:     "store-1",
		storeName:   "Loja 1",
		storeExists: true,
		roster: []ConsultantProfile{{
			ID:      "consultant-1",
			StoreID: "store-1",
			Name:    "Consultor 1",
		}},
		snapshot: SnapshotState{
			StoreID: "store-1",
			ActiveServices: []ActiveServiceState{{
				ConsultantID:     "consultant-1",
				ServiceID:        "service-1",
				ServiceStartedAt: time.Now().Add(-30 * time.Minute).UTC().UnixMilli(),
			}},
		},
	}
	coordinator := &fakeAlertCoordinator{rules: OperationalAlertRules{LongOpenServiceMinutes: 25}}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	if err := service.ProcessTimedAlerts(context.Background()); err != nil {
		t.Fatalf("expected first ProcessTimedAlerts to succeed, got %v", err)
	}
	if len(coordinator.received) != 1 || len(coordinator.received[0]) != 1 {
		t.Fatalf("expected one long-open signal on first scan, got %#v", coordinator.received)
	}

	if err := service.ProcessTimedAlerts(context.Background()); err != nil {
		t.Fatalf("expected second ProcessTimedAlerts to succeed, got %v", err)
	}
	if len(coordinator.received) != 1 {
		t.Fatalf("expected timed scan to dedupe repeated signals, got %#v", coordinator.received)
	}

	repository.snapshot.ActiveServices = nil
	if err := service.ProcessTimedAlerts(context.Background()); err != nil {
		t.Fatalf("expected ProcessTimedAlerts to succeed when no services remain, got %v", err)
	}
	if len(coordinator.received) != 1 {
		t.Fatalf("expected no new batches after service resolution, got %#v", coordinator.received)
	}
	if len(service.alertMonitorSeen) != 0 {
		t.Fatalf("expected timed alert cache to be pruned, got %#v", service.alertMonitorSeen)
	}
}
