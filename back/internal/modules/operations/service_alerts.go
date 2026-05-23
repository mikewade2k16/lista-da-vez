package operations

import (
	"context"
	"strings"
	"time"
)

func (service *Service) ProcessTimedAlerts(ctx context.Context) error {
	if service.alertCoordinator == nil {
		return nil
	}

	storeIDs, err := service.repository.ListStoresWithActiveServices(ctx)
	if err != nil {
		return err
	}

	currentActiveServiceIDs := make(map[string]struct{})
	for _, rawStoreID := range storeIDs {
		storeID := strings.TrimSpace(rawStoreID)
		if storeID == "" {
			continue
		}

		_, snapshotState, err := service.loadSnapshotState(ctx, storeID)
		if err != nil {
			continue
		}

		for _, activeService := range snapshotState.ActiveServices {
			if !shouldMonitorLongOpenAlert(activeService) {
				continue
			}
			serviceID := strings.TrimSpace(activeService.ServiceID)
			currentActiveServiceIDs[serviceID] = struct{}{}
		}

		triggerSignals, err := service.buildLongOpenSignals(ctx, storeID, snapshotState, time.Now().UTC())
		if err != nil {
			continue
		}

		triggerSignals = service.filterUnseenTimedAlertSignals(triggerSignals)
		if len(triggerSignals) == 0 {
			continue
		}

		if err := service.alertCoordinator.ReceiveOperationalSignals(ctx, triggerSignals); err == nil {
			service.markTimedAlertSignals(triggerSignals)
		}
	}

	service.pruneTimedAlertSignals(currentActiveServiceIDs)
	return nil
}

func (service *Service) emitAlertSignals(ctx context.Context, storeID string, snapshotState SnapshotState, appendedHistory []ServiceHistoryEntry, explicitSignals []OperationalAlertSignal) {
	if service.alertCoordinator == nil || strings.TrimSpace(storeID) == "" {
		return
	}

	now := time.Now().UTC()
	triggerSignals, err := service.buildLongOpenSignals(ctx, storeID, snapshotState, now)
	signals := make([]OperationalAlertSignal, 0, len(triggerSignals)+len(appendedHistory)+len(explicitSignals))
	if err == nil {
		signals = append(signals, triggerSignals...)
	}

	signals = append(signals, buildLongOpenResolvedSignals(strings.TrimSpace(storeID), appendedHistory, now)...)
	signals = append(signals, explicitSignals...)

	if len(signals) == 0 {
		return
	}

	// Best effort only: alert orchestration cannot block the authoritative operation mutation.
	_ = service.alertCoordinator.ReceiveOperationalSignals(ctx, signals)
}

func (service *Service) buildLongOpenSignals(ctx context.Context, storeID string, snapshotState SnapshotState, now time.Time) ([]OperationalAlertSignal, error) {
	rules, err := service.alertCoordinator.LoadOperationalRules(ctx, storeID)
	if err != nil || rules.LongOpenServiceMinutes <= 0 {
		return nil, err
	}

	threshold := time.Duration(rules.LongOpenServiceMinutes) * time.Minute
	seenServices := make(map[string]struct{}, len(snapshotState.ActiveServices))
	signals := make([]OperationalAlertSignal, 0, len(snapshotState.ActiveServices))
	for _, activeService := range snapshotState.ActiveServices {
		if !shouldMonitorLongOpenAlert(activeService) {
			continue
		}
		serviceID := strings.TrimSpace(activeService.ServiceID)
		if _, exists := seenServices[serviceID]; exists {
			continue
		}
		seenServices[serviceID] = struct{}{}

		startedAt := time.UnixMilli(activeService.ServiceStartedAt).UTC()
		elapsed := now.Sub(startedAt)
		if elapsed < threshold {
			continue
		}

		signals = append(signals, OperationalAlertSignal{
			StoreID:        strings.TrimSpace(storeID),
			ServiceID:      serviceID,
			ConsultantID:   strings.TrimSpace(activeService.ConsultantID),
			SignalType:     SignalLongOpenServiceTriggered,
			TriggeredAt:    now,
			ElapsedMinutes: int(elapsed.Minutes()),
			TriggerType:    TriggerLongOpenService,
			Metadata: map[string]any{
				"serviceStartedAt": activeService.ServiceStartedAt,
				"queueWaitMs":      activeService.QueueWaitMs,
				"thresholdMinutes": rules.LongOpenServiceMinutes,
				"startMode":        strings.TrimSpace(activeService.StartMode),
			},
		})
	}

	return signals, nil
}

func buildLongOpenResolvedSignals(storeID string, appendedHistory []ServiceHistoryEntry, fallback time.Time) []OperationalAlertSignal {
	signals := make([]OperationalAlertSignal, 0, len(appendedHistory))
	for _, historyEntry := range appendedHistory {
		signal := buildLongOpenResolvedSignal(
			storeID,
			historyEntry.ServiceID,
			historyEntry.PersonID,
			fallbackResolvedAt(historyEntry.FinishedAt, fallback),
			map[string]any{
				"action":        actionFinish,
				"finishOutcome": strings.TrimSpace(historyEntry.FinishOutcome),
				"finishedAt":    historyEntry.FinishedAt,
			},
		)
		if signal.ServiceID == "" {
			continue
		}
		signals = append(signals, signal)
	}

	return signals
}

func buildLongOpenResolvedSignal(storeID string, serviceID string, consultantID string, triggeredAt time.Time, metadata map[string]any) OperationalAlertSignal {
	return OperationalAlertSignal{
		StoreID:      strings.TrimSpace(storeID),
		ServiceID:    strings.TrimSpace(serviceID),
		ConsultantID: strings.TrimSpace(consultantID),
		SignalType:   SignalLongOpenServiceResolved,
		TriggeredAt:  triggeredAt,
		Metadata:     metadata,
	}
}

func fallbackResolvedAt(finishedAt int64, fallback time.Time) time.Time {
	if finishedAt > 0 {
		return time.UnixMilli(finishedAt).UTC()
	}

	return fallback
}

func shouldMonitorLongOpenAlert(activeService ActiveServiceState) bool {
	if strings.TrimSpace(activeService.ServiceID) == "" {
		return false
	}
	if activeService.ServiceStartedAt <= 0 {
		return false
	}
	if activeService.StoppedAt > 0 {
		return false
	}

	return true
}

func (service *Service) filterUnseenTimedAlertSignals(signals []OperationalAlertSignal) []OperationalAlertSignal {
	service.alertMonitorMu.Lock()
	defer service.alertMonitorMu.Unlock()

	filtered := make([]OperationalAlertSignal, 0, len(signals))
	for _, signal := range signals {
		serviceID := strings.TrimSpace(signal.ServiceID)
		if serviceID == "" {
			continue
		}
		if _, seen := service.alertMonitorSeen[serviceID]; seen {
			continue
		}
		filtered = append(filtered, signal)
	}

	return filtered
}

func (service *Service) markTimedAlertSignals(signals []OperationalAlertSignal) {
	service.alertMonitorMu.Lock()
	defer service.alertMonitorMu.Unlock()

	for _, signal := range signals {
		serviceID := strings.TrimSpace(signal.ServiceID)
		if serviceID == "" {
			continue
		}
		service.alertMonitorSeen[serviceID] = struct{}{}
	}
}

func (service *Service) pruneTimedAlertSignals(currentActiveServiceIDs map[string]struct{}) {
	service.alertMonitorMu.Lock()
	defer service.alertMonitorMu.Unlock()

	for serviceID := range service.alertMonitorSeen {
		if _, ok := currentActiveServiceIDs[serviceID]; ok {
			continue
		}
		delete(service.alertMonitorSeen, serviceID)
	}
}

func (service *Service) buildLongQueueWaitSignals(ctx context.Context, storeID string, snapshotState SnapshotState, now time.Time) ([]OperationalAlertSignal, error) {
	signals := make([]OperationalAlertSignal, 0)
	return signals, nil
}

func (service *Service) buildLongPauseSignals(ctx context.Context, storeID string, snapshotState SnapshotState, now time.Time) ([]OperationalAlertSignal, error) {
	signals := make([]OperationalAlertSignal, 0)
	return signals, nil
}

func (service *Service) buildIdleStoreSignals(ctx context.Context, storeID string, snapshotState SnapshotState, now time.Time) ([]OperationalAlertSignal, error) {
	signals := make([]OperationalAlertSignal, 0)
	return signals, nil
}

func (service *Service) buildOutsideBusinessHoursSignals(ctx context.Context, storeID string, snapshotState SnapshotState, now time.Time) ([]OperationalAlertSignal, error) {
	signals := make([]OperationalAlertSignal, 0)
	return signals, nil
}

// ScanForRule implements the alerts retroactive scanner without making
// operations depend on the alerts package.
func (service *Service) ScanForRule(ctx context.Context, ruleID string, triggerType string, tenantID string, thresholdMinutes int) ([]OperationalAlertSignal, error) {
	if strings.TrimSpace(triggerType) != TriggerLongOpenService || thresholdMinutes < 1 {
		return []OperationalAlertSignal{}, nil
	}

	storeIDs, err := service.repository.ListStoresWithActiveServicesByTenant(ctx, strings.TrimSpace(tenantID))
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	threshold := time.Duration(thresholdMinutes) * time.Minute
	signals := make([]OperationalAlertSignal, 0)
	for _, rawStoreID := range storeIDs {
		storeID := strings.TrimSpace(rawStoreID)
		if storeID == "" {
			continue
		}

		roster, snapshotState, err := service.loadSnapshotState(ctx, storeID)
		if err != nil {
			continue
		}

		consultantNames := make(map[string]string, len(roster))
		for _, consultant := range roster {
			consultantNames[strings.TrimSpace(consultant.ID)] = strings.TrimSpace(consultant.Name)
		}

		for _, activeService := range snapshotState.ActiveServices {
			if !shouldMonitorLongOpenAlert(activeService) {
				continue
			}

			startedAt := time.UnixMilli(activeService.ServiceStartedAt).UTC()
			elapsed := now.Sub(startedAt)
			if elapsed < threshold {
				continue
			}

			consultantID := strings.TrimSpace(activeService.ConsultantID)
			serviceID := strings.TrimSpace(activeService.ServiceID)
			signals = append(signals, OperationalAlertSignal{
				TenantID:       strings.TrimSpace(tenantID),
				StoreID:        storeID,
				ServiceID:      serviceID,
				ConsultantID:   consultantID,
				SignalType:     SignalLongOpenServiceTriggered,
				TriggeredAt:    now,
				ConsultantName: consultantNames[consultantID],
				ElapsedMinutes: int(elapsed.Minutes()),
				TriggerType:    TriggerLongOpenService,
				Metadata: map[string]any{
					"ruleDefinitionId": strings.TrimSpace(ruleID),
					"serviceStartedAt": activeService.ServiceStartedAt,
					"queueWaitMs":      activeService.QueueWaitMs,
					"thresholdMinutes": thresholdMinutes,
					"startMode":        strings.TrimSpace(activeService.StartMode),
				},
			})
		}
	}

	return signals, nil
}
