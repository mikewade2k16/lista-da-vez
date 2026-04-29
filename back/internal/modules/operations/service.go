package operations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
)

const (
	statusAvailable   = "available"
	statusQueue       = "queue"
	statusService     = "service"
	statusPaused      = "paused"
	actionFinish      = "finish"
	actionCancel      = "cancel"
	actionStop        = "stop"
	startModeQueue    = "queue"
	startModeJump     = "queue-jump"
	startModeParallel = "parallel"
	pauseKindPause    = "pause"
	pauseKindTask     = "assignment"
)

var finishOutcomes = map[string]struct{}{
	"reserva":    {},
	"compra":     {},
	"nao-compra": {},
}

type Service struct {
	repository         Repository
	publisher          EventPublisher
	storeScopeProvider StoreScopeProvider
}

type transition struct {
	personID   string
	nextStatus string
}

type noopEventPublisher struct{}

func (noopEventPublisher) PublishOperationEvent(context.Context, PublishedEvent) {}

func NewService(repository Repository, publisher EventPublisher, storeScopeProvider StoreScopeProvider) *Service {
	if publisher == nil {
		publisher = noopEventPublisher{}
	}

	return &Service{
		repository:         repository,
		publisher:          publisher,
		storeScopeProvider: storeScopeProvider,
	}
}

func (service *Service) Snapshot(ctx context.Context, access AccessContext, storeID string) (Snapshot, error) {
	resolvedStoreID, storeName, roster, snapshotState, err := service.loadSnapshot(ctx, access, storeID)
	if err != nil {
		return Snapshot{}, err
	}

	return buildSnapshotView(resolvedStoreID, storeName, roster, snapshotState), nil
}

func (service *Service) Overview(ctx context.Context, access AccessContext) (OperationOverview, error) {
	if !canReadOperations(access) {
		return OperationOverview{}, ErrForbidden
	}

	if service.storeScopeProvider == nil {
		return OperationOverview{}, ErrForbidden
	}

	accessibleStores, err := service.storeScopeProvider.ListAccessible(ctx, access, StoreScopeFilter{})
	if err != nil {
		return OperationOverview{}, err
	}

	overview := OperationOverview{
		Scope:                "accessible-stores",
		Stores:               make([]OperationOverviewStore, 0, len(accessibleStores)),
		WaitingList:          []OperationOverviewPerson{},
		ActiveServices:       []OperationOverviewPerson{},
		PausedEmployees:      []OperationOverviewPerson{},
		AvailableConsultants: []OperationOverviewPerson{},
	}

	for _, storeView := range accessibleStores {
		storeID := strings.TrimSpace(storeView.ID)
		if storeID == "" {
			continue
		}

		roster, snapshotState, err := service.loadSnapshotState(ctx, storeID)
		if err != nil {
			return OperationOverview{}, err
		}

		rosterByID := mapRosterByID(roster)
		waitingByID := map[string]QueueStateItem{}
		activeByID := map[string]ActiveServiceState{}
		pausedByID := map[string]PausedStateItem{}

		for index, item := range snapshotState.WaitingList {
			waitingByID[item.ConsultantID] = item
			person, ok := rosterByID[item.ConsultantID]
			if !ok {
				continue
			}

			overview.WaitingList = append(overview.WaitingList, OperationOverviewPerson{
				StoreID:         storeID,
				StoreName:       strings.TrimSpace(storeView.Name),
				StoreCode:       strings.TrimSpace(storeView.Code),
				PersonID:        person.ID,
				Name:            person.Name,
				Role:            person.Role,
				Initials:        person.Initials,
				Color:           person.Color,
				MonthlyGoal:     person.MonthlyGoal,
				CommissionRate:  person.CommissionRate,
				Status:          statusQueue,
				StatusStartedAt: snapshotState.ConsultantCurrentStatus[person.ID].StartedAt,
				QueueJoinedAt:   item.QueueJoinedAt,
				QueuePosition:   index + 1,
			})
		}

		for _, item := range snapshotState.ActiveServices {
			activeByID[item.ConsultantID] = item
			person, ok := rosterByID[item.ConsultantID]
			if !ok {
				continue
			}

			overview.ActiveServices = append(overview.ActiveServices, OperationOverviewPerson{
				StoreID:            storeID,
				StoreName:          strings.TrimSpace(storeView.Name),
				StoreCode:          strings.TrimSpace(storeView.Code),
				PersonID:           person.ID,
				Name:               person.Name,
				Role:               person.Role,
				Initials:           person.Initials,
				Color:              person.Color,
				MonthlyGoal:        person.MonthlyGoal,
				CommissionRate:     person.CommissionRate,
				Status:             statusService,
				StatusStartedAt:    snapshotState.ConsultantCurrentStatus[person.ID].StartedAt,
				ServiceID:          item.ServiceID,
				ServiceStartedAt:   item.ServiceStartedAt,
				QueueJoinedAt:      item.QueueJoinedAt,
				QueueWaitMs:        item.QueueWaitMs,
				StartMode:          item.StartMode,
				SkippedPeople:      cloneSkippedPeople(item.SkippedPeople),
				ParallelGroupID:    item.ParallelGroupID,
				ParallelStartIndex: item.ParallelStartIndex,
				SiblingServiceIDs:  cloneStringSlice(item.SiblingServiceIDs),
				StartOffsetMs:      item.StartOffsetMs,
			})
		}

		for _, item := range snapshotState.PausedEmployees {
			pausedByID[item.ConsultantID] = item
			person, ok := rosterByID[item.ConsultantID]
			if !ok {
				continue
			}

			overview.PausedEmployees = append(overview.PausedEmployees, OperationOverviewPerson{
				StoreID:         storeID,
				StoreName:       strings.TrimSpace(storeView.Name),
				StoreCode:       strings.TrimSpace(storeView.Code),
				PersonID:        person.ID,
				Name:            person.Name,
				Role:            person.Role,
				Initials:        person.Initials,
				Color:           person.Color,
				MonthlyGoal:     person.MonthlyGoal,
				CommissionRate:  person.CommissionRate,
				Status:          statusPaused,
				StatusStartedAt: snapshotState.ConsultantCurrentStatus[person.ID].StartedAt,
				PauseReason:     item.Reason,
				PauseKind:       normalizePauseKind(item.Kind),
			})
		}

		availableCount := 0
		for _, person := range roster {
			if _, ok := waitingByID[person.ID]; ok {
				continue
			}
			if _, ok := activeByID[person.ID]; ok {
				continue
			}
			if _, ok := pausedByID[person.ID]; ok {
				continue
			}

			availableCount += 1
			status := snapshotState.ConsultantCurrentStatus[person.ID]
			overview.AvailableConsultants = append(overview.AvailableConsultants, OperationOverviewPerson{
				StoreID:         storeID,
				StoreName:       strings.TrimSpace(storeView.Name),
				StoreCode:       strings.TrimSpace(storeView.Code),
				PersonID:        person.ID,
				Name:            person.Name,
				Role:            person.Role,
				Initials:        person.Initials,
				Color:           person.Color,
				MonthlyGoal:     person.MonthlyGoal,
				CommissionRate:  person.CommissionRate,
				Status:          statusAvailable,
				StatusStartedAt: status.StartedAt,
			})
		}

		overview.Stores = append(overview.Stores, OperationOverviewStore{
			StoreID:        storeID,
			StoreName:      strings.TrimSpace(storeView.Name),
			StoreCode:      strings.TrimSpace(storeView.Code),
			City:           strings.TrimSpace(storeView.City),
			WaitingCount:   len(snapshotState.WaitingList),
			ActiveCount:    len(snapshotState.ActiveServices),
			PausedCount:    len(snapshotState.PausedEmployees),
			AvailableCount: availableCount,
		})
	}

	sort.SliceStable(overview.Stores, func(left int, right int) bool {
		return overview.Stores[left].StoreName < overview.Stores[right].StoreName
	})
	sort.SliceStable(overview.WaitingList, func(left int, right int) bool {
		if overview.WaitingList[left].QueueJoinedAt != overview.WaitingList[right].QueueJoinedAt {
			return overview.WaitingList[left].QueueJoinedAt < overview.WaitingList[right].QueueJoinedAt
		}
		return overview.WaitingList[left].Name < overview.WaitingList[right].Name
	})
	sort.SliceStable(overview.ActiveServices, func(left int, right int) bool {
		if overview.ActiveServices[left].ServiceStartedAt != overview.ActiveServices[right].ServiceStartedAt {
			return overview.ActiveServices[left].ServiceStartedAt < overview.ActiveServices[right].ServiceStartedAt
		}
		return overview.ActiveServices[left].Name < overview.ActiveServices[right].Name
	})
	sort.SliceStable(overview.PausedEmployees, func(left int, right int) bool {
		if overview.PausedEmployees[left].StatusStartedAt != overview.PausedEmployees[right].StatusStartedAt {
			return overview.PausedEmployees[left].StatusStartedAt < overview.PausedEmployees[right].StatusStartedAt
		}
		return overview.PausedEmployees[left].Name < overview.PausedEmployees[right].Name
	})
	sort.SliceStable(overview.AvailableConsultants, func(left int, right int) bool {
		if overview.AvailableConsultants[left].StoreName != overview.AvailableConsultants[right].StoreName {
			return overview.AvailableConsultants[left].StoreName < overview.AvailableConsultants[right].StoreName
		}
		return overview.AvailableConsultants[left].Name < overview.AvailableConsultants[right].Name
	})

	return overview, nil
}

func (service *Service) AddToQueue(ctx context.Context, access AccessContext, input QueueCommandInput) (MutationAck, error) {
	resolvedStoreID, _, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	now := nowUnixMilli()
	rosterByID := mapRosterByID(roster)
	personID := strings.TrimSpace(input.PersonID)
	person, ok := rosterByID[personID]
	if !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	if isWaiting(snapshotState.WaitingList, personID) || isInService(snapshotState.ActiveServices, personID) || isPaused(snapshotState.PausedEmployees, personID) {
		return service.buildAck(resolvedStoreID, "queue", personID), nil
	}

	snapshotState.WaitingList = append(snapshotState.WaitingList, QueueStateItem{
		ConsultantID:  person.ID,
		QueueJoinedAt: now,
	})
	snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
		snapshotState.ConsultantActivitySessions,
		snapshotState.ConsultantCurrentStatus,
		[]transition{{personID: person.ID, nextStatus: statusQueue}},
		now,
	)

	return service.persistAndAck(ctx, resolvedStoreID, "queue", person.ID, snapshotState, nil)
}

func (service *Service) Pause(ctx context.Context, access AccessContext, input PauseCommandInput) (MutationAck, error) {
	return service.pauseLike(ctx, access, input, "pause", pauseKindPause, false)
}

func (service *Service) AssignTask(ctx context.Context, access AccessContext, input AssignTaskCommandInput) (MutationAck, error) {
	return service.pauseLike(ctx, access, PauseCommandInput{
		StoreID:  input.StoreID,
		PersonID: input.PersonID,
		Reason:   input.Reason,
	}, "assign-task", pauseKindTask, true)
}

func (service *Service) pauseLike(
	ctx context.Context,
	access AccessContext,
	input PauseCommandInput,
	action string,
	kind string,
	rejectIfInService bool,
) (MutationAck, error) {
	resolvedStoreID, _, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	personID := strings.TrimSpace(input.PersonID)
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return MutationAck{}, ErrValidation
	}

	if _, ok := mapRosterByID(roster)[personID]; !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	if isInService(snapshotState.ActiveServices, personID) {
		if rejectIfInService {
			return MutationAck{}, ErrConsultantBusy
		}

		return service.buildAck(resolvedStoreID, action, personID), nil
	}

	if isPaused(snapshotState.PausedEmployees, personID) {
		return service.buildAck(resolvedStoreID, action, personID), nil
	}

	now := nowUnixMilli()
	snapshotState.WaitingList = filterWaiting(snapshotState.WaitingList, personID)
	snapshotState.PausedEmployees = append(snapshotState.PausedEmployees, PausedStateItem{
		ConsultantID: personID,
		Reason:       reason,
		Kind:         normalizePauseKind(kind),
		StartedAt:    now,
	})
	snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
		snapshotState.ConsultantActivitySessions,
		snapshotState.ConsultantCurrentStatus,
		[]transition{{personID: personID, nextStatus: statusPaused}},
		now,
	)

	return service.persistAndAck(ctx, resolvedStoreID, action, personID, snapshotState, nil)
}

func (service *Service) Resume(ctx context.Context, access AccessContext, input QueueCommandInput) (MutationAck, error) {
	resolvedStoreID, _, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	personID := strings.TrimSpace(input.PersonID)
	if _, ok := mapRosterByID(roster)[personID]; !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	if !isPaused(snapshotState.PausedEmployees, personID) {
		return service.buildAck(resolvedStoreID, "resume", personID), nil
	}

	now := nowUnixMilli()
	snapshotState.PausedEmployees = filterPaused(snapshotState.PausedEmployees, personID)
	if !isWaiting(snapshotState.WaitingList, personID) && !isInService(snapshotState.ActiveServices, personID) {
		snapshotState.WaitingList = append(snapshotState.WaitingList, QueueStateItem{
			ConsultantID:  personID,
			QueueJoinedAt: now,
		})
	}

	nextStatus := statusQueue
	if isInService(snapshotState.ActiveServices, personID) {
		nextStatus = statusService
	}

	snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
		snapshotState.ConsultantActivitySessions,
		snapshotState.ConsultantCurrentStatus,
		[]transition{{personID: personID, nextStatus: nextStatus}},
		now,
	)

	return service.persistAndAck(ctx, resolvedStoreID, "resume", personID, snapshotState, nil)
}

func (service *Service) Start(ctx context.Context, access AccessContext, input StartCommandInput) (MutationAck, error) {
	resolvedStoreID, _, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	if len(snapshotState.WaitingList) == 0 {
		return service.buildAck(resolvedStoreID, "start", ""), nil
	}

	maxConcurrentServices, err := service.repository.GetMaxConcurrentServices(ctx, resolvedStoreID)
	if err != nil {
		return MutationAck{}, err
	}

	if len(snapshotState.ActiveServices) >= maxConcurrentServices {
		return service.buildAck(resolvedStoreID, "start", ""), nil
	}

	targetIndex := 0
	personID := strings.TrimSpace(input.PersonID)
	if personID != "" {
		targetIndex = indexOfWaiting(snapshotState.WaitingList, personID)
		if targetIndex < 0 {
			return service.buildAck(resolvedStoreID, "start", personID), nil
		}
	}

	now := nowUnixMilli()
	nextPerson := snapshotState.WaitingList[targetIndex]
	remainingQueue := make([]QueueStateItem, 0, len(snapshotState.WaitingList)-1)
	for _, item := range snapshotState.WaitingList {
		if item.ConsultantID != nextPerson.ConsultantID {
			remainingQueue = append(remainingQueue, item)
		}
	}

	rosterByID := mapRosterByID(roster)
	person, ok := rosterByID[nextPerson.ConsultantID]
	if !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	skippedPeople := make([]SkippedPerson, 0, targetIndex)
	for _, item := range snapshotState.WaitingList[:targetIndex] {
		if skipped, exists := rosterByID[item.ConsultantID]; exists {
			skippedPeople = append(skippedPeople, SkippedPerson{
				ID:   skipped.ID,
				Name: skipped.Name,
			})
		}
	}

	startMode := startModeQueue
	if targetIndex > 0 {
		startMode = startModeJump
	}

	snapshotState.WaitingList = remainingQueue
	snapshotState.ActiveServices = append(snapshotState.ActiveServices, ActiveServiceState{
		ConsultantID:         person.ID,
		ServiceID:            createServiceID(person.ID, now),
		ServiceStartedAt:     now,
		QueueJoinedAt:        nextPerson.QueueJoinedAt,
		QueueWaitMs:          maxInt64(0, now-nextPerson.QueueJoinedAt),
		QueuePositionAtStart: intPtr(targetIndex + 1),
		StartMode:            startMode,
		SkippedPeople:        skippedPeople,
	})
	snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
		snapshotState.ConsultantActivitySessions,
		snapshotState.ConsultantCurrentStatus,
		[]transition{{personID: person.ID, nextStatus: statusService}},
		now,
	)

	return service.persistAndAck(ctx, resolvedStoreID, "start", person.ID, snapshotState, nil)
}

func (service *Service) StartParallel(ctx context.Context, access AccessContext, input StartParallelCommandInput) (MutationAck, error) {
	resolvedStoreID, _, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	personID := strings.TrimSpace(input.PersonID)
	rosterByID := mapRosterByID(roster)
	if _, ok := rosterByID[personID]; !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	// Check if consultant is currently in service
	activeIndex := indexOfActiveService(snapshotState.ActiveServices, personID)
	if activeIndex < 0 {
		return MutationAck{}, ErrConsultantNotAvailable
	}

	// Get max concurrent services per consultant
	maxPerConsultant, err := service.repository.GetMaxConcurrentServicesPerConsultant(ctx, resolvedStoreID)
	if err != nil {
		return MutationAck{}, err
	}

	// Count active services for this consultant
	activeCountForConsultant := countActiveServicesForConsultant(snapshotState.ActiveServices, personID)
	if activeCountForConsultant >= maxPerConsultant {
		return MutationAck{}, ErrConcurrentServiceLimitPerConsultantReached
	}

	// Check store-level limit still applies
	maxConcurrentServices, err := service.repository.GetMaxConcurrentServices(ctx, resolvedStoreID)
	if err != nil {
		return MutationAck{}, err
	}

	if len(snapshotState.ActiveServices) >= maxConcurrentServices {
		return MutationAck{}, ErrConcurrentServiceLimitReached
	}

	now := nowUnixMilli()

	// Get existing active services for this consultant to compute parallel metadata
	anchorService := snapshotState.ActiveServices[activeIndex]
	siblingServiceIDs := extractServiceIDsForConsultant(snapshotState.ActiveServices, personID)
	parallelGroupID := deriveParallelGroupID(snapshotState.ActiveServices, personID, now)
	parallelStartIndex := activeCountForConsultant + 1
	startOffsetMs := deriveStartOffsetMs(snapshotState.ActiveServices, personID, now)
	queuePositionAtStart := deriveQueuePositionAtStart(anchorService, snapshotState.ActiveServices, snapshotState.ServiceHistory)

	// Create new parallel service
	newService := ActiveServiceState{
		ConsultantID:         personID,
		ServiceID:            createServiceID(personID, now),
		ServiceStartedAt:     now,
		QueueJoinedAt:        anchorService.QueueJoinedAt,
		QueueWaitMs:          anchorService.QueueWaitMs,
		QueuePositionAtStart: queuePositionAtStart,
		StartMode:            "parallel",
		SkippedPeople:        cloneSkippedPeople(anchorService.SkippedPeople),
		ParallelGroupID:      parallelGroupID,
		ParallelStartIndex:   intPtr(parallelStartIndex),
		SiblingServiceIDs:    siblingServiceIDs,
		StartOffsetMs:        startOffsetMs,
	}

	snapshotState.ActiveServices = append(snapshotState.ActiveServices, newService)

	// No status transition: consultant already in 'service' status
	// (applyStatusTransitions will be a noop since consultant is already in 'service')
	ack, err := service.persistAndAck(ctx, resolvedStoreID, "start-parallel", personID, snapshotState, nil)
	if err == nil {
		ack.ServiceID = newService.ServiceID
	}

	return ack, err
}

func (service *Service) Finish(ctx context.Context, access AccessContext, input FinishCommandInput) (MutationAck, error) {
	resolvedStoreID, storeName, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	action := strings.TrimSpace(input.Action)
	if action == "" {
		action = actionFinish
	}

	serviceID := strings.TrimSpace(input.ServiceID)
	if serviceID == "" {
		// Fallback: try to find by PersonID for backward compatibility
		personID := strings.TrimSpace(input.PersonID)
		activeIndex := indexOfActiveService(snapshotState.ActiveServices, personID)
		if activeIndex >= 0 {
			serviceID = snapshotState.ActiveServices[activeIndex].ServiceID
		}
	}

	if serviceID == "" {
		return MutationAck{}, ErrValidation
	}

	if action == actionFinish {
		if _, ok := finishOutcomes[strings.TrimSpace(input.Outcome)]; !ok {
			return MutationAck{}, ErrValidation
		}
	}

	if action != actionFinish && action != actionCancel && action != actionStop {
		return MutationAck{}, ErrValidation
	}

	activeIndex := indexOfActiveServiceByServiceID(snapshotState.ActiveServices, serviceID)
	if activeIndex < 0 {
		return MutationAck{}, ErrValidation
	}

	activeService := snapshotState.ActiveServices[activeIndex]
	personID := activeService.ConsultantID
	now := nowUnixMilli()

	if action == actionStop {
		snapshotState.ActiveServices[activeIndex].StoppedAt = now
		snapshotState.ActiveServices[activeIndex].StopReason = strings.TrimSpace(input.StopReason)
		return service.persistAndAck(ctx, resolvedStoreID, actionStop, personID, snapshotState, nil)
	}

	effectiveFallback := now
	if activeService.StoppedAt > 0 {
		effectiveFallback = activeService.StoppedAt
	}
	effectiveFinishedAt := deriveSequentialServiceEndAt(activeService, snapshotState.ActiveServices, snapshotState.ServiceHistory, effectiveFallback)
	queuePositionAtStart := deriveQueuePositionAtStart(activeService, snapshotState.ActiveServices, snapshotState.ServiceHistory)
	snapshotState.ActiveServices = filterActiveServicesByServiceID(snapshotState.ActiveServices, serviceID)

	rosterByID := mapRosterByID(roster)
	person, ok := rosterByID[personID]
	if !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	// Check if consultant has any remaining active services
	remainingServicesCount := countActiveServicesForConsultant(snapshotState.ActiveServices, personID)
	isLastService := remainingServicesCount == 0

	if action == actionCancel {
		if isLastService {
			// Cancel: reinsere o consultor na posicao relativa correta usando dois criterios:
			// 1o) QueueJoinedAt: quem entrou na fila antes fica na frente.
			// 2o) QueuePositionAtStart como tiebreaker: quando dois consultores
			//     entraram no mesmo milissegundo, o que tinha posicao menor (mais a frente)
			//     na fila original fica na frente.
			originalJoinedAt := activeService.QueueJoinedAt
			originalPos := 0
			if activeService.QueuePositionAtStart != nil {
				originalPos = *activeService.QueuePositionAtStart // 1-indexed
			}

			queueEntry := QueueStateItem{
				ConsultantID:  person.ID,
				QueueJoinedAt: originalJoinedAt,
			}

			insertAt := len(snapshotState.WaitingList)
			for i, entry := range snapshotState.WaitingList {
				if entry.QueueJoinedAt > originalJoinedAt {
					insertAt = i
					break
				}
				// Tiebreaker: mesmo QueueJoinedAt, usar posicao original
				if entry.QueueJoinedAt == originalJoinedAt && originalPos > 0 && i >= originalPos-1 {
					insertAt = i
					break
				}
			}

			tail := make([]QueueStateItem, len(snapshotState.WaitingList[insertAt:]))
			copy(tail, snapshotState.WaitingList[insertAt:])
			snapshotState.WaitingList = append(snapshotState.WaitingList[:insertAt], append([]QueueStateItem{queueEntry}, tail...)...)

			snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
				snapshotState.ConsultantActivitySessions,
				snapshotState.ConsultantCurrentStatus,
				[]transition{{personID: person.ID, nextStatus: statusQueue}},
				now,
			)
		}

		return service.persistAndAck(ctx, resolvedStoreID, actionCancel, person.ID, snapshotState, nil)
	}

	queueEntry := QueueStateItem{
		ConsultantID:  person.ID,
		QueueJoinedAt: now,
	}

	// Only return to queue and transition status if this was the last active service
	if isLastService {
		snapshotState.WaitingList = append(snapshotState.WaitingList, queueEntry)
	}

	historyEntry := normalizeHistoryEntry(ServiceHistoryEntry{
		ServiceID:                  activeService.ServiceID,
		StoreID:                    resolvedStoreID,
		StoreName:                  storeName,
		PersonID:                   person.ID,
		PersonName:                 person.Name,
		StartedAt:                  activeService.ServiceStartedAt,
		FinishedAt:                 effectiveFinishedAt,
		DurationMs:                 maxInt64(0, effectiveFinishedAt-activeService.ServiceStartedAt),
		FinishOutcome:              strings.TrimSpace(input.Outcome),
		StartMode:                  activeService.StartMode,
		QueuePositionAtStart:       queuePositionAtStart,
		QueueWaitMs:                activeService.QueueWaitMs,
		SkippedPeople:              cloneSkippedPeople(activeService.SkippedPeople),
		SkippedCount:               len(activeService.SkippedPeople),
		ParallelGroupID:            activeService.ParallelGroupID,
		ParallelStartIndex:         activeService.ParallelStartIndex,
		SiblingServiceIDs:          cloneStringSlice(activeService.SiblingServiceIDs),
		StartOffsetMs:              activeService.StartOffsetMs,
		IsWindowService:            input.IsWindowService,
		IsGift:                     input.IsGift,
		ProductSeen:                input.ProductSeen,
		ProductClosed:              input.ProductClosed,
		ProductDetails:             input.ProductDetails,
		ProductsSeen:               cloneProducts(input.ProductsSeen),
		ProductsClosed:             cloneProducts(input.ProductsClosed),
		ProductsSeenNone:           input.ProductsSeenNone,
		VisitReasonsNotInformed:    input.VisitReasonsNotInformed,
		CustomerSourcesNotInformed: input.CustomerSourcesNotInformed,
		CustomerName:               input.CustomerName,
		CustomerPhone:              input.CustomerPhone,
		CustomerEmail:              input.CustomerEmail,
		IsExistingCustomer:         input.IsExistingCustomer,
		VisitReasons:               normalizeStringSlice(input.VisitReasons),
		VisitReasonDetails:         normalizeStringMap(input.VisitReasonDetails),
		CustomerSources:            normalizeStringSlice(input.CustomerSources),
		CustomerSourceDetails:      normalizeStringMap(input.CustomerSourceDetails),
		LossReasons:                normalizeStringSlice(input.LossReasons),
		LossReasonDetails:          normalizeStringMap(input.LossReasonDetails),
		LossReasonID:               input.LossReasonID,
		LossReason:                 input.LossReason,
		SaleAmount:                 maxFloat(input.SaleAmount, 0),
		CustomerProfession:         input.CustomerProfession,
		QueueJumpReason:            input.QueueJumpReason,
		StopReason:                 strings.TrimSpace(activeService.StopReason),
		Notes:                      input.Notes,
		CampaignMatches:            normalizeCampaignMatches(input.CampaignMatches),
		CampaignBonusTotal:         maxFloat(input.CampaignBonusTotal, 0),
	})

	if historyEntry.FinishOutcome != "nao-compra" {
		historyEntry.LossReasons = nil
		historyEntry.LossReasonDetails = map[string]string{}
		historyEntry.LossReasonID = ""
		historyEntry.LossReason = ""
	}

	snapshotState.ServiceHistory = append(snapshotState.ServiceHistory, historyEntry)

	// Only transition status if this was the last active service for the consultant
	if isLastService {
		snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
			snapshotState.ConsultantActivitySessions,
			snapshotState.ConsultantCurrentStatus,
			[]transition{{personID: person.ID, nextStatus: statusQueue}},
			now,
		)
	}

	return service.persistAndAck(ctx, resolvedStoreID, actionFinish, person.ID, snapshotState, []ServiceHistoryEntry{historyEntry})
}

func (service *Service) buildAck(storeID string, action string, personID string) MutationAck {
	return MutationAck{
		OK:       true,
		StoreID:  storeID,
		SavedAt:  time.Now().UTC(),
		Action:   strings.TrimSpace(action),
		PersonID: strings.TrimSpace(personID),
	}
}

func (service *Service) persistAndAck(
	ctx context.Context,
	storeID string,
	action string,
	personID string,
	snapshotState SnapshotState,
	appendedHistory []ServiceHistoryEntry,
) (MutationAck, error) {
	appendedSessions := []ConsultantSession{}
	if len(snapshotState.ConsultantActivitySessions) > 0 {
		appendedSessions = []ConsultantSession{
			snapshotState.ConsultantActivitySessions[len(snapshotState.ConsultantActivitySessions)-1],
		}
	}

	if err := service.repository.Persist(ctx, PersistInput{
		StoreID:          storeID,
		WaitingList:      snapshotState.WaitingList,
		ActiveServices:   snapshotState.ActiveServices,
		PausedEmployees:  snapshotState.PausedEmployees,
		CurrentStatus:    snapshotState.ConsultantCurrentStatus,
		AppendedSessions: appendedSessions,
		AppendedHistory:  appendedHistory,
	}); err != nil {
		return MutationAck{}, err
	}

	ack := service.buildAck(storeID, action, personID)
	service.publisher.PublishOperationEvent(ctx, PublishedEvent{
		StoreID:  ack.StoreID,
		Action:   ack.Action,
		PersonID: ack.PersonID,
		SavedAt:  ack.SavedAt,
	})

	return ack, nil
}

func (service *Service) loadSnapshot(
	ctx context.Context,
	access AccessContext,
	storeID string,
) (string, string, []ConsultantProfile, SnapshotState, error) {
	resolvedStoreID, err := service.resolveStoreID(ctx, access, storeID)
	if err != nil {
		return "", "", nil, SnapshotState{}, err
	}

	storeName, err := service.repository.GetStoreName(ctx, resolvedStoreID)
	if err != nil {
		return "", "", nil, SnapshotState{}, err
	}

	roster, snapshotState, err := service.loadSnapshotState(ctx, resolvedStoreID)
	if err != nil {
		return "", "", nil, SnapshotState{}, err
	}

	return resolvedStoreID, storeName, roster, snapshotState, nil
}

func (service *Service) loadSnapshotState(ctx context.Context, storeID string) ([]ConsultantProfile, SnapshotState, error) {
	roster, err := service.repository.ListRoster(ctx, storeID)
	if err != nil {
		return nil, SnapshotState{}, err
	}

	snapshotState, err := service.repository.LoadSnapshot(ctx, storeID)
	if err != nil {
		return nil, SnapshotState{}, err
	}

	return roster, normalizeSnapshotState(storeID, roster, snapshotState), nil
}

func (service *Service) resolveStoreID(ctx context.Context, access AccessContext, storeID string) (string, error) {
	if !canReadOperations(access) {
		return "", ErrForbidden
	}

	trimmedStoreID := strings.TrimSpace(storeID)
	if trimmedStoreID == "" {
		return "", ErrStoreRequired
	}

	exists, err := service.repository.StoreExists(ctx, trimmedStoreID)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", ErrStoreNotFound
	}

	if access.Role == RolePlatformAdmin {
		return trimmedStoreID, nil
	}

	for _, accessibleStoreID := range access.StoreIDs {
		if accessibleStoreID == trimmedStoreID {
			return trimmedStoreID, nil
		}
	}

	return "", ErrForbidden
}

func canReadOperations(access AccessContext) bool {
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(access.Permissions, accesscontrol.PermissionOperationsView)
	}

	return CanAccessOperationsRole(access.Role)
}

func CanAccessOperationsRole(role string) bool {
	switch role {
	case RoleConsultant, RoleStoreTerminal, RoleManager, RoleMarketing, RoleDirector, RoleOwner, RolePlatformAdmin:
		return true
	default:
		return false
	}
}

func CanMutateOperationsRole(role string) bool {
	switch role {
	case RoleConsultant, RoleStoreTerminal, RoleManager, RoleOwner, RolePlatformAdmin:
		return true
	default:
		return false
	}
}

func canMutateOperations(access AccessContext) bool {
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(access.Permissions, accesscontrol.PermissionOperationsEdit)
	}

	return CanMutateOperationsRole(access.Role)
}

func buildSnapshotView(storeID string, storeName string, roster []ConsultantProfile, snapshotState SnapshotState) Snapshot {
	rosterByID := mapRosterByID(roster)
	waitingList := make([]QueueEntry, 0, len(snapshotState.WaitingList))
	for _, item := range snapshotState.WaitingList {
		person, ok := rosterByID[item.ConsultantID]
		if !ok {
			continue
		}

		waitingList = append(waitingList, QueueEntry{
			ID:             person.ID,
			Name:           person.Name,
			Role:           person.Role,
			Initials:       person.Initials,
			Color:          person.Color,
			MonthlyGoal:    person.MonthlyGoal,
			CommissionRate: person.CommissionRate,
			QueueJoinedAt:  item.QueueJoinedAt,
		})
	}

	activeServices := make([]ActiveService, 0, len(snapshotState.ActiveServices))
	for _, item := range snapshotState.ActiveServices {
		person, ok := rosterByID[item.ConsultantID]
		if !ok {
			continue
		}

		activeServices = append(activeServices, ActiveService{
			ID:                   person.ID,
			Name:                 person.Name,
			Role:                 person.Role,
			Initials:             person.Initials,
			Color:                person.Color,
			MonthlyGoal:          person.MonthlyGoal,
			CommissionRate:       person.CommissionRate,
			ServiceID:            item.ServiceID,
			ServiceStartedAt:     item.ServiceStartedAt,
			QueueJoinedAt:        item.QueueJoinedAt,
			QueueWaitMs:          item.QueueWaitMs,
			QueuePositionAtStart: item.QueuePositionAtStart,
			StartMode:            item.StartMode,
			SkippedPeople:        cloneSkippedPeople(item.SkippedPeople),
			ParallelGroupID:      strings.TrimSpace(item.ParallelGroupID),
			ParallelStartIndex:   item.ParallelStartIndex,
			SiblingServiceIDs:    cloneStringSlice(item.SiblingServiceIDs),
			StartOffsetMs:        maxInt64(item.StartOffsetMs, 0),
			StoppedAt:            maxInt64(item.StoppedAt, 0),
			StopReason:           strings.TrimSpace(item.StopReason),
		})
	}

	pausedEmployees := make([]PausedEmployee, 0, len(snapshotState.PausedEmployees))
	for _, item := range snapshotState.PausedEmployees {
		pausedEmployees = append(pausedEmployees, PausedEmployee{
			PersonID:  item.ConsultantID,
			Reason:    item.Reason,
			Kind:      normalizePauseKind(item.Kind),
			StartedAt: item.StartedAt,
		})
	}

	history := make([]ServiceHistoryEntry, 0, len(snapshotState.ServiceHistory))
	for _, entry := range snapshotState.ServiceHistory {
		normalized := normalizeHistoryEntry(entry)
		if normalized.StoreID == "" {
			normalized.StoreID = storeID
		}
		if normalized.StoreName == "" {
			normalized.StoreName = storeName
		}
		history = append(history, normalized)
	}

	return Snapshot{
		StoreID:                    storeID,
		WaitingList:                waitingList,
		ActiveServices:             activeServices,
		PausedEmployees:            pausedEmployees,
		ConsultantActivitySessions: cloneSessions(snapshotState.ConsultantActivitySessions),
		ConsultantCurrentStatus:    cloneCurrentStatus(snapshotState.ConsultantCurrentStatus),
		ServiceHistory:             history,
	}
}

func normalizeSnapshotState(storeID string, roster []ConsultantProfile, snapshotState SnapshotState) SnapshotState {
	rosterByID := mapRosterByID(roster)
	now := nowUnixMilli()

	waitingList := make([]QueueStateItem, 0, len(snapshotState.WaitingList))
	for _, item := range snapshotState.WaitingList {
		if _, ok := rosterByID[item.ConsultantID]; ok {
			waitingList = append(waitingList, QueueStateItem{
				ConsultantID:  item.ConsultantID,
				QueueJoinedAt: item.QueueJoinedAt,
			})
		}
	}

	activeServices := make([]ActiveServiceState, 0, len(snapshotState.ActiveServices))
	for _, item := range snapshotState.ActiveServices {
		if _, ok := rosterByID[item.ConsultantID]; ok {
			activeServices = append(activeServices, ActiveServiceState{
				ConsultantID:         item.ConsultantID,
				ServiceID:            strings.TrimSpace(item.ServiceID),
				ServiceStartedAt:     item.ServiceStartedAt,
				QueueJoinedAt:        item.QueueJoinedAt,
				QueueWaitMs:          item.QueueWaitMs,
				QueuePositionAtStart: item.QueuePositionAtStart,
				StartMode:            normalizeStartMode(item.StartMode),
				SkippedPeople:        cloneSkippedPeople(item.SkippedPeople),
				ParallelGroupID:      strings.TrimSpace(item.ParallelGroupID),
				ParallelStartIndex:   item.ParallelStartIndex,
				SiblingServiceIDs:    cloneStringSlice(item.SiblingServiceIDs),
				StartOffsetMs:        maxInt64(item.StartOffsetMs, 0),
				StoppedAt:            maxInt64(item.StoppedAt, 0),
				StopReason:           strings.TrimSpace(item.StopReason),
			})
		}
	}

	pausedEmployees := make([]PausedStateItem, 0, len(snapshotState.PausedEmployees))
	for _, item := range snapshotState.PausedEmployees {
		if _, ok := rosterByID[item.ConsultantID]; ok {
			pausedEmployees = append(pausedEmployees, PausedStateItem{
				ConsultantID: item.ConsultantID,
				Reason:       strings.TrimSpace(item.Reason),
				Kind:         normalizePauseKind(item.Kind),
				StartedAt:    item.StartedAt,
			})
		}
	}

	currentStatus := map[string]ConsultantStatus{}
	for consultantID, status := range snapshotState.ConsultantCurrentStatus {
		if _, ok := rosterByID[consultantID]; ok {
			currentStatus[consultantID] = ConsultantStatus{
				Status:    normalizeStatus(status.Status),
				StartedAt: status.StartedAt,
			}
		}
	}

	for _, person := range roster {
		derivedStatus := deriveConsultantStatus(waitingList, activeServices, pausedEmployees, person.ID)
		expectedStartedAt := deriveConsultantStartedAt(waitingList, activeServices, pausedEmployees, person.ID, now)
		previous, hasPrevious := currentStatus[person.ID]

		if hasPrevious && previous.Status == derivedStatus {
			startedAt := previous.StartedAt
			if derivedStatus != statusAvailable {
				startedAt = expectedStartedAt
			}

			currentStatus[person.ID] = ConsultantStatus{
				Status:    derivedStatus,
				StartedAt: startedAt,
			}
			continue
		}

		startedAt := expectedStartedAt
		if derivedStatus == statusAvailable {
			startedAt = now
		}

		currentStatus[person.ID] = ConsultantStatus{
			Status:    derivedStatus,
			StartedAt: startedAt,
		}
	}

	return SnapshotState{
		StoreID:                    storeID,
		WaitingList:                waitingList,
		ActiveServices:             activeServices,
		PausedEmployees:            pausedEmployees,
		ConsultantActivitySessions: cloneSessions(snapshotState.ConsultantActivitySessions),
		ConsultantCurrentStatus:    currentStatus,
		ServiceHistory:             cloneHistory(snapshotState.ServiceHistory),
	}
}

func applyStatusTransitions(
	currentSessions []ConsultantSession,
	currentStatus map[string]ConsultantStatus,
	transitions []transition,
	now int64,
) ([]ConsultantSession, map[string]ConsultantStatus) {
	nextSessions := cloneSessions(currentSessions)
	nextStatus := cloneCurrentStatus(currentStatus)

	for _, item := range transitions {
		if item.personID == "" || item.nextStatus == "" {
			continue
		}

		previous, ok := nextStatus[item.personID]
		if !ok {
			previous = ConsultantStatus{
				Status:    statusAvailable,
				StartedAt: now,
			}
		}

		if previous.Status == item.nextStatus {
			nextStatus[item.personID] = previous
			continue
		}

		nextSessions = append(nextSessions, ConsultantSession{
			PersonID:   item.personID,
			Status:     previous.Status,
			StartedAt:  previous.StartedAt,
			EndedAt:    now,
			DurationMs: maxInt64(0, now-previous.StartedAt),
		})

		nextStatus[item.personID] = ConsultantStatus{
			Status:    item.nextStatus,
			StartedAt: now,
		}
	}

	return nextSessions, nextStatus
}

func deriveConsultantStatus(waitingList []QueueStateItem, activeServices []ActiveServiceState, pausedEmployees []PausedStateItem, consultantID string) string {
	if isInService(activeServices, consultantID) {
		return statusService
	}
	if isWaiting(waitingList, consultantID) {
		return statusQueue
	}
	if isPaused(pausedEmployees, consultantID) {
		return statusPaused
	}
	return statusAvailable
}

func deriveConsultantStartedAt(waitingList []QueueStateItem, activeServices []ActiveServiceState, pausedEmployees []PausedStateItem, consultantID string, now int64) int64 {
	for _, item := range activeServices {
		if item.ConsultantID == consultantID {
			return item.ServiceStartedAt
		}
	}
	for _, item := range waitingList {
		if item.ConsultantID == consultantID {
			return item.QueueJoinedAt
		}
	}
	for _, item := range pausedEmployees {
		if item.ConsultantID == consultantID {
			return item.StartedAt
		}
	}
	return now
}

func normalizeHistoryEntry(entry ServiceHistoryEntry) ServiceHistoryEntry {
	entry.ServiceID = strings.TrimSpace(entry.ServiceID)
	entry.StoreID = strings.TrimSpace(entry.StoreID)
	entry.StoreName = strings.TrimSpace(entry.StoreName)
	entry.PersonID = strings.TrimSpace(entry.PersonID)
	entry.PersonName = strings.TrimSpace(entry.PersonName)
	entry.FinishOutcome = normalizeOutcome(entry.FinishOutcome)
	entry.StartMode = normalizeStartMode(entry.StartMode)
	entry.ParallelGroupID = strings.TrimSpace(entry.ParallelGroupID)
	entry.SiblingServiceIDs = normalizeStringSlice(entry.SiblingServiceIDs)
	entry.StartOffsetMs = maxInt64(entry.StartOffsetMs, 0)
	entry.ProductSeen = strings.TrimSpace(entry.ProductSeen)
	entry.ProductClosed = strings.TrimSpace(entry.ProductClosed)
	entry.ProductDetails = strings.TrimSpace(entry.ProductDetails)
	entry.ProductsSeen = cloneProducts(entry.ProductsSeen)
	entry.ProductsClosed = cloneProducts(entry.ProductsClosed)
	entry.CustomerName = strings.TrimSpace(entry.CustomerName)
	entry.CustomerPhone = strings.TrimSpace(entry.CustomerPhone)
	entry.CustomerEmail = strings.TrimSpace(entry.CustomerEmail)
	entry.VisitReasons = normalizeStringSlice(entry.VisitReasons)
	entry.VisitReasonDetails = normalizeStringMap(entry.VisitReasonDetails)
	entry.CustomerSources = normalizeStringSlice(entry.CustomerSources)
	entry.CustomerSourceDetails = normalizeStringMap(entry.CustomerSourceDetails)
	entry.LossReasons = normalizeStringSlice(entry.LossReasons)
	entry.LossReasonDetails = normalizeStringMap(entry.LossReasonDetails)
	entry.LossReasonID = strings.TrimSpace(entry.LossReasonID)
	entry.LossReason = strings.TrimSpace(entry.LossReason)
	entry.CustomerProfession = strings.TrimSpace(entry.CustomerProfession)
	entry.QueueJumpReason = strings.TrimSpace(entry.QueueJumpReason)
	entry.Notes = strings.TrimSpace(entry.Notes)
	entry.CampaignMatches = normalizeCampaignMatches(entry.CampaignMatches)
	entry.CampaignBonusTotal = maxFloat(entry.CampaignBonusTotal, 0)
	entry.SaleAmount = maxFloat(entry.SaleAmount, 0)
	entry.SkippedPeople = cloneSkippedPeople(entry.SkippedPeople)
	entry.SkippedCount = len(entry.SkippedPeople)
	if entry.ProductSeen == "" && len(entry.ProductsSeen) > 0 {
		entry.ProductSeen = entry.ProductsSeen[0].Name
	}
	if entry.ProductClosed == "" && len(entry.ProductsClosed) > 0 {
		entry.ProductClosed = entry.ProductsClosed[0].Name
	}
	if entry.ProductDetails == "" {
		entry.ProductDetails = firstNonEmpty(entry.ProductClosed, entry.ProductSeen)
	}
	return entry
}

func mapRosterByID(roster []ConsultantProfile) map[string]ConsultantProfile {
	index := make(map[string]ConsultantProfile, len(roster))
	for _, consultant := range roster {
		index[consultant.ID] = consultant
	}
	return index
}

func isWaiting(waitingList []QueueStateItem, consultantID string) bool {
	return indexOfWaiting(waitingList, consultantID) >= 0
}

func isInService(activeServices []ActiveServiceState, consultantID string) bool {
	return indexOfActiveService(activeServices, consultantID) >= 0
}

func isPaused(pausedEmployees []PausedStateItem, consultantID string) bool {
	for _, item := range pausedEmployees {
		if item.ConsultantID == consultantID {
			return true
		}
	}
	return false
}

func normalizePauseKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case pauseKindTask:
		return pauseKindTask
	default:
		return pauseKindPause
	}
}

func indexOfWaiting(waitingList []QueueStateItem, consultantID string) int {
	for index, item := range waitingList {
		if item.ConsultantID == consultantID {
			return index
		}
	}
	return -1
}

func indexOfActiveService(activeServices []ActiveServiceState, consultantID string) int {
	for index, item := range activeServices {
		if item.ConsultantID == consultantID {
			return index
		}
	}
	return -1
}

func indexOfActiveServiceByServiceID(activeServices []ActiveServiceState, serviceID string) int {
	for index, item := range activeServices {
		if item.ServiceID == serviceID {
			return index
		}
	}
	return -1
}

func filterWaiting(waitingList []QueueStateItem, consultantID string) []QueueStateItem {
	filtered := make([]QueueStateItem, 0, len(waitingList))
	for _, item := range waitingList {
		if item.ConsultantID != consultantID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterActiveServices(activeServices []ActiveServiceState, consultantID string) []ActiveServiceState {
	filtered := make([]ActiveServiceState, 0, len(activeServices))
	for _, item := range activeServices {
		if item.ConsultantID != consultantID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterActiveServicesByServiceID(activeServices []ActiveServiceState, serviceID string) []ActiveServiceState {
	filtered := make([]ActiveServiceState, 0, len(activeServices))
	for _, item := range activeServices {
		if item.ServiceID != serviceID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterPaused(pausedEmployees []PausedStateItem, consultantID string) []PausedStateItem {
	filtered := make([]PausedStateItem, 0, len(pausedEmployees))
	for _, item := range pausedEmployees {
		if item.ConsultantID != consultantID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func cloneSessions(sessions []ConsultantSession) []ConsultantSession {
	cloned := make([]ConsultantSession, 0, len(sessions))
	for _, item := range sessions {
		cloned = append(cloned, item)
	}
	return cloned
}

func cloneCurrentStatus(currentStatus map[string]ConsultantStatus) map[string]ConsultantStatus {
	cloned := make(map[string]ConsultantStatus, len(currentStatus))
	for key, value := range currentStatus {
		cloned[key] = value
	}
	return cloned
}

func cloneHistory(history []ServiceHistoryEntry) []ServiceHistoryEntry {
	cloned := make([]ServiceHistoryEntry, 0, len(history))
	for _, item := range history {
		cloned = append(cloned, normalizeHistoryEntry(item))
	}
	return cloned
}

func cloneProducts(products []ProductEntry) []ProductEntry {
	cloned := make([]ProductEntry, 0, len(products))
	for _, item := range products {
		cloned = append(cloned, ProductEntry{
			ID:       strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Name),
			Code:     strings.ToUpper(strings.TrimSpace(item.Code)),
			Price:    maxFloat(item.Price, 0),
			IsCustom: item.IsCustom,
		})
	}
	return cloned
}

func cloneSkippedPeople(items []SkippedPerson) []SkippedPerson {
	cloned := make([]SkippedPerson, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, SkippedPerson{
			ID:   strings.TrimSpace(item.ID),
			Name: strings.TrimSpace(item.Name),
		})
	}
	return cloned
}

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	cloned := make([]string, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, item)
	}
	return cloned
}

func normalizeStringSlice(values []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeStringMap(values map[string]string) map[string]string {
	normalized := map[string]string{}
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalized[trimmedKey] = strings.TrimSpace(value)
	}
	return normalized
}

func normalizeCampaignMatches(matches []CampaignMatch) []CampaignMatch {
	normalized := make([]CampaignMatch, 0, len(matches))
	for _, item := range matches {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		if id == "" && name == "" {
			continue
		}
		normalized = append(normalized, CampaignMatch{
			ID:          id,
			Name:        name,
			BonusAmount: maxFloat(item.BonusAmount, 0),
		})
	}
	return normalized
}

func normalizeOutcome(value string) string {
	trimmed := strings.TrimSpace(value)
	if _, ok := finishOutcomes[trimmed]; ok {
		return trimmed
	}
	return "nao-compra"
}

func normalizeStartMode(value string) string {
	switch strings.TrimSpace(value) {
	case startModeJump:
		return startModeJump
	case startModeParallel:
		return startModeParallel
	}
	return startModeQueue
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case statusQueue, statusService, statusPaused:
		return strings.TrimSpace(value)
	default:
		return statusAvailable
	}
}

func createServiceID(personID string, timestamp int64) string {
	buffer := make([]byte, 3)
	if _, err := rand.Read(buffer); err != nil {
		return personID + "-" + time.Now().UTC().Format("20060102150405")
	}
	return personID + "-" + strings.TrimSpace(time.UnixMilli(timestamp).UTC().Format("20060102150405")) + "-" + hex.EncodeToString(buffer)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxFloat(value float64, minimum float64) float64 {
	if value < minimum {
		return minimum
	}
	return value
}

func maxInt64(value int64, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}

func nowUnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
}

func intPtr(v int) *int {
	return &v
}

func countActiveServicesForConsultant(activeServices []ActiveServiceState, consultantID string) int {
	count := 0
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			count++
		}
	}
	return count
}

func extractServiceIDsForConsultant(activeServices []ActiveServiceState, consultantID string) []string {
	ids := make([]string, 0)
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			ids = append(ids, service.ServiceID)
		}
	}
	return ids
}

func deriveParallelGroupID(activeServices []ActiveServiceState, consultantID string, now int64) string {
	// If consultant already has active services, use the first one's parallel group ID
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			if service.ParallelGroupID != "" {
				return service.ParallelGroupID
			}
		}
	}
	// Otherwise, create a new group ID based on first service's timestamp
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			return createServiceID(consultantID, service.ServiceStartedAt)
		}
	}
	// Fallback (shouldn't happen if we validated consultant is in service)
	return createServiceID(consultantID, now)
}

func deriveStartOffsetMs(activeServices []ActiveServiceState, consultantID string, now int64) int64 {
	// Find the earliest started service for this consultant
	var earliestStartedAt int64 = now
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			if service.ServiceStartedAt < earliestStartedAt {
				earliestStartedAt = service.ServiceStartedAt
			}
		}
	}
	return maxInt64(0, now-earliestStartedAt)
}

func deriveQueuePositionAtStart(target ActiveServiceState, activeServices []ActiveServiceState, history []ServiceHistoryEntry) *int {
	if target.QueuePositionAtStart != nil {
		return intPtr(*target.QueuePositionAtStart)
	}

	targetConsultantID := strings.TrimSpace(target.ConsultantID)
	targetGroupID := strings.TrimSpace(target.ParallelGroupID)

	for _, service := range activeServices {
		if service.ServiceID == target.ServiceID {
			continue
		}
		if strings.TrimSpace(service.ConsultantID) != targetConsultantID {
			continue
		}
		if targetGroupID != "" && strings.TrimSpace(service.ParallelGroupID) != targetGroupID {
			continue
		}
		if service.QueuePositionAtStart != nil {
			return intPtr(*service.QueuePositionAtStart)
		}
	}

	for _, entry := range history {
		if strings.TrimSpace(entry.PersonID) != targetConsultantID {
			continue
		}
		if targetGroupID != "" && strings.TrimSpace(entry.ParallelGroupID) != targetGroupID {
			continue
		}
		if entry.QueuePositionAtStart != nil {
			return intPtr(*entry.QueuePositionAtStart)
		}
	}

	return intPtr(1)
}

func deriveSequentialServiceEndAt(target ActiveServiceState, activeServices []ActiveServiceState, history []ServiceHistoryEntry, fallback int64) int64 {
	targetConsultantID := strings.TrimSpace(target.ConsultantID)
	targetGroupID := strings.TrimSpace(target.ParallelGroupID)
	targetStartedAt := target.ServiceStartedAt
	nextStartedAt := int64(0)

	consider := func(candidateStartedAt int64) {
		if candidateStartedAt <= targetStartedAt {
			return
		}
		if nextStartedAt == 0 || candidateStartedAt < nextStartedAt {
			nextStartedAt = candidateStartedAt
		}
	}

	for _, service := range activeServices {
		if service.ServiceID == target.ServiceID {
			continue
		}
		if strings.TrimSpace(service.ConsultantID) != targetConsultantID {
			continue
		}
		if targetGroupID != "" && strings.TrimSpace(service.ParallelGroupID) != targetGroupID {
			continue
		}
		consider(service.ServiceStartedAt)
	}

	for _, entry := range history {
		if strings.TrimSpace(entry.PersonID) != targetConsultantID {
			continue
		}
		if targetGroupID != "" && strings.TrimSpace(entry.ParallelGroupID) != targetGroupID {
			continue
		}
		consider(entry.StartedAt)
	}

	if nextStartedAt > 0 {
		return nextStartedAt
	}

	return maxInt64(fallback, targetStartedAt)
}
