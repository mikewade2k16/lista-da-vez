package site

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresProductErpRepository implementa ProductErpRepository cruzando
// site.products com a tabela publica erp_item_current.
//
// erp_item_current (sem schema) tem colunas sku, identifier, name, description,
// tenant_id, store_id. sku == identifier == codigo do produto. O escopo do ERP
// e tenant_id (== account_id do site). site.products.code pode conter varios
// codigos separados por '_'; cada segmento casa com um sku do ERP.
type PostgresProductErpRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresProductErpRepository cria o repo.
func NewPostgresProductErpRepository(pool *pgxpool.Pool) *PostgresProductErpRepository {
	return &PostgresProductErpRepository{pool: pool}
}

// erpMatchUpsertSQL faz o UPSERT em site.product_erp_links: para cada produto
// ativo da account, explode code em segmentos (string_to_array por '_') e junta
// com erp_item_current por sku == segmento e tenant_id == account_id do produto.
// distinct on (p.id, e.sku) garante 1 link por par produto+sku.
//
// $1 = accountID. Parametrizado.
const erpMatchUpsertSQL = `
insert into site.product_erp_links (account_id, product_id, erp_sku, erp_name, erp_description, matched_at)
select distinct on (p.id, e.sku)
       p.account_id, p.id, e.sku, e.name, e.description, now()
from site.products p
cross join lateral unnest(string_to_array(p.code, '_')) as seg(code)
join erp_item_current e
  on e.sku = nullif(trim(seg.code), '')
 and e.tenant_id = p.account_id::uuid
where p.account_id = $1::uuid
  and p.is_active = true
order by p.id, e.sku
on conflict (product_id, erp_sku) do update
   set erp_name = excluded.erp_name,
       erp_description = excluded.erp_description,
       matched_at = now()
`

// erpMatchDeleteOrphansSQL remove links cujo par produto/sku nao casa mais
// (produto inativo/removido, code alterado ou sku sumiu do ERP).
//
// $1 = accountID. Parametrizado.
const erpMatchDeleteOrphansSQL = `
delete from site.product_erp_links l
where l.account_id = $1::uuid
  and not exists (
      select 1
      from site.products p
      cross join lateral unnest(string_to_array(p.code, '_')) as seg(code)
      join erp_item_current e
        on e.sku = nullif(trim(seg.code), '')
       and e.tenant_id = p.account_id::uuid
      where p.id = l.product_id
        and p.account_id = l.account_id
        and p.is_active = true
        and e.sku = l.erp_sku
  )
`

// erpMatchCountSQL retorna {matched, products} apos o upsert/cleanup.
// $1 = accountID.
const erpMatchCountSQL = `
select count(*)::int as matched,
       count(distinct product_id)::int as products
from site.product_erp_links
where account_id = $1::uuid
`

func (r *PostgresProductErpRepository) MatchERP(ctx context.Context, accountID string) (ErpMatchResult, error) {
	if _, err := r.pool.Exec(ctx, erpMatchUpsertSQL, accountID); err != nil {
		return ErpMatchResult{}, err
	}
	if _, err := r.pool.Exec(ctx, erpMatchDeleteOrphansSQL, accountID); err != nil {
		return ErpMatchResult{}, err
	}
	var res ErpMatchResult
	if err := r.pool.QueryRow(ctx, erpMatchCountSQL, accountID).Scan(&res.Matched, &res.Products); err != nil {
		return ErpMatchResult{}, err
	}
	return res, nil
}

// erpMatchUpsertForProductSQL e a variante de erpMatchUpsertSQL escopada a um
// unico produto ($2 = productID), usada apos criar um produto a partir do ERP.
// $1 = accountID, $2 = productID.
const erpMatchUpsertForProductSQL = `
insert into site.product_erp_links (account_id, product_id, erp_sku, erp_name, erp_description, matched_at)
select distinct on (p.id, e.sku)
       p.account_id, p.id, e.sku, e.name, e.description, now()
from site.products p
cross join lateral unnest(string_to_array(p.code, '_')) as seg(code)
join erp_item_current e
  on e.sku = nullif(trim(seg.code), '')
 and e.tenant_id = p.account_id::uuid
where p.account_id = $1::uuid
  and p.id = $2::uuid
  and p.is_active = true
order by p.id, e.sku
on conflict (product_id, erp_sku) do update
   set erp_name = excluded.erp_name,
       erp_description = excluded.erp_description,
       matched_at = now()
`

func (r *PostgresProductErpRepository) MatchERPForProduct(ctx context.Context, accountID, productID string) error {
	_, err := r.pool.Exec(ctx, erpMatchUpsertForProductSQL, accountID, productID)
	return err
}

// erpUnmatchedBaseFrom e o FROM/WHERE compartilhado entre o count e o data de
// ListUnmatched: itens do ERP do tenant cujo sku NAO e segmento de nenhum code
// de produto ativo da account. distinct por sku.
//
// $1 = accountID. Filtro opcional q (sku/name ilike) entra como $2.
func erpUnmatchedWhere(q string) (string, []any) {
	args := []any{nil} // placeholder; $1 setado pelo caller
	where := []string{
		"e.tenant_id = $1::uuid",
		`not exists (
			select 1
			from site.products p
			cross join lateral unnest(string_to_array(p.code, '_')) as seg(code)
			where p.account_id = $1::uuid
			  and p.is_active = true
			  and nullif(trim(seg.code), '') = e.sku
		)`,
	}
	if strings.TrimSpace(q) != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
		where = append(where, "(lower(e.sku) like $2 or lower(e.name) like $2)")
		args = append(args, pattern)
	}
	return strings.Join(where, " and "), args
}

func (r *PostgresProductErpRepository) ListUnmatched(ctx context.Context, filter ErpUnmatchedFilter) ([]ErpUnmatchedItem, int, error) {
	where, args := erpUnmatchedWhere(filter.Q)
	args[0] = filter.AccountID

	countSQL := fmt.Sprintf(`
		select count(*) from (
			select distinct e.sku
			from erp_item_current e
			where %s
		) t
	`, where)

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	args = append(args, perPage, (page-1)*perPage)

	dataSQL := fmt.Sprintf(`
		select distinct on (e.sku) e.sku, e.name, e.description
		from erp_item_current e
		where %s
		order by e.sku
		limit $%d offset $%d
	`, where, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]ErpUnmatchedItem, 0)
	for rows.Next() {
		var item ErpUnmatchedItem
		var name, description *string
		if err := rows.Scan(&item.Sku, &name, &description); err != nil {
			return nil, 0, err
		}
		if name != nil {
			item.Name = *name
		}
		if description != nil {
			item.Description = *description
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *PostgresProductErpRepository) FindErpItem(ctx context.Context, accountID, sku string) (ErpUnmatchedItem, error) {
	const query = `
		select e.sku, e.name, e.description
		from erp_item_current e
		where e.tenant_id = $1::uuid and e.sku = $2
		limit 1
	`
	var item ErpUnmatchedItem
	var name, description *string
	err := r.pool.QueryRow(ctx, query, accountID, sku).Scan(&item.Sku, &name, &description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErpUnmatchedItem{}, ErrErpItemNotFound
		}
		return ErpUnmatchedItem{}, err
	}
	if name != nil {
		item.Name = *name
	}
	if description != nil {
		item.Description = *description
	}
	return item, nil
}
