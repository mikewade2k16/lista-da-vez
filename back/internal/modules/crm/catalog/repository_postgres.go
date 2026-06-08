package catalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) SearchProducts(ctx context.Context, query SearchProductsQuery) (SearchProductsResponse, error) {
	source, ok := resolveProductSearchSource(query.SourceKey)
	if !ok {
		return SearchProductsResponse{}, ErrUnsupportedProductSource
	}

	sql, args := buildSearchProductsStatement(source, query)

	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return SearchProductsResponse{}, err
	}
	defer rows.Close()

	items := make([]ProductSearchItem, 0, query.Limit)
	for rows.Next() {
		var (
			item       ProductSearchItem
			priceCents int64
		)
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &priceCents); err != nil {
			return SearchProductsResponse{}, err
		}
		item.Price = float64(priceCents) / 100
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SearchProductsResponse{}, err
	}

	return SearchProductsResponse{
		SourceKey: query.SourceKey,
		Term:      query.Term,
		Limit:     query.Limit,
		Items:     items,
	}, nil
}

func buildSearchProductsStatement(source productSearchSource, query SearchProductsQuery) (string, []any) {
	switch source.Scope {
	case productSearchScopeTenantShared:
		sql := fmt.Sprintf(`
			with ranked as (
				select
					coalesce(%s, '') as id,
					coalesce(%s, '') as code,
					coalesce(%s, '') as name,
					coalesce(%s, 0) as price_cents,
					row_number() over (
						partition by coalesce(%s, '')
						order by store_id asc, coalesce(%s, '') asc, coalesce(%s, '') asc
					) as sku_rank
				from %s
				where tenant_id = $1::uuid
				  and (%s)
			)
			select
				id,
				code,
				name,
				price_cents
			from ranked
			where sku_rank = 1
			order by name asc, code asc
			limit $3;
		`,
			source.IDExpression,
			source.CodeExpression,
			source.NameExpression,
			source.PriceCentsExpr,
			source.CodeExpression,
			source.NameExpression,
			source.CodeExpression,
			source.TableName,
			buildPrefixSearchCondition(source.PrefixExpressions, "$2"),
		)
		return sql, []any{query.TenantID, query.Term + "%", query.Limit}
	default:
		sql := fmt.Sprintf(`
			with matched as (
				select
					coalesce(%s, '') as id,
					coalesce(%s, '') as code,
					coalesce(%s, '') as name,
					coalesce(%s, 0) as price_cents
				from %s
				where tenant_id = $1::uuid
				  and store_id = $2::uuid
				  and (%s)
			)
			select
				id,
				code,
				name,
				price_cents
			from matched
			order by name asc, code asc
			limit $4;
		`,
			source.IDExpression,
			source.CodeExpression,
			source.NameExpression,
			source.PriceCentsExpr,
			source.TableName,
			buildPrefixSearchCondition(source.PrefixExpressions, "$3"),
		)
		return sql, []any{query.TenantID, query.StoreID, query.Term + "%", query.Limit}
	}
}
