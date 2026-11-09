package cardapio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This test owns a disposable database; never use the application database.
func TestLayoutPublicationPostgres(t *testing.T) {
	dsn := os.Getenv("CARDAPIO_LAYOUT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires a disposable layout database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if !strings.HasPrefix(pool.Config().ConnConfig.Database, "omni_cardapio_layout_test_") {
		t.Fatal("requires database prefix omni_cardapio_layout_test_")
	}
	_, err = pool.Exec(ctx, `create schema core;
 create table core.accounts(id uuid primary key);
 create schema cardapio;
 create table cardapio.restaurants(id uuid primary key);`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "platform", "database", "migrations", "0170_cardapio_site_layouts.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	const account = "00000000-0000-0000-0000-000000000001"
	const outside = "00000000-0000-0000-0000-000000000002"
	const restaurant = "00000000-0000-0000-0000-000000000003"
	if _, err = pool.Exec(ctx, `insert into core.accounts values ($1)`, account); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `insert into cardapio.restaurants values ($1)`, restaurant); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	var version int64
	for _, title := range []string{"Bari 🌿", "Bari", ""} {
		props, err := json.Marshal(map[string]string{"titleEm": title, "titleTail": ""})
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(SiteLayout{Pages: map[string]PageLayout{"home": {Page: "home", Blocks: []LayoutBlock{{ID: "hero", Type: "hero.cinematografico", Visible: true, Props: props}}}}})
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := validateSiteLayout(body)
		if err != nil {
			t.Fatal(err)
		}
		_, version, err = store.PutDraftLayout(ctx, account, restaurant, canonical, &version)
		if err != nil {
			t.Fatal(err)
		}
		// A new repository represents a reload, with no frontend state involved.
		fresh := NewStore(pool)
		draft, loadedVersion, found, err := fresh.GetDraftLayout(ctx, account, restaurant)
		if err != nil || !found || loadedVersion != version {
			t.Fatalf("reload: found=%v version=%d err=%v", found, loadedVersion, err)
		}
		assertLayoutTitle(t, draft, title)
		_, version, err = store.PublishLayout(ctx, account, restaurant)
		if err != nil {
			t.Fatal(err)
		}
		published, publicVersion, err := fresh.GetPublishedLayout(ctx, account, restaurant)
		if err != nil || publicVersion != version {
			t.Fatalf("public read: version=%d err=%v", publicVersion, err)
		}
		assertLayoutTitle(t, published, title)
	}
	if _, _, err := store.GetPublishedLayout(ctx, outside, restaurant); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant scope: %v", err)
	}
}

func assertLayoutTitle(t *testing.T, raw json.RawMessage, want string) {
	t.Helper()
	var layout SiteLayout
	if err := json.Unmarshal(raw, &layout); err != nil {
		t.Fatal(err)
	}
	var props map[string]string
	if err := json.Unmarshal(layout.Pages["home"].Blocks[0].Props, &props); err != nil {
		t.Fatal(err)
	}
	value, found := props["titleEm"]
	if !found || value != want || props["titleTail"] != "" {
		t.Fatalf("lost edit: %v want=%q", props, want)
	}
}
