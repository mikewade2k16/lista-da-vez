package operations

import (
	"context"
	"sort"
	"strings"
	"sync"
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
	alertCoordinator   AlertCoordinator
	alertMonitorMu     sync.Mutex
	alertMonitorSeen   map[string]struct{}
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
		alertMonitorSeen:   make(map[string]struct{}),
	}
}

func (service *Service) SetAlertCoordinator(coordinator AlertCoordinator) {
	service.alertCoordinator = coordinator
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
				StoreID:              storeID,
				StoreName:            strings.TrimSpace(storeView.Name),
				StoreCode:            strings.TrimSpace(storeView.Code),
				PersonID:             person.ID,
				Name:                 person.Name,
				Role:                 person.Role,
				Initials:             person.Initials,
				Color:                person.Color,
				MonthlyGoal:          person.MonthlyGoal,
				CommissionRate:       person.CommissionRate,
				Status:               statusService,
				StatusStartedAt:      snapshotState.ConsultantCurrentStatus[person.ID].StartedAt,
				ServiceID:            item.ServiceID,
				ServiceStartedAt:     item.ServiceStartedAt,
				QueueJoinedAt:        item.QueueJoinedAt,
				QueueWaitMs:          item.QueueWaitMs,
				QueuePositionAtStart: item.QueuePositionAtStart,
				StartMode:            item.StartMode,
				SkippedPeople:        cloneSkippedPeople(item.SkippedPeople),
				ParallelGroupID:      item.ParallelGroupID,
				ParallelStartIndex:   item.ParallelStartIndex,
				SiblingServiceIDs:    cloneStringSlice(item.SiblingServiceIDs),
				StartOffsetMs:        item.StartOffsetMs,
				StoppedAt:            maxInt64(item.StoppedAt, 0),
				EffectiveFinishedAt:  deriveActiveServiceFreezeAt(item, snapshotState.ActiveServices, snapshotState.ServiceHistory),
				StopReason:           strings.TrimSpace(item.StopReason),
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
	explicitSignals []OperationalAlertSignal,
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
	service.emitAlertSignals(ctx, ack.StoreID, snapshotState, appendedHistory, explicitSignals)

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
