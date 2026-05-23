package erp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func crmDateRangeArgs(query CRMOverviewQuery) (any, any) {
	var dateFrom any
	if !query.DateFrom.IsZero() {
		dateFrom = query.DateFrom.UTC()
	}

	var dateToExclusive any
	if !query.DateTo.IsZero() {
		if query.DateToHasTime {
			dateToExclusive = query.DateTo.UTC().Add(time.Minute)
		} else {
			dateToExclusive = query.DateTo.UTC().AddDate(0, 0, 1)
		}
	}

	return dateFrom, dateToExclusive
}

func (repository *PostgresRepository) listCRMOrderAggregates(ctx context.Context, store StoreScope, query CRMOverviewQuery) ([]crmOrderAggregate, error) {
	dateFrom, dateToExclusive := crmDateRangeArgs(query)
	rows, err := repository.pool.Query(ctx, `
		with raw_lines as (
			select
				order_id,
				coalesce(nullif(trim(store_id_raw), ''), '') as explicit_store_key,
				coalesce(nullif(trim(store_cnpj), ''), '') as fallback_store_cnpj,
				coalesce(nullif(trim(employee_id), ''), '') as employee_id,
				coalesce(total_amount_cents, 0) as total_amount_cents,
				coalesce(amount_cents, 0) as amount_cents,
				coalesce(nullif(trim(sku), ''), 'sem-sku') as sku,
				case when coalesce(quantity, 0) > 0 then quantity else 1 end as quantity
			from erp_order_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and ($3::timestamptz is null or order_date >= $3::timestamptz)
			  and ($4::timestamptz is null or order_date < $4::timestamptz)
			  and nullif(trim(order_id), '') is not null
		), order_items as (
			-- dedup por (order_id, sku): evita inflar units com linhas de parcelas de pagamento
			select
				order_id,
				sku,
				max(explicit_store_key)    as explicit_store_key,
				max(fallback_store_cnpj)   as fallback_store_cnpj,
				max(employee_id)           as employee_id,
				max(total_amount_cents)    as total_amount_cents,
				max(amount_cents)          as amount_cents,
				max(quantity)              as quantity
			from raw_lines
			group by order_id, sku
		), canceled_orders as (
			select distinct order_id
			from erp_order_canceled_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
		), orders_grouped as (
			select
				order_id,
				coalesce(max(explicit_store_key), '') as explicit_store_key,
				coalesce(max(fallback_store_cnpj), '') as fallback_store_cnpj,
				coalesce(max(employee_id), '') as employee_id,
				case
					when max(total_amount_cents) > 0 then max(total_amount_cents)::bigint
					else sum(amount_cents)::bigint
				end as order_total_cents,
				sum(amount_cents)::bigint as product_sales_cents,
				sum(quantity)::bigint as units
			from order_items
			group by order_id
		), active_orders as (
			select *
			from orders_grouped grouped
			where not exists (
				select 1
				from canceled_orders canceled
				where canceled.order_id = grouped.order_id
			)
		)
		select
			explicit_store_key,
			fallback_store_cnpj,
			employee_id,
			coalesce(units, 0)::bigint as units,
			coalesce(order_total_cents, 0)::bigint as sales_cents,
			coalesce(product_sales_cents, 0)::bigint as product_sales_cents
		from active_orders
		order by sales_cents desc, order_id asc;
	`, store.TenantID, store.StoreID, dateFrom, dateToExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]crmOrderAggregate, 0, 128)
	for rows.Next() {
		var aggregate crmOrderAggregate
		if err := rows.Scan(
			&aggregate.ExplicitStoreCNPJ,
			&aggregate.FallbackStoreCNPJ,
			&aggregate.EmployeeID,
			&aggregate.Units,
			&aggregate.SalesCents,
			&aggregate.ProductSalesCents,
		); err != nil {
			return nil, err
		}

		aggregate.ExplicitStoreCNPJ = onlyDigits(strings.TrimSpace(aggregate.ExplicitStoreCNPJ))
		aggregate.FallbackStoreCNPJ = onlyDigits(strings.TrimSpace(aggregate.FallbackStoreCNPJ))
		aggregate.EmployeeID = strings.TrimSpace(aggregate.EmployeeID)
		orders = append(orders, aggregate)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (repository *PostgresRepository) listCRMCanceledStoreAggregates(ctx context.Context, store StoreScope, query CRMOverviewQuery) ([]crmCanceledStoreAggregate, error) {
	dateFrom, dateToExclusive := crmDateRangeArgs(query)
	rows, err := repository.pool.Query(ctx, `
		select
			coalesce(nullif(trim(store_cnpj), ''), '') as store_cnpj,
			count(distinct order_id)::int              as canceled_orders
		from erp_order_canceled_raw
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and ($3::timestamptz is null or order_date >= $3::timestamptz)
		  and ($4::timestamptz is null or order_date < $4::timestamptz)
		  and nullif(trim(order_id), '') is not null
		group by store_cnpj
	`, store.TenantID, store.StoreID, dateFrom, dateToExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aggregates := make([]crmCanceledStoreAggregate, 0, 8)
	for rows.Next() {
		var a crmCanceledStoreAggregate
		if err := rows.Scan(&a.StoreCNPJ, &a.CanceledOrders); err != nil {
			return nil, err
		}
		a.StoreCNPJ = onlyDigits(strings.TrimSpace(a.StoreCNPJ))
		aggregates = append(aggregates, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return aggregates, nil
}

func (repository *PostgresRepository) listCRMStoreAggregates(ctx context.Context, store StoreScope, query CRMOverviewQuery) ([]crmStoreAggregate, error) {
	orders, err := repository.listCRMOrderAggregates(ctx, store, query)
	if err != nil {
		return nil, err
	}

	employeeStoreFallbacks, err := repository.listCRMEmployeeStoreFallbacks(ctx, store.TenantID)
	if err != nil {
		return nil, err
	}

	employeeDominantStoreKeys, err := repository.listCRMDominantEmployeeStoreKeys(ctx, store)
	if err != nil {
		return nil, err
	}

	rowsByStore := make(map[string]*crmStoreAggregate, 8)
	for _, order := range orders {
		storeKey := resolveCRMOrderStoreKey(order.ExplicitStoreCNPJ, order.FallbackStoreCNPJ, order.EmployeeID, employeeStoreFallbacks, employeeDominantStoreKeys)
		row, ok := rowsByStore[storeKey]
		if !ok {
			row = &crmStoreAggregate{StoreCNPJ: storeKey}
			rowsByStore[storeKey] = row
		}
		row.Orders += 1
		row.Units += order.Units
		row.SalesCents += order.SalesCents
		row.ProductSalesCents += order.ProductSalesCents
	}

	aggregates := make([]crmStoreAggregate, 0, len(rowsByStore))
	for _, aggregate := range rowsByStore {
		aggregates = append(aggregates, *aggregate)
	}

	sort.Slice(aggregates, func(left int, right int) bool {
		if aggregates[left].SalesCents != aggregates[right].SalesCents {
			return aggregates[left].SalesCents > aggregates[right].SalesCents
		}
		return aggregates[left].StoreCNPJ < aggregates[right].StoreCNPJ
	})

	return aggregates, nil
}

func (repository *PostgresRepository) listCRMConsultantAggregates(ctx context.Context, store StoreScope, query CRMOverviewQuery) ([]crmConsultantAggregate, error) {
	orders, err := repository.listCRMOrderAggregates(ctx, store, query)
	if err != nil {
		return nil, err
	}

	employeeStoreFallbacks, err := repository.listCRMEmployeeStoreFallbacks(ctx, store.TenantID)
	if err != nil {
		return nil, err
	}

	employeeDominantStoreKeys, err := repository.listCRMDominantEmployeeStoreKeys(ctx, store)
	if err != nil {
		return nil, err
	}

	employeeNames, err := repository.listCRMEmployeeNames(ctx, store)
	if err != nil {
		return nil, err
	}

	rowsByConsultant := make(map[string]*crmConsultantAggregate, 32)
	for _, order := range orders {
		consultantID := strings.TrimSpace(order.EmployeeID)
		if consultantID == "" {
			consultantID = "sem-id"
		}

		storeKey := resolveCRMOrderStoreKey(order.ExplicitStoreCNPJ, order.FallbackStoreCNPJ, order.EmployeeID, employeeStoreFallbacks, employeeDominantStoreKeys)
		rowKey := consultantID + "\x00" + storeKey
		row, ok := rowsByConsultant[rowKey]
		if !ok {
			consultantName := strings.TrimSpace(employeeNames[consultantID])
			if consultantName == "" {
				consultantName = fmt.Sprintf("Consultor ERP %s", consultantID)
			}

			row = &crmConsultantAggregate{
				ConsultantID:   consultantID,
				ConsultantName: consultantName,
				StoreCNPJ:      storeKey,
			}
			rowsByConsultant[rowKey] = row
		}

		row.Orders += 1
		row.Units += order.Units
		row.SalesCents += order.SalesCents
		row.ProductSalesCents += order.ProductSalesCents
	}

	aggregates := make([]crmConsultantAggregate, 0, len(rowsByConsultant))
	for _, aggregate := range rowsByConsultant {
		aggregates = append(aggregates, *aggregate)
	}

	sort.Slice(aggregates, func(left int, right int) bool {
		if aggregates[left].SalesCents != aggregates[right].SalesCents {
			return aggregates[left].SalesCents > aggregates[right].SalesCents
		}
		if aggregates[left].StoreCNPJ != aggregates[right].StoreCNPJ {
			return aggregates[left].StoreCNPJ < aggregates[right].StoreCNPJ
		}
		return aggregates[left].ConsultantName < aggregates[right].ConsultantName
	})

	return aggregates, nil
}
