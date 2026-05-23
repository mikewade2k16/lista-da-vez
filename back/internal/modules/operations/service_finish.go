package operations

import (
	"context"
	"strings"
	"time"
)

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
		return service.persistAndAck(ctx, resolvedStoreID, actionStop, personID, snapshotState, nil, []OperationalAlertSignal{buildLongOpenResolvedSignal(
			resolvedStoreID,
			activeService.ServiceID,
			personID,
			time.UnixMilli(now).UTC(),
			map[string]any{
				"action":     actionStop,
				"stoppedAt":  now,
				"stopReason": strings.TrimSpace(input.StopReason),
			},
		)})
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
				originalPos = *activeService.QueuePositionAtStart
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

		return service.persistAndAck(ctx, resolvedStoreID, actionCancel, person.ID, snapshotState, nil, []OperationalAlertSignal{buildLongOpenResolvedSignal(
			resolvedStoreID,
			activeService.ServiceID,
			person.ID,
			time.UnixMilli(now).UTC(),
			map[string]any{
				"action":       actionCancel,
				"cancelledAt":  now,
				"cancelReason": strings.TrimSpace(input.CancelReason),
			},
		)})
	}

	queueEntry := QueueStateItem{
		ConsultantID:  person.ID,
		QueueJoinedAt: now,
	}

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
		PurchaseCode:               input.PurchaseCode,
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
	if historyEntry.FinishOutcome != "compra" {
		historyEntry.PurchaseCode = ""
	}

	snapshotState.ServiceHistory = append(snapshotState.ServiceHistory, historyEntry)

	if isLastService {
		snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
			snapshotState.ConsultantActivitySessions,
			snapshotState.ConsultantCurrentStatus,
			[]transition{{personID: person.ID, nextStatus: statusQueue}},
			now,
		)
	}

	return service.persistAndAck(ctx, resolvedStoreID, actionFinish, person.ID, snapshotState, []ServiceHistoryEntry{historyEntry}, nil)
}
