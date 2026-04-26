package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

var analyticsLocation = func() *time.Location {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}

	return location
}()

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

func buildRankingRowsAcrossBundles(bundles []bundle, scope string) []RankingRow {
	rows := make([]RankingRow, 0)
	for _, item := range bundles {
		storeRows := buildRankingRows(item.history, item.roster, scope)
		for index := range storeRows {
			storeRows[index].StoreID = item.storeID
			storeRows[index].StoreName = item.storeView.Name
		}

		rows = append(rows, storeRows...)
	}

	sortRankingRows(rows)
	return rows
}

func buildConsultantAlertsAcrossBundles(bundles []bundle) []ConsultantAlert {
	alerts := make([]ConsultantAlert, 0)
	for _, item := range bundles {
		alerts = append(alerts, buildConsultantAlerts(item.history, item.roster, item.settings)...)
	}

	return alerts
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

func buildCombinedTimeIntelligence(bundles []bundle) TimeIntelligence {
	combined := TimeIntelligence{}
	totalAttendances := 0
	totalQueueWaitMs := 0.0
	totalQueueJumpAttendances := 0.0

	for _, item := range bundles {
		timeIntelligence := buildTimeIntelligence(item)
		attendanceCount := len(item.history)

		combined.QuickHighPotentialCount += timeIntelligence.QuickHighPotentialCount
		combined.LongLowSaleCount += timeIntelligence.LongLowSaleCount
		combined.LongNoSaleCount += timeIntelligence.LongNoSaleCount
		combined.QuickNoSaleCount += timeIntelligence.QuickNoSaleCount
		combined.TotalsByStatus.Available += timeIntelligence.TotalsByStatus.Available
		combined.TotalsByStatus.Queue += timeIntelligence.TotalsByStatus.Queue
		combined.TotalsByStatus.Service += timeIntelligence.TotalsByStatus.Service
		combined.TotalsByStatus.Paused += timeIntelligence.TotalsByStatus.Paused
		combined.ConsultantsInQueueMs += timeIntelligence.ConsultantsInQueueMs
		combined.ConsultantsPausedMs += timeIntelligence.ConsultantsPausedMs
		combined.ConsultantsInServiceMs += timeIntelligence.ConsultantsInServiceMs

		totalAttendances += attendanceCount
		totalQueueWaitMs += timeIntelligence.AvgQueueWaitMs * float64(attendanceCount)
		totalQueueJumpAttendances += (timeIntelligence.NotUsingQueueRate / 100) * float64(attendanceCount)
	}

	if totalAttendances > 0 {
		combined.AvgQueueWaitMs = totalQueueWaitMs / float64(totalAttendances)
		combined.NotUsingQueueRate = (totalQueueJumpAttendances / float64(totalAttendances)) * 100
	}

	return combined
}

func buildRankingRows(history []operations.ServiceHistoryEntry, roster []operations.ConsultantProfile, scope string) []RankingRow {
	now := time.Now().In(analyticsLocation)
	currentMonth := monthStamp(now)
	currentDay := dayStamp(now)

	rows := make([]RankingRow, 0, len(roster))
	for _, consultant := range roster {
		entries := make([]operations.ServiceHistoryEntry, 0)
		for _, entry := range history {
			if strings.TrimSpace(entry.PersonID) != strings.TrimSpace(consultant.ID) {
				continue
			}

			finishedAt := time.UnixMilli(entry.FinishedAt).In(analyticsLocation)
			if scope == "today" {
				if dayStamp(finishedAt) != currentDay {
					continue
				}
			} else if monthStamp(finishedAt) != currentMonth {
				continue
			}

			entries = append(entries, entry)
		}

		converted := make([]operations.ServiceHistoryEntry, 0)
		soldValue := 0.0
		totalPieces := 0
		completeEntries := 0
		totalDuration := int64(0)
		queueJumpCount := 0
		nonClientConversions := 0

		for _, entry := range entries {
			if isSaleOutcome(entry.FinishOutcome) {
				converted = append(converted, entry)
				soldValue += maxFloat(entry.SaleAmount, 0)
				if !entry.IsExistingCustomer {
					nonClientConversions++
				}
			}

			totalPieces += len(entry.ProductsClosed)
			if isCompleteEntry(entry) {
				completeEntries++
			}

			totalDuration += maxInt64(entry.DurationMs, 0)
			if strings.TrimSpace(entry.StartMode) == "queue-jump" {
				queueJumpCount++
			}
		}

		attendances := len(entries)
		conversions := len(converted)
		ticketAverage := 0.0
		paScore := 0.0
		qualityScore := 0.0
		avgDurationMs := 0.0
		conversionRate := 0.0
		queueJumpRate := 0.0

		if conversions > 0 {
			ticketAverage = soldValue / float64(conversions)
		}
		if attendances > 0 {
			paScore = float64(totalPieces) / float64(attendances)
			qualityScore = (float64(completeEntries) / float64(attendances)) * 100
			avgDurationMs = float64(totalDuration) / float64(attendances)
			conversionRate = (float64(conversions) / float64(attendances)) * 100
			queueJumpRate = (float64(queueJumpCount) / float64(attendances)) * 100
		}

		rows = append(rows, RankingRow{
			ConsultantID:         consultant.ID,
			ConsultantName:       consultant.Name,
			SoldValue:            soldValue,
			Attendances:          attendances,
			Conversions:          conversions,
			NonConversions:       attendances - conversions,
			ConversionRate:       conversionRate,
			TicketAverage:        ticketAverage,
			PAScore:              paScore,
			QualityScore:         qualityScore,
			AvgDurationMs:        avgDurationMs,
			NonClientConversions: nonClientConversions,
			QueueJumpServices:    queueJumpCount,
			QueueJumpRate:        queueJumpRate,
		})
	}

	sortRankingRows(rows)

	return rows
}

func sortRankingRows(rows []RankingRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SoldValue == rows[j].SoldValue {
			if rows[i].Conversions == rows[j].Conversions {
				return rows[i].ConversionRate > rows[j].ConversionRate
			}

			return rows[i].Conversions > rows[j].Conversions
		}

		return rows[i].SoldValue > rows[j].SoldValue
	})
}

func buildConsultantAlerts(history []operations.ServiceHistoryEntry, roster []operations.ConsultantProfile, settings StoreSettings) []ConsultantAlert {
	alerts := make([]ConsultantAlert, 0)
	now := time.Now().In(analyticsLocation)
	currentMonth := monthStamp(now)

	for _, consultant := range roster {
		entries := make([]operations.ServiceHistoryEntry, 0)
		for _, entry := range history {
			if strings.TrimSpace(entry.PersonID) != strings.TrimSpace(consultant.ID) {
				continue
			}

			if monthStamp(time.UnixMilli(entry.FinishedAt).In(analyticsLocation)) != currentMonth {
				continue
			}

			entries = append(entries, entry)
		}

		if len(entries) == 0 {
			continue
		}

		conversions := 0
		queueJumpCount := 0
		soldValue := 0.0
		totalPieces := 0
		for _, entry := range entries {
			if isSaleOutcome(entry.FinishOutcome) {
				conversions++
				soldValue += maxFloat(entry.SaleAmount, 0)
			}

			if strings.TrimSpace(entry.StartMode) == "queue-jump" {
				queueJumpCount++
			}

			totalPieces += len(entry.ProductsClosed)
		}

		conversionRate := (float64(conversions) / float64(len(entries))) * 100
		queueJumpRate := (float64(queueJumpCount) / float64(len(entries))) * 100
		ticketAverage := 0.0
		if conversions > 0 {
			ticketAverage = soldValue / float64(conversions)
		}
		paScore := float64(totalPieces) / float64(len(entries))

		if settings.AlertMinConversionRate > 0 && conversionRate < settings.AlertMinConversionRate {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "conversion", Value: conversionRate, Threshold: settings.AlertMinConversionRate})
		}
		if settings.AlertMaxQueueJumpRate > 0 && queueJumpRate > settings.AlertMaxQueueJumpRate {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "queueJump", Value: queueJumpRate, Threshold: settings.AlertMaxQueueJumpRate})
		}
		if settings.AlertMinPAScore > 0 && paScore < settings.AlertMinPAScore {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "pa", Value: paScore, Threshold: settings.AlertMinPAScore})
		}
		if settings.AlertMinTicketAverage > 0 && ticketAverage < settings.AlertMinTicketAverage {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "ticket", Value: ticketAverage, Threshold: settings.AlertMinTicketAverage})
		}
	}

	return alerts
}

func buildSoldProducts(history []operations.ServiceHistoryEntry) []CountRow {
	labels := make([]string, 0)
	for _, entry := range history {
		if !isSaleOutcome(entry.FinishOutcome) {
			continue
		}

		labels = append(labels, closedProductLabels(entry)...)
	}

	return groupLabels(labels, 8)
}

func buildRequestedProducts(history []operations.ServiceHistoryEntry) []CountRow {
	labels := make([]string, 0)
	for _, entry := range history {
		if len(entry.ProductsSeen) > 0 {
			for _, product := range entry.ProductsSeen {
				label := firstNonEmpty(product.Name, product.Code)
				if label != "" {
					labels = append(labels, label)
				}
			}
			continue
		}

		label := firstNonEmpty(entry.ProductSeen, entry.ProductDetails)
		if label != "" {
			labels = append(labels, label)
		}
	}

	return groupLabels(labels, 8)
}

func buildVisitReasons(history []operations.ServiceHistoryEntry, labels map[string]string) []CountRow {
	values := make([]string, 0)
	for _, entry := range history {
		for _, value := range entry.VisitReasons {
			values = append(values, firstNonEmpty(labels[strings.TrimSpace(value)], value))
		}
	}

	return groupLabels(values, 8)
}

func buildCustomerSources(history []operations.ServiceHistoryEntry, labels map[string]string) []CountRow {
	values := make([]string, 0)
	for _, entry := range history {
		for _, value := range entry.CustomerSources {
			values = append(values, firstNonEmpty(labels[strings.TrimSpace(value)], value))
		}
	}

	return groupLabels(values, 8)
}

func buildProfessions(history []operations.ServiceHistoryEntry) []CountRow {
	values := make([]string, 0)
	for _, entry := range history {
		values = append(values, strings.TrimSpace(entry.CustomerProfession))
	}

	return groupLabels(values, 6)
}

func buildOutcomeSummary(history []operations.ServiceHistoryEntry) []CountRow {
	values := make([]string, 0, len(history))
	for _, entry := range history {
		switch strings.TrimSpace(entry.FinishOutcome) {
		case "compra":
			values = append(values, "Compra")
		case "reserva":
			values = append(values, "Reserva")
		default:
			values = append(values, "Nao compra")
		}
	}

	return groupLabels(values, 0)
}

func buildHourlySales(history []operations.ServiceHistoryEntry) []HourlySalesRow {
	counter := map[string]*HourlySalesRow{}
	for _, entry := range history {
		if !isSaleOutcome(entry.FinishOutcome) {
			continue
		}

		hour := time.UnixMilli(entry.FinishedAt).In(analyticsLocation).Format("15")
		key := hour + "h"
		row, ok := counter[key]
		if !ok {
			row = &HourlySalesRow{Label: key}
			counter[key] = row
		}

		row.Count++
		row.Value += maxFloat(entry.SaleAmount, 0)
	}

	rows := make([]HourlySalesRow, 0, len(counter))
	for _, row := range counter {
		rows = append(rows, *row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Value == rows[j].Value {
			return rows[i].Label < rows[j].Label
		}
		return rows[i].Value > rows[j].Value
	})

	if len(rows) > 8 {
		return rows[:8]
	}

	return rows
}

func buildTimeIntelligence(data bundle) TimeIntelligence {
	now := time.Now().UnixMilli()
	fastThresholdMs := int64(maxInt(data.settings.TimingFastCloseMinutes, 5)) * 60000
	longThresholdMs := int64(maxInt(data.settings.TimingLongServiceMinutes, 25)) * 60000
	lowSaleThreshold := maxFloat(data.settings.TimingLowSaleAmount, 1200)

	quickHighPotentialCount := 0
	longLowSaleCount := 0
	longNoSaleCount := 0
	quickNoSaleCount := 0
	queueJumpCount := 0
	totalQueueWait := int64(0)

	for _, entry := range data.history {
		if strings.TrimSpace(entry.StartMode) == "queue-jump" {
			queueJumpCount++
		}
		totalQueueWait += maxInt64(entry.QueueWaitMs, 0)

		switch {
		case isSaleOutcome(entry.FinishOutcome) && maxInt64(entry.DurationMs, 0) <= fastThresholdMs:
			quickHighPotentialCount++
		case isSaleOutcome(entry.FinishOutcome) && maxInt64(entry.DurationMs, 0) >= longThresholdMs && maxFloat(entry.SaleAmount, 0) <= lowSaleThreshold:
			longLowSaleCount++
		case strings.TrimSpace(entry.FinishOutcome) == "nao-compra" && maxInt64(entry.DurationMs, 0) >= longThresholdMs:
			longNoSaleCount++
		case strings.TrimSpace(entry.FinishOutcome) == "nao-compra" && maxInt64(entry.DurationMs, 0) <= fastThresholdMs:
			quickNoSaleCount++
		}
	}

	avgQueueWaitMs := 0.0
	if len(data.history) > 0 {
		avgQueueWaitMs = float64(totalQueueWait) / float64(len(data.history))
	}

	totals := StatusTotals{}
	for _, session := range data.snapshot.ConsultantActivitySessions {
		addStatusDuration(&totals, session.Status, session.DurationMs)
	}

	for _, consultant := range data.roster {
		status, startedAt := resolveLiveStatusSnapshot(data.snapshot, consultant.ID, now)
		addStatusDuration(&totals, status, maxInt64(now-startedAt, 0))
	}

	consultantsInQueueMs := int64(0)
	for _, item := range data.snapshot.WaitingList {
		consultantsInQueueMs += maxInt64(now-item.QueueJoinedAt, 0)
	}

	consultantsPausedMs := int64(0)
	for _, item := range data.snapshot.PausedEmployees {
		consultantsPausedMs += maxInt64(now-item.StartedAt, 0)
	}

	consultantsInServiceMs := int64(0)
	for _, item := range data.snapshot.ActiveServices {
		consultantsInServiceMs += maxInt64(now-item.ServiceStartedAt, 0)
	}

	notUsingQueueRate := 0.0
	if len(data.history) > 0 {
		notUsingQueueRate = (float64(queueJumpCount) / float64(len(data.history))) * 100
	}

	return TimeIntelligence{
		QuickHighPotentialCount: quickHighPotentialCount,
		LongLowSaleCount:        longLowSaleCount,
		LongNoSaleCount:         longNoSaleCount,
		QuickNoSaleCount:        quickNoSaleCount,
		AvgQueueWaitMs:          avgQueueWaitMs,
		TotalsByStatus:          totals,
		ConsultantsInQueueMs:    consultantsInQueueMs,
		ConsultantsPausedMs:     consultantsPausedMs,
		ConsultantsInServiceMs:  consultantsInServiceMs,
		NotUsingQueueRate:       notUsingQueueRate,
	}
}

func buildOperationalIntelligence(data bundle) IntelligenceResponse {
	return buildOperationalIntelligenceSummary(data.storeID, data.tenantID, data.history, buildTimeIntelligence(data))
}

func buildOperationalIntelligenceSummary(storeID string, tenantID string, history []operations.ServiceHistoryEntry, timeIntelligence TimeIntelligence) IntelligenceResponse {
	totalAttendances := len(history)
	conversions := 0
	soldValue := 0.0
	noSales := 0
	for _, entry := range history {
		if isSaleOutcome(entry.FinishOutcome) {
			conversions++
			soldValue += maxFloat(entry.SaleAmount, 0)
		} else if strings.TrimSpace(entry.FinishOutcome) == "nao-compra" {
			noSales++
		}
	}

	conversionRate := 0.0
	if totalAttendances > 0 {
		conversionRate = (float64(conversions) / float64(totalAttendances)) * 100
	}

	ticketAverage := 0.0
	if conversions > 0 {
		ticketAverage = soldValue / float64(conversions)
	}

	quickNoSaleRate := 0.0
	longNoSaleRate := 0.0
	quickCloseRate := 0.0
	longLowSaleRate := 0.0
	queueToServiceRatio := 0.0
	idleVsServiceRatio := 0.0

	if noSales > 0 {
		quickNoSaleRate = (float64(timeIntelligence.QuickNoSaleCount) / float64(noSales)) * 100
		longNoSaleRate = (float64(timeIntelligence.LongNoSaleCount) / float64(noSales)) * 100
	}
	if conversions > 0 {
		quickCloseRate = (float64(timeIntelligence.QuickHighPotentialCount) / float64(conversions)) * 100
		longLowSaleRate = (float64(timeIntelligence.LongLowSaleCount) / float64(conversions)) * 100
	}
	if timeIntelligence.ConsultantsInServiceMs > 0 {
		queueToServiceRatio = float64(timeIntelligence.ConsultantsInQueueMs) / float64(timeIntelligence.ConsultantsInServiceMs)
	}
	if timeIntelligence.TotalsByStatus.Service > 0 {
		idleVsServiceRatio = float64(timeIntelligence.TotalsByStatus.Available) / float64(timeIntelligence.TotalsByStatus.Service)
	}

	diagnosis := make([]IntelligenceDiagnosis, 0, 6)
	appendDiagnosis := func(item IntelligenceDiagnosis) {
		diagnosis = append(diagnosis, item)
	}

	if totalAttendances < 6 {
		appendDiagnosis(IntelligenceDiagnosis{
			ID:         "sample-size",
			Level:      "attention",
			Title:      "Base ainda pequena para conclusoes fortes",
			Reading:    fmt.Sprintf("%d atendimentos registrados ate agora.", totalAttendances),
			Hypothesis: "A amostra ainda pode distorcer as leituras de tempo e conversao.",
			Action:     "Coletar mais atendimentos antes de tomar decisoes estruturais.",
		})
	}

	switch {
	case timeIntelligence.NotUsingQueueRate >= 25:
		appendDiagnosis(IntelligenceDiagnosis{"queue-discipline", "critical", "Uso da fila comprometido", fmt.Sprintf("%s dos atendimentos foram fora da vez.", formatPercent(timeIntelligence.NotUsingQueueRate)), "A regra da fila pode estar sendo ignorada com frequencia.", "Reforcar criterio para furar fila e auditar motivos por consultor diariamente."})
	case timeIntelligence.NotUsingQueueRate >= 12:
		appendDiagnosis(IntelligenceDiagnosis{"queue-discipline", "attention", "Uso da fila acima do ideal", fmt.Sprintf("%s dos atendimentos foram fora da vez.", formatPercent(timeIntelligence.NotUsingQueueRate)), "Pode haver excesso de excecoes no fluxo da loja.", "Acompanhar quem mais fura fila e validar se os motivos fazem sentido."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"queue-discipline", "healthy", "Disciplina de fila estavel", fmt.Sprintf("Atendimento fora da vez em %s.", formatPercent(timeIntelligence.NotUsingQueueRate)), "As excecoes estao sob controle operacional.", "Manter monitoramento dos motivos para manter consistencia."})
	}

	switch {
	case queueToServiceRatio >= 1.2 && timeIntelligence.ConsultantsInQueueMs >= 20*60000:
		appendDiagnosis(IntelligenceDiagnosis{"live-backlog", "critical", "Backlog atual de fila elevado", fmt.Sprintf("Fila atual acumulada %s vs atendimento atual %s.", formatDurationMinutes(timeIntelligence.ConsultantsInQueueMs), formatDurationMinutes(timeIntelligence.ConsultantsInServiceMs)), "Equipe pode estar sem tracao de inicio de atendimento no momento.", "Acionar lider para redistribuir entrada em atendimento nos proximos minutos."})
	case queueToServiceRatio >= 0.7 && timeIntelligence.ConsultantsInQueueMs >= 10*60000:
		appendDiagnosis(IntelligenceDiagnosis{"live-backlog", "attention", "Fila atual crescendo", fmt.Sprintf("Fila atual acumulada %s.", formatDurationMinutes(timeIntelligence.ConsultantsInQueueMs)), "A demanda atual pode estar maior que a capacidade ativa.", "Priorizar chamadas da fila e reduzir pausas nao essenciais."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"live-backlog", "healthy", "Ritmo atual equilibrado", fmt.Sprintf("Fila atual em %s.", formatDurationMinutes(timeIntelligence.ConsultantsInQueueMs)), "Fluxo atual entre espera e atendimento esta proporcional.", "Manter ritmo de chamada e monitorar picos por horario."})
	}

	switch {
	case quickNoSaleRate >= 45:
		appendDiagnosis(IntelligenceDiagnosis{"quick-no-sale", "critical", "Nao compra muito rapida", fmt.Sprintf("%s dos nao fechamentos encerram muito rapido.", formatPercent(quickNoSaleRate)), "Abordagem inicial pode estar curta, sem exploracao de oportunidade.", "Testar script de descoberta de motivo e sugestao de 2a opcao antes de encerrar."})
	case quickNoSaleRate >= 25:
		appendDiagnosis(IntelligenceDiagnosis{"quick-no-sale", "attention", "Nao compra rapida em alta", fmt.Sprintf("%s dos nao fechamentos foram rapidos.", formatPercent(quickNoSaleRate)), "Pode haver descarte precoce de atendimento.", "Acompanhar atendimentos curtos e criar checklist minimo antes de encerrar."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"quick-no-sale", "healthy", "Tempo minimo de exploracao razoavel", fmt.Sprintf("%s dos nao fechamentos foram rapidos.", formatPercent(quickNoSaleRate)), "A equipe tende a investigar melhor antes de encerrar sem venda.", "Manter rotina de registro do motivo de nao compra."})
	}

	switch {
	case longLowSaleRate >= 30 || longNoSaleRate >= 35:
		appendDiagnosis(IntelligenceDiagnosis{"long-service-low-return", "critical", "Atendimento longo com retorno baixo", fmt.Sprintf("%s de vendas longas com ticket baixo e %s de nao compra longa.", formatPercent(longLowSaleRate), formatPercent(longNoSaleRate)), "Tempo alto sem progresso pode indicar baixa objetividade na conducao.", "Criar checkpoints de 5 em 5 minutos para avancar proposta, upsell ou encerramento."})
	case longLowSaleRate >= 18 || longNoSaleRate >= 20:
		appendDiagnosis(IntelligenceDiagnosis{"long-service-low-return", "attention", "Parte dos atendimentos longos sem retorno", fmt.Sprintf("%s de vendas longas com ticket baixo.", formatPercent(longLowSaleRate)), "Existe espaco para melhorar conducao de atendimento demorado.", "Revisar casos de maior duracao e mapear pontos de trava."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"long-service-low-return", "healthy", "Duracao e retorno em equilibrio", "Nao houve excesso relevante de atendimento longo com baixo retorno.", "O tempo investido esta proporcional ao resultado comercial.", "Continuar monitorando para manter estabilidade."})
	}

	if quickCloseRate >= 45 {
		appendDiagnosis(IntelligenceDiagnosis{"quick-close", "attention", "Fechamento rapido em excesso", fmt.Sprintf("%s das conversoes encerram muito rapido.", formatPercent(quickCloseRate)), "Pode existir perda de oportunidade de relacionamento ou venda complementar.", "Adicionar passo obrigatorio de relacionamento antes de fechar atendimento."})
	} else {
		appendDiagnosis(IntelligenceDiagnosis{"quick-close", "healthy", "Tempo de fechamento sob controle", fmt.Sprintf("%s das conversoes sao muito rapidas.", formatPercent(quickCloseRate)), "Fechamento sem sinal forte de pressa excessiva.", "Manter foco em coletar dados do cliente no encerramento."})
	}

	if idleVsServiceRatio >= 1 && totalAttendances >= 8 {
		appendDiagnosis(IntelligenceDiagnosis{"idle-capacity", "attention", "Tempo ocioso acima do tempo atendendo", fmt.Sprintf("Historico ocioso %s vs atendendo %s.", formatDurationMinutes(timeIntelligence.TotalsByStatus.Available), formatDurationMinutes(timeIntelligence.TotalsByStatus.Service)), "Pode haver capacidade parada ou falha de uso da lista em horarios de baixa.", "Criar rotina de ativacao em horarios ociosos (vitrine, WhatsApp, base de leads)."})
	} else {
		appendDiagnosis(IntelligenceDiagnosis{"idle-capacity", "healthy", "Uso de capacidade dentro do esperado", fmt.Sprintf("Historico atendendo %s.", formatDurationMinutes(timeIntelligence.TotalsByStatus.Service)), "Nao ha sinal forte de ociosidade acima do esperado.", "Manter acompanhamento por turno para ajustar escala."})
	}

	severityCounts := SeverityCounts{}
	recommendedActions := make([]string, 0, 4)
	for _, item := range diagnosis {
		switch item.Level {
		case "critical":
			severityCounts.Critical++
		case "attention":
			severityCounts.Attention++
		default:
			severityCounts.Healthy++
		}

		if item.Level != "healthy" && len(recommendedActions) < 4 {
			recommendedActions = append(recommendedActions, item.Action)
		}
	}

	healthScore := 100 - float64(severityCounts.Critical*18) - float64(severityCounts.Attention*8)
	if healthScore < 0 {
		healthScore = 0
	}

	return IntelligenceResponse{
		StoreID:            storeID,
		TenantID:           tenantID,
		TotalAttendances:   totalAttendances,
		ConversionRate:     conversionRate,
		TicketAverage:      ticketAverage,
		HealthScore:        healthScore,
		SeverityCounts:     severityCounts,
		Diagnosis:          diagnosis,
		RecommendedActions: recommendedActions,
		Time:               timeIntelligence,
	}
}

func isCompleteEntry(entry operations.ServiceHistoryEntry) bool {
	hasCustomer := hasText(entry.CustomerName) || hasText(entry.CustomerPhone)
	hasProduct := len(entry.ProductsSeen) > 0 || hasText(entry.ProductSeen) || hasText(entry.ProductDetails) || entry.ProductsSeenNone
	hasReason := len(entry.VisitReasons) > 0 || entry.VisitReasonsNotInformed
	hasSource := len(entry.CustomerSources) > 0 || entry.CustomerSourcesNotInformed
	return hasCustomer && hasProduct && hasReason && hasSource
}

func resolveLiveStatusSnapshot(snapshot operations.SnapshotState, consultantID string, now int64) (string, int64) {
	for _, item := range snapshot.ActiveServices {
		if strings.TrimSpace(item.ConsultantID) == strings.TrimSpace(consultantID) {
			return "service", maxInt64(item.ServiceStartedAt, now)
		}
	}

	for _, item := range snapshot.WaitingList {
		if strings.TrimSpace(item.ConsultantID) == strings.TrimSpace(consultantID) {
			return "queue", maxInt64(item.QueueJoinedAt, now)
		}
	}

	for _, item := range snapshot.PausedEmployees {
		if strings.TrimSpace(item.ConsultantID) == strings.TrimSpace(consultantID) {
			return "paused", maxInt64(item.StartedAt, now)
		}
	}

	if currentStatus, ok := snapshot.ConsultantCurrentStatus[consultantID]; ok {
		return strings.TrimSpace(currentStatus.Status), maxInt64(currentStatus.StartedAt, now)
	}

	return "available", now
}

func addStatusDuration(totals *StatusTotals, status string, duration int64) {
	switch strings.TrimSpace(status) {
	case "queue":
		totals.Queue += maxInt64(duration, 0)
	case "service":
		totals.Service += maxInt64(duration, 0)
	case "paused":
		totals.Paused += maxInt64(duration, 0)
	default:
		totals.Available += maxInt64(duration, 0)
	}
}

func groupLabels(values []string, limit int) []CountRow {
	counter := map[string]*CountRow{}
	for _, rawValue := range values {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}

		key := strings.ToLower(value)
		row, ok := counter[key]
		if !ok {
			row = &CountRow{Label: value}
			counter[key] = row
		}
		row.Count++
	}

	rows := make([]CountRow, 0, len(counter))
	for _, row := range counter {
		rows = append(rows, *row)
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

func closedProductLabels(entry operations.ServiceHistoryEntry) []string {
	labels := make([]string, 0, len(entry.ProductsClosed))
	for _, product := range entry.ProductsClosed {
		label := firstNonEmpty(product.Name, product.Code)
		if label != "" {
			labels = append(labels, label)
		}
	}

	if len(labels) > 0 {
		return labels
	}

	fallback := firstNonEmpty(entry.ProductClosed, entry.ProductSeen, entry.ProductDetails)
	if fallback == "" {
		return []string{}
	}

	return []string{fallback}
}

func monthStamp(moment time.Time) string {
	return moment.Format("2006-01")
}

func dayStamp(moment time.Time) string {
	return moment.Format("2006-01-02")
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func formatDurationMinutes(valueMs int64) string {
	minutes := int64(0)
	if valueMs > 0 {
		minutes = valueMs / 60000
		if valueMs%60000 >= 30000 {
			minutes++
		}
	}

	return fmt.Sprintf("%d min", minutes)
}

func isSaleOutcome(outcome string) bool {
	switch strings.TrimSpace(outcome) {
	case "compra", "reserva":
		return true
	default:
		return false
	}
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
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

func maxFloat(value float64, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}

	return value
}

func maxInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func maxInt64(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}

	return value
}
