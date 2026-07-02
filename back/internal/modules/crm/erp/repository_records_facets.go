package erp

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// GetRecordsFacets devolve as opcoes de filtro (lojas e consultores) presentes no
// escopo + periodo da aba Compras/Cancelados. As opcoes vem SEMPRE dos dados reais
// (distinct no raw), nunca de lista hardcoded. Nao aplica os proprios filtros de
// loja/consultor: o dropdown precisa mostrar todas as opcoes para o usuario trocar.
//
// Lojas: agrupadas pelo MESMO coalesce(nullif(store_id_raw,''), store_cnpj) que
// resolve o label exibido; varias chaves com o mesmo label (ex.: os 2 CNPJs de Treze)
// viram UMA opcao cujo value e' as chaves separadas por virgula (o que o filtro espera).
// Consultores: 1 opcao por employee_id, com o nome resolvido em lote.
func (repository *PostgresRepository) GetRecordsFacets(ctx context.Context, store StoreScope, query RecordsFacetsQuery) (RecordsFacetsResponse, error) {
	resp := RecordsFacetsResponse{DataType: query.DataType, Stores: []RecordsFacetOption{}, Employees: []RecordsFacetOption{}}

	var tableName string
	switch query.DataType {
	case DataTypeOrder:
		tableName = "erp_order_raw"
	case DataTypeOrderCanceled:
		tableName = "erp_order_canceled_raw"
	default:
		return resp, nil
	}

	dateColumn, batchStyle := resolveOrderDateColumn(query.DataType, query.DateField)
	dateClause := dateRangeClause(dateColumn, batchStyle, "$3", "$4")
	cancelClause := activeOrderCancelClause(query.DataType)
	args := []any{store.TenantID, store.StoreID, strings.TrimSpace(query.DateFrom), strings.TrimSpace(query.DateTo)}

	stores, err := repository.loadStoreFacets(ctx, tableName, dateClause, cancelClause, args)
	if err != nil {
		return RecordsFacetsResponse{}, err
	}
	// Facetas de consultor filtradas pela loja escolhida (cascata); as de loja acima
	// seguem completas para permitir a troca.
	employees, err := repository.loadEmployeeFacets(ctx, store.TenantID, tableName, dateClause, cancelClause, strings.TrimSpace(query.StoreFilter), args)
	if err != nil {
		return RecordsFacetsResponse{}, err
	}

	resp.Stores = stores
	resp.Employees = employees
	return resp, nil
}

func (repository *PostgresRepository) loadStoreFacets(ctx context.Context, tableName, dateClause, cancelClause string, args []any) ([]RecordsFacetOption, error) {
	sql := fmt.Sprintf(`
		select coalesce(nullif(store_id_raw, ''), store_cnpj) as store_key, count(*)::bigint
		from %s o
		where tenant_id = $1::uuid and store_id = $2::uuid
			%s
			%s
		group by store_key`, tableName, dateClause, cancelClause)

	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Agrupa por label: varias chaves (CNPJs) que resolvem o mesmo label viram 1 opcao.
	type storeGroup struct {
		keys  []string
		count int64
	}
	groups := make(map[string]*storeGroup)
	order := make([]string, 0)
	for rows.Next() {
		var storeKey string
		var count int64
		if err := rows.Scan(&storeKey, &count); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(storeKey)
		if key == "" {
			continue
		}
		label := resolveERPStoreLabel(key)
		group, exists := groups[label]
		if !exists {
			group = &storeGroup{}
			groups[label] = group
			order = append(order, label)
		}
		group.keys = append(group.keys, key)
		group.count += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	options := make([]RecordsFacetOption, 0, len(order))
	for _, label := range order {
		group := groups[label]
		options = append(options, RecordsFacetOption{
			Value: strings.Join(group.keys, ","),
			Label: label,
			Count: group.count,
		})
	}
	sort.SliceStable(options, func(i, j int) bool { return options[i].Count > options[j].Count })
	return options, nil
}

func (repository *PostgresRepository) loadEmployeeFacets(ctx context.Context, tenantID, tableName, dateClause, cancelClause, storeFilter string, args []any) ([]RecordsFacetOption, error) {
	// Copia local dos args: nao mutar o slice que loadStoreFacets tambem usa.
	queryArgs := append([]any{}, args...)
	storeClause := ""
	if storeFilter != "" {
		storeClause = fmt.Sprintf(
			"and coalesce(nullif(store_id_raw, ''), store_cnpj) = any(string_to_array($%d, ','))",
			len(queryArgs)+1,
		)
		queryArgs = append(queryArgs, storeFilter)
	}

	sql := fmt.Sprintf(`
		select employee_id, count(*)::bigint
		from %s o
		where tenant_id = $1::uuid and store_id = $2::uuid
			and nullif(trim(employee_id), '') is not null
			%s
			%s
			%s
		group by employee_id
		order by count(*) desc`, tableName, dateClause, cancelClause, storeClause)

	rows, err := repository.pool.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	counts := make(map[string]int64)
	for rows.Next() {
		var employeeID string
		var count int64
		if err := rows.Scan(&employeeID, &count); err != nil {
			return nil, err
		}
		id := strings.TrimSpace(employeeID)
		if id == "" {
			continue
		}
		if _, seen := counts[id]; !seen {
			ids = append(ids, id)
		}
		counts[id] += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	names, err := repository.loadEmployeeNamesByOriginalID(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}

	options := make([]RecordsFacetOption, 0, len(ids))
	for _, id := range ids {
		label := toTitleCase(names[id])
		if label == "" {
			label = id
		}
		options = append(options, RecordsFacetOption{Value: id, Label: label, Count: counts[id]})
	}
	return options, nil
}
