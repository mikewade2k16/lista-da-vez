package cardapio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Requires a dedicated empty database; never points to the application's database.
func TestProductTagsPostgres(t *testing.T) {
	dsn := os.Getenv("CARDAPIO_TAGS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CARDAPIO_TAGS_TEST_DATABASE_URL requires a disposable database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if !strings.HasPrefix(pool.Config().ConnConfig.Database, "omni_cardapio_tags_test_") {
		t.Fatal("database name must start with omni_cardapio_tags_test_")
	}
	if _, err := pool.Exec(ctx, `create schema core;
		create table core.accounts (id uuid primary key);`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"0153_cardapio_schema.sql", "0167_cardapio_restaurant_extra.sql",
		"0168_cardapio_catalog_optional_fields.sql", "0173_cardapio_category_banner.sql",
	} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "platform", "database", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	const account = "00000000-0000-0000-0000-000000000001"
	const outside = "00000000-0000-0000-0000-000000000002"
	if _, err := pool.Exec(ctx, `insert into core.accounts(id) values ($1), ($2)`, account, outside); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	svc := NewService(store, ServiceConfig{})
	restaurant, err := store.CreateRestaurant(ctx, account, "tags-test", "Tags test")
	if err != nil {
		t.Fatal(err)
	}
	in := ProductInput{Slug: "produto", Name: "Produto", PriceCents: 1200, IsAvailable: true}
	product, err := svc.CreateProduct(ctx, account, restaurant.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, tags := range [][]string{{" Novo ", "Picante", "novo", ""}, {}} {
		in.Tags = tags
		updated, err := svc.UpdateProduct(ctx, account, product.ID, in)
		if err != nil {
			t.Fatal(err)
		}
		want := normalizeProductTags(tags)
		full, err := svc.GetProduct(ctx, account, product.ID)
		if err != nil || !reflect.DeepEqual(full.Tags, want) {
			t.Fatalf("detail: tags=%v err=%v", full.Tags, err)
		}
		items, err := svc.ListProducts(ctx, account, restaurant.ID)
		if err != nil || len(items) != 1 {
			t.Fatalf("list: count=%d err=%v", len(items), err)
		}
		if !reflect.DeepEqual(items[0].Tags, want) || !reflect.DeepEqual(updated.Tags, want) {
			t.Fatalf("tags not persisted: list=%v updated=%v want=%v", items[0].Tags, updated.Tags, want)
		}
		if !items[0].CreatedAt.Equal(product.CreatedAt) || full.PriceCents != in.PriceCents {
			t.Fatal("tag update changed creation date or price")
		}
		encoded, err := json.Marshal(items[0])
		if err != nil || !strings.Contains(string(encoded), `"tags":[`) {
			t.Fatalf("list DTO must include a tags array: %s err=%v", encoded, err)
		}
	}
	if _, err := svc.ListProducts(ctx, outside, restaurant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant list: %v", err)
	}
	if _, err := svc.UpdateProduct(ctx, outside, product.ID, in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant write: %v", err)
	}
	items, err := store.ListProductsLean(ctx, outside, restaurant.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("repository tenant scope: count=%d err=%v", len(items), err)
	}
	items, err = store.ListProductsLean(ctx, account, "00000000-0000-0000-0000-000000000099")
	if err != nil || len(items) != 0 {
		t.Fatalf("repository restaurant scope: count=%d err=%v", len(items), err)
	}
}
