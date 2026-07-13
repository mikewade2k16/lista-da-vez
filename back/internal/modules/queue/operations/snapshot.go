package operations

import (
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

// buildSnapshotView e PURO/testavel: recebe o map de GoalStats por consultor
// (chave = consultant.ID de perfil) e embute em cada item da waitingList e dos
// activeServices que tenham entrada no map. map nil/vazio => GoalStats fica nil
// (degradacao graciosa).
func buildSnapshotView(storeID string, storeName string, roster []ConsultantProfile, snapshotState SnapshotState, goalStatsByConsultant map[string]GoalStats) Snapshot {
	rosterByID := mapRosterByID(roster)
	rosterView := make([]RosterMember, 0, len(roster))
	for _, person := range roster {
		rosterView = append(rosterView, RosterMember{
			ID:       person.ID,
			StoreID:  person.StoreID,
			Name:     person.Name,
			Role:     person.Role,
			Initials: person.Initials,
			Color:    person.Color,
		})
	}

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
			GoalStats:      lookupGoalStats(goalStatsByConsultant, person.ID),
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
			EffectiveFinishedAt:  deriveActiveServiceFreezeAt(item, snapshotState.ActiveServices, snapshotState.ServiceHistory),
			StopReason:           strings.TrimSpace(item.StopReason),
			GraceDeadline:        maxInt64(item.GraceDeadline, 0),
			SnoozedUntil:         maxInt64(item.SnoozedUntil, 0),
			SnoozeCount:          maxInt(item.SnoozeCount, 0),
			GoalStats:            lookupGoalStats(goalStatsByConsultant, person.ID),
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
	pendingValidations := make([]PendingValidation, 0)
	for _, entry := range snapshotState.ServiceHistory {
		normalized := normalizeHistoryEntry(entry)
		if normalized.StoreID == "" {
			normalized.StoreID = storeID
		}
		if normalized.StoreName == "" {
			normalized.StoreName = storeName
		}
		history = append(history, normalized)

		if normalized.ValidationStatus == validationStatusPending {
			pendingValidations = append(pendingValidations, PendingValidation{
				ServiceID:    normalized.ServiceID,
				StoreID:      normalized.StoreID,
				StoreName:    normalized.StoreName,
				PersonID:     normalized.PersonID,
				PersonName:   normalized.PersonName,
				StartedAt:    normalized.StartedAt,
				FinishedAt:   normalized.FinishedAt,
				AutoClosedAt: normalized.FinishedAt,
				DurationMs:   normalized.DurationMs,
				SnoozeCount:  normalized.SnoozeCount,
			})
		}
	}

	return Snapshot{
		StoreID:                    storeID,
		Roster:                     rosterView,
		WaitingList:                waitingList,
		ActiveServices:             activeServices,
		PausedEmployees:            pausedEmployees,
		ConsultantActivitySessions: cloneSessions(snapshotState.ConsultantActivitySessions),
		ConsultantCurrentStatus:    cloneCurrentStatus(snapshotState.ConsultantCurrentStatus),
		ServiceHistory:             history,
		PendingValidations:         pendingValidations,
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
				GraceDeadline:        maxInt64(item.GraceDeadline, 0),
				SnoozedUntil:         maxInt64(item.SnoozedUntil, 0),
				SnoozeCount:          maxInt(item.SnoozeCount, 0),
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

		closedSession := ConsultantSession{
			PersonID:   item.personID,
			Status:     previous.Status,
			StartedAt:  previous.StartedAt,
			EndedAt:    now,
			DurationMs: maxInt64(0, now-previous.StartedAt),
		}
		if previous.Status == statusPaused {
			closedSession.Reason = strings.TrimSpace(item.reason)
			closedSession.Kind = normalizePauseKind(item.kind)
		}
		nextSessions = append(nextSessions, closedSession)

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
	entry.PurchaseCode = strings.TrimSpace(entry.PurchaseCode)
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
	entry.ValidationReason = strings.TrimSpace(entry.ValidationReason)
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
		entry.ProductDetails = stringsx.FirstNonEmpty(entry.ProductClosed, entry.ProductSeen)
	}
	return entry
}

// lookupGoalStats devolve um PONTEIRO para uma copia do GoalStats do consultor
// quando ha entrada no map; caso contrario nil. Copiar evita compartilhar a mesma
// struct entre waitingList e activeServices (aliasing acidental no JSON).
func lookupGoalStats(goalStatsByConsultant map[string]GoalStats, consultantID string) *GoalStats {
	if len(goalStatsByConsultant) == 0 {
		return nil
	}
	stats, ok := goalStatsByConsultant[consultantID]
	if !ok {
		return nil
	}
	copyStats := stats
	return &copyStats
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

// pauseReasonAndKind devolve o motivo e o tipo da pausa corrente de um
// consultor (para preservar na sessao de pausa fechada no resume).
func pauseReasonAndKind(pausedEmployees []PausedStateItem, consultantID string) (string, string) {
	for _, item := range pausedEmployees {
		if item.ConsultantID == consultantID {
			return strings.TrimSpace(item.Reason), normalizePauseKind(item.Kind)
		}
	}
	return "", ""
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
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			if service.ParallelGroupID != "" {
				return service.ParallelGroupID
			}
		}
	}
	for _, service := range activeServices {
		if service.ConsultantID == consultantID {
			return createServiceID(consultantID, service.ServiceStartedAt)
		}
	}
	return createServiceID(consultantID, now)
}

func deriveStartOffsetMs(activeServices []ActiveServiceState, consultantID string, now int64) int64 {
	var earliestStartedAt = now
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

func deriveActiveServiceFreezeAt(target ActiveServiceState, activeServices []ActiveServiceState, history []ServiceHistoryEntry) int64 {
	targetConsultantID := strings.TrimSpace(target.ConsultantID)
	targetGroupID := strings.TrimSpace(target.ParallelGroupID)
	targetStartedAt := target.ServiceStartedAt
	freezeAt := int64(0)

	consider := func(candidateStartedAt int64) {
		if candidateStartedAt <= targetStartedAt {
			return
		}
		if freezeAt == 0 || candidateStartedAt < freezeAt {
			freezeAt = candidateStartedAt
		}
	}

	consider(target.StoppedAt)

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

	return freezeAt
}

func deriveSequentialServiceEndAt(target ActiveServiceState, activeServices []ActiveServiceState, history []ServiceHistoryEntry, fallback int64) int64 {
	if freezeAt := deriveActiveServiceFreezeAt(target, activeServices, history); freezeAt > 0 {
		return freezeAt
	}

	return maxInt64(fallback, target.ServiceStartedAt)
}
