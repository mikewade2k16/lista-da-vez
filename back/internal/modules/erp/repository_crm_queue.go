package erp

import (
	"context"
	"sort"
	"strings"
	"time"
)

func crmQueueDateArgs(query CRMOverviewQuery) (any, any) {
	var dateFromMs any
	if !query.DateFrom.IsZero() {
		dateFromMs = query.DateFrom.UTC().UnixMilli()
	}

	var dateToExclusiveMs any
	if !query.DateTo.IsZero() {
		if query.DateToHasTime {
			dateToExclusiveMs = query.DateTo.UTC().Add(time.Minute).UnixMilli()
		} else {
			dateToExclusiveMs = query.DateTo.UTC().AddDate(0, 0, 1).UnixMilli()
		}
	}
	return dateFromMs, dateToExclusiveMs
}

func (repository *PostgresRepository) listCRMQueueConsultantStats(ctx context.Context, store StoreScope, query CRMOverviewQuery, allowedStoreIDs []string) ([]crmQueueConsultantStat, error) {
	dateFromMs, dateToExclusiveMs := crmQueueDateArgs(query)

	rows, err := repository.pool.Query(ctx, `
		select
			h.person_id::text,
			coalesce(nullif(trim(h.person_name), ''), 'Consultor sem nome') as person_name,
			s.id::text,
			coalesce(s.code, '') as store_code,
			coalesce(s.name, '') as store_name,
			count(*)::int                                                                            as attendances,
			count(*) filter (where finish_outcome = 'compra')::int                                  as conversions,
			count(*) filter (
				where nullif(trim(coalesce(to_jsonb(h)->>'cancel_reason', '')), '') is not null
			)::int                                                                                  as queue_cancellations
		from operation_service_history h
		join stores s on s.id = h.store_id
		where s.tenant_id = $1::uuid
		  and s.is_active = true
		  and ($2::uuid[] is null or s.id = any($2::uuid[]))
		  and ($3::bigint is null or finished_at >= $3::bigint)
		  and ($4::bigint is null or finished_at < $4::bigint)
		  and finished_at > 0
		group by h.person_id, h.person_name, s.id, s.code, s.name
		order by count(*) filter (where finish_outcome = 'compra') desc, person_name asc
	`, store.TenantID, crmQueueAllowedStoreIDsArg(allowedStoreIDs), dateFromMs, dateToExclusiveMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]crmQueueConsultantStat, 0, 32)
	for rows.Next() {
		var s crmQueueConsultantStat
		if err := rows.Scan(&s.PersonID, &s.PersonName, &s.StoreID, &s.StoreCode, &s.StoreName, &s.Attendances, &s.Conversions, &s.QueueCancellations); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (repository *PostgresRepository) listCRMQueueStoreStats(ctx context.Context, store StoreScope, query CRMOverviewQuery, allowedStoreIDs []string) ([]crmQueueStoreStat, error) {
	dateFromMs, dateToExclusiveMs := crmQueueDateArgs(query)

	rows, err := repository.pool.Query(ctx, `
		select
			s.id::text,
			coalesce(s.code, '') as store_code,
			coalesce(s.name, '') as store_name,
			count(*)::int                                                                            as attendances,
			count(*) filter (where finish_outcome = 'compra')::int                                  as conversions,
			count(*) filter (
				where nullif(trim(coalesce(to_jsonb(h)->>'cancel_reason', '')), '') is not null
			)::int                                                                                  as queue_cancellations
		from operation_service_history h
		join stores s on s.id = h.store_id
		where s.tenant_id = $1::uuid
		  and s.is_active = true
		  and ($2::uuid[] is null or s.id = any($2::uuid[]))
		  and ($3::bigint is null or finished_at >= $3::bigint)
		  and ($4::bigint is null or finished_at < $4::bigint)
		  and finished_at > 0
		group by s.id, s.code, s.name
	`, store.TenantID, crmQueueAllowedStoreIDsArg(allowedStoreIDs), dateFromMs, dateToExclusiveMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]crmQueueStoreStat, 0, 8)
	for rows.Next() {
		var s crmQueueStoreStat
		if err := rows.Scan(&s.StoreID, &s.StoreCode, &s.StoreName, &s.Attendances, &s.Conversions, &s.QueueCancellations); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func buildQueueStats(storeStats []crmQueueStoreStat, consultantStats []crmQueueConsultantStat) QueueStats {
	result := QueueStats{
		ByStore:      make([]QueueStoreStats, 0, len(storeStats)),
		ByConsultant: make([]QueueConsultantStats, 0, len(consultantStats)),
	}

	storesByKey := make(map[string]*QueueStoreStats, len(storeStats))
	consultantsByKey := make(map[string]*QueueConsultantStats, len(consultantStats))

	for _, s := range storeStats {
		result.TotalAttendances += s.Attendances
		result.TotalConversions += s.Conversions
		result.TotalCancellations += s.QueueCancellations

		storeSlug, storeLabel := crmStoreSlugFromOperationalStore(s.StoreCode, s.StoreName)
		key := crmQueueStoreAggregateKey(storeSlug, s.StoreID)
		row, ok := storesByKey[key]
		if !ok {
			row = &QueueStoreStats{
				StoreID:    s.StoreID,
				StoreSlug:  storeSlug,
				StoreLabel: storeLabel,
			}
			storesByKey[key] = row
		}
		row.Attendances += s.Attendances
		row.Conversions += s.Conversions
		row.QueueCancellations += s.QueueCancellations
	}

	if result.TotalAttendances > 0 {
		result.ConversionRate = float64(result.TotalConversions) / float64(result.TotalAttendances) * 100
		result.CancellationRate = float64(result.TotalCancellations) / float64(result.TotalAttendances) * 100
	}

	for _, s := range consultantStats {
		storeSlug, storeLabel := crmStoreSlugFromOperationalStore(s.StoreCode, s.StoreName)
		key := crmQueueConsultantAggregateKey(storeSlug, s.PersonID, s.PersonName)
		row, ok := consultantsByKey[key]
		if !ok {
			row = &QueueConsultantStats{
				PersonID:   s.PersonID,
				PersonName: s.PersonName,
				StoreID:    s.StoreID,
				StoreSlug:  storeSlug,
				StoreLabel: storeLabel,
			}
			consultantsByKey[key] = row
		}
		row.Attendances += s.Attendances
		row.Conversions += s.Conversions
		row.QueueCancellations += s.QueueCancellations
	}

	for _, row := range storesByKey {
		if row.Attendances > 0 {
			row.ConversionRate = float64(row.Conversions) / float64(row.Attendances) * 100
			row.QueueCancellationRate = float64(row.QueueCancellations) / float64(row.Attendances) * 100
		}
		result.ByStore = append(result.ByStore, *row)
	}
	sort.Slice(result.ByStore, func(i, j int) bool {
		leftLabel := result.ByStore[i].StoreLabel
		if leftLabel == "" {
			leftLabel = result.ByStore[i].StoreSlug
		}
		rightLabel := result.ByStore[j].StoreLabel
		if rightLabel == "" {
			rightLabel = result.ByStore[j].StoreSlug
		}
		return leftLabel < rightLabel
	})

	for _, row := range consultantsByKey {
		if row.Attendances > 0 {
			row.ConversionRate = float64(row.Conversions) / float64(row.Attendances) * 100
			row.QueueCancellationRate = float64(row.QueueCancellations) / float64(row.Attendances) * 100
		}
		result.ByConsultant = append(result.ByConsultant, *row)
	}

	sort.Slice(result.ByConsultant, func(i, j int) bool {
		if result.ByConsultant[i].Conversions != result.ByConsultant[j].Conversions {
			return result.ByConsultant[i].Conversions > result.ByConsultant[j].Conversions
		}
		return result.ByConsultant[i].PersonName < result.ByConsultant[j].PersonName
	})

	return result
}

func crmQueueAllowedStoreIDsArg(allowedStoreIDs []string) any {
	if len(allowedStoreIDs) == 0 {
		return nil
	}
	return allowedStoreIDs
}

func crmQueueStoreAggregateKey(storeSlug string, storeID string) string {
	if trimmedSlug := strings.TrimSpace(storeSlug); trimmedSlug != "" {
		return trimmedSlug
	}
	return strings.TrimSpace(storeID)
}

func crmQueueConsultantAggregateKey(storeSlug string, personID string, personName string) string {
	base := strings.TrimSpace(personID)
	if base == "" {
		base = strings.ToLower(strings.TrimSpace(personName))
	}
	return crmQueueStoreAggregateKey(storeSlug, "") + "\x00" + base
}
