package operations

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	statusAvailable      = "available"
	statusQueue          = "queue"
	statusService        = "service"
	statusPaused         = "paused"
	actionFinish         = "finish"
	actionCancel         = "cancel"
	actionStop           = "stop"
	actionAutoClose      = "auto_close"
	actionAutoCloseGrace = "auto_grace"
	actionKeepOpen       = "keep_open"
	actionValidate       = "validate"
	actionCancelMetric   = "cancel_metric"
	startModeQueue       = "queue"
	startModeJump        = "queue-jump"
	startModeParallel    = "parallel"
	pauseKindPause       = "pause"
	pauseKindTask        = "assignment"

	// Auto-encerramento (2h): motivos de fechamento e status de validacao gravados
	// em queue.operation_service_history.
	closeReasonManual = "manual"
	closeReasonAuto   = "auto"

	validationStatusValidated = "validated"
	validationStatusPending   = "pending"
	validationStatusCancelled = "cancelled"

	// outcomeAuto e o sentinela de finish_outcome de um atendimento auto-encerrado
	// (aguardando o gerente gravar o desfecho real na validacao).
	outcomeAuto = "auto"

	// fallbackSnoozeRepromptMinutes e o adiamento do "Continuar" quando a config do
	// tenant nao trouxer um valor valido (mesmo default de tenant_operational_alert_rules).
	fallbackSnoozeRepromptMinutes = 30
)

var finishOutcomes = map[string]struct{}{
	"reserva":    {},
	"compra":     {},
	"nao-compra": {},
}

type Service struct {
	repository           Repository
	publisher            EventPublisher
	storeScopeProvider   StoreScopeProvider
	alertCoordinator     AlertCoordinator
	goalProgressProvider GoalProgressProvider
	alertMonitorMu       sync.Mutex
	alertMonitorSeen     map[string]struct{}

	// Caches em memoria do hot path de leitura (snapshot/overview). Detalhes,
	// TTLs e chaves em service_goal_stats.go.
	effectiveGoalsMu    sync.Mutex
	effectiveGoalsCache map[string]effectiveGoalsCacheEntry
	storeTenantMu       sync.Mutex
	storeTenantCache    map[string]storeTenantCacheEntry
}

type transition struct {
	personID   string
	nextStatus string
	// reason/kind descrevem a pausa que esta sendo encerrada por esta transicao
	// (ex.: resume). So tem efeito quando o status anterior era paused.
	reason string
	kind   string
}

type noopEventPublisher struct{}

func (noopEventPublisher) PublishOperationEvent(context.Context, PublishedEvent) {}

func NewService(repository Repository, publisher EventPublisher, storeScopeProvider StoreScopeProvider) *Service {
	if publisher == nil {
		publisher = noopEventPublisher{}
	}

	return &Service{
		repository:          repository,
		publisher:           publisher,
		storeScopeProvider:  storeScopeProvider,
		alertMonitorSeen:    make(map[string]struct{}),
		effectiveGoalsCache: make(map[string]effectiveGoalsCacheEntry),
		storeTenantCache:    make(map[string]storeTenantCacheEntry),
	}
}

func (service *Service) SetAlertCoordinator(coordinator AlertCoordinator) {
	service.alertCoordinator = coordinator
}

// SetGoalProgressProvider injeta a ponte com o CRM/ERP para enriquecer o snapshot
// e o overview com GoalStats por consultor. Opcional: sem provider, o snapshot
// continua funcionando com GoalStats=nil (degradacao graciosa).
func (service *Service) SetGoalProgressProvider(provider GoalProgressProvider) {
	service.goalProgressProvider = provider
}

func (service *Service) Snapshot(ctx context.Context, access AccessContext, storeID string) (Snapshot, error) {
	resolvedStoreID, storeName, roster, snapshotState, err := service.loadSnapshot(ctx, access, storeID)
	if err != nil {
		return Snapshot{}, err
	}

	// Escopo do ERP: account/tenant do principal; se vazio (ex.: platform_admin em
	// rota RequireAuth sem X-Account-Id), cai no tenant_id da loja (cacheado).
	scopeTenantID := access.ScopeTenantID()
	if scopeTenantID == "" {
		scopeTenantID = service.storeTenantID(ctx, resolvedStoreID)
	}

	providerStats := service.goalStatsForTenant(ctx, scopeTenantID)
	metaByConsultant := service.effectiveGoalsByConsultant(ctx, []string{resolvedStoreID})
	goalStats := combineGoalStats(providerStats, metaByConsultant)

	snapshot := buildSnapshotView(resolvedStoreID, storeName, roster, snapshotState, goalStats)
	snapshot.ServerTime = time.Now().UTC()
	return snapshot, nil
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

	// GoalStats por consultor: meta CANONICA (operation_goal_targets) de todas as
	// lojas acessiveis + vendido do ERP (tenant-wide). Map indexado por consultant.ID
	// de perfil (mesmo id do snapshot/roster).
	overviewStoreIDs := make([]string, 0, len(accessibleStores))
	scopeTenantID := access.ScopeTenantID()
	for _, storeView := range accessibleStores {
		if trimmed := strings.TrimSpace(storeView.ID); trimmed != "" {
			overviewStoreIDs = append(overviewStoreIDs, trimmed)
		}
		// Fallback de escopo ERP quando o principal nao traz account/tenant: usa o
		// tenant_id de uma loja acessivel (todas pertencem a mesma account ativa).
		if scopeTenantID == "" {
			scopeTenantID = strings.TrimSpace(storeView.TenantID)
		}
	}
	providerStats := service.goalStatsForTenant(ctx, scopeTenantID)
	metaByConsultant := service.effectiveGoalsByConsultant(ctx, overviewStoreIDs)
	goalStats := combineGoalStats(providerStats, metaByConsultant)

	overview := OperationOverview{
		Scope:                "accessible-stores",
		Stores:               make([]OperationOverviewStore, 0, len(accessibleStores)),
		WaitingList:          []OperationOverviewPerson{},
		ActiveServices:       []OperationOverviewPerson{},
		PausedEmployees:      []OperationOverviewPerson{},
		AvailableConsultants: []OperationOverviewPerson{},
		PendingValidations:   []PendingValidation{},
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

		// Auto-encerramento (2h): agrega as pendencias de validacao desta loja para a
		// caixa de Pendencias funcionar na visao integrada "Todas as lojas".
		for _, entry := range snapshotState.ServiceHistory {
			if entry.ValidationStatus != validationStatusPending {
				continue
			}
			overview.PendingValidations = append(overview.PendingValidations, PendingValidation{
				ServiceID:    entry.ServiceID,
				StoreID:      storeID,
				StoreName:    strings.TrimSpace(storeView.Name),
				PersonID:     entry.PersonID,
				PersonName:   entry.PersonName,
				StartedAt:    entry.StartedAt,
				FinishedAt:   entry.FinishedAt,
				AutoClosedAt: entry.FinishedAt,
				DurationMs:   entry.DurationMs,
				SnoozeCount:  entry.SnoozeCount,
			})
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
				GoalStats:       lookupGoalStats(goalStats, person.ID),
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
				GoalStats:            lookupGoalStats(goalStats, person.ID),
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
				GoalStats:       lookupGoalStats(goalStats, person.ID),
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
				GoalStats:       lookupGoalStats(goalStats, person.ID),
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

	overview.ServerTime = time.Now().UTC()
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
	return service.loadSnapshotInternal(ctx, access, storeID, false)
}

// loadSnapshotAllowArchived carrega snapshot mesmo se a loja estiver arquivada.
// Usado por Finish para permitir encerrar atendimentos em curso apos arquivamento.
func (service *Service) loadSnapshotAllowArchived(
	ctx context.Context,
	access AccessContext,
	storeID string,
) (string, string, []ConsultantProfile, SnapshotState, error) {
	return service.loadSnapshotInternal(ctx, access, storeID, true)
}

func (service *Service) loadSnapshotInternal(
	ctx context.Context,
	access AccessContext,
	storeID string,
	allowArchived bool,
) (string, string, []ConsultantProfile, SnapshotState, error) {
	var resolvedStoreID string
	var err error
	if allowArchived {
		resolvedStoreID, err = service.resolveStoreIDAllowArchived(ctx, access, storeID)
	} else {
		resolvedStoreID, err = service.resolveStoreID(ctx, access, storeID)
	}
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
	return service.resolveStoreIDInternal(ctx, access, storeID, false)
}

// resolveStoreIDAllowArchived aceita lojas arquivadas. Usado pelo Finish para
// permitir encerrar atendimentos ja em andamento mesmo apos a loja ser
// arquivada.
func (service *Service) resolveStoreIDAllowArchived(ctx context.Context, access AccessContext, storeID string) (string, error) {
	return service.resolveStoreIDInternal(ctx, access, storeID, true)
}

func (service *Service) resolveStoreIDInternal(ctx context.Context, access AccessContext, storeID string, allowArchived bool) (string, error) {
	if !canReadOperations(access) {
		return "", ErrForbidden
	}

	trimmedStoreID := strings.TrimSpace(storeID)
	if trimmedStoreID == "" {
		return "", ErrStoreRequired
	}

	var exists bool
	var err error
	if allowArchived {
		exists, err = service.repository.StoreExistsIncludingArchived(ctx, trimmedStoreID)
	} else {
		exists, err = service.repository.StoreExists(ctx, trimmedStoreID)
	}
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
