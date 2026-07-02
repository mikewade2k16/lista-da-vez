package erp

import (
	"context"
	"fmt"
	"strings"
)

// resolveOrderDateColumn decide a coluna usada no filtro de periodo. Para pedidos,
// "batch_date" usa o lote importado (auditoria); qualquer outro valor usa a data
// real da compra (order_date, PADRAO). Para os demais tipos, sempre o lote.
func resolveOrderDateColumn(dataType, dateField string) (column string, batchStyle bool) {
	switch dataType {
	case DataTypeOrder, DataTypeOrderCanceled:
		if dateField == "batch_date" {
			return "source_batch_date", true
		}
		return "order_date", false
	default:
		return "source_batch_date", true
	}
}

// dateRangeClause monta o predicado de periodo. O modo lote (batchStyle) compara
// data com as duas pontas inclusivas; o modo order_date usa fim exclusivo
// (< dia+1) para abarcar o dia inteiro de uma coluna timestamptz. Os placeholders
// sao posicoes de parametro ($3, $9, ...), nunca entrada do usuario.
func dateRangeClause(column string, batchStyle bool, fromPlaceholder, toPlaceholder string) string {
	if batchStyle {
		return fmt.Sprintf(
			"and (%s = '' or %s::date >= %s::date) and (%s = '' or %s::date <= %s::date)",
			fromPlaceholder, column, fromPlaceholder,
			toPlaceholder, column, toPlaceholder,
		)
	}
	return fmt.Sprintf(
		"and (%s = '' or %s >= %s::date) and (%s = '' or %s < (%s::date + 1))",
		fromPlaceholder, column, fromPlaceholder,
		toPlaceholder, column, toPlaceholder,
	)
}

// activeOrderCancelClause exclui, via anti-join, todo pedido que conste como
// cancelado. So se aplica a pedidos ATIVOS (erp_order_raw, alias o). A aba de
// cancelados (DataTypeOrderCanceled) nao recebe o anti-join. nullif(trim(...))
// nas duas pontas ignora order_id vazio (nunca casa NULL = NULL).
func activeOrderCancelClause(dataType string) string {
	if dataType != DataTypeOrder {
		return ""
	}
	return `
				and not exists (
					select 1 from erp_order_canceled_raw canceled
					where canceled.tenant_id = $1::uuid and canceled.store_id = $2::uuid
					  and nullif(trim(canceled.order_id), '') = nullif(trim(o.order_id), '')
				)`
}

// customerNameMatchSubquery casa o CPF do pedido (customer_id) com clientes cujo
// NOME/APELIDO bate o termo de busca, mas SO o do lote mais recente de cada CPF — o
// mesmo nome que a coluna "Nome" exibe (enrichOrderRecords). O recorte "lote mais
// recente" e' essencial: o CPF generico de "consumidor sem CPF" (ex.: 82541150016)
// tem milhares de nomes historicos; sem o recorte, buscar qualquer nome traria
// pedidos rotulados com outro nome.
//
// Implementado com DISTINCT ON (identifier) guiado pelo indice de recencia
// erp_customer_raw_tenant_identifier_recency_idx (0184): pega o ultimo lote por CPF
// via Index Scan, sem sort e sem hash. NAO usar NOT EXISTS aqui: o planner o resolvia
// como anti-join paralelo que hasheava as ~345k linhas e estourava o /dev/shm do
// Postgres (SQLSTATE 53100). `identifier <> ''` espelha o predicado parcial do indice.
// O placeholder do termo (%termo%) e' $4 na lista e $6 no stats; o tenant e' sempre $1.
func customerNameMatchSubquery(likePlaceholder string) string {
	return fmt.Sprintf(`customer_id in (
					select latest.identifier
					from (
						select distinct on (identifier) identifier, name, nickname
						from erp_customer_raw
						where tenant_id = $1::uuid and identifier <> ''
						order by identifier, source_batch_date desc, created_at_imported desc
					) latest
					where latest.name ilike %[1]s or latest.nickname ilike %[1]s
				)`, likePlaceholder)
}

// storeEmployeeFilterClause restringe pedidos por loja (uma ou mais chaves de loja
// separadas por virgula) e/ou consultor (employee_id exato). So faz sentido em
// order/ordercanceled (colunas store_id_raw/store_cnpj/employee_id no raw). A chave de
// loja e' o MESMO coalesce(nullif(store_id_raw,''), store_cnpj) que resolve o label
// exibido, entao o filtro casa exatamente com a coluna "Loja". Placeholders posicionais
// variam por ramo (count/list na lista; stats): sempre passados como os ULTIMOS args.
func storeEmployeeFilterClause(storePlaceholder, employeePlaceholder string) string {
	return fmt.Sprintf(`
					and (%[1]s = '' or coalesce(nullif(store_id_raw, ''), store_cnpj) = any(string_to_array(%[1]s, ',')))
					and (%[2]s = '' or employee_id = %[2]s)`, storePlaceholder, employeePlaceholder)
}

func (repository *PostgresRepository) GetRecordsStats(ctx context.Context, store StoreScope, query RecordsStatsQuery) (RecordsStatsResponse, error) {
	resp := RecordsStatsResponse{DataType: query.DataType}
	search := strings.TrimSpace(query.Search)
	likeSearch := "%" + search + "%"
	specificSearch := strings.TrimSpace(query.SpecificSearch)
	specificLike := specificSearch + "%"
	specificDigits := onlyDigits(specificSearch)
	specificDigitsLike := specificDigits + "%"

	switch query.DataType {
	case DataTypeOrder, DataTypeOrderCanceled:
		var tableName string
		if query.DataType == DataTypeOrderCanceled {
			tableName = "erp_order_canceled_raw"
		} else {
			tableName = "erp_order_raw"
		}

		dateColumn, batchStyle := resolveOrderDateColumn(query.DataType, query.DateField)
		dateClause := dateRangeClause(dateColumn, batchStyle, "$3", "$4")
		// cancelClause carrega tambem o filtro de loja/consultor ($12/$13), injetado no
		// mesmo %s da filtered CTE; args passados como os ultimos (apos o minvalue $11).
		cancelClause := activeOrderCancelClause(query.DataType) + storeEmployeeFilterClause("$12", "$13")

		sql := fmt.Sprintf(`
			WITH filtered AS NOT MATERIALIZED (
				SELECT *
				FROM %s o
				WHERE tenant_id = $1::uuid AND store_id = $2::uuid
					%s
					AND (
						$5 = ''
						OR order_id ilike $6
						OR identifier ilike $6
						OR store_id_raw ilike $6
						OR customer_id ilike $6
						OR order_date_raw ilike $6
						OR total_amount_raw ilike $6
						OR sku ilike $6
						OR amount_raw ilike $6
						OR quantity_raw ilike $6
						OR employee_id ilike $6
						OR payment_type ilike $6
						OR total_exclusion_raw ilike $6
						OR total_debit_raw ilike $6
						OR %s
					)
					AND (
						$7 = ''
						OR order_id ilike $8
						OR ($9 <> '' AND order_id ilike $10)
					)
					%s
			),
			latest_order_files AS (
				SELECT DISTINCT ON (order_id)
					order_id,
					file_id
				FROM filtered
				WHERE nullif(trim(order_id), '') is not null
				ORDER BY order_id, source_batch_date DESC, created_at_imported DESC, source_file_name DESC, file_id DESC
			),
			latest_order_lines AS (
				SELECT filtered.*
				FROM filtered
				JOIN latest_order_files latest
				  ON latest.order_id = filtered.order_id
				 AND latest.file_id = filtered.file_id
			),
			order_totals AS (
				SELECT
					order_id,
					coalesce(
						nullif(max(coalesce(total_amount_cents, 0)), 0),
						sum(coalesce(amount_cents, 0)),
						0
					)::bigint AS order_total_cents,
					coalesce(sum(case when coalesce(quantity, 0) > 0 then quantity else 1 end), 0)::bigint AS total_items
				FROM latest_order_lines
				GROUP BY order_id
			)
			SELECT
				count(*)::bigint AS order_count,
				coalesce(sum(order_total_cents), 0)::bigint AS total_amount_cents,
				CASE
					WHEN count(*) > 0 THEN (coalesce(sum(order_total_cents), 0)::bigint / count(*)::bigint)
					ELSE 0
				END AS avg_amount_cents,
				coalesce(sum(total_items), 0)::bigint AS total_items
			FROM order_totals
			WHERE ($11 = 0 OR order_total_cents >= $11);
		`, tableName, dateClause, customerNameMatchSubquery("$6"), cancelClause)

		var orderCount, totalAmountCents, avgAmountCents, totalItems int64
		err := repository.pool.QueryRow(ctx, sql,
			store.TenantID,
			store.StoreID,
			query.DateFrom,
			query.DateTo,
			search,
			likeSearch,
			specificSearch,
			specificLike,
			specificDigits,
			specificDigitsLike,
			query.MinValueCents,
			query.StoreFilter,
			query.EmployeeFilter,
		).Scan(&orderCount, &totalAmountCents, &avgAmountCents, &totalItems)
		if err != nil {
			return RecordsStatsResponse{}, err
		}

		resp.OrderCount = orderCount
		resp.TotalAmountCents = totalAmountCents
		resp.AvgAmountCents = avgAmountCents
		resp.TotalItems = totalItems
		if orderCount > 0 {
			resp.PA = float64(totalItems) / float64(orderCount)
		}

	case DataTypeCustomer:
		sql := `
			SELECT
				count(*),
				count(distinct coalesce(nullif(cpf, ''), id::text))
			FROM erp_customer_raw
			WHERE tenant_id = $1::uuid AND store_id = $2::uuid
				AND ($3 = '' OR source_batch_date >= $3::date)
				AND ($4 = '' OR source_batch_date <= $4::date)
				AND (
					$5 = ''
					OR name ilike $6
					OR nickname ilike $6
					OR cpf ilike $6
					OR email ilike $6
					OR phone ilike $6
					OR mobile ilike $6
					OR city ilike $6
					OR uf ilike $6
					OR zipcode ilike $6
					OR employee_id ilike $6
					OR store_id_raw ilike $6
					OR original_id ilike $6
					OR identifier ilike $6
					OR tags ilike $6
				)
				AND (
					$7 = ''
					OR cpf ilike $8
					OR ($9 <> '' AND regexp_replace(cpf, '\D', '', 'g') ilike $10)
				)
		`

		var totalCount, dedupedCount int64
		err := repository.pool.QueryRow(ctx, sql,
			store.TenantID,
			store.StoreID,
			query.DateFrom,
			query.DateTo,
			search,
			likeSearch,
			specificSearch,
			specificLike,
			specificDigits,
			specificDigitsLike,
		).Scan(&totalCount, &dedupedCount)
		if err != nil {
			return RecordsStatsResponse{}, err
		}

		resp.CustomerCount = dedupedCount

	default:
		return RecordsStatsResponse{}, ErrUnsupportedDataType
	}

	return resp, nil
}
