package cardapio

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Use only a disposable database: this test owns its minimal cardapio schema.
func TestBulkProductsPostgres(t *testing.T) {
	dsn := os.Getenv("CARDAPIO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CARDAPIO_TEST_DATABASE_URL requires a disposable PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `create schema cardapio;
		create table cardapio.products (
			id uuid primary key, account_id uuid not null, restaurant_id uuid not null,
			is_available boolean not null, is_featured boolean not null,
			updated_at timestamptz not null default now());`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	const account = "00000000-0000-0000-0000-000000000001"
	const restaurant = "00000000-0000-0000-0000-000000000002"
	const p1 = "00000000-0000-0000-0000-000000000003"
	const p2 = "00000000-0000-0000-0000-000000000004"
	const outside = "00000000-0000-0000-0000-000000000005"

	for _, action := range []ProductBulkAction{ProductBulkEnable, ProductBulkDisable, ProductBulkFeature, ProductBulkRemoveFeature, ProductBulkDelete} {
		t.Run(string(action), func(t *testing.T) {
			_, err := pool.Exec(ctx, `truncate cardapio.products;
				insert into cardapio.products(id,account_id,restaurant_id,is_available,is_featured)
				values ('`+p1+`','`+account+`','`+restaurant+`',true,true),
				('`+p2+`','`+account+`','`+restaurant+`',false,false),
				('`+outside+`','`+account+`','00000000-0000-0000-0000-000000000099',true,true);`)
			if err != nil {
				t.Fatal(err)
			}
			// A mixed-restaurant selection must roll back the entire action.
			if _, err := store.BulkProducts(ctx, account, restaurant, []string{p1, outside}, action); !errors.Is(err, ErrNotFound) {
				t.Fatalf("mixed selection: expected not found, got %v", err)
			}
			if _, err := store.BulkProducts(ctx, "00000000-0000-0000-0000-000000000098", restaurant, []string{p1}, action); !errors.Is(err, ErrNotFound) {
				t.Fatalf("wrong account: expected not found, got %v", err)
			}
			n, err := store.BulkProducts(ctx, account, restaurant, []string{p1, p2}, action)
			if err != nil || n != 2 {
				t.Fatalf("bulk action: affected=%d err=%v", n, err)
			}
			var outsideAvailable, outsideFeatured bool
			if err := pool.QueryRow(ctx, `select is_available,is_featured from cardapio.products where id=$1`, outside).Scan(&outsideAvailable, &outsideFeatured); err != nil || !outsideAvailable || !outsideFeatured {
				t.Fatalf("outside product changed: %v", err)
			}
			if action == ProductBulkDelete {
				var count int
				if err := pool.QueryRow(ctx, `select count(*) from cardapio.products where restaurant_id=$1`, restaurant).Scan(&count); err != nil || count != 0 {
					t.Fatalf("delete: count=%d err=%v", count, err)
				}
				return
			}
			var enabled, featured int
			if err := pool.QueryRow(ctx, `select count(*) filter(where is_available),count(*) filter(where is_featured) from cardapio.products where restaurant_id=$1`, restaurant).Scan(&enabled, &featured); err != nil {
				t.Fatal(err)
			}
			wantEnabled, wantFeatured := 1, 1
			switch action {
			case ProductBulkEnable:
				wantEnabled = 2
			case ProductBulkDisable:
				wantEnabled = 0
			case ProductBulkFeature:
				wantFeatured = 2
			case ProductBulkRemoveFeature:
				wantFeatured = 0
			}
			if enabled != wantEnabled || featured != wantFeatured {
				t.Fatalf("enabled=%d featured=%d, want %d/%d", enabled, featured, wantEnabled, wantFeatured)
			}
		})
	}
}
