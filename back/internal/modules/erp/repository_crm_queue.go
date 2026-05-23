package erp

import (
	"context"
	"sort"
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

func (repository *PostgresRepository) listCRMQueueConsultantStats(ctx context.Context, store StoreScope, query CRMOverviewQuery) ([]crmQueueConsultantStat, error) {
	dateFromMs, dateToExclusiveMs := crmQueueDateArgs(query)

	rows, err := repository.pool.Query(ctx, `
		select
			person_id::text,
			coalesce(nullif(trim(person_name), ''), 'Consultor sem nome') as person_name,
			store_id::text,
			count(*)::int                                                                            as attendances,
			count(*) filter (where finish_outcome = 'compra')::int                                  as conversions,
			count(*) filter (
				where nullif(trim(coalesce(to_jsonb(h)->>'cancel_reason', '')), '') is not null
			)::int                                                                                  as queue_cancellations
		from operation_service_history h
		where store_id = $1::uuid
		  and ($2::bigint is null or finished_at >= $2::bigint)
		  and ($3::bigint is null or finished_at < $3::bigint)
		  and finished_at > 0
		group by person_id, person_name, store_id
		order by count(*) filter (where finish_outcome = 'compra') desc, person_name asc
	`, store.StoreID, dateFromMs, dateToExclusiveMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]crmQueueConsultantStat, 0, 32)
	for rows.Next() {
		var s crmQueueConsultantStat
		if err := rows.Scan(&s.PersonID, &s.PersonName, &s.StoreID, &s.Attendances, &s.Conversions, &s.QueueCancellations); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (repository *PostgresRepository) listCRMQueueStoreStats(ctx context.Context, store StoreScope, query CRMOverviewQuery) ([]crmQueueStoreStat, error) {
	dateFromMs, dateToExclusiveMs := crmQueueDateArgs(query)

	rows, err := repository.pool.Query(ctx, `
		select
			store_id::text,
			count(*)::int                                                                            as attendances,
			count(*) filter (where finish_outcome = 'compra')::int                                  as conversions,
			count(*) filter (
				where nullif(trim(coalesce(to_jsonb(h)->>'cancel_reason', '')), '') is not null
			)::int                                                                                  as queue_cancellations
		from operation_service_history h
		where store_id = $1::uuid
		  and ($2::bigint is null or finished_at >= $2::bigint)
		  and ($3::bigint is null or finished_at < $3::bigint)
		  and finished_at > 0
		group by store_id
	`, store.StoreID, dateFromMs, dateToExclusiveMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]crmQueueStoreStat, 0, 8)
	for rows.Next() {
		var s crmQueueStoreStat
		if err := rows.Scan(&s.StoreID, &s.Attendances, &s.Conversions, &s.QueueCancellations); err != nil {
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

	for _, s := range storeStats {
		result.TotalAttendances += s.Attendances
		result.TotalConversions += s.Conversions
		result.TotalCancellations += s.QueueCancellations

		convRate := 0.0
		if s.Attendances > 0 {
			convRate = float64(s.Conversions) / float64(s.Attendances) * 100
		}
		cancRate := 0.0
		if s.Attendances > 0 {
			cancRate = float64(s.QueueCancellations) / float64(s.Attendances) * 100
		}
		result.ByStore = append(result.ByStore, QueueStoreStats{
			StoreID:               s.StoreID,
			Attendances:           s.Attendances,
			Conversions:           s.Conversions,
			ConversionRate:        convRate,
			QueueCancellations:    s.QueueCancellations,
			QueueCancellationRate: cancRate,
		})
	}

	if result.TotalAttendances > 0 {
		result.ConversionRate = float64(result.TotalConversions) / float64(result.TotalAttendances) * 100
		result.CancellationRate = float64(result.TotalCancellations) / float64(result.TotalAttendances) * 100
	}

	for _, s := range consultantStats {
		convRate := 0.0
		if s.Attendances > 0 {
			convRate = float64(s.Conversions) / float64(s.Attendances) * 100
		}
		cancRate := 0.0
		if s.Attendances > 0 {
			cancRate = float64(s.QueueCancellations) / float64(s.Attendances) * 100
		}
		result.ByConsultant = append(result.ByConsultant, QueueConsultantStats{
			PersonID:              s.PersonID,
			PersonName:            s.PersonName,
			StoreID:               s.StoreID,
			Attendances:           s.Attendances,
			Conversions:           s.Conversions,
			ConversionRate:        convRate,
			QueueCancellations:    s.QueueCancellations,
			QueueCancellationRate: cancRate,
		})
	}

	sort.Slice(result.ByConsultant, func(i, j int) bool {
		if result.ByConsultant[i].Conversions != result.ByConsultant[j].Conversions {
			return result.ByConsultant[i].Conversions > result.ByConsultant[j].Conversions
		}
		return result.ByConsultant[i].PersonName < result.ByConsultant[j].PersonName
	})

	return result
}
