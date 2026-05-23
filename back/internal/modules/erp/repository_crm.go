package erp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (repository *PostgresRepository) GetCRMOverview(ctx context.Context, store StoreScope, query CRMOverviewQuery) (CRMOverviewResponse, error) {
	targets, err := repository.listCRMStoreTargets(ctx, store.TenantID)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	storeAggregates, err := repository.listCRMStoreAggregates(ctx, store, query)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	consultantAggregates, err := repository.listCRMConsultantAggregates(ctx, store, query)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	canceledAggregates, err := repository.listCRMCanceledStoreAggregates(ctx, store, query)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	queueStoreStats, err := repository.listCRMQueueStoreStats(ctx, store, query)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	queueConsultantStats, err := repository.listCRMQueueConsultantStats(ctx, store, query)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	// index cancelamentos ERP por CNPJ para lookup O(1)
	canceledByCNPJ := make(map[string]int, len(canceledAggregates))
	for _, c := range canceledAggregates {
		canceledByCNPJ[c.StoreCNPJ] += c.CanceledOrders
	}

	rowsByKey := make(map[string]*CRMStoreMetric, len(targets)+len(storeAggregates))
	productSalesByKey := make(map[string]int64, len(targets)+len(storeAggregates))
	unmappedCNPJs := make([]string, 0)

	for _, target := range targets {
		rowsByKey[target.Slug] = &CRMStoreMetric{
			StoreSlug:          target.Slug,
			StoreLabel:         target.Label,
			StoreCode:          target.Code,
			StoreName:          target.Name,
			StoreCNPJs:         []string{},
			Mapped:             true,
			MonthlyGoalCents:   target.MonthlyGoalCents,
			AvgTicketGoalCents: target.AvgTicketGoalCents,
			PAGoal:             target.PAGoal,
		}
		productSalesByKey[target.Slug] = 0
	}

	for _, aggregate := range storeAggregates {
		key, row := repository.resolveCRMStoreMetricRow(rowsByKey, targets, aggregate.StoreCNPJ)
		if !row.Mapped {
			unmappedCNPJs = appendUniqueString(unmappedCNPJs, aggregate.StoreCNPJ)
		}
		row.StoreCNPJs = appendUniqueString(row.StoreCNPJs, aggregate.StoreCNPJ)
		row.Orders += aggregate.Orders
		row.Units += aggregate.Units
		row.SalesCents += aggregate.SalesCents
		productSalesByKey[key] += aggregate.ProductSalesCents
	}

	// acumular cancelamentos ERP por slug de loja
	canceledBySlug := make(map[string]int, len(rowsByKey))
	for cnpj, count := range canceledByCNPJ {
		if alias, ok := resolveCRMStoreAlias(cnpj); ok {
			canceledBySlug[alias.Slug] += count
		}
	}

	storeRows := make([]CRMStoreMetric, 0, len(rowsByKey))
	for key, row := range rowsByKey {
		row.TicketAverageCents, row.ValuePerProductCents, row.PAScore = buildCRMMetricValues(row.Orders, row.Units, row.SalesCents, productSalesByKey[key])
		if row.MonthlyGoalCents > 0 {
			row.GoalProgress = (float64(row.SalesCents) / float64(row.MonthlyGoalCents)) * 100
			row.RemainingToGoalCents = maxCRMRemaining(row.MonthlyGoalCents, row.SalesCents)
		}
		if canceled := canceledBySlug[key]; canceled > 0 {
			total := row.Orders + canceled
			row.ERPCancellations = canceled
			if total > 0 {
				row.ERPCancellationRate = float64(canceled) / float64(total) * 100
			}
		}
		sort.Strings(row.StoreCNPJs)
		storeRows = append(storeRows, *row)
	}

	sort.Slice(storeRows, func(left int, right int) bool {
		leftOrder := crmStoreOrderValue(storeRows[left])
		rightOrder := crmStoreOrderValue(storeRows[right])
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if storeRows[left].Mapped != storeRows[right].Mapped {
			return storeRows[left].Mapped
		}
		if storeRows[left].SalesCents != storeRows[right].SalesCents {
			return storeRows[left].SalesCents > storeRows[right].SalesCents
		}
		return storeRows[left].StoreLabel < storeRows[right].StoreLabel
	})

	consultantRows := make([]CRMConsultantMetric, 0, len(consultantAggregates))
	for _, aggregate := range consultantAggregates {
		alias, mapped := resolveCRMStoreAlias(aggregate.StoreCNPJ)
		row := CRMConsultantMetric{
			ConsultantID:   aggregate.ConsultantID,
			ConsultantName: aggregate.ConsultantName,
			StoreCNPJ:      aggregate.StoreCNPJ,
			Mapped:         mapped,
			Orders:         aggregate.Orders,
			Units:          aggregate.Units,
			SalesCents:     aggregate.SalesCents,
		}
		if mapped {
			row.StoreSlug = alias.Slug
			row.StoreLabel = alias.Label
		} else {
			row.StoreLabel = formatCRMUnknownStoreLabel(aggregate.StoreCNPJ)
		}
		row.TicketAverageCents, row.ValuePerProductCents, row.PAScore = buildCRMMetricValues(aggregate.Orders, aggregate.Units, aggregate.SalesCents, aggregate.ProductSalesCents)
		consultantRows = append(consultantRows, row)
	}

	sort.Slice(consultantRows, func(left int, right int) bool {
		if consultantRows[left].SalesCents != consultantRows[right].SalesCents {
			return consultantRows[left].SalesCents > consultantRows[right].SalesCents
		}
		if consultantRows[left].StoreLabel != consultantRows[right].StoreLabel {
			return consultantRows[left].StoreLabel < consultantRows[right].StoreLabel
		}
		return consultantRows[left].ConsultantName < consultantRows[right].ConsultantName
	})

	summary := CRMSummary{}
	for _, row := range storeRows {
		summary.Orders += row.Orders
		summary.Units += row.Units
		summary.SalesCents += row.SalesCents
		summary.ERPCancellations += row.ERPCancellations
		if row.Mapped {
			summary.MonthlyGoalCents += row.MonthlyGoalCents
		} else {
			summary.UnmappedSalesCents += row.SalesCents
		}
	}

	totalProductSales := int64(0)
	for _, value := range productSalesByKey {
		totalProductSales += value
	}
	summary.TicketAverageCents, summary.ValuePerProductCents, summary.PAScore = buildCRMMetricValues(summary.Orders, summary.Units, summary.SalesCents, totalProductSales)
	if summary.MonthlyGoalCents > 0 {
		summary.GoalProgress = (float64(summary.SalesCents) / float64(summary.MonthlyGoalCents)) * 100
		summary.RemainingToGoalCents = maxCRMRemaining(summary.MonthlyGoalCents, summary.SalesCents)
	}
	if totalERP := summary.Orders + summary.ERPCancellations; totalERP > 0 && summary.ERPCancellations > 0 {
		summary.ERPCancellationRate = float64(summary.ERPCancellations) / float64(totalERP) * 100
	}

	warnings := make([]string, 0, 1)
	if len(unmappedCNPJs) > 0 {
		sort.Strings(unmappedCNPJs)
		warnings = append(warnings, fmt.Sprintf("CNPJs sem mapeamento comercial: %s.", strings.Join(unmappedCNPJs, ", ")))
	}

	queueStats := buildQueueStats(queueStoreStats, queueConsultantStats)
	var queueStatsPtr *QueueStats
	if queueStats.TotalAttendances > 0 {
		queueStatsPtr = &queueStats
	}

	return CRMOverviewResponse{
		Store:       store,
		DateFrom:    formatOptionalCRMDate(query.DateFrom, query.DateFromHasTime),
		DateTo:      formatOptionalCRMDate(query.DateTo, query.DateToHasTime),
		Summary:     summary,
		Stores:      storeRows,
		Consultants: consultantRows,
		QueueStats:  queueStatsPtr,
		Warnings:    warnings,
	}, nil
}

func formatOptionalCRMDate(value time.Time, hasTime bool) string {
	if value.IsZero() {
		return ""
	}
	if hasTime {
		return value.UTC().Format("2006-01-02T15:04")
	}
	return value.UTC().Format("2006-01-02")
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func (repository *PostgresRepository) resolveCRMStoreMetricRow(rowsByKey map[string]*CRMStoreMetric, targets map[string]crmStoreTarget, storeCNPJ string) (string, *CRMStoreMetric) {
	if alias, ok := resolveCRMStoreAlias(storeCNPJ); ok {
		if row, exists := rowsByKey[alias.Slug]; exists {
			return alias.Slug, row
		}

		target := targets[alias.Slug]
		rowsByKey[alias.Slug] = &CRMStoreMetric{
			StoreSlug:          alias.Slug,
			StoreLabel:         alias.Label,
			StoreCode:          target.Code,
			StoreName:          target.Name,
			StoreCNPJs:         []string{},
			Mapped:             true,
			MonthlyGoalCents:   target.MonthlyGoalCents,
			AvgTicketGoalCents: target.AvgTicketGoalCents,
			PAGoal:             target.PAGoal,
		}
		return alias.Slug, rowsByKey[alias.Slug]
	}

	key := strings.TrimSpace(storeCNPJ)
	if key == "" {
		key = "sem-cnpj"
	}
	if row, exists := rowsByKey[key]; exists {
		return key, row
	}

	rowsByKey[key] = &CRMStoreMetric{
		StoreSlug:  key,
		StoreLabel: formatCRMUnknownStoreLabel(storeCNPJ),
		StoreCNPJs: []string{},
		Mapped:     false,
	}
	return key, rowsByKey[key]
}

func resolveCRMStoreAlias(storeCNPJ string) (crmStoreAlias, bool) {
	normalized := strings.TrimSpace(storeCNPJ)
	if alias, ok := crmSpecialStoreAliases[normalized]; ok {
		return alias, true
	}
	alias, ok := crmStoreAliases[normalized]
	return alias, ok
}

func crmStoreSlugFromOperationalStore(code string, name string) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "RIO", "PJ-RIO":
		return "riomar", "Riomar"
	case "JAR", "PJ-JAR":
		return "jardins", "Jardins"
	case "GAR", "PJ-GARCIA":
		return "garcia", "Garcia"
	case "TRE", "PJ-TRE":
		return "treze", "Treze"
	}

	normalizedName := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(normalizedName, "riomar"):
		return "riomar", "Riomar"
	case strings.Contains(normalizedName, "jardins"):
		return "jardins", "Jardins"
	case strings.Contains(normalizedName, "garcia"):
		return "garcia", "Garcia"
	case strings.Contains(normalizedName, "treze"):
		return "treze", "Treze"
	default:
		return "", ""
	}
}

func buildCRMMetricValues(orders int, units int64, salesCents int64, productSalesCents int64) (int64, int64, float64) {
	ticketAverageCents := int64(0)
	if orders > 0 {
		ticketAverageCents = salesCents / int64(orders)
	}

	valuePerProductCents := int64(0)
	baseProductSales := productSalesCents
	if baseProductSales <= 0 {
		baseProductSales = salesCents
	}
	if units > 0 {
		valuePerProductCents = baseProductSales / units
	}

	paScore := 0.0
	if orders > 0 {
		piecesForPA := units
		if piecesForPA < int64(orders) {
			piecesForPA = int64(orders)
		}
		paScore = float64(piecesForPA) / float64(orders)
	}

	return ticketAverageCents, valuePerProductCents, paScore
}

func crmStoreOrderValue(row CRMStoreMetric) int {
	if !row.Mapped {
		return 100
	}
	if value, ok := crmStoreOrder[row.StoreSlug]; ok {
		return value
	}
	return 99
}

func formatCRMUnknownStoreLabel(storeCNPJ string) string {
	normalized := strings.TrimSpace(storeCNPJ)
	if normalized == "" {
		return "Nao mapeada"
	}
	return fmt.Sprintf("Nao mapeada (%s)", normalized)
}

func appendUniqueString(values []string, value string) []string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return values
	}
	for _, current := range values {
		if strings.EqualFold(strings.TrimSpace(current), normalized) {
			return values
		}
	}
	return append(values, normalized)
}

func maxCRMRemaining(goal int64, sales int64) int64 {
	if goal <= sales {
		return 0
	}
	return goal - sales
}
