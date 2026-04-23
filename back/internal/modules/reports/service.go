package reports

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
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

func NewService(repository Repository, storeFinder StoreFinder) *Service {
	return &Service{
		repository:  repository,
		storeFinder: storeFinder,
	}
}

func (service *Service) Overview(ctx context.Context, principal auth.Principal, filters Filters) (OverviewResponse, error) {
	store, normalized, history, err := service.loadEntries(ctx, principal, filters)
	if err != nil {
		return OverviewResponse{}, err
	}

	return OverviewResponse{
		StoreID:   store.ID,
		Filters:   normalized,
		Metrics:   buildMetrics(history),
		Quality:   buildQuality(history),
		ChartData: buildChartData(history),
	}, nil
}

func (service *Service) Results(ctx context.Context, principal auth.Principal, filters Filters) (ResultsResponse, error) {
	store, normalized, history, err := service.loadEntries(ctx, principal, filters)
	if err != nil {
		return ResultsResponse{}, err
	}

	rows := buildResultRows(history)
	total := len(rows)
	pageRows := paginateRows(rows, normalized.Page, normalized.PageSize)

	return ResultsResponse{
		StoreID:  store.ID,
		Filters:  normalized,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Rows:     pageRows,
	}, nil
}

func (service *Service) RecentServices(ctx context.Context, principal auth.Principal, filters Filters) (RecentServicesResponse, error) {
	if filters.PageSize <= 0 {
		filters.PageSize = defaultRecentSize
	}

	store, normalized, history, err := service.loadEntries(ctx, principal, filters)
	if err != nil {
		return RecentServicesResponse{}, err
	}

	rows := buildResultRows(history)
	total := len(rows)
	pageRows := paginateRows(rows, normalized.Page, normalized.PageSize)

	return RecentServicesResponse{
		StoreID:  store.ID,
		Filters:  normalized,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Items:    pageRows,
	}, nil
}

func (service *Service) MultiStoreOverview(ctx context.Context, principal auth.Principal, filters Filters) (MultiStoreOverviewResponse, error) {
	normalized, repositoryInput, err := normalizeFilters(filters)
	if err != nil {
		return MultiStoreOverviewResponse{}, err
	}

	storeRows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{
		TenantID: normalized.TenantID,
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
		quickNoSaleCount := 0
		longNoSaleCount := 0
		quickCloseCount := 0
		longLowSaleCount := 0

		for _, entry := range storeHistory {
			totalPieces += len(entry.ProductsClosed)

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
		if len(storeHistory) > 0 {
			paScore = float64(totalPieces) / float64(len(storeHistory))
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
		TenantID: firstNonEmpty(normalized.TenantID, principal.TenantID),
		Filters:  normalized,
		Summary:  summary,
		Stores:   rows,
	}, nil
}

func (service *Service) loadEntries(
	ctx context.Context,
	principal auth.Principal,
	filters Filters,
) (stores.StoreView, Filters, []operations.ServiceHistoryEntry, error) {
	normalized, repositoryFilters, err := normalizeFilters(filters)
	if err != nil {
		return stores.StoreView{}, Filters{}, nil, err
	}

	if normalized.StoreID == "" {
		return stores.StoreView{}, Filters{}, nil, ErrStoreRequired
	}

	store, err := service.storeFinder.FindAccessible(ctx, principal, normalized.StoreID)
	if err != nil {
		return stores.StoreView{}, Filters{}, nil, err
	}

	history, err := service.repository.ListHistory(ctx, store.ID, repositoryFilters)
	if err != nil {
		return stores.StoreView{}, Filters{}, nil, err
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

	return store, normalized, filtered, nil
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

func buildMetrics(entries []operations.ServiceHistoryEntry) Metrics {
	conversions := 0
	soldValue := 0.0
	totalDuration := int64(0)
	totalQueueWait := int64(0)
	queueJumpCount := 0
	campaignBonusTotal := 0.0

	for _, entry := range entries {
		if isSaleOutcome(entry.FinishOutcome) {
			conversions++
			soldValue += maxFloat(entry.SaleAmount, 0)
		}

		totalDuration += maxInt64(entry.DurationMs, 0)
		totalQueueWait += maxInt64(entry.QueueWaitMs, 0)
		campaignBonusTotal += maxFloat(entry.CampaignBonusTotal, 0)

		if strings.TrimSpace(entry.StartMode) == "queue-jump" {
			queueJumpCount++
		}
	}

	totalAttendances := len(entries)
	nonConversions := totalAttendances - conversions
	averageTicket := 0.0
	if conversions > 0 {
		averageTicket = soldValue / float64(conversions)
	}

	averageDuration := 0.0
	averageQueueWait := 0.0
	conversionRate := 0.0
	queueJumpRate := 0.0

	if totalAttendances > 0 {
		averageDuration = float64(totalDuration) / float64(totalAttendances)
		averageQueueWait = float64(totalQueueWait) / float64(totalAttendances)
		conversionRate = (float64(conversions) / float64(totalAttendances)) * 100
		queueJumpRate = (float64(queueJumpCount) / float64(totalAttendances)) * 100
	}

	return Metrics{
		TotalAttendances:   totalAttendances,
		Conversions:        conversions,
		NonConversions:     nonConversions,
		ConversionRate:     conversionRate,
		SoldValue:          soldValue,
		AverageTicket:      averageTicket,
		AverageDurationMs:  averageDuration,
		AverageQueueWaitMs: averageQueueWait,
		QueueJumpRate:      queueJumpRate,
		CampaignBonusTotal: campaignBonusTotal,
	}
}

func buildQuality(entries []operations.ServiceHistoryEntry) QualityOverview {
	type consultantBucket struct {
		ConsultantID     string
		ConsultantName   string
		TotalAttendances int
		CompleteCount    int
		ExcellentCount   int
		IncompleteCount  int
		NotesCount       int
	}

	consultants := map[string]*consultantBucket{}
	completeCount := 0
	excellentCount := 0
	incompleteCount := 0
	notesCount := 0

	for _, entry := range entries {
		completion := evaluateCompletion(entry)

		switch completion.Level {
		case "excellent":
			excellentCount++
			completeCount++
		case "complete":
			completeCount++
		default:
			incompleteCount++
		}

		if completion.HasNotes {
			notesCount++
		}

		key := strings.TrimSpace(entry.PersonID)
		if key == "" {
			key = strings.TrimSpace(entry.PersonName)
		}

		bucket, ok := consultants[key]
		if !ok {
			bucket = &consultantBucket{
				ConsultantID:   strings.TrimSpace(entry.PersonID),
				ConsultantName: strings.TrimSpace(entry.PersonName),
			}
			consultants[key] = bucket
		}

		bucket.TotalAttendances++
		if completion.Level == "excellent" {
			bucket.ExcellentCount++
			bucket.CompleteCount++
		} else if completion.Level == "complete" {
			bucket.CompleteCount++
		} else {
			bucket.IncompleteCount++
		}

		if completion.HasNotes {
			bucket.NotesCount++
		}
	}

	rows := make([]ConsultantQualityRow, 0, len(consultants))
	for _, bucket := range consultants {
		completeRate := 0.0
		excellentRate := 0.0
		incompleteRate := 0.0
		notesRate := 0.0
		if bucket.TotalAttendances > 0 {
			total := float64(bucket.TotalAttendances)
			completeRate = (float64(bucket.CompleteCount) / total) * 100
			excellentRate = (float64(bucket.ExcellentCount) / total) * 100
			incompleteRate = (float64(bucket.IncompleteCount) / total) * 100
			notesRate = (float64(bucket.NotesCount) / total) * 100
		}

		levelKey, levelLabel := resolveConsultantQualityLevel(completeRate, excellentRate)
		rows = append(rows, ConsultantQualityRow{
			ConsultantID:      bucket.ConsultantID,
			ConsultantName:    bucket.ConsultantName,
			TotalAttendances:  bucket.TotalAttendances,
			CompleteCount:     bucket.CompleteCount,
			ExcellentCount:    bucket.ExcellentCount,
			IncompleteCount:   bucket.IncompleteCount,
			NotesCount:        bucket.NotesCount,
			CompleteRate:      completeRate,
			ExcellentRate:     excellentRate,
			IncompleteRate:    incompleteRate,
			NotesRate:         notesRate,
			QualityLevelKey:   levelKey,
			QualityLevelLabel: levelLabel,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ExcellentRate == rows[j].ExcellentRate {
			if rows[i].CompleteRate == rows[j].CompleteRate {
				return rows[i].TotalAttendances > rows[j].TotalAttendances
			}

			return rows[i].CompleteRate > rows[j].CompleteRate
		}

		return rows[i].ExcellentRate > rows[j].ExcellentRate
	})

	totalAttendances := len(entries)
	completeRate := 0.0
	excellentRate := 0.0
	incompleteRate := 0.0
	notesRate := 0.0

	if totalAttendances > 0 {
		total := float64(totalAttendances)
		completeRate = (float64(completeCount) / total) * 100
		excellentRate = (float64(excellentCount) / total) * 100
		incompleteRate = (float64(incompleteCount) / total) * 100
		notesRate = (float64(notesCount) / total) * 100
	}

	return QualityOverview{
		CompleteCount:   completeCount,
		ExcellentCount:  excellentCount,
		IncompleteCount: incompleteCount,
		NotesCount:      notesCount,
		CompleteRate:    completeRate,
		ExcellentRate:   excellentRate,
		IncompleteRate:  incompleteRate,
		NotesRate:       notesRate,
		ByConsultant:    rows,
	}
}

func buildChartData(entries []operations.ServiceHistoryEntry) ChartData {
	hourlyMap := map[string]HourlyDataPoint{}
	consultantMap := map[string]ConsultantAggRow{}
	visitReasons := map[string]*CountRow{}
	customerSources := map[string]*CountRow{}
	productsClosed := map[string]*CountRow{}
	outcomes := OutcomeCounts{}

	for _, entry := range entries {
		switch strings.TrimSpace(entry.FinishOutcome) {
		case "compra":
			outcomes.Compra++
		case "reserva":
			outcomes.Reserva++
		default:
			outcomes.NaoCompra++
		}

		hour := time.UnixMilli(entry.FinishedAt).UTC().Format("15")
		hourBucket := hourlyMap[hour]
		hourBucket.Hour = hour
		hourBucket.Attendances++
		if isSaleOutcome(entry.FinishOutcome) {
			hourBucket.Conversions++
			hourBucket.SaleAmount += maxFloat(entry.SaleAmount, 0)
		}
		hourlyMap[hour] = hourBucket

		consultantID := strings.TrimSpace(entry.PersonID)
		consultantBucket := consultantMap[consultantID]
		consultantBucket.ConsultantID = consultantID
		consultantBucket.ConsultantName = strings.TrimSpace(entry.PersonName)
		consultantBucket.Attendances++
		if isSaleOutcome(entry.FinishOutcome) {
			consultantBucket.Conversions++
			consultantBucket.SaleAmount += maxFloat(entry.SaleAmount, 0)
		}
		consultantMap[consultantID] = consultantBucket

		for _, value := range entry.VisitReasons {
			incrementCountMap(visitReasons, value)
		}

		for _, value := range entry.CustomerSources {
			incrementCountMap(customerSources, value)
		}

		for _, label := range closedProductLabels(entry) {
			incrementCountMap(productsClosed, label)
		}
	}

	hourlyRows := make([]HourlyDataPoint, 0, len(hourlyMap))
	for _, item := range hourlyMap {
		hourlyRows = append(hourlyRows, item)
	}
	sort.SliceStable(hourlyRows, func(i, j int) bool {
		return hourlyRows[i].Hour < hourlyRows[j].Hour
	})

	consultantRows := make([]ConsultantAggRow, 0, len(consultantMap))
	for _, item := range consultantMap {
		consultantRows = append(consultantRows, item)
	}
	sort.SliceStable(consultantRows, func(i, j int) bool {
		if consultantRows[i].SaleAmount == consultantRows[j].SaleAmount {
			return consultantRows[i].ConsultantName < consultantRows[j].ConsultantName
		}

		return consultantRows[i].SaleAmount > consultantRows[j].SaleAmount
	})

	return ChartData{
		OutcomeCounts:      outcomes,
		HourlyData:         hourlyRows,
		ConsultantAgg:      consultantRows,
		TopProductsClosed:  topCountRows(productsClosed, 8),
		TopVisitReasons:    topCountRows(visitReasons, 8),
		TopCustomerSources: topCountRows(customerSources, 8),
	}
}

func buildResultRows(entries []operations.ServiceHistoryEntry) []ResultRow {
	rows := make([]ResultRow, 0, len(entries))
	for _, entry := range entries {
		completion := evaluateCompletion(entry)
		rows = append(rows, ResultRow{
			ServiceID:          strings.TrimSpace(entry.ServiceID),
			StoreID:            strings.TrimSpace(entry.StoreID),
			StoreName:          strings.TrimSpace(entry.StoreName),
			ConsultantID:       strings.TrimSpace(entry.PersonID),
			ConsultantName:     strings.TrimSpace(entry.PersonName),
			StartedAt:          entry.StartedAt,
			FinishedAt:         entry.FinishedAt,
			DurationMs:         maxInt64(entry.DurationMs, 0),
			QueueWaitMs:        maxInt64(entry.QueueWaitMs, 0),
			Outcome:            strings.TrimSpace(entry.FinishOutcome),
			StartMode:          strings.TrimSpace(entry.StartMode),
			SaleAmount:         maxFloat(entry.SaleAmount, 0),
			IsWindowService:    entry.IsWindowService,
			IsGift:             entry.IsGift,
			IsExistingCustomer: entry.IsExistingCustomer,
			CustomerName:       strings.TrimSpace(entry.CustomerName),
			CustomerPhone:      strings.TrimSpace(entry.CustomerPhone),
			CustomerEmail:      strings.TrimSpace(entry.CustomerEmail),
			CustomerProfession: strings.TrimSpace(entry.CustomerProfession),
			ProductSeen:        strings.TrimSpace(entry.ProductSeen),
			ProductClosed:      primaryClosedProductLabel(entry),
			ProductDetails:     strings.TrimSpace(entry.ProductDetails),
			VisitReasons:       cloneStringSlice(entry.VisitReasons),
			CustomerSources:    cloneStringSlice(entry.CustomerSources),
			CampaignNames:      cloneCampaignNames(entry.CampaignMatches),
			QueueJumpReason:    strings.TrimSpace(entry.QueueJumpReason),
			Notes:              strings.TrimSpace(entry.Notes),
			HasNotes:           completion.HasNotes,
			CompletionLevel:    completion.Level,
			CompletionRate:     completion.CoreFillRate,
			CampaignBonusTotal: maxFloat(entry.CampaignBonusTotal, 0),
		})
	}

	return rows
}

func paginateRows(rows []ResultRow, page int, pageSize int) []ResultRow {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []ResultRow{}
	}

	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}

	return rows[start:end]
}

type completionInfo struct {
	CoreFillRate float64
	HasNotes     bool
	Level        string
}

func evaluateCompletion(entry operations.ServiceHistoryEntry) completionInfo {
	checks := []bool{
		hasText(entry.CustomerName),
		hasText(entry.CustomerPhone),
		hasText(entry.ProductClosed) ||
			hasText(entry.ProductSeen) ||
			hasText(entry.ProductDetails) ||
			len(entry.ProductsSeen) > 0 ||
			entry.ProductsSeenNone,
		len(entry.VisitReasons) > 0 || entry.VisitReasonsNotInformed,
		len(entry.CustomerSources) > 0 || entry.CustomerSourcesNotInformed,
	}

	filled := 0
	for _, item := range checks {
		if item {
			filled++
		}
	}

	coreTotal := len(checks)
	coreFillRate := 0.0
	if coreTotal > 0 {
		coreFillRate = float64(filled) / float64(coreTotal)
	}

	hasNotes := hasText(entry.Notes)
	level := "incomplete"
	if filled == coreTotal {
		if hasNotes {
			level = "excellent"
		} else {
			level = "complete"
		}
	}

	return completionInfo{
		CoreFillRate: coreFillRate,
		HasNotes:     hasNotes,
		Level:        level,
	}
}

func resolveConsultantQualityLevel(completeRate float64, excellentRate float64) (string, string) {
	if completeRate >= 85 && excellentRate >= 35 {
		return "highlight", "Destaque"
	}
	if completeRate >= 70 {
		return "consistent", "Consistente"
	}
	return "attention", "Precisa melhorar"
}

func entryHasCampaign(entry operations.ServiceHistoryEntry, campaignIDs []string) bool {
	for _, match := range entry.CampaignMatches {
		if containsValue(campaignIDs, strings.TrimSpace(match.ID)) {
			return true
		}
	}
	return false
}

func matchesSearch(entry operations.ServiceHistoryEntry, query string) bool {
	searchable := []string{
		entry.StoreName,
		entry.ServiceID,
		entry.PersonName,
		entry.CustomerName,
		entry.CustomerPhone,
		entry.CustomerEmail,
		entry.CustomerProfession,
		entry.ProductSeen,
		entry.ProductClosed,
		entry.ProductDetails,
		entry.Notes,
		entry.QueueJumpReason,
	}

	for _, product := range entry.ProductsSeen {
		searchable = append(searchable, product.Name, product.Code)
	}
	for _, product := range entry.ProductsClosed {
		searchable = append(searchable, product.Name, product.Code)
	}
	searchable = append(searchable, entry.VisitReasons...)
	searchable = append(searchable, entry.CustomerSources...)

	for _, value := range searchable {
		if strings.Contains(comparableText(value), query) {
			return true
		}
	}

	return false
}

func closedProductLabels(entry operations.ServiceHistoryEntry) []string {
	labels := make([]string, 0, len(entry.ProductsClosed)+1)
	for _, product := range entry.ProductsClosed {
		label := firstNonEmpty(strings.TrimSpace(product.Name), strings.TrimSpace(product.Code))
		if label != "" {
			labels = append(labels, label)
		}
	}

	if len(labels) > 0 {
		return labels
	}

	fallback := strings.TrimSpace(entry.ProductClosed)
	if fallback != "" {
		return []string{fallback}
	}

	return []string{}
}

func primaryClosedProductLabel(entry operations.ServiceHistoryEntry) string {
	labels := closedProductLabels(entry)
	if len(labels) > 0 {
		return labels[0]
	}

	return strings.TrimSpace(entry.ProductClosed)
}

func incrementCountMap(counter map[string]*CountRow, label string) {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return
	}

	key := strings.ToLower(trimmed)
	row, ok := counter[key]
	if !ok {
		row = &CountRow{Label: trimmed}
		counter[key] = row
	}

	row.Count++
}

func topCountRows(counter map[string]*CountRow, limit int) []CountRow {
	rows := make([]CountRow, 0, len(counter))
	for _, item := range counter {
		rows = append(rows, *item)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count == rows[j].Count {
			return rows[i].Label < rows[j].Label
		}

		return rows[i].Count > rows[j].Count
	})

	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}

	return rows
}

func normalizeList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, rawValue := range values {
		for _, part := range strings.Split(rawValue, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}

			if _, ok := seen[value]; ok {
				continue
			}

			seen[value] = struct{}{}
			result = append(result, value)
		}
	}

	return result
}

func normalizeAllowedList(values []string, allowed map[string]struct{}) []string {
	normalized := normalizeList(values)
	result := make([]string, 0, len(normalized))
	for _, value := range normalized {
		if _, ok := allowed[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func dayStartMillis(dateValue string) (int64, error) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.UTC
	}

	value, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateValue), location)
	if err != nil {
		return 0, err
	}

	return value.UnixMilli(), nil
}

func dayEndMillis(dateValue string) (int64, error) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.UTC
	}

	value, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateValue), location)
	if err != nil {
		return 0, err
	}

	return value.Add((24 * time.Hour) - time.Millisecond).UnixMilli(), nil
}

func intersectsAny(values []string, required []string) bool {
	if len(required) == 0 {
		return true
	}

	for _, value := range values {
		if containsValue(required, strings.TrimSpace(value)) {
			return true
		}
	}

	return false
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(target) {
			return true
		}
	}

	return false
}

func comparableText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

func isSaleOutcome(outcome string) bool {
	switch strings.TrimSpace(outcome) {
	case "compra", "reserva":
		return true
	default:
		return false
	}
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

func cloneStringSlice(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cloned = append(cloned, trimmed)
		}
	}
	return cloned
}

func cloneCampaignNames(matches []operations.CampaignMatch) []string {
	cloned := make([]string, 0, len(matches))
	for _, match := range matches {
		label := firstNonEmpty(match.Name, match.ID)
		if label != "" {
			cloned = append(cloned, label)
		}
	}

	return cloned
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
