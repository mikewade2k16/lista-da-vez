package reports

import (
	"context"
	"errors"
	"sort"
	"strings"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

const (
	defaultPageSize   = 50
	maxPageSize       = 200
	defaultRecentSize = 20
)

var (
	ErrStoreRequired = errors.New("store required")
	ErrValidation    = errors.New("validation error")
)

type Repository interface {
	ListHistory(ctx context.Context, storeID string, filters repositoryFilters) ([]operations.ServiceHistoryEntry, error)
	ListHistoryByStores(ctx context.Context, storeIDs []string, filters repositoryFilters) ([]operations.ServiceHistoryEntry, error)
	ListLiveCounts(ctx context.Context, storeIDs []string) (map[string]StoreLiveCounts, error)
}

type StoreFinder interface {
	ListAccessible(ctx context.Context, principal auth.Principal, input stores.ListInput) ([]stores.StoreView, error)
	FindAccessible(ctx context.Context, principal auth.Principal, storeID string) (stores.StoreView, error)
}

type Service struct {
	repository  Repository
	storeFinder StoreFinder
}

type reportScope struct {
	StoreID   string
	TenantID  string
	StoreName string
}

func NewService(repository Repository, storeFinder StoreFinder) *Service {
	return &Service{
		repository:  repository,
		storeFinder: storeFinder,
	}
}

func (service *Service) Overview(ctx context.Context, principal auth.Principal, filters Filters) (OverviewResponse, error) {
	if !canViewReports(principal) {
		return OverviewResponse{}, stores.ErrForbidden
	}

	scope, normalized, history, err := service.loadEntries(ctx, principal, filters)
	if err != nil {
		return OverviewResponse{}, err
	}

	return OverviewResponse{
		StoreID:   scope.StoreID,
		Filters:   normalized,
		Metrics:   buildMetrics(history),
		Quality:   buildQuality(history),
		ChartData: buildChartData(history),
	}, nil
}

func (service *Service) Results(ctx context.Context, principal auth.Principal, filters Filters) (ResultsResponse, error) {
	if !canViewReports(principal) {
		return ResultsResponse{}, stores.ErrForbidden
	}

	scope, normalized, history, err := service.loadEntries(ctx, principal, filters)
	if err != nil {
		return ResultsResponse{}, err
	}

	rows := buildResultRows(history)
	total := len(rows)
	pageRows := paginateRows(rows, normalized.Page, normalized.PageSize)

	return ResultsResponse{
		StoreID:  scope.StoreID,
		Filters:  normalized,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Rows:     pageRows,
	}, nil
}

func (service *Service) RecentServices(ctx context.Context, principal auth.Principal, filters Filters) (RecentServicesResponse, error) {
	if !canViewReports(principal) {
		return RecentServicesResponse{}, stores.ErrForbidden
	}

	if filters.PageSize <= 0 {
		filters.PageSize = defaultRecentSize
	}

	scope, normalized, history, err := service.loadEntries(ctx, principal, filters)
	if err != nil {
		return RecentServicesResponse{}, err
	}

	rows := buildResultRows(history)
	total := len(rows)
	pageRows := paginateRows(rows, normalized.Page, normalized.PageSize)

	return RecentServicesResponse{
		StoreID:  scope.StoreID,
		Filters:  normalized,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Items:    pageRows,
	}, nil
}

func (service *Service) MultiStoreOverview(ctx context.Context, principal auth.Principal, filters Filters) (MultiStoreOverviewResponse, error) {
	if !canViewReports(principal) {
		return MultiStoreOverviewResponse{}, stores.ErrForbidden
	}

	normalized, repositoryInput, err := normalizeFilters(filters)
	if err != nil {
		return MultiStoreOverviewResponse{}, err
	}

	resolvedTenantID := firstNonEmpty(normalized.TenantID, principal.TenantID)
	if resolvedTenantID == "" {
		return MultiStoreOverviewResponse{}, ErrStoreRequired
	}

	normalized.TenantID = resolvedTenantID

	storeRows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{
		TenantID: resolvedTenantID,
	})
	if err != nil {
		return MultiStoreOverviewResponse{}, err
	}

	storeIDs := make([]string, 0, len(storeRows))
	for _, store := range storeRows {
		storeIDs = append(storeIDs, store.ID)
	}

	history, err := service.repository.ListHistoryByStores(ctx, storeIDs, repositoryInput)
	if err != nil {
		return MultiStoreOverviewResponse{}, err
	}

	liveCounts, err := service.repository.ListLiveCounts(ctx, storeIDs)
	if err != nil {
		return MultiStoreOverviewResponse{}, err
	}

	historyByStore := make(map[string][]operations.ServiceHistoryEntry, len(storeRows))
	for _, entry := range history {
		storeID := strings.TrimSpace(entry.StoreID)
		if storeID == "" {
			continue
		}

		historyByStore[storeID] = append(historyByStore[storeID], entry)
	}

	rows := make([]MultiStoreOverviewRow, 0, len(storeRows))
	summary := MultiStoreSummary{
		ActiveStores: len(storeRows),
	}

	for _, store := range storeRows {
		storeHistory := historyByStore[store.ID]
		metrics := buildMetrics(storeHistory)
		totalPieces := 0
		conversions := 0
		quickNoSaleCount := 0
		longNoSaleCount := 0
		quickCloseCount := 0
		longLowSaleCount := 0

		for _, entry := range storeHistory {
			if isSaleOutcome(entry.FinishOutcome) {
				conversions++
				totalPieces += len(entry.ProductsClosed)
			}

			switch {
			case strings.TrimSpace(entry.FinishOutcome) == "nao-compra" && maxInt64(entry.DurationMs, 0) <= 5*60000:
				quickNoSaleCount++
			case strings.TrimSpace(entry.FinishOutcome) == "nao-compra" && maxInt64(entry.DurationMs, 0) >= 25*60000:
				longNoSaleCount++
			case isSaleOutcome(entry.FinishOutcome) && maxInt64(entry.DurationMs, 0) <= 5*60000:
				quickCloseCount++
			case isSaleOutcome(entry.FinishOutcome) && maxInt64(entry.DurationMs, 0) >= 25*60000 && maxFloat(entry.SaleAmount, 0) <= 1200:
				longLowSaleCount++
			}
		}

		paScore := 0.0
		if conversions > 0 {
			piecesForPA := totalPieces
			if piecesForPA < conversions {
				piecesForPA = conversions
			}
			paScore = float64(piecesForPA) / float64(conversions)
		}

		live := liveCounts[store.ID]
		healthScore := buildMultiStoreHealthScore(metrics, live, quickNoSaleCount, longNoSaleCount, quickCloseCount, longLowSaleCount)

		row := MultiStoreOverviewRow{
			StoreID:            store.ID,
			StoreName:          store.Name,
			StoreCode:          store.Code,
			StoreCity:          store.City,
			Consultants:        live.Consultants,
			QueueCount:         live.QueueCount,
			ActiveCount:        live.ActiveCount,
			PausedCount:        live.PausedCount,
			Attendances:        metrics.TotalAttendances,
			ConversionRate:     metrics.ConversionRate,
			SoldValue:          metrics.SoldValue,
			TicketAverage:      metrics.AverageTicket,
			PAScore:            paScore,
			AverageQueueWaitMs: metrics.AverageQueueWaitMs,
			QueueJumpRate:      metrics.QueueJumpRate,
			HealthScore:        healthScore,
			MonthlyGoal:        store.MonthlyGoal,
			WeeklyGoal:         store.WeeklyGoal,
			AvgTicketGoal:      store.AvgTicketGoal,
			ConversionGoal:     store.ConversionGoal,
			PAGoal:             store.PAGoal,
			DefaultTemplateID:  store.DefaultTemplateID,
		}
		rows = append(rows, row)

		summary.TotalAttendances += row.Attendances
		summary.TotalSoldValue += row.SoldValue
		summary.TotalQueue += row.QueueCount
		summary.TotalActiveServices += row.ActiveCount
		summary.AverageHealthScore += row.HealthScore
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SoldValue == rows[j].SoldValue {
			return rows[i].ConversionRate > rows[j].ConversionRate
		}

		return rows[i].SoldValue > rows[j].SoldValue
	})

	if len(rows) > 0 {
		summary.AverageHealthScore = summary.AverageHealthScore / float64(len(rows))
	}

	return MultiStoreOverviewResponse{
		TenantID: resolvedTenantID,
		Filters:  normalized,
		Summary:  summary,
		Stores:   rows,
	}, nil
}

func canViewReports(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionReportsView) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionQueueReportsRead)
	}

	return principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner || principal.Role == auth.RoleStoreTerminal
}

func buildMultiStoreHealthScore(
	metrics Metrics,
	live StoreLiveCounts,
	quickNoSaleCount int,
	longNoSaleCount int,
	quickCloseCount int,
	longLowSaleCount int,
) float64 {
	critical := 0
	attention := 0

	if metrics.TotalAttendances < 6 {
		attention++
	}

	switch {
	case metrics.QueueJumpRate >= 25:
		critical++
	case metrics.QueueJumpRate >= 12:
		attention++
	}

	switch {
	case live.ActiveCount == 0 && live.QueueCount > 0:
		critical++
	case live.ActiveCount > 0 && float64(live.QueueCount)/float64(live.ActiveCount) >= 1.2 && live.QueueCount >= 2:
		critical++
	case live.ActiveCount > 0 && float64(live.QueueCount)/float64(live.ActiveCount) >= 0.7 && live.QueueCount >= 1:
		attention++
	}

	switch {
	case metrics.AverageQueueWaitMs >= 20*60000:
		critical++
	case metrics.AverageQueueWaitMs >= 10*60000:
		attention++
	}

	nonConversions := metrics.NonConversions
	if nonConversions > 0 {
		quickNoSaleRate := (float64(quickNoSaleCount) / float64(nonConversions)) * 100
		longNoSaleRate := (float64(longNoSaleCount) / float64(nonConversions)) * 100

		switch {
		case quickNoSaleRate >= 45:
			critical++
		case quickNoSaleRate >= 25:
			attention++
		}

		switch {
		case longNoSaleRate >= 35:
			critical++
		case longNoSaleRate >= 20:
			attention++
		}
	}

	if metrics.Conversions > 0 {
		quickCloseRate := (float64(quickCloseCount) / float64(metrics.Conversions)) * 100
		longLowSaleRate := (float64(longLowSaleCount) / float64(metrics.Conversions)) * 100

		switch {
		case longLowSaleRate >= 30:
			critical++
		case longLowSaleRate >= 18:
			attention++
		}

		if quickCloseRate >= 45 {
			attention++
		}
	}

	score := 100 - float64(critical*18) - float64(attention*8)
	if score < 0 {
		return 0
	}

	return score
}
