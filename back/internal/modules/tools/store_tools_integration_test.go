package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Use only a disposable database: this test creates a minimal core schema and
// applies the real tools migrations (0197 + 0328).
func TestToolsStoresPostgres(t *testing.T) {
	dsn := os.Getenv("TOOLS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TOOLS_TEST_DATABASE_URL requires a disposable PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}
	exec(`drop schema if exists tools cascade; drop schema if exists core cascade;
		create schema core;
		create table core.accounts (id uuid primary key, name text not null, is_active boolean not null default true);
		create table core.account_modules (account_id uuid not null, module_id text not null, enabled boolean not null,
			primary key (account_id, module_id));`)
	_, file, _, _ := runtime.Caller(0)
	migrations := filepath.Join(filepath.Dir(file), "..", "..", "platform", "database", "migrations")
	for _, name := range []string{"0197_tools_module.sql", "0328_tools_short_links_active.sql"} {
		sql, err := os.ReadFile(filepath.Join(migrations, name))
		if err != nil {
			t.Fatal(err)
		}
		exec(string(sql))
		exec(string(sql)) // idempotente: rodar de novo nao pode falhar
	}

	const acc = "00000000-0000-0000-0000-0000000000a1"
	const other = "00000000-0000-0000-0000-0000000000b2"
	exec(`insert into core.accounts values ($1,'Crow',true), ($2,'Outra',true)`, acc, other)
	exec(`insert into core.account_modules values ($1,'tools',true), ($2,'tools',true)`, acc, other)

	sl := NewPostgresShortLinkStore(pool)
	qr := NewPostgresQrCodeStore(pool)
	hits := func(slug string) (n int64, hasLast bool) {
		t.Helper()
		if err := pool.QueryRow(ctx, `select hits, last_hit_at is not null from tools.short_links where slug=$1`, slug).Scan(&n, &hasLast); err != nil {
			t.Fatal(err)
		}
		return
	}
	scans := func(slug string) (n int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `select scan_count from tools.qr_codes where slug=$1`, slug).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return
	}

	t.Run("link existente antes da 0328 nasce ativo", func(t *testing.T) {
		var active bool
		exec(`insert into tools.short_links (account_id, slug, target_url) values ($1,'legado','https://l.com')`, acc)
		if err := pool.QueryRow(ctx, `select is_active from tools.short_links where slug='legado'`).Scan(&active); err != nil || !active {
			t.Fatalf("esperado ativo, veio %v err=%v", active, err)
		}
	})

	link, err := sl.Create(ctx, acc, "promo", "https://destino.com")
	if err != nil || !link.IsActive || link.LastHitAt != "" {
		t.Fatalf("create: %+v err=%v", link, err)
	}

	t.Run("encurtador conta so quando count=true", func(t *testing.T) {
		for _, count := range []bool{true, false, false, true, true} {
			target, err := sl.Resolve(ctx, "promo", count)
			if err != nil || target != "https://destino.com" {
				t.Fatalf("resolve count=%v: %s %v", count, target, err)
			}
		}
		if n, last := hits("promo"); n != 3 || !last {
			t.Fatalf("hits=%d last=%v, esperado 3 e last_hit_at preenchido", n, last)
		}
	})

	t.Run("desativar bloqueia sem contar e reativar volta", func(t *testing.T) {
		before, _ := hits("promo")
		it, err := sl.Update(ctx, link.ID, acc, ShortLinkPatch{IsActive: boolp(false)})
		if err != nil || it.IsActive {
			t.Fatalf("desativar: %+v %v", it, err)
		}
		for _, count := range []bool{true, false} {
			if _, err := sl.Resolve(ctx, "promo", count); !errors.Is(err, ErrShortLinkNotFound) {
				t.Fatalf("inativo devia ser 404 (count=%v), veio %v", count, err)
			}
		}
		if n, _ := hits("promo"); n != before {
			t.Fatalf("acesso a link inativo contou: %d -> %d", before, n)
		}
		if it, err = sl.Update(ctx, link.ID, acc, ShortLinkPatch{IsActive: boolp(true)}); err != nil || !it.IsActive {
			t.Fatalf("reativar: %+v %v", it, err)
		}
		if _, err := sl.Resolve(ctx, "promo", true); err != nil {
			t.Fatalf("reativado devia resolver: %v", err)
		}
		if n, _ := hits("promo"); n != before+1 {
			t.Fatalf("hits=%d, esperado %d", n, before+1)
		}
	})

	t.Run("filtro de status do encurtador", func(t *testing.T) {
		off, _ := sl.Create(ctx, acc, "desligado", "https://x.com")
		if _, err := sl.Update(ctx, off.ID, acc, ShortLinkPatch{IsActive: boolp(false)}); err != nil {
			t.Fatal(err)
		}
		for status, want := range map[string]int{"active": 2, "inactive": 1, "": 3} {
			_, total, err := sl.List(ctx, acc, ListFilter{Status: status, Page: 1, Limit: 50})
			if err != nil || total != want {
				t.Fatalf("status=%q total=%d err=%v, esperado %d", status, total, err, want)
			}
		}
	})

	t.Run("isolamento do Get e do Update por conta", func(t *testing.T) {
		if _, err := sl.Get(ctx, link.ID, other); !errors.Is(err, ErrShortLinkNotFound) {
			t.Fatalf("Get de outra conta devia ser 404, veio %v", err)
		}
		if _, err := sl.Update(ctx, link.ID, other, ShortLinkPatch{IsActive: boolp(false)}); !errors.Is(err, ErrShortLinkNotFound) {
			t.Fatalf("Update de outra conta devia ser 404, veio %v", err)
		}
		if got, _ := sl.Get(ctx, link.ID, acc); !got.IsActive {
			t.Fatal("o Update de outra conta alterou o link")
		}
	})

	t.Run("modulo desligado ou conta inativa nao redireciona", func(t *testing.T) {
		exec(`update core.account_modules set enabled=false where account_id=$1`, acc)
		_, errMod := sl.Resolve(ctx, "promo", true)
		exec(`update core.account_modules set enabled=true where account_id=$1`, acc)
		exec(`update core.accounts set is_active=false where id=$1`, acc)
		_, errAcc := sl.Resolve(ctx, "promo", true)
		exec(`update core.accounts set is_active=true where id=$1`, acc)
		if !errors.Is(errMod, ErrShortLinkNotFound) || !errors.Is(errAcc, ErrShortLinkNotFound) {
			t.Fatalf("esperado 404: modulo=%v conta=%v", errMod, errAcc)
		}
	})

	code, err := qr.Create(ctx, acc, QrCodeRecord{Slug: "cardapio", TargetURL: "https://menu.com",
		FillColor: "#000000", BackColor: "#ffffff", Size: 220, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("QR conta so quando count=true e respeita is_active", func(t *testing.T) {
		for _, count := range []bool{true, false, true} {
			if _, err := qr.Resolve(ctx, "cardapio", count); err != nil {
				t.Fatal(err)
			}
		}
		if n := scans("cardapio"); n != 2 {
			t.Fatalf("scan_count=%d, esperado 2", n)
		}
		if _, err := qr.Update(ctx, code.ID, acc, QrCodePatch{IsActive: boolp(false)}); err != nil {
			t.Fatal(err)
		}
		if _, err := qr.Resolve(ctx, "cardapio", true); !errors.Is(err, ErrQrCodeNotFound) {
			t.Fatalf("QR inativo devia ser 404, veio %v", err)
		}
		if n := scans("cardapio"); n != 2 {
			t.Fatalf("scan em QR inativo contou: %d", n)
		}
		if _, err := qr.Update(ctx, code.ID, acc, QrCodePatch{IsActive: boolp(true)}); err != nil {
			t.Fatal(err)
		}
		if target, err := qr.Resolve(ctx, "cardapio", true); err != nil || target != "https://menu.com" {
			t.Fatalf("QR reativado: %s %v", target, err)
		}
	})

	t.Run("trava de cores: grava so se as cores atuais nao mudaram", func(t *testing.T) {
		fill, back := "#112233", "#ffffff"
		_, err := qr.Update(ctx, code.ID, acc, QrCodePatch{FillColor: &fill, BackColor: &back,
			ExpectFill: "#999999", ExpectBack: "#ffffff"})
		if !errors.Is(err, ErrConcurrentChange) {
			t.Fatalf("esperado ErrConcurrentChange, veio %v", err)
		}
		if got, _ := qr.Get(ctx, code.ID, acc); got.FillColor != "#000000" {
			t.Fatalf("gravou apesar do conflito: %s", got.FillColor)
		}
		got, err := qr.Update(ctx, code.ID, acc, QrCodePatch{FillColor: &fill, BackColor: &back,
			ExpectFill: "#000000", ExpectBack: "#ffffff"})
		if err != nil || got.FillColor != "#112233" {
			t.Fatalf("update com cores certas: %+v %v", got, err)
		}
		if _, err := qr.Update(ctx, code.ID, other, QrCodePatch{FillColor: &fill, BackColor: &back,
			ExpectFill: "#112233", ExpectBack: "#ffffff"}); !errors.Is(err, ErrQrCodeNotFound) {
			t.Fatalf("outra conta devia ser 404 (nao 409), veio %v", err)
		}
	})

	t.Run("slug do QR continua o mesmo depois de qualquer update", func(t *testing.T) {
		target := "https://novo.com"
		got, err := qr.Update(ctx, code.ID, acc, QrCodePatch{TargetURL: &target})
		if err != nil || got.Slug != "cardapio" {
			t.Fatalf("slug mudou: %+v %v", got, err)
		}
		if dest, err := qr.Resolve(ctx, "cardapio", false); err != nil || dest != target {
			t.Fatalf("mesmo QR devia ir pro novo destino: %s %v", dest, err)
		}
	})
}
