package analytics

import (
	"context"
	"strings"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type Service struct {
	repository  Repository
	storeFinder StoreFinder
}

type bundle struct {
	storeID   string
	tenantID  string
	history   []operations.ServiceHistoryEntry
	roster    []operations.ConsultantProfile
	snapshot  operations.SnapshotState
	settings  StoreSettings
	storeView stores.StoreView
}

type analyticsScope struct {
	storeID  string
	tenantID string
}

func NewService(repository Repository, storeFinder StoreFinder) *Service {
	return &Service{
		repository:  repository,
		storeFinder: storeFinder,
	}
}

func (service *Service) Ranking(ctx context.Context, principal auth.Principal, storeID string, tenantID string) (RankingResponse, error) {
	if !canViewRanking(principal) {
		return RankingResponse{}, ErrForbidden
	}

	scope, bundles, err := service.loadBundles(ctx, principal, storeID, tenantID)
	if err != nil {
		return RankingResponse{}, err
	}

	return RankingResponse{
		StoreID:     scope.storeID,
		TenantID:    scope.tenantID,
		MonthlyRows: buildRankingRowsAcrossBundles(bundles, "month"),
		DailyRows:   buildRankingRowsAcrossBundles(bundles, "today"),
		Alerts:      buildConsultantAlertsAcrossBundles(bundles),
	}, nil
}

func (service *Service) Data(ctx context.Context, principal auth.Principal, storeID string, tenantID string) (DataResponse, error) {
	if !canViewData(principal) {
		return DataResponse{}, ErrForbidden
	}

	scope, bundles, err := service.loadBundles(ctx, principal, storeID, tenantID)
	if err != nil {
		return DataResponse{}, err
	}

	combinedHistory := combineHistory(bundles)
	labelSettings := combineSettingsLabels(bundles)
	return DataResponse{
		StoreID:           scope.storeID,
		TenantID:          scope.tenantID,
		TimeIntelligence:  buildCombinedTimeIntelligence(bundles),
		SoldProducts:      buildSoldProducts(combinedHistory),
		RequestedProducts: buildRequestedProducts(combinedHistory),
		VisitReasons:      buildVisitReasons(combinedHistory, labelSettings.VisitReasonLabels),
		CustomerSources:   buildCustomerSources(combinedHistory, labelSettings.CustomerSourceLabels),
		Professions:       buildProfessions(combinedHistory),
		OutcomeSummary:    buildOutcomeSummary(combinedHistory),
		HourlySales:       buildHourlySales(combinedHistory),
	}, nil
}

func (service *Service) Intelligence(ctx context.Context, principal auth.Principal, storeID string, tenantID string) (IntelligenceResponse, error) {
	if !canViewIntelligence(principal) {
		return IntelligenceResponse{}, ErrForbidden
	}

	scope, bundles, err := service.loadBundles(ctx, principal, storeID, tenantID)
	if err != nil {
		return IntelligenceResponse{}, err
	}

	return buildOperationalIntelligenceSummary(scope.storeID, scope.tenantID, combineHistory(bundles), buildCombinedTimeIntelligence(bundles)), nil
}

func canViewRanking(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionRankingView)
	}

	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner || principal.Role == auth.RoleStoreTerminal
}

func canViewData(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionDataView)
	}

	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner || principal.Role == auth.RoleStoreTerminal
}

func canViewIntelligence(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionIntelligenceView)
	}

	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner || principal.Role == auth.RoleStoreTerminal
}

func (service *Service) loadBundles(ctx context.Context, principal auth.Principal, storeID string, tenantID string) (analyticsScope, []bundle, error) {
	normalizedStoreID := strings.TrimSpace(storeID)
	if normalizedStoreID != "" {
		store, err := service.storeFinder.FindAccessible(ctx, principal, normalizedStoreID)
		if err != nil {
			return analyticsScope{}, nil, err
		}

		storeBundle, err := service.loadStoreBundle(ctx, store)
		if err != nil {
			return analyticsScope{}, nil, err
		}

		return analyticsScope{
			storeID:  store.ID,
			tenantID: store.TenantID,
		}, []bundle{storeBundle}, nil
	}

	resolvedTenantID := firstNonEmpty(tenantID, principal.TenantID)
	if resolvedTenantID == "" {
		return analyticsScope{}, nil, ErrScopeRequired
	}

	storeRows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{
		TenantID: resolvedTenantID,
	})
	if err != nil {
		return analyticsScope{}, nil, err
	}

	bundles := make([]bundle, 0, len(storeRows))
	for _, store := range storeRows {
		storeBundle, loadErr := service.loadStoreBundle(ctx, store)
		if loadErr != nil {
			return analyticsScope{}, nil, loadErr
		}

		bundles = append(bundles, storeBundle)
	}

	return analyticsScope{
		tenantID: resolvedTenantID,
	}, bundles, nil
}

func (service *Service) loadStoreBundle(ctx context.Context, store stores.StoreView) (bundle, error) {
	snapshot, err := service.repository.LoadSnapshot(ctx, store.ID)
	if err != nil {
		return bundle{}, err
	}

	roster, err := service.repository.ListRoster(ctx, store.ID)
	if err != nil {
		return bundle{}, err
	}

	settings, err := service.repository.LoadSettings(ctx, store.ID)
	if err != nil {
		return bundle{}, err
	}

	return bundle{
		storeID:   store.ID,
		tenantID:  store.TenantID,
		history:   snapshot.ServiceHistory,
		roster:    roster,
		snapshot:  snapshot,
		settings:  settings,
		storeView: store,
	}, nil
}

func combineHistory(bundles []bundle) []operations.ServiceHistoryEntry {
	total := 0
	for _, item := range bundles {
		total += len(item.history)
	}

	history := make([]operations.ServiceHistoryEntry, 0, total)
	for _, item := range bundles {
		history = append(history, item.history...)
	}

	return history
}

func combineSettingsLabels(bundles []bundle) StoreSettings {
	combined := StoreSettings{
		VisitReasonLabels:    map[string]string{},
		CustomerSourceLabels: map[string]string{},
	}

	for _, item := range bundles {
		for optionID, label := range item.settings.VisitReasonLabels {
			if _, exists := combined.VisitReasonLabels[optionID]; !exists && strings.TrimSpace(label) != "" {
				combined.VisitReasonLabels[optionID] = strings.TrimSpace(label)
			}
		}

		for optionID, label := range item.settings.CustomerSourceLabels {
			if _, exists := combined.CustomerSourceLabels[optionID]; !exists && strings.TrimSpace(label) != "" {
				combined.CustomerSourceLabels[optionID] = strings.TrimSpace(label)
			}
		}
	}

	return combined
}
