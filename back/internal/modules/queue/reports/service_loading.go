package reports

import (
	"context"
	"sort"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

func (service *Service) loadEntries(
	ctx context.Context,
	principal auth.Principal,
	filters Filters,
) (reportScope, Filters, []operations.ServiceHistoryEntry, HistoryWindow, error) {
	normalized, repositoryFilters, err := normalizeFilters(filters)
	if err != nil {
		return reportScope{}, Filters{}, nil, HistoryWindow{}, err
	}

	if normalized.StoreID == "" {
		resolvedTenantID := stringsx.FirstNonEmpty(normalized.TenantID, principal.TenantID)
		if resolvedTenantID == "" {
			return reportScope{}, Filters{}, nil, HistoryWindow{}, ErrStoreRequired
		}

		storeRows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{
			TenantID: resolvedTenantID,
		})
		if err != nil {
			return reportScope{}, Filters{}, nil, HistoryWindow{}, err
		}

		storeLookup := make(map[string]stores.StoreView, len(storeRows))
		storeIDs := make([]string, 0, len(storeRows))
		for _, store := range storeRows {
			storeLookup[store.ID] = store
			storeIDs = append(storeIDs, store.ID)
		}

		history, err := service.repository.ListHistoryByStores(ctx, storeIDs, repositoryFilters)
		if err != nil {
			return reportScope{}, Filters{}, nil, HistoryWindow{}, err
		}

		// A janela mede o resultado bruto do SQL, ANTES dos filtros em memoria.
		window, err := service.computeHistoryWindow(ctx, storeIDs, repositoryFilters, len(history))
		if err != nil {
			return reportScope{}, Filters{}, nil, HistoryWindow{}, err
		}

		for index := range history {
			if strings.TrimSpace(history[index].StoreName) != "" {
				continue
			}

			store, ok := storeLookup[strings.TrimSpace(history[index].StoreID)]
			if ok {
				history[index].StoreName = store.Name
			}
		}

		filtered := filterHistory(history, normalized)
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].FinishedAt == filtered[j].FinishedAt {
				return filtered[i].ServiceID > filtered[j].ServiceID
			}

			return filtered[i].FinishedAt > filtered[j].FinishedAt
		})

		normalized.TenantID = resolvedTenantID
		return reportScope{TenantID: resolvedTenantID}, normalized, filtered, window, nil
	}

	store, err := service.storeFinder.FindAccessible(ctx, principal, normalized.StoreID)
	if err != nil {
		return reportScope{}, Filters{}, nil, HistoryWindow{}, err
	}

	history, err := service.repository.ListHistory(ctx, store.ID, repositoryFilters)
	if err != nil {
		return reportScope{}, Filters{}, nil, HistoryWindow{}, err
	}

	window, err := service.computeHistoryWindow(ctx, []string{store.ID}, repositoryFilters, len(history))
	if err != nil {
		return reportScope{}, Filters{}, nil, HistoryWindow{}, err
	}

	for index := range history {
		if strings.TrimSpace(history[index].StoreName) == "" {
			history[index].StoreName = store.Name
		}
	}

	filtered := filterHistory(history, normalized)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].FinishedAt == filtered[j].FinishedAt {
			return filtered[i].ServiceID > filtered[j].ServiceID
		}

		return filtered[i].FinishedAt > filtered[j].FinishedAt
	})

	return reportScope{
		StoreID:   store.ID,
		TenantID:  store.TenantID,
		StoreName: store.Name,
	}, normalized, filtered, window, nil
}

func normalizeFilters(input Filters) (Filters, repositoryFilters, error) {
	normalized := Filters{
		TenantID:              strings.TrimSpace(input.TenantID),
		StoreID:               strings.TrimSpace(input.StoreID),
		DateFrom:              strings.TrimSpace(input.DateFrom),
		DateTo:                strings.TrimSpace(input.DateTo),
		ConsultantIDs:         normalizeList(input.ConsultantIDs),
		Outcomes:              normalizeAllowedList(input.Outcomes, map[string]struct{}{"compra": {}, "reserva": {}, "nao-compra": {}}),
		SourceIDs:             normalizeList(input.SourceIDs),
		VisitReasonIDs:        normalizeList(input.VisitReasonIDs),
		StartModes:            normalizeAllowedList(input.StartModes, map[string]struct{}{"queue": {}, "queue-jump": {}}),
		ExistingCustomerModes: normalizeAllowedList(input.ExistingCustomerModes, map[string]struct{}{"yes": {}, "no": {}}),
		CompletionLevels:      normalizeAllowedList(input.CompletionLevels, map[string]struct{}{"excellent": {}, "complete": {}, "incomplete": {}}),
		CampaignIDs:           normalizeList(input.CampaignIDs),
		Search:                strings.TrimSpace(input.Search),
		Page:                  input.Page,
		PageSize:              input.PageSize,
		Limit:                 input.Limit,
	}

	if normalized.Page <= 0 {
		normalized.Page = 1
	}

	if normalized.PageSize <= 0 {
		normalized.PageSize = defaultPageSize
	}

	if normalized.PageSize > maxPageSize {
		normalized.PageSize = maxPageSize
	}

	if normalized.Limit <= 0 {
		normalized.Limit = defaultHistoryFetchLimit
	}

	if normalized.Limit > maxHistoryFetchLimit {
		normalized.Limit = maxHistoryFetchLimit
	}

	var repositoryInput repositoryFilters

	if normalized.DateFrom != "" {
		startAt, err := dayStartMillis(normalized.DateFrom)
		if err != nil {
			return Filters{}, repositoryFilters{}, ErrValidation
		}

		repositoryInput.FinishedAtFrom = &startAt
	}

	if normalized.DateTo != "" {
		endAt, err := dayEndMillis(normalized.DateTo)
		if err != nil {
			return Filters{}, repositoryFilters{}, ErrValidation
		}

		repositoryInput.FinishedAtTo = &endAt
	}

	if repositoryInput.FinishedAtFrom != nil && repositoryInput.FinishedAtTo != nil && *repositoryInput.FinishedAtTo < *repositoryInput.FinishedAtFrom {
		return Filters{}, repositoryFilters{}, ErrValidation
	}

	repositoryInput.ConsultantIDs = normalized.ConsultantIDs
	repositoryInput.Outcomes = normalized.Outcomes
	repositoryInput.StartModes = normalized.StartModes
	repositoryInput.Limit = normalized.Limit

	if len(normalized.ExistingCustomerModes) == 1 {
		value := normalized.ExistingCustomerModes[0] == "yes"
		repositoryInput.IsExistingCustomer = &value
	}

	if input.MinSaleAmount != nil {
		value := maxFloat(*input.MinSaleAmount, 0)
		normalized.MinSaleAmount = &value
		repositoryInput.MinSaleAmount = &value
	}

	if input.MaxSaleAmount != nil {
		value := maxFloat(*input.MaxSaleAmount, 0)
		normalized.MaxSaleAmount = &value
		repositoryInput.MaxSaleAmount = &value
	}

	if repositoryInput.MinSaleAmount != nil && repositoryInput.MaxSaleAmount != nil && *repositoryInput.MaxSaleAmount < *repositoryInput.MinSaleAmount {
		return Filters{}, repositoryFilters{}, ErrValidation
	}

	return normalized, repositoryInput, nil
}

func filterHistory(entries []operations.ServiceHistoryEntry, filters Filters) []operations.ServiceHistoryEntry {
	filtered := make([]operations.ServiceHistoryEntry, 0, len(entries))
	query := comparableText(filters.Search)

	for _, entry := range entries {
		completion := evaluateCompletion(entry)

		if len(filters.SourceIDs) > 0 && !intersectsAny(entry.CustomerSources, filters.SourceIDs) {
			continue
		}

		if len(filters.VisitReasonIDs) > 0 && !intersectsAny(entry.VisitReasons, filters.VisitReasonIDs) {
			continue
		}

		if len(filters.CompletionLevels) > 0 && !containsValue(filters.CompletionLevels, completion.Level) {
			continue
		}

		if len(filters.CampaignIDs) > 0 && !entryHasCampaign(entry, filters.CampaignIDs) {
			continue
		}

		if query != "" && !matchesSearch(entry, query) {
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered
}
