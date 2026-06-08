package operations

import (
	"context"
	"strings"
)

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

	return service.persistAndAck(ctx, resolvedStoreID, "queue", person.ID, snapshotState, nil, nil)
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

	return service.persistAndAck(ctx, resolvedStoreID, "resume", personID, snapshotState, nil, nil)
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

	return service.persistAndAck(ctx, resolvedStoreID, "start", person.ID, snapshotState, nil, nil)
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

	activeIndex := indexOfActiveService(snapshotState.ActiveServices, personID)
	if activeIndex < 0 {
		return MutationAck{}, ErrConsultantNotAvailable
	}

	maxPerConsultant, err := service.repository.GetMaxConcurrentServicesPerConsultant(ctx, resolvedStoreID)
	if err != nil {
		return MutationAck{}, err
	}

	activeCountForConsultant := countActiveServicesForConsultant(snapshotState.ActiveServices, personID)
	if activeCountForConsultant >= maxPerConsultant {
		return MutationAck{}, ErrConcurrentServiceLimitPerConsultantReached
	}

	maxConcurrentServices, err := service.repository.GetMaxConcurrentServices(ctx, resolvedStoreID)
	if err != nil {
		return MutationAck{}, err
	}

	if len(snapshotState.ActiveServices) >= maxConcurrentServices {
		return MutationAck{}, ErrConcurrentServiceLimitReached
	}

	now := nowUnixMilli()

	anchorService := snapshotState.ActiveServices[activeIndex]
	siblingServiceIDs := extractServiceIDsForConsultant(snapshotState.ActiveServices, personID)
	parallelGroupID := deriveParallelGroupID(snapshotState.ActiveServices, personID, now)
	parallelStartIndex := activeCountForConsultant + 1
	startOffsetMs := deriveStartOffsetMs(snapshotState.ActiveServices, personID, now)
	queuePositionAtStart := deriveQueuePositionAtStart(anchorService, snapshotState.ActiveServices, snapshotState.ServiceHistory)

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

	ack, err := service.persistAndAck(ctx, resolvedStoreID, "start-parallel", personID, snapshotState, nil, nil)
	if err == nil {
		ack.ServiceID = newService.ServiceID
	}

	return ack, err
}
