package operations

import (
	"context"
	"errors"
	"testing"
	"time"
)

func autoCloseRules() OperationalAlertRules {
	return OperationalAlertRules{
		LongOpenServiceMinutes: 25,
		AutoCloseEnabled:       true,
		AutoCloseMinutes:       120,
		AutoCloseGraceSeconds:  60,
		SnoozeRepromptMinutes:  30,
	}
}

func TestProcessAutoCloseNoopWhenDisabled(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	rules := autoCloseRules()
	rules.AutoCloseEnabled = false
	coordinator := &fakeAlertCoordinator{rules: rules}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	now := time.Now().UTC().UnixMilli()
	snapshotState := SnapshotState{
		StoreID: "store-1",
		ActiveServices: []ActiveServiceState{{
			ConsultantID:     "consultant-1",
			ServiceID:        "service-1",
			ServiceStartedAt: time.Now().Add(-3 * time.Hour).UTC().UnixMilli(),
		}},
	}

	if err := service.processAutoClose(context.Background(), "store-1", nil, snapshotState, now); err != nil {
		t.Fatalf("processAutoClose: %v", err)
	}
	if len(repository.persisted) != 0 {
		t.Fatalf("expected no persist when auto-close disabled, got %d", len(repository.persisted))
	}
}

func TestProcessAutoCloseOpensGraceWhenThresholdReached(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	coordinator := &fakeAlertCoordinator{rules: autoCloseRules()}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	roster := []ConsultantProfile{{ID: "consultant-1", StoreID: "store-1", Name: "Consultor 1"}}
	startedAt := time.Now().Add(-3 * time.Hour).UTC().UnixMilli()
	now := time.Now().UTC().UnixMilli()
	snapshotState := SnapshotState{
		StoreID: "store-1",
		ActiveServices: []ActiveServiceState{{
			ConsultantID:     "consultant-1",
			ServiceID:        "service-1",
			ServiceStartedAt: startedAt,
			StartMode:        startModeQueue,
		}},
		ConsultantCurrentStatus: map[string]ConsultantStatus{
			"consultant-1": {Status: statusService, StartedAt: startedAt},
		},
	}

	if err := service.processAutoClose(context.Background(), "store-1", roster, snapshotState, now); err != nil {
		t.Fatalf("processAutoClose: %v", err)
	}
	if len(repository.persisted) != 1 {
		t.Fatalf("expected one grace persist, got %d", len(repository.persisted))
	}
	persisted := repository.persisted[0]
	if len(persisted.ActiveServices) != 1 {
		t.Fatalf("expected active service kept during grace, got %d", len(persisted.ActiveServices))
	}
	if got, want := persisted.ActiveServices[0].GraceDeadline, now+60*1000; got != want {
		t.Fatalf("expected grace_deadline %d, got %d", want, got)
	}
	if len(persisted.AppendedHistory) != 0 {
		t.Fatalf("expected no history written when only opening grace, got %d", len(persisted.AppendedHistory))
	}
}

func TestProcessAutoCloseSkipsWhileSnoozed(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	coordinator := &fakeAlertCoordinator{rules: autoCloseRules()}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	startedAt := time.Now().Add(-3 * time.Hour).UTC().UnixMilli()
	now := time.Now().UTC().UnixMilli()
	snapshotState := SnapshotState{
		StoreID: "store-1",
		ActiveServices: []ActiveServiceState{{
			ConsultantID:     "consultant-1",
			ServiceID:        "service-1",
			ServiceStartedAt: startedAt,
			SnoozedUntil:     now + 10*60*1000,
		}},
	}

	if err := service.processAutoClose(context.Background(), "store-1", nil, snapshotState, now); err != nil {
		t.Fatalf("processAutoClose: %v", err)
	}
	if len(repository.persisted) != 0 {
		t.Fatalf("expected no persist while snoozed, got %d", len(repository.persisted))
	}
}

func TestAutoCloseServiceWritesPendingHistoryAndReturnsToQueue(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	coordinator := &fakeAlertCoordinator{rules: autoCloseRules()}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	roster := []ConsultantProfile{{ID: "consultant-1", StoreID: "store-1", Name: "Consultor 1"}}
	startedAt := time.Now().Add(-3 * time.Hour).UTC().UnixMilli()
	now := time.Now().UTC().UnixMilli()
	target := ActiveServiceState{
		ConsultantID:     "consultant-1",
		ServiceID:        "service-1",
		ServiceStartedAt: startedAt,
		QueueJoinedAt:    startedAt - 60_000,
		StartMode:        startModeQueue,
		SnoozeCount:      2,
		GraceDeadline:    now - 1_000,
	}
	snapshotState := SnapshotState{
		StoreID:        "store-1",
		ActiveServices: []ActiveServiceState{target},
		ConsultantCurrentStatus: map[string]ConsultantStatus{
			"consultant-1": {Status: statusService, StartedAt: startedAt},
		},
	}

	if err := service.autoCloseService(context.Background(), "store-1", roster, snapshotState, target, now); err != nil {
		t.Fatalf("autoCloseService: %v", err)
	}
	if len(repository.persisted) != 1 {
		t.Fatalf("expected one persist, got %d", len(repository.persisted))
	}
	persisted := repository.persisted[0]

	if len(persisted.AppendedHistory) != 1 {
		t.Fatalf("expected one history entry, got %d", len(persisted.AppendedHistory))
	}
	entry := persisted.AppendedHistory[0]
	if entry.CloseReason != closeReasonAuto {
		t.Fatalf("expected close_reason %q, got %q", closeReasonAuto, entry.CloseReason)
	}
	if entry.ValidationStatus != validationStatusPending {
		t.Fatalf("expected validation_status %q, got %q", validationStatusPending, entry.ValidationStatus)
	}
	if entry.FinishOutcome != outcomeAuto {
		t.Fatalf("expected finish_outcome %q, got %q", outcomeAuto, entry.FinishOutcome)
	}
	if entry.SnoozeCount != 2 {
		t.Fatalf("expected snooze_count 2, got %d", entry.SnoozeCount)
	}
	if entry.DurationMs <= 0 {
		t.Fatalf("expected positive duration, got %d", entry.DurationMs)
	}

	if len(persisted.ActiveServices) != 0 {
		t.Fatalf("expected active services emptied, got %#v", persisted.ActiveServices)
	}
	found := false
	for _, item := range persisted.WaitingList {
		if item.ConsultantID == "consultant-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected consultant back in queue, got %#v", persisted.WaitingList)
	}

	hasResolve := false
	for _, batch := range coordinator.received {
		for _, signal := range batch {
			if signal.ServiceID == "service-1" && signal.SignalType == SignalLongOpenServiceResolved {
				hasResolve = true
			}
		}
	}
	if !hasResolve {
		t.Fatalf("expected long_open_service resolved signal for service-1, got %#v", coordinator.received)
	}
}

func TestKeepOpenSetsSnoozeAndClearsGrace(t *testing.T) {
	startedAt := time.Now().Add(-3 * time.Hour).UTC().UnixMilli()
	repository := &testOperationsRepository{
		storeID:     "store-1",
		storeName:   "Loja 1",
		storeExists: true,
		roster:      []ConsultantProfile{{ID: "consultant-1", StoreID: "store-1", Name: "Consultor 1"}},
		snapshot: SnapshotState{
			StoreID: "store-1",
			ActiveServices: []ActiveServiceState{{
				ConsultantID:     "consultant-1",
				ServiceID:        "service-1",
				ServiceStartedAt: startedAt,
				GraceDeadline:    time.Now().UTC().UnixMilli(),
				StartMode:        startModeQueue,
			}},
			ConsultantCurrentStatus: map[string]ConsultantStatus{
				"consultant-1": {Status: statusService, StartedAt: startedAt},
			},
		},
	}
	coordinator := &fakeAlertCoordinator{rules: autoCloseRules()}
	service := NewService(repository, nil, nil)
	service.SetAlertCoordinator(coordinator)

	ack, err := service.KeepOpen(context.Background(), testAccessContext("store-1"), KeepOpenCommandInput{StoreID: "store-1", ServiceID: "service-1"})
	if err != nil {
		t.Fatalf("KeepOpen: %v", err)
	}
	if !ack.OK {
		t.Fatalf("expected ok ack")
	}
	if len(repository.persisted) != 1 {
		t.Fatalf("expected one persist, got %d", len(repository.persisted))
	}
	svc := repository.persisted[0].ActiveServices[0]
	if svc.GraceDeadline != 0 {
		t.Fatalf("expected grace cleared, got %d", svc.GraceDeadline)
	}
	if svc.SnoozeCount != 1 {
		t.Fatalf("expected snooze count 1, got %d", svc.SnoozeCount)
	}
	if svc.SnoozedUntil <= time.Now().UTC().UnixMilli() {
		t.Fatalf("expected snoozed_until in the future, got %d", svc.SnoozedUntil)
	}
}

func TestValidateAutoCloseBuildsEntryAndScrubsLoss(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeName: "Loja 1", storeExists: true}
	service := NewService(repository, nil, nil)

	ack, err := service.ValidateAutoClose(context.Background(), testAccessContext("store-1"), FinishCommandInput{
		StoreID:          "store-1",
		ServiceID:        "service-1",
		Outcome:          "compra",
		PurchaseCode:     "ABC123",
		LossReasons:      []string{"deveria-sumir"},
		ValidationReason: "consultor esqueceu de encerrar",
	})
	if err != nil {
		t.Fatalf("ValidateAutoClose: %v", err)
	}
	if ack.ServiceID != "service-1" {
		t.Fatalf("expected service id in ack, got %q", ack.ServiceID)
	}
	if len(repository.validatedEntries) != 1 {
		t.Fatalf("expected one validated entry, got %d", len(repository.validatedEntries))
	}
	entry := repository.validatedEntries[0]
	if entry.FinishOutcome != "compra" {
		t.Fatalf("expected outcome compra, got %q", entry.FinishOutcome)
	}
	if entry.PurchaseCode != "ABC123" {
		t.Fatalf("expected purchase code kept for compra, got %q", entry.PurchaseCode)
	}
	if len(entry.LossReasons) != 0 {
		t.Fatalf("expected loss reasons scrubbed for compra, got %#v", entry.LossReasons)
	}
	if entry.ValidationReason != "consultor esqueceu de encerrar" {
		t.Fatalf("expected validation reason recorded, got %q", entry.ValidationReason)
	}
}

func TestValidateAutoCloseRejectsSentinelOutcome(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeExists: true}
	service := NewService(repository, nil, nil)

	_, err := service.ValidateAutoClose(context.Background(), testAccessContext("store-1"), FinishCommandInput{
		StoreID:          "store-1",
		ServiceID:        "service-1",
		Outcome:          outcomeAuto,
		ValidationReason: "qualquer",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error for sentinel outcome, got %v", err)
	}
}

func TestValidateAutoCloseRequiresValidationReason(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeExists: true}
	service := NewService(repository, nil, nil)

	_, err := service.ValidateAutoClose(context.Background(), testAccessContext("store-1"), FinishCommandInput{
		StoreID:   "store-1",
		ServiceID: "service-1",
		Outcome:   "compra",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error for missing validation reason, got %v", err)
	}
}

func TestCancelAutoCloseRequiresReason(t *testing.T) {
	repository := &testOperationsRepository{storeID: "store-1", storeExists: true}
	service := NewService(repository, nil, nil)

	_, err := service.CancelAutoClose(context.Background(), testAccessContext("store-1"), CancelMetricCommandInput{
		StoreID:   "store-1",
		ServiceID: "service-1",
		Reason:    "   ",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error for empty reason, got %v", err)
	}
}
